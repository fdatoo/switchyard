package interestingness

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/eventstore"
)

// Subscriber is a narrow interface for subscribing to the event stream.
// This matches eventstore.Store.Subscribe.
type Subscriber interface {
	Subscribe(ctx context.Context, opts eventstore.SubscribeOptions) (eventstore.Subscription, error)
}

// StoreAppender can append generic events to the event store.
// The Pipeline appends interestingness.tagged events via this interface.
type StoreAppender interface {
	Append(ctx context.Context, e eventstore.Event) (uint64, error)
}

// PipelineConfig holds configuration for the Pipeline.
type PipelineConfig struct {
	// Name is the durable subscription name used for cursor tracking.
	// Defaults to "interestingness-pipeline".
	Name string

	// Logger is optional. If nil, logs are discarded.
	Logger *slog.Logger
}

func (c *PipelineConfig) withDefaults() {
	if c.Name == "" {
		c.Name = "interestingness-pipeline"
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
}

// Pipeline fans incoming events out to all registered Detectors and appends
// interestingness.tagged events to the store for each finding.
//
// The pipeline tracks its last-processed sequence number via a durable
// eventstore subscription so that restarts resume without reprocessing.
type Pipeline struct {
	cfg       PipelineConfig
	detectors []Detector
	store     StoreAppender
	sub       Subscriber

	mu      sync.Mutex
	started bool
}

// NewPipeline creates a Pipeline with the given detectors and store.
func NewPipeline(sub Subscriber, store StoreAppender, detectors []Detector, cfg PipelineConfig) *Pipeline {
	cfg.withDefaults()
	return &Pipeline{
		cfg:       cfg,
		detectors: detectors,
		store:     store,
		sub:       sub,
	}
}

// DefaultDetectors returns the full set of production detectors with default config.
func DefaultDetectors() []Detector {
	return []Detector{
		NewPerformanceDetector(PerformanceConfig{}),
		NewAnomalyDetector(AnomalyConfig{}),
		NewCausationDetector(CausationConfig{}),
		NewFailureDetector(),
		NewSecurityDetector(SecurityConfig{}),
		NewConfigurationDetector(),
		NewNoveltyDetector(NoveltyConfig{}),
	}
}

// Start begins processing events. It blocks until ctx is cancelled.
// Callers should run Start in a goroutine.
func (p *Pipeline) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return fmt.Errorf("pipeline already started")
	}
	p.started = true
	p.mu.Unlock()

	opts := eventstore.SubscribeOptions{
		Durable: true,
		Name:    p.cfg.Name,
	}
	subscription, err := p.sub.Subscribe(ctx, opts)
	if err != nil {
		return fmt.Errorf("pipeline subscribe: %w", err)
	}
	defer func() { _ = subscription.Close() }()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e, ok := <-subscription.C():
			if !ok {
				return nil
			}
			p.processEvent(ctx, e)
			if err := subscription.Ack(e.Position); err != nil {
				p.cfg.Logger.Warn("pipeline: failed to ack position", "position", e.Position, "err", err)
			}
		}
	}
}

// processEvent fans the event out to all detectors and appends tagged events.
func (p *Pipeline) processEvent(ctx context.Context, e eventstore.Event) {
	// Fan out to all detectors in parallel.
	type result struct {
		detector Detector
		tags     []Tag
		err      error
	}
	results := make(chan result, len(p.detectors))
	for _, det := range p.detectors {
		det := det
		go func() {
			tags, err := det.Examine(ctx, e)
			results <- result{detector: det, tags: tags, err: err}
		}()
	}

	for range p.detectors {
		res := <-results
		if res.err != nil {
			p.cfg.Logger.Warn("pipeline: detector error",
				"category", res.detector.Category(),
				"event_pos", e.Position,
				"err", res.err)
			continue
		}
		if len(res.tags) == 0 {
			continue
		}
		// Append one interestingness.tagged event per detector that fired.
		for _, tag := range res.tags {
			p.appendTaggedEvent(ctx, e, tag)
		}
	}
}

// appendTaggedEvent appends a single interestingness.tagged event.
func (p *Pipeline) appendTaggedEvent(ctx context.Context, source eventstore.Event, tag Tag) {
	tagged := eventstore.Event{
		Kind:          "interestingness.tagged",
		Source:        "interestingness/" + string(tag.Category),
		CorrelationID: source.CorrelationID,
		CausePosition: source.Position,
		Payload: &eventv1.Payload{
			Kind: &eventv1.Payload_System{
				System: &eventv1.SystemEvent{
					Kind: "interestingness.tagged",
					Data: map[string]string{
						"target_event_position": fmt.Sprintf("%d", source.Position),
						"category":              string(tag.Category),
						"name":                  tag.Name,
						"explanation":           tag.Explanation,
					},
				},
			},
		},
	}
	if _, err := p.store.Append(ctx, tagged); err != nil {
		p.cfg.Logger.Warn("pipeline: failed to append tagged event",
			"category", tag.Category,
			"name", tag.Name,
			"err", err)
	}
}
