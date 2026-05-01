package api

import (
	"context"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
)

func unimplemented(ctx context.Context, reason string) error {
	return ToConnect(ctx, ErrNotImplemented, reason)
}

// SceneService stub.
type SceneService struct{}

func NewSceneService() *SceneService { return &SceneService{} }

var _ switchyardv1alpha1connect.SceneServiceHandler = (*SceneService)(nil)

func (*SceneService) List(ctx context.Context, _ *connect.Request[v1.ListScenesRequest]) (*connect.Response[v1.ListScenesResponse], error) {
	return nil, unimplemented(ctx, "scene_unimplemented")
}
func (*SceneService) Apply(ctx context.Context, _ *connect.Request[v1.ApplySceneRequest]) (*connect.Response[v1.ApplySceneResponse], error) {
	return nil, unimplemented(ctx, "scene_unimplemented")
}
func (*SceneService) Preview(ctx context.Context, _ *connect.Request[v1.PreviewSceneRequest]) (*connect.Response[v1.PreviewSceneResponse], error) {
	return nil, unimplemented(ctx, "scene_unimplemented")
}

// DashboardService stub.
type DashboardService struct{}

func NewDashboardService() *DashboardService { return &DashboardService{} }

var _ switchyardv1alpha1connect.DashboardServiceHandler = (*DashboardService)(nil)

func (*DashboardService) List(ctx context.Context, _ *connect.Request[v1.ListDashboardsRequest]) (*connect.Response[v1.ListDashboardsResponse], error) {
	return nil, unimplemented(ctx, "dashboard_unimplemented")
}
func (*DashboardService) Get(ctx context.Context, _ *connect.Request[v1.GetDashboardRequest]) (*connect.Response[v1.GetDashboardResponse], error) {
	return nil, unimplemented(ctx, "dashboard_unimplemented")
}
func (*DashboardService) GetWidgetCatalog(ctx context.Context, _ *connect.Request[v1.GetWidgetCatalogRequest]) (*connect.Response[v1.GetWidgetCatalogResponse], error) {
	return nil, unimplemented(ctx, "dashboard_unimplemented")
}
func (*DashboardService) Create(ctx context.Context, _ *connect.Request[v1.CreateDashboardRequest]) (*connect.Response[v1.CreateDashboardResponse], error) {
	return nil, unimplemented(ctx, "dashboard_unimplemented")
}
func (*DashboardService) Delete(ctx context.Context, _ *connect.Request[v1.DeleteDashboardRequest]) (*connect.Response[v1.DeleteDashboardResponse], error) {
	return nil, unimplemented(ctx, "dashboard_unimplemented")
}
func (*DashboardService) SaveLayout(ctx context.Context, _ *connect.Request[v1.SaveDashboardLayoutRequest]) (*connect.Response[v1.SaveDashboardLayoutResponse], error) {
	return nil, unimplemented(ctx, "dashboard_unimplemented")
}
