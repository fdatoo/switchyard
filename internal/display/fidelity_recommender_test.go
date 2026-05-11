package display

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	displayv1 "github.com/fdatoo/switchyard/gen/switchyard/display/v1"
)

func TestFidelityRecommender(t *testing.T) {
	r := NewFidelityRecommender()

	tests := []struct {
		name        string
		room        RoomStats
		wantScenes  int32
		wantMetric  displayv1.TileMetric
		wantWidth   displayv1.TileWidth
	}{
		{
			name: "rich: entity>=5 AND interaction>=10",
			room: RoomStats{RoomID: "a", EntityCount: 6, SensorCount: 0, InteractionCount30d: 12},
			wantScenes: 4,
			wantMetric: displayv1.TileMetric_TILE_METRIC_SENSOR,
			wantWidth:  displayv1.TileWidth_TILE_WIDTH_STANDARD,
		},
		{
			name: "balanced: entity>=3",
			room: RoomStats{RoomID: "b", EntityCount: 3, SensorCount: 0, InteractionCount30d: 0},
			wantScenes: 2,
			wantMetric: displayv1.TileMetric_TILE_METRIC_PRESENCE,
			wantWidth:  displayv1.TileWidth_TILE_WIDTH_STANDARD,
		},
		{
			name: "balanced: interaction>=5",
			room: RoomStats{RoomID: "c", EntityCount: 1, SensorCount: 0, InteractionCount30d: 5},
			wantScenes: 2,
			wantMetric: displayv1.TileMetric_TILE_METRIC_PRESENCE,
			wantWidth:  displayv1.TileWidth_TILE_WIDTH_STANDARD,
		},
		{
			name: "minimal: below all thresholds",
			room: RoomStats{RoomID: "d", EntityCount: 2, SensorCount: 0, InteractionCount30d: 4},
			wantScenes: 0,
			wantMetric: displayv1.TileMetric_TILE_METRIC_NONE,
			wantWidth:  displayv1.TileWidth_TILE_WIDTH_STANDARD,
		},
		{
			name: "sensor override: sensor_count>=1 forces metric=sensor",
			room: RoomStats{RoomID: "e", EntityCount: 1, SensorCount: 1, InteractionCount30d: 0},
			wantScenes: 0, // minimal scenes (entity<3 and interaction<5) but sensor metric
			wantMetric: displayv1.TileMetric_TILE_METRIC_SENSOR,
			wantWidth:  displayv1.TileWidth_TILE_WIDTH_STANDARD,
		},
		{
			name: "sensor override on balanced tier",
			room: RoomStats{RoomID: "f", EntityCount: 3, SensorCount: 2, InteractionCount30d: 0},
			wantScenes: 2,
			wantMetric: displayv1.TileMetric_TILE_METRIC_SENSOR,
			wantWidth:  displayv1.TileWidth_TILE_WIDTH_STANDARD,
		},
		{
			name: "boundary: entity=5 but interaction=9 (not rich)",
			room: RoomStats{RoomID: "g", EntityCount: 5, SensorCount: 0, InteractionCount30d: 9},
			wantScenes: 2, // entity>=3 → balanced
			wantMetric: displayv1.TileMetric_TILE_METRIC_PRESENCE,
			wantWidth:  displayv1.TileWidth_TILE_WIDTH_STANDARD,
		},
		{
			name: "boundary: entity=4 and interaction=10 (not rich, entity<5)",
			room: RoomStats{RoomID: "h", EntityCount: 4, SensorCount: 0, InteractionCount30d: 10},
			wantScenes: 2, // entity>=3 → balanced
			wantMetric: displayv1.TileMetric_TILE_METRIC_PRESENCE,
			wantWidth:  displayv1.TileWidth_TILE_WIDTH_STANDARD,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := r.Recommend([]RoomStats{tc.room})
			require.Contains(t, result, tc.room.RoomID)
			fo := result[tc.room.RoomID]
			assert.Equal(t, tc.wantScenes, fo.Scenes, "scenes mismatch")
			assert.Equal(t, tc.wantMetric, fo.Metric, "metric mismatch")
			assert.Equal(t, tc.wantWidth, fo.Width, "width must always be standard")
		})
	}
}

func TestFidelityRecommender_EmptyRooms(t *testing.T) {
	r := NewFidelityRecommender()
	result := r.Recommend(nil)
	assert.Empty(t, result)
}
