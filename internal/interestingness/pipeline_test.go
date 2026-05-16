package interestingness_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/interestingness"
	"github.com/fdatoo/switchyard/internal/observability"
	"github.com/fdatoo/switchyard/internal/testutil"
)

// newTestStore creates a real, file-backed Store for pipeline integration tests.
func newTestStore(t *testing.T) *eventstore.Store {
	t.Helper()
	db := testutil.NewTestDB(t)
	logger := observability.Init(observability.LogConfig{Level: slog.LevelInfo, Format: "json", Output: &bytes.Buffer{}})
	metrics := observability.NewMetrics()
	s, err := eventstore.Open(context.Background(), eventstore.Config{}, db, logger, metrics)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close(context.Background()) })
	return s
}

func stopPipeline(t *testing.T, cancel context.CancelFunc, done <-chan error) {
	t.Helper()
	cancel()
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		require.NoError(t, err)
	}
}

// failureEvent builds an eventstore.Event of kind "cmd.failed".
func failureEvent() eventstore.Event {
	return eventstore.Event{
		Kind:      "cmd.failed",
		Source:    "test",
		Timestamp: time.Now(),
		Payload: &eventv1.Payload{
			Kind: &eventv1.Payload_System{
				System: &eventv1.SystemEvent{Kind: "cmd.failed"},
			},
		},
	}
}

// nonInterestingEvent builds a plain state-change event.
func nonInterestingEvent(entity string) eventstore.Event {
	return testutil.StateChanged(entity, 100)
}

// TestPipeline_AppendsTwoTaggedEventsForTwoFailures verifies the integration
// scenario: emit 3 events, 2 of which trigger FailureDetector; assert 2
// interestingness.tagged events are appended.
func TestPipeline_AppendsTwoTaggedEventsForTwoFailures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store := newTestStore(t)
	if err := store.Start(ctx); err != nil {
		t.Fatalf("store.Start: %v", err)
	}

	// Append 3 events: 2 failures + 1 normal.
	_, err := store.Append(ctx, failureEvent())
	require.NoError(t, err)
	_, err = store.Append(ctx, nonInterestingEvent("light/living"))
	require.NoError(t, err)
	_, err = store.Append(ctx, failureEvent())
	require.NoError(t, err)

	// Only the FailureDetector so we get deterministic tagged-event counts.
	detectors := []interestingness.Detector{
		interestingness.NewFailureDetector(),
	}

	pipeline := interestingness.NewPipeline(store, store, detectors, interestingness.PipelineConfig{
		Name: "test-pipeline",
	})

	// Start the pipeline in the background.
	pipelineCtx, pipelineCancel := context.WithCancel(ctx)
	pipelineDone := make(chan error, 1)
	go func() {
		pipelineDone <- pipeline.Start(pipelineCtx)
	}()
	defer stopPipeline(t, pipelineCancel, pipelineDone)

	var queryErr error
	require.Eventually(t, func() bool {
		events, err := store.Query(ctx, eventstore.QueryOptions{
			Filter: eventstore.Filter{Kinds: []string{"interestingness.tagged"}},
		})
		if err != nil {
			queryErr = err
			return false
		}
		return len(events) == 2
	}, 5*time.Second, 25*time.Millisecond, "expected exactly 2 interestingness.tagged events")
	require.NoError(t, queryErr)
}

// TestPipeline_NoTagsForNonInterestingEvents verifies that events not matching
// any detector produce no interestingness.tagged events.
func TestPipeline_NoTagsForNonInterestingEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store := newTestStore(t)
	if err := store.Start(ctx); err != nil {
		t.Fatalf("store.Start: %v", err)
	}

	// Only non-interesting events.
	for i := 0; i < 3; i++ {
		_, err := store.Append(ctx, nonInterestingEvent("switch/garage"))
		require.NoError(t, err)
	}

	detectors := []interestingness.Detector{
		interestingness.NewFailureDetector(),
	}

	pipeline := interestingness.NewPipeline(store, store, detectors, interestingness.PipelineConfig{
		Name: "test-pipeline-2",
	})

	pipelineCtx, pipelineCancel := context.WithCancel(ctx)
	pipelineDone := make(chan error, 1)
	go func() { pipelineDone <- pipeline.Start(pipelineCtx) }()
	defer stopPipeline(t, pipelineCancel, pipelineDone)

	time.Sleep(200 * time.Millisecond)

	events, err := store.Query(ctx, eventstore.QueryOptions{
		Filter: eventstore.Filter{Kinds: []string{"interestingness.tagged"}},
	})
	require.NoError(t, err)
	assert.Empty(t, events, "no interestingness.tagged events expected")
}
