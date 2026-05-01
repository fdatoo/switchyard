package mqtt

import (
	"context"
	"net"
	"testing"
	"time"

	mqttserver "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
)

// startBroker starts an in-memory mochi broker on a free TCP port and
// returns its address (host:port). Cleanup is registered via t.
func startBroker(t *testing.T) string {
	t.Helper()
	server := mqttserver.New(nil)
	if err := server.AddHook(new(auth.AllowHook), nil); err != nil {
		t.Fatalf("AddHook: %v", err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	tcp := listeners.NewTCP(listeners.Config{ID: "t1", Address: addr})
	if err := server.AddListener(tcp); err != nil {
		t.Fatalf("AddListener: %v", err)
	}
	go func() { _ = server.Serve() }()
	t.Cleanup(func() { _ = server.Close() })
	// Give the listener a moment to bind.
	time.Sleep(50 * time.Millisecond)
	return addr
}

func TestClientConnectPublishSubscribe(t *testing.T) {
	addr := startBroker(t)

	c, err := New(Config{
		BrokerURL: "tcp://" + addr,
		ClientID:  "test-client",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	got := make(chan []byte, 1)
	if err := c.Subscribe("test/topic", func(_ string, payload []byte) {
		got <- append([]byte(nil), payload...)
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := c.Publish("test/topic", []byte("hello"), false); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case payload := <-got:
		if string(payload) != "hello" {
			t.Errorf("payload = %q, want %q", payload, "hello")
		}
	case <-time.After(2 * time.Second):
		t.Error("subscriber never received payload")
	}
}

func TestClientUnsubscribe(t *testing.T) {
	addr := startBroker(t)
	c, err := New(Config{BrokerURL: "tcp://" + addr, ClientID: "u-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	got := make(chan struct{}, 1)
	_ = c.Subscribe("u/topic", func(_ string, _ []byte) { got <- struct{}{} })
	if err := c.Unsubscribe("u/topic"); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}
	_ = c.Publish("u/topic", []byte("x"), false)
	select {
	case <-got:
		t.Error("received payload after unsubscribe")
	case <-time.After(300 * time.Millisecond):
	}
}

func TestClientOnConnect(t *testing.T) {
	addr := startBroker(t)
	c, err := New(Config{BrokerURL: "tcp://" + addr, ClientID: "cb"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	called := make(chan struct{}, 1)
	c.OnConnect(func() { called <- struct{}{} })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Error("OnConnect callback never fired")
	}
}

func TestClientConfigRequired(t *testing.T) {
	if _, err := New(Config{ClientID: "x"}); err == nil {
		t.Error("expected error for missing BrokerURL")
	}
}
