package api

import (
	"context"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
	"github.com/fdatoo/switchyard/internal/compute"
)

// ConfigService implements validation, apply, reload, and snapshot RPCs.
type ConfigService struct{ be ConfigApplier }

// NewConfigService returns a config service backed by be.
func NewConfigService(be ConfigApplier) *ConfigService { return &ConfigService{be: be} }

var _ switchyardv1alpha1connect.ConfigServiceHandler = (*ConfigService)(nil)

// Validate checks a Pkl bundle without changing daemon state.
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

// Apply validates and applies a Pkl bundle, or dry-runs when requested.
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
		Message:       result.Message,
	}), nil
}

// Reload re-reads config from disk and applies any delta.
func (s *ConfigService) Reload(ctx context.Context, _ *connect.Request[v1.ReloadConfigRequest]) (*connect.Response[v1.ReloadConfigResponse], error) {
	diff, correlationID, err := s.be.Reload(ctx, principalID(ctx))
	if err != nil {
		return nil, ToConnect(ctx, err, "reload_failed")
	}
	return connect.NewResponse(&v1.ReloadConfigResponse{
		Diff:          configDiffToProto(diff),
		CorrelationId: correlationID,
		Error:         s.be.LastReloadError(),
	}), nil
}

// Subscribe streams config-applied notifications and idle heartbeats.
func (s *ConfigService) Subscribe(ctx context.Context, _ *connect.Request[v1.SubscribeConfigRequest], stream *connect.ServerStream[v1.SubscribeConfigEvent]) error {
	src, cancel := s.be.SubscribeConfig()
	defer cancel()

	cfg := currentStreamConfig()
	ticker := NewHeartbeatTicker(ctx, cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-src:
			if !ok {
				return nil
			}
			if err := stream.Send(&v1.SubscribeConfigEvent{
				Event: &v1.SubscribeConfigEvent_Changed{Changed: &v1.ConfigChanged{
					AtUnixMs:   ev.AtUnixMs,
					BundleHash: ev.BundleHash,
				}},
			}); err != nil {
				return err
			}
			ticker.NotePayloadSent()
		case tick := <-ticker.C():
			if err := stream.Send(&v1.SubscribeConfigEvent{
				Event: &v1.SubscribeConfigEvent_Heartbeat{Heartbeat: &v1.ConfigHeartbeat{
					AtUnixMs: tick.UnixMilli(),
				}},
			}); err != nil {
				return err
			}
		}
	}
}

// GetArtifact returns the current compiled config snapshot.
func (s *ConfigService) GetArtifact(ctx context.Context, _ *connect.Request[v1.GetConfigArtifactRequest]) (*connect.Response[v1.GetConfigArtifactResponse], error) {
	snap, err := s.be.CurrentArtifact(ctx)
	if err != nil {
		return nil, ToConnect(ctx, err, "artifact_failed")
	}
	return connect.NewResponse(&v1.GetConfigArtifactResponse{Snapshot: snap}), nil
}

// EvalCompute evaluates a computed dashboard expression.
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
