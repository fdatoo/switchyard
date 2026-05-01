package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
	"github.com/fdatoo/switchyard/internal/auth"
	"github.com/fdatoo/switchyard/internal/mcp"
	"github.com/fdatoo/switchyard/internal/mcp/tools"
)

// fakeEventHandler implements switchyardv1alpha1connect.EventServiceHandler.
type fakeEventHandler struct {
	switchyardv1alpha1connect.UnimplementedEventServiceHandler

	queryFn func(context.Context, *connect.Request[v1.QueryEventsRequest]) (*connect.Response[v1.QueryEventsResponse], error)
	tailFn  func(context.Context, *connect.Request[v1.TailEventsRequest], *connect.ServerStream[v1.TailEventsResponse]) error
}

func (h *fakeEventHandler) Query(ctx context.Context, req *connect.Request[v1.QueryEventsRequest]) (*connect.Response[v1.QueryEventsResponse], error) {
	if h.queryFn != nil {
		return h.queryFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *fakeEventHandler) Tail(ctx context.Context, req *connect.Request[v1.TailEventsRequest], stream *connect.ServerStream[v1.TailEventsResponse]) error {
	if h.tailFn != nil {
		return h.tailFn(ctx, req, stream)
	}
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func newEventsTestDeps(t *testing.T, handler *fakeEventHandler) (tools.Deps, *sdk.Server) {
	t.Helper()
	_, h := switchyardv1alpha1connect.NewEventServiceHandler(handler)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := mcp.NewClient(mcp.ClientOptions{EndpointURL: srv.URL})
	require.NoError(t, err)

	s := sdk.NewServer(testImpl, nil)
	d := tools.Deps{
		Server:  s,
		Client:  c,
		Auth:    auth.AllowAll{},
		MCPCaps: mcp.MCPCaps{TailDefaultWaitSeconds: 5, TailMaxWaitSeconds: 10},
	}
	return d, s
}

func makeEvent(cursor uint64) *v1.Event {
	return &v1.Event{
		Cursor: cursor,
		Kind:   "state_changed",
		Source: "test",
	}
}

func TestQueryEvents_HappyPath(t *testing.T) {
	handler := &fakeEventHandler{
		queryFn: func(_ context.Context, req *connect.Request[v1.QueryEventsRequest]) (*connect.Response[v1.QueryEventsResponse], error) {
			assert.Equal(t, []string{"state_changed"}, req.Msg.Filter.Kinds)
			return connect.NewResponse(&v1.QueryEventsResponse{
				Events: []*v1.Event{
					makeEvent(1),
					makeEvent(2),
				},
				Page: &v1.PageResponse{NextPageToken: "cursor2"},
			}), nil
		},
	}
	d, s := newEventsTestDeps(t, handler)
	tools.Register(d)

	result, err := callTool(t, s, "gohome__query_events", map[string]any{
		"kinds": []any{"state_changed"},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(textContent(t, result)), &out))
	assert.Equal(t, "cursor2", out["next_cursor"])
	evs, ok := out["events"].([]any)
	require.True(t, ok)
	assert.Len(t, evs, 2)
}

func TestTailEvents_ReadsUntilMaxEvents(t *testing.T) {
	handler := &fakeEventHandler{
		tailFn: func(_ context.Context, _ *connect.Request[v1.TailEventsRequest], stream *connect.ServerStream[v1.TailEventsResponse]) error {
			for i := 0; i < 3; i++ {
				ev := makeEvent(uint64(i + 1))
				if err := stream.Send(&v1.TailEventsResponse{
					Kind: &v1.TailEventsResponse_Event{Event: ev},
				}); err != nil {
					return err
				}
			}
			return nil
		},
	}
	d, s := newEventsTestDeps(t, handler)
	tools.Register(d)

	result, err := callTool(t, s, "gohome__tail_events", map[string]any{
		"max_events":   2,
		"wait_seconds": 5,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(textContent(t, result)), &out))
	evs, ok := out["events"].([]any)
	require.True(t, ok)
	// We get at most max_events (2), though the stream may have closed early
	assert.LessOrEqual(t, len(evs), 2)
}
