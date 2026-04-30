package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
	v1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/gen/gohome/v1alpha1/gohomev1alpha1connect"
	"github.com/fdatoo/gohome/internal/api"
)

// liveEventSource is a test EventSource backed by a channel.
type liveEventSource struct {
	ch chan api.Event
}

func newLiveEventSource() *liveEventSource {
	return &liveEventSource{ch: make(chan api.Event, 16)}
}

func (l *liveEventSource) Query(_ context.Context, _ api.EventFilter, _ api.PageReq) ([]api.Event, api.Cursor, error) {
	return nil, api.Cursor{}, nil
}

func (l *liveEventSource) Subscribe(_ context.Context, _ api.EventFilter) (<-chan api.Event, func(), error) {
	return l.ch, func() {}, nil
}

func TestEventService_Tail_StreamsEventAndHeartbeat(t *testing.T) {
	api.SetStreamConfig(api.StreamConfig{HeartbeatInterval: 50 * time.Millisecond, BufSize: 4})
	defer api.SetStreamConfig(api.DefaultStreamConfig())

	src := newLiveEventSource()
	svc := api.NewEventService(src)

	mux := http.NewServeMux()
	path, handler := gohomev1alpha1connect.NewEventServiceHandler(svc)
	mux.Handle(path, handler)

	srv := httptest.NewUnstartedServer(h2c.NewHandler(mux, &http2.Server{}))
	srv.Start()
	defer srv.Close()

	client := gohomev1alpha1connect.NewEventServiceClient(
		srv.Client(),
		srv.URL,
		connect.WithGRPC(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		time.Sleep(10 * time.Millisecond)
		src.ch <- api.Event{
			Cursor:  1,
			Kind:    "state_changed",
			At:      time.Unix(1700, 0),
			Payload: &eventv1.Payload{},
		}
		time.Sleep(200 * time.Millisecond) // let heartbeat fire
		close(src.ch)
	}()

	stream, err := client.Tail(ctx, connect.NewRequest(&v1.TailEventsRequest{}))
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}

	var sawEvent, sawHB bool
	for stream.Receive() {
		msg := stream.Msg()
		if msg.GetEvent() != nil {
			sawEvent = true
		}
		if msg.GetHeartbeat() != nil {
			sawHB = true
		}
		if sawEvent && sawHB {
			break
		}
	}
	if err := stream.Err(); err != nil {
		t.Logf("stream closed: %v", err)
	}
	if !sawEvent {
		t.Error("no event received")
	}
	if !sawHB {
		t.Error("no heartbeat received")
	}
}
