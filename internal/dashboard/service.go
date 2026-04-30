package dashboard

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/gen/gohome/v1alpha1/gohomev1alpha1connect"
)

// Backend is the persistence + business logic interface for dashboards.
type Backend interface {
	List(ctx context.Context) ([]DashboardMeta, error)
	Get(ctx context.Context, slug string) (*DashboardData, error)
	Create(ctx context.Context, slug, title string) (*DashboardData, error)
	Delete(ctx context.Context, slug string, deleteSource bool) error
	SaveLayout(ctx context.Context, d *DashboardData) (*DashboardData, string, error)
	WidgetCatalog(ctx context.Context) ([]WidgetClassInfo, error)
}

// DashboardMeta is the list-level dashboard info.
type DashboardMeta struct {
	Slug  string
	Title string
}

// DashboardData is the full dashboard representation.
type DashboardData struct {
	Slug            string
	Title           string
	Grid            GridData
	Widgets         []WidgetData
	SourcePkl       string
	LayoutPkl       string
	WysiwygWritable bool
}

// GridData holds grid configuration.
type GridData struct {
	Columns   int32
	RowHeight int32
}

// WidgetData holds a single widget instance.
type WidgetData struct {
	ID          string
	ClassID     string
	Pos         PosData
	Props       map[string]any
	IsContainer bool
	ChildGrid   GridData
	Children    []WidgetData
}

// PosData holds widget position.
type PosData struct{ X, Y, W, H int32 }

// Service implements the DashboardService Connect handler.
type Service struct {
	be      Backend
	catalog *Catalog
}

// NewService creates a new dashboard service.
func NewService(be Backend, catalog *Catalog) *Service {
	return &Service{be: be, catalog: catalog}
}

var _ gohomev1alpha1connect.DashboardServiceHandler = (*Service)(nil)

func (s *Service) List(ctx context.Context, _ *connect.Request[v1.ListDashboardsRequest]) (*connect.Response[v1.ListDashboardsResponse], error) {
	metas, err := s.be.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*v1.Dashboard, 0, len(metas))
	for _, m := range metas {
		out = append(out, &v1.Dashboard{Slug: m.Slug, Title: m.Title})
	}
	return connect.NewResponse(&v1.ListDashboardsResponse{Dashboards: out}), nil
}

func (s *Service) Get(ctx context.Context, req *connect.Request[v1.GetDashboardRequest]) (*connect.Response[v1.GetDashboardResponse], error) {
	d, err := s.be.Get(ctx, req.Msg.Slug)
	if err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&v1.GetDashboardResponse{Dashboard: toProto(d)}), nil
}

func (s *Service) GetWidgetCatalog(ctx context.Context, _ *connect.Request[v1.GetWidgetCatalogRequest]) (*connect.Response[v1.GetWidgetCatalogResponse], error) {
	classes, err := s.be.WidgetCatalog(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*v1.WidgetClass, 0, len(classes))
	for _, wc := range classes {
		out = append(out, widgetClassToProto(wc))
	}
	return connect.NewResponse(&v1.GetWidgetCatalogResponse{
		Catalog: &v1.WidgetCatalog{Classes: out},
	}), nil
}

func (s *Service) Create(ctx context.Context, req *connect.Request[v1.CreateDashboardRequest]) (*connect.Response[v1.CreateDashboardResponse], error) {
	d, err := s.be.Create(ctx, req.Msg.Slug, req.Msg.Title)
	if err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&v1.CreateDashboardResponse{Dashboard: toProto(d)}), nil
}

func (s *Service) Delete(ctx context.Context, req *connect.Request[v1.DeleteDashboardRequest]) (*connect.Response[v1.DeleteDashboardResponse], error) {
	if err := s.be.Delete(ctx, req.Msg.Slug, req.Msg.DeleteSourceToo); err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&v1.DeleteDashboardResponse{}), nil
}

func (s *Service) SaveLayout(ctx context.Context, req *connect.Request[v1.SaveDashboardLayoutRequest]) (*connect.Response[v1.SaveDashboardLayoutResponse], error) {
	if req.Msg.Dashboard == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("dashboard required"))
	}
	d := fromProto(req.Msg.Dashboard)
	saved, correlationID, err := s.be.SaveLayout(ctx, d)
	if err != nil {
		return nil, connectErr(err)
	}
	return connect.NewResponse(&v1.SaveDashboardLayoutResponse{
		Dashboard:     toProto(saved),
		CorrelationId: correlationID,
	}), nil
}

// ErrDashboardNotFound is returned when a dashboard slug is not found.
var ErrDashboardNotFound = errors.New("dashboard: not found")

func connectErr(err error) error {
	if errors.Is(err, ErrDashboardNotFound) {
		return connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewError(connect.CodeInternal, err)
}

// --- proto conversions ---

func toProto(d *DashboardData) *v1.Dashboard {
	if d == nil {
		return nil
	}
	return &v1.Dashboard{
		Slug:            d.Slug,
		Title:           d.Title,
		Grid:            gridToProto(d.Grid),
		Widgets:         widgetsToProto(d.Widgets),
		SourcePkl:       d.SourcePkl,
		LayoutPkl:       d.LayoutPkl,
		WysiwygWritable: d.WysiwygWritable,
	}
}

func fromProto(d *v1.Dashboard) *DashboardData {
	return &DashboardData{
		Slug:    d.Slug,
		Title:   d.Title,
		Grid:    gridFromProto(d.Grid),
		Widgets: widgetsFromProto(d.Widgets),
	}
}

func gridToProto(g GridData) *v1.Grid {
	return &v1.Grid{Columns: g.Columns, RowHeight: g.RowHeight}
}

func gridFromProto(g *v1.Grid) GridData {
	if g == nil {
		return GridData{Columns: 12, RowHeight: 60}
	}
	return GridData{Columns: g.Columns, RowHeight: g.RowHeight}
}

func widgetsToProto(ws []WidgetData) []*v1.WidgetInstance {
	out := make([]*v1.WidgetInstance, 0, len(ws))
	for _, w := range ws {
		out = append(out, widgetToProto(w))
	}
	return out
}

func widgetToProto(w WidgetData) *v1.WidgetInstance {
	var props *structpb.Struct
	if len(w.Props) > 0 {
		props, _ = structpb.NewStruct(w.Props)
	}
	wi := &v1.WidgetInstance{
		Id:          w.ID,
		ClassId:     w.ClassID,
		Pos:         &v1.Position{X: w.Pos.X, Y: w.Pos.Y, W: w.Pos.W, H: w.Pos.H},
		Props:       props,
		IsContainer: w.IsContainer,
	}
	if w.IsContainer {
		wi.ChildGrid = gridToProto(w.ChildGrid)
		wi.Children = widgetsToProto(w.Children)
	}
	return wi
}

func widgetsFromProto(ws []*v1.WidgetInstance) []WidgetData {
	out := make([]WidgetData, 0, len(ws))
	for _, w := range ws {
		out = append(out, widgetFromProto(w))
	}
	return out
}

func widgetFromProto(w *v1.WidgetInstance) WidgetData {
	var props map[string]any
	if w.Props != nil {
		props = w.Props.AsMap()
	}
	wd := WidgetData{
		ID:          w.Id,
		ClassID:     w.ClassId,
		Pos:         PosData{X: w.Pos.GetX(), Y: w.Pos.GetY(), W: w.Pos.GetW(), H: w.Pos.GetH()},
		Props:       props,
		IsContainer: w.IsContainer,
	}
	if w.IsContainer {
		wd.ChildGrid = gridFromProto(w.ChildGrid)
		wd.Children = widgetsFromProto(w.Children)
	}
	return wd
}

func widgetClassToProto(wc WidgetClassInfo) *v1.WidgetClass {
	return &v1.WidgetClass{
		ClassId:     wc.ClassID,
		IsContainer: wc.IsContainer,
		IsBuiltin:   wc.IsBuiltin,
		PackName:    wc.PackName,
		PackVersion: wc.PackVersion,
		BundleUrl:   wc.BundleURL,
		BundleHash:  wc.BundleHash,
	}
}
