package api_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/api"
)

type fakeDevices struct {
	devices  []api.Device
	renamed  []renameRecord
	reassign []reassignRecord
}
type renameRecord struct{ id, name, actor string }
type reassignRecord struct{ id, area, actor string }

func (f *fakeDevices) ListDevices(_ context.Context, area string, _ api.PageReq) ([]api.Device, api.Cursor, error) {
	if area == "" {
		return f.devices, api.Cursor{}, nil
	}
	var out []api.Device
	for _, d := range f.devices {
		if d.AreaID == area {
			out = append(out, d)
		}
	}
	return out, api.Cursor{}, nil
}
func (f *fakeDevices) GetDevice(_ context.Context, id string) (api.Device, error) {
	for _, d := range f.devices {
		if d.ID == id {
			return d, nil
		}
	}
	return api.Device{}, api.ErrDeviceNotFound
}
func (f *fakeDevices) RenameDevice(_ context.Context, id, name, actor string) (api.Device, error) {
	for i, d := range f.devices {
		if d.ID == id {
			f.devices[i].FriendlyName = name
			f.renamed = append(f.renamed, renameRecord{id, name, actor})
			return f.devices[i], nil
		}
	}
	return api.Device{}, api.ErrDeviceNotFound
}
func (f *fakeDevices) ReassignDevice(_ context.Context, id, area, actor string) (api.Device, error) {
	for i, d := range f.devices {
		if d.ID == id {
			f.devices[i].AreaID = area
			f.reassign = append(f.reassign, reassignRecord{id, area, actor})
			return f.devices[i], nil
		}
	}
	return api.Device{}, api.ErrDeviceNotFound
}

func TestDeviceService_List_FilterArea(t *testing.T) {
	s := api.NewDeviceService(&fakeDevices{devices: []api.Device{{ID: "a", AreaID: "kitchen"}, {ID: "b", AreaID: "bedroom"}}}, nil)
	resp, err := s.List(context.Background(), connect.NewRequest(&v1.ListDevicesRequest{AreaId: "kitchen"}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Msg.Devices) != 1 || resp.Msg.Devices[0].Id != "a" {
		t.Errorf("got %+v", resp.Msg.Devices)
	}
}

func TestDeviceService_Get_NotFound(t *testing.T) {
	s := api.NewDeviceService(&fakeDevices{}, nil)
	_, err := s.Get(context.Background(), connect.NewRequest(&v1.GetDeviceRequest{Id: "nope"}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeNotFound {
		t.Fatalf("err = %v", err)
	}
}

func TestDeviceService_Rename(t *testing.T) {
	fd := &fakeDevices{devices: []api.Device{{ID: "a", FriendlyName: "old"}}}
	s := api.NewDeviceService(fd, fd)
	resp, err := s.Rename(context.Background(), connect.NewRequest(&v1.RenameDeviceRequest{Id: "a", NewFriendlyName: "new"}))
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if resp.Msg.Device.FriendlyName != "new" {
		t.Errorf("name = %q", resp.Msg.Device.FriendlyName)
	}
	if len(fd.renamed) != 1 || fd.renamed[0].name != "new" {
		t.Errorf("rename record = %+v", fd.renamed)
	}
}

func TestDeviceService_Rename_NotFound(t *testing.T) {
	fd := &fakeDevices{}
	s := api.NewDeviceService(fd, fd)
	_, err := s.Rename(context.Background(), connect.NewRequest(&v1.RenameDeviceRequest{Id: "nope", NewFriendlyName: "x"}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeNotFound {
		t.Fatalf("err = %v", err)
	}
}

func TestDeviceService_Reassign(t *testing.T) {
	fd := &fakeDevices{devices: []api.Device{{ID: "a", AreaID: "kitchen"}}}
	s := api.NewDeviceService(fd, fd)
	resp, err := s.Reassign(context.Background(), connect.NewRequest(&v1.ReassignDeviceRequest{Id: "a", NewAreaId: "bedroom"}))
	if err != nil {
		t.Fatalf("Reassign: %v", err)
	}
	if resp.Msg.Device.AreaId != "bedroom" {
		t.Errorf("area = %q", resp.Msg.Device.AreaId)
	}
	if len(fd.reassign) != 1 || fd.reassign[0].area != "bedroom" {
		t.Errorf("reassign record = %+v", fd.reassign)
	}
}
