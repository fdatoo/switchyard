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
