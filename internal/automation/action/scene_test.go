package action_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/fdatoo/switchyard/internal/automation/action"
	"github.com/fdatoo/switchyard/internal/eventstore"
)

type fakeSceneApplier struct {
	applied []string
	err     error
}

func (f *fakeSceneApplier) Apply(_ context.Context, slug, _ string) error {
	f.applied = append(f.applied, slug)
	return f.err
}

func TestScene_Calls(t *testing.T) {
	f := &fakeSceneApplier{}
	a := &action.SceneAction{Slug: "movie"}
	if err := a.Execute(context.Background(), &action.Run{Scenes: f}); err != nil {
		t.Fatal(err)
	}
	if len(f.applied) != 1 || f.applied[0] != "movie" {
		t.Fatalf("got %v", f.applied)
	}
}

type recordingEventAppender struct {
	events []eventstore.Event
}

func (r *recordingEventAppender) Append(_ context.Context, e eventstore.Event) (uint64, error) {
	r.events = append(r.events, e)
	return uint64(len(r.events)), nil
}

func TestStubSceneApplier_WarnsAndEmitsSceneApplied(t *testing.T) {
	store := &recordingEventAppender{}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	applier := &action.StubSceneApplier{Store: store, Logger: logger}

	if err := applier.Apply(context.Background(), "movie", "corr-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(logs.Bytes(), []byte("scene engine not yet implemented")) {
		t.Fatalf("expected warning log, got %q", logs.String())
	}
	if len(store.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(store.events))
	}
	ev := store.events[0]
	if ev.Kind != "scene_applied" || ev.Source != "scene_stub" {
		t.Fatalf("unexpected event metadata: kind=%q source=%q", ev.Kind, ev.Source)
	}
	sys := ev.Payload.GetSystem()
	if sys == nil {
		t.Fatal("expected SystemEvent payload")
	}
	if sys.Kind != "scene_applied" {
		t.Fatalf("expected system kind scene_applied, got %q", sys.Kind)
	}
	if sys.Data["slug"] != "movie" || sys.Data["correlation_id"] != "corr-1" {
		t.Fatalf("unexpected scene data: %v", sys.Data)
	}
}
