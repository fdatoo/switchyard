package daemon

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"

	configpb "github.com/fdatoo/gohome/gen/gohome/config/v1"
	entityv1 "github.com/fdatoo/gohome/gen/gohome/entity/v1"
	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/api"
	"github.com/fdatoo/gohome/internal/api/listener"
	"github.com/fdatoo/gohome/internal/auth"
	"github.com/fdatoo/gohome/internal/auth/audit"
	"github.com/fdatoo/gohome/internal/auth/credentials"
	"github.com/fdatoo/gohome/internal/auth/identity"
	"github.com/fdatoo/gohome/internal/auth/sessions"
	"github.com/fdatoo/gohome/internal/auth/throttle"
	"github.com/fdatoo/gohome/internal/automation"
	"github.com/fdatoo/gohome/internal/automation/action"
	"github.com/fdatoo/gohome/internal/carport"
	"github.com/fdatoo/gohome/internal/config"
	"github.com/fdatoo/gohome/internal/dashboard"
	"github.com/fdatoo/gohome/internal/eventstore"
	"github.com/fdatoo/gohome/internal/mcp"
	"github.com/fdatoo/gohome/internal/observability"
	"github.com/fdatoo/gohome/internal/policy"
	"github.com/fdatoo/gohome/internal/registry"
	"github.com/fdatoo/gohome/internal/script"
	starlark "github.com/fdatoo/gohome/internal/starlark"
	"github.com/fdatoo/gohome/internal/state"
	"github.com/fdatoo/gohome/internal/storage"
	"github.com/fdatoo/gohome/internal/web"
)

type Daemon struct {
	cfg              Config
	logger           *slog.Logger
	metrics          *observability.Metrics
	lockfile         *storage.Lockfile
	db               *sql.DB
	store            *eventstore.Store
	cache            *state.Cache
	registry         *registry.Registry
	carport          *carport.Host
	configMgr        *config.Manager
	starlarkRuntime  *starlark.Runtime
	scriptEngine     *script.Engine
	automationEngine *automation.Engine
	configDir        string

	phase        atomic.Int32
	recoveryInfo atomic.Pointer[recoveryState]
}

type recoveryState struct {
	reason string
}

// Version, Commit, and GoVersion are set via -ldflags at build time.
var (
	Version   = "dev"
	Commit    = "unknown"
	GoVersion = runtime.Version()
)

// New constructs an unstarted Daemon.
func New(cfg Config, logger *slog.Logger, metrics *observability.Metrics) *Daemon {
	cfg.WithDefaults()
	return &Daemon{cfg: cfg, logger: logger, metrics: metrics}
}

// Run boots through phases 1-5 and blocks until ctx is done.
func (d *Daemon) Run(ctx context.Context) error {
	d.metrics.SetBuildInfo(Version, Commit, GoVersion)
	start := time.Now()

	// Phase 1: cold open
	d.phase.Store(1)
	d.metrics.StartupPhase.Set(1)

	dataDir := expandHome(d.cfg.DataDir)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("mkdir data dir: %w", err)
	}
	lf, err := storage.AcquireLockfile(dataDir)
	if err != nil {
		return fmt.Errorf("lockfile: %w", err)
	}
	d.lockfile = lf
	defer func() { _ = d.lockfile.Release() }()

	db, err := storage.Open(ctx, storage.Config{Path: filepath.Join(dataDir, "gohome.db")})
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	d.db = db
	defer func() { _ = db.Close() }()

	go func() {
		_ = d.metrics.ServeMetrics(ctx, fmt.Sprintf(":%d", d.cfg.AdminPort), d.healthStatus)
	}()

	// Phase 2: construct projectors
	d.phase.Store(2)
	d.metrics.StartupPhase.Set(2)

	d.cache = state.New()
	reg, err := registry.New(ctx, db)
	if err != nil {
		return fmt.Errorf("registry: %w", err)
	}
	d.registry = reg

	// Auth subsystem — runs migrations here so the listener starts without delay.
	identityStore, err := identity.New(ctx, db)
	if err != nil {
		return fmt.Errorf("identity store: %w", err)
	}
	policyRuntime := policy.NewRuntime(roleAdapter{store: identityStore})
	sessKey := make([]byte, 32)
	if _, err := rand.Read(sessKey); err != nil {
		return fmt.Errorf("session key: %w", err)
	}
	sessStore := sessions.New(db, sessions.Config{
		Key:         sessKey,
		AccessTTL:   15 * time.Minute,
		RefreshTTL:  7 * 24 * time.Hour,
		RefreshIdle: 24 * time.Hour,
		AccessName:  "gohome_access",
		RefreshName: "gohome_refresh",
	})
	throttleStore := throttle.New(db, throttle.Config{
		Window:    15 * time.Minute,
		Threshold: 5,
		Block:     5 * time.Minute,
	})

	store, err := eventstore.Open(ctx, eventstore.Config{
		SnapshotEveryEvents: d.cfg.SnapshotEveryEvents,
		SnapshotEveryPeriod: d.cfg.SnapshotEveryPeriod,
	}, db, d.logger, d.metrics)
	if err != nil {
		return fmt.Errorf("eventstore: %w", err)
	}
	d.store = store

	if err := store.RegisterProjector(d.cache, eventstore.ProjectorModeSync); err != nil {
		return fmt.Errorf("register cache projector: %w", err)
	}
	if err := store.RegisterProjector(d.registry, eventstore.ProjectorModeSync); err != nil {
		return fmt.Errorf("register registry projector: %w", err)
	}

	// Phase 3: replay
	d.phase.Store(3)
	d.metrics.StartupPhase.Set(3)

	if err := store.Replay(ctx); err != nil {
		d.enterRecovery(err.Error())
		<-ctx.Done()
		return nil
	}

	// Phase 4: live transition
	d.phase.Store(4)
	d.metrics.StartupPhase.Set(4)

	if err := store.Start(ctx); err != nil {
		return fmt.Errorf("start eventstore: %w", err)
	}

	// Phase 4.5: carport — driver supervisor
	driversTOML := d.cfg.DriversTOMLPath
	if driversTOML == "@data/drivers.toml" {
		driversTOML = filepath.Join(dataDir, "drivers.toml")
	}
	socketDir := d.cfg.CarportSocketDir
	if socketDir == "@data/carport" {
		socketDir = filepath.Join(dataDir, "carport")
	}
	cport, err := carport.New(carport.HostConfig{
		DriversTOMLPath: driversTOML,
		SocketDir:       socketDir,
	}, d.db, d.store, d.registry, d.logger, d.metrics)
	if err != nil {
		return fmt.Errorf("carport: %w", err)
	}
	d.carport = cport
	if err := d.carport.Start(ctx); err != nil {
		return fmt.Errorf("carport start: %w", err)
	}

	// Phase 4.6: config — evaluate and apply Pkl config
	d.phase.Store(46)
	configDir := d.cfg.ConfigDir
	if configDir == "@data/config" {
		configDir = filepath.Join(dataDir, "config")
	}
	d.configDir = configDir

	// Only construct the config.Manager (which spawns a Pkl JVM subprocess) if
	// main.pkl exists. A brand-new install has no config yet; log and proceed
	// rather than refusing to start. A present-but-invalid main.pkl still errors.
	mainPkl := filepath.Join(configDir, "main.pkl")
	switch _, statErr := os.Stat(mainPkl); {
	case statErr == nil:
		cfgMgr, err := config.NewManager(ctx, configDir, d.store, &nopCarportManager{})
		if err != nil {
			return fmt.Errorf("config manager: %w", err)
		}
		d.configMgr = cfgMgr
		if err := d.configMgr.Apply(ctx, false); err != nil {
			d.logger.Error("initial config load failed", "err", err)
			return fmt.Errorf("config load: %w", err)
		}
		d.logger.Info("config applied", "config_dir", configDir)
	case os.IsNotExist(statErr):
		d.logger.Info("no config found — running with empty configuration", "config_dir", configDir, "main_pkl", mainPkl)
	default:
		return fmt.Errorf("stat main.pkl: %w", statErr)
	}

	// Construct Starlark runtime before serving the socket so that
	// starlark_eval requests never see a nil runtime.
	d.starlarkRuntime = starlark.NewRuntime(
		&stateAdapter{cache: d.cache},
		&carportAdapter{host: d.carport},
		d.store,
		d.logger,
		configDir,
		d.metrics,
	)
	d.logger.Info("starlark runtime initialised", "config_dir", configDir)

	// Construct script + automation engines from current config snapshot.
	currentSnap := &configpb.ConfigSnapshot{}
	if d.configMgr != nil {
		if snap := d.configMgr.Current(); snap != nil {
			currentSnap = snap
		}
	}
	scriptMap, err := script.CompileScripts(currentSnap)
	if err != nil {
		d.logger.Error("script compile failed", "err", err)
		scriptMap = map[string]*script.Script{}
	}
	d.scriptEngine = script.NewEngine(scriptMap, d.starlarkRuntime, script.Deps{
		Store: d.store, Logger: d.logger, Metrics: d.metrics,
	})
	autos, err := automation.CompileAutomations(currentSnap, d.scriptEngine, d.starlarkRuntime)
	if err != nil {
		d.logger.Error("automation compile failed", "err", err)
		autos = map[string]*automation.Automation{}
	}
	d.automationEngine = automation.NewEngine(autos, d.scriptEngine, d.starlarkRuntime, automation.Deps{ //nolint:contextcheck // holdFn closure in registerTriggers captures lifecycle context; callback signature is fixed
		State:      &stateAdapter{cache: d.cache},
		Dispatcher: &carportAdapter{host: d.carport},
		Store:      d.store,
		Scenes:     &action.StubSceneApplier{Store: d.store, Logger: d.logger},
		Logger:     d.logger,
		Metrics:    d.metrics,
	})
	if err := d.automationEngine.Start(ctx); err != nil {
		d.logger.Error("automation engine start", "err", err)
	}

	// Compile and apply policy from the initial config snapshot.
	{
		compileStart := time.Now()
		compiled, cerr := compilePolicyFromSnapshot(currentSnap)
		if d.metrics != nil {
			d.metrics.PolicyCompileDurationSeconds.Observe(time.Since(compileStart).Seconds())
			if cerr == nil {
				d.metrics.PolicyCompileGeneration.Set(float64(compiled.Generation))
			}
		}
		if cerr != nil {
			d.logger.Warn("policy compile failed on startup", "err", cerr)
		} else {
			policyRuntime.Replace(compiled)
		}
	}

	if d.configMgr != nil {
		d.configMgr.OnApplied(func(snap *configpb.ConfigSnapshot) { //nolint:contextcheck // Reload→registerTriggers closure captures lifecycle context; OnApplied callback receives no context
			if err := d.scriptEngine.Reload(snap); err != nil {
				d.logger.Warn("script reload", "err", err)
				return
			}
			if err := d.automationEngine.Reload(snap); err != nil {
				d.logger.Warn("automation reload", "err", err)
			}
			compileStart := time.Now()
			compiled, cerr := compilePolicyFromSnapshot(snap)
			if d.metrics != nil {
				d.metrics.PolicyCompileDurationSeconds.Observe(time.Since(compileStart).Seconds())
				if cerr == nil {
					d.metrics.PolicyCompileGeneration.Set(float64(compiled.Generation))
				}
			}
			if cerr != nil {
				d.logger.Warn("policy compile failed", "err", cerr)
			} else {
				policyRuntime.Replace(compiled)
			}
		})
	}

	// Wire API listener
	listenerCfg := &configpb.ListenerConfig{}
	if d.configMgr != nil {
		if snap := d.configMgr.Current(); snap != nil && snap.GetListener() != nil {
			listenerCfg = snap.GetListener()
		}
	}
	hbInterval := 30 * time.Second
	if listenerCfg.GetStreamHeartbeatIntervalMs() > 0 {
		hbInterval = time.Duration(listenerCfg.GetStreamHeartbeatIntervalMs()) * time.Millisecond
	}
	api.SetStreamConfig(api.StreamConfig{
		HeartbeatInterval: hbInterval,
		BufSize:           10000,
	})

	// Build adapters
	sysBE := &systemBackendAdapter{store: d.store, metrics: d.metrics, phase: d}
	entRd := &entityReaderAdapter{reg: d.registry, cache: d.cache}
	capCall := &capabilityCallerAdapter{sup: d.carport}
	devRd := &deviceReaderAdapter{reg: d.registry}
	devWr := &deviceWriterAdapter{reg: d.registry, store: d.store}
	areaRd := &areaReaderAdapter{reg: d.registry}
	zoneRd := &zoneReaderAdapter{reg: d.registry}
	drvCtl := &driverControlAdapter{sup: d.carport, reg: d.registry}
	evtSrc := &eventSourceAdapter{store: d.store}
	cfgAppl := &configApplierAdapter{mgr: d.configMgr}
	autoCtl := &automationControlAdapter{eng: d.automationEngine, store: d.store}
	scriptRun := &scriptRunnerAdapter{eng: d.scriptEngine, rt: d.starlarkRuntime, configDir: configDir}
	wbRouter := &webhookRouterAdapter{mgr: d.configMgr}
	wbApp := &webhookAppenderAdapter{store: d.store}

	entSvc := api.NewEntityService(entRd, capCall)
	// TODO: wire EntityStreamSource adapter when eventstore Subscribe is ready

	auditRecorder := audit.New(store)

	services := listener.Services{
		System:     api.NewSystemService(sysBE),
		Area:       api.NewAreaService(areaRd),
		Zone:       api.NewZoneService(zoneRd),
		Device:     api.NewDeviceService(devRd, devWr),
		Entity:     entSvc,
		Driver:     api.NewDriverService(drvCtl),
		Event:      api.NewEventService(evtSrc),
		Config:     api.NewConfigService(cfgAppl),
		Automation: api.NewAutomationService(autoCtl),
		Script:     api.NewScriptService(scriptRun, &eventAppenderAdapter{store: d.store}, sysBE),
		Scene:      api.NewSceneService(),
		Dashboard:  dashboard.NewService(newDashboardBackend(), dashboard.NewCatalog(nil)),
		Auth: api.NewAuthService(api.AuthDeps{
			Identity:   identityStore,
			Password:   credentials.NewPassword(db, credentials.DefaultArgon2idParams()),
			Tokens:     credentials.NewTokens(db),
			Sessions:   sessStore,
			Enrollment: credentials.NewEnrollment(db),
			Throttle:   throttleStore,
			Audit:      auditRecorder,
			Policy:     policyRuntime,
			Metrics:    d.metrics,
		}),
	}

	// TODO(C9): wire SO_PEERCRED into UDS connections so LocalPeerCred works.
	// acceptAllAuthn remains as a fallback until token/session auth is fully wired.
	authnChain := auth.Chain(auth.LocalPeerCred{}, acceptAllAuthn{})
	interceptors := []connect.Interceptor{
		listener.RecoverInterceptor(),
		listener.RequestIDInterceptor(),
		api.SourceInterceptor(),
		api.MCPInterceptor(d.metrics),
		listener.SlogInterceptor(),
		listener.MetricsInterceptor(d.metrics),
		api.NewAuthenticate(authnChain, nil, nil),
		api.NewAuthorize(policyRuntime, nil, auditRecorder, d.metrics),
	}

	routes := listener.BuildRoutes(services, interceptors...)
	wbHandler := api.NewWebhookHandler(wbRouter, wbApp, &webhookMetricsAdapter{m: d.metrics})

	udsPath := expandPath(listenerCfg.GetUds().GetPath(), dataDir)
	if udsPath == "" {
		udsPath = filepath.Join(dataDir, "gohomed.sock")
	}
	udsMode := os.FileMode(listenerCfg.GetUds().GetMode())
	if udsMode == 0 {
		udsMode = 0o600
	}
	tcpBind := listenerCfg.GetTcp().GetBind()
	if tcpBind == "" {
		tcpBind = "127.0.0.1:0"
	}

	mcpHTTPHandler := mcp.NewHTTPHandler(mcp.Deps{
		Version: Version,
	}, mcp.HTTPConfig{
		SessionIdleTimeout: 30 * time.Minute,
	})
	webHandler, err := web.NewHandler(web.Config{Version: Version})
	if err != nil {
		return fmt.Errorf("daemon: web handler: %w", err)
	}

	apiListener, err := listener.Build(listener.Config{
		UDSPath: udsPath,
		UDSMode: udsMode,
		TCPBind: tcpBind,
	}, listener.Deps{
		HealthProbe: func() error {
			_, code := d.healthStatus()
			if code != 200 {
				return fmt.Errorf("not ready")
			}
			return nil
		},
		ConnectRoutes:  routes,
		WebhookHandler: wbHandler,
		MCPHandler:     api.MCPAuthMiddleware(nil, mcpHTTPHandler),
		WebHandler:     webHandler,
	})
	if err != nil {
		return fmt.Errorf("daemon: build api listener: %w", err)
	}
	if err := apiListener.Start(ctx); err != nil {
		return fmt.Errorf("daemon: start api listener: %w", err)
	}
	defer func() { _ = apiListener.Shutdown(context.Background()) }() //nolint:contextcheck

	// Phase 5: all listeners up — health now returns 200.
	d.phase.Store(5)
	d.metrics.StartupPhase.Set(5)
	d.metrics.StartupDuration.Observe(time.Since(start).Seconds())

	// Subscribe to config.applied events so the Starlark module cache is
	// invalidated whenever configuration is re-applied.
	configSub, err := store.Subscribe(ctx, eventstore.SubscribeOptions{
		FromPosition: store.LatestPosition(),
		Filter:       eventstore.Filter{Kinds: []string{"config.applied"}},
	})
	if err != nil {
		d.logger.Warn("could not subscribe for config invalidation", "err", err)
	} else {
		go func() {
			defer func() { _ = configSub.Close() }()
			for range configSub.C() {
				d.starlarkRuntime.InvalidateModuleCache()
			}
		}()
	}

	if _, err := store.Append(ctx, eventstore.Event{
		Kind:      "system",
		Source:    "gohomed",
		Timestamp: time.Now(),
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_System{
			System: &eventv1.SystemEvent{Kind: "startup", Data: map[string]string{
				"version":    Version,
				"commit":     Commit,
				"go_version": GoVersion,
			}},
		}},
	}); err != nil {
		d.logger.Error("failed to append startup event", "err", err)
	}
	d.logger.Info("gohomed ready", "version", Version, "data_dir", dataDir, "admin_port", d.cfg.AdminPort)

	<-ctx.Done()
	d.logger.Info("shutdown requested")

	// shutCtx is derived from Background intentionally — the parent context is already cancelled at this point.
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := store.SnapshotNow(shutCtx, "state_cache"); err != nil { //nolint:contextcheck
		d.logger.Warn("final snapshot failed", "err", err)
	}
	if d.automationEngine != nil {
		d.automationEngine.Stop(shutCtx) //nolint:contextcheck
	}
	if d.scriptEngine != nil {
		_ = d.scriptEngine.Stop(shutCtx) //nolint:contextcheck
	}
	if d.carport != nil {
		d.carport.Stop(shutCtx) //nolint:contextcheck
	}
	_ = store.Close(ctx)
	return nil
}

func (d *Daemon) healthStatus() (string, int) {
	switch d.phase.Load() {
	case 5:
		return "ready", 200
	case -1:
		return "recovery", 503
	default:
		return "starting", 503
	}
}

func (d *Daemon) enterRecovery(reason string) {
	d.metrics.RecoveryModeEntered.Inc()
	d.phase.Store(-1)
	d.metrics.StartupPhase.Set(-1)
	d.recoveryInfo.Store(&recoveryState{reason: reason})
	d.logger.Error("entering recovery mode", "reason", reason)
}

func expandHome(path string) string {
	if len(path) == 0 || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}

func expandPath(path, dataDir string) string {
	if strings.HasPrefix(path, "@data/") {
		return filepath.Join(dataDir, path[len("@data/"):])
	}
	return path
}

// stateAdapter wraps state.Cache to satisfy starlark.StateReader.
type stateAdapter struct{ cache *state.Cache }

func (a *stateAdapter) Get(entityID string) (*starlark.EntityState, bool) {
	s, ok := a.cache.Get(entityID)
	if !ok {
		return nil, false
	}
	raw, _ := protojson.Marshal(s.Attributes)
	var attrs map[string]any
	_ = json.Unmarshal(raw, &attrs)
	if attrs == nil {
		attrs = map[string]any{}
	}
	stateStr := entityStateStr(s.Attributes)
	return &starlark.EntityState{StateStr: stateStr, Attributes: attrs}, true
}

func entityStateStr(a *entityv1.Attributes) string {
	if a == nil {
		return "unknown"
	}
	switch kind := a.GetKind().(type) {
	case *entityv1.Attributes_Light:
		if kind.Light.GetOn() {
			return "on"
		}
		return "off"
	case *entityv1.Attributes_SwitchDevice:
		if kind.SwitchDevice.GetOn() {
			return "on"
		}
		return "off"
	case *entityv1.Attributes_Sensor:
		return fmt.Sprintf("%g", kind.Sensor.GetValue())
	default:
		return "unknown"
	}
}

// carportAdapter wraps carport.Host to satisfy starlark.CommandDispatcher.
type carportAdapter struct{ host *carport.Host }

func (a *carportAdapter) Dispatch(ctx context.Context, entityID, capability string, args map[string]string) (*starlark.DispatchResult, error) {
	res, err := a.host.Dispatch(ctx, entityID, capability, args)
	if err != nil {
		return nil, err
	}
	return &starlark.DispatchResult{Ok: res.GetOk(), Error: res.GetErrorMessage()}, nil
}

// nopCarportManager satisfies config.CarportManager until carport.Host gains
// RegisterInstance/UnregisterInstance methods (C5+).
type nopCarportManager struct{}

func (n *nopCarportManager) RegisterInstance(_ context.Context, _, _ string, _ []byte) error {
	return nil
}
func (n *nopCarportManager) UnregisterInstance(_ context.Context, _ string) error {
	return nil
}
