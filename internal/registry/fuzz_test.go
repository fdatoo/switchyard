package registry_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/eventstore"
	"github.com/fdatoo/gohome/internal/registry"
	"github.com/fdatoo/gohome/internal/storage"
)

// FuzzRegistryApply feeds arbitrary bytes as a serialized event payload into
// the registry projector. The projector must never panic regardless of the
// input — invalid payloads should surface as decode errors or be ignored,
// not crash the daemon. This guards the boundary between the eventstore
// (which delivers untyped bytes via proto.Unmarshal) and the projector.
func FuzzRegistryApply(f *testing.F) {
	// Seed corpus: well-formed payloads representative of real traffic.
	seedPayloads := [][]byte{}

	for _, p := range []*eventv1.Payload{
		{Kind: &eventv1.Payload_DriverEvent{DriverEvent: &eventv1.DriverEvent{
			DriverInstanceId: "drv1", Kind: "started",
		}}},
		{Kind: &eventv1.Payload_DriverEvent{DriverEvent: &eventv1.DriverEvent{
			DriverInstanceId: "drv1", Kind: "heartbeat",
		}}},
		{Kind: &eventv1.Payload_EntityUnregistered{EntityUnregistered: &eventv1.EntityUnregistered{}}},
	} {
		b, err := proto.Marshal(p)
		if err == nil {
			seedPayloads = append(seedPayloads, b)
		}
	}
	for _, b := range seedPayloads {
		f.Add(b)
	}
	// Plus a couple of degenerate seeds.
	f.Add([]byte{})
	f.Add([]byte{0xff, 0xff, 0xff, 0xff})

	ctx := context.Background()
	dir := f.TempDir()
	db, err := storage.Open(ctx, storage.Config{Path: filepath.Join(dir, "fuzz.db")})
	if err != nil {
		f.Fatalf("storage.Open: %v", err)
	}
	f.Cleanup(func() { _ = db.Close() })
	reg, err := registry.New(ctx, db)
	if err != nil {
		f.Fatalf("registry.New: %v", err)
	}

	f.Fuzz(func(t *testing.T, payloadBytes []byte) {
		payload := &eventv1.Payload{}
		if err := proto.Unmarshal(payloadBytes, payload); err != nil {
			// Malformed payload — eventstore would surface this as a decode error.
			// The projector never sees it, so nothing to fuzz here.
			return
		}
		evt := eventstore.Event{
			Position:  1,
			Timestamp: time.Unix(0, 0),
			Kind:      "fuzz",
			Entity:    "fuzz.entity",
			Source:    "fuzz",
			Payload:   payload,
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Skipf("BeginTx: %v", err) // tx-availability flakes shouldn't fail the fuzz run
			return
		}
		// Apply must not panic. It may return an error (e.g. FK conflict from
		// a rolled-back state); rollback and continue. Any panic = bug.
		_ = reg.Apply(ctx, tx, evt)
		_ = tx.Rollback()
	})
}
