// Command hue-driver is a Carport driver for the Philips Hue bridge.
// It mirrors all lights on one bridge into gohome as light.* entities
// over the CLIP v2 API (HTTPS + server-sent events).
package main

import (
	"context"
	"errors"
	"log"
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
		log.Fatalf("hue-driver: config: %v", err)
	}

	client, err := bridge.New(cfg.Address, cfg.APIKey, cfg.TLSSkipVerify)
	if err != nil {
		log.Fatalf("hue-driver: bridge: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)

	d, cache, err := buildDriver(ctx, client)
	if err != nil {
		cancel()
		log.Fatalf("hue-driver: build: %v", err)
	}

	go runEventLoop(ctx, client, d, cache)

	runErr := d.Run(ctx)
	cancel()
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		log.Fatalf("hue-driver: run: %v", runErr)
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
		if err := d.AddEntity(entityID, driver.EntitySpec{
			EntityType:   "light",
			FriendlyName: l.Metadata.Name,
			Capabilities: capabilities,
		}); err != nil {
			return nil, nil, err
		}
		cache.byEntID[entityID] = state.LightToAttrs(l).GetLight()
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
	})
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
		if err := streamOnce(ctx, client, d, cache); err != nil {
			log.Printf("hue-driver: events: %v", err)
		}
		if ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
		// Resync state before reopening the stream.
		if err := resync(ctx, client, d, cache); err != nil {
			log.Printf("hue-driver: resync: %v", err)
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
		merged := state.MergeEvent(prev, ev)
		cache.byEntID[entityID] = merged.GetLight()
		cache.mu.Unlock()

		if err := d.EmitState(entityID, merged); err != nil && !errors.Is(err, driver.ErrNotConnected) {
			log.Printf("hue-driver: emit %s: %v", entityID, err)
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
		attrs := state.LightToAttrs(l)
		cache.byEntID[entityID] = attrs.GetLight()
		cache.mu.Unlock()
		if err := d.EmitState(entityID, attrs); err != nil && !errors.Is(err, driver.ErrNotConnected) {
			log.Printf("hue-driver: emit resync %s: %v", entityID, err)
		}
	}
	return nil
}
