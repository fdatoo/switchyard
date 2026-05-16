package api

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
)

// EventService implements historical event queries and live tails.
type EventService struct{ be EventSource }

// NewEventService returns an event service backed by be.
func NewEventService(be EventSource) *EventService { return &EventService{be: be} }

var _ switchyardv1alpha1connect.EventServiceHandler = (*EventService)(nil)

// Query returns historical events matching the request filter.
func (s *EventService) Query(ctx context.Context, req *connect.Request[v1.QueryEventsRequest]) (*connect.Response[v1.QueryEventsResponse], error) {
	cur, err := DecodeCursor(pageToken(req.Msg.Page))
	if err != nil {
		return nil, ToConnect(ctx, ErrValidationFailed, "bad_page_token")
	}
	filter := eventFilterFromProto(req.Msg.Filter)
	events, next, err := s.be.Query(ctx, filter, PageReq{Size: ClampPageSize(pageSize(req.Msg.Page)), Cursor: cur})
	if err != nil {
		return nil, ToConnect(ctx, err, "query_failed")
	}
	out := &v1.QueryEventsResponse{Page: &v1.PageResponse{}}
	if tok, _ := EncodeCursor(next); tok != "" {
		out.Page.NextPageToken = tok
	}
	for _, e := range events {
		out.Events = append(out.Events, eventToProto(e))
	}
	return connect.NewResponse(out), nil
}

// Tail streams live events and idle heartbeats.
func (s *EventService) Tail(ctx context.Context, req *connect.Request[v1.TailEventsRequest], stream *connect.ServerStream[v1.TailEventsResponse]) error {
	cfg := currentStreamConfig()
	filter := eventFilterFromProto(req.Msg.Filter)
	src, cancel, err := s.be.Subscribe(ctx, filter)
	if err != nil {
		return ToConnect(ctx, err, "subscribe_failed")
	}
	defer cancel()

	buffered, done := BoundedFanOut(ctx, src, cfg.BufSize)
	ticker := NewHeartbeatTicker(ctx, cfg.HeartbeatInterval)
	defer ticker.Stop()

	var latest uint64
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-done:
			if errors.Is(err, ErrSubscriptionOverflow) {
				return ToConnect(ctx, ErrSubscriptionOverflow, "subscription_overflow")
			}
			return nil
		case ev, ok := <-buffered:
			if !ok {
				return nil
			}
			latest = ev.Cursor
			if err := stream.Send(&v1.TailEventsResponse{
				Kind: &v1.TailEventsResponse_Event{Event: eventToProto(ev)},
			}); err != nil {
				return err
			}
			ticker.NotePayloadSent()
		case t := <-ticker.C():
			if err := stream.Send(&v1.TailEventsResponse{
				Kind: &v1.TailEventsResponse_Heartbeat{Heartbeat: &v1.Heartbeat{
					LatestCursor: latest, ServerTime: ProtoTime(t),
				}},
			}); err != nil {
				return err
			}
		}
	}
}

func eventFilterFromProto(f *v1.EventFilter) EventFilter {
	if f == nil {
		return EventFilter{}
	}
	return EventFilter{
		Kinds:        f.Kinds,
		EntityPrefix: f.EntityPrefix,
		Sources:      f.Sources,
		FromCursor:   f.FromCursor,
		ToCursor:     f.ToCursor,
		FromTime:     GoTime(f.FromTime),
		ToTime:       GoTime(f.ToTime),
	}
}

func eventToProto(e Event) *v1.Event {
	return &v1.Event{
		Cursor:        e.Cursor,
		At:            ProtoTime(e.At),
		Kind:          e.Kind,
		Entity:        e.Entity,
		Source:        e.Source,
		CorrelationId: e.CorrelationID,
		CauseId:       e.CauseID,
		Payload:       e.Payload,
	}
}
