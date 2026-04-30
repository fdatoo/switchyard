package starlark

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	starlarkgo "go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// makeLoader returns a thread.Load function resolving "//..."-prefixed paths
// relative to configDir. inProgress tracks the current load chain for cycle
// detection — callers must pass a fresh empty map per Execute call.
func (r *Runtime) makeLoader(inProgress map[string]bool) func(*starlarkgo.Thread, string) (starlarkgo.StringDict, error) {
	return func(thread *starlarkgo.Thread, module string) (starlarkgo.StringDict, error) {
		if !strings.HasPrefix(module, "//") {
			return nil, fmt.Errorf("load: only //...-prefixed paths are supported, got %q", module)
		}
		rel := module[2:]
		if strings.Contains(rel, "..") {
			return nil, fmt.Errorf("load: path traversal not allowed: %q", module)
		}

		absPath := filepath.Join(r.configDir, rel)
		if !strings.HasPrefix(absPath+string(filepath.Separator), r.configDir+string(filepath.Separator)) {
			return nil, fmt.Errorf("load: path escapes configDir: %q", module)
		}

		// cache hit
		r.mu.RLock()
		if dict, ok := r.moduleCache[absPath]; ok {
			r.mu.RUnlock()
			return dict, nil
		}
		r.mu.RUnlock()

		// cycle detection
		if inProgress[absPath] {
			return nil, fmt.Errorf("load: circular dependency detected: %q", module)
		}
		inProgress[absPath] = true
		defer delete(inProgress, absPath)

		src, err := os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("load %q: %w", module, err)
		}

		cfg := limitsFor(KindScript)
		loadThread := &starlarkgo.Thread{
			Name: "load:" + module,
			Load: r.makeLoader(inProgress),
		}
		loadThread.SetMaxExecutionSteps(cfg.MaxSteps)

		dict, err := starlarkgo.ExecFileOptions(&syntax.FileOptions{TopLevelControl: true, GlobalReassign: true}, loadThread, absPath, src, starlarkgo.StringDict{})
		if err != nil {
			return nil, fmt.Errorf("load %q: %w", module, err)
		}

		// export only names not starting with "_"
		exported := make(starlarkgo.StringDict, len(dict))
		for k, v := range dict {
			if !strings.HasPrefix(k, "_") {
				exported[k] = v
			}
		}

		r.mu.Lock()
		r.moduleCache[absPath] = exported
		r.mu.Unlock()

		return exported, nil
	}
}
