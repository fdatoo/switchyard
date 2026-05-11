package api

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	systemv1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
	"github.com/fdatoo/switchyard/internal/auth"
)

type SystemService struct {
	be SystemBackend
}

func NewSystemService(be SystemBackend) *SystemService {
	return &SystemService{be: be}
}

var _ switchyardv1alpha1connect.SystemServiceHandler = (*SystemService)(nil)

func (s *SystemService) Version(_ context.Context, _ *connect.Request[systemv1.VersionRequest]) (*connect.Response[systemv1.VersionResponse], error) {
	v := s.be.Version()
	return connect.NewResponse(&systemv1.VersionResponse{
		BinaryVersion: v.BinaryVersion,
		GitCommit:     v.GitCommit,
		BuildDate:     v.BuildDate,
		SchemaVersion: v.SchemaVersion,
	}), nil
}

func (s *SystemService) Health(ctx context.Context, _ *connect.Request[systemv1.HealthRequest]) (*connect.Response[systemv1.HealthResponse], error) {
	ok, summary, subs := s.be.Health(ctx)
	out := &systemv1.HealthResponse{Ok: ok, Summary: summary}
	for _, sub := range subs {
		out.Subsystems = append(out.Subsystems, &systemv1.SubsystemHealth{
			Name: sub.Name, Ok: sub.OK, Detail: sub.Detail,
		})
	}
	return connect.NewResponse(out), nil
}

func (s *SystemService) Metrics(ctx context.Context, _ *connect.Request[systemv1.MetricsRequest]) (*connect.Response[systemv1.MetricsResponse], error) {
	text, err := s.be.MetricsText()
	if err != nil {
		return nil, ToConnect(ctx, err, "metrics_unavailable")
	}
	return connect.NewResponse(&systemv1.MetricsResponse{PrometheusText: text}), nil
}

func (s *SystemService) Diagnostics(ctx context.Context, _ *connect.Request[systemv1.DiagnosticsRequest]) (*connect.Response[systemv1.DiagnosticsResponse], error) {
	bundle, hash, t, err := s.be.Diagnostics(ctx)
	if err != nil {
		return nil, ToConnect(ctx, err, "diagnostics_failed")
	}
	return connect.NewResponse(&systemv1.DiagnosticsResponse{
		Bundle:      bundle,
		ConfigHash:  hash,
		GeneratedAt: ProtoTime(t),
	}), nil
}

func (s *SystemService) CreateSnapshot(ctx context.Context, req *connect.Request[systemv1.CreateSnapshotRequest]) (*connect.Response[systemv1.CreateSnapshotResponse], error) {
	cursor, t, err := s.be.CreateSnapshot(ctx, req.Msg.Owner, req.Msg.Reason)
	if err != nil {
		return nil, ToConnect(ctx, err, "snapshot_failed")
	}
	return connect.NewResponse(&systemv1.CreateSnapshotResponse{
		Cursor:    cursor,
		CreatedAt: ProtoTime(t),
	}), nil
}

func (s *SystemService) GetConfigDir(ctx context.Context, _ *connect.Request[systemv1.GetConfigDirRequest]) (*connect.Response[systemv1.GetConfigDirResponse], error) {
	dir, err := s.be.ConfigDir(ctx)
	if err != nil {
		return nil, ToConnect(ctx, err, "config_dir_unavailable")
	}
	return connect.NewResponse(&systemv1.GetConfigDirResponse{ConfigDir: dir}), nil
}

func (s *SystemService) GetMCPConfig(ctx context.Context, _ *connect.Request[systemv1.GetMCPConfigRequest]) (*connect.Response[systemv1.GetMCPConfigResponse], error) {
	cfg, err := s.be.MCPConfig(ctx)
	if err != nil {
		return nil, ToConnect(ctx, err, "mcp_config_unavailable")
	}
	return connect.NewResponse(&systemv1.GetMCPConfigResponse{
		EvalResultMaxBytes:       cfg.EvalResultMaxBytes,
		ReadFileMaxBytes:         cfg.ReadFileMaxBytes,
		EntitySubscriptionBuffer: cfg.EntitySubscriptionBuffer,
		TraceSubscriptionBuffer:  cfg.TraceSubscriptionBuffer,
		TailDefaultWaitSeconds:   cfg.TailDefaultWaitSeconds,
		TailMaxWaitSeconds:       cfg.TailMaxWaitSeconds,
	}), nil
}

func (s *SystemService) RecordConfigFileEdit(ctx context.Context, req *connect.Request[systemv1.RecordConfigFileEditRequest]) (*connect.Response[systemv1.RecordConfigFileEditResponse], error) {
	p, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("no principal"))
	}
	cursor, err := s.be.RecordConfigFileEdit(ctx, p, req.Msg.SessionId, req.Msg.Path, req.Msg.Sha256Hex, req.Msg.SizeBytes)
	if err != nil {
		return nil, ToConnect(ctx, err, "record_failed")
	}
	return connect.NewResponse(&systemv1.RecordConfigFileEditResponse{EventCursor: cursor}), nil
}

// ExportSupportBundle builds and returns a downloadable support bundle for
// operator diagnostics (added by UI v2 plan 09).
func (s *SystemService) ExportSupportBundle(ctx context.Context, _ *connect.Request[systemv1.ExportSupportBundleRequest]) (*connect.Response[systemv1.ExportSupportBundleResponse], error) {
	bundle, filename, configHash, generatedAt, err := s.be.ExportSupportBundle(ctx)
	if err != nil {
		return nil, ToConnect(ctx, err, "export_support_bundle_failed")
	}
	return connect.NewResponse(&systemv1.ExportSupportBundleResponse{
		Bundle:      bundle,
		Filename:    filename,
		ConfigHash:  configHash,
		GeneratedAt: ProtoTime(generatedAt),
	}), nil
}

// GetEventStoreStats returns size, age, and snapshot count for the event store
// (added by UI v2 plan 09).
func (s *SystemService) GetEventStoreStats(ctx context.Context, _ *connect.Request[systemv1.GetEventStoreStatsRequest]) (*connect.Response[systemv1.GetEventStoreStatsResponse], error) {
	stats, err := s.be.EventStoreStats(ctx)
	if err != nil {
		return nil, ToConnect(ctx, err, "event_store_stats_failed")
	}
	return connect.NewResponse(&systemv1.GetEventStoreStatsResponse{
		SizeBytes:             stats.SizeBytes,
		OldestEventAgeSeconds: stats.OldestEventAgeSeconds,
		SnapshotCount:         stats.SnapshotCount,
	}), nil
}
