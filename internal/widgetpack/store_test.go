package widgetpack_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fdatoo/switchyard/internal/widgetpack"
)

func TestStore_AddPersistsAndReloads(t *testing.T) {
	dir := t.TempDir()
	s := widgetpack.NewStore(filepath.Join(dir, "widgets"))
	if err := s.Load(context.Background()); err != nil {
		t.Fatalf("Load empty: %v", err)
	}

	// Pre-create the pack directory so Load on a fresh store doesn't prune this entry.
	if err := os.MkdirAll(filepath.Join(dir, "widgets/p/1.0.0"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if err := s.Add(context.Background(), widgetpack.InstalledPack{
		Name: "p", Version: "1.0.0", SHA256: "sha256:abc",
		Classes: []string{"X"}, SignatureStatus: "verified",
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	// Reopen → entry must be there.
	s2 := widgetpack.NewStore(filepath.Join(dir, "widgets"))
	if err := s2.Load(context.Background()); err != nil {
		t.Fatalf("reload Load: %v", err)
	}
	got, err := s2.Get(context.Background(), "p", "1.0.0")
	if err != nil {
		t.Fatalf("Get after reload: %v", err)
	}
	if got.SHA256 != "sha256:abc" {
		t.Errorf("SHA256=%q, want sha256:abc", got.SHA256)
	}
}

func TestStore_LoadDropsStaleEntries(t *testing.T) {
	dir := t.TempDir()
	s := widgetpack.NewStore(filepath.Join(dir, "widgets"))
	_ = s.Load(context.Background())
	_ = s.Add(context.Background(), widgetpack.InstalledPack{
		Name: "ghost", Version: "1.0.0", SHA256: "sha256:zzz",
	})
	// Don't actually create the pack dir — Load on a fresh store should drop it.
	s2 := widgetpack.NewStore(filepath.Join(dir, "widgets"))
	if err := s2.Load(context.Background()); err != nil {
		t.Fatalf("reload Load: %v", err)
	}
	if _, err := s2.Get(context.Background(), "ghost", "1.0.0"); err == nil {
		t.Error("expected stale entry dropped after Load")
	}
}

func TestStore_SubscribeFanOut(t *testing.T) {
	s := widgetpack.NewStore(t.TempDir())
	_ = s.Load(context.Background())

	chA := make(chan widgetpack.WatchEvent, 4)
	chB := make(chan widgetpack.WatchEvent, 4)
	unsubA := s.Subscribe(chA)
	unsubB := s.Subscribe(chB)
	defer unsubA()
	defer unsubB()

	pack := widgetpack.InstalledPack{Name: "p", Version: "1.0.0", SHA256: "sha256:x"}
	if err := s.Add(context.Background(), pack); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Both subscribers receive the event.
	for i, ch := range []chan widgetpack.WatchEvent{chA, chB} {
		select {
		case ev := <-ch:
			if ev.Installed == nil || ev.Installed.Name != "p" {
				t.Errorf("subscriber %d: bad event %+v", i, ev)
			}
		case <-time.After(time.Second):
			t.Errorf("subscriber %d: no event delivered", i)
		}
	}

	// Unsubscribe A; subsequent event only reaches B.
	unsubA()
	if err := s.Remove(context.Background(), "p", "1.0.0"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	select {
	case ev := <-chB:
		if ev.Uninstalled == nil {
			t.Errorf("expected uninstalled event, got %+v", ev)
		}
	case <-time.After(time.Second):
		t.Error("subscriber B: no uninstall event")
	}
	select {
	case ev := <-chA:
		t.Errorf("unsubscribed A still received event: %+v", ev)
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestStore_MultiVersion(t *testing.T) {
	s := widgetpack.NewStore(t.TempDir())
	_ = s.Load(context.Background())
	for _, v := range []string{"1.0.0", "1.1.0", "2.0.0"} {
		if err := s.Add(context.Background(), widgetpack.InstalledPack{Name: "p", Version: v, SHA256: "sha256:" + v}); err != nil {
			t.Fatalf("Add %s: %v", v, err)
		}
	}
	packs, _ := s.List(context.Background())
	if len(packs) != 3 {
		t.Errorf("List len = %d, want 3", len(packs))
	}
}

func TestStore_ConcurrentAddRemove(t *testing.T) {
	s := widgetpack.NewStore(t.TempDir())
	_ = s.Load(context.Background())
	const N = 50
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			pack := widgetpack.InstalledPack{Name: "p", Version: fmtV(i), SHA256: "sha256:x"}
			_ = s.Add(context.Background(), pack)
			_ = s.Remove(context.Background(), pack.Name, pack.Version)
		}()
	}
	wg.Wait()
}

func TestStore_AddEventDoesNotAliasLiveState(t *testing.T) {
	s := widgetpack.NewStore(t.TempDir())
	_ = s.Load(context.Background())

	ch := make(chan widgetpack.WatchEvent, 1)
	unsub := s.Subscribe(ch)
	defer unsub()

	if err := s.Add(context.Background(), widgetpack.InstalledPack{
		Name: "p", Version: "1.0.0", SHA256: "sha256:original",
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	ev := <-ch
	if ev.Installed == nil {
		t.Fatal("expected Installed event")
	}
	// Mutate the event payload — must not affect the live store entry.
	ev.Installed.SHA256 = "sha256:mutated"

	got, err := s.Get(context.Background(), "p", "1.0.0")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.SHA256 != "sha256:original" {
		t.Errorf("live store SHA256 = %q after mutating event payload; want sha256:original", got.SHA256)
	}
}

func TestStore_ClassesView(t *testing.T) {
	s := widgetpack.NewStore(t.TempDir())
	_ = s.Load(context.Background())
	_ = s.Add(context.Background(), widgetpack.InstalledPack{
		Name: "bar", Version: "1.0.0", SHA256: "sha256:abc",
		Classes: []string{"BarChart", "PieChart"},
	})
	view := s.ClassesView()
	if len(view) != 1 || view[0].Name != "bar" {
		t.Fatalf("view = %+v", view)
	}
	if len(view[0].Classes) != 2 {
		t.Errorf("classes = %d", len(view[0].Classes))
	}
	if view[0].Classes[0].BundleURL != "/widgets/bar/1.0.0/bundle.js?h=sha256:abc" {
		t.Errorf("BundleURL = %q", view[0].Classes[0].BundleURL)
	}
}

func fmtV(i int) string { return "1.0." + itoa(i) }

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + itoa(i%10)
}
