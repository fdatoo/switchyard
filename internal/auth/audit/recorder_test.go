package audit_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fdatoo/switchyard/internal/auth/audit"
	"github.com/fdatoo/switchyard/internal/eventstore/eventstoretest"
)

func TestRecorder_LoginSucceeded_EmitsAuthEvent(t *testing.T) {
	es := eventstoretest.New(t)
	r := audit.New(es)
	ctx := context.Background()
	require.NoError(t, r.LoginSucceeded(ctx, audit.Identity{
		PrincipalID: "user:fdatoo",
		SourceIP:    "10.0.0.1",
		RequestID:   "req-1",
	}, audit.LoginSucceeded{
		AuthMethod: "passkey",
		UserSlug:   "fdatoo",
		SessionID:  "ses-1",
	}))
	events := es.All()
	require.Len(t, events, 1)
	payload := events[0].Payload.GetAuthEvent()
	require.NotNil(t, payload)
	ls := payload.GetLoginSucceeded()
	require.NotNil(t, ls)
	require.Equal(t, "passkey", ls.AuthMethod)
	require.Equal(t, "fdatoo", ls.UserSlug)
}
