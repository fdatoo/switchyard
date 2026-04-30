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

// GetStateInput is the input schema for gohome__get_state.
type GetStateInput struct {
	EntityID string `json:"entity_id"`
}

// ListEntitiesInput is the input schema for gohome__list_entities.
type ListEntitiesInput struct {
	Areas    []string `json:"areas,omitempty"`
	Zones    []string `json:"zones,omitempty"`
	Classes  []string `json:"classes,omitempty"`
	DeviceID string   `json:"device_id,omitempty"`
	Limit    int      `json:"limit,omitempty"`
	Cursor   string   `json:"cursor,omitempty"`
}

// CallCapabilityInput is the input schema for gohome__call_capability.
type CallCapabilityInput struct {
	EntityID   string         `json:"entity_id"`
	Capability string         `json:"capability"`
	Params     map[string]any `json:"params,omitempty"`
}

func registerEntities(d Deps) {
	sdk.AddTool(d.Server, &sdk.Tool{
		Name:        "gohome__get_state",
		Description: "Get the current state of a single entity by ID.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in GetStateInput) (*sdk.CallToolResult, any, error) {
		req := connect.NewRequest(&v1.GetEntityRequest{Id: in.EntityID})
		mcp.SetToolHeader("gohome__get_state").Apply(req)
		resp, err := d.Client.Entity.Get(ctx, req)
		if err != nil {
			return nil, nil, toToolError(err)
		}
		b, err := marshalEntity(resp.Msg.Entity)
		if err != nil {
			return nil, nil, toToolError(err)
		}
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: string(b)}}}, nil, nil
	})

	sdk.AddTool(d.Server, &sdk.Tool{
		Name:        "gohome__list_entities",
		Description: "List entities, optionally filtered by area, zone, class, or device.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in ListEntitiesInput) (*sdk.CallToolResult, any, error) {
		var pageSize uint32
		if in.Limit > 0 {
			pageSize = uint32(in.Limit)
		}
		selector := &v1.EntitySelector{
			Areas:   in.Areas,
			Zones:   in.Zones,
			Classes: in.Classes,
		}
		if in.DeviceID != "" {
			selector.DeviceIds = []string{in.DeviceID}
		}
		req := connect.NewRequest(&v1.ListEntitiesRequest{
			Page:     &v1.PageRequest{PageSize: pageSize, PageToken: in.Cursor},
			Selector: selector,
		})
		mcp.SetToolHeader("gohome__list_entities").Apply(req)
		resp, err := d.Client.Entity.List(ctx, req)
		if err != nil {
			return nil, nil, toToolError(err)
		}
		entities := make([]json.RawMessage, 0, len(resp.Msg.Entities))
		for _, e := range resp.Msg.Entities {
			b, merr := marshalEntity(e)
			if merr != nil {
				return nil, nil, toToolError(merr)
			}
			entities = append(entities, b)
		}
		nextCursor := ""
		if resp.Msg.Page != nil {
			nextCursor = resp.Msg.Page.NextPageToken
		}
		out := map[string]any{
			"entities":    entities,
			"next_cursor": nextCursor,
		}
		b, _ := json.Marshal(out)
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: string(b)}}}, nil, nil
	})

	sdk.AddTool(d.Server, &sdk.Tool{
		Name:        "gohome__call_capability",
		Description: "Call a capability on an entity (e.g. turn on/off, set brightness).",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in CallCapabilityInput) (*sdk.CallToolResult, any, error) {
		var params *structpb.Struct
		if len(in.Params) > 0 {
			var serr error
			params, serr = structpb.NewStruct(in.Params)
			if serr != nil {
				return nil, nil, toToolError(serr)
			}
		}
		req := connect.NewRequest(&v1.CallCapabilityRequest{
			EntityId:   in.EntityID,
			Capability: in.Capability,
			Parameters: params,
		})
		mcp.SetToolHeader("gohome__call_capability").Apply(req)
		resp, err := d.Client.Entity.CallCapability(ctx, req)
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

// marshalEntity serialises a proto Entity to JSON, renaming friendly_name → name.
func marshalEntity(e *v1.Entity) (json.RawMessage, error) {
	mo := protojson.MarshalOptions{UseProtoNames: true}
	raw, err := mo.Marshal(e)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if fn, ok := m["friendly_name"]; ok {
		m["name"] = fn
		delete(m, "friendly_name")
	}
	return json.Marshal(m)
}
