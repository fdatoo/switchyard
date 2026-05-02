package api_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/api"
)

type fakeAreas struct{ areas []api.Area }

func (f *fakeAreas) ListAreas(_ context.Context, _ api.PageReq) ([]api.Area, api.Cursor, error) {
	return f.areas, api.Cursor{}, nil
}
func (f *fakeAreas) GetArea(_ context.Context, id string) (api.Area, error) {
	for _, a := range f.areas {
		if a.ID == id {
			return a, nil
		}
	}
	return api.Area{}, api.ErrAreaNotFound
}

func TestAreaService_List(t *testing.T) {
	s := api.NewAreaService(&fakeAreas{areas: []api.Area{{ID: "kitchen"}, {ID: "bedroom"}}})
	resp, err := s.List(context.Background(), connect.NewRequest(&v1.ListAreasRequest{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Msg.Areas) != 2 {
		t.Errorf("len = %d", len(resp.Msg.Areas))
	}
}

func TestAreaService_Get_NotFound(t *testing.T) {
	s := api.NewAreaService(&fakeAreas{})
	_, err := s.Get(context.Background(), connect.NewRequest(&v1.GetAreaRequest{Id: "nope"}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeNotFound {
		t.Fatalf("err = %v", err)
	}
}
