package observability

import (
	"context"
	"sync"
)

// Span is the minimal tracing surface. C1 ships a no-op implementation;
// C13 replaces this with an OpenTelemetry bridge — call sites do not change.
type Span interface {
	End()
	SetAttr(key string, value any)
	AddEvent(name string, attrs ...any)
	RecordError(err error)
}

type noopSpan struct{}

func (noopSpan) End()                    {}
func (noopSpan) SetAttr(string, any)     {}
func (noopSpan) AddEvent(string, ...any) {}
func (noopSpan) RecordError(error)       {}

type SpanStarter func(ctx context.Context, name string) (context.Context, Span)

type spanContextKey struct{}

var (
	spanStarterMu sync.RWMutex
	spanStarter   SpanStarter = startNoopSpan
)

func startNoopSpan(ctx context.Context, _ string) (context.Context, Span) {
	return ctx, noopSpan{}
}

func StartSpan(ctx context.Context, name string) (context.Context, Span) {
	spanStarterMu.RLock()
	start := spanStarter
	spanStarterMu.RUnlock()
	ctx, span := start(ctx, name)
	if span == nil {
		span = noopSpan{}
	}
	return context.WithValue(ctx, spanContextKey{}, span), span
}

func SpanFromContext(ctx context.Context) (Span, bool) {
	span, ok := ctx.Value(spanContextKey{}).(Span)
	if !ok || span == nil {
		return nil, false
	}
	return span, true
}

func SetSpanStarterForTest(start SpanStarter) func() {
	if start == nil {
		start = startNoopSpan
	}

	spanStarterMu.Lock()
	prev := spanStarter
	spanStarter = start
	spanStarterMu.Unlock()

	return func() {
		spanStarterMu.Lock()
		spanStarter = prev
		spanStarterMu.Unlock()
	}
}
