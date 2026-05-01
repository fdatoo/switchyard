package tools

import (
	"context"
	"encoding/json"

	"connectrpc.com/connect"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/mcp"
)

// ApplySceneInput is the input schema for gohome__apply_scene.
type ApplySceneInput struct {
	Slug string `json:"slug"`
}

func registerScenes(d Deps) {
	sdk.AddTool(d.Server, &sdk.Tool{
		Name:        "gohome__apply_scene",
		Description: "Apply a named scene. Currently UNIMPLEMENTED — scene service is not yet shipped.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in ApplySceneInput) (*sdk.CallToolResult, any, error) {
		req := connect.NewRequest(&v1.ApplySceneRequest{Id: in.Slug})
		mcp.SetToolHeader("gohome__apply_scene").Apply(req)
		resp, err := d.Client.Scene.Apply(ctx, req)
		if err != nil {
			return nil, nil, toToolError(err)
		}
		out := map[string]any{
			"correlation_id": resp.Msg.GetCorrelationId(),
		}
		b, _ := json.Marshal(out)
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: string(b)}}}, nil, nil
	})
}
