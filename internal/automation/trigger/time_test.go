package trigger_test

import (
	"context"
	"testing"
	"time"

	"github.com/fdatoo/switchyard/internal/automation/trigger"
)

func TestTimeScheduler_EveryFires(t *testing.T) {
	sch := trigger.NewTimeScheduler(time.Local)
	if err := sch.AddEvery("a", 30*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sch.Run(ctx)
	select {
	case m := <-sch.Ready():
		if m.AutomationID != "a" || m.TriggerKind != "time" {
			t.Fatalf("bad %+v", m)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("no fire")
	}
}

func TestTimeScheduler_ParseCron(t *testing.T) {
	sch := trigger.NewTimeScheduler(time.Local)
	if err := sch.AddCron("a", "*/5 * * * *"); err != nil {
		t.Fatal(err)
	}
	if err := sch.AddCron("b", "bogus"); err == nil {
		t.Fatal("want err")
	}
}

func TestTimeScheduler_ParseAt(t *testing.T) {
	sch := trigger.NewTimeScheduler(time.Local)
	if err := sch.AddAt("a", "07:30"); err != nil {
		t.Fatal(err)
	}
	if err := sch.AddAt("b", "99:99"); err == nil {
		t.Fatal("want err")
	}
}
