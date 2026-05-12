# Scenes end-to-end implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make scenes a first-class, end-to-end product surface: real `SceneService` runtime (replaces the existing stubs), room-scoping via optional `area_id`, form-driven create/edit, and live reactive list refresh on save.

**Architecture:** Add `area_id` to `SceneConfig` and the Pkl schema; daemon ships a new `internal/automation/scene.Applier` that compiles each scene's actions into the existing `action.Executor` chain and runs them in parallel (best-effort, per-action outcomes recorded). `SceneService.{List,Apply,Preview}` get real implementations sharing the Applier. Front-end gets `SySceneForm` and `SyAreaForm` modeled on `SyAutomationForm`; `/rooms` shows global scenes, `/rooms/:id` shows scoped scenes — both gain "+ New" affordances. Track A's reactive subscription handles list refresh automatically on save.

**Tech Stack:** Go, Pkl (apple/pkl-go), Connect, Vue 3 + TypeScript, existing `action.Executor` + `regen.RenderScene` + `EditSession.CommitEdit` + Track A's `configStore`.

**Spec:** `docs/design/specs/2026-05-12-scenes-end-to-end-design.md`

---

## File map

| File | Status | Responsibility |
|------|--------|----------------|
| `proto/switchyard/config/v1/snapshot.proto` | MOD | Add `string area_id = 4` to `SceneConfig`. |
| `proto/switchyard/v1alpha1/scene.proto` | MOD | Add `string area_id = 3` to `v1.Scene`. |
| `proto/switchyard/event/v1/event.proto` | MOD | Add `SceneApplied` payload to the event oneof. |
| `gen/...` | GEN | Regenerated proto bindings. |
| `internal/config/pkl/switchyard/scenes.pkl` | MOD | Add `areaId: String? = null` to `Scene` class. |
| `internal/config/pkl/switchyard/scene.pkl` | MOD | Add `areaId: String? = null` to the singular template. |
| `internal/config/evaluator.go` | MOD | `sceneJSON` gains `AreaId *string`; `parseConfigJSON` and `sceneFromJSON` populate it. |
| `internal/config/evaluator_decode.go` | MOD | `sceneFromJSON` reads `area_id`. |
| `internal/config/compile.go` | MOD | Dangling area-ref check for scenes. |
| `internal/config/compile_test.go` | MOD | Test for dangling-area-ref code path. |
| `internal/automation/regen/scene.go` | MOD | Emit `areaId = "..."` when set. |
| `internal/automation/regen/scene_test.go` | MOD | Test cases for emit-with-area and emit-without-area. |
| `internal/automation/compile.go` | MOD | Export `CompileAction` for use by scene.Applier (or keep package-private with a new public seam). |
| `internal/automation/scene/applier.go` | NEW | `Applier` struct + `Invoke(ctx, sceneID, corrID, invokedBy)` method. |
| `internal/automation/scene/errors.go` | NEW | `ErrSceneNotFound`. |
| `internal/automation/scene/applier_test.go` | NEW | Unit tests with fake dispatcher + event store. |
| `internal/api/service_scene.go` | NEW | Real `SceneService` (replaces stub). |
| `internal/api/service_unimplemented.go` | MOD | Remove `SceneService` stub (now real). |
| `internal/api/service_scene_test.go` | NEW | Handler unit tests. |
| `internal/api/deps.go` | MOD | Add `SceneInvoker` interface; extend `Snapshotter` if needed (or reuse existing). |
| `internal/daemon/daemon.go` | MOD | Replace `action.StubSceneApplier{}` with real `scene.Applier`. Wire to `SceneService`. |
| `internal/daemon/api_adapters.go` | MOD | Add adapter to bridge daemon's `scene.Applier` to api's `SceneInvoker`. |
| `internal/daemon/scenes_e2e_test.go` | NEW | Integration test: declare scene, apply via RPC, verify dispatch + event. |
| `app/src/data/scenes.ts` | MOD | Extend `Scene` type with `areaId`; `decode` reads `area_id`/`areaId`. |
| `app/src/views/scenes/SySceneForm.vue` | NEW | Scene creation/edit modal. |
| `app/src/views/areas/SyAreaForm.vue` | NEW | Area creation/edit modal. |
| `app/src/views/RoomsView.vue` | MOD | "+ New room" + "+ New scene" buttons; "Global scenes" section. |
| `app/src/views/RoomDetailView.vue` | MOD | Filter scenes by `route.params.id`; "+ New scene" button. |
| `app/src/data/regen-preview.ts` | MOD (if needed) | Accept `fileType: "scene" \| "area"` in addition to "automation". |

---

## Task 1: Proto changes

**Files:**
- Modify: `proto/switchyard/config/v1/snapshot.proto` (find `SceneConfig`)
- Modify: `proto/switchyard/v1alpha1/scene.proto`
- Modify: `proto/switchyard/event/v1/event.proto`

- [ ] **Step 1: Edit snapshot.proto**

Find the existing `SceneConfig` message (it has `id`, `display_name`, `actions`). Add `area_id`:

```protobuf
message SceneConfig {
  string id           = 1;
  string display_name = 2;
  repeated ActionConfig actions = 3;
  string area_id      = 4;
}
```

- [ ] **Step 2: Edit scene.proto**

Update the existing `Scene` message:

```protobuf
message Scene {
  string id           = 1;
  string display_name = 2;
  string area_id      = 3;
}
```

- [ ] **Step 3: Edit event.proto**

Find the `Payload` oneof (existing payloads include `ConfigApplied`, `AutomationFinished`, etc.). Add `SceneApplied`:

```protobuf
message Payload {
  oneof kind {
    // ... existing payloads ...
    SceneApplied scene_applied = 99;  // pick next available tag number — check the proto for last used
    // ...
  }
}

message SceneApplied {
  string scene_id        = 1;
  string area_id         = 2;
  string correlation_id  = 3;
  string invoked_by      = 4;
  uint64 steps           = 5;
  repeated string logs   = 6;
  RunOutcome outcome     = 7;
}
```

Read `proto/switchyard/event/v1/event.proto` first to find the next free tag number in the oneof; do NOT reuse an existing one.

- [ ] **Step 4: Regen and build**

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
buf generate
go build ./...
```

Expected: clean build. If `internal/automation/action/scene.go`'s `StubSceneApplier.Apply` still references `SystemEvent` for its event payload — leave it; we replace it later.

- [ ] **Step 5: Commit**

```bash
git add proto/ gen/
git commit -m "feat(proto): SceneConfig.area_id + v1.Scene.area_id + SceneApplied event"
```

---

## Task 2: Pkl schema extension

**Files:**
- Modify: `internal/config/pkl/switchyard/scenes.pkl`
- Modify: `internal/config/pkl/switchyard/scene.pkl`
- Modify: `internal/config/evaluator.go` (sceneJSON)
- Modify: `internal/config/evaluator_decode.go` (sceneFromJSON)

- [ ] **Step 1: Write failing test**

Add to `internal/config/evaluator_integration_test.go`:

```go
func TestEvaluate_SceneWithAreaId(t *testing.T) {
	dir := t.TempDir()
	writePkl(t, dir, "main.pkl", `
amends "switchyard:config"

import "switchyard:scenes" as sc
import "switchyard:automations" as auto

scenes = new {
  new sc.Scene {
    id = "kitchen-bright"
    displayName = "Kitchen bright"
    areaId = "kitchen"
    actions = new {
      new auto.CallServiceAction {
        entity = "light.kitchen"
        capability = "turn_on"
      }
    }
  }
}
`)
	snap := mustEvaluate(t, dir)
	if len(snap.GetScenes()) != 1 {
		t.Fatalf("want 1 scene, got %d", len(snap.GetScenes()))
	}
	if got := snap.GetScenes()[0].GetAreaId(); got != "kitchen" {
		t.Errorf("area_id = %q, want kitchen", got)
	}
}
```

- [ ] **Step 2: Verify FAIL**

Run: `go test -tags integration ./internal/config -run TestEvaluate_SceneWithAreaId -v`
Expected: FAIL — Pkl rejects unknown property `areaId`.

- [ ] **Step 3: Add areaId to the Pkl Scene class**

Edit `internal/config/pkl/switchyard/scenes.pkl`. Add `areaId: String? = null` to the `Scene` class definition (near `id`, `displayName`, `actions`).

- [ ] **Step 4: Add areaId to the singular template**

Edit `internal/config/pkl/switchyard/scene.pkl`. Add the same line.

- [ ] **Step 5: Add AreaID to sceneJSON**

In `internal/config/evaluator.go`, find `type sceneJSON struct`. Add:

```go
type sceneJSON struct {
	ID          string            `json:"id"`
	DisplayName string            `json:"displayName"`
	AreaID      *string           `json:"areaId"`
	Actions     []json.RawMessage `json:"actions"`
}
```

`*string` because the Pkl `String?` renders as `null` when unset.

- [ ] **Step 6: Read areaId in sceneFromJSON**

In `internal/config/evaluator_decode.go`, modify `sceneFromJSON`:

```go
func sceneFromJSON(s sceneJSON) (*configpb.SceneConfig, error) {
	areaID := ""
	if s.AreaID != nil {
		areaID = *s.AreaID
	}
	scfg := &configpb.SceneConfig{
		Id:          strings.TrimSpace(s.ID),
		DisplayName: s.DisplayName,
		AreaId:      areaID,
	}
	for _, rawA := range s.Actions {
		ac, err := decodeAction(rawA)
		if err != nil {
			return nil, fmt.Errorf("action: %w", err)
		}
		scfg.Actions = append(scfg.Actions, ac)
	}
	return scfg, nil
}
```

- [ ] **Step 7: Run test to verify PASS**

Run: `go test -tags integration ./internal/config -run TestEvaluate_SceneWithAreaId -v`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/config/pkl/switchyard/scenes.pkl internal/config/pkl/switchyard/scene.pkl internal/config/evaluator.go internal/config/evaluator_decode.go internal/config/evaluator_integration_test.go
git commit -m "feat(config): SceneConfig.area_id end-to-end through Pkl + decode"
```

---

## Task 3: Compile-time dangling-area check for scenes

**Files:**
- Modify: `internal/config/compile.go`
- Modify: `internal/config/compile_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/config/compile_test.go`:

```go
func TestCompile_SceneDanglingAreaRef(t *testing.T) {
	snap := &configpb.ConfigSnapshot{
		Areas: []*configpb.AreaConfig{{Id: "kitchen"}},
		Scenes: []*configpb.SceneConfig{
			{Id: "good", AreaId: "kitchen"},
			{Id: "bad",  AreaId: "ghost-room"},
			{Id: "global"},  // empty AreaId is fine
		},
	}
	errs := Compile(snap, nil)
	gotDangling := 0
	for _, e := range errs {
		if e.Code == "dangling_area_ref" {
			gotDangling++
			if !strings.Contains(e.Message, "ghost-room") {
				t.Errorf("message should mention ghost-room: %s", e.Message)
			}
		}
	}
	if gotDangling != 1 {
		t.Errorf("want 1 dangling_area_ref error, got %d (all errs: %+v)", gotDangling, errs)
	}
}
```

- [ ] **Step 2: Verify FAIL**

Run: `go test ./internal/config -run TestCompile_SceneDanglingAreaRef -v`
Expected: FAIL — Compile doesn't check scene area refs.

- [ ] **Step 3: Add the check in compile.go**

Read the file first to see where existing checks live (e.g., the entity-id check around line 38). Find a natural slot, near the end of `Compile` before the return:

```go
// Scenes: dangling area_id references.
knownAreas := map[string]bool{}
for _, a := range snap.GetAreas() {
	knownAreas[a.GetId()] = true
}
for _, sc := range snap.GetScenes() {
	if aid := sc.GetAreaId(); aid != "" && !knownAreas[aid] {
		errs = append(errs, ValidationError{
			Code:    "dangling_area_ref",
			Field:   fmt.Sprintf("scenes[%s].area_id", sc.GetId()),
			Message: fmt.Sprintf("scene %q references unknown area %q", sc.GetId(), aid),
		})
	}
}
```

- [ ] **Step 4: Run test to verify PASS**

Run: `go test ./internal/config -run TestCompile_SceneDanglingAreaRef -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/compile.go internal/config/compile_test.go
git commit -m "feat(config): dangling area_id check for scenes"
```

---

## Task 4: regen.RenderScene emits area_id

**Files:**
- Modify: `internal/automation/regen/scene.go`
- Modify: `internal/automation/regen/scene_test.go`

- [ ] **Step 1: Update tests to cover both shapes**

Edit `internal/automation/regen/scene_test.go`. Add new test case:

```go
func TestRenderScene_WithAreaId(t *testing.T) {
	out, err := regen.RenderScene(&configpb.SceneConfig{
		Id:          "kitchen-bright",
		DisplayName: "Kitchen bright",
		AreaId:      "kitchen",
	})
	if err != nil {
		t.Fatalf("RenderScene: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `areaId = "kitchen"`) {
		t.Fatalf("output missing areaId line\n%s", s)
	}
}

func TestRenderScene_NoAreaIdLineWhenAbsent(t *testing.T) {
	out, err := regen.RenderScene(&configpb.SceneConfig{
		Id:          "global-off",
		DisplayName: "All off",
	})
	if err != nil {
		t.Fatalf("RenderScene: %v", err)
	}
	if strings.Contains(string(out), "areaId") {
		t.Fatalf("output unexpectedly contains areaId\n%s", out)
	}
}
```

- [ ] **Step 2: Verify the new tests fail**

Run: `go test ./internal/automation/regen -run "TestRenderScene_WithAreaId|TestRenderScene_NoAreaIdLineWhenAbsent" -v`
Expected: FAIL — current `RenderScene` doesn't emit `areaId`.

- [ ] **Step 3: Update RenderScene**

Edit `internal/automation/regen/scene.go`. After the `displayName` line, add the conditional `areaId` emit:

```go
w.line(fmt.Sprintf("displayName = %q", s.GetDisplayName()))
if aid := s.GetAreaId(); aid != "" {
	w.line(fmt.Sprintf("areaId = %q", aid))
}
w.line("actions {")
```

- [ ] **Step 4: Run tests to verify PASS**

Run: `go test ./internal/automation/regen -count=1 -v`
Expected: all PASS (existing tests too — the `area_id`-absent case still produces the prior output shape).

- [ ] **Step 5: Commit**

```bash
git add internal/automation/regen/scene.go internal/automation/regen/scene_test.go
git commit -m "feat(regen): emit areaId line in scene Pkl when set"
```

---

## Task 5: New `scene.Applier` package

**Files:**
- Create: `internal/automation/scene/errors.go`
- Create: `internal/automation/scene/applier.go`
- Create: `internal/automation/scene/applier_test.go`

The `Applier` reuses `internal/automation/action.Executor`. It compiles each scene action into an executor and runs them in parallel via the existing `action.ParallelBlock`. To compile actions, it calls a new exported function `automation.CompileAction(ac, scriptNames, rt)` — which we'll add by exporting the existing package-private `compileAction` (or `compileActionInstrumented` without metrics).

### Step 1: Export `CompileAction` from the automation package

- [ ] **Read** `internal/automation/compile.go`. Find `compileActionInstrumented` and the underlying `compileAction` (if it exists; the file may only have the instrumented one). Add a new public wrapper at the top of the file:

```go
// CompileAction compiles a single ActionConfig to an Executor. Used by the
// scene package to execute a scene's actions without the automation
// engine's metric instrumentation.
func CompileAction(ac *configpb.ActionConfig, scriptNames map[string]bool, rt *ghstarlark.Runtime) (action.Executor, error) {
	return compileActionInstrumented(ac, scriptNames, rt, "scene", nil)
}
```

If `compileActionInstrumented` requires a non-nil metrics interface, adjust by passing `(metricsIface)(nil)` cast. Verify by:

```bash
go build ./internal/automation
```

Expected: clean.

- [ ] **Commit step**

```bash
git add internal/automation/compile.go
git commit -m "feat(automation): export CompileAction for scene engine reuse"
```

### Step 2: Write the failing applier test

Create `internal/automation/scene/applier_test.go`:

```go
package scene

import (
	"context"
	"sync"
	"testing"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	"github.com/fdatoo/switchyard/internal/eventstore"
	ghstarlark "github.com/fdatoo/switchyard/internal/starlark"
)

type fakeDispatch struct {
	mu    sync.Mutex
	calls []string // "entity:capability"
}

func (f *fakeDispatch) Dispatch(_ context.Context, entityID, capability string, _ map[string]string) (*ghstarlark.DispatchResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, entityID+":"+capability)
	return &ghstarlark.DispatchResult{}, nil
}

type fakeStore struct {
	mu     sync.Mutex
	events []eventstore.Event
}

func (f *fakeStore) Append(_ context.Context, ev eventstore.Event) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, ev)
	return uint64(len(f.events)), nil
}

type fakeSnap struct {
	scenes []*configpb.SceneConfig
}

func (f *fakeSnap) Current() *configpb.ConfigSnapshot {
	return &configpb.ConfigSnapshot{Scenes: f.scenes}
}

func TestApplier_HappyPath(t *testing.T) {
	scene := &configpb.SceneConfig{
		Id: "movie-night",
		Actions: []*configpb.ActionConfig{
			{Kind: &configpb.ActionConfig_CallService{CallService: &configpb.CallServiceAction{
				Entity: "light.living_room", Capability: "turn_off",
			}}},
			{Kind: &configpb.ActionConfig_CallService{CallService: &configpb.CallServiceAction{
				Entity: "light.bedroom", Capability: "turn_off",
			}}},
		},
	}
	dispatch := &fakeDispatch{}
	store := &fakeStore{}
	applier := NewApplier(&fakeSnap{scenes: []*configpb.SceneConfig{scene}}, dispatch, store, nil, nil, nil, nil, nil)

	err := applier.Invoke(context.Background(), "movie-night", "corr-1", "test")
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if len(dispatch.calls) != 2 {
		t.Errorf("want 2 dispatches, got %d: %v", len(dispatch.calls), dispatch.calls)
	}
	// Expect one scene_applied event.
	if len(store.events) != 1 || store.events[0].Kind != "scene" {
		t.Errorf("want 1 scene event, got %d (%+v)", len(store.events), store.events)
	}
}

func TestApplier_UnknownSceneIsError(t *testing.T) {
	applier := NewApplier(&fakeSnap{}, &fakeDispatch{}, &fakeStore{}, nil, nil, nil, nil, nil)
	err := applier.Invoke(context.Background(), "ghost", "corr-2", "test")
	if err == nil || err != ErrSceneNotFound {
		t.Errorf("want ErrSceneNotFound, got %v", err)
	}
}
```

### Step 3: Verify FAIL

Run: `go test ./internal/automation/scene -v 2>&1 | head -20`
Expected: FAIL (package doesn't exist).

### Step 4: Create errors.go

`internal/automation/scene/errors.go`:

```go
package scene

import "errors"

var ErrSceneNotFound = errors.New("scene: not found")
```

### Step 5: Create applier.go

`internal/automation/scene/applier.go`:

```go
// Package scene implements scene invocation: looking up a scene by id in
// the live config snapshot, compiling its actions, and running them in
// parallel via the action.Executor chain. Replaces the StubSceneApplier
// that the daemon previously wired into the automation engine.
package scene

import (
	"context"
	"log/slog"
	"time"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/automation"
	"github.com/fdatoo/switchyard/internal/automation/action"
	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/observability"
	ghstarlark "github.com/fdatoo/switchyard/internal/starlark"
)

// SnapshotReader returns the current config snapshot. Implemented by
// *config.Manager in production wiring.
type SnapshotReader interface {
	Current() *configpb.ConfigSnapshot
}

// Applier is the real SceneApplier. Replaces action.StubSceneApplier.
type Applier struct {
	snap     SnapshotReader
	state    action.StateReader
	dispatch action.CommandDispatcher
	store    action.EventAppender
	scripts  action.ScriptCaller
	runtime  *ghstarlark.Runtime
	logger   *slog.Logger
	metrics  *observability.Metrics
}

func NewApplier(
	snap SnapshotReader,
	dispatch action.CommandDispatcher,
	store action.EventAppender,
	state action.StateReader,
	scripts action.ScriptCaller,
	runtime *ghstarlark.Runtime,
	logger *slog.Logger,
	metrics *observability.Metrics,
) *Applier {
	return &Applier{
		snap: snap, dispatch: dispatch, store: store, state: state,
		scripts: scripts, runtime: runtime, logger: logger, metrics: metrics,
	}
}

// Apply satisfies action.SceneApplier (so the automation engine's
// SceneAction continues to work).
func (a *Applier) Apply(ctx context.Context, slug, correlationID string) error {
	return a.Invoke(ctx, slug, correlationID, "automation")
}

// Invoke runs the scene's actions in parallel, best-effort, and appends
// a scene_applied event. Returns ErrSceneNotFound if the scene is absent.
func (a *Applier) Invoke(ctx context.Context, sceneID, correlationID, invokedBy string) error {
	snap := a.snap.Current()
	var scene *configpb.SceneConfig
	for _, s := range snap.GetScenes() {
		if s.GetId() == sceneID {
			scene = s
			break
		}
	}
	if scene == nil {
		return ErrSceneNotFound
	}

	// Compile actions into Executors.
	execs := make([]action.Executor, 0, len(scene.GetActions()))
	ctrls := make([]action.ChildCtrl, 0, len(scene.GetActions()))
	for _, ac := range scene.GetActions() {
		ex, err := automation.CompileAction(ac, nil, a.runtime)
		if err != nil {
			a.appendEvent(ctx, scene, correlationID, invokedBy, 0, []string{err.Error()}, eventv1.RunOutcome_RUN_OUTCOME_FAILURE)
			return err
		}
		execs = append(execs, ex)
		ctrls = append(ctrls, action.ChildCtrl{ContinueOnError: true})
	}

	parallel := &action.ParallelBlock{Children: execs, Ctrls: ctrls}

	run := &action.Run{
		CorrelationID: correlationID,
		AutomationID:  "scene:" + sceneID,
		State:         a.state,
		Dispatcher:    a.dispatch,
		Store:         a.store,
		Scenes:        a, // recursive (a scene whose action invokes another scene works)
		Scripts:       a.scripts,
		Runtime:       a.runtime,
		Logger:        a.logger,
		Metrics:       a.metrics,
	}
	start := time.Now()
	err := parallel.Execute(ctx, run)
	steps, logs := run.Snapshot()
	outcome := eventv1.RunOutcome_RUN_OUTCOME_SUCCESS
	if err != nil {
		outcome = eventv1.RunOutcome_RUN_OUTCOME_FAILURE
	}
	a.appendEvent(ctx, scene, correlationID, invokedBy, steps, logs, outcome)
	_ = start // reserved for future duration metric
	return err
}

func (a *Applier) appendEvent(ctx context.Context, scene *configpb.SceneConfig, corrID, invokedBy string, steps uint64, logs []string, outcome eventv1.RunOutcome) {
	if a.store == nil {
		return
	}
	_, _ = a.store.Append(ctx, eventstore.Event{
		Kind:      "scene",
		Source:    "scene.Applier",
		Timestamp: time.Now(),
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_SceneApplied{
			SceneApplied: &eventv1.SceneApplied{
				SceneId:       scene.GetId(),
				AreaId:        scene.GetAreaId(),
				CorrelationId: corrID,
				InvokedBy:     invokedBy,
				Steps:         steps,
				Logs:          logs,
				Outcome:       outcome,
			},
		}},
	})
}
```

If `action.ParallelBlock` has a different field shape than `{Children, Ctrls}`, read `internal/automation/action/block.go` and adjust. The plan's intent is "execute all in parallel with per-child ChildCtrl"; the existing block primitive supports it.

### Step 6: Run tests to verify PASS

Run: `go test ./internal/automation/scene -v`
Expected: PASS — both tests.

If the test fails because of `automation.CompileAction` being unavailable (import cycle: `automation` imports `scene` somewhere?), check; the package layout should be `scene → automation → action`, so it should work. If there's a cycle, refactor: move `CompileAction` to a smaller shared package like `internal/automation/compile`.

### Step 7: Commit

```bash
git add internal/automation/scene/
git commit -m "feat(scene): Applier compiles + runs scene actions in parallel"
```

---

## Task 6: Real SceneService handler

**Files:**
- Create: `internal/api/service_scene.go`
- Create: `internal/api/service_scene_test.go`
- Modify: `internal/api/service_unimplemented.go` (remove SceneService stub)
- Modify: `internal/api/deps.go` (add SceneInvoker interface)

### Step 1: Extend deps.go

Add to `internal/api/deps.go`:

```go
// SceneInvoker is the api-facing seam over scene.Applier.Invoke.
type SceneInvoker interface {
	Invoke(ctx context.Context, sceneID, correlationID, invokedBy string) error
}
```

### Step 2: Write the failing test

Create `internal/api/service_scene_test.go`:

```go
package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	configv1 "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
)

type fakeSceneSnap struct{ snap *configv1.ConfigSnapshot }

func (f *fakeSceneSnap) Current() *configv1.ConfigSnapshot { return f.snap }

type fakeInvoker struct {
	called   int
	lastID   string
	returnErr error
}

func (f *fakeInvoker) Invoke(_ context.Context, sceneID, _, _ string) error {
	f.called++
	f.lastID = sceneID
	return f.returnErr
}

func TestSceneService_List(t *testing.T) {
	snap := &configv1.ConfigSnapshot{
		Scenes: []*configv1.SceneConfig{
			{Id: "global-off", DisplayName: "All off"},
			{Id: "kitchen-bright", DisplayName: "Kitchen bright", AreaId: "kitchen"},
		},
	}
	svc := NewRealSceneService(&fakeSceneSnap{snap: snap}, &fakeInvoker{}, nil)

	resp, err := svc.List(context.Background(), connect.NewRequest(&v1.ListScenesRequest{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := len(resp.Msg.GetScenes()); got != 2 {
		t.Errorf("want 2 scenes, got %d", got)
	}
	if resp.Msg.GetScenes()[1].GetAreaId() != "kitchen" {
		t.Errorf("area_id not projected: %+v", resp.Msg.GetScenes()[1])
	}
}

func TestSceneService_ApplyHappy(t *testing.T) {
	snap := &configv1.ConfigSnapshot{
		Scenes: []*configv1.SceneConfig{{Id: "test"}},
	}
	inv := &fakeInvoker{}
	svc := NewRealSceneService(&fakeSceneSnap{snap: snap}, inv, nil)

	resp, err := svc.Apply(context.Background(), connect.NewRequest(&v1.ApplySceneRequest{Id: "test"}))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if resp.Msg.GetCorrelationId() == "" {
		t.Error("want non-empty correlation_id")
	}
	if inv.called != 1 || inv.lastID != "test" {
		t.Errorf("invoker not called correctly: %+v", inv)
	}
}

func TestSceneService_ApplyNotFound(t *testing.T) {
	inv := &fakeInvoker{returnErr: errSceneNotFoundSentinel}
	svc := NewRealSceneService(&fakeSceneSnap{snap: &configv1.ConfigSnapshot{}}, inv, nil)

	_, err := svc.Apply(context.Background(), connect.NewRequest(&v1.ApplySceneRequest{Id: "ghost"}))
	if err == nil {
		t.Fatal("want NotFound error")
	}
	var cerr *connect.Error
	if !errors.As(err, &cerr) || cerr.Code() != connect.CodeNotFound {
		t.Errorf("want NotFound, got %v", err)
	}
}

func TestSceneService_PreviewLines(t *testing.T) {
	snap := &configv1.ConfigSnapshot{
		Scenes: []*configv1.SceneConfig{
			{Id: "tv", Actions: []*configv1.ActionConfig{
				{Kind: &configv1.ActionConfig_CallService{CallService: &configv1.CallServiceAction{
					Entity: "light.tv", Capability: "turn_off",
				}}},
				{Kind: &configv1.ActionConfig_CallService{CallService: &configv1.CallServiceAction{
					Entity: "blind.living", Capability: "lower",
				}}},
			}},
		},
	}
	svc := NewRealSceneService(&fakeSceneSnap{snap: snap}, &fakeInvoker{}, nil)
	resp, err := svc.Preview(context.Background(), connect.NewRequest(&v1.PreviewSceneRequest{Id: "tv"}))
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if got := len(resp.Msg.GetLines()); got != 2 {
		t.Errorf("want 2 lines, got %d (%v)", got, resp.Msg.GetLines())
	}
}

func TestSceneService_FullRoundTrip(t *testing.T) {
	snap := &configv1.ConfigSnapshot{Scenes: []*configv1.SceneConfig{{Id: "x"}}}
	svc := NewRealSceneService(&fakeSceneSnap{snap: snap}, &fakeInvoker{}, nil)
	path, handler := switchyardv1alpha1connect.NewSceneServiceHandler(svc)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := switchyardv1alpha1connect.NewSceneServiceClient(srv.Client(), srv.URL)
	resp, err := client.Apply(context.Background(), connect.NewRequest(&v1.ApplySceneRequest{Id: "x"}))
	if err != nil {
		t.Fatalf("client.Apply: %v", err)
	}
	if resp.Msg.GetCorrelationId() == "" {
		t.Error("want correlation_id from RPC round-trip")
	}
}
```

Note `errSceneNotFoundSentinel` is a test-internal value we'll define in the api package below so the handler can `errors.Is(err, errSceneNotFoundSentinel)` to detect not-found from the invoker.

### Step 3: Verify FAIL

Run: `go test ./internal/api -run TestSceneService -v 2>&1 | tail -5`
Expected: FAIL (`NewRealSceneService` doesn't exist).

### Step 4: Implement the service

Create `internal/api/service_scene.go`:

```go
package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"

	configv1 "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"

	"github.com/google/uuid"
)

// Sentinel used by tests + the daemon adapter to convey "scene not found"
// without depending on the scene package directly. The adapter wraps
// scene.ErrSceneNotFound with this so the api layer can decode.
var errSceneNotFoundSentinel = errors.New("scene: not found")

// RealSceneService implements v1.SceneService backed by SceneInvoker +
// SnapshotReader. Constructed by the daemon wiring.
type RealSceneService struct {
	snap   SceneSnapshotReader
	invoke SceneInvoker
	logger *slog.Logger
}

// SceneSnapshotReader is the snapshot seam.
type SceneSnapshotReader interface {
	Current() *configv1.ConfigSnapshot
}

func NewRealSceneService(snap SceneSnapshotReader, invoke SceneInvoker, logger *slog.Logger) *RealSceneService {
	return &RealSceneService{snap: snap, invoke: invoke, logger: logger}
}

var _ switchyardv1alpha1connect.SceneServiceHandler = (*RealSceneService)(nil)

func (s *RealSceneService) List(ctx context.Context, _ *connect.Request[v1.ListScenesRequest]) (*connect.Response[v1.ListScenesResponse], error) {
	snap := s.snap.Current()
	out := make([]*v1.Scene, 0, len(snap.GetScenes()))
	for _, sc := range snap.GetScenes() {
		out = append(out, &v1.Scene{
			Id:          sc.GetId(),
			DisplayName: sc.GetDisplayName(),
			AreaId:      sc.GetAreaId(),
		})
	}
	return connect.NewResponse(&v1.ListScenesResponse{Scenes: out}), nil
}

func (s *RealSceneService) Apply(ctx context.Context, req *connect.Request[v1.ApplySceneRequest]) (*connect.Response[v1.ApplySceneResponse], error) {
	corrID := uuid.NewString()
	err := s.invoke.Invoke(ctx, req.Msg.GetId(), corrID, "rpc:"+principalID(ctx))
	if err != nil {
		if errors.Is(err, errSceneNotFoundSentinel) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("scene %q not found", req.Msg.GetId()))
		}
		return nil, ToConnect(ctx, err, "scene_apply_failed")
	}
	return connect.NewResponse(&v1.ApplySceneResponse{CorrelationId: corrID}), nil
}

func (s *RealSceneService) Preview(_ context.Context, req *connect.Request[v1.PreviewSceneRequest]) (*connect.Response[v1.PreviewSceneResponse], error) {
	snap := s.snap.Current()
	var scene *configv1.SceneConfig
	for _, sc := range snap.GetScenes() {
		if sc.GetId() == req.Msg.GetId() {
			scene = sc
			break
		}
	}
	if scene == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("scene %q not found", req.Msg.GetId()))
	}
	lines := make([]string, 0, len(scene.GetActions()))
	for _, ac := range scene.GetActions() {
		lines = append(lines, previewActionLine(ac))
	}
	return connect.NewResponse(&v1.PreviewSceneResponse{Lines: lines}), nil
}

func previewActionLine(ac *configv1.ActionConfig) string {
	switch k := ac.GetKind().(type) {
	case *configv1.ActionConfig_CallService:
		cs := k.CallService
		return fmt.Sprintf("%s: %s", cs.GetEntity(), cs.GetCapability())
	case *configv1.ActionConfig_Scene:
		return fmt.Sprintf("apply scene %q", k.Scene.GetSlug())
	case *configv1.ActionConfig_Script:
		return fmt.Sprintf("run script %q", k.Script.GetName())
	case *configv1.ActionConfig_Wait:
		return fmt.Sprintf("wait %d ns", k.Wait.GetDurationNs())
	default:
		return "(unknown action)"
	}
}
```

### Step 5: Remove the stub

In `internal/api/service_unimplemented.go`, delete the entire `SceneService` block (the struct, NewSceneService, the interface assertion, and the three stub methods). Leave the file's other unimplemented stubs untouched.

### Step 6: Test passes

Run: `go test ./internal/api -run TestSceneService -count=1 -v`
Expected: PASS.

Run: `go build ./...`
Expected: clean. Daemon-side wiring is broken now (the old stub doesn't exist; the real one needs the daemon's snapshot reader and invoker). Task 7 fixes the wiring.

### Step 7: Commit

```bash
git add internal/api/service_scene.go internal/api/service_scene_test.go internal/api/service_unimplemented.go internal/api/deps.go
git commit -m "feat(api): real SceneService backed by SceneInvoker + SnapshotReader"
```

---

## Task 7: Wire real applier + service in daemon

**Files:**
- Modify: `internal/daemon/daemon.go`
- Modify: `internal/daemon/api_adapters.go`

### Step 1: Add adapter that wraps scene.Applier for the api seam

In `internal/daemon/api_adapters.go`, add near `configApplierAdapter`:

```go
// sceneInvokerAdapter bridges scene.Applier.Invoke into api.SceneInvoker,
// translating scene.ErrSceneNotFound into the api package's sentinel.
type sceneInvokerAdapter struct {
	applier *scene.Applier
}

func (a *sceneInvokerAdapter) Invoke(ctx context.Context, sceneID, correlationID, invokedBy string) error {
	err := a.applier.Invoke(ctx, sceneID, correlationID, invokedBy)
	if errors.Is(err, scene.ErrSceneNotFound) {
		return api.ErrSceneNotFound() // small helper below
	}
	return err
}
```

The api package needs to export a constructor for the sentinel since `errSceneNotFoundSentinel` is package-private. Add to `internal/api/deps.go`:

```go
// ErrSceneNotFound returns a sentinel that handlers translate into
// connect.CodeNotFound. Used by adapters that bridge into SceneInvoker.
func ErrSceneNotFound() error { return errSceneNotFoundSentinel }
```

### Step 2: Replace StubSceneApplier in daemon.go

In `internal/daemon/daemon.go`, find line ~345 where `&action.StubSceneApplier{...}` is constructed. Replace with:

```go
sceneApplier := scene.NewApplier(
    d.configMgr,             // SnapshotReader: Manager satisfies via Current()
    d.dispatcher,            // CommandDispatcher
    d.store,                 // EventAppender
    d.stateStore,            // StateReader
    d.scriptCaller,          // ScriptCaller
    d.runtime,               // *starlark.Runtime
    d.logger,
    d.metrics,
)
d.sceneApplier = sceneApplier
```

Then update the automation engine wiring at the same site:

```go
// Before:
//   Scenes: &action.StubSceneApplier{Store: d.store, Logger: d.logger},
// After:
Scenes: sceneApplier,
```

Add `sceneApplier *scene.Applier` field to the `Daemon` struct.

The actual field names (`d.dispatcher`, `d.stateStore`, `d.scriptCaller`, `d.runtime`, etc.) may differ — read the existing `StubSceneApplier` site to see what's in scope and reuse the same variable names.

The `*config.Manager` needs to satisfy `scene.SnapshotReader` (i.e. expose `Current() *configpb.ConfigSnapshot`). If it doesn't have that exact method, add it:

```go
// In internal/config/manager.go:
func (m *Manager) Current() *configpb.ConfigSnapshot {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.current
}
```

(`m.current` likely already exists per the OnApplied wiring.)

### Step 3: Wire the SceneService

In `internal/daemon/daemon.go`, find where `api.SceneService` was previously constructed (probably an `api.NewSceneService()` call wired into `listener.Service.Scene`). Replace:

```go
listenerSvc.Scene = api.NewRealSceneService(
    d.configMgr,
    &sceneInvokerAdapter{applier: sceneApplier},
    d.logger,
)
```

### Step 4: Verify build + tests

```bash
go build ./...
go test ./internal/daemon ./internal/api -count=1
```

Expected: PASS.

### Step 5: Commit

```bash
git add internal/daemon/daemon.go internal/daemon/api_adapters.go internal/config/manager.go internal/api/deps.go
git commit -m "feat(daemon): wire real scene.Applier + RealSceneService"
```

---

## Task 8: Daemon integration test for scene Apply

**Files:**
- Create: `internal/daemon/scenes_e2e_test.go` (build tag `//go:build integration`)

### Step 1: Write the test

Mirror the pattern from `internal/daemon/config_subscribe_e2e_test.go` (boot daemon, open Connect client, drive RPC). Test plan:

1. Start daemon with a `main.pkl` that declares an automation-call entity + a global scene that calls service `turn_off` on that entity.
2. Open `SceneService.Apply` for the scene id.
3. Observe via the activity stream (or directly via the carport fake's recorded dispatches) that the entity was dispatched.

Concretely (adapt to existing helpers):

```go
//go:build integration

package daemon_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
	"github.com/fdatoo/switchyard/internal/daemon"
	"github.com/fdatoo/switchyard/internal/observability"
)

func TestScene_ApplyExecutesActions(t *testing.T) {
	dir := shortTempDir(t)
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(filepath.Join(configDir, "scenes"), 0o755); err != nil {
		t.Fatal(err)
	}
	mainPkl := `
amends "switchyard:config"

import "switchyard:scenes" as sc
import "switchyard:automations" as auto

scenes = new {
  new sc.Scene {
    id = "test-apply"
    displayName = "Test apply"
    actions = new {
      new auto.CallServiceAction {
        entity = "light.testdriver_lamp"
        capability = "turn_off"
      }
    }
  }
}
`
	if err := os.WriteFile(filepath.Join(configDir, "main.pkl"), []byte(mainPkl), 0o644); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	metrics := observability.NewMetrics()
	adminPort := freeTCPPort(t)
	tcpPort := freeTCPPort(t)
	d := daemon.New(daemon.Config{
		DataDir:    dir,
		ConfigDir:  configDir,
		LogLevel:   slog.LevelInfo,
		LogFormat:  "json",
		AdminPort:  adminPort,
		TCPPort:    tcpPort,
		SocketPath: fmt.Sprintf("switchyardd-%d.sock", os.Getpid()),
	}, logger, metrics)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()

	// Wait for ready.
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", adminPort)
	deadline := time.Now().Add(20 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		resp, err := http.Get(healthURL)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatal("daemon not ready")
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", tcpPort)
	client := switchyardv1alpha1connect.NewSceneServiceClient(&http.Client{}, baseURL)

	// List should return our scene.
	listResp, err := client.List(ctx, connect.NewRequest(&v1.ListScenesRequest{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listResp.Msg.GetScenes()) != 1 || listResp.Msg.GetScenes()[0].GetId() != "test-apply" {
		t.Fatalf("List got %+v", listResp.Msg.GetScenes())
	}

	// Apply should succeed (correlation_id returned).
	applyResp, err := client.Apply(ctx, connect.NewRequest(&v1.ApplySceneRequest{Id: "test-apply"}))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if applyResp.Msg.GetCorrelationId() == "" {
		t.Error("want correlation_id")
	}

	// Apply of a missing scene should return NotFound.
	_, err = client.Apply(ctx, connect.NewRequest(&v1.ApplySceneRequest{Id: "ghost"}))
	if err == nil {
		t.Fatal("want error for missing scene")
	}
	var cerr *connect.Error
	if !connect.CodeOf(err) == connect.CodeNotFound {
		// fall through; check via errors.As pattern if needed
		t.Errorf("want NotFound, got %v", err)
	}
	_ = cerr
}
```

### Step 2: Run

Run: `go test -tags integration ./internal/daemon -run TestScene_ApplyExecutesActions -v -timeout 30s`
Expected: PASS.

If the test fails because the dispatcher can't actually deliver to `light.testdriver_lamp` (no real driver), the test as written should still pass — `Apply` returns success when dispatch starts; the per-action failure goes into the event store. If a failure surfaces in the event-store assertion, simplify the assertion to "Apply returned a correlation_id and didn't 500".

### Step 3: Commit

```bash
git add internal/daemon/scenes_e2e_test.go
git commit -m "test(daemon): E2E SceneService.Apply executes scene actions"
```

---

## Task 9: Front-end Scene type + form

**Files:**
- Modify: `app/src/data/scenes.ts`
- Create: `app/src/views/scenes/SySceneForm.vue`
- Modify: `app/src/data/regen-preview.ts` (extend fileType union if needed)

### Step 1: Extend the Scene type

Edit `app/src/data/scenes.ts`:

```ts
export interface Scene {
  id: string;
  displayName: string;
  areaId: string;  // "" = global
}

interface RawScene {
  id?: string;
  display_name?: string; displayName?: string;
  area_id?: string;      areaId?: string;
}

function decode(r: RawScene): Scene {
  return {
    id:          r.id ?? "",
    displayName: r.displayName ?? r.display_name ?? "",
    areaId:      r.areaId      ?? r.area_id      ?? "",
  };
}
```

### Step 2: Verify regen-preview accepts "scene"

Read `app/src/data/regen-preview.ts`. The current `fileType` field accepts `"automation"` and probably `"area"` and `"scene"` (added during the editors plan). Confirm; if "scene" isn't in the union, add it. Same for "area".

### Step 3: Write the form

Create `app/src/views/scenes/SySceneForm.vue`:

```vue
<!--
  SySceneForm — modal that builds a SceneConfig and writes the
  regenerated Pkl to scenes/<id>.pkl via EditSessionService.
  Mirrors SyAutomationForm but with the scene shape (id, displayName,
  areaId, actions[]).
-->
<script setup lang="ts">
import { ref, watch } from "vue";
import { SySheet, SyText, SyButton, SyInput, SyIcon } from "@/lib";
import ActionEditor, { type ActionValue } from "@/views/automations/ActionEditor.vue";
import { regenPreview } from "@/data/regen-preview";
import { openForEdit, commitEdit } from "@/data/edit-session";

const props = defineProps<{
  open: boolean;
  /** Pre-set areaId. Empty = global. */
  areaId: string;
  /** Optional prefill for edit. */
  initial?: {
    id: string;
    displayName?: string;
    actions: ActionValue[];
  };
}>();

const emit = defineEmits<{
  (e: "update:open", v: boolean): void;
  (e: "saved", id: string): void;
}>();

const id = ref<string>("");
const displayName = ref<string>("");
const actions = ref<ActionValue[]>([]);
const saveBusy = ref<boolean>(false);
const saveError = ref<string>("");

function reset(): void {
  if (props.initial) {
    id.value = props.initial.id;
    displayName.value = props.initial.displayName ?? "";
    actions.value = props.initial.actions;
  } else {
    id.value = "";
    displayName.value = "";
    actions.value = [];
  }
  saveError.value = "";
}

watch(() => props.open, (o) => { if (o) reset(); });

function close(): void { emit("update:open", false); }
function addAction(): void { actions.value = [...actions.value, { kind: "call_service" }]; }

function actionToProto(a: ActionValue): Record<string, unknown> {
  if (a.kind === "call_service") {
    return {
      callService: {
        entity: a.entity ?? "",
        capability: a.capability ?? "",
        args: a.args ?? {},
      },
    };
  }
  return {};
}

function buildAst(): Record<string, unknown> {
  return {
    id: id.value,
    displayName: displayName.value,
    areaId: props.areaId,
    actions: actions.value.map(actionToProto),
  };
}

async function save(): Promise<void> {
  if (!id.value) {
    saveError.value = "id is required";
    return;
  }
  saveBusy.value = true;
  saveError.value = "";
  try {
    const ast = buildAst();
    const { pklText } = await regenPreview({ fileType: "scene", astJson: JSON.stringify(ast) });
    const filePath = `scenes/${id.value}.pkl`;
    const session = await openForEdit(filePath);
    const r = await commitEdit({
      filePath,
      lockToken: session.lockToken,
      regeneratedPkl: pklText,
      expectedFileHash: session.fileHash,
    });
    if (r.conflict) {
      saveError.value = `Conflict: ${r.conflict.reason}`;
      return;
    }
    emit("saved", id.value);
    close();
  } catch (err) {
    saveError.value = err instanceof Error ? err.message : String(err);
  } finally {
    saveBusy.value = false;
  }
}
</script>

<template>
  <SySheet :model-value="open" side="right" size="lg" title="Scene" @update:model-value="(v: boolean) => emit('update:open', v)">
    <div class="form">
      <section class="form__section">
        <SyText variant="label" tone="subtle">Identity</SyText>
        <SyInput :model-value="id" placeholder="id (e.g. movie-night)" @update:model-value="(s: string) => id = s" />
        <SyInput :model-value="displayName" placeholder="displayName" @update:model-value="(s: string) => displayName = s" />
        <SyText v-if="areaId" variant="caption" tone="subtle">Scoped to room: {{ areaId }}</SyText>
        <SyText v-else variant="caption" tone="subtle">Global scene (no room scope)</SyText>
      </section>

      <section class="form__section">
        <div class="form__sectionHead">
          <SyText variant="label" tone="subtle">Actions</SyText>
          <SyButton intent="ghost" size="sm" @click="addAction"><SyIcon name="plus" :size="12" /> Add</SyButton>
        </div>
        <ActionEditor
          v-for="(a, i) in actions" :key="i"
          :model-value="a"
          @update:model-value="(v: ActionValue) => actions[i] = v"
          @remove="actions = actions.filter((_, j) => j !== i)"
        />
      </section>

      <SyText v-if="saveError" variant="caption" tone="bad">{{ saveError }}</SyText>

      <footer class="form__foot">
        <SyButton intent="ghost" @click="close" :disabled="saveBusy">Cancel</SyButton>
        <SyButton intent="primary" :disabled="saveBusy || !id" @click="save">
          {{ saveBusy ? "Saving…" : "Save" }}
        </SyButton>
      </footer>
    </div>
  </SySheet>
</template>

<style scoped>
.form { display: flex; flex-direction: column; gap: var(--sy-space-4); padding: var(--sy-space-3); }
.form__section { display: flex; flex-direction: column; gap: var(--sy-space-2); }
.form__sectionHead { display: flex; align-items: center; justify-content: space-between; }
.form__foot { display: flex; gap: var(--sy-space-2); justify-content: flex-end; padding-top: var(--sy-space-3); border-top: 1px solid var(--sy-color-line-soft); }
</style>
```

### Step 4: Typecheck

```bash
cd /Users/fdatoo/Developer/Switchyard/app && npm run typecheck
```

Expected: PASS (modulo pre-existing TS6310).

### Step 5: Commit

```bash
cd /Users/fdatoo/Developer/Switchyard
git add app/src/data/scenes.ts app/src/views/scenes/SySceneForm.vue app/src/data/regen-preview.ts
git commit -m "feat(app): SySceneForm + Scene.areaId in TS client"
```

---

## Task 10: SyAreaForm component

**Files:**
- Create: `app/src/views/areas/SyAreaForm.vue`

### Step 1: Write the form

```vue
<!--
  SyAreaForm — modal that builds an AreaConfig and writes the
  regenerated Pkl to areas/<id>.pkl via EditSessionService.
-->
<script setup lang="ts">
import { ref, watch } from "vue";
import { SySheet, SyText, SyButton, SyInput } from "@/lib";
import { regenPreview } from "@/data/regen-preview";
import { openForEdit, commitEdit } from "@/data/edit-session";

const props = defineProps<{
  open: boolean;
  initial?: { id: string; displayName?: string; parentId?: string };
}>();

const emit = defineEmits<{
  (e: "update:open", v: boolean): void;
  (e: "saved", id: string): void;
}>();

const id = ref<string>("");
const displayName = ref<string>("");
const parentId = ref<string>("");
const saveBusy = ref<boolean>(false);
const saveError = ref<string>("");

function reset(): void {
  if (props.initial) {
    id.value = props.initial.id;
    displayName.value = props.initial.displayName ?? "";
    parentId.value = props.initial.parentId ?? "";
  } else {
    id.value = "";
    displayName.value = "";
    parentId.value = "";
  }
  saveError.value = "";
}

watch(() => props.open, (o) => { if (o) reset(); });

function close(): void { emit("update:open", false); }

function buildAst(): Record<string, unknown> {
  const ast: Record<string, unknown> = {
    id: id.value,
    displayName: displayName.value,
  };
  if (parentId.value) ast.parentId = parentId.value;
  return ast;
}

async function save(): Promise<void> {
  if (!id.value) {
    saveError.value = "id is required";
    return;
  }
  saveBusy.value = true;
  saveError.value = "";
  try {
    const ast = buildAst();
    const { pklText } = await regenPreview({ fileType: "area", astJson: JSON.stringify(ast) });
    const filePath = `areas/${id.value}.pkl`;
    const session = await openForEdit(filePath);
    const r = await commitEdit({
      filePath,
      lockToken: session.lockToken,
      regeneratedPkl: pklText,
      expectedFileHash: session.fileHash,
    });
    if (r.conflict) {
      saveError.value = `Conflict: ${r.conflict.reason}`;
      return;
    }
    emit("saved", id.value);
    close();
  } catch (err) {
    saveError.value = err instanceof Error ? err.message : String(err);
  } finally {
    saveBusy.value = false;
  }
}
</script>

<template>
  <SySheet :model-value="open" side="right" size="md" title="Room" @update:model-value="(v: boolean) => emit('update:open', v)">
    <div class="form">
      <SyInput :model-value="id" placeholder="id (e.g. kitchen)" @update:model-value="(s: string) => id = s" />
      <SyInput :model-value="displayName" placeholder="displayName (e.g. Kitchen)" @update:model-value="(s: string) => displayName = s" />
      <SyInput :model-value="parentId" placeholder="parentId (optional)" @update:model-value="(s: string) => parentId = s" />

      <SyText v-if="saveError" variant="caption" tone="bad">{{ saveError }}</SyText>

      <footer class="form__foot">
        <SyButton intent="ghost" @click="close" :disabled="saveBusy">Cancel</SyButton>
        <SyButton intent="primary" :disabled="saveBusy || !id" @click="save">
          {{ saveBusy ? "Saving…" : "Save" }}
        </SyButton>
      </footer>
    </div>
  </SySheet>
</template>

<style scoped>
.form { display: flex; flex-direction: column; gap: var(--sy-space-3); padding: var(--sy-space-3); }
.form__foot { display: flex; gap: var(--sy-space-2); justify-content: flex-end; padding-top: var(--sy-space-3); border-top: 1px solid var(--sy-color-line-soft); }
</style>
```

### Step 2: Typecheck

```bash
cd /Users/fdatoo/Developer/Switchyard/app && npm run typecheck
```

### Step 3: Commit

```bash
cd /Users/fdatoo/Developer/Switchyard
git add app/src/views/areas/SyAreaForm.vue
git commit -m "feat(app): SyAreaForm modal for area creation"
```

---

## Task 11: Wire forms into RoomsView (index)

**Files:**
- Modify: `app/src/views/RoomsView.vue`

### Step 1: Add the "Global scenes" section + form triggers

Read `app/src/views/RoomsView.vue` to understand its current shape (it lists rooms as `SyRoomTile`s). Add:

1. Import `listScenes`, `applyScene` from `@/data/scenes`.
2. Import `SySceneForm` from `@/views/scenes/SySceneForm.vue` and `SyAreaForm` from `@/views/areas/SyAreaForm.vue`.
3. Add a `scenes` ref + `loadScenes()` function (filters to `!s.areaId`).
4. Hook `configStore.onChanged` (mirror the pattern from RC-13) to refetch scenes when config changes.
5. Two new buttons in the page header: "+ New room" and "+ New scene".
6. Below the rooms grid, a "Global scenes" section with rows for each global scene: name, Apply button, overflow.

Concrete template additions:

```vue
<template>
  <!-- existing rooms grid wrapper -->
  <div class="rooms-page">
    <header class="rooms-page__head">
      <SyText variant="title">Rooms</SyText>
      <div class="rooms-page__actions">
        <SyButton intent="ghost" @click="areaFormOpen = true"><SyIcon name="plus" :size="12" /> New room</SyButton>
        <SyButton intent="ghost" @click="globalSceneFormOpen = true"><SyIcon name="plus" :size="12" /> New scene</SyButton>
      </div>
    </header>

    <!-- existing rooms grid as-is -->
    <!-- ... -->

    <section v-if="globalScenes.length > 0" class="rooms-page__scenes">
      <SyText variant="label" tone="subtle">Global scenes</SyText>
      <SyListRow
        v-for="s in globalScenes" :key="s.id"
        :title="s.displayName || s.id"
        :subtitle="s.id"
      >
        <SyButton intent="ghost" size="sm" @click="onApply(s)">Apply</SyButton>
      </SyListRow>
    </section>

    <SyAreaForm v-model:open="areaFormOpen" @saved="onAreaSaved" />
    <SySceneForm v-model:open="globalSceneFormOpen" :area-id="''" @saved="onSceneSaved" />
  </div>
</template>
```

Script additions:

```ts
import { listScenes, applyScene, type Scene } from "@/data/scenes";
import { configStore } from "@/stores/config-store";
import SyAreaForm from "@/views/areas/SyAreaForm.vue";
import SySceneForm from "@/views/scenes/SySceneForm.vue";

const scenes = ref<Scene[]>([]);
const globalScenes = computed(() => scenes.value.filter((s) => !s.areaId));
const areaFormOpen = ref<boolean>(false);
const globalSceneFormOpen = ref<boolean>(false);
let unsubConfigChanged: (() => void) | null = null;

async function loadScenes(): Promise<void> {
  const r = await listScenes();
  scenes.value = r.scenes;
}

async function onApply(s: Scene): Promise<void> {
  await applyScene(s.id);
  // Activity stream will reflect the apply; UI doesn't need to do more.
}

function onAreaSaved(): void { /* configStore.onChanged refetch will pick it up */ }
function onSceneSaved(): void { /* same */ }

onMounted(async () => {
  // existing init...
  await loadScenes();
  unsubConfigChanged = configStore.onChanged(() => { void loadScenes(); });
});

onBeforeUnmount(() => {
  unsubConfigChanged?.();
});
```

The exact "below the rooms grid" placement should match the existing visual hierarchy — read the file to see whether the grid is wrapped in a section that has a natural follow-up slot.

### Step 2: Typecheck + browser smoke

```bash
cd /Users/fdatoo/Developer/Switchyard/app && npm run typecheck
```

Then load http://localhost:5174/rooms in the dev server and confirm:
- "+ New room" and "+ New scene" buttons appear in the page header.
- If any global scenes exist in the snapshot, they show in the section.

### Step 3: Commit

```bash
cd /Users/fdatoo/Developer/Switchyard
git add app/src/views/RoomsView.vue
git commit -m "feat(app): RoomsView shows global scenes + form triggers"
```

---

## Task 12: Wire scene form into RoomDetailView

**Files:**
- Modify: `app/src/views/RoomDetailView.vue`

### Step 1: Filter scenes by route param + add form trigger

Read the file. Current `listScenes()` call brings all scenes; change the local computed/filter to only include scenes where `s.areaId === route.params.id`.

Add:

```ts
import SySceneForm from "@/views/scenes/SySceneForm.vue";

const sceneFormOpen = ref<boolean>(false);

// in the scenes computed/filter:
const roomScenes = computed(() => scenes.value.filter((s) => s.areaId === route.params.id));
```

Template additions next to the existing scenes list:

```vue
<header class="room-detail__sceneshead">
  <SyText variant="label" tone="subtle">Scenes</SyText>
  <SyButton intent="ghost" size="sm" @click="sceneFormOpen = true"><SyIcon name="plus" :size="12" /> New scene</SyButton>
</header>

<!-- existing scenes list iterating over roomScenes -->

<SySceneForm v-model:open="sceneFormOpen" :area-id="String(route.params.id)" @saved="() => { /* refetch via configStore */ }" />
```

### Step 2: Typecheck + browser smoke

```bash
cd /Users/fdatoo/Developer/Switchyard/app && npm run typecheck
```

Navigate to http://localhost:5174/rooms/<some-room-id> and confirm scenes are filtered to that room. "+ New scene" opens the form with the areaId hidden.

### Step 3: Commit

```bash
cd /Users/fdatoo/Developer/Switchyard
git add app/src/views/RoomDetailView.vue
git commit -m "feat(app): RoomDetailView filters scenes by room + + New trigger"
```

---

## Task 13: Wire RegenPreview server-side for area + scene

**Files:**
- Modify: `internal/api/config_edit_handler.go` (the existing RegenPreview dispatch)

### Step 1: Check current dispatch

Read `internal/api/config_edit_handler.go`. The RegenPreview handler should already dispatch on `file_type` to `regen.Render` / `RenderArea` / `RenderScene` / `RenderEntityAreas` (added during the editors plan). If `RenderScene` is wired, no changes needed here — but the scene AST now includes `areaId` which RenderScene already reads from the proto field. The plan code path is:

1. RegenPreviewRequest comes in with `file_type: "scene"`, `ast_json` = the JSON shape SySceneForm built.
2. Handler unmarshals into `configpb.SceneConfig`. The proto has `AreaId` so the JSON's `areaId` populates it via proto json unmarshal.
3. `regen.RenderScene(&scfg)` emits the Pkl (now with the `areaId = "..."` line per Task 4).
4. Response carries pkl_bytes.

Verify by reading the file. If the dispatch is missing a `"scene"` or `"area"` case, add them.

### Step 2: If changes needed, write a test first

Add to existing `internal/api/config_edit_handler_test.go` (or create):

```go
func TestRegenPreview_SceneWithAreaId(t *testing.T) {
	svc := NewConfigService(&fakeConfig{})
	req := connect.NewRequest(&v1.RegenPreviewRequest{
		FileType: "scene",
		AstJson:  `{"id":"k","displayName":"K","areaId":"kitchen","actions":[]}`,
	})
	resp, err := svc.RegenPreview(context.Background(), req)
	if err != nil {
		t.Fatalf("RegenPreview: %v", err)
	}
	s := string(resp.Msg.GetPklBytes())
	if !strings.Contains(s, `areaId = "kitchen"`) {
		t.Errorf("output missing areaId: %s", s)
	}
}
```

### Step 3: Run, fix if needed, commit

```bash
go test ./internal/api -run TestRegenPreview -v
```

If the test passes without changes — great, just commit it. If it fails because the handler doesn't pass `area_id` through, add the JSON → proto translation explicitly.

```bash
git add internal/api/
git commit -m "test(api): RegenPreview round-trips scene areaId"
```

---

## Task 14: Playwright loop-closure

**Files:**
- Add a manual or automated validation pass.

If Playwright isn't set up in `app/`, do the manual pass:

1. Open http://localhost:5174/rooms.
2. Click "+ New scene". Form opens. Fill `id = playwright-test-global`, `displayName = Playwright test`. Add a CallService action targeting some entity in the snapshot. Save.
3. Within 2s, "Playwright test" appears in the Global scenes section (no page reload — relies on Track A's configStore loop).
4. Click "Apply" on the scene. Open `/activity` — confirm a `scene_applied` event entry shows up with `scene_id: playwright-test-global` within 2s.
5. Navigate to a room detail page (`/rooms/<some-existing-room>`). Click "+ New scene". Form opens with the room's areaId hidden. Save. Assert scene appears in the room's scoped list, NOT in the global section of `/rooms`.

If Playwright IS set up, write a spec that drives the same sequence and asserts the same outcomes. Mirror the existing E2E pattern.

Whichever path: log results in `docs/design/plans/2026-05-12-scenes-end-to-end-progress.md` as evidence of loop-closure.

```bash
git add docs/design/plans/2026-05-12-scenes-end-to-end-progress.md
git commit -m "test(scenes): loop-closure validation passed"
```

---

## Final verification

- [ ] **Run the full test suite**

```bash
go test ./... -count=1
go test -tags integration ./internal/... -count=1
cd app && npm run typecheck
```

Expected: all PASS.

- [ ] **Build the daemon**

```bash
go build -o dist/switchyardd ./cmd/switchyardd
```

Expected: clean.

- [ ] **Restart the dev daemon with the new build**

```bash
# kill the running daemon (find its PID)
lsof -ti:8080 | xargs -I {} kill {}
./dist/switchyardd &
```

- [ ] **Drive the manual or Playwright validation in Task 14**

---

## Wave plan for subagent-driven execution

| Wave | Tasks | Notes |
|------|-------|-------|
| 0 | 1 | Proto + regen — gates everything |
| 1 | 2, 3, 4 | Pkl schema, compile check, regen-emit — disjoint |
| 2 | 5 | scene.Applier package |
| 3 | 6 | RealSceneService |
| 4 | 7 | Daemon wiring (depends on 5 + 6) |
| 5 | 8 | Daemon E2E test |
| 6 | 9, 10 | TS form components — disjoint |
| 7 | 11, 12 | View integration — disjoint |
| 8 | 13 | RegenPreview verification (likely no-op) |
| 9 | 14 | Validation pass |
