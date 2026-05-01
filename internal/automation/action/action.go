// Package action owns automation-engine action executors. Every ActionConfig
// variant compiles to one Executor; SequenceBlock/ParallelBlock nest. Run
// threads per-invocation context through every nested Executor.
package action

import (
	"context"
	"log/slog"
	"sync"

	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/observability"
	ghstarlark "github.com/fdatoo/switchyard/internal/starlark"
)

type Run struct {
	CorrelationID string
	AutomationID  string

	State        StateReader
	Dispatcher   CommandDispatcher
	Store        EventAppender
	Scenes       SceneApplier
	Scripts      ScriptCaller
	Runtime      *ghstarlark.Runtime
	Logger       *slog.Logger
	Metrics      *observability.Metrics
	TriggerEvent *eventstore.Event

	mu    sync.Mutex
	Steps uint64
	Logs  []string
}

func (r *Run) AddLog(s string)   { r.mu.Lock(); r.Logs = append(r.Logs, s); r.mu.Unlock() }
func (r *Run) AddSteps(n uint64) { r.mu.Lock(); r.Steps += n; r.mu.Unlock() }
func (r *Run) Snapshot() (uint64, []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.Logs))
	copy(out, r.Logs)
	return r.Steps, out
}

type Executor interface {
	Execute(ctx context.Context, run *Run) error
}

// ChildCtrl holds per-child flags from ActionConfig; attached alongside the
// Executor slice so we don't need to repeat fields on every concrete type.
type ChildCtrl struct {
	ContinueOnError bool
}

type StateReader interface {
	Get(entityID string) (*ghstarlark.EntityState, bool)
}

type CommandDispatcher interface {
	Dispatch(ctx context.Context, entityID, capability string, args map[string]string) (*ghstarlark.DispatchResult, error)
}

type EventAppender interface {
	Append(ctx context.Context, e eventstore.Event) (uint64, error)
}

// SceneApplier is injected by the daemon. v1 wires StubSceneApplier (below).
type SceneApplier interface {
	Apply(ctx context.Context, slug, correlationID string) error
}

type ScriptCaller interface {
	Call(ctx context.Context, name string, args map[string]string, invokedBy, sharedCorrID string) (ScriptCallResult, error)
}

type ScriptCallResult interface {
	Succeeded() bool
	GetError() string
	GetSteps() uint64
	GetLogs() []string
}
