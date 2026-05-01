package carport

import "fmt"

// State enumerates the lifecycle phases a driver instance moves through.
type State int

const (
	StateDeclared State = iota // instance registered via RegisterInstance; nothing running yet
	StateSpawning
	StateAwaitingHandshake
	StateRunning
	StateFailed
	StateBackoff
	StateQuarantined
	StateStopping
	StateStopped
)

func (s State) String() string {
	return [...]string{
		"declared", "spawning", "awaiting_handshake", "running",
		"failed", "backoff", "quarantined", "stopping", "stopped",
	}[s]
}

// Trigger names the cause of a state change.
type Trigger int

const (
	TriggerStart Trigger = iota
	TriggerSpawned
	TriggerSpawnError
	TriggerHandshakeOK
	TriggerHandshakeFail
	TriggerCrash
	TriggerHealthFail
	TriggerStreamError
	TriggerBackoffScheduled
	TriggerBackoffElapsed
	TriggerBudgetExhausted
	TriggerManualRestart
	TriggerShutdown
	TriggerExited
)

func (t Trigger) String() string {
	return [...]string{
		"start", "spawned", "spawn_error", "handshake_ok", "handshake_fail",
		"crash", "health_fail", "stream_error", "backoff_scheduled",
		"backoff_elapsed", "budget_exhausted", "manual_restart", "shutdown", "exited",
	}[t]
}

// transitions is the total legal-transition table.
var transitions = map[State]map[Trigger]State{
	StateDeclared: {
		TriggerStart: StateSpawning,
	},
	StateSpawning: {
		TriggerSpawned:    StateAwaitingHandshake,
		TriggerSpawnError: StateFailed,
		TriggerShutdown:   StateStopping,
	},
	StateAwaitingHandshake: {
		TriggerHandshakeOK:   StateRunning,
		TriggerHandshakeFail: StateFailed,
		TriggerShutdown:      StateStopping,
	},
	StateRunning: {
		TriggerCrash:       StateFailed,
		TriggerHealthFail:  StateFailed,
		TriggerStreamError: StateFailed,
		TriggerShutdown:    StateStopping,
	},
	StateFailed: {
		TriggerBackoffScheduled: StateBackoff,
		TriggerShutdown:         StateStopping,
	},
	StateBackoff: {
		TriggerBackoffElapsed:  StateSpawning,
		TriggerBudgetExhausted: StateQuarantined,
		TriggerShutdown:        StateStopping,
	},
	StateQuarantined: {
		TriggerManualRestart: StateSpawning,
		TriggerShutdown:      StateStopping,
	},
	StateStopping: {
		TriggerExited: StateStopped,
	},
}

// IsLegal reports whether from --trigger--> to is permitted.
func IsLegal(from State, trigger Trigger, to State) bool {
	m, ok := transitions[from]
	if !ok {
		return false
	}
	got, ok := m[trigger]
	return ok && got == to
}

// Next returns the destination state for (from, trigger) or an error.
func Next(from State, trigger Trigger) (State, error) {
	m, ok := transitions[from]
	if !ok {
		return 0, fmt.Errorf("no transitions from state %s", from)
	}
	to, ok := m[trigger]
	if !ok {
		return 0, fmt.Errorf("illegal trigger %s from state %s", trigger, from)
	}
	return to, nil
}
