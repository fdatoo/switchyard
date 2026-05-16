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

// NewApplier wires scene execution to config snapshots, command dispatch, events, and scripts.
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
// SceneAction continues to work; an automation invoking a scene goes
// through here just like a direct RPC).
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
			a.appendEvent(ctx, scene, correlationID, invokedBy, 0, []string{err.Error()}, eventv1.RunOutcome_OUTCOME_ACTION_ERROR)
			return err
		}
		execs = append(execs, ex)
		ctrls = append(ctrls, action.ChildCtrl{ContinueOnError: true})
	}

	parallel := &action.ParallelBlock{Children: execs, ChildCtrl: ctrls}

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
	err := parallel.Execute(ctx, run)
	steps, logs := run.Snapshot()
	outcome := eventv1.RunOutcome_OUTCOME_OK
	if err != nil {
		outcome = eventv1.RunOutcome_OUTCOME_ACTION_ERROR
	}
	a.appendEvent(ctx, scene, correlationID, invokedBy, steps, logs, outcome)
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
