// Package interestingness implements the interestingness detector pipeline for
// the Switchyard activity feed (UI v2, plan 03).
//
// Each detector examines a single event and returns zero or more Tag values
// when it finds something noteworthy. The Pipeline fans events out to all
// registered detectors and appends interestingness.tagged events to the store.
package interestingness

import (
	"context"

	"github.com/fdatoo/switchyard/internal/eventstore"
)

// Category represents an interestingness detector category.
type Category string

const (
	CategoryPerformance   Category = "performance"
	CategoryAnomaly       Category = "anomaly"
	CategoryCausation     Category = "causation"
	CategoryFailure       Category = "failure"
	CategorySecurity      Category = "security"
	CategoryConfiguration Category = "configuration"
	CategoryNovelty       Category = "novelty"
)

// Tag is a single interestingness finding produced by a Detector.
type Tag struct {
	Category    Category
	Name        string
	Explanation string
}

// Detector examines a single event and returns zero or more Tags.
// Implementations must be safe for concurrent use.
type Detector interface {
	// Category returns the detector's interestingness category.
	Category() Category

	// Examine inspects the event and returns any relevant tags.
	// An empty slice means the event is not interesting to this detector.
	Examine(ctx context.Context, e eventstore.Event) ([]Tag, error)
}
