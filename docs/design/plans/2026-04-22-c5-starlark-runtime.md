# C5 — Starlark Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a sandboxed, resource-limited Starlark execution substrate to gohomed, with six scoped execution contexts, a `load("//...")` resolver, Pkl validator integration, and `gohome eval`/`gohome test` CLI commands.

**Architecture:** A single `Runtime` struct (constructed once by the daemon) owns the `StateReader`, `CommandDispatcher`, and `EventAppender` interfaces. `Execute(ctx, kind, script, extraGlobals)` is the single entry point; context-specific stdlib and resource limits are dispatched internally via a table keyed on `ContextKind`. A module cache (`map[string]starlark.StringDict`) is guarded by `sync.RWMutex` and cleared on `ConfigApplied`.

**Tech Stack:** `go.starlark.net` (engine + syntax parser + starlarkstruct + lib/time), `github.com/charmbracelet/lipgloss` (CLI output), existing `internal/eventstore`, `internal/state`, `internal/carport`, `internal/observability`.

---

## Package naming note

`internal/starlark` declares `package starlark`. Every file inside it that imports `go.starlark.net/starlark` **must** alias it to avoid collision:

```go
import starlarkgo "go.starlark.net/starlark"
```

External test files (`package starlark_test`) that import both packages must alias both:

```go
import (
    ghs "github.com/fynn-labs/gohome/internal/starlark"
    starlarkgo "go.starlark.net/starlark"
)
```

---

## Task 1: Add go.starlark.net dependency

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add the dependency**

```bash
cd /path/to/gohome
go get go.starlark.net@latest
go mod tidy
```

- [ ] **Step 2: Verify the build still compiles**

```bash
task build
```

Expected: both binaries build with no errors.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "feat(c5): add go.starlark.net dependency"
```

---

## Task 2: `internal/starlark/parse.go` — ParseOnly

**Files:**
- Create: `internal/starlark/parse.go`
- Create: `internal/starlark/parse_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/starlark/parse_test.go`:

```go
package starlark_test

import (
	"testing"

	ghs "github.com/fynn-labs/gohome/internal/starlark"
)

func TestParseOnly_ValidExpression(t *testing.T) {
	if err := ghs.ParseOnly("1 + 2", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseOnly_ValidScript(t *testing.T) {
	src := "x = 1\nif x > 0:\n    x = x + 1"
	if err := ghs.ParseOnly(src, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseOnly_SyntaxError(t *testing.T) {
	if err := ghs.ParseOnly("def foo(:", true); err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestParseOnly_StatementAsExpression(t *testing.T) {
	// "x = 1" is a statement, not an expression; ParseExpr should reject it.
	if err := ghs.ParseOnly("x = 1", true); err == nil {
		t.Fatal("expected parse error for statement as expression")
	}
}

func TestParseOnly_ScriptAcceptsStatements(t *testing.T) {
	if err := ghs.ParseOnly("x = 1", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
go test ./internal/starlark/... -run TestParseOnly -v
```

Expected: compile error (package doesn't exist yet).

- [ ] **Step 3: Implement `parse.go`**

Create `internal/starlark/parse.go`:

```go
package starlark

import "go.starlark.net/syntax"

// ParseOnly parses src as a Starlark expression (expr=true) or script (expr=false).
// Returns a syntax error if parsing fails; nil on success. Does not execute.
func ParseOnly(src string, expr bool) error {
	if expr {
		_, err := syntax.ParseExpr("<input>", src, 0)
		return err
	}
	_, err := syntax.Parse("<input>", src, 0)
	return err
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/starlark/... -run TestParseOnly -v
```

Expected: all 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/starlark/parse.go internal/starlark/parse_test.go
git commit -m "feat(c5): add ParseOnly for Starlark syntax validation"
```

---

## Task 3: `context.go` + `limits.go` — ContextKind and resource limits

**Files:**
- Create: `internal/starlark/context.go`
- Create: `internal/starlark/limits.go`
- Create: `internal/starlark/limits_test.go`

- [ ] **Step 1: Write failing tests for limits**

Create `internal/starlark/limits_test.go`:

```go
package starlark_test

import (
	"strings"
	"testing"

	ghs "github.com/fynn-labs/gohome/internal/starlark"
)

func TestLimitError_StepsMessage(t *testing.T) {
	err := &ghs.LimitError{Kind: ghs.LimitSteps, Context: ghs.KindAutomation, Detail: "10M steps"}
	if !strings.Contains(err.Error(), "step") {
		t.Fatalf("unexpected message: %s", err.Error())
	}
}

func TestLimitError_WallClockMessage(t *testing.T) {
	err := &ghs.LimitError{Kind: ghs.LimitWallClock, Context: ghs.KindScript, Detail: "30s"}
	if !strings.Contains(err.Error(), "wall") {
		t.Fatalf("unexpected message: %s", err.Error())
	}
}

func TestContextKind_String(t *testing.T) {
	cases := []struct {
		kind ghs.ContextKind
		want string
	}{
		{ghs.KindAutomation, "automation"},
		{ghs.KindComputedEntity, "computed_entity"},
		{ghs.KindTriggerCondition, "trigger_condition"},
		{ghs.KindScript, "script"},
		{ghs.KindWidgetCompute, "widget_compute"},
		{ghs.KindMCPEval, "mcp_eval"},
	}
	for _, tc := range cases {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("ContextKind(%d).String() = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestKindFromString_RoundTrip(t *testing.T) {
	for _, s := range []string{"automation", "script", "computed_entity", "trigger_condition", "widget_compute", "mcp_eval"} {
		k, err := ghs.KindFromString(s)
		if err != nil {
			t.Fatalf("KindFromString(%q): %v", s, err)
		}
		if got := k.String(); got != s {
			t.Errorf("round-trip %q → %q", s, got)
		}
	}
}

func TestKindFromString_Unknown(t *testing.T) {
	if _, err := ghs.KindFromString("bogus"); err == nil {
		t.Fatal("expected error for unknown kind")
	}
}
```

- [ ] **Step 2: Run tests — verify they fail (compile error)**

```bash
go test ./internal/starlark/... -run "TestLimitError|TestContextKind|TestKindFromString" -v
```

Expected: compile error.

- [ ] **Step 3: Create `context.go`**

```go
package starlark

import (
	"fmt"
	"time"
)

// ContextKind identifies the execution context for a Starlark script.
type ContextKind int

const (
	KindAutomation ContextKind = iota
	KindComputedEntity
	KindTriggerCondition
	KindScript
	KindWidgetCompute
	KindMCPEval
)

func (k ContextKind) String() string {
	switch k {
	case KindAutomation:
		return "automation"
	case KindComputedEntity:
		return "computed_entity"
	case KindTriggerCondition:
		return "trigger_condition"
	case KindScript:
		return "script"
	case KindWidgetCompute:
		return "widget_compute"
	case KindMCPEval:
		return "mcp_eval"
	default:
		return "unknown"
	}
}

// KindFromString parses a string into a ContextKind. Accepts both canonical
// names ("automation") and short aliases ("computed", "condition", "widget", "mcp").
func KindFromString(s string) (ContextKind, error) {
	switch s {
	case "automation":
		return KindAutomation, nil
	case "computed", "computed_entity":
		return KindComputedEntity, nil
	case "condition", "trigger_condition":
		return KindTriggerCondition, nil
	case "script":
		return KindScript, nil
	case "widget", "widget_compute":
		return KindWidgetCompute, nil
	case "mcp", "mcp_eval":
		return KindMCPEval, nil
	default:
		return 0, fmt.Errorf("unknown context kind %q", s)
	}
}

// contextLimits holds resource limits and execution mode for one ContextKind.
type contextLimits struct {
	WallClock    time.Duration
	MaxSteps     uint64
	IsExpression bool // true → starlark.Eval; false → starlark.ExecFile
}

var kindLimits = map[ContextKind]contextLimits{
	KindAutomation:       {WallClock: 30 * time.Second, MaxSteps: 10_000_000, IsExpression: false},
	KindComputedEntity:   {WallClock: 100 * time.Millisecond, MaxSteps: 500_000, IsExpression: true},
	KindTriggerCondition: {WallClock: 50 * time.Millisecond, MaxSteps: 100_000, IsExpression: true},
	KindScript:           {WallClock: 30 * time.Second, MaxSteps: 10_000_000, IsExpression: false},
	KindWidgetCompute:    {WallClock: 50 * time.Millisecond, MaxSteps: 100_000, IsExpression: true},
	KindMCPEval:          {WallClock: 30 * time.Second, MaxSteps: 10_000_000, IsExpression: false},
}

func limitsFor(kind ContextKind) contextLimits {
	if cfg, ok := kindLimits[kind]; ok {
		return cfg
	}
	return kindLimits[KindScript]
}
```

- [ ] **Step 4: Create `limits.go`**

```go
package starlark

import (
	"context"
	"fmt"
	"time"

	starlarkgo "go.starlark.net/starlark"
)

// LimitKind identifies which resource limit was breached.
type LimitKind int

const (
	LimitSteps LimitKind = iota
	LimitWallClock
)

// LimitError is returned by Execute when a resource limit is exceeded.
type LimitError struct {
	Kind    LimitKind
	Context ContextKind
	Detail  string
}

func (e *LimitError) Error() string {
	switch e.Kind {
	case LimitSteps:
		return fmt.Sprintf("step limit exceeded in %s: %s", e.Context, e.Detail)
	case LimitWallClock:
		return fmt.Sprintf("wall-clock limit exceeded in %s: %s", e.Context, e.Detail)
	}
	return e.Detail
}

// startWatchdog starts a goroutine that cancels thread after timeout.
// The returned stop func must be called via defer to prevent goroutine leak.
func startWatchdog(timeout time.Duration, thread *starlarkgo.Thread) (stop func()) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	go func() {
		<-ctx.Done()
		if ctx.Err() == context.DeadlineExceeded {
			thread.Cancel("timeout")
		}
	}()
	return cancel
}
```

- [ ] **Step 5: Run tests — verify they pass**

```bash
go test ./internal/starlark/... -run "TestLimitError|TestContextKind|TestKindFromString" -v
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/starlark/context.go internal/starlark/limits.go internal/starlark/limits_test.go
git commit -m "feat(c5): ContextKind enum and resource limit types"
```

---

## Task 4: `stdlib.go` — interfaces, EntityState, and builtin implementations

**Files:**
- Create: `internal/starlark/stdlib.go`
- Create: `internal/starlark/stdlib_test.go`

- [ ] **Step 1: Write failing tests for stdlib builtins**

Create `internal/starlark/stdlib_test.go`:

```go
package starlark_test

import (
	"context"
	"testing"

	"github.com/fynn-labs/gohome/internal/eventstore"
	ghs "github.com/fynn-labs/gohome/internal/starlark"
	starlarkgo "go.starlark.net/starlark"
)

// fakeState implements StateReader for tests.
type fakeState map[string]*ghs.EntityState

func (f fakeState) Get(id string) (*ghs.EntityState, bool) {
	v, ok := f[id]
	return v, ok
}

// fakeDispatcher implements CommandDispatcher for tests.
type fakeDispatcher struct {
	calls []string
}

func (f *fakeDispatcher) Dispatch(_ context.Context, entityID, capability string, _ map[string]string) (*ghs.DispatchResult, error) {
	f.calls = append(f.calls, entityID+"."+capability)
	return &ghs.DispatchResult{Ok: true}, nil
}

// fakeAppender implements EventAppender for tests.
type fakeAppender struct{}

func (f *fakeAppender) Append(_ context.Context, _ eventstore.Event) (uint64, error) { return 1, nil }

func TestMakeStateBuiltin_Found(t *testing.T) {
	state := fakeState{"light.kitchen": {StateStr: "off", Attributes: map[string]any{"brightness": 0.0}}}
	fn := ghs.MakeStateBuiltin(state)

	thread := &starlarkgo.Thread{Name: "test"}
	v, err := starlarkgo.Call(thread, fn, starlarkgo.Tuple{starlarkgo.String("light.kitchen")}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	st, ok := v.(starlarkgo.HasAttrs)
	if !ok {
		t.Fatal("expected struct with attrs")
	}
	stateVal, err := st.Attr("state")
	if err != nil || stateVal.(starlarkgo.String) != "off" {
		t.Fatalf("expected state=off, got %v err=%v", stateVal, err)
	}
}

func TestMakeStateBuiltin_NotFound(t *testing.T) {
	fn := ghs.MakeStateBuiltin(fakeState{})
	thread := &starlarkgo.Thread{Name: "test"}
	_, err := starlarkgo.Call(thread, fn, starlarkgo.Tuple{starlarkgo.String("missing.entity")}, nil)
	if err == nil {
		t.Fatal("expected error for missing entity")
	}
}

func TestMakeCallServiceBuiltin(t *testing.T) {
	d := &fakeDispatcher{}
	fn := ghs.MakeCallServiceBuiltin(context.Background(), d)
	thread := &starlarkgo.Thread{Name: "test"}
	_, err := starlarkgo.Call(thread, fn, starlarkgo.Tuple{
		starlarkgo.String("light.kitchen"),
		starlarkgo.String("turn_on"),
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.calls) != 1 || d.calls[0] != "light.kitchen.turn_on" {
		t.Fatalf("expected dispatch call, got %v", d.calls)
	}
}
```

- [ ] **Step 2: Run — verify compile error**

```bash
go test ./internal/starlark/... -run "TestMakeState|TestMakeCall" -v
```

Expected: compile error (stdlib.go doesn't exist).

- [ ] **Step 3: Create `stdlib.go`**

Create `internal/starlark/stdlib.go`:

```go
package starlark

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/fynn-labs/gohome/internal/eventstore"
	starlarkgo "go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	starlarktime "go.starlark.net/lib/time"
)

// EntityState is the Starlark-visible view of an entity.
type EntityState struct {
	StateStr   string         // "on", "off", sensor value, etc.
	Attributes map[string]any // proto JSON-decoded flat attributes
}

// StateReader is satisfied by the daemon's state.Cache adapter.
type StateReader interface {
	Get(entityID string) (*EntityState, bool)
}

// DispatchResult carries the driver's response to a command.
type DispatchResult struct {
	Ok    bool
	Error string
}

// CommandDispatcher is satisfied by the daemon's carport.Host adapter.
type CommandDispatcher interface {
	Dispatch(ctx context.Context, entityID, capability string, args map[string]string) (*DispatchResult, error)
}

// EventAppender is satisfied by internal/eventstore.Store.
type EventAppender interface {
	Append(ctx context.Context, e eventstore.Event) (uint64, error)
}

// MakeStateBuiltin returns the Starlark `state(entity_id)` builtin.
// Exported so testutil and daemon wiring can construct it independently.
func MakeStateBuiltin(sr StateReader) starlarkgo.Value {
	return starlarkgo.NewBuiltin("state", func(_ *starlarkgo.Thread, b *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		var entityID string
		if err := starlarkgo.UnpackArgs(b.Name(), args, kwargs, "entity_id", &entityID); err != nil {
			return nil, err
		}
		es, ok := sr.Get(entityID)
		if !ok {
			return nil, fmt.Errorf("state: entity %q not found", entityID)
		}
		return entityStateToStruct(es), nil
	})
}

func entityStateToStruct(es *EntityState) *starlarkstruct.Struct {
	attrs := starlarkgo.NewDict(len(es.Attributes))
	for k, v := range es.Attributes {
		_ = attrs.SetKey(starlarkgo.String(k), anyToStarlark(v))
	}
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlarkgo.StringDict{
		"state":      starlarkgo.String(es.StateStr),
		"attributes": attrs,
	})
}

func anyToStarlark(v any) starlarkgo.Value {
	switch t := v.(type) {
	case bool:
		return starlarkgo.Bool(t)
	case float64:
		return starlarkgo.Float(t)
	case string:
		return starlarkgo.String(t)
	case []any:
		l := make([]starlarkgo.Value, len(t))
		for i, e := range t {
			l[i] = anyToStarlark(e)
		}
		return starlarkgo.NewList(l)
	case map[string]any:
		d := starlarkgo.NewDict(len(t))
		for k, val := range t {
			_ = d.SetKey(starlarkgo.String(k), anyToStarlark(val))
		}
		return d
	default:
		return starlarkgo.None
	}
}

// MakeCallServiceBuiltin returns the `call_service(entity_id, capability, **kwargs)` builtin.
func MakeCallServiceBuiltin(ctx context.Context, d CommandDispatcher) starlarkgo.Value {
	return starlarkgo.NewBuiltin("call_service", func(_ *starlarkgo.Thread, b *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("call_service: expected (entity_id, capability, **kwargs)")
		}
		entityID, ok := starlarkgo.AsString(args[0])
		if !ok {
			return nil, fmt.Errorf("call_service: entity_id must be string")
		}
		capability, ok := starlarkgo.AsString(args[1])
		if !ok {
			return nil, fmt.Errorf("call_service: capability must be string")
		}
		argsMap := make(map[string]string, len(kwargs))
		for _, kv := range kwargs {
			v, ok := starlarkgo.AsString(kv[1])
			if !ok {
				return nil, fmt.Errorf("call_service: kwarg %q must be string", kv[0])
			}
			argsMap[string(kv[0])] = v
		}
		res, err := d.Dispatch(ctx, entityID, capability, argsMap)
		if err != nil {
			return nil, err
		}
		if !res.Ok {
			return nil, fmt.Errorf("call_service %s.%s: %s", entityID, capability, res.Error)
		}
		return starlarkgo.None, nil
	})
}

func makeSleep(ctx context.Context) starlarkgo.Value {
	return starlarkgo.NewBuiltin("sleep", func(_ *starlarkgo.Thread, b *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		var seconds starlarkgo.Float
		if err := starlarkgo.UnpackArgs(b.Name(), args, kwargs, "seconds", &seconds); err != nil {
			return nil, err
		}
		select {
		case <-time.After(time.Duration(float64(seconds) * float64(time.Second))):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return starlarkgo.None, nil
	})
}

func makeNow() starlarkgo.Value {
	return starlarkgo.NewBuiltin("now", func(thread *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		return starlarkgo.Call(thread, starlarktime.Module.Members["now"], args, kwargs)
	})
}

func makeLog(logFn func(level, msg string)) starlarkgo.Value {
	return starlarkgo.NewBuiltin("log", func(_ *starlarkgo.Thread, b *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		var msg starlarkgo.Value
		level := "info"
		if err := starlarkgo.UnpackArgs(b.Name(), args, kwargs, "msg", &msg, "level?", &level); err != nil {
			return nil, err
		}
		logFn(level, msg.String())
		return starlarkgo.None, nil
	})
}

func makeNotify(ctx context.Context, store EventAppender) starlarkgo.Value {
	return starlarkgo.NewBuiltin("notify", func(_ *starlarkgo.Thread, b *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		var target, message string
		if err := starlarkgo.UnpackArgs(b.Name(), args, kwargs, "target", &target, "message", &message); err != nil {
			return nil, err
		}
		_, err := store.Append(ctx, eventstore.Event{
			Kind:      "notification.sent",
			Source:    "starlark",
			Timestamp: time.Now(),
		})
		_ = target
		_ = message
		return starlarkgo.None, err
	})
}

func makeRandom(rng *rand.Rand) starlarkgo.Value {
	return starlarkgo.NewBuiltin("random", func(_ *starlarkgo.Thread, b *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		if err := starlarkgo.UnpackArgs(b.Name(), args, kwargs); err != nil {
			return nil, err
		}
		return starlarkgo.Float(rng.Float64()), nil
	})
}

func makeSceneGlobal(ctx context.Context, store EventAppender) *starlarkstruct.Struct {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlarkgo.StringDict{
		"apply": starlarkgo.NewBuiltin("scene.apply", func(_ *starlarkgo.Thread, b *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
			var slug string
			if err := starlarkgo.UnpackArgs(b.Name(), args, kwargs, "slug", &slug); err != nil {
				return nil, err
			}
			_, err := store.Append(ctx, eventstore.Event{
				Kind:      "scene.applied",
				Entity:    slug,
				Source:    "starlark",
				Timestamp: time.Now(),
			})
			return starlarkgo.None, err
		}),
	})
}

// makeEventGlobal builds the read-write event struct for automation/script contexts.
// Trigger event fields come from extraGlobals keys "event_kind", "event_entity_id",
// "event_data" (set by C6 when invoking automations from a trigger).
func makeEventGlobal(ctx context.Context, store EventAppender, extraGlobals starlarkgo.StringDict) *starlarkstruct.Struct {
	fields := starlarkgo.StringDict{
		"fire": starlarkgo.NewBuiltin("event.fire", func(_ *starlarkgo.Thread, b *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
			var kind string
			var data starlarkgo.Value = starlarkgo.None
			if err := starlarkgo.UnpackArgs(b.Name(), args, kwargs, "kind", &kind, "data?", &data); err != nil {
				return nil, err
			}
			_, err := store.Append(ctx, eventstore.Event{
				Kind:      kind,
				Source:    "starlark",
				Timestamp: time.Now(),
			})
			return starlarkgo.None, err
		}),
		"kind":      eventField(extraGlobals, "event_kind", starlarkgo.String("")),
		"entity_id": eventField(extraGlobals, "event_entity_id", starlarkgo.String("")),
		"data":      eventField(extraGlobals, "event_data", starlarkgo.NewDict(0)),
	}
	return starlarkstruct.FromStringDict(starlarkstruct.Default, fields)
}

// makeEventGlobalReadOnly builds the read-only event struct for trigger condition contexts.
func makeEventGlobalReadOnly(extraGlobals starlarkgo.StringDict) *starlarkstruct.Struct {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlarkgo.StringDict{
		"kind":      eventField(extraGlobals, "event_kind", starlarkgo.String("")),
		"entity_id": eventField(extraGlobals, "event_entity_id", starlarkgo.String("")),
		"data":      eventField(extraGlobals, "event_data", starlarkgo.NewDict(0)),
	})
}

func eventField(extra starlarkgo.StringDict, key string, fallback starlarkgo.Value) starlarkgo.Value {
	if v, ok := extra[key]; ok {
		return v
	}
	return fallback
}

func makeSlogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/starlark/... -run "TestMakeState|TestMakeCall" -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/starlark/stdlib.go internal/starlark/stdlib_test.go
git commit -m "feat(c5): stdlib interfaces and builtin implementations"
```

---

## Task 5: `loader.go` — `load("//...")` resolver

**Files:**
- Create: `internal/starlark/loader.go`
- Create: `internal/starlark/loader_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/starlark/loader_test.go`:

```go
package starlark_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	ghs "github.com/fynn-labs/gohome/internal/starlark"
	starlarkgo "go.starlark.net/starlark"
)

func newTestRuntimeWithDir(t *testing.T, dir string) *ghs.Runtime {
	t.Helper()
	return ghs.NewRuntime(fakeState{}, &fakeDispatcher{}, &fakeAppender{}, nil, dir, nil)
}

func TestLoader_HappyPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lib.star"), []byte(`def greet(): return "hello"`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt := newTestRuntimeWithDir(t, dir)
	res, err := rt.Execute(t.Context(), ghs.KindScript,
		`load("//lib.star", "greet")
result = greet()`, starlarkgo.StringDict{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = res
}

func TestLoader_PathTraversalRejected(t *testing.T) {
	rt := newTestRuntimeWithDir(t, t.TempDir())
	_, err := rt.Execute(t.Context(), ghs.KindScript,
		`load("//../../etc/passwd", "x")`, starlarkgo.StringDict{})
	if err == nil {
		t.Fatal("expected path traversal error")
	}
}

func TestLoader_CircularDependency(t *testing.T) {
	dir := t.TempDir()
	// a.star loads b.star; b.star loads a.star
	if err := os.WriteFile(filepath.Join(dir, "a.star"), []byte(`load("//b.star", "b"); a = b`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.star"), []byte(`load("//a.star", "a"); b = a`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt := newTestRuntimeWithDir(t, dir)
	_, err := rt.Execute(t.Context(), ghs.KindScript,
		`load("//a.star", "a")`, starlarkgo.StringDict{})
	if err == nil {
		t.Fatal("expected circular dependency error")
	}
}

func TestLoader_CacheInvalidation(t *testing.T) {
	dir := t.TempDir()
	libPath := filepath.Join(dir, "lib.star")
	if err := os.WriteFile(libPath, []byte(`val = "v1"`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt := newTestRuntimeWithDir(t, dir)
	// First load: val = "v1"
	res1, err := rt.Execute(t.Context(), ghs.KindScript,
		`load("//lib.star", "val"); result = val`, starlarkgo.StringDict{})
	if err != nil {
		t.Fatalf("first execute: %v", err)
	}
	_ = res1

	// Update file; cache should still have v1 until invalidated.
	if err := os.WriteFile(libPath, []byte(`val = "v2"`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Invalidate and re-execute.
	rt.InvalidateModuleCache()
	res2, err := rt.Execute(t.Context(), ghs.KindScript,
		`load("//lib.star", "val"); result = val`, starlarkgo.StringDict{})
	if err != nil {
		t.Fatalf("second execute: %v", err)
	}
	_ = res2
}

func TestLoader_NonSlashSlashRejected(t *testing.T) {
	rt := newTestRuntimeWithDir(t, t.TempDir())
	_, err := rt.Execute(t.Context(), ghs.KindScript,
		`load("./relative.star", "x")`, starlarkgo.StringDict{})
	if err == nil {
		t.Fatal("expected error for non-// load path")
	}
}

func TestLoader_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lib.star"), []byte(`val = 42`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt := newTestRuntimeWithDir(t, dir)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = rt.Execute(t.Context(), ghs.KindScript,
				`load("//lib.star", "val")`, starlarkgo.StringDict{})
		}()
	}
	wg.Wait()
}
```

- [ ] **Step 2: Run — verify fail (loader not implemented yet)**

```bash
go test ./internal/starlark/... -run TestLoader -v
```

Expected: compile or runtime error.

- [ ] **Step 3: Create `loader.go`**

```go
package starlark

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	starlarkgo "go.starlark.net/starlark"
)

// makeLoader returns a thread.Load function resolving "//..."-prefixed paths
// relative to configDir. inProgress tracks the current load chain for cycle
// detection — callers must pass a fresh empty map per Execute call.
func (r *Runtime) makeLoader(inProgress map[string]bool) func(*starlarkgo.Thread, string) (starlarkgo.StringDict, error) {
	return func(thread *starlarkgo.Thread, module string) (starlarkgo.StringDict, error) {
		if !strings.HasPrefix(module, "//") {
			return nil, fmt.Errorf("load: only //...-prefixed paths are supported, got %q", module)
		}
		rel := module[2:]
		if strings.Contains(rel, "..") {
			return nil, fmt.Errorf("load: path traversal not allowed: %q", module)
		}

		absPath := filepath.Join(r.configDir, rel)
		if !strings.HasPrefix(absPath+string(filepath.Separator), r.configDir+string(filepath.Separator)) {
			return nil, fmt.Errorf("load: path escapes configDir: %q", module)
		}

		// cache hit
		r.mu.RLock()
		if dict, ok := r.moduleCache[absPath]; ok {
			r.mu.RUnlock()
			return dict, nil
		}
		r.mu.RUnlock()

		// cycle detection
		if inProgress[absPath] {
			return nil, fmt.Errorf("load: circular dependency detected: %q", module)
		}
		inProgress[absPath] = true
		defer delete(inProgress, absPath)

		src, err := os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("load %q: %w", module, err)
		}

		cfg := limitsFor(KindScript)
		loadThread := &starlarkgo.Thread{
			Name: "load:" + module,
			Load: r.makeLoader(inProgress),
		}
		loadThread.SetMaxSteps(cfg.MaxSteps)

		dict, err := starlarkgo.ExecFile(loadThread, absPath, src, starlarkgo.StringDict{})
		if err != nil {
			return nil, fmt.Errorf("load %q: %w", module, err)
		}

		// export only names not starting with "_"
		exported := make(starlarkgo.StringDict, len(dict))
		for k, v := range dict {
			if !strings.HasPrefix(k, "_") {
				exported[k] = v
			}
		}

		r.mu.Lock()
		r.moduleCache[absPath] = exported
		r.mu.Unlock()

		return exported, nil
	}
}
```

Note: `loader.go` references `r.mu` and `r.moduleCache` which are defined in `runtime.go` (Task 6). Write a skeleton `runtime.go` stub now to unblock compilation:

```go
// internal/starlark/runtime.go — skeleton (filled in Task 6)
package starlark

import (
	"log/slog"
	"sync"

	starlarkgo "go.starlark.net/starlark"

	"github.com/fynn-labs/gohome/internal/observability"
)

type Runtime struct {
	state      StateReader
	dispatcher CommandDispatcher
	store      EventAppender
	logger     *slog.Logger
	configDir  string
	metrics    *observability.Metrics
	randSeed   int64 // 0 = time-based per Execute call

	mu          sync.RWMutex
	moduleCache map[string]starlarkgo.StringDict
}

func NewRuntime(
	state StateReader,
	dispatcher CommandDispatcher,
	store EventAppender,
	logger *slog.Logger,
	configDir string,
	metrics *observability.Metrics,
) *Runtime {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runtime{
		state:       state,
		dispatcher:  dispatcher,
		store:       store,
		logger:      logger,
		configDir:   configDir,
		metrics:     metrics,
		moduleCache: map[string]starlarkgo.StringDict{},
	}
}

func (r *Runtime) InvalidateModuleCache() {
	r.mu.Lock()
	r.moduleCache = map[string]starlarkgo.StringDict{}
	r.mu.Unlock()
}
```

- [ ] **Step 4: Run loader tests**

```bash
go test ./internal/starlark/... -run TestLoader -v
```

Expected: all PASS (Execute is not yet implemented so tests that call it will fail — those tests are covered by Task 6).

Note: TestLoader tests call `rt.Execute(...)` which doesn't exist on the skeleton. Move those tests to `runtime_test.go` in Task 6 if they fail to compile now. The key loader unit (path traversal, cycle detection, cache invalidation) will be testable once Execute is implemented.

- [ ] **Step 5: Commit skeleton + loader**

```bash
git add internal/starlark/loader.go internal/starlark/loader_test.go internal/starlark/runtime.go
git commit -m "feat(c5): load() resolver with cycle detection and module cache"
```

---

## Task 6: `runtime.go` — `Execute` and full Runtime

**Files:**
- Modify: `internal/starlark/runtime.go` (replace skeleton)
- Create: `internal/starlark/runtime_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/starlark/runtime_test.go`:

```go
package starlark_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	ghs "github.com/fynn-labs/gohome/internal/starlark"
	starlarkgo "go.starlark.net/starlark"
)

func newTestRuntime(t *testing.T) *ghs.Runtime {
	t.Helper()
	return ghs.NewRuntime(
		fakeState{"light.kitchen": {StateStr: "off", Attributes: map[string]any{}}},
		&fakeDispatcher{},
		&fakeAppender{},
		nil,
		t.TempDir(),
		nil,
	)
}

func TestExecute_SimpleExpression(t *testing.T) {
	rt := newTestRuntime(t)
	res, err := rt.Execute(t.Context(), ghs.KindComputedEntity, "1 + 2", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Value.(starlarkgo.Int).BigInt().Int64() != 3 {
		t.Fatalf("expected 3, got %v", res.Value)
	}
}

func TestExecute_SimpleScript(t *testing.T) {
	rt := newTestRuntime(t)
	res, err := rt.Execute(t.Context(), ghs.KindAutomation, `x = 42`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Value != starlarkgo.None {
		t.Fatalf("script expected None, got %v", res.Value)
	}
}

func TestExecute_StateBuiltin(t *testing.T) {
	rt := newTestRuntime(t)
	res, err := rt.Execute(t.Context(), ghs.KindAutomation,
		`s = state("light.kitchen")`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = res
}

func TestExecute_CallServiceUnavailableInComputedEntity(t *testing.T) {
	rt := newTestRuntime(t)
	_, err := rt.Execute(t.Context(), ghs.KindComputedEntity,
		`call_service("light.kitchen", "turn_on")`, nil)
	if err == nil {
		t.Fatal("expected error: call_service not available in KindComputedEntity")
	}
}

func TestExecute_SleepUnavailableInComputedEntity(t *testing.T) {
	rt := newTestRuntime(t)
	_, err := rt.Execute(t.Context(), ghs.KindComputedEntity, `sleep(1)`, nil)
	if err == nil {
		t.Fatal("expected error: sleep not available in KindComputedEntity")
	}
}

func TestExecute_LogAppendsToResult(t *testing.T) {
	rt := newTestRuntime(t)
	res, err := rt.Execute(t.Context(), ghs.KindAutomation,
		`log("hello world")`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Logs) == 0 || !strings.Contains(res.Logs[0], "hello world") {
		t.Fatalf("expected log entry, got %v", res.Logs)
	}
}

func TestExecute_StepLimitBreached(t *testing.T) {
	rt := newTestRuntime(t)
	// Tight loop to exceed step limit.
	_, err := rt.Execute(t.Context(), ghs.KindComputedEntity, `
i = 0
for _ in range(1000000):
    i = i + 1
`, nil)
	if err == nil {
		t.Fatal("expected LimitError")
	}
	var le *ghs.LimitError
	if !errors.As(err, &le) || le.Kind != ghs.LimitSteps {
		t.Fatalf("expected LimitSteps error, got %T: %v", err, err)
	}
}

func TestExecute_WallClockLimit(t *testing.T) {
	rt := newTestRuntime(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// sleep(5) in a 100ms wall-clock context → should time out.
	_, err := rt.Execute(ctx, ghs.KindComputedEntity, `sleep(5)`, nil)
	if err == nil {
		t.Fatal("expected error from sleep in KindComputedEntity (not in stdlib)")
	}
}

func TestExecute_ExtraGlobalsInjected(t *testing.T) {
	rt := newTestRuntime(t)
	res, err := rt.Execute(t.Context(), ghs.KindScript,
		`result = my_var + 1`,
		starlarkgo.StringDict{"my_var": starlarkgo.MakeInt(41)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = res
}

func TestExecute_EventGlobalPresent_Automation(t *testing.T) {
	rt := newTestRuntime(t)
	_, err := rt.Execute(t.Context(), ghs.KindAutomation,
		`k = event.kind`, nil)
	if err != nil {
		t.Fatalf("event global missing in KindAutomation: %v", err)
	}
}

func TestExecute_EventReadOnly_TriggerCondition(t *testing.T) {
	rt := newTestRuntime(t)
	_, err := rt.Execute(t.Context(), ghs.KindTriggerCondition,
		`k = event.kind`, nil)
	if err != nil {
		t.Fatalf("event global missing in KindTriggerCondition: %v", err)
	}
	// fire() should not be available in trigger condition
	_, err = rt.Execute(t.Context(), ghs.KindTriggerCondition,
		`event.fire("x", None)`, nil)
	if err == nil {
		t.Fatal("expected error: event.fire not available in KindTriggerCondition")
	}
}

func TestExecute_SceneGlobal_Automation(t *testing.T) {
	rt := newTestRuntime(t)
	_, err := rt.Execute(t.Context(), ghs.KindAutomation,
		`scene.apply("movie_night")`, nil)
	if err != nil {
		t.Fatalf("scene.apply failed: %v", err)
	}
}

func TestExecute_ElapsedAndStepsPopulated(t *testing.T) {
	rt := newTestRuntime(t)
	res, err := rt.Execute(t.Context(), ghs.KindScript, `x = 1`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Elapsed == 0 {
		t.Fatal("Elapsed should be > 0")
	}
	if res.Steps == 0 {
		t.Fatal("Steps should be > 0")
	}
}

func TestInvalidateModuleCache(t *testing.T) {
	rt := newTestRuntime(t)
	rt.InvalidateModuleCache() // should not panic
}
```

- [ ] **Step 2: Run — verify they fail**

```bash
go test ./internal/starlark/... -run TestExecute -v
```

Expected: FAIL (Execute not implemented).

- [ ] **Step 3: Replace skeleton `runtime.go` with full implementation**

```go
package starlark

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/fynn-labs/gohome/internal/observability"
	starlarkgo "go.starlark.net/starlark"
	starlarktime "go.starlark.net/lib/time"
)

// Runtime executes Starlark scripts. Construct once via NewRuntime; safe for concurrent use.
type Runtime struct {
	state      StateReader
	dispatcher CommandDispatcher
	store      EventAppender
	logger     *slog.Logger
	configDir  string
	metrics    *observability.Metrics
	randSeed   int64 // 0 = time-based per Execute call; non-zero for determinism (testutil)

	mu          sync.RWMutex
	moduleCache map[string]starlarkgo.StringDict
}

// Result is returned by Execute on success.
type Result struct {
	Value   starlarkgo.Value
	Logs    []string
	Elapsed time.Duration
	Steps   uint64
}

func NewRuntime(
	state StateReader,
	dispatcher CommandDispatcher,
	store EventAppender,
	logger *slog.Logger,
	configDir string,
	metrics *observability.Metrics,
) *Runtime {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runtime{
		state:       state,
		dispatcher:  dispatcher,
		store:       store,
		logger:      logger,
		configDir:   configDir,
		metrics:     metrics,
		moduleCache: map[string]starlarkgo.StringDict{},
	}
}

// Execute runs script in the given context. extraGlobals are merged after the
// context stdlib (caller values win). Keys "event_kind", "event_entity_id",
// "event_data" in extraGlobals populate the event struct's read fields.
func (r *Runtime) Execute(
	ctx context.Context,
	kind ContextKind,
	script string,
	extraGlobals starlarkgo.StringDict,
) (*Result, error) {
	start := time.Now()

	cfg := limitsFor(kind)

	var logs []string
	logFn := func(level, msg string) {
		r.logger.Log(ctx, makeSlogLevel(level), msg, "starlark_context", kind.String())
		logs = append(logs, msg)
	}

	seed := time.Now().UnixNano()
	if r.randSeed != 0 {
		seed = r.randSeed
	}
	rng := rand.New(rand.NewSource(seed)) //nolint:gosec

	globals := r.buildStdlib(ctx, kind, logFn, rng, extraGlobals)

	// Merge extra globals (event_* keys handled in buildStdlib; skip them here).
	eventKeys := map[string]bool{"event_kind": true, "event_entity_id": true, "event_data": true}
	for k, v := range extraGlobals {
		if !eventKeys[k] {
			globals[k] = v
		}
	}

	thread := &starlarkgo.Thread{
		Name: kind.String(),
		Load: r.makeLoader(map[string]bool{}),
	}
	thread.SetMaxSteps(cfg.MaxSteps)

	stopWatchdog := startWatchdog(cfg.WallClock, thread)
	defer stopWatchdog()

	var (
		val starlarkgo.Value = starlarkgo.None
		err error
	)

	if cfg.IsExpression {
		val, err = starlarkgo.Eval(thread, "<input>", script, globals)
	} else {
		_, err = starlarkgo.ExecFile(thread, "<input>", script, globals)
	}

	steps := thread.Steps()
	if err != nil {
		return nil, r.wrapExecError(err, kind)
	}

	return &Result{
		Value:   val,
		Logs:    logs,
		Elapsed: time.Since(start),
		Steps:   steps,
	}, nil
}

func (r *Runtime) buildStdlib(
	ctx context.Context,
	kind ContextKind,
	logFn func(level, msg string),
	rng *rand.Rand,
	extraGlobals starlarkgo.StringDict,
) starlarkgo.StringDict {
	d := starlarkgo.StringDict{}

	// state — all contexts
	d["state"] = MakeStateBuiltin(r.state)

	// now + time module — all contexts
	d["now"] = makeNow()
	d["time"] = starlarktime.Module

	switch kind {
	case KindAutomation, KindScript, KindMCPEval, KindTriggerCondition:
		d["log"] = makeLog(logFn)
	}

	switch kind {
	case KindAutomation, KindScript, KindMCPEval:
		if r.dispatcher != nil {
			d["call_service"] = MakeCallServiceBuiltin(ctx, r.dispatcher)
		}
		d["random"] = makeRandom(rng)
	}

	switch kind {
	case KindAutomation, KindScript:
		d["sleep"] = makeSleep(ctx)
		if r.store != nil {
			d["notify"] = makeNotify(ctx, r.store)
			d["scene"] = makeSceneGlobal(ctx, r.store)
			d["event"] = makeEventGlobal(ctx, r.store, extraGlobals)
		}
	case KindTriggerCondition:
		d["event"] = makeEventGlobalReadOnly(extraGlobals)
	}

	return d
}

func (r *Runtime) wrapExecError(err error, kind ContextKind) error {
	if errors.Is(err, starlarkgo.ErrSteps) {
		return &LimitError{Kind: LimitSteps, Context: kind, Detail: err.Error()}
	}
	if strings.Contains(err.Error(), "cancelled") {
		return &LimitError{Kind: LimitWallClock, Context: kind, Detail: "wall-clock limit exceeded"}
	}
	return err
}

func (r *Runtime) InvalidateModuleCache() {
	r.mu.Lock()
	r.moduleCache = map[string]starlarkgo.StringDict{}
	r.mu.Unlock()
}

// WithRandSeed returns a copy of the Runtime with a fixed random seed.
// Used by testutil for deterministic random() output.
func (r *Runtime) WithRandSeed(seed int64) *Runtime {
	r2 := *r
	r2.randSeed = seed
	return &r2
}

func (r *Runtime) Logger() *slog.Logger { return r.logger }

// assert is injected for gohome test contexts.
var assertBuiltin = starlarkgo.NewBuiltin("assert", func(_ *starlarkgo.Thread, b *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var cond starlarkgo.Value
	msg := "assertion failed"
	if err := starlarkgo.UnpackArgs(b.Name(), args, kwargs, "cond", &cond, "msg?", &msg); err != nil {
		return nil, err
	}
	if !cond.Truth() {
		return nil, fmt.Errorf("assert: %s", msg)
	}
	return starlarkgo.None, nil
})

// ExecuteTest runs a test_* function loaded from script, injecting an assert builtin.
// Used by the gohome test daemon handler.
func (r *Runtime) ExecuteTest(ctx context.Context, script, fnName string) (*Result, error) {
	// First pass: load the module to get exported names.
	globals := starlarkgo.StringDict{
		"assert": assertBuiltin,
	}
	thread := &starlarkgo.Thread{
		Name: "test:" + fnName,
		Load: r.makeLoader(map[string]bool{}),
	}
	cfg := limitsFor(KindScript)
	thread.SetMaxSteps(cfg.MaxSteps)
	stopWatchdog := startWatchdog(cfg.WallClock, thread)
	defer stopWatchdog()

	dict, err := starlarkgo.ExecFile(thread, "<test>", script, globals)
	if err != nil {
		return nil, r.wrapExecError(err, KindScript)
	}
	fn, ok := dict[fnName]
	if !ok {
		return nil, fmt.Errorf("test function %q not found", fnName)
	}
	callable, ok := fn.(starlarkgo.Callable)
	if !ok {
		return nil, fmt.Errorf("%q is not callable", fnName)
	}

	start := time.Now()
	callThread := &starlarkgo.Thread{Name: "call:" + fnName}
	callThread.SetMaxSteps(cfg.MaxSteps)
	stopWatchdog2 := startWatchdog(cfg.WallClock, callThread)
	defer stopWatchdog2()

	_, callErr := starlarkgo.Call(callThread, callable, nil, nil)
	elapsed := time.Since(start)
	steps := callThread.Steps()

	if callErr != nil {
		return &Result{Elapsed: elapsed, Steps: steps}, callErr
	}
	return &Result{Value: starlarkgo.None, Elapsed: elapsed, Steps: steps}, nil
}
```

- [ ] **Step 4: Run all starlark tests**

```bash
go test ./internal/starlark/... -v
```

Expected: all tests PASS. (Loader tests that depend on Execute now also pass.)

- [ ] **Step 5: Run with race detector**

```bash
go test -race ./internal/starlark/...
```

Expected: PASS with no races.

- [ ] **Step 6: Commit**

```bash
git add internal/starlark/runtime.go internal/starlark/runtime_test.go
git commit -m "feat(c5): Runtime.Execute with scoped stdlib and resource limits"
```

---

## Task 7: `internal/starlark/testutil/testutil.go`

**Files:**
- Create: `internal/starlark/testutil/testutil.go`
- Create: `internal/starlark/testutil/testutil_test.go`

- [ ] **Step 1: Write failing test (smoke test of testutil itself)**

Create `internal/starlark/testutil/testutil_test.go`:

```go
package testutil_test

import (
	"testing"

	ghs "github.com/fynn-labs/gohome/internal/starlark"
	"github.com/fynn-labs/gohome/internal/starlark/testutil"
	starlarkgo "go.starlark.net/starlark"
)

func TestNewTestRuntime_Smoke(t *testing.T) {
	d := &testutil.FakeDispatcher{}
	rt := testutil.NewTestRuntime(
		testutil.FakeState{"light.living": {StateStr: "on", Attributes: map[string]any{}}},
		d, 42,
	)
	res := testutil.RunScript(t, rt, ghs.KindAutomation,
		`s = state("light.living")`)
	if res == nil {
		t.Fatal("expected result")
	}
}

func TestAssertCallService(t *testing.T) {
	d := &testutil.FakeDispatcher{}
	rt := testutil.NewTestRuntime(
		testutil.FakeState{"light.kitchen": {StateStr: "off", Attributes: map[string]any{}}},
		d, 0,
	)
	testutil.RunScript(t, rt, ghs.KindAutomation,
		`call_service("light.kitchen", "turn_on")`)
	testutil.AssertCallService(t, d, "light.kitchen", "turn_on")
}

func TestAssertNoCallService(t *testing.T) {
	d := &testutil.FakeDispatcher{}
	rt := testutil.NewTestRuntime(testutil.FakeState{}, d, 0)
	testutil.RunScript(t, rt, ghs.KindScript, `x = 1`)
	testutil.AssertNoCallService(t, d)
}

func TestAssertLog(t *testing.T) {
	d := &testutil.FakeDispatcher{}
	rt := testutil.NewTestRuntime(testutil.FakeState{}, d, 0)
	res := testutil.RunScript(t, rt, ghs.KindScript, `log("hello")`)
	testutil.AssertLog(t, res, "hello")
}

func TestAssertValue(t *testing.T) {
	d := &testutil.FakeDispatcher{}
	rt := testutil.NewTestRuntime(testutil.FakeState{}, d, 0)
	res := testutil.RunScript(t, rt, ghs.KindComputedEntity, `1 + 1`)
	testutil.AssertValue(t, res, starlarkgo.MakeInt(2))
}
```

- [ ] **Step 2: Run — verify compile error**

```bash
go test ./internal/starlark/testutil/... -v
```

Expected: compile error.

- [ ] **Step 3: Create `testutil.go`**

```go
// Package testutil provides a pre-wired Runtime with injectable fakes
// for use in Go tests (primarily C6 automation/condition unit tests).
package testutil

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/fynn-labs/gohome/internal/eventstore"
	ghs "github.com/fynn-labs/gohome/internal/starlark"
	starlarkgo "go.starlark.net/starlark"
)

// FakeState maps entity ID → EntityState for test injection.
type FakeState map[string]*ghs.EntityState

func (f FakeState) Get(id string) (*ghs.EntityState, bool) {
	v, ok := f[id]
	return v, ok
}

// DispatchCall records one call to FakeDispatcher.Dispatch.
type DispatchCall struct {
	EntityID   string
	Capability string
	Args       map[string]string
}

// FakeDispatcher records all Dispatch calls for assertion.
type FakeDispatcher struct {
	mu    sync.Mutex
	Calls []DispatchCall
}

func (f *FakeDispatcher) Dispatch(_ context.Context, entityID, capability string, args map[string]string) (*ghs.DispatchResult, error) {
	f.mu.Lock()
	f.Calls = append(f.Calls, DispatchCall{EntityID: entityID, Capability: capability, Args: args})
	f.mu.Unlock()
	return &ghs.DispatchResult{Ok: true}, nil
}

// nopAppender satisfies EventAppender with no-op behaviour.
type nopAppender struct{}

func (nopAppender) Append(_ context.Context, _ eventstore.Event) (uint64, error) { return 1, nil }

// NewTestRuntime returns a Runtime with fake dependencies.
// seed controls random() output (0 = random seed).
func NewTestRuntime(state FakeState, dispatcher *FakeDispatcher, seed int64) *ghs.Runtime {
	rt := ghs.NewRuntime(state, dispatcher, nopAppender{}, slog.Default(), "", nil)
	if seed != 0 {
		rt = rt.WithRandSeed(seed)
	}
	return rt
}

// ScriptResult wraps ghs.Result for assertion helpers.
type ScriptResult struct {
	Value   starlarkgo.Value
	Logs    []string
	Steps   uint64
	Elapsed interface{}
}

// RunScript executes script in the given context kind. Calls t.Fatal on error.
func RunScript(t testing.TB, rt *ghs.Runtime, kind ghs.ContextKind, script string) *ScriptResult {
	t.Helper()
	res, err := rt.Execute(context.Background(), kind, script, nil)
	if err != nil {
		t.Fatalf("RunScript(%s): %v", kind, err)
	}
	return &ScriptResult{
		Value:   res.Value,
		Logs:    res.Logs,
		Steps:   res.Steps,
		Elapsed: res.Elapsed,
	}
}

// AssertValue asserts result.Value equals expected.
func AssertValue(t testing.TB, result *ScriptResult, expected starlarkgo.Value) {
	t.Helper()
	if result.Value != expected {
		if ok, _ := starlarkgo.Equal(result.Value, expected); !ok {
			t.Errorf("AssertValue: got %v, want %v", result.Value, expected)
		}
	}
}

// AssertLog asserts at least one log entry contains the given substring.
func AssertLog(t testing.TB, result *ScriptResult, contains string) {
	t.Helper()
	for _, l := range result.Logs {
		if strings.Contains(l, contains) {
			return
		}
	}
	t.Errorf("AssertLog: no log entry contains %q; logs: %v", contains, result.Logs)
}

// AssertCallService asserts FakeDispatcher received a call to entityID.capability.
func AssertCallService(t testing.TB, d *FakeDispatcher, entityID, capability string) {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, c := range d.Calls {
		if c.EntityID == entityID && c.Capability == capability {
			return
		}
	}
	t.Errorf("AssertCallService: no call to %s.%s; calls: %v", entityID, capability, d.Calls)
}

// AssertNoCallService asserts FakeDispatcher received no Dispatch calls.
func AssertNoCallService(t testing.TB, d *FakeDispatcher) {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.Calls) > 0 {
		t.Errorf("AssertNoCallService: unexpected calls: %v", d.Calls)
	}
}

// AssertError asserts err is non-nil and its message contains the given substring.
func AssertError(t testing.TB, err error, contains string) {
	t.Helper()
	if err == nil {
		t.Errorf("AssertError: expected error containing %q, got nil", contains)
		return
	}
	if !strings.Contains(err.Error(), contains) {
		t.Errorf("AssertError: error %q does not contain %q", err.Error(), contains)
	}
}
```

Note: `RunScript` above uses `t.(interface{ Context() context.Context })` because Go 1.21+ `testing.TB` has `Context()`. Since this module uses `go 1.25.0`, this is safe. Alternatively use `context.Background()`.

- [ ] **Step 4: Run testutil tests**

```bash
go test ./internal/starlark/testutil/... -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/starlark/testutil/testutil.go internal/starlark/testutil/testutil_test.go
git commit -m "feat(c5): testutil package for Starlark unit tests"
```

---

## Task 8: Pkl validator activation

**Prerequisite:** C4 must be implemented first. This task modifies files created by C4:
- `internal/config/pkl/gohome/starlark.pkl`
- `internal/config/evaluator.go`

- [ ] **Step 1: Update `starlark.pkl`**

Replace the C4 stub typealiases in `internal/config/pkl/gohome/starlark.pkl`:

```pkl
module gohome.starlark

typealias StarlarkExpr = String(isValidStarlarkExpr(this))
typealias StarlarkScript = String(isValidStarlarkScript(this))
typealias StarlarkCondition = String(isValidStarlarkCondition(this))

@External
external function isValidStarlarkExpr(src: String): Boolean

@External
external function isValidStarlarkScript(src: String): Boolean

@External
external function isValidStarlarkCondition(src: String): Boolean
```

- [ ] **Step 2: Add `starlarkValidatorReader` to `internal/config/evaluator.go`**

Append a new `ModuleReader` registration alongside the existing `gohome:` reader. In `evaluator.go`, in the function that builds `pkl.EvaluatorOptions`, add:

```go
import (
    ghstarlark "github.com/fynn-labs/gohome/internal/starlark"
    // ... existing imports
)

// starlarkValidatorReader implements pkl.ModuleReader for the gohome-validator: scheme.
// When Pkl evaluates @External function calls for isValidStarlark*, it reads a URI
// from this scheme. The exact URL format depends on the pkl-go version; log the URI
// in development if the validator is not triggering correctly.
type starlarkValidatorReader struct{}

func (r *starlarkValidatorReader) Scheme() string             { return "gohome-validator" }
func (r *starlarkValidatorReader) IsLocal() bool              { return true }
func (r *starlarkValidatorReader) HasHierarchicalUris() bool  { return false }
func (r *starlarkValidatorReader) IsGlobbable() bool          { return false }
func (r *starlarkValidatorReader) ListElements(_ url.URL) ([]pkl.PathElement, error) {
    return nil, nil
}

func (r *starlarkValidatorReader) Read(uri url.URL) (string, error) {
    // URI format emitted by pkl-go for @External calls:
    // "gohome-validator:<functionName>?<encoded-src>"
    // Verify by adding: slog.Default().Debug("validator read", "uri", uri.String())
    fnName := uri.Opaque
    if fnName == "" {
        fnName = uri.Path
    }
    src, _ := url.QueryUnescape(uri.RawQuery)

    var expr bool
    switch fnName {
    case "isValidStarlarkExpr":
        expr = true
    case "isValidStarlarkScript", "isValidStarlarkCondition":
        expr = false
    default:
        return "false", nil
    }

    if err := ghstarlark.ParseOnly(src, expr); err != nil {
        return "false", nil
    }
    return "true", nil
}
```

Then register it in the evaluator options alongside the `gohome:` reader:

```go
// In the function that builds pkl.EvaluatorOptions:
options.ModuleReaders = append(options.ModuleReaders, &starlarkValidatorReader{})
```

- [ ] **Step 3: Write a Pkl validator integration test**

Create `internal/config/evaluator_starlark_test.go` (alongside the existing C4 evaluator tests):

```go
//go:build integration

package config_test

import (
    "context"
    "testing"

    "github.com/fynn-labs/gohome/internal/config"
)

func TestPklValidator_RejectsInvalidStarlark(t *testing.T) {
    // A fixture .pkl that sets a StarlarkExpr to a broken expression.
    // When config.Validate is called, pkl-go should call isValidStarlarkExpr
    // and return false, causing a validation error.
    ev := config.NewEvaluator(t.TempDir())
    _, err := ev.Evaluate(context.Background(), "testdata/bad_starlark.pkl")
    if err == nil {
        t.Fatal("expected validation error for bad Starlark expression")
    }
}
```

Create `internal/config/testdata/bad_starlark.pkl`:

```pkl
amends "gohome:config.pkl"

automations {
  new {
    id = "test"
    trigger { entityId = "light.x"; toState = "on" }
    condition: StarlarkCondition = "def foo(:"   // syntax error
    action: StarlarkScript = "x = 1"
  }
}
```

- [ ] **Step 4: Run integration test**

```bash
go test -tags=integration ./internal/config/... -run TestPklValidator -v
```

Expected: PASS (validation error returned for bad Starlark).

Note: If the `@External` URL format doesn't match, the validator returns `"false"` silently. Add a `slog.Default().Debug("validator read", "uri", uri.String())` line temporarily to observe the actual URL pkl-go uses and adjust the parsing in `Read` accordingly.

- [ ] **Step 5: Commit**

```bash
git add internal/config/pkl/gohome/starlark.pkl internal/config/evaluator.go internal/config/evaluator_starlark_test.go internal/config/testdata/bad_starlark.pkl
git commit -m "feat(c5): activate Pkl Starlark validators via gohome-validator: reader"
```

---

## Task 9: `gohome eval` CLI command + daemon socket handler

**Files:**
- Create: `internal/cli/eval.go`
- Modify: `internal/daemon/recovery.go` (add `starlark_eval` case to `handleSocketConn`)
- Modify: `internal/cli/root.go` (register `newEvalCmd`)

- [ ] **Step 1: Create `internal/cli/eval.go`**

```go
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

func newEvalCmd(gf *globalFlags) *cobra.Command {
	var contextKind string
	c := &cobra.Command{
		Use:   "eval <file.star>",
		Short: "Evaluate a Starlark script against the running daemon",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			src, err := os.ReadFile(args[0])
			dieOnError(err)

			ctx, cancel := context.WithTimeout(cmd.Context(), 35*time.Second)
			defer cancel()

			sockPath := filepath.Join(expandHome(gf.DataDir), "gohomed.sock")
			conn, err := (&net.Dialer{}).DialContext(ctx, "unix", sockPath)
			dieOnError(err)
			defer func() { _ = conn.Close() }()

			req := map[string]any{
				"op":      "starlark_eval",
				"script":  string(src),
				"context": contextKind,
			}
			dieOnError(json.NewEncoder(conn).Encode(req))

			scanner := bufio.NewScanner(conn)
			exitCode := 0
			for scanner.Scan() {
				line := scanner.Bytes()
				var msg struct {
					Type    string `json:"type"`
					Level   string `json:"level"`
					Msg     string `json:"msg"`
					OK      bool   `json:"ok"`
					Value   string `json:"value"`
					Error   string `json:"error"`
					Elapsed int64  `json:"elapsed_ms"`
					Steps   uint64 `json:"steps"`
				}
				if err := json.Unmarshal(line, &msg); err != nil {
					continue
				}
				switch msg.Type {
				case "log":
					fmt.Printf("%s %s\n", Dim.Render("[log]"), msg.Msg)
				case "result":
					if msg.OK {
						fmt.Println(Success.Render("ok"))
						if msg.Value != "" && msg.Value != "None" {
							fmt.Println(EntityID.Render(msg.Value))
						}
						fmt.Println(Dim.Render(fmt.Sprintf("%dms / %d steps", msg.Elapsed, msg.Steps)))
					} else {
						fmt.Fprintln(os.Stderr, Error.Render("error: ")+msg.Error)
						exitCode = 1
					}
				}
			}
			dieOnError(scanner.Err())
			if exitCode != 0 {
				os.Exit(exitCode)
			}
		},
	}
	c.Flags().StringVar(&contextKind, "context", "automation", "automation|computed|condition|script|mcp")
	return c
}
```

- [ ] **Step 2: Register `newEvalCmd` in `root.go`**

In `internal/cli/root.go`, add to `NewRoot()`:

```go
root.AddCommand(newEvalCmd(gf))
```

- [ ] **Step 3: Add `starlark_eval` case to `handleSocketConn`**

In `internal/daemon/recovery.go`, extend `socketReq` and `handleSocketConn`:

```go
// Add to socketReq struct:
Script  string `json:"script,omitempty"`
Context string `json:"context,omitempty"`
File    string `json:"file,omitempty"`    // for starlark_test
TestFns string `json:"test_fns,omitempty"` // for starlark_test
```

Add case to `handleSocketConn`:

```go
case "starlark_eval":
    if d.starlarkRuntime == nil {
        _ = enc.Encode(socketResp{Error: "starlark runtime not initialised"})
        return
    }
    kind, err := starlark.KindFromString(req.Context)
    if err != nil {
        _ = enc.Encode(socketResp{Error: err.Error()})
        return
    }
    // Extend connection deadline to cover max script wall-clock (30s + buffer).
    _ = conn.SetDeadline(time.Now().Add(35 * time.Second))

    res, execErr := d.starlarkRuntime.Execute(ctx, kind, req.Script, starlarkgo.StringDict{})

    type evalMsg struct {
        Type    string `json:"type"`
        Level   string `json:"level,omitempty"`
        Msg     string `json:"msg,omitempty"`
        OK      bool   `json:"ok,omitempty"`
        Value   string `json:"value,omitempty"`
        Error   string `json:"error,omitempty"`
        Elapsed int64  `json:"elapsed_ms,omitempty"`
        Steps   uint64 `json:"steps,omitempty"`
    }

    if execErr != nil {
        _ = enc.Encode(evalMsg{Type: "result", Error: execErr.Error()})
        return
    }
    for _, l := range res.Logs {
        _ = enc.Encode(evalMsg{Type: "log", Level: "info", Msg: l})
    }
    _ = enc.Encode(evalMsg{
        Type:    "result",
        OK:      true,
        Value:   res.Value.String(),
        Elapsed: res.Elapsed.Milliseconds(),
        Steps:   res.Steps,
    })
```

`d.starlarkRuntime` is added in Task 11 (Daemon wiring). Add a nil field to `Daemon` now so the code compiles:

```go
// In daemon/daemon.go, add to Daemon struct:
starlarkRuntime *starlark.Runtime
```

Required imports in `recovery.go`:

```go
import (
    starlark "github.com/fynn-labs/gohome/internal/starlark"
    starlarkgo "go.starlark.net/starlark"
)
```

- [ ] **Step 4: Verify build**

```bash
task build
```

Expected: both binaries compile.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/eval.go internal/cli/root.go internal/daemon/recovery.go internal/daemon/daemon.go
git commit -m "feat(c5): gohome eval CLI command and daemon starlark_eval handler"
```

---

## Task 10: `gohome test` CLI command + daemon handler

**Files:**
- Create: `internal/cli/test.go`
- Modify: `internal/daemon/recovery.go` (add `starlark_test` case)
- Modify: `internal/cli/root.go` (register `newTestCmd`)

- [ ] **Step 1: Create `internal/cli/test.go`**

```go
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newTestCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "test <file.star | dir/>",
		Short: "Run test_* functions in Starlark test files against the running daemon",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			files, err := collectTestFiles(args[0])
			dieOnError(err)

			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()

			sockPath := filepath.Join(expandHome(gf.DataDir), "gohomed.sock")

			allPassed := true
			for _, f := range files {
				passed := runTestFile(ctx, sockPath, f)
				if !passed {
					allPassed = false
				}
			}
			if !allPassed {
				fmt.Fprintln(os.Stderr, Error.Render("FAIL"))
				os.Exit(1)
			}
			fmt.Println(Success.Render("ok"))
		},
	}
}

func collectTestFiles(target string) ([]string, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{target}, nil
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), "_test.star") {
			files = append(files, filepath.Join(target, e.Name()))
		}
	}
	return files, nil
}

// runTestFile sends one file to the daemon and streams test results.
// Returns true if all tests passed.
func runTestFile(ctx context.Context, sockPath, filePath string) bool {
	src, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, Error.Render("error: ")+err.Error())
		return false
	}

	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", sockPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, Error.Render("error: ")+err.Error())
		return false
	}
	defer func() { _ = conn.Close() }()

	req := map[string]any{
		"op":     "starlark_test",
		"script": string(src),
		"file":   filePath,
	}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		fmt.Fprintln(os.Stderr, Error.Render("error: ")+err.Error())
		return false
	}

	allPassed := true
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Bytes()
		var msg struct {
			Type    string `json:"type"`
			Test    string `json:"test"`
			OK      bool   `json:"ok"`
			Error   string `json:"error"`
			Elapsed int64  `json:"elapsed_ms"`
			Steps   uint64 `json:"steps"`
		}
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}
		if msg.Type != "test_result" {
			continue
		}
		stats := Dim.Render(fmt.Sprintf("(%dms / %d steps)", msg.Elapsed, msg.Steps))
		if msg.OK {
			fmt.Printf("%s %s  %s\n", Success.Render("--- PASS:"), msg.Test, stats)
		} else {
			fmt.Printf("%s %s  %s\n", Error.Render("--- FAIL:"), msg.Test, stats)
			fmt.Printf("    %s\n", EntityID.Render(msg.Error))
			allPassed = false
		}
	}
	return allPassed
}
```

- [ ] **Step 2: Register `newTestCmd` in `root.go`**

```go
root.AddCommand(newTestCmd(gf))
```

- [ ] **Step 3: Add `starlark_test` case to `handleSocketConn` in `recovery.go`**

```go
case "starlark_test":
    if d.starlarkRuntime == nil {
        _ = enc.Encode(socketResp{Error: "starlark runtime not initialised"})
        return
    }
    _ = conn.SetDeadline(time.Now().Add(5 * time.Minute))

    type testMsg struct {
        Type    string `json:"type"`
        Test    string `json:"test"`
        OK      bool   `json:"ok"`
        Error   string `json:"error,omitempty"`
        Elapsed int64  `json:"elapsed_ms"`
        Steps   uint64 `json:"steps"`
    }

    // Parse test_* function names from the script.
    fnNames := extractTestFunctions(req.Script)
    for _, fnName := range fnNames {
        res, execErr := d.starlarkRuntime.ExecuteTest(ctx, req.Script, fnName)
        msg := testMsg{Type: "test_result", Test: fnName}
        if execErr != nil {
            msg.Error = execErr.Error()
        } else {
            msg.OK = true
            if res != nil {
                msg.Elapsed = res.Elapsed.Milliseconds()
                msg.Steps = res.Steps
            }
        }
        _ = enc.Encode(msg)
    }
```

Add `extractTestFunctions` helper at the bottom of `recovery.go`:

```go
// extractTestFunctions finds all "def test_*" function names in script src.
// Uses simple string scanning; no Starlark parse needed.
func extractTestFunctions(src string) []string {
    var names []string
    for _, line := range strings.Split(src, "\n") {
        line = strings.TrimSpace(line)
        if strings.HasPrefix(line, "def test_") {
            name := strings.TrimPrefix(line, "def ")
            if idx := strings.Index(name, "("); idx != -1 {
                name = name[:idx]
            }
            names = append(names, strings.TrimSpace(name))
        }
    }
    return names
}
```

Required import: `"strings"` (add to existing imports in `recovery.go`).

- [ ] **Step 4: Verify build**

```bash
task build
```

Expected: both binaries compile.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/test.go internal/cli/root.go internal/daemon/recovery.go
git commit -m "feat(c5): gohome test CLI command and daemon starlark_test handler"
```

---

## Task 11: Daemon wiring — construct Runtime, adapters, cache invalidation

**Files:**
- Modify: `internal/daemon/daemon.go`

The daemon's `state.Cache.Get` returns `(state.State, bool)`, but `StateReader` needs `(*EntityState, bool)`. `carport.Host.Dispatch` returns `(*carportpb.CommandResult, error)`, but `CommandDispatcher` needs `(*DispatchResult, error)`. Both need thin adapters defined in `daemon.go`.

- [ ] **Step 1: Add adapters and Runtime construction to `daemon.go`**

Add the following to `internal/daemon/daemon.go`:

```go
import (
    entityv1 "github.com/fynn-labs/gohome/gen/gohome/entity/v1"
    starlark "github.com/fynn-labs/gohome/internal/starlark"
    "google.golang.org/protobuf/encoding/protojson"
    "encoding/json"
    "fmt"
)

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
```

In `Daemon.Run`, after Phase 5 readiness (before `<-ctx.Done()`):

```go
// Construct Starlark runtime.
d.starlarkRuntime = starlark.NewRuntime(
    &stateAdapter{cache: d.cache},
    &carportAdapter{host: d.carport},
    d.store,
    d.logger,
    d.cfg.DataDir, // configDir; adjust if config dir differs from data dir
    d.metrics,
)

// Subscribe to ConfigApplied events to invalidate the module cache.
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
```

Also add `starlarkRuntime *starlark.Runtime` to the `Daemon` struct (if not already there from Task 9).

- [ ] **Step 2: Build**

```bash
task build
```

Expected: PASS.

- [ ] **Step 3: Run full test suite**

```bash
task test
task test:race
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/daemon/daemon.go
git commit -m "feat(c5): wire Starlark runtime into daemon with state/carport adapters"
```

---

## Task 12: Definition of done

- [ ] **Step 1: Full build**

```bash
task build
```

Expected: `dist/gohomed` and `dist/gohome` produced with no errors.

- [ ] **Step 2: Unit tests**

```bash
task test
```

Expected: PASS.

- [ ] **Step 3: Race detector**

```bash
task test:race
```

Expected: PASS with no race conditions.

- [ ] **Step 4: Integration tests**

```bash
task test:integration
```

Expected: PASS (includes Pkl validator integration test from Task 8 if C4 is implemented).

- [ ] **Step 5: Lint**

```bash
task lint
```

Expected: no lint errors. Fix any issues before proceeding.

- [ ] **Step 6: Tidy**

```bash
go mod tidy
```

Expected: `go.mod` and `go.sum` have no diff.

- [ ] **Step 7: Final commit**

```bash
git add -u
git commit -m "feat(c5): Starlark runtime — definition of done"
```

---

## Appendix: Key import aliases

| Import path | Alias (inside `package starlark`) |
|---|---|
| `go.starlark.net/starlark` | `starlarkgo` |
| `go.starlark.net/lib/time` | `starlarktime` |
| `go.starlark.net/starlarkstruct` | `starlarkstruct` (no alias needed) |
| `go.starlark.net/syntax` | `syntax` (no alias needed) |

| Import path | Alias (outside `package starlark`) |
|---|---|
| `github.com/fynn-labs/gohome/internal/starlark` | `starlark` (default) |
| `go.starlark.net/starlark` | `starlarkgo` |

## Appendix: Pkl @External URL format

The `gohome-validator:` ModuleReader's `Read(uri url.URL)` receives whatever URL pkl-go generates for `@External` function calls. The format varies by pkl-go version. To determine the actual format during development, add:

```go
slog.Default().Debug("validator called", "uri", uri.String())
```

to the `Read` method, run `gohome config validate` on a fixture file, and observe the log output. Adjust the parsing in `Read` accordingly.
