// Package stories owns the story coalescer pipeline for the Activity feed.
// A Story groups correlated events (same correlation_id) into a single
// narrative unit with a derived title and interestingness tags.
package stories

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	activityv1 "github.com/fdatoo/switchyard/gen/switchyard/activity/v1"
	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/interestingness"
)

// Story is the in-memory Go representation of a grouped activity story.
type Story struct {
	ID            string
	Title         string
	InnerEventIDs []string
	OccurredAt    time.Time
	Tags          []interestingness.Tag
	Source        string
	EntityIDs     []string
}

// ToProto converts a Story to its proto representation.
func (s Story) ToProto() *activityv1.Story {
	tags := make([]*activityv1.InterestingnessTag, 0, len(s.Tags))
	for _, t := range s.Tags {
		tags = append(tags, &activityv1.InterestingnessTag{
			Category:    string(t.Category),
			Name:        t.Name,
			Explanation: t.Explanation,
		})
	}

	story := &activityv1.Story{
		Id:             s.ID,
		Title:          s.Title,
		InnerEventIds:  s.InnerEventIDs,
		Tags:           tags,
		Source:         s.Source,
		EntityIds:      s.EntityIDs,
	}
	if !s.OccurredAt.IsZero() {
		story.OccurredAt = timestamppb.New(s.OccurredAt)
	}
	return story
}

// FromEvents maps a slice of correlated events to a Story.
// The initiating event is the earliest by sequence.
func FromEvents(events []eventstore.Event, tags []interestingness.Tag) Story {
	if len(events) == 0 {
		return Story{}
	}

	// Earliest event by position is the initiating event.
	initiating := events[0]
	for _, e := range events[1:] {
		if e.Position < initiating.Position {
			initiating = e
		}
	}

	// Collect unique entities.
	entitySet := make(map[string]struct{})
	for _, e := range events {
		if e.Entity != "" {
			entitySet[e.Entity] = struct{}{}
		}
	}
	entities := make([]string, 0, len(entitySet))
	for eid := range entitySet {
		entities = append(entities, eid)
	}

	// Collect event IDs (use position as string ID).
	ids := make([]string, 0, len(events))
	for _, e := range events {
		ids = append(ids, eventPositionToID(e.Position))
	}

	title := deriveTitle(initiating, entities)

	return Story{
		ID:            initiating.CorrelationID.String(),
		Title:         title,
		InnerEventIDs: ids,
		OccurredAt:    initiating.Timestamp,
		Tags:          tags,
		Source:        initiating.Source,
		EntityIDs:     entities,
	}
}

// deriveTitle generates a human-readable title from the initiating event.
func deriveTitle(e eventstore.Event, entities []string) string {
	title := "Activity"
	switch e.Kind {
	case "cmd.issued", "command.issued":
		title = "Command issued"
	case "state.changed", "state_changed":
		title = "State changed"
	case "config.applied":
		title = "Configuration applied"
	case "automation.triggered":
		title = "Automation triggered"
	case "driver.restarted":
		title = "Driver restarted"
	case "auth.failed":
		title = "Authentication failure"
	default:
		if e.Kind != "" {
			title = e.Kind
		}
	}
	if len(entities) == 1 {
		title += " — " + entities[0]
	} else if len(entities) > 1 {
		title += " — " + entities[0] + " and " + itoa(len(entities)-1) + " more"
	}
	return title
}

func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

func eventPositionToID(pos uint64) string {
	if pos == 0 {
		return "0"
	}
	result := ""
	for pos > 0 {
		result = string(rune('0'+pos%10)) + result
		pos /= 10
	}
	return result
}
