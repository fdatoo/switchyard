package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func ExportSwitchyardPklModules(dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("mkdir switchyard pkl namespace: %w", err)
	}
	return fs.WalkDir(pklFS, "pkl/switchyard", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".pkl") {
			return nil
		}
		data, err := pklFS.ReadFile(path)
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(path, "pkl/switchyard/")
		out := filepath.Join(dst, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			return err
		}
		return nil
	})
}
