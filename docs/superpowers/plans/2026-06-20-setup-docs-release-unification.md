# Setup, Docs, And Release Unification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Glade easy to install, update, document, extend, and release from one clear docs and distribution path.

**Architecture:** Keep `glade` as the product front door. Keep `glade-tools` as the maintenance and first-party plugin source, but render selected maintainer docs into the same VitePress site under a separate maintainer lane. Use a product release manifest for install/update, and keep plugin artifacts in the plugin registry lane.

**Tech Stack:** Go CLI, VitePress site, shell installer, GitHub Actions, Cloudflare Pages or static hosting, first-party plugin tarballs from `glade-tools`.

---

## Current Starting State

- `/Users/matt/Dev/glade` has uncommitted site doc work:
  - `site/.vitepress/config.ts`
  - `site/docs-src/guide/overview.md`
  - `site/docs-src/guide/tester-field-guide.md`
  - `site/tests/theme.test.mjs`
  - `site/docs-src/guide/ai-assisted-apex.md`
- `/Users/matt/Dev/glade-tools` is clean on `main`.
- `site/install.sh` resolves GitHub release metadata and downloads release assets from GitHub.
- `scripts/release-build.sh` builds one product archive for the runner OS and arch, includes the VS Code extension package, verifies `doctor --json`, and writes `release-manifest.json`.
- `glade-tools/scripts/build-plugin-archives.sh` builds plugin archives and can write a plugin `index.json` when `PLUGIN_ASSET_BASE_URL` is set.
- Public site navigation currently exposes plugin author pages while the registry is still preview.
- Some user-facing LWC docs point directly at sibling `glade-tools` commands.

## Decisions

1. Keep one docs site: `glade.sh`.
2. Use two lanes inside that site:
   - `/guide/...` for users and architects using Glade.
   - `/maintainer/...` for people extending Glade, running parity captures, or cutting releases.
3. Remove plugin authoring from the public user lane for now.
4. Keep `@glade/compat` as the technical plugin name for one release, but present it as "Maintainer support tools" in docs.
5. Do not merge all compat tooling back into base `glade`. The commands are heavy maintenance work.
6. Make install and update use the same manifest contract.
7. Make release proof a command, not a remembered checklist.

## File Structure

### Product Repo: `/Users/matt/Dev/glade`

- Modify: `site/.vitepress/config.ts`
  - Keep user docs short.
  - Add a collapsed `Maintainer` section.
  - Remove public plugin author pages from the visible user path.
- Modify: `site/package.json`
  - Add docs sync and release-link checks.
- Create: `site/scripts/sync-tools-docs.mjs`
  - Copy selected `glade-tools` markdown into `site/docs-src/maintainer/tools/`.
  - Support `--check` for CI.
- Create: `site/scripts/check-doc-routes.mjs`
  - Fail when a nav route points to a missing markdown page.
  - Fail when selected source docs lack a site route or explicit repo-only marker.
- Create: `site/docs-src/maintainer/index.md`
  - Explain the maintainer lane.
- Create: `site/docs-src/maintainer/extend-runtime.md`
  - Show the red-test, product fix, generated docs, and proof loop.
- Create: `site/docs-src/maintainer/release.md`
  - Single release checklist for product, plugins, docs, install, and upgrade proof.
- Create: `site/docs-src/maintainer/glade-tools.md`
  - Explain where `glade-tools` fits.
- Move content out of public nav:
  - `site/docs-src/guide/plugins/build.md`
  - `site/docs-src/guide/plugins/manifest.md`
  - `site/docs-src/guide/plugins/marketplace.md`
  - `site/docs-src/guide/plugins/publish.md`
- Modify: `site/docs-src/guide/plugins.md`
  - Keep first-party plugin use only.
  - Point plugin authoring to maintainer docs only if needed.
- Modify: `site/docs-src/guide/lwc-local-shell.md`
  - Remove direct `cd ../glade-tools` recipes.
  - Link to `/maintainer/glade-tools` for oracle capture.
- Modify: `docs/README.md`
  - Say the docs site is canonical.
  - Leave repo docs as source, generated ledgers, and release runbooks.
- Modify: `docs/PLUGINS.md`
  - Align with the site split.
  - Keep technical plugin contract here or move it to maintainer site content.
- Modify: `site/install.sh`
  - Prefer a product release manifest.
  - Keep GitHub as fallback while releases are private or migration is in flight.
- Create: `internal/gladecli/update_command.go`
  - Add `glade update`.
- Modify: `internal/gladecli/cli.go`
  - Route `update`.
- Modify: `internal/cliui/help.go`
  - Add `update` help and examples.
- Modify: `internal/gladecli/cli_test.go`
  - Test update routing, help, and dry-run behavior.
- Create: `scripts/release-check.sh`
  - One local preflight command.
- Modify: `.github/workflows/ci.yml`
  - Add site tests and site build.
- Modify: `.github/workflows/release.yml`
  - Publish a release manifest suitable for `install.sh` and `glade update`.

### Tools Repo: `/Users/matt/Dev/glade-tools`

- Modify: `README.md`
  - Replace "one migration release" wording with a concrete release fence.
  - Present plugins and maintainer commands as one toolkit.
- Modify: `docs/plugin-registry.md`
  - Keep endpoint setup.
  - Add the product release train relationship.
- Create: `docs/maintainer-site.md`
  - Mark which `glade-tools` docs sync into the product docs site.
- Create: `.github/workflows/ci.yml`
  - Run Go tests and plugin manifest smoke.
- Create: `.github/workflows/release.yml`
  - Build plugin archives for supported platforms.
  - Upload plugin artifacts and generated `index.json`.
- Modify: `scripts/build-plugin-archives.sh`
  - Add a `--check` or `CHECK=1` mode that validates manifests and archive shape without publishing.
- Create: `scripts/release-check.sh`
  - Run `go test ./...`, plugin manifest smoke, and one archive build for the host platform.

## Task 1: Preserve Current Site Work

**Files:**
- Inspect: `site/docs-src/guide/ai-assisted-apex.md`
- Inspect: `site/.vitepress/config.ts`
- Inspect: `site/tests/theme.test.mjs`

- [ ] **Step 1: Confirm current dirty files**

Run:

```bash
git status --short
```

Expected: the AI-assisted Apex guide and its nav/test edits are the only dirty files.

- [ ] **Step 2: Run the existing focused site proof**

Run:

```bash
npm test --prefix site
```

Expected: all site tests pass.

- [ ] **Step 3: Commit current AI guide work before structural changes**

Run:

```bash
git add site/.vitepress/config.ts site/docs-src/guide/overview.md site/docs-src/guide/tester-field-guide.md site/docs-src/guide/ai-assisted-apex.md site/tests/theme.test.mjs
git commit -m "docs: add AI-assisted Apex guide"
```

Expected: clean tree before the larger docs rework begins.

## Task 2: Recut Site Navigation Around One Docs Site

**Files:**
- Modify: `site/.vitepress/config.ts`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Write the nav test first**

Add assertions in `site/tests/theme.test.mjs` that verify:

```js
assert.match(config, /text: 'Maintainer'/);
assert.doesNotMatch(config, /text: 'Build a plugin'/);
assert.doesNotMatch(config, /text: 'Publish'/);
assert.match(config, /link: '\/maintainer\/release'/);
assert.match(config, /link: '\/maintainer\/extend-runtime'/);
```

- [ ] **Step 2: Run test and see it fail**

Run:

```bash
npm test --prefix site
```

Expected: the new nav assertions fail.

- [ ] **Step 3: Update the sidebar**

In `site/.vitepress/config.ts`:

- Keep `Start`, `Workflows`, `Reference`, and `Advanced`.
- Under `Advanced > Plugins`, keep only:
  - `First-party plugins`
  - `Install and manage`
  - `Lock files and CI`
- Add a collapsed `Maintainer` group after `Advanced`:
  - `Maintainer home` -> `/maintainer/`
  - `Extend runtime support` -> `/maintainer/extend-runtime`
  - `Release runbook` -> `/maintainer/release`
  - `glade-tools` -> `/maintainer/glade-tools`

- [ ] **Step 4: Run test again**

Run:

```bash
npm test --prefix site
```

Expected: site tests pass.

- [ ] **Step 5: Commit**

Run:

```bash
git add site/.vitepress/config.ts site/tests/theme.test.mjs
git commit -m "docs: simplify public navigation"
```

## Task 3: Move Plugin Authoring Out Of User Docs

**Files:**
- Modify: `site/docs-src/guide/plugins.md`
- Move or delete:
  - `site/docs-src/guide/plugins/build.md`
  - `site/docs-src/guide/plugins/manifest.md`
  - `site/docs-src/guide/plugins/marketplace.md`
  - `site/docs-src/guide/plugins/publish.md`
- Create: `site/docs-src/maintainer/plugin-runtime.md`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Write route assertions**

Add test assertions:

```js
assert.doesNotMatch(config, /\/guide\/plugins\/build/);
assert.doesNotMatch(config, /\/guide\/plugins\/manifest/);
assert.doesNotMatch(config, /\/guide\/plugins\/marketplace/);
assert.doesNotMatch(config, /\/guide\/plugins\/publish/);
```

- [ ] **Step 2: Replace public plugin page copy**

In `site/docs-src/guide/plugins.md`, keep this shape:

```markdown
# Plugins

Most Glade work does not require plugins. Use plugins when you need a first-party extension that stays outside the base runtime.

## First-party plugins

- `@glade/performance`: project performance scan.
- `@glade/orgpackage`: installed package artifact capture.
- `@glade/compat`: maintainer support matrix, fixtures, and parity tools.

Registry installs are preview. Use a configured registry, direct archive, or linked executable.
```

- [ ] **Step 3: Move authoring material**

Create `site/docs-src/maintainer/plugin-runtime.md` with:

```markdown
# Plugin Runtime

This page is for Glade maintainers and plugin authors. It is not part of the first-run path.

Plugins are executable processes. Glade reads `manifest --json`, validates command roots, and dispatches arguments without loading plugin code into the Glade process.
```

Move useful manifest and archive details into that page. Remove public nav links to the old author pages.

- [ ] **Step 4: Run route and site tests**

Run:

```bash
npm test --prefix site
npm run build --prefix site
```

Expected: tests and build pass.

- [ ] **Step 5: Commit**

Run:

```bash
git add site/docs-src/guide/plugins.md site/docs-src/maintainer/plugin-runtime.md site/tests/theme.test.mjs site/.vitepress/config.ts
git rm site/docs-src/guide/plugins/build.md site/docs-src/guide/plugins/manifest.md site/docs-src/guide/plugins/marketplace.md site/docs-src/guide/plugins/publish.md
git commit -m "docs: move plugin authoring to maintainer lane"
```

## Task 4: Add Maintainer Docs For Extending Glade

**Files:**
- Create: `site/docs-src/maintainer/index.md`
- Create: `site/docs-src/maintainer/extend-runtime.md`
- Create: `site/docs-src/maintainer/glade-tools.md`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Write tests for maintainer docs**

Add assertions that the pages include these exact command strings:

```js
assert.match(maintainerIndex, /glade stays the product front door/);
assert.match(extendRuntime, /Write the failing fixture or product test first/);
assert.match(extendRuntime, /go test \.\/internal\/vm \.\/internal\/apextest/);
assert.match(gladeTools, /go run \.\/cmd\/glade-plugin-compat manifest --json/);
assert.match(gladeTools, /scripts\/build-plugin-archives\.sh 0\.1\.0/);
```

- [ ] **Step 2: Create maintainer home**

`site/docs-src/maintainer/index.md` should explain:

- product repo owns runtime behavior
- tools repo owns maintenance scanners and ledgers
- user docs live in `/guide`
- maintainer docs live in `/maintainer`

- [ ] **Step 3: Create extension workflow page**

`site/docs-src/maintainer/extend-runtime.md` should show this sequence:

```bash
go test ./internal/vm ./internal/apextest
go test ./internal/gladecli
go test ./internal/repoguard
npm test --prefix site
```

It should also spell out:

- add a failing product test for runtime behavior
- add or update a `glade-tools` fixture when the change affects support ledgers
- update generated support docs
- keep unsupported hosted Salesforce behavior explicit

- [ ] **Step 4: Create glade-tools page**

`site/docs-src/maintainer/glade-tools.md` should show:

```bash
cd ../glade-tools
go run ./cmd/glade-tools --help
go run ./cmd/glade-plugin-compat manifest --json
go run ./cmd/glade-plugin-performance manifest --json
go run ./cmd/glade-plugin-orgpackage manifest --json
```

It should describe `@glade/compat` as maintainer support tools, not first-run user setup.

- [ ] **Step 5: Run tests**

Run:

```bash
npm test --prefix site
```

Expected: maintainer docs assertions pass.

- [ ] **Step 6: Commit**

Run:

```bash
git add site/docs-src/maintainer site/tests/theme.test.mjs site/.vitepress/config.ts
git commit -m "docs: add maintainer extension guide"
```

## Task 5: Sync Selected glade-tools Docs Into The Site

**Files:**
- Create: `site/scripts/sync-tools-docs.mjs`
- Modify: `site/package.json`
- Create: `site/docs-src/maintainer/tools/README.md`
- Modify in tools repo: `/Users/matt/Dev/glade-tools/docs/maintainer-site.md`

- [ ] **Step 1: Add sync manifest in `glade-tools`**

Create `/Users/matt/Dev/glade-tools/docs/maintainer-site.md`:

```markdown
# Maintainer Site Sources

These pages are safe to render in the Glade docs site:

- `README.md`
- `docs/plugin-registry.md`
- `docs/CAPABILITY_WORK_QUEUE.md`
- `docs/visualforce-oracle.md`
- `docs/reports/lwc-shell-oracle-support.md`

Do not sync raw fixture JSON, generated JSON, local reports, or private corpus paths.
```

- [ ] **Step 2: Add sync script**

Create `site/scripts/sync-tools-docs.mjs` that:

- reads from `../glade-tools`
- copies only the listed markdown files
- writes to `site/docs-src/maintainer/tools/`
- rewrites top-level headings with a `glade-tools:` prefix when needed
- supports `--check` by comparing generated content without writing

- [ ] **Step 3: Wire the script into site commands**

Modify `site/package.json`:

```json
{
  "scripts": {
    "sync:tools-docs": "node scripts/sync-tools-docs.mjs",
    "generate:editor-support": "node scripts/build-editor-support.mjs",
    "dev": "npm run sync:tools-docs && vitepress dev .",
    "build": "npm run sync:tools-docs -- --check && npm run generate:editor-support -- --check && vitepress build . && cp install.sh .vitepress/dist/install.sh",
    "test": "npm run sync:tools-docs -- --check && npm run generate:editor-support -- --check && node --test tests/*.test.mjs",
    "preview": "vitepress preview ."
  }
}
```

- [ ] **Step 4: Run tests**

Run:

```bash
npm test --prefix site
npm run build --prefix site
```

Expected: synced maintainer docs exist and site build passes.

- [ ] **Step 5: Commit both repos**

In `/Users/matt/Dev/glade-tools`:

```bash
git add docs/maintainer-site.md
git commit -m "docs: mark maintainer site sources"
```

In `/Users/matt/Dev/glade`:

```bash
git add site/package.json site/scripts/sync-tools-docs.mjs site/docs-src/maintainer/tools
git commit -m "docs: sync tools docs into site"
```

## Task 6: Remove Direct glade-tools Recipes From User Pages

**Files:**
- Modify: `site/docs-src/guide/lwc-local-shell.md`
- Modify: `site/docs-src/guide/local-testing.md`
- Modify: `docs/LWC_SUPPORT.md`
- Modify: `docs/LWC_LOCAL_SHELL.md`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Add tests that protect user pages**

Add assertions:

```js
assert.doesNotMatch(lwcLocalShell, /cd \.\.\/glade-tools/);
assert.doesNotMatch(localTesting, /cd \.\.\/glade-tools/);
assert.match(lwcLocalShell, /maintainer\/glade-tools/);
```

- [ ] **Step 2: Rewrite user pages**

Replace direct sibling checkout commands with:

```markdown
Maintainers can run browser oracle captures from the `glade-tools` lane. See [glade-tools maintainer guide](/maintainer/glade-tools).
```

- [ ] **Step 3: Keep repo docs technical**

In `docs/LWC_SUPPORT.md` and `docs/LWC_LOCAL_SHELL.md`, either:

- point to the maintainer site route, or
- keep the commands but mark the page as repo maintainer reference at the top.

- [ ] **Step 4: Run tests**

Run:

```bash
npm test --prefix site
```

Expected: user pages no longer tell first-run users to operate sibling tooling.

- [ ] **Step 5: Commit**

Run:

```bash
git add site/docs-src/guide/lwc-local-shell.md site/docs-src/guide/local-testing.md docs/LWC_SUPPORT.md docs/LWC_LOCAL_SHELL.md site/tests/theme.test.mjs
git commit -m "docs: move oracle commands to maintainer lane"
```

## Task 7: Improve Install And Update

**Files:**
- Modify: `site/install.sh`
- Create: `internal/gladecli/update_command.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/gladecli/cli_test.go`
- Modify: `internal/cliui/help.go`
- Modify: `site/docs-src/guide/installation.md`
- Modify: `docs/INSTALL.md`

- [ ] **Step 1: Add CLI tests for `glade update`**

In `internal/gladecli/cli_test.go`, add tests:

```go
func TestRunUpdateHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"update", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "glade update") {
		t.Fatalf("help omitted update usage:\n%s", stdout.String())
	}
}
```

Add a dry-run test:

```go
func TestRunUpdateDryRunPrintsInstallCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"update", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "curl -fsSL https://glade.sh/install.sh | sh") {
		t.Fatalf("dry run omitted install command:\n%s", stdout.String())
	}
}
```

- [ ] **Step 2: Implement minimal `glade update`**

`glade update --dry-run` prints the installer command and current version.

`glade update` runs:

```bash
curl -fsSL https://glade.sh/install.sh | sh
```

The first cut may refuse to run when `GLADE_UPDATE_ALLOW_SHELL=1` is not set. If that guard is used, the command prints the exact command to run. This avoids surprising self-replacement behavior.

- [ ] **Step 3: Add help**

Add `update` to `internal/cliui/help.go`:

```go
{
	Name:        "update",
	Description: "Update the Glade binary and bundled assets.",
	Usage:       []string{"glade update [--dry-run]"},
	Flags:       []FlagHelp{{Name: "--dry-run", Description: "Print the update command without running it."}},
	Examples:    []string{"glade update --dry-run", "glade update"},
}
```

- [ ] **Step 4: Make installer manifest-first**

Update `site/install.sh` so it checks:

```text
https://downloads.glade.sh/index.json
https://downloads.glade.sh/latest/release-manifest.json
```

If those fail, fall back to the current GitHub API flow.

- [ ] **Step 5: Document upgrade**

In both install docs, add:

```bash
glade update --dry-run
glade update
glade version
glade doctor
```

- [ ] **Step 6: Run proof**

Run:

```bash
go test ./internal/gladecli ./internal/cliui
npm test --prefix site
```

- [ ] **Step 7: Commit**

Run:

```bash
git add site/install.sh internal/gladecli/update_command.go internal/gladecli/cli.go internal/gladecli/cli_test.go internal/cliui/help.go site/docs-src/guide/installation.md docs/INSTALL.md
git commit -m "feat: add update path"
```

## Task 8: Add One Release Check Command

**Files:**
- Create: `scripts/release-check.sh`
- Modify: `docs/DISTRIBUTION_WORKFLOW.md`
- Modify: `docs/RELEASE_POLICY.md`
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Create release check script**

Create `scripts/release-check.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

git diff --check
go test ./internal/repoguard
go test ./internal/gladecli ./internal/cliui
npm test --prefix site
npm run build --prefix site
go test ./...
scripts/smoke.sh
```

- [ ] **Step 2: Make it executable**

Run:

```bash
chmod +x scripts/release-check.sh
```

- [ ] **Step 3: Add CI coverage**

In `.github/workflows/ci.yml`, add:

```yaml
      - run: npm ci --prefix site
      - run: npm test --prefix site
      - run: npm run build --prefix site
```

- [ ] **Step 4: Update release docs**

Replace separate first-step commands with:

```bash
scripts/release-check.sh
```

Keep the manual real-project check after it.

- [ ] **Step 5: Run proof**

Run:

```bash
scripts/release-check.sh
```

Expected: all checks pass before release work proceeds.

- [ ] **Step 6: Commit**

Run:

```bash
git add scripts/release-check.sh docs/DISTRIBUTION_WORKFLOW.md docs/RELEASE_POLICY.md .github/workflows/ci.yml
git commit -m "chore: add release check command"
```

## Task 9: Smooth Product Release Publishing

**Files:**
- Modify: `scripts/release-build.sh`
- Modify: `.github/workflows/release.yml`
- Modify: `site/install.sh`
- Modify: `docs/DISTRIBUTION_WORKFLOW.md`

- [ ] **Step 1: Extend manifest shape**

Add these fields to `dist/release-manifest.json`:

```json
{
  "schemaVersion": 2,
  "channel": "stable",
  "version": "v0.2.0",
  "assets": [
    {
      "os": "darwin",
      "arch": "arm64",
      "url": "https://downloads.glade.sh/v0.2.0/glade_v0.2.0_darwin_arm64.tar.gz",
      "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
    }
  ],
  "installScript": "https://glade.sh/install.sh",
  "pluginRegistry": "https://plugins.glade.sh/index.json"
}
```

- [ ] **Step 2: Publish product index**

In `.github/workflows/release.yml`, after assembling checksums, also assemble:

```text
dist/index.json
dist/latest/release-manifest.json
```

The first version can upload these as GitHub release assets. The hosting job can copy them to `downloads.glade.sh` later.

- [ ] **Step 3: Keep GitHub fallback**

`site/install.sh` should:

1. try `GLADE_DOWNLOAD_BASE` or `https://downloads.glade.sh`
2. fall back to GitHub API
3. verify SHA-256 in both paths

- [ ] **Step 4: Run local release build**

Run:

```bash
VERSION=v0.2.0 DIST_DIR=/tmp/glade-release scripts/release-build.sh
ls /tmp/glade-release
cat /tmp/glade-release/release-manifest.json
```

Expected: archive, `.sha256`, `SHA256SUMS.txt`, and manifest exist.

- [ ] **Step 5: Commit**

Run:

```bash
git add scripts/release-build.sh .github/workflows/release.yml site/install.sh docs/DISTRIBUTION_WORKFLOW.md
git commit -m "chore: publish release manifest"
```

## Task 10: Add glade-tools CI And Release Rail

**Files in `/Users/matt/Dev/glade-tools`:**
- Create: `.github/workflows/ci.yml`
- Create: `.github/workflows/release.yml`
- Create: `scripts/release-check.sh`
- Modify: `scripts/build-plugin-archives.sh`
- Modify: `README.md`
- Modify: `docs/plugin-registry.md`

- [ ] **Step 1: Add tools release check**

Create `scripts/release-check.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

git diff --check
go test ./...
go run ./cmd/glade-plugin-compat manifest --json >/tmp/glade-plugin-compat-manifest.json
go run ./cmd/glade-plugin-performance manifest --json >/tmp/glade-plugin-performance-manifest.json
go run ./cmd/glade-plugin-orgpackage manifest --json >/tmp/glade-plugin-orgpackage-manifest.json
OUT_DIR=/tmp/glade-plugin-release TARGETS="$(go env GOOS)/$(go env GOARCH)" scripts/build-plugin-archives.sh 0.2.0
```

- [ ] **Step 2: Add CI workflow**

Create `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v6
        with:
          go-version: "1.26.3"
      - run: scripts/release-check.sh
```

- [ ] **Step 3: Add release workflow**

Create `.github/workflows/release.yml` that runs on tags `v*`, builds plugin archives with `scripts/build-plugin-archives.sh`, uploads tarballs, `checksums.txt`, and `index.json`.

- [ ] **Step 4: Document release pairing**

In `README.md` and `docs/plugin-registry.md`, state:

- product release assets go to `downloads.glade.sh`
- plugin assets go to `plugins.glade.sh`
- both can share the same version tag
- `@glade/compat` is maintainer-facing

- [ ] **Step 5: Run proof**

Run:

```bash
cd /Users/matt/Dev/glade-tools
scripts/release-check.sh
```

- [ ] **Step 6: Commit**

Run:

```bash
git add .github/workflows/ci.yml .github/workflows/release.yml scripts/release-check.sh scripts/build-plugin-archives.sh README.md docs/plugin-registry.md
git commit -m "chore: add tools release rail"
```

## Task 11: Reframe compat Without Breaking Scripts

**Files:**
- Modify in Glade: `site/docs-src/guide/plugins.md`
- Modify in Glade: `site/docs-src/maintainer/glade-tools.md`
- Modify in tools: `/Users/matt/Dev/glade-tools/README.md`
- Modify in tools: `/Users/matt/Dev/glade-tools/plugins/compat/plugin.json`
- Modify in tools: `/Users/matt/Dev/glade-tools/scripts/build-plugin-archives.sh`

- [ ] **Step 1: Keep the package name**

Do not rename `@glade/compat` in this pass.

- [ ] **Step 2: Change summaries**

Use "Maintainer support tools" in summaries:

```text
Maintainer support tools, fixtures, surface ledgers, and parity scanners.
```

- [ ] **Step 3: Remove compat from first-run docs**

Public install examples should not include:

```bash
glade plugins install @glade/compat
```

unless the page is a maintainer page.

- [ ] **Step 4: Preserve existing command roots**

Keep current roots for one release:

```text
compat
surface
local-tests
post-parity
dashboard
gaps
stdlib
```

- [ ] **Step 5: Add future rename note**

In maintainer docs, say a later release may introduce `@glade/maintainer` as a friendlier name after the registry and release train are stable.

- [ ] **Step 6: Run proof**

Run in tools:

```bash
go run ./cmd/glade-plugin-compat manifest --json
go test ./...
```

Run in product:

```bash
npm test --prefix site
```

## Task 12: Add Route And Docs Inventory Guards

**Files:**
- Create: `site/scripts/check-doc-routes.mjs`
- Modify: `site/package.json`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Add route checker**

Create a Node script that:

- imports or parses `site/.vitepress/config.ts`
- collects nav/sidebar links beginning with `/`
- maps `/guide/foo` to `site/docs-src/guide/foo.md`
- maps `/maintainer/foo` to `site/docs-src/maintainer/foo.md`
- fails on missing files

- [ ] **Step 2: Add package script**

Add:

```json
"check:routes": "node scripts/check-doc-routes.mjs"
```

Run it from `test` and `build`.

- [ ] **Step 3: Add public wording guard**

Extend `site/tests/theme.test.mjs` to fail on:

```js
assert.doesNotMatch(allPublicGuideText, /cd \.\.\/glade-tools/);
assert.doesNotMatch(allPublicGuideText, /oaer-probe-max/);
assert.doesNotMatch(allPublicGuideText, /Salesforce Docs Scraper/);
```

- [ ] **Step 4: Run proof**

Run:

```bash
npm test --prefix site
npm run build --prefix site
```

## Task 13: Final Integrated Proof

**Files:**
- No source files should change in this task.

- [ ] **Step 1: Product proof**

Run:

```bash
cd /Users/matt/Dev/glade
git diff --check
scripts/release-check.sh
```

- [ ] **Step 2: Tools proof**

Run:

```bash
cd /Users/matt/Dev/glade-tools
git diff --check
scripts/release-check.sh
```

- [ ] **Step 3: Install proof from local release**

Run:

```bash
cd /Users/matt/Dev/glade
VERSION=v0.2.0 DIST_DIR=/tmp/glade-release scripts/release-build.sh
GLADE_INSTALL_DIR=/tmp/glade-install GLADE_DOWNLOAD_BASE=file:///tmp/glade-release sh site/install.sh
/tmp/glade-install/glade version
/tmp/glade-install/glade doctor
```

If `file://` support is not added to `site/install.sh`, use a local static server:

```bash
cd /tmp/glade-release
python3 -m http.server 8765
```

Then run the installer with:

```bash
GLADE_INSTALL_DIR=/tmp/glade-install GLADE_DOWNLOAD_BASE=http://127.0.0.1:8765 sh /Users/matt/Dev/glade/site/install.sh
```

- [ ] **Step 4: Plugin proof from local registry**

Run:

```bash
cd /Users/matt/Dev/glade-tools
OUT_DIR=/tmp/glade-plugins TARGETS="$(go env GOOS)/$(go env GOARCH)" PLUGIN_ASSET_BASE_URL="http://127.0.0.1:8766" scripts/build-plugin-archives.sh 0.2.0
cd /tmp/glade-plugins
python3 -m http.server 8766
```

In another terminal:

```bash
GLADE_HOME=/tmp/glade-home GLADE_PLUGIN_REGISTRY_URL=http://127.0.0.1:8766/index.json glade plugins available --no-progress
GLADE_HOME=/tmp/glade-home GLADE_PLUGIN_REGISTRY_URL=http://127.0.0.1:8766/index.json glade plugins install @glade/performance --yes --no-progress
GLADE_HOME=/tmp/glade-home glade plugins which performance
```

- [ ] **Step 5: Commit final docs and release rail changes**

Run in each repo:

```bash
git status --short
```

Commit any remaining planned changes with focused messages.

## Rollout Order

1. Commit the current AI-assisted Apex guide.
2. Simplify public docs navigation.
3. Add maintainer docs and sync selected `glade-tools` docs into the single site.
4. Remove direct sibling-tool recipes from user pages.
5. Add update command and manifest-first installer.
6. Add product release check.
7. Add `glade-tools` CI and release check.
8. Reframe `@glade/compat` as maintainer support tools.
9. Add docs route and wording guards.
10. Run final integrated product plus plugin install proof.

## Self-Review

- Spec coverage: setup, single docs site, removing plugin authoring, compat organization, DX, DevOps, release, and upgrade are covered.
- Product boundary: base `glade` remains product-facing; heavy scanners stay in `glade-tools` or first-party plugins.
- Release boundary: product downloads and plugin registry stay separate.
- User path: first-run docs stay short. Maintainer paths are available in the same site.
- Risk: `glade update` can surprise users if it self-replaces. The first cut should support `--dry-run` and either prompt or require an explicit environment opt-in for execution.
