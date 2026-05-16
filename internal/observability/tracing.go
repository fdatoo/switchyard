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

// SpanStarter starts a tracing span and returns the context that should carry it.
type SpanStarter func(ctx context.Context, name string) (context.Context, Span)

type spanContextKey struct{}

var (
	spanStarterMu sync.RWMutex
	spanStarter   SpanStarter = startNoopSpan
)

func startNoopSpan(ctx context.Context, _ string) (context.Context, Span) {
	return ctx, noopSpan{}
}

// StartSpan starts a span using the configured starter and stores it in context.
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

// SpanFromContext returns the span previously attached by StartSpan.
func SpanFromContext(ctx context.Context) (Span, bool) {
	span, ok := ctx.Value(spanContextKey{}).(Span)
	if !ok || span == nil {
		return nil, false
	}
	return span, true
}

// SetSpanStarterForTest replaces the span starter and returns a restore function.
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
