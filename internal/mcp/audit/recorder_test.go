package audit_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	systempb "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/mcp/audit"
)

type fakeSysClient struct {
	lastReq *systempb.RecordConfigFileEditRequest
	cursor  uint64
}

func (f *fakeSysClient) RecordConfigFileEdit(_ context.Context, req *connect.Request[systempb.RecordConfigFileEditRequest]) (*connect.Response[systempb.RecordConfigFileEditResponse], error) {
	f.lastReq = req.Msg
	return connect.NewResponse(&systempb.RecordConfigFileEditResponse{EventCursor: f.cursor}), nil
}

func TestRecorder_ConfigFileEdited(t *testing.T) {
	fake := &fakeSysClient{cursor: 42}
	r := audit.NewRecorder(fake)
	cursor, err := r.ConfigFileEdited(context.Background(), audit.ConfigFileEditEvent{
		SessionID: "sess1", Path: "automations/lights.pkl", Sha256Hex: "abc", SizeBytes: 100,
	})
	require.NoError(t, err)
	require.Equal(t, uint64(42), cursor)
	require.Equal(t, "sess1", fake.lastReq.SessionId)
	require.Equal(t, "automations/lights.pkl", fake.lastReq.Path)
}
