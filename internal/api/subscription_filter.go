package api

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	errorv1 "github.com/fdatoo/gohome/gen/gohome/error/v1alpha1"
	commonpb "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/internal/auth"
	"github.com/fdatoo/gohome/internal/policy"
)

type EntityFilter struct {
	rt   *policy.Runtime
	mode commonpb.PolicyMode
}

func NewEntityFilter(rt *policy.Runtime, mode commonpb.PolicyMode) *EntityFilter {
	if mode == commonpb.PolicyMode_POLICY_MODE_UNSPECIFIED {
		mode = commonpb.PolicyMode_POLICY_MODE_FILTER
	}
	return &EntityFilter{rt: rt, mode: mode}
}

// Preflight evaluates candidates at subscription open. In STRICT mode,
// returns CodePermissionDenied if any entity is denied. In FILTER mode,
// returns the narrowed list silently. Nil rt allows all.
func (f *EntityFilter) Preflight(ctx context.Context, p auth.Principal, candidates []policy.Target) ([]policy.Target, error) {
	if f.rt == nil {
		return candidates, nil
	}
	allowed, denied := f.rt.FilterEntities(ctx, p, "read", candidates)
	if f.mode == commonpb.PolicyMode_POLICY_MODE_STRICT && len(denied) > 0 {
		ce := connect.NewError(connect.CodePermissionDenied, errors.New("subscription_filtered"))
		detail := &errorv1.ErrorDetail{Reason: "forbidden", RequestId: requestIDFromCtx(ctx)}
		if d, derr := connect.NewErrorDetail(detail); derr == nil {
			ce.AddDetail(d)
		}
		return nil, ce
	}
	return allowed, nil
}

func (f *EntityFilter) AllowsEntity(ctx context.Context, p auth.Principal, t policy.Target) bool {
	if f.rt == nil {
		return true
	}
	allowed, _ := f.rt.FilterEntities(ctx, p, "read", []policy.Target{t})
	return len(allowed) == 1
}
