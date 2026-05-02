package main

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mqttserver "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	mqttlisteners "github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"

	"github.com/fdatoo/switchyard-driverkit/driver"
	"github.com/fdatoo/switchyard-driverkit/drivertest"

	"github.com/fdatoo/switchyard/drivers/z2m/internal/mqtt"
)

const baseTopic = "zigbee2mqtt"

// capturedPublish records one PUBLISH packet observed by the broker.
type capturedPublish struct {
	topic   string
	payload []byte
}

// startBroker brings up an in-process MQTT broker and returns
// (addr, server) so the test can publish from the broker side.
func startBroker(t *testing.T) (string, *mqttserver.Server) {
	t.Helper()
	server := mqttserver.New(&mqttserver.Options{InlineClient: true})
	if err := server.AddHook(new(auth.AllowHook), nil); err != nil {
		t.Fatalf("AddHook: %v", err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	tcp := mqttlisteners.NewTCP(mqttlisteners.Config{ID: "t1", Address: addr})
	if err := server.AddListener(tcp); err != nil {
		t.Fatalf("AddListener: %v", err)
	}
	go func() { _ = server.Serve() }()
	t.Cleanup(func() { _ = server.Close() })
	time.Sleep(50 * time.Millisecond)
	return addr, server
}

// publish helper: from-broker direction (simulates Z2M).
func publish(t *testing.T, server *mqttserver.Server, topic string, payload []byte, retained bool) {
	t.Helper()
	if err := server.Publish(topic, payload, retained, 0); err != nil {
		t.Fatalf("server.Publish %s: %v", topic, err)
	}
}

// buildTestApp wires up an *app pointing at the test broker and a
// stand-in driverkit Driver. Returns the app, the driver, and a
// drivertest harness already connected.
func buildTestApp(t *testing.T, brokerAddr string) (*app, *driver.Driver, *drivertest.Harness) {
	t.Helper()
	mq, err := mqtt.New(mqtt.Config{
		BrokerURL: "tcp://" + brokerAddr,
		ClientID:  "test-driver",
	})
	if err != nil {
		t.Fatalf("mqtt.New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := mq.Connect(ctx); err != nil {
		t.Fatalf("mq.Connect: %v", err)
	}
	t.Cleanup(mq.Close)

	d := driver.New(driverName, driverVersion)
	a := &app{
		cfg:   config{BaseTopic: baseTopic},
		mq:    mq,
		d:     d,
		cache: newStateCache(),
	}
	a.subscribeBridgeTopics()

	h := drivertest.New(t, d)
	t.Cleanup(h.Close)
	return a, d, h
}

func loadFixturePayload(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "z2m", "testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return raw
}

// suppressUnused stops the linter complaining about unused imports
// when individual tests don't reach for every helper.
var _ = packets.Properties{}
var _ atomic.Bool
var _ sync.Mutex

// ---- tests ----

func TestZ2M_InitialReconcile(t *testing.T) {
	addr, server := startBroker(t)
	a, _, h := buildTestApp(t, addr)
	_ = a

	// Publish the fixture as retained on bridge/devices.
	publish(t, server, baseTopic+"/bridge/devices",
		loadFixturePayload(t, "bridge_devices.json"), true)

	// drivertest's harness only sees entities at handshake time. Open
	// a fresh harness AFTER reconciliation has run so we observe the
	// post-add state.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		a.cache.mu.Lock()
		n := len(a.cache.entities)
		a.cache.mu.Unlock()
		if n >= 8 { // 1 light + 4 motion + 2 contact + 1 plug-power
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	a.cache.mu.Lock()
	got := len(a.cache.entities)
	a.cache.mu.Unlock()
	if got != 8 {
		t.Errorf("entity count after initial reconcile: got %d, want 8", got)
	}
	_ = h
}

func TestZ2M_TurnOnRoundtrip(t *testing.T) {
	addr, server := startBroker(t)
	a, _, _ := buildTestApp(t, addr)

	// Capture publishes from the driver to the broker — Z2M-side observer.
	pub := make(chan capturedPublish, 16)
	if err := server.AddHook(&capturingHook{out: pub}, nil); err != nil {
		t.Fatalf("AddHook: %v", err)
	}

	publish(t, server, baseTopic+"/bridge/devices",
		loadFixturePayload(t, "bridge_devices.json"), true)
	// Wait for entities to register.
	waitFor(t, func() bool {
		a.cache.mu.Lock()
		_, ok := a.cache.friendlyByEnt["light.z2m_01234abc"]
		a.cache.mu.Unlock()
		return ok
	})

	// Reconnect drivertest so it sees the registered entity.
	h := drivertest.New(t, a.d)
	defer h.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := h.SendCommand(ctx, "light.z2m_01234abc", "turn_on", nil)
	if err != nil {
		t.Fatalf("SendCommand: %v", err)
	}
	if !res.GetOk() {
		t.Errorf("turn_on returned ok=false: %s", res.GetErrorMessage())
	}

	// Drain observed publishes; expect one /set publish containing state:ON.
	deadline := time.Now().Add(5 * time.Second)
	var seenSet bool
	for time.Now().Before(deadline) && !seenSet {
		select {
		case p := <-pub:
			if p.topic == baseTopic+"/kitchen_light/set" {
				var decoded map[string]any
				_ = json.Unmarshal(p.payload, &decoded)
				if decoded["state"] == "ON" {
					seenSet = true
				}
			}
		case <-time.After(50 * time.Millisecond):
		}
	}
	if !seenSet {
		t.Error("did not observe /set publish with state:ON")
	}
}

func TestZ2M_HotAddRemove(t *testing.T) {
	addr, server := startBroker(t)
	a, _, _ := buildTestApp(t, addr)

	// Publish a single-device list.
	one := []byte(`[{"ieee_address":"0x00158d0001234abc","friendly_name":"kitchen_light","type":"Router","definition":{"vendor":"","model":"","description":"","exposes":[]}}]`)
	publish(t, server, baseTopic+"/bridge/devices", one, true)
	waitFor(t, func() bool {
		a.cache.mu.Lock()
		defer a.cache.mu.Unlock()
		return len(a.cache.devices) == 1
	})

	// Republish with a different device — old should disappear, new should appear.
	two := []byte(`[{"ieee_address":"0x00158d0009876543","friendly_name":"hallway_motion","type":"EndDevice","definition":{"vendor":"","model":"","description":"","exposes":[{"type":"binary","name":"occupancy","property":"occupancy","access":1}]}}]`)
	publish(t, server, baseTopic+"/bridge/devices", two, true)
	waitFor(t, func() bool {
		a.cache.mu.Lock()
		defer a.cache.mu.Unlock()
		_, oldGone := a.cache.devices["0x00158d0001234abc"]
		_, newPresent := a.cache.devices["0x00158d0009876543"]
		return !oldGone && newPresent
	})
}

func TestZ2M_BridgeOfflineMarksEntitiesUnavailable(t *testing.T) {
	addr, server := startBroker(t)
	a, _, _ := buildTestApp(t, addr)

	publish(t, server, baseTopic+"/bridge/devices",
		loadFixturePayload(t, "bridge_devices.json"), true)
	waitFor(t, func() bool {
		a.cache.mu.Lock()
		defer a.cache.mu.Unlock()
		return len(a.cache.entities) >= 8
	})

	publish(t, server, baseTopic+"/bridge/state", []byte(`{"state":"offline"}`), true)
	waitFor(t, func() bool {
		a.cache.mu.Lock()
		defer a.cache.mu.Unlock()
		for _, attrs := range a.cache.entities {
			if attrs.GetAvailable() {
				return false
			}
		}
		return true
	})
}

// ---- helpers ----

// capturingHook is a mochi hook that records every PUBLISH the driver
// sends to the broker (i.e. every /set publish in the integration tests).
type capturingHook struct {
	mqttserver.HookBase
	out chan<- capturedPublish
}

func (h *capturingHook) ID() string           { return "capturing" }
func (h *capturingHook) Provides(b byte) bool { return b == mqttserver.OnPublished }
func (h *capturingHook) OnPublished(_ *mqttserver.Client, pk packets.Packet) {
	select {
	case h.out <- capturedPublish{topic: pk.TopicName, payload: append([]byte(nil), pk.Payload...)}:
	default:
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("waitFor: condition never became true")
}
