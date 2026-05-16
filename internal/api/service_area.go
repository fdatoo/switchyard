package api

import (
	"context"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
)

// AreaService implements read-only area RPCs.
type AreaService struct{ be AreaReader }

// NewAreaService returns an area service backed by be.
func NewAreaService(be AreaReader) *AreaService { return &AreaService{be: be} }

var _ switchyardv1alpha1connect.AreaServiceHandler = (*AreaService)(nil)

// List returns configured areas with API pagination.
func (s *AreaService) List(ctx context.Context, req *connect.Request[v1.ListAreasRequest]) (*connect.Response[v1.ListAreasResponse], error) {
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
	areas, next, err := s.be.ListAreas(ctx, pr)
	if err != nil {
		return nil, ToConnect(ctx, err, "list_failed")
	}
	out := &v1.ListAreasResponse{Page: &v1.PageResponse{}}
	if tok2, _ := EncodeCursor(next); tok2 != "" {
		out.Page.NextPageToken = tok2
	}
	for _, a := range areas {
		out.Areas = append(out.Areas, &v1.Area{Id: a.ID, DisplayName: a.DisplayName, ParentId: a.ParentID})
	}
	return connect.NewResponse(out), nil
}

// Get returns one configured area by id.
func (s *AreaService) Get(ctx context.Context, req *connect.Request[v1.GetAreaRequest]) (*connect.Response[v1.GetAreaResponse], error) {
	a, err := s.be.GetArea(ctx, req.Msg.Id)
	if err != nil {
		return nil, ToConnect(ctx, err, "area_not_found")
	}
	return connect.NewResponse(&v1.GetAreaResponse{Area: &v1.Area{Id: a.ID, DisplayName: a.DisplayName, ParentId: a.ParentID}}), nil
}
