package script

import (
	"context"
	stderr "errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	starlarkgo "go.starlark.net/starlark"

	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/observability"
	ghstarlark "github.com/fdatoo/switchyard/internal/starlark"
)

// Deps is the set of external dependencies the script engine needs.
type Deps struct {
	Store   EventAppender
	Logger  *slog.Logger
	Metrics *observability.Metrics
}

// EventAppender is the eventstore subset used by the script engine.
type EventAppender interface {
	Append(ctx context.Context, e eventstore.Event) (uint64, error)
}

// Engine runs compiled scripts. Safe for concurrent use.
type Engine struct {
	runtime *ghstarlark.Runtime
	deps    Deps

	mu      sync.RWMutex
	scripts map[string]*Script

	inFlight sync.WaitGroup
}

// NewEngine constructs an engine with an initial script map. Nil runtime is
// permitted for tests that only exercise List/Get; Call will error without it.
func NewEngine(scripts map[string]*Script, runtime *ghstarlark.Runtime, deps Deps) *Engine {
	if scripts == nil {
		scripts = map[string]*Script{}
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	e := &Engine{runtime: runtime, deps: deps, scripts: scripts}
	if deps.Metrics != nil {
		deps.Metrics.ScriptRegistered.Set(float64(len(scripts)))
	}
	return e
}

// List returns script names sorted.
func (e *Engine) List() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]string, 0, len(e.scripts))
	for k := range e.scripts {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Get returns the runtime Script by name or ErrScriptNotFound.
func (e *Engine) Get(name string) (*Script, error) {
	e.mu.RLock()
	s, ok := e.scripts[name]
	e.mu.RUnlock()
	if !ok {
		return nil, ErrScriptNotFound
	}
	return s, nil
}

// Runtime returns the underlying Starlark runtime (used by automation actions).
func (e *Engine) Runtime() *ghstarlark.Runtime { return e.runtime }

// CallResult summarises one script invocation.
type CallResult struct {
	CorrelationID string
	Outcome       eventv1.RunOutcome
	Error         string
	Elapsed       time.Duration
	Steps         uint64
	Logs          []string
	ReturnValue   string
}

// Call executes the named script. `invokedBy` is a provenance string like
// "cli:<user>" or "automation:<id>". `sharedCorrID` is used when the call
// is nested inside an automation run (so all events share one corr_id);
// empty means mint a fresh UUID.
func (e *Engine) Call(
	ctx context.Context,
	name string,
	args map[string]string,
	invokedBy string,
	sharedCorrID string,
) (*CallResult, error) {
	e.mu.RLock()
	s, ok := e.scripts[name]
	e.mu.RUnlock()
	if !ok {
		return nil, ErrScriptNotFound
	}
	if e.runtime == nil {
		return nil, fmt.Errorf("script %q: runtime not configured", name)
	}

	corrID := sharedCorrID
	if corrID == "" {
		corrID = uuid.NewString()
	}
	corrUUID, err := uuid.Parse(corrID)
	if err != nil {
		return nil, fmt.Errorf("invalid correlation id %q: %w", corrID, err)
	}

	coerced, err := e.validateArgs(s, args)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrScriptArgs, err)
	}

	e.inFlight.Add(1)
	defer e.inFlight.Done()

	// Emit ScriptInvoked.
	if e.deps.Store != nil {
		_, _ = e.deps.Store.Append(ctx, eventstore.Event{
			Kind:          "script_invoked",
			Source:        "script",
			Timestamp:     time.Now(),
			CorrelationID: corrUUID,
			Payload: &eventv1.Payload{Kind: &eventv1.Payload_ScriptInvoked{
				ScriptInvoked: &eventv1.ScriptInvoked{
					ScriptName:    name,
					CorrelationId: corrID,
					InvokedBy:     invokedBy,
					Args:          args,
				},
			}},
		})
	}

	extra := starlarkgo.StringDict{
		"params":         argsToStarlarkDict(coerced),
		"correlation_id": starlarkgo.String(corrID),
	}

	start := time.Now()
	execRes, execErr := e.runtime.Execute(ctx, ghstarlark.KindScript, s.Handler, extra)
	elapsed := time.Since(start)

	result := &CallResult{
		CorrelationID: corrID,
		Elapsed:       elapsed,
	}
	if execErr != nil {
		result.Outcome = classifyExecError(execErr, ctx)
		result.Error = execErr.Error()
	} else {
		result.Outcome = eventv1.RunOutcome_OUTCOME_OK
		if execRes != nil {
			result.Steps = execRes.Steps
			result.Logs = execRes.Logs
			if execRes.Value != nil {
				result.ReturnValue = execRes.Value.String()
			}
		}
	}

	if e.deps.Store != nil {
		_, _ = e.deps.Store.Append(ctx, eventstore.Event{
			Kind:          "script_finished",
			Source:        "script",
			Timestamp:     time.Now(),
			CorrelationID: corrUUID,
			Payload: &eventv1.Payload{Kind: &eventv1.Payload_ScriptFinished{
				ScriptFinished: &eventv1.ScriptFinished{
					ScriptName:    name,
					CorrelationId: corrID,
					Outcome:       result.Outcome,
					Error:         result.Error,
					ElapsedMs:     result.Elapsed.Milliseconds(),
					StarlarkSteps: result.Steps,
					LogLines:      result.Logs,
					ReturnValue:   result.ReturnValue,
				},
			}},
		})
	}

	if e.deps.Metrics != nil {
		outcomeLabel := scriptOutcomeLabel(result.Outcome)
		invokedByKind := invokedByKind(invokedBy)
		e.deps.Metrics.ScriptInvocationsTotal.WithLabelValues(name, outcomeLabel, invokedByKind).Inc()
		e.deps.Metrics.ScriptDurationSeconds.WithLabelValues(name).Observe(elapsed.Seconds())
	}

	if execErr != nil {
		return result, execErr
	}
	return result, nil
}

// scriptOutcomeLabel converts a RunOutcome to the lower-snake label string
// for switchyard_script_invocations_total{outcome}. Strips "OUTCOME_" prefix.
func scriptOutcomeLabel(o eventv1.RunOutcome) string {
	s := o.String()
	const prefix = "OUTCOME_"
	if len(s) > len(prefix) {
		return strings.ToLower(s[len(prefix):])
	}
	return strings.ToLower(s)
}

// invokedByKind extracts the kind prefix from an invokedBy string of the form
// "kind:detail" (e.g. "cli:user", "automation:my_id"). Returns "unknown" if
// the string has no colon.
func invokedByKind(invokedBy string) string {
	if i := strings.IndexByte(invokedBy, ':'); i > 0 {
		return invokedBy[:i]
	}
	if invokedBy != "" {
		return invokedBy
	}
	return "unknown"
}

// Stop waits for in-flight Calls to drain.
func (e *Engine) Stop(ctx context.Context) error {
	done := make(chan struct{})
	go func() { e.inFlight.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (e *Engine) validateArgs(s *Script, args map[string]string) (map[string]any, error) {
	knownParams := map[string]Param{}
	for _, p := range s.Params {
		knownParams[p.Name] = p
	}
	for k := range args {
		if _, ok := knownParams[k]; !ok {
			return nil, fmt.Errorf("unknown argument %q", k)
		}
	}
	out := map[string]any{}
	for _, p := range s.Params {
		raw, provided := args[p.Name]
		if !provided || raw == "" {
			if p.Required {
				return nil, fmt.Errorf("missing required argument %q", p.Name)
			}
			if p.HasDefault {
				out[p.Name] = p.Default
			}
			continue
		}
		v, err := p.Coerce(raw)
		if err != nil {
			return nil, err
		}
		out[p.Name] = v
	}
	return out, nil
}

func argsToStarlarkDict(m map[string]any) *starlarkgo.Dict {
	d := starlarkgo.NewDict(len(m))
	for k, v := range m {
		_ = d.SetKey(starlarkgo.String(k), toStarlarkValue(v))
	}
	return d
}

func toStarlarkValue(v any) starlarkgo.Value {
	switch x := v.(type) {
	case string:
		return starlarkgo.String(x)
	case int64:
		return starlarkgo.MakeInt64(x)
	case float64:
		return starlarkgo.Float(x)
	case bool:
		return starlarkgo.Bool(x)
	default:
		return starlarkgo.None
	}
}

func classifyExecError(err error, ctx context.Context) eventv1.RunOutcome {
	if err == nil {
		return eventv1.RunOutcome_OUTCOME_OK
	}
	var le *ghstarlark.LimitError
	if stderr.As(err, &le) {
		return eventv1.RunOutcome_OUTCOME_LIMIT_EXCEEDED
	}
	if ctx.Err() != nil {
		return eventv1.RunOutcome_OUTCOME_CANCELLED
	}
	return eventv1.RunOutcome_OUTCOME_ACTION_ERROR
}
