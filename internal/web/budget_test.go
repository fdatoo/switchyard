package web_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/fdatoo/switchyard/internal/web"
)

const (
	maxInitialChunkBytes = 350 * 1024 * 4 // 350 KB gzipped ≈ 1.4 MB raw heuristic
	// Monaco is bundled into a lazy chunk for the Pkl/Starlark editor (Plan 12).
	// Total raw assets allow up to ~16 MB to accommodate Monaco + workers, all
	// of which load on demand (operators opening the editor) rather than on
	// the initial daily-use path.
	maxTotalAssetsBytes = 4000 * 1024 * 4 // 4000 KB gzipped ≈ 16 MB raw heuristic
)

func TestAssetBudget(t *testing.T) {
	dist, err := fs.Sub(web.Assets, "dist/assets")
	if err != nil {
		t.Skipf("dist/assets not found (run web:build first): %v", err)
	}
	var total int64
	var initial int64
	err = fs.WalkDir(dist, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		if strings.HasPrefix(p, "index-") && strings.HasSuffix(p, ".js") {
			initial += info.Size()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if initial > int64(maxInitialChunkBytes) {
		t.Errorf("initial chunk %d B exceeds budget %d B (raw)", initial, maxInitialChunkBytes)
	}
	if total > int64(maxTotalAssetsBytes) {
		t.Errorf("total assets %d B exceeds budget %d B (raw)", total, maxTotalAssetsBytes)
	}
}
