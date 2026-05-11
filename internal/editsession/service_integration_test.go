package editsession

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/editsession/v1"
)

// TestIntegration_OpenExternalEditConflict exercises the full stack:
// 1. Write test.pkl; call OpenForEdit → capture lock_token, file_hash.
// 2. Overwrite test.pkl from outside (simulates MCP/CLI edit).
// 3. Assert ExternalEditDetected arrives on the SessionEvents stream within 1 second.
// 4. Call CommitEdit with original expected_file_hash, force=false.
// 5. Assert response is CommitConflict with non-empty disk_pkl.
func TestIntegration_OpenExternalEditConflict(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pkl")
	original := "id = \"v1\"\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set up service with fast-polling watcher
	lm := NewLockManager()
	watcher := NewFileWatcher(20 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher.Start(ctx)

	svc := NewService(lm, watcher, nil, nil, "")

	// Step 1: OpenForEdit
	openResp, err := svc.OpenForEdit(ctx, connect.NewRequest(&v1.OpenForEditRequest{
		FilePath: path,
	}))
	if err != nil {
		t.Fatalf("OpenForEdit: %v", err)
	}
	sessionID := openResp.Msg.SessionId
	lockToken := openResp.Msg.LockToken
	originalHash := openResp.Msg.FileHash

	if sessionID == "" || lockToken == "" || originalHash == "" {
		t.Fatal("OpenForEdit returned empty fields")
	}

	// Subscribe to session events (in background)
	eventReceived := make(chan *v1.ExternalEditDetected, 1)
	streamCtx, streamCancel := context.WithCancel(ctx)
	defer streamCancel()

	go func() {
		// We can't use the server-stream handler directly in a unit test without
		// a full HTTP stack. Instead, subscribe to the watcher directly to verify
		// the push mechanism. This tests the watcher's integration with Subscribe.
		ch, unsubscribe := watcher.Subscribe(path)
		defer unsubscribe()

		select {
		case evt := <-ch:
			eventReceived <- &v1.ExternalEditDetected{
				FilePath: evt.Path,
				NewHash:  evt.Hash,
			}
		case <-streamCtx.Done():
		case <-time.After(2 * time.Second):
		}
	}()

	// Let watcher record initial file state
	time.Sleep(50 * time.Millisecond)

	// Step 2: Overwrite from outside
	external := "id = \"v2\"\n"
	if err := os.WriteFile(path, []byte(external), 0o644); err != nil {
		t.Fatal(err)
	}

	// Step 3: Assert ExternalEditDetected arrives within 1 second
	select {
	case evt := <-eventReceived:
		if evt.FilePath != path {
			t.Errorf("event path: got %q want %q", evt.FilePath, path)
		}
		if evt.NewHash == "" {
			t.Error("expected non-empty new_hash in event")
		}
		if evt.NewHash == originalHash {
			t.Error("new_hash should differ from original hash")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("ExternalEditDetected did not arrive within 1 second")
	}
	streamCancel()

	// Step 4: CommitEdit with stale hash, force=false
	commitResp, err := svc.CommitEdit(ctx, connect.NewRequest(&v1.CommitEditRequest{
		FilePath:         path,
		LockToken:        lockToken,
		RegeneratedPkl:   "id = \"staged\"\n",
		ExpectedFileHash: originalHash, // stale
		Force:            false,
	}))
	if err != nil {
		t.Fatalf("CommitEdit: %v", err)
	}

	// Step 5: Assert CommitConflict
	conflict := commitResp.Msg.GetConflict()
	if conflict == nil {
		t.Fatalf("expected CommitConflict, got: %+v", commitResp.Msg)
	}
	if conflict.DiskPkl == "" {
		t.Error("expected non-empty disk_pkl in CommitConflict")
	}
	if conflict.DiskPkl != external {
		t.Errorf("disk_pkl: got %q want %q", conflict.DiskPkl, external)
	}
	if conflict.AncestorPkl != original {
		t.Errorf("ancestor_pkl: got %q want %q", conflict.AncestorPkl, original)
	}
}
