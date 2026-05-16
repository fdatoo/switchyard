package observability

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTPHandler returns the Prometheus scrape handler for this metrics registry.
func (m *Metrics) HTTPHandler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
}

// ServeMetrics runs an HTTP server exposing /metrics, /health, and (while in
// recovery mode) the five recovery endpoints. Blocks until ctx is cancelled.
func (m *Metrics) ServeMetrics(ctx context.Context, addr string, healthFn func() (string, int), recovery RecoveryProvider) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", m.HTTPHandler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		status := "ok"
		code := http.StatusOK
		if healthFn != nil {
			status, code = healthFn()
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = w.Write([]byte(`{"status":"` + status + `"}`))
	})

	mux.HandleFunc("GET /events", HandleRecoveryEvents(recovery))
	mux.HandleFunc("GET /projection-cursors", HandleProjectionCursors(recovery))
	mux.HandleFunc("GET /skipped-events", HandleSkippedEvents(recovery))
	mux.HandleFunc("POST /events/{position}/skip", HandleSkipEvent(recovery))
	mux.HandleFunc("POST /shutdown", HandleShutdown(recovery))

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			errCh <- err
			return
		}
		errCh <- srv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		// Fresh context: parent is already cancelled, but shutdown needs time to drain.
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx) //nolint:contextcheck
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
