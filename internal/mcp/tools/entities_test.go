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

	v1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/gen/gohome/v1alpha1/gohomev1alpha1connect"
	"github.com/fdatoo/gohome/internal/auth"
	"github.com/fdatoo/gohome/internal/mcp"
	"github.com/fdatoo/gohome/internal/mcp/tools"
)

// fakeEntityHandler implements gohomev1alpha1connect.EntityServiceHandler.
type fakeEntityHandler struct {
	gohomev1alpha1connect.UnimplementedEntityServiceHandler

	getEntity      func(context.Context, *connect.Request[v1.GetEntityRequest]) (*connect.Response[v1.GetEntityResponse], error)
	listEntities   func(context.Context, *connect.Request[v1.ListEntitiesRequest]) (*connect.Response[v1.ListEntitiesResponse], error)
	callCapability func(context.Context, *connect.Request[v1.CallCapabilityRequest]) (*connect.Response[v1.CallCapabilityResponse], error)
}

func (h *fakeEntityHandler) Get(ctx context.Context, req *connect.Request[v1.GetEntityRequest]) (*connect.Response[v1.GetEntityResponse], error) {
	if h.getEntity != nil {
		return h.getEntity(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *fakeEntityHandler) List(ctx context.Context, req *connect.Request[v1.ListEntitiesRequest]) (*connect.Response[v1.ListEntitiesResponse], error) {
	if h.listEntities != nil {
		return h.listEntities(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *fakeEntityHandler) CallCapability(ctx context.Context, req *connect.Request[v1.CallCapabilityRequest]) (*connect.Response[v1.CallCapabilityResponse], error) {
	if h.callCapability != nil {
		return h.callCapability(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func newEntitiesTestDeps(t *testing.T, handler *fakeEntityHandler) (tools.Deps, *sdk.Server) {
	t.Helper()
	_, handler2 := gohomev1alpha1connect.NewEntityServiceHandler(handler)
	srv := httptest.NewServer(handler2)
	t.Cleanup(srv.Close)

	c, err := mcp.NewClient(mcp.ClientOptions{EndpointURL: srv.URL})
	require.NoError(t, err)

	s := sdk.NewServer(testImpl, nil)
	d := tools.Deps{
		Server: s,
		Client: c,
		Auth:   auth.AllowAll{},
	}
	return d, s
}

func TestGetState_HappyPath(t *testing.T) {
	handler := &fakeEntityHandler{
		getEntity: func(_ context.Context, req *connect.Request[v1.GetEntityRequest]) (*connect.Response[v1.GetEntityResponse], error) {
			assert.Equal(t, "light.kitchen", req.Msg.Id)
			return connect.NewResponse(&v1.GetEntityResponse{
				Entity: &v1.Entity{
					Id:           "light.kitchen",
					Type:         "light",
					FriendlyName: "Kitchen Light",
				},
			}), nil
		},
	}
	d, s := newEntitiesTestDeps(t, handler)
	tools.Register(d)

	result, err := callTool(t, s, "gohome__get_state", map[string]any{"entity_id": "light.kitchen"})
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(textContent(t, result)), &m))
	assert.Equal(t, "light.kitchen", m["id"])
	assert.Equal(t, "Kitchen Light", m["name"])
	assert.NotContains(t, m, "friendly_name")
}

func TestGetState_NotFound(t *testing.T) {
	handler := &fakeEntityHandler{
		getEntity: func(_ context.Context, _ *connect.Request[v1.GetEntityRequest]) (*connect.Response[v1.GetEntityResponse], error) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("entity not found"))
		},
	}
	d, s := newEntitiesTestDeps(t, handler)
	tools.Register(d)

	result, err := callTool(t, s, "gohome__get_state", map[string]any{"entity_id": "light.missing"})
	require.NoError(t, err)
	assert.True(t, result.IsError, "expected IsError=true for not_found")
}

func TestListEntities_PassesSelector(t *testing.T) {
	var capturedReq *v1.ListEntitiesRequest
	handler := &fakeEntityHandler{
		listEntities: func(_ context.Context, req *connect.Request[v1.ListEntitiesRequest]) (*connect.Response[v1.ListEntitiesResponse], error) {
			capturedReq = req.Msg
			return connect.NewResponse(&v1.ListEntitiesResponse{
				Entities: []*v1.Entity{
					{Id: "light.a", Type: "light", FriendlyName: "A"},
				},
				Page: &v1.PageResponse{NextPageToken: "tok2"},
			}), nil
		},
	}
	d, s := newEntitiesTestDeps(t, handler)
	tools.Register(d)

	result, err := callTool(t, s, "gohome__list_entities", map[string]any{
		"areas":     []any{"kitchen"},
		"device_id": "dev1",
		"limit":     10,
		"cursor":    "tok1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	require.NotNil(t, capturedReq)
	assert.Equal(t, []string{"kitchen"}, capturedReq.Selector.Areas)
	assert.Equal(t, []string{"dev1"}, capturedReq.Selector.DeviceIds)
	assert.Equal(t, uint32(10), capturedReq.Page.PageSize)
	assert.Equal(t, "tok1", capturedReq.Page.PageToken)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(textContent(t, result)), &out))
	assert.Equal(t, "tok2", out["next_cursor"])
	entities, ok := out["entities"].([]any)
	require.True(t, ok)
	assert.Len(t, entities, 1)
}

func TestCallCapability_HappyPath(t *testing.T) {
	handler := &fakeEntityHandler{
		callCapability: func(_ context.Context, req *connect.Request[v1.CallCapabilityRequest]) (*connect.Response[v1.CallCapabilityResponse], error) {
			assert.Equal(t, "light.kitchen", req.Msg.EntityId)
			assert.Equal(t, "turn_on", req.Msg.Capability)
			return connect.NewResponse(&v1.CallCapabilityResponse{
				CorrelationId: "cmd-123",
			}), nil
		},
	}
	d, s := newEntitiesTestDeps(t, handler)
	tools.Register(d)

	result, err := callTool(t, s, "gohome__call_capability", map[string]any{
		"entity_id":  "light.kitchen",
		"capability": "turn_on",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(textContent(t, result)), &out))
	assert.Equal(t, "cmd-123", out["correlation_id"])
}
