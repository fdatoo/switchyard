// Package editsession implements the EditSessionService Connect-RPC handler.
// It manages long-lived Pkl file edit sessions: the UI is canonical during a
// session; the on-disk file is not written until CommitEdit. Multiple sessions
// on the same file are allowed; first-to-commit wins.
package editsession

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/editsession/v1"
	"github.com/fdatoo/switchyard/gen/switchyard/editsession/v1/editsessionv1connect"
)

// ConfigReloader can reload configuration after a successful CommitEdit.
// Optional; if nil, no reload is triggered.
type ConfigReloader interface {
	Apply(ctx context.Context, force bool) error
}

// sessionMeta holds the snapshot taken when a session was opened.
type sessionMeta struct {
	filePath    string
	fileHash    string
	ancestorPkl string
	lockToken   string
}

// Service implements EditSessionServiceHandler.
type Service struct {
	locks     *LockManager
	watcher   *FileWatcher
	reloader  ConfigReloader
	logger    *slog.Logger
	configDir string // root directory for Pkl/Starlark config files

	mu       sync.Mutex
	sessions map[string]*sessionMeta // key: session_id
}

// Compile-time assertion.
var _ editsessionv1connect.EditSessionServiceHandler = (*Service)(nil)

// NewService creates a Service. watcher may be nil in tests that do not
// exercise the SessionEvents stream. configDir is the root directory for
// Pkl/Starlark files; it may be empty in tests that do not exercise ListFiles.
func NewService(locks *LockManager, watcher *FileWatcher, reloader ConfigReloader, logger *slog.Logger, configDir string) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		locks:     locks,
		watcher:   watcher,
		reloader:  reloader,
		logger:    logger,
		configDir: configDir,
		sessions:  make(map[string]*sessionMeta),
	}
}

// OpenForEdit opens a file for editing.
func (s *Service) OpenForEdit(_ context.Context, req *connect.Request[v1.OpenForEditRequest]) (*connect.Response[v1.OpenForEditResponse], error) {
	path := req.Msg.GetFilePath()
	if path == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("file_path required"))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("file not found: %s", path))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	fileHash := hashBytes(data)
	ancestorPkl := string(data)

	// AST JSON: we return an empty object for now.
	// A full AST parser would populate this field.
	// TODO: populate ast_json once Pkl exposes a stable parse API.
	astJSON := "{}"

	token, err := s.locks.Acquire(path)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	sessionID, err := newToken()
	if err != nil {
		s.locks.Release(token)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	s.mu.Lock()
	s.sessions[sessionID] = &sessionMeta{
		filePath:    path,
		fileHash:    fileHash,
		ancestorPkl: ancestorPkl,
		lockToken:   token,
	}
	s.mu.Unlock()

	return connect.NewResponse(&v1.OpenForEditResponse{
		SessionId:   sessionID,
		LockToken:   token,
		FileHash:    fileHash,
		AncestorPkl: ancestorPkl,
		AstJson:     astJSON,
	}), nil
}

// CommitEdit writes the regenerated Pkl to disk.
func (s *Service) CommitEdit(_ context.Context, req *connect.Request[v1.CommitEditRequest]) (*connect.Response[v1.CommitEditResponse], error) {
	path := req.Msg.GetFilePath()
	token := req.Msg.GetLockToken()

	// Validate lock
	valid, expired := s.locks.Validate(token)
	if expired {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("LOCK_EXPIRED"))
	}
	if !valid {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("invalid lock token"))
	}

	// Retrieve session meta for ancestor_pkl
	s.mu.Lock()
	var meta *sessionMeta
	for _, m := range s.sessions {
		if m.lockToken == token {
			meta = m
			break
		}
	}
	s.mu.Unlock()

	ancestorPkl := ""
	if meta != nil {
		ancestorPkl = meta.ancestorPkl
	}

	// Read current disk content
	diskData, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	diskHash := ""
	if diskData != nil {
		diskHash = hashBytes(diskData)
	}

	force := req.Msg.GetForce()
	expectedHash := req.Msg.GetExpectedFileHash()

	// Hash mismatch + not force → conflict
	if !force && diskHash != expectedHash {
		s.logger.Info("edit session commit conflict",
			"path", path,
			"expected_hash", expectedHash,
			"disk_hash", diskHash,
		)
		return connect.NewResponse(&v1.CommitEditResponse{
			Result: &v1.CommitEditResponse_Conflict{
				Conflict: &v1.CommitConflict{
					DiskHash:    diskHash,
					DiskPkl:     string(diskData),
					AncestorPkl: ancestorPkl,
				},
			},
		}), nil
	}

	if force {
		s.logger.Warn("edit session force overwrite",
			"path", path,
			"disk_hash", diskHash,
			"expected_hash", expectedHash,
		)
	}

	// Write file
	newContent := req.Msg.GetRegeneratedPkl()
	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("write file: %w", err))
	}

	newHash := hashBytes([]byte(newContent))
	s.logger.Info("edit session committed", "path", path, "new_hash", newHash)

	return connect.NewResponse(&v1.CommitEditResponse{
		Result: &v1.CommitEditResponse_Success{
			Success: &v1.CommitSuccess{
				NewFileHash: newHash,
			},
		},
	}), nil
}

// AbandonEdit releases the session lock.
func (s *Service) AbandonEdit(_ context.Context, req *connect.Request[v1.AbandonEditRequest]) (*connect.Response[v1.AbandonEditResponse], error) {
	token := req.Msg.GetLockToken()
	s.locks.Release(token)

	// Clean up session metadata
	s.mu.Lock()
	for id, m := range s.sessions {
		if m.lockToken == token {
			delete(s.sessions, id)
			break
		}
	}
	s.mu.Unlock()

	return connect.NewResponse(&v1.AbandonEditResponse{}), nil
}

// SessionEvents streams events to the client for the given session.
// ExternalEditDetected events are pushed when the watched file changes.
// A heartbeat every 5 minutes resets the server-side lock TTL.
func (s *Service) SessionEvents(ctx context.Context, req *connect.Request[v1.SessionEventsRequest], stream *connect.ServerStream[v1.SessionEvent]) error {
	sessionID := req.Msg.GetSessionId()
	if sessionID == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("session_id required"))
	}

	s.mu.Lock()
	meta := s.sessions[sessionID]
	s.mu.Unlock()

	if meta == nil {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("session %s not found", sessionID))
	}

	// Register watcher subscription if watcher is available
	var eventCh <-chan FileEvent
	var unsubscribe func()
	if s.watcher != nil {
		ch, unsub := s.watcher.Subscribe(meta.filePath)
		eventCh = ch
		unsubscribe = unsub
		defer unsubscribe()
	} else {
		// No watcher: use a nil channel (blocks forever on select)
		eventCh = nil
	}

	// Heartbeat ticker
	heartbeatTicker := time.NewTicker(5 * time.Minute)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case evt, ok := <-eventCh:
			if !ok {
				return nil
			}
			if err := stream.Send(&v1.SessionEvent{
				Kind: &v1.SessionEvent_ExternalEdit{
					ExternalEdit: &v1.ExternalEditDetected{
						FilePath:   evt.Path,
						NewHash:    evt.Hash,
						ModifiedAt: timestamppb.New(evt.ModifiedAt),
					},
				},
			}); err != nil {
				return err
			}

		case t := <-heartbeatTicker.C:
			// Reset lock TTL
			_ = s.locks.Heartbeat(meta.lockToken)
			if err := stream.Send(&v1.SessionEvent{
				Kind: &v1.SessionEvent_Heartbeat{
					Heartbeat: &v1.SessionHeartbeat{
						ServerTime: timestamppb.New(t),
					},
				},
			}); err != nil {
				return err
			}
		}
	}
}

// AnalyzeRegenerability delegates to AnalyzeFile.
func (s *Service) AnalyzeRegenerability(_ context.Context, req *connect.Request[v1.AnalyzeRegenerabilityRequest]) (*connect.Response[v1.RegenerabilityReport], error) {
	path := req.Msg.GetFilePath()
	if path == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("file_path required"))
	}

	regions, err := AnalyzeFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("file not found: %s", path))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	pbRegions := make([]*v1.FileOnlyRegion, 0, len(regions))
	for _, r := range regions {
		pbRegions = append(pbRegions, &v1.FileOnlyRegion{
			StartLine: r.StartLine,
			EndLine:   r.EndLine,
			Reason:    r.Reason,
		})
	}

	return connect.NewResponse(&v1.RegenerabilityReport{
		FileOnlyRegions: pbRegions,
	}), nil
}

// ListFiles walks the config root and returns all Pkl and Starlark files.
func (s *Service) ListFiles(_ context.Context, _ *connect.Request[v1.ListFilesRequest]) (*connect.Response[v1.ListFilesResponse], error) {
	if s.configDir == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("config_dir not configured"))
	}

	var entries []*v1.FileEntry
	err := filepath.WalkDir(s.configDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".pkl") && !strings.HasSuffix(name, ".star") {
			return nil
		}
		rel, relErr := filepath.Rel(s.configDir, path)
		if relErr != nil {
			return nil
		}
		entries = append(entries, &v1.FileEntry{
			Path:     rel,
			HasError: false,
		})
		return nil
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("walk config dir: %w", err))
	}

	return connect.NewResponse(&v1.ListFilesResponse{
		Files: entries,
	}), nil
}

// hashBytes returns the SHA-256 hex digest of b.
func hashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
