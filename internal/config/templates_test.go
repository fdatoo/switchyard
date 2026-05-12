//go:build integration

package config

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apple/pkl-go/pkl"
)

// evalPklText writes content to a temp .pkl file and returns the
// evaluator's serialized output. Used to verify the singular template
// modules accept amend-style usage.
func evalPklText(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pkl")
	writePkl(t, dir, "test.pkl", content)

	ev, err := newPklEvaluator(context.Background(), "")
	if err != nil {
		t.Fatalf("evaluator: %v", err)
	}
	defer func() { _ = ev.ev.Close() }()

	text, err := ev.ev.EvaluateOutputText(context.Background(), pkl.FileSource(path))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	return text
}

func TestTemplate_AutomationAmends(t *testing.T) {
	out := evalPklText(t, `
amends "switchyard:automation"
import "switchyard:automations" as auto

id = "test-auto"
enabled = true
triggers {
  new auto.EventTrigger { kind = "sun.sunset" }
}
actions {
  new auto.CallServiceAction {
    entity = "light.x"
    capability = "turn_on"
  }
}
`)
	if !strings.Contains(out, `"test-auto"`) {
		t.Errorf("missing id in output: %s", out)
	}
	if !strings.Contains(out, `"sun.sunset"`) {
		t.Errorf("missing trigger kind: %s", out)
	}
}

func TestTemplate_AreaAmends(t *testing.T) {
	out := evalPklText(t, `
amends "switchyard:area"

id = "kitchen"
displayName = "Kitchen"
`)
	if !strings.Contains(out, `"kitchen"`) {
		t.Errorf("missing id: %s", out)
	}
	if !strings.Contains(out, `"Kitchen"`) {
		t.Errorf("missing displayName: %s", out)
	}
}

func TestTemplate_SceneAmends(t *testing.T) {
	out := evalPklText(t, `
amends "switchyard:scene"
import "switchyard:automations" as auto

id = "movie-night"
displayName = "Movie Night"
actions {
  new auto.CallServiceAction {
    entity = "light.x"
    capability = "turn_off"
  }
}
`)
	if !strings.Contains(out, `"movie-night"`) {
		t.Errorf("missing id: %s", out)
	}
}

func TestTemplate_EntityAreasAmends(t *testing.T) {
	out := evalPklText(t, `
amends "switchyard:entity-areas"

entityAreas {
  ["light.living_room"] = "living-room"
  ["sensor.kitchen"] = "kitchen"
}
`)
	if !strings.Contains(out, `"light.living_room"`) || !strings.Contains(out, `"living-room"`) {
		t.Errorf("missing mapping: %s", out)
	}
}
