package activity

import (
	"context"
	"fmt"
	"os"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	activityv1 "github.com/fdatoo/switchyard/gen/switchyard/activity/v1"
	"github.com/fdatoo/switchyard/gen/switchyard/activity/v1/activityv1connect"
	"github.com/fdatoo/switchyard/internal/activity/mock"
	"github.com/fdatoo/switchyard/internal/activity/stories"
	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/interestingness"
)

// Compile-time assertion that ActivityService implements the handler interface.
var _ activityv1connect.ActivityServiceHandler = (*ActivityService)(nil)

// storeReader is the event-query interface needed by the service.
type storeReader interface {
	stories.EventQuerier
}

// ActivityServiceConfig holds ActivityService configuration.
type ActivityServiceConfig struct {
	// Mock controls whether the service returns synthetic data instead of
	// querying the real event store. Set via SY_ACTIVITY_MOCK=1.
	Mock bool

	// SavedQueriesDir is the directory used to persist saved queries.
	SavedQueriesDir string
}

// IsMockEnabled returns true when SY_ACTIVITY_MOCK=1 is set in the environment.
func IsMockEnabled() bool {
	return os.Getenv("SY_ACTIVITY_MOCK") == "1"
}

// ActivityService implements the ActivityServiceHandler ConnectRPC interface.
type ActivityService struct {
	cfg       ActivityServiceConfig
	coalescer *stories.Coalescer
	savedQ    *SavedQueryStore
	detectors []interestingness.Detector
}

// NewActivityService creates an ActivityService.
// When cfg.Mock is true (or SY_ACTIVITY_MOCK=1) the service returns synthetic
// data and does not require a real event store.
func NewActivityService(store storeReader, cfg ActivityServiceConfig) *ActivityService {
	if !cfg.Mock {
		cfg.Mock = IsMockEnabled()
	}

	var coalescer *stories.Coalescer
	if store != nil {
		coalescer = stories.NewCoalescer(store, stories.CoalescerConfig{})
	}

	savedQDir := cfg.SavedQueriesDir
	if savedQDir == "" {
		savedQDir = os.TempDir() + "/switchyard-saved-queries"
	}

	return &ActivityService{
		cfg:       cfg,
		coalescer: coalescer,
		savedQ:    NewSavedQueryStore(savedQDir),
		detectors: interestingness.DefaultDetectors(),
	}
}

// Stories streams Story groups in reverse-chronological order.
func (s *ActivityService) Stories(
	ctx context.Context,
	req *connect.Request[activityv1.StoriesRequest],
	stream *connect.ServerStream[activityv1.StoriesResponse],
) error {
	if s.cfg.Mock {
		return s.storiesMock(ctx, req, stream)
	}
	return s.storiesLive(ctx, req, stream)
}

func (s *ActivityService) storiesMock(
	ctx context.Context,
	req *connect.Request[activityv1.StoriesRequest],
	stream *connect.ServerStream[activityv1.StoriesResponse],
) error {
	syntheticStories := mock.GenerateStories(ctx)
	filter := req.Msg.GetFilter()

	for _, story := range syntheticStories {
		if !matchesStoriesFilter(story, filter) {
			continue
		}
		if err := stream.Send(&activityv1.StoriesResponse{Story: story}); err != nil {
			return err
		}
	}
	return nil
}

func (s *ActivityService) storiesLive(
	ctx context.Context,
	req *connect.Request[activityv1.StoriesRequest],
	stream *connect.ServerStream[activityv1.StoriesResponse],
) error {
	if s.coalescer == nil {
		return connect.NewError(connect.CodeUnavailable, fmt.Errorf("event store not available"))
	}

	filter := req.Msg.GetFilter()
	since, until := filterWindow(filter.GetSince(), filter.GetUntil())

	storyList, err := s.coalescer.CoalesceWindow(ctx, since, until)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("coalesce: %w", err))
	}

	for _, story := range storyList {
		proto := story.ToProto()
		if !matchesStoriesFilter(proto, filter) {
			continue
		}
		if err := stream.Send(&activityv1.StoriesResponse{Story: proto}); err != nil {
			return err
		}
	}
	return nil
}

// Events streams individual event records.
func (s *ActivityService) Events(
	ctx context.Context,
	req *connect.Request[activityv1.EventsRequest],
	stream *connect.ServerStream[activityv1.EventsResponse],
) error {
	if s.cfg.Mock {
		return s.eventsMock(ctx, req, stream)
	}
	return s.eventsLive(ctx, req, stream)
}

func (s *ActivityService) eventsMock(
	ctx context.Context,
	req *connect.Request[activityv1.EventsRequest],
	stream *connect.ServerStream[activityv1.EventsResponse],
) error {
	events := mock.GenerateEvents(ctx)
	filter := req.Msg.GetFilter()

	for _, ev := range events {
		if filter != nil && filter.Kind != "" && ev.Kind != filter.Kind {
			continue
		}
		if err := stream.Send(&activityv1.EventsResponse{Event: ev}); err != nil {
			return err
		}
	}
	return nil
}

func (s *ActivityService) eventsLive(
	ctx context.Context,
	req *connect.Request[activityv1.EventsRequest],
	stream *connect.ServerStream[activityv1.EventsResponse],
) error {
	if s.coalescer == nil {
		return connect.NewError(connect.CodeUnavailable, fmt.Errorf("event store not available"))
	}

	filter := req.Msg.GetFilter()
	since, until := filterWindow(filter.GetSince(), filter.GetUntil())

	events, err := s.coalescer.QueryEvents(ctx, since, until, filter.GetKind())
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("events query: %w", err))
	}

	for _, e := range events {
		if e.Kind == "interestingness.tagged" {
			continue
		}
		rec := eventToProto(e)
		if err := stream.Send(&activityv1.EventsResponse{Event: rec}); err != nil {
			return err
		}
	}
	return nil
}

// EventDetail returns a single event with tags and causation chain.
func (s *ActivityService) EventDetail(
	ctx context.Context,
	req *connect.Request[activityv1.EventDetailRequest],
) (*connect.Response[activityv1.EventDetailResponse], error) {
	if s.cfg.Mock {
		events := mock.GenerateEvents(ctx)
		for _, ev := range events {
			if ev.EventId == req.Msg.EventId {
				return connect.NewResponse(&activityv1.EventDetailResponse{
					Event:           ev,
					CausationChain: nil,
				}), nil
			}
		}
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("event %q not found", req.Msg.EventId))
	}
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("EventDetail requires event store integration"))
}

// SaveQuery persists a named query.
func (s *ActivityService) SaveQuery(
	ctx context.Context,
	req *connect.Request[activityv1.SaveQueryRequest],
) (*connect.Response[activityv1.SaveQueryResponse], error) {
	q, err := s.savedQ.Save(ctx, req.Msg.Name, req.Msg.Filter, req.Msg.Cron)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save query: %w", err))
	}
	return connect.NewResponse(&activityv1.SaveQueryResponse{Query: q}), nil
}

// ListSavedQueries returns all saved queries.
func (s *ActivityService) ListSavedQueries(
	ctx context.Context,
	_ *connect.Request[activityv1.ListSavedQueriesRequest],
) (*connect.Response[activityv1.ListSavedQueriesResponse], error) {
	queries, err := s.savedQ.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list queries: %w", err))
	}
	return connect.NewResponse(&activityv1.ListSavedQueriesResponse{Queries: queries}), nil
}

// DeleteSavedQuery removes a saved query by id.
func (s *ActivityService) DeleteSavedQuery(
	ctx context.Context,
	req *connect.Request[activityv1.DeleteSavedQueryRequest],
) (*connect.Response[activityv1.DeleteSavedQueryResponse], error) {
	if err := s.savedQ.Delete(ctx, req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("delete query: %w", err))
	}
	return connect.NewResponse(&activityv1.DeleteSavedQueryResponse{}), nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func filterWindow(since, until *timestamppb.Timestamp) (time.Time, time.Time) {
	var s, u time.Time
	if since != nil {
		s = since.AsTime()
	} else {
		s = time.Now().Add(-24 * time.Hour)
	}
	if until != nil {
		u = until.AsTime()
	}
	return s, u
}

func kindFilter(kind string) []string {
	if kind == "" {
		return nil
	}
	return []string{kind}
}

func matchesStoriesFilter(story *activityv1.Story, filter *activityv1.StoriesFilter) bool {
	if filter == nil {
		return true
	}
	if filter.InterestingOnly && len(story.Tags) == 0 {
		return false
	}
	if filter.InterestingCategory != "" {
		found := false
		for _, t := range story.Tags {
			if t.Category == filter.InterestingCategory {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func eventToProto(e eventstore.Event) *activityv1.EventRecord {
	rec := &activityv1.EventRecord{
		EventId:       fmt.Sprintf("%d", e.Position),
		CorrelationId: e.CorrelationID.String(),
		Kind:          e.Kind,
		Entity:        e.Entity,
		Source:        e.Source,
		Sequence:      e.Position,
		OccurredAt:    timestamppb.New(e.Timestamp),
	}
	if e.CausePosition > 0 {
		rec.CausationId = fmt.Sprintf("%d", e.CausePosition)
	}
	return rec
}
