package api

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/gen/gohome/v1alpha1/gohomev1alpha1connect"
)

type DriverService struct{ be DriverControl }

func NewDriverService(be DriverControl) *DriverService { return &DriverService{be: be} }

var _ gohomev1alpha1connect.DriverServiceHandler = (*DriverService)(nil)

func (s *DriverService) ListDrivers(ctx context.Context, req *connect.Request[v1.ListDriversRequest]) (*connect.Response[v1.ListDriversResponse], error) {
	cur, err := DecodeCursor(pageToken(req.Msg.Page))
	if err != nil {
		return nil, ToConnect(ctx, ErrValidationFailed, "bad_page_token")
	}
	drivers, next, err := s.be.ListDrivers(ctx, PageReq{Size: ClampPageSize(pageSize(req.Msg.Page)), Cursor: cur})
	if err != nil {
		return nil, ToConnect(ctx, err, "list_failed")
	}
	out := &v1.ListDriversResponse{Page: &v1.PageResponse{}}
	if tok, _ := EncodeCursor(next); tok != "" {
		out.Page.NextPageToken = tok
	}
	for _, d := range drivers {
		out.Drivers = append(out.Drivers, &v1.Driver{
			Name:          d.Name,
			Version:       d.Version,
			Description:   d.Description,
			EntityClasses: d.EntityClasses,
		})
	}
	return connect.NewResponse(out), nil
}

func (s *DriverService) ListInstances(ctx context.Context, req *connect.Request[v1.ListInstancesRequest]) (*connect.Response[v1.ListInstancesResponse], error) {
	cur, err := DecodeCursor(pageToken(req.Msg.Page))
	if err != nil {
		return nil, ToConnect(ctx, ErrValidationFailed, "bad_page_token")
	}
	insts, next, err := s.be.ListInstances(ctx, PageReq{Size: ClampPageSize(pageSize(req.Msg.Page)), Cursor: cur})
	if err != nil {
		return nil, ToConnect(ctx, err, "list_failed")
	}
	out := &v1.ListInstancesResponse{Page: &v1.PageResponse{}}
	if tok, _ := EncodeCursor(next); tok != "" {
		out.Page.NextPageToken = tok
	}
	for _, inst := range insts {
		var ts *timestamppb.Timestamp
		if inst.LastHandshakeUnixMs > 0 {
			ts = timestamppb.New(time.UnixMilli(inst.LastHandshakeUnixMs).UTC())
		}
		out.Instances = append(out.Instances, &v1.DriverInstance{
			Id:            inst.ID,
			DriverName:    inst.DriverName,
			Status:        inst.Status,
			EntityCount:   inst.EntityCount,
			LastHandshake: ts,
		})
	}
	return connect.NewResponse(out), nil
}

func (s *DriverService) InstanceHealth(ctx context.Context, req *connect.Request[v1.InstanceHealthRequest]) (*connect.Response[v1.InstanceHealthResponse], error) {
	ok, detail, err := s.be.InstanceHealth(ctx, req.Msg.InstanceId)
	if err != nil {
		return nil, ToConnect(ctx, err, "health_failed")
	}
	return connect.NewResponse(&v1.InstanceHealthResponse{Ok: ok, Detail: detail}), nil
}

func (s *DriverService) RestartInstance(ctx context.Context, req *connect.Request[v1.RestartInstanceRequest]) (*connect.Response[v1.RestartInstanceResponse], error) {
	if err := s.be.RestartInstance(ctx, req.Msg.InstanceId, req.Msg.Reason, principalID(ctx)); err != nil {
		return nil, ToConnect(ctx, err, "restart_failed")
	}
	return connect.NewResponse(&v1.RestartInstanceResponse{Restarted: true}), nil
}
