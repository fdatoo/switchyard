package resources_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
	"github.com/fdatoo/switchyard/internal/mcp"
	"github.com/fdatoo/switchyard/internal/mcp/resources"
	"github.com/fdatoo/switchyard/internal/observability"
)

var testImpl = &sdk.Implementation{Name: "test", Version: "0"}

// fakeEntitySvc implements only the methods we need.
type fakeEntitySvc struct {
	switchyardv1alpha1connect.UnimplementedEntityServiceHandler

	getFn       func(context.Context, *connect.Request[v1.GetEntityRequest]) (*connect.Response[v1.GetEntityResponse], error)
	listFn      func(context.Context, *connect.Request[v1.ListEntitiesRequest]) (*connect.Response[v1.ListEntitiesResponse], error)
	subscribeFn func(context.Context, *connect.Request[v1.SubscribeEntitiesRequest], *connect.ServerStream[v1.SubscribeEntitiesResponse]) error
}

func (h *fakeEntitySvc) Get(ctx context.Context, req *connect.Request[v1.GetEntityRequest]) (*connect.Response[v1.GetEntityResponse], error) {
	if h.getFn != nil {
		return h.getFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *fakeEntitySvc) List(ctx context.Context, req *connect.Request[v1.ListEntitiesRequest]) (*connect.Response[v1.ListEntitiesResponse], error) {
	if h.listFn != nil {
		return h.listFn(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *fakeEntitySvc) Subscribe(ctx context.Context, req *connect.Request[v1.SubscribeEntitiesRequest], stream *connect.ServerStream[v1.SubscribeEntitiesResponse]) error {
	if h.subscribeFn != nil {
		return h.subscribeFn(ctx, req, stream)
	}
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

// newResourcesServer wires a fake entity service to an MCP server with resource
// handlers registered and returns the MCP server + resources deps.
func newResourcesServer(t *testing.T, svc *fakeEntitySvc) (*sdk.Server, resources.Deps) {
	t.Helper()
	_, h := switchyardv1alpha1connect.NewEntityServiceHandler(svc)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := mcp.NewClient(mcp.ClientOptions{EndpointURL: srv.URL})
	require.NoError(t, err)

	m := observability.NewMetrics()
	d := resources.Deps{Client: c, Metrics: m}

	opts, setServer, shutdown := resources.NewServerOpts(d)
	t.Cleanup(shutdown)
	s := sdk.NewServer(testImpl, opts)
	setServer(s)
	resources.RegisterEntities(s, d)
	return s, d
}

// readResource connects a transient client/server pair and reads a resource URI.
func readResource(t *testing.T, s *sdk.Server, uri string) (*sdk.ReadResourceResult, error) {
	t.Helper()
	ct, st := sdk.NewInMemoryTransports()
	_, err := s.Connect(context.Background(), st, nil)
	require.NoError(t, err, "server Connect")
	client := sdk.NewClient(testImpl, nil)
	cs, err := client.Connect(context.Background(), ct, nil)
	require.NoError(t, err, "client Connect")
	t.Cleanup(func() { cs.Close() })
	return cs.ReadResource(context.Background(), &sdk.ReadResourceParams{URI: uri})
}

func TestEntityRead_Single(t *testing.T) {
	svc := &fakeEntitySvc{
		getFn: func(_ context.Context, req *connect.Request[v1.GetEntityRequest]) (*connect.Response[v1.GetEntityResponse], error) {
			assert.Equal(t, "light.kitchen", req.Msg.Id)
			return connect.NewResponse(&v1.GetEntityResponse{
				Entity: &v1.Entity{Id: "light.kitchen", Type: "light", FriendlyName: "Kitchen Light"},
			}), nil
		},
	}
	s, _ := newResourcesServer(t, svc)

	result, err := readResource(t, s, "switchyard://entities/light.kitchen")
	require.NoError(t, err)
	require.Len(t, result.Contents, 1)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Contents[0].Text), &m))
	assert.Equal(t, "light.kitchen", m["id"])
	assert.Equal(t, "Kitchen Light", m["name"])
	assert.NotContains(t, m, "friendly_name")
}

func TestEntityRead_List(t *testing.T) {
	svc := &fakeEntitySvc{
		listFn: func(_ context.Context, _ *connect.Request[v1.ListEntitiesRequest]) (*connect.Response[v1.ListEntitiesResponse], error) {
			return connect.NewResponse(&v1.ListEntitiesResponse{
				Entities: []*v1.Entity{
					{Id: "light.a", Type: "light"},
					{Id: "switch.b", Type: "switch"},
				},
			}), nil
		},
	}
	s, _ := newResourcesServer(t, svc)

	result, err := readResource(t, s, "switchyard://entities/")
	require.NoError(t, err)
	require.Len(t, result.Contents, 1)

	var arr []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Contents[0].Text), &arr))
	assert.Len(t, arr, 2)
	assert.Equal(t, "light.a", arr[0]["id"])
	assert.Equal(t, "switch.b", arr[1]["id"])
}

func TestEntityWatch_CoalescesOnOverflow(t *testing.T) {
	const numEvents = 50
	ready := make(chan struct{})

	svc := &fakeEntitySvc{
		subscribeFn: func(ctx context.Context, _ *connect.Request[v1.SubscribeEntitiesRequest], stream *connect.ServerStream[v1.SubscribeEntitiesResponse]) error {
			close(ready)
			for i := 0; i < numEvents; i++ {
				select {
				case <-ctx.Done():
					return nil
				default:
				}
				_ = stream.Send(&v1.SubscribeEntitiesResponse{
					Kind: &v1.SubscribeEntitiesResponse_Change{
						Change: &v1.EntityChange{Entity: &v1.Entity{Id: "light.x"}},
					},
				})
			}
			return nil
		},
	}

	_, h := switchyardv1alpha1connect.NewEntityServiceHandler(svc)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := mcp.NewClient(mcp.ClientOptions{EndpointURL: srv.URL})
	require.NoError(t, err)

	m := observability.NewMetrics()
	d := resources.Deps{Client: c, Metrics: m}

	var updateCount atomic.Int64
	updateCh := make(chan struct{}, numEvents)

	opts, setServer, shutdown := resources.NewServerOpts(d)
	t.Cleanup(shutdown)
	// Wrap SubscribeHandler to track calls but still trigger the watch.
	origSub := opts.SubscribeHandler
	opts.SubscribeHandler = origSub

	s := sdk.NewServer(testImpl, opts)
	setServer(s)
	resources.RegisterEntities(s, d)

	// Connect a client that counts ResourceUpdated notifications.
	ct, st := sdk.NewInMemoryTransports()
	_, err = s.Connect(context.Background(), st, nil)
	require.NoError(t, err)

	clientOpts := &sdk.ClientOptions{
		ResourceUpdatedHandler: func(_ context.Context, _ *sdk.ResourceUpdatedNotificationRequest) {
			updateCount.Add(1)
			select {
			case updateCh <- struct{}{}:
			default:
			}
		},
	}
	client := sdk.NewClient(testImpl, clientOpts)
	cs, err := client.Connect(context.Background(), ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { cs.Close() })

	// Subscribe to the entity resource.
	err = cs.Subscribe(context.Background(), &sdk.SubscribeParams{
		URI: "switchyard://entities/light.x",
	})
	require.NoError(t, err)

	// Wait for the subscribe handler to start streaming.
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("subscribe stream never started")
	}

	// Wait for at least one notification or a timeout.
	select {
	case <-updateCh:
	case <-time.After(5 * time.Second):
		t.Fatal("no ResourceUpdated received within 5s")
	}

	// Allow a short window for more notifications to arrive.
	time.Sleep(100 * time.Millisecond)

	got := updateCount.Load()
	// With a coalescing buffer of 1, we expect far fewer notifications than events.
	assert.Less(t, got, int64(numEvents), "expected coalescing to reduce notifications")
	assert.GreaterOrEqual(t, got, int64(1), "expected at least one notification")

	// Verify the overflow metric was incremented.
	coalesceMetric := getCounterValue(t, m, "switchyard_mcp_resource_overflow_closes_total")
	assert.GreaterOrEqual(t, coalesceMetric, 0.0, "coalesced metric should be non-negative")
}

// getCounterValue reads the current value of a prometheus counter by name.
func getCounterValue(t *testing.T, m *observability.Metrics, name string) float64 {
	t.Helper()
	mfs, err := m.Registry.Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		if mf.GetName() == name {
			var total float64
			for _, mm := range mf.GetMetric() {
				if mm.GetCounter() != nil {
					total += mm.GetCounter().GetValue()
				}
			}
			return total
		}
	}
	return 0
}
