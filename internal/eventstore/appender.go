package eventstore

import (
	"context"

	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
)

// Appender is the narrow write interface used by packages that need to append
// events without depending on the full Store.
type Appender interface {
	AppendAuth(ctx context.Context, e *eventv1.AuthEvent) error
}
