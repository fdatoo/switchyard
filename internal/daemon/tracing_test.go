package daemon_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/fdatoo/switchyard/internal/daemon"
	"github.com/fdatoo/switchyard/internal/observability"
)

type daemonRecordedSpan struct {
	name  string
	attrs map[string]any
}

type daemonSpanRecorder struct {
	mu    sync.Mutex
	spans []*daemonRecordedSpan
}

func (r *daemonSpanRecorder) start(ctx context.Context, name string) (context.Context, observability.Span) {
	r.mu.Lock()
	defer r.mu.Unlock()

	span := &daemonRecordingSpan{daemonRecordedSpan: &daemonRecordedSpan{name: name, attrs: map[string]any{}}}
	r.spans = append(r.spans, span.daemonRecordedSpan)
	return ctx, span
}

func (r *daemonSpanRecorder) hasStartupPhase(phase int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, span := range r.spans {
		if span.name == "startup.phase" && span.attrs["phase"] == phase {
			return true
		}
	}
	return false
}

type daemonRecordingSpan struct {
	mu sync.Mutex
	*daemonRecordedSpan
}

func (s *daemonRecordingSpan) End() {}

func (s *daemonRecordingSpan) SetAttr(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attrs[key] = value
}

func (s *daemonRecordingSpan) AddEvent(string, ...any) {}

func (s *daemonRecordingSpan) RecordError(error) {}

func TestDaemon_StartupPhasesOpenSpans(t *testing.T) {
	rec := &daemonSpanRecorder{}
	restore := observability.SetSpanStarterForTest(rec.start)
	t.Cleanup(restore)

	dir := shortTempDir(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	metrics := observability.NewMetrics()

	adminPort := freeTCPPort(t)
	d := daemon.New(daemon.Config{
		DataDir:    dir,
		LogLevel:   slog.LevelInfo,
		LogFormat:  "json",
		AdminPort:  adminPort,
		SocketPath: fmt.Sprintf("switchyardd-%d.sock", os.Getpid()),
	}, logger, metrics)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()

	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", adminPort)
	deadline := time.Now().Add(20 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		resp, err := http.Get(healthURL) //nolint:noctx
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		cancel()
		<-done
		t.Fatal("daemon did not report ready within 20s")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Run did not return after cancel")
	}

	for phase := 1; phase <= 5; phase++ {
		if !rec.hasStartupPhase(phase) {
			t.Fatalf("missing startup phase span %d", phase)
		}
	}
}
