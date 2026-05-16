package api_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	configv1 "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
	"github.com/fdatoo/switchyard/internal/api"
)

type fakeConfig struct {
	validateValid bool
	validateErrs  []string
	validateDiff  api.ConfigDiff
	validateHash  string
	validateErr   error

	applyResult  api.ConfigApplyResult
	applyErr     error
	applyMessage string

	reloadDiff   api.ConfigDiff
	reloadCorrID string
	reloadErr    error

	snapshot    *configv1.ConfigSnapshot
	artifactErr error

	lastReloadError string

	subscribeCh     chan api.ConfigChangedEvent
	subscribeCancel func()
}

func (f *fakeConfig) Validate(_ context.Context, _ []byte) (bool, []string, api.ConfigDiff, string, error) {
	return f.validateValid, f.validateErrs, f.validateDiff, f.validateHash, f.validateErr
}

func (f *fakeConfig) Apply(_ context.Context, _ []byte, message, _ string, _, _ bool, _ string) (api.ConfigApplyResult, error) {
	f.applyMessage = message
	return f.applyResult, f.applyErr
}

func (f *fakeConfig) Reload(_ context.Context, _ string) (api.ConfigDiff, string, error) {
	return f.reloadDiff, f.reloadCorrID, f.reloadErr
}

func (f *fakeConfig) CurrentArtifact(_ context.Context) (*configv1.ConfigSnapshot, error) {
	return f.snapshot, f.artifactErr
}

func (f *fakeConfig) LastReloadError() string { return f.lastReloadError }

func (f *fakeConfig) SubscribeConfig() (<-chan api.ConfigChangedEvent, func()) {
	if f.subscribeCh != nil {
		cancel := f.subscribeCancel
		if cancel == nil {
			cancel = func() {}
		}
		return f.subscribeCh, cancel
	}
	ch := make(chan api.ConfigChangedEvent)
	close(ch)
	return ch, func() {}
}

var _ api.ConfigApplier = (*fakeConfig)(nil)

func TestConfigService_Validate_Valid(t *testing.T) {
	fc := &fakeConfig{
		validateValid: true,
		validateHash:  "abc123",
		validateDiff:  api.ConfigDiff{DriverAdded: 1},
	}
	s := api.NewConfigService(fc)
	resp, err := s.Validate(context.Background(), connect.NewRequest(&v1.ValidateConfigRequest{PklBundle: []byte("pkl")}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !resp.Msg.Valid || resp.Msg.BundleHash != "abc123" {
		t.Errorf("unexpected response: %+v", resp.Msg)
	}
	if resp.Msg.Diff.DriverInstancesAdded != 1 {
		t.Errorf("diff not mapped: %+v", resp.Msg.Diff)
	}
}

func TestConfigService_Validate_Invalid(t *testing.T) {
	fc := &fakeConfig{
		validateValid: false,
		validateErrs:  []string{"error: unknown driver"},
	}
	s := api.NewConfigService(fc)
	resp, err := s.Validate(context.Background(), connect.NewRequest(&v1.ValidateConfigRequest{}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Msg.Valid || len(resp.Msg.Errors) != 1 {
		t.Errorf("unexpected: %+v", resp.Msg)
	}
}

func TestConfigService_Validate_BackendError(t *testing.T) {
	fc := &fakeConfig{validateErr: errors.New("disk full")}
	s := api.NewConfigService(fc)
	_, err := s.Validate(context.Background(), connect.NewRequest(&v1.ValidateConfigRequest{}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeInternal {
		t.Fatalf("expected CodeInternal, got: %v", err)
	}
}

func TestConfigService_Apply(t *testing.T) {
	fc := &fakeConfig{
		applyResult: api.ConfigApplyResult{
			Applied:       true,
			CorrelationID: "corr-1",
			BundleHash:    "hash-1",
			Message:       "config(repo): validate golden config",
		},
	}
	s := api.NewConfigService(fc)
	resp, err := s.Apply(context.Background(), connect.NewRequest(&v1.ApplyConfigRequest{PklBundle: []byte("pkl"), Message: "config(repo): validate golden config"}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fc.applyMessage != "config(repo): validate golden config" {
		t.Fatalf("applyMessage = %q", fc.applyMessage)
	}
	if !resp.Msg.Applied || resp.Msg.CorrelationId != "corr-1" || resp.Msg.BundleHash != "hash-1" || resp.Msg.Message != "config(repo): validate golden config" {
		t.Errorf("unexpected: %+v", resp.Msg)
	}
}

func TestConfigService_Reload(t *testing.T) {
	fc := &fakeConfig{
		reloadDiff:   api.ConfigDiff{EntitiesAdded: 2},
		reloadCorrID: "reload-corr-1",
	}
	s := api.NewConfigService(fc)
	resp, err := s.Reload(context.Background(), connect.NewRequest(&v1.ReloadConfigRequest{}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Msg.CorrelationId != "reload-corr-1" {
		t.Errorf("unexpected correlation_id: %s", resp.Msg.CorrelationId)
	}
	if resp.Msg.Diff.EntitiesAdded != 2 {
		t.Errorf("diff not mapped: %+v", resp.Msg.Diff)
	}
}

func TestConfigService_GetArtifact(t *testing.T) {
	snap := &configv1.ConfigSnapshot{ConfigDir: "/etc/switchyard"}
	fc := &fakeConfig{snapshot: snap}
	s := api.NewConfigService(fc)
	resp, err := s.GetArtifact(context.Background(), connect.NewRequest(&v1.GetConfigArtifactRequest{}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Msg.Snapshot == nil || resp.Msg.Snapshot.ConfigDir != "/etc/switchyard" {
		t.Errorf("unexpected snapshot: %+v", resp.Msg.Snapshot)
	}
}

func TestConfigService_Subscribe_ChangedEvent(t *testing.T) {
	api.SetStreamConfig(api.StreamConfig{HeartbeatInterval: 10 * time.Millisecond, BufSize: 4})
	defer api.SetStreamConfig(api.DefaultStreamConfig())

	ch := make(chan api.ConfigChangedEvent, 1)
	ch <- api.ConfigChangedEvent{AtUnixMs: 1234567890, BundleHash: "abc123"}

	fc := &fakeConfig{subscribeCh: ch}
	s := api.NewConfigService(fc)
	client, cleanup := newConfigServiceClient(t, s)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stream, err := client.Subscribe(ctx, connect.NewRequest(&v1.SubscribeConfigRequest{}))
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer stream.Close()

	for stream.Receive() {
		if changed := stream.Msg().GetChanged(); changed != nil {
			if changed.BundleHash != "abc123" {
				t.Errorf("bundle_hash = %q, want %q", changed.BundleHash, "abc123")
			}
			if changed.AtUnixMs != 1234567890 {
				t.Errorf("at_unix_ms = %d, want %d", changed.AtUnixMs, 1234567890)
			}
			return
		}
		// heartbeat before the change event: keep waiting
	}
	if err := stream.Err(); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("stream error: %v", err)
	}
	t.Fatal("stream closed before receiving a Changed event")
}

func newConfigServiceClient(t *testing.T, svc *api.ConfigService) (switchyardv1alpha1connect.ConfigServiceClient, func()) {
	t.Helper()
	mux := http.NewServeMux()
	path, handler := switchyardv1alpha1connect.NewConfigServiceHandler(svc)
	mux.Handle(path, handler)
	srv := httptest.NewUnstartedServer(h2c.NewHandler(mux, &http2.Server{}))
	srv.Start()
	return switchyardv1alpha1connect.NewConfigServiceClient(srv.Client(), srv.URL, connect.WithGRPC()), srv.Close
}
