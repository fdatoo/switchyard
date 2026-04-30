package tools

import (
	"context"
	"encoding/json"

	"connectrpc.com/connect"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/internal/mcp"
)

// RunScriptInput is the input schema for gohome__run_script.
type RunScriptInput struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

// EvalStarlarkInput is the input schema for gohome__eval_starlark.
type EvalStarlarkInput struct {
	Source string `json:"source"`
}

func registerScripts(d Deps) {
	sdk.AddTool(d.Server, &sdk.Tool{
		Name:        "gohome__run_script",
		Description: "Run a named Starlark script by name, passing optional arguments.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in RunScriptInput) (*sdk.CallToolResult, any, error) {
		var args *structpb.Struct
		if len(in.Args) > 0 {
			var serr error
			args, serr = structpb.NewStruct(in.Args)
			if serr != nil {
				return nil, nil, toToolError(serr)
			}
		}
		req := connect.NewRequest(&v1.RunScriptRequest{
			Name: in.Name,
			Args: args,
		})
		mcp.SetToolHeader("gohome__run_script").Apply(req)
		resp, err := d.Client.Script.Run(ctx, req)
		if err != nil {
			return nil, nil, toToolError(err)
		}
		mo := protojson.MarshalOptions{UseProtoNames: true}
		var resultRaw json.RawMessage
		if resp.Msg.Result != nil {
			b, merr := mo.Marshal(resp.Msg.Result)
			if merr == nil {
				resultRaw = b
			}
		}
		out := map[string]any{
			"run_id": resp.Msg.GetRunId(),
			"result": resultRaw,
		}
		b, _ := json.Marshal(out)
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: string(b)}}}, nil, nil
	})

	sdk.AddTool(d.Server, &sdk.Tool{
		Name:        "gohome__eval_starlark",
		Description: "Evaluate a Starlark expression and return the result.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in EvalStarlarkInput) (*sdk.CallToolResult, any, error) {
		req := connect.NewRequest(&v1.EvalScriptRequest{Expr: in.Source})
		mcp.SetToolHeader("gohome__eval_starlark").Apply(req)
		resp, err := d.Client.Script.Eval(ctx, req)
		if err != nil {
			return nil, nil, toToolError(err)
		}
		mo := protojson.MarshalOptions{UseProtoNames: true}
		var resultRaw json.RawMessage
		if resp.Msg.Result != nil {
			b, merr := mo.Marshal(resp.Msg.Result)
			if merr == nil {
				resultRaw = b
			}
		}
		out := map[string]any{
			"result":      resultRaw,
			"stdout":      resp.Msg.GetStdout(),
			"duration_ms": resp.Msg.GetDurationMs(),
			"truncated":   resp.Msg.GetTruncated(),
		}
		b, _ := json.Marshal(out)
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: string(b)}}}, nil, nil
	})
}
