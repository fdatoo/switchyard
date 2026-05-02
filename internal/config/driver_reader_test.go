package config

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDriverModuleReader_Scheme(t *testing.T) {
	r := &driverModuleReader{root: "/tmp/whatever"}
	if got := r.Scheme(); got != "driver" {
		t.Fatalf("Scheme() = %q, want %q", got, "driver")
	}
}

func TestDriverModuleReader_ReadValidManifest(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "hue"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := "extends \"switchyard:driver\"\nconst name = \"hue\"\nconst version = \"1.0\"\nproduces = new { \"light\" }\n"
	if err := os.WriteFile(filepath.Join(root, "hue", "manifest.pkl"), []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &driverModuleReader{root: root}
	got, err := r.Read(url.URL{Scheme: "driver", Opaque: "hue"})
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got != want {
		t.Fatalf("Read() = %q, want %q", got, want)
	}
}

func TestDriverModuleReader_ReadMissingManifest(t *testing.T) {
	r := &driverModuleReader{root: t.TempDir()}
	_, err := r.Read(url.URL{Scheme: "driver", Opaque: "ghost"})
	if err == nil {
		t.Fatal("expected error for missing manifest, got nil")
	}
	if !strings.Contains(err.Error(), "ghost") || !strings.Contains(err.Error(), "manifest not found") {
		t.Fatalf("error = %q; want it to mention driver name and 'manifest not found'", err.Error())
	}
}

func TestDriverModuleReader_ReadRejectsInvalidNames(t *testing.T) {
	root := t.TempDir()
	r := &driverModuleReader{root: root}
	bad := []string{
		"",
		"../etc/passwd",
		"a/b",
		"UPPERCASE",
		".hidden",
		"with space",
		strings.Repeat("a", 65),
	}
	for _, name := range bad {
		_, err := r.Read(url.URL{Scheme: "driver", Opaque: name})
		if err == nil {
			t.Errorf("Read(driver:%q) should have failed", name)
		}
	}
}

func TestValidDriverName_Allowed(t *testing.T) {
	good := []string{"hue", "z2m", "zigbee2mqtt", "test-driver", "test_driver", "a", "ab12_3-4"}
	for _, name := range good {
		if !validDriverName(name) {
			t.Errorf("validDriverName(%q) = false, want true", name)
		}
	}
}
