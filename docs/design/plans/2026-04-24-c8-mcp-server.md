# C8 — MCP Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a stdio-transport MCP server for `gohomed` as a `gohome mcp serve` CLI subcommand, exposing 12 tools and 2 resource types backed by the C7 Connect-RPC API. Local-only via UDS peer-cred; HTTP transport and token auth deferred to C9.

**Architecture:** New `internal/mcp` Go package built on `github.com/modelcontextprotocol/go-sdk`. Tool / resource handlers translate to C7 Connect calls via a UDS Connect-go client tagged with `x-gohome-source: mcp`. Filesystem tools touch the config dir directly with strict path containment, but emit audit via a daemon-side RPC. Two new event payloads (`MCPEvalRequested`, `ConfigFileEdited`) extend the existing `gohome.event.v1.Payload` oneof in C7's "external ingress" 60-69 block. `ScriptService.Eval` gains a 64 KiB result cap when called from MCP.

**Tech Stack:** Go 1.25, `github.com/modelcontextprotocol/go-sdk` v1.5.0+, `connectrpc.com/connect` (already in C7), existing `internal/eventstore`, `internal/state`, `internal/registry`, `internal/config`, `internal/automation`, `internal/script`, `internal/api` (from C7), `internal/auth` (from C7), `internal/observability`. Pkl evaluator (already in C4).

**Depends on:** C7 must be merged before this plan can land (this plan assumes `internal/api/`, `internal/auth/`, `proto/gohome/v1alpha1/`, the `x-gohome-source` interceptor wiring point, and the C7 listener stack all exist).

---

## Codebase orientation

Before starting, read these files to understand existing patterns:

| File | Why |
|---|---|
| `docs/superpowers/specs/2026-04-24-c8-mcp-server-design.md` | This plan's source of truth |
| `docs/superpowers/specs/2026-04-23-c7-connect-rpc-api-design.md` | The Connect surface this wraps |
| `internal/api/listener/listener.go` | Where the C7 listener is built; not modified by C8 |
| `internal/api/listener/interceptors.go` | Interceptor stack; C8 adds the source-header reader |
| `internal/api/service_system.go` | C7 SystemService impl; C8 adds three RPC handlers here |
| `internal/api/service_script.go` | C7 ScriptService impl; C8 enhances `Eval` |
| `internal/auth/auth.go` | Principal, Authenticator, Authorizer interfaces; C8 consumes them |
| `internal/cli/cliutil.go` | Pattern for dialing the daemon; reused by `gohome mcp serve` |
| `internal/cli/styles.go`, `internal/cli/styles_automation.go` | Lipgloss style modules; `gohome mcp tools` follows the same pattern |
| `internal/cli/command.go`, `internal/cli/root.go` | Subcommand registration pattern |
| `internal/observability/metrics.go` | Where Prometheus metrics are registered |
| `internal/starlark/context.go` | `KindMCPEval` already exists from C5 |
| `internal/script/eval.go` (or wherever C7 lands ScriptService.Eval) | Existing eval handler the cap layers onto |
| `proto/gohome/event/v1/event.proto` | `Payload` oneof to extend (tags 61, 62) |
| `proto/gohome/v1alpha1/system.proto` | C7 SystemService proto; C8 adds three RPCs |
| `internal/config/pkl/gohome/` | Pkl module layout; new `mcp.pkl` lands here |
| `docs/proto-hygiene.md` (in `gohome/`) | Grouped-numbering + reserved-forever rules |

---

## File map

### New files (in `gohome/`)

| Path | Responsibility |
|---|---|
| `internal/config/pkl/gohome/mcp.pkl` | `MCPConfig` Pkl class with caps and buffer sizes |
| `internal/api/source.go` | `x-gohome-source` server-side interceptor + context helpers |
| `internal/api/mcp_interceptor.go` | Reads `x-gohome-mcp-tool` / `-resource` / `-session` headers; emits `gohome_mcp_*` metrics |
| `internal/mcp/server.go` | Builds SDK server, registers tools/resources, runs stdio loop |
| `internal/mcp/deps.go` | Interface set tools/resources depend on |
| `internal/mcp/client.go` | Connect-go client wired to UDS, source/session/per-call header interceptors |
| `internal/mcp/errors.go` | Connect → MCP tool error mapping |
| `internal/mcp/actions.go` | Action catalog stub (per-tool, per-resource) |
| `internal/mcp/audit/recorder.go` | Calls `SystemService.RecordConfigFileEdit` |
| `internal/mcp/fs/safepath.go` | Path containment + symlink resolution |
| `internal/mcp/fs/syntax.go` | Best-effort `.pkl` and `.star` syntax check |
| `internal/mcp/tools/tool.go` | Tool registration helpers, common marshaling |
| `internal/mcp/tools/entities.go` | `get_state`, `list_entities`, `call_capability` |
| `internal/mcp/tools/events.go` | `query_events`, `tail_events` |
| `internal/mcp/tools/scenes.go` | `apply_scene` (UNIMPLEMENTED passthrough) |
| `internal/mcp/tools/scripts.go` | `run_script`, `eval_starlark` |
| `internal/mcp/tools/config.go` | `validate_config`, `apply_config` |
| `internal/mcp/tools/files.go` | `read_config_file`, `write_config_file` |
| `internal/mcp/resources/resource.go` | Resource registration + per-session subscription map |
| `internal/mcp/resources/entities.go` | Entity URI scheme + subscribe |
| `internal/mcp/resources/traces.go` | Trace URI scheme + subscribe |
| `internal/mcp/server_test.go` | SDK-level integration test |
| `internal/mcp/integration_test.go` | End-to-end with real subprocess (`//go:build integration`) |
| `internal/cli/cmd_mcp.go` | `gohome mcp serve`, `gohome mcp tools` |
| `internal/cli/styles_mcp.go` | Three new lipgloss badge styles + helpers |
| (test files) | One `*_test.go` per source file noted above |

### Modified files (in `gohome/`)

| Path | Change |
|---|---|
| `go.mod`, `go.sum` | Add `github.com/modelcontextprotocol/go-sdk` |
| `proto/gohome/event/v1/event.proto` | Add `MCPEvalRequested = 61`, `ConfigFileEdited = 62` payloads |
| `proto/gohome/v1alpha1/system.proto` | Add `GetConfigDir`, `RecordConfigFileEdit`, `GetMCPConfig` RPCs |
| `internal/api/service_system.go` | Implement the three new RPC handlers |
| `internal/api/service_script.go` | Enforce 64 KiB cap and emit `MCPEvalRequested` when source is `mcp` |
| `internal/api/listener/interceptors.go` | Install `source.Interceptor` and `mcp_interceptor.Interceptor` |
| `internal/observability/metrics.go` | Register `gohome_mcp_*` metrics; add `source` label to `gohome_api_requests_total` |
| `internal/config/loader.go` (or equivalent) | Load `gohome.mcp` Pkl module into the daemon's config struct |
| `internal/daemon/daemon.go` | Surface MCP config to the API layer |
| `cmd/gohome/main.go` | Register `mcp` subcommand |
| `Taskfile.yml` | (No change unless lint excludes are needed) |

---

## Task 1: Add the official MCP Go SDK dependency

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add the dependency**

```bash
cd gohome
go get github.com/modelcontextprotocol/go-sdk@latest
go mod tidy
```

- [ ] **Step 2: Verify build still compiles**

```bash
task build
```

Expected: both `gohomed` and `gohome` build without errors.

- [ ] **Step 3: Sanity-check the imported version**

```bash
go list -m github.com/modelcontextprotocol/go-sdk
```

Expected: a version `>= v1.5.0`. If the resolved version is below `v1.5.0`, set the floor explicitly:

```bash
go get github.com/modelcontextprotocol/go-sdk@v1.5.0
go mod tidy
```

- [ ] **Step 4: Skim the SDK README to confirm the symbols this plan uses exist**

Open `https://github.com/modelcontextprotocol/go-sdk` in a browser (or `go doc github.com/modelcontextprotocol/go-sdk` locally). Confirm:
- A server constructor (e.g. `mcp.NewServer`).
- A tool registration helper (e.g. `mcp.AddTool` or `server.AddTool`).
- A resource registration helper (e.g. `mcp.AddResource` / `mcp.AddResourceTemplate`).
- A subscription notification helper (e.g. `server.Notify("notifications/resources/updated", ...)` or a typed wrapper).
- A stdio transport (e.g. `mcp.Stdio(...)` or `server.Run(ctx, transport)`).

**If any name in this plan disagrees with the SDK,** prefer the SDK's actual name and adjust subsequent tasks consistently. The shapes in this plan are intentionally close to common Go SDK conventions but not authoritative.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum
git commit -m "feat(c8): add modelcontextprotocol/go-sdk dependency"
```

---

## Task 2: Extend `event.proto` with C8 audit payloads

**Files:**
- Modify: `proto/gohome/event/v1/event.proto`

- [ ] **Step 1: Read the current `Payload` oneof** to find where C7 left off (block 60-69 should already have `WebhookReceived = 60`).

```bash
grep -n "60-69\|WebhookReceived\|ConfigFileEdited\|MCPEvalRequested" proto/gohome/event/v1/event.proto
```

Expected: `WebhookReceived webhook_received = 60;` exists from C7. Tags 61 and 62 are unused.

- [ ] **Step 2: Add the new oneof entries**

In the `Payload` oneof, immediately after the `WebhookReceived` line, insert:

```protobuf
    MCPEvalRequested mcp_eval_requested = 61;
    ConfigFileEdited config_file_edited = 62;
```

- [ ] **Step 3: Add the message definitions at the bottom of the file**

```protobuf
message MCPEvalRequested {
  // 1-9: identity
  string principal_id = 1;        // "system:local" in v1
  string session_id   = 2;        // ULID minted by `gohome mcp serve` at startup

  // 10-19: payload
  string source            = 10;  // raw Starlark source as submitted
  string result_sha256_hex = 11;  // sha256 of the (post-truncation) repr output
  bool   truncated         = 12;
  uint32 result_bytes      = 13;  // pre-truncation size in bytes

  // 20-29: outcome
  uint32 duration_ms = 20;
  string error       = 21;        // empty on success
}

message ConfigFileEdited {
  // 1-9: identity
  string principal_id = 1;
  string session_id   = 2;        // empty when source is not MCP

  // 10-19: change
  string path        = 10;        // relative to config dir, as supplied by the writer
  string sha256_hex  = 11;        // post-write sha256
  uint32 size_bytes  = 12;
}
```

- [ ] **Step 4: Regenerate**

```bash
task proto
```

Expected: `gen/gohome/event/v1/event.pb.go` updated with new types; no errors.

- [ ] **Step 5: Verify build**

```bash
task build
```

Expected: compiles cleanly. (No handler is consuming the new payloads yet.)

- [ ] **Step 6: Commit**

```bash
git add proto/gohome/event/v1/event.proto gen/gohome/event/v1/
git commit -m "feat(c8): add MCPEvalRequested and ConfigFileEdited event payloads"
```

---

## Task 3: Add three new RPCs to `SystemService`

**Files:**
- Modify: `proto/gohome/v1alpha1/system.proto`

- [ ] **Step 1: Read the current SystemService definition**

```bash
grep -n "service SystemService\|rpc " proto/gohome/v1alpha1/system.proto
```

Expected: existing RPCs `Version`, `Health`, `Metrics`, `Diagnostics`, `CreateSnapshot` (the last one added by C7 §9.3).

- [ ] **Step 2: Add the three new RPCs**

Inside `service SystemService { ... }`, after the last existing RPC, add:

```protobuf
  rpc GetConfigDir       (GetConfigDirRequest)        returns (GetConfigDirResponse);
  rpc RecordConfigFileEdit(RecordConfigFileEditRequest) returns (RecordConfigFileEditResponse);
  rpc GetMCPConfig       (GetMCPConfigRequest)        returns (GetMCPConfigResponse);
```

- [ ] **Step 3: Add the request/response messages at the bottom of the file**

```protobuf
message GetConfigDirRequest {}

message GetConfigDirResponse {
  // 1-9: identity
  string config_dir = 1;          // absolute path
}

message RecordConfigFileEditRequest {
  // 1-9: identity (session id is also picked up from the x-gohome-mcp-session header)
  string session_id = 1;

  // 10-19: change
  string path       = 10;         // relative to config dir
  string sha256_hex = 11;
  uint32 size_bytes = 12;
}

message RecordConfigFileEditResponse {
  // 1-9: result
  uint64 event_cursor = 1;        // cursor of the appended ConfigFileEdited event
}

message GetMCPConfigRequest {}

message GetMCPConfigResponse {
  // 1-9: caps
  uint32 eval_result_max_bytes        = 1;
  uint32 read_file_max_bytes          = 2;

  // 10-19: subscription buffers
  uint32 entity_subscription_buffer = 10;
  uint32 trace_subscription_buffer  = 11;

  // 20-29: tail
  uint32 tail_default_wait_seconds = 20;
  uint32 tail_max_wait_seconds     = 21;
}
```

- [ ] **Step 4: Regenerate**

```bash
task proto
```

Expected: new types appear in `gen/gohome/v1alpha1/`; the connect interfaces gain three new methods. The build will now fail because the existing `service_system.go` does not implement them.

- [ ] **Step 5: Verify the failure is exactly the expected one**

```bash
task build 2>&1 | grep -E "GetConfigDir|RecordConfigFileEdit|GetMCPConfig"
```

Expected: errors complaining that `*systemService` does not implement the new methods. Anything else is unexpected and should be investigated.

- [ ] **Step 6: Commit (proto only — handlers come in Task 4)**

```bash
git add proto/gohome/v1alpha1/system.proto gen/gohome/v1alpha1/
git commit -m "feat(c8): add GetConfigDir, RecordConfigFileEdit, GetMCPConfig RPCs (proto only)"
```

---

## Task 4: Implement the three new `SystemService` handlers

**Files:**
- Modify: `internal/api/service_system.go`
- Modify: `internal/api/deps.go` (extend the dep interface)
- Test: `internal/api/service_system_test.go`

- [ ] **Step 1: Extend the dep interface in `internal/api/deps.go`**

Find the `SystemControl` (or similarly named) interface that C7 created for `SystemService`'s deps. Add three methods:

```go
type SystemControl interface {
    // ... existing methods (Version, Health, Metrics, Diagnostics, CreateSnapshot) ...

    // ConfigDir returns the resolved daemon config directory (absolute path).
    ConfigDir(ctx context.Context) (string, error)

    // MCPConfig returns the in-memory snapshot of gohome.mcp Pkl values.
    MCPConfig(ctx context.Context) (MCPConfig, error)

    // RecordConfigFileEdit appends a ConfigFileEdited event for the supplied
    // path (which MUST be inside ConfigDir — implementations re-validate as
    // defense in depth) and returns the appended event's cursor.
    RecordConfigFileEdit(ctx context.Context, principal auth.Principal, sessionID, path, sha256Hex string, sizeBytes uint32) (uint64, error)
}

type MCPConfig struct {
    EvalResultMaxBytes        uint32
    ReadFileMaxBytes          uint32
    EntitySubscriptionBuffer  uint32
    TraceSubscriptionBuffer   uint32
    TailDefaultWaitSeconds    uint32
    TailMaxWaitSeconds        uint32
}
```

- [ ] **Step 2: Write the failing handler tests**

Create or extend `internal/api/service_system_test.go`. Add three test cases:

```go
func TestSystemService_GetConfigDir(t *testing.T) {
    fake := &fakeSystemControl{configDir: "/var/lib/gohome"}
    s := newSystemService(fake)
    resp, err := s.GetConfigDir(context.Background(), connect.NewRequest(&systempb.GetConfigDirRequest{}))
    require.NoError(t, err)
    require.Equal(t, "/var/lib/gohome", resp.Msg.ConfigDir)
}

func TestSystemService_GetMCPConfig(t *testing.T) {
    fake := &fakeSystemControl{
        mcpConfig: api.MCPConfig{
            EvalResultMaxBytes:       65536,
            ReadFileMaxBytes:         1048576,
            EntitySubscriptionBuffer: 256,
            TraceSubscriptionBuffer:  1024,
            TailDefaultWaitSeconds:   0,
            TailMaxWaitSeconds:       60,
        },
    }
    s := newSystemService(fake)
    resp, err := s.GetMCPConfig(context.Background(), connect.NewRequest(&systempb.GetMCPConfigRequest{}))
    require.NoError(t, err)
    require.Equal(t, uint32(65536), resp.Msg.EvalResultMaxBytes)
    require.Equal(t, uint32(60), resp.Msg.TailMaxWaitSeconds)
}

func TestSystemService_RecordConfigFileEdit(t *testing.T) {
    fake := &fakeSystemControl{
        configDir:    "/var/lib/gohome",
        recordResult: 4242,
    }
    s := newSystemService(fake)

    ctx := auth.WithPrincipal(context.Background(), auth.Principal{ID: "system:local", Kind: "system"})
    resp, err := s.RecordConfigFileEdit(ctx, connect.NewRequest(&systempb.RecordConfigFileEditRequest{
        SessionId: "01HZTESTSESSION",
        Path:      "automations/lights.pkl",
        Sha256Hex: "abc123",
        SizeBytes: 512,
    }))
    require.NoError(t, err)
    require.Equal(t, uint64(4242), resp.Msg.EventCursor)
    require.Equal(t, "automations/lights.pkl", fake.lastRecord.path)
    require.Equal(t, "system:local", fake.lastRecord.principal.ID)
}

func TestSystemService_RecordConfigFileEdit_RejectsPathEscape(t *testing.T) {
    fake := &fakeSystemControl{
        configDir: "/var/lib/gohome",
        recordErr: api.ErrPathEscape,
    }
    s := newSystemService(fake)
    ctx := auth.WithPrincipal(context.Background(), auth.Principal{ID: "system:local"})
    _, err := s.RecordConfigFileEdit(ctx, connect.NewRequest(&systempb.RecordConfigFileEditRequest{
        Path: "../../etc/passwd",
    }))
    require.Error(t, err)
    var connectErr *connect.Error
    require.ErrorAs(t, err, &connectErr)
    require.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}
```

Add a fake to `internal/api/fakes_test.go`:

```go
type fakeSystemControl struct {
    configDir    string
    mcpConfig    api.MCPConfig
    recordResult uint64
    recordErr    error
    lastRecord   struct {
        principal auth.Principal
        path      string
    }
    // ... existing fake fields for Version/Health/Metrics/Diagnostics/CreateSnapshot ...
}

func (f *fakeSystemControl) ConfigDir(_ context.Context) (string, error) { return f.configDir, nil }
func (f *fakeSystemControl) MCPConfig(_ context.Context) (api.MCPConfig, error) { return f.mcpConfig, nil }
func (f *fakeSystemControl) RecordConfigFileEdit(_ context.Context, p auth.Principal, _ , path, _ string, _ uint32) (uint64, error) {
    f.lastRecord.principal = p
    f.lastRecord.path = path
    if f.recordErr != nil {
        return 0, f.recordErr
    }
    return f.recordResult, nil
}
```

- [ ] **Step 3: Run the tests to verify they fail with the expected error**

```bash
go test ./internal/api/... -run TestSystemService_Get -v
```

Expected: compile error on missing handlers OR runtime fail on missing dispatch. Either way, the failure is in the production code path — not in the test.

- [ ] **Step 4: Implement the three handlers**

Add to `internal/api/service_system.go`:

```go
func (s *systemService) GetConfigDir(ctx context.Context, _ *connect.Request[systempb.GetConfigDirRequest]) (*connect.Response[systempb.GetConfigDirResponse], error) {
    dir, err := s.deps.ConfigDir(ctx)
    if err != nil {
        return nil, mapErr(err, "system", "config_dir_unavailable")
    }
    return connect.NewResponse(&systempb.GetConfigDirResponse{ConfigDir: dir}), nil
}

func (s *systemService) GetMCPConfig(ctx context.Context, _ *connect.Request[systempb.GetMCPConfigRequest]) (*connect.Response[systempb.GetMCPConfigResponse], error) {
    cfg, err := s.deps.MCPConfig(ctx)
    if err != nil {
        return nil, mapErr(err, "system", "mcp_config_unavailable")
    }
    return connect.NewResponse(&systempb.GetMCPConfigResponse{
        EvalResultMaxBytes:       cfg.EvalResultMaxBytes,
        ReadFileMaxBytes:         cfg.ReadFileMaxBytes,
        EntitySubscriptionBuffer: cfg.EntitySubscriptionBuffer,
        TraceSubscriptionBuffer:  cfg.TraceSubscriptionBuffer,
        TailDefaultWaitSeconds:   cfg.TailDefaultWaitSeconds,
        TailMaxWaitSeconds:       cfg.TailMaxWaitSeconds,
    }), nil
}

func (s *systemService) RecordConfigFileEdit(ctx context.Context, req *connect.Request[systempb.RecordConfigFileEditRequest]) (*connect.Response[systempb.RecordConfigFileEditResponse], error) {
    p, ok := auth.PrincipalFromContext(ctx)
    if !ok {
        return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("no principal"))
    }
    cursor, err := s.deps.RecordConfigFileEdit(ctx, p, req.Msg.SessionId, req.Msg.Path, req.Msg.Sha256Hex, req.Msg.SizeBytes)
    if err != nil {
        return nil, mapErr(err, "system", "record_failed")
    }
    return connect.NewResponse(&systempb.RecordConfigFileEditResponse{EventCursor: cursor}), nil
}
```

`mapErr` is the C7 helper from `internal/api/errors.go` — it maps `api.ErrPathEscape` to `INVALID_ARGUMENT`/`path_escape`. Add that mapping if not already present:

```go
// internal/api/errors.go
var ErrPathEscape = errors.New("path escapes config dir")

// In mapErr's switch:
case errors.Is(err, ErrPathEscape):
    return connect.NewError(connect.CodeInvalidArgument, err) // detail.reason = "path_escape"
```

- [ ] **Step 5: Run the tests**

```bash
go test ./internal/api/... -run TestSystemService_ -v
```

Expected: all four pass.

- [ ] **Step 6: Wire the daemon-side `SystemControl` impl**

In `internal/daemon/system_control.go` (or wherever C7 implemented `SystemControl`), add the three methods. `ConfigDir` returns the value already stored in the daemon's resolved config struct. `MCPConfig` projects from the loaded Pkl (Task 5 puts the value in place; until then return zeros — but write the wiring now).

`RecordConfigFileEdit` does:

```go
func (s *systemControl) RecordConfigFileEdit(ctx context.Context, p auth.Principal, sessionID, path, sha256Hex string, sizeBytes uint32) (uint64, error) {
    abs := filepath.Join(s.configDir, path)
    rel, err := filepath.Rel(s.configDir, abs)
    if err != nil || strings.HasPrefix(rel, "..") || strings.Contains(rel, ".."+string(os.PathSeparator)) {
        return 0, api.ErrPathEscape
    }
    cursor, err := s.events.Append(ctx, eventstore.NewEvent(eventv1.Payload{Kind: &eventv1.Payload_ConfigFileEdited{
        ConfigFileEdited: &eventv1.ConfigFileEdited{
            PrincipalId: p.ID,
            SessionId:   sessionID,
            Path:        path,
            Sha256Hex:   sha256Hex,
            SizeBytes:   sizeBytes,
        },
    }}))
    if err != nil {
        return 0, fmt.Errorf("append: %w", err)
    }
    return cursor, nil
}
```

(The exact `eventstore.NewEvent` / append call shape comes from C1; match what existing daemon-side event emitters use.)

- [ ] **Step 7: Run all tests**

```bash
task test
```

Expected: full suite passes.

- [ ] **Step 8: Commit**

```bash
git add internal/api/service_system.go internal/api/service_system_test.go internal/api/fakes_test.go internal/api/deps.go internal/api/errors.go internal/daemon/
git commit -m "feat(c8): implement GetConfigDir, RecordConfigFileEdit, GetMCPConfig handlers"
```

---

## Task 5: Add `gohome.mcp` Pkl module

**Files:**
- Create: `internal/config/pkl/gohome/mcp.pkl`
- Modify: `internal/config/pkl/gohome/main.pkl` (or core.pkl — wherever C4/C7 placed top-level config)
- Modify: `internal/config/loader.go` (or equivalent — load mcp into the Go config struct)
- Modify: `internal/daemon/system_control.go` — return real values from `MCPConfig`

- [ ] **Step 1: Create the Pkl module**

`internal/config/pkl/gohome/mcp.pkl`:

```pkl
/// MCP server configuration. See docs/superpowers/specs/2026-04-24-c8-mcp-server-design.md §9.
@ModuleInfo { minPklVersion = "0.27.0" }
module gohome.mcp

class MCPConfig {
  /// Maximum bytes returned from gohome__eval_starlark before truncation.
  /// Hard cap; agents that need more should write a named script.
  evalResultMaxBytes: UInt = 65536            // 64 KiB

  /// Maximum file size readable via gohome__read_config_file.
  readFileMaxBytes: UInt = 1048576            // 1 MiB

  /// Per-subscription notifications/resources/updated buffer for entity URIs.
  entitySubscriptionBuffer: UInt = 256

  /// Same for trace URIs (higher-fidelity, larger buffer; overflow closes the resource).
  traceSubscriptionBuffer: UInt = 1024

  /// Default and maximum wait_seconds on gohome__tail_events.
  tailDefaultWaitSeconds: UInt = 0
  tailMaxWaitSeconds:     UInt = 60
}
```

- [ ] **Step 2: Reference it from the main config module**

In `main.pkl` (or wherever the daemon-level config is composed), add:

```pkl
import "gohome/mcp.pkl" as mcp

mcp: mcp.MCPConfig = new mcp.MCPConfig {}
```

- [ ] **Step 3: Add the Go-side struct**

If `internal/config/types.go` (or wherever Pkl-evaluated config lands as a Go struct) has a top-level `Config` struct, extend it:

```go
type Config struct {
    // ... existing fields ...
    MCP MCPConfig `pkl:"mcp"`
}

type MCPConfig struct {
    EvalResultMaxBytes        uint32 `pkl:"evalResultMaxBytes"`
    ReadFileMaxBytes          uint32 `pkl:"readFileMaxBytes"`
    EntitySubscriptionBuffer  uint32 `pkl:"entitySubscriptionBuffer"`
    TraceSubscriptionBuffer   uint32 `pkl:"traceSubscriptionBuffer"`
    TailDefaultWaitSeconds    uint32 `pkl:"tailDefaultWaitSeconds"`
    TailMaxWaitSeconds        uint32 `pkl:"tailMaxWaitSeconds"`
}
```

- [ ] **Step 4: Wire the daemon's `SystemControl.MCPConfig` to return the loaded values**

In `internal/daemon/system_control.go`:

```go
func (s *systemControl) MCPConfig(_ context.Context) (api.MCPConfig, error) {
    c := s.cfg.MCP
    return api.MCPConfig{
        EvalResultMaxBytes:       c.EvalResultMaxBytes,
        ReadFileMaxBytes:         c.ReadFileMaxBytes,
        EntitySubscriptionBuffer: c.EntitySubscriptionBuffer,
        TraceSubscriptionBuffer:  c.TraceSubscriptionBuffer,
        TailDefaultWaitSeconds:   c.TailDefaultWaitSeconds,
        TailMaxWaitSeconds:       c.TailMaxWaitSeconds,
    }, nil
}
```

- [ ] **Step 5: Add a Pkl-evaluation test**

Where existing Pkl-eval tests live (probably `internal/config/loader_test.go`), add:

```go
func TestLoad_MCPDefaults(t *testing.T) {
    cfg := loadFixture(t, "fixtures/minimal.pkl")
    require.Equal(t, uint32(65536), cfg.MCP.EvalResultMaxBytes)
    require.Equal(t, uint32(1048576), cfg.MCP.ReadFileMaxBytes)
    require.Equal(t, uint32(256), cfg.MCP.EntitySubscriptionBuffer)
    require.Equal(t, uint32(60), cfg.MCP.TailMaxWaitSeconds)
}

func TestLoad_MCPOverride(t *testing.T) {
    cfg := loadFixture(t, "fixtures/mcp_override.pkl")
    require.Equal(t, uint32(8192), cfg.MCP.EvalResultMaxBytes)
}
```

Create `internal/config/fixtures/mcp_override.pkl` overriding `evalResultMaxBytes`:

```pkl
amends "../pkl/gohome/main.pkl"

mcp { evalResultMaxBytes = 8192 }
```

- [ ] **Step 6: Run the tests**

```bash
go test ./internal/config/... -run TestLoad_MCP -v
```

Expected: both pass.

- [ ] **Step 7: Commit**

```bash
git add internal/config/pkl/gohome/mcp.pkl internal/config/pkl/gohome/main.pkl internal/config/types.go internal/config/loader_test.go internal/config/fixtures/mcp_override.pkl internal/daemon/system_control.go
git commit -m "feat(c8): add gohome.mcp Pkl module + daemon plumbing"
```

---

## Task 6: `x-gohome-source` server-side interceptor

**Files:**
- Create: `internal/api/source.go`
- Create: `internal/api/source_test.go`
- Modify: `internal/api/listener/interceptors.go`
- Modify: `internal/observability/metrics.go`

- [ ] **Step 1: Write the failing test for the context helpers**

`internal/api/source_test.go`:

```go
package api_test

import (
    "context"
    "net/http"
    "testing"

    "connectrpc.com/connect"
    "github.com/fynn-labs/gohome/internal/api"
    "github.com/stretchr/testify/require"
)

func TestSourceFromContext_Default(t *testing.T) {
    src, ok := api.SourceFromContext(context.Background())
    require.False(t, ok)
    require.Equal(t, "cli", src) // default
}

func TestSourceFromContext_Explicit(t *testing.T) {
    ctx := api.WithSource(context.Background(), "mcp")
    src, ok := api.SourceFromContext(ctx)
    require.True(t, ok)
    require.Equal(t, "mcp", src)
}

func TestSourceInterceptor_ReadsHeader(t *testing.T) {
    var observed string
    next := connect.UnaryFunc(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
        observed, _ = api.SourceFromContext(ctx)
        return nil, nil
    })

    interceptor := api.SourceInterceptor()
    h := connect.UnaryFunc(interceptor.WrapUnary(next))

    req := connect.NewRequest(&struct{}{})
    req.Header().Set("x-gohome-source", "mcp")
    _, _ = h(context.Background(), req)
    require.Equal(t, "mcp", observed)
}

func TestSourceInterceptor_DefaultCLI(t *testing.T) {
    var observed string
    next := connect.UnaryFunc(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
        observed, _ = api.SourceFromContext(ctx)
        return nil, nil
    })

    interceptor := api.SourceInterceptor()
    h := connect.UnaryFunc(interceptor.WrapUnary(next))

    req := connect.NewRequest(&struct{}{})
    _, _ = h(context.Background(), req) // no header set
    require.Equal(t, "cli", observed)
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/api/... -run TestSource -v
```

Expected: compile errors on missing `api.SourceFromContext`, `api.WithSource`, `api.SourceInterceptor`.

- [ ] **Step 3: Implement**

`internal/api/source.go`:

```go
package api

import (
    "context"

    "connectrpc.com/connect"
)

type sourceCtxKey struct{}

// WithSource attaches the request source ("cli" / "mcp" / "web") to ctx.
func WithSource(ctx context.Context, source string) context.Context {
    return context.WithValue(ctx, sourceCtxKey{}, source)
}

// SourceFromContext returns the source previously set via WithSource. The
// returned bool reports whether the source was explicitly set; callers can
// rely on the default "cli" string when ok is false.
func SourceFromContext(ctx context.Context) (string, bool) {
    if v, ok := ctx.Value(sourceCtxKey{}).(string); ok {
        return v, true
    }
    return "cli", false
}

// SourceInterceptor reads the x-gohome-source header (default "cli") and
// places it on the request context for downstream handlers and metric labels.
func SourceInterceptor() connect.Interceptor {
    return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
        return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
            source := req.Header().Get("x-gohome-source")
            if source == "" {
                source = "cli"
            }
            return next(WithSource(ctx, source), req)
        }
    })
}
```

The connect-go `Interceptor` interface also needs streaming wrappers — copy the same pattern from C7's request-id interceptor for `WrapStreamingClient` / `WrapStreamingHandler`. (If C7's request-id interceptor is in the same file in `internal/api/listener/interceptors.go`, follow its shape exactly.)

- [ ] **Step 4: Verify tests pass**

```bash
go test ./internal/api/... -run TestSource -v
```

Expected: all four pass.

- [ ] **Step 5: Install in the listener interceptor stack**

In `internal/api/listener/interceptors.go`, find the place C7 composes the interceptor chain and add `api.SourceInterceptor()` between `slog` and `metrics` (so the metric labels can read the source).

- [ ] **Step 6: Add the `source` label to `gohome_api_requests_total`**

In `internal/observability/metrics.go`, find the `gohome_api_requests_total` counter (or wherever C7 declared it — possibly in `internal/api/listener/`). Add `"source"` to the label set. Update the emission site to include `api.SourceFromContext(ctx)` as the source value.

Update any existing tests that assert on the metric to include the source label (default `"cli"`).

- [ ] **Step 7: Run the full API and observability test suites**

```bash
go test ./internal/api/... ./internal/observability/... -v
```

Expected: all green. Investigate any failures — likely tests that constructed metric expectations without the new label.

- [ ] **Step 8: Commit**

```bash
git add internal/api/source.go internal/api/source_test.go internal/api/listener/interceptors.go internal/observability/metrics.go
git commit -m "feat(c8): add x-gohome-source interceptor and metric label"
```

---

## Task 7: 64 KiB cap and `MCPEvalRequested` emission in `ScriptService.Eval`

**Files:**
- Modify: `internal/api/service_script.go`
- Modify: `internal/api/service_script_test.go`
- Modify: `internal/api/deps.go` (add an event-append dep if not already present for ScriptService)

- [ ] **Step 1: Confirm where the cap is read from**

The cap value lives in `MCPConfig.EvalResultMaxBytes` (set in Task 5). `ScriptService` already has a `SystemControl` accessor available through its deps OR can take a `MCPCapsProvider` mini-interface. Pick one — match the C7 pattern. For this plan we assume `ScriptService` gains a `mcpCaps func(ctx) (api.MCPConfig, error)` dep populated at daemon-construction time.

- [ ] **Step 2: Write the failing tests**

Add to `internal/api/service_script_test.go`:

```go
func TestScriptService_Eval_TruncatesAtCap_WhenSourceIsMCP(t *testing.T) {
    fakeRunner := &fakeScriptRunner{
        evalResult: strings.Repeat("x", 100_000), // 100 KiB
    }
    fakeEvents := &fakeEventAppender{}
    s := newScriptService(fakeRunner, fakeEvents, fixedMCPConfig{evalCap: 65536})

    ctx := api.WithSource(context.Background(), "mcp")
    ctx = auth.WithPrincipal(ctx, auth.Principal{ID: "system:local"})
    resp, err := s.Eval(ctx, connect.NewRequest(&scriptpb.EvalRequest{Source: "noop"}))
    require.NoError(t, err)
    require.True(t, resp.Msg.Truncated)
    require.LessOrEqual(t, len(resp.Msg.Result), 65536)
    require.Contains(t, resp.Msg.Result, "...[truncated; result was 100000 bytes]")

    // MCPEvalRequested emitted exactly once.
    require.Len(t, fakeEvents.appended, 1)
    payload := fakeEvents.appended[0].GetMcpEvalRequested()
    require.NotNil(t, payload)
    require.True(t, payload.Truncated)
    require.Equal(t, uint32(100_000), payload.ResultBytes)
}

func TestScriptService_Eval_NoCap_WhenSourceIsCLI(t *testing.T) {
    fakeRunner := &fakeScriptRunner{
        evalResult: strings.Repeat("x", 100_000),
    }
    fakeEvents := &fakeEventAppender{}
    s := newScriptService(fakeRunner, fakeEvents, fixedMCPConfig{evalCap: 65536})

    ctx := api.WithSource(context.Background(), "cli") // explicit
    resp, err := s.Eval(ctx, connect.NewRequest(&scriptpb.EvalRequest{Source: "noop"}))
    require.NoError(t, err)
    require.False(t, resp.Msg.Truncated)
    require.Equal(t, 100_000, len(resp.Msg.Result))
    require.Empty(t, fakeEvents.appended) // no audit event
}

func TestScriptService_Eval_EmitsAuditOnError_WhenSourceIsMCP(t *testing.T) {
    fakeRunner := &fakeScriptRunner{evalErr: errors.New("type error: int")}
    fakeEvents := &fakeEventAppender{}
    s := newScriptService(fakeRunner, fakeEvents, fixedMCPConfig{evalCap: 65536})
    ctx := api.WithSource(context.Background(), "mcp")
    ctx = auth.WithPrincipal(ctx, auth.Principal{ID: "system:local"})
    _, err := s.Eval(ctx, connect.NewRequest(&scriptpb.EvalRequest{Source: "broken"}))
    require.Error(t, err)
    require.Len(t, fakeEvents.appended, 1)
    payload := fakeEvents.appended[0].GetMcpEvalRequested()
    require.Equal(t, "type error: int", payload.Error)
}
```

Add the supporting fakes if not already present:

```go
type fakeEventAppender struct {
    appended []*eventv1.Payload
}
func (f *fakeEventAppender) Append(_ context.Context, p *eventv1.Payload) (uint64, error) {
    f.appended = append(f.appended, p)
    return uint64(len(f.appended)), nil
}

type fixedMCPConfig struct{ evalCap uint32 }
func (m fixedMCPConfig) MCPConfig(context.Context) (api.MCPConfig, error) {
    return api.MCPConfig{EvalResultMaxBytes: m.evalCap}, nil
}
```

- [ ] **Step 3: Run to verify failures**

```bash
go test ./internal/api/... -run TestScriptService_Eval -v
```

Expected: existing `Eval` test passes; the three new tests fail because the cap and audit logic don't exist yet.

- [ ] **Step 4: Implement the cap + audit**

Modify the `Eval` handler in `internal/api/service_script.go`. Replace the current handler with:

```go
func (s *scriptService) Eval(ctx context.Context, req *connect.Request[scriptpb.EvalRequest]) (*connect.Response[scriptpb.EvalResponse], error) {
    source, _ := api.SourceFromContext(ctx)
    fromMCP := source == "mcp"

    var cap uint32
    var sessionID string
    if fromMCP {
        cfg, err := s.mcpCaps.MCPConfig(ctx)
        if err == nil {
            cap = cfg.EvalResultMaxBytes
        }
        sessionID = req.Header().Get("x-gohome-mcp-session")
    }

    started := time.Now()
    result, runErr := s.runner.Eval(ctx, req.Msg.Source)
    duration := time.Since(started)

    fullBytes := uint32(len(result))
    truncated := false
    if fromMCP && cap > 0 && uint32(len(result)) > cap {
        marker := fmt.Sprintf("...[truncated; result was %d bytes]", fullBytes)
        keep := int(cap) - len(marker)
        if keep < 0 {
            keep = 0
        }
        result = result[:keep] + marker
        truncated = true
    }

    if fromMCP {
        principal, _ := auth.PrincipalFromContext(ctx)
        sum := sha256.Sum256([]byte(result))
        payload := &eventv1.Payload{Kind: &eventv1.Payload_McpEvalRequested{
            McpEvalRequested: &eventv1.MCPEvalRequested{
                PrincipalId:      principal.ID,
                SessionId:        sessionID,
                Source:           req.Msg.Source,
                ResultSha256Hex:  hex.EncodeToString(sum[:]),
                Truncated:        truncated,
                ResultBytes:      fullBytes,
                DurationMs:       uint32(duration.Milliseconds()),
                Error:            errString(runErr),
            },
        }}
        if _, appendErr := s.events.Append(ctx, payload); appendErr != nil {
            slog.WarnContext(ctx, "audit append failed", "error", appendErr)
            // Audit failure does not fail the user-facing call.
        }
    }

    if runErr != nil {
        return nil, mapErr(runErr, "script", "eval_failed")
    }
    return connect.NewResponse(&scriptpb.EvalResponse{
        Result:     result,
        DurationMs: uint32(duration.Milliseconds()),
        Truncated:  truncated,
    }), nil
}

func errString(err error) string {
    if err == nil {
        return ""
    }
    return err.Error()
}
```

If the existing `EvalResponse` proto does not have a `Truncated` field, add one in the next proto regeneration cycle — or, if C7 sized `Eval`'s response without it, add it to `proto/gohome/v1alpha1/script.proto` and regenerate first. The plan assumes C7 already included `Truncated bool` — if not, this is a one-line proto add.

- [ ] **Step 5: Run the tests**

```bash
go test ./internal/api/... -run TestScriptService_Eval -v
```

Expected: all four pass (1 existing + 3 new).

- [ ] **Step 6: Run the race detector against the script suite**

```bash
go test -race ./internal/api/... -run TestScriptService_ -v
```

Expected: clean (no data race; the sha256 + payload assembly is single-goroutine).

- [ ] **Step 7: Commit**

```bash
git add internal/api/service_script.go internal/api/service_script_test.go internal/api/deps.go internal/api/fakes_test.go proto/gohome/v1alpha1/script.proto gen/gohome/v1alpha1/
git commit -m "feat(c8): enforce 64 KiB cap and emit MCPEvalRequested when ScriptService.Eval source is mcp"
```

---

## Task 8: `gohome_mcp_*` metrics + tool/resource header interceptor

**Files:**
- Create: `internal/api/mcp_interceptor.go`
- Create: `internal/api/mcp_interceptor_test.go`
- Modify: `internal/observability/metrics.go`
- Modify: `internal/api/listener/interceptors.go` (install the new interceptor)

- [ ] **Step 1: Register the new metrics**

In `internal/observability/metrics.go`, add:

```go
var (
    MCPToolCallsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "gohome_mcp_tool_calls_total",
            Help: "Total MCP tool dispatches by tool and outcome.",
        },
        []string{"tool", "result"}, // result: ok | error | unimplemented
    )

    MCPToolCallDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "gohome_mcp_tool_call_duration_seconds",
            Help:    "Latency of MCP tool dispatches.",
            Buckets: prometheus.DefBuckets,
        },
        []string{"tool", "result"},
    )

    MCPResourceSubscriptionsActive = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "gohome_mcp_resource_subscriptions_active",
            Help: "Currently-open MCP resource subscriptions.",
        },
        []string{"kind"}, // entity | trace
    )

    MCPResourceUpdatesSent = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "gohome_mcp_resource_updates_sent_total",
            Help: "MCP notifications/resources/updated fired.",
        },
        []string{"kind"},
    )

    MCPResourceOverflowCloses = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "gohome_mcp_resource_overflow_closes_total",
            Help: "MCP subscriptions affected by buffer overflow.",
        },
        []string{"kind", "reason"}, // reason: coalesced | trace_overflow
    )

    MCPEvalStarlarkTruncated = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "gohome_mcp_eval_starlark_truncated_total",
            Help: "eval_starlark calls whose output exceeded the cap.",
        },
    )

    MCPConfigFileWrites = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "gohome_mcp_config_file_writes_total",
            Help: "filesystem-tool writes by extension and outcome.",
        },
        []string{"extension", "result"},
    )
)
```

(Adapt to the project's actual metric registration convention if it uses a different package than `promauto`.)

- [ ] **Step 2: Write the failing test for the interceptor**

`internal/api/mcp_interceptor_test.go`:

```go
package api_test

import (
    "context"
    "testing"

    "connectrpc.com/connect"
    "github.com/fynn-labs/gohome/internal/api"
    "github.com/fynn-labs/gohome/internal/observability"
    "github.com/prometheus/client_golang/prometheus/testutil"
    "github.com/stretchr/testify/require"
)

func TestMCPInterceptor_TagsToolCall(t *testing.T) {
    next := connect.UnaryFunc(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
        return connect.NewResponse(&struct{}{}), nil
    })
    interceptor := api.MCPInterceptor()
    h := interceptor.WrapUnary(next)

    req := connect.NewRequest(&struct{}{})
    req.Header().Set("x-gohome-source", "mcp")
    req.Header().Set("x-gohome-mcp-tool", "gohome__get_state")

    before := testutil.ToFloat64(observability.MCPToolCallsTotal.WithLabelValues("gohome__get_state", "ok"))
    _, err := h(api.WithSource(context.Background(), "mcp"), req)
    require.NoError(t, err)
    after := testutil.ToFloat64(observability.MCPToolCallsTotal.WithLabelValues("gohome__get_state", "ok"))
    require.Equal(t, 1.0, after-before)
}

func TestMCPInterceptor_NoMetricForCLISource(t *testing.T) {
    next := connect.UnaryFunc(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
        return connect.NewResponse(&struct{}{}), nil
    })
    interceptor := api.MCPInterceptor()
    h := interceptor.WrapUnary(next)
    req := connect.NewRequest(&struct{}{})
    // No headers set, so source defaults to cli; no MCP metric should fire.
    before := testutil.CollectAndCount(observability.MCPToolCallsTotal)
    _, err := h(api.WithSource(context.Background(), "cli"), req)
    require.NoError(t, err)
    after := testutil.CollectAndCount(observability.MCPToolCallsTotal)
    require.Equal(t, before, after)
}
```

- [ ] **Step 3: Run to verify failures**

```bash
go test ./internal/api/... -run TestMCPInterceptor -v
```

Expected: compile error on missing `api.MCPInterceptor`.

- [ ] **Step 4: Implement**

`internal/api/mcp_interceptor.go`:

```go
package api

import (
    "context"
    "time"

    "connectrpc.com/connect"
    "github.com/fynn-labs/gohome/internal/observability"
)

// MCPInterceptor extracts MCP-specific request labels (x-gohome-mcp-tool,
// x-gohome-mcp-resource) and emits gohome_mcp_* metrics. It is a no-op when
// the request source is not "mcp".
func MCPInterceptor() connect.Interceptor {
    return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
        return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
            source, _ := SourceFromContext(ctx)
            if source != "mcp" {
                return next(ctx, req)
            }
            tool := req.Header().Get("x-gohome-mcp-tool")
            // Resource header is ignored here for unary; resource flows go
            // through streaming handlers and emit metrics there.

            started := time.Now()
            resp, err := next(ctx, req)
            elapsed := time.Since(started).Seconds()

            if tool != "" {
                result := classify(err)
                observability.MCPToolCallsTotal.WithLabelValues(tool, result).Inc()
                observability.MCPToolCallDuration.WithLabelValues(tool, result).Observe(elapsed)
            }
            return resp, err
        }
    })
}

func classify(err error) string {
    if err == nil {
        return "ok"
    }
    var ce *connect.Error
    if errors.As(err, &ce) && ce.Code() == connect.CodeUnimplemented {
        return "unimplemented"
    }
    return "error"
}
```

- [ ] **Step 5: Verify tests pass**

```bash
go test ./internal/api/... -run TestMCPInterceptor -v
```

Expected: both pass.

- [ ] **Step 6: Install in the listener interceptor stack**

In `internal/api/listener/interceptors.go`, add `api.MCPInterceptor()` after `api.SourceInterceptor()` and after the existing metrics interceptor (so it observes outcomes after the metrics layer has labeled them but does its own MCP-specific tagging).

- [ ] **Step 7: Run the full API test suite**

```bash
go test ./internal/api/... -v
```

Expected: green.

- [ ] **Step 8: Commit**

```bash
git add internal/api/mcp_interceptor.go internal/api/mcp_interceptor_test.go internal/observability/metrics.go internal/api/listener/interceptors.go
git commit -m "feat(c8): register gohome_mcp_* metrics and the tool/resource interceptor"
```

---

## Task 9: `internal/mcp/fs/safepath.go` (path containment)

**Files:**
- Create: `internal/mcp/fs/safepath.go`
- Create: `internal/mcp/fs/safepath_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/mcp/fs/safepath_test.go`:

```go
package fs_test

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/fynn-labs/gohome/internal/mcp/fs"
    "github.com/stretchr/testify/require"
)

func TestResolve_OK(t *testing.T) {
    root := t.TempDir()
    sub := filepath.Join(root, "automations")
    require.NoError(t, os.Mkdir(sub, 0o755))
    require.NoError(t, os.WriteFile(filepath.Join(sub, "lights.pkl"), []byte("x = 1"), 0o644))

    got, err := fs.Resolve(root, "automations/lights.pkl")
    require.NoError(t, err)
    require.Equal(t, filepath.Join(sub, "lights.pkl"), got)
}

func TestResolve_RejectsParentTraversal(t *testing.T) {
    root := t.TempDir()
    _, err := fs.Resolve(root, "../etc/passwd")
    require.ErrorIs(t, err, fs.ErrPathEscape)
}

func TestResolve_RejectsAbsolutePath(t *testing.T) {
    root := t.TempDir()
    _, err := fs.Resolve(root, "/etc/passwd")
    require.ErrorIs(t, err, fs.ErrPathEscape)
}

func TestResolve_RejectsSymlinkEscape(t *testing.T) {
    root := t.TempDir()
    outside := t.TempDir()
    require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644))
    require.NoError(t, os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "link.txt")))

    _, err := fs.Resolve(root, "link.txt")
    require.ErrorIs(t, err, fs.ErrPathEscape)
}

func TestResolve_AllowsInternalSymlink(t *testing.T) {
    root := t.TempDir()
    require.NoError(t, os.WriteFile(filepath.Join(root, "real.pkl"), []byte("x = 1"), 0o644))
    require.NoError(t, os.Symlink(filepath.Join(root, "real.pkl"), filepath.Join(root, "alias.pkl")))

    got, err := fs.Resolve(root, "alias.pkl")
    require.NoError(t, err)
    require.Equal(t, filepath.Join(root, "real.pkl"), got)
}

func TestResolve_AllowsNonexistentTarget(t *testing.T) {
    // For write_config_file, the target may not exist yet.
    root := t.TempDir()
    got, err := fs.Resolve(root, "automations/new.pkl")
    require.NoError(t, err)
    require.Equal(t, filepath.Join(root, "automations", "new.pkl"), got)
}
```

- [ ] **Step 2: Run to verify failures**

```bash
go test ./internal/mcp/fs/... -v
```

Expected: package not found / type not defined.

- [ ] **Step 3: Implement**

`internal/mcp/fs/safepath.go`:

```go
// Package fs provides MCP filesystem-tool helpers: strict path containment
// and best-effort syntax validation for .pkl / .star files.
package fs

import (
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

// ErrPathEscape is returned when a requested path would resolve outside the
// supplied root, either through `..` traversal, an absolute path, or a
// symlink whose target escapes.
var ErrPathEscape = errors.New("path escapes config dir")

// Resolve joins root and rel, refuses absolute rel paths and ..-traversals,
// and follows symlinks (using filepath.EvalSymlinks on the longest existing
// prefix) to verify the final destination is still inside root. The returned
// path is always inside root.
//
// Resolve allows the target to not yet exist, so callers can use it for both
// reads and writes.
func Resolve(root, rel string) (string, error) {
    if filepath.IsAbs(rel) {
        return "", fmt.Errorf("%w: absolute path %q", ErrPathEscape, rel)
    }
    cleanRoot, err := filepath.Abs(root)
    if err != nil {
        return "", fmt.Errorf("resolve root: %w", err)
    }
    cleanRoot, err = filepath.EvalSymlinks(cleanRoot)
    if err != nil {
        return "", fmt.Errorf("eval symlinks on root %q: %w", root, err)
    }

    joined := filepath.Join(cleanRoot, rel)
    cleaned := filepath.Clean(joined)
    if !inside(cleanRoot, cleaned) {
        return "", fmt.Errorf("%w: %q resolves to %q", ErrPathEscape, rel, cleaned)
    }

    // If the target exists, verify symlink escape can't smuggle us out.
    if resolved, lerr := filepath.EvalSymlinks(cleaned); lerr == nil {
        if !inside(cleanRoot, resolved) {
            return "", fmt.Errorf("%w: %q resolves through symlinks to %q", ErrPathEscape, rel, resolved)
        }
        return resolved, nil
    } else if !errors.Is(lerr, os.ErrNotExist) {
        return "", fmt.Errorf("eval symlinks on %q: %w", cleaned, lerr)
    }

    // Target doesn't exist yet (write path) — verify the longest existing
    // prefix is still inside root.
    prefix := cleaned
    for {
        prefix = filepath.Dir(prefix)
        if prefix == cleanRoot || prefix == filepath.Dir(cleanRoot) {
            break
        }
        if resolved, lerr := filepath.EvalSymlinks(prefix); lerr == nil {
            if !inside(cleanRoot, resolved) {
                return "", fmt.Errorf("%w: parent %q escapes root", ErrPathEscape, prefix)
            }
            break
        }
    }
    return cleaned, nil
}

func inside(root, p string) bool {
    rel, err := filepath.Rel(root, p)
    if err != nil {
        return false
    }
    if rel == "." {
        return true
    }
    if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
        return false
    }
    return true
}
```

- [ ] **Step 4: Verify tests pass**

```bash
go test ./internal/mcp/fs/... -v
```

Expected: all six pass.

- [ ] **Step 5: Run with the race detector**

```bash
go test -race ./internal/mcp/fs/... -v
```

Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add internal/mcp/fs/safepath.go internal/mcp/fs/safepath_test.go
git commit -m "feat(c8): add safepath path-containment helper for filesystem tools"
```

---

## Task 10: `internal/mcp/fs/syntax.go` (best-effort `.pkl` and `.star` parse)

**Files:**
- Create: `internal/mcp/fs/syntax.go`
- Create: `internal/mcp/fs/syntax_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/mcp/fs/syntax_test.go`:

```go
package fs_test

import (
    "testing"

    "github.com/fynn-labs/gohome/internal/mcp/fs"
    "github.com/stretchr/testify/require"
)

func TestCheckSyntax_PklOK(t *testing.T) {
    err := fs.CheckSyntax("config.pkl", []byte(`x = 1`))
    require.NoError(t, err)
}

func TestCheckSyntax_PklBroken(t *testing.T) {
    err := fs.CheckSyntax("config.pkl", []byte(`x =`))
    require.Error(t, err)
    var se *fs.SyntaxError
    require.ErrorAs(t, err, &se)
    require.Equal(t, "config.pkl", se.Path)
    require.NotZero(t, se.Line)
}

func TestCheckSyntax_StarlarkOK(t *testing.T) {
    err := fs.CheckSyntax("script.star", []byte(`def f(): return 1`))
    require.NoError(t, err)
}

func TestCheckSyntax_StarlarkBroken(t *testing.T) {
    err := fs.CheckSyntax("script.star", []byte(`def f(`))
    require.Error(t, err)
    var se *fs.SyntaxError
    require.ErrorAs(t, err, &se)
    require.Equal(t, "script.star", se.Path)
    require.NotZero(t, se.Line)
}

func TestCheckSyntax_UnsupportedExtension(t *testing.T) {
    err := fs.CheckSyntax("README.md", []byte(`# hi`))
    require.ErrorIs(t, err, fs.ErrUnsupportedExtension)
}
```

- [ ] **Step 2: Run to verify failures**

```bash
go test ./internal/mcp/fs/... -run TestCheckSyntax -v
```

Expected: not defined.

- [ ] **Step 3: Implement**

`internal/mcp/fs/syntax.go`:

```go
package fs

import (
    "errors"
    "fmt"
    "path/filepath"
    "strings"

    "go.starlark.net/syntax"
    pklparser "github.com/fynn-labs/gohome/internal/config/pklparser" // C4-introduced parser; or substitute available API
)

var ErrUnsupportedExtension = errors.New("unsupported extension; expected .pkl or .star")

// SyntaxError carries the offending path plus 1-based line/column for the
// MCP error envelope.
type SyntaxError struct {
    Path    string
    Line    int
    Column  int
    Message string
}

func (e *SyntaxError) Error() string {
    return fmt.Sprintf("%s:%d:%d: %s", e.Path, e.Line, e.Column, e.Message)
}

// CheckSyntax does a best-effort parse of the file content based on its
// extension. .pkl files use the C4 Pkl parser; .star files use the
// go.starlark.net parser. Any other extension returns ErrUnsupportedExtension.
func CheckSyntax(path string, content []byte) error {
    switch strings.ToLower(filepath.Ext(path)) {
    case ".pkl":
        return checkPkl(path, content)
    case ".star":
        return checkStarlark(path, content)
    default:
        return ErrUnsupportedExtension
    }
}

func checkStarlark(path string, content []byte) error {
    _, err := syntax.Parse(path, content, 0)
    if err == nil {
        return nil
    }
    if se, ok := err.(syntax.Error); ok {
        return &SyntaxError{
            Path:    path,
            Line:    int(se.Pos.Line),
            Column:  int(se.Pos.Col),
            Message: se.Msg,
        }
    }
    return &SyntaxError{Path: path, Line: 1, Message: err.Error()}
}

func checkPkl(path string, content []byte) error {
    pos, err := pklparser.Parse(path, content)
    if err == nil {
        return nil
    }
    return &SyntaxError{
        Path:    path,
        Line:    pos.Line,
        Column:  pos.Col,
        Message: err.Error(),
    }
}
```

If C4 did not produce a Go-callable Pkl parser, replace `checkPkl` with an out-of-process `pkl eval --noop` style invocation OR a syntax-only check using the Java Pkl CLI (if available). Document the chosen path inline. **Best-effort** is the contract — false negatives (rejecting valid Pkl) are bugs, but a missing Pkl parse layer is acceptable as long as the tool surfaces a clear "Pkl syntax check unavailable" error rather than silently accepting.

- [ ] **Step 4: Verify tests pass**

```bash
go test ./internal/mcp/fs/... -run TestCheckSyntax -v
```

Expected: all five pass.

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/fs/syntax.go internal/mcp/fs/syntax_test.go
git commit -m "feat(c8): best-effort .pkl and .star syntax check for filesystem tools"
```

---

## Task 11: `internal/mcp/client.go` (Connect client over UDS)

**Files:**
- Create: `internal/mcp/client.go`
- Create: `internal/mcp/client_test.go`

- [ ] **Step 1: Write the failing test**

`internal/mcp/client_test.go`:

```go
package mcp_test

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/fynn-labs/gohome/internal/mcp"
    "github.com/stretchr/testify/require"
)

func TestClient_SetsSourceAndSessionHeaders(t *testing.T) {
    var observed http.Header
    srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
        observed = r.Header
    }))
    defer srv.Close()

    c, err := mcp.NewClient(mcp.ClientOptions{
        EndpointURL: srv.URL,
        SessionID:   "01HZTESTSESSION",
    })
    require.NoError(t, err)

    // Direct hit via the http.Client we wrapped (Connect calls would set the
    // headers identically through the interceptor).
    req, _ := http.NewRequestWithContext(context.Background(), "POST", srv.URL+"/test", nil)
    _, _ = c.HTTPClient().Do(c.AnnotateRequest(req, "gohome__get_state", ""))
    require.Equal(t, "mcp", observed.Get("x-gohome-source"))
    require.Equal(t, "01HZTESTSESSION", observed.Get("x-gohome-mcp-session"))
    require.Equal(t, "gohome__get_state", observed.Get("x-gohome-mcp-tool"))
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/mcp/... -run TestClient -v
```

Expected: package not found.

- [ ] **Step 3: Implement**

`internal/mcp/client.go`:

```go
// Package mcp implements the gohome MCP server (stdio transport).
package mcp

import (
    "context"
    "fmt"
    "net"
    "net/http"
    "net/url"
    "strings"

    "connectrpc.com/connect"
    systemconnect "github.com/fynn-labs/gohome/gen/gohome/v1alpha1/v1alpha1connect"
    // ... other connect-go service client packages from C7 ...
)

// ClientOptions configures the per-process Connect client.
type ClientOptions struct {
    // EndpointURL is the daemon endpoint. unix:// schemes use a UDS-aware
    // dialer; http(s):// schemes go through the default transport.
    EndpointURL string
    // SessionID is a ULID minted at subprocess startup; placed in
    // x-gohome-mcp-session on every outgoing call.
    SessionID string
}

// Client is the MCP server's daemon-side connector. It owns the http.Client
// and exposes typed Connect-go service clients via accessor methods.
type Client struct {
    httpClient *http.Client
    baseURL    string
    sessionID  string

    System    systemconnect.SystemServiceClient
    Entity    entityconnect.EntityServiceClient
    Event     eventconnect.EventServiceClient
    Script    scriptconnect.ScriptServiceClient
    Config    configconnect.ConfigServiceClient
    Automation automationconnect.AutomationServiceClient
    // ... etc per C7's connect packages ...
}

func NewClient(opts ClientOptions) (*Client, error) {
    u, err := url.Parse(opts.EndpointURL)
    if err != nil {
        return nil, fmt.Errorf("parse endpoint: %w", err)
    }
    httpc := &http.Client{Transport: buildTransport(u)}
    base := opts.EndpointURL
    if u.Scheme == "unix" {
        base = "http://gohomed" // host header doesn't matter for UDS; the dialer ignores
    }
    interceptors := connect.WithInterceptors(headerInterceptor(opts.SessionID))
    c := &Client{
        httpClient: httpc,
        baseURL:    base,
        sessionID:  opts.SessionID,
        System:     systemconnect.NewSystemServiceClient(httpc, base, interceptors),
        Entity:     entityconnect.NewEntityServiceClient(httpc, base, interceptors),
        Event:      eventconnect.NewEventServiceClient(httpc, base, interceptors),
        Script:     scriptconnect.NewScriptServiceClient(httpc, base, interceptors),
        Config:     configconnect.NewConfigServiceClient(httpc, base, interceptors),
        Automation: automationconnect.NewAutomationServiceClient(httpc, base, interceptors),
    }
    return c, nil
}

func buildTransport(u *url.URL) http.RoundTripper {
    if u.Scheme == "unix" {
        socket := strings.TrimPrefix(u.Path, "/") // url.Parse leaves /@data/...; trim
        if socket == "" {
            socket = u.Host + u.Path
        }
        return &http.Transport{
            DialContext: func(ctx context.Context, _ , _ string) (net.Conn, error) {
                d := &net.Dialer{}
                return d.DialContext(ctx, "unix", socket)
            },
        }
    }
    return http.DefaultTransport
}

// headerInterceptor sets x-gohome-source: mcp and x-gohome-mcp-session on
// every outgoing call. Per-tool / per-resource headers are set inside each
// tool/resource handler via AnnotateRequest below.
func headerInterceptor(sessionID string) connect.UnaryInterceptorFunc {
    return func(next connect.UnaryFunc) connect.UnaryFunc {
        return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
            req.Header().Set("x-gohome-source", "mcp")
            req.Header().Set("x-gohome-mcp-session", sessionID)
            return next(ctx, req)
        }
    }
}

// HTTPClient and AnnotateRequest are exposed for tests; tools use
// SetToolHeaders/SetResourceHeaders below.
func (c *Client) HTTPClient() *http.Client { return c.httpClient }

func (c *Client) AnnotateRequest(req *http.Request, tool, resource string) *http.Request {
    req.Header.Set("x-gohome-source", "mcp")
    req.Header.Set("x-gohome-mcp-session", c.sessionID)
    if tool != "" {
        req.Header.Set("x-gohome-mcp-tool", tool)
    }
    if resource != "" {
        req.Header.Set("x-gohome-mcp-resource", resource)
    }
    return req
}

// SetToolHeader returns a connect.CallOption that adds the tool header to a
// single Connect call. Used inside tool handlers.
func SetToolHeader(name string) connect.CallOption {
    return connect.WithHeader("x-gohome-mcp-tool", name)
}

// SetResourceHeader returns a connect.CallOption that adds the resource header.
func SetResourceHeader(uri string) connect.CallOption {
    return connect.WithHeader("x-gohome-mcp-resource", uri)
}
```

The exact streaming-interceptor shape is omitted for brevity — copy the C7 streaming pattern (the `WrapStreamingClient` half of the interceptor must also set the same headers).

- [ ] **Step 4: Run the test**

```bash
go test ./internal/mcp/... -run TestClient -v
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/client.go internal/mcp/client_test.go
git commit -m "feat(c8): MCP-side Connect client with UDS dialer + source/session headers"
```

---

## Task 12: `internal/mcp/errors.go` (Connect → MCP error mapping)

**Files:**
- Create: `internal/mcp/errors.go`
- Create: `internal/mcp/errors_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/mcp/errors_test.go`:

```go
package mcp_test

import (
    "encoding/json"
    "errors"
    "testing"

    "connectrpc.com/connect"
    "github.com/fynn-labs/gohome/internal/mcp"
    errorpb "github.com/fynn-labs/gohome/gen/gohome/error/v1alpha1"
    "github.com/stretchr/testify/require"
    "google.golang.org/protobuf/types/known/anypb"
)

func TestToMCPError_PreservesReason(t *testing.T) {
    detail, _ := connect.NewErrorDetail(&errorpb.ErrorDetail{
        Reason: "entity_not_found",
        Domain: "registry",
        Metadata: map[string]string{"entity_id": "light.foo"},
        RequestId: "01HZ...",
    })
    cerr := connect.NewError(connect.CodeNotFound, errors.New("not found"))
    cerr.AddDetail(detail)

    envelope := mcp.ToMCPErrorEnvelope(cerr)
    require.Equal(t, "entity_not_found", envelope.Reason)
    require.Equal(t, "light.foo", envelope.Metadata["entity_id"])
    require.Equal(t, "01HZ...", envelope.RequestID)
}

func TestToMCPError_CodeWithoutDetail(t *testing.T) {
    cerr := connect.NewError(connect.CodeUnimplemented, errors.New("scenes not yet"))
    envelope := mcp.ToMCPErrorEnvelope(cerr)
    require.Equal(t, "unimplemented", envelope.Reason)
}

func TestToMCPError_PlainGoError(t *testing.T) {
    envelope := mcp.ToMCPErrorEnvelope(errors.New("oops"))
    require.Equal(t, "internal", envelope.Reason)
    require.Contains(t, envelope.Message, "oops")
}

func TestEnvelope_JSONShape(t *testing.T) {
    e := mcp.MCPErrorEnvelope{
        Reason:   "x",
        Message:  "y",
        Metadata: map[string]string{"a": "b"},
    }
    b, err := json.Marshal(e)
    require.NoError(t, err)
    require.JSONEq(t, `{"reason":"x","message":"y","metadata":{"a":"b"},"request_id":"","correlation_id":""}`, string(b))
}
```

- [ ] **Step 2: Run failures**

```bash
go test ./internal/mcp/... -run TestToMCPError -v
go test ./internal/mcp/... -run TestEnvelope -v
```

- [ ] **Step 3: Implement**

`internal/mcp/errors.go`:

```go
package mcp

import (
    "errors"

    "connectrpc.com/connect"
    errorpb "github.com/fynn-labs/gohome/gen/gohome/error/v1alpha1"
)

// MCPErrorEnvelope is the JSON shape returned in MCP tool errors.
type MCPErrorEnvelope struct {
    Reason        string            `json:"reason"`
    Message       string            `json:"message"`
    Metadata      map[string]string `json:"metadata"`
    RequestID     string            `json:"request_id"`
    CorrelationID string            `json:"correlation_id"`
}

// ToMCPErrorEnvelope converts a Go error (typically a connect.Error from a
// daemon RPC, but tolerant of any error) into an MCP tool error envelope.
func ToMCPErrorEnvelope(err error) MCPErrorEnvelope {
    var ce *connect.Error
    if !errors.As(err, &ce) {
        return MCPErrorEnvelope{
            Reason:  "internal",
            Message: err.Error(),
        }
    }

    env := MCPErrorEnvelope{
        Reason:  reasonFromCode(ce.Code()),
        Message: ce.Message(),
        Metadata: map[string]string{},
    }

    for _, d := range ce.Details() {
        m, derr := d.Value()
        if derr != nil {
            continue
        }
        if detail, ok := m.(*errorpb.ErrorDetail); ok {
            if detail.Reason != "" {
                env.Reason = detail.Reason
            }
            for k, v := range detail.Metadata {
                env.Metadata[k] = v
            }
            env.RequestID = detail.RequestId
            env.CorrelationID = detail.CorrelationId
            break
        }
    }
    return env
}

func reasonFromCode(c connect.Code) string {
    switch c {
    case connect.CodeInvalidArgument:
        return "invalid_argument"
    case connect.CodeNotFound:
        return "not_found"
    case connect.CodeFailedPrecondition:
        return "failed_precondition"
    case connect.CodePermissionDenied:
        return "forbidden"
    case connect.CodeUnauthenticated:
        return "unauthenticated"
    case connect.CodeResourceExhausted:
        return "resource_exhausted"
    case connect.CodeDeadlineExceeded:
        return "deadline_exceeded"
    case connect.CodeUnimplemented:
        return "unimplemented"
    case connect.CodeUnavailable:
        return "unavailable"
    case connect.CodeInternal:
        return "internal"
    default:
        return "internal"
    }
}
```

- [ ] **Step 4: Verify tests pass**

```bash
go test ./internal/mcp/... -run TestToMCPError -v
go test ./internal/mcp/... -run TestEnvelope -v
```

Expected: green.

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/errors.go internal/mcp/errors_test.go
git commit -m "feat(c8): Connect → MCP error mapping with ErrorDetail preservation"
```

---

## Task 13: `internal/mcp/actions.go` (action catalog stub)

**Files:**
- Create: `internal/mcp/actions.go`
- Create: `internal/mcp/actions_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/mcp/actions_test.go`:

```go
package mcp_test

import (
    "testing"

    "github.com/fynn-labs/gohome/internal/mcp"
    "github.com/stretchr/testify/require"
)

func TestToolActions_AllRegisteredToolsHaveActions(t *testing.T) {
    expected := []string{
        "gohome__get_state",
        "gohome__list_entities",
        "gohome__call_capability",
        "gohome__query_events",
        "gohome__tail_events",
        "gohome__apply_scene",
        "gohome__run_script",
        "gohome__validate_config",
        "gohome__apply_config",
        "gohome__eval_starlark",
        "gohome__read_config_file",
        "gohome__write_config_file",
    }
    for _, name := range expected {
        action, ok := mcp.ToolActions[name]
        require.True(t, ok, "missing action for tool %q", name)
        require.Equal(t, "MCP", action.Service)
        require.NotEmpty(t, action.Verb)
    }
}

func TestResourceActions(t *testing.T) {
    a, ok := mcp.ResourceActions["gohome://entities/"]
    require.True(t, ok)
    require.Equal(t, "read", a.Verb)

    a, ok = mcp.ResourceActions["gohome://automations/"]
    require.True(t, ok)
    require.Equal(t, "read", a.Verb)
}

func TestToolActions_VerbDistribution(t *testing.T) {
    counts := map[string]int{}
    for _, a := range mcp.ToolActions {
        counts[a.Verb]++
    }
    require.Equal(t, 6, counts["read"])  // get_state, list_entities, query_events, tail_events, validate_config, read_config_file
    require.Equal(t, 4, counts["call"])  // call_capability, apply_scene, run_script, eval_starlark
    require.Equal(t, 2, counts["admin"]) // apply_config, write_config_file
}
```

- [ ] **Step 2: Run failures**

```bash
go test ./internal/mcp/... -run TestToolActions -v
go test ./internal/mcp/... -run TestResourceActions -v
```

- [ ] **Step 3: Implement**

`internal/mcp/actions.go`:

```go
package mcp

import "github.com/fynn-labs/gohome/internal/auth"

// ToolActions maps an MCP tool name to the auth.Action used by the
// pre-dispatch Authorize call. The C7 AllowAllAuthorizer makes the result a
// no-op; C9 swaps in the policy-backed Authorizer without changing this table.
var ToolActions = map[string]auth.Action{
    "gohome__get_state":         {Service: "MCP", Method: "get_state",         Verb: "read"},
    "gohome__list_entities":     {Service: "MCP", Method: "list_entities",     Verb: "read"},
    "gohome__call_capability":   {Service: "MCP", Method: "call_capability",   Verb: "call"},
    "gohome__query_events":      {Service: "MCP", Method: "query_events",      Verb: "read"},
    "gohome__tail_events":       {Service: "MCP", Method: "tail_events",       Verb: "read"},
    "gohome__apply_scene":       {Service: "MCP", Method: "apply_scene",       Verb: "call"},
    "gohome__run_script":        {Service: "MCP", Method: "run_script",        Verb: "call"},
    "gohome__validate_config":   {Service: "MCP", Method: "validate_config",   Verb: "read"},
    "gohome__apply_config":      {Service: "MCP", Method: "apply_config",      Verb: "admin"},
    "gohome__eval_starlark":     {Service: "MCP", Method: "eval_starlark",    Verb: "call"},
    "gohome__read_config_file":  {Service: "MCP", Method: "read_config_file",  Verb: "read"},
    "gohome__write_config_file": {Service: "MCP", Method: "write_config_file", Verb: "admin"},
}

var ResourceActions = map[string]auth.Action{
    "gohome://entities/":    {Service: "MCP", Method: "subscribe_entities", Verb: "read"},
    "gohome://automations/": {Service: "MCP", Method: "trace_automation",   Verb: "read"},
}
```

- [ ] **Step 4: Verify tests pass**

```bash
go test ./internal/mcp/... -run TestToolActions -v
go test ./internal/mcp/... -run TestResourceActions -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/actions.go internal/mcp/actions_test.go
git commit -m "feat(c8): action catalog stub for tools and resources"
```

---

## Task 14: `internal/mcp/audit/recorder.go`

**Files:**
- Create: `internal/mcp/audit/recorder.go`
- Create: `internal/mcp/audit/recorder_test.go`

- [ ] **Step 1: Write the failing test**

`internal/mcp/audit/recorder_test.go`:

```go
package audit_test

import (
    "context"
    "testing"

    systempb "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
    "github.com/fynn-labs/gohome/internal/mcp/audit"
    "github.com/stretchr/testify/require"
    "connectrpc.com/connect"
)

type fakeSystemClient struct {
    lastReq *systempb.RecordConfigFileEditRequest
    cursor  uint64
    err     error
}

func (f *fakeSystemClient) RecordConfigFileEdit(_ context.Context, req *connect.Request[systempb.RecordConfigFileEditRequest]) (*connect.Response[systempb.RecordConfigFileEditResponse], error) {
    f.lastReq = req.Msg
    if f.err != nil {
        return nil, f.err
    }
    return connect.NewResponse(&systempb.RecordConfigFileEditResponse{EventCursor: f.cursor}), nil
}

func TestRecorder_RecordsConfigFileEdit(t *testing.T) {
    fake := &fakeSystemClient{cursor: 9876}
    r := audit.NewRecorder(fake)
    cursor, err := r.ConfigFileEdited(context.Background(), audit.ConfigFileEditEvent{
        SessionID: "01HZ",
        Path:      "automations/lights.pkl",
        Sha256Hex: "abc",
        SizeBytes: 512,
    })
    require.NoError(t, err)
    require.Equal(t, uint64(9876), cursor)
    require.Equal(t, "01HZ", fake.lastReq.SessionId)
    require.Equal(t, "automations/lights.pkl", fake.lastReq.Path)
}
```

- [ ] **Step 2: Run failure, then implement**

```bash
go test ./internal/mcp/audit/... -v
```

`internal/mcp/audit/recorder.go`:

```go
// Package audit emits MCP-source audit events through the daemon.
//
// Audit emission goes via the daemon (SystemService.RecordConfigFileEdit and
// ScriptService.Eval) rather than from the subprocess, so the event store has
// a single writer. This package is a thin wrapper around the SystemService
// client.
package audit

import (
    "context"

    "connectrpc.com/connect"
    systempb "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
)

type SystemClient interface {
    RecordConfigFileEdit(context.Context, *connect.Request[systempb.RecordConfigFileEditRequest]) (*connect.Response[systempb.RecordConfigFileEditResponse], error)
}

type Recorder struct {
    sys SystemClient
}

type ConfigFileEditEvent struct {
    SessionID string
    Path      string
    Sha256Hex string
    SizeBytes uint32
}

func NewRecorder(sys SystemClient) *Recorder { return &Recorder{sys: sys} }

func (r *Recorder) ConfigFileEdited(ctx context.Context, ev ConfigFileEditEvent) (uint64, error) {
    resp, err := r.sys.RecordConfigFileEdit(ctx, connect.NewRequest(&systempb.RecordConfigFileEditRequest{
        SessionId: ev.SessionID,
        Path:      ev.Path,
        Sha256Hex: ev.Sha256Hex,
        SizeBytes: ev.SizeBytes,
    }))
    if err != nil {
        return 0, err
    }
    return resp.Msg.EventCursor, nil
}
```

- [ ] **Step 3: Verify**

```bash
go test ./internal/mcp/audit/... -v
```

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/audit/recorder.go internal/mcp/audit/recorder_test.go
git commit -m "feat(c8): audit recorder facade over SystemService.RecordConfigFileEdit"
```

---

## Task 15: `internal/mcp/server.go` skeleton (no tools yet)

**Files:**
- Create: `internal/mcp/server.go`
- Create: `internal/mcp/deps.go`

- [ ] **Step 1: Define the dep set**

`internal/mcp/deps.go`:

```go
package mcp

import (
    "context"
    "io"

    "github.com/fynn-labs/gohome/internal/mcp/audit"
)

// Deps is the closure of inputs the MCP server needs at construction time.
type Deps struct {
    // Client is the daemon-side Connect client (UDS by default).
    Client *Client
    // Audit emits ConfigFileEdited via the daemon.
    Audit *audit.Recorder
    // ConfigDir is the resolved daemon config directory; supplied via the
    // GetConfigDir RPC at server startup.
    ConfigDir string
    // MCPCaps is the daemon's MCP config snapshot (caps, buffer sizes).
    MCPCaps MCPCaps
    // Version is the gohome version embedded in mcp.ServerInfo.
    Version string
    // Stdin / Stdout / Stderr — set in production from os.Std{in,out,err};
    // tests inject pipes.
    Stdin  io.Reader
    Stdout io.Writer
    Stderr io.Writer
}

// MCPCaps mirrors api.MCPConfig but lives in the mcp package so tests don't
// need to import internal/api just for the struct shape.
type MCPCaps struct {
    EvalResultMaxBytes        uint32
    ReadFileMaxBytes          uint32
    EntitySubscriptionBuffer  uint32
    TraceSubscriptionBuffer   uint32
    TailDefaultWaitSeconds    uint32
    TailMaxWaitSeconds        uint32
}
```

- [ ] **Step 2: Server skeleton**

`internal/mcp/server.go`:

```go
package mcp

import (
    "context"
    "fmt"

    sdk "github.com/modelcontextprotocol/go-sdk"
)

// Run blocks running the stdio MCP server until ctx is canceled or stdin
// reaches EOF. Tools and resources must already be registered on deps.
func Run(ctx context.Context, deps Deps) error {
    server := sdk.NewServer(sdk.ServerInfo{
        Name:    "gohome",
        Version: deps.Version,
    })

    // Tool and resource registration is done in subsequent tasks.

    transport := sdk.Stdio(deps.Stdin, deps.Stdout, deps.Stderr)
    if err := server.Serve(ctx, transport); err != nil {
        return fmt.Errorf("mcp serve: %w", err)
    }
    return nil
}
```

The actual SDK API may name things differently (`sdk.NewServerStdio`, etc.) — the implementer should consult the SDK README and adjust both this task and Task 1's verification step. The shape (constructor + transport + Serve) is what's load-bearing.

- [ ] **Step 3: Skeleton sanity check**

`internal/mcp/server_skel_test.go`:

```go
package mcp_test

import (
    "context"
    "io"
    "testing"
    "time"

    "github.com/fynn-labs/gohome/internal/mcp"
    "github.com/stretchr/testify/require"
)

func TestRun_ExitsOnStdinEOF(t *testing.T) {
    pr, pw := io.Pipe()
    _ = pw.Close() // immediate EOF

    done := make(chan error, 1)
    go func() {
        done <- mcp.Run(context.Background(), mcp.Deps{
            Stdin:   pr,
            Stdout:  io.Discard,
            Stderr:  io.Discard,
            Version: "test",
        })
    }()
    select {
    case err := <-done:
        require.NoError(t, err)
    case <-time.After(2 * time.Second):
        t.Fatal("Run did not exit on stdin EOF")
    }
}
```

- [ ] **Step 4: Verify skeleton compiles and the EOF test passes**

```bash
go test ./internal/mcp/... -run TestRun_ExitsOnStdinEOF -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/deps.go internal/mcp/server_skel_test.go
git commit -m "feat(c8): MCP server skeleton (stdio loop, no tools yet)"
```

---

## Task 16: Tool group — entities (`get_state`, `list_entities`, `call_capability`)

**Files:**
- Create: `internal/mcp/tools/tool.go`
- Create: `internal/mcp/tools/entities.go`
- Create: `internal/mcp/tools/entities_test.go`

- [ ] **Step 1: Common tool registration helpers**

`internal/mcp/tools/tool.go`:

```go
// Package tools registers MCP tool handlers against the SDK server.
package tools

import (
    "context"

    sdk "github.com/modelcontextprotocol/go-sdk"
    "github.com/fynn-labs/gohome/internal/auth"
    "github.com/fynn-labs/gohome/internal/mcp"
)

// Deps is what every tool group needs to register handlers.
type Deps struct {
    Server *sdk.Server
    MCP    mcp.Deps
    Auth   auth.Authorizer // C7 stub allows all
}

// Register registers all tool groups on the supplied server.
func Register(d Deps) {
    registerEntities(d)
    registerEvents(d)
    registerScenes(d)
    registerScripts(d)
    registerConfig(d)
    registerFiles(d)
}

// authorize calls the supplied authorizer for the named tool. Returns nil to
// proceed; an MCP-shaped error to refuse.
func authorize(ctx context.Context, d Deps, toolName string, target auth.Target) error {
    action, ok := mcp.ToolActions[toolName]
    if !ok {
        return nil // safety: an unknown tool can't be authorized; treat as deny in C9 update
    }
    p, _ := auth.PrincipalFromContext(ctx)
    return d.Auth.Authorize(ctx, p, action, target)
}
```

- [ ] **Step 2: Entity tools — failing tests first**

`internal/mcp/tools/entities_test.go`:

```go
package tools_test

import (
    "context"
    "encoding/json"
    "testing"

    "connectrpc.com/connect"
    entitypb "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
    "github.com/fynn-labs/gohome/internal/mcp/tools"
    "github.com/stretchr/testify/require"
)

func TestGetState_HappyPath(t *testing.T) {
    fake := &fakeEntityClient{
        getResp: &entitypb.GetEntityResponse{
            Entity: &entitypb.Entity{
                Id:           "light.living_room",
                Type:         "light",
                FriendlyName: "Living Room Ceiling",
            },
        },
    }
    h := tools.GetStateHandler(fake)
    out, err := h(context.Background(), tools.GetStateInput{EntityID: "light.living_room"})
    require.NoError(t, err)
    var parsed map[string]any
    require.NoError(t, json.Unmarshal(out, &parsed))
    require.Equal(t, "light.living_room", parsed["id"])
    require.Equal(t, "Living Room Ceiling", parsed["name"]) // renamed from friendly_name
}

func TestGetState_NotFound(t *testing.T) {
    fake := &fakeEntityClient{getErr: connect.NewError(connect.CodeNotFound, errors.New("missing"))}
    h := tools.GetStateHandler(fake)
    _, err := h(context.Background(), tools.GetStateInput{EntityID: "light.foo"})
    require.Error(t, err)
    var te *tools.ToolError
    require.ErrorAs(t, err, &te)
    require.Equal(t, "not_found", te.Reason)
}

func TestListEntities_PassesSelectorAndPagination(t *testing.T) {
    fake := &fakeEntityClient{
        listResp: &entitypb.ListEntitiesResponse{
            Entities: []*entitypb.Entity{{Id: "light.a"}},
            Page:     &commonpb.PageResponse{NextPageToken: "next-token"},
        },
    }
    h := tools.ListEntitiesHandler(fake)
    out, err := h(context.Background(), tools.ListEntitiesInput{
        Areas:  []string{"kitchen"},
        Limit:  50,
        Cursor: "prev-token",
    })
    require.NoError(t, err)
    var parsed map[string]any
    require.NoError(t, json.Unmarshal(out, &parsed))
    require.Equal(t, "next-token", parsed["next_cursor"])
    require.Equal(t, []string{"kitchen"}, fake.listReq.Selector.Areas)
    require.Equal(t, uint32(50), fake.listReq.Page.PageSize)
    require.Equal(t, "prev-token", fake.listReq.Page.PageToken)
}

func TestCallCapability_HappyPath(t *testing.T) {
    fake := &fakeEntityClient{
        callResp: &entitypb.CallCapabilityResponse{Accepted: true, CommandId: "cmd-1"},
    }
    h := tools.CallCapabilityHandler(fake)
    out, err := h(context.Background(), tools.CallCapabilityInput{
        EntityID:   "light.kitchen",
        Capability: "turn_on",
        Params:     map[string]any{"brightness": 80},
    })
    require.NoError(t, err)
    var parsed map[string]any
    require.NoError(t, json.Unmarshal(out, &parsed))
    require.Equal(t, true, parsed["accepted"])
    require.Equal(t, "cmd-1", parsed["command_id"])
}
```

Add fakes (in a `fakes_test.go` shared across tool tests):

```go
type fakeEntityClient struct {
    getResp  *entitypb.GetEntityResponse
    getErr   error
    listResp *entitypb.ListEntitiesResponse
    listReq  *entitypb.ListEntitiesRequest
    listErr  error
    callResp *entitypb.CallCapabilityResponse
    callErr  error
}

// implement entityconnect.EntityServiceClient methods, capturing inputs ...
```

- [ ] **Step 3: Run failures**

```bash
go test ./internal/mcp/tools/... -run TestGetState -v
go test ./internal/mcp/tools/... -run TestListEntities -v
go test ./internal/mcp/tools/... -run TestCallCapability -v
```

- [ ] **Step 4: Implement**

`internal/mcp/tools/entities.go`:

```go
package tools

import (
    "context"
    "encoding/json"

    "connectrpc.com/connect"
    "google.golang.org/protobuf/encoding/protojson"

    entitypb "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
    entityconnect "github.com/fynn-labs/gohome/gen/gohome/v1alpha1/v1alpha1connect"
    commonpb "github.com/fynn-labs/gohome/gen/gohome/v1alpha1"
    "github.com/fynn-labs/gohome/internal/mcp"
)

// --- input types ---

type GetStateInput struct {
    EntityID string `json:"entity_id" jsonschema:"required" jsonschema_description:"Dotted entity ID, e.g. 'light.living_room'"`
}

type ListEntitiesInput struct {
    Areas    []string `json:"areas,omitempty"`
    Zones    []string `json:"zones,omitempty"`
    Classes  []string `json:"classes,omitempty"`
    DeviceID string   `json:"device_id,omitempty"`
    Limit    int      `json:"limit,omitempty"  jsonschema:"minimum=1,maximum=1000"`
    Cursor   string   `json:"cursor,omitempty"`
}

type CallCapabilityInput struct {
    EntityID   string         `json:"entity_id" jsonschema:"required"`
    Capability string         `json:"capability" jsonschema:"required"`
    Params     map[string]any `json:"params,omitempty"`
}

// ToolError is a thin helper used in tests to read the reason out of an
// error without going through the MCP envelope JSON. Production tool
// dispatch uses mcp.ToMCPErrorEnvelope at the SDK boundary.
type ToolError struct {
    Reason  string
    Message string
    Cause   error
}

func (e *ToolError) Error() string { return e.Reason + ": " + e.Message }
func (e *ToolError) Unwrap() error { return e.Cause }

// --- handlers ---

func GetStateHandler(c entityconnect.EntityServiceClient) func(context.Context, GetStateInput) ([]byte, error) {
    return func(ctx context.Context, in GetStateInput) ([]byte, error) {
        resp, err := c.Get(ctx, connect.NewRequest(&entitypb.GetEntityRequest{Id: in.EntityID}), mcp.SetToolHeader("gohome__get_state"))
        if err != nil {
            return nil, toToolError(err)
        }
        return marshalEntity(resp.Msg.Entity)
    }
}

func ListEntitiesHandler(c entityconnect.EntityServiceClient) func(context.Context, ListEntitiesInput) ([]byte, error) {
    return func(ctx context.Context, in ListEntitiesInput) ([]byte, error) {
        page := &commonpb.PageRequest{PageToken: in.Cursor}
        if in.Limit > 0 {
            page.PageSize = uint32(in.Limit)
        }
        sel := &commonpb.EntitySelector{
            Areas: in.Areas, Zones: in.Zones, Classes: in.Classes,
        }
        if in.DeviceID != "" {
            sel.DeviceIds = []string{in.DeviceID}
        }
        resp, err := c.List(ctx, connect.NewRequest(&entitypb.ListEntitiesRequest{
            Page: page, Selector: sel,
        }), mcp.SetToolHeader("gohome__list_entities"))
        if err != nil {
            return nil, toToolError(err)
        }
        out := struct {
            Entities   []json.RawMessage `json:"entities"`
            NextCursor string            `json:"next_cursor"`
        }{NextCursor: resp.Msg.Page.GetNextPageToken()}
        for _, e := range resp.Msg.Entities {
            b, _ := marshalEntity(e)
            out.Entities = append(out.Entities, b)
        }
        return json.Marshal(out)
    }
}

func CallCapabilityHandler(c entityconnect.EntityServiceClient) func(context.Context, CallCapabilityInput) ([]byte, error) {
    return func(ctx context.Context, in CallCapabilityInput) ([]byte, error) {
        params, err := structFromMap(in.Params)
        if err != nil {
            return nil, &ToolError{Reason: "invalid_argument", Message: err.Error(), Cause: err}
        }
        resp, err := c.CallCapability(ctx, connect.NewRequest(&entitypb.CallCapabilityRequest{
            EntityId:   in.EntityID,
            Capability: in.Capability,
            Params:     params,
        }), mcp.SetToolHeader("gohome__call_capability"))
        if err != nil {
            return nil, toToolError(err)
        }
        return json.Marshal(map[string]any{
            "accepted":   resp.Msg.Accepted,
            "command_id": resp.Msg.CommandId,
        })
    }
}

// marshalEntity protojson-serializes an Entity, then renames friendly_name → name.
func marshalEntity(e *entitypb.Entity) ([]byte, error) {
    b, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(e)
    if err != nil {
        return nil, err
    }
    var m map[string]any
    if err := json.Unmarshal(b, &m); err != nil {
        return nil, err
    }
    if v, ok := m["friendly_name"]; ok {
        m["name"] = v
        delete(m, "friendly_name")
    }
    return json.Marshal(m)
}

func toToolError(err error) error {
    env := mcp.ToMCPErrorEnvelope(err)
    return &ToolError{Reason: env.Reason, Message: env.Message, Cause: err}
}

func structFromMap(m map[string]any) (*structpb.Struct, error) {
    if m == nil {
        return nil, nil
    }
    return structpb.NewStruct(m)
}

// registerEntities binds the three handlers to the SDK server. SDK API names
// here are illustrative — adapt to the official-SDK shape.
func registerEntities(d Deps) {
    sdk.AddTool(d.Server, "gohome__get_state", "Read current state of one entity",
        GetStateHandler(d.MCP.Client.Entity))
    sdk.AddTool(d.Server, "gohome__list_entities", "Browse entities with optional filters",
        ListEntitiesHandler(d.MCP.Client.Entity))
    sdk.AddTool(d.Server, "gohome__call_capability", "Invoke a capability on one entity",
        CallCapabilityHandler(d.MCP.Client.Entity))
}
```

- [ ] **Step 5: Verify tests**

```bash
go test ./internal/mcp/tools/... -run "TestGetState|TestListEntities|TestCallCapability" -v
```

- [ ] **Step 6: Wire into `Register` (already done in tool.go) and adjust `internal/mcp/server.go`** to call `tools.Register(tools.Deps{Server: server, MCP: deps, Auth: deps.Authorizer})` between server construction and `server.Serve`.

- [ ] **Step 7: Commit**

```bash
git add internal/mcp/tools/tool.go internal/mcp/tools/entities.go internal/mcp/tools/entities_test.go internal/mcp/tools/fakes_test.go internal/mcp/server.go internal/mcp/deps.go
git commit -m "feat(c8): MCP entity tools (get_state, list_entities, call_capability)"
```

---

## Task 17: Tool group — events (`query_events`, `tail_events`)

**Files:**
- Create: `internal/mcp/tools/events.go`
- Create: `internal/mcp/tools/events_test.go`

- [ ] **Step 1: Failing tests** for `query_events` (passes filter through; output shape) and `tail_events` (opens stream, reads up to MaxEvents within WaitSeconds, returns slice + next_cursor, drops heartbeats). Keep deadline tests fast — synthetic stream that yields N events then closes; another that yields slowly so WaitSeconds bounds the call.

- [ ] **Step 2: Implement** following the same pattern as Task 16: hand-written input struct → Connect call → protojson result. For `tail_events`, the implementation opens a `c.Tail(ctx, ...)` server-stream and uses a `time.After(time.Duration(in.WaitSeconds) * time.Second)` timer plus an event-count check to terminate. Heartbeats (the `Heartbeat` oneof case in `TailEventsResponse`) are skipped silently. Always close the stream on return (defer).

```go
type TailEventsInput struct {
    Kinds        []string `json:"kinds,omitempty"`
    EntityPrefix string   `json:"entity_prefix,omitempty"`
    Sources      []string `json:"sources,omitempty"`
    FromCursor   uint64   `json:"from_cursor,omitempty"`
    MaxEvents    int      `json:"max_events,omitempty" jsonschema:"minimum=1,maximum=1000"`
    WaitSeconds  int      `json:"wait_seconds,omitempty" jsonschema:"minimum=0,maximum=60"`
}
```

The MaxEvents default is 100 (applied if zero); WaitSeconds default is `MCPCaps.TailDefaultWaitSeconds`; both are bounded by `MCPCaps.TailMaxWaitSeconds`.

- [ ] **Step 3: Verify, commit**

```bash
go test ./internal/mcp/tools/... -run "TestQueryEvents|TestTailEvents" -v
git add internal/mcp/tools/events.go internal/mcp/tools/events_test.go
git commit -m "feat(c8): MCP event tools (query_events, tail_events)"
```

---

## Task 18: Tool group — scenes (`apply_scene` UNIMPLEMENTED passthrough)

**Files:**
- Create: `internal/mcp/tools/scenes.go`
- Create: `internal/mcp/tools/scenes_test.go`

- [ ] **Step 1: Failing test**

```go
func TestApplyScene_Unimplemented(t *testing.T) {
    fake := &fakeSceneClient{applyErr: connect.NewError(connect.CodeUnimplemented, errors.New("not yet"))}
    h := tools.ApplySceneHandler(fake)
    _, err := h(context.Background(), tools.ApplySceneInput{Slug: "movie_night"})
    require.Error(t, err)
    var te *tools.ToolError
    require.ErrorAs(t, err, &te)
    require.Equal(t, "unimplemented", te.Reason)
}
```

- [ ] **Step 2: Implement**

```go
type ApplySceneInput struct {
    Slug string `json:"slug" jsonschema:"required" jsonschema_description:"Scene slug"`
}

func ApplySceneHandler(c sceneconnect.SceneServiceClient) func(context.Context, ApplySceneInput) ([]byte, error) {
    return func(ctx context.Context, in ApplySceneInput) ([]byte, error) {
        _, err := c.Apply(ctx, connect.NewRequest(&scenepb.ApplySceneRequest{Slug: in.Slug}), mcp.SetToolHeader("gohome__apply_scene"))
        if err != nil {
            return nil, toToolError(err)
        }
        return json.Marshal(map[string]any{"applied": true})
    }
}
```

The handler's tool description (registered against the SDK) explicitly notes "Currently UNIMPLEMENTED — Scene service spec is in flight."

- [ ] **Step 3: Verify, commit**

```bash
go test ./internal/mcp/tools/... -run TestApplyScene -v
git add internal/mcp/tools/scenes.go internal/mcp/tools/scenes_test.go
git commit -m "feat(c8): MCP apply_scene tool (UNIMPLEMENTED passthrough)"
```

---

## Task 19: Tool group — scripts (`run_script`, `eval_starlark`)

**Files:**
- Create: `internal/mcp/tools/scripts.go`
- Create: `internal/mcp/tools/scripts_test.go`

- [ ] **Step 1: Failing tests**

For `run_script`: passes Name/Args/Timeout to `ScriptService.Run`; returns `{run_id, result, duration_ms}`.

For `eval_starlark`: passes Source to `ScriptService.Eval`; returns `{result, duration_ms, truncated}`. The cap is enforced **server-side** in Task 7 — the tool just propagates the `Truncated` field. A test asserts that when the daemon returns `Truncated: true` the JSON output includes `"truncated": true`.

```go
func TestEvalStarlark_PropagatesTruncation(t *testing.T) {
    fake := &fakeScriptClient{
        evalResp: &scriptpb.EvalResponse{
            Result:     "x...[truncated; result was 100000 bytes]",
            DurationMs: 12,
            Truncated:  true,
        },
    }
    h := tools.EvalStarlarkHandler(fake)
    out, err := h(context.Background(), tools.EvalStarlarkInput{Source: "noop"})
    require.NoError(t, err)
    var parsed map[string]any
    require.NoError(t, json.Unmarshal(out, &parsed))
    require.Equal(t, true, parsed["truncated"])
}
```

- [ ] **Step 2: Implement** following the same template. Tool descriptions on the SDK side must explicitly state "Read-only stdlib only (state, now, log, repr). 30s wall-clock and 10M-step limits. Output capped at 64 KiB by default; oversized output is truncated with a marker." for `eval_starlark`.

- [ ] **Step 3: Verify, commit**

```bash
go test ./internal/mcp/tools/... -run "TestRunScript|TestEvalStarlark" -v
git add internal/mcp/tools/scripts.go internal/mcp/tools/scripts_test.go
git commit -m "feat(c8): MCP script tools (run_script, eval_starlark)"
```

---

## Task 20: Tool group — config (`validate_config`, `apply_config`)

**Files:**
- Create: `internal/mcp/tools/config.go`
- Create: `internal/mcp/tools/config_test.go`

- [ ] **Step 1: Failing tests**

For `validate_config`: passes the `[]byte` Pkl bundle through; returns `{valid, diff, errors}`.

For `apply_config`: passes bundle + message + dry_run + strict; returns `{applied, diff, applied_at}`. A test asserts that `dry_run: true` is forwarded faithfully.

```go
func TestApplyConfig_ForwardsDryRun(t *testing.T) {
    fake := &fakeConfigClient{}
    h := tools.ApplyConfigHandler(fake)
    _, err := h(context.Background(), tools.ApplyConfigInput{
        PklBundle: []byte("dummy"),
        Message:   "test",
        DryRun:    true,
    })
    require.NoError(t, err)
    require.True(t, fake.lastApply.DryRun)
}
```

- [ ] **Step 2: Implement** with the same template.

- [ ] **Step 3: Verify, commit**

```bash
go test ./internal/mcp/tools/... -run "TestValidateConfig|TestApplyConfig" -v
git add internal/mcp/tools/config.go internal/mcp/tools/config_test.go
git commit -m "feat(c8): MCP config tools (validate_config, apply_config)"
```

---

## Task 21: Tool group — files (`read_config_file`, `write_config_file`)

**Files:**
- Create: `internal/mcp/tools/files.go`
- Create: `internal/mcp/tools/files_test.go`

- [ ] **Step 1: Failing tests**

For `read_config_file`:
- Reads a file inside config dir → returns content + sha256.
- File outside config dir → `path_escape` error, no read.
- File >1 MiB → `file_too_large` error.
- Non-UTF-8 file → `not_utf8` error.
- Directory path → `not_a_regular_file` error.

For `write_config_file`:
- Writes a `.pkl` with valid syntax → file present, audit Recorder called once.
- Writes a `.star` with valid syntax → ditto.
- Writes a `.pkl` with broken syntax → `syntax_error` error, file NOT written.
- Writes a `.txt` → `unsupported_extension` error, file NOT written.
- Writes a path that escapes config dir → `path_escape` error, no Recorder call.
- Writes succeed atomically (simulate failure between fsync and rename — no `.tmp` file left).

`internal/mcp/tools/files_test.go`:

```go
func TestWriteConfigFile_HappyPath(t *testing.T) {
    root := t.TempDir()
    fakeAudit := &fakeAuditRecorder{}
    h := tools.WriteConfigFileHandler(tools.FilesDeps{
        ConfigDir: root,
        Audit:     fakeAudit,
        Caps:      mcp.MCPCaps{ReadFileMaxBytes: 1024 * 1024},
        SessionID: "01HZ",
    })
    out, err := h(context.Background(), tools.WriteConfigFileInput{
        Path:    "automations/lights.pkl",
        Content: "x = 1\n",
    })
    require.NoError(t, err)
    require.FileExists(t, filepath.Join(root, "automations/lights.pkl"))
    require.Len(t, fakeAudit.calls, 1)

    var parsed map[string]any
    require.NoError(t, json.Unmarshal(out, &parsed))
    require.Equal(t, "automations/lights.pkl", parsed["path"])
    require.NotEmpty(t, parsed["sha256_hex"])
}

func TestWriteConfigFile_RejectsPathEscape(t *testing.T) {
    root := t.TempDir()
    fakeAudit := &fakeAuditRecorder{}
    h := tools.WriteConfigFileHandler(tools.FilesDeps{ConfigDir: root, Audit: fakeAudit})
    _, err := h(context.Background(), tools.WriteConfigFileInput{Path: "../escape.pkl", Content: "x"})
    require.Error(t, err)
    var te *tools.ToolError
    require.ErrorAs(t, err, &te)
    require.Equal(t, "path_escape", te.Reason)
    require.Empty(t, fakeAudit.calls)
}

func TestWriteConfigFile_RejectsBadExtension(t *testing.T) {
    root := t.TempDir()
    fakeAudit := &fakeAuditRecorder{}
    h := tools.WriteConfigFileHandler(tools.FilesDeps{ConfigDir: root, Audit: fakeAudit})
    _, err := h(context.Background(), tools.WriteConfigFileInput{Path: "notes.txt", Content: "x"})
    require.Error(t, err)
    var te *tools.ToolError
    require.ErrorAs(t, err, &te)
    require.Equal(t, "unsupported_extension", te.Reason)
    require.Empty(t, fakeAudit.calls)
}

func TestWriteConfigFile_RejectsBadSyntax(t *testing.T) {
    root := t.TempDir()
    fakeAudit := &fakeAuditRecorder{}
    h := tools.WriteConfigFileHandler(tools.FilesDeps{ConfigDir: root, Audit: fakeAudit})
    _, err := h(context.Background(), tools.WriteConfigFileInput{Path: "broken.pkl", Content: "x ="})
    require.Error(t, err)
    var te *tools.ToolError
    require.ErrorAs(t, err, &te)
    require.Equal(t, "syntax_error", te.Reason)
    require.NoFileExists(t, filepath.Join(root, "broken.pkl"))
}
```

For `read_config_file`, table-driven test with the five cases above.

- [ ] **Step 2: Implement**

`internal/mcp/tools/files.go`:

```go
package tools

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "io/fs"
    "os"
    "path/filepath"
    "unicode/utf8"

    "github.com/fynn-labs/gohome/internal/mcp"
    "github.com/fynn-labs/gohome/internal/mcp/audit"
    mcpfs "github.com/fynn-labs/gohome/internal/mcp/fs"
)

type ReadConfigFileInput struct {
    Path string `json:"path" jsonschema:"required"`
}

type WriteConfigFileInput struct {
    Path    string `json:"path"    jsonschema:"required"`
    Content string `json:"content" jsonschema:"required"`
}

type FilesDeps struct {
    ConfigDir string
    Audit     audit.Recorder // interface; fakeAuditRecorder satisfies for tests
    Caps      mcp.MCPCaps
    SessionID string
}

func ReadConfigFileHandler(d FilesDeps) func(context.Context, ReadConfigFileInput) ([]byte, error) {
    return func(ctx context.Context, in ReadConfigFileInput) ([]byte, error) {
        abs, err := mcpfs.Resolve(d.ConfigDir, in.Path)
        if err != nil {
            return nil, &ToolError{Reason: "path_escape", Message: err.Error(), Cause: err}
        }
        info, err := os.Stat(abs)
        if err != nil {
            if errors.Is(err, fs.ErrNotExist) {
                return nil, &ToolError{Reason: "file_not_found", Message: in.Path}
            }
            return nil, &ToolError{Reason: "internal", Message: err.Error()}
        }
        if !info.Mode().IsRegular() {
            return nil, &ToolError{Reason: "not_a_regular_file", Message: in.Path}
        }
        if d.Caps.ReadFileMaxBytes > 0 && uint32(info.Size()) > d.Caps.ReadFileMaxBytes {
            return nil, &ToolError{Reason: "file_too_large", Message: fmt.Sprintf("%d > %d", info.Size(), d.Caps.ReadFileMaxBytes)}
        }
        b, err := os.ReadFile(abs)
        if err != nil {
            return nil, &ToolError{Reason: "internal", Message: err.Error()}
        }
        if !utf8.Valid(b) {
            return nil, &ToolError{Reason: "not_utf8", Message: in.Path}
        }
        sum := sha256.Sum256(b)
        return json.Marshal(map[string]any{
            "path":       in.Path,
            "content":    string(b),
            "size_bytes": info.Size(),
            "sha256_hex": hex.EncodeToString(sum[:]),
        })
    }
}

func WriteConfigFileHandler(d FilesDeps) func(context.Context, WriteConfigFileInput) ([]byte, error) {
    return func(ctx context.Context, in WriteConfigFileInput) ([]byte, error) {
        abs, err := mcpfs.Resolve(d.ConfigDir, in.Path)
        if err != nil {
            return nil, &ToolError{Reason: "path_escape", Message: err.Error()}
        }
        if err := mcpfs.CheckSyntax(in.Path, []byte(in.Content)); err != nil {
            if errors.Is(err, mcpfs.ErrUnsupportedExtension) {
                return nil, &ToolError{Reason: "unsupported_extension", Message: in.Path}
            }
            var se *mcpfs.SyntaxError
            if errors.As(err, &se) {
                return nil, &ToolError{Reason: "syntax_error", Message: se.Error()}
            }
            return nil, &ToolError{Reason: "internal", Message: err.Error()}
        }
        if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
            return nil, &ToolError{Reason: "internal", Message: err.Error()}
        }
        if err := atomicWrite(abs, []byte(in.Content)); err != nil {
            return nil, &ToolError{Reason: "internal", Message: err.Error()}
        }
        sum := sha256.Sum256([]byte(in.Content))
        sumHex := hex.EncodeToString(sum[:])
        if d.Audit != nil {
            if _, err := d.Audit.ConfigFileEdited(ctx, audit.ConfigFileEditEvent{
                SessionID: d.SessionID,
                Path:      in.Path,
                Sha256Hex: sumHex,
                SizeBytes: uint32(len(in.Content)),
            }); err != nil {
                // Audit failure on write is logged but does not undo the write.
                slog.WarnContext(ctx, "audit recorder failed", "error", err)
            }
        }
        return json.Marshal(map[string]any{
            "path":       in.Path,
            "sha256_hex": sumHex,
            "size_bytes": len(in.Content),
        })
    }
}

func atomicWrite(target string, data []byte) error {
    tmp := target + ".tmp." + randSuffix()
    f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
    if err != nil {
        return err
    }
    if _, err := f.Write(data); err != nil {
        _ = f.Close()
        _ = os.Remove(tmp)
        return err
    }
    if err := f.Sync(); err != nil {
        _ = f.Close()
        _ = os.Remove(tmp)
        return err
    }
    if err := f.Close(); err != nil {
        _ = os.Remove(tmp)
        return err
    }
    return os.Rename(tmp, target)
}

func randSuffix() string {
    var b [4]byte
    _, _ = rand.Read(b[:])
    return hex.EncodeToString(b[:])
}
```

The `fakeAuditRecorder` in the tests satisfies an interface that `audit.Recorder` should implement (extract one if needed, e.g. `audit.ConfigFileEditedRecorder`).

- [ ] **Step 3: Verify**

```bash
go test ./internal/mcp/tools/... -run "TestReadConfigFile|TestWriteConfigFile" -v
```

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/tools/files.go internal/mcp/tools/files_test.go
git commit -m "feat(c8): MCP filesystem tools (read_config_file, write_config_file)"
```

---

## Task 22: Resource — entities (subscribe with coalescing backpressure)

**Files:**
- Create: `internal/mcp/resources/resource.go`
- Create: `internal/mcp/resources/entities.go`
- Create: `internal/mcp/resources/entities_test.go`

- [ ] **Step 1: Common resource registration helpers** in `resource.go` — typed wrappers for `sdk.AddResource`, `sdk.AddResourceTemplate`, and a per-session `subscriptions` map keyed by `(sessionID, uri)` with a `Notify(uri, payload)` method that fires `notifications/resources/updated` when called.

- [ ] **Step 2: Failing tests for entity resource**

```go
func TestEntityResource_ReadSingle(t *testing.T) {
    fake := &fakeEntityClient{
        getResp: &entitypb.GetEntityResponse{
            Entity: &entitypb.Entity{Id: "light.x", FriendlyName: "X"},
        },
    }
    r := resources.NewEntityResource(fake)
    body, err := r.Read(context.Background(), "gohome://entities/light.x")
    require.NoError(t, err)
    var parsed map[string]any
    require.NoError(t, json.Unmarshal(body, &parsed))
    require.Equal(t, "light.x", parsed["id"])
}

func TestEntityResource_Subscribe_FiresUpdates(t *testing.T) {
    fake := newFakeEntityStream()
    r := resources.NewEntityResource(fake)
    fired := 0
    notifier := resources.NotifierFunc(func(uri string) { fired++ })
    sub, err := r.Subscribe(context.Background(), "gohome://entities/light.x", notifier, mcp.MCPCaps{EntitySubscriptionBuffer: 8})
    require.NoError(t, err)
    defer sub.Close()
    fake.send(&entitypb.SubscribeEntitiesResponse{ /* state changed event */ })
    fake.send(&entitypb.SubscribeEntitiesResponse{ /* heartbeat — should be silent */ HeartBeat: ...})
    time.Sleep(50 * time.Millisecond)
    require.Equal(t, 1, fired) // heartbeat doesn't count
}

func TestEntityResource_Subscribe_BackpressureCoalesces(t *testing.T) {
    fake := newFakeEntityStream()
    r := resources.NewEntityResource(fake)
    fired := 0
    notifier := resources.NotifierFunc(func(_ string) { fired++; time.Sleep(10 * time.Millisecond) }) // slow
    sub, err := r.Subscribe(context.Background(), "gohome://entities/light.x", notifier, mcp.MCPCaps{EntitySubscriptionBuffer: 2})
    require.NoError(t, err)
    defer sub.Close()
    for i := 0; i < 100; i++ {
        fake.send(&entitypb.SubscribeEntitiesResponse{ /* state change */ })
    }
    time.Sleep(200 * time.Millisecond)
    require.Less(t, fired, 100, "should have coalesced")
    require.Greater(t, observability.MCPResourceOverflowCloses.WithLabelValues("entity", "coalesced").Gauge(), 0.0)
}
```

- [ ] **Step 3: Implement** the resource. The subscribe loop runs in its own goroutine; the buffered channel is drained on every tick of the notifier. On overflow, the channel is non-blocking-replaced with a single "snapshot pending" sentinel (coalesce). Increment `MCPResourceOverflowCloses{kind=entity, reason=coalesced}` once per coalesce window. Maintain `MCPResourceSubscriptionsActive{kind=entity}` gauge increment/decrement on Subscribe/Close.

For the URI parsing, accept both `gohome://entities/{id}` and `gohome://entities?selector=<base64>` forms.

- [ ] **Step 4: Verify**

```bash
go test ./internal/mcp/resources/... -run TestEntityResource -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/resources/resource.go internal/mcp/resources/entities.go internal/mcp/resources/entities_test.go
git commit -m "feat(c8): MCP entity resource with subscription + coalescing backpressure"
```

---

## Task 23: Resource — automation traces (subscribe with overflow-close)

**Files:**
- Create: `internal/mcp/resources/traces.go`
- Create: `internal/mcp/resources/traces_test.go`

- [ ] **Step 1: Failing tests** following the pattern in Task 22. The trace-overflow test asserts that on buffer overflow the subscription closes, the resource fires one final updated notification with `error: "trace_overflow"`, and `MCPResourceOverflowCloses{kind=trace, reason=trace_overflow}` increments.

- [ ] **Step 2: Implement** trace resource. URI parser extracts `automation_id` and `run_id` from `gohome://automations/{automation_id}/runs/{run_id}/trace`. Read drains up to 5s or run completion. Subscribe opens `AutomationService.Trace` from the appropriate cursor; overflow closes the subscription cleanly and fires the final notification.

- [ ] **Step 3: Verify, commit**

```bash
go test ./internal/mcp/resources/... -run TestTraceResource -v
git add internal/mcp/resources/traces.go internal/mcp/resources/traces_test.go
git commit -m "feat(c8): MCP automation-trace resource with overflow-close backpressure"
```

---

## Task 24: CLI — `gohome mcp serve` and `gohome mcp tools`

**Files:**
- Create: `internal/cli/cmd_mcp.go`
- Create: `internal/cli/styles_mcp.go`
- Modify: `cmd/gohome/main.go`

- [ ] **Step 1: Wire the subcommand**

`internal/cli/cmd_mcp.go`:

```go
package cli

import (
    "context"
    "fmt"
    "os"

    "github.com/oklog/ulid/v2"
    "github.com/spf13/cobra"

    "github.com/fynn-labs/gohome/internal/mcp"
    "github.com/fynn-labs/gohome/internal/mcp/audit"
    "github.com/fynn-labs/gohome/internal/mcp/resources"
    "github.com/fynn-labs/gohome/internal/mcp/tools"
)

func mcpCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "mcp",
        Short: "Model Context Protocol server commands",
    }
    cmd.AddCommand(mcpServeCommand())
    cmd.AddCommand(mcpToolsCommand())
    return cmd
}

func mcpServeCommand() *cobra.Command {
    var endpoint string
    cmd := &cobra.Command{
        Use:   "serve",
        Short: "Run the MCP server on stdio",
        RunE: func(cmd *cobra.Command, _ []string) error {
            ctx := cmd.Context()
            session := ulid.Make().String()

            client, err := mcp.NewClient(mcp.ClientOptions{
                EndpointURL: resolveEndpoint(endpoint),
                SessionID:   session,
            })
            if err != nil {
                return fmt.Errorf("dial daemon: %w", err)
            }

            cd, err := client.System.GetConfigDir(ctx, /* request */ )
            if err != nil {
                fmt.Fprintf(os.Stderr, "gohome mcp: cannot reach gohomed: %v\n", err)
                return err
            }
            mcfg, err := client.System.GetMCPConfig(ctx, /* request */ )
            if err != nil {
                return fmt.Errorf("fetch mcp config: %w", err)
            }
            // ... build mcp.Deps, register tools and resources, run mcp.Run ...

            deps := mcp.Deps{
                Client:    client,
                Audit:     audit.NewRecorder(client.System),
                ConfigDir: cd.Msg.ConfigDir,
                MCPCaps:   capsFromProto(mcfg.Msg),
                Version:   buildVersion(),
                Stdin:     os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
            }
            return mcp.Run(ctx, deps)
        },
    }
    cmd.Flags().StringVar(&endpoint, "endpoint", "", "Override daemon endpoint (default: from $GOHOME_ENDPOINT or unix://@data/gohomed.sock)")
    return cmd
}

func mcpToolsCommand() *cobra.Command {
    var asJSON bool
    cmd := &cobra.Command{
        Use:   "tools",
        Short: "Print the MCP tool catalog and exit",
        RunE: func(cmd *cobra.Command, _ []string) error {
            // build a static description of all 12 tools (no daemon dial needed for v1)
            cat := tools.Catalog()
            if asJSON {
                return printToolsJSON(cmd.OutOrStdout(), cat)
            }
            return printToolsHuman(cmd.OutOrStdout(), cat)
        },
    }
    cmd.Flags().BoolVar(&asJSON, "json", false, "Emit JSON instead of styled table")
    return cmd
}
```

`tools.Catalog()` is a small struct: `[]ToolDescriptor{{Name, Summary, Verb, Status, InputSchema}}`. It is built statically from the same data the SDK registration uses; it does not need to dial the daemon for v1 (the `apply_scene` `UNIMPLEMENTED` status is known statically).

- [ ] **Step 2: Lipgloss styles**

`internal/cli/styles_mcp.go`:

```go
package cli

import "github.com/charmbracelet/lipgloss"

var (
    BadgeRead  = lipgloss.NewStyle().Background(lipgloss.Color("#3B82F6")).Foreground(lipgloss.Color("#FFFFFF")).Bold(true).Padding(0, 1) // blue
    BadgeCall  = lipgloss.NewStyle().Background(lipgloss.Color("#10B981")).Foreground(lipgloss.Color("#FFFFFF")).Bold(true).Padding(0, 1) // green
    BadgeAdmin = lipgloss.NewStyle().Background(lipgloss.Color("#F59E0B")).Foreground(lipgloss.Color("#1F2937")).Bold(true).Padding(0, 1) // amber on dark
    BadgeWarn  = lipgloss.NewStyle().Background(lipgloss.Color("#FCD34D")).Foreground(lipgloss.Color("#1F2937")).Bold(true).Padding(0, 1) // warning yellow

    ToolName    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A78BFA"))
    SubtleText  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
    FieldName   = lipgloss.NewStyle().Bold(true)
    TypeName    = lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#9CA3AF"))
    Required    = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
    Divider     = lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))
)
```

(Align color choices with the existing `internal/cli/styles.go` palette — match its accent colors rather than these literal values.)

- [ ] **Step 3: `printToolsHuman`** renders a section per tool: header bar with `ToolName` and the verb badge (BadgeRead/Call/Admin or BadgeWarn for `apply_scene`), `SubtleText` summary line, then the input schema as a `FieldName : TypeName` list with `Required` markers. End with a `Divider` between tools and a footer summary line counting tool kinds.

- [ ] **Step 4: Register on the root command**

In `cmd/gohome/main.go` (and/or wherever the CLI root is composed), add `cli.MCPCommand()` (or `cli.RegisterMCP(rootCmd)`) alongside the other subcommands.

- [ ] **Step 5: Tests**

`internal/cli/cmd_mcp_test.go` covers:
- `gohome mcp tools --json` output is valid JSON of the right shape (12 tools, two of which are admin).
- `gohome mcp tools` (human) output contains all 12 tool names and the `UNIMPLEMENTED` badge text on `apply_scene`.

```bash
go test ./internal/cli/... -run TestMCPTools -v
```

- [ ] **Step 6: Manual smoke test**

```bash
task build
./dist/gohome mcp tools          # human-readable
./dist/gohome mcp tools --json   # machine-readable
```

Expected: 12 tools listed; no daemon required (catalog is static).

- [ ] **Step 7: Commit**

```bash
git add internal/cli/cmd_mcp.go internal/cli/styles_mcp.go internal/cli/cmd_mcp_test.go cmd/gohome/main.go internal/mcp/tools/catalog.go
git commit -m "feat(c8): gohome mcp serve and gohome mcp tools subcommands"
```

---

## Task 25: SDK-level integration tests

**Files:**
- Create: `internal/mcp/server_test.go` (extends the skeleton test from Task 15)

- [ ] **Step 1: Helper to spin up the MCP server in-process and drive it via the SDK client**

Use the official SDK's MCP client over a pipe pair connected to the server's stdin/stdout. Spawn the server via `mcp.Run` in a goroutine.

- [ ] **Step 2: Test cases**

- `tools/list` returns exactly 12 tools, in the order documented in §10.2 of the spec.
- `tools/call` for each of the 11 live tools (including `apply_scene` which expects an `unimplemented` error result) returns the expected shape against a fully-faked daemon (assemble fakes for every Connect service).
- `resources/list` returns one entry per fake-daemon entity plus the two templates.
- `resources/read` for an entity URI returns the right shape.
- `resources/subscribe` for an entity URI fires `notifications/resources/updated` when the fake daemon sends a state-change event on the underlying stream.
- `resources/subscribe` then `resources/unsubscribe` cleanly closes — `MCPResourceSubscriptionsActive{kind=entity}` returns to its baseline.
- Backpressure: configure `EntitySubscriptionBuffer=2`, send 50 events without reading; assert exactly one or two `updated` notifications fire (coalescing) and the metric increments.
- Trace overflow: configure `TraceSubscriptionBuffer=2`, send 50 events; assert subscription is closed with an `error: "trace_overflow"` notification.
- Stdio EOF: close the pipe writer; assert `mcp.Run` returns within 1 second.

```bash
go test ./internal/mcp/... -run TestServer -v
```

- [ ] **Step 3: Commit**

```bash
git add internal/mcp/server_test.go internal/mcp/testfakes_test.go
git commit -m "test(c8): SDK-level integration tests for tools, resources, backpressure"
```

---

## Task 26: End-to-end integration (`//go:build integration`)

**Files:**
- Create: `internal/mcp/integration_test.go`

- [ ] **Step 1: Compose a real daemon fixture**

Reuse `internal/testutil`'s daemon-with-fake-driver helper (created in C7). The fixture starts a real `gohomed` against an in-memory SQLite + a fake Carport driver + a Pkl config containing two automations, two scripts, and a few entities.

- [ ] **Step 2: Spawn `gohome mcp serve` as a real subprocess**

```go
//go:build integration

func TestE2E_MCPServer(t *testing.T) {
    daemon := testutil.StartDaemon(t)
    defer daemon.Stop()

    cmd := exec.Command("./dist/gohome", "mcp", "serve", "--endpoint", daemon.UDS())
    stdin, _ := cmd.StdinPipe()
    stdout, _ := cmd.StdoutPipe()
    cmd.Stderr = os.Stderr
    require.NoError(t, cmd.Start())
    defer cmd.Process.Kill()

    client := sdkClient.New(stdout, stdin) // SDK MCP client
    require.NoError(t, client.Initialize(context.Background()))

    // Walk the catalog (same as 10.3 in spec).
    // 1. list entities; assert two entries.
    entities, err := client.CallTool(ctx, "gohome__list_entities", map[string]any{})
    require.NoError(t, err)
    // ...

    // 2. get one. 3. call_capability. 4. tail_events. 5. subscribe and observe state change.
    // 6. eval_starlark; assert MCPEvalRequested in event store via daemon.QueryEvents.
    // 7. read+write+validate+apply_config; assert ConfigFileEdited and ConfigApplied.
    // 8. trigger an automation; subscribe to trace; observe events.
}
```

- [ ] **Step 3: Run**

```bash
task build
task test:integration -- -run TestE2E_MCPServer -v
```

Expected: pass. (Build the binary first since the test execs it.)

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/integration_test.go
git commit -m "test(c8): end-to-end MCP integration with real subprocess and daemon"
```

---

## Task 27: README / docs update

**Files:**
- Modify: `README.md`
- Modify (or create): `docs/mcp-setup.md` (in `gohome/`, not the docs submodule — implementation docs)

- [ ] **Step 1: Add an MCP section to the gohome README**

Brief:

> ### Connect AI agents via MCP
>
> gohome ships an MCP server out of the box. To wire up Claude Code:
> ```sh
> claude mcp add gohome -- gohome mcp serve
> ```
> Or for Claude Desktop, add to your `claude_desktop_config.json`:
> ```json
> {"mcpServers":{"gohome":{"command":"gohome","args":["mcp","serve"]}}}
> ```
> See [docs/mcp-setup.md](docs/mcp-setup.md) for the full tool catalog and security notes.

- [ ] **Step 2: Write `docs/mcp-setup.md`** with:
  - Setup snippets for Claude Desktop, Claude Code, Cursor.
  - Catalog: the 12 tools + 2 resource families with one-line summaries (cribbed from the spec — keep updated).
  - Security notes: local-only via UDS until C9; how to disable specific tools after C9.
  - Troubleshooting: `gohomed not running` errors, where to look in `gohome events tail`.

- [ ] **Step 3: Commit**

```bash
git add README.md docs/mcp-setup.md
git commit -m "docs(c8): MCP setup instructions and tool catalog"
```

---

## Final Verification

Before opening a PR:

```bash
task lint                               # or whatever the project uses
task test                               # unit tests
task test:race                          # race detector
task build                              # both binaries
task test:integration                   # end-to-end (requires the build above)
```

Smoke-test by hand:

```bash
./dist/gohome mcp tools                            # prints the catalog
./dist/gohomed --config ./testdata/sample.pkl &    # daemon
GOHOME_ENDPOINT=unix://./gohomed.sock ./dist/gohome mcp serve  # in another shell, with manual JSON-RPC
```

Verify in the daemon's metrics endpoint that `gohome_mcp_*` metrics increment when you exercise tools through a real MCP client (Claude Desktop, Claude Code).

---

*End of C8 implementation plan.*
