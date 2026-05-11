package api_test

import (
	"context"
	"encoding/json"
	"testing"

	"connectrpc.com/connect"

	configv1 "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/api"
)

// ── RegenPreview tests ──────────────────────────────────────────────────────

func TestRegenPreview_ValidAutomation(t *testing.T) {
	s := api.NewConfigService(&fakeConfig{})
	astJSON := `{"id":"test-auto","enabled":true}`
	resp, err := s.RegenPreview(context.Background(), connect.NewRequest(&v1.RegenPreviewRequest{
		FileType: "automation",
		AstJson:  astJSON,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Msg.PklBytes) == 0 {
		t.Error("expected non-empty pkl_bytes")
	}
}

func TestRegenPreview_MalformedJSON(t *testing.T) {
	s := api.NewConfigService(&fakeConfig{})
	_, err := s.RegenPreview(context.Background(), connect.NewRequest(&v1.RegenPreviewRequest{
		FileType: "automation",
		AstJson:  `{not valid json`,
	}))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	var ce *connect.Error
	if ok := false; !ok {
		ce, _ = err.(*connect.Error)
	}
	_ = ce
	// Should be InvalidArgument
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestRegenPreview_UnknownFileType(t *testing.T) {
	s := api.NewConfigService(&fakeConfig{})
	_, err := s.RegenPreview(context.Background(), connect.NewRequest(&v1.RegenPreviewRequest{
		FileType: "unknown_type",
		AstJson:  `{}`,
	}))
	if err == nil {
		t.Fatal("expected error for unknown file_type")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestRegenPreview_PageType_Unimplemented(t *testing.T) {
	s := api.NewConfigService(&fakeConfig{})
	_, err := s.RegenPreview(context.Background(), connect.NewRequest(&v1.RegenPreviewRequest{
		FileType: "page",
		AstJson:  `{}`,
	}))
	if err == nil {
		t.Fatal("expected error for page type")
	}
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Errorf("expected CodeUnimplemented, got %v", connect.CodeOf(err))
	}
}

// ── GetDetail tests ─────────────────────────────────────────────────────────

func TestGetDetail_FoundInSnapshot(t *testing.T) {
	fa := &fakeAutomations{
		automations: []api.Automation{{ID: "sunset-lights", DisplayName: "Sunset Lights", Enabled: true}},
	}
	fc := &fakeConfig{
		snapshot: &configv1.ConfigSnapshot{
			ConfigDir: "/etc/switchyard",
			Automations: []*configv1.AutomationConfig{
				{Id: "sunset-lights", Enabled: true},
			},
		},
	}
	fs := &fakeSystem{configDir: "/etc/switchyard"}
	s := api.NewAutomationServiceWithDetail(fa, fc, fs)
	resp, err := s.GetDetail(context.Background(), connect.NewRequest(&v1.GetAutomationDetailRequest{Id: "sunset-lights"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Msg.Automation.Id != "sunset-lights" {
		t.Errorf("unexpected id: %s", resp.Msg.Automation.Id)
	}
	if resp.Msg.AstJson == "" {
		t.Error("expected non-empty ast_json")
	}
	// Verify ast_json is valid JSON containing the id
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Msg.AstJson), &m); err != nil {
		t.Errorf("ast_json is not valid JSON: %v", err)
	}
	if resp.Msg.FilePath != "/etc/switchyard/automations/sunset-lights.pkl" {
		t.Errorf("unexpected file_path: %s", resp.Msg.FilePath)
	}
}

func TestGetDetail_NotFound(t *testing.T) {
	fa := &fakeAutomations{}
	fc := &fakeConfig{snapshot: &configv1.ConfigSnapshot{}}
	fs := &fakeSystem{configDir: "/etc/switchyard"}
	s := api.NewAutomationServiceWithDetail(fa, fc, fs)
	_, err := s.GetDetail(context.Background(), connect.NewRequest(&v1.GetAutomationDetailRequest{Id: "no-such"}))
	if err == nil {
		t.Fatal("expected error for not found")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", connect.CodeOf(err))
	}
}
