// Package cause provides a context key for threading automation correlation
// information through call chains. It is intentionally minimal to avoid import
// cycles: both internal/automation/action and internal/carport can import it
// without either depending on the other.
package cause

import "context"

type correlationKey struct{}

// Correlation carries the automation identity of the call that triggered a
// downstream dispatch. Downstream consumers (e.g. carport.Host.Dispatch) read
// it via FromCorrelation to stamp the Source field on CommandIssued events.
type Correlation struct {
	AutomationID  string
	CorrelationID string
}

// WithCorrelation attaches an automation Correlation to ctx.
func WithCorrelation(ctx context.Context, c Correlation) context.Context {
	return context.WithValue(ctx, correlationKey{}, c)
}

// FromCorrelation returns the Correlation attached to ctx, if any.
func FromCorrelation(ctx context.Context) (Correlation, bool) {
	v, ok := ctx.Value(correlationKey{}).(Correlation)
	return v, ok
}
