---
title: GoHome → Switchyard rename
date: 2026-05-01
status: approved
---

# GoHome → Switchyard Rename

## Motivation

"GoHome" is heavily overloaded — it collides with the common English phrase, Go-language tooling associations, and produces poor search/discoverability results. "Switchyard" (a routing yard for signals) maps directly onto the project's core identity: a daemon that dispatches capability calls to the right driver.

## Scope

Pre-release project with no external users. Hard cutover, no backwards compatibility or migration story required.

## Section 1: Identity — modules and binaries

| Current | New |
|---|---|
| Go module `github.com/fdatoo/gohome` | `github.com/fdatoo/switchyard` |
| Go module `github.com/fdatoo/gohome-driverkit` | `github.com/fdatoo/switchyard-driverkit` |
| Binary `gohomed` | `switchyardd` |
| Binary `gohome` | `switchyard` |
| Directory `cmd/gohomed/` | `cmd/switchyardd/` |
| Directory `cmd/gohome/` | `cmd/switchyard/` |
| Directory `gohome-driverkit/` | `switchyard-driverkit/` |

The daemon keeps the conventional `d` suffix (`switchyardd`). The CLI is `switchyard` — no short alias. The internal `carport` subsystem name is left untouched; it doesn't reference the product name.

## Section 2: Proto/API package names

| Current | New |
|---|---|
| `proto/gohome/` | `proto/switchyard/` |
| `package gohome.entity.v1` | `package switchyard.entity.v1` |
| `package gohome.carport.v1alpha1` | `package switchyard.carport.v1alpha1` |
| `package gohome.v1alpha1` | `package switchyard.v1alpha1` |
| `gen/gohome/` (generated Go) | `gen/switchyard/` |

The buf config, all `.proto` imports, and all generated Go import paths must be updated atomically in a single pass: `sed` rename + `buf generate`. Generated import paths pick up both the module rename (section 1) and the `gen/gohome/` → `gen/switchyard/` path change.

## Section 3: External surfaces

| Surface | Current | New |
|---|---|---|
| GitHub repo | `fdatoo/gohome` | `fdatoo/switchyard` |
| GitHub repo | `fdatoo/gohome-driverkit` | `fdatoo/switchyard-driverkit` |
| Docs site title | GoHome | Switchyard |
| `README.md` | all references | updated |

GitHub repo renames are non-destructive — GitHub redirects old URLs automatically.

No config directory migration is needed (pre-release, no users).
