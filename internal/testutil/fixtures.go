package testutil

import (
	"bufio"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/eventstore"
)

var updateGolden = flag.Bool("update", false, "rewrite golden files")

// LoadFixture reads testdata/fixtures/<name>.jsonl — one JSON object per line.
// Returns events with Position=0 (store assigns positions on Append).
func LoadFixture(t *testing.T, name string) []eventstore.Event {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "fixtures", name+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		path = filepath.Join("testdata", "fixtures", name+".jsonl")
		f, err = os.Open(path)
		if err != nil {
			t.Fatalf("open fixture %s: %v", name, err)
		}
	}
	defer f.Close() //nolint:errcheck

	var out []eventstore.Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		var rec struct {
			Kind    string          `json:"kind"`
			Entity  string          `json:"entity"`
			Source  string          `json:"source"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatalf("unmarshal fixture line: %v — %s", err, string(line))
		}
		payload := &eventv1.Payload{}
		if err := protojson.Unmarshal(rec.Payload, payload); err != nil {
			t.Fatalf("unmarshal payload: %v — %s", err, string(rec.Payload))
		}
		out = append(out, eventstore.Event{
			Kind:    rec.Kind,
			Entity:  rec.Entity,
			Source:  rec.Source,
			Payload: payload,
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan fixture %s: %v", name, err)
	}
	return out
}

// AssertGolden compares got with testdata/fixtures/<name>.golden.json.
// Pass -update to regenerate.
func AssertGolden(t *testing.T, name string, got map[string]any) {
	t.Helper()
	// Mirror the two-path fallback from LoadFixture so callers at any depth work.
	path := filepath.Join("..", "..", "testdata", "fixtures", name+".golden.json")
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		path = filepath.Join("testdata", "fixtures", name+".golden.json")
	}

	raw, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if *updateGolden {
		if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create)", path, err)
	}
	if string(expected) != string(raw)+"\n" {
		t.Fatalf("golden mismatch for %s\n--- expected:\n%s\n--- got:\n%s",
			name, expected, raw)
	}
}
