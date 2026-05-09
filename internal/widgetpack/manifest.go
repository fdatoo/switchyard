package widgetpack

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/apple/pkl-go/pkl"

	"github.com/fdatoo/switchyard/internal/config"
)

// Manifest mirrors switchyard.widgets.PackManifest.
type Manifest struct {
	Name        string   `pkl:"name"        json:"name"`
	Version     string   `pkl:"version"     json:"version"`
	Protocol    string   `pkl:"protocol"    json:"protocol"`
	SDKVersion  string   `pkl:"sdkVersion"  json:"sdkVersion"`
	Bundle      string   `pkl:"bundle"      json:"bundle"`
	BundleHash  string   `pkl:"bundleHash"  json:"bundleHash"`
	Classes     []string `pkl:"classes"     json:"classes"`
	Description string   `pkl:"description" json:"description"`
	Homepage    string   `pkl:"homepage"    json:"homepage"`
	License     string   `pkl:"license"     json:"license"`
}

// EvalManifest evaluates a manifest.pkl file using a fresh Pkl evaluator and
// returns the decoded Manifest. The Pkl module's class constraints (e.g.
// protocol == "v1", bundleHash startsWith "sha256:") become evaluator errors
// here, which is the validation we want.
//
// The evaluator is sandboxed: it omits WithOsEnv (so manifests cannot read
// host environment variables via read("env:...")), and sets RootDir to the
// manifest's directory (spec §6 step 4) to prevent file: reads outside the
// staging area.
func EvalManifest(ctx context.Context, manifestPath string) (*Manifest, error) {
	// Hand-composed options — deliberately omits WithOsEnv to prevent untrusted
	// manifests from reading host environment variables.
	manifestEvaluatorOptions := []func(*pkl.EvaluatorOptions){
		pkl.WithDefaultAllowedResources,
		pkl.WithDefaultAllowedModules,
		pkl.WithDefaultCacheDir,
		config.SwitchyardSchemeReaderOption(),
		func(opts *pkl.EvaluatorOptions) {
			opts.OutputFormat = "json"
			opts.RootDir = filepath.Dir(manifestPath)
			opts.Logger = pkl.NoopLogger
		},
	}
	ev, err := pkl.NewEvaluator(ctx, manifestEvaluatorOptions...)
	if err != nil {
		return nil, fmt.Errorf("widgetpack: pkl evaluator: %w", err)
	}
	defer func() { _ = ev.Close() }()

	text, err := ev.EvaluateOutputText(ctx, pkl.FileSource(manifestPath))
	if err != nil {
		return nil, fmt.Errorf("widgetpack: evaluate manifest %q: %w", manifestPath, err)
	}

	var wrapper struct {
		Manifest *Manifest `json:"manifest"`
	}
	if err := json.Unmarshal([]byte(text), &wrapper); err != nil {
		return nil, fmt.Errorf("widgetpack: decode manifest %q: %w", manifestPath, err)
	}
	if wrapper.Manifest == nil {
		return nil, fmt.Errorf("widgetpack: manifest %q: 'manifest' property is null or unset", manifestPath)
	}
	return wrapper.Manifest, nil
}
