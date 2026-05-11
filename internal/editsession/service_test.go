package editsession

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/editsession/v1"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	lm := NewLockManager()
	svc := NewService(lm, nil, nil, nil, "")
	return svc
}

func writeTmpPkl(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestService_OpenForEdit_HappyPath(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(t)

	content := `amends "switchyard:automations"` + "\nid = \"test\"\n"
	path := writeTmpPkl(t, dir, "test.pkl", content)

	resp, err := svc.OpenForEdit(context.Background(), connect.NewRequest(&v1.OpenForEditRequest{
		FilePath: path,
	}))
	if err != nil {
		t.Fatalf("OpenForEdit: %v", err)
	}
	if resp.Msg.SessionId == "" {
		t.Error("expected non-empty session_id")
	}
	if resp.Msg.LockToken == "" {
		t.Error("expected non-empty lock_token")
	}
	if resp.Msg.FileHash == "" {
		t.Error("expected non-empty file_hash")
	}
	if resp.Msg.AncestorPkl != content {
		t.Errorf("ancestor_pkl mismatch: got %q", resp.Msg.AncestorPkl)
	}
}

func TestService_OpenForEdit_NotFound(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.OpenForEdit(context.Background(), connect.NewRequest(&v1.OpenForEditRequest{
		FilePath: "/nonexistent/file.pkl",
	}))
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
	var connErr *connect.Error
	if ok := false; !ok {
		// just check it is a connect error
		connErr, ok = err.(*connect.Error)
		if !ok || connErr.Code() != connect.CodeNotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	}
}

func TestService_CommitEdit_HappyPath(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(t)

	content := "id = \"orig\"\n"
	path := writeTmpPkl(t, dir, "commit.pkl", content)

	openResp, _ := svc.OpenForEdit(context.Background(), connect.NewRequest(&v1.OpenForEditRequest{FilePath: path}))

	newContent := "id = \"updated\"\n"
	commitResp, err := svc.CommitEdit(context.Background(), connect.NewRequest(&v1.CommitEditRequest{
		FilePath:         path,
		LockToken:        openResp.Msg.LockToken,
		RegeneratedPkl:   newContent,
		ExpectedFileHash: openResp.Msg.FileHash,
		Force:            false,
	}))
	if err != nil {
		t.Fatalf("CommitEdit: %v", err)
	}
	if commitResp.Msg.GetSuccess() == nil {
		t.Fatalf("expected success, got conflict: %+v", commitResp.Msg)
	}

	// Verify on-disk content
	got, _ := os.ReadFile(path)
	if string(got) != newContent {
		t.Errorf("disk content: got %q want %q", string(got), newContent)
	}
}

func TestService_CommitEdit_Conflict_OnHashMismatch(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(t)

	path := writeTmpPkl(t, dir, "conflict.pkl", "id = \"v1\"\n")

	openResp, _ := svc.OpenForEdit(context.Background(), connect.NewRequest(&v1.OpenForEditRequest{FilePath: path}))

	// Simulate external edit: overwrite the file
	if err := os.WriteFile(path, []byte("id = \"v2\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	commitResp, err := svc.CommitEdit(context.Background(), connect.NewRequest(&v1.CommitEditRequest{
		FilePath:         path,
		LockToken:        openResp.Msg.LockToken,
		RegeneratedPkl:   "id = \"staged\"\n",
		ExpectedFileHash: openResp.Msg.FileHash, // stale hash
		Force:            false,
	}))
	if err != nil {
		t.Fatalf("CommitEdit unexpected error: %v", err)
	}
	conflict := commitResp.Msg.GetConflict()
	if conflict == nil {
		t.Fatalf("expected conflict, got success: %+v", commitResp.Msg)
	}
	if conflict.DiskPkl != "id = \"v2\"\n" {
		t.Errorf("conflict.disk_pkl: got %q", conflict.DiskPkl)
	}
	if conflict.AncestorPkl != "id = \"v1\"\n" {
		t.Errorf("conflict.ancestor_pkl: got %q", conflict.AncestorPkl)
	}
}

func TestService_CommitEdit_Force_OverridesConflict(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(t)

	path := writeTmpPkl(t, dir, "force.pkl", "id = \"v1\"\n")

	openResp, _ := svc.OpenForEdit(context.Background(), connect.NewRequest(&v1.OpenForEditRequest{FilePath: path}))

	// External edit
	_ = os.WriteFile(path, []byte("id = \"v2\"\n"), 0o644)

	commitResp, err := svc.CommitEdit(context.Background(), connect.NewRequest(&v1.CommitEditRequest{
		FilePath:         path,
		LockToken:        openResp.Msg.LockToken,
		RegeneratedPkl:   "id = \"forced\"\n",
		ExpectedFileHash: openResp.Msg.FileHash,
		Force:            true,
	}))
	if err != nil {
		t.Fatalf("CommitEdit force: %v", err)
	}
	if commitResp.Msg.GetSuccess() == nil {
		t.Fatalf("expected success with force=true")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "id = \"forced\"\n" {
		t.Errorf("force overwrite: got %q", string(got))
	}
}

func TestService_AbandonEdit_ReleasesLock(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(t)

	path := writeTmpPkl(t, dir, "abandon.pkl", "id = \"x\"\n")

	openResp, _ := svc.OpenForEdit(context.Background(), connect.NewRequest(&v1.OpenForEditRequest{FilePath: path}))
	token := openResp.Msg.LockToken

	_, err := svc.AbandonEdit(context.Background(), connect.NewRequest(&v1.AbandonEditRequest{
		FilePath:  path,
		LockToken: token,
	}))
	if err != nil {
		t.Fatalf("AbandonEdit: %v", err)
	}

	// Lock should now be invalid
	ok, expired := svc.locks.Validate(token)
	if ok || expired {
		t.Errorf("expected invalid (not expired) after abandon, got ok=%v expired=%v", ok, expired)
	}
}

func TestService_CommitEdit_ExpiredLock(t *testing.T) {
	dir := t.TempDir()
	lm := newLockManagerWithTTL(0) // immediately expired
	svc := NewService(lm, nil, nil, nil, "")

	path := writeTmpPkl(t, dir, "expired.pkl", "id = \"e\"\n")

	// Acquire a token manually (already expired since TTL=0)
	token, _ := lm.Acquire(path)

	_, err := svc.CommitEdit(context.Background(), connect.NewRequest(&v1.CommitEditRequest{
		FilePath:         path,
		LockToken:        token,
		RegeneratedPkl:   "id = \"new\"\n",
		ExpectedFileHash: "anyhash",
	}))
	if err == nil {
		t.Fatal("expected error for expired lock")
	}
	connErr, ok := err.(*connect.Error)
	if !ok || connErr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", err)
	}
}

func TestService_AnalyzeRegenerability(t *testing.T) {
	dir := t.TempDir()
	svc := newTestService(t)

	path := writeTmpPkl(t, dir, "analyze.pkl", "starlark(\"x\")\n")

	resp, err := svc.AnalyzeRegenerability(context.Background(), connect.NewRequest(&v1.AnalyzeRegenerabilityRequest{
		FilePath: path,
	}))
	if err != nil {
		t.Fatalf("AnalyzeRegenerability: %v", err)
	}
	if len(resp.Msg.FileOnlyRegions) != 1 {
		t.Errorf("expected 1 region, got %d", len(resp.Msg.FileOnlyRegions))
	}
}

func TestService_ListFiles_HappyPath(t *testing.T) {
	dir := t.TempDir()

	// Create a mix of Pkl, Starlark, and other files.
	writeTmpPkl(t, dir, "main.pkl", "")
	writeTmpPkl(t, dir, "automations.pkl", "")
	if err := os.WriteFile(filepath.Join(dir, "script.star"), []byte(""), 0o644); err != nil {
		t.Fatalf("write star: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write json: %v", err)
	}

	lm := NewLockManager()
	svc := NewService(lm, nil, nil, nil, dir)

	resp, err := svc.ListFiles(context.Background(), connect.NewRequest(&v1.ListFilesRequest{}))
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}

	paths := make(map[string]bool, len(resp.Msg.Files))
	for _, f := range resp.Msg.Files {
		paths[f.Path] = true
	}

	if !paths["main.pkl"] {
		t.Error("expected main.pkl in ListFiles response")
	}
	if !paths["automations.pkl"] {
		t.Error("expected automations.pkl in ListFiles response")
	}
	if !paths["script.star"] {
		t.Error("expected script.star in ListFiles response")
	}
	if paths["ignored.json"] {
		t.Error("ignored.json should not appear in ListFiles response")
	}
	if len(resp.Msg.Files) != 3 {
		t.Errorf("expected 3 files, got %d", len(resp.Msg.Files))
	}
}

func TestService_ListFiles_NoConfigDir(t *testing.T) {
	lm := NewLockManager()
	svc := NewService(lm, nil, nil, nil, "")

	_, err := svc.ListFiles(context.Background(), connect.NewRequest(&v1.ListFilesRequest{}))
	if err == nil {
		t.Fatal("expected error when configDir is empty")
	}
	connErr, ok := err.(*connect.Error)
	if !ok || connErr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", err)
	}
}
