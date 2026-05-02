package api

import (
	"context"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
)

type ZoneService struct{ be ZoneReader }

func NewZoneService(be ZoneReader) *ZoneService { return &ZoneService{be: be} }

var _ switchyardv1alpha1connect.ZoneServiceHandler = (*ZoneService)(nil)

func (s *ZoneService) List(ctx context.Context, req *connect.Request[v1.ListZonesRequest]) (*connect.Response[v1.ListZonesResponse], error) {
	var tok string
	var sz uint32
	if req.Msg.Page != nil {
		tok = req.Msg.Page.PageToken
		sz = req.Msg.Page.PageSize
	}
	cur, err := DecodeCursor(tok)
	if err != nil {
		return nil, ToConnect(ctx, ErrValidationFailed, "bad_page_token")
	}
	pr := PageReq{Size: ClampPageSize(sz), Cursor: cur}
	zones, next, err := s.be.ListZones(ctx, pr)
	if err != nil {
		return nil, ToConnect(ctx, err, "list_failed")
	}
	out := &v1.ListZonesResponse{Page: &v1.PageResponse{}}
	if tok2, _ := EncodeCursor(next); tok2 != "" {
		out.Page.NextPageToken = tok2
	}
	for _, z := range zones {
		out.Zones = append(out.Zones, &v1.Zone{Id: z.ID, DisplayName: z.DisplayName, AreaIds: z.AreaIDs})
	}
	return connect.NewResponse(out), nil
}

func (s *ZoneService) Get(ctx context.Context, req *connect.Request[v1.GetZoneRequest]) (*connect.Response[v1.GetZoneResponse], error) {
	z, err := s.be.GetZone(ctx, req.Msg.Id)
	if err != nil {
		return nil, ToConnect(ctx, err, "zone_not_found")
	}
	return connect.NewResponse(&v1.GetZoneResponse{Zone: &v1.Zone{Id: z.ID, DisplayName: z.DisplayName, AreaIds: z.AreaIDs}}), nil
}
