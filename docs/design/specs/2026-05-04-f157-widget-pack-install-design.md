# F-157: Widget Pack Install — OCI Pull + Cosign Verification

**Status:** Draft — design ready for implementation planning.
**Linear:** F-157 (parent: C10 Web UI Architecture).
**Date:** 2026-05-04.
**Lands independently of:** F-156 (dashboard backend) — see §11.

---

## 1. Goal

Make `switchyard widget install <oci-ref>` actually pull an OCI artifact, verify its cosign signature against the configured trust policy, stage the pack files for serving, validate the manifest, register the pack's classes into the dashboard catalog, and emit an installation event — replacing the current metadata-registration stub in `internal/widgetpack/install.go`.

This implements the full C10 spec §15.4 server-side install flow (all 11 steps), §15.2 pack manifest schema, and §15.7 runtime serving — exposed via a new admin-authz'd `WidgetPackService` Connect-RPC and the existing `switchyard widget {install,list,uninstall}` CLI scaffolding.

## 2. Background

### 2.1 Today's state

- `internal/widgetpack/install.go` is a stub: ignores `req.Ref`, sets `SHA256: "pending"` and `SignatureStatus: "unsigned"`, calls `store.Add`. No OCI fetch, no signature verification, no file staging.
- `internal/widgetpack/trust.go::Verify` is a string-switch (`"verified"` → ok; `"unsigned"` → policy bool) — no real signature math.
- `internal/widgetpack/store.go` is in-memory only, no persistence across restarts.
- The package is not wired anywhere: no Connect-RPC, no daemon hookup, no CLI plumbing. `internal/cli/cmd_widget.go` exists with `RunE` stubs returning `nil`.
- `internal/config/pkl/switchyard/widgets.pkl` defines a minimal `PackManifest` (`name`, `version`, `classes`) and `PackPolicy` (`allowedSigners`, `allowUnsigned`) — no top-level instance, no `bundle`/`bundleHash`/`sdkVersion` fields, no `WidgetInstance` re-export for pack authors.
- The web client (`web/src/dashboard/pack-loader.ts:10`) already fetches packs from `/widgets/<pack>/<version>/bundle.js?h=<hash>` — but no server-side handler serves that path.

### 2.2 What this spec covers

The full server-side install flow with admin authz and end-to-end CLI usability, including the §15.2 manifest schema work that would otherwise fall through the cracks (no other Linear ticket tracks it). It does *not* cover browser-side cache invalidation (that depends on the C10 multiplexer, which isn't yet implemented), nor pack pinning, nor raw-pubkey signing — see §11.

## 3. Architecture overview

### 3.1 New / modified components

| Path | New? | Purpose |
|------|------|---------|
| `internal/config/pkl/switchyard/widgets.pkl` | modified | Full §15.2 `PackManifest` + re-export of `WidgetInstance` (and its helper classes); `widgetPackPolicy` instance |
| `internal/config/pkl/switchyard/dashboards.pkl` | modified | Import `WidgetInstance` from `widgets.pkl` rather than declaring locally |
| `internal/config/pkl/switchyard/policy.pkl` | modified | Add top-level `widgetPackPolicy: PackPolicy` |
| `proto/switchyard/v1alpha1/widget_pack.proto` | new | `WidgetPackService { Install, List, Uninstall, Watch }` |
| `proto/switchyard/config/v1/...` | modified | New `WidgetPackPolicy` message on `ConfigSnapshot` |
| `internal/widgetpack/install.go` | rewritten | Full §15.4 flow |
| `internal/widgetpack/trust.go` | rewritten | Real cosign keyless verification via `sigstore-go` |
| `internal/widgetpack/oci.go` | new | `oras-go` artifact pull + signature retrieval |
| `internal/widgetpack/manifest.go` | new | Pkl-evaluator-driven manifest validation |
| `internal/widgetpack/store.go` | extended | On-disk `.registry.json`, `Subscribe`, multi-version |
| `internal/widgetpack/serve.go` | new | `http.Handler` for `/widgets/<pack>/<version>/<file>` |
| `internal/widgetpack/service.go` | new | Connect handler implementing `WidgetPackService` |
| `internal/api/service_widget_pack.go` | new | Procedure-catalog entries for authz |
| `internal/api/listener/routes.go` | modified | Add `WidgetPack` to `Services`, mount RPC route |
| `internal/api/listener/listener.go` | modified | Mount `/widgets/*` static handler outside `/api` tree |
| `internal/daemon/daemon.go` | modified | Construct store + installer; wire `OnApplied` → `TrustPolicy` updates |
| `internal/cli/cmd_widget.go` | rewritten | Replace stubs with Dial + Connect client calls |
| `internal/widgetpack/*_test.go` | extended/new | See §10 |

### 3.2 Boundaries

- `widgetpack` owns: OCI fetch, signature verify, on-disk staging, manifest validation, registry persistence, bundle serving, install/uninstall/list/watch operations.
- `dashboard.Catalog` reads from `widgetpack.Store` for non-builtin classes via a small `Store.ClassesView()` method; `widgetpack` does not import `dashboard`.
- `daemon` is the sole wiring layer: builds `Store` + `Installer` against `DataDir`, plumbs `TrustPolicy` from config, registers handlers.

## 4. Pkl schema (§15.2 manifest + policy)

### 4.1 `widgets.pkl`

```pkl
module switchyard.widgets

// Re-exported so pack manifests can extend without importing dashboards.pkl.
// (Same shape as previously in dashboards.pkl.)
abstract class WidgetInstance { /* ... */ }

// Class-name string constants (existing — unchanged).
const gauge:        String = "Gauge"
const lineChart:    String = "LineChart"
const entityToggle: String = "EntityToggle"
const markdown:     String = "Markdown"
const scriptButton: String = "ScriptButton"
const cameraStream: String = "CameraStream"
const entityList:   String = "EntityList"
const groupCard:    String = "GroupCard"

class PackManifest {
  name:        String
  version:     String                // semver
  protocol:    String                // "v1" — manifest protocol
  sdkVersion:  String                // semver of @switchyard/widget-sdk
  bundle:      String                // path inside artifact, typically "bundle.js"
  bundleHash:  String                // "sha256:<hex>"
  classes:     Listing<String>       // class names exported by the bundle
  description: String?
  homepage:    String?
  license:     String?
}

class PackPolicy {
  allowedSigners: Listing<String> = new {}
  allowUnsigned:  Boolean          = false
}
```

`dashboards.pkl` updates to import `WidgetInstance` from `widgets.pkl` instead of declaring it locally. `ContainerWidget`, `LeafWidget`, and `Dashboard` stay in `dashboards.pkl` — pack authors don't author containers in v1.0 (only the builtin `GroupCard` is a container).

### 4.2 `policy.pkl`

Adds a top-level `widgetPackPolicy: widgets.PackPolicy = new {}` next to other policy config. The exact placement is in the appropriate `policy.pkl` slot, picked to match how other policies are exposed.

## 5. Proto schema

### 5.1 `proto/switchyard/v1alpha1/widget_pack.proto`

```proto
syntax = "proto3";
package switchyard.v1alpha1;
import "google/protobuf/timestamp.proto";

service WidgetPackService {
  rpc Install   (InstallRequest)   returns (InstallResponse);
  rpc List      (ListRequest)      returns (ListResponse);
  rpc Uninstall (UninstallRequest) returns (UninstallResponse);
  rpc Watch     (WatchRequest)     returns (stream WatchEvent);
}

message InstallRequest  { string ref = 1; }
message InstallResponse { InstalledPack pack = 1; }

message UninstallRequest { string name = 1; string version = 2; bool force = 3; }
message UninstallResponse {}

message ListRequest  {}
message ListResponse { repeated InstalledPack packs = 1; }

message InstalledPack {
  string                    name             = 1;
  string                    version          = 2;
  string                    sha256           = 3;
  SignatureStatus           signature        = 4;
  string                    signer_identity  = 5;
  repeated string           classes          = 6;
  string                    bundle_url       = 7;
  string                    description      = 8;
  string                    homepage         = 9;
  string                    license          = 10;
  google.protobuf.Timestamp installed_at     = 11;
}

// SignatureStatus is reused from proto/switchyard/v1alpha1/dashboard.proto
// (proto3 same-package enums must be unique). The existing enum's values
// are SIGNATURE_UNKNOWN/VERIFIED/UNSIGNED/INVALID/EXPIRED — a strict superset.
// SIGNATURE_EXPIRED maps to FAILED_PRECONDITION/signature_expired in §5.2.

message WatchRequest {}
message WatchEvent {
  oneof kind {
    InstalledPack    installed   = 1;
    UninstalledPack  uninstalled = 2;
  }
}
message UninstalledPack { string name = 1; string version = 2; }
```

### 5.2 Connect error code mapping

| Server error | Connect code | `ErrorDetail.reason` |
|---|---|---|
| Empty / malformed `ref` | `INVALID_ARGUMENT` | `bad_ref` |
| Caller lacks `widget_pack.install` | `PERMISSION_DENIED` | (set by authz interceptor) |
| Signature rejected by trust policy | `FAILED_PRECONDITION` | `signature_invalid` |
| Signing certificate expired | `FAILED_PRECONDITION` | `signature_expired` |
| `bundle.js` SHA256 ≠ `manifest.bundleHash` | `FAILED_PRECONDITION` | `hash_mismatch` |
| `manifest.sdkVersion` major mismatch | `FAILED_PRECONDITION` | `sdk_incompatible` |
| Class collision with builtin or installed pack | `FAILED_PRECONDITION` | `class_collision` |
| Pkl manifest fails schema validation | `FAILED_PRECONDITION` | `manifest_invalid` |
| Multi-layer artifact / wrong media type | `FAILED_PRECONDITION` | `bad_artifact` |
| Path escape in tarball entry | `FAILED_PRECONDITION` | `bad_artifact` |
| Registry unreachable | `UNAVAILABLE` | `registry_unreachable` |
| Same `name@version` already installed | `ALREADY_EXISTS` | (default) |
| Anything else | `INTERNAL` | `internal` |

`Uninstall` adds `FAILED_PRECONDITION/in_use` (referenced by a dashboard, `force=false`) and `NOT_FOUND` (no such pack).

### 5.3 `WidgetPackPolicy` in `ConfigSnapshot`

A new message `switchyard.config.v1.WidgetPackPolicy { repeated string allowed_signers = 1; bool allow_unsigned = 2; }` and a corresponding field on the snapshot. The decoder in `internal/config/evaluator_decode.go` extracts it from the Pkl evaluation result.

## 6. Install flow (§15.4 — all 11 steps)

```
Install(ctx, InstallRequest{Ref}) → (InstalledPack, error)

1. Pull         → oras-go: copy artifact from <Ref> into a memory store.
                  Verify single layer with mediaType
                  "application/vnd.switchyard.widgetpack.v1+tar+gzip".
                  Pull cosign signature artifact from <Ref>.sig (cosign convention).
2. Verify       → trust.Verify(ctx, descriptor, signatureBlob, TrustPolicy):
                  • If TrustPolicy.AllowUnsigned && no signature
                    → SignatureStatus = UNSIGNED, continue.
                  • Else: sigstore-go verifier with default Sigstore TUF root,
                    Fulcio cert subject-identity matched against
                    TrustPolicy.AllowedSigners (path.Match glob).
                  • Reject on any failure unless AllowUnsigned (and even then
                    only the "no signature present" case is allowed; INVALID never is).
3. Stage        → mkdir <DataDir>/widgets/.staging/<rand>/, gunzip+untar layer there.
                  Reject if any tarball entry path escapes the staging dir.
4. Manifest     → Pkl-evaluate <staging>/manifest.pkl against the
                  switchyard.widgets.PackManifest schema using a fresh evaluator
                  rooted at <staging>/. Decode into Go struct.
5. Hash verify  → sha256 of <staging>/<manifest.bundle> must equal
                  manifest.bundleHash. Persist the computed hash on InstalledPack.
6. SDK check    → semver major(manifest.sdkVersion) == major(host SDK version).
                  Host SDK version is a build-time const in the widgetpack package.
                  Mismatch → FAILED_PRECONDITION/sdk_incompatible.
7. Collisions   → store.List() + builtins → set of taken classIDs.
                  For each new class: "<name>/<class>" must not already be taken;
                  "<class>" alone must not match a builtin.
                  Same-name reinstall of a different version: fine (multi-version).
                  Same name@version: ALREADY_EXISTS.
8. Commit       → os.Rename(<staging>/, <DataDir>/widgets/<name>/<version>/).
                  store.Add(pack) — also persists the registry sidecar.
9. Reload       → no-op for F-157 standalone (see §12).
10. Emit        → store fires OnPackInstalled(pack) hook synchronously after Add;
                  Service.Watch implementations forward to subscribers.
11. Return      → InstalledPack with full metadata.

Cleanup: defer-removes the staging dir on any failure between step 3 and step 8.
On step-8 failure (rename succeeded but store.Add failed): rename-back +
RemoveAll narrow-window rollback.
```

### 6.1 Concurrency

Per-`(name@version)` mutex via `sync.Map`; concurrent installs of different packs run in parallel. Two concurrent installs of the same `name@version`: the first wins; the second sees `ALREADY_EXISTS` once step 8 completes (or progresses to step 8 with a fresh state if the first failed).

### 6.2 Sigstore-go specifics

- Default Sigstore TUF root in production. Tests inject a `TrustedRoot` built from a test CA + test Rekor public key.
- Verifier configured for keyless / Fulcio cert-identity verification only. Raw-pubkey path is intentionally not enabled (see §11).
- `AllowedSigners` glob match: each entry runs through Go `path.Match` against the cert subject identity URI; multiple patterns OR.

### 6.3 `oras-go` specifics

- `oras.land/oras-go/v2`.
- Anonymous by default; reads `~/.docker/config.json` for credentials so `docker login ghcr.io` flows through transparently.
- Single-layer assumption checked explicitly; multi-layer artifacts rejected with `FAILED_PRECONDITION/bad_artifact`.

**Known limitation — cosign signature lookup:** F-157 v1 reads cosign signatures only from the legacy tag-based layout (`<digest>.sig`). Cosign 2.x against OCI 1.1-capable registries (ghcr.io, AWS ECR, Docker Hub since 2024) defaults to attaching signatures as Referrers (manifest.subject), which this fetcher does not query. Modern-layout signed artifacts will appear unsigned to F-157. Tracked separately as **F-289** ("widget pack OCI 1.1 Referrers signature lookup").

### 6.4 Storage layout under `<DataDir>/widgets/`

```
<DataDir>/widgets/
├── .registry.json                          # source of truth, atomic-rename-on-write
├── .staging/                               # transient
│   └── <rand>/                             # one per in-flight install
├── bar-widgets/
│   ├── 1.0.0/{manifest.pkl, bundle.js, README.md}
│   └── 1.1.0/{manifest.pkl, bundle.js}
└── foo-widgets/
    └── 2.3.0/{manifest.pkl, bundle.js}
```

Default `DataDir`: `~/.local/share/switchyard` (existing daemon convention).

On startup, `Store.Load(ctx)` reads `.registry.json` and verifies each entry's directory + bundle file still exist. Stale entries are dropped with a warning log; the registry is rewritten if anything changed.

## 7. Uninstall flow

1. Authz: `widget_pack.uninstall`.
2. Lookup `name@version` in store → `NOT_FOUND` if absent.
3. Reference check (when `!force`): scan `dashboard.Backend.List()` for any widget instance with `classID == "<name>/<class>"` for any class in the pack. If matches found → `FAILED_PRECONDITION/in_use` with the dashboard slugs in the error detail. **Today (F-156 unimplemented) this scan returns empty, so uninstall proceeds; the code path is in place for when F-156 lands.**
4. `os.RemoveAll(<DataDir>/widgets/<name>/<version>/)`, `store.Remove(name, version)`, persist registry.
5. Fire `OnPackUninstalled(name, version)` hook → `Service.Watch` subscribers receive the event.

## 8. Bundle serving (§15.7)

`/widgets/<pack>/<version>/<file>` → `widgetpack.NewBundleHandler(store, dataDir)`:

- Resolves to `<DataDir>/widgets/<pack>/<version>/<file>`.
- Path-traversal defense: `filepath.Clean`; reject if cleaned path escapes the version dir.
- Pack must be present in `store` — half-installed (staging-only) packs not served.
- Headers:
  - `Cache-Control: public, max-age=31536000, immutable`
  - `Content-Type` from extension (`.js` → `text/javascript`, `.map` → `application/json`, `.css` → `text/css`)
  - `Content-Length` set; `ETag: "<sha256>"` from registry
- Method allowlist: `GET`, `HEAD`. Anything else → `405`.
- `If-None-Match` matching `ETag` → `304`.
- **No auth** on `/widgets/*`. Justification: the bundle is install-time-signature-verified, name+version-immutable static content; the browser's dynamic `import()` only speaks plain HTTP; the `?h=<sha>` cache-key URL is the entire reason for the design; CSP `script-src 'self'` requires same-origin static URLs; the security boundary is install-time, not per-request.

The handler is mounted by `internal/api/listener/listener.go` outside the `/api` tree (the listener mux already discriminates static asset paths from RPC paths).

## 9. RPC + CLI wiring

### 9.1 Routes (`internal/api/listener/routes.go`)

```go
type Services struct {
  // ... existing fields ...
  WidgetPack switchyardv1alpha1connect.WidgetPackServiceHandler
}

func BuildRoutes(svc Services, interceptors ...connect.Interceptor) []Route {
  // ... existing routes ...
  p, h := switchyardv1alpha1connect.NewWidgetPackServiceHandler(svc.WidgetPack, opts)
  routes = append(routes, Route{Path: p, Handler: h})
  return routes
}
```

### 9.2 Authz (`internal/api/service_widget_pack.go`)

Procedure catalog entries (matching the existing `procedureCatalog` shape used by other services):

| Procedure | Action service | Method | Verb |
|---|---|---|---|
| `Install` | `widget_pack` | `install` | `write` |
| `Uninstall` | `widget_pack` | `uninstall` | `write` |
| `List` | `widget_pack` | `list` | `read` |
| `Watch` | `widget_pack` | `watch` | `read` |

Default policy (in C9 policy Pkl): `widget_pack.install` + `widget_pack.uninstall` allowed for `admin` role only; `list` + `watch` allowed for any authenticated user (so browsers can render the catalog).

### 9.6 Authz wiring dependency (F-184)

`internal/daemon/daemon.go:408` currently passes `nil` as the `ProcedureCatalog` argument to `api.NewAuthorize`, which makes the authz interceptor pass-through (`if rt == nil || catalog == nil { return next(ctx, req) }`). C9 shipped the policy runtime, role graph, and interceptors, but no procedure catalog implementation exists in the daemon today.

F-157 declares procedure-catalog registration code for the four `widget_pack` procedures, packaged as a `registerWidgetPackProcedures(*Catalog)` helper in `internal/api/service_widget_pack.go`. This code is **inert** until **F-184** ("C9: wire ProcedureCatalog implementation into daemon authz interceptor") lands, at which point F-157's entries become live with no further changes required.

Until F-184 lands, `Install`/`Uninstall` are reachable by any caller that can dial the UDS or TCP listener. UDS file permissions (`0o600`) provide a coarse local gate; the TCP listener relies on TLS termination upstream. This is the same de-facto authz posture every other write RPC has today (Area, Zone, Device, Automation, Script, etc.). F-185 ("C9: populate ProcedureCatalog entries for all existing RPC services") tracks closing the gap across the whole API surface.

### 9.3 Service handler skeleton

```go
type Service struct {
  installer *Installer
  store     *Store
}

func (s *Service) Install(ctx context.Context,
    req *connect.Request[v1.InstallRequest],
) (*connect.Response[v1.InstallResponse], error) {
  pack, err := s.installer.Install(ctx, InstallRequest{Ref: req.Msg.GetRef()})
  if err != nil {
    return nil, mapInstallErr(err)
  }
  return connect.NewResponse(&v1.InstallResponse{Pack: toProto(pack)}), nil
}

func (s *Service) Watch(ctx context.Context,
    _ *connect.Request[v1.WatchRequest],
    stream *connect.ServerStream[v1.WatchEvent],
) error {
  ch := make(chan WatchEvent, 16)
  unsub := s.store.Subscribe(ch)
  defer unsub()
  for {
    select {
    case <-ctx.Done():
      return nil
    case ev := <-ch:
      if err := stream.Send(eventToProto(ev)); err != nil {
        return err
      }
    }
  }
}
```

### 9.4 Daemon wiring (`internal/daemon/daemon.go`)

```go
packStore := widgetpack.NewStore(filepath.Join(cfg.DataDir, "widgets"))
if err := packStore.Load(ctx); err != nil { return err }

trustPolicy := &widgetpack.TrustPolicy{}
installer   := widgetpack.NewInstaller(packStore, trustPolicy, cfg.DataDir)
packService := widgetpack.NewService(installer, packStore)

cfgManager.OnApplied(func(snap *configpb.ConfigSnapshot) {
  if p := snap.GetWidgetPackPolicy(); p != nil {
    trustPolicy.Set(p.GetAllowedSigners(), p.GetAllowUnsigned())
  }
})

listener.Mount(svc, widgetpack.NewBundleHandler(packStore, cfg.DataDir))
```

`TrustPolicy.Set` uses an internal mutex so the swap is thread-safe; in-flight installs that already snapshotted the policy aren't disturbed.

### 9.5 CLI (`internal/cli/cmd_widget.go`)

Replace stubs with Dial + Connect client calls. Patterns mirror `cmd_automation.go`:

- `install <ref>` → `WidgetPackServiceClient.Install({Ref: args[0]})`. Render success line with `styles_widget.PackVerified` / `PackUnsigned`. On `FAILED_PRECONDITION`, print the `reason` detail; for `signature_invalid` add a hint about `--allow-unsigned` (only meaningful in dev mode).
- `list` → `WidgetPackServiceClient.List({})` → table: `NAME  VERSION  SIG  CLASSES`.
- `uninstall <name>` with flags `--version` and `--force`:
  - Without `--version`: client-side List → iterate versions → call Uninstall once per version (server semantics stay single-version-per-call).
  - `--force` passes through to the server.

## 10. Testing plan

### 10.1 Unit tests (`internal/widgetpack/`)

- `trust_test.go` — extend with real cosign-verifier paths:
  - `AllowedSigners` glob match: single, multiple-OR, none-match → reject.
  - Verifier rejects expired Fulcio cert.
  - Verifier rejects mismatched bundle (signature over different content).
  - `AllowUnsigned=true` + no signature → `UNSIGNED` status, accept.
  - `AllowUnsigned=true` + INVALID signature → still reject (only "absent" is allowed under unsigned mode).
- `manifest_test.go` — Pkl evaluation:
  - Required fields missing (`name`, `version`, `bundle`, `bundleHash`, `sdkVersion`, `protocol`) → reject.
  - `protocol != "v1"` → reject.
  - SDK semver mismatch (different major) → reject.
  - Valid manifest → struct populated; optional fields nil-friendly.
- `serve_test.go` — bundle handler:
  - GET → 200, correct `Content-Type`, `Cache-Control: immutable`, body matches.
  - HEAD → 200, headers, no body.
  - Path traversal (`../`, encoded variants) → 400.
  - Unknown pack/version → 404.
  - Method other than GET/HEAD → 405.
  - `If-None-Match` matching `ETag` → 304.
- `store_test.go` — extend:
  - `.registry.json` round-trip across `Load`/`Add`/`Remove`.
  - Stale entry on startup (registry references missing dir) → dropped + warning.
  - Concurrent `Add`/`Remove` race-free (`-race` clean).
  - `Subscribe` fan-out: multiple subscribers each receive each event; unsubscribe removes; closed channels don't block other subscribers.

### 10.2 Integration test (`internal/widgetpack/install_integration_test.go`)

End-to-end against an in-process OCI registry (`go-containerregistry/pkg/registry`) and a sigstore-go test trust root.

```go
func TestInstaller_Install_Integration(t *testing.T) {
  reg := registry.New()
  srv := httptest.NewServer(reg)
  defer srv.Close()

  trustRoot := buildTestTrustRoot(t)                 // test CA + test Rekor pubkey
  packBlob := buildTestPack(t, manifestSrc, bundleSrc)
  ref := pushArtifact(t, srv.URL, "bar-widgets", "1.0.0", packBlob)
  signWithTestRoot(t, ref, trustRoot)                // pushes <ref>.sig

  inst := newInstallerForTest(t, trustRoot, []string{"https://test/identity"})

  pack, err := inst.Install(ctx, InstallRequest{Ref: ref})
  // assertions on pack.SHA256, pack.SignatureStatus, pack.Classes

  assertFileExists(t, dataDir, "widgets/bar-widgets/1.0.0/bundle.js")
  assertFileExists(t, dataDir, "widgets/bar-widgets/1.0.0/manifest.pkl")

  resp := httpGet(t, bundleHandlerSrv.URL, pack.BundleURL)
  assertStatus(t, resp, 200)
  assertHeader(t, resp, "Cache-Control", "public, max-age=31536000, immutable")
  assertBodyHash(t, resp, pack.SHA256)

  classes := catalog.WidgetClasses()
  assertContains(t, classes, "bar-widgets/BarChart")
}
```

Sub-tests (same file) for rejection paths:
- Unsigned + `AllowUnsigned=false` → rejected; nothing staged.
- Signed but signer identity not in `AllowedSigners` → rejected.
- Bundle hash mismatch in manifest → rejected.
- Class collision against builtin (`EntityToggle`) → rejected.
- Class collision against another installed pack → rejected.
- Invalid OCI ref (registry unreachable) → rejected.
- Two concurrent installs of same `name@version` → exactly one wins; other gets `ALREADY_EXISTS`.

### 10.3 RPC tests (`internal/api/service_widget_pack_test.go`)

- Authz: caller without `widget_pack.install` permission → `PERMISSION_DENIED`.
- Error code mapping: each install-side error class maps to the right Connect code.
- `Watch`: subscribing client receives an `Installed` event after a concurrent install; cancellation cleans up the subscription.

### 10.4 CLI tests (`internal/cli/cmd_widget_test.go`)

Smoke test using a fake `WidgetPackServiceClient` — exercises argument parsing, output rendering, error-message paths. No real daemon.

## 11. Out of scope (deferred)

Tracked via follow-up Linear tickets filed after this spec is approved:

1. **Browser-side `Watch` consumer** — multiplexer subscribes to `WidgetPackService.Watch` for catalog cache invalidation. Depends on C10 multiplexer infrastructure that isn't in any current ticket.
2. **Pack pinning** — `widgets-lock.pkl` per spec §15.5.
3. **Raw-pubkey cosign verification** — keyless-only for v1.0.
4. **`switchyard widget update`** — polished update command + progress reporting.
5. **`switchyard widget search`** — discovery against registries.

Also explicitly deferred (no ticket):
- Iframe sandbox for pack bundles (spec §15.8 — "v1.x may add").
- OCI registry credential management UI beyond `~/.docker/config.json`.
- Container widgets beyond `GroupCard` (spec §1.2).
- Client-side Starlark, WebRTC for cameras, etc. (spec §1.2).

## 12. Dependencies on F-156

F-157 lands independently of F-156. Two integration points are forward-compatible:

- **§7 step 3 (uninstall reference check):** scans `dashboard.Backend.List()`. Today returns empty (the no-op stub), so uninstall is permissive in practice. When F-156 lands, the same code becomes a real reference check with no F-157 changes required.
- **§9 step 9 ("trigger config reload"):** is a no-op today because nothing on the server caches dashboards or holds a derived catalog. When F-156 lands, `dashboard.Backend.Get` re-evaluates Pkl per call — new pack classes resolve on next `Get` without any explicit reload trigger. The `OnPackInstalled` hook + `Watch` stream remain the right primitives for the eventual browser-side cache invalidation.

## 13. Acceptance criteria (from F-157)

- [ ] OCI pull works against a real OCI registry (in-process registry in tests; real registry usable in production).
- [ ] Cosign verification accepts/rejects per trust policy.
- [ ] SHA256 stored is the real bundle hash.
- [ ] Bundle served at a stable URL.
- [ ] Catalog populated from `manifest.pkl`.
- [ ] Integration test covers signed and unsigned paths.
- [ ] No unrelated refactors. (§15.2 manifest schema work is in scope as part of completing C10 spec §15 alongside the install path; no other ticket would carry it.)
