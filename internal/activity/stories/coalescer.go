package stories

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/interestingness"
)

// CoalescerConfig holds tunable parameters for the Coalescer.
type CoalescerConfig struct {
	// Window is the time window for grouping events by correlation_id.
	// Defaults to 1 hour.
	Window time.Duration
}

func (c *CoalescerConfig) withDefaults() {
	if c.Window == 0 {
		c.Window = time.Hour
	}
}

// EventQuerier can query the event store for historical events.
type EventQuerier interface {
	Query(ctx context.Context, opts eventstore.QueryOptions) ([]eventstore.Event, error)
}

// Coalescer groups events by correlation_id within a time window and
// produces Story values. It is stateless — it reads from the event store
// on demand and does not maintain any projection state.
type Coalescer struct {
	cfg   CoalescerConfig
	store EventQuerier
}

// NewCoalescer creates a Coalescer.
func NewCoalescer(store EventQuerier, cfg CoalescerConfig) *Coalescer {
	cfg.withDefaults()
	return &Coalescer{cfg: cfg, store: store}
}

// QueryEvents returns raw events in [since, until) filtered by kind.
// This is a convenience method used by the ActivityService live Events handler.
func (c *Coalescer) QueryEvents(ctx context.Context, since, until time.Time, kind string) ([]eventstore.Event, error) {
	filter := eventstore.Filter{MinTs: since, MaxTs: until}
	if kind != "" {
		filter.Kinds = []string{kind}
	}
	return c.store.Query(ctx, eventstore.QueryOptions{Filter: filter})
}

// CoalesceWindow reads all events in the window [since, until), groups them
// by correlation_id, joins interestingness.tagged events, and returns Stories
// in reverse-chronological order.
func (c *Coalescer) CoalesceWindow(ctx context.Context, since, until time.Time) ([]Story, error) {
	events, err := c.store.Query(ctx, eventstore.QueryOptions{
		Filter: eventstore.Filter{
			MinTs: since,
			MaxTs: until,
		},
	})
	if err != nil {
		return nil, err
	}

	// Separate source events from interestingness.tagged events.
	groups := make(map[uuid.UUID][]eventstore.Event)
	taggedBySource := make(map[uint64][]interestingness.Tag) // source position → tags

	for _, e := range events {
		if e.Kind == "interestingness.tagged" {
			// Decode tags from the system event data.
			if e.Payload != nil {
				if sys := e.Payload.GetSystem(); sys != nil {
					tag := interestingness.Tag{
						Category:    interestingness.Category(sys.Data["category"]),
						Name:        sys.Data["name"],
						Explanation: sys.Data["explanation"],
					}
					taggedBySource[e.CausePosition] = append(taggedBySource[e.CausePosition], tag)
				}
			}
			continue
		}
		corrID := e.CorrelationID
		groups[corrID] = append(groups[corrID], e)
	}

	// Build stories from groups, joining tags.
	stories := make([]Story, 0, len(groups))
	for _, groupEvents := range groups {
		// Collect all tags for events in this group.
		var allTags []interestingness.Tag
		for _, e := range groupEvents {
			allTags = append(allTags, taggedBySource[e.Position]...)
		}

		story := FromEvents(groupEvents, allTags)
		if story.ID == "" {
			continue
		}
		stories = append(stories, story)
	}

	// Sort reverse-chronological.
	sortStoriesDesc(stories)
	return stories, nil
}

// sortStoriesDesc sorts stories in reverse-chronological order (newest first).
func sortStoriesDesc(stories []Story) {
	// Simple insertion sort — fine for typical window sizes.
	for i := 1; i < len(stories); i++ {
		key := stories[i]
		j := i - 1
		for j >= 0 && stories[j].OccurredAt.Before(key.OccurredAt) {
			stories[j+1] = stories[j]
			j--
		}
		stories[j+1] = key
	}
}
