package widgetpack_test

import (
	"context"
	"testing"

	"github.com/fdatoo/switchyard/internal/widgetpack"
)

func TestInstaller_Install(t *testing.T) {
	store := widgetpack.NewStore()
	installer := widgetpack.NewInstaller(store)

	pack, err := installer.Install(context.Background(), widgetpack.InstallRequest{
		Name:    "test-widgets",
		Version: "1.0.0",
		Ref:     "registry.example.com/test-widgets:1.0.0",
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if pack.Name != "test-widgets" {
		t.Errorf("Name = %q, want test-widgets", pack.Name)
	}
}

func TestInstaller_Install_MissingName(t *testing.T) {
	store := widgetpack.NewStore()
	installer := widgetpack.NewInstaller(store)
	_, err := installer.Install(context.Background(), widgetpack.InstallRequest{Ref: "ref"})
	if err == nil {
		t.Error("expected error for missing name/version")
	}
}

func TestInstaller_Install_MissingVersion(t *testing.T) {
	store := widgetpack.NewStore()
	installer := widgetpack.NewInstaller(store)
	_, err := installer.Install(context.Background(), widgetpack.InstallRequest{
		Name: "my-widgets",
		Ref:  "registry.example.com/my-widgets:latest",
	})
	if err == nil {
		t.Error("expected error for missing version")
	}
}
