package automation_test

// race_test.go — §16.4 race matrix: mode admission and reload contention.
// These tests carry no build tag; they run under both the standard test runner
// and `go test -race`. All assertions are designed to remain correct regardless
// of goroutine scheduling order so they don't flake under the race detector.

import (
	"context"
	"sync"
	"testing"
	"time"

	configpb "github.com/fdatoo/gohome/gen/gohome/config/v1"
	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/automation"
	automTestutil "github.com/fdatoo/gohome/internal/automation/testutil"
	"github.com/fdatoo/gohome/internal/script"
	sltestutil "github.com/fdatoo/gohome/internal/starlark/testutil"
)

// buildRaceEng builds an engine ready for race tests with the given mode.
// The action is a tiny wait so runs are non-trivial but fast.
func buildRaceEng(t *testing.T, mode configpb.AutomationConfig_Mode, waitMs int64) (*automation.Engine, *automTestutil.FakeEventStore) {
	t.Helper()
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	store := &automTestutil.FakeEventStore{}
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{{
		Id: "race_auto", Enabled: true, Mode: mode,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
			StateChange: &configpb.StateChangeTrigger{Entities: []string{"sensor.race"}},
		}}},
		Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_Wait{
			Wait: &configpb.WaitAction{DurationNs: waitMs * int64(time.Millisecond)},
		}}},
	}}}
	scripts, _ := script.CompileScripts(snap)
	se := script.NewEngine(scripts, rt, script.Deps{Store: store})
	autos, err := automation.CompileAutomations(snap, se, rt)
	if err != nil {
		t.Fatal(err)
	}
	eng := automation.NewEngine(autos, se, rt, automation.Deps{
		State:      automTestutil.NewFakeState(),
		Dispatcher: &automTestutil.FakeDispatcher{},
		Store:      store,
		Scenes:     &automTestutil.FakeSceneApplier{},
	})
	return eng, store
}

// drainFinished polls until the event store contains exactly n
// automation_finished events (or times out after the given deadline).
func drainFinished(store *automTestutil.FakeEventStore, n int, deadline time.Time) int {
	for time.Now().Before(deadline) {
		count := 0
		for _, ev := range store.CopyEvents() {
			if ev.Kind == "automation_finished" {
				count++
			}
		}
		if count >= n {
			return count
		}
		time.Sleep(5 * time.Millisecond)
	}
	count := 0
	for _, ev := range store.CopyEvents() {
		if ev.Kind == "automation_finished" {
			count++
		}
	}
	return count
}

// TestRace_ModeSingle100Fires fires 100 concurrent triggers against a
// MODE_SINGLE automation. Exactly one must finish OK and the rest must be
// SKIPPED (some small timing slack is allowed: ok >=1, ok <=2).
// Spec §16.4.
func TestRace_ModeSingle100Fires(t *testing.T) {
	const n = 100
	eng, store := buildRaceEng(t, configpb.AutomationConfig_MODE_SINGLE, 5)
	ctx := context.Background()
	if err := eng.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { eng.Stop(context.Background()) })

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = eng.Trigger(ctx, "race_auto", "race")
		}()
	}
	wg.Wait()

	// Wait for all n finished events to appear (generous 10s deadline).
	total := drainFinished(store, n, time.Now().Add(10*time.Second))
	if total != n {
		t.Fatalf("want %d automation_finished events; got %d", n, total)
	}

	ok, skipped := 0, 0
	for _, ev := range store.CopyEvents() {
		if ev.Kind != "automation_finished" {
			continue
		}
		fin := ev.Payload.GetAutomationFinished()
		if fin == nil {
			continue
		}
		switch fin.GetOutcome() {
		case eventv1.RunOutcome_OUTCOME_OK:
			ok++
		case eventv1.RunOutcome_OUTCOME_SKIPPED:
			skipped++
		}
	}

	if ok < 1 || ok > 2 {
		t.Errorf("want 1 <= ok <= 2, got ok=%d skipped=%d total=%d", ok, skipped, total)
	}
	if ok+skipped != n {
		t.Errorf("want ok+skipped==%d, got ok=%d skipped=%d", n, ok, skipped)
	}
}

// TestRace_ModeParallel100Fires fires 100 concurrent triggers against a
// MODE_PARALLEL automation with a 5ms wait action. All 100 must finish OK.
// Spec §16.4.
func TestRace_ModeParallel100Fires(t *testing.T) {
	const n = 100
	eng, store := buildRaceEng(t, configpb.AutomationConfig_MODE_PARALLEL, 5)
	ctx := context.Background()
	if err := eng.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { eng.Stop(context.Background()) })

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = eng.Trigger(ctx, "race_auto", "race")
		}()
	}
	wg.Wait()

	// All 100 should finish OK within a generous deadline.
	total := drainFinished(store, n, time.Now().Add(10*time.Second))
	if total != n {
		t.Fatalf("want %d automation_finished events; got %d", n, total)
	}

	ok := 0
	for _, ev := range store.CopyEvents() {
		if ev.Kind != "automation_finished" {
			continue
		}
		fin := ev.Payload.GetAutomationFinished()
		if fin != nil && fin.GetOutcome() == eventv1.RunOutcome_OUTCOME_OK {
			ok++
		}
	}
	if ok != n {
		t.Errorf("want %d OUTCOME_OK, got %d", n, ok)
	}
}

// TestRace_ReloadDuringActiveFire runs concurrent Trigger and Reload calls for
// 200ms against a MODE_SINGLE automation. The test asserts:
//   - No panic (enforced implicitly by the test harness).
//   - At least one run completed OK.
//   - The engine's final state is consistent (List returns the expected automation).
//
// Spec §16.4 "reload racing active fire".
func TestRace_ReloadDuringActiveFire(t *testing.T) {
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	store := &automTestutil.FakeEventStore{}
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{{
		Id: "reload_race", Enabled: true, Mode: configpb.AutomationConfig_MODE_SINGLE,
		Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
			StateChange: &configpb.StateChangeTrigger{Entities: []string{"sensor.reload"}},
		}}},
		Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_Wait{
			Wait: &configpb.WaitAction{DurationNs: int64(20 * time.Millisecond)},
		}}},
	}}}

	scripts, _ := script.CompileScripts(snap)
	se := script.NewEngine(scripts, rt, script.Deps{Store: store})
	autos, err := automation.CompileAutomations(snap, se, rt)
	if err != nil {
		t.Fatal(err)
	}
	eng := automation.NewEngine(autos, se, rt, automation.Deps{
		State:      automTestutil.NewFakeState(),
		Dispatcher: &automTestutil.FakeDispatcher{},
		Store:      store,
		Scenes:     &automTestutil.FakeSceneApplier{},
	})
	ctx, cancel := context.WithCancel(context.Background())
	// cancel is called to stop in-flight goroutines that fire after Stop.
	defer cancel()

	if err := eng.Start(ctx); err != nil {
		t.Fatal(err)
	}

	stop := make(chan struct{})

	// Goroutine 1: repeatedly trigger.
	var triggerWg sync.WaitGroup
	triggerWg.Add(1)
	go func() {
		defer triggerWg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = eng.Trigger(ctx, "reload_race", "race")
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()

	// Goroutine 2: repeatedly reload with the same snapshot.
	var reloadWg sync.WaitGroup
	reloadWg.Add(1)
	go func() {
		defer reloadWg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = eng.Reload(snap)
				time.Sleep(7 * time.Millisecond)
			}
		}
	}()

	// Run for 200ms then stop the goroutines.
	time.Sleep(200 * time.Millisecond)
	close(stop)
	triggerWg.Wait()
	reloadWg.Wait()

	// Cancel the context so in-flight engine goroutines exit cleanly, then
	// drain in-flight runs via Stop.
	cancel()
	eng.Stop(context.Background())

	// At least one run must have completed OK.
	ok := 0
	for _, ev := range store.CopyEvents() {
		if ev.Kind != "automation_finished" {
			continue
		}
		fin := ev.Payload.GetAutomationFinished()
		if fin != nil && fin.GetOutcome() == eventv1.RunOutcome_OUTCOME_OK {
			ok++
		}
	}
	if ok < 1 {
		t.Errorf("want at least 1 OUTCOME_OK after reload race; got %d", ok)
	}

	// Final engine state: List must include the expected automation.
	summaries := eng.List()
	found := false
	for _, s := range summaries {
		if s.ID == "reload_race" {
			found = true
		}
	}
	if !found {
		t.Errorf("reload_race automation missing from engine.List() after race; got %v", summaries)
	}
}
