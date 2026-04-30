# C7 — Connect-RPC API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the first user-facing wire surface of `gohomed` — a Connect-RPC API defined as `gohome.v1alpha1.*`, served over UDS and TCP with Connect/gRPC/gRPC-Web, exercised by a rewritten `gohome` CLI. Close C6's webhook deferral. Retire the JSON-op surface.

**Architecture:** Proto sources under `proto/gohome/v1alpha1/`. One `internal/api/` Go package of thin handlers against interface-typed backends (engines from C1-C6 stay as source of truth). One `http.Server` bound to UDS + TCP via `h2c`. Shared interceptor stack for recover, request-id, slog, metrics, tracing, authenticate, authorize. `internal/auth` defines the seam with a local-peer-cred stub; C9 replaces. Webhook receiver lives on the same mux as Connect; emits a new `WebhookReceived` event that drives C6's `WebhookTrigger` matcher.

**Tech Stack:** Go 1.25, `connectrpc.com/connect`, `golang.org/x/net/http2` + `golang.org/x/net/http2/h2c`, `connectrpc.com/otelconnect`, existing `internal/eventstore`, `internal/state`, `internal/registry`, `internal/carport`, `internal/config`, `internal/automation`, `internal/script`, `internal/observability`. Buf remote plugin `buf.build/connectrpc/go` for Connect interface generation.

---

## Codebase orientation

Before starting, read these files to understand existing patterns:

| File | Why |
|---|---|
| `internal/daemon/daemon.go` | Where the API listener is wired. `serveSocket` lives here today; C7 replaces it |
| `internal/daemon/recovery.go` | Current JSON-op surface. Deleted in Task 25 |
| `internal/cli/cliutil.go` | Current `sendReq` helper. Replaced in Task 22 |
| `internal/cli/cmd_automation.go` | CLI command + rendering pattern |
| `internal/cli/styles.go`, `styles_automation.go` | Lipgloss styles used by CLI output |
| `internal/config/pkl/gohome/` | Pkl module layout; listener config lives in `config.pkl` or a new `core.pkl` |
| `buf.yaml`, `buf.gen.yaml` | Current buf config; Task 1 adds the Connect plugin |
| `proto/gohome/event/v1/event.proto` | Has `Payload` oneof to extend (new event kinds in Task 2) |
| `docs/proto-hygiene.md` (in `gohome/`) | Grouped-numbering + reserved-forever rules |
| `internal/automation/trigger/event.go` | `WebhookTrigger` matcher — Task 18 activates its wiring |
| `internal/observability/metrics.go` | Pattern for registering Prometheus metrics |
| `internal/observability/logging.go` | `slog` handler setup |

---

## File map

### New files

| Path | Responsibility |
|---|---|
| `proto/gohome/v1alpha1/common.proto` | `PageRequest`, `PageResponse`, `EntitySelector`, `Heartbeat`, `EventFilter` |
| `proto/gohome/v1alpha1/system.proto` | `SystemService` + types |
| `proto/gohome/v1alpha1/area.proto` | `AreaService` + types |
| `proto/gohome/v1alpha1/zone.proto` | `ZoneService` + types |
| `proto/gohome/v1alpha1/device.proto` | `DeviceService` + types |
| `proto/gohome/v1alpha1/entity.proto` | `EntityService` + types |
| `proto/gohome/v1alpha1/driver.proto` | `DriverService` + types |
| `proto/gohome/v1alpha1/event.proto` | `EventService` + types |
| `proto/gohome/v1alpha1/config.proto` | `ConfigService` + types |
| `proto/gohome/v1alpha1/automation.proto` | `AutomationService` + types |
| `proto/gohome/v1alpha1/script.proto` | `ScriptService` + types |
| `proto/gohome/v1alpha1/scene.proto` | `SceneService` (UNIMPLEMENTED) |
| `proto/gohome/v1alpha1/dashboard.proto` | `DashboardService` (UNIMPLEMENTED) |
| `proto/gohome/v1alpha1/auth.proto` | `AuthService` (UNIMPLEMENTED) |
| `proto/gohome/error/v1alpha1/error.proto` | `ErrorDetail` |
| `internal/auth/auth.go` | `Principal`, `Authenticator`, `Authorizer`, `Action`, `Target`, context helpers |
| `internal/auth/local.go` | `LocalPeerCredAuthenticator`, `AllowAllAuthorizer`, `Chain` |
| `internal/auth/reject.go` | `RejectAllAuthenticator` |
| `internal/auth/auth_test.go` | Tests for auth stubs + chain |
| `internal/api/deps.go` | Backend interfaces (`EntityReader`, `EventSource`, `AutomationControl`, ...) |
| `internal/api/errors.go` | Domain-error → `connect.Error` mapping + `ErrorDetail` catalog |
| `internal/api/pagination.go` | Opaque cursor codec |
| `internal/api/time.go` | Timestamp conversion |
| `internal/api/streaming.go` | Heartbeat ticker + bounded fan-out |
| `internal/api/service_system.go` | `SystemService` impl |
| `internal/api/service_area.go` | `AreaService` impl |
| `internal/api/service_zone.go` | `ZoneService` impl |
| `internal/api/service_device.go` | `DeviceService` impl |
| `internal/api/service_entity.go` | `EntityService` impl |
| `internal/api/service_driver.go` | `DriverService` impl |
| `internal/api/service_event.go` | `EventService` impl |
| `internal/api/service_config.go` | `ConfigService` impl |
| `internal/api/service_automation.go` | `AutomationService` impl |
| `internal/api/service_script.go` | `ScriptService` impl |
| `internal/api/service_unimplemented.go` | Scene/Dashboard/Auth stubs |
| `internal/api/webhook.go` | `/webhooks/{slug}` handler |
| `internal/api/fakes_test.go` | Shared fakes for handler tests |
| `internal/api/service_*_test.go` | One per live service |
| `internal/api/listener/listener.go` | `http.Server` build, mux, UDS + TCP listeners, shutdown |
| `internal/api/listener/h2c.go` | Plaintext HTTP/2 setup |
| `internal/api/listener/interceptors.go` | Request-id, slog, metrics, tracing, recover, authenticate, authorize |
| `internal/api/listener/interceptors_test.go` | Per-interceptor tests |
| `internal/api/listener/listener_integration_test.go` | End-to-end with real listener (integration build tag) |

### Modified files

| Path | Change |
|---|---|
| `go.mod`, `go.sum` | Add `connectrpc.com/connect`, `golang.org/x/net/http2`, `connectrpc.com/otelconnect` |
| `buf.gen.yaml` | Add `buf.build/connectrpc/go` plugin |
| `proto/gohome/event/v1/event.proto` | New payload kinds: `WebhookReceived` (60), `DeviceRenamed` (70), `DeviceReassigned` (71), `DriverInstanceRestarted` (80) |
| `internal/config/pkl/gohome/*.pkl` | New `Listener` class in core config module |
| `internal/daemon/daemon.go` | Construct listener, wire all services, replace `serveSocket` call |
| `internal/daemon/recovery.go` | Delete JSON-op switch (last step of CLI rewrite) |
| `internal/cli/cliutil.go` | Replace `sendReq` with Connect dialer helper |
| `internal/cli/cmd_automation.go` | Port to `AutomationService` client |
| `internal/cli/cmd_script.go` | Port to `ScriptService` client |
| `internal/cli/snapshot.go` | Port to `SystemService.CreateSnapshot` |
| `internal/cli/eval.go` | Port to `ScriptService.Eval` |
| `internal/cli/test.go` | Port to `ScriptService.RunTests` |
| `internal/cli/driver.go` | Port to `DriverService` |
| `internal/cli/events.go` | Port to `EventService` |
| `internal/cli/state.go` | Port to `EntityService.Get` |
| `internal/cli/config.go` | Port to `ConfigService` |
| `internal/cli/root.go` | Add `--endpoint` / `GOHOME_ENDPOINT` flag |
| `internal/automation/trigger/event.go` | Subscribe `WebhookTrigger` matchers to `WebhookReceived` payload |
| `internal/observability/metrics.go` | Register `gohome_api_*` metrics |

---

## Task 1: Dependencies and Buf Connect plugin

**Files:**
- Modify: `go.mod`, `go.sum`, `buf.gen.yaml`

- [ ] **Step 1: Add Go dependencies**

```bash
cd /path/to/gohome
go get connectrpc.com/connect@latest
go get connectrpc.com/otelconnect@latest
go get golang.org/x/net/http2@latest
go mod tidy
```

- [ ] **Step 2: Extend `buf.gen.yaml` with the Connect plugin**

Replace the `plugins:` block in `buf.gen.yaml` with:

```yaml
plugins:
  - remote: buf.build/protocolbuffers/go
    out: gen
    opt: paths=source_relative
  - remote: buf.build/connectrpc/go
    out: gen
    opt: paths=source_relative
  - local: protoc-gen-go-grpc
    out: gen
    opt: paths=source_relative,require_unimplemented_servers=false
```

Note: the `protoc-gen-go-grpc` plugin stays — it generates the gRPC interfaces that Connect can serve alongside Connect's own. `require_unimplemented_servers=false` ensures we don't need to embed `UnimplementedXServer` in every impl.

- [ ] **Step 3: Verify build compiles**

```bash
task build
```

Expected: both binaries build with no errors.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum buf.gen.yaml
git commit -m "feat(c7): add connect-go and buf Connect plugin"
```

---

## Task 2: Proto scaffolding (common, error, event extensions)

**Files:**
- Create: `proto/gohome/v1alpha1/common.proto`
- Create: `proto/gohome/error/v1alpha1/error.proto`
- Modify: `proto/gohome/event/v1/event.proto`

- [ ] **Step 1: Create `common.proto`**

```bash
mkdir -p proto/gohome/v1alpha1 proto/gohome/error/v1alpha1
```

Create `proto/gohome/v1alpha1/common.proto`:

```protobuf
// See docs/proto-hygiene.md for grouping conventions.

syntax = "proto3";

package gohome.v1alpha1;

import "google/protobuf/timestamp.proto";

// 1-9: pagination
message PageRequest {
  uint32 page_size  = 1;   // clamped server-side to 1000; default 100 if 0
  string page_token = 2;   // opaque; empty means "first page"
}

message PageResponse {
  string next_page_token = 1;  // empty means "no more pages"
  uint32 total_size      = 2;  // optional; 0 means "unknown"
}

// 10-19: streaming
message Heartbeat {
  uint64                    latest_cursor = 1;
  google.protobuf.Timestamp server_time   = 2;
}

// 20-29: selectors
message EntitySelector {
  repeated string entity_ids = 1;
  repeated string device_ids = 2;
  repeated string areas      = 3;
  repeated string zones      = 4;
  repeated string classes    = 5;  // "light", "switch", ...
}

// 30-39: event filtering (used by EventService.Query and Tail)
message EventFilter {
  repeated string kinds         = 1;   // e.g. "state_changed"
  string          entity_prefix = 2;   // e.g. "light."
  repeated string sources       = 3;   // source strings
  uint64          from_cursor   = 10;  // 0 means "no lower bound"
  uint64          to_cursor     = 11;  // 0 means "no upper bound"
  google.protobuf.Timestamp     from_time = 12;
  google.protobuf.Timestamp     to_time   = 13;
}
```

- [ ] **Step 2: Create `error.proto`**

Create `proto/gohome/error/v1alpha1/error.proto`:

```protobuf
// See docs/proto-hygiene.md for grouping conventions.

syntax = "proto3";

package gohome.error.v1alpha1;

message ErrorDetail {
  // 1-9: classification
  string reason = 1;   // stable constant, e.g. "entity_not_found"
  string domain = 2;   // subsystem, e.g. "automation", "eventstore"

  // 10-19: correlation
  string request_id     = 10;
  string correlation_id = 11;

  // 20-29: detail
  map<string, string> metadata = 20;
}
```

- [ ] **Step 3: Extend `event.proto` with new payload kinds**

Read the current `proto/gohome/event/v1/event.proto`. In the `Payload` oneof, the existing ranges are 1-9, 10-19, 20-29, 30-39, 40-49, 50-59. Add three new range-comment headers plus their initial members:

Insert after line containing `ScriptFinished  script_finished  = 53;`:

```protobuf
    // 60-69: external ingress
    WebhookReceived webhook_received = 60;
    // 70-79: registry mutations
    DeviceRenamed    device_renamed    = 70;
    DeviceReassigned device_reassigned = 71;
    // 80-89: driver control
    DriverInstanceRestarted driver_instance_restarted = 80;
```

Then, at the bottom of the file (after existing message definitions), add:

```protobuf
message WebhookReceived {
  // 1-9: identity
  string slug = 1;

  // 10-19: payload
  bytes               body    = 10;
  map<string, string> headers = 11;

  // 20-29: source
  string source_ip = 20;
}

message DeviceRenamed {
  // 1-9: identity
  string device_id = 1;

  // 10-19: payload
  string old_friendly_name = 10;
  string new_friendly_name = 11;
}

message DeviceReassigned {
  // 1-9: identity
  string device_id = 1;

  // 10-19: payload
  string old_area_id = 10;
  string new_area_id = 11;
}

message DriverInstanceRestarted {
  // 1-9: identity
  string driver_instance_id = 1;

  // 10-19: context
  string reason = 10;
  string actor  = 11;  // principal id if restart was user-initiated
}
```

- [ ] **Step 4: Regenerate protos**

```bash
task proto
```

Expected: no errors. New files appear in `gen/gohome/v1alpha1/`, `gen/gohome/error/v1alpha1/`, updated `gen/gohome/event/v1/event.pb.go`.

- [ ] **Step 5: Verify build**

```bash
task build
```

Expected: compiles cleanly.

- [ ] **Step 6: Commit**

```bash
git add proto/ gen/
git commit -m "feat(c7): common/error protos + new event payload kinds"
```

---

## Task 3: `internal/auth` — seam and stubs

**Files:**
- Create: `internal/auth/auth.go`
- Create: `internal/auth/local.go`
- Create: `internal/auth/reject.go`
- Create: `internal/auth/auth_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/auth/auth_test.go`:

```go
package auth_test

import (
	"context"
	"errors"
	"net/http"
	"syscall"
	"testing"

	"github.com/fynn-labs/gohome/internal/auth"
)

func TestLocalPeerCred_GrantsSystemLocalOnUDS(t *testing.T) {
	a := auth.LocalPeerCred{}
	p, err := a.Authenticate(context.Background(), auth.Request{
		Scheme:   "uds:peercred",
		PeerCred: &syscall.Ucred{Uid: 1000, Pid: 123},
	})
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if p.ID != "system:local" {
		t.Errorf("ID = %q, want system:local", p.ID)
	}
	if p.Kind != "system" {
		t.Errorf("Kind = %q, want system", p.Kind)
	}
}

func TestLocalPeerCred_NotApplicableOnTCP(t *testing.T) {
	a := auth.LocalPeerCred{}
	_, err := a.Authenticate(context.Background(), auth.Request{
		Scheme:     "bearer",
		RemoteAddr: "1.2.3.4:5678",
	})
	if !errors.Is(err, auth.ErrNotApplicable) {
		t.Fatalf("err = %v, want ErrNotApplicable", err)
	}
}

func TestRejectAll_AlwaysUnauthenticated(t *testing.T) {
	a := auth.RejectAll{}
	_, err := a.Authenticate(context.Background(), auth.Request{})
	if !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated", err)
	}
}

func TestChain_FallsThroughOnNotApplicable(t *testing.T) {
	a := auth.Chain(auth.LocalPeerCred{}, auth.RejectAll{})
	_, err := a.Authenticate(context.Background(), auth.Request{
		Scheme: "bearer",
		Headers: http.Header{"Authorization": []string{"Bearer x"}},
	})
	if !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated (TCP reject)", err)
	}
}

func TestChain_StopsOnSuccess(t *testing.T) {
	a := auth.Chain(auth.LocalPeerCred{}, auth.RejectAll{})
	p, err := a.Authenticate(context.Background(), auth.Request{
		Scheme:   "uds:peercred",
		PeerCred: &syscall.Ucred{Uid: 1000},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if p.ID != "system:local" {
		t.Errorf("ID = %q, want system:local", p.ID)
	}
}

func TestAllowAll_Allows(t *testing.T) {
	a := auth.AllowAll{}
	err := a.Authorize(context.Background(),
		auth.Principal{ID: "user:x"},
		auth.Action{Service: "EntityService", Method: "List", Verb: "read"},
		auth.Target{Kind: "entity"})
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
}

func TestContextPrincipal_Roundtrip(t *testing.T) {
	p := auth.Principal{ID: "user:a", Kind: "user"}
	ctx := auth.WithPrincipal(context.Background(), p)
	got, ok := auth.PrincipalFromContext(ctx)
	if !ok || got.ID != "user:a" {
		t.Fatalf("PrincipalFromContext = %v, %v", got, ok)
	}
}
```

- [ ] **Step 2: Run tests — they fail (package does not exist)**

```bash
go test ./internal/auth/... -v
```

Expected: compile error.

- [ ] **Step 3: Write `auth.go`**

Create `internal/auth/auth.go`:

```go
// Package auth defines the authentication and authorization seam used by the
// Connect-RPC API. C7 ships the interfaces and a local-peer-cred stub; C9
// replaces the stub with real passkey / OIDC / token machinery.
package auth

import (
	"context"
	"errors"
	"net/http"
	"syscall"
)

var (
	ErrNotApplicable   = errors.New("auth: not applicable to this request")
	ErrUnauthenticated = errors.New("auth: unauthenticated")
	ErrForbidden       = errors.New("auth: forbidden")
)

// Principal is the identity established by authentication.
type Principal struct {
	ID          string            // "user:fdatoo", "token:xyz", "system:local"
	DisplayName string
	Kind        string            // "user", "token", "system"
	Metadata    map[string]string // roles, audit bits, ...
}

// Request is the raw material an Authenticator inspects.
type Request struct {
	Scheme     string          // "uds:peercred", "bearer", "cookie", ...
	Headers    http.Header
	PeerCred   *syscall.Ucred  // set only when Scheme == "uds:peercred"
	RemoteAddr string
	Method     string          // Connect method, e.g. "/gohome.v1alpha1.EntityService/List"
}

// Action names what the principal is attempting.
type Action struct {
	Service string
	Method  string
	Verb    string // "read" | "write" | "call" | "admin"
}

// Target identifies what Action acts on.
type Target struct {
	Kind string            // "entity" | "automation" | "script" | "config" | "driver" | ""
	ID   string
	Attr map[string]string
}

// Authenticator inspects a request and returns a Principal.
type Authenticator interface {
	Authenticate(ctx context.Context, req Request) (Principal, error)
}

// Authorizer decides whether a Principal may perform Action on Target.
type Authorizer interface {
	Authorize(ctx context.Context, p Principal, a Action, t Target) error
}

type principalCtxKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalCtxKey{}, p)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalCtxKey{}).(Principal)
	return p, ok
}
```

- [ ] **Step 4: Write `local.go`**

Create `internal/auth/local.go`:

```go
package auth

import (
	"context"
	"errors"
)

// LocalPeerCred grants Principal{ID: "system:local"} on UDS peer-cred requests
// and signals ErrNotApplicable on everything else (TCP falls through to the
// next authenticator in the chain).
type LocalPeerCred struct{}

func (LocalPeerCred) Authenticate(_ context.Context, req Request) (Principal, error) {
	if req.Scheme != "uds:peercred" || req.PeerCred == nil {
		return Principal{}, ErrNotApplicable
	}
	return Principal{
		ID:          "system:local",
		DisplayName: "local",
		Kind:        "system",
	}, nil
}

// AllowAll authorizer used until C9 ships. Accepts every call.
type AllowAll struct{}

func (AllowAll) Authorize(_ context.Context, _ Principal, _ Action, _ Target) error {
	return nil
}

// Chain tries each Authenticator in order; ErrNotApplicable falls through.
// Any other error (including ErrUnauthenticated) short-circuits and is
// returned to the caller. The first successful result wins.
func Chain(as ...Authenticator) Authenticator {
	return chain(as)
}

type chain []Authenticator

func (c chain) Authenticate(ctx context.Context, req Request) (Principal, error) {
	var lastErr error
	for _, a := range c {
		p, err := a.Authenticate(ctx, req)
		if err == nil {
			return p, nil
		}
		if !errors.Is(err, ErrNotApplicable) {
			return Principal{}, err
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = ErrUnauthenticated
	}
	return Principal{}, ErrUnauthenticated
}
```

- [ ] **Step 5: Write `reject.go`**

Create `internal/auth/reject.go`:

```go
package auth

import "context"

// RejectAll returns ErrUnauthenticated for every request. Used as the tail of
// the default chain so TCP (and anything else) is dead-ended until C9 installs
// real credentials.
type RejectAll struct{}

func (RejectAll) Authenticate(_ context.Context, _ Request) (Principal, error) {
	return Principal{}, ErrUnauthenticated
}
```

- [ ] **Step 6: Run tests — they pass**

```bash
go test ./internal/auth/... -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/auth/
git commit -m "feat(c7): internal/auth seam with local-peer-cred stub"
```

---

## Task 4: `internal/api/listener` — HTTP server skeleton with UDS + TCP + `/healthz`

**Files:**
- Create: `internal/api/listener/listener.go`
- Create: `internal/api/listener/h2c.go`
- Create: `internal/api/listener/listener_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/api/listener/listener_test.go`:

```go
package listener_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fynn-labs/gohome/internal/api/listener"
)

func TestListener_HealthzOnTCP(t *testing.T) {
	dir := t.TempDir()
	cfg := listener.Config{
		UDSPath: filepath.Join(dir, "sock"),
		UDSMode: 0o600,
		TCPBind: "127.0.0.1:0",
	}
	l, err := listener.Build(cfg, listener.Deps{HealthProbe: func() error { return nil }})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := l.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer l.Shutdown(context.Background())

	resp, err := http.Get("http://" + l.TCPAddr().String() + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d, body = %q", resp.StatusCode, b)
	}
}

func TestListener_UDSFileMode(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "sock")
	cfg := listener.Config{UDSPath: sock, UDSMode: 0o600, TCPBind: "127.0.0.1:0"}
	l, err := listener.Build(cfg, listener.Deps{HealthProbe: func() error { return nil }})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := l.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer l.Shutdown(context.Background())

	info, err := os.Stat(sock)
	if err != nil {
		t.Fatalf("stat sock: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perm = %v, want 0600", info.Mode().Perm())
	}
}

func TestListener_HealthzOnUDS(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "sock")
	cfg := listener.Config{UDSPath: sock, UDSMode: 0o600, TCPBind: "127.0.0.1:0"}
	l, err := listener.Build(cfg, listener.Deps{HealthProbe: func() error { return nil }})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := l.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer l.Shutdown(context.Background())

	client := &http.Client{Transport: &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", sock)
		},
	}}
	resp, err := client.Get("http://unix/healthz")
	if err != nil {
		t.Fatalf("GET over UDS: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestListener_ShutdownRemovesSocket(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "sock")
	cfg := listener.Config{UDSPath: sock, UDSMode: 0o600, TCPBind: "127.0.0.1:0"}
	l, err := listener.Build(cfg, listener.Deps{HealthProbe: func() error { return nil }})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := l.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	cancel()
	shutdownCtx, sc := context.WithTimeout(context.Background(), 5*time.Second)
	defer sc()
	_ = l.Shutdown(shutdownCtx)

	if _, err := os.Stat(sock); !os.IsNotExist(err) {
		t.Errorf("sock still exists after shutdown, err = %v", err)
	}
}
```

- [ ] **Step 2: Run tests — they fail**

```bash
go test ./internal/api/listener/... -v
```

Expected: compile error.

- [ ] **Step 3: Write `h2c.go`**

Create `internal/api/listener/h2c.go`:

```go
package listener

import (
	"net/http"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// newH2CServer wraps an http.Handler so the returned Handler speaks plaintext
// HTTP/2 (h2c) for clients that upgrade, and HTTP/1.1 for clients that do not.
// Connect-go uses this to serve Connect, gRPC, and gRPC-Web from one mux
// without TLS.
func newH2CServer(h http.Handler) http.Handler {
	return h2c.NewHandler(h, &http2.Server{})
}
```

- [ ] **Step 4: Write `listener.go`**

Create `internal/api/listener/listener.go`:

```go
package listener

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// Config is the listener's runtime configuration. Sourced from
// gohome.core.pkl's Listener block.
type Config struct {
	UDSPath string
	UDSMode os.FileMode

	TCPBind string

	// TLSCertFile / TLSKeyFile are optional. If both are empty, the TCP
	// listener serves plaintext (HTTP/1.1 + h2c).
	TLSCertFile string
	TLSKeyFile  string
}

// Deps bundles everything the listener needs to wire a handler tree.
// C7 task 6 adds ConnectHandlers; task 18 adds WebhookHandler; task 5 adds
// Interceptors (via the deps wiring in daemon.go).
type Deps struct {
	// HealthProbe returns nil when the daemon is healthy. Called by /healthz.
	HealthProbe func() error

	// ConnectRoutes is a list of (path, handler) pairs returned by each
	// service's NewXServiceHandler. Empty in task 4; populated from task 6 on.
	ConnectRoutes []Route

	// WebhookHandler serves POST /webhooks/{slug}. Nil in task 4; populated
	// in task 18.
	WebhookHandler http.Handler
}

type Route struct {
	Path    string
	Handler http.Handler
}

type Listener struct {
	cfg  Config
	deps Deps

	mu         sync.Mutex
	tcpLis     net.Listener
	udsLis     net.Listener
	srv        *http.Server
	startedCh  chan struct{}
}

func Build(cfg Config, deps Deps) (*Listener, error) {
	if deps.HealthProbe == nil {
		return nil, errors.New("listener: HealthProbe required")
	}
	return &Listener{cfg: cfg, deps: deps, startedCh: make(chan struct{})}, nil
}

// Start binds both listeners and begins serving. Returns once both listeners
// are bound; the server goroutines run until Shutdown is called or ctx is
// cancelled.
func (l *Listener) Start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", l.healthzHandler)
	for _, r := range l.deps.ConnectRoutes {
		mux.Handle(r.Path, r.Handler)
	}
	if l.deps.WebhookHandler != nil {
		mux.Handle("/webhooks/", l.deps.WebhookHandler)
	}

	l.srv = &http.Server{
		Handler: newH2CServer(mux),
	}

	tcpLis, err := net.Listen("tcp", l.cfg.TCPBind)
	if err != nil {
		return fmt.Errorf("listener: tcp bind %q: %w", l.cfg.TCPBind, err)
	}
	if l.cfg.TLSCertFile != "" && l.cfg.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(l.cfg.TLSCertFile, l.cfg.TLSKeyFile)
		if err != nil {
			_ = tcpLis.Close()
			return fmt.Errorf("listener: load tls: %w", err)
		}
		tcpLis = tls.NewListener(tcpLis, &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		})
	}
	l.tcpLis = tcpLis

	// Remove any stale socket file.
	if err := os.Remove(l.cfg.UDSPath); err != nil && !os.IsNotExist(err) {
		_ = tcpLis.Close()
		return fmt.Errorf("listener: remove stale uds: %w", err)
	}
	udsLis, err := net.Listen("unix", l.cfg.UDSPath)
	if err != nil {
		_ = tcpLis.Close()
		return fmt.Errorf("listener: uds bind %q: %w", l.cfg.UDSPath, err)
	}
	if err := os.Chmod(l.cfg.UDSPath, l.cfg.UDSMode); err != nil {
		_ = tcpLis.Close()
		_ = udsLis.Close()
		return fmt.Errorf("listener: chmod uds: %w", err)
	}
	l.udsLis = udsLis

	go l.serve(l.tcpLis)
	go l.serve(l.udsLis)
	close(l.startedCh)
	return nil
}

func (l *Listener) serve(ls net.Listener) {
	if err := l.srv.Serve(ls); err != nil && !errors.Is(err, http.ErrServerClosed) {
		// Caller observes this via Shutdown; log-only here.
		_ = err
	}
}

func (l *Listener) TCPAddr() net.Addr {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.tcpLis == nil {
		return nil
	}
	return l.tcpLis.Addr()
}

func (l *Listener) Shutdown(ctx context.Context) error {
	l.mu.Lock()
	srv := l.srv
	udsPath := l.cfg.UDSPath
	l.mu.Unlock()
	if srv == nil {
		return nil
	}
	if ctx == nil {
		ctx, _ = context.WithTimeout(context.Background(), 10*time.Second)
	}
	err := srv.Shutdown(ctx)
	_ = os.Remove(udsPath)
	return err
}

func (l *Listener) healthzHandler(w http.ResponseWriter, _ *http.Request) {
	if err := l.deps.HealthProbe(); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}
```

- [ ] **Step 5: Run tests — they pass**

```bash
go test ./internal/api/listener/... -v
```

Expected: PASS.

- [ ] **Step 6: Verify race**

```bash
go test -race ./internal/api/listener/... -v
```

Expected: PASS — no data races.

- [ ] **Step 7: Commit**

```bash
git add internal/api/listener/
git commit -m "feat(c7): api listener skeleton (UDS+TCP, /healthz, h2c)"
```

---

## Task 5: `internal/api` shared helpers — errors, pagination, time

**Files:**
- Create: `internal/api/errors.go`
- Create: `internal/api/pagination.go`
- Create: `internal/api/time.go`
- Create: `internal/api/errors_test.go`
- Create: `internal/api/pagination_test.go`

- [ ] **Step 1: Write failing tests for errors**

Create `internal/api/errors_test.go`:

```go
package api_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	errorv1 "github.com/fynn-labs/gohome/gen/gohome/error/v1alpha1"
	"github.com/fynn-labs/gohome/internal/api"
	"github.com/fynn-labs/gohome/internal/auth"
)

var errSentinel = errors.New("sentinel")

func TestToConnect_NotFound(t *testing.T) {
	err := api.ToConnect(context.Background(), api.ErrEntityNotFound, "entity_not_found")
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("err not connect.Error: %v", err)
	}
	if ce.Code() != connect.CodeNotFound {
		t.Errorf("code = %v, want NotFound", ce.Code())
	}
	// Detail is attached.
	var detail errorv1.ErrorDetail
	if !hasDetail(ce, &detail) {
		t.Fatalf("no ErrorDetail attached")
	}
	if detail.Reason != "entity_not_found" {
		t.Errorf("reason = %q", detail.Reason)
	}
}

func TestToConnect_Unauthenticated(t *testing.T) {
	err := api.ToConnect(context.Background(), auth.ErrUnauthenticated, "unauthenticated")
	var ce *connect.Error
	errors.As(err, &ce)
	if ce.Code() != connect.CodeUnauthenticated {
		t.Errorf("code = %v, want Unauthenticated", ce.Code())
	}
}

func TestToConnect_InternalFallback(t *testing.T) {
	err := api.ToConnect(context.Background(), errSentinel, "")
	var ce *connect.Error
	errors.As(err, &ce)
	if ce.Code() != connect.CodeInternal {
		t.Errorf("code = %v, want Internal", ce.Code())
	}
	// Internal errors hide the original message.
	if ce.Message() == errSentinel.Error() {
		t.Errorf("internal error leaked raw message: %q", ce.Message())
	}
}

func hasDetail(ce *connect.Error, out *errorv1.ErrorDetail) bool {
	for _, d := range ce.Details() {
		v, err := d.Value()
		if err != nil {
			continue
		}
		if ed, ok := v.(*errorv1.ErrorDetail); ok {
			*out = *ed
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Write failing tests for pagination**

Create `internal/api/pagination_test.go`:

```go
package api_test

import (
	"testing"

	"github.com/fynn-labs/gohome/internal/api"
)

func TestCursor_Roundtrip(t *testing.T) {
	token, err := api.EncodeCursor(api.Cursor{Position: 4242, Tiebreak: "x"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := api.DecodeCursor(token)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Position != 4242 || got.Tiebreak != "x" {
		t.Errorf("got %+v", got)
	}
}

func TestCursor_EmptyIsZero(t *testing.T) {
	got, err := api.DecodeCursor("")
	if err != nil {
		t.Fatalf("decode empty: %v", err)
	}
	if got.Position != 0 || got.Tiebreak != "" {
		t.Errorf("got %+v, want zero", got)
	}
}

func TestCursor_Garbage(t *testing.T) {
	_, err := api.DecodeCursor("not-base64!!!")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClampPageSize(t *testing.T) {
	for _, tc := range []struct {
		in, out uint32
	}{
		{0, 100},
		{50, 50},
		{1000, 1000},
		{5000, 1000},
	} {
		if got := api.ClampPageSize(tc.in); got != tc.out {
			t.Errorf("ClampPageSize(%d) = %d, want %d", tc.in, got, tc.out)
		}
	}
}
```

- [ ] **Step 3: Run tests — they fail**

```bash
go test ./internal/api/... -v
```

Expected: compile error.

- [ ] **Step 4: Write `errors.go`**

Create `internal/api/errors.go`:

```go
// Package api holds Connect-RPC handler implementations for gohome.v1alpha1
// services plus shared helpers (error mapping, pagination, time, streaming).
// Handlers depend on backend engines through the narrow interfaces in deps.go,
// never through direct imports of internal/automation, internal/eventstore,
// etc.
package api

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	errorv1 "github.com/fynn-labs/gohome/gen/gohome/error/v1alpha1"
	"github.com/fynn-labs/gohome/internal/auth"
	"github.com/fynn-labs/gohome/internal/observability"
)

// Sentinel errors backend engines return; api.ToConnect maps them.
var (
	ErrEntityNotFound      = errors.New("entity not found")
	ErrDeviceNotFound      = errors.New("device not found")
	ErrAreaNotFound        = errors.New("area not found")
	ErrZoneNotFound        = errors.New("zone not found")
	ErrDriverNotFound      = errors.New("driver not found")
	ErrInstanceNotFound    = errors.New("driver instance not found")
	ErrAutomationNotFound  = errors.New("automation not found")
	ErrScriptNotFound      = errors.New("script not found")
	ErrAutomationDisabled  = errors.New("automation disabled")
	ErrRunNotFound         = errors.New("run not found")
	ErrRunAlreadyFinished  = errors.New("run already finished")
	ErrCapabilityUnknown   = errors.New("capability unknown")
	ErrDriverUnavailable   = errors.New("driver unavailable")
	ErrSubscriptionOverflow = errors.New("subscription overflow")
	ErrValidationFailed    = errors.New("validation failed")
	ErrNotImplemented      = errors.New("not implemented")
)

// ToConnect maps a domain error to a connect.Error with an attached
// ErrorDetail. Callers supply the canonical reason string (entity_not_found,
// automation_disabled, etc.) so the ErrorDetail is self-describing.
//
// Unmapped errors become CodeInternal with a generic client-visible message;
// the original error is logged but not returned, so stack traces and internal
// paths stay server-side.
func ToConnect(ctx context.Context, err error, reason string) error {
	code := classify(err)
	msg := err.Error()
	if code == connect.CodeInternal {
		requestID, _ := observability.RequestIDFromContext(ctx)
		slog.ErrorContext(ctx, "api: internal error",
			slog.String("request_id", requestID),
			slog.Any("error", err))
		msg = "internal error"
	}

	ce := connect.NewError(code, errors.New(msg))

	requestID, _ := observability.RequestIDFromContext(ctx)
	detail := &errorv1.ErrorDetail{
		Reason:    reason,
		RequestId: requestID,
	}
	if d, derr := connect.NewErrorDetail(detail); derr == nil {
		ce.AddDetail(d)
	}
	return ce
}

func classify(err error) connect.Code {
	switch {
	case errors.Is(err, ErrEntityNotFound),
		errors.Is(err, ErrDeviceNotFound),
		errors.Is(err, ErrAreaNotFound),
		errors.Is(err, ErrZoneNotFound),
		errors.Is(err, ErrDriverNotFound),
		errors.Is(err, ErrInstanceNotFound),
		errors.Is(err, ErrAutomationNotFound),
		errors.Is(err, ErrScriptNotFound),
		errors.Is(err, ErrRunNotFound):
		return connect.CodeNotFound
	case errors.Is(err, ErrAutomationDisabled),
		errors.Is(err, ErrRunAlreadyFinished):
		return connect.CodeFailedPrecondition
	case errors.Is(err, ErrCapabilityUnknown),
		errors.Is(err, ErrValidationFailed):
		return connect.CodeInvalidArgument
	case errors.Is(err, ErrDriverUnavailable):
		return connect.CodeUnavailable
	case errors.Is(err, ErrSubscriptionOverflow):
		return connect.CodeResourceExhausted
	case errors.Is(err, ErrNotImplemented):
		return connect.CodeUnimplemented
	case errors.Is(err, auth.ErrUnauthenticated):
		return connect.CodeUnauthenticated
	case errors.Is(err, auth.ErrForbidden):
		return connect.CodePermissionDenied
	case errors.Is(err, context.Canceled):
		return connect.CodeCanceled
	case errors.Is(err, context.DeadlineExceeded):
		return connect.CodeDeadlineExceeded
	}
	return connect.CodeInternal
}
```

- [ ] **Step 5: Write `pagination.go`**

Create `internal/api/pagination.go`:

```go
package api

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
)

const (
	DefaultPageSize = 100
	MaxPageSize     = 1000
)

// Cursor is the decoded form of a PageRequest.page_token. Position is the
// primary key (entity slug ordinal, C1 event cursor, ...); Tiebreak breaks
// ties when Position is not unique on its own.
type Cursor struct {
	Position uint64
	Tiebreak string
}

// EncodeCursor produces an opaque base64 token. Empty Cursor encodes to "".
func EncodeCursor(c Cursor) (string, error) {
	if c.Position == 0 && c.Tiebreak == "" {
		return "", nil
	}
	buf := make([]byte, 8+len(c.Tiebreak))
	binary.BigEndian.PutUint64(buf[:8], c.Position)
	copy(buf[8:], c.Tiebreak)
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// DecodeCursor decodes a token produced by EncodeCursor. Empty string decodes
// to the zero Cursor. Malformed tokens return an error (mapped to
// INVALID_ARGUMENT by the handler).
func DecodeCursor(token string) (Cursor, error) {
	if token == "" {
		return Cursor{}, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return Cursor{}, err
	}
	if len(b) < 8 {
		return Cursor{}, errors.New("cursor: short token")
	}
	return Cursor{
		Position: binary.BigEndian.Uint64(b[:8]),
		Tiebreak: string(b[8:]),
	}, nil
}

// ClampPageSize applies the service-wide default and cap.
func ClampPageSize(n uint32) uint32 {
	switch {
	case n == 0:
		return DefaultPageSize
	case n > MaxPageSize:
		return MaxPageSize
	default:
		return n
	}
}
```

- [ ] **Step 6: Write `time.go`**

Create `internal/api/time.go`:

```go
package api

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func ProtoTime(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func GoTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}
```

- [ ] **Step 7: Add `observability.RequestIDFromContext` if absent**

Check `internal/observability/context.go`. If it does not expose a context key for request-id, add:

```go
package observability

import "context"

type requestIDKey struct{}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

func RequestIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDKey{}).(string)
	return id, ok
}
```

(If `internal/observability/context.go` already has a similar request-ID helper, reuse it and update the import in `errors.go`.)

- [ ] **Step 8: Run tests**

```bash
go test ./internal/api/... ./internal/observability/... -v
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/api/errors.go internal/api/pagination.go internal/api/time.go internal/api/errors_test.go internal/api/pagination_test.go internal/observability/context.go
git commit -m "feat(c7): api error mapping, pagination, time helpers"
```

---

## Task 6: Interceptor stack

**Files:**
- Create: `internal/api/listener/interceptors.go`
- Create: `internal/api/listener/interceptors_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/api/listener/interceptors_test.go`:

```go
package listener_test

import (
	"context"
	"errors"
	"net/http"
	"syscall"
	"testing"

	"connectrpc.com/connect"
	"github.com/fynn-labs/gohome/internal/api/listener"
	"github.com/fynn-labs/gohome/internal/auth"
	"github.com/fynn-labs/gohome/internal/observability"
)

func TestRequestID_MintsIfAbsent(t *testing.T) {
	var seen string
	ic := listener.RequestIDInterceptor()
	handler := ic.WrapUnary(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		id, _ := observability.RequestIDFromContext(ctx)
		seen = id
		return nil, nil
	})
	_, _ = handler(context.Background(), &fakeAnyReq{})
	if seen == "" {
		t.Fatal("expected a minted request id")
	}
}

func TestRequestID_EchoesInbound(t *testing.T) {
	ic := listener.RequestIDInterceptor()
	var seen string
	handler := ic.WrapUnary(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		seen, _ = observability.RequestIDFromContext(ctx)
		return nil, nil
	})
	req := &fakeAnyReq{headers: http.Header{"X-Request-Id": []string{"abc123"}}}
	_, _ = handler(context.Background(), req)
	if seen != "abc123" {
		t.Errorf("seen = %q, want abc123", seen)
	}
}

func TestAuthenticate_AttachesPrincipal(t *testing.T) {
	ic := listener.AuthenticateInterceptor(auth.LocalPeerCred{}, udsRequestMarker{})
	var seen auth.Principal
	handler := ic.WrapUnary(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		p, _ := auth.PrincipalFromContext(ctx)
		seen = p
		return nil, nil
	})
	ctx := listener.WithPeerCred(context.Background(), &syscall.Ucred{Uid: 1000})
	_, _ = handler(ctx, &fakeAnyReq{})
	if seen.ID != "system:local" {
		t.Errorf("principal = %+v, want system:local", seen)
	}
}

func TestAuthenticate_Rejects(t *testing.T) {
	ic := listener.AuthenticateInterceptor(auth.RejectAll{}, tcpRequestMarker{})
	handler := ic.WrapUnary(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, nil
	})
	_, err := handler(context.Background(), &fakeAnyReq{})
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeUnauthenticated {
		t.Fatalf("err = %v, want Unauthenticated", err)
	}
}

func TestRecover_TurnsPanicIntoInternal(t *testing.T) {
	ic := listener.RecoverInterceptor()
	handler := ic.WrapUnary(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		panic("boom")
	})
	_, err := handler(context.Background(), &fakeAnyReq{})
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeInternal {
		t.Fatalf("err = %v, want Internal", err)
	}
}

// fakeAnyReq implements connect.AnyRequest for testing.
type fakeAnyReq struct {
	headers http.Header
	spec    connect.Spec
}

func (f *fakeAnyReq) Any() any              { return nil }
func (f *fakeAnyReq) Spec() connect.Spec    { return f.spec }
func (f *fakeAnyReq) Peer() connect.Peer    { return connect.Peer{} }
func (f *fakeAnyReq) Header() http.Header   { if f.headers == nil { f.headers = http.Header{} }; return f.headers }
func (f *fakeAnyReq) HTTPMethod() string    { return http.MethodPost }

type udsRequestMarker struct{}
type tcpRequestMarker struct{}

func (udsRequestMarker) Classify(connect.AnyRequest) (scheme string, isUDS bool) { return "uds:peercred", true }
func (tcpRequestMarker) Classify(connect.AnyRequest) (scheme string, isUDS bool) { return "bearer", false }
```

- [ ] **Step 2: Run tests — they fail**

```bash
go test ./internal/api/listener/... -v
```

Expected: compile error.

- [ ] **Step 3: Write `interceptors.go`**

Create `internal/api/listener/interceptors.go`:

```go
package listener

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"syscall"
	"time"

	"connectrpc.com/connect"
	errorv1 "github.com/fynn-labs/gohome/gen/gohome/error/v1alpha1"
	"github.com/fynn-labs/gohome/internal/auth"
	"github.com/fynn-labs/gohome/internal/observability"
	"github.com/oklog/ulid/v2"
)

// SchemeClassifier reports the auth scheme for a request (used by the
// authenticate interceptor to build auth.Request). Implementations know
// whether the request arrived on UDS (read PeerCred from context) or TCP.
type SchemeClassifier interface {
	Classify(req connect.AnyRequest) (scheme string, isUDS bool)
}

type peerCredKey struct{}

func WithPeerCred(ctx context.Context, c *syscall.Ucred) context.Context {
	return context.WithValue(ctx, peerCredKey{}, c)
}
func peerCredFromContext(ctx context.Context) *syscall.Ucred {
	c, _ := ctx.Value(peerCredKey{}).(*syscall.Ucred)
	return c
}

func RequestIDInterceptor() connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			id := req.Header().Get("X-Request-Id")
			if id == "" {
				id = ulid.Make().String()
			}
			ctx = observability.WithRequestID(ctx, id)
			resp, err := next(ctx, req)
			if resp != nil {
				resp.Header().Set("X-Request-Id", id)
			}
			return resp, err
		}
	})
}

func SlogInterceptor() connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			id, _ := observability.RequestIDFromContext(ctx)
			method := req.Spec().Procedure
			code := connect.CodeOf(err)
			slog.InfoContext(ctx, "api request",
				slog.String("request_id", id),
				slog.String("method", method),
				slog.String("code", code.String()),
				slog.Duration("duration", time.Since(start)))
			return resp, err
		}
	})
}

func MetricsInterceptor(m *observability.Metrics) connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			if m != nil && m.APIRequestsTotal != nil {
				m.APIRequestsTotal.WithLabelValues(
					req.Spec().Procedure,
					connect.CodeOf(err).String(),
				).Inc()
				m.APIRequestDurationSeconds.WithLabelValues(
					req.Spec().Procedure,
					connect.CodeOf(err).String(),
				).Observe(time.Since(start).Seconds())
			}
			return resp, err
		}
	})
}

func RecoverInterceptor() connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (resp connect.AnyResponse, err error) {
			defer func() {
				if r := recover(); r != nil {
					stack := debug.Stack()
					id, _ := observability.RequestIDFromContext(ctx)
					slog.ErrorContext(ctx, "api: panic",
						slog.String("request_id", id),
						slog.Any("panic", r),
						slog.String("stack", string(stack)))
					ce := connect.NewError(connect.CodeInternal, errors.New("internal error"))
					detail := &errorv1.ErrorDetail{Reason: "panic", RequestId: id}
					if d, derr := connect.NewErrorDetail(detail); derr == nil {
						ce.AddDetail(d)
					}
					err = ce
				}
			}()
			return next(ctx, req)
		}
	})
}

func AuthenticateInterceptor(a auth.Authenticator, cls SchemeClassifier) connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			scheme, _ := cls.Classify(req)
			ar := auth.Request{
				Scheme:     scheme,
				Headers:    req.Header(),
				RemoteAddr: req.Peer().Addr,
				Method:     req.Spec().Procedure,
				PeerCred:   peerCredFromContext(ctx),
			}
			p, err := a.Authenticate(ctx, ar)
			if err != nil {
				id, _ := observability.RequestIDFromContext(ctx)
				ce := connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
				detail := &errorv1.ErrorDetail{Reason: "unauthenticated", RequestId: id}
				if d, derr := connect.NewErrorDetail(detail); derr == nil {
					ce.AddDetail(d)
				}
				return nil, ce
			}
			ctx = auth.WithPrincipal(ctx, p)
			return next(ctx, req)
		}
	})
}

// AuthorizeInterceptor reads the per-method Action from the provided table
// and asks the authorizer. Target extraction (entity id / automation id / ...)
// is handler-specific; we do a coarse "read vs write vs call" check here and
// let handlers do target-specific checks as needed via Authorizer.Authorize.
func AuthorizeInterceptor(az auth.Authorizer, actions map[string]auth.Action) connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			a, ok := actions[req.Spec().Procedure]
			if !ok {
				// Unknown method — fall through; handler may be catch-all or unregistered.
				return next(ctx, req)
			}
			p, _ := auth.PrincipalFromContext(ctx)
			if err := az.Authorize(ctx, p, a, auth.Target{}); err != nil {
				id, _ := observability.RequestIDFromContext(ctx)
				ce := connect.NewError(connect.CodePermissionDenied, fmt.Errorf("forbidden"))
				detail := &errorv1.ErrorDetail{Reason: "forbidden", RequestId: id}
				if d, derr := connect.NewErrorDetail(detail); derr == nil {
					ce.AddDetail(d)
				}
				return nil, ce
			}
			return next(ctx, req)
		}
	})
}
```

- [ ] **Step 4: Add `ulid` dependency if absent**

```bash
go get github.com/oklog/ulid/v2@latest
go mod tidy
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/api/listener/... -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/listener/interceptors.go internal/api/listener/interceptors_test.go go.mod go.sum
git commit -m "feat(c7): interceptor stack (request-id, slog, metrics, recover, auth)"
```

Add `gohome_api_*` metrics to `internal/observability/metrics.go`. Locate the `Metrics` struct and append the new fields, plus initialize them in the constructor:

```go
// In Metrics struct (alongside existing counters):
APIRequestsTotal          *prometheus.CounterVec
APIRequestDurationSeconds *prometheus.HistogramVec
APIStreamEventsSentTotal  *prometheus.CounterVec
APIStreamHeartbeatsSentTotal *prometheus.CounterVec
APIStreamBackpressureClosesTotal *prometheus.CounterVec
APIWebhookReceivedTotal   *prometheus.CounterVec
APIActiveStreams          *prometheus.GaugeVec
```

In the constructor (alongside existing `prometheus.NewCounterVec(...)` calls):

```go
m.APIRequestsTotal = promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
    Name: "gohome_api_requests_total",
    Help: "Completed API RPCs by procedure and code.",
}, []string{"procedure", "code"})

m.APIRequestDurationSeconds = promauto.With(reg).NewHistogramVec(prometheus.HistogramOpts{
    Name:    "gohome_api_request_duration_seconds",
    Help:    "Latency of completed API RPCs.",
    Buckets: prometheus.ExponentialBuckets(0.001, 2, 14),
}, []string{"procedure", "code"})

m.APIStreamEventsSentTotal = promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
    Name: "gohome_api_stream_events_sent_total",
    Help: "Streamed payload events sent (excludes heartbeats).",
}, []string{"procedure"})

m.APIStreamHeartbeatsSentTotal = promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
    Name: "gohome_api_stream_heartbeats_sent_total",
    Help: "Heartbeats sent on streaming RPCs.",
}, []string{"procedure"})

m.APIStreamBackpressureClosesTotal = promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
    Name: "gohome_api_stream_backpressure_closes_total",
    Help: "Streaming RPCs closed with RESOURCE_EXHAUSTED.",
}, []string{"procedure"})

m.APIWebhookReceivedTotal = promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
    Name: "gohome_api_webhook_received_total",
    Help: "Webhook outcomes by slug and result.",
}, []string{"slug", "result"})

m.APIActiveStreams = promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
    Name: "gohome_api_active_streams",
    Help: "Currently-open server streams.",
}, []string{"procedure"})
```

Commit:

```bash
git add internal/observability/metrics.go
git commit -m "feat(c7): register gohome_api_* metrics"
```

---

## Task 7: `SystemService` — proto, handler, end-to-end test

Proves the full pipeline (proto → buf → handler → listener → request) on the smallest service.

**Files:**
- Create: `proto/gohome/v1alpha1/system.proto`
- Create: `internal/api/deps.go` (initial; expanded in later tasks)
- Create: `internal/api/service_system.go`
- Create: `internal/api/service_system_test.go`
- Create: `internal/api/fakes_test.go`

- [ ] **Step 1: Create `system.proto`**

```protobuf
// See docs/proto-hygiene.md for grouping conventions.

syntax = "proto3";

package gohome.v1alpha1;

import "google/protobuf/timestamp.proto";

service SystemService {
  rpc Version       (VersionRequest)        returns (VersionResponse);
  rpc Health        (HealthRequest)         returns (HealthResponse);
  rpc Metrics       (MetricsRequest)        returns (MetricsResponse);
  rpc Diagnostics   (DiagnosticsRequest)    returns (DiagnosticsResponse);
  rpc CreateSnapshot(CreateSnapshotRequest) returns (CreateSnapshotResponse);
}

// 1-9: Version
message VersionRequest {}
message VersionResponse {
  string binary_version = 1;
  string git_commit     = 2;
  string build_date     = 3;
  string schema_version = 4;
}

// 10-19: Health
message HealthRequest {}
message HealthResponse {
  // 1-9: status
  bool   ok      = 1;
  string summary = 2;

  // 10-19: per-subsystem
  repeated SubsystemHealth subsystems = 10;
}
message SubsystemHealth {
  string name    = 1;   // "eventstore", "registry", "automation", "driver:hue-1", ...
  bool   ok      = 2;
  string detail  = 3;
}

// 20-29: Metrics
message MetricsRequest {}
message MetricsResponse {
  string prometheus_text = 1;
}

// 30-39: Diagnostics
message DiagnosticsRequest {}
message DiagnosticsResponse {
  bytes  bundle      = 1;   // tar.gz; capped at 10 MiB
  string config_hash = 2;
  google.protobuf.Timestamp generated_at = 3;
}

// 40-49: Snapshot
message CreateSnapshotRequest {
  string owner  = 1;   // projector owner, e.g. "state_cache"
  string reason = 2;
}
message CreateSnapshotResponse {
  uint64 cursor       = 1;
  google.protobuf.Timestamp created_at = 2;
}
```

- [ ] **Step 2: Regenerate**

```bash
task proto
task build
```

Expected: `gen/gohome/v1alpha1/system.pb.go` and `gen/gohome/v1alpha1/v1alpha1connect/system.connect.go` are created. Build clean.

- [ ] **Step 3: Write `deps.go` (initial)**

Create `internal/api/deps.go`:

```go
package api

import (
	"context"
	"time"
)

// VersionInfo is what SystemService.Version exposes.
type VersionInfo struct {
	BinaryVersion string
	GitCommit     string
	BuildDate     string
	SchemaVersion string
}

// SubsystemHealth mirrors the proto, in pure-Go form.
type SubsystemHealth struct {
	Name   string
	OK     bool
	Detail string
}

// SystemBackend is what SystemService delegates to.
type SystemBackend interface {
	Version() VersionInfo
	Health(ctx context.Context) (ok bool, summary string, sub []SubsystemHealth)
	MetricsText() (string, error)
	Diagnostics(ctx context.Context) (bundle []byte, configHash string, generatedAt time.Time, err error)
	CreateSnapshot(ctx context.Context, owner, reason string) (cursor uint64, createdAt time.Time, err error)
}
```

- [ ] **Step 4: Write fakes**

Create `internal/api/fakes_test.go`:

```go
package api_test

import (
	"context"
	"errors"
	"time"

	"github.com/fynn-labs/gohome/internal/api"
)

type fakeSystem struct {
	version       api.VersionInfo
	healthy       bool
	subs          []api.SubsystemHealth
	metrics       string
	bundle        []byte
	configHash    string
	snapshotErr   error
	lastOwner     string
	lastReason    string
}

func (f *fakeSystem) Version() api.VersionInfo { return f.version }
func (f *fakeSystem) Health(_ context.Context) (bool, string, []api.SubsystemHealth) {
	if f.healthy {
		return true, "ok", f.subs
	}
	return false, "degraded", f.subs
}
func (f *fakeSystem) MetricsText() (string, error) { return f.metrics, nil }
func (f *fakeSystem) Diagnostics(_ context.Context) ([]byte, string, time.Time, error) {
	return f.bundle, f.configHash, time.Unix(1700000000, 0).UTC(), nil
}
func (f *fakeSystem) CreateSnapshot(_ context.Context, owner, reason string) (uint64, time.Time, error) {
	if f.snapshotErr != nil {
		return 0, time.Time{}, f.snapshotErr
	}
	f.lastOwner = owner
	f.lastReason = reason
	return 1234, time.Unix(1700000001, 0).UTC(), nil
}

var _ api.SystemBackend = (*fakeSystem)(nil)

var errBackend = errors.New("backend exploded")
```

- [ ] **Step 5: Write failing handler test**

Create `internal/api/service_system_test.go`:

```go
package api_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	systemv1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/internal/api"
)

func TestSystemService_Version(t *testing.T) {
	s := api.NewSystemService(&fakeSystem{
		version: api.VersionInfo{BinaryVersion: "0.1.0", GitCommit: "abc"},
	})
	resp, err := s.Version(context.Background(), connect.NewRequest(&systemv1.VersionRequest{}))
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if resp.Msg.BinaryVersion != "0.1.0" || resp.Msg.GitCommit != "abc" {
		t.Errorf("got %+v", resp.Msg)
	}
}

func TestSystemService_Health_OK(t *testing.T) {
	s := api.NewSystemService(&fakeSystem{healthy: true, subs: []api.SubsystemHealth{{Name: "eventstore", OK: true}}})
	resp, err := s.Health(context.Background(), connect.NewRequest(&systemv1.HealthRequest{}))
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if !resp.Msg.Ok {
		t.Error("ok=false, want true")
	}
	if len(resp.Msg.Subsystems) != 1 || resp.Msg.Subsystems[0].Name != "eventstore" {
		t.Errorf("subs = %+v", resp.Msg.Subsystems)
	}
}

func TestSystemService_CreateSnapshot(t *testing.T) {
	fs := &fakeSystem{}
	s := api.NewSystemService(fs)
	resp, err := s.CreateSnapshot(context.Background(),
		connect.NewRequest(&systemv1.CreateSnapshotRequest{Owner: "state_cache", Reason: "manual"}))
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	if resp.Msg.Cursor != 1234 {
		t.Errorf("cursor = %d", resp.Msg.Cursor)
	}
	if fs.lastOwner != "state_cache" || fs.lastReason != "manual" {
		t.Errorf("backend not called with right args: owner=%q reason=%q", fs.lastOwner, fs.lastReason)
	}
}
```

- [ ] **Step 6: Run — fail (NewSystemService not defined)**

```bash
go test ./internal/api/... -v
```

Expected: compile error.

- [ ] **Step 7: Write `service_system.go`**

Create `internal/api/service_system.go`:

```go
package api

import (
	"context"

	"connectrpc.com/connect"
	systemv1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/gen/gohome/v1alpha1/v1alpha1connect"
)

type SystemService struct {
	be SystemBackend
}

func NewSystemService(be SystemBackend) *SystemService {
	return &SystemService{be: be}
}

var _ v1alpha1connect.SystemServiceHandler = (*SystemService)(nil)

func (s *SystemService) Version(_ context.Context, _ *connect.Request[systemv1.VersionRequest]) (*connect.Response[systemv1.VersionResponse], error) {
	v := s.be.Version()
	return connect.NewResponse(&systemv1.VersionResponse{
		BinaryVersion: v.BinaryVersion,
		GitCommit:     v.GitCommit,
		BuildDate:     v.BuildDate,
		SchemaVersion: v.SchemaVersion,
	}), nil
}

func (s *SystemService) Health(ctx context.Context, _ *connect.Request[systemv1.HealthRequest]) (*connect.Response[systemv1.HealthResponse], error) {
	ok, summary, subs := s.be.Health(ctx)
	out := &systemv1.HealthResponse{Ok: ok, Summary: summary}
	for _, s := range subs {
		out.Subsystems = append(out.Subsystems, &systemv1.SubsystemHealth{
			Name: s.Name, Ok: s.OK, Detail: s.Detail,
		})
	}
	return connect.NewResponse(out), nil
}

func (s *SystemService) Metrics(ctx context.Context, _ *connect.Request[systemv1.MetricsRequest]) (*connect.Response[systemv1.MetricsResponse], error) {
	text, err := s.be.MetricsText()
	if err != nil {
		return nil, ToConnect(ctx, err, "metrics_unavailable")
	}
	return connect.NewResponse(&systemv1.MetricsResponse{PrometheusText: text}), nil
}

func (s *SystemService) Diagnostics(ctx context.Context, _ *connect.Request[systemv1.DiagnosticsRequest]) (*connect.Response[systemv1.DiagnosticsResponse], error) {
	bundle, hash, t, err := s.be.Diagnostics(ctx)
	if err != nil {
		return nil, ToConnect(ctx, err, "diagnostics_failed")
	}
	return connect.NewResponse(&systemv1.DiagnosticsResponse{
		Bundle:      bundle,
		ConfigHash:  hash,
		GeneratedAt: ProtoTime(t),
	}), nil
}

func (s *SystemService) CreateSnapshot(ctx context.Context, req *connect.Request[systemv1.CreateSnapshotRequest]) (*connect.Response[systemv1.CreateSnapshotResponse], error) {
	cursor, t, err := s.be.CreateSnapshot(ctx, req.Msg.Owner, req.Msg.Reason)
	if err != nil {
		return nil, ToConnect(ctx, err, "snapshot_failed")
	}
	return connect.NewResponse(&systemv1.CreateSnapshotResponse{
		Cursor:    cursor,
		CreatedAt: ProtoTime(t),
	}), nil
}
```

- [ ] **Step 8: Run tests — pass**

```bash
go test ./internal/api/... -v
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add proto/gohome/v1alpha1/system.proto gen/gohome/v1alpha1/ internal/api/deps.go internal/api/service_system.go internal/api/service_system_test.go internal/api/fakes_test.go
git commit -m "feat(c7): SystemService proto + handler"
```

---

## Task 8: `AreaService` and `ZoneService` — read-only thin services

**Files:**
- Create: `proto/gohome/v1alpha1/area.proto`
- Create: `proto/gohome/v1alpha1/zone.proto`
- Modify: `internal/api/deps.go`
- Create: `internal/api/service_area.go`
- Create: `internal/api/service_zone.go`
- Create: `internal/api/service_area_test.go`
- Create: `internal/api/service_zone_test.go`
- Modify: `internal/api/fakes_test.go`

- [ ] **Step 1: Write `area.proto`**

```protobuf
syntax = "proto3";
package gohome.v1alpha1;
import "gohome/v1alpha1/common.proto";

service AreaService {
  rpc List(ListAreasRequest) returns (ListAreasResponse);
  rpc Get (GetAreaRequest)   returns (GetAreaResponse);
}

message Area {
  string id           = 1;
  string display_name = 2;
  string parent_id    = 3;  // empty for root
}

message ListAreasRequest  { PageRequest page = 1; }
message ListAreasResponse {
  repeated Area areas = 1;
  PageResponse  page  = 2;
}

message GetAreaRequest  { string id = 1; }
message GetAreaResponse { Area area = 1; }
```

- [ ] **Step 2: Write `zone.proto`** (identical shape, swap `Area`→`Zone`):

```protobuf
syntax = "proto3";
package gohome.v1alpha1;
import "gohome/v1alpha1/common.proto";

service ZoneService {
  rpc List(ListZonesRequest) returns (ListZonesResponse);
  rpc Get (GetZoneRequest)   returns (GetZoneResponse);
}

message Zone {
  string id           = 1;
  string display_name = 2;
  repeated string area_ids = 3;
}

message ListZonesRequest  { PageRequest page = 1; }
message ListZonesResponse {
  repeated Zone zones = 1;
  PageResponse  page  = 2;
}

message GetZoneRequest  { string id = 1; }
message GetZoneResponse { Zone zone = 1; }
```

- [ ] **Step 3: Regenerate**

```bash
task proto
```

- [ ] **Step 4: Extend `deps.go`**

Append to `internal/api/deps.go`:

```go
type Area struct {
	ID          string
	DisplayName string
	ParentID    string
}

type Zone struct {
	ID          string
	DisplayName string
	AreaIDs     []string
}

type AreaReader interface {
	ListAreas(ctx context.Context, page PageReq) ([]Area, Cursor, error)
	GetArea(ctx context.Context, id string) (Area, error)
}

type ZoneReader interface {
	ListZones(ctx context.Context, page PageReq) ([]Zone, Cursor, error)
	GetZone(ctx context.Context, id string) (Zone, error)
}

// PageReq is the decoded form of PageRequest.
type PageReq struct {
	Size   uint32  // already clamped via ClampPageSize
	Cursor Cursor
}
```

- [ ] **Step 5: Failing tests for both services**

Create `internal/api/service_area_test.go`:

```go
package api_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/internal/api"
)

type fakeAreas struct {
	areas []api.Area
}

func (f *fakeAreas) ListAreas(_ context.Context, _ api.PageReq) ([]api.Area, api.Cursor, error) {
	return f.areas, api.Cursor{}, nil
}
func (f *fakeAreas) GetArea(_ context.Context, id string) (api.Area, error) {
	for _, a := range f.areas {
		if a.ID == id {
			return a, nil
		}
	}
	return api.Area{}, api.ErrAreaNotFound
}

func TestAreaService_List(t *testing.T) {
	s := api.NewAreaService(&fakeAreas{areas: []api.Area{{ID: "kitchen"}, {ID: "bedroom"}}})
	resp, err := s.List(context.Background(), connect.NewRequest(&v1.ListAreasRequest{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Msg.Areas) != 2 {
		t.Errorf("len = %d", len(resp.Msg.Areas))
	}
}

func TestAreaService_Get_NotFound(t *testing.T) {
	s := api.NewAreaService(&fakeAreas{})
	_, err := s.Get(context.Background(), connect.NewRequest(&v1.GetAreaRequest{Id: "nope"}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeNotFound {
		t.Fatalf("err = %v", err)
	}
}
```

Create `internal/api/service_zone_test.go` (parallel):

```go
package api_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/internal/api"
)

type fakeZones struct{ zones []api.Zone }

func (f *fakeZones) ListZones(_ context.Context, _ api.PageReq) ([]api.Zone, api.Cursor, error) {
	return f.zones, api.Cursor{}, nil
}
func (f *fakeZones) GetZone(_ context.Context, id string) (api.Zone, error) {
	for _, z := range f.zones {
		if z.ID == id {
			return z, nil
		}
	}
	return api.Zone{}, api.ErrZoneNotFound
}

func TestZoneService_List(t *testing.T) {
	s := api.NewZoneService(&fakeZones{zones: []api.Zone{{ID: "downstairs"}}})
	resp, err := s.List(context.Background(), connect.NewRequest(&v1.ListZonesRequest{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Msg.Zones) != 1 {
		t.Errorf("len = %d", len(resp.Msg.Zones))
	}
}
```

- [ ] **Step 6: Run — fail**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 7: Write services**

Create `internal/api/service_area.go`:

```go
package api

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/gen/gohome/v1alpha1/v1alpha1connect"
)

type AreaService struct{ be AreaReader }

func NewAreaService(be AreaReader) *AreaService { return &AreaService{be: be} }

var _ v1alpha1connect.AreaServiceHandler = (*AreaService)(nil)

func (s *AreaService) List(ctx context.Context, req *connect.Request[v1.ListAreasRequest]) (*connect.Response[v1.ListAreasResponse], error) {
	cur, err := DecodeCursor(pageToken(req.Msg.Page))
	if err != nil {
		return nil, ToConnect(ctx, ErrValidationFailed, "bad_page_token")
	}
	pr := PageReq{Size: ClampPageSize(pageSize(req.Msg.Page)), Cursor: cur}
	areas, next, err := s.be.ListAreas(ctx, pr)
	if err != nil {
		return nil, ToConnect(ctx, err, "list_failed")
	}
	out := &v1.ListAreasResponse{Page: &v1.PageResponse{}}
	tok, _ := EncodeCursor(next)
	out.Page.NextPageToken = tok
	for _, a := range areas {
		out.Areas = append(out.Areas, &v1.Area{Id: a.ID, DisplayName: a.DisplayName, ParentId: a.ParentID})
	}
	return connect.NewResponse(out), nil
}

func (s *AreaService) Get(ctx context.Context, req *connect.Request[v1.GetAreaRequest]) (*connect.Response[v1.GetAreaResponse], error) {
	a, err := s.be.GetArea(ctx, req.Msg.Id)
	if err != nil {
		return nil, ToConnect(ctx, err, "area_not_found")
	}
	return connect.NewResponse(&v1.GetAreaResponse{Area: &v1.Area{Id: a.ID, DisplayName: a.DisplayName, ParentId: a.ParentID}}), nil
}

func pageToken(p *v1.PageRequest) string { if p == nil { return "" }; return p.PageToken }
func pageSize(p *v1.PageRequest) uint32  { if p == nil { return 0 }; return p.PageSize }
```

Create `internal/api/service_zone.go` (mirrors area):

```go
package api

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/gen/gohome/v1alpha1/v1alpha1connect"
)

type ZoneService struct{ be ZoneReader }

func NewZoneService(be ZoneReader) *ZoneService { return &ZoneService{be: be} }

var _ v1alpha1connect.ZoneServiceHandler = (*ZoneService)(nil)

func (s *ZoneService) List(ctx context.Context, req *connect.Request[v1.ListZonesRequest]) (*connect.Response[v1.ListZonesResponse], error) {
	cur, err := DecodeCursor(pageToken(req.Msg.Page))
	if err != nil {
		return nil, ToConnect(ctx, ErrValidationFailed, "bad_page_token")
	}
	pr := PageReq{Size: ClampPageSize(pageSize(req.Msg.Page)), Cursor: cur}
	zones, next, err := s.be.ListZones(ctx, pr)
	if err != nil {
		return nil, ToConnect(ctx, err, "list_failed")
	}
	out := &v1.ListZonesResponse{Page: &v1.PageResponse{}}
	tok, _ := EncodeCursor(next)
	out.Page.NextPageToken = tok
	for _, z := range zones {
		out.Zones = append(out.Zones, &v1.Zone{Id: z.ID, DisplayName: z.DisplayName, AreaIds: z.AreaIDs})
	}
	return connect.NewResponse(out), nil
}

func (s *ZoneService) Get(ctx context.Context, req *connect.Request[v1.GetZoneRequest]) (*connect.Response[v1.GetZoneResponse], error) {
	z, err := s.be.GetZone(ctx, req.Msg.Id)
	if err != nil {
		return nil, ToConnect(ctx, err, "zone_not_found")
	}
	return connect.NewResponse(&v1.GetZoneResponse{Zone: &v1.Zone{Id: z.ID, DisplayName: z.DisplayName, AreaIds: z.AreaIDs}}), nil
}
```

- [ ] **Step 8: Run tests — pass**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 9: Commit**

```bash
git add proto/gohome/v1alpha1/area.proto proto/gohome/v1alpha1/zone.proto gen/gohome/v1alpha1/ internal/api/deps.go internal/api/service_area.go internal/api/service_zone.go internal/api/service_area_test.go internal/api/service_zone_test.go
git commit -m "feat(c7): AreaService + ZoneService"
```

---

## Task 9: `DeviceService` — list/get/rename/reassign with new mutation events

**Files:**
- Create: `proto/gohome/v1alpha1/device.proto`
- Modify: `internal/api/deps.go`
- Create: `internal/api/service_device.go`
- Create: `internal/api/service_device_test.go`

- [ ] **Step 1: Write `device.proto`**

```protobuf
syntax = "proto3";
package gohome.v1alpha1;
import "gohome/v1alpha1/common.proto";

service DeviceService {
  rpc List    (ListDevicesRequest)    returns (ListDevicesResponse);
  rpc Get     (GetDeviceRequest)      returns (GetDeviceResponse);
  rpc Rename  (RenameDeviceRequest)   returns (RenameDeviceResponse);
  rpc Reassign(ReassignDeviceRequest) returns (ReassignDeviceResponse);
}

message Device {
  string id                 = 1;
  string friendly_name      = 2;
  string area_id            = 3;
  string driver_instance_id = 4;
  repeated string entity_ids = 5;
}

message ListDevicesRequest  { PageRequest page = 1; string area_id = 2; }
message ListDevicesResponse { repeated Device devices = 1; PageResponse page = 2; }

message GetDeviceRequest  { string id = 1; }
message GetDeviceResponse { Device device = 1; }

message RenameDeviceRequest  { string id = 1; string new_friendly_name = 2; }
message RenameDeviceResponse { Device device = 1; }

message ReassignDeviceRequest  { string id = 1; string new_area_id = 2; }
message ReassignDeviceResponse { Device device = 1; }
```

- [ ] **Step 2: Regenerate**

```bash
task proto
```

- [ ] **Step 3: Extend `deps.go`**

Append:

```go
type Device struct {
	ID               string
	FriendlyName     string
	AreaID           string
	DriverInstanceID string
	EntityIDs        []string
}

type DeviceReader interface {
	ListDevices(ctx context.Context, areaID string, page PageReq) ([]Device, Cursor, error)
	GetDevice(ctx context.Context, id string) (Device, error)
}

// DeviceWriter mutates devices and emits the corresponding registry-mutation
// events (DeviceRenamed, DeviceReassigned). actor is the principal id of the
// caller; empty string means "system".
type DeviceWriter interface {
	RenameDevice(ctx context.Context, id, newName, actor string) (Device, error)
	ReassignDevice(ctx context.Context, id, newAreaID, actor string) (Device, error)
}
```

- [ ] **Step 4: Failing tests**

Create `internal/api/service_device_test.go`:

```go
package api_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/internal/api"
)

type fakeDevices struct {
	devices  []api.Device
	renamed  []renameRecord
	reassign []reassignRecord
}
type renameRecord   struct{ id, name, actor string }
type reassignRecord struct{ id, area, actor string }

func (f *fakeDevices) ListDevices(_ context.Context, area string, _ api.PageReq) ([]api.Device, api.Cursor, error) {
	if area == "" {
		return f.devices, api.Cursor{}, nil
	}
	var out []api.Device
	for _, d := range f.devices {
		if d.AreaID == area {
			out = append(out, d)
		}
	}
	return out, api.Cursor{}, nil
}
func (f *fakeDevices) GetDevice(_ context.Context, id string) (api.Device, error) {
	for _, d := range f.devices {
		if d.ID == id {
			return d, nil
		}
	}
	return api.Device{}, api.ErrDeviceNotFound
}
func (f *fakeDevices) RenameDevice(_ context.Context, id, name, actor string) (api.Device, error) {
	for i, d := range f.devices {
		if d.ID == id {
			f.devices[i].FriendlyName = name
			f.renamed = append(f.renamed, renameRecord{id, name, actor})
			return f.devices[i], nil
		}
	}
	return api.Device{}, api.ErrDeviceNotFound
}
func (f *fakeDevices) ReassignDevice(_ context.Context, id, area, actor string) (api.Device, error) {
	for i, d := range f.devices {
		if d.ID == id {
			f.devices[i].AreaID = area
			f.reassign = append(f.reassign, reassignRecord{id, area, actor})
			return f.devices[i], nil
		}
	}
	return api.Device{}, api.ErrDeviceNotFound
}

func TestDeviceService_List_FilterArea(t *testing.T) {
	s := api.NewDeviceService(&fakeDevices{devices: []api.Device{{ID: "a", AreaID: "kitchen"}, {ID: "b", AreaID: "bedroom"}}}, nil)
	resp, err := s.List(context.Background(), connect.NewRequest(&v1.ListDevicesRequest{AreaId: "kitchen"}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Msg.Devices) != 1 || resp.Msg.Devices[0].Id != "a" {
		t.Errorf("got %+v", resp.Msg.Devices)
	}
}

func TestDeviceService_Rename(t *testing.T) {
	fd := &fakeDevices{devices: []api.Device{{ID: "a", FriendlyName: "old"}}}
	s := api.NewDeviceService(fd, fd)
	resp, err := s.Rename(context.Background(), connect.NewRequest(&v1.RenameDeviceRequest{Id: "a", NewFriendlyName: "new"}))
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if resp.Msg.Device.FriendlyName != "new" {
		t.Errorf("name = %q", resp.Msg.Device.FriendlyName)
	}
	if len(fd.renamed) != 1 || fd.renamed[0].name != "new" {
		t.Errorf("rename record = %+v", fd.renamed)
	}
}

func TestDeviceService_Rename_NotFound(t *testing.T) {
	fd := &fakeDevices{}
	s := api.NewDeviceService(fd, fd)
	_, err := s.Rename(context.Background(), connect.NewRequest(&v1.RenameDeviceRequest{Id: "nope", NewFriendlyName: "x"}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeNotFound {
		t.Fatalf("err = %v", err)
	}
}
```

- [ ] **Step 5: Run — fail**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 6: Write `service_device.go`**

```go
package api

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/gen/gohome/v1alpha1/v1alpha1connect"
	"github.com/fynn-labs/gohome/internal/auth"
)

type DeviceService struct {
	r DeviceReader
	w DeviceWriter
}

func NewDeviceService(r DeviceReader, w DeviceWriter) *DeviceService { return &DeviceService{r: r, w: w} }

var _ v1alpha1connect.DeviceServiceHandler = (*DeviceService)(nil)

func (s *DeviceService) List(ctx context.Context, req *connect.Request[v1.ListDevicesRequest]) (*connect.Response[v1.ListDevicesResponse], error) {
	cur, err := DecodeCursor(pageToken(req.Msg.Page))
	if err != nil {
		return nil, ToConnect(ctx, ErrValidationFailed, "bad_page_token")
	}
	pr := PageReq{Size: ClampPageSize(pageSize(req.Msg.Page)), Cursor: cur}
	devs, next, err := s.r.ListDevices(ctx, req.Msg.AreaId, pr)
	if err != nil {
		return nil, ToConnect(ctx, err, "list_failed")
	}
	out := &v1.ListDevicesResponse{Page: &v1.PageResponse{}}
	tok, _ := EncodeCursor(next)
	out.Page.NextPageToken = tok
	for _, d := range devs {
		out.Devices = append(out.Devices, deviceToProto(d))
	}
	return connect.NewResponse(out), nil
}

func (s *DeviceService) Get(ctx context.Context, req *connect.Request[v1.GetDeviceRequest]) (*connect.Response[v1.GetDeviceResponse], error) {
	d, err := s.r.GetDevice(ctx, req.Msg.Id)
	if err != nil {
		return nil, ToConnect(ctx, err, "device_not_found")
	}
	return connect.NewResponse(&v1.GetDeviceResponse{Device: deviceToProto(d)}), nil
}

func (s *DeviceService) Rename(ctx context.Context, req *connect.Request[v1.RenameDeviceRequest]) (*connect.Response[v1.RenameDeviceResponse], error) {
	if req.Msg.NewFriendlyName == "" {
		return nil, ToConnect(ctx, ErrValidationFailed, "empty_friendly_name")
	}
	d, err := s.w.RenameDevice(ctx, req.Msg.Id, req.Msg.NewFriendlyName, principalID(ctx))
	if err != nil {
		return nil, ToConnect(ctx, err, "rename_failed")
	}
	return connect.NewResponse(&v1.RenameDeviceResponse{Device: deviceToProto(d)}), nil
}

func (s *DeviceService) Reassign(ctx context.Context, req *connect.Request[v1.ReassignDeviceRequest]) (*connect.Response[v1.ReassignDeviceResponse], error) {
	d, err := s.w.ReassignDevice(ctx, req.Msg.Id, req.Msg.NewAreaId, principalID(ctx))
	if err != nil {
		return nil, ToConnect(ctx, err, "reassign_failed")
	}
	return connect.NewResponse(&v1.ReassignDeviceResponse{Device: deviceToProto(d)}), nil
}

func deviceToProto(d Device) *v1.Device {
	return &v1.Device{
		Id:               d.ID,
		FriendlyName:     d.FriendlyName,
		AreaId:           d.AreaID,
		DriverInstanceId: d.DriverInstanceID,
		EntityIds:        d.EntityIDs,
	}
}

func principalID(ctx context.Context) string {
	if p, ok := auth.PrincipalFromContext(ctx); ok {
		return p.ID
	}
	return ""
}
```

- [ ] **Step 7: Run — pass**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 8: Commit**

```bash
git add proto/gohome/v1alpha1/device.proto gen/gohome/v1alpha1/ internal/api/deps.go internal/api/service_device.go internal/api/service_device_test.go
git commit -m "feat(c7): DeviceService"
```

> **Note:** the actual emit of `DeviceRenamed` / `DeviceReassigned` events happens in the `internal/registry` adapter (Task 21 wires `DeviceWriter` to a registry method that does `eventstore.Append(DeviceRenamed{...})`). The handler is event-agnostic; the backend owns the side effect.

---

## Task 10: `EntityService` — List, Get, CallCapability (Subscribe in Task 17)

**Files:**
- Create: `proto/gohome/v1alpha1/entity.proto`
- Modify: `internal/api/deps.go`
- Create: `internal/api/service_entity.go`
- Create: `internal/api/service_entity_test.go`

- [ ] **Step 1: Write `entity.proto`**

```protobuf
syntax = "proto3";
package gohome.v1alpha1;
import "gohome/v1alpha1/common.proto";
import "gohome/entity/v1/attributes.proto";
import "google/protobuf/struct.proto";
import "google/protobuf/timestamp.proto";

service EntityService {
  rpc List          (ListEntitiesRequest)    returns (ListEntitiesResponse);
  rpc Get           (GetEntityRequest)       returns (GetEntityResponse);
  rpc CallCapability(CallCapabilityRequest)  returns (CallCapabilityResponse);
  rpc Subscribe     (SubscribeEntitiesRequest) returns (stream SubscribeEntitiesResponse);
}

message Entity {
  // 1-9: identity
  string id            = 1;
  string type          = 2;
  string device_id     = 3;
  string area_id       = 4;
  string zone_id       = 5;
  string friendly_name = 6;

  // 10-19: payload
  gohome.entity.v1.Attributes state        = 10;
  gohome.entity.v1.Attributes capabilities = 11;
}

message ListEntitiesRequest {
  PageRequest    page     = 1;
  EntitySelector selector = 2;
}
message ListEntitiesResponse {
  repeated Entity entities = 1;
  PageResponse    page     = 2;
}

message GetEntityRequest  { string id = 1; }
message GetEntityResponse { Entity entity = 1; }

message CallCapabilityRequest {
  string                  entity_id  = 1;
  string                  capability = 2;
  google.protobuf.Struct  parameters = 3;
}
message CallCapabilityResponse {
  string correlation_id = 1;  // command id
}

message SubscribeEntitiesRequest {
  EntitySelector selector    = 1;
  uint64         from_cursor = 2;  // 0 = live from now
}
message SubscribeEntitiesResponse {
  oneof kind {
    EntityChange change    = 1;
    Heartbeat    heartbeat = 2;
  }
}

message EntityChange {
  string                       entity_id = 1;
  uint64                       cursor    = 2;
  google.protobuf.Timestamp    at        = 3;
  // Either state changed or registry mutated; both convey current state.
  Entity                       entity    = 10;
}
```

- [ ] **Step 2: Regenerate**

```bash
task proto
```

- [ ] **Step 3: Extend `deps.go`**

```go
import (
	"context"

	entityv1 "github.com/fynn-labs/gohome/gen/gohome/entity/v1"
)

type Entity struct {
	ID, Type, DeviceID, AreaID, ZoneID, FriendlyName string
	State        *entityv1.Attributes
	Capabilities *entityv1.Attributes
}

type EntitySelector struct {
	EntityIDs []string
	DeviceIDs []string
	Areas     []string
	Zones     []string
	Classes   []string
}

type EntityReader interface {
	ListEntities(ctx context.Context, sel EntitySelector, page PageReq) ([]Entity, Cursor, error)
	GetEntity(ctx context.Context, id string) (Entity, error)
}

type CapabilityCaller interface {
	// Call dispatches the capability invocation through the carport supervisor;
	// blocks until the driver acks or ctx is cancelled. Returns the command's
	// correlation id (the CommandIssued event id) on success.
	Call(ctx context.Context, entityID, capability string, params map[string]any) (correlationID string, err error)
}
```

- [ ] **Step 4: Failing tests**

Create `internal/api/service_entity_test.go`:

```go
package api_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/internal/api"
	"google.golang.org/protobuf/types/known/structpb"
)

type fakeEntities struct{ entities []api.Entity }

func (f *fakeEntities) ListEntities(_ context.Context, sel api.EntitySelector, _ api.PageReq) ([]api.Entity, api.Cursor, error) {
	var out []api.Entity
	for _, e := range f.entities {
		if len(sel.Areas) > 0 && !contains(sel.Areas, e.AreaID) {
			continue
		}
		out = append(out, e)
	}
	return out, api.Cursor{}, nil
}
func (f *fakeEntities) GetEntity(_ context.Context, id string) (api.Entity, error) {
	for _, e := range f.entities {
		if e.ID == id {
			return e, nil
		}
	}
	return api.Entity{}, api.ErrEntityNotFound
}

type fakeCaller struct {
	called    []callRec
	returnErr error
}
type callRec struct{ id, cap string }

func (f *fakeCaller) Call(_ context.Context, id, cap string, _ map[string]any) (string, error) {
	if f.returnErr != nil {
		return "", f.returnErr
	}
	f.called = append(f.called, callRec{id, cap})
	return "cmd-" + id, nil
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func TestEntityService_List_AreaFilter(t *testing.T) {
	s := api.NewEntityService(&fakeEntities{entities: []api.Entity{
		{ID: "light.a", AreaID: "kitchen"},
		{ID: "light.b", AreaID: "bedroom"},
	}}, nil)
	resp, err := s.List(context.Background(), connect.NewRequest(&v1.ListEntitiesRequest{
		Selector: &v1.EntitySelector{Areas: []string{"kitchen"}},
	}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Msg.Entities) != 1 || resp.Msg.Entities[0].Id != "light.a" {
		t.Errorf("got %+v", resp.Msg.Entities)
	}
}

func TestEntityService_CallCapability(t *testing.T) {
	fc := &fakeCaller{}
	s := api.NewEntityService(&fakeEntities{entities: []api.Entity{{ID: "light.a"}}}, fc)
	params, _ := structpb.NewStruct(map[string]any{"brightness": 75})
	resp, err := s.CallCapability(context.Background(), connect.NewRequest(&v1.CallCapabilityRequest{
		EntityId: "light.a", Capability: "set_brightness", Parameters: params,
	}))
	if err != nil {
		t.Fatalf("CallCapability: %v", err)
	}
	if resp.Msg.CorrelationId != "cmd-light.a" {
		t.Errorf("correlation = %q", resp.Msg.CorrelationId)
	}
}

func TestEntityService_CallCapability_DriverDown(t *testing.T) {
	fc := &fakeCaller{returnErr: api.ErrDriverUnavailable}
	s := api.NewEntityService(&fakeEntities{}, fc)
	_, err := s.CallCapability(context.Background(), connect.NewRequest(&v1.CallCapabilityRequest{
		EntityId: "light.a", Capability: "turn_on",
	}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeUnavailable {
		t.Fatalf("err = %v", err)
	}
}
```

- [ ] **Step 5: Run — fail**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 6: Write `service_entity.go`** (Subscribe stub for now; implemented in Task 17)

```go
package api

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/gen/gohome/v1alpha1/v1alpha1connect"
)

type EntityService struct {
	r      EntityReader
	caller CapabilityCaller

	// streamSource is plugged in by Task 17. Nil means Subscribe returns
	// UNIMPLEMENTED.
	streamSource EntityStreamSource
}

func NewEntityService(r EntityReader, caller CapabilityCaller) *EntityService {
	return &EntityService{r: r, caller: caller}
}

// SetStreamSource wires the live subscription source after construction (used
// by Task 17 to avoid breaking the constructor signature).
func (s *EntityService) SetStreamSource(src EntityStreamSource) { s.streamSource = src }

var _ v1alpha1connect.EntityServiceHandler = (*EntityService)(nil)

func (s *EntityService) List(ctx context.Context, req *connect.Request[v1.ListEntitiesRequest]) (*connect.Response[v1.ListEntitiesResponse], error) {
	cur, err := DecodeCursor(pageToken(req.Msg.Page))
	if err != nil {
		return nil, ToConnect(ctx, ErrValidationFailed, "bad_page_token")
	}
	pr := PageReq{Size: ClampPageSize(pageSize(req.Msg.Page)), Cursor: cur}
	sel := selectorFromProto(req.Msg.Selector)
	ents, next, err := s.r.ListEntities(ctx, sel, pr)
	if err != nil {
		return nil, ToConnect(ctx, err, "list_failed")
	}
	out := &v1.ListEntitiesResponse{Page: &v1.PageResponse{}}
	tok, _ := EncodeCursor(next)
	out.Page.NextPageToken = tok
	for _, e := range ents {
		out.Entities = append(out.Entities, entityToProto(e))
	}
	return connect.NewResponse(out), nil
}

func (s *EntityService) Get(ctx context.Context, req *connect.Request[v1.GetEntityRequest]) (*connect.Response[v1.GetEntityResponse], error) {
	e, err := s.r.GetEntity(ctx, req.Msg.Id)
	if err != nil {
		return nil, ToConnect(ctx, err, "entity_not_found")
	}
	return connect.NewResponse(&v1.GetEntityResponse{Entity: entityToProto(e)}), nil
}

func (s *EntityService) CallCapability(ctx context.Context, req *connect.Request[v1.CallCapabilityRequest]) (*connect.Response[v1.CallCapabilityResponse], error) {
	if req.Msg.EntityId == "" || req.Msg.Capability == "" {
		return nil, ToConnect(ctx, ErrValidationFailed, "missing_required_field")
	}
	var params map[string]any
	if req.Msg.Parameters != nil {
		params = req.Msg.Parameters.AsMap()
	}
	cid, err := s.caller.Call(ctx, req.Msg.EntityId, req.Msg.Capability, params)
	if err != nil {
		return nil, ToConnect(ctx, err, "call_failed")
	}
	return connect.NewResponse(&v1.CallCapabilityResponse{CorrelationId: cid}), nil
}

func entityToProto(e Entity) *v1.Entity {
	return &v1.Entity{
		Id:           e.ID,
		Type:         e.Type,
		DeviceId:     e.DeviceID,
		AreaId:       e.AreaID,
		ZoneId:       e.ZoneID,
		FriendlyName: e.FriendlyName,
		State:        e.State,
		Capabilities: e.Capabilities,
	}
}

func selectorFromProto(p *v1.EntitySelector) EntitySelector {
	if p == nil {
		return EntitySelector{}
	}
	return EntitySelector{
		EntityIDs: p.EntityIds,
		DeviceIDs: p.DeviceIds,
		Areas:     p.Areas,
		Zones:     p.Zones,
		Classes:   p.Classes,
	}
}
```

Add a placeholder for the streaming dep (Task 17 fills it in):

```go
// EntityStreamSource is implemented in Task 17. Declared here so the field
// type compiles; concrete impl is added later.
type EntityStreamSource interface {
	// Subscribe returns a channel of EntityChange events filtered by sel,
	// optionally replaying from fromCursor. The returned cancel func MUST be
	// called to release server-side resources.
	Subscribe(ctx context.Context, sel EntitySelector, fromCursor uint64) (<-chan EntityChange, func(), error)
}

type EntityChange struct {
	EntityID string
	Cursor   uint64
	AtUnixMs int64
	Entity   Entity
}
```

Stub Subscribe in `service_entity.go`:

```go
func (s *EntityService) Subscribe(ctx context.Context, req *connect.Request[v1.SubscribeEntitiesRequest], stream *connect.ServerStream[v1.SubscribeEntitiesResponse]) error {
	if s.streamSource == nil {
		return ToConnect(ctx, ErrNotImplemented, "subscribe_unimplemented")
	}
	// Real impl in Task 17.
	return ToConnect(ctx, ErrNotImplemented, "subscribe_unimplemented")
}
```

- [ ] **Step 7: Run — pass**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 8: Commit**

```bash
git add proto/gohome/v1alpha1/entity.proto gen/gohome/v1alpha1/ internal/api/deps.go internal/api/service_entity.go internal/api/service_entity_test.go
git commit -m "feat(c7): EntityService (List, Get, CallCapability)"
```

---

## Task 11: `DriverService`

**Files:**
- Create: `proto/gohome/v1alpha1/driver.proto`
- Modify: `internal/api/deps.go`
- Create: `internal/api/service_driver.go`
- Create: `internal/api/service_driver_test.go`

- [ ] **Step 1: Write `driver.proto`**

```protobuf
syntax = "proto3";
package gohome.v1alpha1;
import "gohome/v1alpha1/common.proto";
import "google/protobuf/timestamp.proto";

service DriverService {
  rpc ListDrivers    (ListDriversRequest)    returns (ListDriversResponse);
  rpc ListInstances  (ListInstancesRequest)  returns (ListInstancesResponse);
  rpc InstanceHealth (InstanceHealthRequest) returns (InstanceHealthResponse);
  rpc RestartInstance(RestartInstanceRequest) returns (RestartInstanceResponse);
}

message Driver {
  string name              = 1;
  string version           = 2;
  string description       = 3;
  repeated string entity_classes = 4;
}
message DriverInstance {
  string id                 = 1;
  string driver_name        = 2;
  string status             = 3;   // "running" | "down" | "starting" | "crashed"
  uint32 entity_count       = 4;
  google.protobuf.Timestamp last_handshake = 10;
}

message ListDriversRequest    { PageRequest page = 1; }
message ListDriversResponse   { repeated Driver drivers = 1; PageResponse page = 2; }

message ListInstancesRequest  { PageRequest page = 1; }
message ListInstancesResponse { repeated DriverInstance instances = 1; PageResponse page = 2; }

message InstanceHealthRequest  { string instance_id = 1; }
message InstanceHealthResponse { bool ok = 1; string detail = 2; }

message RestartInstanceRequest  { string instance_id = 1; string reason = 2; }
message RestartInstanceResponse { bool restarted = 1; }
```

- [ ] **Step 2: Regenerate**

```bash
task proto
```

- [ ] **Step 3: Extend `deps.go`**

```go
type Driver struct {
	Name, Version, Description string
	EntityClasses              []string
}
type DriverInstance struct {
	ID, DriverName, Status string
	EntityCount            uint32
	LastHandshakeUnixMs    int64
}
type DriverControl interface {
	ListDrivers(ctx context.Context, page PageReq) ([]Driver, Cursor, error)
	ListInstances(ctx context.Context, page PageReq) ([]DriverInstance, Cursor, error)
	InstanceHealth(ctx context.Context, instanceID string) (ok bool, detail string, err error)
	RestartInstance(ctx context.Context, instanceID, reason, actor string) error
}
```

- [ ] **Step 4: Failing tests**

Create `internal/api/service_driver_test.go`:

```go
package api_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/internal/api"
)

type fakeDrivers struct {
	drivers   []api.Driver
	instances []api.DriverInstance
	healthOK  bool
	healthErr error
	restarts  []string
}

func (f *fakeDrivers) ListDrivers(_ context.Context, _ api.PageReq) ([]api.Driver, api.Cursor, error) {
	return f.drivers, api.Cursor{}, nil
}
func (f *fakeDrivers) ListInstances(_ context.Context, _ api.PageReq) ([]api.DriverInstance, api.Cursor, error) {
	return f.instances, api.Cursor{}, nil
}
func (f *fakeDrivers) InstanceHealth(_ context.Context, id string) (bool, string, error) {
	if f.healthErr != nil {
		return false, "", f.healthErr
	}
	return f.healthOK, "", nil
}
func (f *fakeDrivers) RestartInstance(_ context.Context, id, _, _ string) error {
	for _, in := range f.instances {
		if in.ID == id {
			f.restarts = append(f.restarts, id)
			return nil
		}
	}
	return api.ErrInstanceNotFound
}

func TestDriverService_RestartInstance(t *testing.T) {
	fd := &fakeDrivers{instances: []api.DriverInstance{{ID: "hue-1"}}}
	s := api.NewDriverService(fd)
	resp, err := s.RestartInstance(context.Background(), connect.NewRequest(&v1.RestartInstanceRequest{InstanceId: "hue-1", Reason: "manual"}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !resp.Msg.Restarted || len(fd.restarts) != 1 {
		t.Errorf("not restarted")
	}
}

func TestDriverService_RestartInstance_NotFound(t *testing.T) {
	fd := &fakeDrivers{}
	s := api.NewDriverService(fd)
	_, err := s.RestartInstance(context.Background(), connect.NewRequest(&v1.RestartInstanceRequest{InstanceId: "nope"}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeNotFound {
		t.Fatalf("err: %v", err)
	}
}
```

- [ ] **Step 5: Run — fail**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 6: Write `service_driver.go`**

```go
package api

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/gen/gohome/v1alpha1/v1alpha1connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type DriverService struct{ be DriverControl }

func NewDriverService(be DriverControl) *DriverService { return &DriverService{be: be} }

var _ v1alpha1connect.DriverServiceHandler = (*DriverService)(nil)

func (s *DriverService) ListDrivers(ctx context.Context, req *connect.Request[v1.ListDriversRequest]) (*connect.Response[v1.ListDriversResponse], error) {
	cur, err := DecodeCursor(pageToken(req.Msg.Page))
	if err != nil {
		return nil, ToConnect(ctx, ErrValidationFailed, "bad_page_token")
	}
	drivers, next, err := s.be.ListDrivers(ctx, PageReq{Size: ClampPageSize(pageSize(req.Msg.Page)), Cursor: cur})
	if err != nil {
		return nil, ToConnect(ctx, err, "list_failed")
	}
	out := &v1.ListDriversResponse{Page: &v1.PageResponse{}}
	tok, _ := EncodeCursor(next)
	out.Page.NextPageToken = tok
	for _, d := range drivers {
		out.Drivers = append(out.Drivers, &v1.Driver{
			Name: d.Name, Version: d.Version, Description: d.Description, EntityClasses: d.EntityClasses,
		})
	}
	return connect.NewResponse(out), nil
}

func (s *DriverService) ListInstances(ctx context.Context, req *connect.Request[v1.ListInstancesRequest]) (*connect.Response[v1.ListInstancesResponse], error) {
	cur, err := DecodeCursor(pageToken(req.Msg.Page))
	if err != nil {
		return nil, ToConnect(ctx, ErrValidationFailed, "bad_page_token")
	}
	insts, next, err := s.be.ListInstances(ctx, PageReq{Size: ClampPageSize(pageSize(req.Msg.Page)), Cursor: cur})
	if err != nil {
		return nil, ToConnect(ctx, err, "list_failed")
	}
	out := &v1.ListInstancesResponse{Page: &v1.PageResponse{}}
	tok, _ := EncodeCursor(next)
	out.Page.NextPageToken = tok
	for _, in := range insts {
		var t *timestamppb.Timestamp
		if in.LastHandshakeUnixMs > 0 {
			t = timestamppb.New(unixMsToTime(in.LastHandshakeUnixMs))
		}
		out.Instances = append(out.Instances, &v1.DriverInstance{
			Id: in.ID, DriverName: in.DriverName, Status: in.Status, EntityCount: in.EntityCount, LastHandshake: t,
		})
	}
	return connect.NewResponse(out), nil
}

func (s *DriverService) InstanceHealth(ctx context.Context, req *connect.Request[v1.InstanceHealthRequest]) (*connect.Response[v1.InstanceHealthResponse], error) {
	ok, detail, err := s.be.InstanceHealth(ctx, req.Msg.InstanceId)
	if err != nil {
		return nil, ToConnect(ctx, err, "health_failed")
	}
	return connect.NewResponse(&v1.InstanceHealthResponse{Ok: ok, Detail: detail}), nil
}

func (s *DriverService) RestartInstance(ctx context.Context, req *connect.Request[v1.RestartInstanceRequest]) (*connect.Response[v1.RestartInstanceResponse], error) {
	if err := s.be.RestartInstance(ctx, req.Msg.InstanceId, req.Msg.Reason, principalID(ctx)); err != nil {
		return nil, ToConnect(ctx, err, "restart_failed")
	}
	return connect.NewResponse(&v1.RestartInstanceResponse{Restarted: true}), nil
}
```

Add to `internal/api/time.go`:

```go
func unixMsToTime(ms int64) time.Time { return time.Unix(0, ms*int64(time.Millisecond)).UTC() }
```

- [ ] **Step 7: Run — pass**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 8: Commit**

```bash
git add proto/gohome/v1alpha1/driver.proto gen/gohome/v1alpha1/ internal/api/deps.go internal/api/service_driver.go internal/api/service_driver_test.go internal/api/time.go
git commit -m "feat(c7): DriverService"
```

---

## Task 12: `EventService.Query` (unary; Tail in Task 17)

**Files:**
- Create: `proto/gohome/v1alpha1/event.proto`
- Modify: `internal/api/deps.go`
- Create: `internal/api/service_event.go`
- Create: `internal/api/service_event_test.go`

- [ ] **Step 1: Write `event.proto`**

```protobuf
syntax = "proto3";
package gohome.v1alpha1;
import "gohome/v1alpha1/common.proto";
import "gohome/event/v1/event.proto";
import "google/protobuf/timestamp.proto";

service EventService {
  rpc Query(QueryEventsRequest) returns (QueryEventsResponse);
  rpc Tail (TailEventsRequest)  returns (stream TailEventsResponse);
}

message Event {
  // 1-9: identity
  uint64 cursor                            = 1;
  google.protobuf.Timestamp at             = 2;
  string kind                              = 3;
  string entity                            = 4;
  string source                            = 5;
  string correlation_id                    = 6;
  string cause_id                          = 7;

  // 10-19: payload
  gohome.event.v1.Payload payload = 10;
}

message QueryEventsRequest {
  PageRequest page   = 1;
  EventFilter filter = 2;
}
message QueryEventsResponse {
  repeated Event events = 1;
  PageResponse   page   = 2;
}

message TailEventsRequest {
  EventFilter filter = 1;
}
message TailEventsResponse {
  oneof kind {
    Event     event     = 1;
    Heartbeat heartbeat = 2;
  }
}
```

- [ ] **Step 2: Regenerate**

```bash
task proto
```

- [ ] **Step 3: Extend `deps.go`**

```go
import (
	"time"

	eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"
)

type Event struct {
	Cursor        uint64
	At            time.Time
	Kind          string
	Entity        string
	Source        string
	CorrelationID string
	CauseID       string
	Payload       *eventv1.Payload
}

type EventFilter struct {
	Kinds        []string
	EntityPrefix string
	Sources      []string
	FromCursor   uint64
	ToCursor     uint64
	FromTime     time.Time
	ToTime       time.Time
}

type EventSource interface {
	Query(ctx context.Context, filter EventFilter, page PageReq) ([]Event, Cursor, error)
	// Subscribe is implemented in Task 17.
	Subscribe(ctx context.Context, filter EventFilter) (<-chan Event, func(), error)
}
```

- [ ] **Step 4: Failing tests**

Create `internal/api/service_event_test.go`:

```go
package api_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/internal/api"
)

type fakeEvents struct{ events []api.Event }

func (f *fakeEvents) Query(_ context.Context, filter api.EventFilter, _ api.PageReq) ([]api.Event, api.Cursor, error) {
	var out []api.Event
	for _, e := range f.events {
		if len(filter.Kinds) > 0 && !contains(filter.Kinds, e.Kind) {
			continue
		}
		out = append(out, e)
	}
	return out, api.Cursor{}, nil
}
func (f *fakeEvents) Subscribe(_ context.Context, _ api.EventFilter) (<-chan api.Event, func(), error) {
	ch := make(chan api.Event)
	close(ch)
	return ch, func() {}, nil
}

func TestEventService_Query_KindFilter(t *testing.T) {
	s := api.NewEventService(&fakeEvents{events: []api.Event{
		{Cursor: 1, Kind: "state_changed", At: time.Unix(1700, 0)},
		{Cursor: 2, Kind: "command_issued", At: time.Unix(1701, 0), Payload: &eventv1.Payload{}},
	}})
	resp, err := s.Query(context.Background(), connect.NewRequest(&v1.QueryEventsRequest{
		Filter: &v1.EventFilter{Kinds: []string{"state_changed"}},
	}))
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(resp.Msg.Events) != 1 || resp.Msg.Events[0].Kind != "state_changed" {
		t.Errorf("got %+v", resp.Msg.Events)
	}
}
```

- [ ] **Step 5: Run — fail**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 6: Write `service_event.go`** (Tail stubbed; Task 17 fills it in)

```go
package api

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/gen/gohome/v1alpha1/v1alpha1connect"
)

type EventService struct{ src EventSource }

func NewEventService(src EventSource) *EventService { return &EventService{src: src} }

var _ v1alpha1connect.EventServiceHandler = (*EventService)(nil)

func (s *EventService) Query(ctx context.Context, req *connect.Request[v1.QueryEventsRequest]) (*connect.Response[v1.QueryEventsResponse], error) {
	cur, err := DecodeCursor(pageToken(req.Msg.Page))
	if err != nil {
		return nil, ToConnect(ctx, ErrValidationFailed, "bad_page_token")
	}
	filter := filterFromProto(req.Msg.Filter)
	events, next, err := s.src.Query(ctx, filter, PageReq{Size: ClampPageSize(pageSize(req.Msg.Page)), Cursor: cur})
	if err != nil {
		return nil, ToConnect(ctx, err, "query_failed")
	}
	out := &v1.QueryEventsResponse{Page: &v1.PageResponse{}}
	tok, _ := EncodeCursor(next)
	out.Page.NextPageToken = tok
	for _, e := range events {
		out.Events = append(out.Events, eventToProto(e))
	}
	return connect.NewResponse(out), nil
}

func (s *EventService) Tail(ctx context.Context, req *connect.Request[v1.TailEventsRequest], stream *connect.ServerStream[v1.TailEventsResponse]) error {
	// Real impl in Task 17.
	return ToConnect(ctx, ErrNotImplemented, "tail_unimplemented")
}

func filterFromProto(p *v1.EventFilter) EventFilter {
	if p == nil {
		return EventFilter{}
	}
	return EventFilter{
		Kinds:        p.Kinds,
		EntityPrefix: p.EntityPrefix,
		Sources:      p.Sources,
		FromCursor:   p.FromCursor,
		ToCursor:     p.ToCursor,
		FromTime:     GoTime(p.FromTime),
		ToTime:       GoTime(p.ToTime),
	}
}

func eventToProto(e Event) *v1.Event {
	return &v1.Event{
		Cursor:        e.Cursor,
		At:            ProtoTime(e.At),
		Kind:          e.Kind,
		Entity:        e.Entity,
		Source:        e.Source,
		CorrelationId: e.CorrelationID,
		CauseId:       e.CauseID,
		Payload:       e.Payload,
	}
}
```

- [ ] **Step 7: Run — pass**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 8: Commit**

```bash
git add proto/gohome/v1alpha1/event.proto gen/gohome/v1alpha1/ internal/api/deps.go internal/api/service_event.go internal/api/service_event_test.go
git commit -m "feat(c7): EventService.Query (Tail in task 17)"
```

---

## Task 13: `ConfigService`

**Files:**
- Create: `proto/gohome/v1alpha1/config.proto`
- Modify: `internal/api/deps.go`
- Create: `internal/api/service_config.go`
- Create: `internal/api/service_config_test.go`

- [ ] **Step 1: Write `config.proto`**

```protobuf
syntax = "proto3";
package gohome.v1alpha1;
import "gohome/config/v1/snapshot.proto";

service ConfigService {
  rpc Validate   (ValidateConfigRequest)     returns (ValidateConfigResponse);
  rpc Apply      (ApplyConfigRequest)        returns (ApplyConfigResponse);
  rpc Reload     (ReloadConfigRequest)       returns (ReloadConfigResponse);
  rpc GetArtifact(GetConfigArtifactRequest)  returns (GetConfigArtifactResponse);
}

message ValidateConfigRequest  { bytes pkl_bundle = 1; }
message ValidateConfigResponse {
  bool                       valid       = 1;
  repeated string            errors      = 2;
  ConfigDiff                 diff        = 3;
  string                     bundle_hash = 4;  // sha256 hex of input
}

message ApplyConfigRequest {
  // 1-9: payload
  bytes  pkl_bundle = 1;
  string message    = 2;

  // 10-19: mode
  bool   dry_run = 10;
  bool   strict  = 11;
  string expected_bundle_hash = 12;  // required when strict=true
}
message ApplyConfigResponse {
  bool                       applied        = 1;
  ConfigDiff                 diff           = 2;
  string                     correlation_id = 3;  // ConfigApplied event id
  string                     bundle_hash    = 4;
}

message ReloadConfigRequest  {}
message ReloadConfigResponse { ConfigDiff diff = 1; string correlation_id = 2; }

message GetConfigArtifactRequest  {}
message GetConfigArtifactResponse { gohome.config.v1.ConfigSnapshot snapshot = 1; }

message ConfigDiff {
  // 1-9: counts
  int32 driver_instances_added   = 1;
  int32 driver_instances_removed = 2;
  int32 driver_instances_changed = 3;
  int32 entities_added           = 4;
  int32 entities_removed         = 5;
  int32 automations_changed      = 6;

  // 10-19: human-readable lines
  repeated string lines = 10;
}
```

- [ ] **Step 2: Regenerate**

```bash
task proto
```

- [ ] **Step 3: Extend `deps.go`**

```go
import (
	configv1 "github.com/fynn-labs/gohome/gen/gohome/config/v1"
)

type ConfigDiff struct {
	DriverAdded, DriverRemoved, DriverChanged int32
	EntitiesAdded, EntitiesRemoved            int32
	AutomationsChanged                        int32
	Lines                                     []string
}

type ConfigApplyResult struct {
	Applied       bool
	Diff          ConfigDiff
	CorrelationID string
	BundleHash    string
	Errors        []string
}

type ConfigApplier interface {
	Validate(ctx context.Context, pklBundle []byte) (valid bool, errs []string, diff ConfigDiff, hash string, err error)
	Apply(ctx context.Context, pklBundle []byte, message, expectedHash string, dryRun, strict bool, actor string) (ConfigApplyResult, error)
	Reload(ctx context.Context, actor string) (diff ConfigDiff, correlationID string, err error)
	CurrentArtifact(ctx context.Context) (*configv1.ConfigSnapshot, error)
}
```

- [ ] **Step 4: Failing tests**

Create `internal/api/service_config_test.go`:

```go
package api_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	configv1 "github.com/fynn-labs/gohome/gen/gohome/config/v1"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/internal/api"
)

type fakeConfig struct {
	validErrs   []string
	applyErr    error
	applyResult api.ConfigApplyResult
	current     *configv1.ConfigSnapshot
}

func (f *fakeConfig) Validate(_ context.Context, _ []byte) (bool, []string, api.ConfigDiff, string, error) {
	return len(f.validErrs) == 0, f.validErrs, api.ConfigDiff{}, "abc123", nil
}
func (f *fakeConfig) Apply(_ context.Context, _ []byte, _, _ string, _, _ bool, _ string) (api.ConfigApplyResult, error) {
	if f.applyErr != nil {
		return api.ConfigApplyResult{}, f.applyErr
	}
	return f.applyResult, nil
}
func (f *fakeConfig) Reload(_ context.Context, _ string) (api.ConfigDiff, string, error) {
	return api.ConfigDiff{Lines: []string{"reloaded"}}, "corr-1", nil
}
func (f *fakeConfig) CurrentArtifact(_ context.Context) (*configv1.ConfigSnapshot, error) {
	return f.current, nil
}

func TestConfigService_Validate_OK(t *testing.T) {
	s := api.NewConfigService(&fakeConfig{})
	resp, err := s.Validate(context.Background(), connect.NewRequest(&v1.ValidateConfigRequest{PklBundle: []byte("pkl")}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !resp.Msg.Valid || resp.Msg.BundleHash != "abc123" {
		t.Errorf("got %+v", resp.Msg)
	}
}

func TestConfigService_Apply_StrictRequiresHash(t *testing.T) {
	s := api.NewConfigService(&fakeConfig{})
	_, err := s.Apply(context.Background(), connect.NewRequest(&v1.ApplyConfigRequest{Strict: true}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeInvalidArgument {
		t.Fatalf("err = %v", err)
	}
}

func TestConfigService_Apply_DryRun(t *testing.T) {
	s := api.NewConfigService(&fakeConfig{applyResult: api.ConfigApplyResult{Applied: false, Diff: api.ConfigDiff{Lines: []string{"+1 entity"}}, BundleHash: "h"}})
	resp, err := s.Apply(context.Background(), connect.NewRequest(&v1.ApplyConfigRequest{PklBundle: []byte("p"), DryRun: true}))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if resp.Msg.Applied {
		t.Error("Applied true on dry-run")
	}
	if len(resp.Msg.Diff.Lines) != 1 {
		t.Errorf("diff = %+v", resp.Msg.Diff)
	}
}
```

- [ ] **Step 5: Run — fail**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 6: Write `service_config.go`**

```go
package api

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/gen/gohome/v1alpha1/v1alpha1connect"
)

type ConfigService struct{ be ConfigApplier }

func NewConfigService(be ConfigApplier) *ConfigService { return &ConfigService{be: be} }

var _ v1alpha1connect.ConfigServiceHandler = (*ConfigService)(nil)

func (s *ConfigService) Validate(ctx context.Context, req *connect.Request[v1.ValidateConfigRequest]) (*connect.Response[v1.ValidateConfigResponse], error) {
	if len(req.Msg.PklBundle) == 0 {
		return nil, ToConnect(ctx, ErrValidationFailed, "empty_bundle")
	}
	valid, errs, diff, hash, err := s.be.Validate(ctx, req.Msg.PklBundle)
	if err != nil {
		return nil, ToConnect(ctx, err, "validate_failed")
	}
	return connect.NewResponse(&v1.ValidateConfigResponse{
		Valid:      valid,
		Errors:     errs,
		Diff:       diffToProto(diff),
		BundleHash: hash,
	}), nil
}

func (s *ConfigService) Apply(ctx context.Context, req *connect.Request[v1.ApplyConfigRequest]) (*connect.Response[v1.ApplyConfigResponse], error) {
	if req.Msg.Strict && req.Msg.ExpectedBundleHash == "" {
		return nil, ToConnect(ctx, ErrValidationFailed, "strict_requires_hash")
	}
	res, err := s.be.Apply(ctx, req.Msg.PklBundle, req.Msg.Message, req.Msg.ExpectedBundleHash, req.Msg.DryRun, req.Msg.Strict, principalID(ctx))
	if err != nil {
		return nil, ToConnect(ctx, err, "apply_failed")
	}
	return connect.NewResponse(&v1.ApplyConfigResponse{
		Applied:       res.Applied,
		Diff:          diffToProto(res.Diff),
		CorrelationId: res.CorrelationID,
		BundleHash:    res.BundleHash,
	}), nil
}

func (s *ConfigService) Reload(ctx context.Context, _ *connect.Request[v1.ReloadConfigRequest]) (*connect.Response[v1.ReloadConfigResponse], error) {
	diff, cid, err := s.be.Reload(ctx, principalID(ctx))
	if err != nil {
		return nil, ToConnect(ctx, err, "reload_failed")
	}
	return connect.NewResponse(&v1.ReloadConfigResponse{Diff: diffToProto(diff), CorrelationId: cid}), nil
}

func (s *ConfigService) GetArtifact(ctx context.Context, _ *connect.Request[v1.GetConfigArtifactRequest]) (*connect.Response[v1.GetConfigArtifactResponse], error) {
	snap, err := s.be.CurrentArtifact(ctx)
	if err != nil {
		return nil, ToConnect(ctx, err, "get_artifact_failed")
	}
	return connect.NewResponse(&v1.GetConfigArtifactResponse{Snapshot: snap}), nil
}

func diffToProto(d ConfigDiff) *v1.ConfigDiff {
	return &v1.ConfigDiff{
		DriverInstancesAdded:   d.DriverAdded,
		DriverInstancesRemoved: d.DriverRemoved,
		DriverInstancesChanged: d.DriverChanged,
		EntitiesAdded:          d.EntitiesAdded,
		EntitiesRemoved:        d.EntitiesRemoved,
		AutomationsChanged:     d.AutomationsChanged,
		Lines:                  d.Lines,
	}
}
```

- [ ] **Step 7: Run — pass**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 8: Commit**

```bash
git add proto/gohome/v1alpha1/config.proto gen/gohome/v1alpha1/ internal/api/deps.go internal/api/service_config.go internal/api/service_config_test.go
git commit -m "feat(c7): ConfigService"
```

---

## Task 14: `AutomationService` — unary RPCs (Trace stream in Task 17)

**Files:**
- Create: `proto/gohome/v1alpha1/automation.proto`
- Modify: `internal/api/deps.go`
- Create: `internal/api/service_automation.go`
- Create: `internal/api/service_automation_test.go`

- [ ] **Step 1: Write `automation.proto`**

```protobuf
syntax = "proto3";
package gohome.v1alpha1;
import "gohome/v1alpha1/common.proto";
import "google/protobuf/timestamp.proto";

service AutomationService {
  rpc List   (ListAutomationsRequest)        returns (ListAutomationsResponse);
  rpc Get    (GetAutomationRequest)          returns (GetAutomationResponse);
  rpc Enable (SetAutomationEnabledRequest)   returns (SetAutomationEnabledResponse);
  rpc Disable(SetAutomationEnabledRequest)   returns (SetAutomationEnabledResponse);
  rpc Trigger(TriggerAutomationRequest)      returns (TriggerAutomationResponse);
  rpc Trace  (TraceAutomationRequest)        returns (stream TraceAutomationResponse);
}

message Automation {
  string id            = 1;
  string display_name  = 2;
  bool   enabled       = 3;
  string mode          = 4;   // "single" | "restart" | "queued"
  uint32 in_flight     = 5;
}

message ListAutomationsRequest  { PageRequest page = 1; }
message ListAutomationsResponse { repeated Automation automations = 1; PageResponse page = 2; }

message GetAutomationRequest  { string id = 1; }
message GetAutomationResponse { Automation automation = 1; }

message SetAutomationEnabledRequest  { string id = 1; }
message SetAutomationEnabledResponse { Automation automation = 1; }

message TriggerAutomationRequest  { string id = 1; }
message TriggerAutomationResponse { string run_id = 1; }

message TraceAutomationRequest {
  string run_id      = 1;   // "" = follow live runs of this id
  string automation_id = 2;
  uint64 from_cursor = 3;
}

message TraceAutomationResponse {
  oneof kind {
    TraceEvent event     = 1;
    Heartbeat  heartbeat = 2;
  }
}

message TraceEvent {
  uint64                    cursor         = 1;
  google.protobuf.Timestamp at             = 2;
  string                    automation_id  = 3;
  string                    run_id         = 4;
  string                    kind           = 5;   // "triggered" | "condition" | "action_started" | "action_finished" | "finished"
  string                    detail         = 6;   // free-form line
  map<string, string>       metadata       = 7;
}
```

- [ ] **Step 2: Regenerate**

```bash
task proto
```

- [ ] **Step 3: Extend `deps.go`**

```go
type Automation struct {
	ID, DisplayName, Mode string
	Enabled               bool
	InFlight              uint32
}

type TraceEvent struct {
	Cursor        uint64
	At            time.Time
	AutomationID  string
	RunID         string
	Kind          string
	Detail        string
	Metadata      map[string]string
}

type AutomationControl interface {
	List(ctx context.Context, page PageReq) ([]Automation, Cursor, error)
	Get(ctx context.Context, id string) (Automation, error)
	SetEnabled(ctx context.Context, id string, enabled bool, actor string) (Automation, error)
	Trigger(ctx context.Context, id, actor string) (runID string, err error)
	// Trace is implemented in Task 17.
	Trace(ctx context.Context, automationID, runID string, fromCursor uint64) (<-chan TraceEvent, func(), error)
}
```

- [ ] **Step 4: Failing tests**

Create `internal/api/service_automation_test.go`:

```go
package api_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/internal/api"
)

type fakeAutomations struct {
	autos        []api.Automation
	triggerErr   error
	enabledCalls []enabledRec
	triggered    []string
}
type enabledRec struct {
	id      string
	enabled bool
	actor   string
}

func (f *fakeAutomations) List(_ context.Context, _ api.PageReq) ([]api.Automation, api.Cursor, error) {
	return f.autos, api.Cursor{}, nil
}
func (f *fakeAutomations) Get(_ context.Context, id string) (api.Automation, error) {
	for _, a := range f.autos {
		if a.ID == id {
			return a, nil
		}
	}
	return api.Automation{}, api.ErrAutomationNotFound
}
func (f *fakeAutomations) SetEnabled(_ context.Context, id string, en bool, actor string) (api.Automation, error) {
	for i, a := range f.autos {
		if a.ID == id {
			f.autos[i].Enabled = en
			f.enabledCalls = append(f.enabledCalls, enabledRec{id, en, actor})
			return f.autos[i], nil
		}
	}
	return api.Automation{}, api.ErrAutomationNotFound
}
func (f *fakeAutomations) Trigger(_ context.Context, id, _ string) (string, error) {
	if f.triggerErr != nil {
		return "", f.triggerErr
	}
	f.triggered = append(f.triggered, id)
	return "run-" + id, nil
}
func (f *fakeAutomations) Trace(_ context.Context, _, _ string, _ uint64) (<-chan api.TraceEvent, func(), error) {
	ch := make(chan api.TraceEvent)
	close(ch)
	return ch, func() {}, nil
}

func TestAutomationService_Trigger(t *testing.T) {
	fa := &fakeAutomations{autos: []api.Automation{{ID: "test", Enabled: true}}}
	s := api.NewAutomationService(fa)
	resp, err := s.Trigger(context.Background(), connect.NewRequest(&v1.TriggerAutomationRequest{Id: "test"}))
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if resp.Msg.RunId != "run-test" {
		t.Errorf("run = %q", resp.Msg.RunId)
	}
}

func TestAutomationService_Trigger_Disabled(t *testing.T) {
	fa := &fakeAutomations{triggerErr: api.ErrAutomationDisabled}
	s := api.NewAutomationService(fa)
	_, err := s.Trigger(context.Background(), connect.NewRequest(&v1.TriggerAutomationRequest{Id: "x"}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeFailedPrecondition {
		t.Fatalf("err = %v", err)
	}
}

func TestAutomationService_Enable(t *testing.T) {
	fa := &fakeAutomations{autos: []api.Automation{{ID: "x"}}}
	s := api.NewAutomationService(fa)
	resp, err := s.Enable(context.Background(), connect.NewRequest(&v1.SetAutomationEnabledRequest{Id: "x"}))
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !resp.Msg.Automation.Enabled {
		t.Error("not enabled")
	}
}
```

- [ ] **Step 5: Run — fail**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 6: Write `service_automation.go`** (Trace stub)

```go
package api

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/gen/gohome/v1alpha1/v1alpha1connect"
)

type AutomationService struct{ be AutomationControl }

func NewAutomationService(be AutomationControl) *AutomationService { return &AutomationService{be: be} }

var _ v1alpha1connect.AutomationServiceHandler = (*AutomationService)(nil)

func (s *AutomationService) List(ctx context.Context, req *connect.Request[v1.ListAutomationsRequest]) (*connect.Response[v1.ListAutomationsResponse], error) {
	cur, err := DecodeCursor(pageToken(req.Msg.Page))
	if err != nil {
		return nil, ToConnect(ctx, ErrValidationFailed, "bad_page_token")
	}
	autos, next, err := s.be.List(ctx, PageReq{Size: ClampPageSize(pageSize(req.Msg.Page)), Cursor: cur})
	if err != nil {
		return nil, ToConnect(ctx, err, "list_failed")
	}
	out := &v1.ListAutomationsResponse{Page: &v1.PageResponse{}}
	tok, _ := EncodeCursor(next)
	out.Page.NextPageToken = tok
	for _, a := range autos {
		out.Automations = append(out.Automations, automationToProto(a))
	}
	return connect.NewResponse(out), nil
}

func (s *AutomationService) Get(ctx context.Context, req *connect.Request[v1.GetAutomationRequest]) (*connect.Response[v1.GetAutomationResponse], error) {
	a, err := s.be.Get(ctx, req.Msg.Id)
	if err != nil {
		return nil, ToConnect(ctx, err, "automation_not_found")
	}
	return connect.NewResponse(&v1.GetAutomationResponse{Automation: automationToProto(a)}), nil
}

func (s *AutomationService) Enable(ctx context.Context, req *connect.Request[v1.SetAutomationEnabledRequest]) (*connect.Response[v1.SetAutomationEnabledResponse], error) {
	return s.setEnabled(ctx, req.Msg.Id, true)
}

func (s *AutomationService) Disable(ctx context.Context, req *connect.Request[v1.SetAutomationEnabledRequest]) (*connect.Response[v1.SetAutomationEnabledResponse], error) {
	return s.setEnabled(ctx, req.Msg.Id, false)
}

func (s *AutomationService) setEnabled(ctx context.Context, id string, en bool) (*connect.Response[v1.SetAutomationEnabledResponse], error) {
	a, err := s.be.SetEnabled(ctx, id, en, principalID(ctx))
	if err != nil {
		return nil, ToConnect(ctx, err, "set_enabled_failed")
	}
	return connect.NewResponse(&v1.SetAutomationEnabledResponse{Automation: automationToProto(a)}), nil
}

func (s *AutomationService) Trigger(ctx context.Context, req *connect.Request[v1.TriggerAutomationRequest]) (*connect.Response[v1.TriggerAutomationResponse], error) {
	runID, err := s.be.Trigger(ctx, req.Msg.Id, principalID(ctx))
	if err != nil {
		return nil, ToConnect(ctx, err, "trigger_failed")
	}
	return connect.NewResponse(&v1.TriggerAutomationResponse{RunId: runID}), nil
}

func (s *AutomationService) Trace(ctx context.Context, req *connect.Request[v1.TraceAutomationRequest], stream *connect.ServerStream[v1.TraceAutomationResponse]) error {
	// Real impl in Task 17.
	return ToConnect(ctx, ErrNotImplemented, "trace_unimplemented")
}

func automationToProto(a Automation) *v1.Automation {
	return &v1.Automation{
		Id: a.ID, DisplayName: a.DisplayName, Enabled: a.Enabled, Mode: a.Mode, InFlight: a.InFlight,
	}
}
```

- [ ] **Step 7: Run — pass**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 8: Commit**

```bash
git add proto/gohome/v1alpha1/automation.proto gen/gohome/v1alpha1/ internal/api/deps.go internal/api/service_automation.go internal/api/service_automation_test.go
git commit -m "feat(c7): AutomationService unary RPCs"
```

---

## Task 15: `ScriptService` — Run, Cancel, Eval, RunTests, List

**Files:**
- Create: `proto/gohome/v1alpha1/script.proto`
- Modify: `internal/api/deps.go`
- Create: `internal/api/service_script.go`
- Create: `internal/api/service_script_test.go`

- [ ] **Step 1: Write `script.proto`**

```protobuf
syntax = "proto3";
package gohome.v1alpha1;
import "gohome/v1alpha1/common.proto";
import "google/protobuf/struct.proto";
import "google/protobuf/timestamp.proto";

service ScriptService {
  rpc List    (ListScriptsRequest)        returns (ListScriptsResponse);
  rpc Run     (RunScriptRequest)          returns (RunScriptResponse);
  rpc Cancel  (CancelScriptRequest)       returns (CancelScriptResponse);
  rpc Eval    (EvalScriptRequest)         returns (EvalScriptResponse);   // diagnostic-flavor
  rpc RunTests(RunStarlarkTestsRequest)   returns (stream RunStarlarkTestsResponse);  // diagnostic-flavor
}

message Script {
  string name        = 1;
  string description = 2;
}

message ListScriptsRequest  { PageRequest page = 1; }
message ListScriptsResponse { repeated Script scripts = 1; PageResponse page = 2; }

message RunScriptRequest {
  string                  name = 1;
  google.protobuf.Struct  args = 2;
}
message RunScriptResponse {
  string                  run_id = 1;
  google.protobuf.Value   result = 2;
}

message CancelScriptRequest  { string run_id = 1; }
message CancelScriptResponse {}

message EvalScriptRequest  { string expr = 1; }
message EvalScriptResponse {
  google.protobuf.Value result = 1;
  string                stdout = 2;
}

message RunStarlarkTestsRequest  { string path = 1; }
message RunStarlarkTestsResponse {
  oneof kind {
    StarlarkTestEvent event     = 1;
    Heartbeat         heartbeat = 2;
  }
}
message StarlarkTestEvent {
  string                    name     = 1;
  string                    outcome  = 2;   // "pass" | "fail" | "skip"
  string                    detail   = 3;
  google.protobuf.Timestamp at       = 4;
}
```

- [ ] **Step 2: Regenerate**

```bash
task proto
```

- [ ] **Step 3: Extend `deps.go`**

```go
import (
	"google.golang.org/protobuf/types/known/structpb"
)

type Script struct {
	Name, Description string
}

type ScriptRunResult struct {
	RunID  string
	Result *structpb.Value
}

type ScriptRunner interface {
	List(ctx context.Context, page PageReq) ([]Script, Cursor, error)
	Run(ctx context.Context, name string, args map[string]any, actor string) (ScriptRunResult, error)
	Cancel(ctx context.Context, runID string) error
	Eval(ctx context.Context, expr string, actor string) (result *structpb.Value, stdout string, err error)

	// RunTests is implemented in Task 17.
	RunTests(ctx context.Context, path string) (<-chan StarlarkTestEvent, func(), error)
}

type StarlarkTestEvent struct {
	Name, Outcome, Detail string
	At                    time.Time
}
```

- [ ] **Step 4: Failing tests**

Create `internal/api/service_script_test.go`:

```go
package api_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/internal/api"
	"google.golang.org/protobuf/types/known/structpb"
)

type fakeScripts struct {
	scripts []api.Script
	cancels []string
}

func (f *fakeScripts) List(_ context.Context, _ api.PageReq) ([]api.Script, api.Cursor, error) {
	return f.scripts, api.Cursor{}, nil
}
func (f *fakeScripts) Run(_ context.Context, name string, _ map[string]any, _ string) (api.ScriptRunResult, error) {
	if name == "missing" {
		return api.ScriptRunResult{}, api.ErrScriptNotFound
	}
	v, _ := structpb.NewValue("ok")
	return api.ScriptRunResult{RunID: "run-" + name, Result: v}, nil
}
func (f *fakeScripts) Cancel(_ context.Context, runID string) error {
	for _, r := range f.cancels {
		if r == runID {
			return api.ErrRunAlreadyFinished
		}
	}
	f.cancels = append(f.cancels, runID)
	return nil
}
func (f *fakeScripts) Eval(_ context.Context, expr string, _ string) (*structpb.Value, string, error) {
	v, _ := structpb.NewValue(expr)
	return v, "", nil
}
func (f *fakeScripts) RunTests(_ context.Context, _ string) (<-chan api.StarlarkTestEvent, func(), error) {
	ch := make(chan api.StarlarkTestEvent)
	close(ch)
	return ch, func() {}, nil
}

func TestScriptService_Run(t *testing.T) {
	s := api.NewScriptService(&fakeScripts{})
	resp, err := s.Run(context.Background(), connect.NewRequest(&v1.RunScriptRequest{Name: "hello"}))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Msg.RunId != "run-hello" {
		t.Errorf("run = %q", resp.Msg.RunId)
	}
}

func TestScriptService_Run_NotFound(t *testing.T) {
	s := api.NewScriptService(&fakeScripts{})
	_, err := s.Run(context.Background(), connect.NewRequest(&v1.RunScriptRequest{Name: "missing"}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeNotFound {
		t.Fatalf("err = %v", err)
	}
}

func TestScriptService_Cancel_AlreadyFinished(t *testing.T) {
	s := api.NewScriptService(&fakeScripts{cancels: []string{"r1"}})
	_, err := s.Cancel(context.Background(), connect.NewRequest(&v1.CancelScriptRequest{RunId: "r1"}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeFailedPrecondition {
		t.Fatalf("err = %v", err)
	}
}

func TestScriptService_Eval(t *testing.T) {
	s := api.NewScriptService(&fakeScripts{})
	resp, err := s.Eval(context.Background(), connect.NewRequest(&v1.EvalScriptRequest{Expr: "1+1"}))
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if resp.Msg.Result.GetStringValue() != "1+1" {
		t.Errorf("result = %v", resp.Msg.Result)
	}
}
```

- [ ] **Step 5: Run — fail**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 6: Write `service_script.go`** (RunTests stub)

```go
package api

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/gen/gohome/v1alpha1/v1alpha1connect"
)

type ScriptService struct{ be ScriptRunner }

func NewScriptService(be ScriptRunner) *ScriptService { return &ScriptService{be: be} }

var _ v1alpha1connect.ScriptServiceHandler = (*ScriptService)(nil)

func (s *ScriptService) List(ctx context.Context, req *connect.Request[v1.ListScriptsRequest]) (*connect.Response[v1.ListScriptsResponse], error) {
	cur, err := DecodeCursor(pageToken(req.Msg.Page))
	if err != nil {
		return nil, ToConnect(ctx, ErrValidationFailed, "bad_page_token")
	}
	scripts, next, err := s.be.List(ctx, PageReq{Size: ClampPageSize(pageSize(req.Msg.Page)), Cursor: cur})
	if err != nil {
		return nil, ToConnect(ctx, err, "list_failed")
	}
	out := &v1.ListScriptsResponse{Page: &v1.PageResponse{}}
	tok, _ := EncodeCursor(next)
	out.Page.NextPageToken = tok
	for _, sc := range scripts {
		out.Scripts = append(out.Scripts, &v1.Script{Name: sc.Name, Description: sc.Description})
	}
	return connect.NewResponse(out), nil
}

func (s *ScriptService) Run(ctx context.Context, req *connect.Request[v1.RunScriptRequest]) (*connect.Response[v1.RunScriptResponse], error) {
	if req.Msg.Name == "" {
		return nil, ToConnect(ctx, ErrValidationFailed, "missing_script_name")
	}
	var args map[string]any
	if req.Msg.Args != nil {
		args = req.Msg.Args.AsMap()
	}
	res, err := s.be.Run(ctx, req.Msg.Name, args, principalID(ctx))
	if err != nil {
		return nil, ToConnect(ctx, err, "run_failed")
	}
	return connect.NewResponse(&v1.RunScriptResponse{RunId: res.RunID, Result: res.Result}), nil
}

func (s *ScriptService) Cancel(ctx context.Context, req *connect.Request[v1.CancelScriptRequest]) (*connect.Response[v1.CancelScriptResponse], error) {
	if err := s.be.Cancel(ctx, req.Msg.RunId); err != nil {
		return nil, ToConnect(ctx, err, "cancel_failed")
	}
	return connect.NewResponse(&v1.CancelScriptResponse{}), nil
}

func (s *ScriptService) Eval(ctx context.Context, req *connect.Request[v1.EvalScriptRequest]) (*connect.Response[v1.EvalScriptResponse], error) {
	if req.Msg.Expr == "" {
		return nil, ToConnect(ctx, ErrValidationFailed, "empty_expr")
	}
	v, stdout, err := s.be.Eval(ctx, req.Msg.Expr, principalID(ctx))
	if err != nil {
		return nil, ToConnect(ctx, err, "eval_failed")
	}
	return connect.NewResponse(&v1.EvalScriptResponse{Result: v, Stdout: stdout}), nil
}

func (s *ScriptService) RunTests(ctx context.Context, req *connect.Request[v1.RunStarlarkTestsRequest], stream *connect.ServerStream[v1.RunStarlarkTestsResponse]) error {
	// Real impl in Task 17.
	return ToConnect(ctx, ErrNotImplemented, "runtests_unimplemented")
}
```

- [ ] **Step 7: Run — pass**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 8: Commit**

```bash
git add proto/gohome/v1alpha1/script.proto gen/gohome/v1alpha1/ internal/api/deps.go internal/api/service_script.go internal/api/service_script_test.go
git commit -m "feat(c7): ScriptService (List/Run/Cancel/Eval; RunTests stub)"
```

---

## Task 16: `streaming.go` — heartbeat ticker and bounded fan-out

Shared helpers used by the four streaming RPCs in Task 17.

**Files:**
- Create: `internal/api/streaming.go`
- Create: `internal/api/streaming_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/api/streaming_test.go`:

```go
package api_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fynn-labs/gohome/internal/api"
)

func TestHeartbeatTicker_FiresOnIdle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tk := api.NewHeartbeatTicker(ctx, 50*time.Millisecond)
	defer tk.Stop()

	select {
	case <-tk.C():
		// ok
	case <-time.After(time.Second):
		t.Fatal("no heartbeat fired in time")
	}
}

func TestHeartbeatTicker_ResetOnSent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tk := api.NewHeartbeatTicker(ctx, 100*time.Millisecond)
	defer tk.Stop()

	// Reset just before the timer would have fired.
	time.Sleep(80 * time.Millisecond)
	tk.NotePayloadSent()

	select {
	case <-tk.C():
		// Should NOT fire for at least another ~80ms.
		// Failure mode: it fires immediately because we didn't reset.
		t.Fatal("heartbeat fired despite recent payload")
	case <-time.After(60 * time.Millisecond):
		// pass
	}
}

func TestBoundedFanOut_DropsOnOverflow(t *testing.T) {
	in := make(chan int, 1)
	out, done := api.BoundedFanOut(context.Background(), in, 4)

	// Push more than capacity without reading.
	for i := 0; i < 100; i++ {
		select {
		case in <- i:
		default:
			// in is full; that's fine — we want to overflow.
		}
		time.Sleep(time.Microsecond)
	}
	close(in)

	overflowed := false
	for {
		select {
		case <-out:
		case err := <-done:
			if errors.Is(err, api.ErrSubscriptionOverflow) {
				overflowed = true
			}
			if !overflowed {
				t.Fatalf("expected overflow, done err = %v", err)
			}
			return
		case <-time.After(2 * time.Second):
			t.Fatal("timed out")
		}
	}
}
```

- [ ] **Step 2: Run — fail**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 3: Write `streaming.go`**

```go
package api

import (
	"context"
	"sync"
	"time"
)

// HeartbeatTicker fires every interval of silence. Call NotePayloadSent
// each time a real payload is sent to the wire to reset the timer.
type HeartbeatTicker struct {
	interval time.Duration
	c        chan time.Time
	resetCh  chan struct{}
	done     chan struct{}
	once     sync.Once
}

func NewHeartbeatTicker(ctx context.Context, interval time.Duration) *HeartbeatTicker {
	t := &HeartbeatTicker{
		interval: interval,
		c:        make(chan time.Time, 1),
		resetCh:  make(chan struct{}, 16),
		done:     make(chan struct{}),
	}
	go t.run(ctx)
	return t
}

func (t *HeartbeatTicker) run(ctx context.Context) {
	timer := time.NewTimer(t.interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.done:
			return
		case <-t.resetCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(t.interval)
		case now := <-timer.C:
			select {
			case t.c <- now:
			default:
			}
			timer.Reset(t.interval)
		}
	}
}

func (t *HeartbeatTicker) C() <-chan time.Time { return t.c }

func (t *HeartbeatTicker) NotePayloadSent() {
	select {
	case t.resetCh <- struct{}{}:
	default:
	}
}

func (t *HeartbeatTicker) Stop() {
	t.once.Do(func() { close(t.done) })
}

// BoundedFanOut copies items from in to out. If out cannot keep up and the
// internal buffer (capacity bufSize) fills, it sends ErrSubscriptionOverflow
// on the returned done channel and closes out. Caller must drain out until it
// is closed.
func BoundedFanOut[T any](ctx context.Context, in <-chan T, bufSize int) (<-chan T, <-chan error) {
	out := make(chan T, bufSize)
	done := make(chan error, 1)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				done <- ctx.Err()
				return
			case v, ok := <-in:
				if !ok {
					done <- nil
					return
				}
				select {
				case out <- v:
				default:
					done <- ErrSubscriptionOverflow
					return
				}
			}
		}
	}()
	return out, done
}
```

- [ ] **Step 4: Run — pass**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 5: Race**

```bash
go test -race ./internal/api/... -v
```

Expected: PASS — no data races.

- [ ] **Step 6: Commit**

```bash
git add internal/api/streaming.go internal/api/streaming_test.go
git commit -m "feat(c7): streaming helpers (heartbeat, bounded fan-out)"
```

---

## Task 17: Streaming endpoints — `EntityService.Subscribe`, `EventService.Tail`, `AutomationService.Trace`, `ScriptService.RunTests`

**Files:**
- Modify: `internal/api/service_entity.go`
- Modify: `internal/api/service_event.go`
- Modify: `internal/api/service_automation.go`
- Modify: `internal/api/service_script.go`
- Create: `internal/api/streaming_endpoints_test.go`

**Pattern.** All four endpoints follow the same loop:

```go
ticker := NewHeartbeatTicker(ctx, 30*time.Second)
defer ticker.Stop()
buffered, done := BoundedFanOut(ctx, source, 10000)
for {
    select {
    case <-ctx.Done(): return ctx.Err()
    case err := <-done:
        if errors.Is(err, ErrSubscriptionOverflow) {
            return ToConnect(ctx, ErrSubscriptionOverflow, "subscription_overflow")
        }
        return nil // upstream closed cleanly
    case ev, ok := <-buffered:
        if !ok { return nil }
        if err := stream.Send(payload(ev)); err != nil { return err }
        ticker.NotePayloadSent()
    case t := <-ticker.C():
        if err := stream.Send(heartbeat(t, latestCursor())); err != nil { return err }
    }
}
```

The bufSize (10000) and heartbeat interval (30s) live in the listener `Config`; the handlers read them from a shared package-level config (see Step 6 below).

- [ ] **Step 1: Failing test for `EventService.Tail`**

Append to `internal/api/streaming_endpoints_test.go`:

```go
package api_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	eventv1 "github.com/fynn-labs/gohome/gen/gohome/event/v1"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/internal/api"
)

type liveEvents struct {
	ch chan api.Event
}

func newLiveEvents() *liveEvents { return &liveEvents{ch: make(chan api.Event, 16)} }

func (l *liveEvents) Query(_ context.Context, _ api.EventFilter, _ api.PageReq) ([]api.Event, api.Cursor, error) {
	return nil, api.Cursor{}, nil
}
func (l *liveEvents) Subscribe(_ context.Context, _ api.EventFilter) (<-chan api.Event, func(), error) {
	return l.ch, func() {}, nil
}

// fakeServerStream collects sends; Connect's ServerStream requires a real
// http.ResponseWriter, so we wrap with a tiny shim.
type tailRecorder struct {
	sent []*v1.TailEventsResponse
}

func TestEventService_Tail_StreamsThenCloses(t *testing.T) {
	api.SetStreamConfig(api.StreamConfig{HeartbeatInterval: 50 * time.Millisecond, BufSize: 4})
	defer api.SetStreamConfig(api.DefaultStreamConfig())

	src := newLiveEvents()
	s := api.NewEventService(src)

	rec := &tailRecorder{}
	stream := api.NewTestServerStream[v1.TailEventsResponse](func(m *v1.TailEventsResponse) error {
		rec.sent = append(rec.sent, m)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		src.ch <- api.Event{Cursor: 1, Kind: "state_changed", At: time.Unix(1700, 0), Payload: &eventv1.Payload{}}
		time.Sleep(150 * time.Millisecond) // let a heartbeat fire too
		close(src.ch)
	}()

	err := s.TailWithStream(ctx, connect.NewRequest(&v1.TailEventsRequest{}), stream)
	cancel()
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("Tail: %v", err)
	}
	var sawEvent, sawHB bool
	for _, m := range rec.sent {
		if m.GetEvent() != nil {
			sawEvent = true
		}
		if m.GetHeartbeat() != nil {
			sawHB = true
		}
	}
	if !sawEvent {
		t.Error("no event sent")
	}
	if !sawHB {
		t.Error("no heartbeat sent")
	}
}
```

- [ ] **Step 2: Add `internal/api/streaming.go` test helper**

Append to `internal/api/streaming.go`:

```go
// StreamConfig is the package-level configuration for streaming RPCs. The
// daemon (Task 21) calls SetStreamConfig with values from gohome.core.pkl.
type StreamConfig struct {
	HeartbeatInterval time.Duration
	BufSize           int
}

func DefaultStreamConfig() StreamConfig {
	return StreamConfig{HeartbeatInterval: 30 * time.Second, BufSize: 10000}
}

var (
	streamCfgMu sync.RWMutex
	streamCfg   = DefaultStreamConfig()
)

func SetStreamConfig(c StreamConfig) {
	streamCfgMu.Lock()
	defer streamCfgMu.Unlock()
	streamCfg = c
}

func currentStreamConfig() StreamConfig {
	streamCfgMu.RLock()
	defer streamCfgMu.RUnlock()
	return streamCfg
}
```

- [ ] **Step 3: Add a test-only ServerStream helper**

Create `internal/api/teststream.go` (compiled only under test build tag would be ideal; we accept it being in regular sources because it's a tiny utility used by the streaming endpoints test):

```go
package api

import (
	"net/http"

	"connectrpc.com/connect"
)

// NewTestServerStream returns a ServerStream that calls send(msg) for each
// stream.Send(msg). Used by streaming-endpoint tests to record outbound
// messages without spinning up a full Connect transport.
func NewTestServerStream[T any](send func(*T) error) *connect.ServerStream[T] {
	return connect.NewServerStream[T](&testStreamWriter{send: any(send)})
}

type testStreamWriter struct{ send any }

func (w *testStreamWriter) Header() http.Header        { return http.Header{} }
func (w *testStreamWriter) Write([]byte) (int, error)  { return 0, nil }
func (w *testStreamWriter) WriteHeader(int)            {}
```

> **Note:** `connect.NewServerStream` may have a different exact signature in the version of `connectrpc.com/connect` you pulled in. If the API differs, write an `internal/api/teststream.go` that drives the streaming handler via a Connect HTTP test server (httptest.Server + the generated client). Either approach is fine; the goal is to test that Send is called with the right messages.

- [ ] **Step 4: Implement `EventService.Tail`**

Replace the stub in `internal/api/service_event.go` with:

```go
func (s *EventService) Tail(ctx context.Context, req *connect.Request[v1.TailEventsRequest], stream *connect.ServerStream[v1.TailEventsResponse]) error {
	return s.TailWithStream(ctx, req, stream)
}

// TailWithStream is exported to make streaming endpoints unit-testable
// without going through a real Connect transport.
func (s *EventService) TailWithStream(ctx context.Context, req *connect.Request[v1.TailEventsRequest], stream *connect.ServerStream[v1.TailEventsResponse]) error {
	cfg := currentStreamConfig()
	filter := filterFromProto(req.Msg.Filter)
	src, cancel, err := s.src.Subscribe(ctx, filter)
	if err != nil {
		return ToConnect(ctx, err, "subscribe_failed")
	}
	defer cancel()

	buffered, done := BoundedFanOut(ctx, src, cfg.BufSize)
	ticker := NewHeartbeatTicker(ctx, cfg.HeartbeatInterval)
	defer ticker.Stop()

	var latest uint64
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-done:
			if errors.Is(err, ErrSubscriptionOverflow) {
				return ToConnect(ctx, ErrSubscriptionOverflow, "subscription_overflow")
			}
			return nil
		case ev, ok := <-buffered:
			if !ok {
				return nil
			}
			latest = ev.Cursor
			if err := stream.Send(&v1.TailEventsResponse{Kind: &v1.TailEventsResponse_Event{Event: eventToProto(ev)}}); err != nil {
				return err
			}
			ticker.NotePayloadSent()
		case t := <-ticker.C():
			if err := stream.Send(&v1.TailEventsResponse{Kind: &v1.TailEventsResponse_Heartbeat{Heartbeat: &v1.Heartbeat{LatestCursor: latest, ServerTime: ProtoTime(t)}}}); err != nil {
				return err
			}
		}
	}
}
```

Add the `errors` import.

- [ ] **Step 5: Implement `EntityService.Subscribe`**

Replace the stub in `internal/api/service_entity.go` with:

```go
func (s *EntityService) Subscribe(ctx context.Context, req *connect.Request[v1.SubscribeEntitiesRequest], stream *connect.ServerStream[v1.SubscribeEntitiesResponse]) error {
	if s.streamSource == nil {
		return ToConnect(ctx, ErrNotImplemented, "subscribe_unimplemented")
	}
	cfg := currentStreamConfig()
	src, cancel, err := s.streamSource.Subscribe(ctx, selectorFromProto(req.Msg.Selector), req.Msg.FromCursor)
	if err != nil {
		return ToConnect(ctx, err, "subscribe_failed")
	}
	defer cancel()

	buffered, done := BoundedFanOut(ctx, src, cfg.BufSize)
	ticker := NewHeartbeatTicker(ctx, cfg.HeartbeatInterval)
	defer ticker.Stop()

	var latest uint64
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-done:
			if errors.Is(err, ErrSubscriptionOverflow) {
				return ToConnect(ctx, ErrSubscriptionOverflow, "subscription_overflow")
			}
			return nil
		case ch, ok := <-buffered:
			if !ok {
				return nil
			}
			latest = ch.Cursor
			if err := stream.Send(&v1.SubscribeEntitiesResponse{Kind: &v1.SubscribeEntitiesResponse_Change{Change: &v1.EntityChange{
				EntityId: ch.EntityID, Cursor: ch.Cursor, At: ProtoTime(unixMsToTime(ch.AtUnixMs)), Entity: entityToProto(ch.Entity),
			}}}); err != nil {
				return err
			}
			ticker.NotePayloadSent()
		case t := <-ticker.C():
			if err := stream.Send(&v1.SubscribeEntitiesResponse{Kind: &v1.SubscribeEntitiesResponse_Heartbeat{Heartbeat: &v1.Heartbeat{LatestCursor: latest, ServerTime: ProtoTime(t)}}}); err != nil {
				return err
			}
		}
	}
}
```

- [ ] **Step 6: Implement `AutomationService.Trace`**

Replace the stub in `internal/api/service_automation.go`:

```go
func (s *AutomationService) Trace(ctx context.Context, req *connect.Request[v1.TraceAutomationRequest], stream *connect.ServerStream[v1.TraceAutomationResponse]) error {
	cfg := currentStreamConfig()
	src, cancel, err := s.be.Trace(ctx, req.Msg.AutomationId, req.Msg.RunId, req.Msg.FromCursor)
	if err != nil {
		return ToConnect(ctx, err, "trace_failed")
	}
	defer cancel()

	buffered, done := BoundedFanOut(ctx, src, cfg.BufSize)
	ticker := NewHeartbeatTicker(ctx, cfg.HeartbeatInterval)
	defer ticker.Stop()

	var latest uint64
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-done:
			if errors.Is(err, ErrSubscriptionOverflow) {
				return ToConnect(ctx, ErrSubscriptionOverflow, "subscription_overflow")
			}
			return nil
		case te, ok := <-buffered:
			if !ok {
				return nil
			}
			latest = te.Cursor
			if err := stream.Send(&v1.TraceAutomationResponse{Kind: &v1.TraceAutomationResponse_Event{Event: &v1.TraceEvent{
				Cursor: te.Cursor, At: ProtoTime(te.At), AutomationId: te.AutomationID, RunId: te.RunID, Kind: te.Kind, Detail: te.Detail, Metadata: te.Metadata,
			}}}); err != nil {
				return err
			}
			ticker.NotePayloadSent()
		case t := <-ticker.C():
			if err := stream.Send(&v1.TraceAutomationResponse{Kind: &v1.TraceAutomationResponse_Heartbeat{Heartbeat: &v1.Heartbeat{LatestCursor: latest, ServerTime: ProtoTime(t)}}}); err != nil {
				return err
			}
		}
	}
}
```

- [ ] **Step 7: Implement `ScriptService.RunTests`**

Replace the stub in `internal/api/service_script.go`:

```go
func (s *ScriptService) RunTests(ctx context.Context, req *connect.Request[v1.RunStarlarkTestsRequest], stream *connect.ServerStream[v1.RunStarlarkTestsResponse]) error {
	if req.Msg.Path == "" {
		return ToConnect(ctx, ErrValidationFailed, "missing_path")
	}
	cfg := currentStreamConfig()
	src, cancel, err := s.be.RunTests(ctx, req.Msg.Path)
	if err != nil {
		return ToConnect(ctx, err, "runtests_failed")
	}
	defer cancel()

	buffered, done := BoundedFanOut(ctx, src, cfg.BufSize)
	ticker := NewHeartbeatTicker(ctx, cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-done:
			if errors.Is(err, ErrSubscriptionOverflow) {
				return ToConnect(ctx, ErrSubscriptionOverflow, "subscription_overflow")
			}
			return nil
		case te, ok := <-buffered:
			if !ok {
				return nil
			}
			if err := stream.Send(&v1.RunStarlarkTestsResponse{Kind: &v1.RunStarlarkTestsResponse_Event{Event: &v1.StarlarkTestEvent{
				Name: te.Name, Outcome: te.Outcome, Detail: te.Detail, At: ProtoTime(te.At),
			}}}); err != nil {
				return err
			}
			ticker.NotePayloadSent()
		case t := <-ticker.C():
			if err := stream.Send(&v1.RunStarlarkTestsResponse{Kind: &v1.RunStarlarkTestsResponse_Heartbeat{Heartbeat: &v1.Heartbeat{ServerTime: ProtoTime(t)}}}); err != nil {
				return err
			}
		}
	}
}
```

- [ ] **Step 8: Run tests — pass**

```bash
go test ./internal/api/... -v
go test -race ./internal/api/... -v
```

- [ ] **Step 9: Commit**

```bash
git add internal/api/streaming.go internal/api/teststream.go internal/api/streaming_endpoints_test.go internal/api/service_event.go internal/api/service_entity.go internal/api/service_automation.go internal/api/service_script.go
git commit -m "feat(c7): streaming endpoints (Tail, Subscribe, Trace, RunTests)"
```

---

## Task 18: Webhook handler + `WebhookTrigger` matcher activation

**Files:**
- Create: `internal/api/webhook.go`
- Create: `internal/api/webhook_test.go`
- Modify: `internal/automation/trigger/event.go` (subscribe to `WebhookReceived`)

- [ ] **Step 1: Failing tests**

Create `internal/api/webhook_test.go`:

```go
package api_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fynn-labs/gohome/internal/api"
)

type fakeWebhookRouter struct {
	secrets   map[string]string  // slug → secret
	maxBytes  int64
	appended  []api.AppendedWebhook
}
type fakeAppender struct {
	app []api.AppendedWebhook
}

func (f *fakeWebhookRouter) SecretFor(slug string) (string, bool) {
	s, ok := f.secrets[slug]
	return s, ok
}
func (f *fakeWebhookRouter) MaxBodyBytes() int64 { return f.maxBytes }
func (f *fakeAppender) AppendWebhook(_ context.Context, w api.AppendedWebhook) error {
	f.app = append(f.app, w)
	return nil
}

func sign(secret, body string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(body))
	return "v1=" + hex.EncodeToString(h.Sum(nil))
}

func TestWebhook_Accepts_ValidSignature(t *testing.T) {
	r := &fakeWebhookRouter{secrets: map[string]string{"foo": "shh"}, maxBytes: 1024}
	app := &fakeAppender{}
	h := api.NewWebhookHandler(r, app, nil)

	body := `{"x":1}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/foo", bytes.NewBufferString(body))
	req.Header.Set("X-GoHome-Signature", sign("shh", body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Errorf("status = %d, body = %q", rr.Code, rr.Body.String())
	}
	if len(app.app) != 1 || app.app[0].Slug != "foo" {
		t.Errorf("appended = %+v", app.app)
	}
}

func TestWebhook_Rejects_BadSignature(t *testing.T) {
	r := &fakeWebhookRouter{secrets: map[string]string{"foo": "shh"}, maxBytes: 1024}
	app := &fakeAppender{}
	h := api.NewWebhookHandler(r, app, nil)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/foo", bytes.NewBufferString("body"))
	req.Header.Set("X-GoHome-Signature", "v1=deadbeef")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d", rr.Code)
	}
	if len(app.app) != 0 {
		t.Error("appended on bad sig")
	}
}

func TestWebhook_Rejects_UnknownSlug(t *testing.T) {
	r := &fakeWebhookRouter{secrets: map[string]string{}, maxBytes: 1024}
	app := &fakeAppender{}
	h := api.NewWebhookHandler(r, app, nil)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/nope", bytes.NewBufferString(""))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d", rr.Code)
	}
}

func TestWebhook_Rejects_BodyTooLarge(t *testing.T) {
	r := &fakeWebhookRouter{secrets: map[string]string{"foo": "shh"}, maxBytes: 4}
	app := &fakeAppender{}
	h := api.NewWebhookHandler(r, app, nil)

	body := "more than four bytes"
	req := httptest.NewRequest(http.MethodPost, "/webhooks/foo", bytes.NewBufferString(body))
	req.Header.Set("X-GoHome-Signature", sign("shh", body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge && rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rr.Code)
	}
}

func TestWebhook_RejectsNonPost(t *testing.T) {
	r := &fakeWebhookRouter{secrets: map[string]string{"foo": "shh"}, maxBytes: 1024}
	app := &fakeAppender{}
	h := api.NewWebhookHandler(r, app, nil)
	req := httptest.NewRequest(http.MethodGet, "/webhooks/foo", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run — fail**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 3: Write `webhook.go`**

```go
package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/fynn-labs/gohome/internal/observability"
)

// WebhookRouter knows which slugs are currently registered (via Pkl/automation
// engine reload) and what their HMAC secret is. Implementation lives in the
// daemon wiring (Task 21) and is updated on every ConfigApplied.
type WebhookRouter interface {
	SecretFor(slug string) (string, bool)
	MaxBodyBytes() int64
}

// WebhookAppender persists the inbound webhook as a WebhookReceived event so
// the C6 WebhookTrigger matcher can pick it up.
type WebhookAppender interface {
	AppendWebhook(ctx context.Context, w AppendedWebhook) error
}

type AppendedWebhook struct {
	Slug     string
	Body     []byte
	Headers  map[string]string
	SourceIP string
}

// WebhookMetrics is optional; nil is fine in tests.
type WebhookMetrics interface {
	Inc(slug, result string)
}

// NewWebhookHandler returns an http.Handler mounted at /webhooks/.
func NewWebhookHandler(router WebhookRouter, app WebhookAppender, m WebhookMetrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			incWebhook(m, "", "method_not_allowed")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		slug := strings.TrimPrefix(r.URL.Path, "/webhooks/")
		if slug == "" || strings.Contains(slug, "/") {
			incWebhook(m, slug, "bad_slug")
			http.Error(w, "bad slug", http.StatusBadRequest)
			return
		}
		secret, ok := router.SecretFor(slug)
		if !ok {
			incWebhook(m, slug, "unknown_slug")
			http.Error(w, "unknown slug", http.StatusNotFound)
			return
		}

		max := router.MaxBodyBytes()
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, max))
		if err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				incWebhook(m, slug, "too_large")
				http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
				return
			}
			incWebhook(m, slug, "bad_body")
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}

		sig := r.Header.Get("X-GoHome-Signature")
		if !verifySignature(secret, body, sig) {
			incWebhook(m, slug, "bad_signature")
			http.Error(w, "bad signature", http.StatusUnauthorized)
			return
		}

		ctx := r.Context()
		id, _ := observability.RequestIDFromContext(ctx)
		_ = id

		headers := map[string]string{
			"content-type": r.Header.Get("Content-Type"),
			"user-agent":   r.Header.Get("User-Agent"),
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			headers["x-forwarded-for"] = xff
		}

		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if err := app.AppendWebhook(ctx, AppendedWebhook{
			Slug:     slug,
			Body:     body,
			Headers:  headers,
			SourceIP: ip,
		}); err != nil {
			incWebhook(m, slug, "append_failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		incWebhook(m, slug, "accepted")
		w.WriteHeader(http.StatusAccepted)
	})
}

func verifySignature(secret string, body []byte, header string) bool {
	const prefix = "v1="
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	want := header[len(prefix):]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	got := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(got), []byte(want))
}

func incWebhook(m WebhookMetrics, slug, result string) {
	if m != nil {
		m.Inc(slug, result)
	}
}
```

- [ ] **Step 4: Run — pass**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 5: Activate `WebhookTrigger` matcher in C6**

Read `internal/automation/trigger/event.go`. Locate where the engine subscribes to event payloads. The existing matcher iterates known event payload kinds; ensure `WebhookReceived` is included in the dispatch. If the matcher is registry-based (registered by engine setup), wire the new payload type at the registration site.

Concretely, find where the engine subscribes via `eventstore.Subscribe(...)` — typically `internal/automation/engine.go`. Add `WebhookReceived` to the kind filter so `WebhookTrigger.slug` matchers see those events. The expected change is one line in the kind list and the matcher's `Match` function reading `payload.GetWebhookReceived().GetSlug()`.

Pseudocode of the modification (the actual API name comes from C6's matcher):

```go
// in internal/automation/trigger/event.go, WebhookTriggerMatcher.Match:
case *eventv1.Payload_WebhookReceived:
    if p.WebhookReceived.GetSlug() == m.slug {
        return true
    }
```

Run the full automation test suite:

```bash
go test ./internal/automation/... -v
```

Expected: PASS — existing tests unchanged plus a new `WebhookTriggerMatcher` test if not already present.

- [ ] **Step 6: Commit**

```bash
git add internal/api/webhook.go internal/api/webhook_test.go internal/automation/trigger/event.go
git commit -m "feat(c7): webhook receiver + WebhookTrigger activation"
```

---

## Task 19: UNIMPLEMENTED stubs — `SceneService`, `DashboardService`, `AuthService`

**Files:**
- Create: `proto/gohome/v1alpha1/scene.proto`
- Create: `proto/gohome/v1alpha1/dashboard.proto`
- Create: `proto/gohome/v1alpha1/auth.proto`
- Create: `internal/api/service_unimplemented.go`
- Create: `internal/api/service_unimplemented_test.go`

- [ ] **Step 1: Write `scene.proto`**

```protobuf
syntax = "proto3";
package gohome.v1alpha1;
import "gohome/v1alpha1/common.proto";

service SceneService {
  rpc List   (ListScenesRequest)  returns (ListScenesResponse);
  rpc Apply  (ApplySceneRequest)  returns (ApplySceneResponse);
  rpc Preview(PreviewSceneRequest) returns (PreviewSceneResponse);
}

message Scene { string id = 1; string display_name = 2; }
message ListScenesRequest  { PageRequest page = 1; }
message ListScenesResponse { repeated Scene scenes = 1; PageResponse page = 2; }
message ApplySceneRequest  { string id = 1; }
message ApplySceneResponse { string correlation_id = 1; }
message PreviewSceneRequest  { string id = 1; }
message PreviewSceneResponse { repeated string lines = 1; }
```

- [ ] **Step 2: Write `dashboard.proto`**

```protobuf
syntax = "proto3";
package gohome.v1alpha1;
import "gohome/v1alpha1/common.proto";
import "google/protobuf/struct.proto";

service DashboardService {
  rpc List      (ListDashboardsRequest)   returns (ListDashboardsResponse);
  rpc Get       (GetDashboardRequest)     returns (GetDashboardResponse);
  rpc SaveLayout(SaveDashboardLayoutRequest) returns (SaveDashboardLayoutResponse);
}

message Dashboard {
  string slug         = 1;
  string display_name = 2;
}

message ListDashboardsRequest  { PageRequest page = 1; }
message ListDashboardsResponse { repeated Dashboard dashboards = 1; PageResponse page = 2; }

message GetDashboardRequest  { string slug = 1; }
message GetDashboardResponse { google.protobuf.Struct ast = 1; }

message SaveDashboardLayoutRequest  { string slug = 1; google.protobuf.Struct ast = 2; }
message SaveDashboardLayoutResponse { string correlation_id = 1; }
```

- [ ] **Step 3: Write `auth.proto`**

```protobuf
syntax = "proto3";
package gohome.v1alpha1;
import "gohome/v1alpha1/common.proto";

service AuthService {
  rpc Login                (LoginRequest)                returns (LoginResponse);
  rpc Logout               (LogoutRequest)               returns (LogoutResponse);
  rpc CurrentUser          (CurrentUserRequest)          returns (CurrentUserResponse);
  rpc CreateToken          (CreateTokenRequest)          returns (CreateTokenResponse);
  rpc RevokeToken          (RevokeTokenRequest)          returns (RevokeTokenResponse);
  rpc ListUsers            (ListUsersRequest)            returns (ListUsersResponse);
  rpc RegisterPasskey      (RegisterPasskeyRequest)      returns (RegisterPasskeyResponse);
  rpc StartWebAuthnChallenge(StartWebAuthnChallengeRequest) returns (StartWebAuthnChallengeResponse);
}

message User {
  string slug         = 1;
  string display_name = 2;
  bool   active       = 3;
  repeated string roles = 4;
}

message LoginRequest  { string username = 1; string password = 2; }
message LoginResponse { string session_token = 1; }
message LogoutRequest  {}
message LogoutResponse {}
message CurrentUserRequest  {}
message CurrentUserResponse { User user = 1; }
message CreateTokenRequest  { string display_name = 1; repeated string scopes = 2; }
message CreateTokenResponse { string token = 1; string token_id = 2; }
message RevokeTokenRequest  { string token_id = 1; }
message RevokeTokenResponse {}
message ListUsersRequest    { PageRequest page = 1; }
message ListUsersResponse   { repeated User users = 1; PageResponse page = 2; }
message RegisterPasskeyRequest  { bytes public_key_credential = 1; string user_slug = 2; }
message RegisterPasskeyResponse { string credential_id = 1; }
message StartWebAuthnChallengeRequest  { string user_slug = 1; }
message StartWebAuthnChallengeResponse { bytes challenge = 1; }
```

- [ ] **Step 4: Regenerate**

```bash
task proto
```

- [ ] **Step 5: Failing tests**

Create `internal/api/service_unimplemented_test.go`:

```go
package api_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/internal/api"
)

func TestSceneService_AllUnimplemented(t *testing.T) {
	s := api.NewSceneService()
	_, err := s.Apply(context.Background(), connect.NewRequest(&v1.ApplySceneRequest{Id: "x"}))
	assertUnimplemented(t, err)
}

func TestDashboardService_AllUnimplemented(t *testing.T) {
	d := api.NewDashboardService()
	_, err := d.Get(context.Background(), connect.NewRequest(&v1.GetDashboardRequest{Slug: "main"}))
	assertUnimplemented(t, err)
}

func TestAuthService_AllUnimplemented(t *testing.T) {
	a := api.NewAuthService()
	_, err := a.Login(context.Background(), connect.NewRequest(&v1.LoginRequest{Username: "u", Password: "p"}))
	assertUnimplemented(t, err)
}

func assertUnimplemented(t *testing.T, err error) {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeUnimplemented {
		t.Fatalf("err = %v, want Unimplemented", err)
	}
}
```

- [ ] **Step 6: Run — fail**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 7: Write `service_unimplemented.go`**

```go
package api

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/gen/gohome/v1alpha1/v1alpha1connect"
)

func unimplemented(ctx context.Context, reason string) error {
	return ToConnect(ctx, ErrNotImplemented, reason)
}

// SceneService stub.
type SceneService struct{}

func NewSceneService() *SceneService { return &SceneService{} }

var _ v1alpha1connect.SceneServiceHandler = (*SceneService)(nil)

func (*SceneService) List(ctx context.Context, _ *connect.Request[v1.ListScenesRequest]) (*connect.Response[v1.ListScenesResponse], error) {
	return nil, unimplemented(ctx, "scene_unimplemented")
}
func (*SceneService) Apply(ctx context.Context, _ *connect.Request[v1.ApplySceneRequest]) (*connect.Response[v1.ApplySceneResponse], error) {
	return nil, unimplemented(ctx, "scene_unimplemented")
}
func (*SceneService) Preview(ctx context.Context, _ *connect.Request[v1.PreviewSceneRequest]) (*connect.Response[v1.PreviewSceneResponse], error) {
	return nil, unimplemented(ctx, "scene_unimplemented")
}

// DashboardService stub.
type DashboardService struct{}

func NewDashboardService() *DashboardService { return &DashboardService{} }

var _ v1alpha1connect.DashboardServiceHandler = (*DashboardService)(nil)

func (*DashboardService) List(ctx context.Context, _ *connect.Request[v1.ListDashboardsRequest]) (*connect.Response[v1.ListDashboardsResponse], error) {
	return nil, unimplemented(ctx, "dashboard_unimplemented")
}
func (*DashboardService) Get(ctx context.Context, _ *connect.Request[v1.GetDashboardRequest]) (*connect.Response[v1.GetDashboardResponse], error) {
	return nil, unimplemented(ctx, "dashboard_unimplemented")
}
func (*DashboardService) SaveLayout(ctx context.Context, _ *connect.Request[v1.SaveDashboardLayoutRequest]) (*connect.Response[v1.SaveDashboardLayoutResponse], error) {
	return nil, unimplemented(ctx, "dashboard_unimplemented")
}

// AuthService stub.
type AuthService struct{}

func NewAuthService() *AuthService { return &AuthService{} }

var _ v1alpha1connect.AuthServiceHandler = (*AuthService)(nil)

func (*AuthService) Login(ctx context.Context, _ *connect.Request[v1.LoginRequest]) (*connect.Response[v1.LoginResponse], error) {
	return nil, unimplemented(ctx, "auth_unimplemented")
}
func (*AuthService) Logout(ctx context.Context, _ *connect.Request[v1.LogoutRequest]) (*connect.Response[v1.LogoutResponse], error) {
	return nil, unimplemented(ctx, "auth_unimplemented")
}
func (*AuthService) CurrentUser(ctx context.Context, _ *connect.Request[v1.CurrentUserRequest]) (*connect.Response[v1.CurrentUserResponse], error) {
	return nil, unimplemented(ctx, "auth_unimplemented")
}
func (*AuthService) CreateToken(ctx context.Context, _ *connect.Request[v1.CreateTokenRequest]) (*connect.Response[v1.CreateTokenResponse], error) {
	return nil, unimplemented(ctx, "auth_unimplemented")
}
func (*AuthService) RevokeToken(ctx context.Context, _ *connect.Request[v1.RevokeTokenRequest]) (*connect.Response[v1.RevokeTokenResponse], error) {
	return nil, unimplemented(ctx, "auth_unimplemented")
}
func (*AuthService) ListUsers(ctx context.Context, _ *connect.Request[v1.ListUsersRequest]) (*connect.Response[v1.ListUsersResponse], error) {
	return nil, unimplemented(ctx, "auth_unimplemented")
}
func (*AuthService) RegisterPasskey(ctx context.Context, _ *connect.Request[v1.RegisterPasskeyRequest]) (*connect.Response[v1.RegisterPasskeyResponse], error) {
	return nil, unimplemented(ctx, "auth_unimplemented")
}
func (*AuthService) StartWebAuthnChallenge(ctx context.Context, _ *connect.Request[v1.StartWebAuthnChallengeRequest]) (*connect.Response[v1.StartWebAuthnChallengeResponse], error) {
	return nil, unimplemented(ctx, "auth_unimplemented")
}
```

- [ ] **Step 8: Run — pass**

```bash
go test ./internal/api/... -v
```

- [ ] **Step 9: Commit**

```bash
git add proto/gohome/v1alpha1/scene.proto proto/gohome/v1alpha1/dashboard.proto proto/gohome/v1alpha1/auth.proto gen/gohome/v1alpha1/ internal/api/service_unimplemented.go internal/api/service_unimplemented_test.go
git commit -m "feat(c7): UNIMPLEMENTED stubs (Scene, Dashboard, Auth)"
```

---

## Task 20: Pkl `Listener` config additions

**Files:**
- Modify: `internal/config/pkl/gohome/<core module>.pkl` (whichever holds the existing `core` config; check `internal/config/pkl/gohome/` for the file that defines daemon-level paths/data-dirs and add the Listener block there. If no core module exists yet, add `core.pkl`.)
- Modify: `internal/config/manager.go` or equivalent that surfaces config to the daemon — add a `Listener` field on the public config type so the daemon can read it
- Create: `internal/config/testdata/listener-defaults/main.pkl` (smoke fixture)
- Modify: `internal/config/manager_test.go` (or evaluator integration test) to assert defaults

- [ ] **Step 1: Add `Listener` Pkl class**

In the appropriate Pkl module (likely `internal/config/pkl/gohome/config.pkl` since C4 puts most root config there), add:

```pkl
class Listener {
  uds: UDSListener = new UDSListener {}
  tcp: TCPListener = new TCPListener {}
  webhooks: WebhookConfig = new WebhookConfig {}
  stream_heartbeat_interval: Duration = 30.s
}

class UDSListener {
  path: String = "@data/gohomed.sock"
  mode: UInt   = 0o600
}

class TCPListener {
  bind: String = "127.0.0.1:8080"
  tls: TLSConfig? = null
}

class TLSConfig {
  cert_file: String
  key_file:  String
}

class WebhookConfig {
  max_body_bytes: UInt        = 1048576
  trusted_proxies: List<String> = List()
}
```

And expose it on the root module:

```pkl
listener: Listener = new Listener {}
```

- [ ] **Step 2: Surface in Go config types**

In `internal/config` (likely `manager.go` or wherever the public config struct is exposed to `daemon.go`), add fields mirroring the Pkl shape. Example (the exact public struct name depends on what C4 shipped):

```go
type ListenerConfig struct {
	UDS                     UDSListenerConfig
	TCP                     TCPListenerConfig
	Webhooks                WebhookListenerConfig
	StreamHeartbeatInterval time.Duration
}

type UDSListenerConfig struct {
	Path string
	Mode os.FileMode
}

type TCPListenerConfig struct {
	Bind string
	TLS  *TLSListenerConfig
}

type TLSListenerConfig struct {
	CertFile, KeyFile string
}

type WebhookListenerConfig struct {
	MaxBodyBytes   int64
	TrustedProxies []string
}
```

The Pkl evaluator (C4's `Evaluate(...)`) returns JSON that decodes into this. Update the corresponding decoder.

- [ ] **Step 3: Smoke fixture**

Create `internal/config/testdata/listener-defaults/main.pkl`:

```pkl
amends "gohome:config"

// Use defaults for everything.
```

- [ ] **Step 4: Failing test**

In `internal/config/manager_test.go` or an evaluator integration test, add:

```go
func TestEvaluate_ListenerDefaults(t *testing.T) {
	snap, err := evaluateFixture(t, "testdata/listener-defaults")
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if snap.Listener.TCP.Bind != "127.0.0.1:8080" {
		t.Errorf("tcp bind = %q", snap.Listener.TCP.Bind)
	}
	if snap.Listener.UDS.Mode != 0o600 {
		t.Errorf("uds mode = %v", snap.Listener.UDS.Mode)
	}
	if snap.Listener.StreamHeartbeatInterval != 30*time.Second {
		t.Errorf("hb = %v", snap.Listener.StreamHeartbeatInterval)
	}
}
```

(`evaluateFixture` is the existing test helper in `internal/config`. Use the same pattern other fixture tests use; if no helper exists, write one inline.)

- [ ] **Step 5: Run — fail, then pass after Step 1-2 land**

```bash
go test ./internal/config/... -v -run ListenerDefaults
```

- [ ] **Step 6: Commit**

```bash
git add internal/config/pkl/gohome/ internal/config/ internal/config/testdata/listener-defaults/
git commit -m "feat(c7): Listener block in Pkl core config"
```

---

## Task 21: Daemon wiring — construct listener, wire all services

This is the keystone task. It assembles the pieces from Tasks 3-19 inside `internal/daemon/daemon.go`, sets the package-level stream config, and replaces the legacy `serveSocket` call.

**Files:**
- Create: `internal/api/listener/routes.go` (helper that builds `ConnectRoutes` for a given service set)
- Create: `internal/daemon/api_adapters.go` (engine → api-deps adapters)
- Modify: `internal/daemon/daemon.go`
- Modify: `internal/daemon/daemon_test.go`

- [ ] **Step 1: Write `routes.go`**

Create `internal/api/listener/routes.go`:

```go
package listener

import (
	"net/http"

	"connectrpc.com/connect"
	"github.com/fynn-labs/gohome/gen/gohome/v1alpha1/v1alpha1connect"
)

// Services is the set of handler implementations the listener needs.
// Daemon constructs this with concrete impls from internal/api.
type Services struct {
	System     v1alpha1connect.SystemServiceHandler
	Area       v1alpha1connect.AreaServiceHandler
	Zone       v1alpha1connect.ZoneServiceHandler
	Device     v1alpha1connect.DeviceServiceHandler
	Entity     v1alpha1connect.EntityServiceHandler
	Driver     v1alpha1connect.DriverServiceHandler
	Event      v1alpha1connect.EventServiceHandler
	Config     v1alpha1connect.ConfigServiceHandler
	Automation v1alpha1connect.AutomationServiceHandler
	Script     v1alpha1connect.ScriptServiceHandler
	Scene      v1alpha1connect.SceneServiceHandler
	Dashboard  v1alpha1connect.DashboardServiceHandler
	Auth       v1alpha1connect.AuthServiceHandler
}

// BuildRoutes returns the (path, handler) pairs to mount on the listener mux.
// All handlers are wrapped with the same interceptor stack.
func BuildRoutes(svc Services, interceptors ...connect.Interceptor) []Route {
	opts := connect.WithInterceptors(interceptors...)
	add := func(routes *[]Route, path string, h http.Handler) {
		*routes = append(*routes, Route{Path: path, Handler: h})
	}
	var routes []Route
	add(&routes, v1alpha1connect.NewSystemServiceHandler(svc.System, opts))
	add(&routes, v1alpha1connect.NewAreaServiceHandler(svc.Area, opts))
	add(&routes, v1alpha1connect.NewZoneServiceHandler(svc.Zone, opts))
	add(&routes, v1alpha1connect.NewDeviceServiceHandler(svc.Device, opts))
	add(&routes, v1alpha1connect.NewEntityServiceHandler(svc.Entity, opts))
	add(&routes, v1alpha1connect.NewDriverServiceHandler(svc.Driver, opts))
	add(&routes, v1alpha1connect.NewEventServiceHandler(svc.Event, opts))
	add(&routes, v1alpha1connect.NewConfigServiceHandler(svc.Config, opts))
	add(&routes, v1alpha1connect.NewAutomationServiceHandler(svc.Automation, opts))
	add(&routes, v1alpha1connect.NewScriptServiceHandler(svc.Script, opts))
	add(&routes, v1alpha1connect.NewSceneServiceHandler(svc.Scene, opts))
	add(&routes, v1alpha1connect.NewDashboardServiceHandler(svc.Dashboard, opts))
	add(&routes, v1alpha1connect.NewAuthServiceHandler(svc.Auth, opts))
	return routes
}
```

> **Note:** the Connect-Go generated `NewXServiceHandler` returns `(string, http.Handler)`. The `add` helper above is sketched assuming that; adjust the call style to match what the generator produces in your `gen/` tree (the canonical form is `path, h := v1alpha1connect.NewSystemServiceHandler(svc.System, opts); routes = append(routes, Route{path, h})`).

- [ ] **Step 2: Write engine adapters**

Create `internal/daemon/api_adapters.go`. This is the bridge between the existing C1-C6 engine packages and the `internal/api` interface set. Each adapter is a thin struct with engine-typed fields whose methods produce the `api.Xxx` shapes.

Sketch (one per dep — copy the pattern):

```go
package daemon

import (
	"context"
	"time"

	"github.com/fynn-labs/gohome/internal/api"
	"github.com/fynn-labs/gohome/internal/automation"
	"github.com/fynn-labs/gohome/internal/carport"
	"github.com/fynn-labs/gohome/internal/config"
	"github.com/fynn-labs/gohome/internal/eventstore"
	"github.com/fynn-labs/gohome/internal/registry"
	"github.com/fynn-labs/gohome/internal/script"
	"github.com/fynn-labs/gohome/internal/state"
)

type entityReaderAdapter struct {
	reg   *registry.Registry
	cache *state.Cache
}

func (a entityReaderAdapter) ListEntities(_ context.Context, sel api.EntitySelector, _ api.PageReq) ([]api.Entity, api.Cursor, error) {
	// Drive registry list, mapping to api.Entity. Apply the selector. Return a cursor that's a registry-position pointer (or zero if the list is small).
	// Concrete shape depends on registry.Registry's existing list method. Implement it here, do NOT add to internal/registry.
	// ...
	return nil, api.Cursor{}, nil
}

func (a entityReaderAdapter) GetEntity(_ context.Context, id string) (api.Entity, error) {
	rec, ok := a.reg.GetEntity(id)
	if !ok {
		return api.Entity{}, api.ErrEntityNotFound
	}
	st, _ := a.cache.Get(id)
	return api.Entity{
		ID: rec.ID, Type: rec.Type, DeviceID: rec.DeviceID, AreaID: rec.AreaID,
		ZoneID: rec.ZoneID, FriendlyName: rec.FriendlyName,
		State: st, Capabilities: rec.Capabilities,
	}, nil
}

// capabilityCallerAdapter routes Call(...) through the carport supervisor.
type capabilityCallerAdapter struct {
	sup *carport.Supervisor
}

func (a capabilityCallerAdapter) Call(ctx context.Context, entityID, capability string, params map[string]any) (string, error) {
	// Translate api.ErrDriverUnavailable etc. from carport errors via classifyCarportErr below.
	cid, err := a.sup.IssueCommand(ctx, entityID, capability, params)
	if err != nil {
		return "", classifyCarportErr(err)
	}
	return cid, nil
}

// EventSource adapter; Subscribe wraps eventstore.Subscribe with filter.
type eventSourceAdapter struct {
	store *eventstore.Store
}

func (a eventSourceAdapter) Query(ctx context.Context, f api.EventFilter, p api.PageReq) ([]api.Event, api.Cursor, error) {
	// Build eventstore.Filter from api.EventFilter; translate p.Cursor to cursor uint64; iterate.
	// ...
	return nil, api.Cursor{}, nil
}

func (a eventSourceAdapter) Subscribe(ctx context.Context, f api.EventFilter) (<-chan api.Event, func(), error) {
	// eventstore.Subscribe gives back its own channel; rewrap items into api.Event.
	// ...
	return nil, func() {}, nil
}

// ... (deviceReaderAdapter, deviceWriterAdapter, areaReaderAdapter, zoneReaderAdapter,
// driverControlAdapter, configApplierAdapter, automationControlAdapter, scriptRunnerAdapter,
// systemBackendAdapter, webhookRouterAdapter, webhookAppenderAdapter)

// classifyCarportErr maps carport-side errors to api sentinels.
func classifyCarportErr(err error) error {
	// e.g. errors.Is(err, carport.ErrInstanceDown) => api.ErrDriverUnavailable
	return err
}
```

> Each adapter is short; the shape repeats. The implementer reads each engine package's existing public API to fill in the bodies. The plan does not enumerate each engine method here because the C1-C6 packages already exist and the methods are visible — this is mechanical glue.

- [ ] **Step 3: Modify `daemon.go` — construct everything, replace `serveSocket`**

Find the existing `Run`/`Start` body in `internal/daemon/daemon.go`. Locate the `go d.serveSocket(ctx, socketPath)` line. Replace with API listener construction:

```go
// internal/daemon/daemon.go (within Run/Start, after engines are constructed)

// Stream config from Pkl.
api.SetStreamConfig(api.StreamConfig{
    HeartbeatInterval: cfg.Listener.StreamHeartbeatInterval,
    BufSize:           10000,
})

// Adapters.
sysBE := systemBackendAdapter{store: d.eventstore, healthAggregator: d.healthAggregator(), version: d.versionInfo()}
entRd := entityReaderAdapter{reg: d.registry, cache: d.stateCache}
capCall := capabilityCallerAdapter{sup: d.carport}
devRd := deviceReaderAdapter{reg: d.registry}
devWr := deviceWriterAdapter{reg: d.registry, store: d.eventstore}
areaRd := areaReaderAdapter{reg: d.registry}
zoneRd := zoneReaderAdapter{reg: d.registry}
drvCtl := driverControlAdapter{sup: d.carport, store: d.eventstore}
evtSrc := eventSourceAdapter{store: d.eventstore}
cfgAppl := configApplierAdapter{mgr: d.config, store: d.eventstore}
autoCtl := automationControlAdapter{eng: d.automation}
scriptRun := scriptRunnerAdapter{eng: d.script}
webhookRouter := webhookRouterAdapter{automation: d.automation, max: cfg.Listener.Webhooks.MaxBodyBytes}
webhookApp := webhookAppenderAdapter{store: d.eventstore}

// Service constructors.
entSvc := api.NewEntityService(entRd, capCall)
// (Once Subscribe lands, also: entSvc.SetStreamSource(entityStreamSourceAdapter{...}))

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
    Script:     api.NewScriptService(scriptRun),
    Scene:      api.NewSceneService(),
    Dashboard:  api.NewDashboardService(),
    Auth:       api.NewAuthService(),
}

// Auth seam (stub for v1).
authn := auth.Chain(auth.LocalPeerCred{}, auth.RejectAll{})
authz := auth.AllowAll{}
schemeCls := schemeClassifier{} // Defined below; reads listener.WithPeerCred-attached creds.

interceptors := []connect.Interceptor{
    listener.RecoverInterceptor(),
    listener.RequestIDInterceptor(),
    listener.SlogInterceptor(),
    listener.MetricsInterceptor(d.metrics),
    listener.AuthenticateInterceptor(authn, schemeCls),
    listener.AuthorizeInterceptor(authz, actionTable),
}

routes := listener.BuildRoutes(services, interceptors...)
webhook := api.NewWebhookHandler(webhookRouter, webhookApp, webhookMetricsAdapter{d.metrics})

l, err := listener.Build(listener.Config{
    UDSPath:     cfg.Listener.UDS.Path,
    UDSMode:     cfg.Listener.UDS.Mode,
    TCPBind:     cfg.Listener.TCP.Bind,
    TLSCertFile: cfg.Listener.TCP.TLS.CertFile, // safe even if TLS is nil — use helper
    TLSKeyFile:  cfg.Listener.TCP.TLS.KeyFile,
}, listener.Deps{
    HealthProbe:    d.healthProbe,
    ConnectRoutes:  routes,
    WebhookHandler: webhook,
})
if err != nil {
    return fmt.Errorf("daemon: build api listener: %w", err)
}
if err := l.Start(ctx); err != nil {
    return fmt.Errorf("daemon: start api listener: %w", err)
}
defer func() { _ = l.Shutdown(context.Background()) }()
```

Concrete pieces required:

- A `schemeClassifier` that examines the connection (UDS vs TCP) — set via a per-connection context value populated by the listener's `ConnContext` callback. Add to `internal/api/listener/listener.go` an HTTP `ConnContext` hook that, when the underlying conn is `*net.UnixConn`, calls `WithPeerCred(ctx, ucred)` after reading credentials via `getPeerCred(unixConn)`. Implementation:

```go
// in internal/api/listener/listener.go, on the http.Server:
l.srv.ConnContext = connContext

// helper:
import (
    "syscall"
    "golang.org/x/sys/unix"
)

func connContext(ctx context.Context, c net.Conn) context.Context {
    if uc, ok := c.(*net.UnixConn); ok {
        if cred, err := readPeerCred(uc); err == nil {
            return WithPeerCred(ctx, cred)
        }
    }
    return ctx
}

func readPeerCred(c *net.UnixConn) (*syscall.Ucred, error) {
    raw, err := c.SyscallConn()
    if err != nil { return nil, err }
    var ucred *unix.Ucred
    var sysErr error
    err = raw.Control(func(fd uintptr) {
        ucred, sysErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
    })
    if err != nil { return nil, err }
    if sysErr != nil { return nil, sysErr }
    return &syscall.Ucred{Pid: ucred.Pid, Uid: ucred.Uid, Gid: ucred.Gid}, nil
}
```

- An `actionTable` map[string]auth.Action. Build inline by enumerating each generated `<Service>_<Method>` procedure constant from `v1alpha1connect`:

```go
actionTable := map[string]auth.Action{
    v1alpha1connect.EntityServiceListProcedure:           {Service: "EntityService", Method: "List", Verb: "read"},
    v1alpha1connect.EntityServiceGetProcedure:            {Service: "EntityService", Method: "Get", Verb: "read"},
    v1alpha1connect.EntityServiceCallCapabilityProcedure: {Service: "EntityService", Method: "CallCapability", Verb: "call"},
    v1alpha1connect.EntityServiceSubscribeProcedure:      {Service: "EntityService", Method: "Subscribe", Verb: "read"},
    // ... one row per (service, method); ~50 rows total.
}
```

- A `webhookMetricsAdapter` implementing `api.WebhookMetrics`:

```go
type webhookMetricsAdapter struct{ m *observability.Metrics }
func (a webhookMetricsAdapter) Inc(slug, result string) {
    if a.m != nil && a.m.APIWebhookReceivedTotal != nil {
        a.m.APIWebhookReceivedTotal.WithLabelValues(slug, result).Inc()
    }
}
```

- [ ] **Step 4: Update existing daemon tests**

`internal/daemon/daemon_test.go` may construct a `Daemon` and assert the socket exists. Update or replace those assertions to use the API listener: dial the listener over UDS using a Connect client, call `SystemService.Version`, assert no error.

```go
import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/gen/gohome/v1alpha1/v1alpha1connect"
)

func TestDaemon_APIListener_Version(t *testing.T) {
	d, dir := startTestDaemon(t)
	defer d.Stop()

	sock := filepath.Join(dir, "gohomed.sock")
	httpClient := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", sock)
		},
	}}
	client := v1alpha1connect.NewSystemServiceClient(httpClient, "http://unix")
	resp, err := client.Version(context.Background(), connect.NewRequest(&v1.VersionRequest{}))
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if resp.Msg.BinaryVersion == "" {
		t.Error("empty version")
	}
}
```

(`startTestDaemon` is the existing test helper; the precise name and signature vary. Match the pattern used by other `daemon_test.go` cases.)

- [ ] **Step 5: Build + test**

```bash
task build
task test
```

Expected: PASS.

- [ ] **Step 6: Race**

```bash
task test:race
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/api/listener/routes.go internal/api/listener/listener.go internal/daemon/api_adapters.go internal/daemon/daemon.go internal/daemon/daemon_test.go
git commit -m "feat(c7): wire api listener into daemon"
```

---

## Task 22: CLI dialer + first command port (`gohome system version`)

**Files:**
- Modify: `internal/cli/cliutil.go`
- Modify: `internal/cli/root.go`
- Create: `internal/cli/cmd_system.go`
- Create: `internal/cli/cmd_system_test.go`

- [ ] **Step 1: Failing test for the dialer**

Create `internal/cli/cliutil_dial_test.go`:

```go
package cli_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fynn-labs/gohome/internal/cli"
)

func TestDial_TCPEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, base, err := cli.Dial(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if c == nil || base != srv.URL {
		t.Errorf("got client=%v base=%q", c, base)
	}
}

func TestDial_UDSEndpoint(t *testing.T) {
	c, base, err := cli.Dial(context.Background(), "unix:///tmp/nonexistent.sock")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if c == nil || base != "http://unix" {
		t.Errorf("got client=%v base=%q", c, base)
	}
}

func TestResolveEndpoint_DefaultUDS(t *testing.T) {
	got := cli.ResolveEndpoint("", "/data")
	if got != "unix:///data/gohomed.sock" {
		t.Errorf("got %q", got)
	}
}

func TestResolveEndpoint_FlagWins(t *testing.T) {
	t.Setenv("GOHOME_ENDPOINT", "tcp://127.0.0.1:9000")
	got := cli.ResolveEndpoint("tcp://127.0.0.1:8888", "/data")
	if got != "tcp://127.0.0.1:8888" {
		t.Errorf("got %q", got)
	}
}

func TestResolveEndpoint_EnvWins(t *testing.T) {
	t.Setenv("GOHOME_ENDPOINT", "tcp://127.0.0.1:9000")
	got := cli.ResolveEndpoint("", "/data")
	if got != "tcp://127.0.0.1:9000" {
		t.Errorf("got %q", got)
	}
}
```

- [ ] **Step 2: Run — fail**

```bash
go test ./internal/cli/... -v
```

- [ ] **Step 3: Add dialer to `cliutil.go`**

Append to `internal/cli/cliutil.go`:

```go
package cli

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"
)

// ResolveEndpoint picks the API endpoint to dial.
// Precedence: explicit flag value > GOHOME_ENDPOINT env > unix://<dataDir>/gohomed.sock.
func ResolveEndpoint(flagValue, dataDir string) string {
	if flagValue != "" {
		return flagValue
	}
	if env := os.Getenv("GOHOME_ENDPOINT"); env != "" {
		return env
	}
	return "unix://" + dataDir + "/gohomed.sock"
}

// Dial returns an http.Client and a base URL suitable for a Connect-Go
// generated client constructor.
//
// For unix:// endpoints, the returned http.Client transport dials the socket
// path and the base URL is "http://unix" (the Host part is just a placeholder
// — the transport ignores it).
//
// For http(s):// or tcp:// endpoints, the base URL is the input (with tcp://
// rewritten to http://) and the client is the standard one.
func Dial(_ context.Context, endpoint string) (*http.Client, string, error) {
	switch {
	case strings.HasPrefix(endpoint, "unix://"):
		sock := strings.TrimPrefix(endpoint, "unix://")
		return &http.Client{Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", sock)
			},
		}}, "http://unix", nil
	case strings.HasPrefix(endpoint, "tcp://"):
		return http.DefaultClient, "http://" + strings.TrimPrefix(endpoint, "tcp://"), nil
	default:
		return http.DefaultClient, endpoint, nil
	}
}
```

- [ ] **Step 4: Add `--endpoint` flag**

In `internal/cli/root.go`, add a persistent flag:

```go
var endpointFlag string

func init() {
    rootCmd.PersistentFlags().StringVar(&endpointFlag, "endpoint", "", "API endpoint (unix:///path or tcp://host:port). Defaults to UDS in data dir or $GOHOME_ENDPOINT.")
}
```

Add a helper that other `cmd_*.go` files use:

```go
func endpointFromFlags() string {
    return ResolveEndpoint(endpointFlag, dataDirFromFlags()) // dataDirFromFlags is the existing data-dir resolution helper
}
```

- [ ] **Step 5: Implement `gohome system version`**

Create `internal/cli/cmd_system.go`:

```go
package cli

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/gen/gohome/v1alpha1/v1alpha1connect"
	"github.com/spf13/cobra"
)

func newSystemCmd() *cobra.Command {
	c := &cobra.Command{Use: "system", Short: "Daemon system commands"}
	c.AddCommand(newSystemVersionCmd())
	c.AddCommand(newSystemHealthCmd())
	return c
}

func newSystemVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show daemon version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			httpClient, base, err := Dial(cmd.Context(), endpointFromFlags())
			if err != nil {
				return err
			}
			cli := v1alpha1connect.NewSystemServiceClient(httpClient, base)
			resp, err := cli.Version(cmd.Context(), connect.NewRequest(&v1.VersionRequest{}))
			if err != nil {
				return renderConnectErr(err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), Success.Render(resp.Msg.BinaryVersion))
			fmt.Fprintln(cmd.OutOrStdout(), Dim.Render(fmt.Sprintf("commit %s · built %s · schema %s",
				resp.Msg.GitCommit, resp.Msg.BuildDate, resp.Msg.SchemaVersion)))
			return nil
		},
	}
}

func newSystemHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Show daemon health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), defaultRPCTimeout)
			defer cancel()
			httpClient, base, err := Dial(ctx, endpointFromFlags())
			if err != nil {
				return err
			}
			cli := v1alpha1connect.NewSystemServiceClient(httpClient, base)
			resp, err := cli.Health(ctx, connect.NewRequest(&v1.HealthRequest{}))
			if err != nil {
				return renderConnectErr(err)
			}
			out := cmd.OutOrStdout()
			if resp.Msg.Ok {
				fmt.Fprintln(out, Success.Render("OK"))
			} else {
				fmt.Fprintln(out, Error.Render("DEGRADED"))
			}
			fmt.Fprintln(out, Dim.Render(resp.Msg.Summary))
			for _, sub := range resp.Msg.Subsystems {
				marker := Success.Render("✓")
				if !sub.Ok {
					marker = Error.Render("✗")
				}
				fmt.Fprintf(out, "  %s %s — %s\n", marker, sub.Name, sub.Detail)
			}
			return nil
		},
	}
}
```

Add `defaultRPCTimeout` and `renderConnectErr` to `cliutil.go`:

```go
import (
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	errorv1 "github.com/fynn-labs/gohome/gen/gohome/error/v1alpha1"
)

const defaultRPCTimeout = 30 * time.Second

func renderConnectErr(err error) error {
	if err == nil {
		return nil
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		return err
	}
	for _, d := range ce.Details() {
		v, derr := d.Value()
		if derr != nil {
			continue
		}
		if ed, ok := v.(*errorv1.ErrorDetail); ok && ed.Reason != "" {
			return fmt.Errorf("%s: %s (request_id=%s)", ce.Code(), ed.Reason, ed.RequestId)
		}
	}
	return fmt.Errorf("%s: %s", ce.Code(), ce.Message())
}
```

Register `newSystemCmd()` in `internal/cli/root.go`:

```go
rootCmd.AddCommand(newSystemCmd())
```

- [ ] **Step 6: Run — pass**

```bash
go test ./internal/cli/... -v
```

- [ ] **Step 7: Manual smoke test**

```bash
task build
./dist/gohomed --config-dir testdata/fixtures/<some-fixture> &
sleep 1
./dist/gohome system version
./dist/gohome system health
kill %1
```

Expected: version + health render; `system health` shows per-subsystem status with the configured lipgloss styles.

- [ ] **Step 8: Commit**

```bash
git add internal/cli/cliutil.go internal/cli/cliutil_dial_test.go internal/cli/root.go internal/cli/cmd_system.go internal/cli/cmd_system_test.go
git commit -m "feat(c7): cli dialer + gohome system version/health"
```

---

## Task 23: Port `automation`, `script`, `state`, `events` commands

**Files:**
- Modify: `internal/cli/cmd_automation.go`
- Modify: `internal/cli/cmd_script.go`
- Modify: `internal/cli/state.go`
- Modify: `internal/cli/events.go`

For each command file: replace the `sendReq(conn, map[string]any{"op": ...})` calls with the corresponding Connect-Go client call. Keep the existing rendering code (lipgloss styles, table rendering) intact — only the data source changes from `map[string]any` to a typed proto message. Per-command pattern:

```go
// Before:
_ = sendReq(conn, map[string]any{"op": "automation_list"})
// After:
httpClient, base, err := Dial(cmd.Context(), endpointFromFlags())
if err != nil { return err }
client := v1alpha1connect.NewAutomationServiceClient(httpClient, base)
resp, err := client.List(cmd.Context(), connect.NewRequest(&v1.ListAutomationsRequest{}))
if err != nil { return renderConnectErr(err) }
renderAutomations(cmd.OutOrStdout(), resp.Msg.Automations)  // existing renderer
```

- [ ] **Step 1: Port `cmd_automation.go`**

Replace the `RunE` bodies for `list`, `get`, `enable`, `disable`, `trigger`, `trace`, `watch`. The `trace` command uses streaming:

```go
client := v1alpha1connect.NewAutomationServiceClient(httpClient, base)
stream, err := client.Trace(cmd.Context(), connect.NewRequest(&v1.TraceAutomationRequest{
    AutomationId: automationID, RunId: runID,
}))
if err != nil { return renderConnectErr(err) }
defer stream.Close()
for stream.Receive() {
    msg := stream.Msg()
    if ev := msg.GetEvent(); ev != nil {
        renderTraceLine(cmd.OutOrStdout(), ev)
    }
    // Heartbeats are ignored for the human watcher.
}
if err := stream.Err(); err != nil {
    return renderConnectErr(err)
}
```

`gohome automation watch` (which currently tails events filtered to automation-related kinds) becomes:

```go
evtClient := v1alpha1connect.NewEventServiceClient(httpClient, base)
stream, err := evtClient.Tail(cmd.Context(), connect.NewRequest(&v1.TailEventsRequest{
    Filter: &v1.EventFilter{Kinds: []string{"automation_triggered", "automation_finished", "script_invoked", "script_finished"}},
}))
// ... loop as above, render via existing watch renderer
```

Update existing tests in `cmd_automation_test.go` (if present) to drive a real `httptest.Server` running the Connect handlers. Pattern:

```go
func TestCmd_Automation_List_RendersTable(t *testing.T) {
    fa := &fakeAutomationsBackend{...}
    h := api.NewAutomationService(fa)
    mux := http.NewServeMux()
    mux.Handle(v1alpha1connect.NewAutomationServiceHandler(h))
    srv := httptest.NewServer(mux)
    defer srv.Close()

    var out bytes.Buffer
    rootCmd.SetOut(&out)
    rootCmd.SetArgs([]string{"--endpoint", srv.URL, "automation", "list"})
    if err := rootCmd.Execute(); err != nil { t.Fatal(err) }
    if !strings.Contains(out.String(), "expected-name") { t.Errorf("got %q", out.String()) }
}
```

- [ ] **Step 2: Port `cmd_script.go`**

Replace `script_list` and `script_run` with `ScriptService.List` / `ScriptService.Run`. Run the same way:

```go
client := v1alpha1connect.NewScriptServiceClient(httpClient, base)
args, err := structpb.NewStruct(parsedArgs)
if err != nil { return err }
resp, err := client.Run(cmd.Context(), connect.NewRequest(&v1.RunScriptRequest{Name: name, Args: args}))
if err != nil { return renderConnectErr(err) }
renderScriptResult(cmd.OutOrStdout(), resp.Msg)  // existing renderer
```

- [ ] **Step 3: Port `state.go`**

`gohome state get <id>` now calls `EntityService.Get`:

```go
client := v1alpha1connect.NewEntityServiceClient(httpClient, base)
resp, err := client.Get(cmd.Context(), connect.NewRequest(&v1.GetEntityRequest{Id: id}))
if err != nil { return renderConnectErr(err) }
renderEntity(cmd.OutOrStdout(), resp.Msg.Entity)  // existing renderer takes the proto directly
```

`gohome state list` (if present) calls `EntityService.List` with selector flags.

- [ ] **Step 4: Port `events.go`**

`gohome events list` → `EventService.Query` (paginated; iterate page tokens until empty, or stop after `--limit`).

`gohome events tail` → `EventService.Tail`:

```go
client := v1alpha1connect.NewEventServiceClient(httpClient, base)
stream, err := client.Tail(cmd.Context(), connect.NewRequest(&v1.TailEventsRequest{
    Filter: filterFromFlags(),
}))
if err != nil { return renderConnectErr(err) }
defer stream.Close()
for stream.Receive() {
    msg := stream.Msg()
    if ev := msg.GetEvent(); ev != nil {
        renderEvent(cmd.OutOrStdout(), ev)
    }
}
if err := stream.Err(); err != nil {
    return renderConnectErr(err)
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/cli/... -v
```

Expected: PASS. Update / add fixture-based tests as you go.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/cmd_automation.go internal/cli/cmd_script.go internal/cli/state.go internal/cli/events.go internal/cli/*_test.go
git commit -m "feat(c7): port automation/script/state/events CLI to Connect"
```

---

## Task 24: Port `config`, `driver`, `snapshot`, `eval`, `test` commands

**Files:**
- Modify: `internal/cli/config.go`
- Modify: `internal/cli/driver.go`
- Modify: `internal/cli/snapshot.go`
- Modify: `internal/cli/eval.go`
- Modify: `internal/cli/test.go`

- [ ] **Step 1: Port `config.go`**

Replace the existing `validate`/`apply` op calls with `ConfigService.Validate` and `ConfigService.Apply`. The Pkl bundle is the tar.gz of the config dir produced by the existing helper. If the helper writes to a temp file, read it back:

```go
bundle, err := os.ReadFile(bundlePath)
if err != nil { return err }
client := v1alpha1connect.NewConfigServiceClient(httpClient, base)
resp, err := client.Validate(cmd.Context(), connect.NewRequest(&v1.ValidateConfigRequest{PklBundle: bundle}))
if err != nil { return renderConnectErr(err) }
renderValidateResult(cmd.OutOrStdout(), resp.Msg)  // existing renderer takes a typed shape now
```

`apply` adds `Message` and `DryRun` flags; `--strict` populates `ExpectedBundleHash` from a prior `validate`.

- [ ] **Step 2: Port `driver.go`**

`gohome driver list-instances` → `DriverService.ListInstances`. `gohome driver restart <id>` → `DriverService.RestartInstance`. `gohome driver health <id>` → `DriverService.InstanceHealth`.

- [ ] **Step 3: Port `snapshot.go`**

`gohome snapshot create --owner <o> --reason <r>` → `SystemService.CreateSnapshot`:

```go
client := v1alpha1connect.NewSystemServiceClient(httpClient, base)
resp, err := client.CreateSnapshot(cmd.Context(), connect.NewRequest(&v1.CreateSnapshotRequest{
    Owner: owner, Reason: reason,
}))
if err != nil { return renderConnectErr(err) }
fmt.Fprintf(cmd.OutOrStdout(), "snapshot: cursor %d at %s\n", resp.Msg.Cursor, resp.Msg.CreatedAt.AsTime())
```

- [ ] **Step 4: Port `eval.go`**

`gohome eval <expr>` → `ScriptService.Eval`:

```go
client := v1alpha1connect.NewScriptServiceClient(httpClient, base)
resp, err := client.Eval(cmd.Context(), connect.NewRequest(&v1.EvalScriptRequest{Expr: expr}))
if err != nil { return renderConnectErr(err) }
if resp.Msg.Stdout != "" {
    fmt.Fprint(cmd.OutOrStdout(), resp.Msg.Stdout)
}
fmt.Fprintln(cmd.OutOrStdout(), formatStarlarkValue(resp.Msg.Result))
```

- [ ] **Step 5: Port `test.go`**

`gohome test <path>` → `ScriptService.RunTests` (server-stream):

```go
client := v1alpha1connect.NewScriptServiceClient(httpClient, base)
stream, err := client.RunTests(cmd.Context(), connect.NewRequest(&v1.RunStarlarkTestsRequest{Path: path}))
if err != nil { return renderConnectErr(err) }
defer stream.Close()
var pass, fail int
for stream.Receive() {
    if ev := stream.Msg().GetEvent(); ev != nil {
        renderTestEvent(cmd.OutOrStdout(), ev)
        if ev.Outcome == "pass" { pass++ }
        if ev.Outcome == "fail" { fail++ }
    }
}
if err := stream.Err(); err != nil {
    return renderConnectErr(err)
}
renderTestSummary(cmd.OutOrStdout(), pass, fail)
if fail > 0 {
    os.Exit(1)
}
return nil
```

- [ ] **Step 6: Run tests**

```bash
go test ./internal/cli/... -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/config.go internal/cli/driver.go internal/cli/snapshot.go internal/cli/eval.go internal/cli/test.go internal/cli/*_test.go
git commit -m "feat(c7): port config/driver/snapshot/eval/test CLI to Connect"
```

---

## Task 25: Delete the JSON-op surface; end-to-end integration tests

**Files:**
- Modify (delete code): `internal/daemon/recovery.go`
- Modify: `internal/daemon/daemon.go` (remove the `serveSocket` call if any reference remains; remove the `socketPath` config plumbing if the API listener fully replaces it)
- Modify: `internal/cli/cliutil.go` (delete `sendReq` and any Unix-socket dialing helpers used only by the JSON-op path)
- Create: `internal/api/listener/listener_integration_test.go`

- [ ] **Step 1: Delete the JSON-op switch**

In `internal/daemon/recovery.go`, the function `serveSocket` (and its op-dispatch `switch` statement) become dead code as of Task 21. Delete the entire file. Also delete:

- The `serveSocket` reference from `internal/daemon/daemon.go`
- Any helper functions in `internal/daemon/recovery.go` that are no longer used (the entire op handler functions: `handleSnapshot`, `handleAutomationList`, etc.)
- The legacy socket-path config field if it is now unused (check `internal/daemon/config.go` — the API listener uses `Listener.UDS.Path` from C7's Pkl additions, so the old `SocketPath` may be removable)

In `internal/cli/cliutil.go`, delete `sendReq` and any helpers that were only used by the JSON-op CLI commands.

- [ ] **Step 2: Build — confirm no dead references**

```bash
task build
```

Expected: no compile errors, no references to deleted symbols.

- [ ] **Step 3: Failing integration test**

Create `internal/api/listener/listener_integration_test.go`:

```go
//go:build integration

package listener_test

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
	"github.com/fynn-labs/gohome/gen/gohome/v1alpha1/v1alpha1connect"
	"github.com/fynn-labs/gohome/internal/testutil"
)

func TestEndToEnd_SystemVersionOverUDS(t *testing.T) {
	d := testutil.StartDaemon(t)
	defer d.Stop()

	sock := filepath.Join(d.DataDir, "gohomed.sock")
	httpClient := udsClient(sock)
	client := v1alpha1connect.NewSystemServiceClient(httpClient, "http://unix")
	resp, err := client.Version(context.Background(), connect.NewRequest(&v1.VersionRequest{}))
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if resp.Msg.BinaryVersion == "" {
		t.Error("empty version")
	}
}

func TestEndToEnd_TailResumeAcrossReconnect(t *testing.T) {
	d := testutil.StartDaemon(t)
	defer d.Stop()

	sock := filepath.Join(d.DataDir, "gohomed.sock")
	httpClient := udsClient(sock)
	client := v1alpha1connect.NewEventServiceClient(httpClient, "http://unix")

	// Inject an event via the testutil helper, then tail from cursor 0.
	d.AppendTestEvent(t, "state_changed", "light.a")

	stream, err := client.Tail(context.Background(), connect.NewRequest(&v1.TailEventsRequest{
		Filter: &v1.EventFilter{FromCursor: 0},
	}))
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}

	var firstCursor uint64
	for stream.Receive() {
		if ev := stream.Msg().GetEvent(); ev != nil {
			firstCursor = ev.Cursor
			break
		}
	}
	stream.Close()
	if firstCursor == 0 {
		t.Fatal("no event seen on first tail")
	}

	// Inject one more, reconnect with from_cursor, expect to see the new one.
	d.AppendTestEvent(t, "state_changed", "light.b")

	stream2, err := client.Tail(context.Background(), connect.NewRequest(&v1.TailEventsRequest{
		Filter: &v1.EventFilter{FromCursor: firstCursor},
	}))
	if err != nil {
		t.Fatalf("Tail2: %v", err)
	}
	defer stream2.Close()

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("did not see resumed event")
		default:
		}
		if !stream2.Receive() {
			t.Fatalf("Receive: %v", stream2.Err())
		}
		if ev := stream2.Msg().GetEvent(); ev != nil && ev.Cursor > firstCursor {
			return // pass
		}
	}
}

func TestEndToEnd_HeartbeatFires(t *testing.T) {
	d := testutil.StartDaemon(t, testutil.WithStreamHeartbeat(50*time.Millisecond))
	defer d.Stop()

	httpClient := udsClient(filepath.Join(d.DataDir, "gohomed.sock"))
	client := v1alpha1connect.NewEventServiceClient(httpClient, "http://unix")
	stream, err := client.Tail(context.Background(), connect.NewRequest(&v1.TailEventsRequest{}))
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	defer stream.Close()

	deadline := time.After(time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("no heartbeat in 1s")
		default:
		}
		if !stream.Receive() {
			t.Fatalf("Receive: %v", stream.Err())
		}
		if stream.Msg().GetHeartbeat() != nil {
			return // pass
		}
	}
}

func TestEndToEnd_TCPRequiresAuth(t *testing.T) {
	d := testutil.StartDaemon(t)
	defer d.Stop()

	resp, err := http.Get("http://" + d.TCPAddr() + "/healthz")
	if err != nil {
		t.Fatalf("/healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/healthz status = %d", resp.StatusCode)
	}

	tcpClient := v1alpha1connect.NewSystemServiceClient(http.DefaultClient, "http://"+d.TCPAddr())
	_, err = tcpClient.Version(context.Background(), connect.NewRequest(&v1.VersionRequest{}))
	var ce *connect.Error
	if err == nil {
		t.Fatal("expected error on TCP without creds")
	}
	if errors.As(err, &ce) && ce.Code() != connect.CodeUnauthenticated {
		t.Errorf("code = %v, want Unauthenticated", ce.Code())
	}
}

func TestEndToEnd_WebhookFiresAutomation(t *testing.T) {
	d := testutil.StartDaemon(t, testutil.WithFixture("webhook-fires-automation"))
	defer d.Stop()

	body := `{"x":1}`
	resp, err := d.PostWebhook(t, "test_hook", "shh", body)
	if err != nil {
		t.Fatalf("post webhook: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	// Wait for automation_triggered event to appear in the store.
	if !d.WaitForEvent(t, "automation_triggered", 2*time.Second) {
		t.Fatal("automation did not fire")
	}
}

func udsClient(sock string) *http.Client {
	return &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", sock)
		},
	}}
}
```

(`internal/testutil` should expose `StartDaemon(t, ...opts)`, `AppendTestEvent`, `WaitForEvent`, `PostWebhook`, `TCPAddr()` and `WithStreamHeartbeat`/`WithFixture` options. C1's testutil already has the basic daemon-start helper; extend it with the new options as Step 4.)

- [ ] **Step 4: Extend `internal/testutil`**

Add to `internal/testutil`:

```go
type DaemonOpt func(*daemonOpts)
type daemonOpts struct {
	heartbeat time.Duration
	fixture   string
}

func WithStreamHeartbeat(d time.Duration) DaemonOpt { return func(o *daemonOpts) { o.heartbeat = d } }
func WithFixture(name string) DaemonOpt              { return func(o *daemonOpts) { o.fixture = name } }

func StartDaemon(t *testing.T, opts ...DaemonOpt) *RunningDaemon {
	// Build a daemon.Daemon with an in-memory SQLite, optionally a Pkl fixture
	// dir, optionally an overridden heartbeat. Start it. Return a handle with
	// helpers used by integration tests.
	// ...
}

type RunningDaemon struct {
	DataDir string
	tcpAddr string
	// ...
}

func (d *RunningDaemon) TCPAddr() string { return d.tcpAddr }
func (d *RunningDaemon) Stop()           { /* shut everything down */ }
func (d *RunningDaemon) AppendTestEvent(t *testing.T, kind, entity string) { /* call eventstore.Append */ }
func (d *RunningDaemon) WaitForEvent(t *testing.T, kind string, dur time.Duration) bool { /* tail and watch */ }
func (d *RunningDaemon) PostWebhook(t *testing.T, slug, secret, body string) (*http.Response, error) {
    // Sign with HMAC-SHA256, POST to http://<tcpAddr>/webhooks/<slug>.
    // ...
}
```

The fixture `webhook-fires-automation` is a Pkl dir under `internal/testutil/fixtures/webhook-fires-automation/main.pkl` that declares one automation with a `WebhookTrigger{slug = "test_hook", secret = "shh"}` and an action that emits a known event.

- [ ] **Step 5: Run integration tests**

```bash
task test:integration
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/listener/listener_integration_test.go internal/testutil/ internal/daemon/recovery.go internal/daemon/daemon.go internal/daemon/config.go internal/cli/cliutil.go
git commit -m "feat(c7): delete JSON-op surface; add api integration suite"
```

---

## Task 26: Definition of done

- [ ] **Step 1: Full build**

```bash
task build
```

Expected: both binaries build cleanly.

- [ ] **Step 2: Unit tests**

```bash
task test
```

Expected: PASS.

- [ ] **Step 3: Race detector**

```bash
task test:race
```

Expected: PASS — no data races. Streaming tests (`Task 16-17`) and listener tests (`Task 4`) are the most likely to surface a race; if any pop, fix the underlying synchronization (do not paper over with `sync.Mutex` on hot paths).

- [ ] **Step 4: Integration tests**

```bash
task test:integration
```

Expected: PASS.

- [ ] **Step 5: Lint**

```bash
task lint
```

Expected: no issues. Common new-code complaints to watch for:
- `errcheck` on `stream.Send(...)` returns — every send in the streaming endpoints must check err and abort the loop on failure. (Task 17 already does this.)
- `revive` / `staticcheck` on the generated `_ = unused` shadow imports in `service_unimplemented.go` — fix by removing the unused import or renaming.
- `gocritic` / `gocyclo` on the bigger handlers — split a helper if any handler crosses 50 cyclomatic.

- [ ] **Step 6: Tidy**

```bash
go mod tidy
```

Expected: no diff to `go.mod`/`go.sum`. If diffs, stage and commit.

- [ ] **Step 7: Verify proto is current**

```bash
git diff gen/
```

Expected: clean. If any `gen/` files differ from the last `task proto` run, run it again, stage, commit.

- [ ] **Step 8: Smoke the binary**

```bash
./dist/gohomed --config-dir internal/config/testdata/<a-fixture> &
sleep 1
./dist/gohome system version
./dist/gohome system health
./dist/gohome automation list
./dist/gohome events tail &  # should connect, print heartbeats every 30s
sleep 1
kill %2 %1
```

Expected:
- `system version` prints styled version info.
- `system health` prints OK with subsystem rows.
- `automation list` prints a table (possibly empty).
- `events tail` connects, prints any current events, then idles printing nothing (heartbeats are filtered out for the human watcher).
- The daemon exits cleanly on SIGTERM and the UDS file is removed.

- [ ] **Step 9: Final commit if needed**

If steps 6-8 surfaced any cleanup:

```bash
git add <files>
git commit -m "chore(c7): final cleanup after definition-of-done"
```

---

## Self-review checklist (for the implementer)

After all tasks pass:

- [ ] `gohome --endpoint unix:///path/to/sock system version` prints styled version
- [ ] `gohome --endpoint tcp://127.0.0.1:8080 system version` returns `UNAUTHENTICATED` with a clear error
- [ ] `gohome events tail` resumes from `last_seen_cursor` on reconnect (kill the CLI mid-stream, restart, verify no missed events)
- [ ] `gohome events tail` emits a heartbeat through the wire when idle (the CLI hides them; check via `grpcurl` or by setting a low heartbeat interval and reading `gohome_api_stream_heartbeats_sent_total`)
- [ ] A `WebhookTrigger`-backed automation fires when the operator POSTs to `/webhooks/<slug>` with a valid HMAC
- [ ] A POST with a bad signature returns 401 and increments `gohome_api_webhook_received_total{result="bad_signature"}`
- [ ] A POST with an unknown slug returns 404
- [ ] `gohome script eval '1 + 1'` prints `2`
- [ ] `gohome snapshot create --owner state_cache --reason manual` returns a cursor
- [ ] `gohome config validate` and `gohome config apply --dry-run` accept a Pkl bundle and render the diff
- [ ] `internal/daemon/recovery.go` is gone; nothing in the tree imports `sendReq`
- [ ] `gohome_api_requests_total{procedure=...,code="ok"}` increments per RPC
- [ ] Killing `gohomed` with SIGTERM removes the UDS socket file from disk
- [ ] Streaming RPCs close with `RESOURCE_EXHAUSTED` when a client stalls past 10k buffered events (verify via the integration backpressure test)
- [ ] All thirteen `gohome.v1alpha1.*` services appear in `buf curl --schema buf.build/... gohome.v1alpha1` (or `grpcurl -plaintext 127.0.0.1:8080 list`); `Scene`, `Dashboard`, `Auth` methods return `UNIMPLEMENTED`

---

## Appendix: Connect-Go API gotchas

- **Server stream closes don't propagate `connect.CodeOK`.** Returning `nil` from a streaming handler closes the stream cleanly; no need to send a final empty message.
- **`connect.NewError`** wraps a Go error, but the wrapped error's text becomes the wire message — never wrap a sensitive error directly. Use `errors.New("internal error")` or a sanitized message and log the original via slog.
- **`http.MaxBytesReader`** returns `*http.MaxBytesError` (Go 1.20+); use `errors.As` for the size-cap check (Task 18 webhook handler).
- **`h2c.NewHandler`** requires the underlying `http.Server` to NOT have its own `TLSConfig`; we serve TLS only on the TCP listener via `tls.NewListener` wrapping the raw TCP listener, so the `http.Server.Handler` stays h2c-wrapped.
- **`connect.WithInterceptors`** composes interceptors in declaration order: the first listed runs outermost. Order in Task 21 matches the spec's interceptor stack: recover (outermost) → request-id → slog → metrics → authenticate → authorize.
- **Generated `*Procedure` constants** live in `v1alpha1connect` and look like `EntityServiceListProcedure = "/gohome.v1alpha1.EntityService/List"`. Use them as the keys of `actionTable` (Task 21) so the action map is rename-safe.

---

## Appendix: Adapter implementation pointers

When filling in the engine adapters (Task 21), these are the existing C1-C6 entry points:

| api.Interface method | Backing call |
|---|---|
| `EntityReader.GetEntity(id)` | `registry.Registry.GetEntity(id)` + `state.Cache.Get(id)` |
| `EntityReader.ListEntities` | `registry.Registry.IterEntities` (or equivalent) with selector applied in the adapter |
| `CapabilityCaller.Call` | `carport.Supervisor.IssueCommand` (or whatever C2/C3 named the dispatch entry point) |
| `EventSource.Query` | `eventstore.Store.Query(ctx, filter, limit)` |
| `EventSource.Subscribe` | `eventstore.Store.Subscribe(ctx, filter, fromCursor)` |
| `DriverControl.RestartInstance` | `carport.Supervisor.Restart(ctx, instanceID)` plus an explicit `eventstore.Append(DriverInstanceRestarted{...})` |
| `DeviceWriter.RenameDevice` | `registry.Registry.UpdateDevice(...)` plus `eventstore.Append(DeviceRenamed{...})` |
| `AutomationControl.Trigger` | `automation.Engine.Trigger(ctx, id)` |
| `AutomationControl.Trace` | `automation.Engine.Trace(ctx, automationID, runID, fromCursor)` (already exists for `gohome automation trace`) |
| `ScriptRunner.Run` | `script.Engine.Run(ctx, name, args)` |
| `ScriptRunner.Eval` | `starlark.Runtime.Execute(ctx, KindScratch, expr, nil)` |
| `ScriptRunner.RunTests` | `starlark.Runtime.RunTests(ctx, path)` |
| `ConfigApplier.Validate` | `config.Manager.Validate(ctx, bundle)` |
| `ConfigApplier.Apply` | `config.Manager.Apply(ctx, bundle, ...)` |
| `SystemBackend.MetricsText` | `observability.Metrics.GatherText()` (existing) |
| `SystemBackend.CreateSnapshot` | `eventstore.Store.Snapshot(owner, reason)` |
| `WebhookRouter.SecretFor` | `automation.Engine.WebhookSecret(slug)` (new method exposing per-slug secret from the compiled automation graph) |
| `WebhookAppender.AppendWebhook` | `eventstore.Store.Append(WebhookReceived{...})` |

Where a method does not yet exist on the C1-C6 surface, add the *minimum* method needed (no extra abstraction) on the engine package, in the same commit as the adapter that uses it. Keep the engine method name explicit and narrow.

---

*End of C7 implementation plan.*




