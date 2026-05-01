package tools

import (
	"context"
	"encoding/json"

	"connectrpc.com/connect"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/mcp"
)

// ValidateConfigInput is the input schema for gohome__validate_config.
// PklBundle is a base64-encoded PKL bundle or a plain string containing PKL source.
type ValidateConfigInput struct {
	PklBundle string `json:"pkl_bundle"`
}

// ApplyConfigInput is the input schema for gohome__apply_config.
// PklBundle is a base64-encoded PKL bundle or a plain string containing PKL source.
type ApplyConfigInput struct {
	PklBundle string `json:"pkl_bundle"`
	Message   string `json:"message,omitempty"`
	DryRun    bool   `json:"dry_run,omitempty"`
	Strict    bool   `json:"strict,omitempty"`
}

func registerConfig(d Deps) {
	sdk.AddTool(d.Server, &sdk.Tool{
		Name:        "gohome__validate_config",
		Description: "Validate a PKL config bundle without applying it.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in ValidateConfigInput) (*sdk.CallToolResult, any, error) {
		req := connect.NewRequest(&v1.ValidateConfigRequest{
			PklBundle: []byte(in.PklBundle),
		})
		mcp.SetToolHeader("gohome__validate_config").Apply(req)
		resp, err := d.Client.Config.Validate(ctx, req)
		if err != nil {
			return nil, nil, toToolError(err)
		}
		mo := protojson.MarshalOptions{UseProtoNames: true}
		var diffRaw json.RawMessage
		if resp.Msg.Diff != nil {
			b, merr := mo.Marshal(resp.Msg.Diff)
			if merr == nil {
				diffRaw = b
			}
		}
		out := map[string]any{
			"valid":  resp.Msg.GetValid(),
			"errors": resp.Msg.GetErrors(),
			"diff":   diffRaw,
		}
		b, _ := json.Marshal(out)
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: string(b)}}}, nil, nil
	})

	sdk.AddTool(d.Server, &sdk.Tool{
		Name:        "gohome__apply_config",
		Description: "Apply a PKL config bundle to the daemon.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in ApplyConfigInput) (*sdk.CallToolResult, any, error) {
		req := connect.NewRequest(&v1.ApplyConfigRequest{
			PklBundle: []byte(in.PklBundle),
			Message:   in.Message,
			DryRun:    in.DryRun,
			Strict:    in.Strict,
		})
		mcp.SetToolHeader("gohome__apply_config").Apply(req)
		resp, err := d.Client.Config.Apply(ctx, req)
		if err != nil {
			return nil, nil, toToolError(err)
		}
		mo := protojson.MarshalOptions{UseProtoNames: true}
		var diffRaw json.RawMessage
		if resp.Msg.Diff != nil {
			b, merr := mo.Marshal(resp.Msg.Diff)
			if merr == nil {
				diffRaw = b
			}
		}
		out := map[string]any{
			"applied": resp.Msg.GetApplied(),
			"diff":    diffRaw,
		}
		b, _ := json.Marshal(out)
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: string(b)}}}, nil, nil
	})
}
