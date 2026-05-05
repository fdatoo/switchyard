# F-157: Widget Pack Install — OCI Pull + Cosign Verification — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the stub in `internal/widgetpack/install.go` with the full C10 §15.4 install flow — OCI pull, cosign keyless verification, tarball extraction, manifest validation, bundle hashing, class-collision check, atomic commit, and event emission — exposed via a new `WidgetPackService` Connect-RPC, an HTTP bundle handler at `/widgets/*`, and the existing `switchyard widget {install,list,uninstall}` CLI scaffolding.

**Architecture:** A rewritten `widgetpack` package owns OCI fetch (`oras-go/v2`), cosign keyless verification (`sigstore-go` against the default Sigstore TUF trust root, test root injectable), Pkl-evaluator-driven manifest validation, on-disk staging with atomic rename to `<DataDir>/widgets/<pack>/<version>/`, and an `http.Handler` that serves bundles with immutable cache headers. A new Connect-RPC `WidgetPackService { Install, List, Uninstall, Watch }` wraps the package. Daemon wiring constructs the store + installer at startup; `config.Manager.OnApplied` keeps the in-memory `TrustPolicy` in sync with `widgetPackPolicy` from Pkl config.

**Tech Stack:** Go 1.23+, `oras.land/oras-go/v2`, `github.com/sigstore/sigstore-go`, `github.com/google/go-containerregistry/pkg/registry` (test only), `connectrpc.com/connect`, Apple `pkl-go`, Cobra (CLI).

**Spec:** `docs/design/specs/2026-05-04-f157-widget-pack-install-design.md`

**Dependencies:**
- F-184 ("C9: wire ProcedureCatalog into daemon authz interceptor") — Task 12's procedure-catalog entries remain inert until F-184 lands. F-157 does not depend on F-184 to merge; it merges with the entries in place but unwired.
- F-156 ("dashboard backend filesystem persistence") — Task 10's reference-check is forward-compatible (returns empty today; activates when F-156 lands).

---

## File structure

| Path | Status | Responsibility |
|---|---|---|
| `internal/config/pkl/switchyard/widgets.pkl` | **modify** | Full §15.2 `PackManifest` (`bundle`, `bundleHash`, `sdkVersion`, `protocol`, optional metadata); re-export `abstract class WidgetInstance` so pack manifests can extend without importing `dashboards.pkl`. |
| `internal/config/pkl/switchyard/dashboards.pkl` | **modify** | Drop local `WidgetInstance`; `import "switchyard:widgets" as widgets` and use `widgets.WidgetInstance`. |
| `internal/config/pkl/switchyard/policy.pkl` | **modify** | Add top-level `widgetPackPolicy: widgets.PackPolicy = new {}`. |
| `proto/switchyard/config/v1/config.proto` | **modify** | Add `WidgetPackPolicy { repeated string allowed_signers; bool allow_unsigned; }` and a field on `ConfigSnapshot`. |
| `internal/config/evaluator_decode.go` | **modify** | Decode the new `widgetPackPolicy` field. |
| `proto/switchyard/v1alpha1/widget_pack.proto` | **new** | `WidgetPackService { Install, List, Uninstall, Watch }`, supporting messages, `SignatureStatus` enum. |
| `internal/widgetpack/store.go` | **rewrite** | On-disk `.registry.json` persistence; `Subscribe(ch)` fan-out for `OnPackInstalled`/`OnPackUninstalled`; multi-version. |
| `internal/widgetpack/store_test.go` | **rewrite** | Persistence round-trip, stale-entry pruning, fan-out tests. |
| `internal/widgetpack/trust.go` | **rewrite** | sigstore-go keyless verification, glob-match against `AllowedSigners`, injectable trust root, thread-safe `Set`. |
| `internal/widgetpack/trust_test.go` | **rewrite** | Glob match, expired cert reject, mismatched-bundle reject, unsigned policy. |
| `internal/widgetpack/oci.go` | **new** | `oras-go` artifact pull + cosign signature artifact retrieval. |
| `internal/widgetpack/manifest.go` | **new** | Pkl evaluator wrapper for `manifest.pkl` → `Manifest` Go struct. |
| `internal/widgetpack/manifest_test.go` | **new** | Required-field, protocol, sdkVersion, optional-field tests. |
| `internal/widgetpack/serve.go` | **new** | `http.Handler` for `/widgets/<pack>/<version>/<file>`. |
| `internal/widgetpack/serve_test.go` | **new** | Path traversal, content-type, cache headers, ETag, method allowlist. |
| `internal/widgetpack/install.go` | **rewrite** | Chains pull → verify → stage → manifest → hash → SDK → collisions → commit → emit; deferred cleanup. |
| `internal/widgetpack/install_test.go` | **rewrite** | Reset (the existing trivial tests don't exercise enough); Task 16 covers integration. |
| `internal/widgetpack/install_integration_test.go` | **new** | End-to-end against in-process `go-containerregistry` registry + test trust root; covers signed/unsigned/mismatch/collision/concurrent. |
| `internal/widgetpack/service.go` | **new** | Connect handler implementing `WidgetPackService`. |
| `internal/widgetpack/service_test.go` | **new** | Error code mapping; Watch event delivery; cancellation cleanup. |
| `internal/widgetpack/testutil_test.go` | **new** | Shared test helpers: build test pack, push to in-process registry, sign with test trust root, build test trust root. |
| `internal/api/service_widget_pack.go` | **new** | `registerWidgetPackProcedures(*Catalog)` registrar (inert until F-184). |
| `internal/api/listener/routes.go` | **modify** | Add `WidgetPack` to `Services`, mount RPC route. |
| `internal/daemon/daemon.go` | **modify** | Construct `widgetpack.Store` + `Installer` + `Service`; register `OnApplied` callback updating `TrustPolicy`; pass services to listener. |
| `internal/cli/cmd_widget.go` | **rewrite** | Real `RunE` handlers for install/list/uninstall using `WidgetPackServiceClient`. |
| `internal/cli/cmd_widget_test.go` | **new** | Smoke tests with fake client. |
| `internal/dashboard/catalog.go` | **modify** | `Catalog` reads pack classes via a `widgetpack.Store` reference (added optionally — see Task 13). |

---

## Conventions used in this plan

- All `task` invocations refer to the project-root `Taskfile.yml`. `task proto` runs `buf generate`; `task test` runs `go test ./...`.
- Commit messages use the project's existing convention (`feat(widgetpack): ...`, `fix(...)`, `docs(...)`). The plan suggests messages but they're not load-bearing.
- "Run tests" steps state the *expected* outcome (FAIL or PASS) so the executor can verify; if the actual outcome doesn't match, stop and investigate before proceeding.

---

## Task 1: Extend `switchyard.widgets` Pkl module

**Files:**
- Modify: `internal/config/pkl/switchyard/widgets.pkl`
- Modify: `internal/config/pkl/switchyard/dashboards.pkl`

This task lands the §15.2 manifest schema and relocates `WidgetInstance` into `widgets.pkl` so pack authors can extend it without importing `dashboards.pkl`. Doing it first keeps later Pkl-evaluator code (Task 7) referencing the right shape.

- [ ] **Step 1: Read the current `widgets.pkl` and `dashboards.pkl`**

```bash
cat internal/config/pkl/switchyard/widgets.pkl
cat internal/config/pkl/switchyard/dashboards.pkl
```

Expected: `widgets.pkl` is 22 lines (current minimal `PackManifest` + `PackPolicy`); `dashboards.pkl` declares `abstract class WidgetInstance`.

- [ ] **Step 2: Rewrite `widgets.pkl`**

Replace with:

```pkl
module switchyard.widgets

// Re-exported so pack manifests can extend WidgetInstance without importing
// dashboards.pkl. Same shape as the previous declaration in dashboards.pkl.
abstract class WidgetInstance {
  id: String(!isEmpty)
  classID: String(!isEmpty)
  pos: Position
  props: Mapping<String, Any>(!isEmpty) = new {}
}

class Position {
  x: Int(this >= 0)
  y: Int(this >= 0)
  w: Int(this > 0)
  h: Int(this > 0)
}

// Built-in class IDs. Pack class IDs are namespaced as "<pack>/<class>".
const gauge:        String = "Gauge"
const lineChart:    String = "LineChart"
const entityToggle: String = "EntityToggle"
const markdown:     String = "Markdown"
const scriptButton: String = "ScriptButton"
const cameraStream: String = "CameraStream"
const entityList:   String = "EntityList"
const groupCard:    String = "GroupCard"

class PackManifest {
  name:        String(!isEmpty)
  version:     String(!isEmpty)
  protocol:    String(this == "v1")
  sdkVersion:  String(!isEmpty)
  bundle:      String(!isEmpty)
  bundleHash:  String(this.startsWith("sha256:"))
  classes:     Listing<String>(!isEmpty)
  description: String?
  homepage:    String?
  license:     String?
}

class PackPolicy {
  allowedSigners: Listing<String> = new {}
  allowUnsigned:  Boolean         = false
}
```

> **Note on `WidgetInstance`'s shape:** the exact field set should match what `dashboards.pkl` had. Step 1's read tells you the current shape; if it differs from the example above (e.g. uses `referencedEntityIds`, `EntitySelector`, etc.), preserve every existing field and constraint when relocating. The point of this task is **schema relocation, not redesign**.

- [ ] **Step 3: Update `dashboards.pkl` to import `WidgetInstance` from `widgets.pkl`**

In `dashboards.pkl`:
- Replace the local `abstract class WidgetInstance { ... }` declaration with `import "switchyard:widgets" as widgets`.
- Replace every reference to the unqualified `WidgetInstance` with `widgets.WidgetInstance`.
- `LeafWidget`, `ContainerWidget`, and `Dashboard` (and their `widgets: Listing<WidgetInstance>` fields) all stay in `dashboards.pkl` — only the abstract base moves.

- [ ] **Step 4: Run the existing Pkl-config tests to confirm nothing breaks**

```bash
go test ./internal/config/...
```

Expected: PASS. If any test fails because a fixture references the old `WidgetInstance` location, update the fixture (most likely under `internal/config/testdata/`).

- [ ] **Step 5: Commit**

```bash
git add internal/config/pkl/switchyard/widgets.pkl internal/config/pkl/switchyard/dashboards.pkl
git commit -m "feat(pkl): full §15.2 PackManifest; relocate WidgetInstance into widgets.pkl"
```

---

## Task 2: Add `widgetPackPolicy` to Pkl + proto

**Files:**
- Modify: `internal/config/pkl/switchyard/policy.pkl`
- Modify: `proto/switchyard/config/v1/config.proto`
- Modify: `internal/config/evaluator_decode.go`

- [ ] **Step 1: Read `policy.pkl` to find the right location for the new field**

```bash
cat internal/config/pkl/switchyard/policy.pkl
```

- [ ] **Step 2: Add `widgetPackPolicy` to `policy.pkl`**

At the appropriate top-level section (next to other policy declarations), add:

```pkl
import "switchyard:widgets" as widgets

widgetPackPolicy: widgets.PackPolicy = new {}
```

If `policy.pkl` is `amends`-shaped rather than an `open module` the right place may differ — match the file's existing convention.

- [ ] **Step 3: Add `WidgetPackPolicy` to the config proto**

In `proto/switchyard/config/v1/config.proto`:

```proto
message WidgetPackPolicy {
  repeated string allowed_signers = 1;
  bool             allow_unsigned  = 2;
}
```

Add the field to `ConfigSnapshot` (or whichever sub-message holds policy config — match existing nesting):

```proto
message ConfigSnapshot {
  // ... existing fields ...
  WidgetPackPolicy widget_pack_policy = N;  // pick the next free tag number
}
```

- [ ] **Step 4: Regenerate proto bindings**

```bash
task proto
```

Expected: clean run; `gen/switchyard/config/v1/...` updated.

- [ ] **Step 5: Decode the new field in `evaluator_decode.go`**

Read the file to find where other top-level fields are decoded; add a parallel block for `widgetPackPolicy`:

```go
if v, ok := raw["widgetPackPolicy"].(map[string]any); ok {
    pol := &configpb.WidgetPackPolicy{}
    if signers, ok := v["allowedSigners"].([]any); ok {
        for _, s := range signers {
            if str, ok := s.(string); ok {
                pol.AllowedSigners = append(pol.AllowedSigners, str)
            }
        }
    }
    if au, ok := v["allowUnsigned"].(bool); ok {
        pol.AllowUnsigned = au
    }
    snap.WidgetPackPolicy = pol
}
```

The exact key paths into `raw` depend on how the existing decoder walks Pkl JSON output — match patterns already in the file. If `evaluator_decode.go` uses a struct-based decode rather than `map[string]any`, add a typed struct field and let the JSON decode populate it.

- [ ] **Step 6: Run config tests**

```bash
go test ./internal/config/...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/config/pkl/switchyard/policy.pkl proto/switchyard/config/v1/config.proto internal/config/evaluator_decode.go gen/switchyard/config/v1/
git commit -m "feat(config): add widgetPackPolicy to Pkl + proto + decoder"
```

---

## Task 3: Define `WidgetPackService` proto

**Files:**
- Create: `proto/switchyard/v1alpha1/widget_pack.proto`

- [ ] **Step 1: Create the proto file**

```proto
syntax = "proto3";

package switchyard.v1alpha1;

import "google/protobuf/timestamp.proto";

option go_package = "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1;v1alpha1";

service WidgetPackService {
  rpc Install   (InstallWidgetPackRequest)   returns (InstallWidgetPackResponse);
  rpc List      (ListWidgetPacksRequest)     returns (ListWidgetPacksResponse);
  rpc Uninstall (UninstallWidgetPackRequest) returns (UninstallWidgetPackResponse);
  rpc Watch     (WatchWidgetPacksRequest)    returns (stream WidgetPackEvent);
}

message InstallWidgetPackRequest  { string ref = 1; }
message InstallWidgetPackResponse { InstalledPack pack = 1; }

message UninstallWidgetPackRequest { string name = 1; string version = 2; bool force = 3; }
message UninstallWidgetPackResponse {}

message ListWidgetPacksRequest  {}
message ListWidgetPacksResponse { repeated InstalledPack packs = 1; }

message WatchWidgetPacksRequest {}
message WidgetPackEvent {
  oneof kind {
    InstalledPack    installed   = 1;
    UninstalledPack  uninstalled = 2;
  }
}
message UninstalledPack { string name = 1; string version = 2; }

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

enum SignatureStatus {
  SIGNATURE_STATUS_UNSPECIFIED = 0;
  SIGNATURE_STATUS_VERIFIED    = 1;
  SIGNATURE_STATUS_UNSIGNED    = 2;
  SIGNATURE_STATUS_INVALID     = 3;
}
```

- [ ] **Step 2: Regenerate**

```bash
task proto
```

Expected: `gen/switchyard/v1alpha1/widget_pack.pb.go` and `gen/switchyard/v1alpha1/switchyardv1alpha1connect/widget_pack.connect.go` are produced.

- [ ] **Step 3: Verify generated code compiles**

```bash
go build ./gen/...
```

Expected: PASS (no output).

- [ ] **Step 4: Commit**

```bash
git add proto/switchyard/v1alpha1/widget_pack.proto gen/switchyard/v1alpha1/
git commit -m "feat(proto): add WidgetPackService"
```

---

## Task 4: Extend `widgetpack.Store` with persistence and Subscribe

**Files:**
- Rewrite: `internal/widgetpack/store.go`
- Rewrite: `internal/widgetpack/store_test.go`

`Store` becomes the source of truth across daemon restarts via `<DataDir>/widgets/.registry.json`, fans out events to subscribers, and handles multi-version coexistence.

- [ ] **Step 1: Write the failing tests first**

Replace `internal/widgetpack/store_test.go` with:

```go
package widgetpack_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fdatoo/switchyard/internal/widgetpack"
)

func TestStore_AddPersistsAndReloads(t *testing.T) {
	dir := t.TempDir()
	s := widgetpack.NewStore(filepath.Join(dir, "widgets"))
	if err := s.Load(context.Background()); err != nil {
		t.Fatalf("Load empty: %v", err)
	}
	if err := s.Add(context.Background(), widgetpack.InstalledPack{
		Name: "p", Version: "1.0.0", SHA256: "sha256:abc",
		Classes: []string{"X"}, SignatureStatus: "verified",
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	// Reopen → entry must be there.
	s2 := widgetpack.NewStore(filepath.Join(dir, "widgets"))
	if err := s2.Load(context.Background()); err != nil {
		t.Fatalf("reload Load: %v", err)
	}
	got, err := s2.Get(context.Background(), "p", "1.0.0")
	if err != nil {
		t.Fatalf("Get after reload: %v", err)
	}
	if got.SHA256 != "sha256:abc" {
		t.Errorf("SHA256=%q, want sha256:abc", got.SHA256)
	}
}

func TestStore_LoadDropsStaleEntries(t *testing.T) {
	dir := t.TempDir()
	s := widgetpack.NewStore(filepath.Join(dir, "widgets"))
	_ = s.Load(context.Background())
	_ = s.Add(context.Background(), widgetpack.InstalledPack{
		Name: "ghost", Version: "1.0.0", SHA256: "sha256:zzz",
	})
	// Don't actually create the pack dir — Load on a fresh store should drop it.
	s2 := widgetpack.NewStore(filepath.Join(dir, "widgets"))
	if err := s2.Load(context.Background()); err != nil {
		t.Fatalf("reload Load: %v", err)
	}
	if _, err := s2.Get(context.Background(), "ghost", "1.0.0"); err == nil {
		t.Error("expected stale entry dropped after Load")
	}
}

func TestStore_SubscribeFanOut(t *testing.T) {
	s := widgetpack.NewStore(t.TempDir())
	_ = s.Load(context.Background())

	chA := make(chan widgetpack.WatchEvent, 4)
	chB := make(chan widgetpack.WatchEvent, 4)
	unsubA := s.Subscribe(chA)
	unsubB := s.Subscribe(chB)
	defer unsubA()
	defer unsubB()

	pack := widgetpack.InstalledPack{Name: "p", Version: "1.0.0", SHA256: "sha256:x"}
	if err := s.Add(context.Background(), pack); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Both subscribers receive the event.
	for i, ch := range []chan widgetpack.WatchEvent{chA, chB} {
		select {
		case ev := <-ch:
			if ev.Installed == nil || ev.Installed.Name != "p" {
				t.Errorf("subscriber %d: bad event %+v", i, ev)
			}
		case <-time.After(time.Second):
			t.Errorf("subscriber %d: no event delivered", i)
		}
	}

	// Unsubscribe A; subsequent event only reaches B.
	unsubA()
	if err := s.Remove(context.Background(), "p", "1.0.0"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	select {
	case ev := <-chB:
		if ev.Uninstalled == nil {
			t.Errorf("expected uninstalled event, got %+v", ev)
		}
	case <-time.After(time.Second):
		t.Error("subscriber B: no uninstall event")
	}
	select {
	case ev := <-chA:
		t.Errorf("unsubscribed A still received event: %+v", ev)
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestStore_MultiVersion(t *testing.T) {
	s := widgetpack.NewStore(t.TempDir())
	_ = s.Load(context.Background())
	for _, v := range []string{"1.0.0", "1.1.0", "2.0.0"} {
		if err := s.Add(context.Background(), widgetpack.InstalledPack{Name: "p", Version: v, SHA256: "sha256:" + v}); err != nil {
			t.Fatalf("Add %s: %v", v, err)
		}
	}
	packs, _ := s.List(context.Background())
	if len(packs) != 3 {
		t.Errorf("List len = %d, want 3", len(packs))
	}
}

func TestStore_ConcurrentAddRemove(t *testing.T) {
	s := widgetpack.NewStore(t.TempDir())
	_ = s.Load(context.Background())
	const N = 50
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			pack := widgetpack.InstalledPack{Name: "p", Version: fmtV(i), SHA256: "sha256:x"}
			_ = s.Add(context.Background(), pack)
			_ = s.Remove(context.Background(), pack.Name, pack.Version)
		}()
	}
	wg.Wait()
}

func fmtV(i int) string { return "1.0." + itoa(i) }

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + itoa(i%10)
}
```

Also extend the `WatchEvent` shape — declare it in the test using the production type once you write the production code in the next step.

- [ ] **Step 2: Rewrite `store.go`**

Replace with:

```go
package widgetpack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrPackNotFound = errors.New("widgetpack: not found")

// InstalledPack describes an installed widget pack.
type InstalledPack struct {
	Name            string
	Version         string
	SHA256          string
	SignatureStatus string // "verified", "unsigned", "invalid"
	SignerIdentity  string
	Classes         []string
	Description     string
	Homepage        string
	License         string
	InstalledAt     time.Time
}

// WatchEvent carries an install/uninstall notification to a Subscribe channel.
// Exactly one of Installed or Uninstalled is non-nil.
type WatchEvent struct {
	Installed   *InstalledPack
	Uninstalled *struct{ Name, Version string }
}

// Store manages the on-disk widget pack registry.
type Store struct {
	root string // <DataDir>/widgets

	mu          sync.RWMutex
	packs       map[string]*InstalledPack // key: name@version
	subscribers map[chan WatchEvent]struct{}
}

// NewStore creates a Store rooted at root. Caller must invoke Load before use.
func NewStore(root string) *Store {
	return &Store{
		root:        root,
		packs:       make(map[string]*InstalledPack),
		subscribers: make(map[chan WatchEvent]struct{}),
	}
}

// Root returns the on-disk root for installed packs.
func (s *Store) Root() string { return s.root }

// Load reads .registry.json and prunes any entries whose pack directory or
// bundle file are missing.
func (s *Store) Load(_ context.Context) error {
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", s.root, err)
	}
	regPath := filepath.Join(s.root, ".registry.json")
	data, err := os.ReadFile(regPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", regPath, err)
	}
	var on disk
	if err := json.Unmarshal(data, &on); err != nil {
		return fmt.Errorf("parse %s: %w", regPath, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stale := false
	for _, p := range on.Packs {
		if !s.dirExists(p.Name, p.Version) {
			stale = true
			continue
		}
		pp := p
		s.packs[p.Name+"@"+p.Version] = &pp
	}
	if stale {
		return s.persistLocked()
	}
	return nil
}

func (s *Store) dirExists(name, version string) bool {
	info, err := os.Stat(filepath.Join(s.root, name, version))
	return err == nil && info.IsDir()
}

// Add registers a pack and persists. Fires an install event to subscribers.
func (s *Store) Add(_ context.Context, pack InstalledPack) error {
	s.mu.Lock()
	if pack.InstalledAt.IsZero() {
		pack.InstalledAt = time.Now().UTC()
	}
	s.packs[pack.Name+"@"+pack.Version] = &pack
	if err := s.persistLocked(); err != nil {
		delete(s.packs, pack.Name+"@"+pack.Version)
		s.mu.Unlock()
		return err
	}
	subs := make([]chan WatchEvent, 0, len(s.subscribers))
	for ch := range s.subscribers {
		subs = append(subs, ch)
	}
	s.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- WatchEvent{Installed: &pack}:
		default:
		}
	}
	return nil
}

// Remove unregisters and persists. Fires an uninstall event.
func (s *Store) Remove(_ context.Context, name, version string) error {
	s.mu.Lock()
	key := name + "@" + version
	if _, ok := s.packs[key]; !ok {
		s.mu.Unlock()
		return ErrPackNotFound
	}
	delete(s.packs, key)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	subs := make([]chan WatchEvent, 0, len(s.subscribers))
	for ch := range s.subscribers {
		subs = append(subs, ch)
	}
	s.mu.Unlock()
	un := &struct{ Name, Version string }{Name: name, Version: version}
	for _, ch := range subs {
		select {
		case ch <- WatchEvent{Uninstalled: un}:
		default:
		}
	}
	return nil
}

// Get returns a pack snapshot or ErrPackNotFound.
func (s *Store) Get(_ context.Context, name, version string) (*InstalledPack, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.packs[name+"@"+version]
	if !ok {
		return nil, ErrPackNotFound
	}
	cp := *p
	return &cp, nil
}

// List returns all installed packs (snapshots).
func (s *Store) List(_ context.Context) ([]InstalledPack, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]InstalledPack, 0, len(s.packs))
	for _, p := range s.packs {
		out = append(out, *p)
	}
	return out, nil
}

// Subscribe registers ch to receive install/uninstall events. Returns an
// unsubscribe func; sends to a full ch are dropped (non-blocking).
func (s *Store) Subscribe(ch chan WatchEvent) func() {
	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		delete(s.subscribers, ch)
		s.mu.Unlock()
	}
}

// persistLocked writes .registry.json atomically. Caller holds s.mu.
func (s *Store) persistLocked() error {
	on := disk{Packs: make([]InstalledPack, 0, len(s.packs))}
	for _, p := range s.packs {
		on.Packs = append(on.Packs, *p)
	}
	data, err := json.MarshalIndent(on, "", "  ")
	if err != nil {
		return err
	}
	regPath := filepath.Join(s.root, ".registry.json")
	tmp := regPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, regPath)
}

type disk struct {
	Packs []InstalledPack `json:"packs"`
}
```

- [ ] **Step 3: Run the tests to confirm pass**

```bash
go test ./internal/widgetpack/... -run TestStore -race
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/widgetpack/store.go internal/widgetpack/store_test.go
git commit -m "feat(widgetpack): on-disk Store with persistence + Subscribe"
```

---

## Task 5: Real cosign keyless verification in `trust.go`

**Files:**
- Rewrite: `internal/widgetpack/trust.go`
- Rewrite: `internal/widgetpack/trust_test.go`
- Create: `internal/widgetpack/testutil_test.go`

This task introduces the test-trust-root infrastructure used by both `trust_test.go` and the integration test (Task 16). It deliberately does the test-helper work first because the verifier is hard to test without it.

- [ ] **Step 1: Add the sigstore-go dependency**

```bash
go get github.com/sigstore/sigstore-go@latest
```

- [ ] **Step 2: Create `testutil_test.go` with the test trust-root helper**

```go
package widgetpack_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/sigstore/sigstore-go/pkg/root"
)

// TestTrustRoot is a test-only sigstore trust root: an in-memory CA cert + a
// signing key that produces certs chained to it, plus a Rekor signing key.
type TestTrustRoot struct {
	CA           *x509.Certificate
	CAKey        *ecdsa.PrivateKey
	RekorKey     *ecdsa.PrivateKey
	TrustedRoot  *root.TrustedRoot
}

func newTestTrustRoot(t *testing.T) *TestTrustRoot {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("CA key: %v", err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-ca"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("CA cert: %v", err)
	}
	caCert, _ := x509.ParseCertificate(caBytes)

	rekorKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("rekor key: %v", err)
	}

	// Build a sigstore-go *root.TrustedRoot from these in-memory artifacts.
	// sigstore-go's NewTrustedRootFromProtobuf takes a TrustedRoot proto;
	// see sigstore-go testdata for an example shape we mirror here.
	tr, err := buildTestTrustedRoot(caCert, &rekorKey.PublicKey)
	if err != nil {
		t.Fatalf("build TrustedRoot: %v", err)
	}

	return &TestTrustRoot{
		CA: caCert, CAKey: caKey, RekorKey: rekorKey, TrustedRoot: tr,
	}
}

// IssueCert issues a Fulcio-style cert binding to the given identity URI.
func (r *TestTrustRoot) IssueCert(t *testing.T, identityURI string) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	leafKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: identityURI},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(10 * time.Minute),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		URIs:         parseURIs(t, identityURI),
	}
	leafBytes, err := x509.CreateCertificate(rand.Reader, tmpl, r.CA, &leafKey.PublicKey, r.CAKey)
	if err != nil {
		t.Fatalf("issue leaf: %v", err)
	}
	leaf, _ := x509.ParseCertificate(leafBytes)
	return leaf, leafKey
}

// buildTestTrustedRoot is implemented inline (rather than from sigstore-go's
// helpers) because the helpers expect serialized JSON. Build the proto in
// memory and pass it to root.NewTrustedRootFromProtobuf.
//
// (Implementation detail elided here — see sigstore-go's
// pkg/root/trusted_root.go and tests for examples.)
func buildTestTrustedRoot(_ *x509.Certificate, _ crypto.PublicKey) (*root.TrustedRoot, error) {
	// The simplest path is to use sigstore-go's ProtobufTrustedRoot and call
	// root.NewTrustedRootFromProtobuf with a hand-built proto containing
	// one CertificateAuthority entry (the test CA) and one TLog entry
	// (the test Rekor pubkey). See the package's TestNewTrustedRootFromPath
	// for a worked example. ~30 lines.
	//
	// TODO during execution: copy the construction pattern from
	// vendor/github.com/sigstore/sigstore-go/pkg/root/trusted_root_test.go.
	panic("implement during execution; see comment")
}

func parseURIs(t *testing.T, s string) []*url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatalf("parse identity URI %q: %v", s, err)
	}
	return []*url.URL{u}
}
```

> **Note:** the `buildTestTrustedRoot` body is the one place this plan punts on showing complete code — sigstore-go's `TrustedRoot` proto construction is verbose and the implementer should mirror sigstore-go's own `pkg/root/trusted_root_test.go` rather than recreate it from scratch. The pattern is well-trodden; pulling the relevant ~30 lines is the right move.

- [ ] **Step 3: Write `trust_test.go`**

```go
package widgetpack_test

import (
	"context"
	"testing"

	"github.com/fdatoo/switchyard/internal/widgetpack"
)

func TestVerify_AllowedSignerGlob(t *testing.T) {
	root := newTestTrustRoot(t)
	leaf, leafKey := root.IssueCert(t, "https://github.com/myhandle/foo")
	bundle, sig := signBlobWithLeaf(t, []byte("payload"), leaf, leafKey, root)

	v := widgetpack.NewVerifier(root.TrustedRoot)
	pol := &widgetpack.TrustPolicy{}
	pol.Set([]string{"https://github.com/myhandle/*"}, false)

	res, err := v.Verify(context.Background(), []byte("payload"), bundle, sig, pol)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if res.Status != "verified" {
		t.Errorf("Status=%q, want verified", res.Status)
	}
	if res.SignerIdentity != "https://github.com/myhandle/foo" {
		t.Errorf("Identity=%q, want full URI", res.SignerIdentity)
	}
}

func TestVerify_SignerGlob_NoMatch_Rejected(t *testing.T) {
	root := newTestTrustRoot(t)
	leaf, leafKey := root.IssueCert(t, "https://github.com/randomattacker/foo")
	bundle, sig := signBlobWithLeaf(t, []byte("payload"), leaf, leafKey, root)

	v := widgetpack.NewVerifier(root.TrustedRoot)
	pol := &widgetpack.TrustPolicy{}
	pol.Set([]string{"https://github.com/myhandle/*"}, false)

	if _, err := v.Verify(context.Background(), []byte("payload"), bundle, sig, pol); err == nil {
		t.Error("expected rejection for unmatched signer identity")
	}
}

func TestVerify_NoSignature_AllowUnsigned(t *testing.T) {
	v := widgetpack.NewVerifier(newTestTrustRoot(t).TrustedRoot)
	pol := &widgetpack.TrustPolicy{}
	pol.Set(nil, true)
	res, err := v.Verify(context.Background(), []byte("payload"), nil, nil, pol)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if res.Status != "unsigned" {
		t.Errorf("Status=%q, want unsigned", res.Status)
	}
}

func TestVerify_NoSignature_DenyUnsigned(t *testing.T) {
	v := widgetpack.NewVerifier(newTestTrustRoot(t).TrustedRoot)
	pol := &widgetpack.TrustPolicy{}
	pol.Set(nil, false)
	if _, err := v.Verify(context.Background(), []byte("payload"), nil, nil, pol); err == nil {
		t.Error("expected rejection for unsigned with allowUnsigned=false")
	}
}

func TestVerify_BundleMismatch_Rejected(t *testing.T) {
	root := newTestTrustRoot(t)
	leaf, leafKey := root.IssueCert(t, "https://github.com/myhandle/foo")
	bundle, sig := signBlobWithLeaf(t, []byte("payload-A"), leaf, leafKey, root)

	v := widgetpack.NewVerifier(root.TrustedRoot)
	pol := &widgetpack.TrustPolicy{}
	pol.Set([]string{"https://github.com/myhandle/*"}, false)

	if _, err := v.Verify(context.Background(), []byte("payload-B"), bundle, sig, pol); err == nil {
		t.Error("expected rejection for payload mismatch")
	}
}
```

Add `signBlobWithLeaf` to `testutil_test.go` — it builds a sigstore Bundle using the leaf cert + key + the test Rekor key (to produce a valid TLog entry). Mirror sigstore-go's signing tests.

- [ ] **Step 4: Write `trust.go`**

```go
package widgetpack

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"path"
	"sync"

	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

// TrustPolicy is the in-memory mirror of switchyard.widgets.PackPolicy.
type TrustPolicy struct {
	mu             sync.RWMutex
	allowedSigners []string
	allowUnsigned  bool
}

// Set replaces the policy. Safe to call from config-reload callbacks.
func (p *TrustPolicy) Set(signers []string, allowUnsigned bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.allowedSigners = append([]string(nil), signers...)
	p.allowUnsigned = allowUnsigned
}

func (p *TrustPolicy) snapshot() (signers []string, allowUnsigned bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return append([]string(nil), p.allowedSigners...), p.allowUnsigned
}

// VerificationResult is what Verify returns when verification succeeds (or
// when the policy permits unsigned).
type VerificationResult struct {
	Status         string // "verified" | "unsigned"
	SignerIdentity string // empty for unsigned
}

// Verifier wraps a sigstore-go verifier rooted at a TrustedRoot. Production
// uses sigstore's default TUF root; tests inject an in-memory root.
type Verifier struct {
	trustedRoot *root.TrustedRoot
}

// NewVerifier constructs a Verifier from a sigstore TrustedRoot.
func NewVerifier(tr *root.TrustedRoot) *Verifier { return &Verifier{trustedRoot: tr} }

// Verify checks that signatureBundle is a valid cosign keyless signature over
// payload, that the cert chains to the trusted root, and that the cert
// subject identity matches one of pol.allowedSigners (path.Match glob).
//
// signatureBundle is the sigstore Bundle (cert + sig + Rekor entry); rawSig is
// the raw cosign signature blob. Tests pass nil for both to exercise the
// unsigned path.
func (v *Verifier) Verify(
	ctx context.Context,
	payload []byte,
	signatureBundle []byte,
	rawSig []byte,
	pol *TrustPolicy,
) (*VerificationResult, error) {
	signers, allowUnsigned := pol.snapshot()

	if signatureBundle == nil && rawSig == nil {
		if !allowUnsigned {
			return nil, errors.New("widgetpack: unsigned pack rejected by trust policy")
		}
		return &VerificationResult{Status: "unsigned"}, nil
	}

	// Build verifier with the bundled trusted root.
	sv, err := verify.NewSignedEntityVerifier(
		v.trustedRoot,
		verify.WithSignedTimestamps(0),
		verify.WithTransparencyLog(1),
		verify.WithObserverTimestamps(1),
	)
	if err != nil {
		return nil, fmt.Errorf("widgetpack: build verifier: %w", err)
	}
	// Decode the bundle, run sigstore-go's verification.
	res, err := runVerify(ctx, sv, payload, signatureBundle, rawSig)
	if err != nil {
		return nil, err
	}
	identity := certIdentity(res.LeafCert)
	if !globMatchAny(signers, identity) {
		return nil, fmt.Errorf("widgetpack: signer identity %q not in allowedSigners", identity)
	}
	return &VerificationResult{Status: "verified", SignerIdentity: identity}, nil
}

// runVerify decodes a sigstore Bundle and runs verification. The exact
// sigstore-go API call used here depends on the bundle format produced by the
// signer; see the sigstore-go README for current shape.
func runVerify(_ context.Context, _ *verify.SignedEntityVerifier, _ []byte, _ []byte, _ []byte) (*verify.VerificationResult, error) {
	// IMPLEMENT: parse the protobuf-bundle, build verify.Input, call
	// SignedEntityVerifier.Verify(input). See sigstore-go README.
	return nil, errors.New("not implemented")
}

func certIdentity(c *x509.Certificate) string {
	if c == nil {
		return ""
	}
	for _, u := range c.URIs {
		return u.String()
	}
	return c.Subject.CommonName
}

func globMatchAny(patterns []string, s string) bool {
	if s == "" {
		return false
	}
	for _, p := range patterns {
		if ok, _ := path.Match(p, s); ok {
			return true
		}
	}
	return false
}
```

The two `IMPLEMENT:` markers (`buildTestTrustedRoot` in step 2; `runVerify` here) are the deliberate hand-offs to the live sigstore-go API. Do not invent their bodies; copy from the sigstore-go test fixtures and README, which are stable.

- [ ] **Step 5: Run tests**

```bash
go test ./internal/widgetpack/... -run TestVerify -race
```

Expected: PASS once both `IMPLEMENT:` markers are filled in.

- [ ] **Step 6: Commit**

```bash
git add internal/widgetpack/trust.go internal/widgetpack/trust_test.go internal/widgetpack/testutil_test.go go.mod go.sum
git commit -m "feat(widgetpack): real cosign keyless verification via sigstore-go"
```

---

## Task 6: OCI pull via `oras-go`

**Files:**
- Create: `internal/widgetpack/oci.go`

This task fetches the artifact bytes plus the cosign signature artifact. It does *not* extract the tarball (that's Task 9's responsibility, since extraction has to happen in the staging dir owned by Install).

- [ ] **Step 1: Add the dependency**

```bash
go get oras.land/oras-go/v2@latest
```

- [ ] **Step 2: Write `oci.go`**

```go
package widgetpack

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

// MediaType is the layer media type for switchyard widget pack artifacts.
const MediaType = "application/vnd.switchyard.widgetpack.v1+tar+gzip"

// FetchedArtifact is the result of pulling an artifact from a registry.
type FetchedArtifact struct {
	LayerBlob []byte // gzipped tarball; caller un-tars
	Digest    string // "sha256:..."
	// SignatureBundle is the cosign sigstore-bundle blob if present; nil if
	// no signature artifact exists at <ref>.sig.
	SignatureBundle []byte
}

// Fetcher pulls OCI artifacts plus their cosign signature artifacts.
type Fetcher struct {
	credStore credentials.Store
}

// NewFetcher returns a Fetcher that authenticates against registries using
// ~/.docker/config.json (anonymous access if not present).
func NewFetcher() (*Fetcher, error) {
	cs, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err != nil {
		return nil, fmt.Errorf("widgetpack: docker credentials: %w", err)
	}
	return &Fetcher{credStore: cs}, nil
}

// Fetch pulls the artifact at ref and (if present) its cosign signature
// at <ref>.sig. Rejects multi-layer artifacts and artifacts whose layer
// media type is not MediaType.
func (f *Fetcher) Fetch(ctx context.Context, ref string) (*FetchedArtifact, error) {
	repo, tag, err := parseRef(ref)
	if err != nil {
		return nil, err
	}
	r, err := remote.NewRepository(repo)
	if err != nil {
		return nil, fmt.Errorf("widgetpack: open repo: %w", err)
	}
	r.Client = &auth.Client{
		Credential: credentials.Credential(f.credStore),
	}

	// Pull the artifact into an in-memory store.
	store := memory.New()
	desc, err := oras.Copy(ctx, r, tag, store, tag, oras.DefaultCopyOptions)
	if err != nil {
		return nil, fmt.Errorf("widgetpack: pull %s: %w", ref, err)
	}

	// Walk manifest to find the single layer.
	manifestBytes, err := readBlob(ctx, store, desc)
	if err != nil {
		return nil, fmt.Errorf("widgetpack: read manifest: %w", err)
	}
	layerDesc, err := singleLayerDescriptor(manifestBytes)
	if err != nil {
		return nil, fmt.Errorf("widgetpack: %s: %w", ref, err)
	}
	if layerDesc.MediaType != MediaType {
		return nil, fmt.Errorf("widgetpack: unexpected media type %q (want %q)", layerDesc.MediaType, MediaType)
	}
	layerBlob, err := readBlob(ctx, store, layerDesc)
	if err != nil {
		return nil, fmt.Errorf("widgetpack: read layer: %w", err)
	}

	// Best-effort fetch the cosign signature artifact at <ref>.sig.
	sigTag := cosignSigTagFor(layerDesc.Digest.String())
	sigBundle, _ := f.fetchSignature(ctx, r, sigTag)

	return &FetchedArtifact{
		LayerBlob:       layerBlob,
		Digest:          layerDesc.Digest.String(),
		SignatureBundle: sigBundle,
	}, nil
}

func (f *Fetcher) fetchSignature(ctx context.Context, r *remote.Repository, sigTag string) ([]byte, error) {
	store := memory.New()
	desc, err := oras.Copy(ctx, r, sigTag, store, sigTag, oras.DefaultCopyOptions)
	if err != nil {
		// No signature artifact — not an error.
		return nil, err
	}
	manifestBytes, err := readBlob(ctx, store, desc)
	if err != nil {
		return nil, err
	}
	layerDesc, err := singleLayerDescriptor(manifestBytes)
	if err != nil {
		return nil, err
	}
	return readBlob(ctx, store, layerDesc)
}

// readBlob is a thin io.ReadAll wrapper around store.Fetch.
func readBlob(ctx context.Context, store *memory.Store, desc ocispec.Descriptor) ([]byte, error) {
	rc, err := store.Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// singleLayerDescriptor parses an OCI manifest and returns its single layer.
// Errors if the manifest has zero or more than one layer.
func singleLayerDescriptor(manifest []byte) (ocispec.Descriptor, error) {
	// Parse manifest JSON; return manifest.Layers[0] if len(Layers) == 1.
	// IMPLEMENT during execution: use github.com/opencontainers/image-spec/specs-go/v1
	// for the type. ~10 lines.
	return ocispec.Descriptor{}, errors.New("not implemented")
}

// cosignSigTagFor turns "sha256:abc" into "sha256-abc.sig" — cosign's tag scheme.
func cosignSigTagFor(digest string) string {
	parts := strings.SplitN(digest, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	return parts[0] + "-" + parts[1] + ".sig"
}

func parseRef(ref string) (repo, tag string, err error) {
	idx := strings.LastIndex(ref, ":")
	if idx <= 0 || idx == len(ref)-1 {
		return "", "", fmt.Errorf("widgetpack: bad ref %q (need repo:tag)", ref)
	}
	return ref[:idx], ref[idx+1:], nil
}
```

> **`ocispec`:** add `import ocispec "github.com/opencontainers/image-spec/specs-go/v1"` — `oras-go` already pulls this.

> **`singleLayerDescriptor`:** the implementation is mechanical — `json.Unmarshal` into `ocispec.Manifest`, return `m.Layers[0]` if `len(m.Layers) == 1` else error. Plan punts on fully writing it because the `ocispec.Manifest` shape is stable and the implementer can read it from the package docs without inventing.

- [ ] **Step 3: No unit tests yet**

The `Fetcher` is exercised by Task 16's integration test against an in-process registry. Unit-testing it in isolation requires the same plumbing as the integration test, so we defer.

- [ ] **Step 4: Compile-only check**

```bash
go build ./internal/widgetpack/...
```

Expected: PASS once `singleLayerDescriptor` is filled in.

- [ ] **Step 5: Commit**

```bash
git add internal/widgetpack/oci.go go.mod go.sum
git commit -m "feat(widgetpack): OCI artifact pull via oras-go"
```

---

## Task 7: Manifest validation via Pkl evaluator

**Files:**
- Create: `internal/widgetpack/manifest.go`
- Create: `internal/widgetpack/manifest_test.go`

- [ ] **Step 1: Write `manifest_test.go`**

```go
package widgetpack_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fdatoo/switchyard/internal/widgetpack"
)

const validManifest = `
@ModuleInfo { minPklVersion = "0.27.0" }
amends "switchyard:widgets"

manifest = new PackManifest {
  name = "bar-widgets"
  version = "1.0.0"
  protocol = "v1"
  sdkVersion = "1.0.0"
  bundle = "bundle.js"
  bundleHash = "sha256:abc"
  classes = new { "BarChart"; "PieChart" }
}
`

func TestEvalManifest_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.pkl")
	if err := os.WriteFile(path, []byte(validManifest), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := widgetpack.EvalManifest(context.Background(), path)
	if err != nil {
		t.Fatalf("EvalManifest: %v", err)
	}
	if m.Name != "bar-widgets" {
		t.Errorf("Name = %q", m.Name)
	}
	if len(m.Classes) != 2 {
		t.Errorf("Classes len = %d", len(m.Classes))
	}
}

func TestEvalManifest_MissingRequired(t *testing.T) {
	bad := strings.Replace(validManifest, "name = \"bar-widgets\"", "", 1)
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.pkl")
	_ = os.WriteFile(path, []byte(bad), 0o600)
	if _, err := widgetpack.EvalManifest(context.Background(), path); err == nil {
		t.Error("expected EvalManifest to fail on missing name")
	}
}

func TestEvalManifest_BadProtocol(t *testing.T) {
	bad := strings.Replace(validManifest, "protocol = \"v1\"", "protocol = \"v2\"", 1)
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.pkl")
	_ = os.WriteFile(path, []byte(bad), 0o600)
	if _, err := widgetpack.EvalManifest(context.Background(), path); err == nil {
		t.Error("expected EvalManifest to reject non-v1 protocol")
	}
}

func TestEvalManifest_BadBundleHash(t *testing.T) {
	bad := strings.Replace(validManifest, "bundleHash = \"sha256:abc\"", "bundleHash = \"md5:abc\"", 1)
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.pkl")
	_ = os.WriteFile(path, []byte(bad), 0o600)
	if _, err := widgetpack.EvalManifest(context.Background(), path); err == nil {
		t.Error("expected EvalManifest to reject non-sha256 bundleHash")
	}
}
```

- [ ] **Step 2: Write `manifest.go`**

```go
package widgetpack

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/apple/pkl-go/pkl"
)

// Manifest mirrors switchyard.widgets.PackManifest.
type Manifest struct {
	Name        string   `pkl:"name"        json:"name"`
	Version     string   `pkl:"version"     json:"version"`
	Protocol    string   `pkl:"protocol"    json:"protocol"`
	SDKVersion  string   `pkl:"sdkVersion"  json:"sdkVersion"`
	Bundle      string   `pkl:"bundle"      json:"bundle"`
	BundleHash  string   `pkl:"bundleHash"  json:"bundleHash"`
	Classes     []string `pkl:"classes"     json:"classes"`
	Description string   `pkl:"description" json:"description"`
	Homepage    string   `pkl:"homepage"    json:"homepage"`
	License     string   `pkl:"license"     json:"license"`
}

// EvalManifest evaluates a manifest.pkl file using a fresh Pkl evaluator and
// returns the decoded Manifest. The Pkl module's class constraints (e.g.
// protocol == "v1", bundleHash startsWith "sha256:") become evaluator errors
// here, which is the validation we want.
func EvalManifest(ctx context.Context, manifestPath string) (*Manifest, error) {
	ev, err := pkl.NewEvaluator(ctx, pkl.PreconfiguredOptions)
	if err != nil {
		return nil, fmt.Errorf("widgetpack: pkl evaluator: %w", err)
	}
	defer ev.Close()

	jsonBytes, err := ev.EvaluateOutputBytes(ctx, pkl.FileSource(manifestPath))
	if err != nil {
		return nil, fmt.Errorf("widgetpack: evaluate manifest: %w", err)
	}

	var wrapper struct {
		Manifest Manifest `json:"manifest"`
	}
	if err := json.Unmarshal(jsonBytes, &wrapper); err != nil {
		return nil, fmt.Errorf("widgetpack: decode manifest: %w", err)
	}
	if wrapper.Manifest.Name == "" {
		return nil, fmt.Errorf("widgetpack: manifest missing required fields")
	}
	return &wrapper.Manifest, nil
}
```

> **Note on the `amends "switchyard:widgets"` import in test fixtures:** the Pkl evaluator needs to resolve `switchyard:widgets`. The existing config evaluator setup (`internal/config/evaluator.go::newPklEvaluator`) registers a `switchyard:` ModuleReader. For the manifest evaluator, we need the same reader registered. If `pkl.PreconfiguredOptions` doesn't pull it in, copy the option from `evaluator.go` — likely a `pkl.WithCustomModuleReader(...)`.

- [ ] **Step 3: Run tests**

```bash
go test ./internal/widgetpack/... -run TestEvalManifest
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/widgetpack/manifest.go internal/widgetpack/manifest_test.go
git commit -m "feat(widgetpack): Pkl-evaluator-driven manifest validation"
```

---

## Task 8: Bundle HTTP handler

**Files:**
- Create: `internal/widgetpack/serve.go`
- Create: `internal/widgetpack/serve_test.go`

- [ ] **Step 1: Write `serve_test.go`**

```go
package widgetpack_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/fdatoo/switchyard/internal/widgetpack"
)

func TestBundleHandler_GET(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "bar/1.0.0/bundle.js"), []byte("export const X=1;"))
	store := widgetpack.NewStore(root)
	_ = store.Load(context.Background())
	_ = store.Add(context.Background(), widgetpack.InstalledPack{
		Name: "bar", Version: "1.0.0", SHA256: "sha256:hashval",
	})
	h := widgetpack.NewBundleHandler(store)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/widgets/bar/1.0.0/bundle.js")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if got := resp.Header.Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control=%q", got)
	}
	if got := resp.Header.Get("Content-Type"); got != "text/javascript" {
		t.Errorf("Content-Type=%q", got)
	}
	if got := resp.Header.Get("ETag"); got != `"sha256:hashval"` {
		t.Errorf("ETag=%q", got)
	}
}

func TestBundleHandler_PathTraversal(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "bar/1.0.0/bundle.js"), []byte("ok"))
	mustWrite(t, filepath.Join(root, "secret.txt"), []byte("secret"))
	store := widgetpack.NewStore(root)
	_ = store.Load(context.Background())
	_ = store.Add(context.Background(), widgetpack.InstalledPack{Name: "bar", Version: "1.0.0", SHA256: "sha256:x"})
	h := widgetpack.NewBundleHandler(store)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/widgets/bar/1.0.0/../../secret.txt")
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Error("path traversal not blocked")
	}
}

func TestBundleHandler_UnknownPack(t *testing.T) {
	root := t.TempDir()
	store := widgetpack.NewStore(root)
	_ = store.Load(context.Background())
	h := widgetpack.NewBundleHandler(store)
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, _ := http.Get(srv.URL + "/widgets/unknown/1.0.0/bundle.js")
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("status=%d, want 404", resp.StatusCode)
	}
}

func TestBundleHandler_MethodNotAllowed(t *testing.T) {
	root := t.TempDir()
	store := widgetpack.NewStore(root)
	_ = store.Load(context.Background())
	h := widgetpack.NewBundleHandler(store)
	srv := httptest.NewServer(h)
	defer srv.Close()
	req, _ := http.NewRequest("POST", srv.URL+"/widgets/bar/1.0.0/bundle.js", nil)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != 405 {
		t.Errorf("status=%d, want 405", resp.StatusCode)
	}
}

func TestBundleHandler_IfNoneMatch(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "bar/1.0.0/bundle.js"), []byte("ok"))
	store := widgetpack.NewStore(root)
	_ = store.Load(context.Background())
	_ = store.Add(context.Background(), widgetpack.InstalledPack{Name: "bar", Version: "1.0.0", SHA256: "sha256:hashval"})
	h := widgetpack.NewBundleHandler(store)
	srv := httptest.NewServer(h)
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL+"/widgets/bar/1.0.0/bundle.js", nil)
	req.Header.Set("If-None-Match", `"sha256:hashval"`)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != 304 {
		t.Errorf("status=%d, want 304", resp.StatusCode)
	}
}

func mustWrite(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Write `serve.go`**

```go
package widgetpack

import (
	"context"
	"net/http"
	"path"
	"path/filepath"
	"strings"
)

// NewBundleHandler returns an http.Handler for /widgets/<pack>/<version>/<file>.
// It serves files only for packs known to store; unknown packs return 404 even
// if the file exists on disk (e.g. mid-install staging).
func NewBundleHandler(store *Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Trim "/widgets/" prefix.
		const prefix = "/widgets/"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			http.NotFound(w, r)
			return
		}
		rel := strings.TrimPrefix(r.URL.Path, prefix)
		clean := path.Clean("/" + rel)
		if !strings.HasPrefix(clean, "/") || strings.Contains(clean, "..") {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		parts := strings.SplitN(strings.TrimPrefix(clean, "/"), "/", 3)
		if len(parts) < 3 {
			http.NotFound(w, r)
			return
		}
		pack, version, file := parts[0], parts[1], parts[2]

		p, err := store.Get(context.Background(), pack, version)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		etag := `"` + p.SHA256 + `"`
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		fullPath := filepath.Join(store.Root(), pack, version, file)
		// Re-check escape after Clean+Join.
		expectedPrefix := filepath.Join(store.Root(), pack, version) + string(filepath.Separator)
		if !strings.HasPrefix(fullPath, expectedPrefix) {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}

		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", contentTypeFor(file))
		http.ServeFile(w, r, fullPath)
	})
}

func contentTypeFor(name string) string {
	switch filepath.Ext(name) {
	case ".js", ".mjs":
		return "text/javascript"
	case ".map":
		return "application/json"
	case ".css":
		return "text/css"
	default:
		return "application/octet-stream"
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/widgetpack/... -run TestBundleHandler
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/widgetpack/serve.go internal/widgetpack/serve_test.go
git commit -m "feat(widgetpack): bundle HTTP handler with immutable cache"
```

---

## Task 9: Rewrite `Installer.Install` to chain all steps

**Files:**
- Rewrite: `internal/widgetpack/install.go`
- Rewrite: `internal/widgetpack/install_test.go` (just to keep the existing trivial tests as smoke tests)

- [ ] **Step 1: Rewrite `install.go`**

```go
package widgetpack

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// HostSDKVersion is the @switchyard/widget-sdk version this build is
// compatible with. Bumped on SDK breaking changes; a manifest's sdkVersion
// must match this in major-version semver.
const HostSDKVersion = "1.0.0"

// ErrInstallFailed is returned for any install failure.
var ErrInstallFailed = errors.New("widgetpack: install failed")

// FailureReason is a stable string carried in error details so callers can
// map errors to user-facing messages without parsing the message itself.
type FailureReason string

const (
	ReasonBadRef             FailureReason = "bad_ref"
	ReasonRegistryUnreachable FailureReason = "registry_unreachable"
	ReasonBadArtifact        FailureReason = "bad_artifact"
	ReasonSignatureInvalid   FailureReason = "signature_invalid"
	ReasonHashMismatch       FailureReason = "hash_mismatch"
	ReasonSDKIncompatible    FailureReason = "sdk_incompatible"
	ReasonClassCollision     FailureReason = "class_collision"
	ReasonManifestInvalid    FailureReason = "manifest_invalid"
	ReasonAlreadyExists      FailureReason = "already_exists"
)

// FailureError is returned from Install for known failure modes. Callers can
// use errors.As to extract the Reason and feed it into a Connect error detail.
type FailureError struct {
	Reason FailureReason
	Err    error
}

func (e *FailureError) Error() string { return string(e.Reason) + ": " + e.Err.Error() }
func (e *FailureError) Unwrap() error { return e.Err }

// InstallRequest carries the parameters for a pack installation.
type InstallRequest struct {
	Ref string
}

// Installer chains OCI pull, cosign verify, manifest validate, hash check,
// SDK check, class-collision check, atomic commit, event emit.
type Installer struct {
	store    *Store
	verifier *Verifier
	policy   *TrustPolicy
	fetcher  *Fetcher
	dataDir  string
	builtinClasses []string

	muInflight sync.Map // key string -> *sync.Mutex
}

// NewInstaller wires the install pipeline. builtinClasses is the set of
// builtin class IDs (e.g. "Gauge", "EntityToggle") used for collision checks.
func NewInstaller(
	store *Store, verifier *Verifier, policy *TrustPolicy, fetcher *Fetcher,
	dataDir string, builtinClasses []string,
) *Installer {
	return &Installer{
		store: store, verifier: verifier, policy: policy, fetcher: fetcher,
		dataDir: dataDir, builtinClasses: builtinClasses,
	}
}

// Install runs the full §15.4 flow.
func (i *Installer) Install(ctx context.Context, req InstallRequest) (*InstalledPack, error) {
	if req.Ref == "" {
		return nil, &FailureError{Reason: ReasonBadRef, Err: errors.New("ref required")}
	}

	// 1. Pull artifact + signature.
	art, err := i.fetcher.Fetch(ctx, req.Ref)
	if err != nil {
		return nil, &FailureError{Reason: ReasonRegistryUnreachable, Err: err}
	}

	// 2. Verify signature against trust policy.
	vres, err := i.verifier.Verify(ctx, art.LayerBlob, art.SignatureBundle, nil, i.policy)
	if err != nil {
		return nil, &FailureError{Reason: ReasonSignatureInvalid, Err: err}
	}

	// 3. Stage to <DataDir>/widgets/.staging/<rand>/.
	stagingRoot := filepath.Join(i.store.Root(), ".staging")
	if err := os.MkdirAll(stagingRoot, 0o755); err != nil {
		return nil, fmt.Errorf("%w: mkdir staging: %v", ErrInstallFailed, err)
	}
	stagingDir, err := os.MkdirTemp(stagingRoot, "pack-")
	if err != nil {
		return nil, fmt.Errorf("%w: stage tmp: %v", ErrInstallFailed, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(stagingDir)
		}
	}()

	if err := untarGz(art.LayerBlob, stagingDir); err != nil {
		return nil, &FailureError{Reason: ReasonBadArtifact, Err: err}
	}

	// 4. Manifest validate.
	manifest, err := EvalManifest(ctx, filepath.Join(stagingDir, "manifest.pkl"))
	if err != nil {
		return nil, &FailureError{Reason: ReasonManifestInvalid, Err: err}
	}

	// 5. Hash verify.
	bundlePath := filepath.Join(stagingDir, manifest.Bundle)
	bundleSHA, err := sha256File(bundlePath)
	if err != nil {
		return nil, &FailureError{Reason: ReasonBadArtifact, Err: err}
	}
	if "sha256:"+bundleSHA != manifest.BundleHash {
		return nil, &FailureError{
			Reason: ReasonHashMismatch,
			Err: fmt.Errorf("computed sha256:%s, manifest %s", bundleSHA, manifest.BundleHash),
		}
	}

	// 6. SDK compatibility (major-only for v1).
	if !semverMajorEqual(manifest.SDKVersion, HostSDKVersion) {
		return nil, &FailureError{
			Reason: ReasonSDKIncompatible,
			Err: fmt.Errorf("manifest sdkVersion=%s host=%s", manifest.SDKVersion, HostSDKVersion),
		}
	}

	// 7. Class collisions.
	if err := i.checkCollisions(ctx, manifest); err != nil {
		return nil, &FailureError{Reason: ReasonClassCollision, Err: err}
	}

	// Per-(name@version) install-mutex to serialize concurrent attempts.
	key := manifest.Name + "@" + manifest.Version
	muIface, _ := i.muInflight.LoadOrStore(key, &sync.Mutex{})
	mu := muIface.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	// 7b. Already-exists check.
	if _, err := i.store.Get(ctx, manifest.Name, manifest.Version); err == nil {
		return nil, &FailureError{Reason: ReasonAlreadyExists, Err: errors.New(key)}
	}

	// 8. Commit: atomic rename staging → final.
	finalDir := filepath.Join(i.store.Root(), manifest.Name, manifest.Version)
	if err := os.MkdirAll(filepath.Dir(finalDir), 0o755); err != nil {
		return nil, fmt.Errorf("%w: mkdir parent: %v", ErrInstallFailed, err)
	}
	if err := os.Rename(stagingDir, finalDir); err != nil {
		return nil, fmt.Errorf("%w: rename: %v", ErrInstallFailed, err)
	}
	committed = true

	pack := InstalledPack{
		Name:            manifest.Name,
		Version:         manifest.Version,
		SHA256:          "sha256:" + bundleSHA,
		SignatureStatus: vres.Status,
		SignerIdentity:  vres.SignerIdentity,
		Classes:         manifest.Classes,
		Description:     manifest.Description,
		Homepage:        manifest.Homepage,
		License:         manifest.License,
	}
	if err := i.store.Add(ctx, pack); err != nil {
		// Narrow rollback window: the rename succeeded, so revert.
		_ = os.RemoveAll(finalDir)
		return nil, fmt.Errorf("%w: store.Add: %v", ErrInstallFailed, err)
	}
	return &pack, nil
}

func (i *Installer) checkCollisions(ctx context.Context, m *Manifest) error {
	taken := make(map[string]bool)
	for _, b := range i.builtinClasses {
		taken[b] = true
	}
	packs, _ := i.store.List(ctx)
	for _, p := range packs {
		if p.Name == m.Name && p.Version == m.Version {
			continue
		}
		for _, c := range p.Classes {
			taken[p.Name+"/"+c] = true
		}
	}
	for _, c := range m.Classes {
		if taken[m.Name+"/"+c] || taken[c] {
			return fmt.Errorf("class %q collides", c)
		}
	}
	return nil
}

func untarGz(blob []byte, dest string) error {
	gz, err := gzip.NewReader(bytesReader(blob))
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		clean := filepath.Clean(hdr.Name)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			return fmt.Errorf("path escape: %s", hdr.Name)
		}
		full := filepath.Join(dest, clean)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(full, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return err
			}
			_ = f.Close()
		default:
			// Skip symlinks, devices, etc. — never legitimate in a widget pack.
		}
	}
}

func bytesReader(b []byte) *byteReader { return &byteReader{b: b} }

type byteReader struct{ b []byte; off int }

func (r *byteReader) Read(p []byte) (int, error) {
	if r.off >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.off:])
	r.off += n
	return n, nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// semverMajorEqual returns true if a and b have equal major versions.
func semverMajorEqual(a, b string) bool {
	ma, err1 := majorOf(a)
	mb, err2 := majorOf(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return ma == mb
}

func majorOf(v string) (int, error) {
	v = strings.TrimPrefix(v, "v")
	end := strings.IndexAny(v, ".+-")
	if end < 0 {
		end = len(v)
	}
	return strconv.Atoi(v[:end])
}
```

(Replace `bytesReader` with `bytes.NewReader` from the `bytes` package; the inline reader above is just to avoid an extra import in the plan body. Use the standard library type at execution time.)

- [ ] **Step 2: Drop the now-trivial `install_test.go` content (Task 16 covers behavior)**

Replace `internal/widgetpack/install_test.go` with a single bad-input smoke test:

```go
package widgetpack_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fdatoo/switchyard/internal/widgetpack"
)

func TestInstaller_Install_BadRef(t *testing.T) {
	inst := widgetpack.NewInstaller(nil, nil, nil, nil, "", nil)
	_, err := inst.Install(context.Background(), widgetpack.InstallRequest{Ref: ""})
	var fe *widgetpack.FailureError
	if !errors.As(err, &fe) || fe.Reason != widgetpack.ReasonBadRef {
		t.Errorf("err = %v", err)
	}
}
```

- [ ] **Step 3: Compile-only check**

```bash
go build ./internal/widgetpack/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/widgetpack/install.go internal/widgetpack/install_test.go
git commit -m "feat(widgetpack): full §15.4 install flow"
```

---

## Task 10: Implement `Installer.Uninstall`

**Files:**
- Modify: `internal/widgetpack/install.go`

`Uninstall` adds reference checking against the dashboard backend. Today the reference check is effectively a no-op (the dashboard backend stub returns no dashboards), but the code path is in place for when F-156 lands.

- [ ] **Step 1: Add a small `DashboardLister` interface and `Uninstall` to `install.go`**

Append to `install.go`:

```go
// DashboardLister is the subset of dashboard.Backend that Uninstall queries
// to build the in-use class set. Today (F-156 unimplemented) the only
// production binding returns an empty list — Uninstall always proceeds.
type DashboardLister interface {
	ClassRefs(ctx context.Context) ([]string, error) // list of "<pack>/<class>" or builtin class IDs in any dashboard
}

// emptyDashboardLister is the default; replace via Installer.SetDashboardLister.
type emptyDashboardLister struct{}

func (emptyDashboardLister) ClassRefs(_ context.Context) ([]string, error) { return nil, nil }

// SetDashboardLister wires a real lister once F-156 lands.
func (i *Installer) SetDashboardLister(d DashboardLister) { i.dl = d }

// Uninstall removes a pack. With force=false, returns an error if any
// dashboard references one of the pack's classes.
func (i *Installer) Uninstall(ctx context.Context, name, version string, force bool) error {
	pack, err := i.store.Get(ctx, name, version)
	if err != nil {
		return err
	}
	if !force {
		dl := i.dl
		if dl == nil {
			dl = emptyDashboardLister{}
		}
		refs, err := dl.ClassRefs(ctx)
		if err != nil {
			return fmt.Errorf("widgetpack: list class refs: %w", err)
		}
		inUse := make([]string, 0)
		for _, c := range pack.Classes {
			full := name + "/" + c
			for _, ref := range refs {
				if ref == full {
					inUse = append(inUse, full)
					break
				}
			}
		}
		if len(inUse) > 0 {
			return fmt.Errorf("widgetpack: pack %s in use by classes %v", pack.Name, inUse)
		}
	}
	if err := os.RemoveAll(filepath.Join(i.store.Root(), name, version)); err != nil {
		return fmt.Errorf("widgetpack: remove dir: %w", err)
	}
	return i.store.Remove(ctx, name, version)
}
```

Add field `dl DashboardLister` to `Installer` struct.

- [ ] **Step 2: Add a focused unit test**

Append to `install_test.go`:

```go
func TestInstaller_Uninstall_NotFound(t *testing.T) {
	store := widgetpack.NewStore(t.TempDir())
	_ = store.Load(context.Background())
	inst := widgetpack.NewInstaller(store, nil, nil, nil, "", nil)
	if err := inst.Uninstall(context.Background(), "ghost", "1.0.0", false); err == nil {
		t.Error("expected ErrPackNotFound")
	}
}
```

- [ ] **Step 3: Run**

```bash
go test ./internal/widgetpack/... -run TestInstaller_Uninstall
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/widgetpack/install.go internal/widgetpack/install_test.go
git commit -m "feat(widgetpack): Uninstall with reference check"
```

---

## Task 11: `WidgetPackService` Connect handler

**Files:**
- Create: `internal/widgetpack/service.go`
- Create: `internal/widgetpack/service_test.go`

- [ ] **Step 1: Write `service.go`**

```go
package widgetpack

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
)

// Service implements WidgetPackServiceHandler.
type Service struct {
	installer *Installer
	store     *Store
}

func NewService(installer *Installer, store *Store) *Service {
	return &Service{installer: installer, store: store}
}

var _ switchyardv1alpha1connect.WidgetPackServiceHandler = (*Service)(nil)

func (s *Service) Install(ctx context.Context, req *connect.Request[v1.InstallWidgetPackRequest]) (*connect.Response[v1.InstallWidgetPackResponse], error) {
	pack, err := s.installer.Install(ctx, InstallRequest{Ref: req.Msg.GetRef()})
	if err != nil {
		return nil, mapInstallErr(err)
	}
	return connect.NewResponse(&v1.InstallWidgetPackResponse{Pack: toProto(pack)}), nil
}

func (s *Service) List(ctx context.Context, _ *connect.Request[v1.ListWidgetPacksRequest]) (*connect.Response[v1.ListWidgetPacksResponse], error) {
	packs, err := s.store.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*v1.InstalledPack, 0, len(packs))
	for i := range packs {
		out = append(out, toProto(&packs[i]))
	}
	return connect.NewResponse(&v1.ListWidgetPacksResponse{Packs: out}), nil
}

func (s *Service) Uninstall(ctx context.Context, req *connect.Request[v1.UninstallWidgetPackRequest]) (*connect.Response[v1.UninstallWidgetPackResponse], error) {
	if err := s.installer.Uninstall(ctx, req.Msg.GetName(), req.Msg.GetVersion(), req.Msg.GetForce()); err != nil {
		if errors.Is(err, ErrPackNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connect.NewResponse(&v1.UninstallWidgetPackResponse{}), nil
}

func (s *Service) Watch(ctx context.Context, _ *connect.Request[v1.WatchWidgetPacksRequest], stream *connect.ServerStream[v1.WidgetPackEvent]) error {
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

func toProto(p *InstalledPack) *v1.InstalledPack {
	if p == nil {
		return nil
	}
	return &v1.InstalledPack{
		Name:            p.Name,
		Version:         p.Version,
		Sha256:          p.SHA256,
		Signature:       sigToProto(p.SignatureStatus),
		SignerIdentity:  p.SignerIdentity,
		Classes:         p.Classes,
		BundleUrl:       "/widgets/" + p.Name + "/" + p.Version + "/bundle.js?h=" + p.SHA256,
		Description:     p.Description,
		Homepage:        p.Homepage,
		License:         p.License,
		InstalledAt:     timestamppb.New(p.InstalledAt),
	}
}

func sigToProto(s string) v1.SignatureStatus {
	switch s {
	case "verified":
		return v1.SignatureStatus_SIGNATURE_STATUS_VERIFIED
	case "unsigned":
		return v1.SignatureStatus_SIGNATURE_STATUS_UNSIGNED
	case "invalid":
		return v1.SignatureStatus_SIGNATURE_STATUS_INVALID
	default:
		return v1.SignatureStatus_SIGNATURE_STATUS_UNSPECIFIED
	}
}

func eventToProto(ev WatchEvent) *v1.WidgetPackEvent {
	if ev.Installed != nil {
		return &v1.WidgetPackEvent{Kind: &v1.WidgetPackEvent_Installed{Installed: toProto(ev.Installed)}}
	}
	if ev.Uninstalled != nil {
		return &v1.WidgetPackEvent{Kind: &v1.WidgetPackEvent_Uninstalled{Uninstalled: &v1.UninstalledPack{Name: ev.Uninstalled.Name, Version: ev.Uninstalled.Version}}}
	}
	return &v1.WidgetPackEvent{}
}

func mapInstallErr(err error) error {
	var fe *FailureError
	if errors.As(err, &fe) {
		switch fe.Reason {
		case ReasonBadRef:
			return connect.NewError(connect.CodeInvalidArgument, err)
		case ReasonRegistryUnreachable:
			return connect.NewError(connect.CodeUnavailable, err)
		case ReasonAlreadyExists:
			return connect.NewError(connect.CodeAlreadyExists, err)
		case ReasonBadArtifact, ReasonSignatureInvalid, ReasonHashMismatch,
			ReasonSDKIncompatible, ReasonClassCollision, ReasonManifestInvalid:
			return connect.NewError(connect.CodeFailedPrecondition, err)
		}
	}
	return connect.NewError(connect.CodeInternal, err)
}
```

- [ ] **Step 2: Write `service_test.go`**

```go
package widgetpack_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/widgetpack"
)

func TestService_List_Empty(t *testing.T) {
	store := widgetpack.NewStore(t.TempDir())
	_ = store.Load(context.Background())
	svc := widgetpack.NewService(nil, store)
	resp, err := svc.List(context.Background(), connect.NewRequest(&v1.ListWidgetPacksRequest{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Msg.GetPacks()) != 0 {
		t.Errorf("len = %d", len(resp.Msg.GetPacks()))
	}
}

func TestService_Uninstall_NotFound(t *testing.T) {
	store := widgetpack.NewStore(t.TempDir())
	_ = store.Load(context.Background())
	inst := widgetpack.NewInstaller(store, nil, nil, nil, "", nil)
	svc := widgetpack.NewService(inst, store)
	_, err := svc.Uninstall(context.Background(), connect.NewRequest(&v1.UninstallWidgetPackRequest{Name: "ghost", Version: "1.0.0"}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeNotFound {
		t.Errorf("err = %v", err)
	}
}

func TestService_Watch_DeliversInstall(t *testing.T) {
	store := widgetpack.NewStore(t.TempDir())
	_ = store.Load(context.Background())
	svc := widgetpack.NewService(nil, store)

	// Drive Watch in a goroutine via a manual stream stub. Simplest path:
	// write a tiny serverStream stub that captures Send calls; sigstore tests
	// use this pattern. ~20 lines. Verify that Add → stub.Send fired with
	// Installed=non-nil. Time-bound the wait at 1s.
	_ = svc
	_ = time.Second
	t.Skip("manual server-stream stub — implement during execution")
}
```

The `Watch` test is sketched-out rather than complete because connect's `ServerStream` requires a small fake implementation; the implementer should mirror `connectrpc/connect-go`'s own test helpers.

- [ ] **Step 3: Run tests**

```bash
go test ./internal/widgetpack/... -run TestService
```

Expected: PASS (Watch test skipped; pick up at execution).

- [ ] **Step 4: Commit**

```bash
git add internal/widgetpack/service.go internal/widgetpack/service_test.go
git commit -m "feat(widgetpack): WidgetPackService Connect handler"
```

---

## Task 12: Procedure-catalog entries (inert until F-184)

**Files:**
- Create: `internal/api/service_widget_pack.go`
- Create: `internal/api/service_widget_pack_test.go`

This file declares the procedure-catalog entries for `widget_pack.{install,list,uninstall,watch}`. The daemon does not yet wire a `ProcedureCatalog` into `NewAuthorize` (tracked as F-184), so these entries are dormant. When F-184 lands, the daemon will discover this registrar and call it.

- [ ] **Step 1: Write `service_widget_pack.go`**

```go
package api

import (
	"github.com/fdatoo/switchyard/internal/auth"
)

// RegisterWidgetPackProcedures registers authz catalog entries for the four
// WidgetPackService procedures. Wired into the daemon's catalog construction
// once F-184 lands; until then this is a no-op at startup.
func RegisterWidgetPackProcedures(addProcedure func(string, auth.Action, func(any) auth.Target)) {
	addProcedure(
		"/switchyard.v1alpha1.WidgetPackService/Install",
		auth.Action{Service: "widget_pack", Method: "install", Verb: "write"},
		func(any) auth.Target { return auth.Target{Kind: "widget_pack"} },
	)
	addProcedure(
		"/switchyard.v1alpha1.WidgetPackService/Uninstall",
		auth.Action{Service: "widget_pack", Method: "uninstall", Verb: "write"},
		func(any) auth.Target { return auth.Target{Kind: "widget_pack"} },
	)
	addProcedure(
		"/switchyard.v1alpha1.WidgetPackService/List",
		auth.Action{Service: "widget_pack", Method: "list", Verb: "read"},
		func(any) auth.Target { return auth.Target{Kind: "widget_pack"} },
	)
	addProcedure(
		"/switchyard.v1alpha1.WidgetPackService/Watch",
		auth.Action{Service: "widget_pack", Method: "watch", Verb: "read"},
		func(any) auth.Target { return auth.Target{Kind: "widget_pack"} },
	)
}
```

- [ ] **Step 2: Write a registrar smoke test**

```go
package api_test

import (
	"testing"

	"github.com/fdatoo/switchyard/internal/api"
	"github.com/fdatoo/switchyard/internal/auth"
)

func TestRegisterWidgetPackProcedures(t *testing.T) {
	type entry struct {
		Procedure string
		Action    auth.Action
	}
	var got []entry
	api.RegisterWidgetPackProcedures(func(proc string, a auth.Action, _ func(any) auth.Target) {
		got = append(got, entry{Procedure: proc, Action: a})
	})
	want := []string{"Install", "Uninstall", "List", "Watch"}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d", len(got), len(want))
	}
	for i, m := range want {
		if got[i].Action.Method != map[string]string{
			"Install": "install", "Uninstall": "uninstall", "List": "list", "Watch": "watch",
		}[m] {
			t.Errorf("entry[%d] method = %q", i, got[i].Action.Method)
		}
	}
}
```

- [ ] **Step 3: Run**

```bash
go test ./internal/api/... -run TestRegisterWidgetPackProcedures
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/api/service_widget_pack.go internal/api/service_widget_pack_test.go
git commit -m "feat(api): widget_pack procedure-catalog registrar (inert until F-184)"
```

---

## Task 13: Wire daemon and listener

**Files:**
- Modify: `internal/api/listener/routes.go`
- Modify: `internal/daemon/daemon.go`

- [ ] **Step 1: Extend `Services` and `BuildRoutes`**

In `internal/api/listener/routes.go`:

```go
// add to Services struct:
WidgetPack switchyardv1alpha1connect.WidgetPackServiceHandler
```

```go
// add inside BuildRoutes, near the other NewXServiceHandler calls:
p, h = switchyardv1alpha1connect.NewWidgetPackServiceHandler(svc.WidgetPack, opts)
routes = append(routes, Route{Path: p, Handler: h})
```

Bump the `make([]Route, 0, 13)` capacity hint to 14.

- [ ] **Step 2: Construct widgetpack pieces in `daemon.go`**

Read the existing daemon initialization (`internal/daemon/daemon.go` around lines 380-410) to find where services are constructed. Add (placing near other service constructions, before the `services := listener.Services{...}` literal):

```go
// Widget pack subsystem.
packStore := widgetpack.NewStore(filepath.Join(dataDir, "widgets"))
if err := packStore.Load(ctx); err != nil {
    return nil, fmt.Errorf("widget pack store: %w", err)
}
trustPolicy := &widgetpack.TrustPolicy{}
if snap := cfgManager.Current(); snap != nil {
    if p := snap.GetWidgetPackPolicy(); p != nil {
        trustPolicy.Set(p.GetAllowedSigners(), p.GetAllowUnsigned())
    }
}
cfgManager.OnApplied(func(snap *configpb.ConfigSnapshot) {
    if p := snap.GetWidgetPackPolicy(); p != nil {
        trustPolicy.Set(p.GetAllowedSigners(), p.GetAllowUnsigned())
    }
})
fetcher, err := widgetpack.NewFetcher()
if err != nil {
    return nil, fmt.Errorf("widget pack fetcher: %w", err)
}
verifier, err := widgetpack.NewProductionVerifier(ctx) // see note below
if err != nil {
    return nil, fmt.Errorf("widget pack verifier: %w", err)
}
packInstaller := widgetpack.NewInstaller(
    packStore, verifier, trustPolicy, fetcher, dataDir, dashboard.BuiltinClassIDs,
)
packService := widgetpack.NewService(packInstaller, packStore)
```

> **`NewProductionVerifier`:** add a small constructor in `widgetpack/trust.go` that downloads the default Sigstore TUF root (sigstore-go provides `tuf.DefaultClient` or similar). Until that's in place, fall back to passing `nil` and have the `Verifier` reject all signed verification (only `allowUnsigned` paths succeed). Decision noted in code; the production-root fetch is a small follow-up if not already trivial.

Wire `WidgetPack: packService` into the existing `services := listener.Services{...}` literal.

Also pass the bundle handler:

```go
deps.WidgetsHandler = widgetpack.NewBundleHandler(packStore)
```

(The `listener.Deps` struct already has `WidgetsHandler http.Handler` — see `internal/api/listener/listener.go:32`.)

- [ ] **Step 3: Build the daemon binary**

```bash
go build ./cmd/switchyardd
```

Expected: PASS.

- [ ] **Step 4: Run all tests**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/listener/routes.go internal/daemon/daemon.go
git commit -m "feat(daemon): wire WidgetPackService + bundle handler"
```

---

## Task 14: Replace CLI stubs in `cmd_widget.go`

**Files:**
- Rewrite: `internal/cli/cmd_widget.go`
- Create: `internal/cli/cmd_widget_test.go`

- [ ] **Step 1: Read the existing stub + companion files for the calling pattern**

```bash
cat internal/cli/cmd_widget.go
cat internal/cli/cmd_automation.go | head -80
cat internal/cli/styles_widget.go
```

- [ ] **Step 2: Rewrite `cmd_widget.go`**

```go
package cli

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
)

func newWidgetCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "widget", Short: "Manage widget packs"}
	cmd.AddCommand(newWidgetInstallCmd(gf))
	cmd.AddCommand(newWidgetListCmd(gf))
	cmd.AddCommand(newWidgetUninstallCmd(gf))
	return cmd
}

func newWidgetInstallCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "install <oci-ref>",
		Short: "Install a widget pack from an OCI registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := dialWidgetPack(cmd.Context(), gf)
			if err != nil {
				return err
			}
			resp, err := client.Install(cmd.Context(), connect.NewRequest(&v1.InstallWidgetPackRequest{Ref: args[0]}))
			if err != nil {
				return renderConnectErr(err)
			}
			renderInstalled(resp.Msg.GetPack())
			return nil
		},
	}
}

func newWidgetListCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed widget packs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := dialWidgetPack(cmd.Context(), gf)
			if err != nil {
				return err
			}
			resp, err := client.List(cmd.Context(), connect.NewRequest(&v1.ListWidgetPacksRequest{}))
			if err != nil {
				return renderConnectErr(err)
			}
			packs := resp.Msg.GetPacks()
			if len(packs) == 0 {
				fmt.Println(Dim.Render("no packs installed"))
				return nil
			}
			fmt.Printf("%s\t%s\t%s\t%s\n",
				Header.Render("NAME"), Header.Render("VERSION"),
				Header.Render("SIG"), Header.Render("CLASSES"))
			for _, p := range packs {
				fmt.Printf("%s\t%s\t%s\t%v\n",
					PackName.Render(p.GetName()),
					PackVersion.Render(p.GetVersion()),
					sigBadge(p.GetSignature()),
					p.GetClasses())
			}
			return nil
		},
	}
}

func newWidgetUninstallCmd(gf *globalFlags) *cobra.Command {
	var version string
	var force bool
	cmd := &cobra.Command{
		Use:   "uninstall <name>",
		Short: "Uninstall a widget pack",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := dialWidgetPack(cmd.Context(), gf)
			if err != nil {
				return err
			}
			versions := []string{version}
			if version == "" {
				resp, err := client.List(cmd.Context(), connect.NewRequest(&v1.ListWidgetPacksRequest{}))
				if err != nil {
					return renderConnectErr(err)
				}
				versions = nil
				for _, p := range resp.Msg.GetPacks() {
					if p.GetName() == args[0] {
						versions = append(versions, p.GetVersion())
					}
				}
				if len(versions) == 0 {
					return fmt.Errorf("no installed versions of %q", args[0])
				}
			}
			for _, v := range versions {
				_, err := client.Uninstall(cmd.Context(), connect.NewRequest(&v1.UninstallWidgetPackRequest{
					Name: args[0], Version: v, Force: force,
				}))
				if err != nil {
					return renderConnectErr(err)
				}
				fmt.Printf("%s %s@%s\n", Success.Render("uninstalled"), args[0], v)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "specific version (default: all installed)")
	cmd.Flags().BoolVar(&force, "force", false, "uninstall even if dashboards reference the pack's classes")
	return cmd
}

func dialWidgetPack(ctx context.Context, gf *globalFlags) (switchyardv1alpha1connect.WidgetPackServiceClient, error) {
	ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
	httpClient, base, err := Dial(ctx, ep)
	if err != nil {
		return nil, err
	}
	return switchyardv1alpha1connect.NewWidgetPackServiceClient(httpClient, base), nil
}

func renderInstalled(p *v1.InstalledPack) {
	if p == nil {
		return
	}
	fmt.Printf("%s %s@%s %s\n",
		Success.Render("installed"),
		PackName.Render(p.GetName()),
		PackVersion.Render(p.GetVersion()),
		sigBadge(p.GetSignature()))
	if p.GetSignerIdentity() != "" {
		fmt.Printf("  signer: %s\n", Dim.Render(p.GetSignerIdentity()))
	}
	fmt.Printf("  classes: %v\n", p.GetClasses())
}

func sigBadge(s v1.SignatureStatus) string {
	switch s {
	case v1.SignatureStatus_SIGNATURE_STATUS_VERIFIED:
		return PackVerified.Render("✓ verified")
	case v1.SignatureStatus_SIGNATURE_STATUS_UNSIGNED:
		return PackUnsigned.Render("⚠ unsigned")
	case v1.SignatureStatus_SIGNATURE_STATUS_INVALID:
		return PackExpired.Render("✗ invalid")
	default:
		return Dim.Render("?")
	}
}
```

- [ ] **Step 3: Wire `newWidgetCmd(gf)` into `internal/cli/root.go`**

If the existing call is `newWidgetCmd()` (no args), update it to `newWidgetCmd(gf)`. Read `root.go` to confirm.

- [ ] **Step 4: Tiny smoke test**

`internal/cli/cmd_widget_test.go`:

```go
package cli

import (
	"strings"
	"testing"
)

func TestNewWidgetCmd_HasSubcommands(t *testing.T) {
	cmd := newWidgetCmd(&globalFlags{})
	got := make(map[string]bool)
	for _, c := range cmd.Commands() {
		got[strings.SplitN(c.Use, " ", 2)[0]] = true
	}
	for _, want := range []string{"install", "list", "uninstall"} {
		if !got[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}
```

- [ ] **Step 5: Build the CLI**

```bash
go build ./cmd/switchyard
```

Expected: PASS.

- [ ] **Step 6: Run CLI tests**

```bash
go test ./internal/cli/...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/cmd_widget.go internal/cli/cmd_widget_test.go internal/cli/root.go
git commit -m "feat(cli): wire switchyard widget {install,list,uninstall} to RPC"
```

---

## Task 15: End-to-end integration test

**Files:**
- Create: `internal/widgetpack/install_integration_test.go`
- Modify: `internal/widgetpack/testutil_test.go` (add helpers if not already present)

This test exercises the full install path: in-process registry → push signed pack → `Installer.Install` → bundle reachable over HTTP → catalog updated. Plus rejection paths.

- [ ] **Step 1: Add the dependency**

```bash
go get github.com/google/go-containerregistry@latest
```

- [ ] **Step 2: Build helpers in `testutil_test.go`**

Add (alongside `newTestTrustRoot` / `signBlobWithLeaf` from Task 5):

```go
// startTestRegistry returns the URL of an in-process OCI registry. Caller
// closes via the returned cleanup func.
func startTestRegistry(t *testing.T) (string, func()) {
	t.Helper()
	srv := httptest.NewServer(registry.New())
	return strings.TrimPrefix(srv.URL, "http://"), srv.Close
}

// buildAndPushTestPack builds a tarball with manifest.pkl + bundle.js, pushes
// it as an OCI artifact under the given repo:tag, optionally signs it with
// the given identity, and returns the full ref.
func buildAndPushTestPack(t *testing.T, regHost, repoTag, identity string, root *TestTrustRoot, sign bool) string {
	t.Helper()
	// 1. Build manifest.pkl + bundle.js bytes.
	bundleBytes := []byte("export const Bar = () => null;")
	bundleSHA := sha256Hex(bundleBytes)
	manifestPkl := fmt.Sprintf(`@ModuleInfo { minPklVersion = "0.27.0" }
amends "switchyard:widgets"
manifest = new PackManifest {
  name = "bar-widgets"
  version = "1.0.0"
  protocol = "v1"
  sdkVersion = "1.0.0"
  bundle = "bundle.js"
  bundleHash = "sha256:%s"
  classes = new { "BarChart" }
}`, bundleSHA)

	// 2. Build the gzipped tarball.
	tgz := buildTarGz(t, map[string][]byte{
		"manifest.pkl": []byte(manifestPkl),
		"bundle.js":    bundleBytes,
	})

	// 3. Push via go-containerregistry's remote helpers as an OCI artifact
	//    with our media type. ~30 lines using `remote.Write` with a custom
	//    image type that has one layer.
	ref := pushOCIArtifact(t, regHost, repoTag, widgetpack.MediaType, tgz)

	// 4. If sign==true, sign the layer digest using the test trust root and
	//    push the cosign signature artifact at <ref>.sig. Use sigstore-go
	//    signing helpers; mirror sigstore-go's e2e tests.
	if sign {
		signOCIArtifact(t, regHost, ref, identity, root)
	}
	return ref
}
```

> Several helpers above (`buildTarGz`, `pushOCIArtifact`, `signOCIArtifact`, `sha256Hex`) are mechanical wrappers around standard libraries and `go-containerregistry`. They are 10-30 lines each and don't warrant inline expansion in the plan; the implementer copies the patterns from `go-containerregistry`'s and `sigstore-go`'s own tests.

- [ ] **Step 3: Write `install_integration_test.go`**

```go
package widgetpack_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/fdatoo/switchyard/internal/widgetpack"
)

func TestInstall_Integration_Signed(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	ctx := context.Background()

	regHost, regClose := startTestRegistry(t)
	defer regClose()

	root := newTestTrustRoot(t)
	dataDir := t.TempDir()

	store := widgetpack.NewStore(filepath.Join(dataDir, "widgets"))
	_ = store.Load(ctx)
	pol := &widgetpack.TrustPolicy{}
	pol.Set([]string{"https://test/identity"}, false)
	verifier := widgetpack.NewVerifier(root.TrustedRoot)
	fetcher, _ := widgetpack.NewFetcher()
	inst := widgetpack.NewInstaller(store, verifier, pol, fetcher, dataDir, []string{"Gauge", "EntityToggle"})

	ref := buildAndPushTestPack(t, regHost, "bar-widgets:1.0.0", "https://test/identity", root, true)

	pack, err := inst.Install(ctx, widgetpack.InstallRequest{Ref: ref})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if pack.SignatureStatus != "verified" {
		t.Errorf("SignatureStatus = %q", pack.SignatureStatus)
	}
	if pack.SHA256 == "" || pack.SHA256 == "pending" {
		t.Errorf("SHA256 = %q", pack.SHA256)
	}

	// Bundle reachable over HTTP.
	srv := httptest.NewServer(widgetpack.NewBundleHandler(store))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/widgets/bar-widgets/1.0.0/bundle.js")
	if err != nil {
		t.Fatalf("Get bundle: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
	if resp.Header.Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q", resp.Header.Get("Cache-Control"))
	}

	// Pack appears in store.
	got, err := store.Get(ctx, "bar-widgets", "1.0.0")
	if err != nil || got.Classes[0] != "BarChart" {
		t.Errorf("store.Get: %v %v", err, got)
	}
}

func TestInstall_Integration_UnsignedRejected(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	ctx := context.Background()
	regHost, regClose := startTestRegistry(t)
	defer regClose()
	root := newTestTrustRoot(t)
	dataDir := t.TempDir()
	store := widgetpack.NewStore(filepath.Join(dataDir, "widgets"))
	_ = store.Load(ctx)
	pol := &widgetpack.TrustPolicy{}
	pol.Set(nil, false) // allowUnsigned=false
	verifier := widgetpack.NewVerifier(root.TrustedRoot)
	fetcher, _ := widgetpack.NewFetcher()
	inst := widgetpack.NewInstaller(store, verifier, pol, fetcher, dataDir, nil)

	ref := buildAndPushTestPack(t, regHost, "bar-widgets:1.0.0", "", root, false)
	if _, err := inst.Install(ctx, widgetpack.InstallRequest{Ref: ref}); err == nil {
		t.Error("expected unsigned to be rejected")
	}
	// Nothing staged: pack dir should not exist.
	if _, err := store.Get(ctx, "bar-widgets", "1.0.0"); err == nil {
		t.Error("rejected pack should not be in store")
	}
}

func TestInstall_Integration_SignerNotInPolicy(t *testing.T) {
	// Same shape as Signed test but with allowedSigners = ["https://other/*"].
	// Expect rejection. ~30 lines; follow the pattern above.
	t.Skip("similar shape to TestInstall_Integration_Signed; implement during execution")
}

func TestInstall_Integration_HashMismatch(t *testing.T) {
	// buildAndPushTestPack variant that pushes a manifest with bundleHash
	// pointing to wrong-bytes; expect ReasonHashMismatch.
	t.Skip("implement during execution; ~20 lines mutating buildAndPushTestPack")
}

func TestInstall_Integration_ClassCollisionWithBuiltin(t *testing.T) {
	// Push a pack manifest with classes = { "EntityToggle" }; expect
	// ReasonClassCollision. ~20 lines.
	t.Skip("implement during execution")
}

func TestInstall_Integration_AlreadyExists(t *testing.T) {
	// Run Install twice with the same ref; second returns ReasonAlreadyExists.
	t.Skip("implement during execution")
}
```

The two large cases (`Signed`, `UnsignedRejected`) are full; the four small variant cases are sketched but punted to execution because the helpers from Step 2 make each case ~20 mechanical lines.

- [ ] **Step 4: Run integration tests**

```bash
go test ./internal/widgetpack/... -run TestInstall_Integration -race
```

Expected: PASS for the two full tests; the four `Skip`'d tests show as skipped until the executor fills them in.

- [ ] **Step 5: Commit**

```bash
git add internal/widgetpack/install_integration_test.go internal/widgetpack/testutil_test.go go.mod go.sum
git commit -m "test(widgetpack): end-to-end integration against in-process registry"
```

---

## Final task: full-suite verification

- [ ] **Run the full test suite, race-clean**

```bash
go test ./... -race
```

Expected: PASS.

- [ ] **Run `go vet` and `go build`**

```bash
go vet ./...
go build ./...
```

Expected: PASS, no output.

- [ ] **Sanity-check the daemon starts**

```bash
go run ./cmd/switchyardd --help
```

Expected: usage output, no panic.

- [ ] **Sanity-check the CLI compiles and shows the new subcommand**

```bash
go run ./cmd/switchyard widget --help
```

Expected: shows `install`, `list`, `uninstall` subcommands.

If everything is green, F-157 is mergeable. Authz behavior remains pass-through until F-184 lands; this is documented in the spec §9.6.
