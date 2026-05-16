package daemon

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
	"github.com/go-webauthn/webauthn/protocol"
	wa "github.com/go-webauthn/webauthn/webauthn"
	"google.golang.org/protobuf/encoding/protojson"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	entityv1 "github.com/fdatoo/switchyard/gen/switchyard/entity/v1"
	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/activity"
	"github.com/fdatoo/switchyard/internal/api"
	"github.com/fdatoo/switchyard/internal/api/listener"
	"github.com/fdatoo/switchyard/internal/auth"
	"github.com/fdatoo/switchyard/internal/auth/audit"
	"github.com/fdatoo/switchyard/internal/auth/authn"
	"github.com/fdatoo/switchyard/internal/auth/credentials"
	"github.com/fdatoo/switchyard/internal/auth/identity"
	"github.com/fdatoo/switchyard/internal/auth/sessions"
	"github.com/fdatoo/switchyard/internal/auth/throttle"
	"github.com/fdatoo/switchyard/internal/automation"
	"github.com/fdatoo/switchyard/internal/automation/action"
	"github.com/fdatoo/switchyard/internal/automation/scene"
	"github.com/fdatoo/switchyard/internal/carport"
	"github.com/fdatoo/switchyard/internal/commandcatalog"
	"github.com/fdatoo/switchyard/internal/config"
	"github.com/fdatoo/switchyard/internal/display"
	"github.com/fdatoo/switchyard/internal/driver"
	drvmgmt "github.com/fdatoo/switchyard/internal/driver/management"
	"github.com/fdatoo/switchyard/internal/editsession"
	"github.com/fdatoo/switchyard/internal/entity"
	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/mcp"
	"github.com/fdatoo/switchyard/internal/observability"
	"github.com/fdatoo/switchyard/internal/page"
	"github.com/fdatoo/switchyard/internal/pkl"
	"github.com/fdatoo/switchyard/internal/pkllsp"
	"github.com/fdatoo/switchyard/internal/policy"
	"github.com/fdatoo/switchyard/internal/registry"
	"github.com/fdatoo/switchyard/internal/replay"
	"github.com/fdatoo/switchyard/internal/script"
	"github.com/fdatoo/switchyard/internal/solar"
	starlark "github.com/fdatoo/switchyard/internal/starlark"
	"github.com/fdatoo/switchyard/internal/starlarkls"
	"github.com/fdatoo/switchyard/internal/state"
	"github.com/fdatoo/switchyard/internal/storage"
	"github.com/fdatoo/switchyard/internal/web"
	"github.com/fdatoo/switchyard/internal/widgetpack"
)

// Compile-time assertion: *carport.Host must satisfy config.CarportManager.
var _ config.CarportManager = (*carport.Host)(nil)

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
	sceneApplier     *scene.Applier
	configDir        string
	startTime        time.Time
	configReloader   *config.Reloader
	configPubsub     *config.ConfigPubsub

	phase          atomic.Int32
	recoveryInfo   atomic.Pointer[recoveryState]
	shutdownCancel atomic.Pointer[context.CancelFunc]
}

type recoveryState struct {
	reason    string
	failedPos uint64
}

// Compile-time assertion: *Daemon must satisfy RecoveryProvider.
var _ observability.RecoveryProvider = (*Daemon)(nil)

// Version, Commit, and GoVersion are set via -ldflags at build time.
var (
	Version   = "dev"
	Commit    = "unknown"
	GoVersion = runtime.Version()
)

// New constructs an unstarted Daemon.
func New(cfg Config, logger *slog.Logger, metrics *observability.Metrics) *Daemon {
	cfg.WithDefaults()
	return &Daemon{cfg: cfg, logger: logger, metrics: metrics, startTime: time.Now()}
}

// Run boots through phases 1-5 and blocks until ctx is done.
func (d *Daemon) Run(ctx context.Context) (err error) {
	if d.startTime.IsZero() {
		d.startTime = time.Now()
	}
	ctx, cancel := context.WithCancel(ctx)
	d.shutdownCancel.Store(&cancel)
	defer cancel()
	baseCtx := ctx

	var phaseSpan observability.Span
	endStartupPhase := func() {
		if phaseSpan == nil {
			return
		}
		phaseSpan.End()
		phaseSpan = nil
	}
	startStartupPhase := func(phase int32) {
		endStartupPhase()
		d.phase.Store(phase)
		d.metrics.StartupPhase.Set(float64(phase))
		ctx, phaseSpan = observability.StartSpan(baseCtx, "startup.phase")
		phaseSpan.SetAttr("phase", int(phase))
	}
	defer func() {
		if err != nil && phaseSpan != nil {
			phaseSpan.RecordError(err)
		}
		endStartupPhase()
	}()

	d.metrics.SetBuildInfo(Version, Commit, GoVersion)
	start := time.Now()

	// Phase 1: cold open
	startStartupPhase(1)

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

	db, err := storage.Open(ctx, storage.Config{Path: filepath.Join(dataDir, "switchyard.db")})
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	d.db = db
	defer func() { _ = db.Close() }()

	go func(ctx context.Context) {
		_ = d.metrics.ServeMetrics(ctx, fmt.Sprintf(":%d", d.cfg.AdminPort), d.healthStatus, d)
	}(baseCtx)

	// Phase 2: construct projectors
	startStartupPhase(2)

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
	passwordStore := credentials.NewPassword(db, credentials.DefaultArgon2idParams())
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
		AccessName:  "switchyard_access",
		RefreshName: "switchyard_refresh",
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
	startStartupPhase(3)

	if err := store.Replay(ctx); err != nil {
		phaseSpan.RecordError(err)
		var replayErr *eventstore.ReplayError
		var failedPos uint64
		if errors.As(err, &replayErr) {
			failedPos = replayErr.Position
		}
		d.enterRecovery(ctx, err.Error(), failedPos)
		<-ctx.Done()
		return nil
	}

	// Phase 4: live transition
	startStartupPhase(4)

	if err := store.Start(ctx); err != nil {
		return fmt.Errorf("start eventstore: %w", err)
	}

	// Phase 4.5: carport — driver supervisor
	socketDir := d.cfg.CarportSocketDir
	if socketDir == "@data/carport" {
		socketDir = filepath.Join(dataDir, "carport")
	}
	cport, err := carport.New(carport.HostConfig{
		SocketDir: socketDir,
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

	driversDir := d.cfg.DriversDir
	if driversDir == "" {
		driversDir = filepath.Join(dataDir, "drivers")
	} else {
		driversDir = expandHome(driversDir)
	}

	// One-shot deprecation log for users with a leftover drivers.toml.
	if _, err := os.Stat(filepath.Join(dataDir, "drivers.toml")); err == nil {
		d.logger.Warn("drivers.toml is no longer read; instances are configured in main.pkl",
			"path", filepath.Join(dataDir, "drivers.toml"),
		)
	}

	// Only construct the config.Manager (which spawns a Pkl JVM subprocess) if
	// main.pkl exists. A brand-new install has no config yet; log and proceed
	// rather than refusing to start. A present-but-invalid main.pkl still errors.
	mainPkl := filepath.Join(configDir, "main.pkl")
	switch _, statErr := os.Stat(mainPkl); {
	case statErr == nil:
		cfgMgr, err := config.NewManager(ctx, configDir, driversDir, d.store, d.carport)
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
	if err := applyAuthSnapshot(ctx, identityStore, passwordStore, currentSnap); err != nil {
		return fmt.Errorf("auth config: %w", err)
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
	d.sceneApplier = scene.NewApplier(
		d.configMgr,
		&carportAdapter{host: d.carport},
		d.store,
		&stateAdapter{cache: d.cache},
		&scriptCallerAdapter{eng: d.scriptEngine},
		d.starlarkRuntime,
		d.logger,
		d.metrics,
	)
	d.automationEngine = automation.NewEngine(autos, d.scriptEngine, d.starlarkRuntime, automation.Deps{ //nolint:contextcheck // holdFn closure in registerTriggers captures lifecycle context; callback signature is fixed
		State:      &stateAdapter{cache: d.cache},
		Dispatcher: &carportAdapter{host: d.carport},
		Store:      d.store,
		Scenes:     d.sceneApplier,
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
		// Reactive config subscription: pubsub + debouncing reloader.
		configPubsub := config.NewConfigPubsub(16)
		reloader := config.NewReloader(&managerReloaderApplier{mgr: d.configMgr}, 250*time.Millisecond)
		reloader.Start(ctx)
		d.configReloader = reloader
		d.configPubsub = configPubsub

		// Publish a ConfigChanged event on every successful Apply. v1 does not
		// suppress no-op applies (no bundle_hash field on ConfigSnapshot yet);
		// views re-fetching the same data is benign.
		d.configMgr.OnApplied(func(snap *configpb.ConfigSnapshot) {
			configPubsub.Publish(config.ConfigChangedEvent{
				AtUnixMs:   snap.GetEvaluatedAtUnixMs(),
				BundleHash: "",
			})
		})

		// File watcher for config-driven reloads
		watcher := config.NewWatcher(500 * time.Millisecond)
		watcher.Start(ctx)

		// Watch main.pkl and subdirectories
		watcher.Watch(filepath.Join(d.configDir, "main.pkl"))
		watcher.Watch(filepath.Join(d.configDir, "entity-areas.pkl"))
		for _, subdir := range []string{"automations", "areas", "scenes"} {
			subdir := filepath.Join(d.configDir, subdir)
			if err := filepath.WalkDir(subdir, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if !entry.IsDir() && strings.HasSuffix(path, ".pkl") {
					watcher.Watch(path)
				}
				return nil
			}); err != nil && !os.IsNotExist(err) {
				d.logger.Warn("error watching config dir", "dir", subdir, "err", err)
			}
		}

		// Filesystem-driven config reloads: every change to a *.pkl file under
		// configDir that affects the snapshot wakes the reloader. The reloader
		// itself debounces bursts and ignores no-op applies.
		if d.configReloader != nil {
			watcher.Subscribe(func(path, _ string, _ time.Time) {
				if isConfigRelevantPath(d.configDir, path) {
					d.configReloader.Trigger("watcher")
				}
			})
		}

		// Apply the initial snapshot's areas synchronously so the registry
		// has them by the time the API listener accepts requests. The
		// OnApplied callback below handles subsequent reloads.
		if initial := d.configMgr.Current(); initial != nil {
			syncAreasToRegistry(d.registry, initial)
		}
		d.configMgr.OnApplied(func(snap *configpb.ConfigSnapshot) { //nolint:contextcheck // Reload→registerTriggers closure captures lifecycle context; OnApplied callback receives no context
			syncAreasToRegistry(d.registry, snap)
			if err := applyAuthSnapshot(context.Background(), identityStore, passwordStore, snap); err != nil {
				d.logger.Warn("auth config reload", "err", err)
				return
			}
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
	cfgAppl := &configApplierAdapter{mgr: d.configMgr, reloader: d.configReloader, pubsub: d.configPubsub}
	autoCtl := &automationControlAdapter{eng: d.automationEngine, store: d.store}
	scriptRun := &scriptRunnerAdapter{eng: d.scriptEngine, rt: d.starlarkRuntime, configDir: configDir}
	wbRouter := &webhookRouterAdapter{mgr: d.configMgr}
	wbApp := &webhookAppenderAdapter{store: d.store}

	entSvc := api.NewEntityService(entRd, capCall)
	entSvc.SetStreamSource(&entityStreamSourceAdapter{store: d.store, reader: entRd})

	// Command catalog — register all domain verbs at startup (plan 05).
	cmdCatalogReg := commandcatalog.NewRegistry()
	activity.RegisterCommands(cmdCatalogReg)
	entity.RegisterCommands(cmdCatalogReg)
	automation.RegisterCommands(cmdCatalogReg)
	driver.RegisterCommands(cmdCatalogReg)
	config.RegisterCommands(cmdCatalogReg)
	pkl.RegisterCommands(cmdCatalogReg)
	page.RegisterCommands(cmdCatalogReg)
	widgetpack.RegisterCommands(cmdCatalogReg)
	auth.RegisterCommands(cmdCatalogReg)
	display.RegisterCommands(cmdCatalogReg)
	cmdCatalogSvc := commandcatalog.NewCommandCatalogService(cmdCatalogReg)

	auditRecorder := audit.New(store)

	// Widget pack subsystem (F-157).
	packStore := widgetpack.NewStore(filepath.Join(dataDir, "widgets"))
	if err := packStore.Load(ctx); err != nil {
		return fmt.Errorf("widget pack store: %w", err)
	}

	trustPolicy := &widgetpack.TrustPolicy{}
	// Initial trust policy from current config snapshot, if any.
	if d.configMgr != nil {
		if snap := d.configMgr.Current(); snap != nil {
			if p := snap.GetWidgetPackPolicy(); p != nil {
				if err := trustPolicy.Set(p.GetAllowedSigners(), p.GetAllowUnsigned()); err != nil {
					return fmt.Errorf("widget pack trust policy: %w", err)
				}
			}
		}
		// Hot-reload on config apply.
		d.configMgr.OnApplied(func(snap *configpb.ConfigSnapshot) {
			if p := snap.GetWidgetPackPolicy(); p != nil {
				if err := trustPolicy.Set(p.GetAllowedSigners(), p.GetAllowUnsigned()); err != nil {
					d.logger.Warn("widget pack: bad signer pattern in config", "err", err)
				}
			}
		})
	}

	packFetcher, err := widgetpack.NewFetcher()
	if err != nil {
		return fmt.Errorf("widget pack fetcher: %w", err)
	}

	// NewProductionVerifier is stubbed today (returns an error). Choice:
	// Option A — tolerate a nil verifier; Install() rejects signed packs with
	// ReasonSignatureInvalid while still allowing the unsigned-allowed flow.
	// This is the documented v1 behaviour until the TUF-backed trust root is
	// wired in a follow-up ticket.
	packVerifier, err := widgetpack.NewProductionVerifier(ctx)
	if err != nil {
		d.logger.Warn("widget pack: production verifier unavailable; signed packs cannot be verified", "err", err)
		packVerifier = nil
	}

	packInstaller := widgetpack.NewInstaller(
		packStore, packVerifier, trustPolicy, packFetcher, page.BuiltinClassIDs(), nil,
	)
	packService := widgetpack.NewService(packInstaller, packStore)
	packBundleHandler := widgetpack.NewBundleHandler(packStore)
	webAuthn, err := newWebAuthn(currentSnap.GetAuthSettings())
	if err != nil {
		return fmt.Errorf("webauthn: %w", err)
	}
	passkeys := credentials.NewPasskeys(db, webAuthn)
	webAuthnChallenges := credentials.NewChallengeStore(5 * time.Minute)
	tokens := credentials.NewTokens(db)
	bearer := authn.NewBearer(tokens)

	// EditSession subsystem — file watcher and lock manager with TTL sweep.
	editLockMgr := editsession.NewLockManager()
	editLockMgr.StartSweep(ctx)
	editFileWatcher := editsession.NewFileWatcher(0) // default poll interval
	editFileWatcher.Start(ctx)
	editSvc := editsession.NewService(editLockMgr, editFileWatcher, nil, d.logger, configDir)
	if d.configReloader != nil {
		editSvc.SetOnCommitTrigger(d.configReloader.Trigger)
	}

	pklNamespaceDir := filepath.Join(dataDir, "pkl-lsp", "switchyard")
	if err := config.ExportSwitchyardPklModules(pklNamespaceDir); err != nil {
		d.logger.Warn("pkllsp: failed to export switchyard Pkl namespace", "err", err)
	}
	pklLsSvc := pkllsp.NewService(pkllsp.Config{
		BinaryPath:             d.cfg.PklLspPath,
		ConfigDir:              configDir,
		SwitchyardNamespaceDir: pklNamespaceDir,
		Logger:                 d.logger,
	})

	// StarlarkLs subsystem — symbol extractor for scripts directory.
	starSyms, err := starlarkls.ExtractSymbols(filepath.Join(configDir, "scripts"))
	if err != nil {
		d.logger.Warn("starlarkls: symbol extraction failed (scripts dir may not exist yet)", "err", err)
		starSyms = map[string]starlarkls.SymbolInfo{}
	}
	starLsSvc := starlarkls.NewService(starSyms, filepath.Join(configDir, "scripts"))
	// Activity service (plan 03) — exposes Stories/Events/SavedQueries.
	// When SY_ACTIVITY_MOCK=1 is set, returns synthetic data.
	activitySvc := activity.NewActivityService(d.store, activity.ActivityServiceConfig{
		SavedQueriesDir: filepath.Join(dataDir, "saved-queries"),
	})

	// DriverManagement service (F-404) — settings → drivers, /devices, /devices/:id.
	drvMgmtSvc := drvmgmt.NewService(&driverMgmtRegistryAdapter{
		reg: d.registry,
		sup: d.carport,
	})

	// Replay service (plan 04) — time-machine replay via ReplayService.
	replayAdapter := replay.NewStoreAdapter(d.store)
	replaySvc := replay.NewService(replayAdapter, replayAdapter, replayAdapter, replayAdapter)

	services := listener.Services{
		System:         api.NewSystemService(sysBE),
		Area:           api.NewAreaService(areaRd),
		Zone:           api.NewZoneService(zoneRd),
		Device:         api.NewDeviceService(devRd, devWr),
		Entity:         entSvc,
		Driver:         api.NewDriverService(drvCtl),
		Event:          api.NewEventService(evtSrc),
		Config:         api.NewConfigService(cfgAppl),
		Automation:     api.NewAutomationService(autoCtl),
		Script:         api.NewScriptService(scriptRun, &eventAppenderAdapter{store: d.store}, sysBE),
		Scene:          api.NewRealSceneService(d.configMgr, &sceneInvokerAdapter{applier: d.sceneApplier}, d.logger),
		Page:           page.NewService(newPageBackend(configDir, driversDir, packStore), page.NewCatalog(nil)),
		WidgetPack:     packService,
		CommandCatalog: cmdCatalogSvc,
		Auth: api.NewAuthService(api.AuthDeps{
			Identity:   identityStore,
			Password:   passwordStore,
			Passkeys:   passkeys,
			Tokens:     tokens,
			Sessions:   sessStore,
			Enrollment: credentials.NewEnrollment(db),
			Challenges: webAuthnChallenges,
			Throttle:   throttleStore,
			Audit:      auditRecorder,
			Policy:     policyRuntime,
			Metrics:    d.metrics,
		}),
		EditSession:      editSvc,
		PklLs:            pklLsSvc,
		StarlarkLs:       starLsSvc,
		Activity:         activitySvc,
		DriverManagement: drvMgmtSvc,
		Replay:           replaySvc,
		Display:          display.NewService(filepath.Join(dataDir, "displays"), display.NewPairCodeStore()),
		Solar:            solar.NewService(),
	}

	authnChain := auth.Chain(auth.LocalPeerCred{}, bearer, authn.NewSessionCookie(sessStore), auth.RejectAll{})
	interceptors := []connect.Interceptor{
		listener.RecoverInterceptor(),
		listener.RequestIDInterceptor(),
		api.SourceInterceptor(),
		api.MCPInterceptor(d.metrics),
		listener.SlogInterceptor(),
		listener.MetricsInterceptor(d.metrics),
		api.NewAuthenticate(authnChain, bearer, tokens),
		api.NewAuthorize(policyRuntime, nil, auditRecorder, d.metrics),
	}

	routes := listener.BuildRoutes(services, interceptors...)
	wbHandler := api.NewWebhookHandler(wbRouter, wbApp, &webhookMetricsAdapter{m: d.metrics})

	udsPath := expandPath(listenerCfg.GetUds().GetPath(), dataDir)
	if udsPath == "" {
		udsPath = filepath.Join(dataDir, "switchyardd.sock")
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
		WidgetsHandler: packBundleHandler,
		WebHandler:     webHandler,
	})
	if err != nil {
		return fmt.Errorf("daemon: build api listener: %w", err)
	}
	if err := apiListener.Start(ctx); err != nil {
		return fmt.Errorf("daemon: start api listener: %w", err)
	}
	defer func() {
		if err := apiListener.Close(); err != nil {
			d.logger.Warn("api listener close failed", "err", err)
		}
	}()

	// Phase 5: all listeners up — health now returns 200.
	startStartupPhase(5)
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
		Source:    "switchyardd",
		Timestamp: time.Now(),
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_System{
			System: &eventv1.SystemEvent{Kind: "startup", Data: map[string]string{
				"version":    Version,
				"commit":     Commit,
				"go_version": GoVersion,
			}},
		}},
	}); err != nil {
		phaseSpan.RecordError(err)
		d.logger.Error("failed to append startup event", "err", err)
	}
	d.logger.Info("switchyardd ready", "version", Version, "data_dir", dataDir, "admin_port", d.cfg.AdminPort)
	endStartupPhase()
	ctx = baseCtx

	<-ctx.Done()
	d.logger.Info("shutdown requested")

	// shutCtx is derived from Background intentionally — the parent context is already cancelled at this point.
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := apiListener.Close(); err != nil {
		d.logger.Warn("api listener close failed", "err", err)
	}
	if err := pklLsSvc.Close(shutCtx); err != nil { //nolint:contextcheck
		d.logger.Warn("pkllsp shutdown failed", "err", err)
	}
	if d.configMgr != nil {
		if err := d.configMgr.Close(); err != nil {
			d.logger.Warn("config evaluator shutdown failed", "err", err)
		}
	}

	// Stop event-producing subsystems before snapshotting so the final
	// snapshot doesn't race their writes. SQLite returns BUSY (517) if a
	// writer holds the lock when SnapshotNow tries to start a write tx.
	if d.automationEngine != nil {
		d.automationEngine.Stop(shutCtx) //nolint:contextcheck
	}
	if d.scriptEngine != nil {
		_ = d.scriptEngine.Stop(shutCtx) //nolint:contextcheck
	}
	if d.carport != nil {
		d.carport.Stop(shutCtx) //nolint:contextcheck
	}

	if _, err := store.SnapshotNow(shutCtx, "state_cache"); err != nil { //nolint:contextcheck
		d.logger.Warn("final snapshot failed", "err", err)
	}
	_ = store.Close(shutCtx) //nolint:contextcheck
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

func (d *Daemon) enterRecovery(ctx context.Context, reason string, failedPos uint64) {
	d.metrics.RecoveryModeEntered.Inc()
	d.phase.Store(-1)
	d.metrics.StartupPhase.Set(-1)
	d.recoveryInfo.Store(&recoveryState{reason: reason, failedPos: failedPos})
	d.logger.Error("entering recovery mode", "reason", reason, "failed_position", failedPos)
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				d.logger.Error("daemon in recovery mode — operator action required",
					"reason", reason,
					"failed_position", failedPos,
				)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (d *Daemon) InRecovery() bool {
	return d.phase.Load() == -1
}

func (d *Daemon) RecoveryInfo() (string, uint64) {
	if info := d.recoveryInfo.Load(); info != nil {
		return info.reason, info.failedPos
	}
	return "", 0
}

// QueryEvents returns up to limit events starting from around position.
// Uses position - limit as the exclusive lower bound so the failing event is included.
func (d *Daemon) QueryEvents(ctx context.Context, position uint64, limit int) ([]observability.RecoveryEvent, error) {
	var from uint64
	if position > uint64(limit) {
		from = position - uint64(limit) - 1
	}
	events, err := d.store.Query(ctx, eventstore.QueryOptions{
		FromPosition: from,
		Limit:        limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]observability.RecoveryEvent, len(events))
	for i, e := range events {
		out[i] = observability.RecoveryEvent{
			Position:  e.Position,
			Timestamp: e.Timestamp,
			Kind:      e.Kind,
			Entity:    e.Entity,
			Source:    e.Source,
		}
	}
	return out, nil
}

func (d *Daemon) QueryProjectionCursors(ctx context.Context) ([]observability.ProjectionCursor, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT name, position, updated_at FROM projection_cursors ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []observability.ProjectionCursor
	for rows.Next() {
		var c observability.ProjectionCursor
		var updatedAtNano int64
		if err := rows.Scan(&c.Name, &c.Position, &updatedAtNano); err != nil {
			return nil, err
		}
		c.UpdatedAt = time.Unix(0, updatedAtNano)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *Daemon) QuerySkippedEvents(ctx context.Context) ([]observability.SkippedEvent, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT position, projector, skipped_at, skipped_by, reason FROM skipped_events ORDER BY position, projector`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []observability.SkippedEvent
	for rows.Next() {
		var e observability.SkippedEvent
		var skippedAtNano int64
		if err := rows.Scan(&e.Position, &e.Projector, &skippedAtNano, &e.SkippedBy, &e.Reason); err != nil {
			return nil, err
		}
		e.SkippedAt = time.Unix(0, skippedAtNano)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *Daemon) SkipEvent(ctx context.Context, position uint64, projector, reason, skippedBy string) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO skipped_events (position, projector, skipped_at, skipped_by, reason)
		VALUES (?, ?, ?, ?, ?)`,
		position, projector, time.Now().UnixNano(), skippedBy, reason,
	)
	return err
}

func (d *Daemon) ProjectorNames() []string {
	return d.store.ProjectorNames()
}

func (d *Daemon) Shutdown() {
	if fn := d.shutdownCancel.Load(); fn != nil {
		(*fn)()
	}
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

func newWebAuthn(settings *configpb.AuthSettingsConfig) (*wa.WebAuthn, error) {
	rpID := "localhost"
	rpDisplayName := "switchyard"
	rpOrigins := []string{"http://localhost", "https://localhost"}
	uv := protocol.VerificationPreferred
	if settings != nil {
		if settings.GetRpId() != "" {
			rpID = settings.GetRpId()
		}
		if settings.GetRpDisplayName() != "" {
			rpDisplayName = settings.GetRpDisplayName()
		}
		if len(settings.GetRpOrigins()) > 0 {
			rpOrigins = settings.GetRpOrigins()
		}
		switch settings.GetWebauthnUserVerification() {
		case "required":
			uv = protocol.VerificationRequired
		case "discouraged":
			uv = protocol.VerificationDiscouraged
		case "", "preferred":
			uv = protocol.VerificationPreferred
		default:
			return nil, fmt.Errorf("invalid webauthn_user_verification %q", settings.GetWebauthnUserVerification())
		}
	}
	return wa.New(&wa.Config{
		RPID:          rpID,
		RPDisplayName: rpDisplayName,
		RPOrigins:     rpOrigins,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: uv,
		},
	})
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
	case *entityv1.Attributes_NumericSensor:
		return fmt.Sprintf("%g", kind.NumericSensor.GetValue())
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

// scriptCallerAdapter wraps *script.Engine to satisfy action.ScriptCaller.
// script.Engine.Call returns *script.CallResult; action.ScriptCaller expects
// the return to implement action.ScriptCallResult (an interface).
type scriptCallerAdapter struct{ eng *script.Engine }

func (a *scriptCallerAdapter) Call(ctx context.Context, name string, args map[string]string, invokedBy, sharedCorrID string) (action.ScriptCallResult, error) {
	res, err := a.eng.Call(ctx, name, args, invokedBy, sharedCorrID)
	if err != nil {
		return nil, err
	}
	return &scriptCallResultAdapter{r: res}, nil
}

type scriptCallResultAdapter struct{ r *script.CallResult }

func (s *scriptCallResultAdapter) Succeeded() bool   { return s.r.Error == "" }
func (s *scriptCallResultAdapter) GetError() string  { return s.r.Error }
func (s *scriptCallResultAdapter) GetSteps() uint64  { return s.r.Steps }
func (s *scriptCallResultAdapter) GetLogs() []string { return s.r.Logs }

// isConfigRelevantPath returns true if the watched path is one of the
// files the daemon's config snapshot depends on: main.pkl or any .pkl
// under automations/, areas/, scenes/, or the entity-areas.pkl singleton.
func isConfigRelevantPath(configDir, path string) bool {
	rel, err := filepath.Rel(configDir, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	if rel == "main.pkl" || rel == "entity-areas.pkl" {
		return true
	}
	if strings.HasPrefix(rel, "automations/") && strings.HasSuffix(rel, ".pkl") {
		return true
	}
	if strings.HasPrefix(rel, "areas/") && strings.HasSuffix(rel, ".pkl") {
		return true
	}
	if strings.HasPrefix(rel, "scenes/") && strings.HasSuffix(rel, ".pkl") {
		return true
	}
	return false
}
