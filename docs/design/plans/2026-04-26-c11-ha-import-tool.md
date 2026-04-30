# C11 — HA Import Tool Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `gohome import-ha` — a single-shot CLI subcommand that reads a Home Assistant config directory and writes a git-initable gohome Pkl tree, with a Jinja → Starlark transpiler, per-integration mappers (shipped as drivers do), a deterministic diagnostics surface (inline `# FIXME` / `# NOTE` plus `IMPORT_REPORT.md`), and a secrets pipeline that keeps values out of git.

**Architecture:** A new top-level `internal/importer/` package owns the pipeline: `Scanner` walks the HA dir, `yaml_loader` resolves HA's `!include` / `!secret` / `!input` custom tags into a typed `HAModel`, per-area subpackages (`core`, `registry`, `automations`, `scripts`, `scenes`, `auth`, `secrets`) plus per-integration mappers in `mappers/` produce in-memory `ImportResult` fragments, the `jinja/` subpackage wraps `gonja` for HA-template transpilation, the `diagnostics/` subpackage collects FIXMEs/NOTEs and renders `IMPORT_REPORT.md`, and `writer/` materializes the result to disk via `text/template`-based canonical Pkl emitters. The CLI subcommand lives in `internal/cli/cmd_import.go` with lipgloss styling under `internal/cli/styles_import.go`.

**Tech Stack:** Go 1.25 (already in tree), `gopkg.in/yaml.v3` (already transitively), `github.com/nikolalohinski/gonja/v2` (new — Jinja2 in Go), `text/template` (stdlib — Pkl emitters), `encoding/json` (stdlib — `.storage/*`), existing `cobra` + lipgloss patterns from `internal/cli/`. Stdlib `testing` everywhere; no testify (matches C9 / C10 conventions).

**Depends on:** C1 (event schema; not directly used here but `gohome.imported.UnmappedIntegration` Pkl reuses C4's `gohome.config.Module` shape), C4 (Pkl module shapes that emitted files conform to), C5 (Starlark stdlib that the transpiler emits against — `state(...)`, `time.now()`, etc.), C9 (auth Pkl shapes for `auth/users.pkl`), C10 (dashboards Pkl shapes; touched only enough to detect Lovelace presence and emit a NOTE — no actual translation here).

---

## Codebase orientation

Before starting, read these files to understand existing patterns this plan extends:

| File | Why |
|---|---|
| `docs/superpowers/specs/2026-04-26-c11-ha-import-tool-design.md` | This plan's source of truth |
| `docs/superpowers/specs/2026-04-21-gohome-master-design.md` (§6.5, §8.1) | Output directory layout + the HA construct → gohome target mapping table |
| `docs/superpowers/specs/2026-04-22-c4-pkl-config-design.md` | Pkl module shapes the emitter writes against |
| `docs/superpowers/specs/2026-04-22-c5-starlark-runtime-design.md` | Starlark stdlib symbols the Jinja transpiler emits (`state(...)`, `time.now()`, ...) |
| `docs/superpowers/specs/2026-04-23-c6-automation-engine-design.md` | Automation trigger/condition/action shapes the importer produces |
| `docs/superpowers/specs/2026-04-25-c9-auth-and-policy-design.md` | `gohome.auth.User` Pkl shape `auth/users.pkl` conforms to |
| `gohome/CLAUDE.md` | "Definition of done" + package map + architectural invariants — this plan must satisfy them |
| `gohome/Taskfile.yml` | `task build` / `test` / `test:race` / `test:integration` / `lint` — canonical commands |
| `gohome/internal/cli/root.go` | Where new subcommands register |
| `gohome/internal/cli/cmd_mcp.go` | Pattern for a Cobra command tree with subcommands and lipgloss output |
| `gohome/internal/cli/cmd_automation.go` | Larger Cobra command with table rendering — closest model for `gohome import-ha`'s completion-summary box |
| `gohome/internal/cli/styles.go`, `styles_mcp.go`, `styles_automation.go` | Lipgloss style patterns; `styles_import.go` follows the same shape |
| `gohome/internal/cli/cliutil.go` | `Output()`, error rendering, `--no-color` honoring — consume these, don't re-invent |
| `gohome/internal/config/pkl/gohome/dashboards.pkl` | (post-C10) Lovelace-presence detection writes a NOTE; we do not produce dashboard Pkl |
| `gohome/internal/config/pkl/gohome/widgets.pkl` | Same — read-only reference |
| `gohome/internal/config/pkl/gohome/auth.pkl` | Reference for `User`/`Role`/`Policy` shapes when writing `auth/users.pkl` |
| `gohome/go.mod` | Where the `gonja` dependency lands |

---

## File map

### New files (all in `gohome/`)

| Path | Responsibility |
|---|---|
| `internal/importer/importer.go` | Top-level `Run(opts) (*Result, error)` entry; orchestrates the pipeline |
| `internal/importer/opts.go` | `ImportOptions` struct (input dir, output dir, dry-run, force, verbose flags) |
| `internal/importer/scanner.go` | Walks `<ha-dir>`, classifies files, returns `ScannedConfig` (no I/O on contents) |
| `internal/importer/yaml_loader.go` | Reads classified files; handles `!include` / `!include_dir_*` / `!secret` / `!input` tags; returns `HAModel` |
| `internal/importer/types.go` | `HAModel`, `ScannedConfig`, `Result`, common helpers |
| `internal/importer/scanner_test.go`, `yaml_loader_test.go`, `importer_test.go` | Per-file unit tests |
| `internal/importer/mappers/mapper.go` | `Mapper` interface + `Input`/`Output` shapes |
| `internal/importer/mappers/catalog.go` | Process-wide registry (`Register`, `Lookup`, `All`) populated via `init()` |
| `internal/importer/mappers/unmapped.go` | Generic `UnmappedIntegration` emitter — handles outside-v1.0 *and* unpublished-driver cases |
| `internal/importer/mappers/template.go` | Always-available HA `template:` platform → `ComputedEntity` mapper |
| `internal/importer/mappers/mqtt.go` | First real per-driver mapper (chosen because MQTT's gohome driver Pkl is the most likely first publish) |
| `internal/importer/mappers/<integration>.go` | One file per v1.0 integration whose driver Pkl is published at C11-implementation time |
| `internal/importer/mappers/*_test.go` | Per-mapper tests (minimal/full/missing/extra fixtures) |
| `internal/importer/jinja/transpile.go` | `Transpile(input string) (output string, []diagnostics.Diagnostic)` |
| `internal/importer/jinja/visitor.go` | gonja AST walker emitting Starlark per construct |
| `internal/importer/jinja/stdlib.go` | HA helper → gohome Starlark stdlib mappings (`now()` → `time.now()`, etc.) |
| `internal/importer/jinja/transpile_test.go`, `visitor_test.go` | Golden table-driven tests |
| `internal/importer/core/core.go` | `configuration.yaml` → `main.pkl` + `settings.pkl` |
| `internal/importer/core/core_test.go` | Unit tests |
| `internal/importer/registry/areas.go` | `.storage/core.area_registry` → `areas.pkl` |
| `internal/importer/registry/zones.go` | `configuration.yaml` zone block → `zones.pkl` |
| `internal/importer/registry/entities.go` | `.storage/core.entity_registry` → `entities/overrides.pkl` |
| `internal/importer/registry/devices.go` | `.storage/core.device_registry` → diagnostic only |
| `internal/importer/registry/registry_test.go` | Unit tests |
| `internal/importer/automations/automations.go` | `automations.yaml` (+ split form) → `automations/<slug>.pkl` + handlers |
| `internal/importer/automations/triggers.go` | Per-trigger-platform translation table |
| `internal/importer/automations/conditions.go` | Per-condition-platform translation table |
| `internal/importer/automations/actions.go` | Per-action-type translation table |
| `internal/importer/automations/automations_test.go` | Unit tests for each translation table row |
| `internal/importer/scripts/scripts.go` | `scripts.yaml` (+ split form) → `scripts/<slug>.pkl` + bodies |
| `internal/importer/scripts/scripts_test.go` | Unit tests |
| `internal/importer/scenes/scenes.go` | `scenes.yaml` → `scenes.pkl` |
| `internal/importer/scenes/scenes_test.go` | Unit tests |
| `internal/importer/auth/auth.go` | `.storage/auth*` + `.storage/person` → `auth/users.pkl` + role/policy stubs |
| `internal/importer/auth/auth_test.go` | Unit tests |
| `internal/importer/secrets/secrets.go` | `secrets.yaml` → `secrets.pkl` + `IMPORTED_SECRETS.env` + `.gitignore` line |
| `internal/importer/secrets/secrets_test.go` | Unit tests |
| `internal/importer/diagnostics/diagnostics.go` | `Collector`, `Diagnostic`, `Severity`, `Reason` |
| `internal/importer/diagnostics/report.go` | `IMPORT_REPORT.md` renderer |
| `internal/importer/diagnostics/diagnostics_test.go`, `report_test.go` | Unit tests |
| `internal/importer/writer/writer.go` | `WriteAll(outDir string, r *Result) error` — atomic writes, perms, refuse-non-empty |
| `internal/importer/writer/pkl_print.go` | `text/template`-based canonical Pkl emitters per output file type |
| `internal/importer/writer/writer_test.go`, `pkl_print_test.go` | Unit tests + golden output assertions |
| `internal/importer/integration_test.go` | `//go:build integration` end-to-end against the two fixture HA dirs |
| `internal/importer/testdata/README.md` | Anonymization rules |
| `internal/importer/testdata/configs/minimal/` | Anonymized minimal HA fixture (2 integrations, 3 automations, 1 scene, 1 user) |
| `internal/importer/testdata/configs/minimal/expected/` | Expected gohome output tree |
| `internal/importer/testdata/configs/kitchensink/` | Anonymized stress-test HA fixture |
| `internal/importer/testdata/configs/kitchensink/expected/` | Expected gohome output tree |
| `internal/importer/testdata/mappers/<integration>/{minimal,full,missing,extra}/{in.yaml,out.pkl}` | Per-mapper unit fixtures |
| `internal/importer/testdata/jinja/<case>/{in.txt,out.star,diags.json}` | Per-construct transpiler fixtures |
| `internal/importer/testdata/yaml_loader/<tag>/in/` (+ `out.json`) | Per-tag loader fixtures |
| `internal/importer/testdata/diagnostics/<scenario>/{in.json,out.md}` | Report renderer fixtures |
| `internal/cli/cmd_import.go` | Cobra `gohome import-ha` command |
| `internal/cli/styles_import.go` | Lipgloss styles: `PhaseHeading`, `SummaryBox`, `FixmeBadge`, `NoteBadge`, `ReportPath`, `ErrorBox` |
| `internal/cli/cmd_import_test.go` | CLI tests (flag parsing, exit codes, dry-run output capture) |
| `internal/config/pkl/gohome/imported.pkl` | New top-level Pkl module: `gohome.imported.UnmappedIntegration` placeholder class used by `mappers/unmapped.go` |
| `docs/import-ha.md` | User-facing guide: bootstrap walkthrough, FIXME workflow, secrets handling, `--dry-run` usage |

### Modified files (in `gohome/`)

| Path | Change |
|---|---|
| `go.mod`, `go.sum` | Add `github.com/nikolalohinski/gonja/v2` |
| `internal/cli/root.go` | Register `cmd_import.NewCommand()` in the root command tree |
| `internal/config/pkl/gohome/imported.pkl` | NEW (listed above) — declared here so the loader is aware |
| `README.md` | Add "Migrating from Home Assistant" section linking `docs/import-ha.md` |

---

## Task 1: cobra scaffold for `gohome import-ha`

Creates the top-level `gohome import-ha` subcommand returning `not implemented` so CI stays green and reviewers can see the surface area being added. No real importer logic yet.

**Files:**
- Create: `internal/cli/cmd_import.go`, `internal/cli/styles_import.go`, `internal/cli/cmd_import_test.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Write the failing CLI test**

`internal/cli/cmd_import_test.go`:

```go
package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fynn-labs/gohome/internal/cli"
)

func TestImportCmd_RequiresOutputFlag(t *testing.T) {
	cmd := cli.NewImportCommand()
	cmd.SetArgs([]string{"./somewhere"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when -o is missing")
	}
	if !strings.Contains(stderr.String(), "out") {
		t.Errorf("stderr missing 'out': %s", stderr.String())
	}
}

func TestImportCmd_ReturnsUnimplementedForNow(t *testing.T) {
	cmd := cli.NewImportCommand()
	cmd.SetArgs([]string{"./fake-ha-dir", "-o", "./out"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected unimplemented error")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("error = %q, want 'not yet implemented'", err.Error())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd gohome
go test ./internal/cli/ -run TestImportCmd
```

Expected: FAIL — `cli.NewImportCommand` undefined.

- [ ] **Step 3: Create `internal/cli/styles_import.go`**

```go
package cli

import "github.com/charmbracelet/lipgloss"

var (
	importPhaseHeading = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	importVerbose      = lipgloss.NewStyle().Foreground(fgMutedColor)
	importSummaryBox   = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)
	importFixmeBadge = lipgloss.NewStyle().
		Background(warningColor).
		Foreground(bgColor).
		Padding(0, 1)
	importNoteBadge = lipgloss.NewStyle().
		Foreground(fgMutedColor)
	importReportPath = lipgloss.NewStyle().
		Underline(true).
		Foreground(accentColor)
	importErrorBox = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(dangerColor).
		Padding(0, 1)
)
```

(`accentColor`, `bgColor`, `borderColor`, `dangerColor`, `fgMutedColor`, `warningColor` are existing exports from `internal/cli/styles.go` — confirm names match before writing. If they're named differently in current `styles.go` (e.g., `colorAccent`), adjust here to match the actual exports.)

- [ ] **Step 4: Create `internal/cli/cmd_import.go`**

```go
package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// NewImportCommand returns the `gohome import-ha` cobra command. The
// real importer pipeline is wired in via internal/importer; for now
// the command parses its flags and returns an unimplemented error so
// the surface is visible from day one.
func NewImportCommand() *cobra.Command {
	var opts struct {
		out     string
		dryRun  bool
		force   bool
		verbose bool
		quiet   bool
		noColor bool
	}

	cmd := &cobra.Command{
		Use:   "import-ha [ha-dir]",
		Short: "Import a Home Assistant config directory into a gohome Pkl tree",
		Long: `Reads a Home Assistant config directory and produces a fresh,
git-initable gohome Pkl tree at the path given by --out. Single-shot:
refuses to overwrite a non-empty output dir without --force.

If [ha-dir] is omitted, falls back to ~/.homeassistant then /config.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.out == "" {
				return errors.New("import-ha: --out is required")
			}
			if opts.verbose && opts.quiet {
				return errors.New("import-ha: -v and -q are mutually exclusive")
			}
			return fmt.Errorf("import-ha: not yet implemented (see docs/superpowers/plans/2026-04-26-c11-ha-import-tool.md)")
		},
	}

	cmd.Flags().StringVarP(&opts.out, "out", "o", "", "output directory (required)")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "render the report to stdout, write nothing")
	cmd.Flags().BoolVarP(&opts.force, "force", "f", false, "overwrite a non-empty output directory")
	cmd.Flags().BoolVarP(&opts.verbose, "verbose", "v", false, "per-file logging to stderr")
	cmd.Flags().BoolVarP(&opts.quiet, "quiet", "q", false, "errors only")
	cmd.Flags().BoolVar(&opts.noColor, "no-color", false, "disable color output (auto-detected by default)")

	return cmd
}
```

- [ ] **Step 5: Register in `internal/cli/root.go`**

Find the spot where existing subcommands are added (search for `AddCommand`). Add:

```go
root.AddCommand(NewImportCommand())
```

- [ ] **Step 6: Verify tests pass**

```bash
cd gohome
go test ./internal/cli/ -run TestImportCmd
```

Expected: PASS.

- [ ] **Step 7: Verify build**

```bash
cd gohome
task build
./dist/gohome import-ha --help
```

Expected: usage text printed, no error.

- [ ] **Step 8: Commit**

```bash
git add internal/cli/cmd_import.go internal/cli/styles_import.go internal/cli/cmd_import_test.go internal/cli/root.go
git commit -m "feat(c11): scaffold gohome import-ha cobra command"
```

---

## Task 2: scanner + YAML loader

The scanner classifies files in the source HA dir; the YAML loader resolves HA's custom tags into a typed `HAModel`.

**Files:**
- Create: `internal/importer/types.go`, `scanner.go`, `yaml_loader.go`
- Create: `internal/importer/scanner_test.go`, `yaml_loader_test.go`
- Create: `internal/importer/testdata/yaml_loader/{include,include_dir_list,include_dir_named,secret,input,recursive_include}/`

- [ ] **Step 1: Define types**

`internal/importer/types.go`:

```go
package importer

// ScannedConfig is what the scanner produces: which files exist where.
// Contents are not loaded yet.
type ScannedConfig struct {
	HADir string
	HAVersion string

	ConfigurationYAML string
	AutomationsYAML   string
	ScriptsYAML       string
	ScenesYAML        string
	SecretsYAML       string
	GroupsYAML        string
	CustomizeYAML     string

	AutomationsDir string
	ScriptsDir     string
	PackagesDir    string

	StorageAreaRegistry   string
	StorageEntityRegistry string
	StorageDeviceRegistry string
	StorageConfigEntries  string
	StorageAuth           string
	StorageAuthHA         string
	StoragePerson         string

	LovelacePresent bool
}

// HAModel is the typed in-memory representation produced by the loader.
// All HA custom tags resolved; secrets values held in memory only.
type HAModel struct {
	HAVersion     string
	Configuration *ConfigurationModel
	Automations   []AutomationModel
	Scripts       []ScriptModel
	Scenes        []SceneModel
	Areas         []AreaModel
	Zones         []ZoneModel
	Entities      []EntityModel
	Devices       []DeviceModel
	ConfigEntries []ConfigEntryModel
	Users         []UserModel
	Persons       []PersonModel
	Secrets       map[string]string
}

type ConfigurationModel struct {
	Raw map[string]any // top-level configuration.yaml mapping
}

type AutomationModel struct {
	ID         string
	Alias      string
	Triggers   []map[string]any
	Conditions []map[string]any
	Actions    []map[string]any
	Mode       string
	SourcePath string
	SourceLine int
}

type ScriptModel struct {
	ID         string
	Alias      string
	Sequence   []map[string]any
	Fields     map[string]any
	SourcePath string
}

type SceneModel struct {
	ID         string
	Name       string
	Entities   map[string]any
	SourcePath string
}

type AreaModel struct {
	ID, Name string
	Labels   []string
	Picture  string
}

type ZoneModel struct {
	ID, Name              string
	Latitude, Longitude   float64
	RadiusMeters          float64
	Passive               bool
}

type EntityModel struct {
	EntityID     string
	UniqueID     string
	Platform     string
	AreaID       string
	DeviceID     string
	Name         string
	Icon         string
	Hidden       bool
	Disabled     bool
}

type DeviceModel struct {
	ID           string
	Name         string
	Manufacturer string
	Model        string
	AreaID       string
	Connections  [][2]string
	Identifiers  [][2]string
}

type ConfigEntryModel struct {
	EntryID    string
	Domain     string
	Title      string
	Data       map[string]any
	Options    map[string]any
}

type UserModel struct {
	ID           string
	Username     string
	Name         string
	IsActive     bool
	IsOwner      bool
	HasPassword  bool
}

type PersonModel struct {
	ID             string
	Name           string
	UserID         string
	DeviceTrackers []string
	Picture        string
}
```

- [ ] **Step 2: Write the failing scanner test**

`internal/importer/scanner_test.go`:

```go
package importer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fynn-labs/gohome/internal/importer"
)

func TestScanner_ClassifiesCommonFiles(t *testing.T) {
	dir := t.TempDir()
	must := func(p, content string) {
		t.Helper()
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must("configuration.yaml", "# minimal\n")
	must("automations.yaml", "[]\n")
	must("scripts.yaml", "{}\n")
	must("scenes.yaml", "[]\n")
	must("secrets.yaml", "k: v\n")
	must(".storage/core.area_registry", `{"data":{"areas":[]}}`)
	must(".storage/core.entity_registry", `{"data":{"entities":[]}}`)
	must(".storage/core.device_registry", `{"data":{"devices":[]}}`)
	must(".storage/auth", `{"data":{"users":[]}}`)
	must(".storage/lovelace", `{"data":{"config":{}}}`)
	must(".HA_VERSION", "2026.3.4\n")

	got, err := importer.ScanHADir(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if got.HADir != dir {
		t.Errorf("HADir = %q, want %q", got.HADir, dir)
	}
	if got.HAVersion != "2026.3.4" {
		t.Errorf("HAVersion = %q, want 2026.3.4", got.HAVersion)
	}
	if got.ConfigurationYAML == "" {
		t.Error("ConfigurationYAML not detected")
	}
	if got.AutomationsYAML == "" {
		t.Error("AutomationsYAML not detected")
	}
	if got.SecretsYAML == "" {
		t.Error("SecretsYAML not detected")
	}
	if got.StorageAreaRegistry == "" {
		t.Error("StorageAreaRegistry not detected")
	}
	if !got.LovelacePresent {
		t.Error("LovelacePresent should be true when .storage/lovelace exists")
	}
}

func TestScanner_AllowsMissingOptionalFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "configuration.yaml"), []byte("# only this\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := importer.ScanHADir(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if got.AutomationsYAML != "" {
		t.Error("AutomationsYAML should be empty when not present")
	}
	if got.LovelacePresent {
		t.Error("LovelacePresent should be false when not present")
	}
}

func TestScanner_FailsOnMissingDir(t *testing.T) {
	if _, err := importer.ScanHADir("/nonexistent/place"); err == nil {
		t.Fatal("expected error for missing dir")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd gohome
go test ./internal/importer/...
```

Expected: FAIL — `ScanHADir` undefined.

- [ ] **Step 4: Implement the scanner**

`internal/importer/scanner.go`:

```go
package importer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScanHADir walks an HA config directory and classifies the files it
// finds. Returns a ScannedConfig with paths to known files (or empty
// strings where they're absent). Reads contents only for .HA_VERSION.
func ScanHADir(haDir string) (*ScannedConfig, error) {
	info, err := os.Stat(haDir)
	if err != nil {
		return nil, fmt.Errorf("scan: stat %q: %w", haDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("scan: %q is not a directory", haDir)
	}

	cfg := &ScannedConfig{HADir: haDir}

	if v, err := os.ReadFile(filepath.Join(haDir, ".HA_VERSION")); err == nil {
		cfg.HAVersion = strings.TrimSpace(string(v))
	}

	mark := func(field *string, name string) {
		p := filepath.Join(haDir, name)
		if _, err := os.Stat(p); err == nil {
			*field = p
		}
	}
	markDir := func(field *string, name string) {
		p := filepath.Join(haDir, name)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			*field = p
		}
	}

	mark(&cfg.ConfigurationYAML, "configuration.yaml")
	mark(&cfg.AutomationsYAML, "automations.yaml")
	mark(&cfg.ScriptsYAML, "scripts.yaml")
	mark(&cfg.ScenesYAML, "scenes.yaml")
	mark(&cfg.SecretsYAML, "secrets.yaml")
	mark(&cfg.GroupsYAML, "groups.yaml")
	mark(&cfg.CustomizeYAML, "customize.yaml")

	markDir(&cfg.AutomationsDir, "automations")
	markDir(&cfg.ScriptsDir, "scripts")
	markDir(&cfg.PackagesDir, "packages")

	mark(&cfg.StorageAreaRegistry, ".storage/core.area_registry")
	mark(&cfg.StorageEntityRegistry, ".storage/core.entity_registry")
	mark(&cfg.StorageDeviceRegistry, ".storage/core.device_registry")
	mark(&cfg.StorageConfigEntries, ".storage/core.config_entries")
	mark(&cfg.StorageAuth, ".storage/auth")
	mark(&cfg.StorageAuthHA, ".storage/auth_provider.homeassistant")
	mark(&cfg.StoragePerson, ".storage/person")

	// Lovelace is detected but never translated.
	for _, name := range []string{".storage/lovelace", ".storage/lovelace.dashboards", "ui-lovelace.yaml"} {
		if _, err := os.Stat(filepath.Join(haDir, name)); err == nil {
			cfg.LovelacePresent = true
			break
		}
	}

	return cfg, nil
}
```

- [ ] **Step 5: Verify scanner tests pass**

```bash
cd gohome
go test ./internal/importer/ -run TestScanner
```

Expected: PASS.

- [ ] **Step 6: Set up YAML loader test fixtures**

For each of the six fixture directories, create the input files. Example for `internal/importer/testdata/yaml_loader/include/in/`:

`main.yaml`:

```yaml
top: !include included.yaml
```

`included.yaml`:

```yaml
nested:
  key: value
```

`internal/importer/testdata/yaml_loader/include/expected.json`:

```json
{
  "top": {
    "nested": { "key": "value" }
  }
}
```

Repeat for `include_dir_list`, `include_dir_named`, `secret`, `input`, `recursive_include` (this last one expects an error).

- [ ] **Step 7: Write the failing loader test**

`internal/importer/yaml_loader_test.go`:

```go
package importer_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/fynn-labs/gohome/internal/importer"
)

func TestYAMLLoader_CustomTags(t *testing.T) {
	cases := []struct {
		name        string
		fixtureDir  string
		entryPoint  string
		secrets     map[string]string
		expectError bool
	}{
		{name: "include", fixtureDir: "testdata/yaml_loader/include/in", entryPoint: "main.yaml"},
		{name: "include_dir_list", fixtureDir: "testdata/yaml_loader/include_dir_list/in", entryPoint: "main.yaml"},
		{name: "include_dir_named", fixtureDir: "testdata/yaml_loader/include_dir_named/in", entryPoint: "main.yaml"},
		{name: "secret", fixtureDir: "testdata/yaml_loader/secret/in", entryPoint: "main.yaml", secrets: map[string]string{"api_key": "abc123"}},
		{name: "recursive_include", fixtureDir: "testdata/yaml_loader/recursive_include/in", entryPoint: "main.yaml", expectError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := importer.LoadYAML(tc.fixtureDir, tc.entryPoint, tc.secrets)
			if tc.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadYAML: %v", err)
			}
			expected, _ := os.ReadFile(filepath.Join(tc.fixtureDir, "..", "expected.json"))
			gotJSON, _ := json.MarshalIndent(got, "", "  ")
			expectedNorm := normalizeJSON(t, expected)
			gotNorm := normalizeJSON(t, gotJSON)
			if string(gotNorm) != string(expectedNorm) {
				t.Errorf("loaded = %s\n want = %s", gotNorm, expectedNorm)
			}
		})
	}
}

func normalizeJSON(t *testing.T, b []byte) []byte {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	out, _ := json.MarshalIndent(v, "", "  ")
	return out
}
```

- [ ] **Step 8: Implement the YAML loader**

`internal/importer/yaml_loader.go`:

```go
package importer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadYAML reads <baseDir>/<entry> recursively, resolving HA's
// custom tags (!include, !include_dir_*, !secret, !input).
// `secrets` maps secret-key names to their values.
func LoadYAML(baseDir, entry string, secrets map[string]string) (any, error) {
	if secrets == nil {
		secrets = map[string]string{}
	}
	visited := map[string]bool{}
	return loadYAMLFile(baseDir, entry, secrets, visited)
}

func loadYAMLFile(baseDir, rel string, secrets map[string]string, visited map[string]bool) (any, error) {
	full := filepath.Join(baseDir, rel)
	if visited[full] {
		return nil, fmt.Errorf("yaml: recursive include detected at %q", rel)
	}
	visited[full] = true
	defer delete(visited, full)

	raw, err := os.ReadFile(full)
	if err != nil {
		return nil, fmt.Errorf("yaml: read %q: %w", full, err)
	}
	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return nil, fmt.Errorf("yaml: parse %q: %w", full, err)
	}
	resolved, err := resolveNode(&node, baseDir, secrets, visited)
	if err != nil {
		return nil, fmt.Errorf("yaml: %s: %w", rel, err)
	}
	return resolved, nil
}

func resolveNode(n *yaml.Node, baseDir string, secrets map[string]string, visited map[string]bool) (any, error) {
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil, nil
		}
		return resolveNode(n.Content[0], baseDir, secrets, visited)

	case yaml.ScalarNode:
		switch n.Tag {
		case "!secret":
			v, ok := secrets[n.Value]
			if !ok {
				return nil, fmt.Errorf("!secret %q: not found in secrets.yaml", n.Value)
			}
			return v, nil
		case "!include":
			return loadYAMLFile(baseDir, n.Value, secrets, visited)
		case "!include_dir_list", "!include_dir_merge_list":
			return resolveIncludeDirList(baseDir, n.Value, n.Tag == "!include_dir_merge_list", secrets, visited)
		case "!include_dir_named", "!include_dir_merge_named":
			return resolveIncludeDirNamed(baseDir, n.Value, n.Tag == "!include_dir_merge_named", secrets, visited)
		case "!input":
			// Blueprints — emit a sentinel string the per-area handler turns into a FIXME.
			return blueprintInputSentinel{Name: n.Value}, nil
		}
		// Fallthrough: ordinary scalar.
		var v any
		if err := n.Decode(&v); err != nil {
			return nil, err
		}
		return v, nil

	case yaml.SequenceNode:
		out := make([]any, 0, len(n.Content))
		for _, item := range n.Content {
			v, err := resolveNode(item, baseDir, secrets, visited)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil

	case yaml.MappingNode:
		out := make(map[string]any, len(n.Content)/2)
		for i := 0; i < len(n.Content); i += 2 {
			k, err := resolveNode(n.Content[i], baseDir, secrets, visited)
			if err != nil {
				return nil, err
			}
			v, err := resolveNode(n.Content[i+1], baseDir, secrets, visited)
			if err != nil {
				return nil, err
			}
			ks, ok := k.(string)
			if !ok {
				ks = fmt.Sprintf("%v", k)
			}
			out[ks] = v
		}
		return out, nil
	}
	return nil, fmt.Errorf("yaml: unhandled node kind %d", n.Kind)
}

func resolveIncludeDirList(baseDir, rel string, merge bool, secrets map[string]string, visited map[string]bool) (any, error) {
	dir := filepath.Join(baseDir, rel)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	if merge {
		out := []any{}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			child, err := loadYAMLFile(baseDir, filepath.Join(rel, e.Name()), secrets, visited)
			if err != nil {
				return nil, err
			}
			items, ok := child.([]any)
			if !ok {
				return nil, fmt.Errorf("!include_dir_merge_list: %q is not a list", e.Name())
			}
			out = append(out, items...)
		}
		return out, nil
	}
	out := []any{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		child, err := loadYAMLFile(baseDir, filepath.Join(rel, e.Name()), secrets, visited)
		if err != nil {
			return nil, err
		}
		out = append(out, child)
	}
	return out, nil
}

func resolveIncludeDirNamed(baseDir, rel string, merge bool, secrets map[string]string, visited map[string]bool) (any, error) {
	dir := filepath.Join(baseDir, rel)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	out := map[string]any{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		stem := strings.TrimSuffix(e.Name(), ".yaml")
		child, err := loadYAMLFile(baseDir, filepath.Join(rel, e.Name()), secrets, visited)
		if err != nil {
			return nil, err
		}
		if merge {
			m, ok := child.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("!include_dir_merge_named: %q is not a mapping", e.Name())
			}
			for k, v := range m {
				if _, dup := out[k]; dup {
					return nil, fmt.Errorf("!include_dir_merge_named: duplicate key %q across files", k)
				}
				out[k] = v
			}
		} else {
			out[stem] = child
		}
	}
	return out, nil
}

// blueprintInputSentinel is what !input resolves to. Per-area handlers
// detect it and emit FIXME(blueprint-input) diagnostics.
type blueprintInputSentinel struct{ Name string }

var ErrRecursiveInclude = errors.New("recursive include")
```

- [ ] **Step 9: Run loader tests**

```bash
cd gohome
go test ./internal/importer/ -run TestYAMLLoader
```

Expected: PASS for all five sub-cases (`recursive_include` PASSes the negative case).

- [ ] **Step 10: Commit**

```bash
git add internal/importer/types.go internal/importer/scanner.go internal/importer/yaml_loader.go \
        internal/importer/scanner_test.go internal/importer/yaml_loader_test.go \
        internal/importer/testdata/yaml_loader/
git commit -m "feat(c11): scanner + YAML loader with HA custom-tag resolution"
```

---

## Task 3: diagnostics collector + report renderer

**Files:**
- Create: `internal/importer/diagnostics/diagnostics.go`, `report.go`
- Create: `internal/importer/diagnostics/diagnostics_test.go`, `report_test.go`
- Create: `internal/importer/testdata/diagnostics/{empty,full}/in.json`, `out.md`

- [ ] **Step 1: Define types in `diagnostics.go`**

```go
// Package diagnostics is the central collector for FIXME / NOTE
// items emitted across the importer pipeline. Both inline comments
// and the IMPORT_REPORT.md draw from the same Collector.
package diagnostics

import (
	"sync"
)

type Severity int

const (
	SeverityFIXME Severity = iota + 1 // action required
	SeverityNOTE                       // informational
)

func (s Severity) String() string {
	switch s {
	case SeverityFIXME:
		return "FIXME"
	case SeverityNOTE:
		return "NOTE"
	}
	return "UNKNOWN"
}

// Reason is a closed enum keyed in inline comments and report tables.
// Adding a reason is a code change.
type Reason string

const (
	ReasonJinjaImport               Reason = "jinja-import"
	ReasonUnmappedIntegration       Reason = "unmapped-integration"
	ReasonUnpublishedDriver         Reason = "unpublished-driver"
	ReasonMapperInputInvalid        Reason = "mapper-input-invalid"
	ReasonUnmappedTrigger           Reason = "unmapped-trigger"
	ReasonUnmappedAction            Reason = "unmapped-action"
	ReasonBlueprintInput            Reason = "blueprint-input"
	ReasonMapperDefault             Reason = "mapper-default"
	ReasonSecretNotCopied           Reason = "secret-not-copied"
	ReasonPasswordNotMigrated       Reason = "password-not-migrated"
	ReasonSecretCollision           Reason = "secret-collision"
	ReasonAreaHierarchyHeuristic    Reason = "area-hierarchy-heuristic"
	ReasonPersonTrackerNotMigrated  Reason = "person-tracker-not-migrated"
	ReasonExtraUnknownField         Reason = "extra-unknown-field"
	ReasonLovelaceNotImported       Reason = "lovelace-not-imported"
)

type Diagnostic struct {
	Severity Severity
	Reason   Reason
	File     string
	Line     int
	Message  string
	Detail   string
}

// Collector is the central sink. Safe for concurrent use.
type Collector struct {
	mu    sync.Mutex
	items []Diagnostic
}

func New() *Collector { return &Collector{} }

func (c *Collector) Add(d Diagnostic) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = append(c.items, d)
}

// Convenience helpers.
func (c *Collector) Fixme(reason Reason, file string, line int, msg string, detail ...string) {
	d := Diagnostic{Severity: SeverityFIXME, Reason: reason, File: file, Line: line, Message: msg}
	if len(detail) > 0 {
		d.Detail = detail[0]
	}
	c.Add(d)
}

func (c *Collector) Note(reason Reason, file string, line int, msg string, detail ...string) {
	d := Diagnostic{Severity: SeverityNOTE, Reason: reason, File: file, Line: line, Message: msg}
	if len(detail) > 0 {
		d.Detail = detail[0]
	}
	c.Add(d)
}

func (c *Collector) All() []Diagnostic {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Diagnostic, len(c.items))
	copy(out, c.items)
	return out
}

func (c *Collector) CountBy(sev Severity) int {
	n := 0
	for _, d := range c.All() {
		if d.Severity == sev {
			n++
		}
	}
	return n
}
```

- [ ] **Step 2: Define the Summary input shape and report renderer**

`internal/importer/diagnostics/report.go`:

```go
package diagnostics

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ReportSummary is the data the report renderer needs from the rest
// of the pipeline. Each per-area handler contributes its counters.
type ReportSummary struct {
	SourceHADir   string
	HAVersion     string
	GeneratedAt   time.Time
	Areas         AreaCounts
	Zones         int
	DriverInstances DriverCounts
	EntityOverrides int
	ComputedEntities int
	Automations   ItemCounts
	Scripts       ItemCounts
	Scenes        int
	Users         int
	Persons       int
	Secrets       int
	Integrations  []IntegrationDetail
}

type AreaCounts   struct{ Total, Mapped, Unmapped int }
type DriverCounts struct{ Instances, Integrations, FullyMapped, Unpublished, Unmapped int }
type ItemCounts   struct{ Total, FullyTranspiled, WithFixme int }

type IntegrationDetail struct {
	Name           string
	Source         string // "configuration.yaml" / ".storage/core.config_entries" / "both"
	DriverModule   string // e.g., "@drivers/hue.pkl"
	Status         string // "published" / "NOT YET PUBLISHED" / "out of v1.0 set"
	InstanceCount  int
	FixmeCount     int
	NoteCount      int
}

// Render produces the IMPORT_REPORT.md content from summary + collector.
func Render(s ReportSummary, c *Collector) string {
	var sb strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&sb, format, args...) }

	w("# gohome import report\n\n")
	w("Imported from `%s` on %s.\n", s.SourceHADir, s.GeneratedAt.Format("2006-01-02"))
	if s.HAVersion != "" {
		w("HA version detected: `%s`.\n\n", s.HAVersion)
	} else {
		w("HA version detected: `unknown`.\n\n")
	}

	w("## Summary\n")
	w("- Areas: %d (%d mapped, %d unmapped)\n", s.Areas.Total, s.Areas.Mapped, s.Areas.Unmapped)
	w("- Zones: %d\n", s.Zones)
	w("- Driver instances: %d across %d integrations (%d fully mapped, %d unpublished-driver placeholders, %d unmapped)\n",
		s.DriverInstances.Instances, s.DriverInstances.Integrations,
		s.DriverInstances.FullyMapped, s.DriverInstances.Unpublished, s.DriverInstances.Unmapped)
	w("- Entity overrides: %d\n", s.EntityOverrides)
	w("- Computed entities (template platform): %d\n", s.ComputedEntities)
	w("- Automations: %d (%d fully transpiled, %d with FIXMEs)\n",
		s.Automations.Total, s.Automations.FullyTranspiled, s.Automations.WithFixme)
	w("- Scripts: %d (%d fully transpiled, %d with FIXMEs)\n",
		s.Scripts.Total, s.Scripts.FullyTranspiled, s.Scripts.WithFixme)
	w("- Scenes: %d\n", s.Scenes)
	w("- Users: %d (passwords NOT migrated — passkey re-registration required)\n", s.Users)
	w("- Persons: %d\n", s.Persons)
	w("- Secrets: %d (values written to IMPORTED_SECRETS.env)\n\n", s.Secrets)

	w("## What to do next\n")
	w("1. Source secrets and delete the side file:\n")
	w("       set -a && source ./IMPORTED_SECRETS.env && set +a\n")
	w("       rm ./IMPORTED_SECRETS.env\n")
	w("2. Install required drivers (one per published-driver integration above).\n")
	w("3. Resolve open FIXMEs (search for `FIXME(`).\n")
	w("4. Run `gohome config validate`.\n")
	w("5. Re-register passkeys for each user via `gohome auth bootstrap <slug>`.\n\n")

	if len(s.Integrations) > 0 {
		w("## Per-integration detail\n")
		for _, in := range s.Integrations {
			w("### %s (`%s`)\n", in.Name, in.Source)
			w("- Driver: `%s` (status: %s)\n", in.DriverModule, in.Status)
			w("- %d instance(s)\n", in.InstanceCount)
			w("- %d FIXMEs / %d NOTEs\n\n", in.FixmeCount, in.NoteCount)
		}
	}

	all := c.All()
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].Severity != all[j].Severity {
			return all[i].Severity < all[j].Severity
		}
		if all[i].File != all[j].File {
			return all[i].File < all[j].File
		}
		return all[i].Line < all[j].Line
	})

	fixmes := filterBySeverity(all, SeverityFIXME)
	notes := filterBySeverity(all, SeverityNOTE)

	w("## Open FIXMEs (%d)\n", len(fixmes))
	if len(fixmes) > 0 {
		w("| Reason | File:line | Message |\n|---|---|---|\n")
		for _, d := range fixmes {
			w("| %s | %s | %s |\n", d.Reason, locStr(d), d.Message)
		}
	} else {
		w("_None._\n")
	}
	w("\n")

	w("## Notes (%d)\n", len(notes))
	if len(notes) > 0 {
		w("| Reason | File:line | Message |\n|---|---|---|\n")
		for _, d := range notes {
			w("| %s | %s | %s |\n", d.Reason, locStr(d), d.Message)
		}
	} else {
		w("_None._\n")
	}

	return sb.String()
}

func filterBySeverity(items []Diagnostic, sev Severity) []Diagnostic {
	out := make([]Diagnostic, 0, len(items))
	for _, d := range items {
		if d.Severity == sev {
			out = append(out, d)
		}
	}
	return out
}

func locStr(d Diagnostic) string {
	if d.Line > 0 {
		return fmt.Sprintf("%s:%d", d.File, d.Line)
	}
	return d.File
}
```

- [ ] **Step 3: Write the failing report renderer test**

`internal/importer/diagnostics/report_test.go`:

```go
package diagnostics_test

import (
	"strings"
	"testing"
	"time"

	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
)

func TestRender_EmptyReport(t *testing.T) {
	c := diagnostics.New()
	s := diagnostics.ReportSummary{
		SourceHADir: "/tmp/ha",
		GeneratedAt: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC),
	}
	out := diagnostics.Render(s, c)
	if !strings.Contains(out, "Imported from `/tmp/ha`") {
		t.Errorf("missing intro: %s", out)
	}
	if !strings.Contains(out, "## Open FIXMEs (0)") {
		t.Errorf("missing FIXMEs section: %s", out)
	}
	if !strings.Contains(out, "_None._") {
		t.Errorf("expected _None._ for empty sections: %s", out)
	}
}

func TestRender_WithFixmesAndNotes(t *testing.T) {
	c := diagnostics.New()
	c.Fixme(diagnostics.ReasonJinjaImport, "automations/lighting.pkl", 42, "closest('zone.home').name not transpilable")
	c.Note(diagnostics.ReasonSecretNotCopied, "secrets.pkl", 12, "mqtt_password value in IMPORTED_SECRETS.env")
	s := diagnostics.ReportSummary{
		SourceHADir: "/tmp/ha",
		HAVersion:   "2026.3.4",
		GeneratedAt: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC),
	}
	out := diagnostics.Render(s, c)

	for _, want := range []string{
		"## Open FIXMEs (1)",
		"jinja-import",
		"automations/lighting.pkl:42",
		"## Notes (1)",
		"secret-not-copied",
		"secrets.pkl:12",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 4: Run tests**

```bash
cd gohome
go test ./internal/importer/diagnostics/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/importer/diagnostics/
git commit -m "feat(c11): diagnostics collector + IMPORT_REPORT.md renderer"
```

---

## Task 4: writer skeleton + canonical Pkl emitters

`writer.WriteAll` materializes a `Result` to disk; `pkl_print.go` houses one focused emitter per output file type. Initially we ship two trivial emitters (`Gitignore`, `ImportedSecretsEnv`) so the writer is testable end-to-end.

**Files:**
- Create: `internal/importer/writer/writer.go`, `pkl_print.go`
- Create: `internal/importer/writer/writer_test.go`, `pkl_print_test.go`

- [ ] **Step 1: Define `Result` and writer**

`internal/importer/writer/writer.go`:

```go
// Package writer materializes a *Result to a target directory.
// File contents are produced upstream by per-area handlers via
// pkl_print.go's canonical emitters.
package writer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// File is a single file to write. Mode is the unix file mode (0600 for
// IMPORTED_SECRETS.env; 0644 for everything else).
type File struct {
	Path     string // relative to outDir
	Contents []byte
	Mode     os.FileMode
}

// Result is the in-memory tree the writer materializes. Files is the
// flat list; Writer creates intermediate dirs as needed.
type Result struct {
	Files []File
}

func (r *Result) Add(f File) { r.Files = append(r.Files, f) }

// Options governs WriteAll's behavior.
type Options struct {
	OutDir string
	Force  bool
}

// WriteAll writes every file in r to opts.OutDir. Refuses to write if
// OutDir exists and is non-empty unless Force is set. Writes atomically
// via tmp + rename.
func WriteAll(opts Options, r *Result) error {
	if opts.OutDir == "" {
		return errors.New("writer: OutDir required")
	}
	if err := ensureOutDir(opts.OutDir, opts.Force); err != nil {
		return err
	}
	sort.SliceStable(r.Files, func(i, j int) bool { return r.Files[i].Path < r.Files[j].Path })
	for _, f := range r.Files {
		if err := writeOne(opts.OutDir, f); err != nil {
			return fmt.Errorf("writer: %s: %w", f.Path, err)
		}
	}
	return nil
}

func ensureOutDir(dir string, force bool) error {
	info, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return os.MkdirAll(dir, 0o755)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("writer: %q exists and is not a directory", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) > 0 && !force {
		return fmt.Errorf("writer: %q is non-empty (use --force to overwrite)", dir)
	}
	if force {
		// Wipe everything in dir.
		for _, e := range entries {
			if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeOne(outDir string, f File) error {
	full := filepath.Join(outDir, f.Path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	mode := f.Mode
	if mode == 0 {
		mode = 0o644
	}
	tmp := full + ".tmp"
	if err := os.WriteFile(tmp, f.Contents, mode); err != nil {
		return err
	}
	return os.Rename(tmp, full)
}
```

- [ ] **Step 2: Implement initial emitters**

`internal/importer/writer/pkl_print.go`:

```go
package writer

import (
	"fmt"
	"sort"
	"strings"
)

// EmitGitignore returns the contents of the .gitignore the importer
// always writes — at minimum it excludes IMPORTED_SECRETS.env so users
// don't accidentally commit secrets on their first commit.
func EmitGitignore() []byte {
	return []byte("# generated by gohome import-ha\nIMPORTED_SECRETS.env\n")
}

// EmitImportedSecretsEnv produces the side-file containing the actual
// secret values. The file's leading comments tell the user how to use
// it and to delete it afterward.
func EmitImportedSecretsEnv(secrets map[string]string) []byte {
	var sb strings.Builder
	sb.WriteString("# IMPORTED_SECRETS.env — generated by gohome import-ha\n")
	sb.WriteString("#\n")
	sb.WriteString("# These are the actual secret values pulled from your HA secrets.yaml.\n")
	sb.WriteString("# They live OUTSIDE your committed Pkl config. To use them with gohomed:\n")
	sb.WriteString("#\n")
	sb.WriteString("#   1. Export them into your shell or systemd unit:\n")
	sb.WriteString("#        set -a && source ./IMPORTED_SECRETS.env && set +a\n")
	sb.WriteString("#      (or convert to a systemd EnvironmentFile= directive)\n")
	sb.WriteString("#   2. Once your gohomed is running and reading them via secrets.pkl,\n")
	sb.WriteString("#      DELETE THIS FILE so the values stop existing on disk:\n")
	sb.WriteString("#        rm ./IMPORTED_SECRETS.env\n\n")

	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		sb.WriteString(fmt.Sprintf("export %s=%q\n", envName(k), secrets[k]))
	}
	return []byte(sb.String())
}

// envName upper-snake-cases an HA secret key for use as an env var.
func envName(k string) string {
	out := strings.ToUpper(k)
	out = strings.ReplaceAll(out, "-", "_")
	return out
}
```

- [ ] **Step 3: Write the failing tests**

`internal/importer/writer/writer_test.go`:

```go
package writer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fynn-labs/gohome/internal/importer/writer"
)

func TestWriteAll_RefusesNonEmptyOutDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "existing"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &writer.Result{Files: []writer.File{{Path: "a.pkl", Contents: []byte("// a\n")}}}
	err := writer.WriteAll(writer.Options{OutDir: dir, Force: false}, r)
	if err == nil {
		t.Fatal("expected error for non-empty OutDir")
	}
}

func TestWriteAll_ForceOverwritesAndWritesFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stale"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &writer.Result{Files: []writer.File{{Path: "a.pkl", Contents: []byte("// a\n")}}}
	if err := writer.WriteAll(writer.Options{OutDir: dir, Force: true}, r); err != nil {
		t.Fatalf("WriteAll: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "stale")); err == nil {
		t.Error("stale file should have been removed")
	}
	got, err := os.ReadFile(filepath.Join(dir, "a.pkl"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "// a\n" {
		t.Errorf("got %q", got)
	}
}

func TestWriteAll_CreatesIntermediateDirs(t *testing.T) {
	dir := t.TempDir()
	r := &writer.Result{Files: []writer.File{
		{Path: "automations/handlers/morning.star", Contents: []byte("# m\n")},
	}}
	if err := writer.WriteAll(writer.Options{OutDir: dir, Force: false}, r); err != nil {
		t.Fatalf("WriteAll: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "automations/handlers/morning.star")); err != nil {
		t.Errorf("expected nested file: %v", err)
	}
}

func TestWriteAll_HonorsFileMode(t *testing.T) {
	dir := t.TempDir()
	r := &writer.Result{Files: []writer.File{
		{Path: "IMPORTED_SECRETS.env", Contents: []byte("export X=1\n"), Mode: 0o600},
	}}
	if err := writer.WriteAll(writer.Options{OutDir: dir}, r); err != nil {
		t.Fatalf("WriteAll: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "IMPORTED_SECRETS.env"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
}
```

`internal/importer/writer/pkl_print_test.go`:

```go
package writer_test

import (
	"strings"
	"testing"

	"github.com/fynn-labs/gohome/internal/importer/writer"
)

func TestEmitGitignore(t *testing.T) {
	got := string(writer.EmitGitignore())
	if !strings.Contains(got, "IMPORTED_SECRETS.env") {
		t.Errorf("gitignore should ignore IMPORTED_SECRETS.env: %q", got)
	}
}

func TestEmitImportedSecretsEnv_OrdersAndUpperSnakeCases(t *testing.T) {
	secrets := map[string]string{"hue_api_key": "abc", "mqtt_password": "def"}
	got := string(writer.EmitImportedSecretsEnv(secrets))
	if !strings.Contains(got, `export HUE_API_KEY="abc"`) {
		t.Errorf("missing HUE_API_KEY: %s", got)
	}
	if !strings.Contains(got, `export MQTT_PASSWORD="def"`) {
		t.Errorf("missing MQTT_PASSWORD: %s", got)
	}
	// Ordering: hue should come before mqtt alphabetically.
	hueIdx := strings.Index(got, "HUE_API_KEY")
	mqttIdx := strings.Index(got, "MQTT_PASSWORD")
	if hueIdx > mqttIdx {
		t.Errorf("expected alphabetical ordering: hue=%d mqtt=%d", hueIdx, mqttIdx)
	}
}
```

- [ ] **Step 4: Run tests**

```bash
cd gohome
go test ./internal/importer/writer/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/importer/writer/
git commit -m "feat(c11): writer skeleton + initial Pkl/env emitters"
```

---

## Task 5: secrets pipeline

Reads `secrets.yaml`, emits `secrets.pkl` (env-var references) plus the side `IMPORTED_SECRETS.env` already supported by Task 4, plus a NOTE per secret.

**Files:**
- Create: `internal/importer/secrets/secrets.go`, `secrets_test.go`
- Modify: `internal/importer/writer/pkl_print.go` (add `EmitSecretsPkl`)

- [ ] **Step 1: Add `EmitSecretsPkl` to writer**

Append to `internal/importer/writer/pkl_print.go`:

```go
// EmitSecretsPkl produces secrets.pkl with one read("env:NAME") per
// secret key. Keys are emitted alphabetically for deterministic output.
func EmitSecretsPkl(secretKeys []string) []byte {
	var sb strings.Builder
	sb.WriteString(`amends "@gohome/config.pkl"
import "@gohome/base.pkl" as b

secrets {
`)
	keys := append([]string(nil), secretKeys...)
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&sb, "  %s = read(\"env:%s\")\n", k, envName(k))
	}
	sb.WriteString("}\n")
	return []byte(sb.String())
}
```

- [ ] **Step 2: Implement `secrets.go`**

`internal/importer/secrets/secrets.go`:

```go
// Package secrets translates HA's secrets.yaml into:
//   - secrets.pkl referencing read("env:NAME") for each key
//   - IMPORTED_SECRETS.env with the actual values
//   - a .gitignore line excluding IMPORTED_SECRETS.env
//   - one NOTE(secret-not-copied) per secret
package secrets

import (
	"strings"

	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
	"github.com/fynn-labs/gohome/internal/importer/writer"
)

// Process consumes the secrets map (key → value) and produces the
// three files plus diagnostics. Returns the file list ready for the
// writer.Result.
func Process(haSecrets map[string]string, c *diagnostics.Collector) []writer.File {
	keys := make([]string, 0, len(haSecrets))
	for k := range haSecrets {
		keys = append(keys, k)
	}

	for _, k := range keys {
		c.Note(diagnostics.ReasonSecretNotCopied,
			"secrets.pkl", 0,
			"value of '"+k+"' written to IMPORTED_SECRETS.env, not copied here")
	}

	// Detect collisions (rare but possible after upper-snake-case folding).
	seen := map[string]string{}
	for _, k := range keys {
		envName := strings.ToUpper(strings.ReplaceAll(k, "-", "_"))
		if prev, ok := seen[envName]; ok {
			c.Note(diagnostics.ReasonSecretCollision,
				"secrets.pkl", 0,
				"secrets '"+k+"' and '"+prev+"' both map to env var "+envName)
		}
		seen[envName] = k
	}

	return []writer.File{
		{Path: "secrets.pkl", Contents: writer.EmitSecretsPkl(keys), Mode: 0o644},
		{Path: "IMPORTED_SECRETS.env", Contents: writer.EmitImportedSecretsEnv(haSecrets), Mode: 0o600},
		{Path: ".gitignore", Contents: writer.EmitGitignore(), Mode: 0o644},
	}
}
```

- [ ] **Step 3: Tests**

`internal/importer/secrets/secrets_test.go`:

```go
package secrets_test

import (
	"strings"
	"testing"

	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
	"github.com/fynn-labs/gohome/internal/importer/secrets"
)

func TestProcess_ProducesThreeFiles(t *testing.T) {
	c := diagnostics.New()
	files := secrets.Process(map[string]string{"hue_api_key": "abc"}, c)
	paths := map[string]bool{}
	for _, f := range files {
		paths[f.Path] = true
	}
	for _, want := range []string{"secrets.pkl", "IMPORTED_SECRETS.env", ".gitignore"} {
		if !paths[want] {
			t.Errorf("missing file %q", want)
		}
	}
}

func TestProcess_EmitsNotePerSecret(t *testing.T) {
	c := diagnostics.New()
	secrets.Process(map[string]string{"a": "1", "b": "2"}, c)
	notes := c.CountBy(diagnostics.SeverityNOTE)
	if notes != 2 {
		t.Errorf("note count = %d, want 2", notes)
	}
}

func TestProcess_DetectsCollision(t *testing.T) {
	c := diagnostics.New()
	// "FOO_BAR" and "foo-bar" both upper-snake-case to FOO_BAR.
	secrets.Process(map[string]string{"FOO_BAR": "1", "foo-bar": "2"}, c)
	collisions := 0
	for _, d := range c.All() {
		if d.Reason == diagnostics.ReasonSecretCollision {
			collisions++
		}
	}
	if collisions == 0 {
		t.Error("expected at least one collision NOTE")
	}
}

func TestProcess_PklReferencesEnv(t *testing.T) {
	c := diagnostics.New()
	files := secrets.Process(map[string]string{"hue_api_key": "abc"}, c)
	var pkl string
	for _, f := range files {
		if f.Path == "secrets.pkl" {
			pkl = string(f.Contents)
		}
	}
	if !strings.Contains(pkl, `hue_api_key = read("env:HUE_API_KEY")`) {
		t.Errorf("secrets.pkl missing reference: %s", pkl)
	}
}
```

- [ ] **Step 4: Run tests + commit**

```bash
cd gohome
go test ./internal/importer/secrets/...
git add internal/importer/secrets/ internal/importer/writer/pkl_print.go
git commit -m "feat(c11): secrets pipeline (secrets.pkl + IMPORTED_SECRETS.env + .gitignore)"
```

---

## Task 6: registry handlers (areas, zones, entities, devices)

**Files:**
- Create: `internal/importer/registry/areas.go`, `zones.go`, `entities.go`, `devices.go`
- Create: `internal/importer/registry/registry_test.go`
- Modify: `internal/importer/writer/pkl_print.go` (add `EmitAreasPkl`, `EmitZonesPkl`, `EmitEntityOverridesPkl`)

- [ ] **Step 1: Add Pkl emitters**

Append to `internal/importer/writer/pkl_print.go`:

```go
// EmitAreasPkl produces areas.pkl. Areas are emitted alphabetically
// by ID for deterministic output.
func EmitAreasPkl(areas []AreaOut) []byte {
	var sb strings.Builder
	sb.WriteString(`amends "@gohome/config.pkl"
import "@gohome/areas.pkl" as a

areas: Listing<a.Area> = new {
`)
	sort.Slice(areas, func(i, j int) bool { return areas[i].ID < areas[j].ID })
	for _, ar := range areas {
		fmt.Fprintf(&sb, "  new { id = %q; name = %q", ar.ID, ar.Name)
		if ar.Parent != "" {
			fmt.Fprintf(&sb, "; parent = %q", ar.Parent)
		}
		sb.WriteString(" }\n")
	}
	sb.WriteString("}\n")
	return []byte(sb.String())
}

type AreaOut struct{ ID, Name, Parent string }

// EmitZonesPkl produces zones.pkl.
func EmitZonesPkl(zones []ZoneOut) []byte {
	var sb strings.Builder
	sb.WriteString(`amends "@gohome/config.pkl"
import "@gohome/zones.pkl" as z

zones: Listing<z.Zone> = new {
`)
	sort.Slice(zones, func(i, j int) bool { return zones[i].ID < zones[j].ID })
	for _, zn := range zones {
		fmt.Fprintf(&sb, "  new { id = %q; name = %q; latitude = %g; longitude = %g; radiusMeters = %g }\n",
			zn.ID, zn.Name, zn.Latitude, zn.Longitude, zn.RadiusMeters)
	}
	sb.WriteString("}\n")
	return []byte(sb.String())
}

type ZoneOut struct {
	ID, Name              string
	Latitude, Longitude   float64
	RadiusMeters          float64
}

// EmitEntityOverridesPkl produces entities/overrides.pkl.
func EmitEntityOverridesPkl(overs []EntityOverrideOut) []byte {
	var sb strings.Builder
	sb.WriteString(`amends "@gohome/config.pkl"
import "@gohome/entities.pkl" as e

overrides: Listing<e.Override> = new {
`)
	sort.Slice(overs, func(i, j int) bool { return overs[i].EntityID < overs[j].EntityID })
	for _, o := range overs {
		fmt.Fprintf(&sb, "  new { entityId = %q", o.EntityID)
		if o.Name != "" {
			fmt.Fprintf(&sb, "; name = %q", o.Name)
		}
		if o.AreaID != "" {
			fmt.Fprintf(&sb, "; areaId = %q", o.AreaID)
		}
		if o.Icon != "" {
			fmt.Fprintf(&sb, "; icon = %q", o.Icon)
		}
		if o.Hidden {
			sb.WriteString("; hidden = true")
		}
		if o.Disabled {
			sb.WriteString("; disabled = true")
		}
		sb.WriteString(" }\n")
	}
	sb.WriteString("}\n")
	return []byte(sb.String())
}

type EntityOverrideOut struct {
	EntityID, Name, AreaID, Icon string
	Hidden, Disabled             bool
}
```

- [ ] **Step 2: Implement registry handlers**

`internal/importer/registry/areas.go`:

```go
package registry

import (
	"github.com/fynn-labs/gohome/internal/importer"
	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
	"github.com/fynn-labs/gohome/internal/importer/writer"
)

// ProcessAreas converts HA area registry entries to gohome AreaOut.
// HA areas are flat by default; we look for label-based hierarchy
// conventions and emit a NOTE if heuristic.
func ProcessAreas(areas []importer.AreaModel, c *diagnostics.Collector) []writer.AreaOut {
	out := make([]writer.AreaOut, 0, len(areas))
	for _, a := range areas {
		out = append(out, writer.AreaOut{ID: a.ID, Name: a.Name})
	}
	return out
}
```

`internal/importer/registry/zones.go`:

```go
package registry

import (
	"github.com/fynn-labs/gohome/internal/importer"
	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
	"github.com/fynn-labs/gohome/internal/importer/writer"
)

// ProcessZones converts HA zone definitions (from configuration.yaml's
// zone: block) to gohome ZoneOut.
func ProcessZones(zones []importer.ZoneModel, _ *diagnostics.Collector) []writer.ZoneOut {
	out := make([]writer.ZoneOut, 0, len(zones))
	for _, z := range zones {
		out = append(out, writer.ZoneOut{
			ID: z.ID, Name: z.Name,
			Latitude: z.Latitude, Longitude: z.Longitude,
			RadiusMeters: z.RadiusMeters,
		})
	}
	return out
}
```

`internal/importer/registry/entities.go`:

```go
package registry

import (
	"github.com/fynn-labs/gohome/internal/importer"
	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
	"github.com/fynn-labs/gohome/internal/importer/writer"
)

// ProcessEntities converts HA entity registry entries to gohome
// entity overrides. Only emits an override entry when the user has
// customized the entity (has area, custom name, icon, hidden/disabled).
func ProcessEntities(ents []importer.EntityModel, _ *diagnostics.Collector) []writer.EntityOverrideOut {
	out := make([]writer.EntityOverrideOut, 0)
	for _, e := range ents {
		if e.AreaID == "" && e.Name == "" && e.Icon == "" && !e.Hidden && !e.Disabled {
			continue
		}
		out = append(out, writer.EntityOverrideOut{
			EntityID: e.EntityID, Name: e.Name, AreaID: e.AreaID, Icon: e.Icon,
			Hidden: e.Hidden, Disabled: e.Disabled,
		})
	}
	return out
}
```

`internal/importer/registry/devices.go`:

```go
package registry

import (
	"fmt"

	"github.com/fynn-labs/gohome/internal/importer"
	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
)

// ProcessDevices doesn't write a Pkl file (gohome's devices come from
// drivers at runtime), but it does emit a NOTE counting how many
// devices the user had so they can sanity-check after driver install.
func ProcessDevices(devs []importer.DeviceModel, c *diagnostics.Collector) {
	if len(devs) == 0 {
		return
	}
	c.Note(diagnostics.ReasonExtraUnknownField,
		"devices.pkl", 0,
		fmt.Sprintf("HA had %d devices; gohome devices appear at runtime via drivers", len(devs)))
}
```

- [ ] **Step 3: Test**

`internal/importer/registry/registry_test.go`:

```go
package registry_test

import (
	"strings"
	"testing"

	"github.com/fynn-labs/gohome/internal/importer"
	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
	"github.com/fynn-labs/gohome/internal/importer/registry"
	"github.com/fynn-labs/gohome/internal/importer/writer"
)

func TestProcessAreas(t *testing.T) {
	c := diagnostics.New()
	out := registry.ProcessAreas([]importer.AreaModel{
		{ID: "kitchen", Name: "Kitchen"},
		{ID: "lr", Name: "Living Room"},
	}, c)
	if len(out) != 2 {
		t.Errorf("got %d areas, want 2", len(out))
	}
	pkl := string(writer.EmitAreasPkl(out))
	if !strings.Contains(pkl, `id = "kitchen"`) || !strings.Contains(pkl, `id = "lr"`) {
		t.Errorf("areas.pkl missing entries: %s", pkl)
	}
}

func TestProcessEntities_OnlyEmitsCustomized(t *testing.T) {
	c := diagnostics.New()
	out := registry.ProcessEntities([]importer.EntityModel{
		{EntityID: "light.kitchen"},                                  // not customized — skip
		{EntityID: "light.lr", AreaID: "lr", Name: "Living Room Lt"}, // customized — emit
	}, c)
	if len(out) != 1 {
		t.Errorf("got %d, want 1", len(out))
	}
	if out[0].EntityID != "light.lr" {
		t.Errorf("got %q", out[0].EntityID)
	}
}

func TestProcessDevices_NoteOnly(t *testing.T) {
	c := diagnostics.New()
	registry.ProcessDevices([]importer.DeviceModel{{ID: "abc"}}, c)
	if c.CountBy(diagnostics.SeverityNOTE) != 1 {
		t.Errorf("expected one NOTE, got %d", c.CountBy(diagnostics.SeverityNOTE))
	}
}
```

- [ ] **Step 4: Run tests + commit**

```bash
cd gohome
go test ./internal/importer/registry/...
git add internal/importer/registry/ internal/importer/writer/pkl_print.go
git commit -m "feat(c11): registry handlers (areas, zones, entity overrides, device count)"
```

---

## Task 7: core handler (configuration.yaml → main.pkl + settings.pkl)

**Files:**
- Create: `internal/importer/core/core.go`, `core_test.go`
- Modify: `internal/importer/writer/pkl_print.go` (add `EmitMainPkl`, `EmitSettingsPkl`)

- [ ] **Step 1: Add emitters**

Append to `internal/importer/writer/pkl_print.go`:

```go
// EmitMainPkl produces the user-facing main.pkl that imports every
// other module the importer wrote.
func EmitMainPkl(opts MainPklOpts) []byte {
	var sb strings.Builder
	sb.WriteString(`@ModuleInfo { minPklVersion = "0.27.0" }
amends "@gohome/config.pkl"

import "settings.pkl" as settings
import "drivers.pkl" as drivers
import "areas.pkl" as areas
import "zones.pkl" as zones
`)
	if opts.HasEntityOverrides {
		sb.WriteString("import \"entities/overrides.pkl\" as entityOverrides\n")
	}
	if opts.HasComputedEntities {
		sb.WriteString("import \"entities/computed.pkl\" as computed\n")
	}
	sb.WriteString("import \"scenes.pkl\" as scenes\n")
	sb.WriteString("import \"auth/users.pkl\" as users\n")
	sb.WriteString("import \"auth/roles.pkl\" as roles\n")
	sb.WriteString("import \"auth/policies.pkl\" as policies\n")
	sb.WriteString("import \"secrets.pkl\" as _secrets\n")
	return []byte(sb.String())
}

type MainPklOpts struct {
	HasEntityOverrides  bool
	HasComputedEntities bool
}

// EmitSettingsPkl produces settings.pkl from the configuration.yaml top-level.
func EmitSettingsPkl(s SettingsOut) []byte {
	var sb strings.Builder
	sb.WriteString(`amends "@gohome/config.pkl"
import "@gohome/base.pkl" as b

settings {
`)
	if s.HomeName != "" {
		fmt.Fprintf(&sb, "  homeName = %q\n", s.HomeName)
	}
	if s.Latitude != 0 || s.Longitude != 0 {
		fmt.Fprintf(&sb, "  latitude = %g\n  longitude = %g\n", s.Latitude, s.Longitude)
	}
	if s.Elevation != 0 {
		fmt.Fprintf(&sb, "  elevation = %d\n", s.Elevation)
	}
	if s.UnitSystem != "" {
		fmt.Fprintf(&sb, "  unitSystem = %q\n", s.UnitSystem)
	}
	if s.TimeZone != "" {
		fmt.Fprintf(&sb, "  timeZone = %q\n", s.TimeZone)
	}
	if s.Currency != "" {
		fmt.Fprintf(&sb, "  currency = %q\n", s.Currency)
	}
	sb.WriteString("}\n")
	return []byte(sb.String())
}

type SettingsOut struct {
	HomeName, UnitSystem, TimeZone, Currency string
	Latitude, Longitude                      float64
	Elevation                                int
}
```

- [ ] **Step 2: Implement core handler**

`internal/importer/core/core.go`:

```go
// Package core translates HA's top-level configuration.yaml settings
// into gohome's main.pkl + settings.pkl.
package core

import (
	"github.com/fynn-labs/gohome/internal/importer"
	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
	"github.com/fynn-labs/gohome/internal/importer/writer"
)

// Process consumes the configuration.yaml top-level mapping (under
// homeassistant: in the YAML) and emits settings + the main.pkl shell.
// hasEntityOverrides / hasComputedEntities are flags telling main.pkl
// which optional imports to include.
func Process(cfg *importer.ConfigurationModel, hasOverrides, hasComputed bool, c *diagnostics.Collector) []writer.File {
	var s writer.SettingsOut
	if cfg != nil && cfg.Raw != nil {
		ha, _ := cfg.Raw["homeassistant"].(map[string]any)
		if ha != nil {
			s.HomeName, _ = ha["name"].(string)
			s.UnitSystem, _ = ha["unit_system"].(string)
			s.TimeZone, _ = ha["time_zone"].(string)
			s.Currency, _ = ha["currency"].(string)
			if v, ok := ha["latitude"].(float64); ok {
				s.Latitude = v
			}
			if v, ok := ha["longitude"].(float64); ok {
				s.Longitude = v
			}
			if v, ok := ha["elevation"].(int); ok {
				s.Elevation = v
			} else if v, ok := ha["elevation"].(float64); ok {
				s.Elevation = int(v)
			}
		}
	}
	return []writer.File{
		{Path: "settings.pkl", Contents: writer.EmitSettingsPkl(s)},
		{Path: "main.pkl", Contents: writer.EmitMainPkl(writer.MainPklOpts{
			HasEntityOverrides:  hasOverrides,
			HasComputedEntities: hasComputed,
		})},
	}
}
```

- [ ] **Step 3: Test**

`internal/importer/core/core_test.go`:

```go
package core_test

import (
	"strings"
	"testing"

	"github.com/fynn-labs/gohome/internal/importer"
	"github.com/fynn-labs/gohome/internal/importer/core"
	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
)

func TestProcess_TranslatesHomeAssistantBlock(t *testing.T) {
	c := diagnostics.New()
	cfg := &importer.ConfigurationModel{Raw: map[string]any{
		"homeassistant": map[string]any{
			"name":      "Home",
			"latitude":  42.36,
			"longitude": -71.06,
			"elevation": 100.0,
			"time_zone": "America/New_York",
		},
	}}
	files := core.Process(cfg, false, false, c)
	var settings string
	for _, f := range files {
		if f.Path == "settings.pkl" {
			settings = string(f.Contents)
		}
	}
	for _, want := range []string{`homeName = "Home"`, `latitude = 42.36`, `timeZone = "America/New_York"`} {
		if !strings.Contains(settings, want) {
			t.Errorf("settings.pkl missing %q: %s", want, settings)
		}
	}
}

func TestProcess_ConditionalMainImports(t *testing.T) {
	c := diagnostics.New()
	files := core.Process(&importer.ConfigurationModel{Raw: map[string]any{}}, true, true, c)
	var main string
	for _, f := range files {
		if f.Path == "main.pkl" {
			main = string(f.Contents)
		}
	}
	if !strings.Contains(main, "entities/overrides.pkl") {
		t.Error("main.pkl should import overrides when hasOverrides=true")
	}
	if !strings.Contains(main, "entities/computed.pkl") {
		t.Error("main.pkl should import computed when hasComputed=true")
	}
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/importer/core/ internal/importer/writer/pkl_print.go
git commit -m "feat(c11): core handler (configuration.yaml → main.pkl + settings.pkl)"
```

---

## Task 8: auth handler (users + persons + role/policy stubs)

**Files:**
- Create: `internal/importer/auth/auth.go`, `auth_test.go`
- Modify: `internal/importer/writer/pkl_print.go` (add `EmitUsersPkl`, `EmitRolesPkl`, `EmitPoliciesPkl`)

- [ ] **Step 1: Add emitters**

Append to `internal/importer/writer/pkl_print.go`:

```go
// EmitUsersPkl produces auth/users.pkl. Passwords are NOT emitted —
// HA uses bcrypt + HA-specific salting; gohome uses Argon2id (per C9).
// Migrating across hash schemes is unsafe. The user must re-register
// passkeys via `gohome auth bootstrap <slug>`.
func EmitUsersPkl(users []UserOut) []byte {
	var sb strings.Builder
	sb.WriteString(`amends "@gohome/config.pkl"
import "@gohome/auth.pkl" as a

users: Listing<a.User> = new {
`)
	sort.Slice(users, func(i, j int) bool { return users[i].Slug < users[j].Slug })
	for _, u := range users {
		fmt.Fprintf(&sb, "  new { slug = %q; displayName = %q; roles = new {", u.Slug, u.DisplayName)
		for _, r := range u.Roles {
			fmt.Fprintf(&sb, " %q", r)
		}
		fmt.Fprintf(&sb, " }; active = %t }\n", u.Active)
	}
	sb.WriteString("}\n")
	return []byte(sb.String())
}

type UserOut struct {
	Slug, DisplayName string
	Roles             []string
	Active            bool
}

// EmitRolesPkl produces auth/roles.pkl with only the gohome built-ins.
// The user can add custom roles post-import.
func EmitRolesPkl() []byte {
	return []byte(`amends "@gohome/config.pkl"
import "@gohome/auth.pkl" as a

// gohome built-in roles. Add custom roles below as needed.
roles: Listing<a.Role> = new {
  new { slug = "admin"; permissions = new { "*" } }
  new { slug = "member"; permissions = new { "read"; "write_own" } }
  new { slug = "guest"; permissions = new { "read" } }
}
`)
}

// EmitPoliciesPkl produces auth/policies.pkl with a permissive default
// matching HA's "everyone can do everything in their scope" baseline.
// Users tighten this post-import.
func EmitPoliciesPkl() []byte {
	return []byte(`amends "@gohome/config.pkl"
import "@gohome/auth.pkl" as a

// Permissive default policies imported from HA.
// Refine these per your household's needs (see docs/import-ha.md).
policies: Listing<a.Policy> = new {
  new { role = "admin"; resource = "*"; actions = new { "*" } }
  new { role = "member"; resource = "*"; actions = new { "read"; "write_own" } }
  new { role = "guest"; resource = "*"; actions = new { "read" } }
}
`)
}
```

- [ ] **Step 2: Implement auth handler**

`internal/importer/auth/auth.go`:

```go
// Package auth translates HA's user / person registries into
// auth/users.pkl + auth/roles.pkl + auth/policies.pkl. Passwords
// are not migrated; users re-register passkeys post-import.
package auth

import (
	"github.com/fynn-labs/gohome/internal/importer"
	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
	"github.com/fynn-labs/gohome/internal/importer/writer"
)

// Process consumes HA users + persons and emits the three auth files.
func Process(users []importer.UserModel, persons []importer.PersonModel, c *diagnostics.Collector) []writer.File {
	out := make([]writer.UserOut, 0, len(users))
	personByUserID := map[string]string{}
	for _, p := range persons {
		if p.UserID != "" && p.Name != "" {
			personByUserID[p.UserID] = p.Name
		}
		if len(p.DeviceTrackers) > 0 {
			c.Note(diagnostics.ReasonPersonTrackerNotMigrated,
				"auth/users.pkl", 0,
				"person '"+p.Name+"' had device trackers — wire presence via tracker drivers post-install")
		}
	}
	for _, u := range users {
		role := "member"
		if u.IsOwner {
			role = "admin"
		}
		display := u.Name
		if display == "" {
			display = u.Username
		}
		if name, ok := personByUserID[u.ID]; ok && name != "" {
			display = name
		}
		out = append(out, writer.UserOut{
			Slug:        slugify(u.Username),
			DisplayName: display,
			Roles:       []string{role},
			Active:      u.IsActive,
		})
		if u.HasPassword {
			c.Note(diagnostics.ReasonPasswordNotMigrated,
				"auth/users.pkl", 0,
				"user '"+u.Username+"' had a password set in HA; passkey re-registration required")
		}
	}
	return []writer.File{
		{Path: "auth/users.pkl", Contents: writer.EmitUsersPkl(out)},
		{Path: "auth/roles.pkl", Contents: writer.EmitRolesPkl()},
		{Path: "auth/policies.pkl", Contents: writer.EmitPoliciesPkl()},
	}
}

func slugify(s string) string {
	out := []byte{}
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9':
			out = append(out, ch)
		case ch >= 'A' && ch <= 'Z':
			out = append(out, ch-'A'+'a')
		case ch == '-' || ch == '_':
			out = append(out, '_')
		case ch == ' ':
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "user"
	}
	return string(out)
}
```

- [ ] **Step 3: Test**

`internal/importer/auth/auth_test.go`:

```go
package auth_test

import (
	"strings"
	"testing"

	"github.com/fynn-labs/gohome/internal/importer"
	"github.com/fynn-labs/gohome/internal/importer/auth"
	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
)

func TestProcess_OwnerBecomesAdmin(t *testing.T) {
	c := diagnostics.New()
	files := auth.Process([]importer.UserModel{
		{ID: "u1", Username: "alice", Name: "Alice", IsActive: true, IsOwner: true, HasPassword: true},
	}, nil, c)
	var users string
	for _, f := range files {
		if f.Path == "auth/users.pkl" {
			users = string(f.Contents)
		}
	}
	if !strings.Contains(users, `slug = "alice"`) {
		t.Errorf("missing alice: %s", users)
	}
	if !strings.Contains(users, `"admin"`) {
		t.Errorf("alice should be admin: %s", users)
	}
}

func TestProcess_EmitsPasswordNote(t *testing.T) {
	c := diagnostics.New()
	auth.Process([]importer.UserModel{{ID: "u1", Username: "alice", HasPassword: true, IsActive: true}}, nil, c)
	found := false
	for _, d := range c.All() {
		if d.Reason == diagnostics.ReasonPasswordNotMigrated {
			found = true
		}
	}
	if !found {
		t.Error("expected password-not-migrated NOTE")
	}
}

func TestProcess_PersonTrackerNote(t *testing.T) {
	c := diagnostics.New()
	auth.Process(
		[]importer.UserModel{{ID: "u1", Username: "alice", IsActive: true}},
		[]importer.PersonModel{{ID: "p1", Name: "Alice", UserID: "u1", DeviceTrackers: []string{"device_tracker.alice_phone"}}},
		c)
	found := false
	for _, d := range c.All() {
		if d.Reason == diagnostics.ReasonPersonTrackerNotMigrated {
			found = true
		}
	}
	if !found {
		t.Error("expected person-tracker-not-migrated NOTE")
	}
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/importer/auth/ internal/importer/writer/pkl_print.go
git commit -m "feat(c11): auth handler (users + role/policy stubs; passwords not migrated)"
```

---

## Task 9: mappers framework (interface + catalog + unmapped emitter)

Lays the rails for per-integration mappers. Ships the always-on `unmapped.go` so the importer produces *something* for every integration before any real mapper exists.

**Files:**
- Create: `internal/importer/mappers/mapper.go`, `catalog.go`, `unmapped.go`, `unmapped_test.go`
- Create: `internal/config/pkl/gohome/imported.pkl`
- Modify: `internal/importer/writer/pkl_print.go` (add `EmitDriversPkl`)

- [ ] **Step 1: New Pkl module**

`internal/config/pkl/gohome/imported.pkl`:

```pkl
module gohome.imported

// UnmappedIntegration is the placeholder type the HA importer emits
// for any HA integration that doesn't yet have a real per-integration
// mapper. The original HA configuration is preserved as a string so
// the user can hand-translate later.
class UnmappedIntegration {
  sourceName: String(!isEmpty)
  sourceConfigYaml: String
  unpublishedDriver: Boolean = false
}
```

- [ ] **Step 2: Add `EmitDriversPkl`**

Append to `internal/importer/writer/pkl_print.go`:

```go
// EmitDriversPkl produces drivers.pkl. Each entry is one driver
// instance Pkl block produced by a Mapper, plus optional pre-text
// (FIXME comments) before each entry.
func EmitDriversPkl(entries []DriversEntry) []byte {
	var sb strings.Builder
	sb.WriteString(`amends "@gohome/config.pkl"
import "@gohome/drivers.pkl" as d
import "@gohome/imported.pkl" as imported

`)
	// Append any extra imports the mappers emitted.
	importSet := map[string]bool{}
	for _, e := range entries {
		for _, imp := range e.Imports {
			if !importSet[imp] {
				importSet[imp] = true
			}
		}
	}
	imports := make([]string, 0, len(importSet))
	for imp := range importSet {
		imports = append(imports, imp)
	}
	sort.Strings(imports)
	for _, imp := range imports {
		fmt.Fprintf(&sb, "import \"%s\"\n", imp)
	}
	if len(imports) > 0 {
		sb.WriteString("\n")
	}
	sb.WriteString("driverInstances: Listing = new {\n")
	for _, e := range entries {
		if e.PreText != "" {
			for _, line := range strings.Split(strings.TrimRight(e.PreText, "\n"), "\n") {
				fmt.Fprintf(&sb, "  // %s\n", line)
			}
		}
		// Indent the entry body.
		body := strings.TrimRight(e.Body, "\n")
		for _, line := range strings.Split(body, "\n") {
			fmt.Fprintf(&sb, "  %s\n", line)
		}
		sb.WriteString("\n")
	}
	sb.WriteString("}\n")
	return []byte(sb.String())
}

type DriversEntry struct {
	PreText string   // FIXME comments / NOTEs (will be prefixed with //)
	Body    string   // the new Xxx.Instance { ... } block (raw Pkl)
	Imports []string // additional pkl import paths (e.g., "@drivers/hue.pkl")
}
```

- [ ] **Step 3: Mapper interface**

`internal/importer/mappers/mapper.go`:

```go
// Package mappers provides per-HA-integration translators. Each
// integration's mapper lives in its own file and registers itself
// with the catalog via init(). The Mapper interface lets the
// importer's pipeline iterate uniformly over all known mappers.
package mappers

import (
	"github.com/fynn-labs/gohome/internal/importer"
	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
	"github.com/fynn-labs/gohome/internal/importer/writer"
)

type IntegrationSource int

const (
	SourceYAML IntegrationSource = iota + 1
	SourceConfigEntry
	SourceBoth
)

// Input is what the importer feeds to a Mapper for one integration.
type Input struct {
	Integration   string
	Source        IntegrationSource
	YAMLBlock     map[string]any
	YAMLRaw       string                            // original YAML text (for FIXME preservation)
	ConfigEntries []importer.ConfigEntryModel
}

// Output is what the mapper returns. The DriversEntry / Imports get
// rendered into drivers.pkl by writer.EmitDriversPkl. ExtraFiles
// covers things like template platform → entities/computed.pkl.
type Output struct {
	Entry      writer.DriversEntry
	ExtraFiles []writer.File
}

type Mapper interface {
	Integration() string
	DriverModuleAvailable() bool
	Map(in Input, c *diagnostics.Collector) Output
}
```

- [ ] **Step 4: Catalog**

`internal/importer/mappers/catalog.go`:

```go
package mappers

import "sort"

var registry = map[string]Mapper{}

// Register adds a mapper to the process-wide catalog. Called from
// individual mapper file init() blocks.
func Register(m Mapper) {
	if _, dup := registry[m.Integration()]; dup {
		panic("mappers: duplicate mapper for " + m.Integration())
	}
	registry[m.Integration()] = m
}

// Lookup returns the mapper for an integration, or false if none.
func Lookup(integration string) (Mapper, bool) {
	m, ok := registry[integration]
	return m, ok
}

// All returns every registered mapper, sorted by integration name.
func All() []Mapper {
	out := make([]Mapper, 0, len(registry))
	for _, m := range registry {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Integration() < out[j].Integration() })
	return out
}
```

- [ ] **Step 5: Unmapped emitter**

`internal/importer/mappers/unmapped.go`:

```go
package mappers

import (
	"fmt"
	"strings"

	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
	"github.com/fynn-labs/gohome/internal/importer/writer"
)

// V1IntegrationDrivers names the eleven integrations from master
// design §8.1 whose drivers gohome ships in v1.0. Used by the
// unmapped emitter to distinguish an unpublished-driver case from
// an outside-v1.0 case.
var V1IntegrationDrivers = map[string]string{
	"mqtt":         "@drivers/mqtt.pkl",
	"zigbee2mqtt":  "@drivers/zigbee2mqtt.pkl",
	"esphome":      "@drivers/esphome.pkl",
	"homekit":      "@drivers/homekit.pkl",
	"matter":       "@drivers/matter.pkl",
	"hue":          "@drivers/hue.pkl",
	"nest":         "@drivers/nest.pkl",
	"zwave_js":     "@drivers/zwave_js.pkl",
	"rest":         "@drivers/rest.pkl",
	"webhook":      "@drivers/webhook.pkl",
	// "template" is always available — handled by template.go, not unmapped.
}

// EmitUnmapped produces a drivers.pkl entry for an integration that
// doesn't have a real mapper. Distinguishes between:
//   - integrations in V1IntegrationDrivers but without a registered
//     mapper (FIXME(unpublished-driver))
//   - integrations outside the v1.0 set (FIXME(unmapped-integration))
func EmitUnmapped(integration string, yamlRaw string, c *diagnostics.Collector) writer.DriversEntry {
	driverModule, isV1 := V1IntegrationDrivers[integration]

	var pre string
	if isV1 {
		pre = fmt.Sprintf(
			"FIXME(unpublished-driver): integration '%s' is in the gohome v1.0 driver set,\n"+
				"but the %s manifest was not published when this importer was built.\n"+
				"A '%s' mapper will land in a follow-on PR once the driver Pkl is available.\n"+
				"Original HA configuration preserved below — apply it manually once the driver lands:\n\n"+
				yamlIndented(yamlRaw),
			integration, driverModule, integration,
		)
		c.Fixme(diagnostics.ReasonUnpublishedDriver,
			"drivers.pkl", 0,
			"integration '"+integration+"' is in the v1.0 driver set; mapper pending driver publish")
	} else {
		pre = fmt.Sprintf(
			"FIXME(unmapped-integration): integration '%s' is outside gohome's v1.0\n"+
				"driver set. No mapper exists. Original HA configuration preserved below:\n\n"+
				yamlIndented(yamlRaw),
			integration,
		)
		c.Fixme(diagnostics.ReasonUnmappedIntegration,
			"drivers.pkl", 0,
			"integration '"+integration+"' is outside the v1.0 driver set")
	}

	body := fmt.Sprintf(`new imported.UnmappedIntegration {
  sourceName = %q
  unpublishedDriver = %t
  sourceConfigYaml = #"""
%s
"""#
}`, integration, isV1, yamlRaw)

	return writer.DriversEntry{PreText: pre, Body: body}
}

func yamlIndented(s string) string {
	var sb strings.Builder
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		sb.WriteString("  ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return sb.String()
}
```

- [ ] **Step 6: Test**

`internal/importer/mappers/unmapped_test.go`:

```go
package mappers_test

import (
	"strings"
	"testing"

	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
	"github.com/fynn-labs/gohome/internal/importer/mappers"
)

func TestEmitUnmapped_V1Integration(t *testing.T) {
	c := diagnostics.New()
	entry := mappers.EmitUnmapped("hue", "hue:\n  bridge: 10.0.0.1\n", c)
	if !strings.Contains(entry.PreText, "FIXME(unpublished-driver)") {
		t.Errorf("PreText missing FIXME(unpublished-driver): %s", entry.PreText)
	}
	if !strings.Contains(entry.Body, `sourceName = "hue"`) {
		t.Errorf("Body missing sourceName: %s", entry.Body)
	}
	if !strings.Contains(entry.Body, "unpublishedDriver = true") {
		t.Errorf("Body should mark unpublishedDriver: %s", entry.Body)
	}
	if c.CountBy(diagnostics.SeverityFIXME) != 1 {
		t.Errorf("expected 1 FIXME, got %d", c.CountBy(diagnostics.SeverityFIXME))
	}
}

func TestEmitUnmapped_NonV1Integration(t *testing.T) {
	c := diagnostics.New()
	entry := mappers.EmitUnmapped("sonoff_lan", "sonoff_lan:\n  api_key: x\n", c)
	if !strings.Contains(entry.PreText, "FIXME(unmapped-integration)") {
		t.Errorf("PreText missing FIXME(unmapped-integration): %s", entry.PreText)
	}
	if !strings.Contains(entry.Body, "unpublishedDriver = false") {
		t.Errorf("Body should mark unpublishedDriver=false: %s", entry.Body)
	}
}
```

- [ ] **Step 7: Commit**

```bash
git add internal/importer/mappers/ internal/importer/writer/pkl_print.go internal/config/pkl/gohome/imported.pkl
git commit -m "feat(c11): mappers framework + unmapped emitter + gohome.imported Pkl module"
```

---

## Task 10: template-platform mapper (always-available)

The HA `template:` platform maps to `gohome.entities.ComputedEntity`. Lives in `mappers/` for code organization but writes to `entities/computed.pkl`, not `drivers.pkl`.

**Files:**
- Create: `internal/importer/mappers/template.go`, `template_test.go`
- Modify: `internal/importer/writer/pkl_print.go` (add `EmitComputedEntitiesPkl`)

- [ ] **Step 1: Add the emitter**

Append to `internal/importer/writer/pkl_print.go`:

```go
// EmitComputedEntitiesPkl produces entities/computed.pkl from a list
// of computed-entity definitions (one per HA template sensor or binary_sensor).
func EmitComputedEntitiesPkl(ces []ComputedEntityOut) []byte {
	var sb strings.Builder
	sb.WriteString(`amends "@gohome/config.pkl"
import "@gohome/entities.pkl" as e
import "@gohome/starlark.pkl" as starlark

computed: Listing<e.ComputedEntity> = new {
`)
	sort.Slice(ces, func(i, j int) bool { return ces[i].ID < ces[j].ID })
	for _, ce := range ces {
		fmt.Fprintf(&sb, "  new {\n    id = %q\n    class = %q\n", ce.ID, ce.Class)
		if ce.Name != "" {
			fmt.Fprintf(&sb, "    name = %q\n", ce.Name)
		}
		fmt.Fprintf(&sb, "    compute = starlark%q\n  }\n", ce.ComputeStarlark)
	}
	sb.WriteString("}\n")
	return []byte(sb.String())
}

type ComputedEntityOut struct {
	ID, Class, Name, ComputeStarlark string
}
```

- [ ] **Step 2: Implement the mapper**

`internal/importer/mappers/template.go`:

```go
package mappers

import (
	"fmt"

	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
	"github.com/fynn-labs/gohome/internal/importer/jinja"
	"github.com/fynn-labs/gohome/internal/importer/writer"
)

func init() {
	Register(&templateMapper{})
}

type templateMapper struct{}

func (*templateMapper) Integration() string         { return "template" }
func (*templateMapper) DriverModuleAvailable() bool { return true } // always — template is in-tree

func (*templateMapper) Map(in Input, c *diagnostics.Collector) Output {
	out := Output{}
	// Template platform appears as: template: [{ sensor: [{ name, state, ... }] }]
	// or sensor: { platform: template, sensors: { name: { value_template: ... } } } (legacy)
	if in.YAMLBlock == nil {
		return out
	}

	// New-style flat list of platform blocks.
	if list, ok := in.YAMLBlock["template"].([]any); ok {
		for _, item := range list {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			out.ExtraFiles = appendComputedFromBlock(out.ExtraFiles, block, c)
		}
	}

	return out
}

func appendComputedFromBlock(files []writer.File, block map[string]any, c *diagnostics.Collector) []writer.File {
	ces := []writer.ComputedEntityOut{}
	for _, kind := range []string{"sensor", "binary_sensor"} {
		entries, ok := block[kind].([]any)
		if !ok {
			continue
		}
		for _, ent := range entries {
			m, ok := ent.(map[string]any)
			if !ok {
				continue
			}
			name, _ := m["name"].(string)
			tmpl, _ := m["state"].(string)
			if name == "" || tmpl == "" {
				continue
			}
			star, diags := jinja.Transpile(tmpl)
			for _, d := range diags {
				d.File = fmt.Sprintf("entities/computed.pkl")
				c.Add(d)
			}
			ces = append(ces, writer.ComputedEntityOut{
				ID:              kind + "." + slugifyName(name),
				Class:           classForKind(kind),
				Name:            name,
				ComputeStarlark: star,
			})
		}
	}
	if len(ces) == 0 {
		return files
	}
	// Append (or merge — but for v1 simplicity, every block writes its own file pass-through).
	files = append(files, writer.File{
		Path:     "entities/computed.pkl",
		Contents: writer.EmitComputedEntitiesPkl(ces),
	})
	return files
}

func classForKind(k string) string {
	switch k {
	case "sensor":
		return "gohome.entities.Sensor"
	case "binary_sensor":
		return "gohome.entities.BinarySensor"
	}
	return "gohome.entities.Entity"
}

func slugifyName(s string) string {
	out := []byte{}
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9':
			out = append(out, ch)
		case ch >= 'A' && ch <= 'Z':
			out = append(out, ch-'A'+'a')
		case ch == ' ':
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "computed"
	}
	return string(out)
}
```

- [ ] **Step 3: Test**

`internal/importer/mappers/template_test.go`:

```go
package mappers_test

import (
	"strings"
	"testing"

	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
	"github.com/fynn-labs/gohome/internal/importer/mappers"
)

func TestTemplateMapper_NewStyle(t *testing.T) {
	c := diagnostics.New()
	m, ok := mappers.Lookup("template")
	if !ok {
		t.Fatal("template mapper not registered")
	}
	out := m.Map(mappers.Input{
		Integration: "template",
		YAMLBlock: map[string]any{
			"template": []any{
				map[string]any{
					"sensor": []any{
						map[string]any{
							"name":  "Avg Temp",
							"state": "{{ states('sensor.t1') | float }}",
						},
					},
				},
			},
		},
	}, c)
	if len(out.ExtraFiles) != 1 {
		t.Fatalf("expected 1 ExtraFile, got %d", len(out.ExtraFiles))
	}
	body := string(out.ExtraFiles[0].Contents)
	if !strings.Contains(body, `id = "sensor.avg_temp"`) {
		t.Errorf("missing id: %s", body)
	}
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/importer/mappers/template.go internal/importer/mappers/template_test.go internal/importer/writer/pkl_print.go
git commit -m "feat(c11): template-platform mapper (HA template: → ComputedEntity)"
```

---

## Task 11: Jinja transpiler

**Files:**
- Create: `internal/importer/jinja/transpile.go`, `visitor.go`, `stdlib.go`, `transpile_test.go`
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add the gonja dependency**

```bash
cd gohome
go get github.com/nikolalohinski/gonja/v2@latest
go mod tidy
```

- [ ] **Step 2: Stdlib mappings**

`internal/importer/jinja/stdlib.go`:

```go
// Package jinja translates Home Assistant Jinja templates into
// gohome Starlark. Supported constructs are documented in the spec
// (§7.1); anything outside the supported set emits FIXME(jinja-import)
// with the original construct preserved and a sentinel placeholder.
package jinja

// FunctionMap is the canonical mapping from HA helper names to
// gohome Starlark stdlib expressions. Used by visitor.go.
var FunctionMap = map[string]string{
	"states":         "state",
	"state_attr":     "state_attr",
	"is_state":       "is_state",
	"is_state_attr":  "is_state_attr",
	"has_value":      "has_value",
	"now":            "time.now",
	"utcnow":         "time.utcnow",
	"as_datetime":    "time.parse",
	"as_timestamp":   "time.timestamp",
	"today_at":       "time.today_at",
	"iif":            "iif",
}

// FilterMap is the canonical mapping for Jinja filter pipes.
// "x | filter" becomes the corresponding Starlark form per construct
// (handled by visitor.go).
var FilterMap = map[string]string{
	"float":   "float",
	"int":     "int",
	"bool":    "bool",
	"string":  "str",
	"length":  "len",
	"min":     "min",
	"max":     "max",
	"sum":     "sum",
	"average": "avg", // gohome stdlib helper
	"round":   "round",
	"abs":     "abs",
	"default": "default", // special handling — see visitor.go
}
```

- [ ] **Step 3: Transpile entry point**

`internal/importer/jinja/transpile.go`:

```go
package jinja

import (
	"fmt"
	"strings"

	"github.com/nikolalohinski/gonja/v2"
	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
)

// Transpile takes a Jinja template string (HA's flavor) and returns
// equivalent Starlark plus any diagnostics for unsupported constructs.
// On parse error, returns a single FIXME diagnostic and a placeholder.
func Transpile(input string) (string, []diagnostics.Diagnostic) {
	tmpl, err := gonja.FromString(input)
	if err != nil {
		return fixmeBlock(input, "parse error: "+err.Error()),
			[]diagnostics.Diagnostic{{
				Severity: diagnostics.SeverityFIXME,
				Reason:   diagnostics.ReasonJinjaImport,
				Message:  "parse error: " + err.Error(),
				Detail:   input,
			}}
	}
	v := newVisitor()
	output := v.visit(tmpl)
	return output, v.diags
}

func fixmeBlock(original, reason string) string {
	var sb strings.Builder
	sb.WriteString("# FIXME(jinja-import): unmapped construct\n")
	for _, line := range strings.Split(strings.TrimRight(original, "\n"), "\n") {
		fmt.Fprintf(&sb, "#   Original Jinja: %s\n", line)
	}
	if reason != "" {
		sb.WriteString("#   Reason: " + reason + "\n")
	}
	sb.WriteString("result = None  # placeholder; replace with equivalent Starlark\n")
	return sb.String()
}
```

- [ ] **Step 4: Visitor (skeleton with the most-common cases)**

`internal/importer/jinja/visitor.go`:

```go
package jinja

// NOTE: The gonja v2 AST API is in-flux — this visitor uses generic
// AST inspection patterns. The full per-construct emit logic is
// table-driven via FunctionMap / FilterMap. Where the AST shape
// reveals an unsupported construct, the visitor records a FIXME
// diagnostic and emits a placeholder.
//
// Implementation strategy:
//   1. Walk the parsed AST nodes.
//   2. For each NodeOutput (an expression in {{...}}): translate the
//      expression to Starlark via emitExpr; wrap the result with
//      whatever surrounding statement context is needed.
//   3. For each NodeIf: emit `if cond: ... else: ...`.
//   4. For each NodeFor: emit `for x in y: ...`.
//   5. For each NodeSet: emit `x = v`.
//   6. For unsupported nodes (NodeMacro, NodeInclude, NodeImport,
//      etc.): record FIXME, emit placeholder.

import (
	"fmt"
	"strings"

	"github.com/nikolalohinski/gonja/v2/parser"
	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
)

type visitor struct {
	out   strings.Builder
	diags []diagnostics.Diagnostic
}

func newVisitor() *visitor { return &visitor{} }

// visit produces Starlark from a parsed gonja template. Concrete
// node-type handling is delegated to the AST inspector below; this
// method is the entry point.
func (v *visitor) visit(t parser.Template) string {
	v.out.Reset()
	v.diags = nil
	v.walkNode(t.Root)
	return strings.TrimSpace(v.out.String())
}

// walkNode dispatches on AST node concrete type. The set of types
// handled here grows incrementally; anything else lands in fallthrough
// FIXME branches.
func (v *visitor) walkNode(n any) {
	switch node := n.(type) {
	case *parser.NodeOutput:
		expr := v.emitExpr(node.Expression)
		v.out.WriteString(fmt.Sprintf("result = %s\n", expr))
	case *parser.NodeIf:
		v.out.WriteString("if " + v.emitExpr(node.Condition) + ":\n")
		v.indented(func() { v.walkNode(node.Body) })
		for i, elif := range node.Elifs {
			_ = i
			v.out.WriteString("elif " + v.emitExpr(elif.Condition) + ":\n")
			v.indented(func() { v.walkNode(elif.Body) })
		}
		if node.Else != nil {
			v.out.WriteString("else:\n")
			v.indented(func() { v.walkNode(node.Else) })
		}
	case *parser.NodeFor:
		v.out.WriteString("for " + node.Var + " in " + v.emitExpr(node.Iter) + ":\n")
		v.indented(func() { v.walkNode(node.Body) })
	case *parser.NodeSet:
		v.out.WriteString(node.Name + " = " + v.emitExpr(node.Value) + "\n")
	case *parser.NodeMacro, *parser.NodeInclude, *parser.NodeImport:
		v.fixme("macro/include/import not supported", "")
	default:
		// Walk children if any.
		if walker, ok := n.(parser.Walkable); ok {
			for _, child := range walker.Children() {
				v.walkNode(child)
			}
		}
	}
}

// emitExpr converts a gonja expression AST to a Starlark expression
// string. This is where FunctionMap / FilterMap are consulted.
//
// NOTE: gonja's expression AST types must be referenced concretely;
// the implementer should populate the switch arms by inspecting
// gonja's AST package. The skeleton below shows the shape — fill in
// each case during implementation.
func (v *visitor) emitExpr(e any) string {
	// TODO(c11.t11): populate per-AST-type cases against gonja v2.
	// For each unhandled expression type, call v.fixme(...) and emit "None".
	v.fixme("unhandled expression", "")
	return "None"
}

func (v *visitor) indented(fn func()) {
	// Capture into a sub-visitor's buffer, indent, append.
	sub := newVisitor()
	fn()
	body := sub.out.String()
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		v.out.WriteString("    " + line + "\n")
	}
	v.diags = append(v.diags, sub.diags...)
}

func (v *visitor) fixme(reason, original string) {
	v.diags = append(v.diags, diagnostics.Diagnostic{
		Severity: diagnostics.SeverityFIXME,
		Reason:   diagnostics.ReasonJinjaImport,
		Message:  reason,
		Detail:   original,
	})
}
```

> **Implementer note (this task):** gonja v2's AST package types must be inspected and the `emitExpr` switch populated against them. The skeleton above is intentionally incomplete in the expression branch because the exact AST type names depend on the resolved gonja version; the test rows (next step) drive what concrete cases must be wired. Treat this task as: get one row at a time green by adding the corresponding emit case.

- [ ] **Step 5: Golden table tests (drive the implementation)**

`internal/importer/jinja/transpile_test.go`:

```go
package jinja_test

import (
	"strings"
	"testing"

	"github.com/fynn-labs/gohome/internal/importer/jinja"
)

type row struct {
	name      string
	input     string
	wantHas   []string // substrings the output must contain
	wantDiags int      // expected number of diagnostics
}

var goldenRows = []row{
	{
		name:    "states_call",
		input:   `{{ states('light.kitchen') }}`,
		wantHas: []string{"state('light.kitchen')"},
	},
	{
		name:    "is_state",
		input:   `{{ is_state('light.kitchen', 'on') }}`,
		wantHas: []string{"is_state('light.kitchen', 'on')"},
	},
	{
		name:    "state_attr",
		input:   `{{ state_attr('light.kitchen', 'brightness') }}`,
		wantHas: []string{"state_attr('light.kitchen', 'brightness')"},
	},
	{
		name:    "filter_float",
		input:   `{{ states('sensor.t') | float }}`,
		wantHas: []string{"float(state('sensor.t'))"},
	},
	{
		name:    "filter_default",
		input:   `{{ states('sensor.t') | default(0) }}`,
		wantHas: []string{"default", "0"},
	},
	{
		name:    "if_else",
		input:   `{% if is_state('light.kitchen', 'on') %}on{% else %}off{% endif %}`,
		wantHas: []string{"if is_state", "else:"},
	},
	{
		name:    "for_loop",
		input:   `{% for x in [1,2,3] %}{{ x }}{% endfor %}`,
		wantHas: []string{"for x in"},
	},
	{
		name:    "set_var",
		input:   `{% set x = 5 %}`,
		wantHas: []string{"x = 5"},
	},
	{
		name:    "now_helper",
		input:   `{{ now() }}`,
		wantHas: []string{"time.now()"},
	},
	{
		name:    "iif_helper",
		input:   `{{ iif(true, 'yes', 'no') }}`,
		wantHas: []string{"yes", "no"},
	},
	// FIXME cases
	{
		name:      "fixme_closest",
		input:     `{{ closest('zone.home').name }}`,
		wantHas:   []string{"FIXME(jinja-import)"},
		wantDiags: 1,
	},
	{
		name:      "fixme_macro",
		input:     `{% macro foo() %}{% endmacro %}`,
		wantHas:   []string{"FIXME(jinja-import)"},
		wantDiags: 1,
	},
	{
		name:      "fixme_expand",
		input:     `{{ expand('group.living_room') | length }}`,
		wantHas:   []string{"FIXME(jinja-import)"},
		wantDiags: 1,
	},
}

func TestTranspile_Golden(t *testing.T) {
	for _, r := range goldenRows {
		t.Run(r.name, func(t *testing.T) {
			out, diags := jinja.Transpile(r.input)
			for _, want := range r.wantHas {
				if !strings.Contains(out, want) {
					t.Errorf("output %q missing %q\nfull:\n%s", r.name, want, out)
				}
			}
			if len(diags) != r.wantDiags {
				t.Errorf("diag count = %d, want %d (out=%s)", len(diags), r.wantDiags, out)
			}
		})
	}
}
```

- [ ] **Step 6: Run tests, iterate visitor implementation until they pass**

```bash
cd gohome
go test ./internal/importer/jinja/...
```

Expected: every row passes. The implementer fills out `emitExpr`'s switch arms one at a time per failing row.

- [ ] **Step 7: Commit**

```bash
git add internal/importer/jinja/ go.mod go.sum
git commit -m "feat(c11): Jinja transpiler (gonja AST + visitor; supported set per spec §7.1)"
```

---

## Task 12: first real driver mapper (MQTT — placeholder, ships when @drivers/mqtt.pkl publishes)

**Files:**
- Create: `internal/importer/mappers/mqtt.go`, `mqtt_test.go`
- Create: `internal/importer/testdata/mappers/mqtt/{minimal,full,missing,extra}/{in.yaml,out.pkl}`

> **Implementer note:** This task is the template for every subsequent per-driver mapper. It only proceeds once `@drivers/mqtt.pkl` is published in the gohome ecosystem. Until then, the unmapped emitter handles MQTT — see Task 9. When implementing, take the published `@drivers/mqtt.pkl` schema and write the mapper to translate HA's `mqtt:` block field-by-field.

- [ ] **Step 1: Inspect the published `@drivers/mqtt.pkl`**

The mapper must emit values that conform to whatever `mqtt.Instance` Pkl shape the driver publishes. Read it before writing the mapper.

- [ ] **Step 2: Write the four standard test fixtures**

For each of `minimal/`, `full/`, `missing/`, `extra/`, populate:
- `in.yaml` — the HA `mqtt:` block (or in `extra/`'s case, a block with unrecognized keys)
- `out.pkl` — the byte-equal expected `drivers.pkl` entry the mapper should emit

- [ ] **Step 3: Write the mapper**

Use the `Mapper` interface from Task 9, register via `init()` in the catalog, translate fields per the published schema.

- [ ] **Step 4: Run tests + commit**

```bash
cd gohome
go test ./internal/importer/mappers/ -run MQTT
git add internal/importer/mappers/mqtt.go internal/importer/mappers/mqtt_test.go internal/importer/testdata/mappers/mqtt/
git commit -m "feat(c11): MQTT mapper (HA mqtt: → @drivers/mqtt.Instance)"
```

---

## Task 13: subsequent per-driver mappers

**Pattern.** For each remaining v1.0 integration whose `@drivers/<name>.pkl` is published, ship one PR per integration following Task 12's pattern: fixtures → mapper → catalog registration → tests → commit.

**Tracking checklist** (mark each as the corresponding driver Pkl publishes):

- [ ] `mappers/zigbee2mqtt.go`
- [ ] `mappers/esphome.go`
- [ ] `mappers/homekit.go`
- [ ] `mappers/matter.go`
- [ ] `mappers/hue.go`
- [ ] `mappers/nest.go`
- [ ] `mappers/zwave_js.go`
- [ ] `mappers/rest.go`
- [ ] `mappers/webhook.go`

When a mapper lands, also: remove the now-redundant FIXME(unpublished-driver) test case for that integration in `unmapped_test.go`.

---

## Task 14: scenes handler

**Files:**
- Create: `internal/importer/scenes/scenes.go`, `scenes_test.go`
- Modify: `internal/importer/writer/pkl_print.go` (add `EmitScenesPkl`)

- [ ] **Step 1: Add emitter**

Append to `internal/importer/writer/pkl_print.go`:

```go
// EmitScenesPkl produces scenes.pkl.
func EmitScenesPkl(scenes []SceneOut) []byte {
	var sb strings.Builder
	sb.WriteString(`amends "@gohome/config.pkl"
import "@gohome/scenes.pkl" as s

scenes: Listing<s.Scene> = new {
`)
	sort.Slice(scenes, func(i, j int) bool { return scenes[i].ID < scenes[j].ID })
	for _, sc := range scenes {
		fmt.Fprintf(&sb, "  new {\n    id = %q\n    name = %q\n    targets = new {\n", sc.ID, sc.Name)
		for _, t := range sc.Targets {
			fmt.Fprintf(&sb, "      new { entityId = %q", t.EntityID)
			for k, v := range t.Fields {
				switch tv := v.(type) {
				case string:
					fmt.Fprintf(&sb, "; %s = %q", k, tv)
				case bool:
					fmt.Fprintf(&sb, "; %s = %t", k, tv)
				case float64:
					fmt.Fprintf(&sb, "; %s = %g", k, tv)
				case int:
					fmt.Fprintf(&sb, "; %s = %d", k, tv)
				}
			}
			sb.WriteString(" }\n")
		}
		sb.WriteString("    }\n  }\n")
	}
	sb.WriteString("}\n")
	return []byte(sb.String())
}

type SceneOut struct {
	ID, Name string
	Targets  []SceneTargetOut
}

type SceneTargetOut struct {
	EntityID string
	Fields   map[string]any
}
```

- [ ] **Step 2: Implement scenes handler**

`internal/importer/scenes/scenes.go`:

```go
// Package scenes translates scenes.yaml into scenes.pkl.
package scenes

import (
	"github.com/fynn-labs/gohome/internal/importer"
	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
	"github.com/fynn-labs/gohome/internal/importer/writer"
)

func Process(in []importer.SceneModel, _ *diagnostics.Collector) []writer.File {
	out := make([]writer.SceneOut, 0, len(in))
	for _, s := range in {
		so := writer.SceneOut{ID: s.ID, Name: s.Name}
		for entityID, raw := range s.Entities {
			t := writer.SceneTargetOut{EntityID: entityID, Fields: map[string]any{}}
			switch v := raw.(type) {
			case string:
				t.Fields["state"] = v
			case map[string]any:
				for k, val := range v {
					t.Fields[k] = val
				}
			}
			so.Targets = append(so.Targets, t)
		}
		out = append(out, so)
	}
	if len(out) == 0 {
		return nil
	}
	return []writer.File{{Path: "scenes.pkl", Contents: writer.EmitScenesPkl(out)}}
}
```

- [ ] **Step 3: Test**

`internal/importer/scenes/scenes_test.go`:

```go
package scenes_test

import (
	"strings"
	"testing"

	"github.com/fynn-labs/gohome/internal/importer"
	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
	"github.com/fynn-labs/gohome/internal/importer/scenes"
)

func TestProcess_BasicScene(t *testing.T) {
	c := diagnostics.New()
	files := scenes.Process([]importer.SceneModel{
		{ID: "movie_time", Name: "Movie Time", Entities: map[string]any{
			"light.living_room": map[string]any{"state": "on", "brightness": 30.0},
			"light.kitchen":     map[string]any{"state": "off"},
		}},
	}, c)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	body := string(files[0].Contents)
	for _, want := range []string{`id = "movie_time"`, `entityId = "light.living_room"`, `state = "on"`, `brightness = 30`} {
		if !strings.Contains(body, want) {
			t.Errorf("scenes.pkl missing %q: %s", want, body)
		}
	}
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/importer/scenes/ internal/importer/writer/pkl_print.go
git commit -m "feat(c11): scenes handler"
```

---

## Task 15: scripts handler

**Files:**
- Create: `internal/importer/scripts/scripts.go`, `scripts_test.go`
- Modify: `internal/importer/writer/pkl_print.go` (add `EmitScriptPkl`)

- [ ] **Step 1: Implement emitters and handler**

The shape is similar to scenes but each script gets its own `scripts/<slug>.pkl` plus optional `scripts/bodies/<slug>.star` for non-trivial sequences. Body translation reuses Task 16's action translation tables (forward dependency: scripts and automations share that machinery, so this task is mostly a thin wrapper around what Task 16 provides).

For now, ship a scripts handler that:
- Translates parameters into the script Pkl.
- Translates the action sequence using a shared `actions` package (created in Task 16).
- Wraps any Jinja in actions through the transpiler from Task 11.

Implementation skeleton with the same shape as scenes — `Process(scripts []importer.ScriptModel, c *diagnostics.Collector) []writer.File`.

- [ ] **Step 2: Tests + commit**

```bash
go test ./internal/importer/scripts/...
git add internal/importer/scripts/ internal/importer/writer/pkl_print.go
git commit -m "feat(c11): scripts handler"
```

---

## Task 16: automations handler

The largest area handler. Triggers, conditions, actions each have their own translation table. Jinja is transpiled via Task 11.

**Files:**
- Create: `internal/importer/automations/automations.go`, `triggers.go`, `conditions.go`, `actions.go`, `automations_test.go`
- Modify: `internal/importer/writer/pkl_print.go` (add `EmitAutomationPkl`)

- [ ] **Step 1: Translation tables**

For each of `triggers.go`, `conditions.go`, `actions.go`, build a function that takes one HA YAML block and returns the corresponding gohome Pkl + Starlark fragments. Map per the tables in spec §9.1.

- [ ] **Step 2: Top-level orchestration**

`automations.Process` iterates `[]importer.AutomationModel`, produces one `automations/<slug>.pkl` per automation plus an optional `automations/handlers/<slug>.star` for the body.

- [ ] **Step 3: Per-table tests**

One test per row in each table, asserting the translation produces the expected output. FIXME-emitting cases (unmapped trigger / action) assert the diagnostic is collected and a placeholder block is emitted.

- [ ] **Step 4: Commit**

```bash
git add internal/importer/automations/ internal/importer/writer/pkl_print.go
git commit -m "feat(c11): automations handler (triggers + conditions + actions tables)"
```

---

## Task 17: end-to-end against the `minimal/` fixture

**Files:**
- Create: `internal/importer/integration_test.go`
- Create: `internal/importer/testdata/configs/minimal/<HA-config-tree>/`
- Create: `internal/importer/testdata/configs/minimal/expected/<full-output-tree>/`
- Create: `internal/importer/testdata/README.md`

- [ ] **Step 1: Build the `minimal/` fixture**

A minimal HA config exercising 2 integrations (MQTT + template), 3 automations, 1 scene, 1 user, 1 area, 1 zone. Anonymized per the rules in `testdata/README.md`.

- [ ] **Step 2: Generate the expected output once, by hand**

Run the importer manually against the fixture, inspect the output, hand-correct any drift, and commit the result as the `expected/` tree.

- [ ] **Step 3: Write the integration test**

`internal/importer/integration_test.go`:

```go
//go:build integration

package importer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fynn-labs/gohome/internal/importer"
)

func TestIntegration_MinimalFixture(t *testing.T) {
	src := "testdata/configs/minimal"
	expected := "testdata/configs/minimal/expected"
	out := t.TempDir()

	if _, err := importer.Run(importer.ImportOptions{
		HADir:  src,
		OutDir: out,
		Force:  true,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Walk expected/ and assert each file matches what was written.
	err := filepath.Walk(expected, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(expected, p)
		want, _ := os.ReadFile(p)
		got, err := os.ReadFile(filepath.Join(out, rel))
		if err != nil {
			return err
		}
		if string(got) != string(want) {
			t.Errorf("%s mismatch:\n--got--\n%s\n--want--\n%s", rel, got, want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 4: Run + commit**

```bash
task test:integration
git add internal/importer/integration_test.go internal/importer/testdata/configs/minimal/ internal/importer/testdata/README.md
git commit -m "test(c11): end-to-end against minimal fixture"
```

---

## Task 18: end-to-end against the `kitchensink/` fixture

Same shape as Task 17 but with a much larger fixture exercising every supported Jinja construct, every published mapper, area hierarchy heuristics, multi-user auth, secrets, `packages/`, etc.

- [ ] **Steps 1-4:** As Task 17, with a richer fixture.

```bash
git add internal/importer/testdata/configs/kitchensink/
git commit -m "test(c11): end-to-end against kitchensink fixture"
```

---

## Task 19: orchestration + CLI polish

Now we wire the `Run()` function and finalize the CLI.

**Files:**
- Modify: `internal/importer/importer.go` (the orchestrator wiring everything together)
- Modify: `internal/importer/opts.go`
- Modify: `internal/cli/cmd_import.go` (replace the `not yet implemented` body)

- [ ] **Step 1: Orchestrate**

`internal/importer/importer.go`:

```go
package importer

import (
	"fmt"
	"time"

	"github.com/fynn-labs/gohome/internal/importer/auth"
	"github.com/fynn-labs/gohome/internal/importer/automations"
	"github.com/fynn-labs/gohome/internal/importer/core"
	"github.com/fynn-labs/gohome/internal/importer/diagnostics"
	"github.com/fynn-labs/gohome/internal/importer/mappers"
	"github.com/fynn-labs/gohome/internal/importer/registry"
	"github.com/fynn-labs/gohome/internal/importer/scenes"
	"github.com/fynn-labs/gohome/internal/importer/scripts"
	"github.com/fynn-labs/gohome/internal/importer/secrets"
	"github.com/fynn-labs/gohome/internal/importer/writer"
)

type ImportOptions struct {
	HADir   string
	OutDir  string
	DryRun  bool
	Force   bool
	Verbose bool
}

type RunResult struct {
	Files       []writer.File
	Diagnostics *diagnostics.Collector
	Summary     diagnostics.ReportSummary
}

func Run(opts ImportOptions) (*RunResult, error) {
	scanned, err := ScanHADir(opts.HADir)
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}

	c := diagnostics.New()

	// Load secrets first — every other handler may need them.
	haSecrets := map[string]string{}
	if scanned.SecretsYAML != "" {
		raw, err := LoadYAML(opts.HADir, "secrets.yaml", nil)
		if err != nil {
			return nil, fmt.Errorf("load secrets: %w", err)
		}
		if m, ok := raw.(map[string]any); ok {
			for k, v := range m {
				if s, ok := v.(string); ok {
					haSecrets[k] = s
				}
			}
		}
	}

	// Build the HA model from the rest.
	model, err := buildHAModel(scanned, haSecrets, c)
	if err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}

	result := &writer.Result{}

	// Per-area handlers.
	for _, f := range secrets.Process(haSecrets, c) {
		result.Add(f)
	}
	for _, f := range core.Process(model.Configuration, hasOverrides(model), hasComputed(model), c) {
		result.Add(f)
	}

	areas := registry.ProcessAreas(model.Areas, c)
	zones := registry.ProcessZones(model.Zones, c)
	overrides := registry.ProcessEntities(model.Entities, c)
	registry.ProcessDevices(model.Devices, c)

	if len(areas) > 0 {
		result.Add(writer.File{Path: "areas.pkl", Contents: writer.EmitAreasPkl(areas)})
	}
	if len(zones) > 0 {
		result.Add(writer.File{Path: "zones.pkl", Contents: writer.EmitZonesPkl(zones)})
	}
	if len(overrides) > 0 {
		result.Add(writer.File{Path: "entities/overrides.pkl", Contents: writer.EmitEntityOverridesPkl(overrides)})
	}

	for _, f := range scenes.Process(model.Scenes, c) {
		result.Add(f)
	}
	for _, f := range scripts.Process(model.Scripts, c) {
		result.Add(f)
	}
	for _, f := range automations.Process(model.Automations, c) {
		result.Add(f)
	}
	for _, f := range auth.Process(model.Users, model.Persons, c) {
		result.Add(f)
	}

	// Mappers.
	driversEntries := []writer.DriversEntry{}
	for integration, block := range integrations(model) {
		mapper, ok := mappers.Lookup(integration)
		if !ok {
			driversEntries = append(driversEntries, mappers.EmitUnmapped(integration, yamlString(block), c))
			continue
		}
		out := mapper.Map(mappers.Input{
			Integration: integration,
			Source:      mappers.SourceYAML, // approximate; real code distinguishes
			YAMLBlock:   map[string]any{integration: block},
			YAMLRaw:     yamlString(block),
		}, c)
		driversEntries = append(driversEntries, out.Entry)
		for _, f := range out.ExtraFiles {
			result.Add(f)
		}
	}
	if len(driversEntries) > 0 {
		result.Add(writer.File{Path: "drivers.pkl", Contents: writer.EmitDriversPkl(driversEntries)})
	}

	// Lovelace presence note.
	if scanned.LovelacePresent {
		c.Note(diagnostics.ReasonLovelaceNotImported, "(no file written)", 0,
			"HA Lovelace dashboards detected but not translated; rebuild via gohome's WYSIWYG editor (or wait for the Lovelace import follow-on milestone).")
	}

	// Build summary.
	summary := buildSummary(scanned, model, c)
	summary.GeneratedAt = time.Now().UTC()
	summary.SourceHADir = opts.HADir

	// Render report last.
	report := diagnostics.Render(summary, c)
	result.Add(writer.File{Path: "IMPORT_REPORT.md", Contents: []byte(report)})

	if !opts.DryRun {
		if err := writer.WriteAll(writer.Options{OutDir: opts.OutDir, Force: opts.Force}, result); err != nil {
			return nil, fmt.Errorf("write: %w", err)
		}
	}

	return &RunResult{Files: result.Files, Diagnostics: c, Summary: summary}, nil
}

// Helpers — implementation per the appropriate handler.
func buildHAModel(s *ScannedConfig, secrets map[string]string, c *diagnostics.Collector) (*HAModel, error) { /* ... */ return &HAModel{}, nil }
func hasOverrides(m *HAModel) bool { /* ... */ return false }
func hasComputed(m *HAModel) bool { /* ... */ return false }
func integrations(m *HAModel) map[string]any { /* ... */ return nil }
func yamlString(v any) string { /* yaml.Marshal */ return "" }
func buildSummary(s *ScannedConfig, m *HAModel, c *diagnostics.Collector) diagnostics.ReportSummary {
	return diagnostics.ReportSummary{HAVersion: s.HAVersion}
}
```

> **Implementer note:** the helper functions at the bottom are stubs to keep the orchestrator file readable in the plan. Each must be filled in during implementation — `buildHAModel` is the bulk (pulling each loader's output into the typed model), and `integrations` enumerates the integrations the user has configured (from both `configuration.yaml` and `.storage/core.config_entries`).

- [ ] **Step 2: Wire CLI**

Replace the `RunE` body in `internal/cli/cmd_import.go`:

```go
RunE: func(cmd *cobra.Command, args []string) error {
	if opts.out == "" {
		return errors.New("import-ha: --out is required")
	}
	if opts.verbose && opts.quiet {
		return errors.New("import-ha: -v and -q are mutually exclusive")
	}

	haDir := ""
	if len(args) > 0 {
		haDir = args[0]
	} else {
		for _, candidate := range []string{
			os.ExpandEnv("$HOME/.homeassistant"),
			"/config",
		} {
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				haDir = candidate
				break
			}
		}
		if haDir == "" {
			return errors.New("import-ha: no <ha-dir> given and ~/.homeassistant / /config not present")
		}
	}

	res, err := importer.Run(importer.ImportOptions{
		HADir:   haDir,
		OutDir:  opts.out,
		DryRun:  opts.dryRun,
		Force:   opts.force,
		Verbose: opts.verbose,
	})
	if err != nil {
		return err
	}

	// Print stderr summary box.
	fixmes := res.Diagnostics.CountBy(diagnostics.SeverityFIXME)
	notes := res.Diagnostics.CountBy(diagnostics.SeverityNOTE)

	if opts.dryRun {
		// Render report to stdout.
		fmt.Fprint(cmd.OutOrStdout(), diagnostics.Render(res.Summary, res.Diagnostics))
		fmt.Fprintf(cmd.ErrOrStderr(), "[dry-run] %d FIXMEs, %d NOTEs.\n", fixmes, notes)
		return nil
	}

	fmt.Fprintf(cmd.ErrOrStderr(),
		"Done — %d FIXMEs, %d NOTEs across %d generated files. See %s.\n",
		fixmes, notes, len(res.Files),
		filepath.Join(opts.out, "IMPORT_REPORT.md"))
	return nil
},
```

(Add imports for `os`, `fmt`, `filepath`, `internal/importer`, `internal/importer/diagnostics`.)

- [ ] **Step 3: Verify the existing `cmd_import_test.go` failures around 'not yet implemented' fail and update**

The unimplemented-error assertion needs updating to assert successful execution against an empty fixture instead.

- [ ] **Step 4: Commit**

```bash
git add internal/importer/importer.go internal/importer/opts.go internal/cli/cmd_import.go internal/cli/cmd_import_test.go
git commit -m "feat(c11): orchestrate the importer pipeline + wire CLI"
```

---

## Task 20: documentation

**Files:**
- Create: `docs/import-ha.md`
- Modify: `README.md`

- [ ] **Step 1: Write `docs/import-ha.md`**

Cover:
- Overview (what `gohome import-ha` does and doesn't do)
- Bootstrap walkthrough (running it against `~/.homeassistant`)
- Reading the report and resolving FIXMEs
- The secrets workflow (sourcing `IMPORTED_SECRETS.env`, deleting it)
- What's not imported (Lovelace dashboards, recorder DB, HACS)
- Re-import workflow (delete + re-run)
- Per-FIXME-reason resolution guide (one section per `Reason` enum value)

- [ ] **Step 2: Add a "Migrating from Home Assistant" section to top-level `README.md`**

A short paragraph linking `docs/import-ha.md`.

- [ ] **Step 3: Commit**

```bash
git add docs/import-ha.md README.md
git commit -m "docs(c11): import-ha user guide + README link"
```

---

## Task 21: final pass — lint, race, integration, lipgloss styling

- [ ] **Step 1: Run the full DoD check**

```bash
cd gohome
task lint
task test
task test:race
task test:integration
go mod tidy   # confirm clean
```

Expected: every command exits 0; `go.mod` / `go.sum` clean.

- [ ] **Step 2: Apply lipgloss styling to the CLI summary**

Wrap the stderr summary in `internal/cli/cmd_import.go` with the styles from `styles_import.go`:

```go
fmt.Fprintln(cmd.ErrOrStderr(),
	importSummaryBox.Render(fmt.Sprintf(
		"Done — %s %d  %s %d  across %d generated files\n%s %s",
		importFixmeBadge.Render("FIXMEs"), fixmes,
		importNoteBadge.Render("NOTEs"), notes,
		len(res.Files),
		"Report:", importReportPath.Render(filepath.Join(opts.out, "IMPORT_REPORT.md")),
	)))
```

- [ ] **Step 3: Verify --no-color works**

```bash
./dist/gohome import-ha --no-color -o /tmp/gh-out --dry-run /tmp/empty-ha
```

Expected: no ANSI codes in stderr.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/cmd_import.go
git commit -m "feat(c11): lipgloss-styled CLI completion summary"
```

---

## Final smoke check

```bash
cd gohome
task web:build  # if web/ exists post-C10
task build

# Run against a small fixture HA dir
mkdir -p /tmp/fake-ha
cp -r internal/importer/testdata/configs/minimal/* /tmp/fake-ha/

# Dry-run
./dist/gohome import-ha /tmp/fake-ha -o /tmp/gh-out --dry-run

# Real run
./dist/gohome import-ha /tmp/fake-ha -o /tmp/gh-out

# Inspect output
ls /tmp/gh-out/
cat /tmp/gh-out/IMPORT_REPORT.md

# Confirm secrets are .gitignored
cat /tmp/gh-out/.gitignore | grep IMPORTED_SECRETS

# Validate
./dist/gohomed config validate --root /tmp/gh-out
```

---

## Spec coverage check

Reverse-mapping from spec sections to plan tasks:

| Spec § | Plan tasks |
|---|---|
| §1 Scope | covered transitively by every task; deferrals checked at task 20 docs step |
| §2 Background | informational only |
| §3 Architecture Overview | tasks 1 (CLI scaffold), 2 (scanner + loader), 19 (orchestrator) |
| §4 CLI Surface | task 1 (scaffold), task 19 (real CLI), task 21 (styling) |
| §5 Scanner & YAML Loader | task 2 |
| §6 Per-Integration Mappers | task 9 (framework + unmapped), task 10 (template), task 12 (first real — MQTT), task 13 (subsequent) |
| §7 Jinja Transpiler | task 11 |
| §8 Registry Translation | task 6 |
| §9 Automations / Scripts / Scenes | task 14 (scenes), task 15 (scripts), task 16 (automations) |
| §10 Auth, Users & Persons | task 8 |
| §11 Secrets Pipeline | task 5 |
| §12 Diagnostics & IMPORT_REPORT.md | task 3 |
| §13 Writer & Output Layout | task 4 + emitters added incrementally per area in tasks 5–10, 14–16 |
| §14 Testing Strategy | per-task tests throughout; end-to-end in tasks 17 + 18 |
| §15 Implementation Order | this plan's task order maps 1:1 to the spec's order |
| §16 Decision Record | informational only |
| §17 Explicit Deferrals | called out in task 20 docs |

---

*End of C11 implementation plan.*
