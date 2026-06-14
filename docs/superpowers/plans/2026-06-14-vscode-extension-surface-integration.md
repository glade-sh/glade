# VS Code Extension Surface Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Update the VS Code extension so it exposes Glade's current local workflow: plugin actions, LWC preview, Visualforce preview, toolchain readiness, debug-log analysis, local data, and route-level browser launch.

**Architecture:** Keep Glade as the host for product commands and plugins. The VS Code extension is a thin controller: it calls `glade` commands, reads structured JSON readiness and plugin metadata, owns VS Code views, and maps structured findings to Problems. Core local preview features use `glade dev lwc` and `glade dev vf`; first-party tools stay in plugins and surface through installed plugin metadata.

**Tech Stack:** Go CLI under `internal/gladecli`; plugin host under `internal/pluginhost`; VS Code extension TypeScript under `contrib/vscode-glade`; first-party plugin manifests and adapters under sibling `../glade-tools`; Node tests for extension pure models; Go tests for CLI and plugin host; VSIX package smoke.

---

## Review Findings From Current Main

These are the facts this plan responds to.

1. Recent main includes LWC and Visualforce work:
   - `f60ee7b1 Add local LWC shell support`
   - `c483e5d4 Complete Visualforce rendering support`
   - `400feb62 Harden LWC shell and Visualforce docs`
   - `1e4120d9 Fix LWC shell fixture deploy metadata`
2. `glade dev lwc --help` is live and lists preview routes:
   - `/lwc/preview/component/<namespace>/<component>`
   - `/lwc/preview/record/<Object>/<recordId>?page=<FlexiPage>`
   - `/lwc/preview/app/<Page>`
   - `/lwc/preview/home/<Page>`
   - `/lwc/preview/tab/<Tab>`
3. `glade dev vf --help` is live and supports `--ready-file`.
4. `glade toolchain status` exists and controls whether local LWC and Lightning Out preview is ready.
5. The VS Code extension still has only:
   - Start Here
   - Local Runs
   - Data Environments
   - Local Org
   - Debug
6. The extension has no Local Preview view, no LWC route picker, no Visualforce route picker, no toolchain status row, no `glade dev lwc/vf` process manager, and no plugin action integration.
7. `glade help dev` does not list the newly landed `vf` and `lwc` subcommands even though `glade dev vf --help` and `glade dev lwc --help` work. Do not make the extension scrape help output. Still fix the help text in the same implementation train.
8. `../glade-tools/plugins/compat/plugin.json` already declares `compat lwc capture`, `visualforce`, `post-parity`, and related roots. It has no editor metadata.
9. `../glade-tools/plugins/performance/plugin.json` is now a trace-aware performance analyzer. It has no editor metadata.

## Design Rules

1. The extension must not run plugin binaries directly.
2. The extension must not scrape human output when a JSON contract can be added.
3. The extension should use `--ready-file` for `glade dev lwc` and `glade dev vf`.
4. The extension should keep long-running local preview processes visible and stoppable.
5. LWC and Visualforce belong in a new **Local Preview** view, not in Debug.
6. Plugin actions should appear where they help, but plugin management stays compact.
7. First-party plugins can add heavy workflows. Base Glade remains product runtime.
8. No user-facing text should mention editor-neighbor detection or claim Salesforce extension status.

## Parallel Squad Map

Use one coordinator branch in `/Users/matt/Dev/glade`. Dispatch subagents with disjoint write scopes.

Wave 1 can run in parallel:

- **Agent A: Core CLI JSON Contracts**
  - Owns `internal/gladecli/toolchain_command.go`, `internal/gladecli/dev_command.go`, `internal/gladecli/plugins_command.go`, `internal/pluginhost/model.go`, and Go tests.
- **Agent B: Preview Models**
  - Owns `contrib/vscode-glade/src/preview/model.ts`, `contrib/vscode-glade/src/preview/cli.ts`, and preview tests.
- **Agent C: Plugin Models**
  - Owns `contrib/vscode-glade/src/plugins/model.ts`, `actions.ts`, `findings.ts`, `cli.ts`, and plugin tests.
- **Agent D: First-Party Plugin Metadata**
  - Owns `../glade-tools/plugins/compat/plugin.json`, `../glade-tools/plugins/performance/plugin.json`, and glade-tools findings adapters.

Wave 2 starts after Wave 1 compiles:

- **Agent E: Preview Controller and View**
  - Owns `contrib/vscode-glade/src/preview/controller.ts`, `contrib/vscode-glade/src/views/previewView.ts`, and package contributions.
- **Agent F: Plugin Controller and View**
  - Owns `contrib/vscode-glade/src/plugins/controller.ts`, `contrib/vscode-glade/src/views/pluginsView.ts`, diagnostics, and package contributions.
- **Agent G: Existing View Integration**
  - Owns `Start Here`, `Debug`, `Local Runs`, status bar, and quick-pick integration.

Wave 3 is coordinator work:

- Resolve conflicts.
- Run main repo tests.
- Run glade-tools tests when touched.
- Package and install VSIX.
- Smoke the extension against `testdata/local-tests/lwc-shell` and `testdata/local-tests/visualforce-pages`.

---

## File Structure

### Core Go

- Modify `internal/gladecli/toolchain_command.go`
  - Add `glade toolchain status --json`.
- Modify `internal/gladecli/dev_command.go`
  - Update `glade help dev` to include `vf` and `lwc`.
- Modify `internal/pluginhost/model.go`
  - Add optional `editor.actions` metadata to plugin manifests.
- Modify `internal/gladecli/plugins_command.go`
  - Add `plugins list --json` and `plugins doctor --json`.
- Modify `internal/gladecli/cli_test.go`
  - Add JSON and help coverage.

### VS Code Extension

- Create `contrib/vscode-glade/src/preview/model.ts`
  - Pure parsing for LWC/VF ready files, route labels, server status, and toolchain status.
- Create `contrib/vscode-glade/src/preview/cli.ts`
  - Args for `toolchain status --json`, `toolchain install`, `dev lwc`, and `dev vf`.
- Create `contrib/vscode-glade/src/preview/controller.ts`
  - Long-running process management, ready-file waiting, route opening, and stop actions.
- Create `contrib/vscode-glade/src/views/previewView.ts`
  - Local Preview tree view for toolchain, LWC server, VF server, and routes.
- Create `contrib/vscode-glade/src/plugins/*`
  - Plugin metadata, action filtering, findings parsing, CLI adapter, controller, diagnostics.
- Create `contrib/vscode-glade/src/views/pluginsView.ts`
  - Compact installed plugin and plugin action management view.
- Modify existing views:
  - `startHereView.ts`
  - `runsView.ts`
  - `localOrgView.ts`
  - `debugView.ts`
  - `status.ts`
  - `statusModel.ts`
  - `extension.ts`
  - `package.json`

### Sibling glade-tools

- Modify `../glade-tools/plugins/compat/plugin.json`
  - Add editor actions for local support, Visualforce capture, LWC capture, and parity summaries.
- Modify `../glade-tools/plugins/performance/plugin.json`
  - Add editor action for performance scan.
- Add findings adapters in glade-tools only for commands that will be called from the editor.

---

## JSON Contracts

### Toolchain Status

Add `glade toolchain status --json`:

```json
{
  "ok": true,
  "path": "/Users/matt/.local/share/glade/lwc",
  "detail": "ready"
}
```

When unavailable:

```json
{
  "ok": false,
  "path": "",
  "detail": "toolchain not ready"
}
```

### LWC Ready File

Already written by `glade dev lwc --ready-file`:

```json
{
  "url": "http://127.0.0.1:39410",
  "addr": "127.0.0.1:39410",
  "routes": [
    "/lwc/preview/component/c/contextProbe",
    "/lwc/preview/record/Account/<recordId>?page=Account_Record_Page",
    "/lwc/preview/app/Sales_Dashboard",
    "/lwc/preview/home/Custom_Home",
    "/lwc/preview/tab/Lwc_Probe",
    "/lwc/preview/tab/Visualforce_Tab -> /apex/WidgetHost"
  ]
}
```

### Visualforce Ready File

Already written by `glade dev vf --ready-file`:

```json
{
  "url": "http://127.0.0.1:48321",
  "addr": "127.0.0.1:48321",
  "pages": [
    "/apex/CardHost",
    "/apex/Core"
  ]
}
```

### Plugin Editor Metadata

Add this optional `plugin.json` block:

```json
{
  "editor": {
    "actions": [
      {
        "id": "compat.postParity",
        "title": "Scan Unsupported Local Surfaces",
        "description": "Scan the current project for surfaces Glade cannot run locally yet.",
        "view": "runs",
        "contexts": ["project"],
        "command": ["post-parity"],
        "args": ["--project", "${projectRoot}", "--json", "--editor-findings"],
        "output": "glade.findings.v1",
        "icon": "search"
      }
    ]
  }
}
```

Allowed `view` values:

- `startHere`
- `runs`
- `localOrg`
- `debug`
- `preview`
- `plugins`

Allowed `contexts` values:

- `project`
- `activeApexFile`
- `activeDebugLog`
- `activeDataEnvironment`
- `lwcServerRunning`
- `vfServerRunning`
- `lastLocalRun`

Allowed `output` values:

- `glade.findings.v1`
- `glade.markdownReport.v1`
- `glade.rawText.v1`

Allowed input prompts:

```json
{
  "inputs": [
    {
      "name": "targetOrg",
      "label": "Target org alias",
      "type": "text",
      "required": true
    },
    {
      "name": "pages",
      "label": "Visualforce pages",
      "type": "text",
      "required": false
    }
  ]
}
```

Allowed argument tokens:

- `${projectRoot}`
- `${workspaceFolder}`
- `${activeFile}`
- `${activeDb}`
- `${outputDir}`
- `${input.targetOrg}`
- `${input.pages}`

Validation rule: the first segment of an editor action `command` must match one of the plugin command roots declared in the same manifest.

### Findings Output

Plugins that declare `output: "glade.findings.v1"` print:

```json
{
  "kind": "glade.findings.v1",
  "summary": "2 findings",
  "findings": [
    {
      "severity": "warning",
      "message": "Unsupported Visualforce component",
      "file": "force-app/main/default/pages/Core.page",
      "line": 12,
      "column": 5,
      "ruleId": "vf.component.unsupported",
      "source": "compat"
    }
  ],
  "artifacts": [
    {
      "label": "Markdown report",
      "path": ".glade/reports/compat.md"
    }
  ]
}
```

---

### Task 1: Core JSON Contracts And Help Fix

**Owner:** Agent A

**Files:**
- Modify: `internal/gladecli/toolchain_command.go`
- Modify: `internal/gladecli/dev_command.go`
- Modify: `internal/gladecli/cli_test.go`

- [ ] **Step 1: Add failing tests**

Add to `internal/gladecli/cli_test.go`:

```go
func TestToolchainStatusJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"toolchain", "status", "--json"}, &stdout, &stderr)
	if code != 0 && !strings.Contains(stdout.String(), `"ok":false`) {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{`"ok":`, `"detail":`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("toolchain status JSON missing %s:\n%s", want, stdout.String())
		}
	}
}

func TestHelpDevListsVFAndLWC(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"help", "dev"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{
		"glade dev vf [--project <root>] [--port <port>|--addr <host:port>] [--ready-file <path>]",
		"glade dev lwc [--project <root>] [--port <port>|--addr <host:port>] [--ready-file <path>]",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help dev missing %q:\n%s", want, stdout.String())
		}
	}
}
```

- [ ] **Step 2: Run the failing tests**

Run:

```bash
go test ./internal/gladecli -run 'TestToolchainStatusJSON|TestHelpDevListsVFAndLWC' -count=1
```

Expected: FAIL because `toolchain status --json` is not accepted and `help dev` omits `vf` and `lwc`.

- [ ] **Step 3: Add toolchain JSON output**

In `internal/gladecli/toolchain_command.go`, parse `--json` for `status` and emit:

```go
type toolchainStatusJSON struct {
	OK     bool   `json:"ok"`
	Path   string `json:"path,omitempty"`
	Detail string `json:"detail"`
}
```

Use `json.NewEncoder(w).Encode(toolchainStatusJSON{OK: toolchainOK, Path: toolchainPath, Detail: toolchainDetail})`.

- [ ] **Step 4: Update dev help**

In `internal/gladecli/dev_command.go`, include `vf` and `lwc` usage lines in `printDevHelp`.

- [ ] **Step 5: Verify**

Run:

```bash
go test ./internal/gladecli -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/gladecli/toolchain_command.go internal/gladecli/dev_command.go internal/gladecli/cli_test.go
git commit -m "feat: expose local preview cli metadata"
```

---

### Task 2: Preview Pure Models

**Owner:** Agent B

**Files:**
- Create: `contrib/vscode-glade/src/preview/model.ts`
- Create: `contrib/vscode-glade/src/preview/cli.ts`
- Create: `contrib/vscode-glade/test/previewModel.test.js`
- Modify: `contrib/vscode-glade/package.json`

- [ ] **Step 1: Add failing preview tests**

Create `contrib/vscode-glade/test/previewModel.test.js`:

```js
const assert = require("assert");
const model = require("../out/preview/model");
const cli = require("../out/preview/cli");

const lwc = model.parseLWCReadyFile(JSON.stringify({
  url: "http://127.0.0.1:39410",
  addr: "127.0.0.1:39410",
  routes: [
    "/lwc/preview/component/c/contextProbe",
    "/lwc/preview/tab/Visualforce_Tab -> /apex/WidgetHost"
  ]
}));
assert.strictEqual(lwc.kind, "lwc");
assert.strictEqual(lwc.routes[0].label, "c/contextProbe");
assert.strictEqual(lwc.routes[1].path, "/apex/WidgetHost");
assert.strictEqual(lwc.routes[1].sourcePath, "/lwc/preview/tab/Visualforce_Tab");

const vf = model.parseVFReadyFile(JSON.stringify({
  url: "http://127.0.0.1:48321",
  addr: "127.0.0.1:48321",
  pages: ["/apex/Core"]
}));
assert.strictEqual(vf.kind, "visualforce");
assert.strictEqual(vf.routes[0].label, "Core");

assert.deepStrictEqual(
  cli.devLWCArgs("/repo", "127.0.0.1:0", "/tmp/lwc-ready.json"),
  ["dev", "lwc", "--project", "/repo", "--addr", "127.0.0.1:0", "--ready-file", "/tmp/lwc-ready.json"],
);
assert.deepStrictEqual(
  cli.devVFArgs("/repo", "127.0.0.1:0", "/tmp/vf-ready.json"),
  ["dev", "vf", "--project", "/repo", "--addr", "127.0.0.1:0", "--ready-file", "/tmp/vf-ready.json"],
);
assert.deepStrictEqual(cli.toolchainStatusArgs(), ["toolchain", "status", "--json"]);
```

Add `node test/previewModel.test.js` to the npm test script.

- [ ] **Step 2: Run the failing test**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: FAIL because `preview/model` and `preview/cli` do not exist.

- [ ] **Step 3: Create preview model**

Create `contrib/vscode-glade/src/preview/model.ts`:

```ts
export type PreviewKind = "lwc" | "visualforce";

export interface PreviewRoute {
  label: string;
  path: string;
  sourcePath?: string;
}

export interface PreviewServerState {
  kind: PreviewKind;
  url: string;
  addr: string;
  routes: PreviewRoute[];
  running: boolean;
}

export interface ToolchainStatus {
  ok: boolean;
  path?: string;
  detail: string;
}

export function parseLWCReadyFile(raw: string): PreviewServerState {
  const parsed = JSON.parse(raw) as { url: string; addr: string; routes?: string[] };
  return {
    kind: "lwc",
    url: parsed.url,
    addr: parsed.addr,
    running: true,
    routes: (parsed.routes || []).map(lwcRoute),
  };
}

export function parseVFReadyFile(raw: string): PreviewServerState {
  const parsed = JSON.parse(raw) as { url: string; addr: string; pages?: string[] };
  return {
    kind: "visualforce",
    url: parsed.url,
    addr: parsed.addr,
    running: true,
    routes: (parsed.pages || []).map((page) => ({
      label: page.replace(/^\/apex\//, ""),
      path: page,
    })),
  };
}

function lwcRoute(route: string): PreviewRoute {
  const redirect = route.split(" -> ");
  const visible = redirect[1] || redirect[0];
  return {
    label: labelForRoute(visible),
    path: visible,
    sourcePath: redirect[1] ? redirect[0] : undefined,
  };
}

function labelForRoute(route: string): string {
  if (route.startsWith("/lwc/preview/component/")) {
    return route.replace("/lwc/preview/component/", "");
  }
  if (route.startsWith("/apex/")) {
    return route.replace("/apex/", "");
  }
  const parts = route.split(/[/?]/).filter(Boolean);
  return parts[parts.length - 1] || route;
}

export function parseToolchainStatus(raw: string): ToolchainStatus {
  const parsed = JSON.parse(raw) as ToolchainStatus;
  return { ok: Boolean(parsed.ok), path: parsed.path, detail: parsed.detail || "unknown" };
}
```

- [ ] **Step 4: Create preview CLI helpers**

Create `contrib/vscode-glade/src/preview/cli.ts`:

```ts
export function toolchainStatusArgs(): string[] {
  return ["toolchain", "status", "--json"];
}

export function toolchainInstallArgs(): string[] {
  return ["toolchain", "install"];
}

export function devLWCArgs(projectRoot: string, addr: string, readyFile: string): string[] {
  return ["dev", "lwc", "--project", projectRoot, "--addr", addr, "--ready-file", readyFile];
}

export function devVFArgs(projectRoot: string, addr: string, readyFile: string): string[] {
  return ["dev", "vf", "--project", projectRoot, "--addr", addr, "--ready-file", readyFile];
}
```

- [ ] **Step 5: Verify**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add contrib/vscode-glade/src/preview/model.ts contrib/vscode-glade/src/preview/cli.ts contrib/vscode-glade/test/previewModel.test.js contrib/vscode-glade/package.json
git commit -m "feat: model vscode local preview routes"
```

---

### Task 3: Preview Controller And Local Preview View

**Owner:** Agent E

**Files:**
- Create: `contrib/vscode-glade/src/preview/controller.ts`
- Create: `contrib/vscode-glade/src/views/previewView.ts`
- Modify: `contrib/vscode-glade/src/extension.ts`
- Modify: `contrib/vscode-glade/package.json`
- Modify: `contrib/vscode-glade/test/package.test.js`

- [ ] **Step 1: Add package tests**

Extend `contrib/vscode-glade/test/package.test.js`:

```js
assert(
  manifest.contributes.views.glade.some((view) => view.id === "glade.preview" && view.name === "Local Preview"),
  "glade.preview view must exist",
);

for (const command of [
  "glade.refreshPreview",
  "glade.startLWCPreview",
  "glade.stopLWCPreview",
  "glade.startVFPreview",
  "glade.stopVFPreview",
  "glade.openPreviewRoute",
  "glade.installToolchain",
]) {
  assert(
    manifest.contributes.commands.some((entry) => entry.command === command),
    `${command} must be contributed`,
  );
}
```

- [ ] **Step 2: Run failing test**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: FAIL because the preview view and commands are missing.

- [ ] **Step 3: Create controller skeleton**

Create `contrib/vscode-glade/src/preview/controller.ts`:

```ts
import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import { ChildProcessWithoutNullStreams, spawn } from "child_process";
import * as vscode from "vscode";
import { runGlade, runGladeJSON } from "../gladeCli";
import { GladeProjectContext } from "../projectModel";
import { devLWCArgs, devVFArgs, toolchainInstallArgs, toolchainStatusArgs } from "./cli";
import { parseLWCReadyFile, parseToolchainStatus, parseVFReadyFile, PreviewServerState, ToolchainStatus } from "./model";

export class PreviewController implements vscode.Disposable {
  private lwc?: ChildProcessWithoutNullStreams;
  private vf?: ChildProcessWithoutNullStreams;
  private lwcState?: PreviewServerState;
  private vfState?: PreviewServerState;
  private toolchain?: ToolchainStatus;
  private project?: GladeProjectContext;
  private readonly changed = new vscode.EventEmitter<void>();
  readonly onDidChange = this.changed.event;

  setProject(project: GladeProjectContext | undefined): void {
    this.project = project;
    this.changed.fire();
  }

  snapshot(): { toolchain?: ToolchainStatus; lwc?: PreviewServerState; visualforce?: PreviewServerState } {
    return { toolchain: this.toolchain, lwc: this.lwcState, visualforce: this.vfState };
  }

  async refreshToolchain(): Promise<void> {
    try {
      const result = await runGladeJSON<ToolchainStatus>(toolchainStatusArgs(), { cwd: this.project?.projectRoot }, "glade toolchain status");
      this.toolchain = parseToolchainStatus(JSON.stringify(result));
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      this.toolchain = { ok: false, detail: message };
    }
    this.changed.fire();
  }

  async installToolchain(): Promise<void> {
    const result = await runGlade(toolchainInstallArgs(), { cwd: this.project?.projectRoot });
    if (result.code !== 0) {
      throw new Error(result.stderr.trim() || result.stdout.trim() || `exit code ${result.code}`);
    }
    await this.refreshToolchain();
  }

  async start(kind: "lwc" | "visualforce"): Promise<void> {
    if (!this.project) {
      throw new Error("local preview requires an SFDX project");
    }
    const readyFile = path.join(os.tmpdir(), `glade-${kind}-${Date.now()}.json`);
    const args = kind === "lwc"
      ? devLWCArgs(this.project.projectRoot, "127.0.0.1:0", readyFile)
      : devVFArgs(this.project.projectRoot, "127.0.0.1:0", readyFile);
    const child = spawn("glade", args, { cwd: this.project.projectRoot, env: process.env });
    if (kind === "lwc") {
      this.lwc = child;
    } else {
      this.vf = child;
    }
    child.on("close", () => this.markStopped(kind));
    await this.waitForReadyFile(kind, readyFile);
  }

  stop(kind: "lwc" | "visualforce"): void {
    const child = kind === "lwc" ? this.lwc : this.vf;
    child?.kill();
    this.markStopped(kind);
  }

  dispose(): void {
    this.lwc?.kill();
    this.vf?.kill();
    this.changed.dispose();
  }

  private async waitForReadyFile(kind: "lwc" | "visualforce", readyFile: string): Promise<void> {
    const deadline = Date.now() + 15000;
    while (Date.now() < deadline) {
      if (fs.existsSync(readyFile)) {
        const raw = fs.readFileSync(readyFile, "utf8");
        if (kind === "lwc") {
          this.lwcState = parseLWCReadyFile(raw);
        } else {
          this.vfState = parseVFReadyFile(raw);
        }
        this.changed.fire();
        return;
      }
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    throw new Error(`${kind} preview did not write ready file`);
  }

  private markStopped(kind: "lwc" | "visualforce"): void {
    if (kind === "lwc") {
      this.lwc = undefined;
      if (this.lwcState) this.lwcState = { ...this.lwcState, running: false };
    } else {
      this.vf = undefined;
      if (this.vfState) this.vfState = { ...this.vfState, running: false };
    }
    this.changed.fire();
  }
}
```

- [ ] **Step 4: Create Local Preview view**

Create `contrib/vscode-glade/src/views/previewView.ts`:

```ts
import * as vscode from "vscode";
import { PreviewController } from "../preview/controller";
import { PreviewRoute, PreviewServerState } from "../preview/model";
import { commandItem, GladeTreeItem } from "./tree";

export class PreviewView implements vscode.TreeDataProvider<GladeTreeItem> {
  private readonly changed = new vscode.EventEmitter<GladeTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData = this.changed.event;

  constructor(private readonly preview: PreviewController) {
    preview.onDidChange(() => this.refresh());
  }

  refresh(): void {
    this.changed.fire();
  }

  getTreeItem(element: GladeTreeItem): vscode.TreeItem {
    return element;
  }

  getChildren(): GladeTreeItem[] {
    const snapshot = this.preview.snapshot();
    const items: GladeTreeItem[] = [
      commandItem("Refresh Preview State", "glade.refreshPreview", "Refresh toolchain and route state.", new vscode.ThemeIcon("refresh")),
      toolchainItem(snapshot.toolchain),
      commandItem("Start LWC Shell", "glade.startLWCPreview", "Start glade dev lwc.", new vscode.ThemeIcon("browser")),
      commandItem("Start Visualforce Server", "glade.startVFPreview", "Start glade dev vf.", new vscode.ThemeIcon("preview")),
    ];
    appendServer(items, "LWC", snapshot.lwc, "glade.stopLWCPreview");
    appendServer(items, "Visualforce", snapshot.visualforce, "glade.stopVFPreview");
    return items;
  }
}

function toolchainItem(toolchain: ReturnType<PreviewController["snapshot"]>["toolchain"]): GladeTreeItem {
  if (!toolchain) {
    return commandItem("Toolchain: unknown", "glade.refreshPreview", "Check local LWC toolchain.", new vscode.ThemeIcon("question"));
  }
  if (!toolchain.ok) {
    return commandItem("Toolchain: install required", "glade.installToolchain", toolchain.detail, new vscode.ThemeIcon("warning"));
  }
  const item = new GladeTreeItem("Toolchain: ready");
  item.description = toolchain.path;
  item.tooltip = toolchain.detail;
  item.iconPath = new vscode.ThemeIcon("check");
  return item;
}

function appendServer(items: GladeTreeItem[], label: string, state: PreviewServerState | undefined, stopCommand: string): void {
  if (!state) {
    return;
  }
  const server = new GladeTreeItem(`${label}: ${state.running ? "running" : "stopped"}`);
  server.description = state.addr;
  server.tooltip = state.url;
  server.iconPath = new vscode.ThemeIcon(state.running ? "radio-tower" : "circle-slash");
  items.push(server);
  for (const route of state.routes) {
    items.push(routeItem(state.url, route));
  }
  if (state.running) {
    items.push(commandItem(`Stop ${label}`, stopCommand, `Stop ${label} local preview.`, new vscode.ThemeIcon("debug-stop")));
  }
}

function routeItem(baseURL: string, route: PreviewRoute): GladeTreeItem {
  const item = commandItem(route.label, "glade.openPreviewRoute", `${baseURL}${route.path}`, new vscode.ThemeIcon("link-external"));
  item.description = route.path;
  item.command = {
    command: "glade.openPreviewRoute",
    title: route.label,
    arguments: [`${baseURL}${route.path}`],
  };
  return item;
}
```

- [ ] **Step 5: Wire package and extension**

Add view `glade.preview` named `Local Preview`.

Add commands:

- `glade.refreshPreview`
- `glade.startLWCPreview`
- `glade.stopLWCPreview`
- `glade.startVFPreview`
- `glade.stopVFPreview`
- `glade.openPreviewRoute`
- `glade.installToolchain`

In `extension.ts`, instantiate `PreviewController`, register `PreviewView`, set project during `refreshProject`, and register commands that call controller methods and `vscode.env.openExternal(vscode.Uri.parse(url))`.

- [ ] **Step 6: Verify**

Run:

```bash
cd contrib/vscode-glade
npm test
npm run package
```

Expected: PASS and VSIX package created.

- [ ] **Step 7: Commit**

```bash
git add contrib/vscode-glade
git commit -m "feat: add local preview view to vscode"
```

---

### Task 4: Plugin JSON And Editor Action Support

**Owner:** Agents A, C, F

**Files:**
- Modify: `internal/pluginhost/model.go`
- Modify: `internal/gladecli/plugins_command.go`
- Modify: `internal/gladecli/cli_test.go`
- Create: `contrib/vscode-glade/src/plugins/model.ts`
- Create: `contrib/vscode-glade/src/plugins/actions.ts`
- Create: `contrib/vscode-glade/src/plugins/cli.ts`
- Create: `contrib/vscode-glade/src/plugins/findings.ts`
- Create tests under `contrib/vscode-glade/test/`

- [ ] **Step 1: Add Go manifest tests**

Add tests proving:

- `editor.actions` validates when command root is declared.
- `editor.actions` rejects undeclared roots such as `debug`.
- `plugins list --json` includes editor metadata.
- `plugins doctor --json` returns machine-readable status.

Use command:

```bash
go test ./internal/pluginhost ./internal/gladecli -run 'Plugin|Plugins' -count=1
```

Expected before implementation: FAIL.

- [ ] **Step 2: Add manifest schema**

Add optional structs in `internal/pluginhost/model.go`:

```go
type EditorManifest struct {
	Actions []EditorActionManifest `json:"actions,omitempty"`
}

type EditorActionManifest struct {
	ID          string                `json:"id"`
	Title       string                `json:"title"`
	Description string                `json:"description,omitempty"`
	View        string                `json:"view"`
	Contexts    []string              `json:"contexts,omitempty"`
	Command     []string              `json:"command"`
	Args        []string              `json:"args,omitempty"`
	Inputs      []EditorActionInput   `json:"inputs,omitempty"`
	Output      string                `json:"output"`
	Icon        string                `json:"icon,omitempty"`
}

type EditorActionInput struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Required bool   `json:"required,omitempty"`
	Default  string `json:"default,omitempty"`
}
```

Add `Editor *EditorManifest 'json:"editor,omitempty"'` to `Manifest`.

- [ ] **Step 3: Add `plugins list --json` and `doctor --json`**

Return installed plugin identity, version, roots, manifest path, linked state, and editor metadata loaded from each installed `plugin.json`.

- [ ] **Step 4: Add TypeScript pure tests**

Create tests for:

- action filtering by view and context.
- token expansion for `${projectRoot}`, `${activeFile}`, `${activeDb}`, `${outputDir}`, and `${input.name}`.
- findings parsing for `glade.findings.v1`.

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected before implementation: FAIL.

- [ ] **Step 5: Implement TypeScript plugin modules**

Create modules:

- `plugins/model.ts`
- `plugins/actions.ts`
- `plugins/cli.ts`
- `plugins/findings.ts`

Use the same contracts named in this plan.

- [ ] **Step 6: Verify**

Run:

```bash
go test ./internal/pluginhost ./internal/gladecli -count=1
cd contrib/vscode-glade && npm test
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/pluginhost internal/gladecli contrib/vscode-glade
git commit -m "feat: add editor-aware plugin metadata"
```

---

### Task 5: Plugin Controller, Diagnostics, And Plugins View

**Owner:** Agent F

**Files:**
- Create: `contrib/vscode-glade/src/plugins/controller.ts`
- Create: `contrib/vscode-glade/src/views/pluginsView.ts`
- Modify: `contrib/vscode-glade/src/extension.ts`
- Modify: `contrib/vscode-glade/package.json`
- Modify: `contrib/vscode-glade/test/package.test.js`

- [ ] **Step 1: Add package tests**

Assert view `glade.plugins` and commands:

- `glade.refreshPlugins`
- `glade.managePlugins`
- `glade.runPluginAction`
- `glade.linkLocalPlugin`
- `glade.installPluginArchive`

- [ ] **Step 2: Implement controller**

Controller responsibilities:

- call `glade plugins list --json`.
- keep installed plugin state.
- filter actions by view/context.
- prompt for declared inputs.
- run plugin actions through `glade`.
- parse `glade.findings.v1`.
- publish diagnostics to a `glade-plugins` diagnostic collection.
- show artifacts as clickable rows in the Plugins view.

- [ ] **Step 3: Implement Plugins view**

Rows:

- `Manage Plugins`
- `Refresh`
- `Link Local Plugin`
- `Install Plugin Archive`
- installed plugin rows with version and linked state.
- plugin-contributed actions with plugin identity in description.

- [ ] **Step 4: Wire existing views**

Add plugin actions to:

- Start Here: project-level scans.
- Local Runs: test/readiness/plugin reports.
- Local Org: data checks.
- Debug: active debug log and Visualforce/LWC helpers.
- Local Preview: LWC/VF capture and parity actions.

- [ ] **Step 5: Verify**

Run:

```bash
cd contrib/vscode-glade
npm test
npm run package
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add contrib/vscode-glade
git commit -m "feat: surface plugin actions in vscode"
```

---

### Task 6: First-Party Plugin Editor Metadata

**Owner:** Agent D

**Files:**
- Modify: `../glade-tools/plugins/compat/plugin.json`
- Modify: `../glade-tools/plugins/performance/plugin.json`
- Modify glade-tools command writers that need `--editor-findings`

- [ ] **Step 1: Add compat editor actions**

Add actions:

```json
{
  "id": "compat.postParity",
  "title": "Scan Unsupported Local Surfaces",
  "view": "runs",
  "contexts": ["project"],
  "command": ["post-parity"],
  "args": ["--project", "${projectRoot}", "--json", "--editor-findings"],
  "output": "glade.findings.v1",
  "icon": "search"
}
```

```json
{
  "id": "compat.visualforceLocalCapture",
  "title": "Capture Local Visualforce Evidence",
  "view": "preview",
  "contexts": ["project", "vfServerRunning"],
  "command": ["visualforce", "capture"],
  "args": ["--local", "--glade-bin", "glade", "--project", "${projectRoot}", "--out", "${outputDir}/visualforce-local.json", "--json", "--editor-findings"],
  "output": "glade.findings.v1",
  "icon": "record"
}
```

```json
{
  "id": "compat.lwcCapture",
  "title": "Capture LWC Org Evidence",
  "view": "preview",
  "contexts": ["project", "lwcServerRunning"],
  "command": ["compat", "lwc", "capture"],
  "inputs": [
    { "name": "targetOrg", "label": "Target org alias", "type": "text", "required": true }
  ],
  "args": ["--target-org", "${input.targetOrg}", "--project", "${projectRoot}", "--out", "${outputDir}/lwc-org-capture.json", "--json", "--editor-findings"],
  "output": "glade.findings.v1",
  "icon": "cloud-download"
}
```

- [ ] **Step 2: Add performance editor action**

Add:

```json
{
  "id": "performance.scanProject",
  "title": "Scan Performance Risks",
  "view": "startHere",
  "contexts": ["project"],
  "command": ["performance"],
  "args": ["--project", "${projectRoot}", "--json", "--editor-findings"],
  "output": "glade.findings.v1",
  "icon": "pulse"
}
```

- [ ] **Step 3: Add `--editor-findings` adapters**

For each command called by an editor action, add `--editor-findings` support that emits `glade.findings.v1`.

The adapter must not remove existing `--json` output. It is an extra mode for editor integration.

- [ ] **Step 4: Verify glade-tools**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit in glade-tools**

```bash
cd /Users/matt/Dev/glade-tools
git add plugins internal
git commit -m "feat: expose editor actions for glade plugins"
```

---

### Task 7: Start Here, Status Bar, And Command Palette Polish

**Owner:** Agent G

**Files:**
- Modify: `contrib/vscode-glade/src/startHereModel.ts`
- Modify: `contrib/vscode-glade/src/startHereState.ts`
- Modify: `contrib/vscode-glade/src/statusModel.ts`
- Modify: `contrib/vscode-glade/src/status.ts`
- Modify: `contrib/vscode-glade/src/extension.ts`
- Modify tests for Start Here and status.

- [ ] **Step 1: Add Start Here model tests**

Assert rows include, when project is loaded:

- `Toolchain: ready` or `Toolchain: install required`.
- `LWC preview: stopped` or route count.
- `Visualforce preview: stopped` or page count.
- plugin action rows are absent when no plugins are installed.

- [ ] **Step 2: Implement snapshot fields**

Add fields:

```ts
toolchainReady?: boolean;
lwcRouteCount?: number;
vfRouteCount?: number;
pluginActionCount?: number;
```

- [ ] **Step 3: Update Status Bar text**

Examples:

- `Glade: dev`
- `Glade: preview 5 routes`
- `Glade: plugin 2 findings`
- `Glade: toolchain needed`

Keep the text short. Put details in tooltip.

- [ ] **Step 4: Update command palette quick pick**

Add entries:

- `Start LWC Shell`
- `Start Visualforce Server`
- `Install LWC Toolchain`
- `Manage Plugins`

- [ ] **Step 5: Verify**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add contrib/vscode-glade
git commit -m "feat: surface preview readiness in vscode"
```

---

### Task 8: Docs And User Install Path

**Owner:** Docs agent

**Files:**
- Modify: `docs/EDITOR.md`
- Modify: `docs/LWC_LOCAL_SHELL.md`
- Modify: `contrib/vscode-glade/README.md`
- Modify: `site/docs-src/guide/editor.md`
- Modify: `site/docs-src/guide/lwc-local-shell.md`

- [ ] **Step 1: Update editor docs**

Add a Local Preview section:

````markdown
## Local Preview

The VS Code extension can start and stop local LWC and Visualforce preview
servers.

- `Start LWC Shell` runs `glade dev lwc --addr 127.0.0.1:0 --ready-file <file>`.
- `Start Visualforce Server` runs `glade dev vf --addr 127.0.0.1:0 --ready-file <file>`.
- Routes appear in the Local Preview view.
- Clicking a route opens it in the browser.
- Toolchain status uses `glade toolchain status --json`.

Use `Install LWC Toolchain` when Lightning Out or LWC routes need the runtime:

```bash
glade toolchain install
```
````

- [ ] **Step 2: Add plugin action docs**

Document:

- Extension reads installed plugins through `glade plugins list --json`.
- Extension does not run plugin binaries directly.
- Linked local plugins work before registry install.
- Findings appear in Problems when a plugin emits `glade.findings.v1`.

- [ ] **Step 3: Search for stale wording**

Run:

```bash
rg -n "Salesforce extensions?.*detect|Salesforce Extension Coexistence|extensions found" docs/EDITOR.md contrib/vscode-glade site
```

Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add docs contrib/vscode-glade/README.md site/docs-src
git commit -m "docs: describe vscode preview and plugin actions"
```

---

### Task 9: Integration Smoke

**Owner:** Coordinator

**Files:** No planned source edits.

- [ ] **Step 1: Run focused Go checks**

Run:

```bash
go test ./internal/gladecli ./internal/pluginhost ./internal/lwcshell ./internal/server ./internal/visualforce ./internal/lightningout -count=1
```

Expected: PASS.

- [ ] **Step 2: Run extension checks**

Run:

```bash
cd contrib/vscode-glade
npm test
npm run package
```

Expected: PASS and package writes `dist/vscode-glade-0.0.1.vsix`. The existing `vsce` bundle-size warning is acceptable if packaging exits 0.

- [ ] **Step 3: Run glade-tools checks if touched**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 4: Manual LWC preview smoke**

Run:

```bash
cd /Users/matt/Dev/glade
glade editor install vscode --vsix contrib/vscode-glade/dist/vscode-glade-0.0.1.vsix --force
code /Users/matt/Dev/glade/testdata/local-tests/lwc-shell
```

In VS Code:

1. Open the Glade Activity Bar.
2. Open Local Preview.
3. Click `Refresh Preview State`.
4. If the toolchain is missing, click `Install LWC Toolchain`.
5. Click `Start LWC Shell`.
6. Confirm route rows appear.
7. Open `/lwc/preview/component/c/contextProbe`.
8. Open `/lwc/preview/tab/Visualforce_Tab` and confirm it resolves to `/apex/...`.

- [ ] **Step 5: Manual Visualforce preview smoke**

Open `/Users/matt/Dev/glade/testdata/local-tests/visualforce-pages` in VS Code.

1. Open the Glade Activity Bar.
2. Click `Start Visualforce Server`.
3. Confirm `/apex/<Page>` rows appear.
4. Open one page route.
5. Stop the server.

- [ ] **Step 6: Manual plugin smoke**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go build -o /tmp/glade-plugin-compat ./cmd/glade-plugin-compat
go build -o /tmp/glade-plugin-performance ./cmd/glade-plugin-performance
cd /Users/matt/Dev/glade
glade plugins link --exec /tmp/glade-plugin-compat
glade plugins link --exec /tmp/glade-plugin-performance
glade plugins list --json
```

In VS Code:

1. Open Plugins view.
2. Confirm linked compat and performance plugins appear.
3. Confirm project-level plugin actions appear in Start Here or Local Runs.
4. Run one action that emits `glade.findings.v1`.
5. Confirm Problems panel receives diagnostics.

- [ ] **Step 7: Final checks**

Run:

```bash
git diff --check
rg -n "Salesforce extensions?.*detect|Salesforce Extension Coexistence|extensions found" docs/EDITOR.md contrib/vscode-glade site
```

Expected: no whitespace errors and no stale extension-detection wording.

---

## Stretch Goals

Do these after the main plan passes.

1. Add route search inside Local Preview.
2. Add preview server logs as collapsible rows.
3. Add `glade plugins available --json` and first-party install suggestions when the registry is live.
4. Add markdown report preview for `glade.markdownReport.v1`.
5. Add core Debug view commands for `glade debug parse/profile/explain/repro` against the active `.log` file.
6. Add route screenshots using Playwright only after local preview is stable.

## Final Review Checklist

- [ ] Extension has one new Local Preview view.
- [ ] Extension can start/stop `glade dev lwc`.
- [ ] Extension can start/stop `glade dev vf`.
- [ ] Extension opens routes from ready-file JSON.
- [ ] Extension shows toolchain state and can run install.
- [ ] Extension reads plugin state from `glade plugins list --json`.
- [ ] Extension maps plugin findings to Problems.
- [ ] glade-tools first-party plugins expose editor actions.
- [ ] Registry failure does not break extension startup.
- [ ] No Salesforce extension detection row or wording returns.
- [ ] Main repo focused Go tests pass.
- [ ] VS Code extension tests and package pass.
- [ ] glade-tools tests pass when touched.
