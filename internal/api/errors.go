// Package api provides shared helpers for the Connect-RPC API layer,
// including error mapping, pagination, and time conversion utilities.
package api

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"

	errorv1 "github.com/fdatoo/switchyard/gen/switchyard/error/v1alpha1"
	"github.com/fdatoo/switchyard/internal/auth"
	"github.com/fdatoo/switchyard/internal/observability"
)

var (
	// ErrEntityNotFound maps to Connect NotFound for entity APIs.
	ErrEntityNotFound = errors.New("entity not found")
	// ErrDeviceNotFound maps to Connect NotFound for device APIs.
	ErrDeviceNotFound = errors.New("device not found")
	// ErrAreaNotFound maps to Connect NotFound for area APIs.
	ErrAreaNotFound = errors.New("area not found")
	// ErrZoneNotFound maps to Connect NotFound for zone APIs.
	ErrZoneNotFound = errors.New("zone not found")
	// ErrDriverNotFound maps to Connect NotFound for driver APIs.
	ErrDriverNotFound = errors.New("driver not found")
	// ErrInstanceNotFound maps to Connect NotFound for driver-instance APIs.
	ErrInstanceNotFound = errors.New("driver instance not found")
	// ErrAutomationNotFound maps to Connect NotFound for automation APIs.
	ErrAutomationNotFound = errors.New("automation not found")
	// ErrScriptNotFound maps to Connect NotFound for script APIs.
	ErrScriptNotFound = errors.New("script not found")
	// ErrAutomationDisabled maps to Connect FailedPrecondition.
	ErrAutomationDisabled = errors.New("automation disabled")
	// ErrRunNotFound maps to Connect NotFound for run-scoped APIs.
	ErrRunNotFound = errors.New("run not found")
	// ErrRunAlreadyFinished maps to Connect FailedPrecondition for cancellation.
	ErrRunAlreadyFinished = errors.New("run already finished")
	// ErrCapabilityUnknown maps to Connect InvalidArgument for unsupported calls.
	ErrCapabilityUnknown = errors.New("capability unknown")
	// ErrDriverUnavailable maps to Connect Unavailable while a driver is down.
	ErrDriverUnavailable = errors.New("driver unavailable")
	// ErrSubscriptionOverflow maps to Connect ResourceExhausted for slow streams.
	ErrSubscriptionOverflow = errors.New("subscription overflow")
	// ErrValidationFailed maps to Connect InvalidArgument for user input.
	ErrValidationFailed = errors.New("validation failed")
	// ErrNotImplemented maps to Connect Unimplemented for reserved surfaces.
	ErrNotImplemented = errors.New("not implemented")
	// ErrPathEscape maps to Connect InvalidArgument for config path traversal.
	ErrPathEscape = errors.New("path escapes config dir")
)

// ToConnect converts domain errors to Connect errors with structured details.
func ToConnect(ctx context.Context, err error, reason string) error {
	code := classify(err)
	msg := err.Error()
	if code == connect.CodeInternal {
		requestID, _ := observability.RequestIDFromContext(ctx)
		slog.ErrorContext(ctx, "api: internal error",
			slog.String("request_id", requestID),
			slog.Any("error", err))
		msg = "internal error"
	}

	ce := connect.NewError(code, errors.New(msg))

	requestID, _ := observability.RequestIDFromContext(ctx)
	detail := &errorv1.ErrorDetail{
		Reason:    reason,
		RequestId: requestID,
	}
	if d, derr := connect.NewErrorDetail(detail); derr == nil {
		ce.AddDetail(d)
	}
	return ce
}

func classify(err error) connect.Code {
	switch {
	case errors.Is(err, ErrEntityNotFound),
		errors.Is(err, ErrDeviceNotFound),
		errors.Is(err, ErrAreaNotFound),
		errors.Is(err, ErrZoneNotFound),
		errors.Is(err, ErrDriverNotFound),
		errors.Is(err, ErrInstanceNotFound),
		errors.Is(err, ErrAutomationNotFound),
		errors.Is(err, ErrScriptNotFound),
		errors.Is(err, ErrRunNotFound):
		return connect.CodeNotFound
	case errors.Is(err, ErrAutomationDisabled),
		errors.Is(err, ErrRunAlreadyFinished):
		return connect.CodeFailedPrecondition
	case errors.Is(err, ErrCapabilityUnknown),
		errors.Is(err, ErrValidationFailed),
		errors.Is(err, ErrPathEscape):
		return connect.CodeInvalidArgument
	case errors.Is(err, ErrDriverUnavailable):
		return connect.CodeUnavailable
	case errors.Is(err, ErrSubscriptionOverflow):
		return connect.CodeResourceExhausted
	case errors.Is(err, ErrNotImplemented):
		return connect.CodeUnimplemented
	case errors.Is(err, auth.ErrUnauthenticated):
		return connect.CodeUnauthenticated
	case errors.Is(err, auth.ErrForbidden):
		return connect.CodePermissionDenied
	case errors.Is(err, context.Canceled):
		return connect.CodeCanceled
	case errors.Is(err, context.DeadlineExceeded):
		return connect.CodeDeadlineExceeded
	}
	return connect.CodeInternal
}
