# C11 — HA Import Tool Design

**Parent:** [gohome Master Design](./2026-04-21-gohome-master-design.md)
**Date:** 2026-04-26
**Status:** Draft
**Depends on:** C1 (Event schema for `ConfigApplied`), C4 (Pkl module shapes the importer writes against), C5 (Starlark sandbox the transpiler targets), C9 (auth Pkl shapes), C10 (dashboards Pkl shapes — read-only here, since Lovelace translation is deferred)
**Closes:** the master design's §8.1 "HA import tool" promise, *minus* Lovelace dashboard translation (called out as a follow-on milestone — see §17).

---

## Table of Contents

1. [Scope](#1-scope)
2. [Background](#2-background)
3. [Architecture Overview](#3-architecture-overview)
4. [CLI Surface](#4-cli-surface)
5. [Scanner & YAML Loader](#5-scanner--yaml-loader)
6. [Per-Integration Mappers](#6-per-integration-mappers)
7. [Jinja Transpiler](#7-jinja-transpiler)
8. [Registry Translation](#8-registry-translation)
9. [Automations, Scripts & Scenes](#9-automations-scripts--scenes)
10. [Auth, Users & Persons](#10-auth-users--persons)
11. [Secrets Pipeline](#11-secrets-pipeline)
12. [Diagnostics & `IMPORT_REPORT.md`](#12-diagnostics--import_reportmd)
13. [Writer & Output Layout](#13-writer--output-layout)
14. [Testing Strategy](#14-testing-strategy)
15. [Implementation Order](#15-implementation-order)
16. [Decision Record](#16-decision-record)
17. [Explicit Deferrals](#17-explicit-deferrals)

---

## 1. Scope

C11 ships `gohome import-ha` — a CLI subcommand that reads a Home Assistant config directory and produces a fresh, git-initable gohome Pkl tree the user can review, commit, edit, and point `gohomed` at. The importer covers everything in master design §8.1 *except* Lovelace dashboard translation, which becomes a separate follow-on milestone (rationale in §17). After C11, a prosumer with a working HA setup can run one command and walk away with a structurally-complete gohome config — minus Lovelace dashboards (rebuilt in gohome's WYSIWYG editor) and minus integrations whose gohome drivers haven't been published yet (clear placeholders).

### 1.1 In scope

- **`gohome import-ha <ha-dir> -o <out-dir>` CLI** with `--dry-run`, `--force`, `-v` / `-q` flags. Single-shot model: refuses to overwrite a non-empty output dir without `--force`.
- **HA config file scanner** that walks the input dir, classifies files (`configuration.yaml`, `automations.yaml`, `scripts.yaml`, `scenes.yaml`, `secrets.yaml`, `groups.yaml`, `customize.yaml`, `packages/`, `.storage/core.area_registry`, `.storage/core.entity_registry`, `.storage/core.device_registry`, `.storage/core.config_entries`, `.storage/auth*`, `.storage/person`), and feeds them to the right handlers.
- **HA YAML loader** that handles HA's custom tags: `!include`, `!include_dir_list`, `!include_dir_merge_list`, `!include_dir_named`, `!include_dir_merge_named`, `!secret`, `!input`. Built on `gopkg.in/yaml.v3` with a custom resolver.
- **`Mapper` plug-in interface** in `internal/importer/mappers/` with one focused mapper file per v1.0 integration (the eleven from §8.1: MQTT, Zigbee2MQTT, ESPHome, HomeKit, Matter, Hue, Nest, Z-Wave JS, generic REST, generic webhook, plus the `template` platform mapped to `ComputedEntity`). Each mapper is a deep field-by-field translation of HA's integration config block to the matching gohome `@drivers/<name>.Instance` Pkl.
- **Mapper shipping discipline:** each mapper depends on the corresponding gohome driver's Pkl manifest existing. The C11 implementation plan inventories which drivers have published manifests at planning time and ships those mappers; remaining mappers ship as their drivers do, in follow-on PRs. For unpublished-driver integrations from the v1.0 list, the importer emits a `FIXME(unpublished-driver)` placeholder with the original HA YAML preserved.
- **Generic unmapped-integration handler** for HA integrations entirely outside the v1.0 driver set: emits a `FIXME(unmapped-integration)` placeholder with the original YAML preserved verbatim.
- **Jinja → Starlark transpiler** (`internal/importer/jinja/`) using `github.com/nikolalohinski/gonja` for AST; visitor emits Starlark for the supported construct set in §7.1; emits `FIXME(jinja-import)` with the original construct preserved for everything outside the supported set.
- **Per-area handlers**: separate subpackages for `automations/`, `scripts/`, `scenes/`, `registry/` (areas/zones/entities/devices from `.storage/*`), `core/` (configuration.yaml → main.pkl + settings.pkl), `auth/` (users + persons → `auth/users.pkl`), `secrets/` (`secrets.yaml` → `secrets.pkl` + `IMPORTED_SECRETS.env`).
- **Diagnostics collector** threaded through every package; central `*Collector` argument; renders both inline `# FIXME(<reason>)` / `# NOTE(<reason>)` comments in generated files and a top-level `IMPORT_REPORT.md` per the format in §12.
- **Writer** that takes the in-memory `ImportResult` tree from the pipeline and writes it to the output dir; canonical Pkl emission via `text/template`-based helpers (no general-purpose Pkl AST emitter in v1.0).
- **Two real-world fixture configs** under `internal/importer/testdata/configs/{minimal,kitchensink}/` plus per-construct synthetic fixtures under `internal/importer/testdata/{mappers,jinja}/<case>/`. Documented anonymization rules. `gohome config validate` passes against both real-world fixtures' output in CI.
- **Lipgloss-styled CLI output** for progress + completion summary, mirroring the existing `gohome` CLI's style conventions.

### 1.2 Out of scope (deferred / follow-on)

- **Lovelace dashboard translation.** Becomes a separate follow-on milestone (suggested name: C11.5 or C12, depending on slotting). Rationale: Lovelace card variety is wildly larger than HA's other constructs; design depends on C10's widget set + container schema being battle-tested by real users. See §17 for full rationale.
- **HA recorder DB migration** (history events into gohome's event log). Master design §8.1 explicit non-goal.
- **HACS integration translation.** Master design §8.1 explicit non-goal.
- **Supervisor add-on translation.** Master design §8.1 explicit non-goal.
- **Per-driver mappers for integrations outside the v1.0 list.** Always FIXME placeholder.
- **Per-driver mappers for v1.0 integrations whose drivers aren't yet published** (deferred to when the driver lands; see §6.4).
- **Repeatable / merge-mode imports.** Single-shot only; users delete and re-import to re-sync.
- **Integration filter flag** (`--integrations <list>` for partial imports). YAGNI; full imports are simpler.
- **Lovelace card → widget mapping.** Even partial. The Lovelace milestone owns this entirely.
- **Jinja macros, area expansion (`expand`, `area_entities`, `device_entities`), geographic helpers (`closest`, `distance`), HACS custom filters.** Top ~5% of HA Jinja vocabulary; emit `FIXME(jinja-import)` per §7.

### 1.3 Inherited from the master design

- The mapping table in §8.1 is the contract for what the importer produces.
- The output directory layout matches master design §6.5 (`main.pkl`, `drivers.pkl`, `areas.pkl`, `zones.pkl`, `entities/`, `automations/`, `scripts/`, `scenes.pkl`, `auth/`, `secrets.pkl`).
- "Output is a git-initable directory" framing implies single-shot, user-curated.

---

## 2. Background

The master design's §8.1 commits to an HA importer as part of v1.0's migration story. The pitch to a prospective gohome user is: "you can keep your existing HA setup until you're ready, and when you are, one command produces a starting point you review and refine." Without this, gohome's prosumer audience hits a "rebuild everything by hand" wall that kills migration.

The importer is shaped by three constraints:

1. **HA's config surface is wide and inconsistent.** Some things live in YAML files (`automations.yaml`, `scripts.yaml`), some in JSON files under `.storage/` (registries, auth), some in both (entities can be UI-managed *and* YAML-declared). HA also supports custom YAML tags (`!include`, `!secret`) that interleave files. The importer needs a deliberate scanner / loader rather than ad-hoc per-file readers.
2. **gohome's driver ecosystem ships incrementally.** The eleven v1.0 drivers from master design §2.2 / §8.1 land in their own repos over time. C11 ships when most haven't yet — its design must accommodate per-mapper shipping in follow-on PRs and gracefully degrade to FIXME placeholders for unpublished drivers.
3. **Jinja is HA's lingua franca for any non-trivial logic.** Automations, scripts, computed entities, and notifications all embed Jinja. A useful import requires translating the common Jinja vocabulary; an *honest* import refuses to silently translate things it can't and leaves a clear FIXME the user resolves with full context.

Lovelace is the master design's largest open-ended HA-side surface. Card variety, custom cards via HACS, conditional cards, and the actively-evolving "sections" view together would dwarf the rest of the importer. This spec defers Lovelace to a separate milestone so the core migration value (drivers + automations + scripts + scenes + entities + secrets + users) ships in a focused C11 rather than waiting on Lovelace to settle.

---

## 3. Architecture Overview

### 3.1 Pipeline

```
                              ┌──────────────────────────┐
   <ha-dir>  ──► Scanner ──►  │ classified file inventory │
                              └────────────┬─────────────┘
                                           │
                  ┌────────────────────────┴────────────────────────┐
                  │ YAML Loader (handles !include, !secret, !input) │
                  └────────────────────────┬────────────────────────┘
                                           │ typed in-memory HA model
                                           ▼
       ┌─────────────────────────────────────────────────────────────────┐
       │                   Per-area handlers                             │
       │  ┌──────────────┐ ┌──────────────┐ ┌──────────────────────┐     │
       │  │ core         │ │ registry     │ │ mappers (per integ.) │     │
       │  │ (config.yaml)│ │ (.storage/*) │ │ (drivers, instances) │     │
       │  └──────────────┘ └──────────────┘ └──────────────────────┘     │
       │  ┌──────────────┐ ┌──────────────┐ ┌──────────────────────┐     │
       │  │ automations  │ │ scripts      │ │ scenes               │     │
       │  └──────┬───────┘ └──────┬───────┘ └──────────────────────┘     │
       │         │                │                                      │
       │         └────►  Jinja Transpiler  ◄──┐                          │
       │  ┌──────────────┐                    │                          │
       │  │ auth         │ ┌──────────────┐ ┌─┴──────────────────┐       │
       │  │ (users +     │ │ secrets      │ │ Diagnostics        │       │
       │  │  persons)    │ │ (.yaml +     │ │ Collector          │       │
       │  │              │ │  side-file)  │ │ (FIXME / NOTE)     │       │
       │  └──────────────┘ └──────────────┘ └────────────────────┘       │
       └─────────────────────────────────────────────────────────────────┘
                                           │ ImportResult tree
                                           ▼
                              ┌──────────────────────────┐
                              │         Writer           │
                              │  (canonical Pkl print)   │
                              └────────────┬─────────────┘
                                           │
              ┌────────────────────────────┴────────────────────────────┐
              │                       <out-dir>/                         │
              │  main.pkl  drivers.pkl  areas.pkl  zones.pkl  scenes.pkl │
              │  entities/{computed,overrides}.pkl                       │
              │  automations/*.pkl + handlers/*.star                     │
              │  scripts/*.pkl + bodies/*.star                           │
              │  auth/{users,roles,policies}.pkl                         │
              │  secrets.pkl  IMPORTED_SECRETS.env  .gitignore           │
              │  IMPORT_REPORT.md                                        │
              └──────────────────────────────────────────────────────────┘
```

### 3.2 Package layout

```
internal/importer/                       # top-level entry
├── importer.go                          # Run(opts) — orchestrates the pipeline
├── opts.go                              # ImportOptions struct
├── scanner.go                           # walks <ha-dir>, classifies files
├── yaml_loader.go                       # handles !include / !secret / !include_dir_* / !input
├── importer_test.go
│
├── mappers/                             # per-integration mappers
│   ├── mapper.go                        # Mapper interface + shared types
│   ├── catalog.go                       # registry of all known mappers
│   ├── unmapped.go                      # generic unmapped-integration emitter
│   ├── mqtt.go, hue.go, ...             # one per published-driver integration
│   ├── template.go                      # template platform → ComputedEntity
│   └── *_test.go
│
├── jinja/                               # Jinja → Starlark transpiler
│   ├── transpile.go                     # Transpile(string) → (starlark, []Diagnostic)
│   ├── visitor.go                       # gonja AST walker
│   ├── stdlib.go                        # HA helper mappings
│   └── *_test.go
│
├── core/                                # configuration.yaml → main.pkl, settings.pkl
│   └── *.go, *_test.go
├── registry/                            # .storage/core.*_registry → areas.pkl, zones.pkl, entities/overrides.pkl
│   └── *.go, *_test.go
├── automations/                         # automations.yaml(+split) → automations/*.pkl + handlers/*.star
│   └── *.go, *_test.go
├── scripts/                             # scripts.yaml(+split) → scripts/*.pkl + bodies/*.star
│   └── *.go, *_test.go
├── scenes/                              # scenes.yaml → scenes.pkl
│   └── *.go, *_test.go
├── auth/                                # .storage/auth* + person → auth/users.pkl
│   └── *.go, *_test.go
├── secrets/                             # secrets.yaml → secrets.pkl + IMPORTED_SECRETS.env
│   └── *.go, *_test.go
│
├── writer/                              # filesystem output
│   ├── writer.go                        # WriteAll(outDir, *ImportResult)
│   └── pkl_print.go                     # text/template-based canonical Pkl emitters
│
├── diagnostics/                         # FIXME / NOTE collector + IMPORT_REPORT.md renderer
│   ├── diagnostics.go                   # Collector + Diagnostic struct
│   ├── report.go                        # IMPORT_REPORT.md renderer
│   └── *_test.go
│
├── testdata/                            # synthetic per-construct + real-world fixtures
│   ├── README.md                        # anonymization rules
│   ├── configs/{minimal,kitchensink}/   # full HA dirs (anonymized)
│   ├── mappers/<integration>/           # per-mapper YAML in / Pkl expected out
│   └── jinja/<case>/                    # per-construct Jinja in / Starlark expected out
│
└── integration_test.go                  # //go:build integration

internal/cli/
├── cmd_import.go                        # `gohome import-ha` cobra command
└── styles_import.go                     # lipgloss styles for progress + summary
```

### 3.3 Data flow contract

- `Scanner.Scan(haDir)` returns a typed `ScannedConfig` listing classified files (no I/O on contents yet).
- `yaml_loader.Load(scanned)` returns a typed `HAModel` (areas, zones, entities, devices, integrations, automations, scripts, scenes, users, persons, secrets) — everything resolved through `!include` / `!secret` etc.
- Per-area handlers consume `HAModel` slices and return their respective `ImportResult` fragments. Every handler also receives the `*diagnostics.Collector` and adds to it as needed.
- The top-level `Run` composes fragments into a single `ImportResult` tree (`main.pkl` + `drivers.pkl` + … + `IMPORT_REPORT.md` source data).
- `writer.WriteAll(outDir, *ImportResult)` materializes the tree to disk and renders the diagnostics report last.

The key affordance: every handler returns in-memory results, never touches the filesystem itself. Tests construct `HAModel` fixtures directly without scanner/loader involvement and assert handler output without writer involvement.

### 3.4 Public contracts introduced by C11

1. **`Mapper` interface** in `internal/importer/mappers/mapper.go` — the plug-in shape every per-integration mapper implements. Stable across follow-on driver-mapper PRs.
2. **`ImportOptions` struct** — the inputs `Run()` accepts. Stable for downstream tooling that may invoke the importer programmatically.
3. **`Diagnostic` shape** — collector format used by every package. Drives both inline emission and report rendering.
4. **`IMPORT_REPORT.md` schema** — documented section structure (§12). Users may write tooling that parses it; treat as semi-stable.

Internal Pkl emission helpers, scanner classifier, YAML loader, etc., are all internal-only and free to refactor.

### 3.5 Process boundaries

- **No daemon required.** `gohome import-ha` runs standalone; no `gohomed` socket, no Connect-RPC, no Carport. Pure file-IO + transformation.
- **No network calls.** Importer does not pull driver Pkl manifests from a registry; it knows which drivers exist by checking compile-time-baked metadata (the mapper catalog in `mappers/catalog.go`).
- **No secrets in transit.** Reads `secrets.yaml` from the source HA dir; writes `IMPORTED_SECRETS.env` to the output dir; never logs values, never sends them anywhere.
- **No HA daemon required.** The source HA dir is read filesystem-only; HA need not be running. Useful for users importing from a backup snapshot.

---

## 4. CLI Surface

### 4.1 Command shape

```
gohome import-ha [<ha-dir>] -o <out-dir> [flags]
```

Positional `<ha-dir>` defaults to `~/.homeassistant`, then `/config` (HAOS path). If neither exists and no positional given, error with a clear message.

### 4.2 Flags

| Flag | Default | Purpose |
|---|---|---|
| `-o, --out <dir>` | (required) | Output directory for the gohome config tree |
| `--dry-run` | false | Run the full pipeline; emit `IMPORT_REPORT.md` to stdout; write no files |
| `-f, --force` | false | Overwrite a non-empty `<out-dir>` (otherwise refuse) |
| `-v, --verbose` | false | Per-file logging to stderr |
| `-q, --quiet` | false | Errors only |
| `--no-color` | false (auto-detect TTY) | Disable lipgloss color output |

Mutually exclusive: `-v` + `-q`. Validated at flag parse time.

### 4.3 Exit codes

- `0` — success, regardless of FIXMEs / NOTEs (these are expected; user reads the report)
- `1` — hard failure: source dir doesn't exist, output dir non-empty without `--force`, write error, YAML parse error, internal panic
- `2` — flag parse error

### 4.4 Stdout / stderr discipline

- **stdout**: in `--dry-run`, the rendered `IMPORT_REPORT.md`. In normal runs: empty (silent unless `-v` or report path).
- **stderr**: progress + completion summary. Default verbosity prints one line per major phase (`Scanning ~/.homeassistant`, `Loading 14 YAML files`, `Mapping 6 integrations`, `Transpiling 27 automations`, `Writing 31 files to ./my-gohome`, `Done — 8 FIXMEs, 3 NOTEs. See ./my-gohome/IMPORT_REPORT.md`).

### 4.5 Lipgloss styling

Mirrors the existing `gohome` CLI's style conventions (per `internal/cli/styles*.go`).

| Element | Style |
|---|---|
| Phase headings (`Scanning`, `Mapping`, …) | `lipgloss.Bold().Foreground(accent)` |
| Per-file log lines (`-v`) | `lipgloss.Foreground(fgMuted)` |
| Completion summary box | bordered box, title in `Bold()`, FIXME count in `Foreground(warning)` if > 0, NOTE count in `Foreground(fgMuted)` |
| Hard-failure error box | bordered box, red foreground, with a "what went wrong" line and a "what to try" line |
| FIXME-count badge in summary | `lipgloss.Background(warning).Foreground(bg).Padding(0, 1)` if > 0 |
| Report path | `lipgloss.Underline().Foreground(accent)` |

### 4.6 Examples

```
$ gohome import-ha -o ./my-gohome
Scanning ~/.homeassistant
Loading 14 YAML files
Mapping 6 integrations (3 fully mapped, 3 unpublished-driver placeholders)
Transpiling 27 automations
Writing 31 files to ./my-gohome
Done — 8 FIXMEs, 3 NOTEs. See ./my-gohome/IMPORT_REPORT.md
```

```
$ gohome import-ha -o ./my-gohome --dry-run
[full IMPORT_REPORT.md printed to stdout; nothing written]
```

```
$ gohome import-ha -o ./my-gohome
Error: ./my-gohome already exists and contains files.
  What to try: pick a fresh directory, or pass --force to overwrite (this deletes everything in ./my-gohome).
```

---

## 5. Scanner & YAML Loader

### 5.1 Scanner

`internal/importer/scanner.go` walks `<ha-dir>` once and produces a `ScannedConfig`:

```go
type ScannedConfig struct {
    HADir            string
    HAVersion        string                  // from .HA_VERSION if present, "" otherwise

    ConfigurationYAML string                 // path to configuration.yaml
    AutomationsYAML   string                 // empty if not present
    ScriptsYAML       string                 // empty if not present
    ScenesYAML        string                 // empty if not present
    SecretsYAML       string                 // empty if not present
    GroupsYAML        string                 // empty if not present (legacy)
    CustomizeYAML     string                 // empty if not present (legacy)

    AutomationsDir    string                 // automations/ if split-style
    ScriptsDir        string                 // scripts/ if split-style
    PackagesDir       string                 // packages/ if present (modular config)

    StorageAreaRegistry   string             // .storage/core.area_registry
    StorageEntityRegistry string             // .storage/core.entity_registry
    StorageDeviceRegistry string             // .storage/core.device_registry
    StorageConfigEntries  string             // .storage/core.config_entries (UI-installed integrations)
    StorageAuth           string             // .storage/auth
    StorageAuthHA         string             // .storage/auth_provider.homeassistant (password hashes)
    StoragePerson         string             // .storage/person

    LovelacePresent       bool               // we detect it but don't translate
}
```

Classification is path + filename based. The scanner does no I/O on file *contents*; it only confirms each file exists and is readable.

`Scanner.Scan(haDir)` returns `ScannedConfig` plus an inventory of unrecognized files (logged at `-v`, no FIXME).

### 5.2 YAML loader

`internal/importer/yaml_loader.go` consumes a `ScannedConfig` and returns an `HAModel`:

```go
type HAModel struct {
    HAVersion       string
    Configuration   *ConfigurationModel        // from configuration.yaml + packages
    Automations     []AutomationModel          // resolved across .yaml + automations/ + packages
    Scripts         []ScriptModel
    Scenes          []SceneModel
    Areas           []AreaModel                // from .storage/core.area_registry
    Zones           []ZoneModel                // from configuration.yaml zones: section
    Entities        []EntityModel              // from .storage/core.entity_registry
    Devices         []DeviceModel              // from .storage/core.device_registry
    ConfigEntries   []ConfigEntryModel         // from .storage/core.config_entries (UI integrations)
    Users           []UserModel                // from .storage/auth + auth_provider.homeassistant
    Persons         []PersonModel              // from .storage/person
    Secrets         map[string]string          // from secrets.yaml — values held in memory only, never logged
}
```

The loader is a focused unit: read each known file, parse with `yaml.v3` (for YAML) or `encoding/json` (for `.storage/*`), normalize into the model.

### 5.3 HA YAML custom tags

`gopkg.in/yaml.v3` allows custom tag resolution via `Decoder.SetExperimentalLanguage()` / `Unmarshaler` interface. The loader registers handlers for:

| HA tag | Behavior |
|---|---|
| `!include filename` | Inline the contents of `<ha-dir>/<filename>` |
| `!include_dir_list dirname` | Inline a YAML list whose entries are the YAML contents of each file in `<ha-dir>/<dirname>/` (sorted by filename) |
| `!include_dir_merge_list dirname` | Inline a single flat YAML list merged from the lists in each file in the dir |
| `!include_dir_named dirname` | Inline a YAML mapping where keys are stem filenames and values are file contents |
| `!include_dir_merge_named dirname` | Inline a single flat mapping merged from the mappings in each file in the dir |
| `!secret name` | Replace with `secret_marker(name)` in the parsed model — the `secrets/` package later resolves these to `read("env:NAME")` Pkl |
| `!input name` | Used in HA blueprints; for v1.0, emit a `FIXME(blueprint-input)` note since blueprints aren't a v1.0 concept in gohome |

A test fixture under `testdata/yaml_loader/` covers each tag form.

### 5.4 Failure modes

- File present but unreadable → hard error, exit 1
- File present but malformed YAML → hard error with file:line, exit 1
- File present but unexpected schema (e.g., `automations.yaml` is a string instead of a list) → hard error
- File absent: silently skipped (most HA setups don't have all of them)

---

## 6. Per-Integration Mappers

### 6.1 The `Mapper` interface

`internal/importer/mappers/mapper.go`:

```go
package mappers

type Input struct {
    Integration   string                  // e.g., "hue", "mqtt"
    Source        IntegrationSource       // YAML, ConfigEntry, or both
    YAMLBlock     map[string]any          // configuration.yaml block, if any
    ConfigEntries []ha.ConfigEntryModel   // .storage/core.config_entries entries for this integration, if any
}

type IntegrationSource int
const (
    SourceYAML         IntegrationSource = iota + 1   // declared in configuration.yaml
    SourceConfigEntry                                  // installed via UI (.storage)
    SourceBoth                                         // both — merge
)

type Output struct {
    DriverInstances []DriverInstancePkl                // emitted into drivers.pkl
    Imports         []string                            // additional Pkl imports needed
    Diagnostics     []diagnostics.Diagnostic            // any FIXMEs / NOTEs this mapper produced
}

type Mapper interface {
    Integration() string                                // matches HA integration domain ("hue", "mqtt", ...)
    DriverModuleAvailable() bool                        // whether @drivers/<x>.pkl is published
    Map(in Input, c *diagnostics.Collector) Output
}
```

Each mapper file defines one type implementing this interface and registers itself in `catalog.go` via an `init()` function.

### 6.2 Catalog

`internal/importer/mappers/catalog.go`:

```go
var registry = map[string]Mapper{}

func Register(m Mapper) {
    if _, dup := registry[m.Integration()]; dup {
        panic("duplicate mapper: " + m.Integration())
    }
    registry[m.Integration()] = m
}

func Lookup(integration string) (Mapper, bool) {
    m, ok := registry[integration]
    return m, ok
}

func All() []Mapper { /* sorted by integration name */ }
```

The catalog is a closed set baked at compile time. New mappers = new files + `init()`-time `Register()` calls.

### 6.3 v1.0 target list

The eleven integrations from master design §8.1:

| Integration | Mapper file | Driver module | Status (placeholder — confirmed at C11 plan time) |
|---|---|---|---|
| `mqtt` | `mappers/mqtt.go` | `@drivers/mqtt.pkl` | TBC |
| `zigbee2mqtt` | `mappers/zigbee2mqtt.go` | `@drivers/zigbee2mqtt.pkl` | TBC |
| `esphome` | `mappers/esphome.go` | `@drivers/esphome.pkl` | TBC |
| `homekit` | `mappers/homekit.go` | `@drivers/homekit.pkl` | TBC |
| `matter` | `mappers/matter.go` | `@drivers/matter.pkl` | TBC |
| `hue` | `mappers/hue.go` | `@drivers/hue.pkl` | TBC |
| `nest` | `mappers/nest.go` | `@drivers/nest.pkl` | TBC |
| `zwave_js` | `mappers/zwave_js.go` | `@drivers/zwave_js.pkl` | TBC |
| `rest` | `mappers/rest.go` | `@drivers/rest.pkl` | TBC |
| `webhook` | `mappers/webhook.go` | `@drivers/webhook.pkl` | TBC |
| `template` | `mappers/template.go` | (in-tree, maps to `gohome.entities.ComputedEntity`) | always available |

The "Status" column is filled in by the C11 implementation plan at planning time, by inventorying which driver Pkl modules are published to `@drivers/*` at that moment.

### 6.4 Mapper shipping discipline

C11's implementation PR ships the importer engine + every component below the `mappers/` line + mappers for whichever drivers in the v1.0 list are published at planning time + the `unmapped.go` generic emitter + the `template.go` mapper (always available).

For unpublished v1.0-list drivers, the `unmapped.go` emitter handles them with a more informative variant of its diagnostic:

```pkl
// FIXME(unpublished-driver): integration 'hue' is in the gohome v1.0 driver set,
// but the @drivers/hue.pkl manifest was not published when this importer was built.
// A hue mapper will land in a follow-on PR once the driver Pkl is available.
// Original HA configuration preserved below — apply it manually once the driver lands:
//
//   <yaml-original>
new gohome.imported.UnmappedIntegration {
  sourceName = "hue"
  sourceConfigYaml = #"""
  <yaml-original>
  """#
}
```

When a new driver Pkl is published, a follow-on PR adds the corresponding mapper file, registers it in the catalog, adds tests, and removes the unpublished-driver FIXME for that integration. Each follow-on is a small, focused PR.

For HA integrations entirely outside the v1.0 driver set, the `unmapped.go` emitter uses its baseline diagnostic:

```pkl
// FIXME(unmapped-integration): integration 'sonoff_lan' is outside gohome's v1.0
// driver set. No mapper exists. Original HA configuration preserved below:
//
//   <yaml-original>
new gohome.imported.UnmappedIntegration { sourceName = "sonoff_lan"; ... }
```

### 6.5 Mapper depth — what "deep" means concretely

For each shipped mapper, the C11 implementation must:

- Translate every well-documented HA field for that integration to the matching gohome `instanceConfig` field. Where the gohome field has a different name, default, or type, the mapper handles the translation explicitly.
- Pick reasonable defaults for gohome fields HA doesn't expose. Defaults documented in a per-mapper comment.
- Add a `NOTE(mapper-default)` diagnostic for any non-trivial default chosen ("HA didn't specify pollIntervalSeconds; using gohome default of 30").
- Validate inputs locally (e.g., the Hue mapper checks that `bridge` looks like an IP or hostname); on failure, emit a `FIXME(mapper-input-invalid)` rather than producing broken Pkl.
- Have at least four tests: minimal config, full config, missing-required-field (asserting the FIXME), extra-unknown-field (asserting it's preserved as a NOTE rather than silently dropped).

### 6.6 Multi-instance integrations

HA allows multiple instances of an integration (e.g., two Hue bridges). The mapper handles both single-instance (YAML block is a mapping) and multi-instance (YAML block is a list of mappings) forms. Each emitted `DriverInstancePkl` carries a unique slug derived from `<integration>_<index_or_name>` (e.g., `hue_main`, `hue_basement`).

For UI-installed integrations (in `.storage/core.config_entries`), the mapper receives one or more `ConfigEntryModel` entries and produces one driver instance per entry. The `Source` field of `Input` distinguishes YAML vs ConfigEntry vs Both.

---

## 7. Jinja Transpiler

### 7.1 Supported constructs

| Category | Covered | Form |
|---|---|---|
| State access | ✓ | `states('x')`, `state_attr('x', 'a')`, `is_state('x', v)`, `is_state_attr('x', 'a', v)`, `has_value('x')` |
| Type coercion | ✓ | `\| float`, `\| int`, `\| bool`, `\| string` |
| Math | ✓ | `\| min`, `\| max`, `\| sum`, `\| average`, `\| round(n)`, `\| abs` |
| Strings | ✓ | `\| default(v)`, `\| length`, basic `.format()` |
| Time | ✓ | `now()`, `utcnow()`, `as_datetime`, `as_timestamp`, `today_at()` |
| Logical | ✓ | `iif`, `not`, `and`, `or`, `==`, `!=`, `<`, `>`, `<=`, `>=`, `in` |
| Lists | ✓ | `\| selectattr('field', 'eq', v)`, `\| rejectattr('field', 'eq', v)` (basic forms only — `==`/`in` operators) |
| Control flow | ✓ | `{% if %}` / `{% elif %}` / `{% else %}` / `{% endif %}`, `{% for x in y %}` / `{% endfor %}`, `{% set x = v %}` |
| Area expansion | ✗ | `expand(group)`, `area_entities(area_id)`, `device_entities(device_id)`, `area_id(entity_id)`, `area_name(area_id)` → `FIXME(jinja-import)` |
| Geographic | ✗ | `closest()`, `distance()` → `FIXME(jinja-import)` |
| Macros | ✗ | `{% macro %}` / `{% endmacro %}` → `FIXME(jinja-import)` |
| HACS custom filters | ✗ | Anything not in the above list → `FIXME(jinja-import)` |
| Includes / imports | ✗ | `{% include %}`, `{% import %}` → `FIXME(jinja-import)` |
| Custom tests | ✗ | `is foo` for non-stdlib `foo` → `FIXME(jinja-import)` |

### 7.2 Implementation

Built on `github.com/nikolalohinski/gonja` v2 (the maintained Go port of Jinja2). The transpiler:

1. Parses the input template via `gonja.FromString(...)` — gives an AST.
2. Walks the AST with a custom visitor (`internal/importer/jinja/visitor.go`).
3. For each AST node type, the visitor either emits Starlark or registers a `FIXME(jinja-import)` diagnostic and emits a placeholder.
4. Returns the assembled Starlark string + any collected diagnostics.

### 7.3 FIXME format

When a construct can't be transpiled, the visitor emits the Starlark placeholder:

```python
# FIXME(jinja-import): unmapped construct
#   Original Jinja: {{ closest('zone.home').name }}
#   At: automations/lighting.pkl:42 (automation 'arrival_lights')
result = None  # placeholder; replace with equivalent Starlark
```

The placeholder is `result = None` for expression contexts and `pass  # FIXME` for statement contexts. The generated `.star` file always parses cleanly; the user's job is to replace the placeholder with real logic.

### 7.4 Cross-cutting helpers

The transpiled Starlark relies on a small set of helpers exposed by the gohome Starlark stdlib (per C5). The transpiler emits:

| Jinja | Starlark |
|---|---|
| `states('light.kitchen')` | `state('light.kitchen')` |
| `states('x').state` | `state('x').value` |
| `state_attr('x', 'brightness')` | `state('x').attr('brightness')` |
| `is_state('x', 'on')` | `state('x').value == 'on'` |
| `is_state_attr('x', 'a', v)` | `state('x').attr('a') == v` |
| `has_value('x')` | `state('x') != None` |
| `now()` | `time.now()` |
| `utcnow()` | `time.utcnow()` |
| `as_datetime(v)` | `time.parse(v)` |
| `as_timestamp(v)` | `time.timestamp(v)` |
| `today_at(hh:mm)` | `time.today_at(hh, mm)` |
| `iif(cond, t, f)` | `(t if cond else f)` |
| `... \| float` | `float(...)` |
| `... \| int` | `int(...)` |
| `... \| default(v)` | `(... if ... != None else v)` |
| `... \| min` | `min(...)` |
| `... \| length` | `len(...)` |
| `... \| selectattr('a', 'eq', v)` | `[x for x in ... if x.a == v]` |
| `{% if c %}...{% else %}...{% endif %}` | `... if c else ...` (in expression) or `if c: ...; else: ...` (in statement) |
| `{% for x in y %}...{% endfor %}` | `for x in y: ...` (statement form only — no Jinja-in-Jinja loops in expression position) |
| `{% set x = v %}` | `x = v` |

If C5's actual stdlib API differs (e.g., `state(x)` vs `entity(x)`), the transpiler's `stdlib.go` is the single place to update — every emit goes through it.

### 7.5 Test coverage

`internal/importer/jinja/transpile_test.go` is table-driven. Each row:

```go
{
    name: "is_state_eq",
    input: "{{ is_state('light.kitchen', 'on') }}",
    want:  "(state('light.kitchen').value == 'on')",
    diags: nil,
},
{
    name: "fixme_closest",
    input: "{{ closest('zone.home').name }}",
    want:  "# FIXME(jinja-import): unmapped construct\n#   Original Jinja: ...\nresult = None",
    diags: []DiagnosticKind{JinjaUnmapped},
},
```

One row per construct in the §7.1 covered table, one row per FIXME case, plus at least four edge-case rows: nested expressions, multiple constructs in one template, whitespace handling, and HA's `{{- ... -}}` whitespace-trim syntax.

---

## 8. Registry Translation

`internal/importer/registry/` translates HA's `.storage/core.area_registry`, `.storage/core.entity_registry`, `.storage/core.device_registry` JSON files into gohome Pkl.

### 8.1 Areas

`.storage/core.area_registry` → `areas.pkl`:

```pkl
amends "@gohome/config.pkl"
import "@gohome/areas.pkl" as a

areas: Listing<a.Area> = new {
  new { id = "kitchen"; name = "Kitchen"; parent = null }
  new { id = "living_room"; name = "Living Room"; parent = null }
  new { id = "main_floor"; name = "Main Floor"; parent = null }
  new { id = "kitchen_in_main"; name = "Kitchen"; parent = "main_floor" }   // if HA had area hierarchy via labels
}
```

HA areas are flat by default. If the user uses the labels feature (HA 2024+) to imply hierarchy, the importer detects common conventions and writes Pkl `parent` accordingly. Otherwise all areas are top-level. NOTE diagnostic if hierarchy detection is heuristic.

### 8.2 Zones

Zones live in `configuration.yaml` under `zone:`. Mapped 1:1 to `zones.pkl`:

```pkl
amends "@gohome/config.pkl"
import "@gohome/zones.pkl" as z

zones: Listing<z.Zone> = new {
  new { id = "home"; name = "Home"; latitude = 42.36; longitude = -71.06; radiusMeters = 100 }
}
```

### 8.3 Entity registry

`.storage/core.entity_registry` → `entities/overrides.pkl`. The override file contains user-customized fields (friendly name, area assignment, icon, hidden flag) — actual entity *existence* comes from drivers, not from this file. Pkl shape matches whatever `gohome.entities.Override` C4 settled on.

### 8.4 Device registry

`.storage/core.device_registry` → diagnostic only. gohome's device concept is driver-managed (devices come from Carport's `RegisterInstance`); the importer does not write a Pkl file for devices but does emit a NOTE in the report counting how many devices were detected so the user can verify against the post-driver-load registry.

### 8.5 Computed entities

HA's `template:` platform → gohome `ComputedEntity` Pkl in `entities/computed.pkl`. Each template sensor / binary_sensor becomes one `ComputedEntity` with the Jinja transpiled to Starlark per §7. Handled by `mappers/template.go` (it's a mapper, not a registry handler — it lives in mappers/ for code organization but writes to `entities/computed.pkl` not `drivers.pkl`).

---

## 9. Automations, Scripts & Scenes

### 9.1 Automations

Source: `automations.yaml` (legacy single-file form) or `automations/*.yaml` (split form) or both. The loader merges and normalizes.

Output: one `automations/<slug>.pkl` per automation, plus a `automations/handlers/<slug>.star` for any non-trivial action body. Pkl declares triggers/conditions/actions structurally; `.star` holds the body when actions are sequences of templated logic.

Trigger translation table:

| HA trigger platform | gohome trigger | Notes |
|---|---|---|
| `state` | `state_changed` (per C6 spec) | direct mapping |
| `numeric_state` | `state_changed` + condition | wraps in a condition for the numeric threshold |
| `time` | `time` | direct |
| `time_pattern` | `time` (cron form) | direct mapping; HA's time_pattern is cron-equivalent |
| `template` | `state_changed` + Starlark condition | Jinja → Starlark in the condition |
| `event` | `event` | direct |
| `homeassistant` (start/shutdown) | `event` (kind=`system_started`/`system_stopping`) | direct |
| `mqtt` | `event` (kind=`driver:mqtt`, payload filter) | NOTE: requires the MQTT driver |
| `webhook` | `event` (kind=`driver:webhook`) | NOTE: requires the webhook driver |
| `sun` | `event` (kind=`sun:rise`/`sun:set`) | NOTE: per C6, sun is a driver in v1.0 |
| `geo_location`, `zone` | `event` | depends on a presence driver |
| Others | FIXME(unmapped-trigger) | |

Action translation table:

| HA action | gohome action | Notes |
|---|---|---|
| `service: light.turn_on` | `entity.<id>.turn_on(...)` capability call | direct |
| `service: <domain>.<svc>` | `entity.<id>.<svc>(...)` if entity selector present, else FIXME | |
| `delay: ...` | `sleep(...)` | direct |
| `wait_template: ...` | `wait_until(starlark expression)` | Jinja → Starlark |
| `condition: ...` (mid-action) | mid-action `if not (...): return` | direct |
| `repeat: ...` | `for ... in range(...)` or `while ...` | depends on shape |
| `choose: ...` | `if/elif/else` chain | direct |
| `parallel: ...` | FIXME(unmapped-action) | gohome has no parallel action in v1.0 |
| `event: ...` | `event.fire(...)` | direct |
| `scene: ...` | `scene.apply(...)` | direct |
| `script: ...` | `script.<name>(...)` | direct |

Conditions translate similarly to triggers (state, numeric_state, time, template, sun).

### 9.2 Scripts

Source: `scripts.yaml` or `scripts/*.yaml`. Each script becomes a `scripts/<slug>.pkl` (declares parameters, callable name) plus `scripts/bodies/<slug>.star` for the body.

Script body action translation reuses the same table as automations.

### 9.3 Scenes

Source: `scenes.yaml`. Each scene becomes one entry in `scenes.pkl`:

```pkl
amends "@gohome/config.pkl"
import "@gohome/scenes.pkl" as s

scenes: Listing<s.Scene> = new {
  new {
    id = "movie_time"
    name = "Movie Time"
    targets = new {
      new { entityId = "light.living_room"; state = "on"; brightness = 30 }
      new { entityId = "light.kitchen"; state = "off" }
    }
  }
}
```

Scenes are pure data; no Jinja, no FIXME concerns under normal conditions.

---

## 10. Auth, Users & Persons

### 10.1 Users

`.storage/auth` lists users; `.storage/auth_provider.homeassistant` holds password hashes. The importer:

- Translates each user to a `gohome.auth.User` entry in `auth/users.pkl`.
- Sets `displayName`, `roles` (HA's `is_owner` + `is_active` flags map to `admin` / `member`).
- **Does not migrate password hashes.** HA uses bcrypt with HA-specific salting; gohome (per C9) uses Argon2id with its own format. Migrating across hash schemes is unsafe and was explicitly noted in the master design.
- Emits a `NOTE(password-not-migrated)` per user whose HA account had a password.
- Adds an actionable line to `IMPORT_REPORT.md`: "Re-register passkeys via `gohome auth bootstrap <slug>` for each user."

### 10.2 Persons

`.storage/person` holds HA's "person" entities (linked to user accounts + device trackers). gohome's user concept doesn't include device-tracker association in v1.0 (presence is per-driver). The importer:

- For each person whose `user_id` matches a User, adds the person's name to the User's `displayName` if HA had one set.
- Emits a `NOTE(person-tracker-not-migrated)` for each person with associated `device_trackers` — the tracker drivers will surface presence entities once installed; the user wires automation to those entities themselves in v1.0.

### 10.3 Roles & Policies

HA does not have a Pkl-equivalent role/policy system; its permissions are flat. The importer creates a default `auth/roles.pkl` and `auth/policies.pkl` containing only the gohome-built-in `admin` / `member` / `guest` roles and a permissive policy that mirrors HA's "everyone can do everything in their scope" default. Users tighten policies post-import via the C9 Pkl shapes.

NOTE in the report: "Default permissive policies imported. Refine in `auth/policies.pkl` per your household's needs."

---

## 11. Secrets Pipeline

### 11.1 Reading

`secrets.yaml` is parsed by the YAML loader into `HAModel.Secrets` (`map[string]string`). Values are held in memory only; never logged, never sent to stderr at any verbosity.

### 11.2 Pkl emission

`secrets.pkl` is emitted with one entry per secret, all wrapped via `read("env:UPPER_SNAKE_CASE")`:

```pkl
amends "@gohome/config.pkl"
import "@gohome/base.pkl" as b

secrets {
  hue_api_key   = read("env:HUE_API_KEY")
  mqtt_password = read("env:MQTT_PASSWORD")
  // ...
}
```

Where the original HA secret name is `hue_api_key`, the env var name is its upper-snake-case form. Where two HA secret names would collide after upper-casing (extremely rare), the importer suffixes with `_2`, `_3`, … and emits a `NOTE(secret-collision)`.

References elsewhere — wherever the importer would have written a literal value but the YAML used `!secret xyz` — instead write `secrets.xyz` with the proper Pkl reference.

### 11.3 Side file: `IMPORTED_SECRETS.env`

Written to the output dir root:

```sh
# IMPORTED_SECRETS.env — generated by gohome import-ha on 2026-04-26
#
# These are the actual secret values pulled from your HA secrets.yaml.
# They live OUTSIDE your committed Pkl config. To use them with gohomed:
#
#   1. Export them into your shell or systemd unit:
#        set -a && source ./IMPORTED_SECRETS.env && set +a
#      (or convert to a systemd `EnvironmentFile=` directive)
#   2. Once your gohomed is running and reading them via secrets.pkl,
#      DELETE THIS FILE so the values stop existing on disk:
#        rm ./IMPORTED_SECRETS.env

export HUE_API_KEY="abc123def456"
export MQTT_PASSWORD="redacted-original-value"
# ...
```

### 11.4 `.gitignore` discipline

The importer writes a `.gitignore` to the output dir root with at minimum:

```
IMPORTED_SECRETS.env
```

If the user wants additional ignores, they edit it post-import. The file is part of the importer's output by design — the user is *strongly* encouraged to `git init && git add .` immediately after import, and the imported `.gitignore` is the safety net against accidentally committing secrets on that first commit.

A NOTE in the report explicitly tells the user to delete `IMPORTED_SECRETS.env` after sourcing it; the report's "What to do next" section repeats it as step 1.

---

## 12. Diagnostics & `IMPORT_REPORT.md`

### 12.1 Diagnostic shape

```go
type Diagnostic struct {
    Severity Severity                  // FIXME or NOTE
    Reason   Reason                    // closed enum: see below
    File     string                    // output file path (for FIXMEs); source file path (for NOTEs about source-side issues)
    Line     int                       // 0 if not applicable
    Message  string                    // single-line human description
    Detail   string                    // optional multi-line detail (e.g., original Jinja)
}

type Severity int
const (
    SeverityFIXME Severity = iota + 1   // action required
    SeverityNOTE                         // informational
)

type Reason string
const (
    ReasonJinjaImport         Reason = "jinja-import"           // FIXME
    ReasonUnmappedIntegration Reason = "unmapped-integration"   // FIXME
    ReasonUnpublishedDriver   Reason = "unpublished-driver"     // FIXME
    ReasonMapperInputInvalid  Reason = "mapper-input-invalid"   // FIXME
    ReasonUnmappedTrigger     Reason = "unmapped-trigger"       // FIXME
    ReasonUnmappedAction      Reason = "unmapped-action"        // FIXME
    ReasonBlueprintInput      Reason = "blueprint-input"        // FIXME

    ReasonMapperDefault       Reason = "mapper-default"           // NOTE
    ReasonSecretNotCopied     Reason = "secret-not-copied"        // NOTE
    ReasonPasswordNotMigrated Reason = "password-not-migrated"    // NOTE
    ReasonSecretCollision     Reason = "secret-collision"         // NOTE
    ReasonAreaHierarchyHeuristic Reason = "area-hierarchy-heuristic"  // NOTE
    ReasonPersonTrackerNotMigrated Reason = "person-tracker-not-migrated"  // NOTE
    ReasonExtraUnknownField   Reason = "extra-unknown-field"      // NOTE
)
```

The `Reason` enum is closed — adding a reason means a code change. This keeps the report's tabular sections grep-able and ensures we don't silently grow categories.

### 12.2 Inline emission

When a handler/mapper/transpiler adds a FIXME or NOTE, it also emits an inline comment in the output Pkl/`.star` file at the appropriate location:

```pkl
// FIXME(unmapped-integration): integration 'sonoff_lan' is outside gohome's
// v1.0 driver set. No mapper exists. Original HA configuration preserved below.
```

```python
# FIXME(jinja-import): unmapped construct
#   Original Jinja: {{ closest('zone.home').name }}
#   At: automations/lighting.pkl:42 (automation 'arrival_lights')
result = None  # placeholder; replace with equivalent Starlark
```

```pkl
// NOTE(secret-not-copied): the value of 'mqtt_password' was written to
// IMPORTED_SECRETS.env. Source that file to populate the env var, then
// delete it.
```

### 12.3 `IMPORT_REPORT.md` structure

Per the format settled in Q5 of the brainstorm. Concretely (template):

```markdown
# gohome import report

Imported from `<source HA dir>` on `<date>`.
HA version detected: `<version or "unknown">`.

## Summary
- Areas: N (M mapped, K unmapped)
- Zones: N
- Driver instances: N across M integrations (X fully mapped, Y unpublished-driver placeholders, Z unmapped)
- Entities (registry overrides): N
- Computed entities (template platform): N
- Automations: N (M fully transpiled, K with FIXMEs)
- Scripts: N (M fully transpiled, K with FIXMEs)
- Scenes: N
- Users: N (passwords NOT migrated — passkey re-registration required)
- Persons: N
- Secrets: N (values written to IMPORTED_SECRETS.env)

## What to do next
1. Source secrets and delete the side file:
       set -a && source ./IMPORTED_SECRETS.env && set +a
       rm ./IMPORTED_SECRETS.env
2. Install required drivers:
       gohome driver install ghcr.io/gohome/driver-mqtt:v1
       gohome driver install ghcr.io/gohome/driver-hue:v1
       (etc.)
3. Resolve open FIXMEs (search for `FIXME(`).
4. Run `gohome config validate`.
5. Re-register passkeys for each user via `gohome auth bootstrap <slug>`.

## Per-integration detail
### <integration> (`<source>`)
- Driver: `@drivers/<name>.pkl` (status: published / NOT YET PUBLISHED / out of v1.0 set)
- N instance(s)
- M FIXMEs / K NOTEs

(repeat per integration)

## Open FIXMEs (N)
| Reason | File:line | Message |
|---|---|---|
| jinja-import | automations/lighting.pkl:42 | `closest('zone.home').name` not transpilable |
| ... |

## Notes (M)
| Reason | File:line | Message |
|---|---|---|
| secret-not-copied | secrets.pkl:12 | mqtt_password value in IMPORTED_SECRETS.env |
| ... |
```

The renderer (`internal/importer/diagnostics/report.go`) takes a `*Collector` + a per-integration summary struct from each mapper's output and emits this Markdown.

### 12.4 Stderr summary line

At completion, the CLI prints a one-line summary (or short multi-line block) to stderr:

```
Done — 8 FIXMEs, 3 NOTEs across 31 generated files. See ./my-gohome/IMPORT_REPORT.md.
```

Lipgloss-styled per §4.5.

---

## 13. Writer & Output Layout

### 13.1 Output directory layout

Matches master design §6.5:

```
<out-dir>/
├── main.pkl                        # root module — imports the rest
├── settings.pkl                    # core HA settings translated
├── drivers.pkl                     # driver instances (one or more per integration)
├── areas.pkl                       # area registry → gohome areas
├── zones.pkl                       # zones from configuration.yaml
├── entities/
│   ├── overrides.pkl               # entity registry overrides
│   └── computed.pkl                # template platform → ComputedEntity
├── automations/
│   ├── <slug>.pkl                  # one per imported automation
│   └── handlers/
│       └── <slug>.star             # body of any non-trivial automation action
├── scripts/
│   ├── <slug>.pkl
│   └── bodies/
│       └── <slug>.star
├── scenes.pkl
├── auth/
│   ├── users.pkl
│   ├── roles.pkl                   # gohome built-ins only by default
│   └── policies.pkl                # permissive default; user tightens
├── secrets.pkl                     # references via read("env:...")
├── IMPORTED_SECRETS.env            # actual values; .gitignore'd
├── .gitignore                      # at minimum, IMPORTED_SECRETS.env
└── IMPORT_REPORT.md                # diagnostics report
```

### 13.2 Pkl emission

`internal/importer/writer/pkl_print.go` defines small `text/template`-based emitters for each output file type. Examples:

- `EmitDriversPkl(ws []DriverInstancePkl) string`
- `EmitAreasPkl(as []AreaModel) string`
- `EmitAutomation(a AutomationModel, jinjaResults map[string]string) string`
- `EmitSecretsPkl(secrets map[string]string) string`
- `EmitImportedSecretsEnv(secrets map[string]string) string`
- `EmitGitignore() string`

Each emitter produces canonical output (deterministic indentation, alphabetical key ordering where order doesn't matter, preserved order where it does — e.g., automation actions). Unit tests assert byte-equal output against goldens.

The emitters are deliberately *not* a general-purpose Pkl AST emitter — they're focused string templates. Refactoring to an AST emitter is a follow-on if/when it provides clear value.

### 13.3 Write semantics

`writer.WriteAll(outDir, *ImportResult)`:

1. Verify `outDir` doesn't exist OR is empty OR `--force` is set.
2. Create `outDir` if needed.
3. For each file in `*ImportResult`, write atomically (write to `<path>.tmp`, rename to `<path>`).
4. Render `IMPORT_REPORT.md` last (it depends on the diagnostics collected during all prior emits).
5. Set conservative permissions: `0644` for files, `0755` for dirs, `0600` for `IMPORTED_SECRETS.env`.

### 13.4 `--dry-run` mode

In dry-run, the writer is bypassed entirely. The CLI renders only `IMPORT_REPORT.md` to stdout and exits. No `outDir` is touched (it doesn't even need to exist).

---

## 14. Testing Strategy

### 14.1 Layers

| Layer | Tool | What it covers |
|---|---|---|
| Per-mapper unit | `testing` table-driven | Each integration mapper: input HA YAML fragment → expected output Pkl byte-equal; minimal/full/missing-required/extra-unknown configurations |
| Jinja transpiler | `testing` golden table | One row per supported construct (positive); one row per FIXME case; edge cases (nesting, whitespace, multiple constructs in one template, `{{- -}}` whitespace trim) |
| Per-area handler | `testing` table-driven | `core/`, `registry/`, `automations/`, `scripts/`, `scenes/`, `auth/`, `secrets/` each have unit tests for their normalization + emission logic |
| YAML loader | `testing` against fixture files | One fixture per HA custom tag; error cases (malformed include, missing secret, recursive include) |
| Diagnostics report | Golden | Given a known `Collector` snapshot, assert `IMPORT_REPORT.md` matches expected sections |
| End-to-end | `//go:build integration` | Run `Run()` against a fixture HA dir; assert output files exist with expected content; assert `gohome config validate` passes |

### 14.2 Fixture strategy

**Synthetic fixtures** under `internal/importer/testdata/`:

- `mappers/<integration>/{minimal,full,missing,extra}/in.yaml` + `out.pkl`
- `jinja/<construct>/in.txt` + `out.star`
- `yaml_loader/<tag>/in/` (a small dir tree) + `out.json` (the resolved model)
- `diagnostics/<scenario>/in.json` (collector snapshot) + `out.md`

**Real-world fixtures** under `internal/importer/testdata/configs/`:

- `minimal/` — 2 integrations (MQTT + template), 3 automations, 1 scene, 1 user. Sanity check that the pipeline composes cleanly.
- `kitchensink/` — all eleven v1.0 integrations, ~50 automations exercising every supported Jinja construct, ~10 scripts, ~5 scenes, areas with hierarchy heuristics, multiple users with mixed credential types, `secrets.yaml` with ~20 secrets, `packages/` directory. Composition stress test.

Both real-world fixtures are anonymized once; the README documents the rules.

### 14.3 Anonymization rules (`internal/importer/testdata/README.md`)

- Entity friendly names → `Entity 1`, `Entity 2`, …
- IPs → RFC 5737 (`192.0.2.x`, `198.51.100.x`, `203.0.113.x`)
- Hostnames → `host1.example.invalid`
- Usernames → `user1`, `user2`, …
- Passwords / API tokens → `redacted-by-test-fixture`
- MAC addresses → `00:11:22:33:44:NN`
- Lat/lon → `0.0`, `0.0` with radius reduced to 100m
- HA `installation_id`, `uuid` fields → fixed sentinel value `00000000-0000-0000-0000-000000000000`

Performed manually when adding a fixture; not automated.

### 14.4 Coverage targets for CI

- Every `Reason` in the diagnostics enum has at least one test asserting its emit format.
- Every Jinja construct in §7.1's covered table has at least one positive test.
- Every shipped mapper has all four standard tests (minimal/full/missing/extra).
- Both real-world fixture outputs validate via `gohome config validate` in CI.
- `--dry-run` exits 0 against both fixtures; `--force` succeeds with a pre-existing non-empty output dir.

### 14.5 What's NOT tested

- Performance / scaling. Real HA configs in v1.0 are bounded (~hundreds of automations, ~thousands of entities). Performance benchmarks deferred.
- Cross-platform path handling beyond Linux/macOS (Windows is a v1.x concern for the importer; gohomed and `gohome` ship Windows binaries per master design §8.2 but the importer's target user almost always runs HA on Linux).
- Concurrent runs (the importer is single-threaded; no concurrent-run safety needed).

---

## 15. Implementation Order

Engineered to make every increment a working, demonstrable improvement.

1. **Top-level `gohome import-ha` cobra scaffold** (`internal/cli/cmd_import.go`, `styles_import.go`). Returns `Code.Unimplemented`-equivalent error. CI green.
2. **`Scanner` + `yaml_loader`** with no handlers wired. End-to-end test reads fixture HA dir → asserts `HAModel` correctness.
3. **`diagnostics/Collector` + `report.go`** with stub data. Assert `IMPORT_REPORT.md` renders correctly against a hand-built collector snapshot.
4. **`writer/` skeleton** + `pkl_print.go` with the trivial emitters (`Gitignore`, `ImportedSecretsEnv`). Assert atomic write semantics + permission discipline.
5. **`secrets/` package** end-to-end. Read `secrets.yaml` → emit `secrets.pkl` + `IMPORTED_SECRETS.env`.
6. **`registry/` package**: areas, zones, entity overrides. Emit Pkl. Hierarchy heuristic optional.
7. **`core/` package**: configuration.yaml → main.pkl + settings.pkl.
8. **`auth/` package**: users + persons → `auth/users.pkl` + roles + policies stubs.
9. **`mappers/` framework**: `Mapper` interface, `catalog.go`, `unmapped.go` generic emitter. No real mappers yet; everything goes through `unmapped.go` and produces FIXME(unmapped-integration) blocks.
10. **`mappers/template.go`** — the always-available template-platform mapper. Wires through to `entities/computed.pkl`. Also exercises the Jinja transpiler dependency (next).
11. **`jinja/` package**: `transpile.go`, `visitor.go`, `stdlib.go`. Implement against the §7.1 table. Golden table tests pass for every covered + uncovered construct.
12. **First real driver mapper** (whichever v1.0 driver Pkl ships first — likely MQTT). Full four-test coverage.
13. **Subsequent mappers** — one PR per mapper as drivers land. Each PR adds the mapper file, registers in catalog, adds tests, removes the unpublished-driver FIXME case for that integration.
14. **`scenes/` package**. Pure data; lowest complexity.
15. **`scripts/` package**. Includes Jinja transpilation in bodies.
16. **`automations/` package**. Triggers + conditions + actions tables; Jinja transpilation. Largest single area handler.
17. **End-to-end integration test** against the `minimal/` fixture. Assert output files match goldens; assert `gohome config validate` passes.
18. **Real-world `kitchensink/` fixture** + integration test. Asserts composition. Surfaces real edge cases that the per-area unit tests miss.
19. **CLI polish**: lipgloss styling, `-v` / `-q` / `--no-color`, error message UX for the "output dir non-empty" case.
20. **Documentation pass**: `docs/import-ha.md` user-facing guide; update `README.md`.
21. **Final `task lint && task test && task test:race && task test:integration` pass**.

Each numbered item is a single PR or a tight commit series. Items 1–9 ship a working "scan + report (no mappings)" tool — useful in itself for users discovering what's in their HA config. Items 10+ progressively turn FIXMEs into real mapped output.

---

## 16. Decision Record

| # | Decision | Alternatives considered | Reason |
|---|---|---|---|
| 1 | C11 = importer + Jinja transpiler; Lovelace deferred to a separate milestone | All in one C11 (option A) / minimal Lovelace pass (option C) | Lovelace is its own design problem; depends on C10 widget set being battle-tested; would dwarf the rest if bundled |
| 2 | Single-shot import with `--force` for overwrite | Repeatable / merge-mode | Merge-mode UX cost is huge; "git-initable directory" framing implies single-shot; users who need re-import delete and re-run |
| 3 | Auto-detect source: `~/.homeassistant` then `/config` then error | Always require explicit path | Light convenience; bounded scope; no broad search |
| 4 | Secrets to side file (`IMPORTED_SECRETS.env`) | Stderr / refuse / Pkl-embedded | Side file with `.gitignore` guard is the only path that keeps git clean while giving users a clear migration handle |
| 5 | Deep per-integration mappers, follow-on PRs per driver | Shallow generic placeholders | Master design names "per-integration mapping" as a C11 responsibility; shallow defeats the migration value |
| 6 | Mappers ship as drivers do; placeholders for unpublished drivers | Block on all drivers / invent shapes | Decouples C11's shipping from per-driver cadence; user gets working tool day-1 with clear path for the rest |
| 7 | Jinja scope = expressions + control flow + time helpers (B); no area / geographic / macros | Expressions only (A) / everything (C) | Loops + `now()` / `as_datetime` cover the common cases; area / geographic helpers need stdlib that doesn't exist; macros are exotic |
| 8 | `gonja` for Jinja parsing | Hand-roll / shell to Python | Maintained Go port; AST visitor pattern is the right shape for emit-Starlark |
| 9 | Two severities: FIXME + NOTE | More (info / warn / error / etc.) | Anything more is over-engineering for a one-shot tool |
| 10 | Closed `Reason` enum | Free-form strings | Keeps report tabular sections grep-able; surfaces unintended category sprawl as a code review |
| 11 | Inline diagnostics + out-of-band `IMPORT_REPORT.md` | Inline only / report only | Inline gives precise locations; report gives skimmable overview; both are cheap once the collector exists |
| 12 | Exit 0 with FIXMEs; exit 1 only on hard failures | Exit non-zero on any FIXME | FIXMEs are expected; non-zero exit would block scripted use |
| 13 | Per-area subpackages (`automations/`, `scripts/`, …) | Single flat `internal/importer` | Each handler answers "what does it do, how do you use it, what does it depend on?" cleanly; tests per package |
| 14 | No general-purpose Pkl AST emitter in v1.0; `text/template` per file type | Build a real AST emitter | Bounded scope; output is grep-able and human-readable; refactor to AST emitter only if duplication or quality issues emerge |
| 15 | `gopkg.in/yaml.v3` with custom resolver for HA tags | Other YAML libs / ad-hoc parsing | yaml.v3 supports custom tag resolution cleanly; HA's tag set is bounded |
| 16 | Hybrid fixture strategy (synthetic per-construct + 2 anonymized real configs) | Synthetic only / real only | Synthetic gives tight coverage; real catches composition issues; one of each is enough |
| 17 | Anonymization rules documented but applied manually | Automated anonymization tooling | Bounded fixture count; manual is faster than building a tool |
| 18 | Importer is pure file-IO; no daemon required | Importer hits running gohomed | Decouples; users can import from a backup; no auth complexity |

---

## 17. Explicit Deferrals

Named here so the spec acknowledges them without blocking.

### 17.1 Deferred to a follow-on Lovelace milestone

- **Lovelace dashboard YAML translation.** All of it — `lovelace`, `lovelace_dashboards`, `.storage/lovelace*`, custom view types, conditional cards, layout cards, sections view, custom cards, theme YAML. Becomes its own milestone; expected name C11.5 (or C12 — slot in the master design's §10 roadmap separately).
  - Rationale: Lovelace is HA's largest open-ended config surface; mapping each card type to gohome's eight built-in widgets (or marking unmappable) is its own substantial design problem; design quality depends on C10's widget set + container schema being battle-tested by real users.
  - In the meantime: the importer detects `.storage/lovelace*` and `lovelace:` in `configuration.yaml` and emits a single `NOTE(lovelace-not-imported)` per detected dashboard, with a pointer to the future Lovelace import milestone.

### 17.2 Deferred to v1.x

- **Per-integration mappers for the v1.0 list whose drivers haven't published Pkl manifests at C11 implementation time.** Each ships as its driver does, in a small focused PR.
- **Repeatable / merge-mode imports.** Single-shot only in v1.0.
- **`--integrations <list>` partial-import filter.** Full imports only in v1.0.
- **Area hierarchy detection beyond simple label conventions.** v1.0 is heuristic + NOTE; richer schemes are v1.x.
- **Person → presence-entity wiring.** v1.0 just NOTE's; users wire presence themselves once their tracker drivers are installed.
- **Password migration** of any kind. Always passkey re-registration in v1.0+.
- **Jinja: area expansion (`expand`, `area_entities`, `device_entities`, `area_id`, `area_name`).** v1.x once gohome has equivalent stdlib helpers.
- **Jinja: geographic helpers (`closest`, `distance`).** v1.x once `gohome.geo` stdlib exists.
- **Jinja macros.** Rare in real configs; v1.x if user demand emerges.
- **HACS custom filter detection / mapping.** Indefinite — HACS plugins are unbounded; the FIXME path is sufficient.
- **HA blueprint `!input` translation.** Blueprints aren't a v1.0 gohome concept; v1.x once gohome has an equivalent.
- **Performance benchmarks for very large configs.** v1.x once we have a complaint or a known scaling concern.
- **Windows path handling.** v1.x; HA on Windows is rare in practice.

### 17.3 Out of scope indefinitely

- **HA recorder DB migration** (history events into gohome's event log). Master design §8.1 explicit non-goal.
- **HACS integration translation.** Master design §8.1 explicit non-goal.
- **Supervisor add-on translation.** Master design §8.1 explicit non-goal.
- **Bidirectional sync** between HA and gohome (importer is one-way only).
- **Live "follow HA changes" mode** (continuously re-import as HA changes — encourages dual-running which the project has explicitly deferred).

---

*End of C11 design document.*
