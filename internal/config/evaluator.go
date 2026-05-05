package config

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/apple/pkl-go/pkl"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	ghstarlark "github.com/fdatoo/switchyard/internal/starlark"
)

//go:embed pkl
var pklFS embed.FS

type pklEvaluator struct {
	ev pkl.Evaluator
}

func newPklEvaluator(ctx context.Context, driversRoot string) (*pklEvaluator, error) {
	ev, err := pkl.NewEvaluator(ctx, pkl.PreconfiguredOptions,
		pkl.WithModuleReader(&switchyardModuleReader{}),
		pkl.WithModuleReader(&driverModuleReader{root: driversRoot}),
		pkl.WithResourceReader(&starlarkValidatorReader{}),
	)
	if err != nil {
		return nil, fmt.Errorf("pkl evaluator: %w", err)
	}
	return &pklEvaluator{ev: ev}, nil
}

type configEvaluator interface {
	Evaluate(ctx context.Context, configDir string) (*configpb.ConfigSnapshot, error)
}

// switchyardModuleReader serves switchyard:* modules from the embedded FS.
type switchyardModuleReader struct{}

func (r *switchyardModuleReader) Scheme() string            { return "switchyard" }
func (r *switchyardModuleReader) IsGlobbable() bool         { return false }
func (r *switchyardModuleReader) HasHierarchicalUris() bool { return false }
func (r *switchyardModuleReader) IsLocal() bool             { return true }
func (r *switchyardModuleReader) ListElements(_ url.URL) ([]pkl.PathElement, error) {
	return nil, nil
}

func (r *switchyardModuleReader) Read(u url.URL) (string, error) {
	name := u.Opaque
	path := "pkl/switchyard/" + name + ".pkl"
	data, err := pklFS.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("switchyard module %q not found", name)
	}
	return string(data), nil
}

// starlarkValidatorReader is a Pkl ResourceReader that validates Starlark
// snippets embedded in config. It is invoked from typealias constraints in
// pkl/switchyard/starlark.pkl via `read("switchyard-validator:<fn>?<src>")`.
//
// The URI takes the form `switchyard-validator:<fn>?<url-encoded-source>`, where
// <fn> is one of "expr", "script", or "condition". The reader's body is the
// ASCII bytes "true" if the source parses, or "false" if it does not. Pkl's
// typealias constraint then fails evaluation for "false".
//
// Returning "false" (instead of returning an error) lets Pkl produce a normal
// constraint-violation error referencing the offending value, rather than a
// low-level reader error.
type starlarkValidatorReader struct{}

func (r *starlarkValidatorReader) Scheme() string            { return "switchyard-validator" }
func (r *starlarkValidatorReader) IsGlobbable() bool         { return false }
func (r *starlarkValidatorReader) HasHierarchicalUris() bool { return false }
func (r *starlarkValidatorReader) ListElements(_ url.URL) ([]pkl.PathElement, error) {
	return nil, nil
}

func (r *starlarkValidatorReader) Read(u url.URL) ([]byte, error) {
	// The URI is switchyard-validator:<fn>?<encoded-src>. With a non-hierarchical
	// URI, the "<fn>?<encoded-src>" portion lands in u.Opaque, and the raw
	// query may be empty (url.URL parses opaque URIs without splitting query).
	opaque := u.Opaque
	fn := opaque
	rawSrc := ""
	if i := strings.IndexByte(opaque, '?'); i >= 0 {
		fn = opaque[:i]
		rawSrc = opaque[i+1:]
	} else if u.RawQuery != "" {
		rawSrc = u.RawQuery
	}
	src, err := url.QueryUnescape(rawSrc)
	if err != nil {
		return []byte("false"), nil
	}
	var expr bool
	switch fn {
	case "expr", "condition":
		expr = true
	case "script":
		expr = false
	default:
		return []byte("false"), nil
	}
	if perr := ghstarlark.ParseOnly(src, expr); perr != nil {
		return []byte("false"), nil
	}
	return []byte("true"), nil
}

func (e *pklEvaluator) Evaluate(ctx context.Context, configDir string) (*configpb.ConfigSnapshot, error) {
	mainPath := configDir + "/main.pkl"
	text, err := e.ev.EvaluateOutputText(ctx, pkl.FileSource(mainPath))
	if err != nil {
		return nil, &EvalError{Message: err.Error()}
	}
	return parseConfigJSON(text, configDir)
}

type mcpConfigJSON struct {
	EvalResultMaxBytes       uint32 `json:"evalResultMaxBytes"`
	ReadFileMaxBytes         uint32 `json:"readFileMaxBytes"`
	EntitySubscriptionBuffer uint32 `json:"entitySubscriptionBuffer"`
	TraceSubscriptionBuffer  uint32 `json:"traceSubscriptionBuffer"`
	TailDefaultWaitSeconds   uint32 `json:"tailDefaultWaitSeconds"`
	TailMaxWaitSeconds       uint32 `json:"tailMaxWaitSeconds"`
}

type widgetPackPolicyJSON struct {
	AllowedSigners []string `json:"allowedSigners"`
	AllowUnsigned  bool     `json:"allowUnsigned"`
}

type configJSON struct {
	DriverInstances  []json.RawMessage    `json:"driverInstances"`
	Entities         []entityJSON         `json:"entities"`
	Automations      []automationJSON     `json:"automations"`
	Scripts          []scriptJSON         `json:"scripts"`
	Dashboards       []dashboardJSON      `json:"dashboards"`
	Users            []userJSON           `json:"users"`
	Roles            []roleJSON           `json:"roles"`
	Policies         []policyJSON         `json:"policies"`
	WidgetPackPolicy widgetPackPolicyJSON `json:"widgetPackPolicy"`
	AuthSettings     *authSettingsJSON    `json:"auth_settings"`
	Listener         listenerJSON         `json:"listener"`
	MCP              mcpConfigJSON        `json:"mcp"`
}

type listenerJSON struct {
	UDS                     udsListenerJSON   `json:"uds"`
	TCP                     tcpListenerJSON   `json:"tcp"`
	Webhooks                webhookConfigJSON `json:"webhooks"`
	StreamHeartbeatInterval string            `json:"streamHeartbeatInterval"` // "30.s"
}

type udsListenerJSON struct {
	Path string `json:"path"`
	Mode uint32 `json:"mode"`
}

type tcpListenerJSON struct {
	Bind string         `json:"bind"`
	TLS  *tlsConfigJSON `json:"tls"`
}

type tlsConfigJSON struct {
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
}

type webhookConfigJSON struct {
	MaxBodyBytes   int64    `json:"maxBodyBytes"`
	TrustedProxies []string `json:"trustedProxies"`
}

type entityJSON struct {
	ID           string `json:"id"`
	FriendlyName string `json:"friendlyName"`
	EntityType   string `json:"type"`
	Area         string `json:"area"`
}

// Pkl renders each typed class with a `_type` discriminator. We re-parse
// each automation / script by reading the raw JSON object and dispatching
// on this field.
type automationJSON struct {
	ID         string            `json:"id"`
	Enabled    bool              `json:"enabled"`
	Mode       string            `json:"mode"`
	MaxQueued  int32             `json:"maxQueued"`
	Triggers   []json.RawMessage `json:"triggers"`
	Conditions []json.RawMessage `json:"conditions"`
	Actions    []json.RawMessage `json:"actions"`
}

type scriptJSON struct {
	Name    string            `json:"name"`
	Params  []scriptParamJSON `json:"params"`
	Handler string            `json:"handler"`
}

type scriptParamJSON struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Default  string `json:"default"`
}

type typedNode struct {
	Type string `json:"_type"`
}

type dashboardJSON struct {
	Slug string `json:"slug"`
}

type userJSON struct {
	Slug                  string     `json:"slug"`
	DisplayName           string     `json:"display_name"`
	Roles                 []roleJSON `json:"roles"`
	Active                bool       `json:"active"`
	PasswordAllowed       bool       `json:"password_allowed"`
	PasskeyAllowed        bool       `json:"passkey_allowed"`
	OIDCSubject           *string    `json:"oidc_subject"`
	BootstrapPasswordHash *string    `json:"bootstrap_password_hash"`
}

type roleJSON struct {
	Slug        string     `json:"slug"`
	DisplayName string     `json:"display_name"`
	Inherits    []roleJSON `json:"inherits"`
}

type entitySelectorJSON struct {
	Areas     []string `json:"areas"`
	Classes   []string `json:"classes"`
	EntityIDs []string `json:"entity_ids"`
}

type capabilityRuleJSON struct {
	Verbs    []string           `json:"verbs"`
	Targets  entitySelectorJSON `json:"targets"`
	Services []string           `json:"services"`
}

type policyJSON struct {
	Name     string               `json:"name"`
	Subjects []roleJSON           `json:"subjects"`
	Allow    []capabilityRuleJSON `json:"allow"`
	Deny     []capabilityRuleJSON `json:"deny"`
}

type authSettingsJSON struct {
	PasswordLoginEnabled     bool     `json:"password_login_enabled"`
	PasskeyLoginEnabled      bool     `json:"passkey_login_enabled"`
	RpID                     string   `json:"rp_id"`
	RpDisplayName            string   `json:"rp_display_name"`
	RpOrigins                []string `json:"rp_origins"`
	WebAuthnUserVerification string   `json:"webauthn_user_verification"`
	Argon2idTime             uint32   `json:"argon2id_time"`
	Argon2idMemoryKib        uint32   `json:"argon2id_memory_kib"`
	Argon2idParallelism      uint32   `json:"argon2id_parallelism"`
	AccessCookieTTL          string   `json:"access_cookie_ttl"`
	RefreshCookieTTL         string   `json:"refresh_cookie_ttl"`
	RefreshIdleTTL           string   `json:"refresh_idle_ttl"`
	FailedAttemptsWindow     string   `json:"failed_attempts_window"`
	FailedAttemptsThreshold  uint32   `json:"failed_attempts_threshold"`
	FailedAttemptsBlock      string   `json:"failed_attempts_block"`
	TokenDefaultTTL          string   `json:"token_default_ttl"`
	TokenMaxTTL              string   `json:"token_max_ttl"`
	TokenLabelRequired       bool     `json:"token_label_required"`
	AccessCookieName         string   `json:"access_cookie_name"`
	RefreshCookieName        string   `json:"refresh_cookie_name"`
	RevealDeniedInExplain    bool     `json:"reveal_denied_in_explain"`
}

func roleJSONToProto(r roleJSON) *configpb.RoleConfig {
	pb := &configpb.RoleConfig{
		Slug:        r.Slug,
		DisplayName: r.DisplayName,
	}
	for _, inh := range r.Inherits {
		pb.Inherits = append(pb.Inherits, roleJSONToProto(inh))
	}
	return pb
}

func capabilityRuleJSONToProto(r capabilityRuleJSON) *configpb.CapabilityRule {
	return &configpb.CapabilityRule{
		Verbs:    r.Verbs,
		Services: r.Services,
		Targets: &configpb.EntitySelector{
			Areas:     r.Targets.Areas,
			Classes:   r.Targets.Classes,
			EntityIds: r.Targets.EntityIDs,
		},
	}
}

func parseConfigJSON(text, configDir string) (*configpb.ConfigSnapshot, error) {
	var raw configJSON
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, fmt.Errorf("parse config output: %w", err)
	}

	snap := &configpb.ConfigSnapshot{
		EvaluatedAtUnixMs: time.Now().UnixMilli(),
		ConfigDir:         configDir,
	}

	for _, rawInst := range raw.DriverInstances {
		var base struct {
			ID         string `json:"id"`
			DriverName string `json:"driverName"`
		}
		if err := json.Unmarshal(rawInst, &base); err != nil {
			return nil, fmt.Errorf("parse driver instance: %w", err)
		}
		h := sha256.Sum256(rawInst)
		// Binary is populated server-side in Manager.Apply by looking up the
		// driver registry. Per-instance enabled and lifecycle live in the
		// raw Params JSON and are decoded by parseInstanceOptions at apply time.
		snap.DriverInstances = append(snap.DriverInstances, &configpb.DriverInstanceConfig{
			Id:         base.ID,
			DriverName: base.DriverName,
			ConfigHash: h[:],
			Params:     rawInst,
		})
	}

	for _, e := range raw.Entities {
		snap.Entities = append(snap.Entities, &configpb.EntityConfig{
			Id:           e.ID,
			FriendlyName: e.FriendlyName,
			EntityType:   e.EntityType,
			Area:         e.Area,
		})
	}

	for _, a := range raw.Automations {
		acfg := &configpb.AutomationConfig{
			Id:        strings.TrimSpace(a.ID),
			Enabled:   a.Enabled,
			Mode:      parseAutomationMode(a.Mode),
			MaxQueued: a.MaxQueued,
		}
		for _, rawT := range a.Triggers {
			tc, err := decodeTrigger(rawT)
			if err != nil {
				return nil, fmt.Errorf("automation %q trigger: %w", a.ID, err)
			}
			acfg.Triggers = append(acfg.Triggers, tc)
		}
		for _, rawC := range a.Conditions {
			cc, err := decodeCondition(rawC)
			if err != nil {
				return nil, fmt.Errorf("automation %q condition: %w", a.ID, err)
			}
			acfg.Conditions = append(acfg.Conditions, cc)
		}
		for _, rawA := range a.Actions {
			ac, err := decodeAction(rawA)
			if err != nil {
				return nil, fmt.Errorf("automation %q action: %w", a.ID, err)
			}
			acfg.Actions = append(acfg.Actions, ac)
		}
		snap.Automations = append(snap.Automations, acfg)
	}

	for _, s := range raw.Scripts {
		scfg := &configpb.ScriptConfig{
			Name:    strings.TrimSpace(s.Name),
			Handler: s.Handler,
		}
		for _, p := range s.Params {
			scfg.Params = append(scfg.Params, &configpb.ScriptParam{
				Name:     p.Name,
				Type:     parseScriptParamType(p.Type),
				Required: p.Required,
				Default:  p.Default,
			})
		}
		snap.Scripts = append(snap.Scripts, scfg)
	}

	for _, d := range raw.Dashboards {
		b, _ := json.Marshal(d)
		snap.Dashboards = append(snap.Dashboards, &configpb.DashboardConfig{
			Slug:    d.Slug,
			Content: b,
		})
	}

	for _, u := range raw.Users {
		pbUser := &configpb.UserConfig{
			Slug:            u.Slug,
			DisplayName:     u.DisplayName,
			Active:          u.Active,
			PasswordAllowed: u.PasswordAllowed,
			PasskeyAllowed:  u.PasskeyAllowed,
		}
		if u.OIDCSubject != nil {
			pbUser.OidcSubject = *u.OIDCSubject
		}
		if u.BootstrapPasswordHash != nil {
			pbUser.BootstrapPasswordHash = *u.BootstrapPasswordHash
		}
		for _, r := range u.Roles {
			pbUser.Roles = append(pbUser.Roles, roleJSONToProto(r))
		}
		snap.Users = append(snap.Users, pbUser)
	}
	for _, r := range raw.Roles {
		snap.Roles = append(snap.Roles, roleJSONToProto(r))
	}
	for _, p := range raw.Policies {
		pbPolicy := &configpb.PolicyConfig{
			Name: p.Name,
		}
		for _, r := range p.Subjects {
			pbPolicy.Subjects = append(pbPolicy.Subjects, roleJSONToProto(r))
		}
		for _, rule := range p.Allow {
			pbPolicy.Allow = append(pbPolicy.Allow, capabilityRuleJSONToProto(rule))
		}
		for _, rule := range p.Deny {
			pbPolicy.Deny = append(pbPolicy.Deny, capabilityRuleJSONToProto(rule))
		}
		snap.Policies = append(snap.Policies, pbPolicy)
	}
	snap.WidgetPackPolicy = &configpb.WidgetPackPolicy{
		AllowedSigners: raw.WidgetPackPolicy.AllowedSigners,
		AllowUnsigned:  raw.WidgetPackPolicy.AllowUnsigned,
	}
	if raw.AuthSettings != nil {
		as := raw.AuthSettings
		pbAS := &configpb.AuthSettingsConfig{
			PasswordLoginEnabled:     as.PasswordLoginEnabled,
			PasskeyLoginEnabled:      as.PasskeyLoginEnabled,
			RpId:                     as.RpID,
			RpDisplayName:            as.RpDisplayName,
			RpOrigins:                as.RpOrigins,
			WebauthnUserVerification: as.WebAuthnUserVerification,
			Argon2IdTime:             as.Argon2idTime,
			Argon2IdMemoryKib:        as.Argon2idMemoryKib,
			Argon2IdParallelism:      as.Argon2idParallelism,
			FailedAttemptsThreshold:  as.FailedAttemptsThreshold,
			TokenLabelRequired:       as.TokenLabelRequired,
			AccessCookieName:         as.AccessCookieName,
			RefreshCookieName:        as.RefreshCookieName,
			RevealDeniedInExplain:    as.RevealDeniedInExplain,
		}
		durFields := []struct {
			name string
			raw  string
			dst  *int64
		}{
			{"access_cookie_ttl", as.AccessCookieTTL, &pbAS.AccessCookieTtlMs},
			{"refresh_cookie_ttl", as.RefreshCookieTTL, &pbAS.RefreshCookieTtlMs},
			{"refresh_idle_ttl", as.RefreshIdleTTL, &pbAS.RefreshIdleTtlMs},
			{"failed_attempts_window", as.FailedAttemptsWindow, &pbAS.FailedAttemptsWindowMs},
			{"failed_attempts_block", as.FailedAttemptsBlock, &pbAS.FailedAttemptsBlockMs},
			{"token_default_ttl", as.TokenDefaultTTL, &pbAS.TokenDefaultTtlMs},
			{"token_max_ttl", as.TokenMaxTTL, &pbAS.TokenMaxTtlMs},
		}
		for _, f := range durFields {
			d, err := parsePklDuration(f.raw)
			if err != nil {
				return nil, fmt.Errorf("auth_settings.%s: %w", f.name, err)
			}
			*f.dst = d.Milliseconds()
		}
		snap.AuthSettings = pbAS
	}

	hbDur, err := parsePklDuration(raw.Listener.StreamHeartbeatInterval)
	if err != nil {
		hbDur = 30 * time.Second // fallback to default
	}
	lc := &configpb.ListenerConfig{
		Uds: &configpb.UDSListenerConfig{
			Path: raw.Listener.UDS.Path,
			Mode: raw.Listener.UDS.Mode,
		},
		Tcp: &configpb.TCPListenerConfig{
			Bind: raw.Listener.TCP.Bind,
		},
		Webhooks: &configpb.WebhookListenerConfig{
			MaxBodyBytes:   raw.Listener.Webhooks.MaxBodyBytes,
			TrustedProxies: raw.Listener.Webhooks.TrustedProxies,
		},
		StreamHeartbeatIntervalMs: hbDur.Milliseconds(),
	}
	if raw.Listener.TCP.TLS != nil {
		lc.Tcp.Tls = &configpb.TLSListenerConfig{
			CertFile: raw.Listener.TCP.TLS.CertFile,
			KeyFile:  raw.Listener.TCP.TLS.KeyFile,
		}
	}
	snap.Listener = lc

	snap.Mcp = &configpb.MCPConfig{
		EvalResultMaxBytes:       raw.MCP.EvalResultMaxBytes,
		ReadFileMaxBytes:         raw.MCP.ReadFileMaxBytes,
		EntitySubscriptionBuffer: raw.MCP.EntitySubscriptionBuffer,
		TraceSubscriptionBuffer:  raw.MCP.TraceSubscriptionBuffer,
		TailDefaultWaitSeconds:   raw.MCP.TailDefaultWaitSeconds,
		TailMaxWaitSeconds:       raw.MCP.TailMaxWaitSeconds,
	}

	return snap, nil
}

// ValidateOffline evaluates the Pkl config in configDir and runs compile-time
// checks without connecting to a running daemon. Returns validation errors
// alongside any snapshot parse errors.
func ValidateOffline(ctx context.Context, configDir, driversRoot string) (*configpb.ConfigSnapshot, []ValidationError, error) {
	ev, err := newPklEvaluator(ctx, driversRoot)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = ev.ev.Close() }()

	snap, err := ev.Evaluate(ctx, configDir)
	if err != nil {
		return nil, nil, err
	}
	errs := Compile(snap, nil)
	return snap, errs, nil
}
