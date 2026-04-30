package action

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/eventstore"
)

type SceneAction struct{ Slug string }

func (a *SceneAction) Execute(ctx context.Context, run *Run) error {
	if run.Scenes == nil {
		return fmt.Errorf("scene: no applier")
	}
	return run.Scenes.Apply(ctx, a.Slug, run.CorrelationID)
}

// StubSceneApplier emits SceneApplied and warn-logs. v1 daemon wires this;
// a real scene engine replaces it in a later spec without changing C6.
type StubSceneApplier struct {
	Store  EventAppender
	Logger *slog.Logger
}

func (s *StubSceneApplier) Apply(ctx context.Context, slug, corrID string) error {
	if s.Logger != nil {
		s.Logger.Warn("scene engine not yet implemented", "slug", slug, "correlation_id", corrID)
	}
	if s.Store == nil {
		return nil
	}
	_, err := s.Store.Append(ctx, eventstore.Event{
		Kind:      "scene_applied",
		Source:    "scene_stub",
		Timestamp: time.Now(),
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_System{
			System: &eventv1.SystemEvent{Kind: "scene_applied", Data: map[string]string{
				"slug":           slug,
				"correlation_id": corrID,
			}},
		}},
	})
	return err
}
