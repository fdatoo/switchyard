package api

import (
	"context"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
	"github.com/fdatoo/switchyard/internal/compute"
)

type ConfigService struct{ be ConfigApplier }

func NewConfigService(be ConfigApplier) *ConfigService { return &ConfigService{be: be} }

var _ switchyardv1alpha1connect.ConfigServiceHandler = (*ConfigService)(nil)

func (s *ConfigService) Validate(ctx context.Context, req *connect.Request[v1.ValidateConfigRequest]) (*connect.Response[v1.ValidateConfigResponse], error) {
	valid, errs, diff, hash, err := s.be.Validate(ctx, req.Msg.PklBundle)
	if err != nil {
		return nil, ToConnect(ctx, err, "validate_failed")
	}
	return connect.NewResponse(&v1.ValidateConfigResponse{
		Valid:      valid,
		Errors:     errs,
		Diff:       configDiffToProto(diff),
		BundleHash: hash,
	}), nil
}

func (s *ConfigService) Apply(ctx context.Context, req *connect.Request[v1.ApplyConfigRequest]) (*connect.Response[v1.ApplyConfigResponse], error) {
	result, err := s.be.Apply(ctx, req.Msg.PklBundle, req.Msg.Message, req.Msg.ExpectedBundleHash, req.Msg.DryRun, req.Msg.Strict, principalID(ctx))
	if err != nil {
		return nil, ToConnect(ctx, err, "apply_failed")
	}
	return connect.NewResponse(&v1.ApplyConfigResponse{
		Applied:       result.Applied,
		Diff:          configDiffToProto(result.Diff),
		CorrelationId: result.CorrelationID,
		BundleHash:    result.BundleHash,
	}), nil
}

func (s *ConfigService) Reload(ctx context.Context, _ *connect.Request[v1.ReloadConfigRequest]) (*connect.Response[v1.ReloadConfigResponse], error) {
	diff, correlationID, err := s.be.Reload(ctx, principalID(ctx))
	if err != nil {
		return nil, ToConnect(ctx, err, "reload_failed")
	}
	return connect.NewResponse(&v1.ReloadConfigResponse{
		Diff:          configDiffToProto(diff),
		CorrelationId: correlationID,
	}), nil
}

func (s *ConfigService) GetArtifact(ctx context.Context, _ *connect.Request[v1.GetConfigArtifactRequest]) (*connect.Response[v1.GetConfigArtifactResponse], error) {
	snap, err := s.be.CurrentArtifact(ctx)
	if err != nil {
		return nil, ToConnect(ctx, err, "artifact_failed")
	}
	return connect.NewResponse(&v1.GetConfigArtifactResponse{Snapshot: snap}), nil
}

func (s *ConfigService) EvalCompute(ctx context.Context, req *connect.Request[v1.EvalComputeRequest]) (*connect.Response[v1.EvalComputeResponse], error) {
	svc := compute.NewService()
	result := svc.Eval(ctx, compute.Request{
		DashboardSlug: req.Msg.GetDashboardSlug(),
		WidgetID:      req.Msg.GetWidgetId(),
		ExprID:        req.Msg.GetExprId(),
	})
	if result.Error != "" {
		return connect.NewResponse(&v1.EvalComputeResponse{Error: result.Error}), nil
	}
	return connect.NewResponse(&v1.EvalComputeResponse{}), nil
}

func configDiffToProto(d ConfigDiff) *v1.ConfigDiff {
	return &v1.ConfigDiff{
		DriverInstancesAdded:   d.DriverAdded,
		DriverInstancesRemoved: d.DriverRemoved,
		DriverInstancesChanged: d.DriverChanged,
		EntitiesAdded:          d.EntitiesAdded,
		EntitiesRemoved:        d.EntitiesRemoved,
		AutomationsChanged:     d.AutomationsChanged,
		Lines:                  d.Lines,
	}
}
