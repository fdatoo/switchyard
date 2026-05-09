package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// RecoveryProvider is implemented by the Daemon to expose recovery-mode state
// to the admin HTTP server. All methods are safe for concurrent use.
type RecoveryProvider interface {
	// InRecovery returns true iff the daemon is in phase -1.
	InRecovery() bool
	// RecoveryInfo returns the failure reason and the event position at which
	// replay halted. failedPos is 0 if the position is unavailable.
	RecoveryInfo() (reason string, failedPos uint64)
	// QueryEvents returns up to limit events centred on position.
	QueryEvents(ctx context.Context, position uint64, limit int) ([]RecoveryEvent, error)
	// QueryProjectionCursors returns all rows from the projection_cursors table.
	QueryProjectionCursors(ctx context.Context) ([]ProjectionCursor, error)
	// QuerySkippedEvents returns all rows from the skipped_events table.
	QuerySkippedEvents(ctx context.Context) ([]SkippedEvent, error)
	// SkipEvent inserts a row into skipped_events. projector must be in ProjectorNames().
	SkipEvent(ctx context.Context, position uint64, projector, reason, skippedBy string) error
	// ProjectorNames returns the names of all registered sync projectors.
	ProjectorNames() []string
	// Shutdown cancels the daemon's root context, triggering a clean exit.
	Shutdown()
}

// RecoveryEvent is a JSON-serialisable projection of an eventstore event.
type RecoveryEvent struct {
	Position  uint64    `json:"position"`
	Timestamp time.Time `json:"timestamp"`
	Kind      string    `json:"kind"`
	Entity    string    `json:"entity,omitempty"`
	Source    string    `json:"source"`
}

// ProjectionCursor is a JSON-serialisable row from projection_cursors.
type ProjectionCursor struct {
	Name      string    `json:"name"`
	Position  uint64    `json:"position"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SkippedEvent is a JSON-serialisable row from skipped_events.
type SkippedEvent struct {
	Position  uint64    `json:"position"`
	Projector string    `json:"projector"`
	SkippedAt time.Time `json:"skipped_at"`
	SkippedBy string    `json:"skipped_by"`
	Reason    string    `json:"reason"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func recoveryGuard(w http.ResponseWriter, r *http.Request, p RecoveryProvider) bool {
	if !p.InRecovery() {
		http.NotFound(w, r)
		return false
	}
	return true
}

// HandleRecoveryEvents returns a handler for GET /events?position=N&limit=K.
// Returns up to limit events starting from around position. Defaults: limit=50, max 200.
func HandleRecoveryEvents(p RecoveryProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !recoveryGuard(w, r, p) {
			return
		}
		const defaultLimit = 50
		const maxLimit = 200
		limit := defaultLimit
		if s := r.URL.Query().Get("limit"); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil || n <= 0 {
				writeError(w, http.StatusBadRequest, "limit must be a positive integer")
				return
			}
			if n > maxLimit {
				n = maxLimit
			}
			limit = n
		}
		var position uint64
		if s := r.URL.Query().Get("position"); s != "" {
			n, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				writeError(w, http.StatusBadRequest, "position must be a non-negative integer")
				return
			}
			position = n
		}
		events, err := p.QueryEvents(r.Context(), position, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
		if events == nil {
			events = []RecoveryEvent{}
		}
		writeJSON(w, http.StatusOK, events)
	}
}

// HandleProjectionCursors returns a handler for GET /projection-cursors.
func HandleProjectionCursors(p RecoveryProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !recoveryGuard(w, r, p) {
			return
		}
		cursors, err := p.QueryProjectionCursors(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
		if cursors == nil {
			cursors = []ProjectionCursor{}
		}
		writeJSON(w, http.StatusOK, cursors)
	}
}

// HandleSkippedEvents returns a handler for GET /skipped-events.
func HandleSkippedEvents(p RecoveryProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !recoveryGuard(w, r, p) {
			return
		}
		skipped, err := p.QuerySkippedEvents(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
		if skipped == nil {
			skipped = []SkippedEvent{}
		}
		writeJSON(w, http.StatusOK, skipped)
	}
}

// HandleSkipEvent returns a handler for POST /events/{position}/skip.
// Body: {"projector":"...","reason":"..."}.
func HandleSkipEvent(p RecoveryProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !recoveryGuard(w, r, p) {
			return
		}
		position, err := strconv.ParseUint(r.PathValue("position"), 10, 64)
		if err != nil || position == 0 {
			writeError(w, http.StatusBadRequest, "position must be a positive integer")
			return
		}
		var body struct {
			Projector string `json:"projector"`
			Reason    string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if body.Projector == "" {
			writeError(w, http.StatusUnprocessableEntity, "projector is required")
			return
		}
		if body.Reason == "" {
			writeError(w, http.StatusUnprocessableEntity, "reason is required")
			return
		}
		known := false
		for _, name := range p.ProjectorNames() {
			if name == body.Projector {
				known = true
				break
			}
		}
		if !known {
			writeError(w, http.StatusUnprocessableEntity,
				fmt.Sprintf("unknown projector %q; known projectors: %v", body.Projector, p.ProjectorNames()))
			return
		}
		if err := p.SkipEvent(r.Context(), position, body.Projector, body.Reason, r.RemoteAddr); err != nil {
			writeError(w, http.StatusInternalServerError, "skip failed: "+err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleShutdown returns a handler for POST /shutdown.
func HandleShutdown(p RecoveryProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !recoveryGuard(w, r, p) {
			return
		}
		p.Shutdown()
		w.WriteHeader(http.StatusAccepted)
	}
}
