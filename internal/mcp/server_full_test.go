package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
	"github.com/fdatoo/switchyard/internal/auth"
	"github.com/fdatoo/switchyard/internal/mcp"
	"github.com/fdatoo/switchyard/internal/mcp/audit"
	"github.com/fdatoo/switchyard/internal/mcp/resources"
	"github.com/fdatoo/switchyard/internal/mcp/tools"
	"github.com/fdatoo/switchyard/internal/observability"
)

// ---------------------------------------------------------------------------
// Fake service implementations
// ---------------------------------------------------------------------------

type fullFakeSystemSvc struct {
	switchyardv1alpha1connect.UnimplementedSystemServiceHandler
	recordFn func(context.Context, *connect.Request[v1.RecordConfigFileEditRequest]) (*connect.Response[v1.RecordConfigFileEditResponse], error)
}

func (h *fullFakeSystemSvc) RecordConfigFileEdit(ctx context.Context, req *connect.Request[v1.RecordConfigFileEditRequest]) (*connect.Response[v1.RecordConfigFileEditResponse], error) {
	if h.recordFn != nil {
		return h.recordFn(ctx, req)
	}
	return connect.NewResponse(&v1.RecordConfigFileEditResponse{EventCursor: 1}), nil
}

type fullFakeEntitySvc struct {
	switchyardv1alpha1connect.UnimplementedEntityServiceHandler
	getFn            func(context.Context, *connect.Request[v1.GetEntityRequest]) (*connect.Response[v1.GetEntityResponse], error)
	listFn           func(context.Context, *connect.Request[v1.ListEntitiesRequest]) (*connect.Response[v1.ListEntitiesResponse], error)
	callCapabilityFn func(context.Context, *connect.Request[v1.CallCapabilityRequest]) (*connect.Response[v1.CallCapabilityResponse], error)
	subscribeFn      func(context.Context, *connect.Request[v1.SubscribeEntitiesRequest], *connect.ServerStream[v1.SubscribeEntitiesResponse]) error
}

func (h *fullFakeEntitySvc) Get(ctx context.Context, req *connect.Request[v1.GetEntityRequest]) (*connect.Response[v1.GetEntityResponse], error) {
	if h.getFn != nil {
		return h.getFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *fullFakeEntitySvc) List(ctx context.Context, req *connect.Request[v1.ListEntitiesRequest]) (*connect.Response[v1.ListEntitiesResponse], error) {
	if h.listFn != nil {
		return h.listFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *fullFakeEntitySvc) CallCapability(ctx context.Context, req *connect.Request[v1.CallCapabilityRequest]) (*connect.Response[v1.CallCapabilityResponse], error) {
	if h.callCapabilityFn != nil {
		return h.callCapabilityFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *fullFakeEntitySvc) Subscribe(ctx context.Context, req *connect.Request[v1.SubscribeEntitiesRequest], stream *connect.ServerStream[v1.SubscribeEntitiesResponse]) error {
	if h.subscribeFn != nil {
		return h.subscribeFn(ctx, req, stream)
	}
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

type fullFakeEventSvc struct {
	switchyardv1alpha1connect.UnimplementedEventServiceHandler
	queryFn func(context.Context, *connect.Request[v1.QueryEventsRequest]) (*connect.Response[v1.QueryEventsResponse], error)
	tailFn  func(context.Context, *connect.Request[v1.TailEventsRequest], *connect.ServerStream[v1.TailEventsResponse]) error
}

func (h *fullFakeEventSvc) Query(ctx context.Context, req *connect.Request[v1.QueryEventsRequest]) (*connect.Response[v1.QueryEventsResponse], error) {
	if h.queryFn != nil {
		return h.queryFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *fullFakeEventSvc) Tail(ctx context.Context, req *connect.Request[v1.TailEventsRequest], stream *connect.ServerStream[v1.TailEventsResponse]) error {
	if h.tailFn != nil {
		return h.tailFn(ctx, req, stream)
	}
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

type fullFakeSceneSvc struct {
	switchyardv1alpha1connect.UnimplementedSceneServiceHandler
	applyFn func(context.Context, *connect.Request[v1.ApplySceneRequest]) (*connect.Response[v1.ApplySceneResponse], error)
}

func (h *fullFakeSceneSvc) Apply(ctx context.Context, req *connect.Request[v1.ApplySceneRequest]) (*connect.Response[v1.ApplySceneResponse], error) {
	if h.applyFn != nil {
		return h.applyFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("scene service not implemented"))
}

type fullFakeScriptSvc struct {
	switchyardv1alpha1connect.UnimplementedScriptServiceHandler
	runFn  func(context.Context, *connect.Request[v1.RunScriptRequest]) (*connect.Response[v1.RunScriptResponse], error)
	evalFn func(context.Context, *connect.Request[v1.EvalScriptRequest]) (*connect.Response[v1.EvalScriptResponse], error)
}

func (h *fullFakeScriptSvc) Run(ctx context.Context, req *connect.Request[v1.RunScriptRequest]) (*connect.Response[v1.RunScriptResponse], error) {
	if h.runFn != nil {
		return h.runFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *fullFakeScriptSvc) Eval(ctx context.Context, req *connect.Request[v1.EvalScriptRequest]) (*connect.Response[v1.EvalScriptResponse], error) {
	if h.evalFn != nil {
		return h.evalFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

type fullFakeConfigSvc struct {
	switchyardv1alpha1connect.UnimplementedConfigServiceHandler
	validateFn func(context.Context, *connect.Request[v1.ValidateConfigRequest]) (*connect.Response[v1.ValidateConfigResponse], error)
	applyFn    func(context.Context, *connect.Request[v1.ApplyConfigRequest]) (*connect.Response[v1.ApplyConfigResponse], error)
}

func (h *fullFakeConfigSvc) Validate(ctx context.Context, req *connect.Request[v1.ValidateConfigRequest]) (*connect.Response[v1.ValidateConfigResponse], error) {
	if h.validateFn != nil {
		return h.validateFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *fullFakeConfigSvc) Apply(ctx context.Context, req *connect.Request[v1.ApplyConfigRequest]) (*connect.Response[v1.ApplyConfigResponse], error) {
	if h.applyFn != nil {
		return h.applyFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

// ---------------------------------------------------------------------------
// testFakes groups all fake service implementations.
// ---------------------------------------------------------------------------

type testFakes struct {
	System *fullFakeSystemSvc
	Entity *fullFakeEntitySvc
	Event  *fullFakeEventSvc
	Scene  *fullFakeSceneSvc
	Script *fullFakeScriptSvc
	Config *fullFakeConfigSvc
}

// ---------------------------------------------------------------------------
// newFullStack wires ALL Connect services as fakes behind a single httptest.Server,
// registers all MCP tools and resources, and returns the connected client session.
// ---------------------------------------------------------------------------

func newFullStack(t *testing.T, configDir ...string) (*sdk.ClientSession, *testFakes) {
	t.Helper()

	dir := ""
	if len(configDir) > 0 {
		dir = configDir[0]
	} else {
		dir = t.TempDir()
	}

	fakes := &testFakes{
		System: &fullFakeSystemSvc{},
		Entity: &fullFakeEntitySvc{},
		Event:  &fullFakeEventSvc{},
		Scene:  &fullFakeSceneSvc{},
		Script: &fullFakeScriptSvc{},
		Config: &fullFakeConfigSvc{},
	}

	// Wire all services onto a single HTTP mux.
	mux := http.NewServeMux()

	p, h := switchyardv1alpha1connect.NewSystemServiceHandler(fakes.System)
	mux.Handle(p, h)

	p, h = switchyardv1alpha1connect.NewEntityServiceHandler(fakes.Entity)
	mux.Handle(p, h)

	p, h = switchyardv1alpha1connect.NewEventServiceHandler(fakes.Event)
	mux.Handle(p, h)

	p, h = switchyardv1alpha1connect.NewSceneServiceHandler(fakes.Scene)
	mux.Handle(p, h)

	p, h = switchyardv1alpha1connect.NewScriptServiceHandler(fakes.Script)
	mux.Handle(p, h)

	p, h = switchyardv1alpha1connect.NewConfigServiceHandler(fakes.Config)
	mux.Handle(p, h)

	// Register automation service for trace subscriptions (unimplemented is fine).
	p, h = switchyardv1alpha1connect.NewAutomationServiceHandler(
		switchyardv1alpha1connect.UnimplementedAutomationServiceHandler{},
	)
	mux.Handle(p, h)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, err := mcp.NewClient(mcp.ClientOptions{EndpointURL: srv.URL})
	require.NoError(t, err)

	m := observability.NewMetrics()

	caps := mcp.MCPCaps{
		TailDefaultWaitSeconds: 5,
		TailMaxWaitSeconds:     10,
		ReadFileMaxBytes:       1024 * 1024,
	}

	resDeps := resources.Deps{Client: c, MCPCaps: caps, Metrics: m}
	opts, setServer, shutdown := resources.NewServerOpts(resDeps)
	t.Cleanup(shutdown)

	impl := &sdk.Implementation{Name: "test-full", Version: "0"}
	s := sdk.NewServer(impl, opts)
	setServer(s)

	tools.Register(tools.Deps{
		Server:    s,
		Client:    c,
		ConfigDir: dir,
		MCPCaps:   caps,
		Auth:      auth.AllowAll{},
		Audit:     audit.NewRecorder(c.System),
	})
	resources.Register(s, resDeps)

	// Connect in-memory transport.
	ct, st := sdk.NewInMemoryTransports()
	_, err = s.Connect(context.Background(), st, nil)
	require.NoError(t, err, "server Connect")

	client := sdk.NewClient(impl, nil)
	cs, err := client.Connect(context.Background(), ct, nil)
	require.NoError(t, err, "client Connect")
	t.Cleanup(func() { cs.Close() })

	return cs, fakes
}

// textFromResult extracts the first TextContent text from a CallToolResult.
func textFromResult(t *testing.T, r *sdk.CallToolResult) string {
	t.Helper()
	require.NotNil(t, r)
	require.NotEmpty(t, r.Content)
	tc, ok := r.Content[0].(*sdk.TextContent)
	require.True(t, ok, "expected TextContent, got %T", r.Content[0])
	return tc.Text
}

// ---------------------------------------------------------------------------
// TestFullStack_ListTools
// ---------------------------------------------------------------------------

func TestFullStack_ListTools(t *testing.T) {
	cs, _ := newFullStack(t)

	result, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Len(t, result.Tools, 12, "expected exactly 12 tools")

	// Collect actual names (SDK may return tools in alphabetical order).
	gotNames := make([]string, len(result.Tools))
	for i, tool := range result.Tools {
		gotNames[i] = tool.Name
	}

	wantNames := []string{
		"switchyard__get_state",
		"switchyard__list_entities",
		"switchyard__call_capability",
		"switchyard__query_events",
		"switchyard__tail_events",
		"switchyard__apply_scene",
		"switchyard__run_script",
		"switchyard__eval_starlark",
		"switchyard__validate_config",
		"switchyard__apply_config",
		"switchyard__read_config_file",
		"switchyard__write_config_file",
	}
	assert.ElementsMatch(t, wantNames, gotNames, "tool names should match catalog")
}

// ---------------------------------------------------------------------------
// TestFullStack_ListResources
// ---------------------------------------------------------------------------

func TestFullStack_ListResources(t *testing.T) {
	cs, _ := newFullStack(t)

	resResult, err := cs.ListResources(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, resResult)
	assert.Len(t, resResult.Resources, 1, "expected 1 concrete resource")
	assert.Equal(t, "switchyard://entities/", resResult.Resources[0].URI)

	tmplResult, err := cs.ListResourceTemplates(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, tmplResult)
	assert.Len(t, tmplResult.ResourceTemplates, 2, "expected 2 resource templates")

	// Collect actual template URIs (SDK may return in any order).
	gotTemplates := make([]string, len(tmplResult.ResourceTemplates))
	for i, tmpl := range tmplResult.ResourceTemplates {
		gotTemplates[i] = tmpl.URITemplate
	}
	wantTemplates := []string{
		"switchyard://entities/{id}",
		"switchyard://automations/{automation_id}/runs/{run_id}/trace",
	}
	assert.ElementsMatch(t, wantTemplates, gotTemplates, "resource template URIs should match")
}

// ---------------------------------------------------------------------------
// TestFullStack_ToolCalls
// ---------------------------------------------------------------------------

func TestFullStack_ToolCalls(t *testing.T) {
	t.Run("get_state", func(t *testing.T) {
		cs, fakes := newFullStack(t)
		fakes.Entity.getFn = func(_ context.Context, _ *connect.Request[v1.GetEntityRequest]) (*connect.Response[v1.GetEntityResponse], error) {
			return connect.NewResponse(&v1.GetEntityResponse{
				Entity: &v1.Entity{
					Id:           "light.a",
					FriendlyName: "A Light",
				},
			}), nil
		}

		result, err := cs.CallTool(context.Background(), &sdk.CallToolParams{
			Name:      "switchyard__get_state",
			Arguments: map[string]any{"entity_id": "light.a"},
		})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError)

		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(textFromResult(t, result)), &m))
		assert.Equal(t, "A Light", m["name"])
		assert.NotContains(t, m, "friendly_name")
	})

	t.Run("list_entities", func(t *testing.T) {
		cs, fakes := newFullStack(t)
		fakes.Entity.listFn = func(_ context.Context, _ *connect.Request[v1.ListEntitiesRequest]) (*connect.Response[v1.ListEntitiesResponse], error) {
			return connect.NewResponse(&v1.ListEntitiesResponse{
				Entities: []*v1.Entity{
					{Id: "light.a", FriendlyName: "A"},
					{Id: "light.b", FriendlyName: "B"},
				},
			}), nil
		}

		result, err := cs.CallTool(context.Background(), &sdk.CallToolParams{
			Name:      "switchyard__list_entities",
			Arguments: map[string]any{},
		})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError)

		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(textFromResult(t, result)), &m))
		entities, ok := m["entities"].([]any)
		require.True(t, ok, "expected entities array")
		assert.Len(t, entities, 2)
	})

	t.Run("call_capability", func(t *testing.T) {
		cs, fakes := newFullStack(t)
		fakes.Entity.callCapabilityFn = func(_ context.Context, _ *connect.Request[v1.CallCapabilityRequest]) (*connect.Response[v1.CallCapabilityResponse], error) {
			return connect.NewResponse(&v1.CallCapabilityResponse{
				CorrelationId: "cid-1",
			}), nil
		}

		result, err := cs.CallTool(context.Background(), &sdk.CallToolParams{
			Name: "switchyard__call_capability",
			Arguments: map[string]any{
				"entity_id":  "light.a",
				"capability": "turn_on",
			},
		})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError)

		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(textFromResult(t, result)), &m))
		assert.Equal(t, "cid-1", m["correlation_id"])
	})

	t.Run("query_events", func(t *testing.T) {
		cs, fakes := newFullStack(t)
		fakes.Event.queryFn = func(_ context.Context, _ *connect.Request[v1.QueryEventsRequest]) (*connect.Response[v1.QueryEventsResponse], error) {
			return connect.NewResponse(&v1.QueryEventsResponse{
				Events: []*v1.Event{
					{Cursor: 1, Kind: "state_changed", Source: "test"},
				},
			}), nil
		}

		result, err := cs.CallTool(context.Background(), &sdk.CallToolParams{
			Name:      "switchyard__query_events",
			Arguments: map[string]any{"limit": 10},
		})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError)

		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(textFromResult(t, result)), &m))
		events, ok := m["events"].([]any)
		require.True(t, ok, "expected events array")
		assert.Len(t, events, 1)
	})

	t.Run("tail_events", func(t *testing.T) {
		cs, fakes := newFullStack(t)
		fakes.Event.tailFn = func(_ context.Context, _ *connect.Request[v1.TailEventsRequest], stream *connect.ServerStream[v1.TailEventsResponse]) error {
			return stream.Send(&v1.TailEventsResponse{
				Kind: &v1.TailEventsResponse_Event{
					Event: &v1.Event{Cursor: 1, Kind: "state_changed", Source: "test"},
				},
			})
		}

		result, err := cs.CallTool(context.Background(), &sdk.CallToolParams{
			Name:      "switchyard__tail_events",
			Arguments: map[string]any{"max_events": 1, "wait_seconds": 1},
		})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError)

		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(textFromResult(t, result)), &m))
		events, ok := m["events"].([]any)
		require.True(t, ok, "expected events array")
		assert.GreaterOrEqual(t, len(events), 1)
	})

	t.Run("apply_scene", func(t *testing.T) {
		cs, _ := newFullStack(t)
		// Scene service returns unimplemented — no fake setup needed.
		result, err := cs.CallTool(context.Background(), &sdk.CallToolParams{
			Name:      "switchyard__apply_scene",
			Arguments: map[string]any{"scene_id": "test-scene"},
		})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError, "expected IsError=true for unimplemented apply_scene")
	})

	t.Run("run_script", func(t *testing.T) {
		cs, fakes := newFullStack(t)
		fakes.Script.runFn = func(_ context.Context, _ *connect.Request[v1.RunScriptRequest]) (*connect.Response[v1.RunScriptResponse], error) {
			return connect.NewResponse(&v1.RunScriptResponse{
				RunId: "sc-1",
			}), nil
		}

		result, err := cs.CallTool(context.Background(), &sdk.CallToolParams{
			Name:      "switchyard__run_script",
			Arguments: map[string]any{"name": "my_script"},
		})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError)

		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(textFromResult(t, result)), &m))
		assert.Equal(t, "sc-1", m["run_id"])
	})

	t.Run("eval_starlark", func(t *testing.T) {
		cs, fakes := newFullStack(t)
		fakes.Script.evalFn = func(_ context.Context, _ *connect.Request[v1.EvalScriptRequest]) (*connect.Response[v1.EvalScriptResponse], error) {
			return connect.NewResponse(&v1.EvalScriptResponse{
				Stdout:     "hello",
				DurationMs: 1,
			}), nil
		}

		result, err := cs.CallTool(context.Background(), &sdk.CallToolParams{
			Name:      "switchyard__eval_starlark",
			Arguments: map[string]any{"source": "print('hello')"},
		})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError)

		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(textFromResult(t, result)), &m))
		assert.Equal(t, "hello", m["stdout"])
	})

	t.Run("validate_config", func(t *testing.T) {
		cs, fakes := newFullStack(t)
		fakes.Config.validateFn = func(_ context.Context, _ *connect.Request[v1.ValidateConfigRequest]) (*connect.Response[v1.ValidateConfigResponse], error) {
			return connect.NewResponse(&v1.ValidateConfigResponse{
				Valid: true,
			}), nil
		}

		result, err := cs.CallTool(context.Background(), &sdk.CallToolParams{
			Name:      "switchyard__validate_config",
			Arguments: map[string]any{"pkl_bundle": "amodule.Config {}"},
		})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError)

		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(textFromResult(t, result)), &m))
		assert.Equal(t, true, m["valid"])
	})

	t.Run("apply_config", func(t *testing.T) {
		cs, fakes := newFullStack(t)
		fakes.Config.applyFn = func(_ context.Context, _ *connect.Request[v1.ApplyConfigRequest]) (*connect.Response[v1.ApplyConfigResponse], error) {
			return connect.NewResponse(&v1.ApplyConfigResponse{
				Applied: true,
			}), nil
		}

		result, err := cs.CallTool(context.Background(), &sdk.CallToolParams{
			Name: "switchyard__apply_config",
			Arguments: map[string]any{
				"pkl_bundle": "amodule.Config {}",
				"message":    "test",
			},
		})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError)

		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(textFromResult(t, result)), &m))
		assert.Equal(t, true, m["applied"])
	})

	t.Run("read_config_file", func(t *testing.T) {
		configDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(configDir, "hello.yaml"), []byte("hello"), 0o644))

		cs, _ := newFullStack(t, configDir)

		result, err := cs.CallTool(context.Background(), &sdk.CallToolParams{
			Name:      "switchyard__read_config_file",
			Arguments: map[string]any{"path": "hello.yaml"},
		})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError)

		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(textFromResult(t, result)), &m))
		assert.Equal(t, "hello", m["content"])
	})

	t.Run("write_config_file", func(t *testing.T) {
		configDir := t.TempDir()
		cs, _ := newFullStack(t, configDir)

		result, err := cs.CallTool(context.Background(), &sdk.CallToolParams{
			Name:      "switchyard__write_config_file",
			Arguments: map[string]any{"path": "out.pkl", "content": "amends \"example\"\n// ok\n"},
		})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError)

		// Verify file was actually written.
		_, statErr := os.Stat(filepath.Join(configDir, "out.pkl"))
		assert.NoError(t, statErr, "out.pkl should exist on disk")
	})
}

// ---------------------------------------------------------------------------
// TestFullStack_EntitySubscribe
// ---------------------------------------------------------------------------

func TestFullStack_EntitySubscribe(t *testing.T) {
	fakes := &testFakes{
		System: &fullFakeSystemSvc{},
		Entity: &fullFakeEntitySvc{},
		Event:  &fullFakeEventSvc{},
		Scene:  &fullFakeSceneSvc{},
		Script: &fullFakeScriptSvc{},
		Config: &fullFakeConfigSvc{},
	}

	// Configure Entity.Subscribe to send 1 change then block until cancelled.
	fakes.Entity.subscribeFn = func(ctx context.Context, _ *connect.Request[v1.SubscribeEntitiesRequest], stream *connect.ServerStream[v1.SubscribeEntitiesResponse]) error {
		_ = stream.Send(&v1.SubscribeEntitiesResponse{
			Kind: &v1.SubscribeEntitiesResponse_Change{
				Change: &v1.EntityChange{
					Entity: &v1.Entity{Id: "light.x"},
				},
			},
		})
		<-ctx.Done()
		return nil
	}

	mux := http.NewServeMux()
	p, h := switchyardv1alpha1connect.NewSystemServiceHandler(fakes.System)
	mux.Handle(p, h)
	p, h = switchyardv1alpha1connect.NewEntityServiceHandler(fakes.Entity)
	mux.Handle(p, h)
	p, h = switchyardv1alpha1connect.NewEventServiceHandler(fakes.Event)
	mux.Handle(p, h)
	p, h = switchyardv1alpha1connect.NewSceneServiceHandler(fakes.Scene)
	mux.Handle(p, h)
	p, h = switchyardv1alpha1connect.NewScriptServiceHandler(fakes.Script)
	mux.Handle(p, h)
	p, h = switchyardv1alpha1connect.NewConfigServiceHandler(fakes.Config)
	mux.Handle(p, h)
	p, h = switchyardv1alpha1connect.NewAutomationServiceHandler(switchyardv1alpha1connect.UnimplementedAutomationServiceHandler{})
	mux.Handle(p, h)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, err := mcp.NewClient(mcp.ClientOptions{EndpointURL: srv.URL})
	require.NoError(t, err)

	caps := mcp.MCPCaps{TailDefaultWaitSeconds: 5, TailMaxWaitSeconds: 10, ReadFileMaxBytes: 1024 * 1024}
	m := observability.NewMetrics()

	resDeps := resources.Deps{Client: c, MCPCaps: caps, Metrics: m}
	opts, setServer, shutdown := resources.NewServerOpts(resDeps)
	t.Cleanup(shutdown)

	impl := &sdk.Implementation{Name: "test-subscribe", Version: "0"}
	s := sdk.NewServer(impl, opts)
	setServer(s)

	tools.Register(tools.Deps{
		Server:    s,
		Client:    c,
		ConfigDir: t.TempDir(),
		MCPCaps:   caps,
		Auth:      auth.AllowAll{},
		Audit:     audit.NewRecorder(c.System),
	})
	resources.Register(s, resDeps)

	// Set up client with ResourceUpdatedHandler.
	// Buffered so the handler doesn't drop the notification if it fires
	// before the test goroutine reaches the select below.
	notified := make(chan struct{}, 1)
	clientOpts := &sdk.ClientOptions{
		ResourceUpdatedHandler: func(_ context.Context, _ *sdk.ResourceUpdatedNotificationRequest) {
			select {
			case notified <- struct{}{}:
			default:
			}
		},
	}

	ct, st := sdk.NewInMemoryTransports()
	_, err = s.Connect(context.Background(), st, nil)
	require.NoError(t, err, "server Connect")

	client := sdk.NewClient(impl, clientOpts)
	cs, err := client.Connect(context.Background(), ct, nil)
	require.NoError(t, err, "client Connect")
	t.Cleanup(func() { cs.Close() })

	// Subscribe to the entity resource.
	err = cs.Subscribe(context.Background(), &sdk.SubscribeParams{URI: "switchyard://entities/light.x"})
	require.NoError(t, err)

	// Wait for ResourceUpdated notification (5s timeout).
	select {
	case <-notified:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for ResourceUpdated notification")
	}

	// Unsubscribe — subscription manager should clean up.
	err = cs.Unsubscribe(context.Background(), &sdk.UnsubscribeParams{URI: "switchyard://entities/light.x"})
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// TestFullStack_ReadEntityResource
// ---------------------------------------------------------------------------

func TestFullStack_ReadEntityResource(t *testing.T) {
	cs, fakes := newFullStack(t)

	fakes.Entity.getFn = func(_ context.Context, req *connect.Request[v1.GetEntityRequest]) (*connect.Response[v1.GetEntityResponse], error) {
		return connect.NewResponse(&v1.GetEntityResponse{
			Entity: &v1.Entity{Id: req.Msg.Id, Type: "light", FriendlyName: "Kitchen Light"},
		}), nil
	}

	result, err := cs.ReadResource(context.Background(), &sdk.ReadResourceParams{URI: "switchyard://entities/light.x"})
	require.NoError(t, err)
	require.Len(t, result.Contents, 1)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Contents[0].Text), &m))
	assert.Equal(t, "light.x", m["id"])
	assert.Equal(t, "Kitchen Light", m["name"])
	assert.NotContains(t, m, "friendly_name")
}
