package starlark

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"sync"
	"time"

	starlarktime "go.starlark.net/lib/time"
	starlarkgo "go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/fdatoo/switchyard/internal/observability"
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
	thread.SetMaxExecutionSteps(cfg.MaxSteps)

	stop, timedOut := startWatchdog(ctx, cfg.WallClock, thread)
	defer stop()

	var (
		val starlarkgo.Value = starlarkgo.None
		err error
	)

	if cfg.IsExpression {
		val, err = starlarkgo.EvalOptions(&syntax.FileOptions{}, thread, "<input>", script, globals)
	} else {
		_, err = starlarkgo.ExecFileOptions(
			&syntax.FileOptions{TopLevelControl: true, GlobalReassign: true},
			thread, "<input>", script, globals,
		)
	}

	steps := thread.Steps
	if err != nil {
		return nil, r.wrapExecError(err, kind, timedOut.Load())
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

func (r *Runtime) wrapExecError(err error, kind ContextKind, timedOut bool) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	// The go.starlark.net version in use does not export a sentinel
	// (ErrSteps was removed); the thread cancel message is the only signal.
	if strings.Contains(msg, "too many steps") {
		return &LimitError{Kind: LimitSteps, Context: kind, Detail: msg}
	}
	if timedOut {
		return &LimitError{Kind: LimitWallClock, Context: kind, Detail: "wall-clock limit exceeded"}
	}
	// Caller-cancelled or other — propagate.
	return err
}

func (r *Runtime) InvalidateModuleCache() {
	r.mu.Lock()
	r.moduleCache = map[string]starlarkgo.StringDict{}
	r.mu.Unlock()
}

// WithRandSeed returns a new Runtime sharing the same dependencies but with a
// fixed random seed. Used by testutil for deterministic random() output.
func (r *Runtime) WithRandSeed(seed int64) *Runtime {
	r.mu.RLock()
	cache := r.moduleCache
	r.mu.RUnlock()
	return &Runtime{
		state:       r.state,
		dispatcher:  r.dispatcher,
		store:       r.store,
		logger:      r.logger,
		configDir:   r.configDir,
		metrics:     r.metrics,
		randSeed:    seed,
		moduleCache: cache,
	}
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
	thread.SetMaxExecutionSteps(cfg.MaxSteps)
	stopWatchdog, timedOut := startWatchdog(ctx, cfg.WallClock, thread)
	defer stopWatchdog()

	fileOpts := &syntax.FileOptions{TopLevelControl: true, GlobalReassign: true}
	dict, err := starlarkgo.ExecFileOptions(fileOpts, thread, "<test>", script, globals)
	if err != nil {
		return nil, r.wrapExecError(err, KindScript, timedOut.Load())
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
	callThread.SetMaxExecutionSteps(cfg.MaxSteps)
	stopWatchdog2, timedOut2 := startWatchdog(ctx, cfg.WallClock, callThread)
	defer stopWatchdog2()

	_, callErr := starlarkgo.Call(callThread, callable, nil, nil)
	elapsed := time.Since(start)
	steps := callThread.Steps

	if callErr != nil {
		return &Result{Elapsed: elapsed, Steps: steps}, r.wrapExecError(callErr, KindScript, timedOut2.Load())
	}
	return &Result{Value: starlarkgo.None, Elapsed: elapsed, Steps: steps}, nil
}
