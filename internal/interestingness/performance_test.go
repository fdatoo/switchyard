package interestingness_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/interestingness"
)

func TestPerformanceDetector_BelowSLO(t *testing.T) {
	// 500ms is below the default 2s SLO — no tags expected.
	d := interestingness.NewPerformanceDetector(interestingness.PerformanceConfig{})
	ctx := context.Background()
	corrID := uuid.New()
	issued := time.Now()

	// Issue command.
	_, err := d.Examine(ctx, eventstore.Event{
		Kind:          "cmd.issued",
		CorrelationID: corrID,
		Timestamp:     issued,
	})
	require.NoError(t, err)

	// Ack arrives 500ms later.
	tags, err := d.Examine(ctx, eventstore.Event{
		Kind:          "cmd.ack",
		CorrelationID: corrID,
		Timestamp:     issued.Add(500 * time.Millisecond),
	})
	require.NoError(t, err)
	assert.Empty(t, tags, "expected no tags for sub-SLO round-trip")
}

func TestPerformanceDetector_SlowAck(t *testing.T) {
	// 3200ms exceeds the default 2s SLO — slow_ack tag expected.
	d := interestingness.NewPerformanceDetector(interestingness.PerformanceConfig{})
	ctx := context.Background()
	corrID := uuid.New()
	issued := time.Now()

	_, err := d.Examine(ctx, eventstore.Event{
		Kind:          "cmd.issued",
		CorrelationID: corrID,
		Timestamp:     issued,
	})
	require.NoError(t, err)

	tags, err := d.Examine(ctx, eventstore.Event{
		Kind:          "cmd.ack",
		CorrelationID: corrID,
		Timestamp:     issued.Add(3200 * time.Millisecond),
	})
	require.NoError(t, err)
	require.Len(t, tags, 1)
	assert.Equal(t, interestingness.CategoryPerformance, tags[0].Category)
	assert.Equal(t, "slow_ack", tags[0].Name)
}

func TestPerformanceDetector_Timeout(t *testing.T) {
	// A cmd.timeout event always produces a "timeout" tag.
	d := interestingness.NewPerformanceDetector(interestingness.PerformanceConfig{})
	ctx := context.Background()

	tags, err := d.Examine(ctx, eventstore.Event{
		Kind:      "cmd.timeout",
		Timestamp: time.Now(),
	})
	require.NoError(t, err)
	require.Len(t, tags, 1)
	assert.Equal(t, interestingness.CategoryPerformance, tags[0].Category)
	assert.Equal(t, "timeout", tags[0].Name)
}

func TestPerformanceDetector_Category(t *testing.T) {
	d := interestingness.NewPerformanceDetector(interestingness.PerformanceConfig{})
	assert.Equal(t, interestingness.CategoryPerformance, d.Category())
}
