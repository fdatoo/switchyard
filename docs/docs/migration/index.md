# Migrating from Home Assistant

!!! status-wip "In development"
    This feature is in active development. The `switchyard import-ha` command is not yet shipped.

If you are running Home Assistant today and want to move to switchyard, this section is your guide. Before reading further, see [switchyard vs. Home Assistant](../introduction/vs-home-assistant.md) — it explains what the two systems do differently and what you give up by switching. Come back here when you have decided to migrate.

---

## Overview

`switchyard import-ha` is a CLI command that reads your Home Assistant configuration directory and produces a fresh switchyard Pkl config tree. The output is a git-initable directory you review, edit, and commit before pointing `switchyardd` at it. The importer is a one-shot tool: run it, inspect the result, resolve any issues it flags, then proceed with your switchyard setup.

The approach is honest about what can and cannot be automatically translated. Everything the importer cannot handle precisely is flagged with a `# FIXME` comment in the generated files, and a summary report (`IMPORT_REPORT.md`) is written alongside the config so you have a single place to review what needs attention.

---

## Prerequisites

- **switchyard installed.** The importer ships as part of the `switchyard` CLI. See [Installation](../installation/index.md).
- **Your Home Assistant config directory is accessible.** This can be your live `~/.homeassistant` or `/config` directory, or a backup snapshot unpacked locally. HA does not need to be running — the importer works purely from the filesystem.
- **The directory contains `configuration.yaml`.** If it does not, the importer errors with a clear message. A partial directory (missing some optional files) is fine; absent files are silently skipped.

---

## What the importer does

The importer runs a multi-stage pipeline:

1. **Scans** the source directory, classifying files: `configuration.yaml`, `automations.yaml`, `scripts.yaml`, `scenes.yaml`, `secrets.yaml`, the split `automations/` and `scripts/` directories, and the `.storage/` registry files.
2. **Loads** all YAML using a custom resolver that handles HA's non-standard tags (`!include`, `!include_dir_list`, `!secret`, `!input`, and variants). All included files and secrets are resolved into an in-memory model before any transformation begins.
3. **Maps** each recognised HA integration to the corresponding switchyard driver instance declaration. Integrations outside switchyard's v1.0 driver set are preserved as labelled placeholders in the output so you know what manual work remains.
4. **Transpiles** Jinja templates (used in automations, scripts, and computed entities) to Starlark. Common Jinja constructs translate automatically; constructs outside the supported set emit `# FIXME` markers with the original Jinja preserved.
5. **Translates** the area registry, entity registry, zones, scenes, users, and persons into their switchyard Pkl equivalents.
6. **Writes** a complete, git-initable output directory: Pkl files, Starlark handlers, a `.gitignore`, and `IMPORT_REPORT.md`.

The pipeline runs without a network connection and without a running `switchyardd` daemon. It is pure file I/O.

---

## What the importer does NOT do

This list is honest.

- **Does not migrate passwords or credentials.** HA stores password hashes in a format incompatible with switchyard's authentication system. All users are migrated structurally (display name, role), but credentials are not transferred. After import you re-register each user's passkey via `switchyard auth bootstrap <slug>`.
- **Does not migrate secrets values into Pkl source.** Secret values from `secrets.yaml` are written to a separate `IMPORTED_SECRETS.env` file, which is added to `.gitignore`. The Pkl config references secrets as `read("env:UPPER_SNAKE_CASE")`. You source `IMPORTED_SECRETS.env` once, verify everything works, then delete it and move values to your preferred secret store.
- **Does not translate Lovelace dashboards.** Lovelace dashboard YAML is detected and noted in the report, but translation is a separate future milestone. Rebuild dashboards using switchyard's WYSIWYG dashboard editor.
- **Does not support HACS custom integrations.** Custom integrations outside switchyard's driver set produce placeholder entries in the output. There is no automatic mapping.
- **Does not migrate HA recorder history.** Event history is not transferred.
- **Does not run in merge mode.** The importer is single-shot. To re-import, delete the output directory and run again with `--force`.

---

## The command

```
switchyard import-ha [<ha-dir>] -o <out-dir>
```

If `<ha-dir>` is omitted, the importer tries `~/.homeassistant` then `/config` (the HAOS path). An explicit path is recommended.

**Example:**

```
$ switchyard import-ha ~/.homeassistant -o ./my-switchyard
Scanning ~/.homeassistant
Loading 14 YAML files
Mapping 6 integrations (3 fully mapped, 3 unpublished-driver placeholders)
Transpiling 27 automations
Writing 31 files to ./my-switchyard
Done — 8 FIXMEs, 3 NOTEs. See ./my-switchyard/IMPORT_REPORT.md
```

**Flags:**

| Flag | Default | Purpose |
|---|---|---|
| `-o, --out <dir>` | (required) | Output directory for the switchyard config tree |
| `--dry-run` | false | Run the full pipeline and print `IMPORT_REPORT.md` to stdout; write no files |
| `-f, --force` | false | Overwrite a non-empty output directory |
| `-v, --verbose` | false | Per-file progress logging to stderr |
| `-q, --quiet` | false | Errors only |

The command exits `0` even when FIXMEs are present — FIXMEs are expected and it is your job to resolve them. A hard failure (unreadable source dir, malformed YAML, write error) exits `1`.

---

## What to read next

- [What transfers](what-transfers.md) — full mapping table from HA constructs to switchyard targets, with confidence levels
- [Jinja to Starlark](jinja-to-starlark.md) — how templates are translated and what needs manual review
- [Post-migration checklist](post-migration.md) — what to do after the import command completes
