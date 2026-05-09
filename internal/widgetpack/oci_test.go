package widgetpack

import "testing"

func TestParseRef(t *testing.T) {
	tests := []struct {
		in   string
		repo string
		tag  string
		err  bool
	}{
		{"ghcr.io/foo/bar:1.0.0", "ghcr.io/foo/bar", "1.0.0", false},
		{"localhost:5000/foo:latest", "localhost:5000/foo", "latest", false},
		{"foo", "", "", true},
		{"foo:", "", "", true},
		{":tag", "", "", true},
	}
	for _, tt := range tests {
		repo, tag, err := parseRef(tt.in)
		if (err != nil) != tt.err {
			t.Errorf("parseRef(%q): err = %v, want err = %v", tt.in, err, tt.err)
			continue
		}
		if !tt.err && (repo != tt.repo || tag != tt.tag) {
			t.Errorf("parseRef(%q): repo=%q tag=%q, want %q %q", tt.in, repo, tag, tt.repo, tt.tag)
		}
	}
}

func TestCosignSigTagFor(t *testing.T) {
	if got := cosignSigTagFor("sha256:abc123"); got != "sha256-abc123.sig" {
		t.Errorf("got %q", got)
	}
}

func TestSingleLayerDescriptor_TwoLayers(t *testing.T) {
	manifest := []byte(`{"layers":[{"mediaType":"a"},{"mediaType":"b"}]}`)
	if _, err := singleLayerDescriptor(manifest); err == nil {
		t.Error("expected error for multi-layer manifest")
	}
}

func TestSingleLayerDescriptor_ZeroLayers(t *testing.T) {
	manifest := []byte(`{"layers":[]}`)
	if _, err := singleLayerDescriptor(manifest); err == nil {
		t.Error("expected error for zero-layer manifest")
	}
}
