package widgetpack

import (
	"context"
	"encoding/json"
	"fmt"

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
func EvalManifest(ctx context.Context, manifestPath string) (*Manifest, error) {
	ev, err := pkl.NewEvaluator(ctx, pkl.PreconfiguredOptions,
		config.SwitchyardSchemeReaderOption(),
		func(opts *pkl.EvaluatorOptions) { opts.OutputFormat = "json" },
	)
	if err != nil {
		return nil, fmt.Errorf("widgetpack: pkl evaluator: %w", err)
	}
	defer ev.Close()

	text, err := ev.EvaluateOutputText(ctx, pkl.FileSource(manifestPath))
	if err != nil {
		return nil, fmt.Errorf("widgetpack: evaluate manifest %q: %w", manifestPath, err)
	}

	var wrapper struct {
		Manifest Manifest `json:"manifest"`
	}
	if err := json.Unmarshal([]byte(text), &wrapper); err != nil {
		return nil, fmt.Errorf("widgetpack: decode manifest %q: %w", manifestPath, err)
	}
	if wrapper.Manifest.Name == "" {
		return nil, fmt.Errorf("widgetpack: manifest %q missing required field: name", manifestPath)
	}
	return &wrapper.Manifest, nil
}
