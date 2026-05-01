package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"github.com/fdatoo/gohome-driverkit/driver"
	entityv1 "github.com/fdatoo/gohome/gen/gohome/entity/v1"

	"github.com/fdatoo/gohome/drivers/z2m/internal/mqtt"
	_ "github.com/fdatoo/gohome/drivers/z2m/internal/state" // used in Task 12
	"github.com/fdatoo/gohome/drivers/z2m/internal/z2m"
)

const driverName, driverVersion = "driver.z2m", "0.1.0"

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "z2m-driver: config: %v\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: parseLogLevel(os.Getenv("Z2M_LOG_LEVEL")),
	})).With(
		"instance_id", os.Getenv("GOHOME_CARPORT_INSTANCE_ID"),
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
		fmt.Fprintf(os.Stderr, "z2m-driver: mqtt new: %v\n", err)
		os.Exit(1)
	}
	if err := mq.Connect(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "z2m-driver: mqtt connect: %v\n", err)
		os.Exit(1)
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

	runErr := d.Run(ctx)
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		slog.Error("driver run exited", "error", runErr)
		os.Exit(1)
	}
}

// config holds parsed environment variables.
type config struct {
	BrokerURL     string
	Username      string
	Password      string
	BaseTopic     string
	ClientID      string
	TLSSkipVerify bool
}

func loadConfig() (config, error) {
	c := config{
		BrokerURL: os.Getenv("Z2M_BROKER_URL"),
		Username:  os.Getenv("Z2M_USERNAME"),
		Password:  os.Getenv("Z2M_PASSWORD"),
		BaseTopic: os.Getenv("Z2M_BASE_TOPIC"),
		ClientID:  os.Getenv("Z2M_CLIENT_ID"),
	}
	if c.BrokerURL == "" {
		return config{}, errors.New("Z2M_BROKER_URL is required")
	}
	if c.BaseTopic == "" {
		c.BaseTopic = "zigbee2mqtt"
	}
	if c.ClientID == "" {
		var b [4]byte
		_, _ = rand.Read(b[:])
		c.ClientID = "gohome-z2m-" + hex.EncodeToString(b[:])
	}
	if v := os.Getenv("Z2M_TLS_SKIP_VERIFY"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return config{}, errors.New("Z2M_TLS_SKIP_VERIFY must be a boolean")
		}
		c.TLSSkipVerify = b
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
	mu             sync.Mutex
	entities       map[string]*entityv1.Attributes // entityID → last attrs
	devices        map[string]z2m.Device           // ieee → last-seen device
	entityByTopic  map[string][]entityListener     // state topic → which entities consume it
	friendlyByEnt  map[string]string               // entityID → friendly_name (for /set)
	ieeeByEnt      map[string]string               // entityID → ieee (for log context)
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
		entities:       map[string]*entityv1.Attributes{},
		devices:        map[string]z2m.Device{},
		entityByTopic:  map[string][]entityListener{},
		friendlyByEnt:  map[string]string{},
		ieeeByEnt:      map[string]string{},
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

// subscribeBridgeTopics is filled in in Task 12. Stubbed here so
// main.go compiles.
func (a *app) subscribeBridgeTopics() {
	// Implementation lands in Task 12.
}
