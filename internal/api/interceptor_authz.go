package api

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	errorv1 "github.com/fdatoo/switchyard/gen/switchyard/error/v1alpha1"
	"github.com/fdatoo/switchyard/internal/auth"
	"github.com/fdatoo/switchyard/internal/auth/audit"
	"github.com/fdatoo/switchyard/internal/observability"
	"github.com/fdatoo/switchyard/internal/policy"
)

// ProcedureCatalog resolves a Connect procedure name and request body to the
// corresponding auth.Action and auth.Target.
type ProcedureCatalog interface {
	Resolve(procedure string, requestAny any) (auth.Action, auth.Target, bool)
}

// NewAuthorize returns the C9 authorize interceptor. When rt is nil the
// interceptor passes all requests through (daemon bring-up before the
// policy runtime is loaded). metrics is optional; pass nil to disable metric
// emission.
func NewAuthorize(rt *policy.Runtime, catalog ProcedureCatalog, recorder *audit.Recorder, metrics *observability.Metrics) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if rt == nil || catalog == nil {
				return next(ctx, req)
			}
			principal, _ := auth.PrincipalFromContext(ctx)
			action, target, ok := catalog.Resolve(req.Spec().Procedure, req.Any())
			if !ok {
				return next(ctx, req)
			}
			start := time.Now()
			err := rt.Authorize(ctx, principal, action, target)
			elapsed := time.Since(start).Seconds()
			if err == nil {
				if metrics != nil {
					metrics.PolicyAuthorizeDurationSeconds.Observe(elapsed)
					metrics.PolicyAuthorizeTotal.WithLabelValues("allowed", "").Inc()
				}
				return next(ctx, req)
			}
			id := requestIDFromCtx(ctx)
			var fb *policy.ErrForbidden
			if errors.As(err, &fb) {
				if metrics != nil {
					metrics.PolicyAuthorizeDurationSeconds.Observe(elapsed)
					metrics.PolicyAuthorizeTotal.WithLabelValues("denied", fb.Reason).Inc()
				}
				if recorder != nil {
					_ = recorder.PolicyDenied(ctx, identityFromCtx(ctx), audit.PolicyDenied{
						ActionService: action.Service, ActionMethod: action.Method, ActionVerb: action.Verb,
						TargetKind: target.Kind, TargetID: target.ID,
						SubReason: fb.Reason, RuleName: fb.RuleName,
					})
				}
				ce := connect.NewError(connect.CodePermissionDenied, fb)
				detail := &errorv1.ErrorDetail{Reason: "forbidden", RequestId: id}
				if d, derr := connect.NewErrorDetail(detail); derr == nil {
					ce.AddDetail(d)
				}
				return nil, ce
			}
			if metrics != nil {
				metrics.PolicyAuthorizeDurationSeconds.Observe(elapsed)
				metrics.PolicyAuthorizeTotal.WithLabelValues("error", "internal").Inc()
			}
			ce := connect.NewError(connect.CodeInternal, errors.New("internal error"))
			detail := &errorv1.ErrorDetail{Reason: "internal", RequestId: id}
			if d, derr := connect.NewErrorDetail(detail); derr == nil {
				ce.AddDetail(d)
			}
			return nil, ce
		}
	}
}

func identityFromCtx(ctx context.Context) audit.Identity {
	p, _ := auth.PrincipalFromContext(ctx)
	return audit.Identity{
		PrincipalID: p.ID,
		RequestID:   requestIDFromCtx(ctx),
		SourceIP:    remoteAddrFromCtx(ctx),
		UserAgent:   userAgentFromCtx(ctx),
	}
}
