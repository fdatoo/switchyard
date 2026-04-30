package starlark_test

import (
	"context"
	"testing"

	starlarkgo "go.starlark.net/starlark"

	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/eventstore"
	ghs "github.com/fdatoo/gohome/internal/starlark"
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

// fakeAppender implements EventAppender for tests, recording appended events.
type fakeAppender struct {
	events []eventstore.Event
}

func (f *fakeAppender) Append(_ context.Context, e eventstore.Event) (uint64, error) {
	f.events = append(f.events, e)
	return uint64(len(f.events)), nil
}

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

func TestNotify_PreservesPayload(t *testing.T) {
	store := &fakeAppender{}
	rt := ghs.NewRuntime(fakeState{}, &fakeDispatcher{}, store, nil, t.TempDir(), nil)
	_, err := rt.Execute(context.Background(), ghs.KindScript, `notify(target="user:alice", message="hello world")`, starlarkgo.StringDict{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(store.events))
	}
	ev := store.events[0]
	if ev.Entity != "user:alice" {
		t.Errorf("expected entity=user:alice, got %q", ev.Entity)
	}
	sys := ev.Payload.GetSystem()
	if sys == nil {
		t.Fatal("expected SystemEvent payload")
	}
	if sys.Kind != "notification.sent" {
		t.Errorf("expected kind=notification.sent, got %q", sys.Kind)
	}
	if sys.Data["target"] != "user:alice" || sys.Data["message"] != "hello world" {
		t.Errorf("unexpected data: %v", sys.Data)
	}
}

func TestSceneApply_PreservesPayload(t *testing.T) {
	store := &fakeAppender{}
	rt := ghs.NewRuntime(fakeState{}, &fakeDispatcher{}, store, nil, t.TempDir(), nil)
	_, err := rt.Execute(context.Background(), ghs.KindScript, `scene.apply(slug="night-mode")`, starlarkgo.StringDict{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(store.events))
	}
	ev := store.events[0]
	if ev.Entity != "night-mode" {
		t.Errorf("expected entity=night-mode, got %q", ev.Entity)
	}
	sys := ev.Payload.GetSystem()
	if sys == nil {
		t.Fatal("expected SystemEvent payload")
	}
	if sys.Data["slug"] != "night-mode" {
		t.Errorf("unexpected data: %v", sys.Data)
	}
}

func TestEventFire_PreservesPayload(t *testing.T) {
	store := &fakeAppender{}
	rt := ghs.NewRuntime(fakeState{}, &fakeDispatcher{}, store, nil, t.TempDir(), nil)
	script := `event.fire(kind="lights.toggled", data={"room": "kitchen", "on": "true"})`
	_, err := rt.Execute(context.Background(), ghs.KindScript, script, starlarkgo.StringDict{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(store.events))
	}
	ev := store.events[0]
	if ev.Kind != "lights.toggled" {
		t.Errorf("expected kind=lights.toggled, got %q", ev.Kind)
	}
	sys := ev.Payload.GetSystem()
	if sys == nil {
		t.Fatal("expected SystemEvent payload")
	}
	if sys.Data["room"] != "kitchen" {
		t.Errorf("expected room=kitchen in data, got %v", sys.Data)
	}
}

func TestEventFire_NoneData(t *testing.T) {
	store := &fakeAppender{}
	rt := ghs.NewRuntime(fakeState{}, &fakeDispatcher{}, store, nil, t.TempDir(), nil)
	_, err := rt.Execute(context.Background(), ghs.KindScript, `event.fire(kind="ping")`, starlarkgo.StringDict{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sys := store.events[0].Payload.GetSystem()
	if sys == nil {
		t.Fatal("expected SystemEvent payload")
	}
	if len(sys.Data) != 0 {
		t.Errorf("expected empty data, got %v", sys.Data)
	}
}

// Compile-time check that eventv1 is used (prevents import pruning).
var _ *eventv1.Payload
