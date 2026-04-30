package api_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/internal/api"
)

type fakeZones struct{ zones []api.Zone }

func (f *fakeZones) ListZones(_ context.Context, _ api.PageReq) ([]api.Zone, api.Cursor, error) {
	return f.zones, api.Cursor{}, nil
}
func (f *fakeZones) GetZone(_ context.Context, id string) (api.Zone, error) {
	for _, z := range f.zones {
		if z.ID == id {
			return z, nil
		}
	}
	return api.Zone{}, api.ErrZoneNotFound
}

func TestZoneService_List(t *testing.T) {
	s := api.NewZoneService(&fakeZones{zones: []api.Zone{{ID: "downstairs"}}})
	resp, err := s.List(context.Background(), connect.NewRequest(&v1.ListZonesRequest{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Msg.Zones) != 1 {
		t.Errorf("len = %d", len(resp.Msg.Zones))
	}
}
