package carport_test

import (
	"bytes"
	"context"
	"database/sql"
	"log/slog"
	"testing"

	carportpb "github.com/fdatoo/gohome/gen/gohome/carport/v1alpha1"
	entitypb "github.com/fdatoo/gohome/gen/gohome/entity/v1"
	eventpb "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/carport"
	"github.com/fdatoo/gohome/internal/carport/fakedriver"
	"github.com/fdatoo/gohome/internal/eventstore"
	"github.com/fdatoo/gohome/internal/observability"
	"github.com/fdatoo/gohome/internal/registry"
	"github.com/fdatoo/gohome/internal/testutil"
)

type storeFixture struct {
	store *eventstore.Store
	db    *sql.DB
}

func newStoreFixtureForTest(t *testing.T) *storeFixture {
	t.Helper()
	db := testutil.NewTestDB(t)
	logger := observability.Init(observability.LogConfig{Level: slog.LevelWarn, Format: "json", Output: &bytes.Buffer{}})
	metrics := observability.NewMetrics()
	s, err := eventstore.Open(context.Background(), eventstore.Config{}, db, logger, metrics)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close(context.Background()) })
	return &storeFixture{store: s, db: db}
}

// fakeDriverTB adapts *testing.T to the fakedriver.TB interface without exposing
// Errorf / FailNow / Log. Defined here so multiple test files can use it.
type fakeDriverTB struct{ t *testing.T }

func (ft fakeDriverTB) Helper()                   { ft.t.Helper() }
func (ft fakeDriverTB) TempDir() string           { return ft.t.TempDir() }
func (ft fakeDriverTB) Fatalf(f string, a ...any) { ft.t.Fatalf(f, a...) }
func (ft fakeDriverTB) Cleanup(fn func())         { ft.t.Cleanup(fn) }

// newHostForDispatch constructs a Host wired to the given storeFixture, with
// the registry projector registered in sync mode so seedHueMainEntity events
// are visible immediately.
func newHostForDispatch(t *testing.T, f *storeFixture, seed func(*storeFixture)) *carport.Host {
	t.Helper()
	reg, err := registry.New(context.Background(), f.db)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.RegisterProjector(reg, eventstore.ProjectorModeSync); err != nil {
		t.Fatal(err)
	}
	if seed != nil {
		seed(f)
	}
	logger := observability.Init(observability.LogConfig{Level: slog.LevelWarn, Format: "json", Output: &bytes.Buffer{}})
	metrics := observability.NewMetrics()
	h, err := carport.New(carport.HostConfig{SocketDir: t.TempDir()},
		f.db, f.store, reg, logger, metrics)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// seedHueMainEntity appends an EntityRegistered for light.kitchen owned by hue_main.
// Needs a non-nil Capabilities proto because the registry projector marshals it
// and writes the result to a NOT NULL column.
func seedHueMainEntity(f *storeFixture) {
	_, _ = f.store.Append(context.Background(), eventstore.Event{
		Kind:   "entity_registered",
		Entity: "light.kitchen",
		Source: "driver:hue_main",
		Payload: &eventpb.Payload{Kind: &eventpb.Payload_EntityRegistered{
			EntityRegistered: &eventpb.EntityRegistered{
				DriverInstanceId: "hue_main",
				EntityType:       "light",
				FriendlyName:     "Kitchen",
				Capabilities:     &entitypb.Attributes{},
			},
		}},
	})
}

// injectRunningFake starts an in-process fakedriver.Double, registers it in the
// Host as a running instance keyed by instanceID, and returns a stop func.
func injectRunningFake(t *testing.T, h *carport.Host, instanceID string, onCmd func(*carportpb.Command) *carportpb.CommandResult) func() {
	t.Helper()
	d := &fakedriver.Double{OnCommand: func(_ context.Context, c *carportpb.Command) *carportpb.CommandResult { return onCmd(c) }}
	sock, stop := d.Serve(fakeDriverTB{t})
	if err := carport.InjectRunningInstanceForTests(h, instanceID, sock); err != nil {
		t.Fatalf("inject: %v", err)
	}
	return stop
}

func anyQueryOptions() eventstore.QueryOptions { return eventstore.QueryOptions{} }
