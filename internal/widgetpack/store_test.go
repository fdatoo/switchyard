package widgetpack_test

import (
	"context"
	"testing"

	"github.com/fdatoo/gohome/internal/widgetpack"
)

func TestStore_AddAndGet(t *testing.T) {
	s := widgetpack.NewStore()
	ctx := context.Background()

	err := s.Add(ctx, widgetpack.InstalledPack{
		Name:            "bar-widgets",
		Version:         "1.0.0",
		SHA256:          "abc123",
		SignatureStatus: "unsigned",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, err := s.Get(ctx, "bar-widgets", "1.0.0")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "bar-widgets" {
		t.Errorf("Name = %q, want bar-widgets", got.Name)
	}
}

func TestStore_GetNotFound(t *testing.T) {
	s := widgetpack.NewStore()
	_, err := s.Get(context.Background(), "nope", "1.0.0")
	if err == nil {
		t.Error("expected error for missing pack")
	}
}

func TestStore_List(t *testing.T) {
	s := widgetpack.NewStore()
	ctx := context.Background()
	_ = s.Add(ctx, widgetpack.InstalledPack{Name: "a", Version: "1.0.0"})
	_ = s.Add(ctx, widgetpack.InstalledPack{Name: "b", Version: "2.0.0"})
	packs, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(packs) != 2 {
		t.Errorf("List len = %d, want 2", len(packs))
	}
}

func TestStore_Remove(t *testing.T) {
	s := widgetpack.NewStore()
	ctx := context.Background()
	_ = s.Add(ctx, widgetpack.InstalledPack{Name: "p", Version: "1.0.0"})
	if err := s.Remove(ctx, "p", "1.0.0"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := s.Get(ctx, "p", "1.0.0"); err == nil {
		t.Error("expected not found after remove")
	}
}
