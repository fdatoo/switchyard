package api_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/internal/api"
)

type fakeDrivers struct {
	drivers   []api.Driver
	instances []api.DriverInstance
	healthOK  bool
	healthErr error
	restarts  []string
}

func (f *fakeDrivers) ListDrivers(_ context.Context, _ api.PageReq) ([]api.Driver, api.Cursor, error) {
	return f.drivers, api.Cursor{}, nil
}
func (f *fakeDrivers) ListInstances(_ context.Context, _ api.PageReq) ([]api.DriverInstance, api.Cursor, error) {
	return f.instances, api.Cursor{}, nil
}
func (f *fakeDrivers) InstanceHealth(_ context.Context, _ string) (bool, string, error) {
	if f.healthErr != nil {
		return false, "", f.healthErr
	}
	return f.healthOK, "", nil
}
func (f *fakeDrivers) RestartInstance(_ context.Context, id, _, _ string) error {
	for _, in := range f.instances {
		if in.ID == id {
			f.restarts = append(f.restarts, id)
			return nil
		}
	}
	return api.ErrInstanceNotFound
}

func TestDriverService_RestartInstance(t *testing.T) {
	fd := &fakeDrivers{instances: []api.DriverInstance{{ID: "hue-1"}}}
	s := api.NewDriverService(fd)
	resp, err := s.RestartInstance(context.Background(), connect.NewRequest(&v1.RestartInstanceRequest{InstanceId: "hue-1", Reason: "manual"}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !resp.Msg.Restarted || len(fd.restarts) != 1 {
		t.Errorf("not restarted")
	}
}

func TestDriverService_RestartInstance_NotFound(t *testing.T) {
	fd := &fakeDrivers{}
	s := api.NewDriverService(fd)
	_, err := s.RestartInstance(context.Background(), connect.NewRequest(&v1.RestartInstanceRequest{InstanceId: "nope"}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeNotFound {
		t.Fatalf("expected CodeNotFound, got: %v", err)
	}
}

func TestDriverService_ListDrivers(t *testing.T) {
	fd := &fakeDrivers{drivers: []api.Driver{
		{Name: "hue", Version: "1.0", Description: "Philips Hue", EntityClasses: []string{"light"}},
	}}
	s := api.NewDriverService(fd)
	resp, err := s.ListDrivers(context.Background(), connect.NewRequest(&v1.ListDriversRequest{}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(resp.Msg.Drivers) != 1 || resp.Msg.Drivers[0].Name != "hue" {
		t.Errorf("unexpected drivers: %+v", resp.Msg.Drivers)
	}
}

func TestDriverService_InstanceHealth(t *testing.T) {
	fd := &fakeDrivers{healthOK: true}
	s := api.NewDriverService(fd)
	resp, err := s.InstanceHealth(context.Background(), connect.NewRequest(&v1.InstanceHealthRequest{InstanceId: "hue-1"}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !resp.Msg.Ok {
		t.Error("expected ok=true")
	}
}
