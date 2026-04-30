package observability_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fdatoo/gohome/internal/observability"
)

func TestMetrics_AppendIncrementsCounter(t *testing.T) {
	m := observability.NewMetrics()
	m.EventsAppended.WithLabelValues("state_changed").Inc()
	m.EventsAppended.WithLabelValues("state_changed").Inc()

	srv := httptest.NewServer(m.HTTPHandler())
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	bodyStr := string(bodyBytes)
	if !strings.Contains(bodyStr, `gohome_events_appended_total{kind="state_changed"} 2`) {
		t.Fatalf("missing counter in /metrics output:\n%s", bodyStr)
	}
}

func TestMetrics_BuildInfoExposed(t *testing.T) {
	m := observability.NewMetrics()
	m.SetBuildInfo("1.2.3", "abcdef", "go1.22")

	srv := httptest.NewServer(m.HTTPHandler())
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	bodyStr := string(bodyBytes)
	if !strings.Contains(bodyStr, `version="1.2.3"`) {
		t.Fatalf("build_info missing version: %s", bodyStr)
	}
}
