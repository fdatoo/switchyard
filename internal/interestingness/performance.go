package interestingness

import (
	"context"
	"sync"
	"time"

	"github.com/fdatoo/switchyard/internal/eventstore"
)

const (
	// DefaultSLOThreshold is the default round-trip SLO for command acks.
	DefaultSLOThreshold = 2 * time.Second

	// DefaultQueueDepthThreshold is the minimum projection lag (in events)
	// that triggers a queue_depth_spike tag.
	DefaultQueueDepthThreshold = 500
)

// PerformanceConfig holds tunable thresholds for the PerformanceDetector.
type PerformanceConfig struct {
	// SLOThreshold is the maximum acceptable round-trip time for a command ack.
	// Defaults to DefaultSLOThreshold.
	SLOThreshold time.Duration

	// QueueDepthThreshold is the event-lag count that triggers a spike tag.
	// Defaults to DefaultQueueDepthThreshold.
	QueueDepthThreshold int
}

func (c *PerformanceConfig) withDefaults() {
	if c.SLOThreshold == 0 {
		c.SLOThreshold = DefaultSLOThreshold
	}
	if c.QueueDepthThreshold == 0 {
		c.QueueDepthThreshold = DefaultQueueDepthThreshold
	}
}

// commandRecord tracks the time a CommandIssued event was recorded so the
// PerformanceDetector can compute round-trips when the matching CommandAck
// arrives.
type commandRecord struct {
	issuedAt time.Time
	kind     string
}

// PerformanceDetector tags command round-trips that exceed a rolling SLO,
// timed-out commands, and large projection queue depths.
type PerformanceDetector struct {
	cfg PerformanceConfig

	mu       sync.Mutex
	inflight map[string]commandRecord // correlationID → issuedAt
}

// NewPerformanceDetector creates a PerformanceDetector with the given config.
// Pass a zero-value PerformanceConfig to use defaults.
func NewPerformanceDetector(cfg PerformanceConfig) *PerformanceDetector {
	cfg.withDefaults()
	return &PerformanceDetector{
		cfg:      cfg,
		inflight: make(map[string]commandRecord),
	}
}

// Category implements Detector.
func (d *PerformanceDetector) Category() Category { return CategoryPerformance }

// Examine implements Detector.
func (d *PerformanceDetector) Examine(_ context.Context, e eventstore.Event) ([]Tag, error) {
	var tags []Tag

	switch e.Kind {
	case "cmd.issued", "command.issued":
		// Record when this command was issued.
		corrID := e.CorrelationID.String()
		if corrID != "" {
			d.mu.Lock()
			d.inflight[corrID] = commandRecord{issuedAt: e.Timestamp, kind: e.Kind}
			d.mu.Unlock()
		}

	case "cmd.ack", "command.ack":
		corrID := e.CorrelationID.String()
		if corrID != "" {
			d.mu.Lock()
			rec, ok := d.inflight[corrID]
			if ok {
				delete(d.inflight, corrID)
			}
			d.mu.Unlock()

			if ok && !rec.issuedAt.IsZero() {
				elapsed := e.Timestamp.Sub(rec.issuedAt)
				if elapsed > d.cfg.SLOThreshold {
					tags = append(tags, Tag{
						Category:    CategoryPerformance,
						Name:        "slow_ack",
						Explanation: "Command round-trip of " + elapsed.Round(time.Millisecond).String() + " exceeded SLO of " + d.cfg.SLOThreshold.String(),
					})
				}
			}
		}

	case "cmd.timeout", "command.timeout":
		tags = append(tags, Tag{
			Category:    CategoryPerformance,
			Name:        "timeout",
			Explanation: "Command timed out before receiving an acknowledgement",
		})
	}

	return tags, nil
}
