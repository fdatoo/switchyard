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
	"sync"
	"syscall"
	"time"

	"github.com/fdatoo/gohome-driverkit/driver"
	entityv1 "github.com/fdatoo/gohome/gen/gohome/entity/v1"

	"github.com/fdatoo/gohome/drivers/hue/internal/bridge"
	"github.com/fdatoo/gohome/drivers/hue/internal/state"
)

const driverName, driverVersion = "driver.hue", "0.1.0"

var capabilities = []string{"turn_on", "turn_off", "set_brightness", "set_color_temp"}

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

	d, cache, err := buildDriver(ctx, client)
	if err != nil {
		cancel()
		slog.Error("build driver failed", "error", err)
		os.Exit(1)
	}

	go runEventLoop(ctx, client, d, cache)

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

// stateCache is the in-memory map of last-known full state per entity ID.
// Guarded by a single mutex; both command handlers and the SSE goroutine
// read+write it.
type stateCache struct {
	mu      sync.Mutex
	byEntID map[string]*entityv1.Light // last known state per gohome entity ID
	hueToID map[string]string          // Hue resource UUID → gohome entity ID
}

func newStateCache() *stateCache {
	return &stateCache{
		byEntID: map[string]*entityv1.Light{},
		hueToID: map[string]string{},
	}
}

// buildDriver enumerates lights, registers each with the driverkit, and
// seeds the state cache. Returns the driver and cache; main wires them into
// the SSE goroutine.
func buildDriver(ctx context.Context, client *bridge.Client) (*driver.Driver, *stateCache, error) {
	lights, err := client.ListLights(ctx)
	if err != nil {
		return nil, nil, err
	}

	d := driver.New(driverName, driverVersion)
	cache := newStateCache()

	for _, l := range lights {
		entityID := state.EntityID(l)
		attrs := state.LightToAttrs(l, true)
		if err := d.AddEntity(entityID, driver.EntitySpec{
			EntityType:   "light",
			FriendlyName: l.Metadata.Name,
			Capabilities: capabilities,
			InitialState: attrs,
		}); err != nil {
			return nil, nil, err
		}
		cache.byEntID[entityID] = attrs.GetLight()
		cache.hueToID[l.ID] = entityID

		hueID := l.ID // pin loop variable
		for _, cap := range capabilities {
			cap := cap
			d.OnCapability(entityID, cap, func(ctx context.Context, entityID string, args map[string]string) (*entityv1.Attributes, error) {
				return handleCommand(ctx, client, cache, hueID, entityID, cap, args)
			})
		}
	}
	return d, cache, nil
}

func handleCommand(ctx context.Context, client *bridge.Client, cache *stateCache, hueID, entityID, capability string, args map[string]string) (*entityv1.Attributes, error) {
	update, err := state.CommandToUpdate(capability, args)
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
	merged := state.MergeEvent(prev, bridge.Event{
		On:               update.On,
		Dimming:          update.Dimming,
		ColorTemperature: update.ColorTemperature,
	}, true)
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
		if err := streamOnce(ctx, client, d, cache); err != nil {
			slog.Warn("sse stream error", "error", err)
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
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
		// Resync state before reopening the stream.
		if err := resync(ctx, client, d, cache); err != nil {
			slog.Warn("resync failed", "error", err)
		}
	}
}

func streamOnce(ctx context.Context, client *bridge.Client, d *driver.Driver, cache *stateCache) error {
	ch, err := client.Events(ctx)
	if err != nil {
		return err
	}
	for ev := range ch {
		cache.mu.Lock()
		entityID, ok := cache.hueToID[ev.ID]
		if !ok {
			cache.mu.Unlock()
			continue // unknown bulb (paired after startup) — out of scope for v0.1
		}
		prev := cache.byEntID[entityID]
		if prev == nil {
			prev = &entityv1.Light{}
		}
		merged := state.MergeEvent(prev, ev, true)
		cache.byEntID[entityID] = merged.GetLight()
		cache.mu.Unlock()

		if err := d.EmitState(entityID, merged); err != nil && !errors.Is(err, driver.ErrNotConnected) {
			slog.Warn("emit state failed", "entity_id", entityID, "error", err)
		}
	}
	return nil
}

func resync(ctx context.Context, client *bridge.Client, d *driver.Driver, cache *stateCache) error {
	lights, err := client.ListLights(ctx)
	if err != nil {
		return err
	}
	for _, l := range lights {
		cache.mu.Lock()
		entityID, ok := cache.hueToID[l.ID]
		if !ok {
			cache.mu.Unlock()
			continue
		}
		attrs := state.LightToAttrs(l, true)
		cache.byEntID[entityID] = attrs.GetLight()
		cache.mu.Unlock()
		if err := d.EmitState(entityID, attrs); err != nil && !errors.Is(err, driver.ErrNotConnected) {
			slog.Warn("emit resync state failed", "entity_id", entityID, "error", err)
		}
	}
	return nil
}
