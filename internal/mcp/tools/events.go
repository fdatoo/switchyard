package tools

import (
	"context"
	"encoding/json"
	"time"

	"connectrpc.com/connect"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"

	v1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/internal/mcp"
)

// QueryEventsInput is the input schema for gohome__query_events.
type QueryEventsInput struct {
	Kinds        []string `json:"kinds,omitempty"`
	EntityPrefix string   `json:"entity_prefix,omitempty"`
	Sources      []string `json:"sources,omitempty"`
	FromCursor   uint64   `json:"from_cursor,omitempty"`
	ToCursor     uint64   `json:"to_cursor,omitempty"`
	Limit        int      `json:"limit,omitempty"`
	Cursor       string   `json:"cursor,omitempty"`
}

// TailEventsInput is the input schema for gohome__tail_events.
type TailEventsInput struct {
	Kinds        []string `json:"kinds,omitempty"`
	EntityPrefix string   `json:"entity_prefix,omitempty"`
	Sources      []string `json:"sources,omitempty"`
	FromCursor   uint64   `json:"from_cursor,omitempty"`
	MaxEvents    int      `json:"max_events,omitempty"`
	WaitSeconds  int      `json:"wait_seconds,omitempty"`
}

func registerEvents(d Deps) {
	sdk.AddTool(d.Server, &sdk.Tool{
		Name:        "gohome__query_events",
		Description: "Query historical events from the event log.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in QueryEventsInput) (*sdk.CallToolResult, any, error) {
		var pageSize uint32
		if in.Limit > 0 {
			pageSize = uint32(in.Limit)
		}
		req := connect.NewRequest(&v1.QueryEventsRequest{
			Page: &v1.PageRequest{PageSize: pageSize, PageToken: in.Cursor},
			Filter: &v1.EventFilter{
				Kinds:        in.Kinds,
				EntityPrefix: in.EntityPrefix,
				Sources:      in.Sources,
				FromCursor:   in.FromCursor,
				ToCursor:     in.ToCursor,
			},
		})
		mcp.SetToolHeader("gohome__query_events").Apply(req)
		resp, err := d.Client.Event.Query(ctx, req)
		if err != nil {
			return nil, nil, toToolError(err)
		}
		mo := protojson.MarshalOptions{UseProtoNames: true}
		events := make([]json.RawMessage, 0, len(resp.Msg.Events))
		for _, ev := range resp.Msg.Events {
			b, merr := mo.Marshal(ev)
			if merr != nil {
				return nil, nil, toToolError(merr)
			}
			events = append(events, b)
		}
		nextCursor := ""
		if resp.Msg.Page != nil {
			nextCursor = resp.Msg.Page.NextPageToken
		}
		out := map[string]any{
			"events":      events,
			"next_cursor": nextCursor,
		}
		b, _ := json.Marshal(out)
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: string(b)}}}, nil, nil
	})

	sdk.AddTool(d.Server, &sdk.Tool{
		Name:        "gohome__tail_events",
		Description: "Stream live events from the event bus, collecting until MaxEvents or WaitSeconds is reached.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in TailEventsInput) (*sdk.CallToolResult, any, error) {
		maxEvents := in.MaxEvents
		if maxEvents <= 0 {
			maxEvents = 100
		}

		waitSecs := in.WaitSeconds
		if waitSecs <= 0 {
			waitSecs = int(d.MCPCaps.TailDefaultWaitSeconds)
		}
		if d.MCPCaps.TailMaxWaitSeconds > 0 && waitSecs > int(d.MCPCaps.TailMaxWaitSeconds) {
			waitSecs = int(d.MCPCaps.TailMaxWaitSeconds)
		}

		req := connect.NewRequest(&v1.TailEventsRequest{
			Filter: &v1.EventFilter{
				Kinds:        in.Kinds,
				EntityPrefix: in.EntityPrefix,
				Sources:      in.Sources,
				FromCursor:   in.FromCursor,
			},
		})
		mcp.SetToolHeader("gohome__tail_events").Apply(req)
		stream, err := d.Client.Event.Tail(ctx, req)
		if err != nil {
			return nil, nil, toToolError(err)
		}
		defer func() { _ = stream.Close() }()

		mo := protojson.MarshalOptions{UseProtoNames: true}
		events := make([]json.RawMessage, 0, maxEvents)
		deadline := time.After(time.Duration(waitSecs) * time.Second)

		done := make(chan struct{})
		go func() {
			defer close(done)
			for stream.Receive() {
				msg := stream.Msg()
				switch k := msg.Kind.(type) {
				case *v1.TailEventsResponse_Event:
					b, merr := mo.Marshal(k.Event)
					if merr == nil {
						events = append(events, b)
					}
					if len(events) >= maxEvents {
						return
					}
				case *v1.TailEventsResponse_Heartbeat:
					// skip
				}
			}
		}()

		select {
		case <-done:
		case <-deadline:
		}

		if serr := stream.Err(); serr != nil && len(events) == 0 {
			return nil, nil, toToolError(serr)
		}

		out := map[string]any{"events": events}
		b, _ := json.Marshal(out)
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: string(b)}}}, nil, nil
	})
}
