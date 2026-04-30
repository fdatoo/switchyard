# Monorepo Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collapse the three GoHome submodules into a single monorepo at `~/Developer/GoHome`, backed by `fdatoo/gohome` on GitHub, starting fresh at `v0.1.0`.

**Architecture:** Fresh-start migration — copy files from each submodule into the new directory structure, update Go module paths from `fynn-labs` to `fdatoo`, wire a `go.work` workspace, consolidate CI into two workflow files, then push to `fdatoo/gohome`, tag `v0.1.0`, and delete the three old repos. No git history is preserved.

**Tech Stack:** Go 1.25, Go workspaces (`go.work`), GitHub Actions (`dorny/paths-filter@v3`), Zensical (docs site), Task, buf (protobuf), Pkl

---

## File Map

### Directories created
- `~/Developer/GoHome/` — new monorepo root
- `~/Developer/GoHome/.github/workflows/`
- `~/Developer/GoHome/gohome/` — copied from `~/Desktop/GoHome/gohome/`
- `~/Developer/GoHome/gohome-driverkit/` — copied from `~/Desktop/GoHome/gohome-driverkit/`
- `~/Developer/GoHome/docs/` — copied from `~/Desktop/GoHome/docs/` (the gohome-docs submodule)
- `~/Developer/GoHome/docs/design/` — renamed from `docs/` inside the gohome-docs submodule

### Files created
- `~/Developer/GoHome/go.work` — workspace linking both Go modules
- `~/Developer/GoHome/.github/workflows/ci.yml` — unified Go CI with path filters
- `~/Developer/GoHome/.github/workflows/docs.yml` — docs deploy
- `~/Developer/GoHome/CLAUDE.md` — updated container instructions (monorepo, not submodules)
- `~/Developer/GoHome/README.md` — copied from container root

### Files modified (within copied content)
- `~/Developer/GoHome/gohome/go.mod` — `fynn-labs` → `fdatoo` in module declaration
- `~/Developer/GoHome/gohome-driverkit/go.mod` — `fynn-labs` → `fdatoo` + remove `replace` directive
- All `*.go` files in both modules — import paths `fynn-labs` → `fdatoo`

---

## Task 1: Create directory skeleton

**Files:**
- Create: `~/Developer/GoHome/.github/workflows/`

- [ ] **Step 1: Create directories**

```bash
mkdir -p ~/Developer/GoHome/.github/workflows
```

- [ ] **Step 2: Verify**

```bash
ls ~/Developer/GoHome/
```

Expected output:
```
.github/
```

---

## Task 2: Copy gohome module

**Files:**
- Create: `~/Developer/GoHome/gohome/`

- [ ] **Step 1: Copy gohome, excluding git history and build artifacts**

```bash
rsync -a --exclude='.git' --exclude='dist/' --exclude='web/node_modules/' \
  ~/Desktop/GoHome/gohome/ ~/Developer/GoHome/gohome/
```

- [ ] **Step 2: Verify key files are present**

```bash
ls ~/Developer/GoHome/gohome/
```

Expected: `go.mod  go.sum  cmd/  internal/  proto/  Taskfile.yml` and other top-level files/dirs.

---

## Task 3: Copy gohome-driverkit module

**Files:**
- Create: `~/Developer/GoHome/gohome-driverkit/`

- [ ] **Step 1: Copy driverkit, excluding git history**

```bash
rsync -a --exclude='.git' \
  ~/Desktop/GoHome/gohome-driverkit/ ~/Developer/GoHome/gohome-driverkit/
```

- [ ] **Step 2: Verify**

```bash
ls ~/Developer/GoHome/gohome-driverkit/
```

Expected: `go.mod  go.sum` and source directories.

---

## Task 4: Copy and restructure docs

The gohome-docs submodule is at `~/Desktop/GoHome/docs/`. Inside it, the `docs/` subdirectory holds the actual content pages — that becomes `design/` in the monorepo.

**Files:**
- Create: `~/Developer/GoHome/docs/`
- Create: `~/Developer/GoHome/docs/design/` (renamed from internal `docs/`)

- [ ] **Step 1: Copy gohome-docs, excluding git history and generated site**

```bash
rsync -a --exclude='.git' --exclude='site/' \
  ~/Desktop/GoHome/docs/ ~/Developer/GoHome/docs/
```

- [ ] **Step 2: Rename the internal docs/ directory to design/**

```bash
mv ~/Developer/GoHome/docs/docs ~/Developer/GoHome/docs/design
```

- [ ] **Step 3: Add docs_dir to zensical.toml**

Zensical defaults `docs_dir` to `docs/`. Since we renamed that to `design/`, the build will fail without this change.

```bash
sed -i '' 's/^\[project\]/docs_dir = "design"\n\n[project]/' \
  ~/Developer/GoHome/docs/zensical.toml
```

- [ ] **Step 4: Verify zensical.toml has the new setting**

```bash
head -5 ~/Developer/GoHome/docs/zensical.toml
```

Expected first line: `docs_dir = "design"`

- [ ] **Step 5: Verify directory structure**

```bash
ls ~/Developer/GoHome/docs/
```

Expected: `design/  zensical.toml` (plus any other top-level files from gohome-docs).

```bash
ls ~/Developer/GoHome/docs/design/ | head -10
```

Expected: content directories like `ai-agents/  automations/  concepts/  drivers/  ...`

---

## Task 5: Copy container-level files and write updated CLAUDE.md

**Files:**
- Create: `~/Developer/GoHome/README.md`
- Create: `~/Developer/GoHome/CLAUDE.md`

- [ ] **Step 1: Copy README**

```bash
cp ~/Desktop/GoHome/README.md ~/Developer/GoHome/README.md
```

- [ ] **Step 2: Write updated CLAUDE.md**

```bash
cat > ~/Developer/GoHome/CLAUDE.md << 'EOF'
# GoHome — Monorepo

## What this is

`GoHome/` is a monorepo containing the sub-projects making up the gohome platform. Each Go sub-project has its own `go.mod`; a `go.work` at the repo root links them for local development.

## Directory map

| Directory | Module | Purpose |
|-----------|--------|---------|
| `gohome/` | `github.com/fdatoo/gohome` | Go module: `gohomed` daemon + `gohome` CLI |
| `gohome-driverkit/` | `github.com/fdatoo/gohome-driverkit` | Driver development kit |
| `docs/` | — | Documentation site (Zensical) |
| `docs/design/` | — | Design specs, implementation plans, architecture docs |

## Rules

- **Documentation and design specs live in `docs/design/`**, not in `gohome/` or anywhere else. If you need to write or update a spec, plan, or architecture doc, the file goes there.
- **Go workspace:** `go.work` at the repo root links `./gohome` and `./gohome-driverkit`. Run Go commands from the repo root or from within a module directory — the workspace is found automatically by walking up the directory tree.
- **Never create a new top-level directory** without checking with the user first. New platform components belong here if they are Go modules added to `go.work`, or standalone tools — not arbitrary folders.
- **The devcontainer** at `.devcontainer/` installs tooling for working across all sub-projects.
EOF
```

- [ ] **Step 3: Verify CLAUDE.md was written**

```bash
head -5 ~/Developer/GoHome/CLAUDE.md
```

Expected: `# GoHome — Monorepo`

---

## Task 6: Rename module paths in all Go files

All import paths and `go.mod` module declarations reference `github.com/fynn-labs`. These become `github.com/fdatoo`.

**Files:**
- Modify: all `*.go` files in `gohome/` and `gohome-driverkit/`
- Modify: `gohome/go.mod`, `gohome-driverkit/go.mod`

- [ ] **Step 1: Replace in all Go source files**

```bash
find ~/Developer/GoHome/gohome ~/Developer/GoHome/gohome-driverkit \
  -type f -name "*.go" \
  -exec sed -i '' 's|github\.com/fynn-labs|github.com/fdatoo|g' {} +
```

- [ ] **Step 2: Replace in both go.mod files**

```bash
sed -i '' 's|github\.com/fynn-labs|github.com/fdatoo|g' \
  ~/Developer/GoHome/gohome/go.mod \
  ~/Developer/GoHome/gohome-driverkit/go.mod
```

- [ ] **Step 3: Verify no fynn-labs references remain**

```bash
grep -r "fynn-labs" ~/Developer/GoHome/gohome ~/Developer/GoHome/gohome-driverkit
```

Expected: no output.

- [ ] **Step 4: Spot-check a renamed import**

```bash
head -5 ~/Developer/GoHome/gohome/go.mod
```

Expected first line: `module github.com/fdatoo/gohome`

---

## Task 7: Remove the replace directive from gohome-driverkit

The `gohome-driverkit/go.mod` currently has a `replace` directive and an accompanying TODO comment. Both must be removed — `go.work` makes them redundant.

**Files:**
- Modify: `~/Developer/GoHome/gohome-driverkit/go.mod`

- [ ] **Step 1: Remove the replace line**

```bash
sed -i '' '/^replace github\.com\/fdatoo\/gohome/d' \
  ~/Developer/GoHome/gohome-driverkit/go.mod
```

- [ ] **Step 2: Remove the TODO comment above it**

```bash
sed -i '' '/TODO: drop replace/d' \
  ~/Developer/GoHome/gohome-driverkit/go.mod
```

- [ ] **Step 3: Verify go.mod is clean**

```bash
grep -E "replace|TODO" ~/Developer/GoHome/gohome-driverkit/go.mod
```

Expected: no output.

```bash
cat ~/Developer/GoHome/gohome-driverkit/go.mod
```

Expected: `module github.com/fdatoo/gohome-driverkit`, a `go` directive, and a `require` block referencing `github.com/fdatoo/gohome`. No `replace` line.

---

## Task 8: Create go.work and sync

**Files:**
- Create: `~/Developer/GoHome/go.work`

- [ ] **Step 1: Write go.work**

```bash
cat > ~/Developer/GoHome/go.work << 'EOF'
go 1.25.0

use (
	./gohome
	./gohome-driverkit
)
EOF
```

- [ ] **Step 2: Run go work sync**

```bash
cd ~/Developer/GoHome && go work sync
```

Expected: exits 0. A `go.work.sum` file is created.

- [ ] **Step 3: Verify workspace resolves both modules**

```bash
cd ~/Developer/GoHome && go list -m all | grep fdatoo
```

Expected:
```
github.com/fdatoo/gohome v0.0.0-...
github.com/fdatoo/gohome-driverkit v0.0.0-...
```

---

## Task 9: Verify the build locally

This is the gate before touching GitHub. All steps must pass.

**Files:** none created

- [ ] **Step 1: Tidy gohome**

```bash
cd ~/Developer/GoHome/gohome && go mod tidy
```

Expected: exits 0. `go.mod` and `go.sum` may have minor updates.

- [ ] **Step 2: Tidy gohome-driverkit**

```bash
cd ~/Developer/GoHome/gohome-driverkit && go mod tidy
```

Expected: exits 0.

- [ ] **Step 3: Build everything from workspace root**

```bash
cd ~/Developer/GoHome && go build ./...
```

Expected: exits 0, no errors.

- [ ] **Step 4: Run gohome tests**

```bash
cd ~/Developer/GoHome/gohome && go test ./...
```

Expected: all pass.

- [ ] **Step 5: Run gohome race tests**

```bash
cd ~/Developer/GoHome/gohome && go test -race ./...
```

Expected: exits 0.

- [ ] **Step 6: Run driverkit tests**

```bash
cd ~/Developer/GoHome/gohome-driverkit && go test -race ./...
```

Expected: exits 0.

- [ ] **Step 7: Verify docs site builds**

```bash
cd ~/Developer/GoHome/docs && zensical build --clean
```

Expected: exits 0, `site/` directory is populated.

---

## Task 10: Write unified CI

**Files:**
- Create: `~/Developer/GoHome/.github/workflows/ci.yml`
- Create: `~/Developer/GoHome/.github/workflows/docs.yml`

- [ ] **Step 1: Write ci.yml**

```bash
cat > ~/Developer/GoHome/.github/workflows/ci.yml << 'YAML'
name: CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  changes:
    runs-on: ubuntu-latest
    outputs:
      gohome: ${{ steps.filter.outputs.gohome }}
      driverkit: ${{ steps.filter.outputs.driverkit }}
    steps:
      - uses: actions/checkout@v4
      - uses: dorny/paths-filter@v3
        id: filter
        with:
          filters: |
            gohome:
              - 'gohome/**'
              - 'go.work'
            driverkit:
              - 'gohome-driverkit/**'
              - 'go.work'

  build-and-test:
    needs: changes
    if: needs.changes.outputs.gohome == 'true'
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    defaults:
      run:
        working-directory: gohome
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
          cache: true

      - name: Install Task
        uses: arduino/setup-task@v2
        with:
          version: 3.x
          repo-token: ${{ secrets.GITHUB_TOKEN }}

      - name: Install buf
        uses: bufbuild/buf-setup-action@v1
        with:
          github_token: ${{ secrets.GITHUB_TOKEN }}

      - name: Install protoc-gen-go-grpc
        run: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.1

      - name: Install Pkl
        run: |
          case "${{ matrix.os }}" in
            ubuntu-latest) asset=pkl-linux-amd64 ;;
            macos-latest)  asset=pkl-macos-aarch64 ;;
            *) echo "unsupported os ${{ matrix.os }}" >&2; exit 1 ;;
          esac
          sudo curl -fsSL -o /usr/local/bin/pkl "https://github.com/apple/pkl/releases/download/0.31.1/$asset"
          sudo chmod +x /usr/local/bin/pkl
          pkl --version

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'
          cache-dependency-path: 'gohome/web/package-lock.json'

      - name: Regenerate proto (verify committed in sync)
        run: |
          buf generate
          git diff --exit-code gen/

      - name: Tidy
        run: go mod tidy && git diff --exit-code go.mod go.sum

      - name: Build
        run: task build

      - name: Test
        run: task test

      - name: Race
        run: task test:race

      - name: Integration
        run: task test:integration

  lint:
    needs: changes
    if: needs.changes.outputs.gohome == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - uses: golangci/golangci-lint-action@v7
        with:
          version: v2.11.4
          working-directory: gohome

  coverage:
    needs: changes
    if: needs.changes.outputs.gohome == 'true'
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: gohome
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
          cache: true
      - name: Run carport coverage (unit + integration, main pkg only)
        run: go test -tags=integration -coverprofile=cover.out ./internal/carport
      - name: Enforce ≥70% coverage
        run: |
          pct=$(go tool cover -func=cover.out | awk '/^total:/ {print $3}' | tr -d %)
          awk -v p="$pct" 'BEGIN{exit !(p+0>=70)}' || {
            echo "carport coverage ${pct}% below 70% gate"
            go tool cover -func=cover.out
            exit 1
          }
          echo "carport coverage ${pct}% — passes ≥70% gate"

  proto:
    needs: changes
    if: needs.changes.outputs.gohome == 'true'
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: gohome
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - uses: bufbuild/buf-setup-action@v1
      - name: Determine diff base
        id: base
        run: |
          if [ "${{ github.event_name }}" = "pull_request" ]; then
            echo "ref=origin/${{ github.event.pull_request.base.ref }}" >> $GITHUB_OUTPUT
          else
            echo "ref=HEAD^" >> $GITHUB_OUTPUT
          fi
      - name: Proto hygiene check
        run: ./scripts/check-proto-hygiene.sh ${{ steps.base.outputs.ref }}
      - name: Buf lint
        run: buf lint

  fuzz-scheduled:
    needs: changes
    if: github.event_name == 'schedule'
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: gohome
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: go test -fuzz=FuzzEventDecode -fuzztime=5m ./internal/eventstore
      - run: go test -fuzz=FuzzFilterMatch -fuzztime=5m ./internal/eventstore
      - run: go test -fuzz=FuzzEnvelopeDecode -fuzztime=5m ./internal/carport

  driverkit-test:
    needs: changes
    if: needs.changes.outputs.driverkit == 'true'
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    defaults:
      run:
        working-directory: gohome-driverkit
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - name: Build
        run: go build ./...
      - name: Tests
        run: go test ./...
      - name: Race tests
        run: go test -race ./...
      - name: Integration tests
        run: go test -race -tags integration ./...

  driverkit-lint:
    needs: changes
    if: needs.changes.outputs.driverkit == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - uses: golangci/golangci-lint-action@v6
        with:
          version: v2.1.6
          working-directory: gohome-driverkit
YAML
```

- [ ] **Step 2: Write docs.yml**

```bash
cat > ~/Developer/GoHome/.github/workflows/docs.yml << 'YAML'
name: Documentation

on:
  push:
    branches: [main]
    paths:
      - 'docs/**'
  workflow_dispatch:

permissions:
  contents: read
  pages: write
  id-token: write

jobs:
  deploy:
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    runs-on: ubuntu-latest
    steps:
      - uses: actions/configure-pages@v5
      - uses: actions/checkout@v4
      - uses: actions/setup-python@v5
        with:
          python-version: 3.x
      - run: pip install zensical
      - name: Build docs
        run: zensical build --clean
        working-directory: docs
      - uses: actions/upload-pages-artifact@v3
        with:
          path: docs/site
      - uses: actions/deploy-pages@v4
        id: deployment
YAML
```

- [ ] **Step 3: Verify both files exist**

```bash
ls ~/Developer/GoHome/.github/workflows/
```

Expected: `ci.yml  docs.yml`

---

## Task 11: Initialize git and make first commit

**Files:** none (git metadata only)

- [ ] **Step 1: Initialize git**

```bash
cd ~/Developer/GoHome && git init -b main
```

- [ ] **Step 2: Stage all files**

```bash
cd ~/Developer/GoHome && git add .
```

- [ ] **Step 3: Confirm node_modules is not staged**

```bash
cd ~/Developer/GoHome && git status --short | grep node_modules
```

Expected: no output. If node_modules appears, the `gohome/.gitignore` excludes it — run `git rm -r --cached gohome/web/node_modules/` to unstage it.

- [ ] **Step 4: Commit**

```bash
cd ~/Developer/GoHome && git commit -m "$(cat <<'EOF'
feat: initialize monorepo at v0.1.0

Consolidates fynn-labs/gohome, fynn-labs/gohome-driverkit, and
fynn-labs/gohome-docs into a single repository. Module paths updated
from github.com/fynn-labs to github.com/fdatoo. Go workspace (go.work)
links both modules, replacing the replace directive in gohome-driverkit.
EOF
)"
```

- [ ] **Step 5: Tag v0.1.0**

```bash
cd ~/Developer/GoHome && git tag v0.1.0
```

---

## Task 12: Push to fdatoo/gohome

- [ ] **Step 1: Inspect current contents of fdatoo/gohome**

```bash
gh repo view fdatoo/gohome --json name,description,defaultBranchRef,pushedAt
```

Note what's there. If anything is worth keeping, save it before proceeding — the force-push in the next step will overwrite it permanently.

- [ ] **Step 2: Add remote and force-push**

```bash
cd ~/Developer/GoHome && git remote add origin git@github.com:fdatoo/gohome.git
git push --force origin main
git push origin v0.1.0
```

`--force` is required because `fdatoo/gohome` has unrelated existing history.

- [ ] **Step 3: Verify the push landed**

```bash
gh repo view fdatoo/gohome --json defaultBranchRef
```

Expected: `"defaultBranchRef": {"name": "main"}` and the ref points to your new commit.

```bash
gh release list --repo fdatoo/gohome 2>/dev/null || true
gh api repos/fdatoo/gohome/tags | jq '.[].name'
```

Expected: `"v0.1.0"` in the tags list.

---

## Task 13: Delete old repositories

Run each deletion separately. The tags in these repos disappear with the repos.

- [ ] **Step 1: Delete fynn-labs/gohome**

```bash
gh repo delete fynn-labs/gohome --yes
```

- [ ] **Step 2: Delete fynn-labs/gohome-driverkit**

```bash
gh repo delete fynn-labs/gohome-driverkit --yes
```

- [ ] **Step 3: Delete fynn-labs/gohome-docs**

```bash
gh repo delete fynn-labs/gohome-docs --yes
```

- [ ] **Step 4: Verify all three are gone**

```bash
gh repo view fynn-labs/gohome 2>&1 | grep -i "not found\|could not resolve"
gh repo view fynn-labs/gohome-driverkit 2>&1 | grep -i "not found\|could not resolve"
gh repo view fynn-labs/gohome-docs 2>&1 | grep -i "not found\|could not resolve"
```

Expected: each prints a "not found" or "could not resolve" message.

---

## Task 14: Archive the old container and update memory

- [ ] **Step 1: Rename the old Desktop container as a fallback**

```bash
mv ~/Desktop/GoHome ~/Desktop/GoHome-old
```

Do NOT delete it yet. Keep it until you've confirmed CI is green on the new monorepo.

- [ ] **Step 2: Verify the new monorepo is the working directory**

```bash
ls ~/Developer/GoHome/
```

Expected: `CLAUDE.md  README.md  go.work  go.work.sum  gohome/  gohome-driverkit/  docs/  .github/`

- [ ] **Step 3: Once CI passes, remove the old container**

After at least one green CI run on `fdatoo/gohome`:

```bash
rm -rf ~/Desktop/GoHome-old
```
