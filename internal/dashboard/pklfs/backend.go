package pklfs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/fdatoo/switchyard/internal/config"
	"github.com/fdatoo/switchyard/internal/dashboard"
	"github.com/fdatoo/switchyard/internal/dashboard/regen"
)

var slugRE = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

type Backend struct {
	configDir  string
	driversDir string
}

func New(configDir, driversDir string) *Backend {
	return &Backend{configDir: configDir, driversDir: driversDir}
}

func (b *Backend) List(ctx context.Context) ([]dashboard.DashboardMeta, error) {
	entries, err := os.ReadDir(b.dashboardDir())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	metas := make([]dashboard.DashboardMeta, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".pkl") || strings.HasSuffix(name, ".layout.pkl") {
			continue
		}
		slug := strings.TrimSuffix(name, ".pkl")
		d, err := b.Get(ctx, slug)
		if err != nil {
			return nil, err
		}
		metas = append(metas, dashboard.DashboardMeta{Slug: d.Slug, Title: d.Title})
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].Slug < metas[j].Slug })
	return metas, nil
}

func (b *Backend) Get(ctx context.Context, slug string) (*dashboard.DashboardData, error) {
	if !validSlug(slug) {
		return nil, dashboard.ErrDashboardNotFound
	}
	sourcePath := b.sourcePath(slug)
	sourcePkl, err := os.ReadFile(sourcePath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, dashboard.ErrDashboardNotFound
	}
	if err != nil {
		return nil, err
	}
	cfg, err := config.EvaluateDashboardFile(ctx, sourcePath, b.driversDir)
	if err != nil {
		return nil, err
	}
	d, err := dataFromConfig(cfg.GetContent())
	if err != nil {
		return nil, err
	}
	d.SourcePkl = string(sourcePkl)
	layoutPkl, err := os.ReadFile(b.layoutPath(slug))
	switch {
	case err == nil:
		d.LayoutPkl = string(layoutPkl)
		d.WysiwygWritable = true
	case errors.Is(err, fs.ErrNotExist):
		d.WysiwygWritable = false
	default:
		return nil, err
	}
	return d, nil
}

func (b *Backend) Create(ctx context.Context, slug, title string) (*dashboard.DashboardData, error) {
	if !validSlug(slug) {
		return nil, fmt.Errorf("dashboard: invalid slug %q", slug)
	}
	if _, err := os.Stat(b.sourcePath(slug)); err == nil {
		return nil, fmt.Errorf("dashboard: %s already exists", slug)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if _, err := os.Stat(b.layoutPath(slug)); err == nil {
		return nil, fmt.Errorf("dashboard: %s layout already exists", slug)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if err := dashboard.ScaffoldDashboard(b.dashboardDir(), slug, title); err != nil {
		return nil, err
	}
	return b.Get(ctx, slug)
}

func (b *Backend) Delete(_ context.Context, slug string, both bool) error {
	if !validSlug(slug) {
		return dashboard.ErrDashboardNotFound
	}
	if err := os.Remove(b.sourcePath(slug)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return dashboard.ErrDashboardNotFound
		}
		return err
	}
	if both {
		if err := os.Remove(b.layoutPath(slug)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (b *Backend) SaveLayout(ctx context.Context, d *dashboard.DashboardData) (*dashboard.DashboardData, string, error) {
	if d == nil || !validSlug(d.Slug) {
		return nil, "", dashboard.ErrDashboardNotFound
	}
	layoutPath := b.layoutPath(d.Slug)
	if _, err := os.Stat(layoutPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "", dashboard.ErrDashboardNotFound
		}
		return nil, "", err
	}
	content, err := regen.Render(d)
	if err != nil {
		return nil, "", err
	}
	if err := atomicWrite(layoutPath, content, 0o644); err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(content)
	saved, err := b.Get(ctx, d.Slug)
	if err != nil {
		return nil, "", err
	}
	return saved, hex.EncodeToString(sum[:]), nil
}

func (b *Backend) dashboardDir() string { return filepath.Join(b.configDir, "dashboards") }
func (b *Backend) sourcePath(slug string) string {
	return filepath.Join(b.dashboardDir(), slug+".pkl")
}
func (b *Backend) layoutPath(slug string) string {
	return filepath.Join(b.dashboardDir(), slug+".layout.pkl")
}

func validSlug(slug string) bool {
	return slugRE.MatchString(slug)
}

func atomicWrite(path string, content []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

type dashboardJSON struct {
	Slug    string          `json:"slug"`
	Title   string          `json:"title"`
	Grid    gridJSON        `json:"grid"`
	Widgets []widgetJSON    `json:"widgets"`
	Raw     json.RawMessage `json:"-"`
}

type gridJSON struct {
	Columns   int32 `json:"columns"`
	RowHeight int32 `json:"rowHeight"`
}

type widgetJSON struct {
	ID          string         `json:"id"`
	WidgetClass string         `json:"widgetClass"`
	Pos         positionJSON   `json:"pos"`
	Props       map[string]any `json:"props"`
	IsContainer bool           `json:"-"`
	ChildGrid   gridJSON       `json:"childGrid"`
	Children    []widgetJSON   `json:"children"`
}

type positionJSON struct {
	X      int32 `json:"x"`
	Y      int32 `json:"y"`
	Width  int32 `json:"width"`
	Height int32 `json:"height"`
}

func dataFromConfig(content []byte) (*dashboard.DashboardData, error) {
	var raw dashboardJSON
	if err := json.Unmarshal(content, &raw); err != nil {
		return nil, fmt.Errorf("dashboard content: %w", err)
	}
	return &dashboard.DashboardData{
		Slug:    raw.Slug,
		Title:   raw.Title,
		Grid:    gridFromJSON(raw.Grid),
		Widgets: widgetsFromJSON(raw.Widgets),
	}, nil
}

func gridFromJSON(g gridJSON) dashboard.GridData {
	if g.Columns == 0 {
		g.Columns = 12
	}
	if g.RowHeight == 0 {
		g.RowHeight = 60
	}
	return dashboard.GridData{Columns: g.Columns, RowHeight: g.RowHeight}
}

func widgetsFromJSON(ws []widgetJSON) []dashboard.WidgetData {
	out := make([]dashboard.WidgetData, 0, len(ws))
	for _, w := range ws {
		isContainer := len(w.Children) > 0 || w.WidgetClass == ""
		out = append(out, dashboard.WidgetData{
			ID:          w.ID,
			ClassID:     w.WidgetClass,
			Pos:         dashboard.PosData{X: w.Pos.X, Y: w.Pos.Y, W: w.Pos.Width, H: w.Pos.Height},
			Props:       w.Props,
			IsContainer: isContainer,
			ChildGrid:   gridFromJSON(w.ChildGrid),
			Children:    widgetsFromJSON(w.Children),
		})
	}
	return out
}
