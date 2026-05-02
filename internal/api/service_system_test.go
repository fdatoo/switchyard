package api_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	systemv1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/api"
	"github.com/fdatoo/switchyard/internal/auth"
)

func TestSystemService_Version(t *testing.T) {
	s := api.NewSystemService(&fakeSystem{
		version: api.VersionInfo{BinaryVersion: "0.1.0", GitCommit: "abc"},
	})
	resp, err := s.Version(context.Background(), connect.NewRequest(&systemv1.VersionRequest{}))
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if resp.Msg.BinaryVersion != "0.1.0" || resp.Msg.GitCommit != "abc" {
		t.Errorf("got %+v", resp.Msg)
	}
}

func TestSystemService_Health_OK(t *testing.T) {
	s := api.NewSystemService(&fakeSystem{healthy: true, subs: []api.SubsystemHealth{{Name: "eventstore", OK: true}}})
	resp, err := s.Health(context.Background(), connect.NewRequest(&systemv1.HealthRequest{}))
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if !resp.Msg.Ok {
		t.Error("ok=false, want true")
	}
	if len(resp.Msg.Subsystems) != 1 || resp.Msg.Subsystems[0].Name != "eventstore" {
		t.Errorf("subs = %+v", resp.Msg.Subsystems)
	}
}

func TestSystemService_CreateSnapshot(t *testing.T) {
	fs := &fakeSystem{}
	s := api.NewSystemService(fs)
	resp, err := s.CreateSnapshot(context.Background(),
		connect.NewRequest(&systemv1.CreateSnapshotRequest{Owner: "state_cache", Reason: "manual"}))
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	if resp.Msg.Cursor != 1234 {
		t.Errorf("cursor = %d", resp.Msg.Cursor)
	}
	if fs.lastOwner != "state_cache" || fs.lastReason != "manual" {
		t.Errorf("backend not called: owner=%q reason=%q", fs.lastOwner, fs.lastReason)
	}
}

func TestSystemService_GetConfigDir(t *testing.T) {
	s := api.NewSystemService(&fakeSystem{configDir: "/etc/switchyard"})
	resp, err := s.GetConfigDir(context.Background(), connect.NewRequest(&systemv1.GetConfigDirRequest{}))
	if err != nil {
		t.Fatalf("GetConfigDir: %v", err)
	}
	if resp.Msg.ConfigDir != "/etc/switchyard" {
		t.Errorf("got %q, want /etc/switchyard", resp.Msg.ConfigDir)
	}
}

func TestSystemService_GetMCPConfig(t *testing.T) {
	s := api.NewSystemService(&fakeSystem{mcpCfg: api.MCPConfig{EvalResultMaxBytes: 65536, TailMaxWaitSeconds: 60}})
	resp, err := s.GetMCPConfig(context.Background(), connect.NewRequest(&systemv1.GetMCPConfigRequest{}))
	if err != nil {
		t.Fatalf("GetMCPConfig: %v", err)
	}
	if resp.Msg.EvalResultMaxBytes != 65536 {
		t.Errorf("EvalResultMaxBytes = %d, want 65536", resp.Msg.EvalResultMaxBytes)
	}
	if resp.Msg.TailMaxWaitSeconds != 60 {
		t.Errorf("TailMaxWaitSeconds = %d, want 60", resp.Msg.TailMaxWaitSeconds)
	}
}

func TestSystemService_RecordConfigFileEdit(t *testing.T) {
	fake := &fakeSystem{configDir: "/etc/switchyard", recordResult: 42}
	s := api.NewSystemService(fake)
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{ID: "system:local"})
	resp, err := s.RecordConfigFileEdit(ctx, connect.NewRequest(&systemv1.RecordConfigFileEditRequest{
		Path: "automations/lights.pkl", Sha256Hex: "abc", SizeBytes: 100,
	}))
	if err != nil {
		t.Fatalf("RecordConfigFileEdit: %v", err)
	}
	if resp.Msg.EventCursor != 42 {
		t.Errorf("EventCursor = %d, want 42", resp.Msg.EventCursor)
	}
	if fake.lastRecord.path != "automations/lights.pkl" {
		t.Errorf("path = %q, want automations/lights.pkl", fake.lastRecord.path)
	}
}

func TestSystemService_RecordConfigFileEdit_NoAuth(t *testing.T) {
	s := api.NewSystemService(&fakeSystem{})
	_, err := s.RecordConfigFileEdit(context.Background(), connect.NewRequest(&systemv1.RecordConfigFileEditRequest{Path: "x.pkl"}))
	if err == nil {
		t.Fatal("expected error for unauthenticated request")
	}
}
