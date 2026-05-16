package listener

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/fdatoo/switchyard/internal/api"
)

// Config selects the listener sockets and optional TCP TLS credentials.
type Config struct {
	UDSPath string
	UDSMode os.FileMode

	TCPBind string

	TLSCertFile string
	TLSKeyFile  string
}

// Deps contains handlers mounted by the listener.
type Deps struct {
	HealthProbe    func() error
	ConnectRoutes  []Route
	WebhookHandler http.Handler
	MCPHandler     http.Handler // nil means /mcp is not served
	WidgetsHandler http.Handler // serves /widgets/ — mounted before the SPA catch-all
	WebHandler     http.Handler // SPA handler — mounted as catch-all
}

// Route is one Connect-RPC path and handler pair.
type Route struct {
	Path    string
	Handler http.Handler
}

// Listener owns the daemon's TCP and Unix-domain HTTP servers.
type Listener struct {
	cfg  Config
	deps Deps

	mu        sync.Mutex
	tcpLis    net.Listener
	udsLis    net.Listener
	srv       *http.Server
	startedCh chan struct{}
}

// Build validates dependencies and returns an unstarted listener.
func Build(cfg Config, deps Deps) (*Listener, error) {
	if deps.HealthProbe == nil {
		return nil, errors.New("listener: HealthProbe required")
	}
	return &Listener{cfg: cfg, deps: deps, startedCh: make(chan struct{})}, nil
}

// Start binds TCP and Unix-domain sockets, then serves until shutdown.
func (l *Listener) Start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", l.healthzHandler)
	mux.HandleFunc("/healthz", l.healthzHandler)
	for _, r := range l.deps.ConnectRoutes {
		handler := r.Handler
		mux.Handle(r.Path, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := api.WithResponseWriter(req.Context(), w)
			ctx = api.WithHTTPRequest(ctx, req)
			handler.ServeHTTP(w, req.WithContext(ctx))
		}))
	}
	if l.deps.WebhookHandler != nil {
		mux.Handle("/webhooks/", l.deps.WebhookHandler)
	}
	if l.deps.MCPHandler != nil {
		mux.Handle("/mcp", l.deps.MCPHandler)
		mux.Handle("/mcp/", l.deps.MCPHandler)
	}
	if l.deps.WidgetsHandler != nil {
		mux.Handle("/widgets/", l.deps.WidgetsHandler)
	}
	// SPA catch-all — must be last so explicit routes (Connect, /health, /healthz, /webhooks/, /mcp, /widgets/) win.
	if l.deps.WebHandler != nil {
		mux.Handle("/", l.deps.WebHandler)
	}

	l.srv = &http.Server{
		Handler:     newH2CServer(mux),
		ConnContext: withConnPeerCred,
	}

	tcpLis, err := net.Listen("tcp", l.cfg.TCPBind)
	if err != nil {
		return fmt.Errorf("listener: tcp bind %q: %w", l.cfg.TCPBind, err)
	}
	if l.cfg.TLSCertFile != "" && l.cfg.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(l.cfg.TLSCertFile, l.cfg.TLSKeyFile)
		if err != nil {
			_ = tcpLis.Close()
			return fmt.Errorf("listener: load tls: %w", err)
		}
		tcpLis = tls.NewListener(tcpLis, &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		})
	}
	l.tcpLis = tcpLis

	if err := os.Remove(l.cfg.UDSPath); err != nil && !os.IsNotExist(err) {
		_ = tcpLis.Close()
		return fmt.Errorf("listener: remove stale uds: %w", err)
	}
	udsLis, err := net.Listen("unix", l.cfg.UDSPath)
	if err != nil {
		_ = tcpLis.Close()
		return fmt.Errorf("listener: uds bind %q: %w", l.cfg.UDSPath, err)
	}
	if err := os.Chmod(l.cfg.UDSPath, l.cfg.UDSMode); err != nil {
		_ = tcpLis.Close()
		_ = udsLis.Close()
		return fmt.Errorf("listener: chmod uds: %w", err)
	}
	l.udsLis = peerCredListener{Listener: udsLis}

	go l.serve(l.tcpLis)
	go l.serve(l.udsLis)
	close(l.startedCh)
	return nil
}

func (l *Listener) serve(ls net.Listener) {
	if err := l.srv.Serve(ls); err != nil && !errors.Is(err, http.ErrServerClosed) {
		_ = err
	}
}

// TCPAddr returns the bound TCP address after Start succeeds.
func (l *Listener) TCPAddr() net.Addr {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.tcpLis == nil {
		return nil
	}
	return l.tcpLis.Addr()
}

// Shutdown gracefully drains active requests and removes the Unix socket.
//
//nolint:contextcheck // ctx may be nil (caller passes nil to trigger a default timeout); Background() is intentional
func (l *Listener) Shutdown(ctx context.Context) error {
	l.mu.Lock()
	srv := l.srv
	udsPath := l.cfg.UDSPath
	l.mu.Unlock()
	if srv == nil {
		return nil
	}
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
	}
	err := srv.Shutdown(ctx)
	_ = os.Remove(udsPath)
	return err
}

// Close immediately closes listeners and removes the Unix socket.
func (l *Listener) Close() error {
	l.mu.Lock()
	srv := l.srv
	udsPath := l.cfg.UDSPath
	l.mu.Unlock()
	if srv == nil {
		return nil
	}
	err := srv.Close()
	_ = os.Remove(udsPath)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (l *Listener) healthzHandler(w http.ResponseWriter, _ *http.Request) {
	if err := l.deps.HealthProbe(); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}
