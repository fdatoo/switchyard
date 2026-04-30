package api_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/internal/api"
)

type fakeEvents struct {
	events []api.Event
}

func (f *fakeEvents) Query(_ context.Context, filter api.EventFilter, _ api.PageReq) ([]api.Event, api.Cursor, error) {
	var out []api.Event
	for _, e := range f.events {
		if len(filter.Kinds) > 0 && !containsStr(filter.Kinds, e.Kind) {
			continue
		}
		out = append(out, e)
	}
	return out, api.Cursor{}, nil
}

func (f *fakeEvents) Subscribe(_ context.Context, _ api.EventFilter) (<-chan api.Event, func(), error) {
	ch := make(chan api.Event)
	return ch, func() { close(ch) }, nil
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func TestEventService_Query_KindFilter(t *testing.T) {
	fe := &fakeEvents{events: []api.Event{
		{Cursor: 1, Kind: "state_changed", At: time.Unix(1700000000, 0)},
		{Cursor: 2, Kind: "command_issued", At: time.Unix(1700000001, 0)},
	}}
	s := api.NewEventService(fe)
	resp, err := s.Query(context.Background(), connect.NewRequest(&v1.QueryEventsRequest{
		Filter: &v1.EventFilter{Kinds: []string{"state_changed"}},
	}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(resp.Msg.Events) != 1 || resp.Msg.Events[0].Kind != "state_changed" {
		t.Errorf("unexpected events: %+v", resp.Msg.Events)
	}
}

func TestEventService_Query_NoFilter(t *testing.T) {
	fe := &fakeEvents{events: []api.Event{
		{Cursor: 1, Kind: "state_changed"},
		{Cursor: 2, Kind: "command_issued"},
	}}
	s := api.NewEventService(fe)
	resp, err := s.Query(context.Background(), connect.NewRequest(&v1.QueryEventsRequest{}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(resp.Msg.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(resp.Msg.Events))
	}
}
