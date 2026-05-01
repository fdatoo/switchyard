package api_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	configv1 "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/api"
)

type fakeConfig struct {
	validateValid bool
	validateErrs  []string
	validateDiff  api.ConfigDiff
	validateHash  string
	validateErr   error

	applyResult api.ConfigApplyResult
	applyErr    error

	reloadDiff   api.ConfigDiff
	reloadCorrID string
	reloadErr    error

	snapshot    *configv1.ConfigSnapshot
	artifactErr error
}

func (f *fakeConfig) Validate(_ context.Context, _ []byte) (bool, []string, api.ConfigDiff, string, error) {
	return f.validateValid, f.validateErrs, f.validateDiff, f.validateHash, f.validateErr
}

func (f *fakeConfig) Apply(_ context.Context, _ []byte, _, _ string, _, _ bool, _ string) (api.ConfigApplyResult, error) {
	return f.applyResult, f.applyErr
}

func (f *fakeConfig) Reload(_ context.Context, _ string) (api.ConfigDiff, string, error) {
	return f.reloadDiff, f.reloadCorrID, f.reloadErr
}

func (f *fakeConfig) CurrentArtifact(_ context.Context) (*configv1.ConfigSnapshot, error) {
	return f.snapshot, f.artifactErr
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
		},
	}
	s := api.NewConfigService(fc)
	resp, err := s.Apply(context.Background(), connect.NewRequest(&v1.ApplyConfigRequest{PklBundle: []byte("pkl"), Message: "test apply"}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !resp.Msg.Applied || resp.Msg.CorrelationId != "corr-1" || resp.Msg.BundleHash != "hash-1" {
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
