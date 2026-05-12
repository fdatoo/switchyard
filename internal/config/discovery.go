package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/apple/pkl-go/pkl"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
)

// discoveredAutomation pairs a discovered AutomationConfig with the source
// file (relative to configDir) so merge-time errors can attribute back to
// the originating file.
type discoveredAutomation struct {
	Path   string
	Config *configpb.AutomationConfig
}

type discoveredArea struct {
	Path   string
	Config *configpb.AreaConfig
}

type discoveredScene struct {
	Path   string
	Config *configpb.SceneConfig
}

type discoveryResult struct {
	Automations []discoveredAutomation
	Areas       []discoveredArea
	Scenes      []discoveredScene
	EntityAreas map[string]string
}

// discoverConfigDir walks <configDir>/{automations,areas,scenes}/*.pkl and
// <configDir>/entity-areas.pkl, evaluates each via the shared Pkl evaluator,
// and decodes the JSON results into proto types. Per-file errors are
// returned as ValidationErrors with File set relative to configDir and the
// corresponding file dropped from the result. Missing directories produce
// no error.
func discoverConfigDir(ctx context.Context, ev *pklEvaluator, configDir string) (discoveryResult, []ValidationError) {
	var (
		mu     sync.Mutex
		result = discoveryResult{EntityAreas: map[string]string{}}
		errs   []ValidationError
	)

	addErr := func(e ValidationError) {
		mu.Lock()
		errs = append(errs, e)
		mu.Unlock()
	}

	jobs := collectJobs(configDir, addErr)

	// Bounded parallelism.
	maxWorkers := 8
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for _, j := range jobs {
		j := j
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			runDiscoveryJob(ctx, ev, j, &mu, &result, addErr)
		}()
	}

	// entity-areas.pkl is a singleton — run inline.
	loadEntityAreas(ctx, ev, configDir, &result, addErr)

	wg.Wait()

	// Deterministic ordering: sort each kind by id.
	sort.Slice(result.Automations, func(i, j int) bool {
		return result.Automations[i].Config.GetId() < result.Automations[j].Config.GetId()
	})
	sort.Slice(result.Areas, func(i, j int) bool {
		return result.Areas[i].Config.GetId() < result.Areas[j].Config.GetId()
	})
	sort.Slice(result.Scenes, func(i, j int) bool {
		return result.Scenes[i].Config.GetId() < result.Scenes[j].Config.GetId()
	})

	return result, errs
}

type discoveryJob struct {
	relPath string
	absPath string
	kind    string // "automation" | "area" | "scene"
}

func collectJobs(configDir string, addErr func(ValidationError)) []discoveryJob {
	kinds := []struct {
		dir  string
		kind string
	}{
		{"automations", "automation"},
		{"areas", "area"},
		{"scenes", "scene"},
	}

	var jobs []discoveryJob
	for _, k := range kinds {
		absDir := filepath.Join(configDir, k.dir)
		entries, err := os.ReadDir(absDir)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			addErr(ValidationError{
				Code:    "discovery_read_dir",
				File:    k.dir,
				Message: err.Error(),
			})
			continue
		}
		for _, ent := range entries {
			if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".pkl") {
				continue
			}
			jobs = append(jobs, discoveryJob{
				relPath: filepath.Join(k.dir, ent.Name()),
				absPath: filepath.Join(absDir, ent.Name()),
				kind:    k.kind,
			})
		}
	}
	return jobs
}

var pklErrLineRE = regexp.MustCompile(`(?m)^\s*(\d+)\s*\|`)

func pklErrorLine(msg string) int {
	m := pklErrLineRE.FindStringSubmatch(msg)
	if len(m) < 2 {
		return 0
	}
	var n int
	fmt.Sscanf(m[1], "%d", &n)
	return n
}

func runDiscoveryJob(ctx context.Context, ev *pklEvaluator, j discoveryJob, mu *sync.Mutex, result *discoveryResult, addErr func(ValidationError)) {
	text, err := ev.ev.EvaluateOutputText(ctx, pkl.FileSource(j.absPath))
	if err != nil {
		addErr(ValidationError{
			Code:    "pkl_eval",
			File:    j.relPath,
			Line:    pklErrorLine(err.Error()),
			Message: err.Error(),
		})
		return
	}

	switch j.kind {
	case "automation":
		var aj automationJSON
		if err := json.Unmarshal([]byte(text), &aj); err != nil {
			addErr(ValidationError{Code: "json_decode", File: j.relPath, Message: err.Error()})
			return
		}
		cfg, err := automationFromJSON(aj)
		if err != nil {
			addErr(ValidationError{Code: "decode", File: j.relPath, Message: err.Error()})
			return
		}
		mu.Lock()
		result.Automations = append(result.Automations, discoveredAutomation{Path: j.relPath, Config: cfg})
		mu.Unlock()
	case "area":
		var aj areaJSON
		if err := json.Unmarshal([]byte(text), &aj); err != nil {
			addErr(ValidationError{Code: "json_decode", File: j.relPath, Message: err.Error()})
			return
		}
		cfg := areaFromJSON(aj)
		mu.Lock()
		result.Areas = append(result.Areas, discoveredArea{Path: j.relPath, Config: cfg})
		mu.Unlock()
	case "scene":
		var sj sceneJSON
		if err := json.Unmarshal([]byte(text), &sj); err != nil {
			addErr(ValidationError{Code: "json_decode", File: j.relPath, Message: err.Error()})
			return
		}
		cfg, err := sceneFromJSON(sj)
		if err != nil {
			addErr(ValidationError{Code: "decode", File: j.relPath, Message: err.Error()})
			return
		}
		mu.Lock()
		result.Scenes = append(result.Scenes, discoveredScene{Path: j.relPath, Config: cfg})
		mu.Unlock()
	}
}

func loadEntityAreas(ctx context.Context, ev *pklEvaluator, configDir string, result *discoveryResult, addErr func(ValidationError)) {
	path := filepath.Join(configDir, "entity-areas.pkl")
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return
		}
		addErr(ValidationError{Code: "discovery_stat", File: "entity-areas.pkl", Message: err.Error()})
		return
	}
	text, err := ev.ev.EvaluateOutputText(ctx, pkl.FileSource(path))
	if err != nil {
		addErr(ValidationError{
			Code:    "pkl_eval",
			File:    "entity-areas.pkl",
			Line:    pklErrorLine(err.Error()),
			Message: err.Error(),
		})
		return
	}
	var raw struct {
		EntityAreas map[string]string `json:"entityAreas"`
	}
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		addErr(ValidationError{Code: "json_decode", File: "entity-areas.pkl", Message: err.Error()})
		return
	}
	for k, v := range raw.EntityAreas {
		result.EntityAreas[k] = v
	}
}
