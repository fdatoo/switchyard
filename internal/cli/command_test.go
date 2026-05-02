package cli_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
	"github.com/fdatoo/switchyard/internal/cli"
)

type fakeEntityService struct {
	switchyardv1alpha1connect.UnimplementedEntityServiceHandler
	called      atomic.Bool
	gotEntity   string
	gotCapab    string
	gotParamKey string
	gotParamVal string
}

func (f *fakeEntityService) CallCapability(_ context.Context, req *connect.Request[v1.CallCapabilityRequest]) (*connect.Response[v1.CallCapabilityResponse], error) {
	f.called.Store(true)
	f.gotEntity = req.Msg.GetEntityId()
	f.gotCapab = req.Msg.GetCapability()
	if p := req.Msg.GetParameters(); p != nil {
		for k, v := range p.GetFields() {
			f.gotParamKey = k
			f.gotParamVal = v.GetStringValue()
		}
	}
	return connect.NewResponse(&v1.CallCapabilityResponse{CorrelationId: "corr-123", Success: true}), nil
}

func TestCommandSend_RoundTripsViaConnect(t *testing.T) {
	svc := &fakeEntityService{}
	mux := http.NewServeMux()
	path, h := switchyardv1alpha1connect.NewEntityServiceHandler(svc)
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	root := cli.NewRoot()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{
		"--endpoint", srv.URL,
		"command", "send", "light.kitchen", "turn_on",
		"--arg", "brightness=80",
	})

	require.NoError(t, root.Execute())
	require.True(t, svc.called.Load(), "EntityService.CallCapability was not called")
	require.Equal(t, "light.kitchen", svc.gotEntity)
	require.Equal(t, "turn_on", svc.gotCapab)
	require.Equal(t, "brightness", svc.gotParamKey)
	require.Equal(t, "80", svc.gotParamVal)

	out := buf.String()
	// Strip ANSI escapes for matching since lipgloss may emit color codes in tests.
	require.True(t, strings.Contains(out, "ok"), "expected 'ok' in output, got: %q", out)
	require.True(t, strings.Contains(out, "light.kitchen"), "expected entity id in output, got: %q", out)
	require.True(t, strings.Contains(out, "turn_on"), "expected capability in output, got: %q", out)
	require.True(t, strings.Contains(out, "corr-123"), "expected correlation id in output, got: %q", out)
}
