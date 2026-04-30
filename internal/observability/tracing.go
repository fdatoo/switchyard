package observability

import "context"

// Span is the minimal tracing surface. C1 ships a no-op implementation;
// C13 replaces this with an OpenTelemetry bridge — call sites do not change.
type Span interface {
	End()
	SetAttr(key string, value any)
	RecordError(err error)
}

type noopSpan struct{}

func (noopSpan) End()                {}
func (noopSpan) SetAttr(string, any) {}
func (noopSpan) RecordError(error)   {}

func StartSpan(ctx context.Context, _ string) (context.Context, Span) {
	return ctx, noopSpan{}
}
