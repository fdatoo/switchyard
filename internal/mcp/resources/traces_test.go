package resources_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/gen/gohome/v1alpha1/gohomev1alpha1connect"
	"github.com/fdatoo/gohome/internal/mcp"
	"github.com/fdatoo/gohome/internal/mcp/resources"
	"github.com/fdatoo/gohome/internal/observability"
)

// fakeAutoSvc implements only the Trace method.
type fakeAutoSvc struct {
	gohomev1alpha1connect.UnimplementedAutomationServiceHandler
	traceFn func(context.Context, *connect.Request[v1.TraceAutomationRequest], *connect.ServerStream[v1.TraceAutomationResponse]) error
}

func (h *fakeAutoSvc) Trace(ctx context.Context, req *connect.Request[v1.TraceAutomationRequest], stream *connect.ServerStream[v1.TraceAutomationResponse]) error {
	if h.traceFn != nil {
		return h.traceFn(ctx, req, stream)
	}
	return connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

// newTraceResourcesServer wires a fake automation service and returns an MCP server.
func newTraceResourcesServer(t *testing.T, svc *fakeAutoSvc, caps mcp.MCPCaps) (*sdk.Server, *observability.Metrics) {
	t.Helper()
	_, h := gohomev1alpha1connect.NewAutomationServiceHandler(svc)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := mcp.NewClient(mcp.ClientOptions{EndpointURL: srv.URL})
	require.NoError(t, err)

	m := observability.NewMetrics()
	d := resources.Deps{Client: c, MCPCaps: caps, Metrics: m}

	opts, setServer, shutdown := resources.NewServerOpts(d)
	t.Cleanup(shutdown)
	s := sdk.NewServer(testImpl, opts)
	setServer(s)
	resources.RegisterTraces(s, d)
	return s, m
}

func TestTraceRead_HappyPath(t *testing.T) {
	callCount := 0
	svc := &fakeAutoSvc{
		traceFn: func(_ context.Context, req *connect.Request[v1.TraceAutomationRequest], stream *connect.ServerStream[v1.TraceAutomationResponse]) error {
			callCount++
			assert.Equal(t, "lights-auto", req.Msg.Id)
			assert.Equal(t, "run-abc", req.Msg.RunId)
			events := []*v1.TraceEvent{
				{Cursor: 1, Kind: "trigger_fired"},
				{Cursor: 2, Kind: "condition_pass"},
			}
			for _, e := range events {
				if err := stream.Send(&v1.TraceAutomationResponse{
					Kind: &v1.TraceAutomationResponse_Event{Event: e},
				}); err != nil {
					return err
				}
			}
			return nil
		},
	}
	s, _ := newTraceResourcesServer(t, svc, mcp.MCPCaps{})

	result, err := readResource(t, s, "gohome://automations/lights-auto/runs/run-abc/trace")
	require.NoError(t, err)
	require.Len(t, result.Contents, 1)

	var arr []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Contents[0].Text), &arr))
	assert.Len(t, arr, 2)
	assert.Equal(t, "trigger_fired", arr[0]["kind"])
	assert.Equal(t, "condition_pass", arr[1]["kind"])
}

func TestTraceWatch_OverflowCloses(t *testing.T) {
	const bufSize = 4
	const sendCount = bufSize + 2 // enough to overflow

	ready := make(chan struct{})
	svc := &fakeAutoSvc{
		traceFn: func(ctx context.Context, _ *connect.Request[v1.TraceAutomationRequest], stream *connect.ServerStream[v1.TraceAutomationResponse]) error {
			close(ready)
			for i := 0; i < sendCount; i++ {
				select {
				case <-ctx.Done():
					return nil
				default:
				}
				_ = stream.Send(&v1.TraceAutomationResponse{
					Kind: &v1.TraceAutomationResponse_Event{
						Event: &v1.TraceEvent{Cursor: uint64(i + 1), Kind: "trigger_fired"},
					},
				})
			}
			// Keep stream open so the watch goroutine has time to overflow.
			<-ctx.Done()
			return nil
		},
	}

	caps := mcp.MCPCaps{TraceSubscriptionBuffer: bufSize}
	s, m := newTraceResourcesServer(t, svc, caps)

	ct, st := sdk.NewInMemoryTransports()
	_, err := s.Connect(context.Background(), st, nil)
	require.NoError(t, err)

	updateCh := make(chan struct{}, sendCount*2)
	clientOpts := &sdk.ClientOptions{
		ResourceUpdatedHandler: func(_ context.Context, _ *sdk.ResourceUpdatedNotificationRequest) {
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

	const uri = "gohome://automations/myauto/runs/run-xyz/trace"
	err = cs.Subscribe(context.Background(), &sdk.SubscribeParams{URI: uri})
	require.NoError(t, err)

	// Wait for the stream to start.
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("trace stream never started")
	}

	// Wait for at least one update notification.
	select {
	case <-updateCh:
	case <-time.After(5 * time.Second):
		t.Fatal("no ResourceUpdated received within 5s")
	}

	// Give some time for overflow to happen.
	time.Sleep(200 * time.Millisecond)

	// Check that overflow metric was incremented.
	overflowMetric := getCounterValue(t, m, "gohome_mcp_resource_overflow_closes_total")
	assert.GreaterOrEqual(t, overflowMetric, 1.0, "expected at least one overflow close")
}
