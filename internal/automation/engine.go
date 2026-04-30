package automation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/automation/action"
	"github.com/fdatoo/gohome/internal/automation/trigger"
	"github.com/fdatoo/gohome/internal/eventstore"
	"github.com/fdatoo/gohome/internal/script"
	ghstarlark "github.com/fdatoo/gohome/internal/starlark"
)

type Engine struct {
	automations  map[string]*Automation
	triggers     *trigger.Registry
	scheduler    *trigger.TimeScheduler
	runtime      *ghstarlark.Runtime
	deps         Deps
	scriptEngine *script.Engine
	scriptCaller action.ScriptCaller

	mu        sync.Mutex
	runStates map[string]*runState
	inFlight  sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	started   bool

	pumpOnces sync.Map // map[*runState]*sync.Once
}

func NewEngine(autos map[string]*Automation, se *script.Engine, rt *ghstarlark.Runtime, deps Deps) *Engine {
	e := &Engine{
		automations:  autos,
		triggers:     trigger.NewRegistry(),
		scheduler:    trigger.NewTimeScheduler(time.Local),
		runtime:      rt,
		deps:         deps,
		runStates:    map[string]*runState{},
		scriptEngine: se,
	}
	if se != nil {
		e.scriptCaller = &scriptCallerAdapter{engine: se}
	}
	// Wrap leaf actions with metricExecutor before populating runStates so that
	// every run uses the instrumented versions. This handles the common case
	// where autos come from CompileAutomations (uninstrumented).
	if deps.Metrics != nil {
		for _, a := range autos {
			wrapActionsWithMetrics(a, deps.Metrics)
		}
	}
	// Populate runStates BEFORE registerTriggers (hold callbacks reference ctxOrBG).
	for id, a := range autos {
		e.runStates[id] = newRunState(a)
	}
	for _, a := range autos {
		e.registerTriggers(a)
	}
	if deps.Metrics != nil {
		deps.Metrics.AutomationRegistered.Set(float64(len(autos)))
	}
	return e
}

func (e *Engine) registerTriggers(a *Automation) {
	for _, m := range a.Triggers {
		switch t := m.(type) {
		case *trigger.StateChangeMatcher:
			holdFn := func(match trigger.Match) { //nolint:contextcheck // lifecycle context; holdFn signature is fixed by StateChangeMatcher API
				ctx := e.ctxOrBG() //nolint:contextcheck
				select {
				case <-ctx.Done():
					return
				default:
				}
				e.inFlight.Add(1)
				go e.fire(ctx, match, "") //nolint:contextcheck
			}
			t.SetDeliverHold(holdFn)
			e.triggers.RegisterState(t)
		case *trigger.EventMatcher:
			e.triggers.RegisterEvent(t)
		case *trigger.WebhookMatcher:
			e.triggers.RegisterEvent(t)
		case *trigger.TimeMatcher:
			switch {
			case t.At != "":
				_ = e.scheduler.AddAt(a.ID, t.At)
			case t.Cron != "":
				_ = e.scheduler.AddCron(a.ID, t.Cron)
			case t.Every > 0:
				_ = e.scheduler.AddEvery(a.ID, t.Every)
			}
		}
	}
}

func (e *Engine) unregisterTriggers(a *Automation) {
	for _, m := range a.Triggers {
		switch t := m.(type) {
		case *trigger.StateChangeMatcher:
			t.Stop()
			e.triggers.Unregister(t)
		case *trigger.EventMatcher:
			e.triggers.Unregister(t)
		case *trigger.WebhookMatcher:
			e.triggers.Unregister(t)
		}
	}
}

func (e *Engine) ctxOrBG() context.Context {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ctx != nil {
		return e.ctx
	}
	return context.Background()
}

func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.started {
		e.mu.Unlock()
		return nil
	}
	engineCtx, cancel := context.WithCancel(ctx)
	e.ctx = engineCtx
	e.cancel = cancel
	e.started = true
	e.mu.Unlock()

	sub, err := e.deps.Store.Subscribe(engineCtx, eventstore.SubscribeOptions{
		FromPosition: e.deps.Store.LatestPosition(),
		Filter:       eventstore.Filter{},
	})
	if err != nil {
		return fmt.Errorf("automation subscribe: %w", err)
	}
	go e.scheduler.Run(engineCtx)
	go e.runLoop(sub)
	return nil
}

func (e *Engine) runLoop(sub eventstore.Subscription) {
	defer func() { _ = sub.Close() }()
	for {
		select {
		case <-e.ctx.Done():
			return
		case ev, ok := <-sub.C():
			if !ok {
				return
			}
			for _, m := range e.triggers.Dispatch(ev) {
				e.inFlight.Add(1)
				go e.fire(e.ctx, m, "")
			}
		case m := <-e.scheduler.Ready():
			e.inFlight.Add(1)
			go e.fire(e.ctx, m, "")
		}
	}
}

func (e *Engine) Stop(ctx context.Context) {
	e.mu.Lock()
	if !e.started {
		e.mu.Unlock()
		return
	}
	cancel := e.cancel
	e.started = false
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	done := make(chan struct{})
	go func() { e.inFlight.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
	case <-time.After(30 * time.Second):
	}
}

func (e *Engine) Trigger(ctx context.Context, id, invokedBy string) error {
	e.mu.Lock()
	rs, ok := e.runStates[id]
	e.mu.Unlock()
	if !ok {
		return fmt.Errorf("automation %q not found", id)
	}
	if !rs.auto.Enabled {
		return fmt.Errorf("automation %q disabled", id)
	}
	e.inFlight.Add(1)
	go e.fire(ctx, trigger.Match{AutomationID: id, TriggerKind: "manual"}, invokedBy)
	return nil
}

type Summary struct {
	ID, Mode string
	Enabled  bool
}

func (e *Engine) List() []Summary {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]Summary, 0, len(e.runStates))
	for id, rs := range e.runStates {
		out = append(out, Summary{ID: id, Mode: rs.auto.Mode.String(), Enabled: rs.auto.Enabled})
	}
	return out
}

func (e *Engine) Get(id string) (Summary, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	rs, ok := e.runStates[id]
	if !ok {
		return Summary{}, false
	}
	return Summary{ID: id, Mode: rs.auto.Mode.String(), Enabled: rs.auto.Enabled}, true
}

func (e *Engine) SetEnabled(id string, enabled bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	rs, ok := e.runStates[id]
	if !ok {
		return fmt.Errorf("automation %q not found", id)
	}
	rs.auto.Enabled = enabled
	return nil
}

func (e *Engine) fire(ctx context.Context, m trigger.Match, invokedBy string) {
	// Callers always do e.inFlight.Add(1) before launching this goroutine;
	// we own the matching Done().
	defer e.inFlight.Done()

	e.mu.Lock()
	rs, ok := e.runStates[m.AutomationID]
	e.mu.Unlock()
	if !ok {
		return
	}
	rs.queueMu.Lock()
	auto := rs.auto
	rs.queueMu.Unlock()
	if !auto.Enabled {
		return
	}

	// Increment trigger counter for all admitted fires (before mode switch).
	if e.deps.Metrics != nil {
		e.deps.Metrics.AutomationTriggersTotal.WithLabelValues(auto.ID, m.TriggerKind).Inc()
	}

	switch auto.Mode {
	case ModeSingle:
		if !rs.running.CompareAndSwap(false, true) {
			e.emitSkipped(ctx, auto, m, "already running", invokedBy)
			return
		}
		defer rs.running.Store(false)
		if e.deps.Metrics != nil {
			e.deps.Metrics.AutomationInflight.WithLabelValues(auto.ID).Inc()
		}
		e.executeRun(ctx, auto, m, invokedBy)
		if e.deps.Metrics != nil {
			e.deps.Metrics.AutomationInflight.WithLabelValues(auto.ID).Dec()
		}
	case ModeRestart:
		rs.restartMu.Lock()
		if rs.activeCancel != nil {
			rs.activeCancel()
		}
		subCtx, cancel := context.WithCancel(ctx)
		rs.activeCancel = cancel
		myGen := rs.restartGen + 1
		rs.restartGen = myGen
		rs.restartMu.Unlock()
		e.inFlight.Add(1)
		if e.deps.Metrics != nil {
			e.deps.Metrics.AutomationInflight.WithLabelValues(auto.ID).Inc()
		}
		go func() {
			defer e.inFlight.Done()
			e.executeRun(subCtx, auto, m, invokedBy)
			cancel() // release resources from context.WithCancel regardless of outcome
			if e.deps.Metrics != nil {
				e.deps.Metrics.AutomationInflight.WithLabelValues(auto.ID).Dec()
			}
			rs.restartMu.Lock()
			if rs.restartGen == myGen {
				rs.activeCancel = nil
			}
			rs.restartMu.Unlock()
		}()
	case ModeQueued:
		q := rs.ensureQueue(auto.MaxQueued)
		select {
		case q <- pending{match: m, enqueuedAt: time.Now()}:
			e.startPumpOnce(ctx, rs)
		default:
			e.emitSkipped(ctx, auto, m, "queue full", invokedBy)
		}
	case ModeParallel:
		e.inFlight.Add(1)
		if e.deps.Metrics != nil {
			e.deps.Metrics.AutomationInflight.WithLabelValues(auto.ID).Inc()
		}
		go func() {
			defer e.inFlight.Done()
			e.executeRun(ctx, auto, m, invokedBy)
			if e.deps.Metrics != nil {
				e.deps.Metrics.AutomationInflight.WithLabelValues(auto.ID).Dec()
			}
		}()
	}
}

func (e *Engine) startPumpOnce(ctx context.Context, rs *runState) {
	o, _ := e.pumpOnces.LoadOrStore(rs, &sync.Once{})
	o.(*sync.Once).Do(func() {
		e.inFlight.Add(1)
		go e.queuePump(ctx, rs)
	})
}

func (e *Engine) queuePump(ctx context.Context, rs *runState) {
	defer e.inFlight.Done()
	q := rs.queue
	for {
		select {
		case p, ok := <-q:
			if !ok {
				return
			}
			if e.deps.Metrics != nil {
				e.deps.Metrics.AutomationInflight.WithLabelValues(rs.auto.ID).Inc()
			}
			e.executeRun(ctx, rs.auto, p.match, "")
			if e.deps.Metrics != nil {
				e.deps.Metrics.AutomationInflight.WithLabelValues(rs.auto.ID).Dec()
			}
		case <-ctx.Done():
			for {
				select {
				case p, ok := <-q:
					if !ok {
						return
					}
					e.emitSkipped(ctx, rs.auto, p.match, "shutdown", "")
				default:
					return
				}
			}
		}
	}
}

func (e *Engine) emitSkipped(ctx context.Context, auto *Automation, m trigger.Match, reason, invokedBy string) {
	corrID := uuid.New()
	triggerPos := uint64(0)
	if m.Event != nil {
		triggerPos = m.Event.Position
	}
	_, _ = e.deps.Store.Append(ctx, eventstore.Event{
		Kind:          "automation_triggered",
		Source:        "automation:" + auto.ID,
		Timestamp:     time.Now(),
		CorrelationID: corrID,
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_AutomationTriggered{
			AutomationTriggered: &eventv1.AutomationTriggered{
				AutomationId:         auto.ID,
				CorrelationId:        corrID.String(),
				TriggerEventPosition: triggerPos,
				TriggerKind:          m.TriggerKind,
				InvokedBy:            invokedBy,
			}}},
	})
	_, _ = e.deps.Store.Append(ctx, eventstore.Event{
		Kind:          "automation_finished",
		Source:        "automation:" + auto.ID,
		Timestamp:     time.Now(),
		CorrelationID: corrID,
		Payload: &eventv1.Payload{Kind: &eventv1.Payload_AutomationFinished{
			AutomationFinished: &eventv1.AutomationFinished{
				AutomationId:  auto.ID,
				CorrelationId: corrID.String(),
				Outcome:       eventv1.RunOutcome_OUTCOME_SKIPPED,
				Error:         reason,
			}}},
	})
	if e.deps.Metrics != nil {
		e.deps.Metrics.AutomationRunsTotal.WithLabelValues(auto.ID, "skipped").Inc()
	}
	if e.deps.Logger != nil {
		e.deps.Logger.Info("automation skipped", "automation_id", auto.ID, "reason", reason, "correlation_id", corrID.String())
	}
}

type scriptCallerAdapter struct{ engine *script.Engine }

func (a *scriptCallerAdapter) Call(ctx context.Context, name string, args map[string]string, invokedBy, shared string) (action.ScriptCallResult, error) {
	res, err := a.engine.Call(ctx, name, args, invokedBy, shared)
	if res == nil {
		return nil, err
	}
	return &scriptResultAdapter{res: res}, err
}

type scriptResultAdapter struct{ res *script.CallResult }

func (a *scriptResultAdapter) Succeeded() bool   { return a.res.Outcome == eventv1.RunOutcome_OUTCOME_OK }
func (a *scriptResultAdapter) GetError() string  { return a.res.Error }
func (a *scriptResultAdapter) GetSteps() uint64  { return a.res.Steps }
func (a *scriptResultAdapter) GetLogs() []string { return a.res.Logs }
