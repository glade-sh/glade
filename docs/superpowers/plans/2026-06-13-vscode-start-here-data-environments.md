# VS Code Start Here and Data Environments Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the top Glade sidebar into a useful daily control panel and make local data environments visible, switchable, cloneable, seedable, resettable, inspectable, and debug-aware.

**Architecture:** Keep Salesforce VS Code extensions in charge of org-backed deploy, retrieve, SOQL Builder, Code Analyzer, and scratch-org test workflows. Keep Glade in charge of the fast local loop: SFDX project context, active local DB, affected local tests, watch status, local DAP launches, and local data environment state. Build pure model functions first, then keep VS Code tree providers as thin renderers over those models.

**Tech Stack:** TypeScript VS Code extension under `contrib/vscode-glade`, plain Node tests under `contrib/vscode-glade/test`, Glade CLI commands through `gladeCli.ts`, local data commands through `glade db`, VS Code TreeDataProvider, VS Code commands and menus.

---

## Design And Brand Gates

Use the research and mockups as product requirements, not optional art:

- Research: `docs/superpowers/plans/2026-06-13-vscode-extension-ui-research.md`
- UI polish: `docs/superpowers/plans/2026-06-13-vscode-ui-polish-deep-dive.md`
- Mockup: `/tmp/picture-it/glade-vscode-extension-brand-iteration.png`
- HTML mockup: `/tmp/picture-it/glade-vscode-extension-brand-iteration.html`

Glade must feel native inside VS Code. Keep the VS Code chrome neutral. Use Glade green only for local-ready state, active local data, successful local proof, and primary local actions.

Required visual rules:

- Use the existing 24px extension mark at `contrib/vscode-glade/media/glade.svg`.
- Keep one Glade Activity Bar item.
- Show these sidebar views in this order: `Start Here`, `Local Runs`, `Data Environments`, `Local Org`, `Debug`.
- Do not keep a duplicate `Apex Tests` sidebar view. Use the native VS Code Testing API for local Apex tests.
- Use one Status Bar item. No custom Status Bar colors.
- Use exact labels from the mockup where possible: `Run local proof`, `Data env: dev`, `Debug last failure`, `Open trace output`.
- Keep Start Here to 5-7 rows.
- Put full paths and exact commands in tooltips or Output, not row labels.
- Use notifications only for blocking failures. Normal success goes to the Status Bar and Output channel.
- Do not add a webview in this slice.

Brand tokens from `site/docs-src/guide/brand-guide.md`:

```text
Glade green: #9BE870
Glade strong: #B7FF8A
Background: #070B0D
Surface: #10191E
Raised surface: #152229
Line: #26363D
Warning: #F5C95F
Danger: #FF6B61
Info: #7DB7FF
```

## Scope

This plan covers the first two next features:

1. **Start Here Panel**
   - Rename the visible `Project` view to `Start Here` while keeping view id `glade.project`.
   - Show readiness, project root, active environment, local DB summary, watch state, last run state, and top daily actions.
   - Add `Glade: Run Local Proof`, which runs the smallest useful local proof from the sidebar.
   - Add empty-state Welcome content that points to one next action when a project, Glade CLI, local DB, tests, or debug state is missing.

2. **Data Environments**
   - Upgrade the environment list from labels and generic commands into row-level actions.
   - Support create, switch, clone, delete, reveal DB, seed, reset, export, and inspect.
   - Keep every test/debug command tied to the active environment.
   - Make the active environment visible in Start Here, Data Environments, Local Org, Debug, Status Bar, and Output.

3. **Native VS Code Integration**
   - Keep Apex tests in the native Testing view.
   - Keep debug sessions in VS Code Run and Debug.
   - Keep parser, semantic, and local-test failures in Problems/diagnostics where available.
   - Keep full command logs in the Glade Output channel.

Out of scope for this plan:

- New org deploy or retrieve behavior.
- A full table/grid DB browser webview.
- A local SOQL Builder clone.
- New Glade database diff command in Go.
- Custom VS Code styling outside supported extension APIs.

## File Structure

Create:

- `contrib/vscode-glade/src/startHereModel.ts`
  - Pure row-building logic for the Start Here tree.
- `contrib/vscode-glade/src/startHereState.ts`
  - Small state holder for last run summary, watch state, and last inspect summary.
- `contrib/vscode-glade/src/statusModel.ts`
  - Pure Status Bar label and tooltip builder.
- `contrib/vscode-glade/src/views/startHereView.ts`
  - Tree provider for view id `glade.project`.
- `contrib/vscode-glade/src/environmentActions.ts`
  - Pure functions for config mutations and DB file copy/delete decisions.
- `contrib/vscode-glade/test/startHereModel.test.js`
  - Plain Node tests for Start Here rows.
- `contrib/vscode-glade/test/statusModel.test.js`
  - Plain Node tests for Status Bar labels.
- `contrib/vscode-glade/test/environmentActions.test.js`
  - Plain Node tests for environment config operations.

Modify:

- `contrib/vscode-glade/package.json`
  - Rename visible view from `Project` to `Start Here`.
  - Rename `Recommended Runs` to `Local Runs`.
  - Rename `Debug And Logs` to `Debug`.
  - Remove the duplicate `Apex Tests` sidebar contribution.
  - Add commands and context menus for environment row actions.
  - Add new test files to `npm test`.
- `contrib/vscode-glade/src/extension.ts`
  - Replace `ProjectView` with `StartHereView`.
  - Wire `StartHereState`.
  - Register new environment commands.
  - Keep refresh behavior centralized.
- `contrib/vscode-glade/src/environments.ts`
  - Add clone/delete helpers and environment metadata helpers.
- `contrib/vscode-glade/src/localOrg.ts`
  - Add command builders for seed/reset/export/inspect by explicit environment.
  - Add helper for revealing DB path.
- `contrib/vscode-glade/src/localOrgModel.ts`
  - Add compact summary labels for Start Here.
- `contrib/vscode-glade/src/status.ts`
  - Render the one Status Bar item from `statusModel.ts`.
- `contrib/vscode-glade/src/views/environmentsView.ts`
  - Render active/inactive environments as actionable rows with `contextValue`.
- `contrib/vscode-glade/src/views/localOrgView.ts`
  - Show active environment name and DB path with inspect state.
- `contrib/vscode-glade/src/views/projectView.ts`
  - Delete after `StartHereView` replaces it.
- `contrib/vscode-glade/src/tests/controller.ts`
  - Report last run summaries into `StartHereState`.
- `contrib/vscode-glade/src/tests/watch.ts`
  - Report watch state into `StartHereState`.
- `contrib/vscode-glade/test/environments.test.js`
  - Extend existing environment model tests.
- `contrib/vscode-glade/test/localOrg.test.js`
  - Extend existing command/path tests.
- `contrib/vscode-glade/README.md`
  - Update screenshot-free install and usage text.
- `docs/EDITOR.md`
  - Update daily workflow notes.
- `site/docs-src/guide/editor.md`
  - Update site docs.

---

### Task 1: Package Manifest For A Real First Panel

**Files:**
- Modify: `contrib/vscode-glade/package.json`
- Modify: `contrib/vscode-glade/test/package.test.js`

- [ ] **Step 1: Write the failing manifest test**

Add these assertions to `contrib/vscode-glade/test/package.test.js`:

```js
const startHereView = manifest.contributes.views.glade.find((view) => view.id === "glade.project");
assert(startHereView, "glade.project view must exist");
assert.strictEqual(startHereView.name, "Start Here");

const viewIds = manifest.contributes.views.glade.map((view) => view.id);
assert.deepStrictEqual(viewIds, [
  "glade.project",
  "glade.recommendedRuns",
  "glade.environments",
  "glade.localOrg",
  "glade.debugLogs",
]);
assert(!viewIds.includes("glade.apexTests"), "local Apex tests must use native Testing, not a duplicate sidebar view");

const localRunsView = manifest.contributes.views.glade.find((view) => view.id === "glade.recommendedRuns");
assert(localRunsView, "glade.recommendedRuns view must exist");
assert.strictEqual(localRunsView.name, "Local Runs");

const debugView = manifest.contributes.views.glade.find((view) => view.id === "glade.debugLogs");
assert(debugView, "glade.debugLogs view must exist");
assert.strictEqual(debugView.name, "Debug");

for (const command of [
  "glade.runLocalProof",
  "glade.cloneEnvironment",
  "glade.deleteEnvironment",
  "glade.revealEnvironmentDb",
  "glade.inspectEnvironment",
  "glade.statusQuickPick",
]) {
  assert(
    manifest.contributes.commands.some((entry) => entry.command === command),
    `${command} must be contributed`,
  );
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: FAIL from `test/package.test.js`, first with `Project !== Start Here` or a missing command assertion.

- [ ] **Step 3: Modify the manifest**

In `contrib/vscode-glade/package.json`, change the `glade` view contribution from:

```json
"views": {
  "glade": [
    {
      "id": "glade.project",
      "name": "Project",
      "when": "config.glade.enableSidebar"
    },
    {
      "id": "glade.recommendedRuns",
      "name": "Recommended Runs",
      "when": "config.glade.enableSidebar"
    },
    {
      "id": "glade.apexTests",
      "name": "Apex Tests",
      "when": "config.glade.enableSidebar"
    },
    {
      "id": "glade.environments",
      "name": "Data Environments",
      "when": "config.glade.enableSidebar"
    },
    {
      "id": "glade.localOrg",
      "name": "Local Org",
      "when": "config.glade.enableSidebar"
    },
    {
      "id": "glade.debugLogs",
      "name": "Debug And Logs",
      "when": "config.glade.enableSidebar"
    }
  ]
}
```

to:

```json
"views": {
  "glade": [
    {
      "id": "glade.project",
      "name": "Start Here",
      "when": "config.glade.enableSidebar"
    },
    {
      "id": "glade.recommendedRuns",
      "name": "Local Runs",
      "when": "config.glade.enableSidebar"
    },
    {
      "id": "glade.environments",
      "name": "Data Environments",
      "when": "config.glade.enableSidebar"
    },
    {
      "id": "glade.localOrg",
      "name": "Local Org",
      "when": "config.glade.enableSidebar"
    },
    {
      "id": "glade.debugLogs",
      "name": "Debug",
      "when": "config.glade.enableSidebar"
    }
  ]
}
```

Add command contributions:

```json
{
  "command": "glade.runLocalProof",
  "title": "Glade: Run Local Proof"
},
{
  "command": "glade.cloneEnvironment",
  "title": "Glade: Clone Local Data Environment"
},
{
  "command": "glade.deleteEnvironment",
  "title": "Glade: Delete Local Data Environment"
},
{
  "command": "glade.revealEnvironmentDb",
  "title": "Glade: Reveal Local Data Environment DB"
},
{
  "command": "glade.inspectEnvironment",
  "title": "Glade: Inspect Local Data Environment"
},
{
  "command": "glade.statusQuickPick",
  "title": "Glade: Show Local Status"
}
```

Add view item menus:

```json
"menus": {
  "view/item/context": [
    {
      "command": "glade.switchEnvironment",
      "when": "view == glade.environments && viewItem == gladeEnvironment",
      "group": "inline@1"
    },
    {
      "command": "glade.cloneEnvironment",
      "when": "view == glade.environments && viewItem =~ /gladeEnvironment/",
      "group": "inline@2"
    },
    {
      "command": "glade.inspectEnvironment",
      "when": "view == glade.environments && viewItem =~ /gladeEnvironment/",
      "group": "inline@3"
    },
    {
      "command": "glade.revealEnvironmentDb",
      "when": "view == glade.environments && viewItem =~ /gladeEnvironment/",
      "group": "navigation@1"
    },
    {
      "command": "glade.deleteEnvironment",
      "when": "view == glade.environments && viewItem == gladeEnvironment",
      "group": "navigation@9"
    }
  ]
}
```

If `menus` already exists, merge this `view/item/context` array into it.

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add contrib/vscode-glade/package.json contrib/vscode-glade/test/package.test.js
git commit -m "feat: define start here and environment actions"
```

---

### Task 2: Pure Start Here Model

**Files:**
- Create: `contrib/vscode-glade/src/startHereModel.ts`
- Create: `contrib/vscode-glade/test/startHereModel.test.js`
- Modify: `contrib/vscode-glade/package.json`

- [ ] **Step 1: Write the failing test**

Create `contrib/vscode-glade/test/startHereModel.test.js`:

```js
const assert = require("assert");
const model = require("../out/startHereModel");

const snapshot = {
  project: {
    workspaceFolder: "/repo",
    projectRoot: "/repo",
    configFound: true,
    namespace: "acme",
    sourceApiVersion: "63.0",
    packageDirs: ["force-app"],
    salesforceExtensions: { apex: true, apexTesting: true, apexLanguageServerTypescript: true },
  },
  activeEnvironment: { name: "dev", dbPath: "/repo/.glade/envs/dev.sqlite" },
  localOrgSummary: { objects: 3, records: 48, users: 1, profiles: 1, permissions: 2 },
  watchRunning: false,
  lastRun: { label: "Changed tests", passed: 8, failed: 1, durationMs: 1420 },
  changedSince: "origin/main",
};

const rows = model.buildStartHereRows(snapshot);
assert.deepStrictEqual(rows.map((row) => row.id), [
  "ready",
  "project",
  "environment",
  "local-proof",
  "last-run",
  "watch",
  "salesforce",
]);
assert.strictEqual(rows[0].label, "Ready");
assert.strictEqual(rows[2].label, "Active data: dev");
assert.strictEqual(rows[2].description, "48 records");
assert.strictEqual(rows[3].command, "glade.runLocalProof");
assert.strictEqual(rows[4].description, "8 passed, 1 failed");

const missingRows = model.buildStartHereRows({ project: undefined, changedSince: "origin/main" });
assert.strictEqual(missingRows[0].label, "Open an SFDX project");
assert.strictEqual(missingRows[0].command, "vscode.openFolder");
```

Add it to `package.json` test script after `node test/package.test.js`:

```json
"test": "npm run compile && node test/commands.test.js && node test/projectModel.test.js && node test/gladeCli.test.js && node test/testDiscovery.test.js && node test/testResults.test.js && node test/environments.test.js && node test/localOrg.test.js && node test/watch.test.js && node test/package.test.js && node test/startHereModel.test.js"
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: FAIL with `Cannot find module '../out/startHereModel'`.

- [ ] **Step 3: Add the model**

Create `contrib/vscode-glade/src/startHereModel.ts`:

```ts
import { GladeEnvironment } from "./environments";
import { LocalOrgSummary } from "./localOrgModel";
import { GladeProjectContext } from "./projectModel";

export interface StartHereRunSummary {
  label: string;
  passed: number;
  failed: number;
  durationMs?: number;
}

export interface StartHereSnapshot {
  project?: GladeProjectContext;
  activeEnvironment?: GladeEnvironment;
  localOrgSummary?: LocalOrgSummary;
  watchRunning?: boolean;
  lastRun?: StartHereRunSummary;
  changedSince: string;
}

export interface StartHereRow {
  id: string;
  label: string;
  description?: string;
  tooltip?: string;
  command?: string;
  contextValue?: string;
}

export function buildStartHereRows(snapshot: StartHereSnapshot): StartHereRow[] {
  if (!snapshot.project) {
    return [
      {
        id: "open-project",
        label: "Open an SFDX project",
        description: "sfdx-project.json required",
        tooltip: "Open a folder containing sfdx-project.json.",
        command: "vscode.openFolder",
        contextValue: "gladeStartHereAction",
      },
    ];
  }
  const project = snapshot.project;
  const environment = snapshot.activeEnvironment;
  const summary = snapshot.localOrgSummary;
  const records = summary ? `${summary.records} records` : "not inspected";
  const lastRun = snapshot.lastRun;
  return [
    {
      id: "ready",
      label: "Ready",
      description: project.configFound ? "project config loaded" : "using SFDX defaults",
      tooltip: project.projectRoot,
      contextValue: "gladeStartHereStatus",
    },
    {
      id: "project",
      label: shortPath(project.projectRoot),
      description: `API ${project.sourceApiVersion || "unknown"}`,
      tooltip: project.projectRoot,
      contextValue: "gladeStartHereStatus",
    },
    {
      id: "environment",
      label: `Active data: ${environment?.name || "dev"}`,
      description: records,
      tooltip: environment?.dbPath || "No active local DB path.",
      command: "glade.inspectLocalOrg",
      contextValue: "gladeStartHereAction",
    },
    {
      id: "local-proof",
      label: "Run local proof",
      description: `changed since ${snapshot.changedSince}`,
      tooltip: "Run changed local Apex tests, inspect the active DB, and update this panel.",
      command: "glade.runLocalProof",
      contextValue: "gladeStartHereAction",
    },
    {
      id: "last-run",
      label: lastRun ? lastRun.label : "No local run yet",
      description: lastRun ? `${lastRun.passed} passed, ${lastRun.failed} failed` : "run local proof",
      tooltip: lastRun ? `Last local run: ${lastRun.passed} passed, ${lastRun.failed} failed.` : "No local test run has been recorded in this window.",
      contextValue: "gladeStartHereStatus",
    },
    {
      id: "watch",
      label: snapshot.watchRunning ? "Watch running" : "Watch stopped",
      description: snapshot.watchRunning ? "local daemon active" : "click to start",
      tooltip: "Start or stop the local Apex watch loop.",
      command: snapshot.watchRunning ? "glade.stopWatch" : "glade.startWatch",
      contextValue: "gladeStartHereAction",
    },
    {
      id: "salesforce",
      label: "Salesforce extensions",
      description: salesforceSummary(project),
      tooltip: "Glade sits beside Salesforce org-backed tools.",
      contextValue: "gladeStartHereStatus",
    },
  ];
}

function shortPath(file: string): string {
  const parts = file.split(/[\\/]+/).filter(Boolean);
  return parts[parts.length - 1] || file;
}

function salesforceSummary(project: GladeProjectContext): string {
  const state = project.salesforceExtensions;
  const count = [state.apex, state.apexTesting, state.apexLanguageServerTypescript].filter(Boolean).length;
  return `${count}/3 detected`;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add contrib/vscode-glade/src/startHereModel.ts contrib/vscode-glade/test/startHereModel.test.js contrib/vscode-glade/package.json
git commit -m "feat: model start here rows"
```

---

### Task 3: Start Here Runtime State

**Files:**
- Create: `contrib/vscode-glade/src/startHereState.ts`
- Create: `contrib/vscode-glade/test/startHereState.test.js`
- Modify: `contrib/vscode-glade/package.json`

- [ ] **Step 1: Write the failing test**

Create `contrib/vscode-glade/test/startHereState.test.js`:

```js
const assert = require("assert");
const state = require("../out/startHereState");

const store = new state.StartHereState();
assert.strictEqual(store.snapshot().watchRunning, false);
assert.strictEqual(store.snapshot().lastRun, undefined);

store.setWatchRunning(true);
store.setLastRun({ label: "Changed tests", passed: 2, failed: 0, durationMs: 500 });
store.setLocalOrgSummary({ objects: 2, records: 12, users: 1, profiles: 1, permissions: 0 });

assert.deepStrictEqual(store.snapshot().lastRun, {
  label: "Changed tests",
  passed: 2,
  failed: 0,
  durationMs: 500,
});
assert.strictEqual(store.snapshot().watchRunning, true);
assert.strictEqual(store.snapshot().localOrgSummary.records, 12);
```

Append `&& node test/startHereState.test.js` to the `package.json` test script.

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: FAIL with `Cannot find module '../out/startHereState'`.

- [ ] **Step 3: Add the state holder**

Create `contrib/vscode-glade/src/startHereState.ts`:

```ts
import { LocalOrgSummary } from "./localOrgModel";
import { StartHereRunSummary } from "./startHereModel";

export interface StartHereRuntimeSnapshot {
  watchRunning: boolean;
  lastRun?: StartHereRunSummary;
  localOrgSummary?: LocalOrgSummary;
}

export class StartHereState {
  private watch = false;
  private run?: StartHereRunSummary;
  private summary?: LocalOrgSummary;

  setWatchRunning(running: boolean): void {
    this.watch = running;
  }

  setLastRun(run: StartHereRunSummary): void {
    this.run = run;
  }

  setLocalOrgSummary(summary: LocalOrgSummary): void {
    this.summary = summary;
  }

  snapshot(): StartHereRuntimeSnapshot {
    return {
      watchRunning: this.watch,
      lastRun: this.run,
      localOrgSummary: this.summary,
    };
  }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add contrib/vscode-glade/src/startHereState.ts contrib/vscode-glade/test/startHereState.test.js contrib/vscode-glade/package.json
git commit -m "feat: track start here runtime state"
```

---

### Task 4: Start Here Tree Provider

**Files:**
- Create: `contrib/vscode-glade/src/views/startHereView.ts`
- Modify: `contrib/vscode-glade/src/extension.ts`
- Delete: `contrib/vscode-glade/src/views/projectView.ts`

- [ ] **Step 1: Write the failing compile check**

Change `contrib/vscode-glade/src/extension.ts` import before the implementation exists:

```ts
import { StartHereView } from "./views/startHereView";
```

Replace:

```ts
const projectView = new ProjectView();
```

with:

```ts
const startHereView = new StartHereView(startHereState);
```

Run:

```bash
cd contrib/vscode-glade
npm run compile
```

Expected: FAIL with `Cannot find module './views/startHereView'`.

- [ ] **Step 2: Add the provider**

Create `contrib/vscode-glade/src/views/startHereView.ts`:

```ts
import * as vscode from "vscode";
import { configuredActiveEnvironment } from "../localOrg";
import { GladeProjectContext } from "../projectModel";
import { StartHereState } from "../startHereState";
import { buildStartHereRows, StartHereRow } from "../startHereModel";
import { GladeTreeItem } from "./tree";

export class StartHereView implements vscode.TreeDataProvider<GladeTreeItem> {
  private project?: GladeProjectContext;
  private readonly changed = new vscode.EventEmitter<GladeTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData = this.changed.event;

  constructor(private readonly state: StartHereState) {}

  setProject(project: GladeProjectContext | undefined): void {
    this.project = project;
    this.refresh();
  }

  refresh(): void {
    this.changed.fire();
  }

  getTreeItem(element: GladeTreeItem): vscode.TreeItem {
    return element;
  }

  getChildren(): GladeTreeItem[] {
    const config = vscode.workspace.getConfiguration("glade");
    const runtime = this.state.snapshot();
    const rows = buildStartHereRows({
      project: this.project,
      activeEnvironment: this.project ? configuredActiveEnvironment(this.project) : undefined,
      localOrgSummary: runtime.localOrgSummary,
      watchRunning: runtime.watchRunning,
      lastRun: runtime.lastRun,
      changedSince: config.get<string>("changedSince") || "origin/main",
    });
    return rows.map(toTreeItem);
  }
}

function toTreeItem(row: StartHereRow): GladeTreeItem {
  const item = new GladeTreeItem(row.label);
  item.id = row.id;
  item.description = row.description;
  item.tooltip = row.tooltip || row.description || row.label;
  item.contextValue = row.contextValue;
  if (row.command) {
    item.command = { command: row.command, title: row.label };
  }
  return item;
}
```

- [ ] **Step 3: Wire the provider**

In `contrib/vscode-glade/src/extension.ts`, remove:

```ts
import { ProjectView } from "./views/projectView";
import { ApexTestsView, RunsView } from "./views/runsView";
```

Add:

```ts
import { StartHereState } from "./startHereState";
import { StartHereView } from "./views/startHereView";
import { RunsView } from "./views/runsView";
```

Near the top of `activate`, add:

```ts
const startHereState = new StartHereState();
const startHereView = new StartHereView(startHereState);
```

Replace every `projectView` call:

```ts
projectView.setProject(project);
projectView.setProject(undefined);
vscode.window.registerTreeDataProvider("glade.project", projectView),
```

with:

```ts
startHereView.setProject(project);
startHereView.setProject(undefined);
vscode.window.registerTreeDataProvider("glade.project", startHereView),
```

Remove the standalone Apex Tests sidebar provider. Delete code shaped like:

```ts
const apexTestsView = new ApexTestsView(tests);
vscode.window.registerTreeDataProvider("glade.apexTests", apexTestsView),
```

Keep the native `GladeTestController`. The native VS Code Testing view remains the test tree.

Delete `contrib/vscode-glade/src/views/projectView.ts`.
Remove the `ApexTestsView` class from `contrib/vscode-glade/src/views/runsView.ts` if no other code imports it.

- [ ] **Step 4: Run compile and tests**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add contrib/vscode-glade/src/extension.ts contrib/vscode-glade/src/views/startHereView.ts contrib/vscode-glade/src/views/runsView.ts
git rm contrib/vscode-glade/src/views/projectView.ts
git commit -m "feat: replace project view with start here"
```

---

### Task 5: Environment Action Model

**Files:**
- Create: `contrib/vscode-glade/src/environmentActions.ts`
- Create: `contrib/vscode-glade/test/environmentActions.test.js`
- Modify: `contrib/vscode-glade/package.json`

- [ ] **Step 1: Write the failing test**

Create `contrib/vscode-glade/test/environmentActions.test.js`:

```js
const assert = require("assert");
const actions = require("../out/environmentActions");

const existing = [
  { name: "dev", dbPath: "/repo/.glade/envs/dev.sqlite" },
  { name: "bug-123", dbPath: "/repo/.glade/envs/bug-123.sqlite", fixturePath: "/repo/fixtures/bug-123.json" },
];

assert.deepStrictEqual(
  actions.addEnvironment(existing, { name: "qa", dbPath: "/repo/.glade/envs/qa.sqlite" }),
  [...existing, { name: "qa", dbPath: "/repo/.glade/envs/qa.sqlite" }],
);

assert.throws(
  () => actions.addEnvironment(existing, { name: "dev", dbPath: "/repo/.glade/envs/other.sqlite" }),
  /environment "dev" already exists/,
);

assert.deepStrictEqual(
  actions.removeEnvironment(existing, "bug-123"),
  [{ name: "dev", dbPath: "/repo/.glade/envs/dev.sqlite" }],
);

assert.throws(() => actions.removeEnvironment(existing, "dev"), /cannot delete the dev environment/);
assert.strictEqual(actions.cloneName("bug-123"), "bug-123-copy");
assert.strictEqual(actions.cloneName("bug-123-copy"), "bug-123-copy-2");
```

Append `&& node test/environmentActions.test.js` to the `package.json` test script.

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: FAIL with `Cannot find module '../out/environmentActions'`.

- [ ] **Step 3: Add pure action helpers**

Create `contrib/vscode-glade/src/environmentActions.ts`:

```ts
import * as path from "path";
import { GladeEnvironment } from "./environments";

export function addEnvironment(existing: GladeEnvironment[], next: GladeEnvironment): GladeEnvironment[] {
  if (existing.some((entry) => entry.name === next.name)) {
    throw new Error(`environment "${next.name}" already exists`);
  }
  return [...existing, next];
}

export function removeEnvironment(existing: GladeEnvironment[], name: string): GladeEnvironment[] {
  if (name === "dev") {
    throw new Error("cannot delete the dev environment");
  }
  const next = existing.filter((entry) => entry.name !== name);
  if (next.length === existing.length) {
    throw new Error(`environment "${name}" does not exist`);
  }
  return next;
}

export function cloneName(name: string): string {
  return name.endsWith("-copy") ? `${name}-2` : `${name}-copy`;
}

export function clonedEnvironment(source: GladeEnvironment, projectRoot: string): GladeEnvironment {
  const name = cloneName(source.name);
  return {
    name,
    dbPath: path.join(projectRoot, ".glade", "envs", `${name}.sqlite`),
    fixturePath: source.fixturePath,
  };
}

export function settingsValue(environments: GladeEnvironment[], projectRoot: string): GladeEnvironment[] {
  return environments.map((entry) => ({
    name: entry.name,
    dbPath: path.isAbsolute(entry.dbPath) ? path.relative(projectRoot, entry.dbPath) : entry.dbPath,
    fixturePath: entry.fixturePath
      ? path.isAbsolute(entry.fixturePath)
        ? path.relative(projectRoot, entry.fixturePath)
        : entry.fixturePath
      : undefined,
  }));
}
```

- [ ] **Step 4: Run tests**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add contrib/vscode-glade/src/environmentActions.ts contrib/vscode-glade/test/environmentActions.test.js contrib/vscode-glade/package.json
git commit -m "feat: model local data environment actions"
```

---

### Task 6: Actionable Data Environments View

**Files:**
- Modify: `contrib/vscode-glade/src/views/environmentsView.ts`
- Modify: `contrib/vscode-glade/src/views/tree.ts`

- [ ] **Step 1: Add tree item icon support**

Modify `contrib/vscode-glade/src/views/tree.ts`:

```ts
export function commandItem(label: string, command: string, tooltip?: string, icon?: vscode.ThemeIcon): GladeTreeItem {
  const item = new GladeTreeItem(label);
  item.command = { command, title: label };
  item.tooltip = tooltip || label;
  item.iconPath = icon;
  return item;
}
```

Run:

```bash
cd contrib/vscode-glade
npm run compile
```

Expected: PASS.

- [ ] **Step 2: Render environments with context values**

Replace `environmentItem` in `contrib/vscode-glade/src/views/environmentsView.ts` with:

```ts
function environmentItem(environment: GladeEnvironment, activeName: string): GladeTreeItem {
  const active = environment.name === activeName;
  const item = labelItem(
    environment.name,
    active ? "active" : path.basename(environment.dbPath),
    environment.dbPath,
  );
  item.contextValue = active ? "gladeEnvironmentActive" : "gladeEnvironment";
  item.iconPath = new vscode.ThemeIcon(active ? "check" : "database");
  item.command = {
    command: "glade.switchEnvironment",
    title: "Switch Environment",
    arguments: [environment],
  };
  return item;
}
```

Change command labels in `getChildren()`:

```ts
commandItem("Create", "glade.createEnvironment", "Create a local data environment", new vscode.ThemeIcon("add")),
commandItem("Seed Active", "glade.seedLocalOrg", "Seed the active local data environment", new vscode.ThemeIcon("cloud-upload")),
commandItem("Reset Active", "glade.resetLocalOrg", "Reset the active local data environment", new vscode.ThemeIcon("discard")),
commandItem("Export Active", "glade.exportLocalOrg", "Export the active local data environment", new vscode.ThemeIcon("save")),
```

- [ ] **Step 3: Run extension tests**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add contrib/vscode-glade/src/views/tree.ts contrib/vscode-glade/src/views/environmentsView.ts
git commit -m "feat: make data environments actionable"
```

---

### Task 7: Environment Commands

**Files:**
- Modify: `contrib/vscode-glade/src/extension.ts`
- Modify: `contrib/vscode-glade/src/localOrg.ts`

- [ ] **Step 1: Add explicit environment command helpers**

In `contrib/vscode-glade/src/localOrg.ts`, add:

```ts
export function dbSeedArgs(project: GladeProjectContext, environment: GladeEnvironment, fixture: string): string[] {
  return ["db", "seed", "--db", environment.dbPath, "--project", project.projectRoot, "--json", fixture];
}

export function dbResetArgs(project: GladeProjectContext, environment: GladeEnvironment): string[] {
  return ["db", "reset", "--db", environment.dbPath, "--project", project.projectRoot, "--json"];
}

export function dbExportArgs(project: GladeProjectContext, environment: GladeEnvironment): string[] {
  return ["db", "export", "--db", environment.dbPath, "--project", project.projectRoot];
}

export function dbInspectArgs(project: GladeProjectContext, environment: GladeEnvironment): string[] {
  return ["db", "inspect", "--db", environment.dbPath, "--project", project.projectRoot, "--json"];
}
```

Refactor existing command builders to call these helpers.

- [ ] **Step 2: Add command registration**

In `contrib/vscode-glade/src/extension.ts`, import:

```ts
import * as fs from "fs";
import { addEnvironment, clonedEnvironment, removeEnvironment, settingsValue } from "./environmentActions";
```

Change the switch command signature:

```ts
vscode.commands.registerCommand("glade.switchEnvironment", async (environment?: GladeEnvironment) => {
  const project = await projectOrWarn();
  if (!project) {
    return;
  }
  let pickedName = environment?.name;
  if (!pickedName) {
    const picked = await vscode.window.showQuickPick(
      configuredEnvironments(project).map((entry) => ({
        label: entry.name,
        description: entry.dbPath,
      })),
      { title: "Switch Local Data Environment" },
    );
    pickedName = picked?.label;
  }
  if (!pickedName) {
    return;
  }
  await vscode.workspace.getConfiguration("glade").update(
    "activeEnvironment",
    pickedName,
    vscode.ConfigurationTarget.Workspace,
  );
  environmentsView.refresh();
  localOrgView.refresh();
  startHereView.refresh();
});
```

Add clone:

```ts
vscode.commands.registerCommand("glade.cloneEnvironment", async (environment?: GladeEnvironment) => {
  const project = await projectOrWarn();
  if (!project) {
    return;
  }
  const source = environment || configuredActiveEnvironment(project);
  const clone = clonedEnvironment(source, project.projectRoot);
  await fs.promises.mkdir(path.dirname(clone.dbPath), { recursive: true });
  if (fs.existsSync(source.dbPath)) {
    await fs.promises.copyFile(source.dbPath, clone.dbPath);
  }
  const current = configuredEnvironments(project);
  const next = addEnvironment(current, clone);
  await vscode.workspace.getConfiguration("glade").update(
    "environments",
    settingsValue(next, project.projectRoot),
    vscode.ConfigurationTarget.Workspace,
  );
  await vscode.workspace.getConfiguration("glade").update("activeEnvironment", clone.name, vscode.ConfigurationTarget.Workspace);
  environmentsView.refresh();
  localOrgView.refresh();
  startHereView.refresh();
});
```

Add delete:

```ts
vscode.commands.registerCommand("glade.deleteEnvironment", async (environment?: GladeEnvironment) => {
  const project = await projectOrWarn();
  if (!project || !environment) {
    return;
  }
  const confirmed = await vscode.window.showWarningMessage(
    `Delete local data environment ${environment.name}?`,
    { modal: true },
    "Delete",
  );
  if (confirmed !== "Delete") {
    return;
  }
  const next = removeEnvironment(configuredEnvironments(project), environment.name);
  await vscode.workspace.getConfiguration("glade").update(
    "environments",
    settingsValue(next, project.projectRoot),
    vscode.ConfigurationTarget.Workspace,
  );
  if (configuredActiveEnvironment(project).name === environment.name) {
    await vscode.workspace.getConfiguration("glade").update("activeEnvironment", "dev", vscode.ConfigurationTarget.Workspace);
  }
  environmentsView.refresh();
  localOrgView.refresh();
  startHereView.refresh();
});
```

Add reveal:

```ts
vscode.commands.registerCommand("glade.revealEnvironmentDb", async (environment?: GladeEnvironment) => {
  const project = await projectOrWarn();
  if (!project) {
    return;
  }
  const target = environment || configuredActiveEnvironment(project);
  await vscode.commands.executeCommand("revealFileInOS", vscode.Uri.file(target.dbPath));
});
```

Add inspect row:

```ts
vscode.commands.registerCommand("glade.inspectEnvironment", async (environment?: GladeEnvironment) => {
  const project = await projectOrWarn();
  if (!project) {
    return;
  }
  const target = environment || configuredActiveEnvironment(project);
  const result = await inspectLocalOrg(project, target);
  localOrgView.setInspect(result, target);
  startHereState.setLocalOrgSummary(summaryFromInspect(result));
  localOrgView.refresh();
  startHereView.refresh();
});
```

- [ ] **Step 3: Add imports required by the snippets**

At the top of `extension.ts`, ensure these imports exist:

```ts
import * as fs from "fs";
import * as path from "path";
import { GladeEnvironment } from "./environments";
import { summaryFromInspect } from "./localOrgModel";
```

- [ ] **Step 4: Run tests**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add contrib/vscode-glade/src/extension.ts contrib/vscode-glade/src/localOrg.ts
git commit -m "feat: wire data environment commands"
```

---

### Task 8: Run Local Proof

**Files:**
- Modify: `contrib/vscode-glade/src/extension.ts`
- Modify: `contrib/vscode-glade/src/tests/runner.ts`
- Modify: `contrib/vscode-glade/src/tests/controller.ts`
- Modify: `contrib/vscode-glade/src/tests/watch.ts`

- [ ] **Step 1: Add summary conversion**

In `contrib/vscode-glade/src/tests/runner.ts`, add:

```ts
import { StartHereRunSummary } from "../startHereModel";

export function startHereSummary(label: string, run: GladeTestRun): StartHereRunSummary {
  return {
    label,
    passed: run.summary?.passed || 0,
    failed: run.summary?.failed || 0,
    durationMs: run.summary?.durationMs,
  };
}
```

- [ ] **Step 2: Register Run Local Proof**

In `contrib/vscode-glade/src/extension.ts`, add:

```ts
vscode.commands.registerCommand("glade.runLocalProof", async () => {
  const project = await projectOrWarn();
  if (!project) {
    return;
  }
  const changedSince = vscode.workspace.getConfiguration("glade").get<string>("changedSince") || "origin/main";
  output.tests.show(true);
  output.tests.appendLine(`$ glade test changed --project ${project.projectRoot} --since ${changedSince} --json`);
  try {
    const run = await runChangedTests(project, changedSince);
    startHereState.setLastRun(startHereSummary("Changed tests", run));
    const environment = configuredActiveEnvironment(project);
    const inspect = await inspectLocalOrg(project, environment);
    startHereState.setLocalOrgSummary(summaryFromInspect(inspect));
    localOrgView.setInspect(inspect, environment);
    tests.refresh();
    startHereView.refresh();
    localOrgView.refresh();
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    output.tests.appendLine(message);
    void vscode.window.showErrorMessage(`Glade local proof failed: ${message}`);
  }
});
```

- [ ] **Step 3: Update Test Explorer controller**

In `contrib/vscode-glade/src/tests/controller.ts`, add an optional callback:

```ts
constructor(
  private readonly context: vscode.ExtensionContext,
  private readonly output: vscode.OutputChannel,
  private readonly onRunComplete?: (summary: StartHereRunSummary) => void,
) {}
```

After a test run completes, call:

```ts
this.onRunComplete?.(startHereSummary("Test Explorer", result));
```

Pass the callback from `extension.ts`:

```ts
const tests = new GladeTestController(context, output.tests, (summary) => {
  startHereState.setLastRun(summary);
  startHereView.refresh();
});
```

- [ ] **Step 4: Update watch state**

In `contrib/vscode-glade/src/tests/watch.ts`, change constructor:

```ts
constructor(
  private readonly output: vscode.OutputChannel,
  private readonly onStateChange?: (running: boolean) => void,
) {}
```

In `start`, after `this.child = child`, add:

```ts
this.onStateChange?.(true);
```

In `close`, `error`, and `dispose`, add:

```ts
this.onStateChange?.(false);
```

Pass the callback from `extension.ts`:

```ts
const watch = new GladeTestWatch(output.tests, (running) => {
  startHereState.setWatchRunning(running);
  startHereView.refresh();
});
```

- [ ] **Step 5: Run tests**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add contrib/vscode-glade/src/extension.ts contrib/vscode-glade/src/tests/runner.ts contrib/vscode-glade/src/tests/controller.ts contrib/vscode-glade/src/tests/watch.ts
git commit -m "feat: add local proof start here action"
```

---

### Task 9: Local Org View Shows Active Environment

**Files:**
- Modify: `contrib/vscode-glade/src/views/localOrgView.ts`
- Modify: `contrib/vscode-glade/src/extension.ts`

- [ ] **Step 1: Change LocalOrgView state**

In `contrib/vscode-glade/src/views/localOrgView.ts`, import:

```ts
import { GladeEnvironment } from "../environments";
```

Change fields:

```ts
private environment?: GladeEnvironment;
```

Change `setInspect`:

```ts
setInspect(result: DBInspectResult, environment?: GladeEnvironment): void {
  this.environment = environment;
  this.summary = summaryFromInspect(result);
  this.rows = objectRowsFromInspect(result);
  this.changed.fire();
}
```

At the top of `getChildren()`, before summary rows, render:

```ts
const environmentRows = this.environment
  ? [
      labelItem(`Active: ${this.environment.name}`, this.environment.dbPath),
    ]
  : [];
```

Return:

```ts
return [
  ...environmentRows,
  summaryItem("Objects", this.summary.objects),
  summaryItem("Records", this.summary.records),
  summaryItem("Users", this.summary.users),
  summaryItem("Profiles", this.summary.profiles),
  summaryItem("Permissions", this.summary.permissions),
  ...this.rows.map((row) => summaryItem(row.name, row.rows)),
  ...commands,
];
```

- [ ] **Step 2: Update all callers**

In `extension.ts`, every call to:

```ts
localOrgView.setInspect(result);
```

must become:

```ts
localOrgView.setInspect(result, environment);
```

- [ ] **Step 3: Run tests**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add contrib/vscode-glade/src/views/localOrgView.ts contrib/vscode-glade/src/extension.ts
git commit -m "feat: show active environment in local org view"
```

---

### Task 10: Docs And Install Proof

**Files:**
- Modify: `contrib/vscode-glade/README.md`
- Modify: `docs/EDITOR.md`
- Modify: `site/docs-src/guide/editor.md`

- [ ] **Step 1: Update docs**

Add this section to each editor-facing doc:

```md
### Daily Local Apex Loop

Open the Glade Activity Bar and start in **Start Here**.

1. Confirm the SFDX root and active local data environment.
2. Click **Run local proof** before pushing work to a scratch org.
3. Use **Data Environments** to clone, seed, reset, inspect, and export local state.
4. Use Salesforce extension commands for org deploy, retrieve, org tests, SOQL Builder, and Code Analyzer.

Glade actions are local. Salesforce actions stay org-backed.
```

- [ ] **Step 2: Run final verification**

Run:

```bash
cd contrib/vscode-glade
npm test
npm run package
```

Expected: PASS and `dist/vscode-glade-0.0.1.vsix` is written.

Run:

```bash
cd /Users/matt/Dev/glade
go test ./internal/gladecli ./internal/cliui -count=1
git diff --check
```

Expected: PASS and no whitespace errors.

- [ ] **Step 3: Install smoke**

Run:

```bash
cd /Users/matt/Dev/glade
glade editor install vscode --force
code --list-extensions --show-versions | rg '^glade\\.vscode-glade@0\\.0\\.1$'
```

Expected: install command exits 0 and the extension id appears.

- [ ] **Step 4: Manual VS Code smoke**

Run:

```bash
code /Users/matt/Dev/glade/testdata/local-tests/enterprise-composed
```

In VS Code:

1. Run `Developer: Reload Window`.
2. Open the Glade Activity Bar.
3. Confirm the first view title is `START HERE`.
4. Confirm Start Here shows a project row, active data row, local proof row, last run row, watch row, and Salesforce extensions row.
5. Open `DATA ENVIRONMENTS`.
6. Confirm `dev` appears as active.
7. Run `Glade: Create Local Data Environment` with `bug-123`.
8. Confirm `bug-123` appears.
9. Click `bug-123` and confirm it becomes active.
10. Run `Glade: Clone Local Data Environment` from the row context.
11. Confirm a `bug-123-copy` environment appears and becomes active.

- [ ] **Step 5: Commit**

```bash
git add contrib/vscode-glade/README.md docs/EDITOR.md site/docs-src/guide/editor.md
git commit -m "docs: describe start here local workflow"
```

---

### Task 11: Status Bar, Native UI, And Brand QA

**Files:**
- Create: `contrib/vscode-glade/src/statusModel.ts`
- Create: `contrib/vscode-glade/test/statusModel.test.js`
- Modify: `contrib/vscode-glade/src/status.ts`
- Modify: `contrib/vscode-glade/src/extension.ts`
- Modify: `contrib/vscode-glade/package.json`
- Modify: `contrib/vscode-glade/README.md`
- Modify: `docs/EDITOR.md`
- Modify: `site/docs-src/guide/editor.md`

- [ ] **Step 1: Write the failing Status Bar model test**

Create `contrib/vscode-glade/test/statusModel.test.js`:

```js
const assert = require("assert");
const status = require("../out/statusModel");

assert.strictEqual(status.buildStatusText({ projectReady: false }), "Glade: no SFDX root");
assert.strictEqual(status.buildStatusText({ projectReady: true, activeEnvironment: "dev" }), "Glade: dev");
assert.strictEqual(
  status.buildStatusText({ projectReady: true, activeEnvironment: "dev", lastRun: { failed: 0, durationMs: 18 } }),
  "Glade: dev 18ms",
);
assert.strictEqual(
  status.buildStatusText({ projectReady: true, activeEnvironment: "billing-case", lastRun: { failed: 1, durationMs: 42 } }),
  "Glade: billing-case 1 fail",
);
assert.strictEqual(
  status.buildStatusText({ projectReady: true, activeEnvironment: "dev", changedRecords: 47 }),
  "Glade: dev 47 changed",
);
assert.strictEqual(
  status.buildStatusText({ projectReady: true, activeEnvironment: "dev", missingDb: true }),
  "Glade: dev no DB",
);
assert.strictEqual(
  status.buildStatusTooltip({
    projectReady: true,
    projectRoot: "/repo",
    activeEnvironment: "dev",
    dbPath: "/repo/.glade/envs/dev.sqlite",
    lastCommand: "glade test changed --project . --since origin/main --json --env dev",
  }),
  "Project: /repo\nEnvironment: dev\nDB: /repo/.glade/envs/dev.sqlite\nLast command: glade test changed --project . --since origin/main --json --env dev",
);
```

Append `&& node test/statusModel.test.js` to the `package.json` test script.

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: FAIL with `Cannot find module '../out/statusModel'`.

- [ ] **Step 3: Add the Status Bar model**

Create `contrib/vscode-glade/src/statusModel.ts`:

```ts
export interface StatusRunSummary {
  failed: number;
  durationMs?: number;
}

export interface GladeStatusSnapshot {
  projectReady: boolean;
  projectRoot?: string;
  activeEnvironment?: string;
  dbPath?: string;
  lastRun?: StatusRunSummary;
  changedRecords?: number;
  missingDb?: boolean;
  busyLabel?: string;
  lastCommand?: string;
}

export function buildStatusText(snapshot: GladeStatusSnapshot): string {
  if (!snapshot.projectReady) {
    return "Glade: no SFDX root";
  }
  if (snapshot.busyLabel) {
    return `Glade: ${snapshot.busyLabel}`;
  }
  const environment = snapshot.activeEnvironment || "dev";
  if (snapshot.missingDb) {
    return `Glade: ${environment} no DB`;
  }
  if (snapshot.lastRun && snapshot.lastRun.failed > 0) {
    return `Glade: ${environment} ${snapshot.lastRun.failed} fail`;
  }
  if (snapshot.lastRun && snapshot.lastRun.durationMs !== undefined) {
    return `Glade: ${environment} ${snapshot.lastRun.durationMs}ms`;
  }
  if (snapshot.changedRecords && snapshot.changedRecords > 0) {
    return `Glade: ${environment} ${snapshot.changedRecords} changed`;
  }
  return `Glade: ${environment}`;
}

export function buildStatusTooltip(snapshot: GladeStatusSnapshot): string {
  if (!snapshot.projectReady) {
    return "Open a Salesforce DX project with sfdx-project.json.";
  }
  const lines = [
    snapshot.projectRoot ? `Project: ${snapshot.projectRoot}` : undefined,
    `Environment: ${snapshot.activeEnvironment || "dev"}`,
    snapshot.dbPath ? `DB: ${snapshot.dbPath}` : undefined,
    snapshot.lastCommand ? `Last command: ${snapshot.lastCommand}` : undefined,
  ].filter((line): line is string => Boolean(line));
  return lines.join("\n");
}
```

- [ ] **Step 4: Wire the single Status Bar item**

Replace `contrib/vscode-glade/src/status.ts` with:

```ts
import * as vscode from "vscode";
import { configuredActiveEnvironment } from "./localOrg";
import { GladeProjectContext } from "./projectModel";
import { buildStatusText, buildStatusTooltip, GladeStatusSnapshot, StatusRunSummary } from "./statusModel";

export class GladeStatus {
  private readonly item = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 40);
  private project?: GladeProjectContext;
  private lastRun?: StatusRunSummary;
  private changedRecords?: number;
  private lastCommand?: string;
  private busyLabel?: string;
  private missingDb = false;

  constructor(context: vscode.ExtensionContext) {
    this.item.command = "glade.statusQuickPick";
    context.subscriptions.push(this.item);
  }

  setProject(project: GladeProjectContext | undefined): void {
    this.project = project;
    this.render();
  }

  setLastRun(run: StatusRunSummary | undefined, command?: string): void {
    this.lastRun = run;
    this.lastCommand = command;
    this.busyLabel = undefined;
    this.render();
  }

  setChangedRecords(count: number | undefined): void {
    this.changedRecords = count;
    this.render();
  }

  setBusy(label: string | undefined): void {
    this.busyLabel = label;
    this.render();
  }

  setMissingDb(missing: boolean): void {
    this.missingDb = missing;
    this.render();
  }

  private snapshot(): GladeStatusSnapshot {
    const environment = this.project ? configuredActiveEnvironment(this.project) : undefined;
    return {
      projectReady: Boolean(this.project),
      projectRoot: this.project?.projectRoot,
      activeEnvironment: environment?.name,
      dbPath: environment?.dbPath,
      lastRun: this.lastRun,
      changedRecords: this.changedRecords,
      missingDb: this.missingDb,
      busyLabel: this.busyLabel,
      lastCommand: this.lastCommand,
    };
  }

  private render(): void {
    const snapshot = this.snapshot();
    this.item.text = buildStatusText(snapshot);
    this.item.tooltip = buildStatusTooltip(snapshot);
    this.item.show();
  }
}
```

Do not set `item.backgroundColor` or custom Status Bar colors. VS Code guidance says warning/error Status Bar backgrounds are a last resort.

- [ ] **Step 5: Add the Status Bar Quick Pick**

In `contrib/vscode-glade/src/extension.ts`, register:

```ts
vscode.commands.registerCommand("glade.statusQuickPick", async () => {
  const picked = await vscode.window.showQuickPick(
    [
      { label: "Switch Local Data Environment", command: "glade.switchEnvironment" },
      { label: "Inspect Active Local Data", command: "glade.inspectLocalOrg" },
      { label: "Run Local Proof", command: "glade.runLocalProof" },
      { label: "Open Glade Output", command: "glade.openOutput" },
    ],
    { placeHolder: "Glade local workflow" },
  );
  if (picked) {
    await vscode.commands.executeCommand(picked.command);
  }
});
```

If `glade.openOutput` does not exist yet, add:

```ts
vscode.commands.registerCommand("glade.openOutput", () => {
  output.tests.show(true);
});
```

Add both commands to `package.json`:

```json
{
  "command": "glade.statusQuickPick",
  "title": "Glade: Show Local Status"
},
{
  "command": "glade.openOutput",
  "title": "Glade: Open Output"
}
```

- [ ] **Step 6: Update local proof state**

In `glade.runLocalProof`, before command execution:

```ts
const command = `glade test changed --project ${project.projectRoot} --since ${changedSince} --json`;
status.setBusy("running");
output.tests.appendLine(`$ ${command}`);
```

After a successful run:

```ts
const summary = startHereSummary("Changed tests", run);
startHereState.setLastRun(summary);
status.setLastRun({ failed: summary.failed, durationMs: summary.durationMs }, command);
```

After inspect updates the local org summary:

```ts
status.setChangedRecords(undefined);
```

When an inspect command reports changed-record count in a later task, call:

```ts
status.setChangedRecords(changedCount);
```

In the catch block:

```ts
status.setLastRun({ failed: 1 }, command);
```

- [ ] **Step 7: Update docs with brand and native-surface rules**

Add this section to `contrib/vscode-glade/README.md`, `docs/EDITOR.md`, and `site/docs-src/guide/editor.md`:

```md
### Native VS Code Surfaces

Glade uses one Activity Bar item and one Status Bar item. The sidebar shows
Start Here, Local Runs, Data Environments, Local Org, and Debug.

Local Apex tests appear in the native VS Code Testing view under `Glade Apex`.
Glade does not add a second Apex Tests sidebar tree. Breakpoints stay in the
normal editor gutter and debug state stays in VS Code Run and Debug.

The Status Bar shows the active local data environment and the latest local
state, such as `Glade: dev`, `Glade: dev 18ms`, or `Glade: billing-case 1 fail`.
Click it to switch data, inspect local data, run local proof, or open output.
```

- [ ] **Step 8: Run automated verification**

Run:

```bash
cd contrib/vscode-glade
npm test
npm run package
cd /Users/matt/Dev/glade
git diff --check
```

Expected: `npm test` exits 0, `npm run package` writes `dist/vscode-glade-0.0.1.vsix`, and `git diff --check` reports no whitespace errors.

- [ ] **Step 9: Run visual acceptance against the brand mockup**

Open:

```bash
open /tmp/picture-it/glade-vscode-extension-brand-iteration.html
code /Users/matt/Dev/glade/testdata/local-tests/enterprise-composed
```

In VS Code, verify:

1. The Glade Activity Bar uses the existing boxed mark.
2. The sidebar order is `START HERE`, `LOCAL RUNS`, `DATA ENVIRONMENTS`, `LOCAL ORG`, `DEBUG`.
3. No `APEX TESTS` sidebar view appears.
4. The native Testing view still shows `Glade Apex`.
5. The Status Bar shows exactly one Glade item.
6. Success state reads like `Glade: dev 18ms`.
7. Failure state reads like `Glade: billing-case 1 fail`.
8. Active data is visible in Start Here, Data Environments, Local Org, Debug, and Output.
9. Full DB paths appear in tooltips or Output, not crowded row labels.
10. Normal success produces no notification popup.

- [ ] **Step 10: Commit**

```bash
git add contrib/vscode-glade/src/statusModel.ts contrib/vscode-glade/src/status.ts contrib/vscode-glade/src/extension.ts contrib/vscode-glade/package.json contrib/vscode-glade/test/statusModel.test.js contrib/vscode-glade/README.md docs/EDITOR.md site/docs-src/guide/editor.md
git commit -m "feat: polish glade vscode status and native surfaces"
```

---

## Stretch Runway

Use these only after Tasks 1-11 pass.

1. **Environment seed history**
   - Track last fixture path and last seed time in workspace settings.
   - Show `Last seeded: fixtures/bug-123.json` in the environment row.

2. **Environment quick create from branch**
   - Add `Glade: Create Environment From Git Branch`.
   - Convert `feature/foo` to `feature-foo`.
   - Use `.glade/envs/feature-foo.sqlite`.

3. **Local DB object drilldown**
   - Add collapsible object rows under Local Org.
   - First level shows object counts only.
   - Do not build a full record grid in this slice.

4. **Proof handoff**
   - Add a Start Here row with suggested next command:
     `sf project deploy validate --source-dir <changed paths> --test-level RunLocalTests`.
   - Keep it as copyable text, not an automatic deploy.

5. **Local record browser webview**
   - Add a webview only for local record tables and local SOQL results.
   - Follow the mockup rule: tree views remain the daily surface; the webview is a detail tab.
   - Use VS Code theme colors and keyboard navigation.

6. **Run report webview**
   - Add a rich report for failed local proof runs.
   - Show failure stack, affected tests, active environment, and exact command.
   - Keep the normal happy path in Output and Status Bar.

## Self-Review

- Spec coverage:
  - Start Here Panel is covered by Tasks 1-4 and 8.
  - Data Environments are covered by Tasks 5-7 and 9.
  - Native Testing ownership is covered by Tasks 1 and 4.
  - Brand, Status Bar, mockup acceptance, and notification restraint are covered by Task 11.
  - Docs and install smoke are covered by Tasks 10 and 11.
- Placeholder scan:
  - No red-flag marker strings.
  - No unnamed test step.
  - No command without expected output.
- Type consistency:
  - `StartHereState`, `StartHereSnapshot`, and `StartHereRunSummary` match across Tasks 2, 3, 4, and 8.
  - `GladeEnvironment` is reused from `environments.ts`.
  - `summaryFromInspect` remains the source for local org summary rows.
