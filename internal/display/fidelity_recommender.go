package display

import (
	displayv1 "github.com/fdatoo/switchyard/gen/switchyard/display/v1"
)

// RoomStats holds scoring inputs for a single room.
type RoomStats struct {
	RoomID            string
	EntityCount       int
	SensorCount       int
	InteractionCount30d int
}

// FidelityRecommender computes default FidelityOverride values based on room
// entity counts and interaction history.
type FidelityRecommender struct{}

// NewFidelityRecommender returns a new FidelityRecommender.
func NewFidelityRecommender() *FidelityRecommender { return &FidelityRecommender{} }

// Recommend returns a map of room ID → FidelityOverride for the given rooms.
// Scoring rules (from Plan 07 spec):
//   - SensorCount ≥ 1         → metric = sensor (always, overrides other rules)
//   - EntityCount ≥ 5 AND Interaction30d ≥ 10 → scenes=4, metric=sensor
//   - EntityCount ≥ 3 OR  Interaction30d ≥ 5  → scenes=2, metric=presence
//   - Otherwise                                → scenes=0, metric=none
//   - Width is always standard (only user overrides can promote to wide)
func (r *FidelityRecommender) Recommend(rooms []RoomStats) map[string]*displayv1.FidelityOverride {
	out := make(map[string]*displayv1.FidelityOverride, len(rooms))
	for _, room := range rooms {
		out[room.RoomID] = r.scoreRoom(room)
	}
	return out
}

func (r *FidelityRecommender) scoreRoom(room RoomStats) *displayv1.FidelityOverride {
	fo := &displayv1.FidelityOverride{
		Width: displayv1.TileWidth_TILE_WIDTH_STANDARD, // always standard by default
	}

	// Determine scenes and base metric from entity/interaction counts.
	switch {
	case room.EntityCount >= 5 && room.InteractionCount30d >= 10:
		fo.Scenes = 4
		fo.Metric = displayv1.TileMetric_TILE_METRIC_SENSOR
	case room.EntityCount >= 3 || room.InteractionCount30d >= 5:
		fo.Scenes = 2
		fo.Metric = displayv1.TileMetric_TILE_METRIC_PRESENCE
	default:
		fo.Scenes = 0
		fo.Metric = displayv1.TileMetric_TILE_METRIC_NONE
	}

	// Sensor count override: if there is at least one sensor, always use sensor metric.
	if room.SensorCount >= 1 {
		fo.Metric = displayv1.TileMetric_TILE_METRIC_SENSOR
	}

	return fo
}
