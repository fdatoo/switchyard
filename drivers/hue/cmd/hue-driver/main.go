// Command hue-driver is a Carport driver for the Philips Hue bridge.
// It mirrors all lights on one bridge into gohome as light.* entities
// over the CLIP v2 API (HTTPS + server-sent events).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fdatoo/switchyard-driverkit/driver"
	entityv1 "github.com/fdatoo/switchyard/gen/switchyard/entity/v1"

	"github.com/fdatoo/switchyard/drivers/hue/internal/bridge"
	"github.com/fdatoo/switchyard/drivers/hue/internal/state"
)

const driverName, driverVersion = "driver.hue", "0.1.0"

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hue-driver: config: %v\n", err)
		os.Exit(1)
	}

	client, err := bridge.New(cfg.Address, cfg.APIKey, cfg.TLSSkipVerify)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hue-driver: bridge: %v\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: parseLogLevel(os.Getenv("HUE_LOG_LEVEL")),
	})).With(
		"instance_id", os.Getenv("GOHOME_CARPORT_INSTANCE_ID"),
		"bridge_address", cfg.Address,
	)
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)

	// Auth fast-fail: a single 401 on the first call means the API key is
	// wrong. The 3-strike threshold elsewhere is for steady-state.
	if _, err := client.ListLights(ctx); err != nil {
		if isAuthError(err) {
			fmt.Fprintf(os.Stderr, "hue-driver: api key rejected on startup: %v\n", err)
			cancel()
			os.Exit(2)
		}
		// Non-auth errors at startup get logged but proceed — buildDriver
		// will retry and the supervisor will quarantine if it keeps failing.
		slog.Warn("initial bridge probe failed; proceeding to buildDriver", "error", err)
	}

	d, cache, err := buildDriver(ctx, client)
	if err != nil {
		cancel()
		slog.Error("build driver failed", "error", err)
		os.Exit(1)
	}

	go runEventLoop(ctx, client, d, cache)
	go periodicResync(ctx, client, d, cache)

	runErr := d.Run(ctx)
	cancel()
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		slog.Error("driver run exited", "error", runErr)
		os.Exit(1)
	}
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

// config holds parsed environment variables.
type config struct {
	Address       string
	APIKey        string
	TLSSkipVerify bool
}

func loadConfig() (config, error) {
	addr := os.Getenv("HUE_BRIDGE_ADDRESS")
	if addr == "" {
		return config{}, errors.New("HUE_BRIDGE_ADDRESS is required")
	}
	key := os.Getenv("HUE_API_KEY")
	if key == "" {
		return config{}, errors.New("HUE_API_KEY is required")
	}
	skip := true
	if v := os.Getenv("HUE_TLS_SKIP_VERIFY"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return config{}, errors.New("HUE_TLS_SKIP_VERIFY must be a boolean")
		}
		skip = b
	}
	return config{Address: addr, APIKey: key, TLSSkipVerify: skip}, nil
}

// reachabilityTracker debounces bridge_unreachable / bridge_recovered
// DriverEvents. It emits bridge_unreachable once after 3 consecutive
// failures, then bridge_recovered once on the first success. Subsequent
// failures restart the count.
type reachabilityTracker struct {
	mu          sync.Mutex
	consecFails int
	announced   bool // true once bridge_unreachable has been emitted
}

func (t *reachabilityTracker) record(d *driver.Driver, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err == nil {
		if t.announced {
			_ = d.EmitDriverEvent("bridge_recovered", "")
			t.announced = false
		}
		t.consecFails = 0
		return
	}
	t.consecFails++
	if t.consecFails >= 3 && !t.announced {
		_ = d.EmitDriverEvent("bridge_unreachable", err.Error())
		t.announced = true
	}
}

// stateCache is the in-memory map of last-known full state per entity ID.
// Guarded by a single mutex; both command handlers and the SSE goroutine
// read+write it.
type stateCache struct {
	mu         sync.Mutex
	byEntID    map[string]*entityv1.Light // last known state per gohome entity ID
	available  map[string]bool            // last known reachability per gohome entity ID
	hueToID    map[string]string          // Hue light resource UUID → gohome entity ID
	deviceToID map[string]string          // Hue device UUID → gohome entity ID (for connectivity events)
	gamuts     map[string]bridge.Gamut    // gohome entity ID → bulb colour gamut (populated by C-Task 8)
	reach      reachabilityTracker
}

func newStateCache() *stateCache {
	return &stateCache{
		byEntID:    map[string]*entityv1.Light{},
		available:  map[string]bool{},
		hueToID:    map[string]string{},
		deviceToID: map[string]string{},
		gamuts:     map[string]bridge.Gamut{},
	}
}

// buildDriver enumerates lights, registers each with the driverkit, and
// seeds the state cache. Returns the driver and cache; main wires them into
// the SSE goroutine.
func buildDriver(ctx context.Context, client *bridge.Client) (*driver.Driver, *stateCache, error) {
	lights, err := client.ListLights(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list lights: %w", err)
	}

	// Best-effort reachability fetch. If it fails, all bulbs default to
	// unreachable (available=false). The driver still works for command
	// dispatch; reachability will refresh on the next resync.
	statuses, err := client.ListDevices(ctx)
	if err != nil {
		slog.Warn("list devices failed; bulbs will start as unavailable", "error", err)
		statuses = map[string]string{}
	}

	d := driver.New(driverName, driverVersion)
	cache := newStateCache()

	for _, l := range lights {
		available := statuses[l.ID] == "connected"
		if err := registerBulb(d, cache, client, l, available); err != nil {
			return nil, nil, fmt.Errorf("register %s: %w", state.EntityID(l), err)
		}
	}
	return d, cache, nil
}

// registerBulb adds one Hue light to the driver and seeds the cache.
// Used by buildDriver at startup and by resync for hot-added bulbs.
// Caller must hold no cache locks; this acquires cache.mu internally.
func registerBulb(d *driver.Driver, cache *stateCache, client *bridge.Client, l bridge.Light, available bool) error {
	entityID := state.EntityID(l)
	caps := state.Capabilities(l)
	attrs := state.LightToAttrs(l, available)

	if err := d.AddEntity(entityID, driver.EntitySpec{
		EntityType:   "light",
		FriendlyName: l.Metadata.Name,
		Capabilities: caps,
		InitialState: attrs,
	}); err != nil {
		return err
	}

	cache.mu.Lock()
	cache.byEntID[entityID] = attrs.GetLight()
	cache.available[entityID] = available
	cache.hueToID[l.ID] = entityID
	if l.Color != nil {
		cache.gamuts[entityID] = l.Color.Gamut
	}
	if l.Owner.RID != "" {
		cache.deviceToID[l.Owner.RID] = entityID
	}
	cache.mu.Unlock()

	hueID := l.ID
	for _, c := range caps {
		c := c
		d.OnCapability(entityID, c, func(ctx context.Context, entityID string, args map[string]string) (*entityv1.Attributes, error) {
			return handleCommand(ctx, client, cache, hueID, entityID, c, args)
		})
	}
	return nil
}

func handleCommand(ctx context.Context, client *bridge.Client, cache *stateCache, hueID, entityID, capability string, args map[string]string) (*entityv1.Attributes, error) {
	cache.mu.Lock()
	gamut := cache.gamuts[entityID]
	cache.mu.Unlock()
	update, err := state.CommandToUpdate(capability, args, gamut)
	if err != nil {
		return nil, err
	}
	if err := client.SetLight(ctx, hueID, update); err != nil {
		return nil, err
	}
	// Optimistically merge the command into cache. The bridge will also
	// emit an SSE event that confirms it; both paths produce the same
	// state, so this just reduces UI lag.
	cache.mu.Lock()
	prev := cache.byEntID[entityID]
	if prev == nil {
		prev = &entityv1.Light{}
	}
	available := cache.available[entityID]
	merged := state.MergeEvent(prev, bridge.Event{
		On:               update.On,
		Dimming:          update.Dimming,
		ColorTemperature: update.ColorTemperature,
	}, available)
	cache.byEntID[entityID] = merged.GetLight()
	cache.mu.Unlock()
	return merged, nil
}

// runEventLoop opens the SSE stream, applies events to the cache, and
// pushes StateChanged events into the driverkit. On disconnect it backs
// off (1s → 30s exponential), resyncs via ListLights, and reopens.
// Exits only on ctx.Done().
func runEventLoop(ctx context.Context, client *bridge.Client, d *driver.Driver, cache *stateCache) {
	backoff := time.Second
	for {
		start := time.Now()
		err := streamOnce(ctx, client, d, cache)
		if err != nil {
			slog.Warn("sse stream error", "error", err)
			if errors.Is(err, bridge.ErrAuthRevoked) {
				slog.Error("api key rejected — re-pair via button press")
				_ = d.EmitDriverEvent("auth_failed", "")
				os.Exit(2)
			}
		}
		if ctx.Err() != nil {
			return
		}
		// If the stream stayed healthy for more than 5 seconds, treat this as a
		// normal disconnect and reset backoff so the next reconnect is fast.
		// A stream that returns immediately (e.g. connection refused) does not
		// trigger the reset, so the backoff grows normally for crash-loop scenarios.
		if time.Since(start) > 5*time.Second {
			backoff = time.Second
		}
		downtime := time.Since(start)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
		_ = d.EmitDriverEvent("sse_reconnected", fmt.Sprintf("%dms", downtime.Milliseconds()))
		// Resync state before reopening the stream.
		if err := resync(ctx, client, d, cache); err != nil {
			slog.Warn("resync failed", "error", err)
			cache.reach.record(d, err)
		} else {
			cache.reach.record(d, nil)
		}
	}
}

func streamOnce(ctx context.Context, client *bridge.Client, d *driver.Driver, cache *stateCache) error {
	ch, err := client.Events(ctx)
	if err != nil {
		return err
	}
	for ev := range ch {
		switch ev.Type {
		case "light":
			applyLightEvent(d, cache, ev)
		case "zigbee_connectivity":
			applyConnectivityEvent(d, cache, ev)
		}
	}
	return nil
}

// applyLightEvent merges a partial light state event into the cache and
// emits a StateChanged through the driver.
func applyLightEvent(d *driver.Driver, cache *stateCache, ev bridge.Event) {
	cache.mu.Lock()
	entityID, ok := cache.hueToID[ev.ID]
	if !ok {
		cache.mu.Unlock()
		return // unknown bulb (paired after startup, before next periodic resync)
	}
	prev := cache.byEntID[entityID]
	if prev == nil {
		prev = &entityv1.Light{}
	}
	available := cache.available[entityID]
	merged := state.MergeEvent(prev, ev, available)
	cache.byEntID[entityID] = merged.GetLight()
	cache.mu.Unlock()

	if err := d.EmitState(entityID, merged); err != nil && !errors.Is(err, driver.ErrNotConnected) {
		slog.Warn("emit state failed", "entity_id", entityID, "error", err)
	}
}

// applyConnectivityEvent updates per-entity availability when a
// zigbee_connectivity event arrives. Owner.RID identifies the device,
// which we map to an entityID via cache.deviceToID.
func applyConnectivityEvent(d *driver.Driver, cache *stateCache, ev bridge.Event) {
	if ev.Owner == nil {
		return
	}
	cache.mu.Lock()
	entityID, ok := cache.deviceToID[ev.Owner.RID]
	if !ok {
		cache.mu.Unlock()
		return
	}
	available := ev.Status == "connected"
	cache.available[entityID] = available
	prev := cache.byEntID[entityID]
	if prev == nil {
		prev = &entityv1.Light{}
	}
	// MergeEvent with an empty Event preserves prev's light fields and
	// applies the new availability.
	attrs := state.MergeEvent(prev, bridge.Event{}, available)
	cache.byEntID[entityID] = attrs.GetLight()
	cache.mu.Unlock()

	if err := d.EmitState(entityID, attrs); err != nil && !errors.Is(err, driver.ErrNotConnected) {
		slog.Warn("emit availability failed", "entity_id", entityID, "error", err)
	}
}

// resync reconciles the driver's view of the bridge with the bridge's
// current state. Bulbs that appeared since the last enumeration are
// registered; bulbs that disappeared are unregistered. State is refreshed
// for every bulb still present.
func resync(ctx context.Context, client *bridge.Client, d *driver.Driver, cache *stateCache) error {
	lights, err := client.ListLights(ctx)
	if err != nil {
		return err
	}
	statuses, err := client.ListDevices(ctx)
	if err != nil {
		slog.Warn("list devices failed during resync; reachability not refreshed", "error", err)
		statuses = map[string]string{}
	}

	seen := make(map[string]bool, len(lights))
	for _, l := range lights {
		seen[l.ID] = true
		available := statuses[l.ID] == "connected"

		cache.mu.Lock()
		entityID, known := cache.hueToID[l.ID]
		cache.mu.Unlock()

		if !known {
			// Hot-add.
			if err := registerBulb(d, cache, client, l, available); err != nil {
				slog.Warn("register hot-added bulb failed", "hue_id", l.ID, "error", err)
				continue
			}
			slog.Info("registered hot-added bulb", "entity_id", state.EntityID(l), "hue_id", l.ID)
			_ = d.EmitDriverEvent("bulb_added", state.EntityID(l))
			continue
		}

		// Existing bulb: refresh state.
		attrs := state.LightToAttrs(l, available)
		cache.mu.Lock()
		cache.byEntID[entityID] = attrs.GetLight()
		cache.available[entityID] = available
		if l.Color != nil {
			cache.gamuts[entityID] = l.Color.Gamut
		}
		cache.mu.Unlock()
		if err := d.EmitState(entityID, attrs); err != nil && !errors.Is(err, driver.ErrNotConnected) {
			slog.Warn("emit resync state failed", "entity_id", entityID, "error", err)
		}
	}

	// Hot-remove: anything in the cache not in `seen` is gone from the bridge.
	cache.mu.Lock()
	type removal struct {
		hueID    string
		entityID string
		deviceID string
	}
	var removed []removal
	for hueID, entityID := range cache.hueToID {
		if !seen[hueID] {
			// Find the device ID for this entity (may be empty).
			var deviceID string
			for did, eid := range cache.deviceToID {
				if eid == entityID {
					deviceID = did
					break
				}
			}
			removed = append(removed, removal{hueID: hueID, entityID: entityID, deviceID: deviceID})
		}
	}
	for _, r := range removed {
		delete(cache.hueToID, r.hueID)
		delete(cache.byEntID, r.entityID)
		delete(cache.available, r.entityID)
		delete(cache.gamuts, r.entityID)
		if r.deviceID != "" {
			delete(cache.deviceToID, r.deviceID)
		}
	}
	cache.mu.Unlock()

	for _, r := range removed {
		if err := d.UnregisterEntity(r.entityID); err != nil && !errors.Is(err, driver.ErrNotConnected) {
			slog.Warn("unregister failed", "entity_id", r.entityID, "error", err)
		}
		slog.Info("unregistered removed bulb", "entity_id", r.entityID, "hue_id", r.hueID)
		_ = d.EmitDriverEvent("bulb_removed", r.entityID)
	}

	return nil
}

// periodicResync runs resync on a 5-minute ticker, regardless of whether
// the SSE stream is active. The SSE-drop path also runs resync, but bulbs
// can be added/removed without us noticing through SSE alone (no
// "device-added" events on the stream we subscribe to).
func periodicResync(ctx context.Context, client *bridge.Client, d *driver.Driver, cache *stateCache) {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := resync(ctx, client, d, cache); err != nil {
				slog.Warn("periodic resync failed", "error", err)
				cache.reach.record(d, err)
			} else {
				cache.reach.record(d, nil)
			}
		}
	}
}

// isAuthError reports whether err looks like a 401 from the bridge — either
// the 3-strike sentinel or a single 401 wrapped in a fmt.Errorf.
func isAuthError(err error) bool {
	if errors.Is(err, bridge.ErrAuthRevoked) {
		return true
	}
	return strings.Contains(err.Error(), "status 401")
}
