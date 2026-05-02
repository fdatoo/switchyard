package carport

import (
	"context"
	"testing"
)

func TestRegisterInstance_HostNotStarted(t *testing.T) {
	h := &Host{
		instances: map[string]*managedInstance{},
		stopped:   make(chan struct{}),
		// ctx is nil — host not started
	}
	err := h.RegisterInstance(context.Background(), "new", "fake", "/bin/fake", nil, true, DefaultLifecycleConfig())
	if err == nil {
		t.Fatal("expected error when host not started (ctx nil)")
	}
}

func TestRegisterInstance_DuplicateID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := &Host{
		instances: map[string]*managedInstance{
			"existing": {cfg: Instance{ID: "existing"}},
		},
		stopped: make(chan struct{}),
		ctx:     ctx,
	}
	err := h.RegisterInstance(context.Background(), "existing", "fake", "/bin/fake", nil, true, DefaultLifecycleConfig())
	if err == nil {
		t.Fatal("expected error for duplicate instance ID")
	}
}

func TestRegisterInstance_HostStopped(t *testing.T) {
	stopped := make(chan struct{})
	close(stopped) // host is already stopped
	h := &Host{
		instances: map[string]*managedInstance{},
		stopped:   stopped,
		ctx:       context.Background(),
	}
	err := h.RegisterInstance(context.Background(), "new", "fake", "/bin/fake", nil, true, DefaultLifecycleConfig())
	if err == nil {
		t.Fatal("expected error when host is stopped")
	}
}

func TestUnregisterInstance_NotFound(t *testing.T) {
	h := &Host{
		instances: map[string]*managedInstance{},
		stopped:   make(chan struct{}),
	}
	err := h.UnregisterInstance(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error for missing instance")
	}
}
