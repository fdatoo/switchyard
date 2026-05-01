package api_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/api"
)

type fakeEntities struct{ entities []api.Entity }

func (f *fakeEntities) ListEntities(_ context.Context, sel api.EntitySelector, _ api.PageReq) ([]api.Entity, api.Cursor, error) {
	var out []api.Entity
	for _, e := range f.entities {
		if len(sel.Areas) > 0 && !contains(sel.Areas, e.AreaID) {
			continue
		}
		out = append(out, e)
	}
	return out, api.Cursor{}, nil
}
func (f *fakeEntities) GetEntity(_ context.Context, id string) (api.Entity, error) {
	for _, e := range f.entities {
		if e.ID == id {
			return e, nil
		}
	}
	return api.Entity{}, api.ErrEntityNotFound
}

type fakeCaller struct {
	called    []callRec
	returnErr error
}
type callRec struct{ id, cap string }

func (f *fakeCaller) Call(_ context.Context, id, cap string, _ map[string]any) (api.CapabilityCallResult, error) {
	if f.returnErr != nil {
		return api.CapabilityCallResult{}, f.returnErr
	}
	f.called = append(f.called, callRec{id, cap})
	return api.CapabilityCallResult{CorrelationID: "cmd-" + id, Success: true}, nil
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func TestEntityService_List_AreaFilter(t *testing.T) {
	s := api.NewEntityService(&fakeEntities{entities: []api.Entity{
		{ID: "light.a", AreaID: "kitchen"},
		{ID: "light.b", AreaID: "bedroom"},
	}}, nil)
	resp, err := s.List(context.Background(), connect.NewRequest(&v1.ListEntitiesRequest{
		Selector: &v1.EntitySelector{Areas: []string{"kitchen"}},
	}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Msg.Entities) != 1 || resp.Msg.Entities[0].Id != "light.a" {
		t.Errorf("got %+v", resp.Msg.Entities)
	}
}

func TestEntityService_CallCapability(t *testing.T) {
	fc := &fakeCaller{}
	s := api.NewEntityService(&fakeEntities{entities: []api.Entity{{ID: "light.a"}}}, fc)
	params, _ := structpb.NewStruct(map[string]any{"brightness": 75})
	resp, err := s.CallCapability(context.Background(), connect.NewRequest(&v1.CallCapabilityRequest{
		EntityId: "light.a", Capability: "set_brightness", Parameters: params,
	}))
	if err != nil {
		t.Fatalf("CallCapability: %v", err)
	}
	if resp.Msg.CorrelationId != "cmd-light.a" {
		t.Errorf("correlation = %q", resp.Msg.CorrelationId)
	}
}

func TestEntityService_CallCapability_DriverDown(t *testing.T) {
	fc := &fakeCaller{returnErr: api.ErrDriverUnavailable}
	s := api.NewEntityService(&fakeEntities{}, fc)
	_, err := s.CallCapability(context.Background(), connect.NewRequest(&v1.CallCapabilityRequest{
		EntityId: "light.a", Capability: "turn_on",
	}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeUnavailable {
		t.Fatalf("err = %v", err)
	}
}
