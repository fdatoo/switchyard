package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"google.golang.org/protobuf/proto"

	"github.com/fdatoo/switchyard-driverkit/driver"
	entityv1 "github.com/fdatoo/switchyard/gen/switchyard/entity/v1"

	"github.com/fdatoo/switchyard/drivers/z2m/internal/mqtt"
	"github.com/fdatoo/switchyard/drivers/z2m/internal/state"
	"github.com/fdatoo/switchyard/drivers/z2m/internal/z2m"
)

const driverName, driverVersion = "driver.z2m", "0.1.0"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "z2m-driver: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: parseLogLevel(os.Getenv("Z2M_LOG_LEVEL")),
	})).With(
		"instance_id", os.Getenv("SWITCHYARD_CARPORT_INSTANCE_ID"),
		"broker_url", cfg.BrokerURL,
		"base_topic", cfg.BaseTopic,
	)
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	mq, err := mqtt.New(mqtt.Config{
		BrokerURL:     cfg.BrokerURL,
		ClientID:      cfg.ClientID,
		Username:      cfg.Username,
		Password:      cfg.Password,
		TLSSkipVerify: cfg.TLSSkipVerify,
	})
	if err != nil {
		return fmt.Errorf("mqtt new: %w", err)
	}
	if err := mq.Connect(ctx); err != nil {
		return fmt.Errorf("mqtt connect: %w", err)
	}
	defer mq.Close()

	d := driver.New(driverName, driverVersion)
	cache := newStateCache()
	app := &app{cfg: cfg, mq: mq, d: d, cache: cache}

	mq.OnDisconnect(func(err error) {
		slog.Warn("mqtt disconnected", "error", err)
		_ = d.EmitDriverEvent("broker_disconnected", err.Error())
	})
	mq.OnConnect(func() {
		_ = d.EmitDriverEvent("broker_reconnected", "")
	})

	app.subscribeBridgeTopics()

	if err := d.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("driver run: %w", err)
	}
	return nil
}

// config is the JSON shape carried in SWITCHYARD_CARPORT_INSTANCE_CONFIG.
// Password is resolved at load time from the env var named by PasswordEnv;
// it is never serialized.
type config struct {
	BrokerURL     string `json:"broker_url"`
	Username      string `json:"username,omitempty"`
	PasswordEnv   string `json:"password_env,omitempty"`
	BaseTopic     string `json:"base_topic,omitempty"`
	ClientID      string `json:"client_id,omitempty"`
	TLSSkipVerify bool   `json:"tls_skip_verify,omitempty"`

	Password string `json:"-"`
}

func loadConfig() (config, error) {
	raw := os.Getenv("SWITCHYARD_CARPORT_INSTANCE_CONFIG")
	if raw == "" {
		return config{}, errors.New("SWITCHYARD_CARPORT_INSTANCE_CONFIG is required")
	}
	c := config{BaseTopic: "zigbee2mqtt"}
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return config{}, fmt.Errorf("parse instance config: %w", err)
	}
	if c.BrokerURL == "" {
		return config{}, errors.New("broker_url is required")
	}
	if c.PasswordEnv != "" {
		c.Password = os.Getenv(c.PasswordEnv)
	}
	if c.ClientID == "" {
		var b [4]byte
		_, _ = rand.Read(b[:])
		c.ClientID = "switchyard-z2m-" + hex.EncodeToString(b[:])
	}
	return c, nil
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// stateCache holds the driver's runtime view: which entities exist
// and what the last published state was, so MergeState has a base to
// merge against; which Z2M IEEE addresses we know about, fed to the
// next Reconcile; which entity IDs are downstream of a given state
// topic (a multi-property device's state topic fans out to N entities).
type stateCache struct {
	mu            sync.Mutex
	entities      map[string]*entityv1.Attributes // entityID → last attrs
	devices       map[string]z2m.Device           // ieee → last-seen device
	entityByTopic map[string][]entityListener     // state topic → which entities consume it
	friendlyByEnt map[string]string               // entityID → friendly_name (for /set)
	ieeeByEnt     map[string]string               // entityID → ieee (for log context)
}

// entityListener is one entity's binding inside a state topic: which
// Z2M property it cares about (empty string means a light, which
// consumes every recognised property in the payload).
type entityListener struct {
	EntityID string
	Property string
}

func newStateCache() *stateCache {
	return &stateCache{
		entities:      map[string]*entityv1.Attributes{},
		devices:       map[string]z2m.Device{},
		entityByTopic: map[string][]entityListener{},
		friendlyByEnt: map[string]string{},
		ieeeByEnt:     map[string]string{},
	}
}

// app bundles the long-lived dependencies that handlers need so we
// can pass one pointer rather than five.
type app struct {
	cfg   config
	mq    *mqtt.Client
	d     *driver.Driver
	cache *stateCache
}

// subscribeBridgeTopics installs the four bridge-level subscriptions
// that drive reconciliation, plus a dummy retained-payload handler
// for bridge/event (logged only).
func (a *app) subscribeBridgeTopics() {
	_ = a.mq.Subscribe(z2m.BridgeDevices(a.cfg.BaseTopic), a.onBridgeDevices)
	_ = a.mq.Subscribe(z2m.BridgeState(a.cfg.BaseTopic), a.onBridgeState)
	_ = a.mq.Subscribe(z2m.BridgeEvent(a.cfg.BaseTopic), a.onBridgeEvent)
}

// onBridgeDevices is the reconciliation entry point. Z2M republishes
// the full device list (retained) on every network change, so this is
// the single source of truth for AddEntity/UnregisterEntity/UpdateAttrs.
func (a *app) onBridgeDevices(_ string, payload []byte) {
	var devices []z2m.Device
	if err := json.Unmarshal(payload, &devices); err != nil {
		// Z2M's republish is idempotent and the next valid payload
		// heals; do not wipe registry on parse error.
		slog.Error("bridge/devices parse failed; skipping reconciliation cycle",
			"error", err, "bytes", len(payload))
		return
	}

	a.cache.mu.Lock()
	prev := make([]z2m.Device, 0, len(a.cache.devices))
	for _, d := range a.cache.devices {
		prev = append(prev, d)
	}
	a.cache.mu.Unlock()

	actions := state.Reconcile(prev, devices)

	for _, action := range actions {
		switch act := action.(type) {
		case state.AddEntity:
			a.applyAdd(act)
		case state.UnregisterEntity:
			a.applyRemove(act)
		case state.UpdateAttrs:
			a.applyUpdate(act)
		}
	}

	a.cache.mu.Lock()
	a.cache.devices = make(map[string]z2m.Device, len(devices))
	for _, d := range devices {
		a.cache.devices[d.IEEEAddress] = d
	}
	a.cache.mu.Unlock()
}

func (a *app) applyAdd(act state.AddEntity) {
	if err := a.d.AddEntity(act.EntityID, act.Spec); err != nil {
		if errors.Is(err, driver.ErrEntityAlreadyRegistered) {
			return // race-safe: bridge/devices replays produce duplicates
		}
		slog.Error("AddEntity failed", "entity_id", act.EntityID, "error", err)
		return
	}

	topics := z2m.DeviceTopics(a.cfg.BaseTopic, act.FriendlyName)

	a.cache.mu.Lock()
	a.cache.friendlyByEnt[act.EntityID] = act.FriendlyName
	a.cache.ieeeByEnt[act.EntityID] = act.IEEE
	if act.Spec.InitialState != nil {
		a.cache.entities[act.EntityID] = act.Spec.InitialState
	}
	a.cache.entityByTopic[topics.State] = append(
		a.cache.entityByTopic[topics.State],
		entityListener{EntityID: act.EntityID, Property: act.Property},
	)
	a.cache.mu.Unlock()

	// Capability handlers — only lights have any.
	if act.Spec.EntityType == "light" {
		ent := act.EntityID
		friendly := act.FriendlyName
		for _, capName := range act.Spec.Capabilities {
			cap := capName
			a.d.OnCapability(ent, cap, func(_ context.Context, _ string, args map[string]string) (*entityv1.Attributes, error) {
				payload, err := state.CommandToPayload(cap, args)
				if err != nil {
					return nil, err
				}
				setTopic := z2m.DeviceTopics(a.cfg.BaseTopic, friendly).Set
				if err := a.mq.Publish(setTopic, payload, false); err != nil {
					return nil, fmt.Errorf("publish to %s: %w", setTopic, err)
				}
				// Don't update state here — the echo on the state topic
				// arrives within ~100ms and goes through MergeState.
				return nil, nil
			})
		}
	}

	// Subscribe ahead of any retained-state delivery so we don't race.
	_ = a.mq.Subscribe(topics.State, a.onDeviceState)
	_ = a.mq.Subscribe(topics.Availability, a.onDeviceAvailability)
}

func (a *app) applyRemove(act state.UnregisterEntity) {
	topics := z2m.DeviceTopics(a.cfg.BaseTopic, act.FriendlyName)

	a.cache.mu.Lock()
	delete(a.cache.entities, act.EntityID)
	delete(a.cache.friendlyByEnt, act.EntityID)
	delete(a.cache.ieeeByEnt, act.EntityID)
	listeners := a.cache.entityByTopic[topics.State]
	pruned := listeners[:0]
	for _, l := range listeners {
		if l.EntityID != act.EntityID {
			pruned = append(pruned, l)
		}
	}
	if len(pruned) == 0 {
		delete(a.cache.entityByTopic, topics.State)
		_ = a.mq.Unsubscribe(topics.State)
		_ = a.mq.Unsubscribe(topics.Availability)
	} else {
		a.cache.entityByTopic[topics.State] = pruned
	}
	a.cache.mu.Unlock()

	if err := a.d.UnregisterEntity(act.EntityID); err != nil && !errors.Is(err, driver.ErrEntityUnknown) {
		slog.Warn("UnregisterEntity failed", "entity_id", act.EntityID, "error", err)
	}
}

func (a *app) applyUpdate(act state.UpdateAttrs) {
	a.cache.mu.Lock()
	prev := a.cache.entities[act.EntityID]
	a.cache.mu.Unlock()
	if prev == nil {
		return
	}
	// Re-emit the same attrs; the EntityRegistered event is register-once,
	// so renames don't propagate via the event log in v0.1. Documented.
	_ = a.d.EmitState(act.EntityID, prev)
}

// onDeviceState handles a per-device state-push payload by fanning it
// out to every entity that subscribes to the topic.
func (a *app) onDeviceState(topic string, payload []byte) {
	var sp z2m.StatePayload
	if err := json.Unmarshal(payload, &sp); err != nil {
		slog.Warn("device state parse failed", "topic", topic, "error", err)
		return
	}

	a.cache.mu.Lock()
	listeners := append([]entityListener(nil), a.cache.entityByTopic[topic]...)
	a.cache.mu.Unlock()

	for _, l := range listeners {
		a.applyStateUpdate(l, sp)
	}
}

func (a *app) applyStateUpdate(l entityListener, sp z2m.StatePayload) {
	a.cache.mu.Lock()
	prev := a.cache.entities[l.EntityID]
	a.cache.mu.Unlock()
	if prev == nil {
		return
	}

	next := prev
	if l.Property == "" {
		// Light: iterate all known properties in the payload.
		for prop, raw := range sp {
			n, err := state.MergeState(next, prop, raw)
			if err != nil {
				slog.Debug("MergeState skipped", "entity_id", l.EntityID, "property", prop, "error", err)
				continue
			}
			next = n
		}
	} else {
		raw, ok := sp[l.Property]
		if !ok {
			return // property not in this payload; ignore
		}
		n, err := state.MergeState(next, l.Property, raw)
		if err != nil {
			slog.Debug("MergeState failed", "entity_id", l.EntityID, "property", l.Property, "error", err)
			return
		}
		next = n
	}
	if next == prev {
		return // no change
	}

	a.cache.mu.Lock()
	a.cache.entities[l.EntityID] = next
	a.cache.mu.Unlock()
	if err := a.d.EmitState(l.EntityID, next); err != nil && !errors.Is(err, driver.ErrNotConnected) {
		slog.Warn("EmitState failed", "entity_id", l.EntityID, "error", err)
	}
}

// onDeviceAvailability sets Available=true/false on every entity
// downstream of the device's state topic.
func (a *app) onDeviceAvailability(topic string, payload []byte) {
	// Topic is .../<friendly>/availability — strip the suffix to get the state topic.
	stateTopic := strings.TrimSuffix(topic, "/availability")

	var av z2m.AvailabilityState
	var available bool
	// Z2M can publish either {"state":"online"} or the bare string "online".
	if err := json.Unmarshal(payload, &av); err == nil && av.State != "" {
		available = av.State == "online"
	} else {
		s := strings.Trim(string(payload), `" `)
		available = s == "online"
	}

	a.cache.mu.Lock()
	listeners := append([]entityListener(nil), a.cache.entityByTopic[stateTopic]...)
	a.cache.mu.Unlock()

	for _, l := range listeners {
		a.cache.mu.Lock()
		prev := a.cache.entities[l.EntityID]
		if prev == nil {
			a.cache.mu.Unlock()
			continue
		}
		next := proto.Clone(prev).(*entityv1.Attributes)
		next.Available = available
		a.cache.entities[l.EntityID] = next
		a.cache.mu.Unlock()
		_ = a.d.EmitState(l.EntityID, next)
	}
}

// onBridgeState marks every entity unavailable when the Z2M bridge
// itself goes offline. On return-to-online the next bridge/devices
// retained replay will restore correct per-entity availability.
func (a *app) onBridgeState(_ string, payload []byte) {
	var bs z2m.BridgeStatePayload
	if err := json.Unmarshal(payload, &bs); err != nil {
		// Tolerate bare-string variant.
		bs.State = strings.Trim(string(payload), `" `)
	}
	if bs.State == "online" {
		_ = a.d.EmitDriverEvent("bridge_online", "")
		return
	}
	_ = a.d.EmitDriverEvent("bridge_offline", "")

	a.cache.mu.Lock()
	ids := make([]string, 0, len(a.cache.entities))
	for id := range a.cache.entities {
		ids = append(ids, id)
	}
	a.cache.mu.Unlock()

	for _, id := range ids {
		a.cache.mu.Lock()
		prev := a.cache.entities[id]
		if prev == nil {
			a.cache.mu.Unlock()
			continue
		}
		next := proto.Clone(prev).(*entityv1.Attributes)
		next.Available = false
		a.cache.entities[id] = next
		a.cache.mu.Unlock()
		_ = a.d.EmitState(id, next)
	}
}

// onBridgeEvent is informational — pairing / removal lifecycle is
// already covered by the bridge/devices retained payload.
func (a *app) onBridgeEvent(_ string, payload []byte) {
	slog.Debug("bridge/event", "payload", string(payload))
}
