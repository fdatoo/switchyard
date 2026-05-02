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
	ErrEntityNotFound       = errors.New("entity not found")
	ErrDeviceNotFound       = errors.New("device not found")
	ErrAreaNotFound         = errors.New("area not found")
	ErrZoneNotFound         = errors.New("zone not found")
	ErrDriverNotFound       = errors.New("driver not found")
	ErrInstanceNotFound     = errors.New("driver instance not found")
	ErrAutomationNotFound   = errors.New("automation not found")
	ErrScriptNotFound       = errors.New("script not found")
	ErrAutomationDisabled   = errors.New("automation disabled")
	ErrRunNotFound          = errors.New("run not found")
	ErrRunAlreadyFinished   = errors.New("run already finished")
	ErrCapabilityUnknown    = errors.New("capability unknown")
	ErrDriverUnavailable    = errors.New("driver unavailable")
	ErrSubscriptionOverflow = errors.New("subscription overflow")
	ErrValidationFailed     = errors.New("validation failed")
	ErrNotImplemented       = errors.New("not implemented")
	ErrPathEscape           = errors.New("path escapes config dir")
)

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
