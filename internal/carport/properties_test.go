package carport_test

import (
	"math/rand"
	"testing"
	"testing/quick"

	"github.com/fdatoo/gohome/internal/carport"
)

// TestProp_FSMLegalReachability — generate random trigger sequences from
// StateDeclared; every accepted transition must be legal per IsLegal.
func TestProp_FSMLegalReachability(t *testing.T) {
	cfg := &quick.Config{MaxCount: 1000, Rand: rand.New(rand.NewSource(42))}
	prop := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))
		state := carport.StateDeclared
		triggers := []carport.Trigger{
			carport.TriggerStart, carport.TriggerSpawned, carport.TriggerSpawnError,
			carport.TriggerHandshakeOK, carport.TriggerHandshakeFail,
			carport.TriggerCrash, carport.TriggerHealthFail, carport.TriggerStreamError,
			carport.TriggerBackoffScheduled, carport.TriggerBackoffElapsed,
			carport.TriggerBudgetExhausted, carport.TriggerManualRestart,
			carport.TriggerShutdown, carport.TriggerExited,
		}
		for i := 0; i < 20; i++ {
			trig := triggers[r.Intn(len(triggers))]
			next, err := carport.Next(state, trig)
			if err != nil {
				continue
			}
			if !carport.IsLegal(state, trig, next) {
				return false
			}
			state = next
		}
		return true
	}
	if err := quick.Check(prop, cfg); err != nil {
		t.Fatal(err)
	}
}
