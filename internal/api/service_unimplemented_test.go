package api_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/api"
)

func TestSceneService_AllUnimplemented(t *testing.T) {
	s := api.NewSceneService()
	_, err := s.Apply(context.Background(), connect.NewRequest(&v1.ApplySceneRequest{Id: "x"}))
	assertUnimplemented(t, err)
}

func TestDashboardService_AllUnimplemented(t *testing.T) {
	d := api.NewDashboardService()
	_, err := d.Get(context.Background(), connect.NewRequest(&v1.GetDashboardRequest{Slug: "main"}))
	assertUnimplemented(t, err)
}

func assertUnimplemented(t *testing.T, err error) {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeUnimplemented {
		t.Fatalf("err = %v, want Unimplemented", err)
	}
}
