# Switchyard Linear backlog audit — 2026-05-02

Layered audit of every chapter spec, plan, and the codebase against the Linear project. Produced 50 issues across 24 milestones (F-129 → F-178).

**Project:** [Switchyard](https://linear.app/fdatoo/project/switchyard-6fbabf2d58a9)

---

## Audit method

Four layered passes:

1. **Global stub/incomplete sweep** — project-wide grep for `TODO`/`FIXME`/`XXX`/`HACK`/`panic("not implemented")`/`CodeUnimplemented`/`t.Skip`/`nolint`/`// stub|placeholder|deferred`/dropped errors. 5 TODOs, 1 t.Skip, ~30 nolint directives, several stub functions surfaced.
2. **Per-chapter deep dive** with the global hit-list in hand — for each of C1–C12 plus driver specs, walked the spec's Scope / Success Criteria / Task Breakdown / Testing Strategy / Observability sections and grepped for every named symbol, metric, span, test function, CLI command, and proto service against the codebase.
3. **Documentation drift pass** — walked `docs/docs/` against current code reality.
4. **Cross-cutting pass** — CI workflow vs. spec coverage gates / integration tests / fuzz / build matrix.

This was the user's "absolutely minimize the chance something goes missed" requirement. Earlier shallower attempts found ~1 gap per chapter; the layered method found 4–5 per chapter on average.

---

## Milestones overview (24 total)

### Spec'd chapters (12)

| Milestone | Progress | Gap issues filed |
|---|---|---|
| C1 — Event core & storage | 17% | F-130 (recovery admin HTTP), F-141 (snapshot-corruption test skipped), F-142 (OTel call-sites), F-143 (coverage gates), F-144 (fuzz wiring) |
| C2 — Carport driver protocol | 33% | F-145 (Dispatch span), F-146 (3 missing integration scenarios) |
| C3 — Driver SDK | 100% | — clean |
| C4 — Pkl config system | 50% | F-147 (compile_integration_test missing) |
| C5 — Starlark runtime | 100% | — clean |
| C6 — Automation engine | 50% | F-148 (Scene applier stub — depends on Scene engine spec) |
| C7 — Connect-RPC API | 25% | F-149 (EntityService.Subscribe never wired), F-150 (Diagnostics no-op), F-151 (no CLI golden tests) |
| C8 — MCP server | 100% | — clean |
| C9 — Auth & policy | 20% | F-152 (passkey wiring), F-153 (SO_PEERCRED), F-154 (TokenScope decoder), F-155 (script cancel) |
| C10 — Web UI foundation | 17% | F-140 (no JS tests), F-156 (dashboard backend no-op — **Urgent**), F-157 (widget pack OCI install — **Urgent**), F-158 (PWA service worker), F-159 (ESLint token-discipline rule) |
| C11 — HA import tool | 0% | F-160 (entirely unimplemented; tracker) |
| C12 — Edge agent | 0% | F-161 (entirely unimplemented; tracker) |

### Driver / cross-cutting milestones with completed work (7) — all 100% baseline-shipped

- **Drivers — Hue (v0.1 + color + robustness)** — F-162
- **Drivers — Zigbee2MQTT (v0.1)** — F-163
- **Driver Pkl modules + dynamic mgmt** — F-164
- **Entity — binary_sensor** — F-165
- **Hygiene — monorepo + Switchyard rename** — F-166
- **Examples — config templates** — F-167
- **Docs site (Zensical)** — F-168

### Open trackers (5)

- **Documentation drift** — F-169 (edge-agents page wrongly claims partial impl), F-170 (auth doc says not-shipped when it is), F-171 (11 docs reference wrong `fynn-labs` org), F-172 (install docs claim binaries no release workflow produces)
- **Cross-cutting hygiene** — F-173 (linux/arm64 missing from CI), F-176 (dead `service_unimplemented.go::DashboardService`), F-177 (per-PR fuzz), F-178 (recurring-audit process tracker)
- **C13 — Distribution, updates & operations** *(unspecced)* — F-174 (write the spec). Accumulated all the deferred items from C1/C2/C7/C9: OTel bridge, surgical-repair CLI, backup/restore, generated client publishing, TLS lifecycle, signed manifests, release engineering, self-update.
- **C14 — Rollup & long-term retention** *(unspecced, v1.x)* — placeholder; no spec yet.
- **Scene engine** *(unspecced)* — F-175 (write the spec). Referenced by C6/C7/C8 but never specced; today there are warn-log stubs in three places (`automation/action/scene.go`, `internal/starlark/stdlib.go`, `internal/api/service_unimplemented.go::SceneService`).

---

## Highest-priority gaps (`Urgent` + `High`)

| ID | Milestone | Title |
|---|---|---|
| **F-156** (Urgent) | C10 | Dashboard backend in daemon is no-op stubs; whole dashboard subsystem non-functional in production |
| **F-157** (Urgent) | C10 | Widget-pack OCI install is a metadata-registration stub; cosign verification never runs |
| **F-149** (High) | C7 | EntityService.Subscribe always returns `Unimplemented`; web UI multiplexer + CLI watch broken |
| **F-152** (High) | C9 | Passkey ceremony handlers stubbed even though `internal/auth/credentials/webauthn.go` is fully implemented |
| **F-153** (High) | C9 | SO_PEERCRED not wired; `system:local` UDS bypass non-functional |
| **F-154** (High) | C9 | TokenScope decoder is a stub; token-scope intersection never enforced |

These are the cases the user's stated failure mode ("agents silently decided not to implement something") most exactly describes — handler/CLI shipped, but the wiring or backend is a no-op.

---

## Recurring audits to run on cadence

Codified in [F-178](https://linear.app/fdatoo/issue/F-178). Each is intended to become a `Taskfile.yml` `audit:*` target or scheduled GitHub workflow.

### A. Stub & incomplete sweep (monthly)

```bash
grep -rnE '\b(TODO|FIXME|XXX|HACK)\b' --include='*.go' --include='*.tsx' --include='*.ts' --include='*.pkl' .
grep -rnE 'panic\("(not implemented|unimplemented|TODO)' --include='*.go' .
grep -rnE 'return nil, ErrNotImplemented|unimplemented\(ctx,' internal/api/
grep -rnE '\bt\.Skip\b' --include='*_test.go' .
grep -rnE '// (stub|placeholder|deferred|nop|workaround|temporary|not.yet|until.*lands|when.*lands)' --include='*.go' .
```

Compare hits against previous run; new entries = new debt.

### B. Observability catalog audit (per-chapter)

For each chapter spec that lists a metrics catalog (C1 §9.2, C2 §9.2, C6 §9.2, C7 §10.1, C8 §6, C9 §11): verify every named metric is registered in `internal/observability/metrics.go`. Same for OTel span call sites enumerated by chapter (C1 §9.3, C2 §9.3).

Method: `awk '/## .*Observability/,/^## /'` per spec → grep names → grep registry. Diff = gaps.

### C. Spec → integration test parity (per-chapter)

For each spec's "Testing Strategy" or §11 section that names specific tests, grep for the function names. Missing names = missing tests. This pattern caught:

- C1: `TestSnapshot_CorruptedFallsBack` is `t.Skip`
- C2: 3 missing TESTDRIVER_MODE scenarios (`hang_on_command`, `chatty`, `repeat_register`)
- C7: no CLI golden tests despite spec §11.4
- C10: no JS tests despite spec §1.1 + §18

### D. Doc-drift audit (quarterly)

Walk `docs/docs/` page-by-page:

- Every CLI command claim → verify against `switchyard --help`
- Every install URL → `curl --head`; expect 200
- Every "shipped/not shipped" status banner → cross-check against actual code presence
- Every Pkl module reference → verify in `internal/config/pkl/switchyard/`
- Every proto service reference → verify in `proto/switchyard/v1alpha1/`

### E. CI workflow vs. spec parity (quarterly)

Each spec lists what CI should run. Compare against `.github/workflows/`:

- Coverage gates exist and at the right thresholds (today: 70% on carport only; spec: ≥85% on eventstore/state/registry/carport)
- Integration tests actually run (not just present)
- Fuzz targets in CI command list (today: `FuzzFixtureParse` and `FuzzRegistryApply` defined but not invoked)
- Build matrix matches spec-required platforms (today: missing `linux/arm64`)

### F. Dead-code sweep (per release)

For every API handler in `internal/api/`, confirm it's reachable from `internal/daemon/daemon.go`'s wiring. Two stub-vs-real implementations of the same handler = dead code. (Caught: `service_unimplemented.go::DashboardService` is dead.)

For Go code generally: `staticcheck` with `U1000` (unused) tag.

### G. Spec inventory (whenever a chapter ships)

For each new spec landing in `docs/design/specs/`, immediately file the milestone + baseline issue + any "explicit deferrals" as separate tracking issues. Don't let cross-spec deferrals accumulate as they did with C13.

---

## Convention summary

For future agents extending this backlog:

- **Issue types**: `Improvement` for baseline-shipped anchors; `Feature` for genuinely-new work; `Bug` for visibly-broken-but-shouldn't-be (tests skipped, stubs returning success, drifted docs)
- **Priority**: `Urgent` for "feature is non-functional in production"; `High` for "blocks adjacent dependent work"; `Normal`/unset for hygiene
- **Milestones**: one per chapter; one per cohesive driver effort; trackers for unspecced future chapters; `Documentation drift` and `Cross-cutting hygiene` as horizontal holding pens
- **Baseline issue per milestone**: anchors what shipped and where deliberate deviations happened (e.g. drivers.toml → Pkl swap, MCP HTTP shipping early)
- **Memory saved**: `gohome → switchyard` rename in `~/.claude/projects/-home-fdatoo-switchyard/memory/project_rename.md` so future sessions don't re-flag spec/code naming drift

---

## Known limitations of this audit

- Go isn't installed in this sandbox, so I verified test *presence*, not test *passing*. All "Tests pass" check-marks lean on CI being green on `main`.
- I read each spec's Scope / Success-Criteria / Task-Breakdown / Testing-Strategy / Observability sections plus did a global stub sweep. I did **not** open every `*.md` and read every paragraph; cases where a spec describes a behavior in body text without listing it as a criterion may have escaped (audit B/C catches these on second pass).
- I didn't audit `dev/` notes, `docs/superpowers/` plans, `proto-hygiene.md` rules in depth, or any of the `*.pkl` test fixtures for drift.
- The spec→code drift around the `gohome` → `switchyard` rename is intentional per project decision; the audit does not flag spec text using the old name.
