// Package mock provides synthetic story and event generators for the
// ActivityService mock mode (SY_ACTIVITY_MOCK=1).
//
// All seven interestingness categories are represented in the generated data
// so that UI tests can verify that all badge styles render correctly.
package mock

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	activityv1 "github.com/fdatoo/switchyard/gen/switchyard/activity/v1"
)

// GenerateStories returns a slice of synthetic Story proto messages covering
// all seven interestingness categories.
func GenerateStories(_ context.Context) []*activityv1.Story {
	now := time.Now()
	stories := []*activityv1.Story{
		{
			Id:            uuid.New().String(),
			Title:         "Kitchen lights turned on — light/kitchen",
			InnerEventIds: []string{"1", "2", "3"},
			OccurredAt:    timestamppb.New(now.Add(-2 * time.Minute)),
			Source:        "cli:admin",
			EntityIds:     []string{"light/kitchen"},
			Tags: []*activityv1.InterestingnessTag{
				{Category: "failure", Name: "command_failed", Explanation: "The command failed unexpectedly."},
			},
		},
		{
			Id:            uuid.New().String(),
			Title:         "Slowack on scene activation",
			InnerEventIds: []string{"4", "5"},
			OccurredAt:    timestamppb.New(now.Add(-10 * time.Minute)),
			Source:        "automation:morning_scene",
			EntityIds:     []string{"light/bedroom", "light/hallway"},
			Tags: []*activityv1.InterestingnessTag{
				{Category: "performance", Name: "slow_ack", Explanation: "Command round-trip exceeded 2s SLO."},
			},
		},
		{
			Id:            uuid.New().String(),
			Title:         "Automation triggered 12 downstream events",
			InnerEventIds: []string{"6", "7", "8", "9"},
			OccurredAt:    timestamppb.New(now.Add(-30 * time.Minute)),
			Source:        "automation:evening_scene",
			EntityIds:     []string{"light/living_room", "thermostat/main"},
			Tags: []*activityv1.InterestingnessTag{
				{Category: "causation", Name: "high_fan_out", Explanation: "Correlation group produced 12 downstream events."},
			},
		},
		{
			Id:            uuid.New().String(),
			Title:         "Sensor re-appeared after 2 days dormant",
			InnerEventIds: []string{"10"},
			OccurredAt:    timestamppb.New(now.Add(-1 * time.Hour)),
			Source:        "driver:zigbee",
			EntityIds:     []string{"sensor/outdoor_temp"},
			Tags: []*activityv1.InterestingnessTag{
				{Category: "anomaly", Name: "dormant_entity_reappeared", Explanation: "Sensor re-appeared after 48h of inactivity."},
			},
		},
		{
			Id:            uuid.New().String(),
			Title:         "Repeated authentication failure from 192.168.1.200",
			InnerEventIds: []string{"11", "12", "13"},
			OccurredAt:    timestamppb.New(now.Add(-2 * time.Hour)),
			Source:        "api:unknown",
			EntityIds:     []string{},
			Tags: []*activityv1.InterestingnessTag{
				{Category: "security", Name: "repeated_auth_failed", Explanation: "3 auth failures from the same source within 5 minutes."},
			},
		},
		{
			Id:            uuid.New().String(),
			Title:         "Configuration applied — 2 drivers added",
			InnerEventIds: []string{"14"},
			OccurredAt:    timestamppb.New(now.Add(-3 * time.Hour)),
			Source:        "cli:admin",
			EntityIds:     []string{},
			Tags: []*activityv1.InterestingnessTag{
				{Category: "configuration", Name: "config_applied", Explanation: "New configuration applied: 2 driver instances added."},
			},
		},
		{
			Id:            uuid.New().String(),
			Title:         "New entity discovered: sensor/basement_humidity",
			InnerEventIds: []string{"15"},
			OccurredAt:    timestamppb.New(now.Add(-4 * time.Hour)),
			Source:        "driver:zigbee",
			EntityIds:     []string{"sensor/basement_humidity"},
			Tags: []*activityv1.InterestingnessTag{
				{Category: "novelty", Name: "first_seen_entity", Explanation: "Entity 'sensor/basement_humidity' has not been seen before."},
			},
		},
	}
	return stories
}

// GenerateEvents returns a slice of synthetic EventRecord protos for mock mode.
func GenerateEvents(_ context.Context) []*activityv1.EventRecord {
	now := time.Now()
	events := []*activityv1.EventRecord{
		{
			EventId:       uuid.New().String(),
			Kind:          "cmd.issued",
			Entity:        "light/kitchen",
			Source:        "cli:admin",
			OccurredAt:    timestamppb.New(now.Add(-2 * time.Minute)),
			CorrelationId: uuid.New().String(),
			Sequence:      1,
		},
		{
			EventId:       uuid.New().String(),
			Kind:          "cmd.failed",
			Entity:        "light/kitchen",
			Source:        "driver:zigbee",
			OccurredAt:    timestamppb.New(now.Add(-2*time.Minute + 3*time.Second)),
			CorrelationId: uuid.New().String(),
			Sequence:      2,
			Tags: []*activityv1.InterestingnessTag{
				{Category: "failure", Name: "command_failed", Explanation: "The command failed."},
			},
		},
		{
			EventId:       uuid.New().String(),
			Kind:          "state_changed",
			Entity:        "light/bedroom",
			Source:        "driver:zigbee",
			OccurredAt:    timestamppb.New(now.Add(-10 * time.Minute)),
			CorrelationId: uuid.New().String(),
			Sequence:      3,
		},
	}
	return events
}
