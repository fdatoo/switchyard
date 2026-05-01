package api_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	errorv1 "github.com/fdatoo/switchyard/gen/switchyard/error/v1alpha1"
	"github.com/fdatoo/switchyard/internal/api"
	"github.com/fdatoo/switchyard/internal/auth"
)

var errSentinel = errors.New("sentinel")

func TestToConnect_NotFound(t *testing.T) {
	err := api.ToConnect(context.Background(), api.ErrEntityNotFound, "entity_not_found")
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("err not connect.Error: %v", err)
	}
	if ce.Code() != connect.CodeNotFound {
		t.Errorf("code = %v, want NotFound", ce.Code())
	}
	var detail errorv1.ErrorDetail
	if !hasDetail(ce, &detail) {
		t.Fatalf("no ErrorDetail attached")
	}
	if detail.Reason != "entity_not_found" {
		t.Errorf("reason = %q", detail.Reason)
	}
}

func TestToConnect_Unauthenticated(t *testing.T) {
	err := api.ToConnect(context.Background(), auth.ErrUnauthenticated, "unauthenticated")
	var ce *connect.Error
	errors.As(err, &ce)
	if ce.Code() != connect.CodeUnauthenticated {
		t.Errorf("code = %v, want Unauthenticated", ce.Code())
	}
}

func TestToConnect_InternalFallback(t *testing.T) {
	err := api.ToConnect(context.Background(), errSentinel, "")
	var ce *connect.Error
	errors.As(err, &ce)
	if ce.Code() != connect.CodeInternal {
		t.Errorf("code = %v, want Internal", ce.Code())
	}
	if ce.Message() == errSentinel.Error() {
		t.Errorf("internal error leaked raw message: %q", ce.Message())
	}
}

func hasDetail(ce *connect.Error, out *errorv1.ErrorDetail) bool {
	for _, d := range ce.Details() {
		v, err := d.Value()
		if err != nil {
			continue
		}
		if ed, ok := v.(*errorv1.ErrorDetail); ok {
			out.Reason = ed.Reason
			return true
		}
	}
	return false
}
