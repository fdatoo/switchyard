package trigger

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// TimeScheduler fires time-based triggers from a single goroutine. Heap-based
// wakeup avoids per-trigger timers.
type TimeScheduler struct {
	loc    *time.Location
	parser cron.Parser

	mu      sync.Mutex
	entries entryHeap
	wakeCh  chan struct{}
	ready   chan Match
}

type scheduledEntry struct {
	automationID string
	schedule     cron.Schedule
	next         time.Time
	index        int
}

type entryHeap []*scheduledEntry

func (h entryHeap) Len() int           { return len(h) }
func (h entryHeap) Less(i, j int) bool { return h[i].next.Before(h[j].next) }
func (h entryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *entryHeap) Push(x any) {
	e := x.(*scheduledEntry)
	e.index = len(*h)
	*h = append(*h, e)
}
func (h *entryHeap) Pop() any {
	old := *h
	n := len(old)
	e := old[n-1]
	*h = old[:n-1]
	return e
}

func NewTimeScheduler(loc *time.Location) *TimeScheduler {
	if loc == nil {
		loc = time.Local
	}
	return &TimeScheduler{
		loc:    loc,
		parser: cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
		wakeCh: make(chan struct{}, 1),
		ready:  make(chan Match, 32),
	}
}

func (s *TimeScheduler) AddCron(id, expr string) error {
	sc, err := s.parser.Parse(expr)
	if err != nil {
		return fmt.Errorf("cron %q: %w", expr, err)
	}
	return s.add(id, sc)
}

func (s *TimeScheduler) AddAt(id, hm string) error {
	var h, m int
	if _, err := fmt.Sscanf(hm, "%d:%d", &h, &m); err != nil {
		return fmt.Errorf("time %q: %w", hm, err)
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return fmt.Errorf("time %q: out of range", hm)
	}
	sc, err := s.parser.Parse(fmt.Sprintf("%d %d * * *", m, h))
	if err != nil {
		return err
	}
	return s.add(id, sc)
}

func (s *TimeScheduler) AddEvery(id string, d time.Duration) error {
	if d <= 0 {
		return fmt.Errorf("every: must be positive")
	}
	return s.add(id, &everySchedule{interval: d})
}

type everySchedule struct{ interval time.Duration }

func (e *everySchedule) Next(t time.Time) time.Time { return t.Add(e.interval) }

func (s *TimeScheduler) add(id string, sc cron.Schedule) error {
	s.mu.Lock()
	now := time.Now().In(s.loc)
	heap.Push(&s.entries, &scheduledEntry{automationID: id, schedule: sc, next: sc.Next(now)})
	s.mu.Unlock()
	select {
	case s.wakeCh <- struct{}{}:
	default:
	}
	return nil
}

func (s *TimeScheduler) Reset() {
	s.mu.Lock()
	s.entries = nil
	s.mu.Unlock()
	select {
	case s.wakeCh <- struct{}{}:
	default:
	}
}

func (s *TimeScheduler) Ready() <-chan Match { return s.ready }

func (s *TimeScheduler) Run(ctx context.Context) {
	for {
		s.mu.Lock()
		var d time.Duration
		has := len(s.entries) > 0
		if has {
			d = time.Until(s.entries[0].next)
			if d < 0 {
				d = 0
			}
		} else {
			d = time.Hour
		}
		s.mu.Unlock()

		if d == 0 && has {
			s.fireDue()
			continue
		}
		t := time.NewTimer(d)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-s.wakeCh:
			t.Stop()
		case <-t.C:
			s.fireDue()
		}
	}
}

func (s *TimeScheduler) fireDue() {
	now := time.Now().In(s.loc)
	for {
		s.mu.Lock()
		if len(s.entries) == 0 || s.entries[0].next.After(now) {
			s.mu.Unlock()
			return
		}
		e := s.entries[0]
		e.next = e.schedule.Next(now)
		heap.Fix(&s.entries, 0)
		s.mu.Unlock()
		select {
		case s.ready <- Match{AutomationID: e.automationID, TriggerKind: "time"}:
		default:
		}
	}
}

// TimeMatcher is a struct-only placeholder the compiler produces; the
// automation engine consults it to populate the scheduler, never dispatches.
type TimeMatcher struct {
	AutomationIDVal string
	At              string
	Cron            string
	Every           time.Duration
}

func (t *TimeMatcher) AutomationID() string { return t.AutomationIDVal }
func (t *TimeMatcher) Kind() string         { return "time" }
