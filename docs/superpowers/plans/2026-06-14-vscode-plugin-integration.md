# VS Code Plugin Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let installed or linked Glade plugins contribute useful VS Code actions and findings without turning the VS Code extension into a second plugin host.

**Architecture:** Glade remains the plugin host. The VS Code extension asks `glade plugins ... --json` for installed plugin metadata, renders editor-safe plugin actions in existing Glade views, and runs actions by calling `glade <plugin-command> ...`. Plugin output is mapped through a small set of structured editor contracts such as `glade.findings.v1`.

**Tech Stack:** Go plugin host and CLI under `internal/pluginhost` and `internal/gladecli`; TypeScript VS Code extension under `contrib/vscode-glade`; first-party plugin manifests under sibling `../glade-tools/plugins`; Node tests for extension models; Go tests for CLI/plugin host.

---

## Design Rules

1. The VS Code extension must not execute plugin binaries directly.
2. The extension must not scrape human plugin output.
3. The extension must work with linked local plugins before any registry call.
4. The extension must not show "Salesforce extensions detected" or similar editor-neighbor status.
5. Plugin actions appear where they help: Start Here, Local Runs, Local Org, Debug, and a compact Plugins management view.
6. Plugin command roots stay outside core command roots. `debug` is core today, so a plugin must not own `glade debug analyze`. Core debug-log actions can be added separately; plugin-owned debug tooling should use a non-core root such as `log-analyze` or `compat`.

## Parallel Squad Map

Use one coordinator branch in `/Users/matt/Dev/glade` and separate subagents for disjoint write scopes.

Wave 1 can run in parallel:

- **Agent A: Core CLI JSON and schema**
  - Owns `internal/pluginhost/model.go`, `internal/gladecli/plugins_command.go`, `internal/gladecli/cli_test.go`.
- **Agent B: VS Code pure plugin models**
  - Owns `contrib/vscode-glade/src/plugins/model.ts`, `contrib/vscode-glade/src/plugins/actions.ts`, `contrib/vscode-glade/test/plugin*.test.js`.
- **Agent C: First-party plugin manifest metadata**
  - Owns sibling files `../glade-tools/plugins/compat/plugin.json`, `../glade-tools/plugins/performance/plugin.json`, and glade-tools manifest tests if present.

Wave 2 starts after Wave 1 contracts compile:

- **Agent D: VS Code plugin CLI and controller**
  - Owns `contrib/vscode-glade/src/plugins/cli.ts`, `contrib/vscode-glade/src/plugins/controller.ts`, extension command registration.
- **Agent E: VS Code views and UX wiring**
  - Owns `contrib/vscode-glade/src/views/pluginsView.ts`, existing view injection points, `contrib/vscode-glade/package.json`.
- **Agent F: Findings output and diagnostics**
  - Owns `contrib/vscode-glade/src/plugins/findings.ts`, diagnostics collection wiring, findings tests.

Wave 3 is coordinator work:

- Integrate conflicts.
- Run full verification.
- Package and install the VSIX.
- Clean worktrees after merge.

---

## File Structure

### Go Core

- Modify `internal/pluginhost/model.go`
  - Add optional editor metadata to `Manifest`.
  - Validate editor action IDs, views, contexts, output contracts, and command roots.
- Modify `internal/gladecli/plugins_command.go`
  - Add `--json` to `plugins list`, `plugins doctor`, `plugins which`, `plugins available`, and `plugins search`.
  - Include editor metadata in `plugins list --json`.
- Modify `internal/gladecli/cli_test.go`
  - Add tests for JSON list, doctor, and editor manifest pass-through.

### VS Code Extension

- Create `contrib/vscode-glade/src/plugins/model.ts`
  - Types for installed plugins, editor actions, action contexts, plugin rows, and findings.
- Create `contrib/vscode-glade/src/plugins/actions.ts`
  - Pure filtering and argument expansion for editor actions.
- Create `contrib/vscode-glade/src/plugins/cli.ts`
  - Calls `glade plugins list --json`, `glade plugins doctor --json`, and plugin action commands.
- Create `contrib/vscode-glade/src/plugins/findings.ts`
  - Parses `glade.findings.v1` and maps findings to VS Code diagnostics.
- Create `contrib/vscode-glade/src/plugins/controller.ts`
  - Owns plugin state, refresh, action execution, diagnostics, and output channel updates.
- Create `contrib/vscode-glade/src/views/pluginsView.ts`
  - Compact management and installed plugin view.
- Modify `contrib/vscode-glade/src/extension.ts`
  - Instantiate plugin controller.
  - Register plugin commands.
  - Refresh plugins with project state.
  - Inject contextual action rows into Start Here, Local Runs, Local Org, and Debug.
- Modify `contrib/vscode-glade/src/views/startHereView.ts`, `runsView.ts`, `localOrgView.ts`, `debugView.ts`
  - Accept plugin action rows from the controller.
- Modify `contrib/vscode-glade/package.json`
  - Add the `glade.plugins` view and commands.

### First-Party Plugins

- Modify `../glade-tools/plugins/compat/plugin.json`
  - Add editor actions for local readiness and project support scans.
- Modify `../glade-tools/plugins/performance/plugin.json`
  - Add editor action for performance scan.

---

## Contract: Editor Metadata

Add this optional manifest block:

```json
{
  "editor": {
    "actions": [
      {
        "id": "compat.localTestsAnalyze",
        "title": "Analyze Local Test Readiness",
        "description": "Inspect the current SFDX project for local test blockers.",
        "view": "startHere",
        "contexts": ["project"],
        "command": ["compat", "local-tests"],
        "args": ["--project", "${projectRoot}", "--analyze", "--json"],
        "output": "glade.findings.v1",
        "icon": "beaker"
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
- `plugins`

Allowed `contexts` values:

- `project`
- `activeApexFile`
- `activeDebugLog`
- `activeDataEnvironment`
- `lastLocalRun`

Allowed `output` values:

- `glade.findings.v1`
- `glade.markdownReport.v1`
- `glade.rawText.v1`

Allowed argument tokens:

- `${projectRoot}`
- `${workspaceFolder}`
- `${activeFile}`
- `${activeDb}`
- `${outputDir}`

Validation rule: the first segment of an editor action `command` must match one of the plugin command roots declared in the same manifest. This keeps plugin-owned actions from pretending to be core commands.

---

## Contract: Findings Output

Plugins that declare `output: "glade.findings.v1"` must print JSON like this:

```json
{
  "kind": "glade.findings.v1",
  "summary": "2 findings",
  "findings": [
    {
      "severity": "warning",
      "message": "SOQL query inside a loop",
      "file": "force-app/main/default/classes/InvoiceService.cls",
      "line": 42,
      "column": 9,
      "ruleId": "apex.soql.loop",
      "source": "compat"
    }
  ],
  "artifacts": [
    {
      "label": "Markdown report",
      "path": ".glade/reports/local-tests-readiness.md"
    }
  ]
}
```

The extension maps severities this way:

- `error` -> `vscode.DiagnosticSeverity.Error`
- `warning` -> `vscode.DiagnosticSeverity.Warning`
- `info` -> `vscode.DiagnosticSeverity.Information`
- `hint` -> `vscode.DiagnosticSeverity.Hint`

Unknown severities become `warning`.

---

### Task 1: Add Plugin Editor Metadata Schema

**Owner:** Agent A

**Files:**
- Modify: `internal/pluginhost/model.go`
- Test: `internal/pluginhost/manifest_test.go`

- [ ] **Step 1: Write failing manifest validation tests**

Add tests to `internal/pluginhost/manifest_test.go`:

```go
func TestManifestAllowsEditorActionsForDeclaredCommandRoots(t *testing.T) {
	manifest := Manifest{
		APIVersion: APIVersion,
		Name:       "compat",
		Version:    "0.1.0",
		Commands: []CommandManifest{{
			Path:    []string{"compat"},
			Summary: "Compatibility commands.",
		}},
		Editor: &EditorManifest{Actions: []EditorActionManifest{{
			ID:          "compat.localTestsAnalyze",
			Title:       "Analyze Local Test Readiness",
			Description: "Inspect the current SFDX project for local test blockers.",
			View:        "startHere",
			Contexts:    []string{"project"},
			Command:     []string{"compat", "local-tests"},
			Args:        []string{"--project", "${projectRoot}", "--analyze", "--json"},
			Output:      "glade.findings.v1",
			Icon:        "beaker",
		}}},
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestManifestRejectsEditorActionForUndeclaredCommandRoot(t *testing.T) {
	manifest := Manifest{
		APIVersion: APIVersion,
		Name:       "compat",
		Version:    "0.1.0",
		Commands: []CommandManifest{{
			Path: []string{"compat"},
		}},
		Editor: &EditorManifest{Actions: []EditorActionManifest{{
			ID:       "compat.bad",
			Title:    "Bad",
			View:     "debug",
			Contexts: []string{"project"},
			Command:  []string{"debug", "analyze"},
			Output:   "glade.findings.v1",
		}}},
	}
	err := manifest.Validate()
	if err == nil || !strings.Contains(err.Error(), "editor action compat.bad command root \"debug\" is not declared by plugin") {
		t.Fatalf("Validate() error = %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/pluginhost -run 'TestManifest(AllowsEditorActions|RejectsEditorAction)' -count=1
```

Expected: compile failure for undefined `EditorManifest` and `EditorActionManifest`.

- [ ] **Step 3: Add schema structs**

Add to `internal/pluginhost/model.go`:

```go
type Manifest struct {
	APIVersion          string            `json:"apiVersion"`
	Name                string            `json:"name"`
	Version             string            `json:"version"`
	Summary             string            `json:"summary,omitempty"`
	Commands            []CommandManifest `json:"commands"`
	Editor              *EditorManifest   `json:"editor,omitempty"`
	MinimumGladeVersion string            `json:"minimumGladeVersion,omitempty"`
	Source              string            `json:"source,omitempty"`
}

type EditorManifest struct {
	Actions []EditorActionManifest `json:"actions,omitempty"`
}

type EditorActionManifest struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	View        string   `json:"view"`
	Contexts    []string `json:"contexts,omitempty"`
	Command     []string `json:"command"`
	Args        []string `json:"args,omitempty"`
	Output      string   `json:"output"`
	Icon        string   `json:"icon,omitempty"`
}
```

- [ ] **Step 4: Add validation helpers**

Add to `internal/pluginhost/model.go`:

```go
var editorViews = map[string]struct{}{
	"startHere": {}, "runs": {}, "localOrg": {}, "debug": {}, "plugins": {},
}

var editorContexts = map[string]struct{}{
	"project": {}, "activeApexFile": {}, "activeDebugLog": {}, "activeDataEnvironment": {}, "lastLocalRun": {},
}

var editorOutputs = map[string]struct{}{
	"glade.findings.v1": {}, "glade.markdownReport.v1": {}, "glade.rawText.v1": {},
}

func (m Manifest) validateEditor() error {
	if m.Editor == nil {
		return nil
	}
	roots := map[string]struct{}{}
	for _, root := range m.CommandRoots() {
		roots[root] = struct{}{}
	}
	seen := map[string]struct{}{}
	for _, action := range m.Editor.Actions {
		if strings.TrimSpace(action.ID) == "" {
			return errors.New("editor action id is required")
		}
		if _, exists := seen[action.ID]; exists {
			return fmt.Errorf("duplicate editor action id %q", action.ID)
		}
		seen[action.ID] = struct{}{}
		if strings.TrimSpace(action.Title) == "" {
			return fmt.Errorf("editor action %s title is required", action.ID)
		}
		if _, ok := editorViews[action.View]; !ok {
			return fmt.Errorf("editor action %s has unsupported view %q", action.ID, action.View)
		}
		for _, context := range action.Contexts {
			if _, ok := editorContexts[context]; !ok {
				return fmt.Errorf("editor action %s has unsupported context %q", action.ID, context)
			}
		}
		if len(action.Command) == 0 || strings.TrimSpace(action.Command[0]) == "" {
			return fmt.Errorf("editor action %s command is required", action.ID)
		}
		if _, ok := roots[action.Command[0]]; !ok {
			return fmt.Errorf("editor action %s command root %q is not declared by plugin", action.ID, action.Command[0])
		}
		for _, part := range action.Command {
			if err := validatePluginPathToken("editor action command segment", part); err != nil {
				return err
			}
		}
		if _, ok := editorOutputs[action.Output]; !ok {
			return fmt.Errorf("editor action %s has unsupported output %q", action.ID, action.Output)
		}
	}
	return nil
}
```

Call `m.validateEditor()` at the end of `Manifest.Validate()`.

- [ ] **Step 5: Run tests**

Run:

```bash
go test ./internal/pluginhost -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/pluginhost/model.go internal/pluginhost/manifest_test.go
git commit -m "feat: add plugin editor action metadata"
```

---

### Task 2: Add JSON Output for Plugin CLI Commands

**Owner:** Agent A

**Files:**
- Modify: `internal/gladecli/plugins_command.go`
- Modify: `internal/gladecli/cli_test.go`

- [ ] **Step 1: Write failing CLI tests**

Add tests near existing plugin CLI tests in `internal/gladecli/cli_test.go`:

```go
func TestPluginsListJSONIncludesEditorActions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GLADE_HOME", home)
	manifestDir := filepath.Join(home, "plugins", "compat", "0.1.0")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(manifestDir, "plugin.json")
	manifest := `{
	  "apiVersion":"glade.plugin.v1",
	  "name":"compat",
	  "version":"0.1.0",
	  "commands":[{"path":["compat"],"summary":"Compatibility commands."}],
	  "editor":{"actions":[{
	    "id":"compat.localTestsAnalyze",
	    "title":"Analyze Local Test Readiness",
	    "view":"startHere",
	    "contexts":["project"],
	    "command":["compat","local-tests"],
	    "args":["--project","${projectRoot}","--analyze","--json"],
	    "output":"glade.findings.v1",
	    "icon":"beaker"
	  }]}
	}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(home, "plugins", "installed.json")
	if err := os.WriteFile(statePath, []byte(fmt.Sprintf(`{"version":1,"plugins":[{"name":"compat","canonicalName":"@glade/compat","version":"0.1.0","commands":["compat"],"manifest":%q,"executable":"/tmp/glade-plugin-compat"}]}`, manifestPath)), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"plugins", "list", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"identityName":"@glade/compat"`) ||
		!strings.Contains(stdout.String(), `"id":"compat.localTestsAnalyze"`) {
		t.Fatalf("plugins list --json missing editor action:\n%s", stdout.String())
	}
}

func TestPluginsDoctorJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GLADE_HOME", home)
	exe := testPluginExecutable(t, `{"apiVersion":"glade.plugin.v1","name":"compat","version":"0.1.0","commands":[{"path":["compat"]}]}`)
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"plugins", "link", "--exec", exe}, &stdout, &stderr); code != 0 {
		t.Fatalf("link code=%d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code := Run(context.Background(), []string{"plugins", "doctor", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("doctor code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"ok":true`) || !strings.Contains(stdout.String(), `"name":"compat"`) {
		t.Fatalf("plugins doctor --json output:\n%s", stdout.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/gladecli -run 'TestPlugins(ListJSONIncludesEditorActions|DoctorJSON)' -count=1
```

Expected: FAIL because `--json` is not supported.

- [ ] **Step 3: Add JSON option parsing and output structs**

Add in `internal/gladecli/plugins_command.go`:

```go
type pluginsJSONOption struct {
	json bool
}

func parsePluginsJSONOption(args []string, usage string) (pluginsJSONOption, error) {
	var opts pluginsJSONOption
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.json = true
		default:
			return opts, fmt.Errorf("%s", usage)
		}
	}
	return opts, nil
}

type pluginsListJSON struct {
	Plugins []pluginsListJSONItem `json:"plugins"`
}

type pluginsListJSONItem struct {
	Name          string                     `json:"name"`
	IdentityName  string                     `json:"identityName"`
	CanonicalName string                     `json:"canonicalName,omitempty"`
	Version       string                     `json:"version"`
	Linked        bool                       `json:"linked"`
	Commands      []string                   `json:"commands"`
	Executable    string                     `json:"executable,omitempty"`
	Manifest      string                     `json:"manifest,omitempty"`
	Source        string                     `json:"source,omitempty"`
	Editor        *pluginhost.EditorManifest `json:"editor,omitempty"`
}

type pluginsDoctorJSON struct {
	Plugins []pluginsDoctorJSONItem `json:"plugins"`
}

type pluginsDoctorJSONItem struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}
```

- [ ] **Step 4: Implement `plugins list --json`**

Change `runPluginsList` to accept args:

```go
func runPluginsList(ctx context.Context, args []string, stdout io.Writer) error {
	opts, err := parsePluginsJSONOption(args, "usage: glade plugins list [--json]")
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	plugins, err := pluginhost.NewStore(pluginhost.DefaultRoot()).ListInstalled()
	if err != nil {
		return err
	}
	if opts.json {
		return writePluginsListJSON(stdout, plugins)
	}
	if len(plugins) == 0 {
		_, err := fmt.Fprintln(stdout, "No plugins installed.")
		return err
	}
	for _, plugin := range plugins {
		link := ""
		if plugin.Linked {
			link = " linked"
		}
		fmt.Fprintf(stdout, "%s %s%s %s\n", plugin.IdentityName(), plugin.Version, link, strings.Join(plugin.Commands, ","))
	}
	return nil
}
```

Update the dispatcher:

```go
case "list":
	return runPluginsList(ctx, args[1:], stdout)
```

Add:

```go
func writePluginsListJSON(stdout io.Writer, plugins []pluginhost.InstalledPlugin) error {
	out := pluginsListJSON{}
	for _, plugin := range plugins {
		item := pluginsListJSONItem{
			Name:          plugin.Name,
			IdentityName:  plugin.IdentityName(),
			CanonicalName: plugin.CanonicalName,
			Version:       plugin.Version,
			Linked:        plugin.Linked,
			Commands:      plugin.Commands,
			Executable:    plugin.Executable,
			Manifest:      plugin.Manifest,
			Source:        plugin.Source,
		}
		if plugin.Manifest != "" {
			if manifest, err := pluginhost.LoadManifestFromFile(plugin.Manifest); err == nil {
				item.Editor = manifest.Editor
			}
		}
		out.Plugins = append(out.Plugins, item)
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
```

Add `encoding/json` to imports.

- [ ] **Step 5: Implement `plugins doctor --json`**

Change `runPluginsDoctor`:

```go
func runPluginsDoctor(ctx context.Context, args []string, stdout io.Writer) error {
	opts, err := parsePluginsJSONOption(args, "usage: glade plugins doctor [--json]")
	if err != nil {
		return err
	}
	results, err := pluginhost.NewStore(pluginhost.DefaultRoot()).Doctor(ctx)
	if err != nil {
		return err
	}
	if opts.json {
		out := pluginsDoctorJSON{}
		for _, result := range results {
			out.Plugins = append(out.Plugins, pluginsDoctorJSONItem{
				Name:    result.Plugin.IdentityName(),
				Version: result.Plugin.Version,
				OK:      result.OK,
				Message: result.Message,
			})
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	for _, result := range results {
		if result.OK {
			fmt.Fprintf(stdout, "%s %s ok\n", result.Plugin.IdentityName(), result.Plugin.Version)
			continue
		}
		fmt.Fprintf(stdout, "%s %s %s\n", result.Plugin.IdentityName(), result.Plugin.Version, result.Message)
	}
	return nil
}
```

Update dispatcher:

```go
case "doctor":
	return runPluginsDoctor(ctx, args[1:], stdout)
```

- [ ] **Step 6: Run tests**

Run:

```bash
go test ./internal/pluginhost ./internal/gladecli -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/gladecli/plugins_command.go internal/gladecli/cli_test.go
git commit -m "feat: expose plugin state as json"
```

---

### Task 3: Add VS Code Plugin Models

**Owner:** Agent B

**Files:**
- Create: `contrib/vscode-glade/src/plugins/model.ts`
- Create: `contrib/vscode-glade/src/plugins/actions.ts`
- Create: `contrib/vscode-glade/test/pluginActions.test.js`
- Modify: `contrib/vscode-glade/package.json`

- [ ] **Step 1: Write failing action model tests**

Create `contrib/vscode-glade/test/pluginActions.test.js`:

```js
const assert = require("assert");
const actions = require("../out/plugins/actions");

const plugin = {
  name: "compat",
  identityName: "@glade/compat",
  version: "0.1.0",
  linked: true,
  commands: ["compat"],
  editor: {
    actions: [
      {
        id: "compat.localTestsAnalyze",
        title: "Analyze Local Test Readiness",
        view: "startHere",
        contexts: ["project"],
        command: ["compat", "local-tests"],
        args: ["--project", "${projectRoot}", "--analyze", "--json"],
        output: "glade.findings.v1",
        icon: "beaker"
      },
      {
        id: "compat.debugLog",
        title: "Analyze Current Debug Log",
        view: "debug",
        contexts: ["activeDebugLog"],
        command: ["compat", "debug-log"],
        args: ["--log", "${activeFile}", "--project", "${projectRoot}", "--json"],
        output: "glade.findings.v1",
        icon: "search"
      }
    ]
  }
};

const state = { plugins: [plugin] };

assert.deepStrictEqual(
  actions.actionsForView(state, "startHere", new Set(["project"])).map((action) => action.id),
  ["compat.localTestsAnalyze"],
);
assert.deepStrictEqual(
  actions.actionsForView(state, "debug", new Set(["project"])).map((action) => action.id),
  [],
);
assert.deepStrictEqual(
  actions.actionsForView(state, "debug", new Set(["project", "activeDebugLog"])).map((action) => action.id),
  ["compat.debugLog"],
);
assert.deepStrictEqual(
  actions.expandActionArgs(plugin.editor.actions[0], {
    projectRoot: "/repo",
    workspaceFolder: "/repo",
    activeFile: "/repo/logs/apex.log",
    activeDb: "/repo/.glade/envs/dev.sqlite",
    outputDir: "/repo/.glade/reports",
  }),
  ["compat", "local-tests", "--project", "/repo", "--analyze", "--json"],
);
```

Add the test to the `test` script in `contrib/vscode-glade/package.json` after `environmentActions.test.js`:

```json
"node test/pluginActions.test.js"
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: FAIL because `../out/plugins/actions` does not exist.

- [ ] **Step 3: Create `model.ts`**

Create `contrib/vscode-glade/src/plugins/model.ts`:

```ts
export type PluginEditorView = "startHere" | "runs" | "localOrg" | "debug" | "plugins";
export type PluginActionContext =
  | "project"
  | "activeApexFile"
  | "activeDebugLog"
  | "activeDataEnvironment"
  | "lastLocalRun";
export type PluginActionOutput = "glade.findings.v1" | "glade.markdownReport.v1" | "glade.rawText.v1";

export interface PluginEditorAction {
  id: string;
  title: string;
  description?: string;
  view: PluginEditorView;
  contexts?: PluginActionContext[];
  command: string[];
  args?: string[];
  output: PluginActionOutput;
  icon?: string;
}

export interface PluginEditorMetadata {
  actions?: PluginEditorAction[];
}

export interface InstalledPlugin {
  name: string;
  identityName: string;
  canonicalName?: string;
  version: string;
  linked?: boolean;
  commands: string[];
  executable?: string;
  manifest?: string;
  source?: string;
  editor?: PluginEditorMetadata;
}

export interface PluginListState {
  plugins: InstalledPlugin[];
}

export interface PluginActionRuntimeContext {
  projectRoot?: string;
  workspaceFolder?: string;
  activeFile?: string;
  activeDb?: string;
  outputDir?: string;
}

export interface ResolvedPluginAction {
  plugin: InstalledPlugin;
  action: PluginEditorAction;
}
```

- [ ] **Step 4: Create `actions.ts`**

Create `contrib/vscode-glade/src/plugins/actions.ts`:

```ts
import {
  InstalledPlugin,
  PluginActionRuntimeContext,
  PluginEditorAction,
  PluginEditorView,
  PluginListState,
  ResolvedPluginAction,
} from "./model";

export function actionsForView(
  state: PluginListState,
  view: PluginEditorView,
  contexts: Set<string>,
): ResolvedPluginAction[] {
  const out: ResolvedPluginAction[] = [];
  for (const plugin of state.plugins) {
    for (const action of plugin.editor?.actions || []) {
      if (action.view !== view) {
        continue;
      }
      if (!actionContextsMatch(action, contexts)) {
        continue;
      }
      out.push({ plugin, action });
    }
  }
  return out;
}

export function actionContextsMatch(action: PluginEditorAction, contexts: Set<string>): boolean {
  for (const required of action.contexts || []) {
    if (!contexts.has(required)) {
      return false;
    }
  }
  return true;
}

export function expandActionArgs(action: PluginEditorAction, context: PluginActionRuntimeContext): string[] {
  return [
    ...action.command,
    ...(action.args || []).map((arg) => expandToken(arg, context)),
  ];
}

function expandToken(value: string, context: PluginActionRuntimeContext): string {
  const replacements: Record<string, string | undefined> = {
    "${projectRoot}": context.projectRoot,
    "${workspaceFolder}": context.workspaceFolder,
    "${activeFile}": context.activeFile,
    "${activeDb}": context.activeDb,
    "${outputDir}": context.outputDir,
  };
  return value.replace(/\$\{[A-Za-z0-9_]+\}/g, (token) => {
    const replacement = replacements[token];
    if (replacement === undefined) {
      throw new Error(`missing plugin action token ${token}`);
    }
    return replacement;
  });
}

export function pluginLabel(plugin: InstalledPlugin): string {
  return plugin.identityName || plugin.canonicalName || plugin.name;
}
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
git add contrib/vscode-glade/src/plugins/model.ts contrib/vscode-glade/src/plugins/actions.ts contrib/vscode-glade/test/pluginActions.test.js contrib/vscode-glade/package.json
git commit -m "feat: model vscode plugin actions"
```

---

### Task 4: Parse Plugin Findings

**Owner:** Agent F

**Files:**
- Create: `contrib/vscode-glade/src/plugins/findings.ts`
- Create: `contrib/vscode-glade/test/pluginFindings.test.js`
- Modify: `contrib/vscode-glade/package.json`

- [ ] **Step 1: Write failing findings tests**

Create `contrib/vscode-glade/test/pluginFindings.test.js`:

```js
const assert = require("assert");
const findings = require("../out/plugins/findings");

const parsed = findings.parseFindings(JSON.stringify({
  kind: "glade.findings.v1",
  summary: "2 findings",
  findings: [
    {
      severity: "warning",
      message: "SOQL query inside a loop",
      file: "force-app/main/default/classes/Foo.cls",
      line: 42,
      column: 9,
      ruleId: "apex.soql.loop",
      source: "compat"
    },
    {
      severity: "bad",
      message: "Unknown severity becomes warning",
      file: "/repo/force-app/main/default/classes/Bar.cls"
    }
  ],
  artifacts: [{ label: "Report", path: ".glade/reports/compat.md" }]
}), "/repo");

assert.strictEqual(parsed.summary, "2 findings");
assert.strictEqual(parsed.findings[0].absoluteFile, "/repo/force-app/main/default/classes/Foo.cls");
assert.strictEqual(parsed.findings[0].line, 42);
assert.strictEqual(parsed.findings[0].severity, "warning");
assert.strictEqual(parsed.findings[1].severity, "warning");
assert.strictEqual(parsed.artifacts[0].absolutePath, "/repo/.glade/reports/compat.md");

assert.throws(
  () => findings.parseFindings(JSON.stringify({ kind: "other", findings: [] }), "/repo"),
  /expected glade.findings.v1/,
);
```

Add the test to the `test` script.

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: FAIL because `../out/plugins/findings` does not exist.

- [ ] **Step 3: Create findings parser**

Create `contrib/vscode-glade/src/plugins/findings.ts`:

```ts
import * as path from "path";

export type PluginFindingSeverity = "error" | "warning" | "info" | "hint";

export interface PluginFinding {
  severity: PluginFindingSeverity;
  message: string;
  file?: string;
  absoluteFile?: string;
  line?: number;
  column?: number;
  ruleId?: string;
  source?: string;
}

export interface PluginArtifact {
  label: string;
  path: string;
  absolutePath?: string;
}

export interface PluginFindingsDocument {
  kind: "glade.findings.v1";
  summary?: string;
  findings: PluginFinding[];
  artifacts?: PluginArtifact[];
}

export function parseFindings(stdout: string, projectRoot: string): PluginFindingsDocument {
  const parsed = JSON.parse(stdout) as PluginFindingsDocument;
  if (parsed.kind !== "glade.findings.v1") {
    throw new Error(`expected glade.findings.v1, got ${String((parsed as { kind?: unknown }).kind)}`);
  }
  const findings = (parsed.findings || []).map((finding) => ({
    ...finding,
    severity: normalizeSeverity(finding.severity),
    absoluteFile: finding.file ? absolutePath(finding.file, projectRoot) : undefined,
  }));
  const artifacts = (parsed.artifacts || []).map((artifact) => ({
    ...artifact,
    absolutePath: absolutePath(artifact.path, projectRoot),
  }));
  return { ...parsed, findings, artifacts };
}

function normalizeSeverity(value: string): PluginFindingSeverity {
  if (value === "error" || value === "warning" || value === "info" || value === "hint") {
    return value;
  }
  return "warning";
}

function absolutePath(file: string, root: string): string {
  return path.isAbsolute(file) ? file : path.join(root, file);
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
git add contrib/vscode-glade/src/plugins/findings.ts contrib/vscode-glade/test/pluginFindings.test.js contrib/vscode-glade/package.json
git commit -m "feat: parse plugin findings for vscode"
```

---

### Task 5: Add VS Code Plugin CLI Adapter

**Owner:** Agent D

**Files:**
- Create: `contrib/vscode-glade/src/plugins/cli.ts`
- Create: `contrib/vscode-glade/test/pluginCli.test.js`
- Modify: `contrib/vscode-glade/package.json`

- [ ] **Step 1: Write failing adapter tests**

Create `contrib/vscode-glade/test/pluginCli.test.js`:

```js
const assert = require("assert");
const cli = require("../out/plugins/cli");

assert.deepStrictEqual(cli.pluginListArgs(), ["plugins", "list", "--json"]);
assert.deepStrictEqual(cli.pluginDoctorArgs(), ["plugins", "doctor", "--json"]);
assert.deepStrictEqual(
  cli.pluginActionArgs(["compat", "local-tests"], ["--project", "/repo", "--json"]),
  ["compat", "local-tests", "--project", "/repo", "--json"],
);
assert.deepStrictEqual(
  cli.parsePluginList(JSON.stringify({
    plugins: [{ name: "compat", identityName: "@glade/compat", version: "0.1.0", commands: ["compat"] }]
  })).plugins[0].identityName,
  "@glade/compat",
);
```

Add the test to the `test` script.

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: FAIL because `../out/plugins/cli` does not exist.

- [ ] **Step 3: Create CLI adapter**

Create `contrib/vscode-glade/src/plugins/cli.ts`:

```ts
import { runGlade, runGladeJSON } from "../gladeCli";
import { PluginListState } from "./model";

export function pluginListArgs(): string[] {
  return ["plugins", "list", "--json"];
}

export function pluginDoctorArgs(): string[] {
  return ["plugins", "doctor", "--json"];
}

export function pluginActionArgs(command: string[], args: string[] = []): string[] {
  return [...command, ...args];
}

export function parsePluginList(stdout: string): PluginListState {
  const parsed = JSON.parse(stdout) as PluginListState;
  return { plugins: parsed.plugins || [] };
}

export async function listInstalledPlugins(cwd?: string): Promise<PluginListState> {
  return runGladeJSON<PluginListState>(pluginListArgs(), { cwd }, "glade plugins list");
}

export async function runPluginAction(args: string[], cwd?: string): Promise<string> {
  const result = await runGlade(args, { cwd });
  if (result.code !== 0) {
    const detail = result.stderr.trim() || result.stdout.trim() || `exit code ${result.code}`;
    throw new Error(`glade plugin action failed: ${detail}`);
  }
  return result.stdout;
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
git add contrib/vscode-glade/src/plugins/cli.ts contrib/vscode-glade/test/pluginCli.test.js contrib/vscode-glade/package.json
git commit -m "feat: add vscode plugin cli adapter"
```

---

### Task 6: Add Plugin Controller and Diagnostics

**Owner:** Agent D and Agent F after Tasks 3-5

**Files:**
- Create: `contrib/vscode-glade/src/plugins/controller.ts`
- Modify: `contrib/vscode-glade/src/extension.ts`

- [ ] **Step 1: Implement controller**

Create `contrib/vscode-glade/src/plugins/controller.ts`:

```ts
import * as fs from "fs";
import * as path from "path";
import * as vscode from "vscode";
import { configuredActiveEnvironment } from "../localOrg";
import { GladeOutput } from "../output";
import { GladeProjectContext } from "../projectModel";
import { actionsForView, expandActionArgs } from "./actions";
import { listInstalledPlugins, runPluginAction } from "./cli";
import { parseFindings, PluginFindingSeverity } from "./findings";
import { PluginActionRuntimeContext, PluginEditorView, PluginListState, ResolvedPluginAction } from "./model";

export class PluginController implements vscode.Disposable {
  private state: PluginListState = { plugins: [] };
  private project?: GladeProjectContext;
  private readonly changed = new vscode.EventEmitter<void>();
  private readonly diagnostics = vscode.languages.createDiagnosticCollection("glade-plugins");
  readonly onDidChange = this.changed.event;

  constructor(private readonly output: GladeOutput) {}

  dispose(): void {
    this.changed.dispose();
    this.diagnostics.dispose();
  }

  setProject(project: GladeProjectContext | undefined): void {
    this.project = project;
  }

  snapshot(): PluginListState {
    return this.state;
  }

  async refresh(): Promise<void> {
    try {
      this.state = await listInstalledPlugins(this.project?.projectRoot);
    } catch (error) {
      this.state = { plugins: [] };
      const message = error instanceof Error ? error.message : String(error);
      this.output.logs.appendLine(`plugin refresh failed: ${message}`);
    }
    this.changed.fire();
  }

  actionsFor(view: PluginEditorView): ResolvedPluginAction[] {
    return actionsForView(this.state, view, this.contexts());
  }

  async run(resolved: ResolvedPluginAction): Promise<void> {
    if (!this.project) {
      void vscode.window.showErrorMessage("Glade plugin actions require an SFDX project.");
      return;
    }
    const runtime = this.runtimeContext();
    const args = expandActionArgs(resolved.action, runtime);
    this.output.logs.show(true);
    this.output.logs.appendLine(`$ glade ${args.join(" ")}`);
    try {
      const stdout = await runPluginAction(args, this.project.projectRoot);
      this.output.logs.appendLine(stdout.trim());
      if (resolved.action.output === "glade.findings.v1") {
        this.applyFindings(stdout, this.project.projectRoot);
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      this.output.logs.appendLine(message);
      void vscode.window.showErrorMessage(`Glade plugin action failed: ${message}`);
    }
  }

  private contexts(): Set<string> {
    const contexts = new Set<string>();
    if (this.project) {
      contexts.add("project");
      contexts.add("activeDataEnvironment");
    }
    const activeFile = vscode.window.activeTextEditor?.document.uri.fsPath;
    if (activeFile && path.extname(activeFile).toLowerCase() === ".cls") {
      contexts.add("activeApexFile");
    }
    if (activeFile && path.extname(activeFile).toLowerCase() === ".log") {
      contexts.add("activeDebugLog");
    }
    return contexts;
  }

  private runtimeContext(): PluginActionRuntimeContext {
    const activeFile = vscode.window.activeTextEditor?.document.uri.fsPath;
    const activeDb = this.project ? configuredActiveEnvironment(this.project).dbPath : undefined;
    const outputDir = this.project ? path.join(this.project.projectRoot, ".glade", "reports") : undefined;
    if (outputDir) {
      fs.mkdirSync(outputDir, { recursive: true });
    }
    return {
      projectRoot: this.project?.projectRoot,
      workspaceFolder: this.project?.workspaceFolder,
      activeFile,
      activeDb,
      outputDir,
    };
  }

  private applyFindings(stdout: string, projectRoot: string): void {
    const document = parseFindings(stdout, projectRoot);
    const byFile = new Map<string, vscode.Diagnostic[]>();
    for (const finding of document.findings) {
      if (!finding.absoluteFile) {
        continue;
      }
      const line = Math.max((finding.line || 1) - 1, 0);
      const column = Math.max((finding.column || 1) - 1, 0);
      const diagnostic = new vscode.Diagnostic(
        new vscode.Range(line, column, line, column + 1),
        finding.message,
        severity(finding.severity),
      );
      diagnostic.code = finding.ruleId;
      diagnostic.source = finding.source || "glade";
      const current = byFile.get(finding.absoluteFile) || [];
      current.push(diagnostic);
      byFile.set(finding.absoluteFile, current);
    }
    this.diagnostics.clear();
    for (const [file, diagnostics] of byFile.entries()) {
      this.diagnostics.set(vscode.Uri.file(file), diagnostics);
    }
    if (document.summary) {
      void vscode.window.showInformationMessage(`Glade plugin: ${document.summary}`);
    }
  }
}

function severity(value: PluginFindingSeverity): vscode.DiagnosticSeverity {
  switch (value) {
    case "error":
      return vscode.DiagnosticSeverity.Error;
    case "info":
      return vscode.DiagnosticSeverity.Information;
    case "hint":
      return vscode.DiagnosticSeverity.Hint;
    case "warning":
    default:
      return vscode.DiagnosticSeverity.Warning;
  }
}
```

- [ ] **Step 2: Wire controller lifecycle**

In `contrib/vscode-glade/src/extension.ts`:

```ts
import { PluginController } from "./plugins/controller";
```

After `const debugView = new DebugView();` add:

```ts
const plugins = new PluginController(output);
context.subscriptions.push(plugins);
```

In `refreshProject()`, after `debugView.setProject(project);` add:

```ts
plugins.setProject(project);
await plugins.refresh();
```

In the catch block add:

```ts
plugins.setProject(undefined);
await plugins.refresh();
```

- [ ] **Step 3: Run tests and compile**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add contrib/vscode-glade/src/plugins/controller.ts contrib/vscode-glade/src/extension.ts
git commit -m "feat: run plugin actions from vscode"
```

---

### Task 7: Add Plugins View and Commands

**Owner:** Agent E

**Files:**
- Create: `contrib/vscode-glade/src/views/pluginsView.ts`
- Modify: `contrib/vscode-glade/src/extension.ts`
- Modify: `contrib/vscode-glade/src/views/tree.ts`
- Modify: `contrib/vscode-glade/package.json`
- Modify: `contrib/vscode-glade/test/package.test.js`

- [ ] **Step 1: Write failing package tests**

Extend `contrib/vscode-glade/test/package.test.js`:

```js
assert(
  manifest.contributes.views.glade.some((view) => view.id === "glade.plugins" && view.name === "Plugins"),
  "glade.plugins view must exist",
);

for (const command of [
  "glade.refreshPlugins",
  "glade.managePlugins",
  "glade.runPluginAction",
  "glade.linkLocalPlugin",
  "glade.installPluginArchive",
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

Expected: FAIL because view and commands are missing.

- [ ] **Step 3: Create Plugins view**

Create `contrib/vscode-glade/src/views/pluginsView.ts`:

```ts
import * as vscode from "vscode";
import { PluginController } from "../plugins/controller";
import { InstalledPlugin, ResolvedPluginAction } from "../plugins/model";
import { commandItem, GladeTreeItem } from "./tree";

export class PluginsView implements vscode.TreeDataProvider<GladeTreeItem> {
  private readonly changed = new vscode.EventEmitter<GladeTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData = this.changed.event;

  constructor(private readonly plugins: PluginController) {
    plugins.onDidChange(() => this.refresh());
  }

  refresh(): void {
    this.changed.fire();
  }

  getTreeItem(element: GladeTreeItem): vscode.TreeItem {
    return element;
  }

  getChildren(): GladeTreeItem[] {
    const state = this.plugins.snapshot();
    const items: GladeTreeItem[] = [
      commandItem("Manage Plugins", "glade.managePlugins", "Install, link, inspect, or refresh Glade plugins.", new vscode.ThemeIcon("extensions")),
      commandItem("Refresh", "glade.refreshPlugins", "Refresh installed Glade plugins.", new vscode.ThemeIcon("refresh")),
    ];
    if (state.plugins.length === 0) {
      items.push(new GladeTreeItem("No plugins installed"));
      items.push(commandItem("Link Local Plugin", "glade.linkLocalPlugin", "Link a local Glade plugin executable.", new vscode.ThemeIcon("link")));
      items.push(commandItem("Install Plugin Archive", "glade.installPluginArchive", "Install a packaged Glade plugin archive.", new vscode.ThemeIcon("archive")));
      return items;
    }
    for (const plugin of state.plugins) {
      items.push(pluginItem(plugin));
    }
    for (const resolved of this.plugins.actionsFor("plugins")) {
      items.push(actionItem(resolved));
    }
    return items;
  }
}

function pluginItem(plugin: InstalledPlugin): GladeTreeItem {
  const item = new GladeTreeItem(plugin.identityName || plugin.name);
  item.description = `${plugin.version}${plugin.linked ? " linked" : ""}`;
  item.tooltip = `${plugin.commands.join(", ")}\n${plugin.manifest || ""}`;
  item.iconPath = new vscode.ThemeIcon(plugin.linked ? "link" : "extensions");
  return item;
}

function actionItem(resolved: ResolvedPluginAction): GladeTreeItem {
  const item = commandItem(resolved.action.title, "glade.runPluginAction", resolved.action.description, new vscode.ThemeIcon(resolved.action.icon || "run"));
  item.command = {
    command: "glade.runPluginAction",
    title: resolved.action.title,
    arguments: [resolved],
  };
  item.description = resolved.plugin.identityName || resolved.plugin.name;
  return item;
}
```

- [ ] **Step 4: Wire package contributions**

In `contrib/vscode-glade/package.json`, add activation events:

```json
"onView:glade.plugins",
"onCommand:glade.refreshPlugins",
"onCommand:glade.managePlugins",
"onCommand:glade.runPluginAction",
"onCommand:glade.linkLocalPlugin",
"onCommand:glade.installPluginArchive"
```

Add view:

```json
{
  "id": "glade.plugins",
  "name": "Plugins",
  "when": "config.glade.enableSidebar"
}
```

Add commands:

```json
{ "command": "glade.refreshPlugins", "title": "Glade: Refresh Plugins" },
{ "command": "glade.managePlugins", "title": "Glade: Manage Plugins" },
{ "command": "glade.runPluginAction", "title": "Glade: Run Plugin Action" },
{ "command": "glade.linkLocalPlugin", "title": "Glade: Link Local Plugin" },
{ "command": "glade.installPluginArchive", "title": "Glade: Install Plugin Archive" }
```

- [ ] **Step 5: Wire extension commands**

In `contrib/vscode-glade/src/extension.ts`, import:

```ts
import { PluginsView } from "./views/pluginsView";
import { ResolvedPluginAction } from "./plugins/model";
```

After plugin controller creation:

```ts
const pluginsView = new PluginsView(plugins);
```

Register the provider:

```ts
vscode.window.registerTreeDataProvider("glade.plugins", pluginsView),
```

Add commands:

```ts
vscode.commands.registerCommand("glade.refreshPlugins", async () => {
  await plugins.refresh();
  pluginsView.refresh();
}),
vscode.commands.registerCommand("glade.runPluginAction", async (resolved?: ResolvedPluginAction) => {
  if (!resolved) {
    await vscode.commands.executeCommand("glade.managePlugins");
    return;
  }
  await plugins.run(resolved);
}),
vscode.commands.registerCommand("glade.managePlugins", async () => {
  const actions = [
    { label: "Refresh Plugins", command: "glade.refreshPlugins" },
    { label: "Link Local Plugin", command: "glade.linkLocalPlugin" },
    { label: "Install Plugin Archive", command: "glade.installPluginArchive" },
    { label: "Open Glade Output", command: "glade.openOutput" },
  ];
  const picked = await vscode.window.showQuickPick(actions, { placeHolder: "Glade plugins" });
  if (picked) {
    await vscode.commands.executeCommand(picked.command);
  }
}),
vscode.commands.registerCommand("glade.linkLocalPlugin", async () => {
  const picked = await vscode.window.showOpenDialog({
    title: "Link Glade Plugin Executable",
    canSelectFiles: true,
    canSelectFolders: false,
    canSelectMany: false,
  });
  const executable = picked?.[0]?.fsPath;
  if (!executable) {
    return;
  }
  output.logs.show(true);
  output.logs.appendLine(`$ glade plugins link --exec ${executable}`);
  const terminal = vscode.window.createTerminal("Glade Plugins");
  terminal.show();
  terminal.sendText(`glade plugins link --exec ${JSON.stringify(executable)}`);
}),
vscode.commands.registerCommand("glade.installPluginArchive", async () => {
  const picked = await vscode.window.showOpenDialog({
    title: "Install Glade Plugin Archive",
    filters: { "Plugin archives": ["gz"] },
    canSelectFiles: true,
    canSelectFolders: false,
    canSelectMany: false,
  });
  const archive = picked?.[0]?.fsPath;
  if (!archive) {
    return;
  }
  const terminal = vscode.window.createTerminal("Glade Plugins");
  terminal.show();
  terminal.sendText(`glade plugins install ${JSON.stringify(archive)}`);
}),
```

- [ ] **Step 6: Run tests**

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add contrib/vscode-glade/package.json contrib/vscode-glade/src/extension.ts contrib/vscode-glade/src/views/pluginsView.ts contrib/vscode-glade/test/package.test.js
git commit -m "feat: add vscode plugin management view"
```

---

### Task 8: Inject Contextual Plugin Actions Into Existing Views

**Owner:** Agent E

**Files:**
- Modify: `contrib/vscode-glade/src/views/startHereView.ts`
- Modify: `contrib/vscode-glade/src/views/runsView.ts`
- Modify: `contrib/vscode-glade/src/views/localOrgView.ts`
- Modify: `contrib/vscode-glade/src/views/debugView.ts`
- Modify: `contrib/vscode-glade/src/extension.ts`

- [ ] **Step 1: Add action rows to views**

In each view class, add:

```ts
import { ResolvedPluginAction } from "../plugins/model";
```

Add a field and setter:

```ts
private pluginActions: ResolvedPluginAction[] = [];

setPluginActions(actions: ResolvedPluginAction[]): void {
  this.pluginActions = actions;
  this.refresh();
}
```

Add helper:

```ts
function pluginActionItem(resolved: ResolvedPluginAction): GladeTreeItem {
  const item = commandItem(
    resolved.action.title,
    "glade.runPluginAction",
    resolved.action.description,
    new vscode.ThemeIcon(resolved.action.icon || "run"),
  );
  item.command = {
    command: "glade.runPluginAction",
    title: resolved.action.title,
    arguments: [resolved],
  };
  item.description = resolved.plugin.identityName || resolved.plugin.name;
  return item;
}
```

Append `...this.pluginActions.map(pluginActionItem)` near the bottom of `getChildren()` for each view.

- [ ] **Step 2: Refresh action rows from extension**

In `contrib/vscode-glade/src/extension.ts`, add:

```ts
function refreshPluginActions(): void {
  startHereView.setPluginActions(plugins.actionsFor("startHere"));
  runsView.setPluginActions(plugins.actionsFor("runs"));
  localOrgView.setPluginActions(plugins.actionsFor("localOrg"));
  debugView.setPluginActions(plugins.actionsFor("debug"));
  pluginsView.refresh();
}
```

After every `await plugins.refresh()` call:

```ts
refreshPluginActions();
```

Listen for active editor changes:

```ts
vscode.window.onDidChangeActiveTextEditor(() => refreshPluginActions()),
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
git add contrib/vscode-glade/src/views/startHereView.ts contrib/vscode-glade/src/views/runsView.ts contrib/vscode-glade/src/views/localOrgView.ts contrib/vscode-glade/src/views/debugView.ts contrib/vscode-glade/src/extension.ts
git commit -m "feat: surface plugin actions in vscode views"
```

---

### Task 9: Add First-Party Plugin Editor Actions

**Owner:** Agent C

**Files:**
- Modify: `../glade-tools/plugins/compat/plugin.json`
- Modify: `../glade-tools/plugins/performance/plugin.json`

- [ ] **Step 1: Update compat manifest**

Add this `editor` block to `../glade-tools/plugins/compat/plugin.json`:

```json
"editor": {
  "actions": [
    {
      "id": "compat.localTestsAnalyze",
      "title": "Analyze Local Test Readiness",
      "description": "Inspect the current SFDX project for local Apex test blockers.",
      "view": "startHere",
      "contexts": ["project"],
      "command": ["compat", "local-tests"],
      "args": ["--project", "${projectRoot}", "--analyze", "--json"],
      "output": "glade.findings.v1",
      "icon": "beaker"
    },
    {
      "id": "compat.postParity",
      "title": "Scan Unsupported Local Surfaces",
      "description": "Scan the current project for surfaces Glade cannot run locally yet.",
      "view": "runs",
      "contexts": ["project"],
      "command": ["post-parity"],
      "args": ["--project", "${projectRoot}", "--json"],
      "output": "glade.findings.v1",
      "icon": "search"
    },
    {
      "id": "compat.uiControllers",
      "title": "Discover Visualforce Controllers",
      "description": "Find Visualforce controller surfaces in the current project.",
      "view": "debug",
      "contexts": ["project"],
      "command": ["ui-controllers"],
      "args": ["--project", "${projectRoot}", "--json"],
      "output": "glade.findings.v1",
      "icon": "symbol-class"
    }
  ]
}
```

If a command root in this block is not declared in the manifest `commands` array, add it there with the existing summary from the current manifest.

- [ ] **Step 2: Update performance manifest**

Add this `editor` block to `../glade-tools/plugins/performance/plugin.json`:

```json
"editor": {
  "actions": [
    {
      "id": "performance.scanProject",
      "title": "Scan Performance Risks",
      "description": "Scan the current SFDX project for local Apex performance risks.",
      "view": "startHere",
      "contexts": ["project"],
      "command": ["performance"],
      "args": ["--project", "${projectRoot}", "--json"],
      "output": "glade.findings.v1",
      "icon": "pulse"
    }
  ]
}
```

- [ ] **Step 3: Verify manifests through Glade validation**

From `/Users/matt/Dev/glade`, run:

```bash
go test ./internal/pluginhost -count=1
```

Expected: PASS.

From `/Users/matt/Dev/glade-tools`, run:

```bash
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 4: Commit in glade-tools**

```bash
cd /Users/matt/Dev/glade-tools
git add plugins/compat/plugin.json plugins/performance/plugin.json
git commit -m "feat: expose editor actions in first-party plugins"
```

---

### Task 10: Normalize First-Party Plugin Output To Findings

**Owner:** Agent C, only after Task 9 exposes actions

**Files:**
- Modify in `../glade-tools/internal/...` as needed for commands used by editor actions.
- Add tests near the command writers touched.

- [ ] **Step 1: Inspect current JSON for each editor action**

Run from `/Users/matt/Dev/glade-tools`:

```bash
go run ./cmd/glade-plugin-compat compat local-tests --project ../glade/testdata/local-tests/enterprise-composed --analyze --json > /tmp/compat-local-tests.json
go run ./cmd/glade-plugin-compat post-parity --project ../glade/testdata/local-tests/enterprise-composed --json > /tmp/compat-post-parity.json
go run ./cmd/glade-plugin-performance performance --project ../glade/testdata/local-tests/enterprise-composed --json > /tmp/performance.json
```

Expected: commands exit 0 and write JSON.

- [ ] **Step 2: Add findings adapter writers if current JSON is not `glade.findings.v1`**

For each command, add an editor-specific flag:

```text
--editor-findings
```

When set, write:

```json
{
  "kind": "glade.findings.v1",
  "summary": "<count> findings",
  "findings": [],
  "artifacts": []
}
```

Map project-level findings without a file to entries with no `file`. Map file-specific findings to relative project paths.

- [ ] **Step 3: Update manifest args if adapters are needed**

Change action args from `--json` to:

```json
["--project", "${projectRoot}", "--json", "--editor-findings"]
```

Use the exact order accepted by the command parser.

- [ ] **Step 4: Verify glade-tools**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit in glade-tools**

```bash
git add .
git commit -m "feat: emit vscode findings from first-party plugins"
```

---

### Task 11: Document Plugin Integration

**Owner:** Docs agent after implementation compiles

**Files:**
- Modify: `docs/EDITOR.md`
- Modify: `contrib/vscode-glade/README.md`
- Modify: `site/docs-src/guide/editor.md`

- [ ] **Step 1: Add editor plugin section**

Add to `docs/EDITOR.md`:

````markdown
## Plugin Actions

The VS Code extension reads installed Glade plugins through `glade plugins list
--json`. It does not run plugin binaries directly.

Linked or installed plugins can contribute editor actions through their
`plugin.json` metadata. Glade shows those actions where they apply:

- Start Here for project-level readiness scans.
- Local Runs for test and support scans.
- Local Org for active data checks.
- Debug for debug-log and local-debug helpers.
- Plugins for plugin management.

Use local plugins during development:

```bash
go build -o /tmp/glade-plugin-compat ../glade-tools/cmd/glade-plugin-compat
glade plugins link --exec /tmp/glade-plugin-compat
glade plugins list --json
```

Plugin findings appear in the Problems panel when an action emits
`glade.findings.v1`.
````

- [ ] **Step 2: Mirror compact instructions in README and site docs**

Add a shorter version to `contrib/vscode-glade/README.md` and `site/docs-src/guide/editor.md`.

- [ ] **Step 3: Run doc-adjacent checks**

Run:

```bash
rg -n "Salesforce extensions detected|Salesforce Extension Coexistence|extensions found" docs contrib/vscode-glade site
```

Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add docs/EDITOR.md contrib/vscode-glade/README.md site/docs-src/guide/editor.md
git commit -m "docs: describe vscode plugin actions"
```

---

### Task 12: Integration Smoke

**Owner:** Coordinator

**Files:** No planned source edits unless smoke finds bugs.

- [ ] **Step 1: Build Glade and plugin binaries**

Run:

```bash
cd /Users/matt/Dev/glade
go build -o /tmp/glade ./cmd/glade
cd /Users/matt/Dev/glade-tools
go build -o /tmp/glade-plugin-compat ./cmd/glade-plugin-compat
go build -o /tmp/glade-plugin-performance ./cmd/glade-plugin-performance
```

Expected: all commands exit 0.

- [ ] **Step 2: Link first-party plugins into an isolated Glade home**

Run:

```bash
export GLADE_HOME="$(mktemp -d)"
PATH="/tmp:$PATH" /tmp/glade plugins link --exec /tmp/glade-plugin-compat
PATH="/tmp:$PATH" /tmp/glade plugins link --exec /tmp/glade-plugin-performance
PATH="/tmp:$PATH" /tmp/glade plugins list --json
```

Expected: JSON includes `compat.localTestsAnalyze` and `performance.scanProject`.

- [ ] **Step 3: Run extension tests**

Run:

```bash
cd /Users/matt/Dev/glade/contrib/vscode-glade
npm test
npm run package
```

Expected: tests pass and `dist/vscode-glade-0.0.1.vsix` is packaged. The current `vsce` bundling warning is acceptable if the package succeeds.

- [ ] **Step 4: Install VSIX**

Run:

```bash
cd /Users/matt/Dev/glade
glade editor install vscode --vsix contrib/vscode-glade/dist/vscode-glade-0.0.1.vsix --force
```

Expected: VS Code reports successful install.

- [ ] **Step 5: Manual VS Code smoke**

Open:

```bash
code /Users/matt/Dev/glade/testdata/local-tests/enterprise-composed
```

In VS Code:

1. Run `Developer: Reload Window`.
2. Open the Glade Activity Bar.
3. Confirm `Plugins` view shows linked `@glade/compat` and `@glade/performance`.
4. Confirm Start Here shows `Analyze Local Test Readiness` and `Scan Performance Risks`.
5. Click `Analyze Local Test Readiness`.
6. Confirm Glade Output shows the command.
7. Confirm Problems panel receives diagnostics if findings include file-level entries.
8. Confirm no sidebar row mentions Salesforce extension detection.

- [ ] **Step 6: Run full focused verification**

Run from `/Users/matt/Dev/glade`:

```bash
go test ./internal/pluginhost ./internal/gladecli -count=1
cd contrib/vscode-glade && npm test && npm run package
git diff --check
```

Run from `/Users/matt/Dev/glade-tools` if Task 9 or 10 changed it:

```bash
go test ./... -count=1
git diff --check
```

Expected: all pass.

---

## Stretch Goals

Do these only after the main plan is green.

1. `glade plugins available --json` and `search --json` for install suggestions.
2. First-party quick install buttons for `@glade/compat` and `@glade/performance` when registry DNS works.
3. Markdown report preview command for `glade.markdownReport.v1`.
4. Status Bar summary such as `Glade: 2 plugin findings`.
5. A core Debug view action for `glade debug profile --log <activeFile> --json`; this is a core Glade debug feature, not a plugin action.

## Final Review Checklist

- [ ] `glade plugins list --json` is the extension source of truth.
- [ ] Extension does not run plugin binaries directly.
- [ ] Extension does not scrape text output.
- [ ] Plugin actions are contextual, not a large marketplace shelf.
- [ ] First-party plugin actions work when linked locally from `../glade-tools`.
- [ ] Registry failures do not break the sidebar.
- [ ] No user-facing row or doc says "Salesforce extensions detected" or similar.
- [ ] `go test ./internal/pluginhost ./internal/gladecli -count=1` passes.
- [ ] `cd contrib/vscode-glade && npm test && npm run package` passes.
- [ ] `cd ../glade-tools && go test ./... -count=1` passes if glade-tools changed.
