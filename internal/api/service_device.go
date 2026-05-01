package api

import (
	"context"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
	"github.com/fdatoo/switchyard/internal/auth"
)

type DeviceService struct {
	r DeviceReader
	w DeviceWriter
}

func NewDeviceService(r DeviceReader, w DeviceWriter) *DeviceService {
	return &DeviceService{r: r, w: w}
}

var _ switchyardv1alpha1connect.DeviceServiceHandler = (*DeviceService)(nil)

func (s *DeviceService) List(ctx context.Context, req *connect.Request[v1.ListDevicesRequest]) (*connect.Response[v1.ListDevicesResponse], error) {
	cur, err := DecodeCursor(pageToken(req.Msg.Page))
	if err != nil {
		return nil, ToConnect(ctx, ErrValidationFailed, "bad_page_token")
	}
	pr := PageReq{Size: ClampPageSize(pageSize(req.Msg.Page)), Cursor: cur}
	devs, next, err := s.r.ListDevices(ctx, req.Msg.AreaId, pr)
	if err != nil {
		return nil, ToConnect(ctx, err, "list_failed")
	}
	out := &v1.ListDevicesResponse{Page: &v1.PageResponse{}}
	tok, _ := EncodeCursor(next)
	out.Page.NextPageToken = tok
	for _, d := range devs {
		out.Devices = append(out.Devices, deviceToProto(d))
	}
	return connect.NewResponse(out), nil
}

func (s *DeviceService) Get(ctx context.Context, req *connect.Request[v1.GetDeviceRequest]) (*connect.Response[v1.GetDeviceResponse], error) {
	d, err := s.r.GetDevice(ctx, req.Msg.Id)
	if err != nil {
		return nil, ToConnect(ctx, err, "device_not_found")
	}
	return connect.NewResponse(&v1.GetDeviceResponse{Device: deviceToProto(d)}), nil
}

func (s *DeviceService) Rename(ctx context.Context, req *connect.Request[v1.RenameDeviceRequest]) (*connect.Response[v1.RenameDeviceResponse], error) {
	if req.Msg.NewFriendlyName == "" {
		return nil, ToConnect(ctx, ErrValidationFailed, "empty_friendly_name")
	}
	d, err := s.w.RenameDevice(ctx, req.Msg.Id, req.Msg.NewFriendlyName, principalID(ctx))
	if err != nil {
		return nil, ToConnect(ctx, err, "rename_failed")
	}
	return connect.NewResponse(&v1.RenameDeviceResponse{Device: deviceToProto(d)}), nil
}

func (s *DeviceService) Reassign(ctx context.Context, req *connect.Request[v1.ReassignDeviceRequest]) (*connect.Response[v1.ReassignDeviceResponse], error) {
	d, err := s.w.ReassignDevice(ctx, req.Msg.Id, req.Msg.NewAreaId, principalID(ctx))
	if err != nil {
		return nil, ToConnect(ctx, err, "reassign_failed")
	}
	return connect.NewResponse(&v1.ReassignDeviceResponse{Device: deviceToProto(d)}), nil
}

func deviceToProto(d Device) *v1.Device {
	return &v1.Device{
		Id:               d.ID,
		FriendlyName:     d.FriendlyName,
		AreaId:           d.AreaID,
		DriverInstanceId: d.DriverInstanceID,
		EntityIds:        d.EntityIDs,
	}
}

func principalID(ctx context.Context) string {
	if p, ok := auth.PrincipalFromContext(ctx); ok {
		return p.ID
	}
	return ""
}
