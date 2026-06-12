# Public Site Launch Feedback Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the landing page and docs into a public-ready Glade launch surface for `glade.sh`.

**Architecture:** Keep VitePress as the site shell. Add a beginner-first docs path, make the homepage proof more concrete, and keep generated compatibility reports behind the public support map. Treat `glade.sh` as the canonical production domain once GitHub Pages or Cloudflare Pages serves this repository's site artifact.

**Tech Stack:** VitePress 1.x, Vue components already registered in `site/.vitepress/theme/index.ts`, plain browser JavaScript in `site/docs-src/public/js/home.js`, CSS in `site/.vitepress/theme/custom.css`, site tests in `site/tests/theme.test.mjs`, shell installer in `site/install.sh`.

---

## Current Review Input

Source review: `/Users/matt/Downloads/glade_landing_docs_review_updated.md`

Top findings to address:

1. `glade.sh/install.sh` must serve this product's installer before public traffic.
2. Docs need a beginner-first path before plugin-author, brand, and maintainer material.
3. The landing page needs stronger proof: a real install verification path, clearer playground CTA, default success in the proof panel, runtime-map links, and a short local loop.
4. Core docs need output examples, direct task titles, and support-limit honesty.
5. Advanced/internal pages should stay public but move out of the first-run path.

Do not touch unrelated dirty files. Check `git status --short` at execution
time and preserve any local work outside this plan's write set.

## Implementation Map

Use separate subagents where possible:

- **Worker A, Launch trust and homepage:** `site/docs-src/index.md`, `site/docs-src/public/js/home.js`, `site/.vitepress/theme/custom.css`, `site/install.sh`, `site/README.md`, `.github/workflows/pages.yml`, `site/tests/theme.test.mjs`.
- **Worker B, Docs IA and first-run pages:** `site/.vitepress/config.ts`, create `site/docs-src/guide/overview.md`, create `site/docs-src/guide/quickstart.md`, update `site/docs-src/guide/installation.md`.
- **Worker C, Core workflow docs:** `site/docs-src/guide/configuration.md`, `site/docs-src/guide/local-testing.md`, `site/docs-src/guide/affected-tests.md`, `site/docs-src/guide/ci-artifacts.md`, `site/docs-src/guide/local-api-server.md`, `site/docs-src/guide/playground.md`.
- **Worker D, Support and advanced docs:** `site/docs-src/guide/support-map.md`, `site/docs-src/guide/compatibility.md`, `site/docs-src/guide/compatibility-dashboard.md`, `site/docs-src/guide/plugins*.md`, `site/docs-src/guide/rich-local-workflows.md`, `site/docs-src/guide/editor.md`, `site/docs-src/guide/test-startup-cache.md`.

Worker boundaries are write ownership. Workers are not alone in the codebase. They must not revert edits outside their files.

## Phase 0: Baseline And Worktree Check

### Task 0.1: Capture The Dirty State

**Files:**
- Read only: `/Users/matt/Dev/glade`

- [ ] **Step 1: Check status before editing**

Run:

```bash
git status --short
```

Expected: note any modified files. Do not revert unrelated changes.

- [ ] **Step 2: Check existing site gates**

Run:

```bash
cd site
npm test
npm run build
```

Expected: both commands pass before edits. If they fail, record exact failures in the task notes before changing files.

## Phase 1: Launch Trust And Domain Readiness

### Task 1.1: Make The Install Surface Launch-Safe

**Files:**
- Modify: `site/docs-src/index.md`
- Modify: `site/docs-src/guide/installation.md`
- Modify: `site/README.md`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Update the homepage install block**

In `site/docs-src/index.md`, keep the production command but add immediate verification commands below the install metadata:

```html
<div class="home-install-verify">
  <code>glade version</code>
  <code>glade doctor</code>
</div>
```

Keep `https://glade.sh/install.sh` as the URL because `glade.sh` is the launch domain.

- [ ] **Step 2: Add manual fallback links on the homepage**

In the same install block, add links:

```html
<a class="home-install-link" href="https://github.com/glade-sh/glade/releases">Releases</a>
<a class="home-install-link" href="https://github.com/glade-sh/glade/releases/latest/download/SHA256SUMS.txt">Checksums</a>
```

Expected: the install block offers script, release, checksum, and verification paths.

- [ ] **Step 3: Rewrite the installation opening**

Replace the opening paragraph of `site/docs-src/guide/installation.md` with:

```markdown
Glade ships as a single local binary for macOS and Linux. Install it, verify
your environment with `glade doctor`, then run your first project check from an
SFDX workspace.
```

- [ ] **Step 4: Add supported platform table**

After `## One-line install`, add:

```markdown
| OS | CPU | Status |
| --- | --- | --- |
| macOS | arm64 | supported release archive |
| macOS | amd64 | supported release archive |
| Linux | amd64 | supported release archive |
| Linux | arm64 | supported release archive |
| Windows | amd64/arm64 | build from source for now |
```

- [ ] **Step 5: Add expected doctor output**

After the `glade doctor` command, add:

```text
glade doctor
parser: ok (tree-sitter)
```

Do not invent other doctor lines unless current CLI output is verified in this repo.

- [ ] **Step 6: Add PATH troubleshooting**

Add a `## Troubleshooting` section with this content:

````markdown
If `glade` is not found after install, add `~/.local/bin` to your shell path:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Then open a new shell and run:

```bash
glade version
glade doctor
```
````

- [ ] **Step 7: Update tests**

In `site/tests/theme.test.mjs`, add assertions that `index` contains:

```js
assert.match(index, /glade version/);
assert.match(index, /glade doctor/);
assert.match(index, /Releases/);
assert.match(index, /Checksums/);
```

- [ ] **Step 8: Verify**

Run:

```bash
cd site
npm test
npm run build
```

Expected: all site tests pass and VitePress builds.

### Task 1.2: Add Deployment Smoke Checks

**Files:**
- Modify: `site/README.md`
- Modify: `.github/workflows/pages.yml`

- [ ] **Step 1: Document the deployment smoke check**

Add this section to `site/README.md`:

````markdown
## Launch Smoke Check

After GitHub Pages or Cloudflare Pages points `glade.sh` at this site, verify
the public routes:

```bash
curl -fsSL https://glade.sh/install.sh | head -n 5
curl -fsSI https://glade.sh/install.sh | grep -i content-type
curl -fsSL https://glade.sh/docs/guide/support-map >/dev/null
curl -fsSL https://glade.sh/ >/dev/null
```

`/install.sh` must return shell script text, not the legacy project HTML.
````

- [ ] **Step 2: Add post-build artifact check**

In `.github/workflows/pages.yml`, add this step after `Assemble Pages artifact`:

```yaml
      - name: Check Pages artifact routes
        run: |
          set -euo pipefail
          test -s site-pages/install.sh
          grep -q 'repo="glade-sh/glade"' site-pages/install.sh
          test -s site-pages/CNAME
          grep -q '^glade.sh$' site-pages/CNAME
```

- [ ] **Step 3: Verify**

Run:

```bash
cd site
npm run build
cd ..
git diff --check
```

Expected: build succeeds and diff check is clean.

## Phase 2: Beginner-First Information Architecture

### Task 2.1: Add Overview Page

**Files:**
- Create: `site/docs-src/guide/overview.md`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Create overview page**

Create `site/docs-src/guide/overview.md`:

````markdown
# What is Glade?

Glade is a local Apex runtime and developer workbench. It loads Salesforce DX
projects, parses and checks supported Apex, runs local Apex tests, executes
anonymous Apex, serves a Salesforce-shaped local REST API, and exposes support
gaps instead of hiding them.

## First local loop

```bash
glade doctor
glade init --project . --yes
glade check --project .
glade test --project . --filter AccountServiceTest
glade test changed --project . --since origin/main
```

## Use Glade When

- You want Apex diagnostics before a deploy.
- You want to run supported Apex tests without logging into an org.
- You want local SOQL, DML, trigger, SObject, and limit feedback.
- You want a Salesforce-shaped local API for development loops.

## Use Salesforce When

- You need live auth, sessions, identity, or org-hosted process engines.
- You need full Visualforce rendering or PDF generation.
- You need Bulk API, Streaming, Pub/Sub, GraphQL, or broad Tooling API parity.
- You need exact production governor accounting.

## Support Claims

Glade models the local paths it can prove. Unsupported platform services fail
with stable diagnostics instead of pretending to work.

Next: [Quickstart](/guide/quickstart) or [What Glade supports](/guide/support-map).
````

- [ ] **Step 2: Add test coverage**

In `site/tests/theme.test.mjs`, read the overview file and assert:

```js
const overview = await readFile(new URL("../docs-src/guide/overview.md", import.meta.url), "utf8");

assert.match(overview, /^# What is Glade\?/m);
assert.match(overview, /Glade models the local paths it can prove/);
assert.match(overview, /Use Salesforce When/);
```

- [ ] **Step 3: Verify**

Run:

```bash
cd site
npm test
```

Expected: tests pass.

### Task 2.2: Add Quickstart Page

**Files:**
- Create: `site/docs-src/guide/quickstart.md`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Create quickstart**

Create `site/docs-src/guide/quickstart.md`:

````markdown
# Quickstart: Check and Test an SFDX Project

This path gets from install to the first local check in a few minutes.

## 1. Install

```bash
curl -fsSL https://glade.sh/install.sh | sh
glade version
glade doctor
```

Expected:

```text
parser: ok (tree-sitter)
```

## 2. Open an SFDX project

```bash
cd path/to/sfdx-project
glade init --project . --yes
glade config validate --project .
```

## 3. Check source

```bash
glade check --project .
```

## 4. Run one test

```bash
glade test --project . --filter AccountServiceTest
```

## 5. Run only affected tests

```bash
glade test changed --project . --since origin/main
```

## 6. Know the limits

Glade is not a full Salesforce emulator. Check [What Glade supports](/guide/support-map)
before relying on platform service APIs, live auth, Visualforce rendering, or
full REST/Tooling API parity.
````

- [ ] **Step 2: Add test coverage**

In `site/tests/theme.test.mjs`, read the quickstart file and assert:

```js
const quickstart = await readFile(new URL("../docs-src/guide/quickstart.md", import.meta.url), "utf8");

assert.match(quickstart, /^# Quickstart: Check and Test an SFDX Project/m);
assert.match(quickstart, /glade check --project \./);
assert.match(quickstart, /glade test changed --project \. --since origin\/main/);
```

- [ ] **Step 3: Verify**

Run:

```bash
cd site
npm test
```

Expected: tests pass.

### Task 2.3: Reorder Sidebar

**Files:**
- Modify: `site/.vitepress/config.ts`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Replace sidebar groups**

Update `themeConfig.sidebar` to this structure while keeping existing paths:

```ts
sidebar: [
  {
    text: 'Start',
    items: [
      { text: 'Overview', link: '/guide/overview' },
      { text: 'Quickstart', link: '/guide/quickstart' },
      { text: 'Installation', link: '/guide/installation' },
      { text: 'What Glade Supports', link: '/guide/support-map' }
    ]
  },
  {
    text: 'Core Workflows',
    items: [
      { text: 'Configure A Glade Project', link: '/guide/configuration' },
      { text: 'CLI Reference', link: '/guide/cli-reference' },
      { text: 'Run Apex Tests Locally', link: '/guide/local-testing' },
      { text: 'Run Only Affected Tests', link: '/guide/affected-tests' },
      { text: 'Use The Local Playground', link: '/guide/playground' },
      { text: 'Run A Local Salesforce-Shaped API', link: '/guide/local-api-server' },
      { text: 'Add Glade To CI', link: '/guide/ci-artifacts' }
    ]
  },
  {
    text: 'Advanced',
    items: [
      { text: 'Speed Up Test Startup', link: '/guide/test-startup-cache' },
      { text: 'Editor, LSP, and DAP', link: '/guide/editor' },
      { text: 'Progress, Wizards, and Package Artifacts', link: '/guide/rich-local-workflows' }
    ]
  },
  {
    text: 'Plugin Authors',
    items: [
      { text: 'Use Plugins', link: '/guide/plugins' },
      { text: 'First-Party Plugins', link: '/guide/plugins/first-party' },
      { text: 'Marketplace And Trust', link: '/guide/plugins/marketplace' },
      { text: 'Install And Manage', link: '/guide/plugins/install-manage' },
      { text: 'Build A Plugin', link: '/guide/plugins/build' },
      { text: 'Manifest Reference', link: '/guide/plugins/manifest' },
      { text: 'Publish A Plugin', link: '/guide/plugins/publish' },
      { text: 'Plugin Lock Files And CI', link: '/guide/plugins/lock-ci' }
    ]
  },
  {
    text: 'Project',
    items: [
      { text: 'Compatibility Policy', link: '/guide/compatibility' },
      { text: 'Maintainer Proof Reports', link: '/guide/compatibility-dashboard' },
      { text: 'Brand Guide', link: '/guide/brand-guide' }
    ]
  }
]
```

- [ ] **Step 2: Update nav label for Playground**

If `/guide/playground` remains a docs page, change top nav text from `Playground` to `Playground Docs`.

- [ ] **Step 3: Update tests**

Add assertions:

```js
assert.match(config, /text: 'Overview'/);
assert.match(config, /text: 'Quickstart'/);
assert.match(config, /text: 'What Glade Supports'/);
assert.ok(config.indexOf("text: 'Overview'") < config.indexOf("text: 'Brand Guide'"));
assert.doesNotMatch(config, /text: 'Project Status'/);
```

- [ ] **Step 4: Verify**

Run:

```bash
cd site
npm test
npm run build
```

Expected: tests pass and build succeeds.

## Phase 3: Landing Page Proof

### Task 3.1: Clarify Playground CTA

**Files:**
- Modify: `site/docs-src/index.md`
- Modify: `site/docs-src/public/js/home.js`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Rename homepage CTA**

In `site/docs-src/index.md`, change:

```yaml
text: Open Playground
link: /guide/playground
```

to:

```yaml
text: Playground Docs
link: /guide/playground
```

Use `Open Playground` only if a hosted browser app route exists.

- [ ] **Step 2: Rename command palette item**

In `site/docs-src/public/js/home.js`, change the command palette label from `Open Playground` to `Playground Docs`.

- [ ] **Step 3: Update tests**

Replace the old assertion:

```js
assert.match(index, /text: Open Playground/);
```

with:

```js
assert.match(index, /text: Playground Docs/);
assert.doesNotMatch(index, /text: Open Playground/);
```

- [ ] **Step 4: Verify**

Run:

```bash
cd site
npm test
```

Expected: tests pass.

### Task 3.2: Make The Proof Panel Show Success By Default

**Files:**
- Modify: `site/docs-src/index.md`
- Modify: `site/docs-src/public/js/home.js`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Change default HTML output**

In `site/docs-src/index.md`, replace the four default output values:

```html
<strong data-output-key="status">Pass</strong>
<strong data-output-key="timing">38 ms</strong>
<strong data-output-key="log">USER_DEBUG | Account count: 1</strong>
<strong data-output-key="state">1 Account inserted · rolled back after run</strong>
```

Set the status class:

```html
<strong class="home-output-pass" data-output-key="status">Pass</strong>
```

- [ ] **Step 2: Update idle state in JavaScript**

In `site/docs-src/public/js/home.js`, set `examples.account.idle` to the same success state. Keep `soql` and `rollback` idle states as neutral preview states with `status: "Not run"`, `timing: "--"`, and short state text.

```js
idle: {
  status: "Pass",
  timing: "38 ms",
  log: "USER_DEBUG | Account count: 1",
  state: "1 Account inserted · rolled back after run"
}
```

- [ ] **Step 3: Preserve interaction**

Keep the run button. It can still replay the same success state after click.

- [ ] **Step 4: Update tests**

Change test expectations:

```js
assert.match(index, /data-output-key="status"[\s\S]*Pass/);
assert.match(index, /data-output-key="timing"[\s\S]*38 ms/);
assert.match(index, /data-output-key="log"[\s\S]*USER_DEBUG \| Account count: 1/);
assert.match(index, /data-output-key="state"[\s\S]*rolled back after run/);
assert.doesNotMatch(index, /Run Example to see output/);
```

- [ ] **Step 5: Verify**

Run:

```bash
cd site
npm test
npm run build
```

Expected: tests pass and build succeeds.

### Task 3.3: Add Local Loop Section

**Files:**
- Modify: `site/docs-src/index.md`
- Modify: `site/.vitepress/theme/custom.css`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Add section after feature cards**

Insert:

```html
<div class="home-section home-loop-section">
  <p class="home-eyebrow">LOCAL LOOP</p>
  <h2 class="home-h2">The local loop before the deploy loop.</h2>
  <pre class="home-code-block"><code class="language-bash">glade check --project .
glade test --project . --filter AccountServiceTest
glade test changed --project . --since origin/main
glade playground --project . --open</code></pre>
  <p class="home-p">Run the checks that fit your edit before Salesforce enters the path.</p>
</div>
```

- [ ] **Step 2: Add compact section CSS**

If spacing is too loose, add:

```css
.home-loop-section {
  max-width: 960px;
  margin-inline: auto;
}
```

- [ ] **Step 3: Add tests**

Add:

```js
assert.match(index, /The local loop before the deploy loop\./);
assert.match(index, /glade playground --project \. --open/);
```

- [ ] **Step 4: Verify**

Run:

```bash
cd site
npm test
npm run build
```

Expected: tests pass and build succeeds.

### Task 3.4: Link Runtime Map To Support Map

**Files:**
- Modify: `site/docs-src/index.md`
- Modify: `site/.vitepress/theme/custom.css`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Convert runtime cards to links**

Change each `.home-runtime-card` from `div` to `a`:

```html
<a class="home-runtime-card" href="/guide/support-map#works-well">
  <span>parse / sema</span>
  <h3>Apex front end</h3>
  <p>Source model, symbols, grouping, diagnostics, and lowering.</p>
  <small>View parser and semantic support →</small>
</a>
```

Use these targets:

- Apex front end: `/guide/support-map#works-well`
- Local execution: `/guide/support-map#works-well`
- Visible support map: `/guide/support-map#not-supported-today`

- [ ] **Step 2: Style the card action**

Add:

```css
.home-runtime-card small {
  color: var(--vp-c-brand-1);
  font-weight: 700;
}
```

- [ ] **Step 3: Add support-limit sentence**

Below the runtime cards, add:

```html
<p class="home-support-note">
  Glade models the local paths it can prove. Unsupported platform services fail with stable diagnostics instead of pretending to work.
  <a href="/guide/support-map">View the support map</a>.
</p>
```

- [ ] **Step 4: Add tests**

Add:

```js
assert.match(index, /href="\/guide\/support-map#works-well"/);
assert.match(index, /Unsupported platform services fail with stable diagnostics/);
assert.match(css, /\.home-runtime-card small/);
```

- [ ] **Step 5: Verify**

Run:

```bash
cd site
npm test
npm run build
```

Expected: tests pass and build succeeds.

## Phase 4: Core Docs Depth

### Task 4.1: Strengthen Configuration Docs

**Files:**
- Modify: `site/docs-src/guide/configuration.md`

- [ ] **Step 1: Rename title**

Change:

```markdown
# Project Configuration
```

to:

```markdown
# Configure a Glade Project
```

- [ ] **Step 2: Add precedence**

Add:

```markdown
## Configuration Precedence

Glade resolves settings in this order:

1. CLI flags
2. `glade.yml`
3. `sfdx-project.json`
4. Glade defaults
```

- [ ] **Step 3: Clarify YAML subset**

Replace `The parser accepts a small YAML subset. Lists use inline brackets.` with:

```markdown
`glade.yml` intentionally supports a small YAML subset. Use inline lists such
as `[force-app]`; avoid anchors, merges, and complex YAML features.
```

- [ ] **Step 4: Add validation error examples**

Add:

````markdown
## Common Validation Errors

```text
unknown package directory: force-app/main/default
```

Check `project.packageDirs` against `sfdx-project.json`.

```text
unsupported org feature: SomeFeature
```

Use only features Glade models locally.
````

### Task 4.2: Add Local Testing Output Examples

**Files:**
- Modify: `site/docs-src/guide/local-testing.md`

- [ ] **Step 1: Rename title**

Change `# Local Testing` to:

```markdown
# Run Apex Tests Locally
```

- [ ] **Step 2: Add first successful run output**

After `glade test --project .`, add:

```text
PASS AccountServiceTest.testCreatesAccount 42ms
Result: 1 passed, 0 failed
```

- [ ] **Step 3: Add outcome examples**

In `## Outcomes`, add:

```text
PASS AccountServiceTest.testCreatesAccount 42ms
FAIL AccountServiceTest.testRejectsBlankName: System.AssertException
UNSUPPORTED ApprovalProcessTest.testSubmit: Approval.process is not supported locally
COMPILE_ERROR InvoiceServiceTest: Unknown type Invoice__c
```

- [ ] **Step 4: Rewrite metaphor**

Replace:

```markdown
A failing assertion and an unsupported platform API leave different tracks.
```

with:

```markdown
A failing assertion means the test ran and failed. An unsupported feature means
the runtime stopped at a known unsupported Salesforce surface.
```

### Task 4.3: Make Affected Tests Procedural

**Files:**
- Modify: `site/docs-src/guide/affected-tests.md`

- [ ] **Step 1: Rename title**

Change `# Affected-Test Selection` to:

```markdown
# Run Only Affected Tests
```

- [ ] **Step 2: Add mental model**

After the opening paragraph, add:

````markdown
```text
Changed file -> Apex reference graph -> selected tests
```
````

- [ ] **Step 3: Replace metaphorical lines**

Replace:

```markdown
A quiet `none` is useful. It tells you the saw did not touch the testable log.
```

with:

```markdown
`none` means Glade did not find tests affected by the current change set.
```

Replace:

```markdown
Better a few extra tests than a hidden split in the handle.
```

with:

```markdown
When Glade cannot prove a smaller safe set, it runs more tests rather than risk
missing a failure.
```

- [ ] **Step 4: Add JSON example**

Add:

```json
{
  "mode": "direct",
  "changed": ["force-app/main/default/classes/AccountService.cls"],
  "tests": ["AccountServiceTest"]
}
```

### Task 4.4: Add Complete CI Workflow

**Files:**
- Modify: `site/docs-src/guide/ci-artifacts.md`

- [ ] **Step 1: Rename title**

Change `# CI And Artifacts` to:

```markdown
# Add Glade to CI
```

- [ ] **Step 2: Add copy-paste workflow**

Add:

```yaml
name: glade
on: [pull_request]
jobs:
  glade:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: curl -fsSL https://glade.sh/install.sh | sh
      - run: echo "$HOME/.local/bin" >> "$GITHUB_PATH"
      - run: glade doctor
      - run: glade check --project . --format sarif --output glade-check.sarif
      - run: glade test changed --project . --since origin/main --json
      - run: glade test --project . --junit reports/glade-junit.xml
```

- [ ] **Step 3: Add artifact upload examples**

Add:

```yaml
      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: glade-results
          path: |
            glade-check.sarif
            reports/glade-junit.xml
            .glade/runs
```

- [ ] **Step 4: Add exit code note**

Add:

```markdown
`glade check` and `glade test` return non-zero for diagnostics or failed test
outcomes that should block a gate. Use `if: always()` only for upload steps that
must run after a failure.
```

### Task 4.5: Add Local API Endpoint Table

**Files:**
- Modify: `site/docs-src/guide/local-api-server.md`

- [ ] **Step 1: Rename title**

Change `# Local API Server` to:

```markdown
# Run a Local Salesforce-Shaped API
```

- [ ] **Step 2: Add endpoint table**

Add:

```markdown
| Area | Endpoint shape | Status |
| --- | --- | --- |
| API discovery | `/services/data/` | supported |
| Describe | `/services/data/vXX.X/sobjects/<Object>/describe` | supported baseline |
| Query | `/services/data/vXX.X/query?q=...` | supported baseline |
| SObject CRUD | `/services/data/vXX.X/sobjects/<Object>/<Id>` | supported baseline |
| Execute Anonymous | Tooling executeAnonymous route | supported where runtime supports code |
```

- [ ] **Step 3: Add JSON response example**

Add:

```json
{
  "totalSize": 1,
  "done": true,
  "records": [
    {
      "attributes": {
        "type": "Account"
      },
      "Name": "Twin Lakes"
    }
  ]
}
```

- [ ] **Step 4: Promote security warning**

Convert the security note to:

```markdown
::: warning Local server only
The local API server does not implement full OAuth or production Salesforce
authentication. Do not expose it to untrusted networks unless an authenticating
reverse proxy stands in front of it.
:::
```

### Task 4.6: Make Playground Page Task-Led

**Files:**
- Modify: `site/docs-src/guide/playground.md`

- [ ] **Step 1: Rename title**

Change `# Playground` to:

```markdown
# Use the Local Playground
```

- [ ] **Step 2: Reorder sections**

Use this order:

```markdown
## Start with built-in examples
## Start from an SFDX project
## Persist or reset local state
## Use memory-only state
## List examples
## Built-in examples
## What the playground shows
## Troubleshooting
```

- [ ] **Step 3: Group built-in examples**

Keep the existing examples, but add a `Group` column:

```markdown
| Group | ID | Name | Command |
| --- | --- | --- | --- |
```

Use groups: `Data and SOQL`, `Triggers and DML`, `Limits`, `Org diff and persistence`, `Business workflow`.

## Phase 5: Support, Policy, Plugins, And Advanced Docs

### Task 5.1: Polish Support Map For Adoption

**Files:**
- Modify: `site/docs-src/guide/support-map.md`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Rename title for sidebar match**

Change:

```markdown
# Apex and Salesforce Support
```

to:

```markdown
# What Glade Supports
```

- [ ] **Step 2: Add adoption checklist**

Add after the intro:

```markdown
## Before You Adopt Glade

- Your local loop uses supported Apex parse, check, test, SOQL, DML, trigger, and SObject paths.
- Your test suite can mock callouts and live side effects.
- Your project can tolerate explicit unsupported diagnostics for Salesforce-hosted services.
- You will keep a Salesforce org gate for features Glade does not model.
```

- [ ] **Step 3: Add unsupported diagnostic example**

Add:

```text
UnsupportedFeature: Approval.process is not supported by Glade's local runtime.
```

- [ ] **Step 4: Update tests**

Change the support-map title assertion:

```js
assert.match(supportMap, /^# What Glade Supports/m);
assert.match(supportMap, /Before You Adopt Glade/);
assert.match(supportMap, /UnsupportedFeature/);
```

### Task 5.2: Clarify Compatibility Policy

**Files:**
- Modify: `site/docs-src/guide/compatibility.md`

- [ ] **Step 1: Add status promotion rules**

Add:

```markdown
## How Support Moves

Unsupported behavior moves only when there is runtime behavior and evidence.

1. Add or confirm a compatibility fixture.
2. Implement the smallest runtime behavior that matches the public contract.
3. Run the focused package gate.
4. Regenerate checked compatibility reports when generated rows change.
5. Promote the row from `unsupported` to `partial` or `supported`.
```

- [ ] **Step 2: Add gap reporting section**

Add:

```markdown
## Report a Gap

When Glade stops at an unsupported surface you need, include:

- the Apex snippet or test that hits the gap
- the command you ran
- the unsupported diagnostic
- whether the behavior is required for local tests, local API use, or editor feedback
```

### Task 5.3: Demote Developer Reports In Copy

**Files:**
- Modify: `site/docs-src/guide/compatibility-dashboard.md`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Rename title**

Change:

```markdown
# Developer Reports
```

to:

```markdown
# Maintainer Proof Reports
```

- [ ] **Step 2: Add warning**

Add immediately after the title:

```markdown
Most users should start with [What Glade supports](/guide/support-map). This
page summarizes generated reports used by maintainers and release gates.
```

- [ ] **Step 3: Update tests**

Change:

```js
assert.match(compatibilityDashboard, /^# Maintainer Proof Reports/m);
assert.match(compatibilityDashboard, /Most users should start/);
```

### Task 5.4: Mark Plugin Marketplace State

**Files:**
- Modify: `site/docs-src/guide/plugins.md`
- Modify: `site/docs-src/guide/plugins/marketplace.md`
- Modify: `site/docs-src/guide/plugins/first-party.md`

- [ ] **Step 1: Add first-party-first note**

In `plugins.md`, add:

```markdown
Most users can ignore plugin author docs on first run. Install first-party
plugins only when you need compatibility fixtures, support reports, or advisory
performance scans.
```

- [ ] **Step 2: Clarify marketplace state**

If `https://plugins.glade.sh/index.json` is not live at implementation time, add:

```markdown
The marketplace model is preview until the production registry is live. First-party
plugin installation is the supported path for now.
```

If it is live, instead add a real `glade plugins available` output sample.

- [ ] **Step 3: Reduce placeholder dominance**

Keep `@acme/quality` examples in author docs, but make first-party plugin examples the first examples users see.

## Phase 6: UI, Accessibility, And Metadata Polish

### Task 6.1: Strengthen Docs Tables And Sidebar States

**Files:**
- Modify: `site/.vitepress/theme/custom.css`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Ensure table mobile scroll remains**

Keep or add:

```css
@media (max-width: 640px) {
  .vp-doc table {
    display: block;
    overflow-x: auto;
    white-space: nowrap;
  }
}
```

- [ ] **Step 2: Strengthen active sidebar link**

Add:

```css
.VPSidebarItem.is-active > .item > .link {
  color: var(--vp-c-brand-1);
  font-weight: 700;
}
```

- [ ] **Step 3: Verify focus state assertions**

Ensure tests still assert:

```js
assert.match(css, /:focus-visible\s*\{[\s\S]*outline: 2px solid var\(--glade-focus-ring\);/);
```

### Task 6.2: Handle Last Updated Metadata

**Files:**
- Modify: `site/.vitepress/config.ts`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Inspect built output**

Run:

```bash
cd site
npm run build
rg -n "Last updated:\\s*</" .vitepress/dist || true
```

- [ ] **Step 2: If empty metadata exists, disable it**

Set:

```ts
lastUpdated: false,
```

If VitePress renders real dates, keep `lastUpdated: true`.

- [ ] **Step 3: Add test for chosen behavior**

If disabled:

```js
assert.match(config, /lastUpdated: false/);
```

If kept:

```js
assert.match(config, /lastUpdated: true/);
```

## Phase 7: Final Verification

### Task 7.1: Run Local Gates

**Files:**
- Read only: repo root

- [ ] **Step 1: Run site tests**

Run:

```bash
cd site
npm test
```

Expected: 0 failures.

- [ ] **Step 2: Run site build**

Run:

```bash
cd site
npm run build
```

Expected: VitePress build completes.

- [ ] **Step 3: Run repo guard**

Run:

```bash
go test ./internal/repoguard
```

Expected: package passes.

- [ ] **Step 4: Run diff check**

Run:

```bash
git diff --check
```

Expected: no whitespace errors.

### Task 7.2: Run Public Route Smoke After Deployment

**Files:**
- Read only: deployed site

- [ ] **Step 1: Verify root and docs**

Run after GitHub Pages or Cloudflare Pages points `glade.sh` at this site:

```bash
curl -fsSL https://glade.sh/ >/dev/null
curl -fsSL https://glade.sh/docs/guide/overview >/dev/null
curl -fsSL https://glade.sh/docs/guide/quickstart >/dev/null
curl -fsSL https://glade.sh/docs/guide/support-map >/dev/null
```

Expected: every command exits 0.

- [ ] **Step 2: Verify installer route**

Run:

```bash
curl -fsSL https://glade.sh/install.sh | head -n 5
curl -fsSI https://glade.sh/install.sh | grep -i content-type
```

Expected: output starts with shell script text and not legacy HTML.

- [ ] **Step 3: Verify install command**

Run on a clean macOS or Linux host with a release available:

```bash
curl -fsSL https://glade.sh/install.sh | sh
glade version
glade doctor
```

Expected: installer downloads from `github.com/glade-sh/glade`, installs `glade`, and `glade doctor` prints `parser: ok (tree-sitter)`.

## Self-Review Checklist

- [ ] The homepage no longer sends users to a docs page while promising an app launch.
- [ ] The install block has script, releases, checksums, and verification commands.
- [ ] The first docs sidebar group answers: what is it, quickstart, install, support.
- [ ] Brand, plugin-author, and maintainer report pages are not in the first-run path.
- [ ] Core docs include expected output examples.
- [ ] The support map is promoted and linked from homepage, installation, quickstart, local testing, and API server docs.
- [ ] Public docs do not claim full Salesforce parity.
- [ ] Generated compatibility reports remain visible as maintainer proof, not as the public front door.
- [ ] `npm test`, `npm run build`, `go test ./internal/repoguard`, and `git diff --check` pass.
- [ ] `glade.sh/install.sh` serves the new installer after deployment.

## Execution Notes

Use subagent-driven execution for speed. Run Phase 1 and Phase 2 first because they set the public route and navigation shape. Phases 3, 4, 5, and 6 can then run in parallel by the worker ownership map above. Merge by running the full Phase 7 gates after each worker returns.
