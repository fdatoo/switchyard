// Package resources registers switchyard MCP resource handlers for entity state
// and automation traces, including live-subscription support.
package resources

import (
	"context"
	"sync"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/fdatoo/switchyard/internal/mcp"
	"github.com/fdatoo/switchyard/internal/observability"
)

// Deps is the set of dependencies needed by resource handlers.
type Deps struct {
	Client  *mcp.Client
	MCPCaps mcp.MCPCaps
	Metrics *observability.Metrics
}

// Manager manages per-URI goroutines for resource subscriptions.
type Manager struct {
	mu     sync.Mutex
	subs   map[string]*managedSub
	server *sdk.Server
}

type managedSub struct {
	cancel context.CancelFunc
	refs   int
}

// NewManager creates a new Manager.
func NewManager() *Manager {
	return &Manager{subs: make(map[string]*managedSub)}
}

// SetServer stores the server reference used by watch goroutines.
// Must be called after sdk.NewServer returns.
func (m *Manager) SetServer(s *sdk.Server) {
	m.mu.Lock()
	m.server = s
	m.mu.Unlock()
}

// getServer returns the stored server reference (may be nil before SetServer).
func (m *Manager) getServer() *sdk.Server {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.server
}

// start starts watching the URI if not already running. startFn is called once
// per URI; it receives a cancellable context rooted at context.Background() so
// that the goroutine outlives the SubscribeHandler invocation. The goroutine is
// cancelled explicitly via stop() when the last subscriber leaves.
//
//nolint:contextcheck // goroutine must outlive the SubscribeHandler call; cancelled via stop()
func (m *Manager) start(_ context.Context, uri string, startFn func(ctx context.Context)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sub, ok := m.subs[uri]; ok {
		sub.refs++
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.subs[uri] = &managedSub{cancel: cancel, refs: 1}
	go startFn(ctx)
}

// StopAll cancels all active subscription goroutines. Call on server shutdown
// or connection close to prevent goroutine leaks when clients disconnect
// without explicitly unsubscribing.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for uri, sub := range m.subs {
		sub.cancel()
		delete(m.subs, uri)
	}
}

// stop decrements the ref count for uri and cancels the goroutine when the
// last subscriber leaves.
func (m *Manager) stop(uri string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sub, ok := m.subs[uri]
	if !ok {
		return
	}
	sub.refs--
	if sub.refs <= 0 {
		sub.cancel()
		delete(m.subs, uri)
	}
}

// NewServerOpts builds sdk.ServerOptions with Subscribe/Unsubscribe handlers
// for both entity and trace URIs. It returns a setter function (call with the
// *sdk.Server immediately after sdk.NewServer) and a shutdown function that
// cancels all active subscription goroutines — call on connection close to
// prevent goroutine leaks when clients disconnect without unsubscribing.
//
// Usage:
//
//	opts, setServer, shutdown := resources.NewServerOpts(d)
//	server := sdk.NewServer(impl, opts)
//	setServer(server)
//	defer shutdown()
func NewServerOpts(d Deps) (*sdk.ServerOptions, func(*sdk.Server), func()) {
	entityMgr := NewManager()
	traceMgr := NewManager()

	opts := &sdk.ServerOptions{
		SubscribeHandler: func(ctx context.Context, req *sdk.SubscribeRequest) error {
			uri := req.Params.URI
			if isEntityURI(uri) {
				entityID := parseEntityID(uri)
				entityMgr.start(ctx, uri, func(ctx context.Context) {
					watchEntity(ctx, entityMgr, uri, entityID, d)
				})
			} else if isTraceURI(uri) {
				automationID, runID := parseTraceURI(uri)
				traceMgr.start(ctx, uri, func(ctx context.Context) {
					watchTrace(ctx, traceMgr, uri, automationID, runID, d)
				})
			}
			return nil
		},
		UnsubscribeHandler: func(ctx context.Context, req *sdk.UnsubscribeRequest) error {
			uri := req.Params.URI
			if isEntityURI(uri) {
				entityMgr.stop(uri)
			} else if isTraceURI(uri) {
				traceMgr.stop(uri)
			}
			return nil
		},
	}

	setServer := func(s *sdk.Server) {
		entityMgr.SetServer(s)
		traceMgr.SetServer(s)
	}

	shutdown := func() {
		entityMgr.StopAll()
		traceMgr.StopAll()
	}

	return opts, setServer, shutdown
}

// Register adds entity and trace resource/template handlers to server.
func Register(server *sdk.Server, d Deps) {
	RegisterEntities(server, d)
	RegisterTraces(server, d)
}
