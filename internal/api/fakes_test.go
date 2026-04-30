package api_test

import (
	"context"
	"time"

	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/api"
	"github.com/fdatoo/gohome/internal/auth"
)

type fakeSystem struct {
	version      api.VersionInfo
	healthy      bool
	subs         []api.SubsystemHealth
	metrics      string
	bundle       []byte
	configHash   string
	snapshotErr  error
	lastOwner    string
	lastReason   string
	configDir    string
	mcpCfg       api.MCPConfig
	recordResult uint64
	recordErr    error
	lastRecord   struct {
		principal auth.Principal
		path      string
	}
}

func (f *fakeSystem) Version() api.VersionInfo { return f.version }
func (f *fakeSystem) Health(_ context.Context) (bool, string, []api.SubsystemHealth) {
	if f.healthy {
		return true, "ok", f.subs
	}
	return false, "degraded", f.subs
}
func (f *fakeSystem) MetricsText() (string, error) { return f.metrics, nil }
func (f *fakeSystem) Diagnostics(_ context.Context) ([]byte, string, time.Time, error) {
	return f.bundle, f.configHash, time.Unix(1700000000, 0).UTC(), nil
}
func (f *fakeSystem) CreateSnapshot(_ context.Context, owner, reason string) (uint64, time.Time, error) {
	if f.snapshotErr != nil {
		return 0, time.Time{}, f.snapshotErr
	}
	f.lastOwner = owner
	f.lastReason = reason
	return 1234, time.Unix(1700000001, 0).UTC(), nil
}
func (f *fakeSystem) ConfigDir(_ context.Context) (string, error) { return f.configDir, nil }
func (f *fakeSystem) MCPConfig(_ context.Context) (api.MCPConfig, error) {
	return f.mcpCfg, nil
}
func (f *fakeSystem) RecordConfigFileEdit(_ context.Context, p auth.Principal, _, path, _ string, _ uint32) (uint64, error) {
	f.lastRecord.principal = p
	f.lastRecord.path = path
	if f.recordErr != nil {
		return 0, f.recordErr
	}
	return f.recordResult, nil
}

var _ api.SystemBackend = (*fakeSystem)(nil)

type fakeEventAppender struct {
	appended []*eventv1.Payload
}

func (f *fakeEventAppender) Append(_ context.Context, p *eventv1.Payload) (uint64, error) {
	f.appended = append(f.appended, p)
	return uint64(len(f.appended)), nil
}

type fixedMCPCaps struct{ evalCap uint32 }

func (m fixedMCPCaps) MCPConfig(context.Context) (api.MCPConfig, error) {
	return api.MCPConfig{EvalResultMaxBytes: m.evalCap}, nil
}
