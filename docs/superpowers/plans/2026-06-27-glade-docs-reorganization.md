# Glade Docs Reorganization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reorganize the Glade docs so first-run paths, daily workflows, product modules, reference pages, help recipes, and maintainer material are separate and easy to scan.

**Architecture:** Keep the existing VitePress site and visual system. Add a clearer information architecture on top of current docs, then split overloaded pages into workflow, module, and reference pages while keeping old URLs available. Pin the new route shape in `site/tests/theme.test.mjs` before moving content.

**Tech Stack:** VitePress 2 alpha, Markdown, TypeScript config, Node test runner, existing `site/scripts/check-doc-routes.mjs`, current Glade docs content.

---

## File Structure

**Modify**
- `site/.vitepress/config.ts`: Replace top nav/sidebar groups with the new IA.
- `site/tests/theme.test.mjs`: Add route, sidebar, and content assertions for the new structure.
- `site/docs-src/guide/local-testing.md`: Keep Apex testing here; move LWC and Visualforce preview sections to first-class pages.
- `site/docs-src/guide/lwc-local-shell.md`: Reframe as the detailed LWC shell/reference page linked from the new module page.
- `site/docs-src/guide/editor.md`: Link debug, LWC preview, and Visualforce preview to their new workflow/module pages.
- `site/docs-src/help/index.md`: Keep help as short recipes and point deeper topics to workflows/modules.
- `docs/README.md`: Mirror the new user/contributor map.
- `README.md`: Update the Docs section to point at the new site structure.

**Create**
- `site/docs-src/guide/workflows.md`
- `site/docs-src/guide/workflows/apex-tests.md`
- `site/docs-src/guide/workflows/debug-apex.md`
- `site/docs-src/guide/workflows/lwc-preview.md`
- `site/docs-src/guide/workflows/visualforce-preview.md`
- `site/docs-src/guide/workflows/local-data.md`
- `site/docs-src/guide/workflows/ci.md`
- `site/docs-src/guide/modules.md`
- `site/docs-src/guide/modules/apex-runtime.md`
- `site/docs-src/guide/modules/test-runner.md`
- `site/docs-src/guide/modules/local-org-data.md`
- `site/docs-src/guide/modules/lwc-preview.md`
- `site/docs-src/guide/modules/visualforce-preview.md`
- `site/docs-src/guide/modules/debug-profile.md`
- `site/docs-src/guide/modules/editor.md`
- `site/docs-src/guide/modules/plugins.md`
- `site/docs-src/reference/cli.md`
- `site/docs-src/reference/config.md`
- `site/docs-src/reference/errors.md`
- `site/docs-src/reference/apex-support.md`
- `site/docs-src/reference/lwc-support.md`
- `site/docs-src/reference/visualforce-support.md`
- `site/docs-src/reference/local-api-routes.md`

**Keep Existing URLs**
- Keep `/guide/quickstart`, `/guide/local-testing`, `/guide/lwc-local-shell`, `/guide/editor`, `/guide/glade-orgs`, `/guide/local-api-server`, `/guide/ci-artifacts`, `/guide/plugins`, `/guide/configuration`, `/guide/cli-reference`, `/guide/errors`, and `/guide/support-map`.
- Old pages should link to the new pages where the new structure is clearer.

---

### Task 1: Pin The New IA With Failing Site Tests

**Files:**
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Add reads for the new docs pages**

Add these near the existing `readFile` declarations:

```js
const workflowsIndex = await readFile(new URL("../docs-src/guide/workflows.md", import.meta.url), "utf8").catch(() => "");
const workflowApexTests = await readFile(new URL("../docs-src/guide/workflows/apex-tests.md", import.meta.url), "utf8").catch(() => "");
const workflowDebugApex = await readFile(new URL("../docs-src/guide/workflows/debug-apex.md", import.meta.url), "utf8").catch(() => "");
const workflowLwcPreview = await readFile(new URL("../docs-src/guide/workflows/lwc-preview.md", import.meta.url), "utf8").catch(() => "");
const workflowVisualforcePreview = await readFile(new URL("../docs-src/guide/workflows/visualforce-preview.md", import.meta.url), "utf8").catch(() => "");
const workflowLocalData = await readFile(new URL("../docs-src/guide/workflows/local-data.md", import.meta.url), "utf8").catch(() => "");
const workflowCi = await readFile(new URL("../docs-src/guide/workflows/ci.md", import.meta.url), "utf8").catch(() => "");
const modulesIndex = await readFile(new URL("../docs-src/guide/modules.md", import.meta.url), "utf8").catch(() => "");
const moduleApexRuntime = await readFile(new URL("../docs-src/guide/modules/apex-runtime.md", import.meta.url), "utf8").catch(() => "");
const moduleTestRunner = await readFile(new URL("../docs-src/guide/modules/test-runner.md", import.meta.url), "utf8").catch(() => "");
const moduleLocalOrgData = await readFile(new URL("../docs-src/guide/modules/local-org-data.md", import.meta.url), "utf8").catch(() => "");
const moduleLwcPreview = await readFile(new URL("../docs-src/guide/modules/lwc-preview.md", import.meta.url), "utf8").catch(() => "");
const moduleVisualforcePreview = await readFile(new URL("../docs-src/guide/modules/visualforce-preview.md", import.meta.url), "utf8").catch(() => "");
const moduleDebugProfile = await readFile(new URL("../docs-src/guide/modules/debug-profile.md", import.meta.url), "utf8").catch(() => "");
const moduleEditor = await readFile(new URL("../docs-src/guide/modules/editor.md", import.meta.url), "utf8").catch(() => "");
const modulePlugins = await readFile(new URL("../docs-src/guide/modules/plugins.md", import.meta.url), "utf8").catch(() => "");
const referenceCli = await readFile(new URL("../docs-src/reference/cli.md", import.meta.url), "utf8").catch(() => "");
const referenceConfig = await readFile(new URL("../docs-src/reference/config.md", import.meta.url), "utf8").catch(() => "");
const referenceErrors = await readFile(new URL("../docs-src/reference/errors.md", import.meta.url), "utf8").catch(() => "");
const referenceApexSupport = await readFile(new URL("../docs-src/reference/apex-support.md", import.meta.url), "utf8").catch(() => "");
const referenceLwcSupport = await readFile(new URL("../docs-src/reference/lwc-support.md", import.meta.url), "utf8").catch(() => "");
const referenceVisualforceSupport = await readFile(new URL("../docs-src/reference/visualforce-support.md", import.meta.url), "utf8").catch(() => "");
const referenceLocalApiRoutes = await readFile(new URL("../docs-src/reference/local-api-routes.md", import.meta.url), "utf8").catch(() => "");
```

- [ ] **Step 2: Add a failing IA test**

Add this test near the existing docs navigation tests:

```js
test("docs navigation exposes workflows modules and references as separate trails", () => {
  assert.match(config, /\{ text: 'Workflows', link: '\/guide\/workflows' \}/);
  assert.match(config, /\{ text: 'Product areas', link: '\/guide\/modules' \}/);
  assert.match(config, /\{ text: 'Reference', link: '\/reference\/cli' \}/);
  assert.match(config, /text: 'Run Apex tests'[\s\S]*link: '\/guide\/workflows\/apex-tests'/);
  assert.match(config, /text: 'Debug Apex'[\s\S]*link: '\/guide\/workflows\/debug-apex'/);
  assert.match(config, /text: 'Preview LWC'[\s\S]*link: '\/guide\/workflows\/lwc-preview'/);
  assert.match(config, /text: 'Preview Visualforce'[\s\S]*link: '\/guide\/workflows\/visualforce-preview'/);
  assert.match(config, /text: 'Apex runtime'[\s\S]*link: '\/guide\/modules\/apex-runtime'/);
  assert.match(config, /text: 'LWC preview'[\s\S]*link: '\/guide\/modules\/lwc-preview'/);
  assert.match(config, /text: 'Visualforce preview'[\s\S]*link: '\/guide\/modules\/visualforce-preview'/);
  assert.match(config, /text: 'Debug and profile'[\s\S]*link: '\/guide\/modules\/debug-profile'/);
  assert.match(config, /text: 'CLI reference'[\s\S]*link: '\/reference\/cli'/);
  assert.match(config, /text: 'LWC support matrix'[\s\S]*link: '\/reference\/lwc-support'/);
  assert.match(config, /text: 'Visualforce support matrix'[\s\S]*link: '\/reference\/visualforce-support'/);
});
```

- [ ] **Step 3: Add a failing content shape test**

Add this test after the IA test:

```js
test("new docs pages use clear page roles and link to deeper references", () => {
  assert.match(workflowsIndex, /^# Choose a Glade workflow/m);
  assert.match(workflowsIndex, /Run Apex tests/);
  assert.match(workflowsIndex, /Preview LWC locally/);
  assert.match(workflowsIndex, /Preview Visualforce locally/);

  assert.match(workflowApexTests, /^# Run Apex tests/m);
  assert.match(workflowApexTests, /glade test --project \./);
  assert.match(workflowApexTests, /\[Test runner\]\(\/guide\/modules\/test-runner\)/);
  assert.match(workflowDebugApex, /^# Debug Apex/m);
  assert.match(workflowDebugApex, /glade dap --project/);
  assert.match(workflowDebugApex, /\[Debug and profile\]\(\/guide\/modules\/debug-profile\)/);
  assert.match(workflowLwcPreview, /^# Preview LWC locally/m);
  assert.match(workflowLwcPreview, /glade dev lwc --project \. --open/);
  assert.match(workflowLwcPreview, /\[LWC support matrix\]\(\/reference\/lwc-support\)/);
  assert.match(workflowVisualforcePreview, /^# Preview Visualforce locally/m);
  assert.match(workflowVisualforcePreview, /glade dev vf --project \./);
  assert.match(workflowVisualforcePreview, /\[Visualforce support matrix\]\(\/reference\/visualforce-support\)/);

  assert.match(modulesIndex, /^# Product areas/m);
  assert.match(moduleApexRuntime, /^# Apex runtime/m);
  assert.match(moduleTestRunner, /^# Test runner/m);
  assert.match(moduleLocalOrgData, /^# Local org and data/m);
  assert.match(moduleLwcPreview, /^# LWC preview/m);
  assert.match(moduleVisualforcePreview, /^# Visualforce preview/m);
  assert.match(moduleDebugProfile, /^# Debug and profile/m);
  assert.match(moduleEditor, /^# Editor and workbench/m);
  assert.match(modulePlugins, /^# Plugins/m);

  assert.match(referenceCli, /^# CLI reference/m);
  assert.match(referenceConfig, /^# Config reference/m);
  assert.match(referenceErrors, /^# Error codes/m);
  assert.match(referenceApexSupport, /^# Apex support map/m);
  assert.match(referenceLwcSupport, /^# LWC support matrix/m);
  assert.match(referenceVisualforceSupport, /^# Visualforce support matrix/m);
  assert.match(referenceLocalApiRoutes, /^# Local API routes/m);
});
```

- [ ] **Step 4: Run the test and verify it fails for the right reason**

Run:

```bash
npm --prefix site test -- --test-name-pattern "docs navigation exposes|new docs pages"
```

Expected: FAIL because the new pages and route strings do not exist yet.

- [ ] **Step 5: Commit the failing test**

```bash
git add site/tests/theme.test.mjs
git commit -m "test: pin docs information architecture"
```

---

### Task 2: Replace The Site Navigation With The New IA

**Files:**
- Modify: `site/.vitepress/config.ts`

- [ ] **Step 1: Replace top-level nav**

Use this `nav` array:

```ts
nav: [
  { text: 'Install', link: '/guide/installation' },
  { text: 'Workflows', link: '/guide/workflows' },
  { text: 'Product areas', link: '/guide/modules' },
  { text: 'What runs locally', link: '/guide/support-map' },
  { text: 'Reference', link: '/reference/cli' },
  { text: 'Help', link: '/help/' },
  { text: 'GitHub', link: 'https://github.com/glade-sh/glade' }
],
```

- [ ] **Step 2: Replace sidebar groups**

Use these groups in `themeConfig.sidebar`:

```ts
sidebar: [
  {
    text: 'Start',
    items: [
      { text: 'What is Glade?', link: '/guide/overview' },
      { text: 'Install', link: '/guide/installation' },
      { text: 'First local check', link: '/guide/quickstart' },
      { text: 'Choose a workflow', link: '/guide/workflows' },
      { text: 'What runs locally', link: '/guide/support-map' },
      { text: 'Security & Trust', link: '/guide/security-trust' },
      { text: 'Playground', link: '/guide/playground' }
    ]
  },
  {
    text: 'Workflows',
    collapsed: false,
    items: [
      { text: 'Run Apex tests', link: '/guide/workflows/apex-tests' },
      { text: 'Debug Apex', link: '/guide/workflows/debug-apex' },
      { text: 'Execute anonymous Apex and SOQL', link: '/guide/workbench' },
      { text: 'Work with local data', link: '/guide/workflows/local-data' },
      { text: 'Preview LWC', link: '/guide/workflows/lwc-preview' },
      { text: 'Preview Visualforce', link: '/guide/workflows/visualforce-preview' },
      { text: 'Add Glade to CI', link: '/guide/workflows/ci' },
      { text: 'Use VS Code', link: '/guide/editor' },
      { text: 'Use plugins', link: '/guide/plugins' }
    ]
  },
  {
    text: 'Product areas',
    collapsed: false,
    items: [
      { text: 'Product area overview', link: '/guide/modules' },
      { text: 'Apex runtime', link: '/guide/modules/apex-runtime' },
      { text: 'Test runner', link: '/guide/modules/test-runner' },
      { text: 'Local org and data', link: '/guide/modules/local-org-data' },
      { text: 'LWC preview', link: '/guide/modules/lwc-preview' },
      { text: 'Visualforce preview', link: '/guide/modules/visualforce-preview' },
      { text: 'Debug and profile', link: '/guide/modules/debug-profile' },
      { text: 'Editor and workbench', link: '/guide/modules/editor' },
      { text: 'Plugins', link: '/guide/modules/plugins' }
    ]
  },
  {
    text: 'Reference',
    collapsed: true,
    items: [
      { text: 'CLI reference', link: '/reference/cli' },
      { text: 'Config reference', link: '/reference/config' },
      { text: 'JSON envelope', link: '/reference/json-schema' },
      { text: 'Automation and JSON', link: '/guide/automation' },
      { text: 'Exit codes', link: '/guide/exit-codes' },
      { text: 'Error codes', link: '/reference/errors' },
      { text: 'Apex support map', link: '/reference/apex-support' },
      { text: 'LWC support matrix', link: '/reference/lwc-support' },
      { text: 'Visualforce support matrix', link: '/reference/visualforce-support' },
      { text: 'Local API routes', link: '/reference/local-api-routes' }
    ]
  },
  {
    text: 'Guided help',
    collapsed: true,
    items: [
      { text: 'Help overview', link: '/help/' },
      { text: 'First local check', link: '/help/first-local-check' },
      { text: 'Run one Apex test', link: '/help/run-one-apex-test' },
      { text: 'Debug with breakpoints', link: '/help/debug-apex-vscode' },
      { text: 'Anonymous Apex scratch', link: '/help/anonymous-apex-scratch' },
      { text: 'Local data environments', link: '/help/local-data-environments' },
      { text: 'Changed tests before a PR', link: '/help/changed-tests-before-pr' },
      { text: 'Glade org data import', link: '/help/glade-org-sf-data-import' },
      { text: 'Profile a debug log', link: '/help/profile-apex-debug-log' },
      { text: 'CI setup', link: '/help/ci-setup' }
    ]
  },
  {
    text: 'Advanced',
    collapsed: true,
    items: [
      { text: 'Enterprise projects', link: '/guide/enterprise-workflows' },
      { text: 'Affected tests', link: '/guide/affected-tests' },
      { text: 'Test startup cache', link: '/guide/test-startup-cache' },
      { text: 'Reports and package artifacts', link: '/guide/rich-local-workflows' },
      { text: 'Built-in examples', link: '/guide/examples' },
      { text: 'Plugin install and manage', link: '/guide/plugins/install-manage' },
      { text: 'Plugin lock files and CI', link: '/guide/plugins/lock-ci' },
      { text: 'First-party plugins', link: '/guide/plugins/first-party' },
      { text: 'AI-assisted Apex', link: '/guide/ai-assisted-apex' }
    ]
  },
  {
    text: 'Maintainer',
    collapsed: true,
    items: [
      { text: 'Maintainer home', link: '/maintainer/' },
      { text: 'Extend runtime support', link: '/maintainer/extend-runtime' },
      { text: 'Release runbook', link: '/maintainer/release' },
      { text: 'glade-tools', link: '/maintainer/glade-tools' },
      { text: 'Plugin runtime', link: '/maintainer/plugin-runtime' }
    ]
  }
],
```

- [ ] **Step 3: Run route check and verify it fails on missing files**

Run:

```bash
npm --prefix site run check:routes
```

Expected: FAIL listing the new `/guide/workflows/...`, `/guide/modules/...`, and `/reference/...` pages.

- [ ] **Step 4: Commit nav shell**

```bash
git add site/.vitepress/config.ts
git commit -m "docs: reshape site navigation"
```

---

### Task 3: Add Workflow Pages

**Files:**
- Create: `site/docs-src/guide/workflows.md`
- Create: `site/docs-src/guide/workflows/apex-tests.md`
- Create: `site/docs-src/guide/workflows/debug-apex.md`
- Create: `site/docs-src/guide/workflows/lwc-preview.md`
- Create: `site/docs-src/guide/workflows/visualforce-preview.md`
- Create: `site/docs-src/guide/workflows/local-data.md`
- Create: `site/docs-src/guide/workflows/ci.md`

- [ ] **Step 1: Create the workflow directory**

```bash
mkdir -p site/docs-src/guide/workflows
```

- [ ] **Step 2: Create the workflow chooser page**

Create `site/docs-src/guide/workflows.md` with this structure:

```markdown
# Choose a Glade workflow

Glade has separate paths for the daily jobs people repeat: test Apex, debug Apex, preview LWC, preview Visualforce, work with local data, and add CI proof.

## Start here

| Job | Page | What you get |
| --- | --- | --- |
| Run supported local Apex tests | [Run Apex tests](/guide/workflows/apex-tests) | selectors, changed tests, watch, JUnit, and startup cache links |
| Step through Apex locally | [Debug Apex](/guide/workflows/debug-apex) | VS Code breakpoints, DAP, anonymous debug, and profile links |
| Open Lightning Web Components before deploy | [Preview LWC locally](/guide/workflows/lwc-preview) | toolchain setup, context presets, local data, and support links |
| Open Visualforce pages before deploy | [Preview Visualforce locally](/guide/workflows/visualforce-preview) | local `/apex` routes, forms, diagnostics, and hosted limits |
| Seed or query local records | [Work with local data](/guide/workflows/local-data) | local org targets, `sf` data commands, fixtures, and local API routes |
| Prove changes in pull requests | [Add Glade to CI](/guide/workflows/ci) | check, affected tests, JSON, JUnit, SARIF, and artifacts |

## When you need more depth

Product area pages explain what each subsystem owns. Reference pages carry flags, schemas, support matrices, and status rows.

- [Product areas](/guide/modules)
- [CLI reference](/reference/cli)
- [What runs locally](/guide/support-map)
```

- [ ] **Step 3: Create workflow pages with the same sections**

Each workflow page must use these headings:

```markdown
# <Workflow name>

<One short paragraph that names the job and product boundary.>

## Before you start

## Steps

## Expected output

## Common wrong turn

## Deeper reference
```

- [ ] **Step 4: Fill `apex-tests.md`**

Required content:
- H1: `# Run Apex tests`
- Commands:

```bash
glade test --project .
glade test --project . --class RefinementServiceTest
glade test --project . --class RefinementServiceTest --method testRefinesFileRow
glade test changed --project . --since origin/main
glade test failed --project .
mkdir -p reports
glade test --project . --junit reports/glade-junit.xml
```

Required links:
- `[Test runner](/guide/modules/test-runner)`
- `[Test startup cache](/guide/test-startup-cache)`
- `[Exit codes](/guide/exit-codes)`

- [ ] **Step 5: Fill `debug-apex.md`**

Required content:
- H1: `# Debug Apex`
- Commands:

```bash
glade dap --project .
glade debug profile --log reports/anonymous-output.txt --format markdown
glade debug profile --log reports/anonymous-output.txt --json
glade profile analyze --log reports/anonymous-output.txt --format pprof
```

Required links:
- `[Debug and profile](/guide/modules/debug-profile)`
- `[Editor and workbench](/guide/modules/editor)`
- `[Debug with breakpoints](/help/debug-apex-vscode)`
- `[Anonymous Apex scratch](/help/anonymous-apex-scratch)`

- [ ] **Step 6: Fill `lwc-preview.md`**

Required content:
- H1: `# Preview LWC locally`
- Commands:

```bash
glade toolchain install
glade dev lwc --project . --open
glade dev lwc --project . --context accountRecord --open
glade dev lwc --project . --target record-page --object Account --record 001000000000001AAA --page Account_Record_Page --open
```

Required boundary sentence:

```markdown
The local shell gives useful local preview routes. It does not replace hosted Lightning Experience.
```

Required links:
- `[LWC preview](/guide/modules/lwc-preview)`
- `[LWC support matrix](/reference/lwc-support)`
- `[Local org and data](/guide/modules/local-org-data)`
- `[Local LWC shell details](/guide/lwc-local-shell)`

- [ ] **Step 7: Fill `visualforce-preview.md`**

Required content:
- H1: `# Preview Visualforce locally`
- Commands:

```bash
glade dev vf --project . --port 8080
glade dev vf --project . --addr 127.0.0.1:0 --ready-file /tmp/glade-vf-ready.json
curl http://127.0.0.1:8080/services/data/v61.0/glade/visualforce/support
```

Required boundary sentence:

```markdown
The local server gives useful `/apex/<PageName>` previews. It does not promise exact hosted Visualforce behavior.
```

Required links:
- `[Visualforce preview](/guide/modules/visualforce-preview)`
- `[Visualforce support matrix](/reference/visualforce-support)`
- `[LWC preview](/guide/modules/lwc-preview)`
- `[What runs locally](/guide/support-map)`

- [ ] **Step 8: Fill `local-data.md`**

Required content:
- H1: `# Work with local data`
- Commands:

```bash
glade org create refinement-local
glade org start refinement-local --project .
glade org auth refinement-local --project .
sf data query -o refinement-local -q "SELECT Id, Name FROM Account"
glade db seed --db .glade/refinement-local.sqlite --project . data/file-rows.json
glade db inspect --db .glade/refinement-local.sqlite --json
```

Required links:
- `[Local org and data](/guide/modules/local-org-data)`
- `[Use Glade as an sf target](/guide/glade-orgs)`
- `[Local API routes](/reference/local-api-routes)`

- [ ] **Step 9: Fill `ci.md`**

Required content:
- H1: `# Add Glade to CI`
- Commands:

```bash
glade check --project . --format sarif --output reports/glade-check.sarif
glade test changed --project . --since origin/main --json --no-progress
glade test --project . --junit reports/glade-junit.xml --no-progress
```

Required links:
- `[Add Glade to CI](/guide/ci-artifacts)`
- `[Automation and JSON](/guide/automation)`
- `[Exit codes](/guide/exit-codes)`
- `[CI setup](/help/ci-setup)`

- [ ] **Step 10: Run focused tests**

Run:

```bash
npm --prefix site run check:routes
npm --prefix site test -- --test-name-pattern "new docs pages use"
```

Expected: route check PASS; content test still FAIL until module and reference pages exist.

- [ ] **Step 11: Commit workflow pages**

```bash
git add site/docs-src/guide/workflows.md site/docs-src/guide/workflows
git commit -m "docs: add workflow pages"
```

---

### Task 4: Add Product Area Pages

**Files:**
- Create: `site/docs-src/guide/modules.md`
- Create: `site/docs-src/guide/modules/apex-runtime.md`
- Create: `site/docs-src/guide/modules/test-runner.md`
- Create: `site/docs-src/guide/modules/local-org-data.md`
- Create: `site/docs-src/guide/modules/lwc-preview.md`
- Create: `site/docs-src/guide/modules/visualforce-preview.md`
- Create: `site/docs-src/guide/modules/debug-profile.md`
- Create: `site/docs-src/guide/modules/editor.md`
- Create: `site/docs-src/guide/modules/plugins.md`

- [ ] **Step 1: Create the module directory**

```bash
mkdir -p site/docs-src/guide/modules
```

- [ ] **Step 2: Create `modules.md`**

Use this opening:

```markdown
# Product areas

Glade has distinct local subsystems. Each area owns a different piece of the local Salesforce loop.

| Area | Use it for | Start with |
| --- | --- | --- |
| [Apex runtime](/guide/modules/apex-runtime) | parse, semantic checks, SOQL, DML, triggers, VM execution | `glade check --project .` |
| [Test runner](/guide/modules/test-runner) | local Apex test discovery, selectors, watch, artifacts | `glade test --project .` |
| [Local org and data](/guide/modules/local-org-data) | SQLite-backed records, `sf` target aliases, local API routes | `glade org create refinement-local` |
| [LWC preview](/guide/modules/lwc-preview) | Lightning workbench, contexts, LDS shims, local component routes | `glade dev lwc --project . --open` |
| [Visualforce preview](/guide/modules/visualforce-preview) | local `/apex` pages, forms, view state, remoting, Lightning Out | `glade dev vf --project . --port 8080` |
| [Debug and profile](/guide/modules/debug-profile) | DAP, breakpoints, anonymous debug, debug log profiling | `glade dap --project .` |
| [Editor and workbench](/guide/modules/editor) | VS Code, Glade Home, LSP, CodeLens, Test Explorer | `glade editor doctor vscode` |
| [Plugins](/guide/modules/plugins) | first-party plugins, linked plugins, locks, plugin findings | `glade plugins list` |
```

- [ ] **Step 3: Use one module template for each module page**

Each module page must use these headings:

```markdown
# <Module name>

<Two or three sentences. Name what this subsystem owns. Name what it does not own.>

## Use it when

## Entry commands

## What this module owns

## Requires Salesforce

## Related workflows

## Reference
```

- [ ] **Step 4: Fill module-specific command and link sets**

Use these required commands and links:

| Page | Commands | Required links |
| --- | --- | --- |
| `apex-runtime.md` | `glade check --project .`, `glade exec --project . "System.debug('local');"`, `glade inspect symbols --project .` | `/reference/apex-support`, `/guide/support-map`, `/guide/enterprise-workflows` |
| `test-runner.md` | `glade test --project .`, `glade test changed --project . --since origin/main`, `glade test serve --project .` | `/guide/workflows/apex-tests`, `/guide/test-startup-cache`, `/guide/ci-artifacts` |
| `local-org-data.md` | `glade org create refinement-local`, `glade org auth refinement-local --project .`, `glade server --project . --db .glade/refinement-local.sqlite --addr 127.0.0.1:8080` | `/guide/workflows/local-data`, `/guide/glade-orgs`, `/reference/local-api-routes` |
| `lwc-preview.md` | `glade toolchain install`, `glade dev lwc --project . --open`, `glade dev lwc --project . --context accountRecord --open` | `/guide/workflows/lwc-preview`, `/guide/lwc-local-shell`, `/reference/lwc-support` |
| `visualforce-preview.md` | `glade dev vf --project . --port 8080`, `curl http://127.0.0.1:8080/services/data/v61.0/glade/visualforce/support` | `/guide/workflows/visualforce-preview`, `/reference/visualforce-support`, `/guide/support-map` |
| `debug-profile.md` | `glade dap --project .`, `glade debug profile --log reports/anonymous-output.txt --format markdown`, `glade profile analyze --log reports/anonymous-output.txt --format pprof` | `/guide/workflows/debug-apex`, `/help/debug-apex-vscode`, `/help/profile-apex-debug-log` |
| `editor.md` | `glade editor doctor vscode`, `glade editor install vscode --force`, `glade lsp --project . --diagnostics-once` | `/guide/editor`, `/guide/workflows/debug-apex`, `/guide/workbench` |
| `plugins.md` | `glade plugins list`, `glade plugins available`, `glade plugins link --exec <plugin-executable>` | `/guide/plugins`, `/guide/plugins/first-party`, `/guide/plugins/lock-ci` |

- [ ] **Step 5: Add shared boundary wording where needed**

Use exact sentences:

```markdown
Salesforce remains the validation gate for hosted platform behavior.
```

Use this on LWC:

```markdown
The LWC shell is a local preview surface, not hosted Lightning Experience.
```

Use this on Visualforce:

```markdown
The Visualforce server is a local preview surface, not hosted Visualforce.
```

- [ ] **Step 6: Run focused tests**

Run:

```bash
npm --prefix site run check:routes
npm --prefix site test -- --test-name-pattern "new docs pages use"
```

Expected: module assertions PASS; reference assertions still FAIL until Task 5.

- [ ] **Step 7: Commit module pages**

```bash
git add site/docs-src/guide/modules.md site/docs-src/guide/modules
git commit -m "docs: add product area pages"
```

---

### Task 5: Add Reference Pages

**Files:**
- Create: `site/docs-src/reference/cli.md`
- Create: `site/docs-src/reference/config.md`
- Create: `site/docs-src/reference/errors.md`
- Create: `site/docs-src/reference/apex-support.md`
- Create: `site/docs-src/reference/lwc-support.md`
- Create: `site/docs-src/reference/visualforce-support.md`
- Create: `site/docs-src/reference/local-api-routes.md`

- [ ] **Step 1: Create reference pages as stable entry points**

Each page should use this shape:

```markdown
# <Reference name>

This page is a stable reference entry point. Use the linked detailed page or generated artifact when you need the full table.

## Start here

## Detailed source

## Related workflows
```

- [ ] **Step 2: Fill reference entry points**

Use this mapping:

| New page | H1 | Detailed source link | Related links |
| --- | --- | --- | --- |
| `cli.md` | `# CLI reference` | `[Full CLI command list](/guide/cli-reference)` | `/guide/workflows`, `/guide/automation`, `/guide/exit-codes` |
| `config.md` | `# Config reference` | `[Configure a Glade project](/guide/configuration)` | `/guide/modules/local-org-data`, `/guide/modules/plugins`, `/reference/errors` |
| `errors.md` | `# Error codes` | `[Error codes and glade explain](/guide/errors)` | `/guide/support-map`, `/reference/apex-support`, `/reference/lwc-support` |
| `apex-support.md` | `# Apex support map` | `[What Glade runs locally](/guide/support-map)` | `/guide/modules/apex-runtime`, `/guide/workflows/apex-tests`, `/guide/enterprise-workflows` |
| `lwc-support.md` | `# LWC support matrix` | `[Local LWC Shell](/guide/lwc-local-shell)` and `[LWC support rows](https://github.com/glade-sh/glade/blob/main/docs/LWC_SUPPORT.md)` | `/guide/modules/lwc-preview`, `/guide/workflows/lwc-preview`, `/guide/modules/visualforce-preview` |
| `visualforce-support.md` | `# Visualforce support matrix` | `[What Glade runs locally](/guide/support-map)` | `/guide/modules/visualforce-preview`, `/guide/workflows/visualforce-preview`, `/guide/modules/lwc-preview` |
| `local-api-routes.md` | `# Local API routes` | `[Run local Salesforce API routes](/guide/local-api-server)` | `/guide/modules/local-org-data`, `/guide/workflows/local-data`, `/guide/glade-orgs` |

- [ ] **Step 3: Keep reference pages short**

Each reference page should be between 25 and 60 lines. Do not copy large generated support tables into the site page in this pass. Link to current detailed sources.

- [ ] **Step 4: Run focused tests**

Run:

```bash
npm --prefix site run check:routes
npm --prefix site test -- --test-name-pattern "docs navigation exposes|new docs pages"
```

Expected: PASS.

- [ ] **Step 5: Commit reference pages**

```bash
git add site/docs-src/reference
git commit -m "docs: add reference entry points"
```

---

### Task 6: Split Overloaded Existing Pages And Update Indexes

**Files:**
- Modify: `site/docs-src/guide/local-testing.md`
- Modify: `site/docs-src/guide/lwc-local-shell.md`
- Modify: `site/docs-src/guide/editor.md`
- Modify: `site/docs-src/help/index.md`
- Modify: `docs/README.md`
- Modify: `README.md`
- Modify: `site/tests/theme.test.mjs`

- [ ] **Step 1: Reduce `local-testing.md` to Apex testing**

Keep these sections:
- `# Run Apex Tests Locally`
- `## Run all tests`
- `## Filter tests`
- `## Limit modes`
- `## Watch mode`
- `## Warm startup across CLI runs`
- `## CI pattern`
- `## Outcomes`

Remove the large `## LWC dev shell` and `## Visualforce dev server` bodies from this page. Replace them with this short section after `## Watch mode`:

```markdown
## Local preview surfaces

LWC and Visualforce preview have their own workflow and product pages.

- [Preview LWC locally](/guide/workflows/lwc-preview)
- [LWC preview](/guide/modules/lwc-preview)
- [Preview Visualforce locally](/guide/workflows/visualforce-preview)
- [Visualforce preview](/guide/modules/visualforce-preview)
```

- [ ] **Step 2: Reframe `lwc-local-shell.md` as deep detail**

Add this line after the intro block:

```markdown
Use [Preview LWC locally](/guide/workflows/lwc-preview) for the task path. This page is the deeper route, context, service, fixture, and diagnostic reference.
```

- [ ] **Step 3: Update `editor.md` links**

In `## LWC and Visualforce preview`, add:

```markdown
Use [Preview LWC locally](/guide/workflows/lwc-preview) and [Preview Visualforce locally](/guide/workflows/visualforce-preview) for task steps. Use [LWC preview](/guide/modules/lwc-preview) and [Visualforce preview](/guide/modules/visualforce-preview) for subsystem boundaries.
```

In `## Debug`, add:

```markdown
Use [Debug Apex](/guide/workflows/debug-apex) for the task path and [Debug and profile](/guide/modules/debug-profile) for the subsystem boundary.
```

- [ ] **Step 4: Update `help/index.md`**

Keep the nine help articles. Add this section after `## Reference docs`:

```markdown
## Product paths

- [Choose a workflow](/guide/workflows)
- [Product areas](/guide/modules)
- [CLI reference](/reference/cli)
- [What runs locally](/guide/support-map)
```

- [ ] **Step 5: Update repo docs index**

Replace the `If You Want To Use Glade` list in `docs/README.md` with:

```markdown
## If You Want To Use Glade

1. Start and install: [INSTALL.md](INSTALL.md)
2. Small pilot and day-to-day workflow: [TESTER_FIELD_GUIDE.md](TESTER_FIELD_GUIDE.md)
3. Project configuration: [CONFIG.md](CONFIG.md)
4. Run Apex tests: [LOCAL_TESTING.md](LOCAL_TESTING.md)
5. Preview LWC locally: [LWC_LOCAL_SHELL.md](LWC_LOCAL_SHELL.md)
6. Editor, Glade Home, and debug setup: [EDITOR.md](EDITOR.md)
7. CI outputs and saved artifacts: [CI_ARTIFACTS.md](CI_ARTIFACTS.md)
8. Security and release proof: [SECURITY_TRUST.md](SECURITY_TRUST.md)
9. Rich local workflows: [RICH_LOCAL_WORKFLOWS.md](RICH_LOCAL_WORKFLOWS.md)
10. Enterprise workflows: [ENTERPRISE_WORKFLOWS.md](ENTERPRISE_WORKFLOWS.md)
11. Test startup cache: [TEST_STARTUP_CACHE.md](TEST_STARTUP_CACHE.md)
12. Install and author plugins: [PLUGINS.md](PLUGINS.md)
13. Public site maps:
   - Workflows: <https://glade.sh/guide/workflows>
   - Product areas: <https://glade.sh/guide/modules>
   - Reference: <https://glade.sh/reference/cli>
   - Support map: <https://glade.sh/guide/support-map>
```

- [ ] **Step 6: Update `README.md` Docs section**

In the Docs section, add links for:
- `https://glade.sh/guide/workflows`
- `https://glade.sh/guide/modules`
- `https://glade.sh/reference/cli`
- `https://glade.sh/guide/support-map`

Keep install, quickstart, and support map visible.

- [ ] **Step 7: Update old tests that expected preview text inside `local-testing.md`**

Find this test:

```js
test("preview surfaces are labeled in public and repo docs", () => {
```

Replace the `localTesting` assertions for LWC and Visualforce with assertions against the new workflow/module pages:

```js
assert.match(workflowLwcPreview, /preview routes/i);
assert.match(workflowLwcPreview, /glade dev lwc --project \. --open/);
assert.match(moduleLwcPreview, /The LWC shell is a local preview surface/);
assert.match(workflowVisualforcePreview, /\/apex\/<PageName>/);
assert.match(workflowVisualforcePreview, /glade dev vf --project \./);
assert.match(moduleVisualforcePreview, /The Visualforce server is a local preview surface/);
assert.match(localTesting, /\[Preview LWC locally\]\(\/guide\/workflows\/lwc-preview\)/);
assert.match(localTesting, /\[Preview Visualforce locally\]\(\/guide\/workflows\/visualforce-preview\)/);
```

- [ ] **Step 8: Run focused tests**

Run:

```bash
npm --prefix site test -- --test-name-pattern "preview surfaces|docs navigation|new docs pages"
```

Expected: PASS.

- [ ] **Step 9: Commit cleanup**

```bash
git add site/docs-src/guide/local-testing.md site/docs-src/guide/lwc-local-shell.md site/docs-src/guide/editor.md site/docs-src/help/index.md docs/README.md README.md site/tests/theme.test.mjs
git commit -m "docs: split previews and update docs indexes"
```

---

### Task 7: Full Verification And Browser Check

**Files:**
- No new files.
- Verify all touched docs and config.

- [ ] **Step 1: Run whitespace check**

```bash
git diff --check
```

Expected: no output.

- [ ] **Step 2: Run site tests**

```bash
npm --prefix site test
```

Expected: all Node tests pass.

- [ ] **Step 3: Build the site**

```bash
npm --prefix site run build
```

Expected: `vitepress build` completes and writes `site/.vitepress/dist`.

- [ ] **Step 4: Preview rendered routes**

Start the preview server:

```bash
npm --prefix site run preview -- --host 127.0.0.1 --port 4173
```

Open these routes in a browser:
- `http://127.0.0.1:4173/guide/workflows`
- `http://127.0.0.1:4173/guide/workflows/lwc-preview`
- `http://127.0.0.1:4173/guide/workflows/visualforce-preview`
- `http://127.0.0.1:4173/guide/modules`
- `http://127.0.0.1:4173/guide/modules/debug-profile`
- `http://127.0.0.1:4173/reference/cli`
- `http://127.0.0.1:4173/reference/lwc-support`

Expected:
- Top nav shows `Install`, `Workflows`, `Product areas`, `What runs locally`, `Reference`, `Help`, `GitHub`.
- Sidebar shows `Start`, `Workflows`, `Product areas`, `Reference`, `Guided help`, `Advanced`, `Maintainer`.
- LWC, Visualforce, and Debug pages have first-class workflow and product-area routes.
- No page is only a list of links; each page explains what to do next.

- [ ] **Step 5: Stop preview server**

Use the terminal running preview and press `Ctrl-C`.

- [ ] **Step 6: Commit verification-only fixes if needed**

If browser or build checks expose wording or route issues, patch them and commit:

```bash
git add site/.vitepress/config.ts site/docs-src docs/README.md README.md site/tests/theme.test.mjs
git commit -m "docs: polish reorganized docs routes"
```

Skip this commit if no changes were needed after Task 6.

---

### Task 8: Final Status Packet

**Files:**
- No file edits unless final verification reveals a concrete doc issue.

- [ ] **Step 1: Inspect final status**

```bash
git status --short --branch
```

Expected:
- Branch is the docs work branch.
- Working tree is clean after commits, or only known untracked preview/build output ignored by `.gitignore`.

- [ ] **Step 2: Collect proof**

Record these command results in the final response:

```bash
git diff --check
npm --prefix site test
npm --prefix site run build
```

- [ ] **Step 3: Name the user-visible change**

Final response should say:
- New workflow landing page exists.
- Product areas are first-class.
- LWC, Visualforce, and Debug each have workflow plus product-area pages.
- Reference entry points exist for CLI, config, errors, support maps, LWC, Visualforce, and local API routes.
- Old docs routes remain available.

---

## Self-Review Checklist

- Spec coverage: The plan covers the visual breakdown: Start, Workflows, Product areas, Reference, Help, Maintainer.
- Product modules: LWC preview, Visualforce preview, Debug and profile, Apex runtime, Test runner, Local org and data, Editor, and Plugins each get a page.
- Workflow pages: Apex tests, Debug Apex, LWC preview, Visualforce preview, Local data, and CI each get a page.
- Reference pages: CLI, config, errors, Apex support, LWC support, Visualforce support, and local API routes each get a page.
- Old routes: Existing public routes stay present.
- Tests: Site tests pin nav, route shape, and content roles before implementation.
- Verification: Route check, site test, site build, and rendered preview are required.
