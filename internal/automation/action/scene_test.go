package action_test

import (
	"context"
	"testing"

	"github.com/fdatoo/gohome/internal/automation/action"
)

type fakeSceneApplier struct {
	applied []string
	err     error
}

func (f *fakeSceneApplier) Apply(_ context.Context, slug, _ string) error {
	f.applied = append(f.applied, slug)
	return f.err
}

func TestScene_Calls(t *testing.T) {
	f := &fakeSceneApplier{}
	a := &action.SceneAction{Slug: "movie"}
	if err := a.Execute(context.Background(), &action.Run{Scenes: f}); err != nil {
		t.Fatal(err)
	}
	if len(f.applied) != 1 || f.applied[0] != "movie" {
		t.Fatalf("got %v", f.applied)
	}
}
