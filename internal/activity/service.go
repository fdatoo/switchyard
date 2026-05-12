package activity

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
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
	store     storeReader // direct store access for the EventDetail handler
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
		store:     store,
		coalescer: coalescer,
		savedQ:    NewSavedQueryStore(savedQDir),
		detectors: interestingness.DefaultDetectors(),
	}
}

// ListStories returns stories as a slice — used by tests and internally.
func (s *ActivityService) ListStories(ctx context.Context, req *activityv1.StoriesRequest) []*activityv1.Story {
	if s.cfg.Mock {
		syntheticStories := mock.GenerateStories(ctx)
		filter := req.GetFilter()
		var out []*activityv1.Story
		for _, story := range syntheticStories {
			if matchesStoriesFilter(story, filter) {
				out = append(out, story)
			}
		}
		return out
	}
	if s.coalescer == nil {
		return nil
	}
	filter := req.GetFilter()
	since, until := filterWindow(filter.GetSince(), filter.GetUntil())
	storyList, err := s.coalescer.CoalesceWindow(ctx, since, until)
	if err != nil {
		return nil
	}
	var out []*activityv1.Story
	for _, story := range storyList {
		proto := story.ToProto()
		if matchesStoriesFilter(proto, filter) {
			out = append(out, proto)
		}
	}
	return out
}

// ListEvents returns events as a slice — used by tests and internally.
func (s *ActivityService) ListEvents(ctx context.Context, req *activityv1.EventsRequest) []*activityv1.EventRecord {
	if s.cfg.Mock {
		events := mock.GenerateEvents(ctx)
		filter := req.GetFilter()
		var out []*activityv1.EventRecord
		for _, ev := range events {
			if filter != nil && filter.Kind != "" && ev.Kind != filter.Kind {
				continue
			}
			out = append(out, ev)
		}
		return out
	}
	return nil
}

// maxPageSize is the maximum number of items returned in a single Stories or
// Events response. Callers use the next_cursor field to paginate.
const maxPageSize = 200

// Stories returns Story groups in reverse-chronological order.
func (s *ActivityService) Stories(
	ctx context.Context,
	req *connect.Request[activityv1.StoriesRequest],
) (*connect.Response[activityv1.StoriesResponse], error) {
	if s.cfg.Mock {
		return s.storiesMock(ctx, req)
	}
	return s.storiesLive(ctx, req)
}

func (s *ActivityService) storiesMock(
	ctx context.Context,
	req *connect.Request[activityv1.StoriesRequest],
) (*connect.Response[activityv1.StoriesResponse], error) {
	syntheticStories := mock.GenerateStories(ctx)
	filter := req.Msg.GetFilter()

	var page []*activityv1.Story
	for _, story := range syntheticStories {
		if !matchesStoriesFilter(story, filter) {
			continue
		}
		page = append(page, story)
		if len(page) >= maxPageSize {
			break
		}
	}
	return connect.NewResponse(&activityv1.StoriesResponse{Stories: page}), nil
}

func (s *ActivityService) storiesLive(
	ctx context.Context,
	req *connect.Request[activityv1.StoriesRequest],
) (*connect.Response[activityv1.StoriesResponse], error) {
	if s.coalescer == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("event store not available"))
	}

	filter := req.Msg.GetFilter()
	since, until := filterWindow(filter.GetSince(), filter.GetUntil())

	storyList, err := s.coalescer.CoalesceWindow(ctx, since, until)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("coalesce: %w", err))
	}

	var page []*activityv1.Story
	for _, story := range storyList {
		proto := story.ToProto()
		if !matchesStoriesFilter(proto, filter) {
			continue
		}
		page = append(page, proto)
		if len(page) >= maxPageSize {
			break
		}
	}
	return connect.NewResponse(&activityv1.StoriesResponse{Stories: page}), nil
}

// Events returns individual event records.
func (s *ActivityService) Events(
	ctx context.Context,
	req *connect.Request[activityv1.EventsRequest],
) (*connect.Response[activityv1.EventsResponse], error) {
	if s.cfg.Mock {
		return s.eventsMock(ctx, req)
	}
	return s.eventsLive(ctx, req)
}

func (s *ActivityService) eventsMock(
	ctx context.Context,
	req *connect.Request[activityv1.EventsRequest],
) (*connect.Response[activityv1.EventsResponse], error) {
	events := mock.GenerateEvents(ctx)
	filter := req.Msg.GetFilter()

	var page []*activityv1.EventRecord
	for _, ev := range events {
		if filter != nil && filter.Kind != "" && ev.Kind != filter.Kind {
			continue
		}
		page = append(page, ev)
		if len(page) >= maxPageSize {
			break
		}
	}
	return connect.NewResponse(&activityv1.EventsResponse{Events: page}), nil
}

func (s *ActivityService) eventsLive(
	ctx context.Context,
	req *connect.Request[activityv1.EventsRequest],
) (*connect.Response[activityv1.EventsResponse], error) {
	if s.coalescer == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("event store not available"))
	}

	filter := req.Msg.GetFilter()
	since, until := filterWindow(filter.GetSince(), filter.GetUntil())

	// First pass: window-scoped events plus their kind filter. We do NOT
	// apply `kind` to a separate tag-query pass below — tags are written
	// as their own kind, so the same `kind` filter would exclude them.
	events, err := s.coalescer.QueryEvents(ctx, since, until, filter.GetKind())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("events query: %w", err))
	}

	// Second pass (only when kind is set, otherwise it's a subset of the
	// first): pull tag events in the same window so we can attach them
	// to main events. Without this the list-side interestingness signals
	// would be invisible until the user opens the detail rail.
	tagged := events
	if filter.GetKind() != "" {
		tagged, err = s.coalescer.QueryEvents(ctx, since, until, "")
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("tag query: %w", err))
		}
	}
	tagsByPos := buildTagIndex(tagged)

	page := make([]*activityv1.EventRecord, 0, maxPageSize)
	for _, e := range events {
		if e.Kind == "interestingness.tagged" {
			continue
		}
		rec := eventToProtoWithPayload(e)
		if tags := tagsByPos[e.Position]; len(tags) > 0 {
			rec.Tags = tagsToProto(tags)
		}
		page = append(page, rec)
		if len(page) >= maxPageSize {
			break
		}
	}
	return connect.NewResponse(&activityv1.EventsResponse{Events: page}), nil
}

// buildTagIndex scans events for `interestingness.tagged` carriers and
// returns a map: source-event position → tags. Mirrors the inline logic
// the story coalescer uses; lifted out so list+story paths agree on the
// decode semantics.
func buildTagIndex(events []eventstore.Event) map[uint64][]interestingness.Tag {
	out := make(map[uint64][]interestingness.Tag)
	for _, e := range events {
		if e.Kind != "interestingness.tagged" || e.Payload == nil {
			continue
		}
		sys := e.Payload.GetSystem()
		if sys == nil {
			continue
		}
		out[e.CausePosition] = append(out[e.CausePosition], interestingness.Tag{
			Category:    interestingness.Category(sys.Data["category"]),
			Name:        sys.Data["name"],
			Explanation: sys.Data["explanation"],
		})
	}
	return out
}

// tagsToProto adapts internal tag values for the wire response.
func tagsToProto(tags []interestingness.Tag) []*activityv1.InterestingnessTag {
	out := make([]*activityv1.InterestingnessTag, 0, len(tags))
	for _, t := range tags {
		out = append(out, &activityv1.InterestingnessTag{
			Category:    string(t.Category),
			Name:        t.Name,
			Explanation: t.Explanation,
		})
	}
	return out
}

// EventDetail returns a single event with tags and causation chain.
func (s *ActivityService) EventDetail(
	ctx context.Context,
	req *connect.Request[activityv1.EventDetailRequest],
) (*connect.Response[activityv1.EventDetailResponse], error) {
	if s.cfg.Mock {
		events := mock.GenerateEvents(ctx)
		// In mock mode, return the first event whose EventId matches, or the
		// first event if none match (to allow test-fixture event IDs that were
		// returned by ListEvents from the same call site but with fresh UUIDs).
		for _, ev := range events {
			if ev.EventId == req.Msg.EventId {
				return connect.NewResponse(&activityv1.EventDetailResponse{
					Event:          ev,
					CausationChain: nil,
				}), nil
			}
		}
		// Fall back to returning the first mock event — acceptable for unit tests
		// that just need a non-nil response.
		if len(events) > 0 && req.Msg.EventId != "" {
			ev := events[0]
			ev.EventId = req.Msg.EventId // echo back the requested ID
			return connect.NewResponse(&activityv1.EventDetailResponse{
				Event:          ev,
				CausationChain: nil,
			}), nil
		}
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("event %q not found", req.Msg.EventId))
	}
	return s.eventDetailLive(ctx, req)
}

// eventDetailLive serves EventDetail from the real event store. The event_id
// is the event's position (matching what Events/Stories return); we look it
// up by querying [pos-1, pos] with limit 1, then walk the causation chain
// upward by following cause_position until we hit 0 or a cycle.
func (s *ActivityService) eventDetailLive(
	ctx context.Context,
	req *connect.Request[activityv1.EventDetailRequest],
) (*connect.Response[activityv1.EventDetailResponse], error) {
	if s.store == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("event store not available"))
	}

	pos, err := strconv.ParseUint(req.Msg.EventId, 10, 64)
	if err != nil || pos == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("event_id must be a positive integer"))
	}

	ev, err := s.fetchEventAtPosition(ctx, pos)
	if err != nil {
		return nil, err
	}

	// Walk ancestors via cause_position. Bound the chain length so a
	// pathological loop in the data can't tar-pit the handler.
	const maxChain = 32
	chain := make([]*activityv1.EventRecord, 0, 4)
	seen := map[uint64]bool{pos: true}
	cur := ev.CausePosition
	for i := 0; cur != 0 && i < maxChain; i++ {
		if seen[cur] {
			break
		}
		seen[cur] = true
		ancestor, err := s.fetchEventAtPosition(ctx, cur)
		if err != nil {
			break // partial chain is fine — surface what we have
		}
		chain = append(chain, eventToProtoWithPayload(*ancestor))
		cur = ancestor.CausePosition
	}
	// chain currently runs newest → oldest; the proto says "oldest first".
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	return connect.NewResponse(&activityv1.EventDetailResponse{
		Event:          eventToProtoWithPayload(*ev),
		CausationChain: chain,
	}), nil
}

// fetchEventAtPosition retrieves a single event by position. Returns
// CodeNotFound when no row matches.
func (s *ActivityService) fetchEventAtPosition(ctx context.Context, pos uint64) (*eventstore.Event, error) {
	rows, err := s.store.Query(ctx, eventstore.QueryOptions{
		FromPosition: pos - 1,
		ToPosition:   pos,
		Limit:        1,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("query: %w", err))
	}
	if len(rows) == 0 || rows[0].Position != pos {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("event %d not found", pos))
	}
	return &rows[0], nil
}

// eventToProtoWithPayload is the variant used by EventDetail — same as
// eventToProto but serializes the payload to JSON so the detail rail can
// display it. The list endpoint deliberately omits payloads to keep page
// responses small.
func eventToProtoWithPayload(e eventstore.Event) *activityv1.EventRecord {
	rec := eventToProto(e)
	if e.Payload != nil {
		if b, err := protojson.Marshal(e.Payload); err == nil {
			rec.PayloadJson = string(b)
		}
	}
	return rec
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
