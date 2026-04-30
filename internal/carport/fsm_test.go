package carport

import "testing"

func TestState_String(t *testing.T) {
	cases := map[State]string{
		StateDeclared:          "declared",
		StateSpawning:          "spawning",
		StateAwaitingHandshake: "awaiting_handshake",
		StateRunning:           "running",
		StateFailed:            "failed",
		StateBackoff:           "backoff",
		StateQuarantined:       "quarantined",
		StateStopping:          "stopping",
		StateStopped:           "stopped",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("State(%d).String() = %q, want %q", s, got, want)
		}
	}
}

func TestTransition_LegalTransitions(t *testing.T) {
	legal := []struct {
		from, to State
		trigger  Trigger
	}{
		{StateDeclared, StateSpawning, TriggerStart},
		{StateSpawning, StateAwaitingHandshake, TriggerSpawned},
		{StateSpawning, StateFailed, TriggerSpawnError},
		{StateAwaitingHandshake, StateRunning, TriggerHandshakeOK},
		{StateAwaitingHandshake, StateFailed, TriggerHandshakeFail},
		{StateRunning, StateFailed, TriggerCrash},
		{StateRunning, StateFailed, TriggerHealthFail},
		{StateRunning, StateFailed, TriggerStreamError},
		{StateFailed, StateBackoff, TriggerBackoffScheduled},
		{StateBackoff, StateSpawning, TriggerBackoffElapsed},
		{StateBackoff, StateQuarantined, TriggerBudgetExhausted},
		{StateQuarantined, StateSpawning, TriggerManualRestart},
		{StateRunning, StateStopping, TriggerShutdown},
		{StateStopping, StateStopped, TriggerExited},
	}
	for _, c := range legal {
		if !IsLegal(c.from, c.trigger, c.to) {
			t.Errorf("expected legal: %s --%s--> %s", c.from, c.trigger, c.to)
		}
	}
}

func TestTransition_IllegalTransitions(t *testing.T) {
	illegal := []struct {
		from, to State
		trigger  Trigger
	}{
		{StateDeclared, StateRunning, TriggerStart}, // skip phases
		{StateQuarantined, StateRunning, TriggerHandshakeOK},
		{StateStopped, StateSpawning, TriggerStart}, // terminal
	}
	for _, c := range illegal {
		if IsLegal(c.from, c.trigger, c.to) {
			t.Errorf("expected illegal: %s --%s--> %s", c.from, c.trigger, c.to)
		}
	}
}
