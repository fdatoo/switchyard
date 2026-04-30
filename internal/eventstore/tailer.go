package eventstore

import "context"

// Start launches the tailer goroutine. Must be called before Subscribe.
func (s *Store) Start(ctx context.Context) error {
	if s.started.Load() {
		return nil
	}
	s.started.Store(true)
	tailerCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	// Capture position before spawning so the goroutine's cursor reflects
	// the store state at Start() time, not at goroutine-schedule time.
	// Without this, an Append between go+schedule would be missed if the
	// cond.Broadcast fires before cond.Wait is entered.
	startPos := s.LatestPosition()
	s.bgWG.Add(1)
	go func() {
		defer s.bgWG.Done()
		s.runTailer(tailerCtx, startPos)
	}()
	s.startSnapshotter(tailerCtx)
	return nil
}

func (s *Store) runTailer(ctx context.Context, cursor uint64) {
	for {
		s.cond.L.Lock()
		for cursor >= s.latestPosition && ctx.Err() == nil {
			s.cond.Wait()
		}
		target := s.latestPosition
		s.cond.L.Unlock()
		if ctx.Err() != nil {
			return
		}

		evts, err := s.Query(ctx, QueryOptions{
			FromPosition: cursor,
			ToPosition:   target,
			Limit:        1000,
		})
		if err != nil {
			s.logger.Error("tailer query failed", "err", err)
			continue
		}
		s.metrics.TailerBatchSize.Observe(float64(len(evts)))
		for _, e := range evts {
			s.dispatch(e)
			cursor = e.Position
		}
		s.metrics.TailerLag.Set(float64(s.LatestPosition() - cursor))
	}
}
