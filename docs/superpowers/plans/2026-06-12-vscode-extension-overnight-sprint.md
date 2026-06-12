# Glade VS Code Extension Overnight Sprint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn `contrib/vscode-glade` into the primary Glade editor extension: SFDX-aware project cockpit, native Test Explorer, LSP, DAP test/debug launches, named local data environments, local org inspection, and one-command install from a Glade release.

**Architecture:** Keep the extension source in this repo for the overnight sprint because the CLI contracts, release archive, and extension code still need to move together. Make `contrib/vscode-glade` a clean package with narrow process boundaries so it can be split into a separate repo once the extension JSON contracts stabilize. Release archives should bundle the VSIX under `share/glade/editor/vscode-glade.vsix`, and `glade editor install vscode` should install that bundled VSIX when the user does not pass `--vsix`.

**Tech Stack:** VS Code Extension API, TypeScript, Node child processes, VS Code Testing API, `vscode-languageclient`, Debug Adapter Protocol over stdio, existing `glade config show --json`, `glade test --json`, `glade test changed --json`, `glade test --daemon --watch`, `glade lsp`, `glade dap`, `glade exec --project <root> --db <path>`, `glade db inspect --json`, GitHub Actions, `@vscode/vsce`.

---

## Product Decision

Keep `contrib/vscode-glade` in `/Users/matt/Dev/glade` for this sprint.

Why:

- The extension needs new CLI JSON contracts and release packaging in the same overnight cut.
- The user path is simpler: install Glade, then run `glade editor install vscode`.
- A separate repo would require cross-repo version pinning before the extension has stable contracts.
- A clean `contrib/vscode-glade` package can still become `glade-vscode` later with no product code move.

Do not create a second repository during this sprint. Do not add a monorepo package manager at the repo root. Keep Node dependencies inside `contrib/vscode-glade`.

## Overnight Scope

Ship these by morning:

- VS Code Activity Bar icon named `Glade`.
- Sidebar views: Project, Recommended Runs, Apex Tests, Data Environments, Local Org, Debug And Logs.
- SFDX project detection from `sfdx-project.json`, with `glade config show --json` as the truth for root, namespace, package dirs, and API version.
- Native Test Explorer with run class, run method, run failed, and run changed commands.
- LSP setting and lifecycle guard: default idle when Salesforce Apex language support is installed, with `glade.enableLsp=true` as the explicit local diagnostics switch.
- DAP launch support for anonymous Apex, current test method, current test class, and selected Test Explorer item.
- Breakpoint-first debug workflow: show active breakpoints in the Glade debug view, let users debug selected local tests with those breakpoints, and keep the standard VS Code breakpoint gutter behavior.
- Local data environments: named SQLite-backed local org states. The overnight MVP must power `glade exec`; it must expose a stable active environment path for test, server, and playground follow-on contracts.
- Local Org view based on the current `glade db inspect --db <path> --project <root> --json` shape, with actions to create, switch, seed, reset, export, and inspect named environments.
- Seamless coexistence with the Salesforce VS Code Extension Pack. Do not replace, shadow, or hijack Salesforce scratch-org commands, CodeLens labels, language server, test sidebar, replay debugger, or default org workflows.
- Bundled VSIX install through `glade editor install vscode`.
- Remove JetBrains plugin source, workflow, and user docs.

## Runway Model

Use three tiers. Do not stop when Tier 1 lands.

Tier 1, morning gate:

- Remove JetBrains.
- Bundle VSIX and make `glade editor install vscode` work without a manual path.
- Build SFDX project context, Activity Bar, Test Explorer, DAP launch commands, DB-backed DAP, breakpoint view, and named local data environments.
- Make `glade exec --project <root> --db <path>` persist successful anonymous Apex DML.
- Keep Glade LSP default-off when Salesforce Apex language support is installed. LSP wiring must not block the test-drive path.
- Prove the extension packages and the CLI gates pass.

Tier 2, fast-agent stretch:

- Add Glade CodeLens with `Run Local Test`, `Debug Local Test`, and `Run Local Changed Impact`.
- Finish full LSP lifecycle polish after the core workflow is usable.
- Add environment clone, export, import, and seed-history commands.
- Add warm daemon watch integration in the sidebar with live status and cancel.
- Add breakpoint validation against resolved Apex source paths before a debug launch.
- Add a first-pass local data snapshot diff for object row counts and changed record ids.
- Add Extension Development Host smoke automation through `@vscode/test-electron`.

Tier 3, extra runway:

- Add tree filtering and quick-pick flows for large test suites.
- Add a local execution history panel that re-runs anonymous Apex snippets.
- Add debug-log X-Ray entry points that open the last local debug log and summarize top frames.
- Add `glade test` environment plumbing only after a CLI contract is designed and tested.
- Add local server/playground environment selection only after the extension no longer hardcodes the active DB path in any command.
- Add release-note screenshots from the VS Code mockup or Extension Development Host.

Hard out-of-scope for this overnight sprint:

- Marketplace publishing.
- Separate repo creation.
- Full SOQL scratch UI, because base `glade db query` is not currently shipped.
- Rich coverage overlay from real coverage data unless the existing test JSON already contains it.
- Webview-based custom dashboards. Use native Tree Views and Testing API first.
- Taking ownership of the generic `Run Test`, `Run All Tests`, `SFDX:*`, or Salesforce Apex Language Server flows supplied by Salesforce extensions.
- Full record-level data editor with grid editing. The stretch diff can count and compare; it should not become a full database UI.

Deprioritize these if the clock gets tight:

- Full LSP feature polish. Salesforce already covers language features for the target user.
- CodeLens. Useful, but Test Explorer and Activity Bar commands prove the local lane first.
- Screenshots and release-note polish. A working Extension Host beats a handsome README.
- Environment diff. Create, switch, seed, execute, and inspect are the workflow spine.
- Release archive smoke before the extension package itself passes. First make the VSIX; then bundle it.

## Tomorrow Test Drive

Target date: Saturday, June 13, 2026.

The first usable build should support this standard local Apex workflow:

1. Install Glade and run:

```bash
glade editor doctor vscode
glade editor install vscode --force
```

Expected: VS Code installs `Glade Local Apex` from the bundled VSIX. No manual VSIX path.

2. Open a normal SFDX project that already has Salesforce VS Code extensions installed.

Expected: Salesforce commands and CodeLens still appear. Glade adds a separate Activity Bar icon and a separate `Glade Apex` test controller.

3. Open the Glade Activity Bar.

Expected rows:

- Project shows SFDX root, package dirs, namespace, and API version.
- Recommended Runs shows changed-test and watch commands.
- Apex Tests shows discovered local test classes and methods.
- Data Environments shows `Active: dev` at `.glade/envs/dev.sqlite`.
- Local Org offers inspect, seed, reset, and export for the active environment.
- Debug And Logs shows active Apex breakpoint count.

4. Run a local test method from Test Explorer.

Expected: `glade test --project <root> --json --class <Class> --method <Method>` runs, output streams to `Glade Tests`, and failures navigate to source.

5. Set a VS Code breakpoint in an Apex class and run `Debug Local Test`.

Expected: the debug session starts through `glade dap --project <root> --db <active-db>` once Squad 6A lands. VS Code sends breakpoints through normal DAP `setBreakpoints`; Glade does not draw a second gutter.

6. Create a data environment named `feature`, switch to it, and execute anonymous Apex:

```apex
insert new Account(Name = 'Glade Local Test');
```

Expected: the command runs as `glade exec --project <root> --db <active-db> ...`, persists on success, and Local Org inspect shows `byObject.Account` increased in `feature` only.

7. Switch back to `dev` and inspect Local Org.

Expected: `dev` row counts are unchanged. This proves environments isolate local data.

8. Click Salesforce `Run Test` CodeLens if present.

Expected: Salesforce still runs org-backed tests. Glade local commands all say `Local`, so there is no hidden hijack.

Tomorrow build is good enough when steps 1 through 8 work in one project without reloading VS Code.

## Salesforce Extension Pack Coexistence

Assume nearly every target user already has the Salesforce Extension Pack installed. That pack owns org-backed Apex workflows: CodeLens links such as `Run Test` and `Run All Tests`, Salesforce command palette entries, org test discovery, Tooling API execution, Apex Language Server features, Apex Replay Debugger, and code coverage tied to org test runs.

Glade must sit beside it as the local Apex lane:

- Use `Glade: ...` command titles and `glade.*` command ids only.
- Use CodeLens labels that include `Local`, such as `Run Local Test`, `Debug Local Test`, and `Run Local Changed Impact`.
- Put Glade results in a separate `Glade Apex` test controller. Do not try to mutate Salesforce test items.
- Put Glade commands in the Glade Activity Bar and context menus. Do not contribute `SFDX:*` commands.
- Start `glade lsp` as an additive local diagnostics provider only when `glade.enableLsp` is true. Default should be false when Salesforce Apex language extensions are installed, so the first overnight build does not double-publish diagnostics or completions.
- Reuse the existing `apex` language id. Do not register a second language id for `.cls` or `.trigger`.
- Keep the existing Salesforce Apex CodeLens visible. Glade CodeLens appears next to it but reads as local.
- Add settings that let users disable each Glade surface: `glade.enableLsp`, `glade.enableCodeLens`, `glade.enableTestExplorer`, `glade.enableSidebar`.
- Detect Salesforce extension presence with `vscode.extensions.getExtension("salesforce.salesforcedx-vscode-apex")`, `vscode.extensions.getExtension("salesforce.salesforcedx-vscode-apex-testing")`, and `vscode.extensions.getExtension("salesforce.apex-language-server-extension")`. Detection should affect defaults and user messaging only. Do not require the Salesforce extension pack.

This is a hard boundary. If a local feature conflicts with an org-backed Salesforce feature, rename or move the Glade feature rather than disabling Salesforce behavior.

## Parallel Squad Layout

Start from a clean branch:

```bash
cd /Users/matt/Dev/glade
git status --short
git switch -c codex/vscode-extension-overnight
```

If the worktree is dirty, stop and record the files. Do not overwrite user work.

Squads can run in parallel after Squad 1 lands the small CLI contract. Squad 2 can begin with stubbed CLI responses and rebase once Squad 1 lands.

| Squad | Owns | Primary files | Can run in parallel |
| --- | --- | --- | --- |
| 0 | JetBrains removal | `contrib/intellij-glade`, `.github/workflows/intellij-glade.yml`, docs | Yes |
| 1 | CLI install and release bundle | `internal/gladecli/editor_command.go`, `scripts/release-build.sh` | Starts first |
| 1A | CLI local data execution contract | `internal/gladecli/cli.go`, `internal/gladecli/cli_test.go` | Starts first |
| 2 | Extension foundation and project context | `contrib/vscode-glade/src/gladeCli.ts`, `projectContext.ts`, `extension.ts` | Yes, after stubs |
| 3 | Activity Bar and sidebar views | `contrib/vscode-glade/package.json`, `src/views/*`, `media/glade.svg` | Yes |
| 4 | Native Test Explorer | `src/tests/*`, `src/testResults.ts` | Yes |
| 5 | LSP client, default-off beside Salesforce | `src/lsp.ts`, `package.json` dependencies | Yes, non-blocking |
| 6 | DAP and editor commands | `src/adapter.ts`, `src/commands.ts`, `src/debug.ts` | Yes |
| 6A | DB-backed DAP sessions | `internal/gladecli/dap_command.go`, `internal/dap/live.go`, `src/adapter.ts`, `src/debug.ts` | CLI after 1A and 6; adapter after 7 |
| 7 | Local data environments, local org, and debug/log views | `src/environments.ts`, `src/views/environmentsView.ts`, `src/views/localOrgView.ts`, `src/views/debugView.ts` | Yes |
| 8 | Docs, CI, final verification | README, docs, workflow, release checks | Last integration |
| 9 | Stretch CodeLens and command polish | `src/codeLens.ts`, `src/commands.ts`, `package.json` menus | Yes after Squad 4 |
| 10 | Stretch environment clone/export/import/diff | `src/environments.ts`, `src/localOrg.ts`, `src/views/environmentsView.ts` | Yes after Squad 7 |
| 11 | Stretch warm daemon watch UX | `src/tests/watch.ts`, `src/views/runsView.ts`, `src/output.ts` | Yes after Squad 4 |
| 12 | Stretch breakpoint validation and debug history | `src/breakpoints.ts`, `src/debug.ts`, `src/views/debugView.ts` | Yes after Squad 6 |
| 13 | Stretch Extension Host smoke automation | `contrib/vscode-glade/test/smoke/*`, `.github/workflows/vscode-glade.yml` | Yes after Squad 8 |
| 14 | Stretch release screenshots and docs polish | `docs/EDITOR.md`, `site/docs-src/guide/editor.md`, `contrib/vscode-glade/README.md` | Yes after Squad 8 |

## File Structure

Keep current package root:

```text
contrib/vscode-glade/
  media/
    glade.svg
  src/
    adapter.ts
    commands.ts
    commandModel.ts
    codeLens.ts
    breakpoints.ts
    debug.ts
    debugHistory.ts
    environmentDiff.ts
    environments.ts
    extension.ts
    gladeCli.ts
    localOrg.ts
    localOrgModel.ts
    lsp.ts
    output.ts
    projectContext.ts
    projectModel.ts
    status.ts
    testResults.ts
    tests/
      controller.ts
      discovery.ts
      model.ts
      runner.ts
      watch.ts
    views/
      debugView.ts
      environmentsView.ts
      localOrgView.ts
      projectView.ts
      runsView.ts
      tree.ts
  test/
    commandModel.test.js
    debugHistory.test.js
    environmentDiff.test.js
    environments.test.js
    gladeCli.test.js
    localOrg.test.js
    projectModel.test.js
    testResults.test.js
    testDiscovery.test.js
```

Responsibility lines:

- `gladeCli.ts`: one wrapper for spawning `glade`, collecting output, and parsing JSON.
- `projectModel.ts`: pure SFDX root, Salesforce extension detection, and `glade config show --json` parsing.
- `projectContext.ts`: VS Code workspace integration around `projectModel.ts`.
- `environments.ts`: named local data environment settings and active DB path resolution.
- `localOrg.ts`: VS Code command wrapper for active-environment DB inspection, seed, reset, and export.
- `localOrgModel.ts`: pure `glade db inspect --json` parsing.
- `codeLens.ts`: Glade-only CodeLens commands with `Local` in every label.
- `breakpoints.ts`: read active VS Code Apex breakpoints for the Debug And Logs view.
- `environmentDiff.ts`: pure count-level comparison between two `glade db inspect --json` results.
- `debugHistory.ts`: pure model for last-run debug launch history.
- `status.ts`: status bar state only.
- `output.ts`: shared output channels only.
- `tests/*`: VS Code Testing API model, discovery, run mapping, watch mapping.
- `views/*`: TreeDataProviders only. No CLI spawning in view files except through injected services.
- `debug.ts`: commands that start debug sessions from editor or test items.
- `lsp.ts`: language client lifecycle.

## Shared Types

Squads should converge on these TypeScript shapes:

```ts
export interface GladeProjectContext {
  workspaceFolder: string;
  projectRoot: string;
  configPath?: string;
  configFound: boolean;
  namespace?: string;
  sourceApiVersion?: string;
  packageDirs: string[];
  salesforceExtensions: {
    apex: boolean;
    apexTesting: boolean;
    apexLanguageServerTypescript: boolean;
  };
}

export interface GladeRunSummary {
  total: number;
  passed: number;
  failed: number;
  skipped: number;
  compileErrors: number;
  runtimeErrors: number;
  unsupported: number;
  errors: number;
  durationMs: number;
}

export interface GladeTestCase {
  name?: string;
  className?: string;
  methodName?: string;
  status: "pass" | "fail" | "skipped" | "compile_error" | "runtime_error" | "unsupported";
  durationMs?: number;
  problem?: {
    type?: string;
    message: string;
    detail?: string;
    stack?: Array<{ symbol?: string; file?: string; line?: number; column?: number }>;
  };
}

export interface GladeTestRun {
  name?: string;
  durationMs?: number;
  summary: GladeRunSummary;
  suites: Array<{ name: string; durationMs?: number; cases: GladeTestCase[] }>;
}

export interface GladeEnvironment {
  name: string;
  dbPath: string;
  fixturePath?: string;
}

export interface DBInspectResult {
  path?: string;
  schemaVersion?: number;
  objects: number;
  records: number;
  byObject?: Record<string, number>;
  users?: number;
  profiles?: number;
  permissions?: number;
}

export interface LocalOrgObjectRow {
  name: string;
  rows: number;
}

export interface GladeBreakpointSummary {
  file: string;
  line: number;
  enabled: boolean;
}
```

The test-run shapes match current `internal/testreport/model.go` JSON names. `DBInspectResult` matches the current `glade db inspect --json` output from `internal/storage/sqlite.go`.

---

## Squad 0: Remove JetBrains Plugin

**Files:**

- Delete: `contrib/intellij-glade/`
- Delete: `.github/workflows/intellij-glade.yml`
- Modify: `.gitignore`
- Modify: `docs/EDITOR.md`
- Modify: `site/docs-src/guide/editor.md` if it mentions IntelliJ support
- Test: `go test ./internal/gladecli`

**Agent prompt:**

```text
You are Squad 0. Remove the JetBrains/IntelliJ plugin from /Users/matt/Dev/glade.
Delete contrib/intellij-glade and .github/workflows/intellij-glade.yml.
Remove IntelliJ user-facing docs, but leave historical docs/superpowers plan files alone.
Trim .gitignore entries that only apply to contrib/intellij-glade.
Do not change product DAP behavior.
Run go test ./internal/gladecli and git diff --check.
Return the exact files removed or edited and test output.
```

- [ ] **Step 1: Confirm references**

Run:

```bash
rg -n "intellij-glade|IntelliJ Support|JetBrains IDE|LSP4IJ|com.glade.apexdebugger" .github contrib docs site internal -S
```

Expected: hits in `contrib/intellij-glade`, `.github/workflows/intellij-glade.yml`, `docs/EDITOR.md`, and possibly `site/docs-src/guide/editor.md`. Hits for `JetBrains Mono` in brand/theme files are font references and must stay.

- [ ] **Step 2: Delete IntelliJ source and workflow**

Run:

```bash
rm -rf contrib/intellij-glade
rm -f .github/workflows/intellij-glade.yml
```

- [ ] **Step 3: Trim `.gitignore`**

Remove these lines from `.gitignore`:

```gitignore
contrib/intellij-glade/.gradle/
contrib/intellij-glade/.intellijPlatform/
contrib/intellij-glade/build/
contrib/intellij-glade/out/
contrib/intellij-glade/sandbox/
contrib/intellij-glade/*.iml
```

- [ ] **Step 4: Remove IntelliJ user docs**

In `docs/EDITOR.md`, delete the whole section from:

```markdown
## IntelliJ Support
```

through the paragraph immediately before:

```markdown
## DAP Startup Cache
```

If `site/docs-src/guide/editor.md` has the same section, make the same deletion there.

- [ ] **Step 5: Verify**

Run:

```bash
rg -n "intellij-glade|IntelliJ Support|LSP4IJ|com.glade.apexdebugger" .github contrib docs site internal -S
go test ./internal/gladecli
git diff --check
```

Expected: the `rg` command has no hits except allowed historical plan files under `docs/superpowers/` or font text containing `JetBrains Mono`. `go test` passes.

- [ ] **Step 6: Commit**

```bash
git add .github .gitignore contrib docs site
git commit -m "chore: remove JetBrains editor plugin"
```

---

## Squad 1: Bundle VSIX And Make Install Easy

**Files:**

- Modify: `internal/gladecli/editor_command.go`
- Modify: `internal/gladecli/cli_test.go`
- Modify: `internal/cliui/help.go`
- Modify: `scripts/release-build.sh`
- Test: `go test ./internal/gladecli`
- Test: `VERSION=dev DIST_DIR=/tmp/glade-release-test scripts/release-build.sh`

**Agent prompt:**

```text
You are Squad 1. Make VS Code installation easy after Glade is installed.
Add JSON output to glade editor doctor vscode.
Allow glade editor install vscode with no --vsix by locating a bundled VSIX.
Bundle contrib/vscode-glade/dist/*.vsix into release archives at share/glade/editor/vscode-glade.vsix.
Do not publish to the VS Code Marketplace.
Run go test ./internal/gladecli and the local release-build smoke.
Return CLI output and the archive path contents proving the VSIX is included.
```

- [ ] **Step 1: Add failing tests for doctor JSON and bundled install**

Append tests in `internal/gladecli/cli_test.go` near existing editor tests:

```go
func TestRunEditorDoctorVSCodeJSONReportsBundledVSIX(t *testing.T) {
	restore := stubEditorCommandDeps(t,
		func(name string) (string, error) {
			switch name {
			case "code":
				return "/usr/local/bin/code", nil
			case "glade":
				return "/Users/matt/.local/bin/glade", nil
			default:
				return "", os.ErrNotExist
			}
		},
		func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, nil
		},
	)
	defer restore()

	var stdout bytes.Buffer
	if err := runEditor(context.Background(), []string{"doctor", "vscode", "--json"}, &stdout); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Target string `json:"target"`
		Editor struct {
			Command string `json:"command"`
			Path    string `json:"path"`
			OK      bool   `json:"ok"`
		} `json:"editor"`
		Glade struct {
			Path string `json:"path"`
			OK   bool   `json:"ok"`
		} `json:"glade"`
		BundledVSIX struct {
			Path   string `json:"path"`
			Exists bool   `json:"exists"`
		} `json:"bundledVsix"`
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout.String())
	}
	if got.Target != "vscode" || got.Editor.Command != "code" || !got.Editor.OK || !got.Glade.OK {
		t.Fatalf("doctor json = %#v", got)
	}
}

func TestRunEditorInstallVSCodeUsesBundledVSIXWhenPathOmitted(t *testing.T) {
	vsix := filepath.Join(t.TempDir(), "vscode-glade.vsix")
	if err := os.WriteFile(vsix, []byte("vsix"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GLADE_VSCODE_VSIX", vsix)
	var ranArgs []string
	restore := stubEditorCommandDeps(t,
		func(name string) (string, error) {
			if name != "code" {
				t.Fatalf("looked up %q, want code", name)
			}
			return "/usr/local/bin/code", nil
		},
		func(ctx context.Context, name string, args ...string) ([]byte, error) {
			ranArgs = append([]string(nil), args...)
			return []byte("installed\n"), nil
		},
	)
	defer restore()

	var stdout bytes.Buffer
	if err := runEditor(context.Background(), []string{"install", "vscode", "--force"}, &stdout); err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"--install-extension", vsix, "--force"}
	if strings.Join(ranArgs, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("ran args = %#v, want %#v", ranArgs, wantArgs)
	}
}
```

Run:

```bash
go test ./internal/gladecli -run 'TestRunEditor(DoctorVSCodeJSONReportsBundledVSIX|InstallVSCodeUsesBundledVSIXWhenPathOmitted)'
```

Expected: FAIL because `--json` and omitted `--vsix` are not supported.

- [ ] **Step 2: Implement editor doctor JSON**

In `internal/gladecli/editor_command.go`, add:

```go
type editorDoctorReport struct {
	Target      string              `json:"target"`
	Editor      editorDoctorCommand `json:"editor"`
	Glade       editorDoctorCommand `json:"glade"`
	BundledVSIX editorBundledVSIX   `json:"bundledVsix"`
	OK          bool                `json:"ok"`
}

type editorDoctorCommand struct {
	Command string `json:"command,omitempty"`
	Path    string `json:"path,omitempty"`
	Error   string `json:"error,omitempty"`
	OK      bool   `json:"ok"`
}

type editorBundledVSIX struct {
	Path   string `json:"path,omitempty"`
	Exists bool   `json:"exists"`
}
```

Extend `runEditorDoctor` to parse `--json`, build the report, and encode it when requested:

```go
jsonOut := false
for i := 0; i < len(args); i++ {
	switch args[i] {
	case "--editor":
		if i+1 >= len(args) {
			return errors.New("--editor requires a value")
		}
		editor = args[i+1]
		i++
	case "--json":
		jsonOut = true
	default:
		return fmt.Errorf("unknown flag %q", args[i])
	}
}

report := editorDoctorReport{Target: "vscode"}
report.Editor.Command = editorCommandName(editor)
if editorPath, err := resolveEditorCommand(editor); err != nil {
	report.Editor.Error = err.Error()
} else {
	report.Editor.Path = editorPath
	report.Editor.OK = true
}
if gladePath, err := editorCommandLookPath("glade"); err != nil {
	report.Glade.Error = err.Error()
} else {
	report.Glade.Path = gladePath
	report.Glade.OK = true
}
if vsix, err := resolveBundledVSIX(); err == nil {
	report.BundledVSIX.Path = vsix
	report.BundledVSIX.Exists = true
}
report.OK = report.Editor.OK && report.Glade.OK
if jsonOut {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
```

Add `encoding/json` and `path/filepath` imports as needed.

- [ ] **Step 3: Resolve bundled VSIX**

Add to `internal/gladecli/editor_command.go`:

```go
func resolveBundledVSIX() (string, error) {
	if fromEnv := strings.TrimSpace(os.Getenv("GLADE_VSCODE_VSIX")); fromEnv != "" {
		return existingVSIX(fromEnv)
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(filepath.Dir(exe), "share", "glade", "editor", "vscode-glade.vsix")
	return existingVSIX(candidate)
}

func existingVSIX(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("vsix %q is a directory", path)
	}
	return path, nil
}
```

Change install parsing so `--vsix` is optional. After parsing flags:

```go
if vsix == "" {
	resolved, err := resolveBundledVSIX()
	if err != nil {
		return fmt.Errorf("--vsix is required when no bundled VS Code extension is available: %w", err)
	}
	vsix = resolved
}
```

- [ ] **Step 4: Update help text**

In `internal/cliui/help.go`, change editor usage to include:

```go
Usage: []string{
	"glade editor install vscode [--vsix <path>] [--editor <code|cursor|windsurf>] [--force]",
	"glade editor doctor vscode [--editor <code|cursor|windsurf>] [--json]",
},
```

Change `--vsix` description to:

```go
{Name: "--vsix", Value: "<path>", Description: "VS Code extension package. Defaults to bundled VSIX when available."},
```

- [ ] **Step 5: Bundle VSIX in release archive**

In `scripts/release-build.sh`, before copying runtime files into `workdir`, add:

```bash
(
	cd "${ROOT}/contrib/vscode-glade"
	if [[ ! -d node_modules ]]; then
		npm ci
	fi
	npm run package
)
mkdir -p "${workdir}/share/glade/editor"
cp "${ROOT}"/contrib/vscode-glade/dist/vscode-glade-*.vsix "${workdir}/share/glade/editor/vscode-glade.vsix"
```

Keep the existing archive logic. The VSIX should ride inside each platform archive. Do not upload a standalone VSIX from the platform matrix in this task.

- [ ] **Step 6: Verify**

Run:

```bash
go test ./internal/gladecli -run 'TestRunEditor'
go test ./internal/gladecli
VERSION=dev DIST_DIR=/tmp/glade-release-test scripts/release-build.sh
tar -tzf /tmp/glade-release-test/glade_dev_$(go env GOOS)_$(go env GOARCH).tar.gz | rg 'share/glade/editor/vscode-glade.vsix'
```

Expected: tests pass, release build succeeds, tar listing prints `share/glade/editor/vscode-glade.vsix`.

- [ ] **Step 7: Commit**

```bash
git add internal/gladecli internal/cliui scripts/release-build.sh
git commit -m "feat: bundle VS Code extension installer"
```

---

## Squad 1A: CLI Local Data Execution Contract

**Files:**

- Modify: `internal/gladecli/cli.go`
- Modify: `internal/gladecli/cli_test.go`
- Modify: `internal/cliui/help.go`
- Test: `go test ./internal/gladecli -run 'TestRunExec.*DB'`
- Test: `go test ./internal/gladecli`

**Agent prompt:**

```text
You are Squad 1A. Make local data environments real for anonymous Apex.
Add glade exec --project <root> --db <path> and --dry-run.
Load the org with the existing openDBStore helper from db_command.go.
Attach that org to the VM before Execute.
Persist the VM org back to SQLite only when execution succeeds and --dry-run is false.
Do not change glade test, server, playground, or db command behavior in this task.
Run the focused DB exec tests and go test ./internal/gladecli.
Return the exact command lines and inspect JSON that prove persistence and dry-run behavior.
```

- [ ] **Step 1: Add failing persistence test**

Append near existing exec tests in `internal/gladecli/cli_test.go`:

```go
func TestRunExecWithDBPersistsAnonymousDML(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"63.0"}`)
	dbPath := filepath.Join(root, ".glade", "envs", "dev.sqlite")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"exec",
		"--project", root,
		"--db", dbPath,
		"insert new Account(Name = 'Pond Supply');",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exec failed code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "inspect", "--db", dbPath, "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("inspect failed code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var inspect struct {
		ByObject map[string]int `json:"byObject"`
		Records  int            `json:"records"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &inspect); err != nil {
		t.Fatalf("inspect json: %v\n%s", err, stdout.String())
	}
	if got := inspect.ByObject["Account"]; got != 1 {
		t.Fatalf("Account rows = %d, want 1; inspect=%s", got, stdout.String())
	}
}
```

Add the local helper only if this test file does not already have one:

```go
func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
```

Run:

```bash
go test ./internal/gladecli -run 'TestRunExecWithDBPersistsAnonymousDML'
```

Expected: FAIL because `glade exec` does not accept `--project` or `--db`.

- [ ] **Step 2: Add failing dry-run test**

Append:

```go
func TestRunExecWithDBDryRunDoesNotPersist(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"63.0"}`)
	dbPath := filepath.Join(root, ".glade", "envs", "dev.sqlite")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"exec",
		"--project", root,
		"--db", dbPath,
		"--dry-run",
		"insert new Account(Name = 'Dry Run Supply');",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exec failed code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "inspect", "--db", dbPath, "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("inspect failed code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var inspect struct {
		ByObject map[string]int `json:"byObject"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &inspect); err != nil {
		t.Fatalf("inspect json: %v\n%s", err, stdout.String())
	}
if got := inspect.ByObject["Account"]; got != 0 {
	t.Fatalf("Account rows = %d, want 0; inspect=%s", got, stdout.String())
}
}
```

- [ ] **Step 3: Add failing execution-error test**

Append:

```go
func TestRunExecWithDBDoesNotPersistOnExecutionError(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"63.0"}`)
	dbPath := filepath.Join(root, ".glade", "envs", "dev.sqlite")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"exec",
		"--project", root,
		"--db", dbPath,
		"insert new Account(Name = 'Bad Supply'); System.assertEquals(1, 2);",
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("exec succeeded, want failure stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "inspect", "--db", dbPath, "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("inspect failed code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var inspect struct {
		ByObject map[string]int `json:"byObject"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &inspect); err != nil {
		t.Fatalf("inspect json: %v\n%s", err, stdout.String())
	}
	if got := inspect.ByObject["Account"]; got != 0 {
		t.Fatalf("Account rows = %d, want 0; inspect=%s", got, stdout.String())
	}
}
```

- [ ] **Step 4: Parse new `exec` flags**

In `runExec`, add flags:

```go
String("project", "p").
String("db", "").
Bool("dry-run", "").
```

After parsing:

```go
projectRoot := strings.TrimSpace(parsed.String("project"))
dbPath := strings.TrimSpace(parsed.String("db"))
dryRun := parsed.Bool("dry-run")
if dbPath != "" && projectRoot == "" {
	projectRoot = "."
}
```

Update the usage error:

```go
return errors.New("usage: glade exec [--project <root>] [--db <path>] [--dry-run] [--json] [--trace <path>] [--debug-log <path>] '<anonymous apex>'")
```

- [ ] **Step 5: Attach DB-backed org to VM**

Before `machine.Execute(program)`:

```go
var store *storage.SQLiteStore
if dbPath != "" {
	loadedStore, org, err := openDBStore(dbPath, projectRoot)
	if err != nil {
		return err
	}
	defer loadedStore.Close()
	store = loadedStore
	machine.SetOrg(&org)
} else if projectRoot != "" {
	org, err := orgForProject(projectRoot)
	if err != nil {
		return err
	}
	machine.SetOrg(&org)
}
```

This reuses `openDBStore` and `orgForProject` from `db_command.go`. Do not duplicate SQLite setup code.

- [ ] **Step 6: Persist only successful non-dry-run DB execution**

After `execErr` handling, and before writing trace/json output:

```go
if store != nil && !dryRun && machine.Org != nil {
	if err := store.Save(storage.SnapshotRuntimeOrg(machine.Org)); err != nil {
		return err
	}
}
```

Keep existing behavior for failed execs: write debug log when requested, return the execution error, and do not save the DB.

- [ ] **Step 7: Update help text**

In `internal/cliui/help.go`, update `glade exec` usage and flags:

```go
"glade exec [--project <root>] [--db <path>] [--dry-run] [--json] '<anonymous apex>'"
```

Add flag descriptions:

```go
{Name: "--project, -p", Value: "<root>", Description: "SFDX project root used for metadata and local org shape."},
{Name: "--db", Value: "<path>", Description: "SQLite local org path for DB-backed anonymous Apex execution."},
{Name: "--dry-run", Description: "Run against the selected local org without saving changes."},
```

- [ ] **Step 8: Verify and commit**

Run:

```bash
go test ./internal/gladecli -run 'TestRunExec.*DB'
go test ./internal/gladecli
git diff --check
```

Expected: focused tests pass, full gladecli tests pass.

Commit:

```bash
git add internal/gladecli internal/cliui
git commit -m "feat: execute anonymous Apex against local data"
```

---

## Squad 2: Extension Foundation And Project Context

**Files:**

- Create: `contrib/vscode-glade/src/gladeCli.ts`
- Create: `contrib/vscode-glade/src/projectModel.ts`
- Create: `contrib/vscode-glade/src/projectContext.ts`
- Create: `contrib/vscode-glade/src/output.ts`
- Create: `contrib/vscode-glade/src/status.ts`
- Modify: `contrib/vscode-glade/src/extension.ts`
- Create: `contrib/vscode-glade/test/gladeCli.test.js`
- Create: `contrib/vscode-glade/test/projectModel.test.js`
- Modify: `contrib/vscode-glade/package.json`
- Test: `cd contrib/vscode-glade && npm test`

**Agent prompt:**

```text
You are Squad 2. Build the extension foundation.
Add a typed glade process wrapper, SFDX project context detector, output channels, and status bar.
Use glade config show --json as the source of project truth.
Detect Salesforce VS Code extensions and expose that in project context. Glade must coexist with Salesforce org-backed features and never require them.
Keep code testable without importing vscode in pure modules.
Do not touch sidebar view files, Testing API files, or DAP code beyond activation wiring.
Run npm test in contrib/vscode-glade.
Return the public TypeScript APIs you created.
```

- [ ] **Step 1: Add failing pure tests**

Create `contrib/vscode-glade/test/projectModel.test.js`:

```js
const assert = require("assert");
const project = require("../out/projectModel");

assert.strictEqual(
  project.nearestSfdxRoot("/repo/force-app/main/default/classes/Foo.cls", [
    "/repo/sfdx-project.json",
  ]),
  "/repo"
);

assert.deepStrictEqual(
  project.parseConfigShowInfo({
    projectRoot: "/repo",
    configFound: true,
    configPath: "/repo/glade.yml",
    namespace: "namz",
    sourceApiVersion: "63.0",
    packageDirs: ["force-app", "unpackaged"],
  }, "/repo", { apex: false, apexTesting: false, apexLanguageServerTypescript: false }),
  {
    workspaceFolder: "/repo",
    projectRoot: "/repo",
    configFound: true,
    configPath: "/repo/glade.yml",
    namespace: "namz",
    sourceApiVersion: "63.0",
    packageDirs: ["force-app", "unpackaged"],
    salesforceExtensions: { apex: false, apexTesting: false, apexLanguageServerTypescript: false },
  }
);

assert.deepStrictEqual(
  project.detectSalesforceExtensions(["salesforce.salesforcedx-vscode-apex", "salesforce.salesforcedx-vscode-apex-testing"]),
  { apex: true, apexTesting: true, apexLanguageServerTypescript: false }
);
```

Create `contrib/vscode-glade/test/gladeCli.test.js`:

```js
const assert = require("assert");
const glade = require("../out/gladeCli");

assert.deepStrictEqual(
  glade.buildGladeArgs("config", ["show", "--project", "/repo", "--json"]),
  ["config", "show", "--project", "/repo", "--json"]
);

assert.deepStrictEqual(
  glade.parseJSONOutput('{"ok":true}\\n', "glade test"),
  { ok: true }
);

assert.throws(
  () => glade.parseJSONOutput("not json", "glade test"),
  /glade test produced invalid JSON/
);
```

Modify `contrib/vscode-glade/package.json` test script:

```json
"test": "npm run compile && node test/commands.test.js && node test/projectModel.test.js && node test/gladeCli.test.js"
```

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: FAIL because the new modules do not exist.

- [ ] **Step 2: Implement `gladeCli.ts`**

Create `contrib/vscode-glade/src/gladeCli.ts`:

```ts
import { spawn } from "child_process";

export interface GladeRunOptions {
  cwd?: string;
  env?: NodeJS.ProcessEnv;
}

export interface GladeRunResult {
  code: number | null;
  stdout: string;
  stderr: string;
}

export function buildGladeArgs(command: string, args: string[]): string[] {
  return [command, ...args];
}

export function runGlade(args: string[], options: GladeRunOptions = {}): Promise<GladeRunResult> {
  return new Promise((resolve, reject) => {
    const child = spawn("glade", args, { cwd: options.cwd, env: options.env });
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (chunk: Buffer) => {
      stdout += chunk.toString();
    });
    child.stderr.on("data", (chunk: Buffer) => {
      stderr += chunk.toString();
    });
    child.on("error", reject);
    child.on("close", (code) => resolve({ code, stdout, stderr }));
  });
}

export function parseJSONOutput<T>(stdout: string, label: string): T {
  try {
    return JSON.parse(stdout) as T;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    throw new Error(`${label} produced invalid JSON: ${message}`);
  }
}

export async function runGladeJSON<T>(args: string[], options: GladeRunOptions = {}, label = "glade"): Promise<T> {
  const result = await runGlade(args, options);
  if (result.code !== 0) {
    const detail = result.stderr.trim() || result.stdout.trim() || `exit code ${result.code}`;
    throw new Error(`${label} failed: ${detail}`);
  }
  return parseJSONOutput<T>(result.stdout, label);
}
```

- [ ] **Step 3: Implement pure `projectModel.ts`**

Create `contrib/vscode-glade/src/projectModel.ts`:

```ts
import * as path from "path";

export interface GladeProjectContext {
  workspaceFolder: string;
  projectRoot: string;
  configPath?: string;
  configFound: boolean;
  namespace?: string;
  sourceApiVersion?: string;
  packageDirs: string[];
  salesforceExtensions: SalesforceExtensionState;
}

export interface SalesforceExtensionState {
  apex: boolean;
  apexTesting: boolean;
  apexLanguageServerTypescript: boolean;
}

interface ConfigShowInfo {
  configPath?: string;
  configFound: boolean;
  projectRoot: string;
  namespace?: string;
  sourceApiVersion?: string;
  packageDirs?: string[];
}

export function nearestSfdxRoot(startPath: string, sfdxProjectFiles: string[]): string | undefined {
  let dir = path.dirname(startPath);
  const roots = new Set(sfdxProjectFiles.map((file) => path.dirname(path.resolve(file))));
  while (true) {
    if (roots.has(path.resolve(dir))) {
      return path.resolve(dir);
    }
    const parent = path.dirname(dir);
    if (parent === dir) {
      return undefined;
    }
    dir = parent;
  }
}

export function parseConfigShowInfo(
  info: ConfigShowInfo,
  workspaceFolder: string | undefined,
  salesforceExtensions: SalesforceExtensionState,
): GladeProjectContext {
  return {
    workspaceFolder: workspaceFolder || info.projectRoot,
    projectRoot: info.projectRoot,
    configFound: info.configFound,
    configPath: info.configPath,
    namespace: info.namespace,
    sourceApiVersion: info.sourceApiVersion,
    packageDirs: info.packageDirs || [],
    salesforceExtensions,
  };
}

export function detectSalesforceExtensions(extensionIds: string[]): SalesforceExtensionState {
  const ids = new Set(extensionIds);
  return {
    apex: ids.has("salesforce.salesforcedx-vscode-apex"),
    apexTesting: ids.has("salesforce.salesforcedx-vscode-apex-testing"),
    apexLanguageServerTypescript: ids.has("salesforce.apex-language-server-extension"),
  };
}
```

- [ ] **Step 4: Implement VS Code `projectContext.ts`**

Create `contrib/vscode-glade/src/projectContext.ts`:

```ts
import * as fs from "fs";
import * as path from "path";
import * as vscode from "vscode";
import { runGladeJSON } from "./gladeCli";
import {
  detectSalesforceExtensions,
  GladeProjectContext,
  nearestSfdxRoot,
  parseConfigShowInfo,
} from "./projectModel";

interface ConfigShowInfo {
  configPath?: string;
  configFound: boolean;
  projectRoot: string;
  namespace?: string;
  sourceApiVersion?: string;
  packageDirs?: string[];
}

function detectInstalledSalesforceExtensions() {
  return detectSalesforceExtensions(vscode.extensions.all.map((extension) => extension.id));
}

export async function findProjectContext(): Promise<GladeProjectContext | undefined> {
  const folders = vscode.workspace.workspaceFolders || [];
  if (folders.length === 0) {
    return undefined;
  }
  const activePath = vscode.window.activeTextEditor?.document.uri.fsPath;
  const sfdxFiles = await vscode.workspace.findFiles("**/sfdx-project.json", "**/{node_modules,.git,.sfdx,.sf}/**", 50);
  const sfdxPaths = sfdxFiles.map((uri) => uri.fsPath);
  let root: string | undefined;
  if (activePath) {
    root = nearestSfdxRoot(activePath, sfdxPaths);
  }
  if (!root && sfdxPaths.length > 0) {
    root = path.dirname(sfdxPaths[0]);
  }
  if (!root) {
    root = folders[0].uri.fsPath;
  }
  const info = await runGladeJSON<ConfigShowInfo>(["config", "show", "--project", root, "--json"], { cwd: root }, "glade config show");
  return parseConfigShowInfo(info, folders[0].uri.fsPath, detectInstalledSalesforceExtensions());
}

export function defaultDbPath(context: GladeProjectContext): string {
  return path.join(context.projectRoot, ".glade", "org.sqlite");
}

export function pathExists(file: string): boolean {
  return fs.existsSync(file);
}
```

- [ ] **Step 5: Add output and status helpers**

Create `contrib/vscode-glade/src/output.ts`:

```ts
import * as vscode from "vscode";

export class GladeOutput {
  readonly tests = vscode.window.createOutputChannel("Glade Tests");
  readonly logs = vscode.window.createOutputChannel("Glade");

  dispose(): void {
    this.tests.dispose();
    this.logs.dispose();
  }
}
```

Create `contrib/vscode-glade/src/status.ts`:

```ts
import * as vscode from "vscode";
import { GladeProjectContext } from "./projectModel";

export class GladeStatus {
  private readonly item = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 40);

  constructor(context: vscode.ExtensionContext) {
    context.subscriptions.push(this.item);
  }

  setProject(project: GladeProjectContext | undefined): void {
    if (!project) {
      this.item.text = "Glade: no SFDX root";
      this.item.tooltip = "Open a Salesforce DX project with sfdx-project.json.";
      this.item.show();
      return;
    }
    const namespace = project.namespace ? ` ${project.namespace}` : "";
    this.item.text = `Glade: SFDX root${namespace}`;
    this.item.tooltip = project.projectRoot;
    this.item.show();
  }
}
```

- [ ] **Step 6: Wire activation**

In `contrib/vscode-glade/src/extension.ts`, create shared services:

```ts
const output = new GladeOutput();
context.subscriptions.push(output);
const status = new GladeStatus(context);
void findProjectContext()
  .then((project) => status.setProject(project))
  .catch((error: Error) => {
    status.setProject(undefined);
    output.logs.appendLine(`project detection failed: ${error.message}`);
  });
```

Keep existing command and DAP registration intact.

- [ ] **Step 7: Verify and commit**

Run:

```bash
cd contrib/vscode-glade
npm test
npm run package
```

Expected: TypeScript compile passes, unit tests pass, VSIX is written under `dist/`.

Commit:

```bash
git add contrib/vscode-glade
git commit -m "feat: add VS Code project context foundation"
```

---

## Squad 3: Activity Bar And Sidebar Views

**Files:**

- Modify: `contrib/vscode-glade/package.json`
- Create: `contrib/vscode-glade/media/glade.svg`
- Create: `contrib/vscode-glade/src/views/tree.ts`
- Create: `contrib/vscode-glade/src/views/projectView.ts`
- Create: `contrib/vscode-glade/src/views/runsView.ts`
- Create: `contrib/vscode-glade/src/views/localOrgView.ts`
- Create: `contrib/vscode-glade/src/views/debugView.ts`
- Modify: `contrib/vscode-glade/src/extension.ts`
- Test: `cd contrib/vscode-glade && npm test && npm run package`

**Agent prompt:**

```text
You are Squad 3. Build the visible Glade sidebar.
Add a Glade Activity Bar icon and five Tree Views: Project, Recommended Runs, Apex Tests, Local Org, Debug And Logs.
Use TreeDataProvider classes. Keep view providers thin and fed by project context or later test services.
Do not implement test execution in this squad.
Run npm test and npm run package.
Return a list of contributed command ids and view ids.
```

- [ ] **Step 1: Add manifest contributions**

In `contrib/vscode-glade/package.json`, change `displayName`, `description`, categories, activation, and contributes:

```json
"displayName": "Glade Local Apex",
"description": "Run, test, debug, and inspect local Apex projects with Glade.",
"categories": ["Debuggers", "Testing", "Programming Languages"],
"activationEvents": [
  "workspaceContains:**/sfdx-project.json",
  "onLanguage:apex",
  "onView:glade.project",
  "onCommand:glade.executeAnonymous",
  "onCommand:glade.debugAnonymous"
],
"contributes": {
  "viewsContainers": {
    "activitybar": [
      {
        "id": "glade",
        "title": "Glade",
        "icon": "media/glade.svg"
      }
    ]
  },
  "views": {
    "glade": [
      { "id": "glade.project", "name": "Project" },
      { "id": "glade.recommendedRuns", "name": "Recommended Runs" },
      { "id": "glade.apexTests", "name": "Apex Tests" },
      { "id": "glade.environments", "name": "Data Environments" },
      { "id": "glade.localOrg", "name": "Local Org" },
      { "id": "glade.debugLogs", "name": "Debug And Logs" }
    ]
  },
  "commands": [
    { "command": "glade.refresh", "title": "Glade: Refresh" },
    { "command": "glade.runChangedTests", "title": "Glade: Run Local Changed Tests" },
    { "command": "glade.runFailedTests", "title": "Glade: Run Local Failed Tests" },
    { "command": "glade.startWatch", "title": "Glade: Start Local Watch" },
    { "command": "glade.stopWatch", "title": "Glade: Stop Local Watch" },
    { "command": "glade.createEnvironment", "title": "Glade: Create Local Data Environment" },
    { "command": "glade.switchEnvironment", "title": "Glade: Switch Local Data Environment" },
    { "command": "glade.seedLocalOrg", "title": "Glade: Seed Local Data Environment" },
    { "command": "glade.resetLocalOrg", "title": "Glade: Reset Local Data Environment" },
    { "command": "glade.exportLocalOrg", "title": "Glade: Export Local Data Environment" },
    { "command": "glade.inspectLocalOrg", "title": "Glade: Inspect Local Data Environment" },
    { "command": "glade.executeAnonymous", "title": "Glade: Execute Local Anonymous Apex" },
    { "command": "glade.debugAnonymous", "title": "Glade: Debug Local Anonymous Apex" }
  ]
}
```

Merge with the existing `languages`, `breakpoints`, and `debuggers` sections. Do not drop those existing contributions.

Do not contribute command titles beginning with `SFDX:` or generic command titles like `Run Test` and `Run All Tests`. Those belong to Salesforce extensions when present.

- [ ] **Step 2: Add icon**

Create `contrib/vscode-glade/media/glade.svg`:

```xml
<svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
  <path d="M4 4.5H20V19.5H4V4.5Z" stroke="#C5C5C5" stroke-width="1.8" rx="2"/>
  <path d="M15.8 8.2C14.9 7.4 13.8 7 12.5 7C9.7 7 7.7 9.1 7.7 12C7.7 14.9 9.8 17 12.6 17C14 17 15.2 16.5 16.1 15.7V12.4H12.5" stroke="#C5C5C5" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
</svg>
```

- [ ] **Step 3: Add shared tree item helper**

Create `contrib/vscode-glade/src/views/tree.ts`:

```ts
import * as vscode from "vscode";

export class GladeTreeItem extends vscode.TreeItem {
  constructor(
    label: string,
    collapsibleState: vscode.TreeItemCollapsibleState = vscode.TreeItemCollapsibleState.None,
  ) {
    super(label, collapsibleState);
  }
}

export function commandItem(label: string, command: string, tooltip?: string): GladeTreeItem {
  const item = new GladeTreeItem(label);
  item.command = { command, title: label };
  item.tooltip = tooltip || label;
  return item;
}
```

- [ ] **Step 4: Add Project view provider**

Create `contrib/vscode-glade/src/views/projectView.ts`:

```ts
import * as vscode from "vscode";
import { GladeProjectContext } from "../projectModel";
import { GladeTreeItem, commandItem } from "./tree";

export class ProjectView implements vscode.TreeDataProvider<GladeTreeItem> {
  private project?: GladeProjectContext;
  private readonly changed = new vscode.EventEmitter<GladeTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData = this.changed.event;

  setProject(project: GladeProjectContext | undefined): void {
    this.project = project;
    this.changed.fire();
  }

  getTreeItem(element: GladeTreeItem): vscode.TreeItem {
    return element;
  }

  getChildren(): GladeTreeItem[] {
    if (!this.project) {
      return [commandItem("Open an SFDX project", "vscode.openFolder", "Open a folder containing sfdx-project.json")];
    }
    const root = new GladeTreeItem(this.project.projectRoot);
    root.description = "SFDX root";
    const namespace = new GladeTreeItem(`Namespace: ${this.project.namespace || "(none)"}`);
    const api = new GladeTreeItem(`Source API: ${this.project.sourceApiVersion || "(unknown)"}`);
    const dirs = new GladeTreeItem(`Package dirs: ${this.project.packageDirs.join(", ") || "."}`);
    return [root, namespace, api, dirs, commandItem("Doctor", "glade.doctor"), commandItem("Refresh", "glade.refresh")];
  }
}
```

- [ ] **Step 5: Add placeholder providers for runs, local org, debug**

Create `runsView.ts`, `localOrgView.ts`, and `debugView.ts` with providers returning concrete command rows. Use this exact shape for `runsView.ts`:

```ts
import * as vscode from "vscode";
import { GladeTreeItem, commandItem } from "./tree";

export class RunsView implements vscode.TreeDataProvider<GladeTreeItem> {
  private readonly changed = new vscode.EventEmitter<GladeTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData = this.changed.event;

  refresh(): void {
    this.changed.fire();
  }

  getTreeItem(element: GladeTreeItem): vscode.TreeItem {
    return element;
  }

  getChildren(): GladeTreeItem[] {
    return [
      commandItem("Run changed since origin/main", "glade.runChangedTests"),
      commandItem("Run failed tests", "glade.runFailedTests"),
      commandItem("Start watch", "glade.startWatch"),
    ];
  }
}
```

For `environmentsView.ts`, return `Active: dev`, `Create environment`, `Switch environment`, `Seed active environment`, `Reset active environment`, and `Export active environment`.

For `localOrgView.ts`, return `Inspect active environment`, `Seed active environment`, `Reset active environment`, and `Export active environment`.

For `debugView.ts`, return `Breakpoints: 0`, `Debug selected Apex`, `Debug current test`, `Open last debug log`, `Analyze trace/profile`.

- [ ] **Step 6: Register views in activation**

In `extension.ts`, construct and register providers:

```ts
const projectView = new ProjectView();
const runsView = new RunsView();
const environmentsView = new EnvironmentsView();
const localOrgView = new LocalOrgView();
const debugView = new DebugView();
context.subscriptions.push(
  vscode.window.registerTreeDataProvider("glade.project", projectView),
  vscode.window.registerTreeDataProvider("glade.recommendedRuns", runsView),
  vscode.window.registerTreeDataProvider("glade.environments", environmentsView),
  vscode.window.registerTreeDataProvider("glade.localOrg", localOrgView),
  vscode.window.registerTreeDataProvider("glade.debugLogs", debugView),
);
```

After project detection resolves, call `projectView.setProject(project)`.

- [ ] **Step 7: Verify and commit**

Run:

```bash
cd contrib/vscode-glade
npm test
npm run package
```

Expected: compile, tests, and VSIX packaging pass.

Commit:

```bash
git add contrib/vscode-glade
git commit -m "feat: add Glade VS Code sidebar"
```

---

## Squad 4: Native Test Explorer

**Files:**

- Create: `contrib/vscode-glade/src/tests/model.ts`
- Create: `contrib/vscode-glade/src/tests/discovery.ts`
- Create: `contrib/vscode-glade/src/tests/runner.ts`
- Create: `contrib/vscode-glade/src/tests/controller.ts`
- Create: `contrib/vscode-glade/src/tests/watch.ts`
- Create: `contrib/vscode-glade/src/testResults.ts`
- Create: `contrib/vscode-glade/test/testDiscovery.test.js`
- Create: `contrib/vscode-glade/test/testResults.test.js`
- Modify: `contrib/vscode-glade/src/extension.ts`
- Modify: `contrib/vscode-glade/package.json`
- Test: `cd contrib/vscode-glade && npm test`

**Agent prompt:**

```text
You are Squad 4. Build native VS Code Test Explorer support.
Discover Apex test classes and methods from workspace files.
Run class and method tests with glade test --json.
Run changed tests with glade test changed --since origin/main --json.
Map JSON results into Test Explorer states and diagnostics.
Keep watch mode as a command that parses NDJSON summary events.
Do not edit LSP or sidebar view files except activation registration.
Run npm test.
Return sample command lines and a sample parsed failure.
```

- [ ] **Step 1: Add failing tests for discovery and result parsing**

Create `contrib/vscode-glade/test/testDiscovery.test.js`:

```js
const assert = require("assert");
const discovery = require("../out/tests/discovery");

const source = `
@IsTest
private class AccountServiceTest {
  @IsTest
  static void createsContact() {}

  testMethod static void legacyMethod() {}
}
`;

const tests = discovery.discoverApexTests("force-app/classes/AccountServiceTest.cls", source);
assert.deepStrictEqual(tests, {
  className: "AccountServiceTest",
  methods: ["createsContact", "legacyMethod"],
});
```

Create `contrib/vscode-glade/test/testResults.test.js`:

```js
const assert = require("assert");
const results = require("../out/testResults");

const run = {
  summary: { total: 1, passed: 0, failed: 1, skipped: 0, compileErrors: 0, runtimeErrors: 0, unsupported: 0, errors: 1, durationMs: 97 },
  suites: [{
    name: "AccountServiceTest",
    cases: [{
      className: "AccountServiceTest",
      methodName: "createsContact",
      status: "fail",
      durationMs: 97,
      problem: {
        message: "expected Pond Supply, actual Pond Supply Primary",
        stack: [{ file: "/repo/force-app/classes/AccountServiceTest.cls", line: 8, column: 5 }],
      },
    }],
  }],
};

const flattened = results.flattenTestCases(run);
assert.strictEqual(flattened.length, 1);
assert.strictEqual(flattened[0].id, "AccountServiceTest.createsContact");
assert.strictEqual(flattened[0].message, "expected Pond Supply, actual Pond Supply Primary");
assert.strictEqual(flattened[0].file, "/repo/force-app/classes/AccountServiceTest.cls");
assert.strictEqual(flattened[0].line, 8);
```

Add these files to `npm test` script.

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: FAIL because modules do not exist.

- [ ] **Step 2: Implement test discovery**

Create `contrib/vscode-glade/src/tests/discovery.ts`:

```ts
export interface DiscoveredApexTestClass {
  className: string;
  methods: string[];
}

export function discoverApexTests(_file: string, source: string): DiscoveredApexTestClass | undefined {
  if (!/@isTest\b/i.test(source) && !/\btestMethod\b/i.test(source)) {
    return undefined;
  }
  const classMatch = source.match(/\bclass\s+([A-Za-z_][A-Za-z0-9_]*)\b/);
  if (!classMatch) {
    return undefined;
  }
  const methods = new Set<string>();
  const isTestMethod = /@isTest[\s\S]{0,240}?\bstatic\s+void\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(/gi;
  for (const match of source.matchAll(isTestMethod)) {
    methods.add(match[1]);
  }
  const legacyMethod = /\btestMethod\s+static\s+void\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(/gi;
  for (const match of source.matchAll(legacyMethod)) {
    methods.add(match[1]);
  }
  return { className: classMatch[1], methods: [...methods].sort() };
}
```

- [ ] **Step 3: Implement result flattening**

Create `contrib/vscode-glade/src/testResults.ts`:

```ts
import { GladeTestRun } from "./tests/model";

export interface FlatTestCaseResult {
  id: string;
  className: string;
  methodName: string;
  status: string;
  durationMs: number;
  message?: string;
  file?: string;
  line?: number;
  column?: number;
}

export function flattenTestCases(run: GladeTestRun): FlatTestCaseResult[] {
  const cases: FlatTestCaseResult[] = [];
  for (const suite of run.suites || []) {
    for (const testCase of suite.cases || []) {
      const className = testCase.className || suite.name;
      const methodName = testCase.methodName || testCase.name || "";
      const frame = testCase.problem?.stack?.find((candidate) => candidate.file) || testCase.problem?.stack?.[0];
      cases.push({
        id: methodName ? `${className}.${methodName}` : className,
        className,
        methodName,
        status: testCase.status,
        durationMs: testCase.durationMs || 0,
        message: testCase.problem?.message,
        file: frame?.file,
        line: frame?.line,
        column: frame?.column,
      });
    }
  }
  return cases;
}
```

Create `src/tests/model.ts` with the shared interfaces from the Shared Types section.

- [ ] **Step 4: Implement runner**

Create `contrib/vscode-glade/src/tests/runner.ts`:

```ts
import * as vscode from "vscode";
import { runGladeJSON } from "../gladeCli";
import { GladeProjectContext } from "../projectModel";
import { GladeTestRun } from "./model";

export async function runApexTest(
  project: GladeProjectContext,
  className?: string,
  methodName?: string,
): Promise<GladeTestRun> {
  const args = ["test", "--project", project.projectRoot, "--json"];
  if (className) {
    args.push("--class", className);
  }
  if (methodName) {
    args.push("--method", methodName);
  }
  return runGladeJSON<GladeTestRun>(args, { cwd: project.projectRoot }, "glade test");
}

export async function runChangedTests(project: GladeProjectContext, since = "origin/main"): Promise<GladeTestRun> {
  return runGladeJSON<GladeTestRun>(
    ["test", "changed", "--project", project.projectRoot, "--since", since, "--json"],
    { cwd: project.projectRoot },
    "glade test changed",
  );
}

export function testUri(file: string): vscode.Uri {
  return vscode.Uri.file(file);
}
```

- [ ] **Step 5: Implement controller**

Create `contrib/vscode-glade/src/tests/controller.ts`:

```ts
import * as fs from "fs";
import * as vscode from "vscode";
import { flattenTestCases } from "../testResults";
import { GladeProjectContext } from "../projectModel";
import { discoverApexTests } from "./discovery";
import { runApexTest, runChangedTests } from "./runner";

interface TestData {
  className: string;
  methodName?: string;
}

export class GladeTestController {
  readonly controller = vscode.tests.createTestController("gladeApexTests", "Glade Apex");
  private project?: GladeProjectContext;

  constructor(private readonly context: vscode.ExtensionContext) {
    context.subscriptions.push(this.controller);
    this.controller.refreshHandler = () => this.discover();
    this.controller.createRunProfile("Run", vscode.TestRunProfileKind.Run, (request, token) => this.run(request, token), true);
    this.controller.createRunProfile("Debug", vscode.TestRunProfileKind.Debug, (request) => this.debug(request), true);
  }

  setProject(project: GladeProjectContext | undefined): void {
    this.project = project;
    void this.discover();
  }

  async discover(): Promise<void> {
    if (!this.project) {
      this.controller.items.replace([]);
      return;
    }
    const pattern = new vscode.RelativePattern(this.project.projectRoot, "**/*.{cls,trigger}");
    const files = await vscode.workspace.findFiles(pattern, "**/{node_modules,.sfdx,.sf,.glade}/**");
    const items: vscode.TestItem[] = [];
    for (const file of files) {
      const source = fs.readFileSync(file.fsPath, "utf8");
      const discovered = discoverApexTests(file.fsPath, source);
      if (!discovered) {
        continue;
      }
      const classItem = this.controller.createTestItem(discovered.className, discovered.className, file);
      classItem.canResolveChildren = false;
      (classItem as vscode.TestItem & { data?: TestData }).data = { className: discovered.className };
      for (const method of discovered.methods) {
        const methodItem = this.controller.createTestItem(`${discovered.className}.${method}`, method, file);
        (methodItem as vscode.TestItem & { data?: TestData }).data = { className: discovered.className, methodName: method };
        classItem.children.add(methodItem);
      }
      items.push(classItem);
    }
    this.controller.items.replace(items);
  }

  async run(request: vscode.TestRunRequest, token: vscode.CancellationToken): Promise<void> {
    if (!this.project) {
      return;
    }
    const run = this.controller.createTestRun(request);
    const queue = request.include && request.include.length > 0 ? request.include : [...this.controller.items];
    for (const item of queue) {
      if (token.isCancellationRequested) {
        break;
      }
      await this.runItem(run, item);
    }
    run.end();
  }

  private async runItem(run: vscode.TestRun, item: vscode.TestItem): Promise<void> {
    const data = (item as vscode.TestItem & { data?: TestData }).data;
    if (!data || !this.project) {
      return;
    }
    run.started(item);
    const result = await runApexTest(this.project, data.className, data.methodName);
    const flat = flattenTestCases(result);
    const failed = flat.find((candidate) => candidate.status !== "pass");
    if (failed) {
      const message = new vscode.TestMessage(failed.message || failed.status);
      if (failed.file && failed.line) {
        const uri = vscode.Uri.file(failed.file);
        const line = Math.max(0, failed.line - 1);
        const column = Math.max(0, (failed.column || 1) - 1);
        message.location = new vscode.Location(uri, new vscode.Position(line, column));
      }
      run.failed(item, message, result.summary.durationMs);
    } else {
      run.passed(item, result.summary.durationMs);
    }
  }

  private async debug(request: vscode.TestRunRequest): Promise<void> {
    const first = request.include?.[0];
    if (!first) {
      return;
    }
    await vscode.commands.executeCommand("glade.debugTestItem", first);
  }

  async runChanged(): Promise<void> {
    if (!this.project) {
      return;
    }
    const request = new vscode.TestRunRequest();
    const run = this.controller.createTestRun(request, "Glade changed tests");
    const result = await runChangedTests(this.project);
    const flat = flattenTestCases(result);
    for (const testCase of flat) {
      const item = this.controller.items.get(testCase.className)?.children.get(testCase.id);
      if (!item) {
        continue;
      }
      if (testCase.status === "pass") {
        run.passed(item, testCase.durationMs);
      } else {
        run.failed(item, new vscode.TestMessage(testCase.message || testCase.status), testCase.durationMs);
      }
    }
    run.end();
  }
}
```

- [ ] **Step 6: Register commands**

In `extension.ts`:

```ts
const tests = new GladeTestController(context);
context.subscriptions.push(
  vscode.commands.registerCommand("glade.runChangedTests", () => tests.runChanged()),
);
```

When project context resolves, call `tests.setProject(project)`.

- [ ] **Step 7: Verify and commit**

Run:

```bash
cd contrib/vscode-glade
npm test
npm run package
```

Expected: compile and tests pass. Manual smoke in Extension Development Host should show `Glade Apex` in Testing and the Glade sidebar.

Commit:

```bash
git add contrib/vscode-glade
git commit -m "feat: add Glade Apex Test Explorer"
```

---

## Squad 5: LSP Client, Default-Off Beside Salesforce

**Files:**

- Modify: `contrib/vscode-glade/package.json`
- Create: `contrib/vscode-glade/src/lsp.ts`
- Modify: `contrib/vscode-glade/src/extension.ts`
- Test: `cd contrib/vscode-glade && npm install && npm test && npm run package`

**Agent prompt:**

```text
You are Squad 5. Wire glade lsp into the VS Code extension without blocking the test-drive workflow.
Use vscode-languageclient.
Start glade lsp --project <root> only when Glade LSP is enabled.
Default Glade LSP off when Salesforce Apex language extensions are installed. Do not double-publish diagnostics by default.
If time is short, land the setting, dependency, and idle message first; full LSP lifecycle can wait behind the test-drive path.
Stop and restart when project context changes.
Keep LSP lifecycle in src/lsp.ts only.
Do not edit Testing API files.
Run npm test and npm run package.
Return the final package dependency diff.
```

- [ ] **Step 1: Add dependency**

Run:

```bash
cd contrib/vscode-glade
npm install vscode-languageclient
```

Expected: `package.json` and `package-lock.json` gain `vscode-languageclient`.

- [ ] **Step 2: Create LSP lifecycle**

Create `contrib/vscode-glade/src/lsp.ts`:

```ts
import * as vscode from "vscode";
import { LanguageClient, LanguageClientOptions, ServerOptions } from "vscode-languageclient/node";
import { GladeProjectContext } from "./projectModel";

export class GladeLanguageClient {
  private client?: LanguageClient;

  async start(project: GladeProjectContext): Promise<void> {
    await this.stop();
    const serverOptions: ServerOptions = {
      command: "glade",
      args: ["lsp", "--project", project.projectRoot],
      options: { cwd: project.projectRoot },
    };
    const clientOptions: LanguageClientOptions = {
      documentSelector: [
        { scheme: "file", language: "apex" },
        { scheme: "file", pattern: `${project.projectRoot.replace(/\\/g, "/")}/**/*.{cls,trigger}` },
      ],
      synchronize: {
        fileEvents: vscode.workspace.createFileSystemWatcher("**/*.{cls,trigger}"),
      },
    };
    this.client = new LanguageClient("gladeLsp", "Glade Apex", serverOptions, clientOptions);
    await this.client.start();
  }

  async stop(): Promise<void> {
    if (!this.client) {
      return;
    }
    const current = this.client;
    this.client = undefined;
    await current.stop();
  }

  dispose(): void {
    void this.stop();
  }
}
```

- [ ] **Step 3: Register in activation**

In `extension.ts`:

```ts
const lsp = new GladeLanguageClient();
context.subscriptions.push({ dispose: () => lsp.dispose() });
```

When project context resolves:

```ts
const enableLspSetting = vscode.workspace.getConfiguration("glade").get<boolean | undefined>("enableLsp");
const salesforceOwnsApexLanguage =
  project?.salesforceExtensions.apex ||
  project?.salesforceExtensions.apexLanguageServerTypescript;
const enableGladeLsp = enableLspSetting ?? !salesforceOwnsApexLanguage;
if (project && enableGladeLsp) {
  void lsp.start(project).catch((error: Error) => {
    output.logs.appendLine(`glade lsp failed: ${error.message}`);
  });
} else if (project && salesforceOwnsApexLanguage) {
  output.logs.appendLine("Glade LSP is idle because Salesforce Apex language support is installed. Set glade.enableLsp=true to enable local Glade diagnostics.");
}
```

- [ ] **Step 4: Verify**

Run:

```bash
cd contrib/vscode-glade
npm test
npm run package
```

Expected: compile and tests pass.

Manual smoke:

```bash
cd /Users/matt/Dev/glade
code contrib/vscode-glade
```

Run `Launch Extension`, open an SFDX test project, open a `.cls` file, and confirm one of these exact outcomes:

- Salesforce Apex language support is installed: Glade writes the idle message and does not start `glade lsp`.
- `glade.enableLsp=true`: hover/diagnostics come from `glade lsp`.

- [ ] **Step 5: Commit**

```bash
git add contrib/vscode-glade/package.json contrib/vscode-glade/package-lock.json contrib/vscode-glade/src
git commit -m "feat: start Glade LSP from VS Code"
```

---

## Squad 6: DAP And Editor Commands

**Files:**

- Modify: `contrib/vscode-glade/src/adapter.ts`
- Modify: `contrib/vscode-glade/src/commandModel.ts`
- Modify: `contrib/vscode-glade/src/commands.ts`
- Create: `contrib/vscode-glade/src/debug.ts`
- Create or modify: `contrib/vscode-glade/src/breakpoints.ts`
- Modify: `contrib/vscode-glade/test/commands.test.js`
- Test: `cd contrib/vscode-glade && npm test`

**Agent prompt:**

```text
You are Squad 6. Improve debug commands.
Keep the existing glade DAP adapter. Add commands for debug current test, debug current class, and debug selected Apex.
Make DAP config generation pure and covered by tests.
Use VS Code's existing Apex breakpoints. Do not add custom breakpoint decorations or a second gutter.
Do not change product DAP Go code.
Run npm test.
Return the launch JSON shapes you produce.
```

- [ ] **Step 1: Add command model tests**

Append to `contrib/vscode-glade/test/commands.test.js`:

```js
assert.deepStrictEqual(
  commands.debugProgramConfig("/repo", "AccountServiceTest.createsContact"),
  {
    type: "glade",
    request: "launch",
    name: "Glade: Debug AccountServiceTest.createsContact",
    project: "/repo",
    program: "AccountServiceTest.createsContact",
  }
);

assert.deepStrictEqual(
  commands.testProgram("AccountServiceTest", "createsContact"),
  "AccountServiceTest.createsContact"
);

assert.deepStrictEqual(
  commands.testProgram("AccountServiceTest", undefined),
  "AccountServiceTest"
);
```

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: FAIL because functions do not exist.

- [ ] **Step 2: Extend command model**

In `commandModel.ts`, add:

```ts
export function testProgram(className: string, methodName?: string): string {
  return methodName ? `${className}.${methodName}` : className;
}

export function debugProgramConfig(project: string | undefined, program: string): GladeDebugConfiguration {
  return {
    type: "glade",
    request: "launch",
    name: `Glade: Debug ${program}`,
    project,
    program,
  };
}
```

Change `GladeDebugConfig` to allow either `source` or `program`:

```ts
export interface GladeDebugConfig {
  type: "glade";
  request: "launch";
  name: string;
  project?: string;
  source?: string;
  program?: string;
}
```

- [ ] **Step 3: Add debug helper**

Create `contrib/vscode-glade/src/debug.ts`:

```ts
import * as vscode from "vscode";
import { debugProgramConfig, testProgram } from "./commandModel";
import { GladeProjectContext } from "./projectModel";

export interface TestItemData {
  className: string;
  methodName?: string;
}

export async function debugTestData(project: GladeProjectContext | undefined, data: TestItemData): Promise<void> {
  const folder = project ? vscode.workspace.getWorkspaceFolder(vscode.Uri.file(project.projectRoot)) : undefined;
  const program = testProgram(data.className, data.methodName);
  await vscode.debug.startDebugging(folder, debugProgramConfig(project?.projectRoot, program));
}
```

- [ ] **Step 4: Register commands**

In `commands.ts`, add a command:

```ts
vscode.commands.registerCommand("glade.debugTestItem", async (item: vscode.TestItem) => {
  const data = (item as vscode.TestItem & { data?: { className: string; methodName?: string } }).data;
  if (!data) {
    void vscode.window.showErrorMessage("Glade test item has no Apex class metadata.");
    return;
  }
  const folder = vscode.workspace.workspaceFolders?.[0];
  await vscode.debug.startDebugging(folder, debugProgramConfig(folder?.uri.fsPath, testProgram(data.className, data.methodName)));
});
```

Keep `glade.debugAnonymous` behavior and use the active project context. Do not pass `--db` to DAP until Squad 6A lands that CLI contract. VS Code sends source breakpoints to the debug adapter through the normal DAP `setBreakpoints` flow after `startDebugging`; this squad should not serialize breakpoints into launch config.

- [ ] **Step 5: Update package commands**

Add command contribution:

```json
{ "command": "glade.debugTestItem", "title": "Glade: Debug Test" }
```

- [ ] **Step 6: Add breakpoint summary hook**

If `src/breakpoints.ts` does not exist yet, create a small VS Code wrapper:

```ts
import * as vscode from "vscode";

export function apexBreakpointCount(): number {
  return vscode.debug.breakpoints
    .filter((breakpoint): breakpoint is vscode.SourceBreakpoint => breakpoint instanceof vscode.SourceBreakpoint)
    .filter((breakpoint) => /\.(cls|trigger)$/i.test(breakpoint.location.uri.fsPath))
    .length;
}
```

Use it only for UI state and command preflight messages. The debug adapter remains the source of execution behavior.

- [ ] **Step 7: Verify and commit**

Run:

```bash
cd contrib/vscode-glade
npm test
npm run package
```

Expected: tests and package pass.

Commit:

```bash
git add contrib/vscode-glade
git commit -m "feat: add Apex test debug launches"
```

---

## Squad 7: Local Data Environments, Local Org, And Debug/Log Views

**Files:**

- Create: `contrib/vscode-glade/src/environments.ts`
- Create: `contrib/vscode-glade/src/localOrgModel.ts`
- Create: `contrib/vscode-glade/src/localOrg.ts`
- Create: `contrib/vscode-glade/src/breakpoints.ts`
- Modify: `contrib/vscode-glade/src/commandModel.ts`
- Modify: `contrib/vscode-glade/src/views/environmentsView.ts`
- Modify: `contrib/vscode-glade/src/views/localOrgView.ts`
- Modify: `contrib/vscode-glade/src/views/debugView.ts`
- Modify: `contrib/vscode-glade/src/extension.ts`
- Create: `contrib/vscode-glade/test/environments.test.js`
- Create: `contrib/vscode-glade/test/localOrg.test.js`
- Modify: `contrib/vscode-glade/test/commands.test.js`
- Modify: `contrib/vscode-glade/package.json`
- Test: `cd contrib/vscode-glade && npm test`

**Agent prompt:**

```text
You are Squad 7. Build named local data environments and the first real local org cockpit.
Use glade db inspect --db <path> --project <root> --json and the current byObject JSON shape.
Default to one environment named dev at <project>/.glade/envs/dev.sqlite.
Let users create, switch, seed, reset, export, and inspect environments from VS Code.
Make Glade anonymous Apex use the active environment by passing --project and --db.
Show active VS Code Apex breakpoints in Debug And Logs. Do not create a custom breakpoint gutter.
Do not add a full SOQL editor or record grid in this task.
Run npm test and npm run package.
Return the settings keys, command ids, and one sample inspect JSON payload.
```

- [ ] **Step 1: Add configuration**

In `package.json`, add or merge these settings under `contributes.configuration.properties`:

```json
"glade.environments": {
  "type": "array",
  "default": [],
  "description": "Named local Glade data environments. Empty means a dev environment at .glade/envs/dev.sqlite.",
  "items": {
    "type": "object",
    "required": ["name", "dbPath"],
    "properties": {
      "name": { "type": "string" },
      "dbPath": { "type": "string" },
      "fixturePath": { "type": "string" }
    }
  }
},
"glade.activeEnvironment": {
  "type": "string",
  "default": "dev",
  "description": "Name of the active Glade local data environment."
},
"glade.changedSince": {
  "type": "string",
  "default": "origin/main",
  "description": "Git ref used by Glade changed-test runs."
},
"glade.enableLsp": {
  "type": "boolean",
  "description": "Enable Glade LSP diagnostics and editor features."
},
"glade.enableCodeLens": {
  "type": "boolean",
  "default": true,
  "description": "Show Glade Local CodeLens commands beside Apex tests."
},
"glade.enableTestExplorer": {
  "type": "boolean",
  "default": true,
  "description": "Show Glade Apex in the VS Code Testing view."
},
"glade.enableSidebar": {
  "type": "boolean",
  "default": true,
  "description": "Show the Glade Activity Bar views."
}
```

Remove any new plan work that depends on a single `glade.dbPath`. That setting can remain as a temporary compatibility alias only if existing released users already have it. New code must use `glade.environments` and `glade.activeEnvironment`.

- [ ] **Step 2: Add failing environment tests**

Create `contrib/vscode-glade/test/environments.test.js`:

```js
const assert = require("assert");
const envs = require("../out/environments");

assert.deepStrictEqual(envs.normalizeEnvironments([], "/repo"), [
  { name: "dev", dbPath: "/repo/.glade/envs/dev.sqlite" },
]);

assert.deepStrictEqual(
  envs.normalizeEnvironments([{ name: "qa", dbPath: ".glade/envs/qa.sqlite", fixturePath: "data/qa.json" }], "/repo"),
  [{ name: "qa", dbPath: "/repo/.glade/envs/qa.sqlite", fixturePath: "/repo/data/qa.json" }],
);

assert.deepStrictEqual(
  envs.activeEnvironment("qa", [
    { name: "dev", dbPath: "/repo/.glade/envs/dev.sqlite" },
    { name: "qa", dbPath: "/repo/.glade/envs/qa.sqlite" },
  ]),
  { name: "qa", dbPath: "/repo/.glade/envs/qa.sqlite" },
);

assert.strictEqual(envs.environmentNameFromInput("  feature/foo  "), "feature-foo");
assert.throws(() => envs.environmentNameFromInput(""), /environment name is required/);
```

Add `node test/environments.test.js` to the `npm test` script.

Run:

```bash
cd contrib/vscode-glade
npm test
```

Expected: FAIL because `environments.ts` does not exist.

- [ ] **Step 3: Implement pure environment model**

Create `contrib/vscode-glade/src/environments.ts`:

```ts
import * as path from "path";

export interface GladeEnvironment {
  name: string;
  dbPath: string;
  fixturePath?: string;
}

export function defaultEnvironment(projectRoot: string): GladeEnvironment {
  return { name: "dev", dbPath: path.join(projectRoot, ".glade", "envs", "dev.sqlite") };
}

export function normalizeEnvironments(raw: GladeEnvironment[] | undefined, projectRoot: string): GladeEnvironment[] {
  const source = raw && raw.length > 0 ? raw : [defaultEnvironment(projectRoot)];
  return source.map((entry) => ({
    name: environmentNameFromInput(entry.name),
    dbPath: absolutePath(entry.dbPath, projectRoot),
    fixturePath: entry.fixturePath ? absolutePath(entry.fixturePath, projectRoot) : undefined,
  }));
}

export function activeEnvironment(activeName: string | undefined, environments: GladeEnvironment[]): GladeEnvironment {
  const wanted = activeName || "dev";
  return environments.find((entry) => entry.name === wanted) || environments[0] || defaultEnvironment(".");
}

export function environmentNameFromInput(input: string): string {
  const name = input.trim().replace(/[^A-Za-z0-9_.-]+/g, "-").replace(/^-+|-+$/g, "");
  if (!name) {
    throw new Error("environment name is required");
  }
  return name;
}

function absolutePath(value: string, projectRoot: string): string {
  return path.isAbsolute(value) ? value : path.join(projectRoot, value);
}
```

Keep this module free of `vscode` imports so fast tests stay plain Node.

- [ ] **Step 4: Add failing local org parser test**

Create `contrib/vscode-glade/test/localOrg.test.js`:

```js
const assert = require("assert");
const localOrg = require("../out/localOrgModel");

const rows = localOrg.objectRowsFromInspect({
  path: "/repo/.glade/envs/dev.sqlite",
  schemaVersion: 1,
  objects: 2,
  records: 46,
  byObject: { Contact: 34, Account: 12 },
  users: 1,
  profiles: 1,
  permissions: 0,
});

assert.deepStrictEqual(rows, [
  { name: "Account", rows: 12 },
  { name: "Contact", rows: 34 },
]);

assert.deepStrictEqual(localOrg.summaryFromInspect({ objects: 2, records: 46, byObject: {} }), {
  objects: 2,
  records: 46,
  users: 0,
  profiles: 0,
  permissions: 0,
});
```

Add `node test/localOrg.test.js` to the `npm test` script.

- [ ] **Step 5: Implement pure local org parser**

Create `contrib/vscode-glade/src/localOrgModel.ts`:

```ts
export interface DBInspectResult {
  path?: string;
  schemaVersion?: number;
  objects: number;
  records: number;
  byObject?: Record<string, number>;
  users?: number;
  profiles?: number;
  permissions?: number;
}

export interface LocalOrgObjectRow {
  name: string;
  rows: number;
}

export interface LocalOrgSummary {
  objects: number;
  records: number;
  users: number;
  profiles: number;
  permissions: number;
}

export function objectRowsFromInspect(result: DBInspectResult): LocalOrgObjectRow[] {
  return Object.entries(result.byObject || {})
    .map(([name, rows]) => ({ name, rows }))
    .sort((a, b) => a.name.localeCompare(b.name));
}

export function summaryFromInspect(result: DBInspectResult): LocalOrgSummary {
  return {
    objects: result.objects || 0,
    records: result.records || 0,
    users: result.users || 0,
    profiles: result.profiles || 0,
    permissions: result.permissions || 0,
  };
}
```

- [ ] **Step 6: Make anonymous exec use the active environment**

Extend `contrib/vscode-glade/test/commands.test.js`:

```js
assert.deepStrictEqual(
  commands.execAnonymousArgs("System.debug('hi');", "/repo", "/repo/.glade/envs/dev.sqlite"),
  ["exec", "--debug-log", "-", "--project", "/repo", "--db", "/repo/.glade/envs/dev.sqlite", "System.debug('hi');"],
);
```

Update `commandModel.ts`:

```ts
export function execAnonymousArgs(source: string, projectRoot?: string, dbPath?: string): string[] {
  const args = ["exec", "--debug-log", "-"];
  if (projectRoot) {
    args.push("--project", projectRoot);
  }
  if (dbPath) {
    args.push("--db", dbPath);
  }
  args.push(source);
  return args;
}
```

Update `commands.ts` so `glade.executeAnonymous` resolves the active environment and passes the active DB path. Leave `glade.debugAnonymous` on project-only DAP until Squad 6A adds `glade dap --db`.

- [ ] **Step 7: Implement VS Code environment services**

Create `contrib/vscode-glade/src/localOrg.ts`:

```ts
import * as vscode from "vscode";
import { runGladeJSON } from "./gladeCli";
import { activeEnvironment, GladeEnvironment, normalizeEnvironments } from "./environments";
import { DBInspectResult, LocalOrgObjectRow, objectRowsFromInspect } from "./localOrgModel";
import { GladeProjectContext } from "./projectModel";

export function configuredEnvironments(project: GladeProjectContext): GladeEnvironment[] {
  const config = vscode.workspace.getConfiguration("glade");
  const raw = config.get<GladeEnvironment[]>("environments") || [];
  return normalizeEnvironments(raw, project.projectRoot);
}

export function configuredActiveEnvironment(project: GladeProjectContext): GladeEnvironment {
  const config = vscode.workspace.getConfiguration("glade");
  return activeEnvironment(config.get<string>("activeEnvironment") || "dev", configuredEnvironments(project));
}

export async function inspectLocalOrg(project: GladeProjectContext, environment = configuredActiveEnvironment(project)): Promise<DBInspectResult> {
  return runGladeJSON<DBInspectResult>(
    ["db", "inspect", "--db", environment.dbPath, "--project", project.projectRoot, "--json"],
    { cwd: project.projectRoot },
    "glade db inspect",
  );
}

export async function inspectLocalOrgRows(project: GladeProjectContext, environment = configuredActiveEnvironment(project)): Promise<LocalOrgObjectRow[]> {
  return objectRowsFromInspect(await inspectLocalOrg(project, environment));
}
```

Seed, reset, and export can use terminals because they are operator actions with visible file paths:

```ts
export function sendLocalOrgTerminal(command: string): void {
  const terminal = vscode.window.createTerminal("Glade Local Data");
  terminal.show();
  terminal.sendText(command);
}
```

- [ ] **Step 8: Build Data Environments view**

Modify `environmentsView.ts` to show:

- `Active: dev` with the active DB path as description.
- One row per configured environment.
- Command rows: `Create Environment`, `Switch Environment`, `Seed Active Environment`, `Reset Active Environment`, `Export Active Environment`.

Register commands in `extension.ts`:

```ts
vscode.commands.registerCommand("glade.createEnvironment", async () => { /* prompt name, update glade.environments, switch active */ });
vscode.commands.registerCommand("glade.switchEnvironment", async () => { /* quick pick, update glade.activeEnvironment */ });
vscode.commands.registerCommand("glade.seedLocalOrg", async () => { /* open file, send glade db seed --db ... */ });
vscode.commands.registerCommand("glade.resetLocalOrg", async () => { /* confirm, send glade db reset --db ... --json */ });
vscode.commands.registerCommand("glade.exportLocalOrg", async () => { /* prompt save path, send glade db export --db ... */ });
vscode.commands.registerCommand("glade.inspectLocalOrg", async () => { /* refresh Local Org view from db inspect */ });
```

Use workspace-level configuration writes:

```ts
await vscode.workspace.getConfiguration("glade").update("activeEnvironment", picked.name, vscode.ConfigurationTarget.Workspace);
```

- [ ] **Step 9: Feed Local Org view**

Modify `localOrgView.ts` so it displays both summary and object rows:

```ts
setInspect(result: DBInspectResult): void {
  this.summary = summaryFromInspect(result);
  this.rows = objectRowsFromInspect(result);
  this.changed.fire();
}
```

When no DB has been inspected, show command rows:

- `Inspect Active Environment`
- `Seed Active Environment`
- `Reset Active Environment`
- `Export Active Environment`

When data is present, show summary rows first, then object row counts.

- [ ] **Step 10: Show real VS Code breakpoints in Debug And Logs**

Create `contrib/vscode-glade/src/breakpoints.ts`:

```ts
import * as vscode from "vscode";

export interface GladeBreakpointSummary {
  file: string;
  line: number;
  enabled: boolean;
}

export function apexBreakpoints(): GladeBreakpointSummary[] {
  return vscode.debug.breakpoints
    .filter((breakpoint): breakpoint is vscode.SourceBreakpoint => breakpoint instanceof vscode.SourceBreakpoint)
    .filter((breakpoint) => /\.(cls|trigger)$/i.test(breakpoint.location.uri.fsPath))
    .map((breakpoint) => ({
      file: breakpoint.location.uri.fsPath,
      line: breakpoint.location.range.start.line + 1,
      enabled: breakpoint.enabled,
    }));
}
```

Modify `debugView.ts` to show:

- `Breakpoints: <count>`
- One row per Apex breakpoint with file basename and line.
- `Debug Current Test`
- `Debug Selected Apex`
- `Open Last Debug Log`

Register:

```ts
context.subscriptions.push(vscode.debug.onDidChangeBreakpoints(() => debugView.refresh()));
```

Do not draw custom breakpoint decorations. The existing `contributes.breakpoints` for language `apex` is the gutter contract.

- [ ] **Step 11: Verify and commit**

Run:

```bash
cd contrib/vscode-glade
npm test
npm run package
git diff --check
```

Expected: tests and package pass.

Commit:

```bash
git add contrib/vscode-glade
git commit -m "feat: manage Glade local data environments in VS Code"
```

---

## Squad 8: Docs, CI, And Final Integration

**Files:**

- Modify: `contrib/vscode-glade/README.md`
- Create: `.github/workflows/vscode-glade.yml`
- Modify: `docs/EDITOR.md`
- Modify: `site/docs-src/guide/editor.md`
- Modify: `docs/INSTALL.md`
- Test: `cd contrib/vscode-glade && npm test && npm run package`
- Test: `go test ./internal/gladecli`
- Test: `scripts/smoke.sh`

**Agent prompt:**

```text
You are Squad 8. Integrate and document the VS Code extension sprint.
Add VS Code extension CI.
Update README and editor docs to show the new easy install path and core workflows.
Remove IntelliJ docs if Squad 0 has not already done so.
Run npm test, npm run package, go test ./internal/gladecli, scripts/smoke.sh, and git diff --check.
Return pass/fail output for every gate.
```

- [ ] **Step 1: Add VS Code extension workflow**

Create `.github/workflows/vscode-glade.yml`:

```yaml
name: vscode-glade

on:
  push:
    paths:
      - "contrib/vscode-glade/**"
      - ".github/workflows/vscode-glade.yml"
  pull_request:
    paths:
      - "contrib/vscode-glade/**"
      - ".github/workflows/vscode-glade.yml"

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: "20"
          cache: npm
          cache-dependency-path: contrib/vscode-glade/package-lock.json
      - name: Install
        working-directory: contrib/vscode-glade
        run: npm ci
      - name: Test
        working-directory: contrib/vscode-glade
        run: npm test
      - name: Package
        working-directory: contrib/vscode-glade
        run: npm run package
```

- [ ] **Step 2: Rewrite `contrib/vscode-glade/README.md`**

Use this structure:

~~~markdown
# Glade Local Apex for VS Code

Run, test, debug, and inspect local Apex in an SFDX project without a Salesforce org login.

## Install From Glade

```bash
glade editor doctor vscode
glade editor install vscode --force
```

Use `--editor cursor` or `--editor windsurf` for compatible VS Code forks.

## What The Extension Adds

- Glade Activity Bar view.
- SFDX project detection from `sfdx-project.json`.
- Native Test Explorer for Apex classes and methods.
- Changed-test and failed-test commands.
- `glade lsp` diagnostics, symbols, hover, rename, references, and completion.
- `glade dap` debugging for anonymous Apex and tests with VS Code breakpoints.
- Named local data environments backed by SQLite.
- Local org inspection through `glade db inspect`.

## Develop

```bash
npm install
npm test
npm run package
```

Open `contrib/vscode-glade` in VS Code and run `Launch Extension`.
~~~

- [ ] **Step 3: Update editor docs**

In `docs/EDITOR.md` and `site/docs-src/guide/editor.md`, add a VS Code extension section before "VS Code Tasks":

~~~markdown
## VS Code Extension

Install the bundled Glade extension after installing the `glade` binary:

```bash
glade editor doctor vscode
glade editor install vscode --force
```

Open a folder with `sfdx-project.json`. The extension starts from that SFDX root and uses `glade config show --json` to read package directories, namespace, and API version. It adds a Glade Activity Bar view, native Apex Test Explorer entries, changed-test commands, `glade lsp` editor features, `glade dap` debug launches with VS Code breakpoints, named local data environments, and local org inspection.
~~~

Keep existing tasks and debug launch examples below this section.

- [ ] **Step 4: Update install docs**

In `docs/INSTALL.md`, add a short editor install subsection after binary install:

~~~markdown
### Install VS Code Extension

Release archives include the VS Code extension package. After the `glade` binary is on `PATH`, run:

```bash
glade editor install vscode --force
```

For Cursor or Windsurf:

```bash
glade editor install vscode --editor cursor --force
glade editor install vscode --editor windsurf --force
```
~~~

- [ ] **Step 5: Full verification**

Run:

```bash
cd /Users/matt/Dev/glade/contrib/vscode-glade
npm test
npm run package
cd /Users/matt/Dev/glade
go test ./internal/gladecli
scripts/smoke.sh
git diff --check
```

Expected: all pass.

- [ ] **Step 6: Manual smoke**

Use VS Code Extension Development Host:

1. Open `contrib/vscode-glade`.
2. Run `Launch Extension`.
3. In the Extension Development Host, open `internal/debuglog/testdata/project`.
4. Confirm the Glade Activity Bar appears.
5. Confirm Project view shows SFDX root and package dirs.
6. Confirm Testing view shows Apex tests.
7. Run one test from Test Explorer.
8. Debug selected anonymous Apex with `System.debug('hello');`.
9. Run `Glade: Run Changed Tests`.

Expected: no extension host errors; Glade output channels show command lines and results.

- [ ] **Step 7: Commit**

```bash
git add .github docs site contrib/vscode-glade
git commit -m "docs: document Glade VS Code extension workflow"
```

---

## Stretch Squad 9: CodeLens And Command Polish

**Files:**

- Create: `contrib/vscode-glade/src/codeLens.ts`
- Modify: `contrib/vscode-glade/src/extension.ts`
- Modify: `contrib/vscode-glade/src/commands.ts`
- Modify: `contrib/vscode-glade/package.json`
- Modify: `contrib/vscode-glade/test/testDiscovery.test.js`
- Test: `cd contrib/vscode-glade && npm test && npm run package`

**Agent prompt:**

```text
You are Stretch Squad 9. Add Glade CodeLens without stepping on Salesforce CodeLens.
Every visible label must include Local.
Use existing test discovery to place lenses above Apex test classes and methods.
Register only when glade.enableCodeLens is true.
Do not hide, replace, or rename Salesforce CodeLens.
Run npm test and npm run package.
Return the label list and one screenshot or manual smoke note.
```

- [ ] Add CodeLens provider for language `apex`.
- [ ] Add lenses: `Run Local Test`, `Debug Local Test`, `Run Local Changed Impact`.
- [ ] Route commands to `glade.runTestAtCursor`, `glade.debugTestAtCursor`, and `glade.runChangedTests`.
- [ ] Keep all command ids under `glade.*`.
- [ ] Verify with Salesforce extensions installed and disabled. Glade CodeLens must still read as the local lane.

## Stretch Squad 10: Environment Clone, Export, Import, And Diff

**Files:**

- Modify: `contrib/vscode-glade/src/environments.ts`
- Modify: `contrib/vscode-glade/src/localOrg.ts`
- Create: `contrib/vscode-glade/src/environmentDiff.ts`
- Modify: `contrib/vscode-glade/src/views/environmentsView.ts`
- Create: `contrib/vscode-glade/test/environmentDiff.test.js`
- Test: `cd contrib/vscode-glade && npm test && npm run package`

**Agent prompt:**

```text
You are Stretch Squad 10. Add useful data-environment workbench actions.
Implement clone, export, import-by-copy, seed history, and object-count diff.
Use file copy for SQLite clone/import. Use glade db export and glade db seed for terminal-backed data actions.
The diff compares inspect JSON byObject counts and total records. Do not build a record grid.
Run npm test and npm run package.
Return before/after inspect JSON for a cloned environment.
```

- [ ] Add pure `diffInspectResults(left, right)` with deltas for `records`, `objects`, and `byObject`.
- [ ] Add `glade.cloneEnvironment`: prompt source and target names, copy SQLite file, add the new environment, switch active.
- [ ] Add `glade.importEnvironmentDB`: prompt for `.sqlite`, copy it under `.glade/envs/<name>.sqlite`, add environment.
- [ ] Add `glade.diffEnvironments`: quick-pick two environments, run `glade db inspect` for both, show a text document with count deltas.
- [ ] Add seed history in workspace state with last five fixture paths per environment.

## Stretch Squad 11: Warm Daemon Watch UX

**Files:**

- Create or modify: `contrib/vscode-glade/src/tests/watch.ts`
- Modify: `contrib/vscode-glade/src/views/runsView.ts`
- Modify: `contrib/vscode-glade/src/output.ts`
- Modify: `contrib/vscode-glade/src/extension.ts`
- Test: `cd contrib/vscode-glade && npm test && npm run package`

**Agent prompt:**

```text
You are Stretch Squad 11. Make the warm local test loop visible.
Start glade test --project <root> --daemon --watch and parse NDJSON events.
Show watch status, last run duration, pass/fail counts, and a stop command in Recommended Runs.
Stream raw output into Glade Tests.
Do not change the Go test daemon in this task.
Run npm test and npm run package.
Return a sample NDJSON event and the view rows it produces.
```

- [ ] Add `WatchSession` class that owns one child process.
- [ ] Add `glade.startWatch` and `glade.stopWatch`.
- [ ] Parse one JSON object per line; unparseable lines go to the output channel.
- [ ] Update Runs view rows: `Watch: idle/running`, `Last run: <summary>`, `Start watch`, `Stop watch`.
- [ ] Ensure deactivate kills the child process.

## Stretch Squad 12: Breakpoint Validation And Debug History

**Files:**

- Modify: `contrib/vscode-glade/src/breakpoints.ts`
- Modify: `contrib/vscode-glade/src/debug.ts`
- Create: `contrib/vscode-glade/src/debugHistory.ts`
- Modify: `contrib/vscode-glade/src/views/debugView.ts`
- Create: `contrib/vscode-glade/test/debugHistory.test.js`
- Test: `cd contrib/vscode-glade && npm test && npm run package`

**Agent prompt:**

```text
You are Stretch Squad 12. Make debug launches feel solid.
Validate Apex breakpoint files before launch and warn when no enabled Apex breakpoints exist.
Keep a local debug history for the last ten launches.
Show history in Debug And Logs with re-run commands.
Do not alter DAP protocol code in Go.
Run npm test and npm run package.
Return one debug-history JSON entry and one breakpoint warning case.
```

- [ ] Add pure history model: `{ name, projectRoot, program, sourcePreview, dbPath, startedAt }`.
- [ ] Store history in `context.workspaceState`.
- [ ] Warn before debug when all Apex breakpoints are disabled or outside the active project root.
- [ ] Add `glade.rerunDebugHistory` command.
- [ ] Keep warning non-blocking. A user can debug without breakpoints.

## Stretch Squad 13: Extension Host Smoke Automation

**Files:**

- Modify: `contrib/vscode-glade/package.json`
- Modify: `contrib/vscode-glade/package-lock.json`
- Create: `contrib/vscode-glade/test/smoke/runTest.ts`
- Create: `contrib/vscode-glade/test/smoke/suite/index.ts`
- Modify: `.github/workflows/vscode-glade.yml`
- Test: `cd contrib/vscode-glade && npm run smoke`

**Agent prompt:**

```text
You are Stretch Squad 13. Add Extension Development Host smoke tests.
Use @vscode/test-electron.
Open a fixture SFDX project, activate the extension, assert Glade views and commands exist.
Keep the smoke test short. It must run in CI.
Run npm run smoke locally.
Return the exact CI command and elapsed time.
```

- [ ] Add `@vscode/test-electron` dev dependency.
- [ ] Add `npm run smoke`.
- [ ] Use `internal/debuglog/testdata/project` or a copied fixture under `/tmp`.
- [ ] Assert commands: `glade.refresh`, `glade.executeAnonymous`, `glade.debugAnonymous`, `glade.inspectLocalOrg`.
- [ ] Add workflow step after package.

## Stretch Squad 14: Release Screenshots And Docs Polish

**Files:**

- Modify: `contrib/vscode-glade/README.md`
- Modify: `docs/EDITOR.md`
- Modify: `site/docs-src/guide/editor.md`
- Optional asset: `contrib/vscode-glade/media/screenshot.png`
- Test: `git diff --check`

**Agent prompt:**

```text
You are Stretch Squad 14. Polish the docs for a user who already has Salesforce VS Code extensions installed.
Show Glade as the local Apex lane beside Salesforce org-backed tools.
Include the install command, sidebar screenshot, active environment model, breakpoint note, and common workflows.
Do not claim Marketplace publishing.
Run git diff --check.
Return the docs changed and image dimensions if a screenshot is added.
```

- [ ] Add a short section: `Glade local lane beside Salesforce`.
- [ ] Add workflows: run local test, debug local test with VS Code breakpoints, switch data environment, execute anonymous Apex against active environment.
- [ ] Use the mockup screenshot only if no Extension Host screenshot exists.
- [ ] Keep docs tied to release archive install, not Marketplace.

## Squad 6A: DB-Backed DAP Sessions

**Files:**

- Modify: `internal/gladecli/dap_command.go`
- Modify: `internal/gladecli/cli_test.go`
- Modify: `internal/dap/handler.go`
- Modify: `internal/dap/live.go`
- Modify: `internal/dap/live_test.go`
- Modify: `contrib/vscode-glade/src/adapter.ts`
- Modify: `contrib/vscode-glade/src/debug.ts`
- Modify: `contrib/vscode-glade/test/commands.test.js`
- Test: `go test ./internal/dap ./internal/gladecli -run 'Test.*DAP.*DB|TestLiveSession'`
- Test: `cd contrib/vscode-glade && npm test && npm run package`

**Agent prompt:**

```text
You are Squad 6A. Let VS Code debug sessions use the active local data environment.
Add glade dap --db <path> and --dry-run.
Load the DB org with openDBStore, attach it to the VM, and save it only when the live session ends without execution error and dry-run is false.
Add a DAP live-session done hook instead of saving from a parallel goroutine.
Update the VS Code adapter launch args to pass the active environment DB path.
Do not change breakpoint protocol behavior.
Run the focused DAP and extension tests.
Return one debug command line and one inspect JSON proving persistence.
```

- [ ] Add DAP CLI flags in `runDAP`: `--db <path>` and `--dry-run`.
- [ ] Extend `handleDAPLaunch` to use `openDBStore(dbPath, projectRoot)` when `dbPath` is set.
- [ ] Add a DAP handler hook:

```go
type LiveSessionDoneHook func(machine *vm.VM, err error) error
```

- [ ] Add `PrepareLiveSessionWithDone(machine, program, hook)` while keeping existing `PrepareLiveSession` as a wrapper.
- [ ] In `StartLiveSession`, call the hook after `machine.Execute(program)` returns and before publishing `terminated`. If the hook returns an error, publish it to stderr output and pass that error to `session.done`.
- [ ] In `dap_command.go`, the hook saves `storage.SnapshotRuntimeOrg(machine.Org)` only when `err == nil`, `machine.Org != nil`, `store != nil`, and `dryRun == false`.
- [ ] Update VS Code adapter process args to include `--db <active.dbPath>` when an active environment exists.
- [ ] Add a manual smoke: set a breakpoint, debug anonymous Apex that inserts `Account`, continue to termination, inspect active environment, confirm `byObject.Account` increased.

---

## Integration Order

Use this order when integrating squad branches:

1. Squad 1: CLI install and release bundle.
2. Squad 1A: CLI local data execution contract.
3. Squad 2: extension foundation.
4. Squad 3: sidebar.
5. Squad 4: tests.
6. Squad 6: DAP and breakpoint launch flow.
7. Squad 7: local data environments and local org.
8. Squad 6A: DB-backed DAP sessions.
9. Squad 5: LSP, default-off beside Salesforce.
10. Squad 0: JetBrains removal can land anytime, but before docs finalization.
11. Squad 8: docs and CI morning gate.
12. Stretch Squad 9: CodeLens.
13. Stretch Squad 10: environment clone/diff.
14. Stretch Squad 11: warm daemon watch UX.
15. Stretch Squad 12: breakpoint validation and debug history.
16. Stretch Squad 13: Extension Host smoke automation.
17. Stretch Squad 14: release screenshots and docs polish.

Reason: project context is the common beam. Sidebar, tests, DAP, LSP, and local org all hang from it.

## Overnight Schedule

Hour 0:

- Create branch.
- Dispatch Squad 0, Squad 1, and Squad 1A.
- Dispatch Squad 2 with stubs.

Hours 1-3:

- Squad 1 lands CLI install behavior.
- Squad 1A lands DB-backed anonymous exec.
- Squad 2 lands project context.
- Squad 3 starts sidebar on Squad 2 APIs.
- Squad 4 starts discovery and result parsing.
- Squad 5 starts LSP after dependency install, but it does not block Tier 1.
- Squad 6 starts DAP pure model tests.
- Squad 7 starts local org parser and view.

Hours 3-6:

- Merge Squads 2, 3, 4.
- Resolve manifest conflicts in `package.json`.
- Run `npm test` after each merge.

Hours 6-8:

- Merge Squads 6, 7, and 6A.
- Run `go test ./internal/gladecli -run 'TestRunExec.*DB'`.
- Run `go test ./internal/dap ./internal/gladecli -run 'Test.*DAP.*DB|TestLiveSession'`.
- Run Extension Development Host manual smoke.
- Fix activation errors and command id mismatches.

Hours 8-10:

- Merge Squad 0 if not merged.
- Merge Squad 5 only if the core workflow is green or the LSP branch is already green.
- Run Squad 8 docs, CI, and release-build smoke.
- Run Tier 1 final verification.

Hours 10+:

- Dispatch stretch squads in this order: 9 and 11 first, then 10 and 12, then 13 and 14.
- Merge each stretch squad only after `npm test` and `npm run package`.
- Re-run Extension Development Host after every two stretch merges.
- Stop stretch merging when a gate breaks and no owner is free to fix it. Leave the remaining stretch branches unmerged with notes.

## Final Verification Gates

Run all:

```bash
cd /Users/matt/Dev/glade/contrib/vscode-glade
npm test
npm run package
cd /Users/matt/Dev/glade
go test ./internal/gladecli
go test ./internal/gladecli -run 'TestRunExec.*DB'
go test ./internal/dap ./internal/gladecli -run 'Test.*DAP.*DB|TestLiveSession'
scripts/smoke.sh
VERSION=dev DIST_DIR=/tmp/glade-release-test scripts/release-build.sh
tar -tzf /tmp/glade-release-test/glade_dev_$(go env GOOS)_$(go env GOARCH).tar.gz | rg 'share/glade/editor/vscode-glade.vsix'
git diff --check
```

Manual smoke:

```bash
cd /Users/matt/Dev/glade/contrib/vscode-glade
code .
```

Run `Launch Extension`. Open `/Users/matt/Dev/glade/internal/debuglog/testdata/project` in the Extension Development Host. Verify:

- Glade Activity Bar is visible.
- Project view shows SFDX root.
- Apex tests appear in Test Explorer.
- `Run changed since origin/main` executes and writes to `Glade Tests`.
- Data Environments shows `Active: dev`.
- `Glade: Execute Local Anonymous Apex` runs against `.glade/envs/dev.sqlite`.
- Local Org view shows `byObject` row counts after inspect.
- `Glade: Debug Anonymous Apex` starts a `glade` debug session.
- A breakpoint set in a `.cls` file appears in Debug And Logs.
- With Salesforce Apex language support installed, Glade LSP stays idle unless `glade.enableLsp=true`.

Stretch gates, when those squads land:

```bash
cd /Users/matt/Dev/glade/contrib/vscode-glade
npm run smoke
npm test
npm run package
```

Manual stretch smoke:

- Glade CodeLens appears as `Run Local Test` beside Salesforce CodeLens.
- Watch mode shows running status and can stop the daemon process.
- Environment diff opens a text document with object count deltas.
- Debug history can re-run the last local test debug launch.
- DB-backed debug can insert one record and persist it after continue-to-end.

## Risk Register

| Risk | Symptom | Fix |
| --- | --- | --- |
| `package.json` merge conflicts | Sidebar, DAP, LSP squads all edit contributes | Squad 8 owns final manifest normalization and runs `npm run package` |
| VS Code Testing API compile drift | TypeScript errors around `TestItem.data` | Use local casting wrapper and keep data access in `tests/controller.ts` |
| LSP package import mismatch | `vscode-languageclient/node` not found | Confirm `npm install vscode-languageclient` updated `package-lock.json` |
| Cold large-project runs feel hung | Output panel quiet during compile | Show command line immediately and stream stderr/stdout into `Glade Tests` |
| Bundled VSIX missing from archive | `glade editor install vscode` asks for `--vsix` | Fix `scripts/release-build.sh` copy and verify tar listing |
| `glade exec --db` saves failed execution | Local data changes after an exception | Save only after `execErr == nil`; prove with a failing-DML test |
| Local org JSON shape drifts | Local Org view empty despite db rows | Parse current `byObject` shape first and keep test fixture tied to `glade db inspect --json` |
| Salesforce CodeLens confusion | Users click org-backed `Run Test` expecting local | Every Glade label says `Local`; Glade commands stay under Activity Bar and `glade.*` ids |
| Breakpoint expectations mismatch | Breakpoint appears in VS Code but local debug does not stop | Use standard DAP `setBreakpoints`, show active breakpoint list, and add validation warnings in Stretch Squad 12 |
| DAP DB save hook saves after disconnect | Debug stop leaves partial data in SQLite | Squad 6A saves only when live execution returns nil error and dry-run is false |
| Stretch work destabilizes Tier 1 | Late squad changes break packaging | Merge stretch only after Tier 1 green; revert the stretch branch, not Tier 1 work |

## Self-Review

Spec coverage:

- JetBrains removal: Squad 0.
- VS Code extension cockpit: Squads 2 and 3.
- Native local Apex running: Squad 4.
- SFDX context: Squad 2.
- DAP: Squad 6.
- LSP: Squad 5.
- Breakpoints: Squad 6 and Squad 7, with DB-backed debug sessions in Squad 6A and validation in Stretch Squad 12.
- Local data environments: Squad 1A and Squad 7, with clone/diff stretch in Squad 10.
- Local org view: Squad 7.
- Easy install after Glade install: Squad 1.
- Salesforce VS Code coexistence: hard boundary plus CodeLens stretch in Squad 9.
- Overnight parallel agents: squads 0 through 14, integration order, and stretch gates above.

Placeholder scan:

- No step relies on unspecified behavior.
- Full SOQL scratch is explicitly out of scope because the current base CLI does not expose `glade db query`.
- Record-grid editing is explicitly out of scope. Named environments and count-level diff give useful data control without building a database editor overnight.

Type consistency:

- `GladeProjectContext`, `GladeTestRun`, and `GladeTestCase` names are stable across tasks.
- Command ids use `glade.*` consistently.
- View ids use `glade.project`, `glade.recommendedRuns`, `glade.apexTests`, `glade.environments`, `glade.localOrg`, and `glade.debugLogs`.
- DB inspect parsing uses `byObject`, `objects`, `records`, `users`, `profiles`, and `permissions`.
