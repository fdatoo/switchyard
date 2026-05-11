package display

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	displayv1 "github.com/fdatoo/switchyard/gen/switchyard/display/v1"
	"github.com/fdatoo/switchyard/gen/switchyard/display/v1/displayv1connect"
)

// Service implements displayv1connect.DisplayServiceHandler.
type Service struct {
	store    *Store
	pairCode *PairCodeStore
}

var _ displayv1connect.DisplayServiceHandler = (*Service)(nil)

// NewService returns a new DisplayService backed by the given data directory.
func NewService(dataDir string, pc *PairCodeStore) *Service {
	return &Service{
		store:    NewStore(dataDir),
		pairCode: pc,
	}
}

// Pair generates a 6-digit pairing code and returns it with its expiry.
func (s *Service) Pair(ctx context.Context, _ *connect.Request[displayv1.PairRequest]) (*connect.Response[displayv1.PairResponse], error) {
	code, exp, err := s.pairCode.Issue()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&displayv1.PairResponse{
		Code:      code,
		ExpiresAt: exp.Unix(),
	}), nil
}

// RedeemPairCode claims a pairing code, creates the Display record, and returns
// a per-display token.
func (s *Service) RedeemPairCode(ctx context.Context, req *connect.Request[displayv1.RedeemPairCodeRequest]) (*connect.Response[displayv1.RedeemPairCodeResponse], error) {
	code := strings.TrimSpace(req.Msg.GetCode())
	if len(code) != 6 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("display: code must be 6 digits"))
	}

	if err := s.pairCode.Redeem(code); err != nil {
		switch {
		case errors.Is(err, errCodeNotFound), errors.Is(err, errCodeExpired):
			return nil, connect.NewError(connect.CodeNotFound, err)
		default:
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	id := uuid.New().String()
	token, err := generateToken()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	now := time.Now()
	rec := displayRecord{
		ID:         id,
		DeviceName: req.Msg.GetDeviceName(),
		CreatedAt:  now,
		Token:      token,
	}

	// Apply fidelity recommender defaults with empty room slice.
	recommender := NewFidelityRecommender()
	rec.TileOverrides = recommender.Recommend(nil)

	if err := s.store.Put(rec); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&displayv1.RedeemPairCodeResponse{
		DisplayId: id,
		Token:     token,
	}), nil
}

// List returns all paired displays.
func (s *Service) List(ctx context.Context, _ *connect.Request[displayv1.ListDisplaysRequest]) (*connect.Response[displayv1.ListDisplaysResponse], error) {
	recs, err := s.store.List()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*displayv1.Display, 0, len(recs))
	for _, r := range recs {
		out = append(out, r.toProto())
	}
	return connect.NewResponse(&displayv1.ListDisplaysResponse{Displays: out}), nil
}

// Get returns a single display by ID.
func (s *Service) Get(ctx context.Context, req *connect.Request[displayv1.GetDisplayRequest]) (*connect.Response[displayv1.GetDisplayResponse], error) {
	rec, err := s.store.Get(req.Msg.GetId())
	if err != nil {
		if errors.Is(err, errNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&displayv1.GetDisplayResponse{Display: rec.toProto()}), nil
}

// Update persists a configuration update for a display.
func (s *Service) Update(ctx context.Context, req *connect.Request[displayv1.UpdateDisplayRequest]) (*connect.Response[displayv1.UpdateDisplayResponse], error) {
	rec, err := s.store.Get(req.Msg.GetId())
	if err != nil {
		if errors.Is(err, errNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	cfg := req.Msg.GetConfig()
	if cfg != nil {
		if cfg.PageSlug != "" {
			rec.PageSlug = cfg.PageSlug
		}
		if cfg.DeviceName != "" {
			rec.DeviceName = cfg.DeviceName
		}
		if cfg.AlertThreshold != displayv1.AlertThreshold_ALERT_THRESHOLD_UNSPECIFIED {
			rec.AlertThreshold = cfg.AlertThreshold
		}
		if cfg.IdleBehavior != nil {
			rec.IdleBehavior = cfg.IdleBehavior
		}
		if len(cfg.AllowedInteractions) > 0 {
			rec.AllowedInteractions = cfg.AllowedInteractions
		}
		if len(cfg.TileOverrides) > 0 {
			if rec.TileOverrides == nil {
				rec.TileOverrides = make(map[string]*displayv1.FidelityOverride)
			}
			for k, v := range cfg.TileOverrides {
				rec.TileOverrides[k] = v
			}
		}
	}

	if err := s.store.Put(rec); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&displayv1.UpdateDisplayResponse{Display: rec.toProto()}), nil
}

// Unpair removes a display.
func (s *Service) Unpair(ctx context.Context, req *connect.Request[displayv1.UnpairDisplayRequest]) (*connect.Response[displayv1.UnpairDisplayResponse], error) {
	if err := s.store.Delete(req.Msg.GetId()); err != nil {
		if errors.Is(err, errNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&displayv1.UnpairDisplayResponse{}), nil
}

// ---------------------------------------------------------------------------
// Token generation
// ---------------------------------------------------------------------------

// generateToken returns a cryptographically random URL-safe token string.
func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("display: generate token: %w", err)
	}
	return "sydisp_" + base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(buf), nil
}

// ---------------------------------------------------------------------------
// Store — JSON filesystem backend
// ---------------------------------------------------------------------------

// displayRecord is the on-disk representation of a Display.
type displayRecord struct {
	ID                  string                             `json:"id"`
	DeviceName          string                             `json:"device_name"`
	PageSlug            string                             `json:"page_slug,omitempty"`
	TileOverrides       map[string]*displayv1.FidelityOverride `json:"tile_overrides,omitempty"`
	IdleBehavior        *displayv1.IdleBehavior            `json:"idle_behavior,omitempty"`
	AllowedInteractions []string                           `json:"allowed_interactions,omitempty"`
	AlertThreshold      displayv1.AlertThreshold           `json:"alert_threshold,omitempty"`
	Token               string                             `json:"token"`
	CreatedAt           time.Time                          `json:"created_at"`
	LastSeenAt          *time.Time                         `json:"last_seen_at,omitempty"`
}

func (r displayRecord) toProto() *displayv1.Display {
	d := &displayv1.Display{
		Id:                  r.ID,
		DeviceName:          r.DeviceName,
		PageSlug:            r.PageSlug,
		TileOverrides:       r.TileOverrides,
		IdleBehavior:        r.IdleBehavior,
		AllowedInteractions: r.AllowedInteractions,
		AlertThreshold:      r.AlertThreshold,
		CreatedAt:           timestamppb.New(r.CreatedAt),
	}
	if r.LastSeenAt != nil {
		d.LastSeenAt = timestamppb.New(*r.LastSeenAt)
	}
	return d
}

// Store is a simple JSON-file-per-display storage backend.
type Store struct {
	dir string
	mu  sync.RWMutex
}

// NewStore returns a Store backed by the given directory.
func NewStore(dir string) *Store { return &Store{dir: dir} }

func (s *Store) path(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func (s *Store) ensureDir() error {
	return os.MkdirAll(s.dir, 0o700)
}

// Put writes a display record atomically.
func (s *Store) Put(r displayRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureDir(); err != nil {
		return err
	}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	tmp := s.path(r.ID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path(r.ID))
}

// Get reads a display record by ID.
func (s *Store) Get(id string) (displayRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := os.ReadFile(s.path(id))
	if errors.Is(err, fs.ErrNotExist) {
		return displayRecord{}, errNotFound
	}
	if err != nil {
		return displayRecord{}, err
	}
	var r displayRecord
	if err := json.Unmarshal(data, &r); err != nil {
		return displayRecord{}, err
	}
	return r, nil
}

// List returns all display records sorted by creation time.
func (s *Store) List() ([]displayRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var recs []displayRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || strings.HasSuffix(e.Name(), ".tmp") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var r displayRecord
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		r.ID = id
		recs = append(recs, r)
	}
	sort.Slice(recs, func(i, j int) bool {
		return recs[i].CreatedAt.Before(recs[j].CreatedAt)
	})
	return recs, nil
}

// Delete removes a display record.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(s.path(id))
	if errors.Is(err, fs.ErrNotExist) {
		return errNotFound
	}
	return err
}

// ValidateToken checks whether the given token matches the stored token for
// the given display ID.
func (s *Store) ValidateToken(id, token string) (bool, error) {
	rec, err := s.Get(id)
	if err != nil {
		return false, err
	}
	return rec.Token == token, nil
}
