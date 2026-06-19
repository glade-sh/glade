# VS Code Developer Hub Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a task-first Glade Home for the VS Code extension, with a second State tab that shows the current project, org, data, test, Salesforce, and plugin state.

**Architecture:** Add an editor-area `WebviewPanel` opened by `Glade: Open Home`, backed by pure hub model builders and a small command allowlist. Home starts daily work; State explains what the commands will touch. Existing sidebar tree views stay in place for narrow, focused access.

**Tech Stack:** VS Code extension API, TypeScript, existing Glade CLI wrappers, existing test harness using `tsc` plus Node `assert`, standalone HTML/CSS/JS rendered inside a VS Code webview.

---

## Product Shape

The hub has two tabs.

Home is the default tab. It is task-first:

- Run: changed tests, failed tests, current test, watch, local proof.
- Data: create or switch local data environments, inspect, seed, reset, export, browse, SOQL scratch, anonymous Apex scratch.
- Debug: debug current test, debug anonymous Apex, open debug output.
- Salesforce: check SF CLI target, import describe JSON through Glade, run capture plugin action when one is present.
- Ship: run local proof, open outputs, inspect plugin findings, open reports.

State is the second tab. It is read-first:

- Project: root, API version, package dirs.
- Glade org: local endpoint when known.
- Data environment: active name, DB path, missing DB flag, record summary.
- Salesforce target: SF CLI default target check result or unknown.
- Tests: watch state, last run, changed-since ref.
- Plugins: action count, latest findings count.

Core Glade owns local work. Live Salesforce fixture capture stays a plugin action.

## Files

- Create `contrib/vscode-glade/src/hub/model.ts`
  - Pure TypeScript types and builders for Home task groups and State sections.
- Create `contrib/vscode-glade/src/hub/actions.ts`
  - Allowed hub command ids and message validation.
- Create `contrib/vscode-glade/src/hub/html.ts`
  - Webview HTML renderer, escaping helpers, CSS, and client-side tab/action script.
- Create `contrib/vscode-glade/src/hub/controller.ts`
  - `GladeHomeController` that owns a `WebviewPanel`, posts snapshots, and runs allowed commands.
- Create `contrib/vscode-glade/test/hubModel.test.js`
  - Pure model coverage for task-first Home and State.
- Create `contrib/vscode-glade/test/hubActions.test.js`
  - Allowlist and message validation tests.
- Create `contrib/vscode-glade/test/hubHtml.test.js`
  - Render tests for tabs, escaped data, command ids, and no raw script injection.
- Modify `contrib/vscode-glade/src/extension.ts`
  - Instantiate the hub, build snapshots from existing extension state, register commands, and refresh the hub after state changes.
- Modify `contrib/vscode-glade/src/startHereModel.ts`
  - Add an "Open Glade Home" first row when a project exists.
- Modify `contrib/vscode-glade/src/localOrg.ts`
  - Add terminal helpers for schema import describe and SF CLI target check.
- Modify `contrib/vscode-glade/package.json`
  - Add command contributions and activation event for `glade.openHome`, `glade.schemaImportDescribe`, and `glade.salesforceTargetStatus`.
- Modify `contrib/vscode-glade/test/package.test.js`
  - Pin the new commands and keep parked preview features out.
- Modify `contrib/vscode-glade/test/startHereModel.test.js`
  - Pin the new Home row.
- Modify `contrib/vscode-glade/package.json` test script
  - Add `hubModel.test.js`, `hubActions.test.js`, and `hubHtml.test.js`.
- Modify `contrib/vscode-glade/README.md`
  - Document Glade Home and the Home/State split.

## Task 1: Hub Model

**Files:**
- Create: `contrib/vscode-glade/src/hub/model.ts`
- Test: `contrib/vscode-glade/test/hubModel.test.js`
- Modify: `contrib/vscode-glade/package.json`

- [ ] **Step 1: Write the failing model test**

Create `contrib/vscode-glade/test/hubModel.test.js`:

```js
const assert = require("assert");
const hub = require("../out/hub/model");

const snapshot = {
  project: {
    workspaceFolder: "/repo",
    projectRoot: "/repo",
    configFound: true,
    namespace: "acme",
    sourceApiVersion: "63.0",
    packageDirs: ["force-app", "unpackaged"],
  },
  activeEnvironment: { name: "dev", dbPath: "/repo/.glade/envs/dev.sqlite" },
  localOrgSummary: { objects: 61, records: 1284, users: 18, profiles: 4, permissions: 11 },
  missingDb: false,
  watchRunning: true,
  lastRun: { label: "Changed tests", passed: 31, failed: 0, durationMs: 18 },
  changedSince: "origin/main",
  pluginActionCount: 3,
  pluginFindingCount: 2,
  salesforceTarget: { label: "core-scratch", state: "stale", detail: "3 days old" },
};

const home = hub.buildHubHome(snapshot);
assert.deepStrictEqual(home.map((group) => group.id), ["run", "data", "debug", "salesforce", "ship"]);
assert.strictEqual(home[0].title, "Run");
assert.strictEqual(home[0].primary.command, "glade.runLocalProof");
assert(home[0].actions.some((action) => action.command === "glade.runChangedTests"));
assert(home[1].actions.some((action) => action.command === "glade.seedLocalOrg"));
assert(home[1].actions.some((action) => action.command === "glade.workbench.newSoql"));
assert(home[2].actions.some((action) => action.command === "glade.debugCurrentTest"));
assert(home[3].actions.some((action) => action.command === "glade.schemaImportDescribe"));
assert(home[3].actions.some((action) => action.command === "glade.runPluginAction"));
assert(home[4].actions.some((action) => action.command === "glade.openOutput"));
assert.strictEqual(home[4].status.tone, "ok");

const state = hub.buildHubState(snapshot);
assert.deepStrictEqual(state.map((section) => section.id), [
  "project",
  "local-org",
  "data",
  "salesforce",
  "tests",
  "plugins",
]);
assert.strictEqual(state.find((section) => section.id === "project").rows[0].value, "/repo");
assert.strictEqual(state.find((section) => section.id === "data").rows.find((row) => row.label === "Records").value, "1284");
assert.strictEqual(state.find((section) => section.id === "salesforce").tone, "warn");
assert.strictEqual(state.find((section) => section.id === "plugins").rows.find((row) => row.label === "Findings").value, "2");

const missing = hub.buildHubHome({ project: undefined, changedSince: "origin/main" });
assert.strictEqual(missing[0].primary.command, "vscode.openFolder");
assert.strictEqual(missing[0].status.tone, "warn");

const noDb = hub.buildHubState({
  project: snapshot.project,
  activeEnvironment: { name: "empty", dbPath: "/repo/.glade/envs/empty.sqlite" },
  missingDb: true,
  changedSince: "origin/main",
});
assert.strictEqual(noDb.find((section) => section.id === "data").tone, "warn");
assert.strictEqual(noDb.find((section) => section.id === "data").rows.find((row) => row.label === "Records").value, "no DB");
```

- [ ] **Step 2: Add the test to the package script**

Modify `contrib/vscode-glade/package.json` and insert `node test/hubModel.test.js` after `node test/startHereState.test.js` in the `test` script.

- [ ] **Step 3: Run the model test and see it fail**

Run:

```bash
cd contrib/vscode-glade
npm run compile && node test/hubModel.test.js
```

Expected: `tsc` fails because `src/hub/model.ts` does not exist, or Node fails with `Cannot find module '../out/hub/model'`.

- [ ] **Step 4: Implement the model**

Create `contrib/vscode-glade/src/hub/model.ts`:

```ts
import { GladeEnvironment } from "../environments";
import { LocalOrgSummary } from "../localOrgModel";
import { GladeProjectContext } from "../projectModel";
import { StartHereRunSummary } from "../startHereModel";

export type HubTone = "ok" | "warn" | "error" | "muted";
export type HubTaskId = "run" | "data" | "debug" | "salesforce" | "ship";
export type HubStateId = "project" | "local-org" | "data" | "salesforce" | "tests" | "plugins";

export interface SalesforceTargetState {
  label: string;
  state: "ready" | "stale" | "missing" | "unknown";
  detail?: string;
}

export interface HubSnapshot {
  project?: GladeProjectContext;
  activeEnvironment?: GladeEnvironment;
  localOrgSummary?: LocalOrgSummary;
  missingDb?: boolean;
  watchRunning?: boolean;
  lastRun?: StartHereRunSummary;
  changedSince: string;
  pluginActionCount?: number;
  pluginFindingCount?: number;
  salesforceTarget?: SalesforceTargetState;
}

export interface HubStatus {
  label: string;
  detail?: string;
  tone: HubTone;
}

export interface HubAction {
  id: string;
  label: string;
  command: string;
  description?: string;
  primary?: boolean;
  disabledReason?: string;
}

export interface HubTaskGroup {
  id: HubTaskId;
  title: string;
  summary: string;
  status: HubStatus;
  primary: HubAction;
  actions: HubAction[];
}

export interface HubStateRow {
  label: string;
  value: string;
  detail?: string;
}

export interface HubStateSection {
  id: HubStateId;
  title: string;
  tone: HubTone;
  rows: HubStateRow[];
}

export function buildHubHome(snapshot: HubSnapshot): HubTaskGroup[] {
  if (!snapshot.project) {
    return [
      {
        id: "run",
        title: "Open project",
        summary: "Open a Salesforce DX project before using Glade.",
        status: { label: "No SFDX project", tone: "warn" },
        primary: {
          id: "open-project",
          label: "Open folder",
          command: "vscode.openFolder",
          primary: true,
        },
        actions: [],
      },
    ];
  }

  const dataStatus = dataStatusFor(snapshot);
  const lastRun = snapshot.lastRun;
  const failed = lastRun?.failed || 0;
  const runTone: HubTone = failed > 0 ? "error" : lastRun ? "ok" : "muted";
  const target = snapshot.salesforceTarget || { label: "default target", state: "unknown" as const, detail: "not checked" };
  const targetTone: HubTone = target.state === "ready" ? "ok" : target.state === "missing" ? "error" : target.state === "stale" ? "warn" : "muted";
  const findings = snapshot.pluginFindingCount || 0;

  return [
    {
      id: "run",
      title: "Run",
      summary: "Run the local Apex loop from the current branch.",
      status: {
        label: lastRun ? `${lastRun.passed} pass, ${lastRun.failed} fail` : "No local run",
        detail: `changed since ${snapshot.changedSince}`,
        tone: runTone,
      },
      primary: { id: "run-proof", label: "Run proof", command: "glade.runLocalProof", primary: true },
      actions: [
        { id: "changed", label: "Changed tests", command: "glade.runChangedTests" },
        { id: "failed", label: "Failed tests", command: "glade.runFailedTests" },
        { id: "watch", label: snapshot.watchRunning ? "Stop watch" : "Start watch", command: snapshot.watchRunning ? "glade.stopWatch" : "glade.startWatch" },
      ],
    },
    {
      id: "data",
      title: "Data",
      summary: "Work with the active local data environment.",
      status: dataStatus,
      primary: { id: "inspect-db", label: "Inspect DB", command: "glade.inspectLocalOrg", primary: true },
      actions: [
        { id: "switch-env", label: "Switch env", command: "glade.switchEnvironment" },
        { id: "create-env", label: "Create env", command: "glade.createEnvironment" },
        { id: "clone-env", label: "Clone env", command: "glade.cloneEnvironment" },
        { id: "seed", label: "Seed", command: "glade.seedLocalOrg" },
        { id: "reset", label: "Reset", command: "glade.resetLocalOrg" },
        { id: "export", label: "Export", command: "glade.exportLocalOrg" },
        { id: "soql", label: "SOQL scratch", command: "glade.workbench.newSoql" },
        { id: "apex", label: "Apex scratch", command: "glade.workbench.newAnonymousApex" },
      ],
    },
    {
      id: "debug",
      title: "Debug",
      summary: "Start local debug work from the current editor.",
      status: { label: "Editor scoped", detail: "uses active Apex context", tone: "muted" },
      primary: { id: "debug-current", label: "Debug current test", command: "glade.debugCurrentTest", primary: true },
      actions: [
        { id: "apex-scratch", label: "Apex scratch", command: "glade.workbench.newAnonymousApex" },
        { id: "output", label: "Open output", command: "glade.openOutput" },
      ],
    },
    {
      id: "salesforce",
      title: "Salesforce",
      summary: "Check the org-backed target and import describe data.",
      status: { label: target.label, detail: target.detail, tone: targetTone },
      primary: { id: "sf-target", label: "Check target", command: "glade.salesforceTargetStatus", primary: true },
      actions: [
        { id: "schema", label: "Import schema", command: "glade.schemaImportDescribe" },
        { id: "capture", label: "Capture fixture", command: "glade.runPluginAction", description: "Runs a plugin action when installed." },
        { id: "plugins", label: "Manage plugins", command: "glade.managePlugins" },
      ],
    },
    {
      id: "ship",
      title: "Ship",
      summary: "Gather local proof and findings before pushing.",
      status: {
        label: findings > 0 ? `${findings} findings` : "No plugin findings",
        detail: lastRun ? lastRun.label : "run proof first",
        tone: failed > 0 || findings > 0 ? "warn" : lastRun ? "ok" : "muted",
      },
      primary: { id: "ship-proof", label: "Run proof", command: "glade.runLocalProof", primary: true },
      actions: [
        { id: "output", label: "Open output", command: "glade.openOutput" },
        { id: "plugins", label: "Plugin findings", command: "glade.managePlugins" },
        { id: "refresh", label: "Refresh", command: "glade.refresh" },
      ],
    },
  ];
}

export function buildHubState(snapshot: HubSnapshot): HubStateSection[] {
  const project = snapshot.project;
  const env = snapshot.activeEnvironment;
  const summary = snapshot.localOrgSummary;
  const target = snapshot.salesforceTarget || { label: "default target", state: "unknown" as const, detail: "not checked" };
  const targetTone: HubTone = target.state === "ready" ? "ok" : target.state === "stale" ? "warn" : target.state === "missing" ? "error" : "muted";

  return [
    {
      id: "project",
      title: "Project",
      tone: project ? "ok" : "warn",
      rows: [
        { label: "Root", value: project?.projectRoot || "none" },
        { label: "API", value: project?.sourceApiVersion || "unknown" },
        { label: "Package dirs", value: project?.packageDirs?.join(", ") || "none" },
      ],
    },
    {
      id: "local-org",
      title: "Glade org",
      tone: project ? "ok" : "muted",
      rows: [
        { label: "Endpoint", value: "127.0.0.1:17911", detail: "default local org port" },
        { label: "Project config", value: project?.configFound ? "loaded" : "defaults" },
      ],
    },
    {
      id: "data",
      title: "Data environment",
      tone: snapshot.missingDb ? "warn" : env ? "ok" : "muted",
      rows: [
        { label: "Active", value: env?.name || "dev" },
        { label: "DB", value: env?.dbPath || "not configured" },
        { label: "Records", value: snapshot.missingDb ? "no DB" : String(summary?.records || 0) },
        { label: "Objects", value: snapshot.missingDb ? "no DB" : String(summary?.objects || 0) },
      ],
    },
    {
      id: "salesforce",
      title: "Salesforce target",
      tone: targetTone,
      rows: [
        { label: "Target", value: target.label },
        { label: "State", value: target.state, detail: target.detail },
      ],
    },
    {
      id: "tests",
      title: "Tests",
      tone: snapshot.lastRun?.failed ? "error" : snapshot.lastRun ? "ok" : "muted",
      rows: [
        { label: "Watch", value: snapshot.watchRunning ? "running" : "stopped" },
        { label: "Changed since", value: snapshot.changedSince },
        { label: "Last run", value: snapshot.lastRun ? `${snapshot.lastRun.passed} pass, ${snapshot.lastRun.failed} fail` : "none" },
      ],
    },
    {
      id: "plugins",
      title: "Plugins",
      tone: snapshot.pluginFindingCount ? "warn" : "muted",
      rows: [
        { label: "Actions", value: String(snapshot.pluginActionCount || 0) },
        { label: "Findings", value: String(snapshot.pluginFindingCount || 0) },
      ],
    },
  ];
}

function dataStatusFor(snapshot: HubSnapshot): HubStatus {
  const env = snapshot.activeEnvironment;
  if (snapshot.missingDb) {
    return { label: `${env?.name || "dev"} has no DB`, detail: env?.dbPath, tone: "warn" };
  }
  if (snapshot.localOrgSummary) {
    return {
      label: `${snapshot.localOrgSummary.records} records`,
      detail: env?.name || "dev",
      tone: "ok",
    };
  }
  return { label: env?.name || "dev", detail: "not inspected", tone: "muted" };
}
```

- [ ] **Step 5: Run the model test**

Run:

```bash
cd contrib/vscode-glade
npm run compile && node test/hubModel.test.js
```

Expected: command exits `0`.

- [ ] **Step 6: Commit**

```bash
git add contrib/vscode-glade/src/hub/model.ts contrib/vscode-glade/test/hubModel.test.js contrib/vscode-glade/package.json
git commit -m "feat(editor): add developer hub model"
```

## Task 2: Hub Command Allowlist

**Files:**
- Create: `contrib/vscode-glade/src/hub/actions.ts`
- Test: `contrib/vscode-glade/test/hubActions.test.js`
- Modify: `contrib/vscode-glade/package.json`

- [ ] **Step 1: Write the failing action test**

Create `contrib/vscode-glade/test/hubActions.test.js`:

```js
const assert = require("assert");
const actions = require("../out/hub/actions");

assert.strictEqual(actions.isHubCommand("glade.runLocalProof"), true);
assert.strictEqual(actions.isHubCommand("glade.schemaImportDescribe"), true);
assert.strictEqual(actions.isHubCommand("glade.salesforceTargetStatus"), true);
assert.strictEqual(actions.isHubCommand("workbench.action.files.delete"), false);
assert.strictEqual(actions.isHubCommand("glade.preview.start"), false);

assert.deepStrictEqual(actions.parseHubMessage({ type: "ready" }), { type: "ready" });
assert.deepStrictEqual(actions.parseHubMessage({ type: "runCommand", command: "glade.runLocalProof" }), {
  type: "runCommand",
  command: "glade.runLocalProof",
});
assert.throws(
  () => actions.parseHubMessage({ type: "runCommand", command: "workbench.action.files.delete" }),
  /command is not allowed/,
);
assert.throws(
  () => actions.parseHubMessage({ type: "unknown" }),
  /unsupported hub message/,
);
```

- [ ] **Step 2: Add the test to the package script**

Insert `node test/hubActions.test.js` after `node test/hubModel.test.js` in `contrib/vscode-glade/package.json`.

- [ ] **Step 3: Run the action test and see it fail**

Run:

```bash
cd contrib/vscode-glade
npm run compile && node test/hubActions.test.js
```

Expected: `tsc` fails because `src/hub/actions.ts` does not exist, or Node fails with `Cannot find module '../out/hub/actions'`.

- [ ] **Step 4: Implement the allowlist**

Create `contrib/vscode-glade/src/hub/actions.ts`:

```ts
export interface HubReadyMessage {
  type: "ready";
}

export interface HubRunCommandMessage {
  type: "runCommand";
  command: string;
}

export type HubClientMessage = HubReadyMessage | HubRunCommandMessage;

export const allowedHubCommands = new Set<string>([
  "vscode.openFolder",
  "glade.refresh",
  "glade.openHome",
  "glade.runChangedTests",
  "glade.runFailedTests",
  "glade.runLocalProof",
  "glade.startWatch",
  "glade.stopWatch",
  "glade.debugCurrentTest",
  "glade.createEnvironment",
  "glade.switchEnvironment",
  "glade.cloneEnvironment",
  "glade.seedLocalOrg",
  "glade.resetLocalOrg",
  "glade.exportLocalOrg",
  "glade.inspectLocalOrg",
  "glade.workbench.newSoql",
  "glade.runSoql",
  "glade.workbench.newAnonymousApex",
  "glade.workbench.describe",
  "glade.workbench.openResult",
  "glade.refreshPlugins",
  "glade.managePlugins",
  "glade.runPluginAction",
  "glade.openOutput",
  "glade.schemaImportDescribe",
  "glade.salesforceTargetStatus",
]);

export function isHubCommand(command: string): boolean {
  return allowedHubCommands.has(command);
}

export function parseHubMessage(value: unknown): HubClientMessage {
  if (!value || typeof value !== "object") {
    throw new Error("unsupported hub message");
  }
  const record = value as Record<string, unknown>;
  if (record.type === "ready") {
    return { type: "ready" };
  }
  if (record.type === "runCommand" && typeof record.command === "string") {
    if (!isHubCommand(record.command)) {
      throw new Error(`hub command is not allowed: ${record.command}`);
    }
    return { type: "runCommand", command: record.command };
  }
  throw new Error("unsupported hub message");
}
```

- [ ] **Step 5: Run the action test**

Run:

```bash
cd contrib/vscode-glade
npm run compile && node test/hubActions.test.js
```

Expected: command exits `0`.

- [ ] **Step 6: Commit**

```bash
git add contrib/vscode-glade/src/hub/actions.ts contrib/vscode-glade/test/hubActions.test.js contrib/vscode-glade/package.json
git commit -m "feat(editor): gate developer hub actions"
```

## Task 3: Hub HTML Renderer

**Files:**
- Create: `contrib/vscode-glade/src/hub/html.ts`
- Test: `contrib/vscode-glade/test/hubHtml.test.js`
- Modify: `contrib/vscode-glade/package.json`
- Reference: `contrib/vscode-glade/prototypes/local-org-dashboard.html`

- [ ] **Step 1: Write the failing renderer test**

Create `contrib/vscode-glade/test/hubHtml.test.js`:

```js
const assert = require("assert");
const html = require("../out/hub/html");

const snapshot = {
  project: {
    workspaceFolder: "/repo",
    projectRoot: "/repo/<script>alert(1)</script>",
    configFound: true,
    namespace: "",
    sourceApiVersion: "63.0",
    packageDirs: ["force-app"],
  },
  activeEnvironment: { name: "dev", dbPath: "/repo/.glade/envs/dev.sqlite" },
  localOrgSummary: { objects: 61, records: 1284, users: 18, profiles: 4, permissions: 11 },
  changedSince: "origin/main",
  pluginActionCount: 2,
  pluginFindingCount: 0,
};

const rendered = html.renderHubHtml(snapshot, {
  cspSource: "vscode-resource:",
  nonce: "abc123",
  initialTab: "home",
});

assert(rendered.includes("Glade Home"));
assert(rendered.includes('data-tab="home"'));
assert(rendered.includes('data-tab="state"'));
assert(rendered.includes('data-command="glade.runLocalProof"'));
assert(rendered.includes('data-command="glade.inspectLocalOrg"'));
assert(rendered.includes("Run"));
assert(rendered.includes("Data"));
assert(rendered.includes("Salesforce"));
assert(rendered.includes("&lt;script&gt;alert(1)&lt;/script&gt;"));
assert(!rendered.includes("/repo/<script>alert(1)</script>"));
assert(rendered.includes("script-src 'nonce-abc123'"));
```

- [ ] **Step 2: Add the test to the package script**

Insert `node test/hubHtml.test.js` after `node test/hubActions.test.js` in `contrib/vscode-glade/package.json`.

- [ ] **Step 3: Run the renderer test and see it fail**

Run:

```bash
cd contrib/vscode-glade
npm run compile && node test/hubHtml.test.js
```

Expected: `tsc` fails because `src/hub/html.ts` does not exist, or Node fails with `Cannot find module '../out/hub/html'`.

- [ ] **Step 4: Implement the renderer shell**

Create `contrib/vscode-glade/src/hub/html.ts`. Use the visual structure from `contrib/vscode-glade/prototypes/local-org-dashboard.html`, but change the heading to `Glade Home`, add top-level tabs `Home` and `State`, and render task groups from `buildHubHome()` plus state sections from `buildHubState()`.

Start the file with these exports and helpers:

```ts
import { buildHubHome, buildHubState, HubAction, HubSnapshot, HubStateSection, HubTaskGroup, HubTone } from "./model";

export interface HubHtmlOptions {
  cspSource: string;
  nonce: string;
  initialTab?: "home" | "state";
}

export function renderHubHtml(snapshot: HubSnapshot, options: HubHtmlOptions): string {
  const initialTab = options.initialTab || "home";
  const home = buildHubHome(snapshot);
  const state = buildHubState(snapshot);
  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src ${options.cspSource} https: data:; style-src 'nonce-${options.nonce}'; script-src 'nonce-${options.nonce}';">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Glade Home</title>
  <style nonce="${options.nonce}">${hubCss()}</style>
</head>
<body>
  <main class="hub" data-active-tab="${escapeAttr(initialTab)}">
    <header class="topbar">
      <div>
        <h1>Glade Home</h1>
        <p>${escapeHtml(snapshot.project?.projectRoot || "Open a Salesforce DX project")}</p>
      </div>
      <nav class="tabs" aria-label="Glade Home tabs">
        <button type="button" data-tab="home" class="${initialTab === "home" ? "active" : ""}">Home</button>
        <button type="button" data-tab="state" class="${initialTab === "state" ? "active" : ""}">State</button>
      </nav>
    </header>
    <section id="home" class="tab-panel ${initialTab === "home" ? "" : "hidden"}">
      ${home.map(renderTaskGroup).join("")}
    </section>
    <section id="state" class="tab-panel state-grid ${initialTab === "state" ? "" : "hidden"}">
      ${state.map(renderStateSection).join("")}
    </section>
  </main>
  <script nonce="${options.nonce}">${hubScript()}</script>
</body>
</html>`;
}

function renderTaskGroup(group: HubTaskGroup): string {
  return `<article class="task tone-${group.status.tone}">
    <header>
      <div>
        <h2>${escapeHtml(group.title)}</h2>
        <p>${escapeHtml(group.summary)}</p>
      </div>
      <span class="status">${escapeHtml(group.status.label)}</span>
    </header>
    <button type="button" class="primary" data-command="${escapeAttr(group.primary.command)}">${escapeHtml(group.primary.label)}</button>
    <div class="actions">
      ${group.actions.map(renderAction).join("")}
    </div>
  </article>`;
}

function renderAction(action: HubAction): string {
  return `<button type="button" data-command="${escapeAttr(action.command)}" title="${escapeAttr(action.description || action.label)}">${escapeHtml(action.label)}</button>`;
}

function renderStateSection(section: HubStateSection): string {
  return `<article class="state tone-${section.tone}">
    <h2>${escapeHtml(section.title)}</h2>
    <dl>
      ${section.rows.map((row) => `<div><dt>${escapeHtml(row.label)}</dt><dd title="${escapeAttr(row.detail || row.value)}">${escapeHtml(row.value)}</dd></div>`).join("")}
    </dl>
  </article>`;
}

function hubCss(): string {
  return `
    :root { color-scheme: dark; --bg: #1e1f22; --panel: #25262a; --panel2: #18191c; --border: #363941; --text: #e6e7ea; --muted: #9da3ad; --blue: #2f6fed; --ok: #53c27f; --warn: #d7a84a; --bad: #f26d6d; }
    * { box-sizing: border-box; letter-spacing: 0; }
    body { margin: 0; background: var(--bg); color: var(--text); font-family: var(--vscode-font-family, system-ui, sans-serif); font-size: var(--vscode-font-size, 13px); }
    button { min-height: 30px; border: 1px solid var(--border); border-radius: 5px; padding: 0 10px; background: #30333b; color: var(--text); cursor: pointer; }
    button.primary { background: var(--blue); border-color: var(--blue); color: white; font-weight: 650; }
    .hub { min-height: 100vh; display: grid; grid-template-rows: auto 1fr; }
    .topbar { display: flex; justify-content: space-between; gap: 16px; align-items: center; padding: 12px 16px; border-bottom: 1px solid var(--border); background: #191a1d; }
    h1, h2, p { margin: 0; }
    .topbar h1 { font-size: 16px; }
    .topbar p { color: var(--muted); margin-top: 4px; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 58vw; }
    .tabs { display: inline-flex; border: 1px solid var(--border); border-radius: 6px; overflow: hidden; }
    .tabs button { border: 0; border-radius: 0; background: transparent; }
    .tabs button.active { background: rgba(47, 111, 237, .25); color: #9cc7ff; }
    .tab-panel { padding: 14px; display: grid; grid-template-columns: repeat(2, minmax(260px, 1fr)); gap: 12px; align-content: start; }
    .state-grid { grid-template-columns: repeat(3, minmax(220px, 1fr)); }
    .hidden { display: none; }
    .task, .state { min-width: 0; border: 1px solid var(--border); border-radius: 8px; background: var(--panel); padding: 12px; display: grid; gap: 12px; }
    .task header { display: flex; justify-content: space-between; gap: 12px; align-items: start; }
    .task h2, .state h2 { font-size: 14px; }
    .task p { color: var(--muted); margin-top: 5px; line-height: 1.35; }
    .status { border: 1px solid var(--border); border-radius: 999px; padding: 3px 8px; color: var(--muted); white-space: nowrap; }
    .actions { display: flex; flex-wrap: wrap; gap: 8px; }
    .tone-ok { box-shadow: inset 3px 0 0 var(--ok); }
    .tone-warn { box-shadow: inset 3px 0 0 var(--warn); }
    .tone-error { box-shadow: inset 3px 0 0 var(--bad); }
    dl { margin: 0; display: grid; gap: 8px; }
    dl div { display: grid; grid-template-columns: 96px minmax(0, 1fr); gap: 8px; }
    dt { color: var(--muted); }
    dd { margin: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    @media (max-width: 820px) { .topbar { align-items: stretch; flex-direction: column; } .tab-panel, .state-grid { grid-template-columns: 1fr; } .topbar p { max-width: 100%; } }
  `;
}

function hubScript(): string {
  return `
    const vscode = acquireVsCodeApi();
    document.querySelectorAll("[data-tab]").forEach((button) => {
      button.addEventListener("click", () => {
        document.querySelectorAll("[data-tab]").forEach((item) => item.classList.remove("active"));
        button.classList.add("active");
        document.querySelectorAll(".tab-panel").forEach((panel) => panel.classList.add("hidden"));
        document.getElementById(button.dataset.tab).classList.remove("hidden");
      });
    });
    document.querySelectorAll("[data-command]").forEach((button) => {
      button.addEventListener("click", () => {
        vscode.postMessage({ type: "runCommand", command: button.dataset.command });
      });
    });
    vscode.postMessage({ type: "ready" });
  `;
}

function escapeHtml(value: string): string {
  return value.replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[char] || char));
}

function escapeAttr(value: string): string {
  return escapeHtml(value);
}
```

- [ ] **Step 5: Run the renderer test**

Run:

```bash
cd contrib/vscode-glade
npm run compile && node test/hubHtml.test.js
```

Expected: command exits `0`.

- [ ] **Step 6: Commit**

```bash
git add contrib/vscode-glade/src/hub/html.ts contrib/vscode-glade/test/hubHtml.test.js contrib/vscode-glade/package.json
git commit -m "feat(editor): render developer hub webview"
```

## Task 4: Hub Controller And Extension Wiring

**Files:**
- Create: `contrib/vscode-glade/src/hub/controller.ts`
- Modify: `contrib/vscode-glade/src/extension.ts`
- Modify: `contrib/vscode-glade/package.json`
- Test: `contrib/vscode-glade/test/package.test.js`

- [ ] **Step 1: Update package test first**

Modify `contrib/vscode-glade/test/package.test.js`:

1. Add `"glade.openHome"` to the command contribution loop.
2. Add `"glade.schemaImportDescribe"` to the command contribution loop.
3. Add `"glade.salesforceTargetStatus"` to the command contribution loop.
4. Add this assertion near the activation event checks:

```js
assert(activationEvents.includes("onCommand:glade.openHome"), "Glade Home command must activate the extension");
```

- [ ] **Step 2: Run package test and see it fail**

Run:

```bash
cd contrib/vscode-glade
npm run compile && node test/package.test.js
```

Expected: assertion fails because `glade.openHome` is not contributed.

- [ ] **Step 3: Add package contributions**

Modify `contrib/vscode-glade/package.json`:

1. Add `"onCommand:glade.openHome"` to `activationEvents`.
2. Add `"onCommand:glade.schemaImportDescribe"` to `activationEvents`.
3. Add `"onCommand:glade.salesforceTargetStatus"` to `activationEvents`.
4. Add these command entries:

```json
{
  "command": "glade.openHome",
  "title": "Glade: Open Home",
  "icon": "$(home)"
},
{
  "command": "glade.schemaImportDescribe",
  "title": "Glade: Import Describe Schema"
},
{
  "command": "glade.salesforceTargetStatus",
  "title": "Glade: Check Salesforce Target"
}
```

- [ ] **Step 4: Create the controller**

Create `contrib/vscode-glade/src/hub/controller.ts`:

```ts
import * as vscode from "vscode";
import { parseHubMessage } from "./actions";
import { renderHubHtml } from "./html";
import { HubSnapshot } from "./model";

export interface GladeHomeControllerOptions {
  snapshot: () => HubSnapshot;
  executeCommand: (command: string) => Thenable<unknown>;
}

export class GladeHomeController implements vscode.Disposable {
  private panel?: vscode.WebviewPanel;
  private readonly disposables: vscode.Disposable[] = [];

  constructor(
    private readonly context: vscode.ExtensionContext,
    private readonly options: GladeHomeControllerOptions,
  ) {}

  open(): void {
    if (this.panel) {
      this.panel.reveal(vscode.ViewColumn.One);
      this.render();
      return;
    }
    const panel = vscode.window.createWebviewPanel(
      "glade.home",
      "Glade Home",
      vscode.ViewColumn.One,
      {
        enableScripts: true,
        retainContextWhenHidden: true,
      },
    );
    this.panel = panel;
    this.disposables.push(
      panel.onDidDispose(() => {
        this.panel = undefined;
      }),
      panel.webview.onDidReceiveMessage((message) => this.handleMessage(message)),
    );
    this.render();
  }

  update(): void {
    if (this.panel) {
      this.render();
    }
  }

  dispose(): void {
    for (const disposable of this.disposables.splice(0)) {
      disposable.dispose();
    }
    this.panel?.dispose();
    this.panel = undefined;
  }

  private render(): void {
    const panel = this.panel;
    if (!panel) {
      return;
    }
    panel.webview.html = renderHubHtml(this.options.snapshot(), {
      cspSource: panel.webview.cspSource,
      nonce: nonce(),
      initialTab: "home",
    });
  }

  private async handleMessage(message: unknown): Promise<void> {
    const parsed = parseHubMessage(message);
    if (parsed.type === "ready") {
      this.update();
      return;
    }
    await this.options.executeCommand(parsed.command);
    this.update();
  }
}

function nonce(): string {
  const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
  let value = "";
  for (let index = 0; index < 24; index += 1) {
    value += chars[Math.floor(Math.random() * chars.length)];
  }
  return value;
}
```

- [ ] **Step 5: Wire the controller in extension activation**

Modify `contrib/vscode-glade/src/extension.ts`:

1. Add imports:

```ts
import { GladeHomeController } from "./hub/controller";
import { HubSnapshot } from "./hub/model";
```

2. After `const workbench = ...`, add:

```ts
  function hubSnapshot(): HubSnapshot {
    const runtime = startHereState.snapshot();
    const config = vscode.workspace.getConfiguration("glade");
    const project = currentProject;
    return {
      project,
      activeEnvironment: project ? configuredActiveEnvironment(project) : undefined,
      localOrgSummary: runtime.localOrgSummary,
      missingDb: runtime.missingDb,
      watchRunning: runtime.watchRunning,
      lastRun: runtime.lastRun,
      changedSince: config.get<string>("changedSince") || "origin/main",
      pluginActionCount: runtime.pluginActionCount,
      pluginFindingCount: plugins.latestFindingCount() || undefined,
      salesforceTarget: undefined,
    };
  }

  const home = new GladeHomeController(context, {
    snapshot: hubSnapshot,
    executeCommand: (command) => Promise.resolve(vscode.commands.executeCommand(command)),
  });
  context.subscriptions.push(home);
```

3. In `syncPluginViews()`, add `home.update();` after `pluginsView.setState(...)`.

4. In `refreshProject()`, add `home.update();` at the end of the success path and the catch path.

5. In the command list, add:

```ts
    vscode.commands.registerCommand("glade.openHome", () => home.open()),
```

6. In `glade.refresh`, after `await refreshPlugins();`, add `home.update();`.

7. In `glade.statusQuickPick`, add this first item:

```ts
          { label: "Open Glade Home", command: "glade.openHome" },
```

- [ ] **Step 6: Run package test and compile**

Run:

```bash
cd contrib/vscode-glade
npm run compile && node test/package.test.js
```

Expected: command exits `0`.

- [ ] **Step 7: Commit**

```bash
git add contrib/vscode-glade/src/hub/controller.ts contrib/vscode-glade/src/extension.ts contrib/vscode-glade/package.json contrib/vscode-glade/test/package.test.js
git commit -m "feat(editor): open developer hub webview"
```

## Task 5: Start Here Entry Point

**Files:**
- Modify: `contrib/vscode-glade/src/startHereModel.ts`
- Modify: `contrib/vscode-glade/test/startHereModel.test.js`

- [ ] **Step 1: Update the failing Start Here test**

Modify `contrib/vscode-glade/test/startHereModel.test.js`. Change the expected row ids to:

```js
assert.deepStrictEqual(rows.map((row) => row.id), [
  "home",
  "ready",
  "project",
  "plugin-actions",
  "environment",
  "local-proof",
  "last-run",
  "watch",
]);
```

Add:

```js
assert.strictEqual(rows[0].label, "Open Glade Home");
assert.strictEqual(rows[0].command, "glade.openHome");
```

- [ ] **Step 2: Run the Start Here test and see it fail**

Run:

```bash
cd contrib/vscode-glade
npm run compile && node test/startHereModel.test.js
```

Expected: row id assertion fails because `home` is absent.

- [ ] **Step 3: Add the Home row**

Modify `contrib/vscode-glade/src/startHereModel.ts`. In `buildStartHereRows()` after `const lastRun = snapshot.lastRun;`, add this row at the start of the returned array:

```ts
    {
      id: "home",
      label: "Open Glade Home",
      description: "daily developer hub",
      tooltip: "Open the task-first Glade Home webview.",
      command: "glade.openHome",
      contextValue: "gladeStartHereAction",
    },
```

- [ ] **Step 4: Give the row an icon**

Modify `iconFor()` in `contrib/vscode-glade/src/views/startHereView.ts`:

```ts
    case "home":
      return new vscode.ThemeIcon("home");
```

- [ ] **Step 5: Run the Start Here test**

Run:

```bash
cd contrib/vscode-glade
npm run compile && node test/startHereModel.test.js
```

Expected: command exits `0`.

- [ ] **Step 6: Commit**

```bash
git add contrib/vscode-glade/src/startHereModel.ts contrib/vscode-glade/src/views/startHereView.ts contrib/vscode-glade/test/startHereModel.test.js
git commit -m "feat(editor): add home entry point"
```

## Task 6: Salesforce Boundary Commands

**Files:**
- Modify: `contrib/vscode-glade/src/localOrg.ts`
- Modify: `contrib/vscode-glade/src/extension.ts`
- Test: `contrib/vscode-glade/test/localOrg.test.js`

- [ ] **Step 1: Write failing terminal argument tests**

Modify `contrib/vscode-glade/test/localOrg.test.js` and add:

```js
assert.deepStrictEqual(
  localOrg.schemaImportDescribeArgs({ projectRoot: "/repo" }, "/repo/reports/org-describe.json"),
  ["schema", "import", "describe", "--input", "/repo/reports/org-describe.json", "--project-cache", "/repo"],
);

assert.deepStrictEqual(
  localOrg.salesforceTargetStatusArgs(),
  ["org", "display", "--json"],
);
```

- [ ] **Step 2: Run the localOrg test and see it fail**

Run:

```bash
cd contrib/vscode-glade
npm run compile && node test/localOrg.test.js
```

Expected: `schemaImportDescribeArgs is not a function`.

- [ ] **Step 3: Add terminal helpers**

Modify `contrib/vscode-glade/src/localOrg.ts`:

```ts
export function schemaImportDescribeArgs(project: GladeProjectContext, input: string): string[] {
  return ["schema", "import", "describe", "--input", input, "--project-cache", project.projectRoot];
}

export function salesforceTargetStatusArgs(): string[] {
  return ["org", "display", "--json"];
}

export function sendGladeTerminal(command: string): void {
  const terminal = vscode.window.createTerminal("Glade");
  terminal.show();
  terminal.sendText(command);
}
```

- [ ] **Step 4: Register schema import and target status commands**

Modify `contrib/vscode-glade/src/extension.ts` imports from `./localOrg` to include:

```ts
  salesforceTargetStatusArgs,
  schemaImportDescribeArgs,
  sendGladeTerminal,
```

Add command registrations:

```ts
    vscode.commands.registerCommand("glade.schemaImportDescribe", async () => {
      const project = await projectOrWarn();
      if (!project) {
        return;
      }
      const picked = await vscode.window.showOpenDialog({
        title: "Import Salesforce Describe JSON",
        filters: { JSON: ["json"] },
        canSelectMany: false,
      });
      const input = picked?.[0]?.fsPath;
      if (!input) {
        return;
      }
      sendGladeTerminal(terminalCommand(["glade", ...schemaImportDescribeArgs(project, input)]));
    }),
    vscode.commands.registerCommand("glade.salesforceTargetStatus", () => {
      sendGladeTerminal(terminalCommand(["sf", ...salesforceTargetStatusArgs()]));
    }),
```

Place them near the other local data command registrations.

- [ ] **Step 5: Run localOrg and package tests**

Run:

```bash
cd contrib/vscode-glade
npm run compile && node test/localOrg.test.js && node test/package.test.js
```

Expected: command exits `0`.

- [ ] **Step 6: Commit**

```bash
git add contrib/vscode-glade/src/localOrg.ts contrib/vscode-glade/src/extension.ts contrib/vscode-glade/test/localOrg.test.js
git commit -m "feat(editor): add hub Salesforce handoff commands"
```

## Task 7: README And Prototype Alignment

**Files:**
- Modify: `contrib/vscode-glade/README.md`
- Modify: `contrib/vscode-glade/prototypes/README.md`
- Modify: `contrib/vscode-glade/prototypes/local-org-dashboard.html`

- [ ] **Step 1: Update README copy**

Modify `contrib/vscode-glade/README.md`. In the Sidebar section, add this paragraph before the bullet list:

```md
Use **Glade: Open Home** for the daily hub. Home is task-first: run, data,
debug, Salesforce, and ship actions sit on the first tab. State is the second
tab: project root, active Glade org, active data environment, Salesforce target,
tests, watch state, and plugin findings.
```

Then change the Start Here bullet to:

```md
- Start Here: SFDX root, active local data environment, local DB state, watch
  state, last run state, plugin action count, and a shortcut into Glade Home.
```

- [ ] **Step 2: Rename prototype heading**

Modify `contrib/vscode-glade/prototypes/local-org-dashboard.html`:

1. Change `<title>Glade Local Org Dashboard Prototype</title>` to `<title>Glade Home Prototype</title>`.
2. Change the top heading text from `Glade local org` to `Glade Home`.
3. Add this markup inside `.top-actions`, before the first badge:

```html
<div class="segmented" aria-label="Prototype tabs">
  <button type="button" class="active" data-prototype-tab="home">Home</button>
  <button type="button" data-prototype-tab="state">State</button>
</div>
```

4. Add this CSS beside the existing `.segmented` rules:

```css
.top-actions .segmented {
  margin-right: 4px;
}
```

5. Add this script beside the existing `data-mode` click handler:

```js
document.querySelectorAll("[data-prototype-tab]").forEach((button) => {
  button.addEventListener("click", () => {
    document.querySelectorAll("[data-prototype-tab]").forEach((item) => item.classList.remove("active"));
    button.classList.add("active");
    appendLog(`Prototype tab: ${button.dataset.prototypeTab}`);
  });
});
```

- [ ] **Step 3: Update prototype README**

Modify `contrib/vscode-glade/prototypes/README.md` so the first paragraph says:

```md
This folder holds a standalone UI prototype for the Glade Home VS Code hub.
Open `local-org-dashboard.html` in a browser.
```

Add:

```md
The real extension should treat Home as the default task surface and State as
the read-only board for project, org, data, test, Salesforce, and plugin state.
```

- [ ] **Step 4: Run doc-adjacent checks**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: command exits `0`.

- [ ] **Step 5: Commit**

```bash
git add contrib/vscode-glade/README.md contrib/vscode-glade/prototypes/README.md contrib/vscode-glade/prototypes/local-org-dashboard.html
git commit -m "docs(editor): document developer hub"
```

## Task 8: Full Extension Verification

**Files:**
- No source changes unless a check fails.

- [ ] **Step 1: Run the full VS Code extension test gate**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: `npm run compile` succeeds and every listed Node test exits `0`, including `hubModel.test.js`, `hubActions.test.js`, and `hubHtml.test.js`.

- [ ] **Step 2: Package the extension**

Run:

```bash
cd contrib/vscode-glade
npm run package
```

Expected: `dist/vscode-glade-0.0.1.vsix` is written.

- [ ] **Step 3: Manual VS Code smoke**

Run:

```bash
code --extensionDevelopmentPath="$(pwd)" /path/to/an/sfdx/project
```

In the Extension Development Host:

1. Run command `Glade: Open Home`.
2. Confirm the editor tab title is `Glade Home`.
3. Confirm Home is the active tab.
4. Click State and confirm Project, Glade org, Data environment, Salesforce target, Tests, and Plugins sections render.
5. Click `Run proof` and confirm it invokes the existing local proof flow.
6. Click `SOQL scratch` and confirm an untitled SOQL editor opens.
7. Click `Apex scratch` and confirm an untitled Apex editor opens.
8. Click `Import schema`, cancel the file picker, and confirm no command runs.
9. Click `Check target` and confirm a `Glade` terminal opens with `sf org display --json`.

- [ ] **Step 4: Check git diff**

Run:

```bash
git diff --check
git status --short
```

Expected: `git diff --check` exits `0`. `git status --short` only lists planned files before the final commit.

- [ ] **Step 5: Commit final fixes if any were needed**

If Step 1, 2, 3, or 4 required fixes:

```bash
git add contrib/vscode-glade docs/superpowers/plans/2026-06-18-vscode-developer-hub.md
git commit -m "fix(editor): finish developer hub verification"
```

If no fixes were needed, do not create an empty commit.

## Self-Review

- Spec coverage: The plan builds a single developer hub, makes Home task-first, adds a separate State tab, keeps daily local work in Glade, and keeps live fixture capture as a plugin action.
- Red-flag scan: Clean.
- Type consistency: `HubSnapshot`, `HubTaskGroup`, `HubStateSection`, `HubAction`, and `HubClientMessage` names match across model, actions, html, and controller tasks.
- Product boundary: LWC and Visualforce preview commands stay out of the hub. Live org capture stays under `glade.runPluginAction`.
