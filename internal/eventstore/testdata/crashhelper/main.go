// Helper binary used by crash-safety tests: opens the DB, appends N events,
// then sleeps forever. Test kills it with -9 mid-loop.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/fdatoo/switchyard/internal/eventstore"
	"github.com/fdatoo/switchyard/internal/observability"
	"github.com/fdatoo/switchyard/internal/storage"
	"github.com/fdatoo/switchyard/internal/testutil"
)

func main() {
	var (
		dbPath = flag.String("db", "", "path to switchyard.db")
		count  = flag.Int("count", 1000, "events to append")
	)
	flag.Parse()

	ctx := context.Background()
	db, err := storage.Open(ctx, storage.Config{Path: *dbPath})
	if err != nil {
		log.Fatal(err)
	}
	store, err := eventstore.Open(ctx, eventstore.Config{}, db, observability.Init(observability.LogConfig{}), observability.NewMetrics())
	if err != nil {
		log.Fatal(err)
	}
	_ = store.Start(ctx)

	_, _ = os.Stderr.WriteString("READY\n")
	_ = os.Stderr.Sync()

	for i := 0; i < *count; i++ {
		if _, err := store.Append(ctx, testutil.StateChanged("light.x", uint32(i))); err != nil {
			log.Fatal(err)
		}
	}
	// Hang — test kills us.
	time.Sleep(time.Hour)
}
