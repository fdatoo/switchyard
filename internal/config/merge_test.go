package config

import (
	"strings"
	"testing"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
)

func TestMergeDiscovered_AppendsNonOverlapping(t *testing.T) {
	snap := &configpb.ConfigSnapshot{
		Automations: []*configpb.AutomationConfig{{Id: "inline-one"}},
	}
	disc := discoveryResult{
		Automations: []discoveredAutomation{
			{Path: "automations/disk-one.pkl", Config: &configpb.AutomationConfig{Id: "disk-one"}},
		},
	}
	merged, errs, err := mergeDiscovered(snap, disc)
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if len(errs) != 0 {
		t.Errorf("unexpected soft errors: %+v", errs)
	}
	if len(merged.Automations) != 2 {
		t.Fatalf("want 2 automations, got %d", len(merged.Automations))
	}
	if merged.Automations[0].GetId() != "inline-one" || merged.Automations[1].GetId() != "disk-one" {
		t.Errorf("order wrong: %+v", merged.Automations)
	}
}

func TestMergeDiscovered_DuplicateIdIsHardError(t *testing.T) {
	snap := &configpb.ConfigSnapshot{
		Automations: []*configpb.AutomationConfig{{Id: "dup"}},
	}
	disc := discoveryResult{
		Automations: []discoveredAutomation{
			{Path: "automations/dup.pkl", Config: &configpb.AutomationConfig{Id: "dup"}},
		},
	}
	_, errs, err := mergeDiscovered(snap, disc)
	if err == nil {
		t.Fatal("expected hard error for duplicate id")
	}
	if len(errs) != 1 || errs[0].Code != "duplicate_id" {
		t.Errorf("want one duplicate_id soft error too, got %+v", errs)
	}
	if !strings.Contains(errs[0].Message, "dup") {
		t.Errorf("message should mention 'dup': %s", errs[0].Message)
	}
}

func TestMergeDiscovered_FilenameMismatchIsSoftDrop(t *testing.T) {
	snap := &configpb.ConfigSnapshot{}
	disc := discoveryResult{
		Automations: []discoveredAutomation{
			{Path: "automations/expected-name.pkl", Config: &configpb.AutomationConfig{Id: "actual-id"}},
		},
	}
	merged, errs, err := mergeDiscovered(snap, disc)
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if len(merged.Automations) != 0 {
		t.Errorf("want file dropped, got %d", len(merged.Automations))
	}
	if len(errs) != 1 || errs[0].Code != "filename_id_mismatch" {
		t.Errorf("want filename_id_mismatch, got %+v", errs)
	}
}

func TestMergeDiscovered_EntityAreasDuplicateKeyIsHardError(t *testing.T) {
	snap := &configpb.ConfigSnapshot{
		EntityAreas: map[string]string{"light.x": "living-room"},
	}
	disc := discoveryResult{
		EntityAreas: map[string]string{"light.x": "kitchen"},
	}
	_, errs, err := mergeDiscovered(snap, disc)
	if err == nil {
		t.Fatal("expected hard error for duplicate entity-area key")
	}
	if len(errs) != 1 || errs[0].Code != "duplicate_entity_area" {
		t.Errorf("want duplicate_entity_area, got %+v", errs)
	}
}

func TestMergeDiscovered_AreaAndSceneSamePath(t *testing.T) {
	snap := &configpb.ConfigSnapshot{
		Areas:  []*configpb.AreaConfig{{Id: "inline-area"}},
		Scenes: []*configpb.SceneConfig{{Id: "inline-scene"}},
	}
	disc := discoveryResult{
		Areas:  []discoveredArea{{Path: "areas/disk-area.pkl", Config: &configpb.AreaConfig{Id: "disk-area"}}},
		Scenes: []discoveredScene{{Path: "scenes/disk-scene.pkl", Config: &configpb.SceneConfig{Id: "disk-scene"}}},
	}
	merged, errs, err := mergeDiscovered(snap, disc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(errs) != 0 {
		t.Errorf("soft errs: %+v", errs)
	}
	if len(merged.Areas) != 2 || len(merged.Scenes) != 2 {
		t.Errorf("got %d areas %d scenes", len(merged.Areas), len(merged.Scenes))
	}
}
