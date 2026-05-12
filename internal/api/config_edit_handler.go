package api

import (
	"context"
	"encoding/json"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/automation/regen"
)

// RegenPreview implements ConfigService.RegenPreview.
// For file_type="automation", it unmarshals ast_json into AutomationConfig,
// calls the automation regenerator, and returns the Pkl bytes.
func (s *ConfigService) RegenPreview(_ context.Context, req *connect.Request[v1.RegenPreviewRequest]) (*connect.Response[v1.RegenPreviewResponse], error) {
	switch req.Msg.GetFileType() {
	case "automation":
		var ac configpb.AutomationConfig
		if err := protojson.Unmarshal([]byte(req.Msg.GetAstJson()), &ac); err != nil {
			return nil, grpcToConnect(codes.InvalidArgument, "malformed ast_json: "+err.Error())
		}
		out, err := regen.Render(&ac)
		if err != nil {
			return nil, grpcToConnect(codes.InvalidArgument, "render failed: "+err.Error())
		}
		return connect.NewResponse(&v1.RegenPreviewResponse{PklBytes: out}), nil

	case "area":
		var a configpb.AreaConfig
		if err := protojson.Unmarshal([]byte(req.Msg.GetAstJson()), &a); err != nil {
			return nil, grpcToConnect(codes.InvalidArgument, "malformed ast_json: "+err.Error())
		}
		out, err := regen.RenderArea(&a)
		if err != nil {
			return nil, grpcToConnect(codes.InvalidArgument, "render failed: "+err.Error())
		}
		return connect.NewResponse(&v1.RegenPreviewResponse{PklBytes: out}), nil

	case "scene":
		var sc configpb.SceneConfig
		if err := protojson.Unmarshal([]byte(req.Msg.GetAstJson()), &sc); err != nil {
			return nil, grpcToConnect(codes.InvalidArgument, "malformed ast_json: "+err.Error())
		}
		out, err := regen.RenderScene(&sc)
		if err != nil {
			return nil, grpcToConnect(codes.InvalidArgument, "render failed: "+err.Error())
		}
		return connect.NewResponse(&v1.RegenPreviewResponse{PklBytes: out}), nil

	case "entity_areas":
		var m map[string]string
		if err := json.Unmarshal([]byte(req.Msg.GetAstJson()), &m); err != nil {
			return nil, grpcToConnect(codes.InvalidArgument, "malformed ast_json: "+err.Error())
		}
		out, err := regen.RenderEntityAreas(m)
		if err != nil {
			return nil, grpcToConnect(codes.InvalidArgument, "render failed: "+err.Error())
		}
		return connect.NewResponse(&v1.RegenPreviewResponse{PklBytes: out}), nil

	case "page":
		return nil, grpcToConnect(codes.Unimplemented, "page regen not yet implemented")

	default:
		return nil, grpcToConnect(codes.InvalidArgument, "unknown file_type: "+req.Msg.GetFileType())
	}
}

// grpcToConnect converts a gRPC status code to a connect error.
func grpcToConnect(c codes.Code, msg string) error {
	st := status.New(c, msg)
	var cc connect.Code
	switch c {
	case codes.InvalidArgument:
		cc = connect.CodeInvalidArgument
	case codes.Unimplemented:
		cc = connect.CodeUnimplemented
	case codes.NotFound:
		cc = connect.CodeNotFound
	default:
		cc = connect.CodeInternal
	}
	return connect.NewError(cc, errors.New(st.Message()))
}
