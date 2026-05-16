package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"

	configv1 "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"

	"github.com/google/uuid"
)

// errSceneNotFoundSentinel is used by tests + the daemon adapter to convey
// "scene not found" without depending on the scene package directly.
var errSceneNotFoundSentinel = errors.New("scene: not found")

// SceneSnapshotReader is the snapshot seam.
type SceneSnapshotReader interface {
	Current() *configv1.ConfigSnapshot
}

// RealSceneService implements v1.SceneService backed by SceneInvoker +
// SceneSnapshotReader. Constructed by the daemon wiring.
type RealSceneService struct {
	snap   SceneSnapshotReader
	invoke SceneInvoker
	logger *slog.Logger
}

// NewRealSceneService returns a scene service backed by live config snapshots and invocation.
func NewRealSceneService(snap SceneSnapshotReader, invoke SceneInvoker, logger *slog.Logger) *RealSceneService {
	return &RealSceneService{snap: snap, invoke: invoke, logger: logger}
}

var _ switchyardv1alpha1connect.SceneServiceHandler = (*RealSceneService)(nil)

// List returns configured scenes from the current config snapshot.
func (s *RealSceneService) List(_ context.Context, _ *connect.Request[v1.ListScenesRequest]) (*connect.Response[v1.ListScenesResponse], error) {
	snap := s.snap.Current()
	out := make([]*v1.Scene, 0, len(snap.GetScenes()))
	for _, sc := range snap.GetScenes() {
		out = append(out, &v1.Scene{
			Id:          sc.GetId(),
			DisplayName: sc.GetDisplayName(),
			AreaId:      sc.GetAreaId(),
		})
	}
	return connect.NewResponse(&v1.ListScenesResponse{Scenes: out}), nil
}

// Apply invokes a scene and returns the generated correlation id.
func (s *RealSceneService) Apply(ctx context.Context, req *connect.Request[v1.ApplySceneRequest]) (*connect.Response[v1.ApplySceneResponse], error) {
	corrID := uuid.NewString()
	err := s.invoke.Invoke(ctx, req.Msg.GetId(), corrID, "rpc:"+principalID(ctx))
	if err != nil {
		if errors.Is(err, errSceneNotFoundSentinel) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("scene %q not found", req.Msg.GetId()))
		}
		return nil, ToConnect(ctx, err, "scene_apply_failed")
	}
	return connect.NewResponse(&v1.ApplySceneResponse{CorrelationId: corrID}), nil
}

// Preview returns human-readable action lines without running the scene.
func (s *RealSceneService) Preview(_ context.Context, req *connect.Request[v1.PreviewSceneRequest]) (*connect.Response[v1.PreviewSceneResponse], error) {
	snap := s.snap.Current()
	var scene *configv1.SceneConfig
	for _, sc := range snap.GetScenes() {
		if sc.GetId() == req.Msg.GetId() {
			scene = sc
			break
		}
	}
	if scene == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("scene %q not found", req.Msg.GetId()))
	}
	lines := make([]string, 0, len(scene.GetActions()))
	for _, ac := range scene.GetActions() {
		lines = append(lines, previewActionLine(ac))
	}
	return connect.NewResponse(&v1.PreviewSceneResponse{Lines: lines}), nil
}

func previewActionLine(ac *configv1.ActionConfig) string {
	switch k := ac.GetKind().(type) {
	case *configv1.ActionConfig_CallService:
		cs := k.CallService
		return fmt.Sprintf("%s: %s", cs.GetEntity(), cs.GetCapability())
	case *configv1.ActionConfig_Scene:
		return fmt.Sprintf("apply scene %q", k.Scene.GetSlug())
	case *configv1.ActionConfig_Script:
		return fmt.Sprintf("run script %q", k.Script.GetName())
	case *configv1.ActionConfig_Wait:
		return fmt.Sprintf("wait %d ns", k.Wait.GetDurationNs())
	default:
		return "(unknown action)"
	}
}
