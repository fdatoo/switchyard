// Package audit emits MCP audit events through the daemon RPC surface.
package audit

import (
	"context"

	"connectrpc.com/connect"

	systempb "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
)

// SystemClient is the subset of the SystemService client that audit needs.
type SystemClient interface {
	RecordConfigFileEdit(context.Context, *connect.Request[systempb.RecordConfigFileEditRequest]) (*connect.Response[systempb.RecordConfigFileEditResponse], error)
}

// Recorder calls the daemon to emit audit events.
type Recorder struct{ sys SystemClient }

// ConfigFileEditEvent holds the data for a config file edit audit event.
type ConfigFileEditEvent struct {
	SessionID string
	Path      string
	Sha256Hex string
	SizeBytes uint32
}

// NewRecorder creates a Recorder.
func NewRecorder(sys SystemClient) *Recorder { return &Recorder{sys: sys} }

// ConfigFileEdited appends a ConfigFileEdited event through the daemon.
func (r *Recorder) ConfigFileEdited(ctx context.Context, ev ConfigFileEditEvent) (uint64, error) {
	resp, err := r.sys.RecordConfigFileEdit(ctx, connect.NewRequest(&systempb.RecordConfigFileEditRequest{
		SessionId: ev.SessionID,
		Path:      ev.Path,
		Sha256Hex: ev.Sha256Hex,
		SizeBytes: ev.SizeBytes,
	}))
	if err != nil {
		return 0, err
	}
	return resp.Msg.EventCursor, nil
}
