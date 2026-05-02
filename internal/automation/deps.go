package automation

import (
	"context"
	"log/slog"

	"github.com/fdatoo/switchyard/internal/automation/action"
	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/observability"
)

type Deps struct {
	State      action.StateReader
	Dispatcher action.CommandDispatcher
	Store      EventStore
	Scenes     action.SceneApplier
	Logger     *slog.Logger
	Metrics    *observability.Metrics
}

type EventStore interface {
	Append(ctx context.Context, e eventstore.Event) (uint64, error)
	Subscribe(ctx context.Context, opts eventstore.SubscribeOptions) (eventstore.Subscription, error)
	LatestPosition() uint64
}
