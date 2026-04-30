package automation_test

import (
	"context"
	"testing"
	"time"

	configpb "github.com/fdatoo/gohome/gen/gohome/config/v1"
	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/automation"
	automTestutil "github.com/fdatoo/gohome/internal/automation/testutil"
	"github.com/fdatoo/gohome/internal/eventstore"
	"github.com/fdatoo/gohome/internal/script"
	sltestutil "github.com/fdatoo/gohome/internal/starlark/testutil"
)

func buildModeEng(t *testing.T, mode configpb.AutomationConfig_Mode) (*automation.Engine, *automTestutil.FakeEventStore) {
	t.Helper()
	rt := sltestutil.NewTestRuntime(nil, nil, 0)
	store := &automTestutil.FakeEventStore{}
	scripts, _ := script.CompileScripts(&configpb.ConfigSnapshot{})
	se := script.NewEngine(scripts, rt, script.Deps{Store: store})
	snap := &configpb.ConfigSnapshot{Automations: []*configpb.AutomationConfig{
		{
			Id: "a1", Enabled: true, Mode: mode, MaxQueued: 2,
			Triggers: []*configpb.TriggerConfig{{Kind: &configpb.TriggerConfig_StateChange{
				StateChange: &configpb.StateChangeTrigger{Entities: []string{"x.y"}}}}},
			Actions: []*configpb.ActionConfig{{Kind: &configpb.ActionConfig_Wait{
				Wait: &configpb.WaitAction{DurationNs: int64(60 * time.Millisecond)}}}},
		},
	}}
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

func countOutcomes(evs []eventstore.Event) (ok, skipped int) {
	for _, e := range evs {
		if e.Kind != "automation_finished" {
			continue
		}
		fin := e.Payload.GetAutomationFinished()
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
	return
}

func TestMode_Single(t *testing.T) {
	eng, store := buildModeEng(t, configpb.AutomationConfig_MODE_SINGLE)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_ = eng.Trigger(ctx, "a1", "cli:t")
	}
	time.Sleep(250 * time.Millisecond)
	eng.Stop(ctx)
	evs := store.CopyEvents()
	ok, skipped := countOutcomes(evs)
	if ok < 1 {
		t.Errorf("want >=1 ok got %d", ok)
	}
	if skipped < 1 {
		t.Errorf("want skipped, got %d", skipped)
	}
}

func TestMode_Parallel(t *testing.T) {
	eng, store := buildModeEng(t, configpb.AutomationConfig_MODE_PARALLEL)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_ = eng.Trigger(ctx, "a1", "cli:t")
	}
	time.Sleep(200 * time.Millisecond)
	eng.Stop(ctx)
	evs := store.CopyEvents()
	ok, _ := countOutcomes(evs)
	if ok != 5 {
		t.Errorf("want 5 ok, got %d", ok)
	}
}

func TestMode_QueuedOverflow(t *testing.T) {
	eng, store := buildModeEng(t, configpb.AutomationConfig_MODE_QUEUED)
	ctx := context.Background()
	for i := 0; i < 6; i++ {
		_ = eng.Trigger(ctx, "a1", "cli:t")
	}
	time.Sleep(400 * time.Millisecond)
	eng.Stop(ctx)
	evs := store.CopyEvents()
	ok, skipped := countOutcomes(evs)
	if ok < 2 {
		t.Errorf("want >=2 ok, got %d", ok)
	}
	if skipped < 3 {
		t.Errorf("want >=3 skipped, got %d", skipped)
	}
}
