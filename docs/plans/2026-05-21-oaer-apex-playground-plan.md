# OAER Apex Playground Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a local-first Apex playground that lets a developer edit Apex class files, run execute-anonymous code against those files, see cached results, and inspect output, variables, limits, trace, diagnostics, and org changes.

**Architecture:** Add `oaer playground` as a local HTTP server over OAER's existing compiler, project runtime registration, VM, storage, and SQLite paths. Store playground files under `.oaer/playground/workspaces/<workspace-id>/`, execute through a first-class playground API, and serve a shadcn-style React UI from embedded static assets. Keep Salesforce-shaped `server` routes separate; the playground API returns OAER-shaped developer output.

**Tech Stack:** Go 1.26, `net/http`, `embed`, existing `internal/project`, `internal/schema`, `internal/typesys`, `internal/apextest`, `internal/vm`, `internal/storage`, React, Vite, TypeScript, Tailwind CSS, shadcn-style local components, Radix primitives, and Prism Apex highlighting.

---

## Product Shape

The first working release is local only. It binds to `127.0.0.1`, opens in a browser, and uses one managed scratch workspace unless `--project` points at an existing SFDX project. The scratch workspace can load built-in examples or configured local project references. The UI should feel closer to DotNetFiddle than a Salesforce admin page: file explorer on the left, main class editor, execute-anonymous editor below it, output and cached-result panels on the right. It should be slick, quiet, and DX focused. The developer should see what ran, whether it came from cache, what changed in the org, and what to fix next.

The first release does not need collaboration, hosted accounts, sharing links, auth, package install UI, or Electron/Tauri packaging. Those can wait until the local web UI proves its shape.

## File Structure

Create:
- `internal/playground/types.go`  
  Request and response structs for workspace, file, run, result cache, diagnostics, limits, trace, and org diff.
- `internal/playground/workspace.go`  
  Workspace path validation, file read/write/delete/list, default workspace creation, and SFDX project skeleton creation under `.oaer/playground/workspaces/default`.
- `internal/playground/runner.go`  
  Compile project files, register project runtime, compile anonymous Apex, execute VM, drain async work, collect logs, vars, limits, trace, diagnostics, and org diff.
- `internal/playground/cache.go`  
  Stable hash calculation, cached result load/store, latest pointer, and cache invalidation.
- `internal/playground/examples.go`  
  Built-in example project definitions for DML, SOQL, triggers, collections, relationships, persist mode, and limit-counter drills.
- `internal/playground/server.go`  
  HTTP routes, JSON helpers, request limits, cancellation, serialized run lock, and static UI serving.
- `internal/playground/orgdiff.go`  
  Before/after org comparison by object, inserted/updated/deleted counts, changed IDs, and record previews.
- `internal/playground/static.go`  
  `embed.FS` wrapper for built UI assets.
- `internal/playground/*_test.go`  
  Focused package tests for file safety, run behavior, cache hits, reset/seed, and route shapes.
- `internal/playground/web/package.json`
- `internal/playground/web/vite.config.ts`
- `internal/playground/web/tsconfig.json`
- `internal/playground/web/src/App.tsx`
- `internal/playground/web/src/components/CodeEditor.tsx`
- `internal/playground/web/src/components/ui/*.tsx`
- `internal/playground/web/src/index.css`
- `internal/playground/web/src/lib/*.ts`
- `internal/playground/web/src/lib/*.test.ts`

Modify:
- `internal/oaercli/cli.go`  
  Add `playground` command and flags.
- `internal/oaercli/cli_test.go`  
  Add CLI route and flag tests.
- `scripts/smoke.sh`  
  Add a non-opening smoke check for `oaer playground --once` or an equivalent test-only startup probe.
- `docs/INSTALL.md`  
  Add local playground usage.
- `docs/EDITOR.md`  
  Add the playground as a browser-first editor loop.
- `docs/RELEASE_NOTES.md`  
  Add the feature note.

Generated or built:
- `internal/playground/static/`  
  Built UI assets embedded into the binary. This directory is the checked-in single-binary bundle, not the source of truth. `internal/playground/web` is the source of truth.

## CLI Contract

Add:

```bash
oaer playground [--project <root>] [--project-ref name=path] [--addr 127.0.0.1:1789] [--db <path>] [--workspace <id>] [--limit-mode strict|permissive] [--open] [--no-open] [--once]
```

Defaults:
- `--addr 127.0.0.1:1789`
- `--workspace default`
- `--db .oaer/playground/org.sqlite`
- `--limit-mode permissive`
- no browser open unless `--open` is set

Behavior:
- `--project` edits the provided SFDX project directly.
- no `--project` creates a managed scratch SFDX-shaped playground workspace.
- `--project-ref name=path` adds a local SFDX folder to the scratch workspace selector. Loading it copies supported files into scratch instead of editing the source folder.
- `--once` starts the handler far enough for tests to prove setup and route registration, then exits without listening forever.
- unknown flags return a stable CLI error.
- if a port is busy, `ListenAndServe` returns the bind error.

## API Contract

All routes live under `/playground/api/`.

`GET /playground/api/workspace`

Returns workspace id, file tree, loaded example id when known, anonymous body, project root, limit mode, and workspace hash.

`GET /playground/api/examples`

Returns built-in examples plus configured local project references. Also returns `canLoad`, which is false when the server was started with `--project`.

`POST /playground/api/examples/load`

Loads a built-in example or project reference into the managed scratch workspace. Loading a reference copies supported files into scratch and clears the latest run pointer.

`PUT /playground/api/files`

Body:

```json
{
  "path": "force-app/main/default/classes/AccountPlayground.cls",
  "content": "public class AccountPlayground { }",
  "version": 4
}
```

Returns the saved file, next version, and workspace hash. Reject absolute paths, `..`, unsupported extensions, oversized files, and stale versions.

`DELETE /playground/api/files`

Deletes a safe workspace file. Reject deletion of `sfdx-project.json`.

`POST /playground/api/run`

Body:

```json
{
  "anonymousBody": "System.debug(AccountPlayground.makeAccount('Twin Lakes Supply').Name);",
  "mode": "scratch",
  "limitMode": "permissive",
  "useCache": true
}
```

Returns:

```json
{
  "runId": "20260521T181530Z-b3f2",
  "cacheHit": false,
  "status": "pass",
  "compileMs": 14,
  "executeMs": 36,
  "diagnostics": [],
  "logs": ["Twin Lakes Supply"],
  "vars": [{"name": "account", "type": "Account", "value": {"Id": "001000000000001", "Name": "Twin Lakes Supply"}}],
  "limits": {"queries": 1, "dmlStatements": 1, "dmlRows": 1, "cpuTimeMs": 3},
  "trace": [],
  "orgDiff": [{"object": "Account", "inserted": 1, "updated": 0, "deleted": 0}],
  "cacheKey": "sha256:..."
}
```

`POST /playground/api/reset`

Resets the local org to the workspace seed and clears volatile run state.

`POST /playground/api/seed`

Applies a fixture from `seed.json` or a posted fixture body.

`GET /playground/api/runs/latest`

Returns the last run result, including whether the visible UI is showing cached output.

## Execution Pipeline

The runner should do the job in this order:

1. Read the workspace file set from disk.
2. Ensure a valid `sfdx-project.json` exists for scratch workspaces.
3. Load project metadata through existing project/schema paths.
4. Build a `typesys.Index`.
5. Create `vm.New(nil)`.
6. Register classes and triggers with `apextest.RegisterProjectRuntimeForRequest`.
7. Compile anonymous code with `vm.CompileAnonymous`.
8. Clone the current org for scratch mode, or use the persistent org for persist mode.
9. Set context timeout on the VM.
10. Set org, user, trace, and limit mode.
11. Execute.
12. Drain async.
13. Compute org diff against the pre-run snapshot.
14. Commit only when mode is `persist` and execution passed.
15. Store run result by cache key.
16. Return OAER-shaped JSON.

Do not route this through Salesforce Tooling `executeAnonymous`. That shape is useful for compatibility. The playground needs richer local output.

## Result Cache

Cache key inputs:
- all workspace file paths and contents
- anonymous body
- seed fixture hash
- project root or scratch workspace id
- limit mode
- run mode
- OAER version string

Cache storage:
- `.oaer/playground/cache/<hash>.json`
- `.oaer/playground/runs/latest.json`

Cache rules:
- if the hash matches and `useCache` is true, return the stored result with `cacheHit: true`.
- if any file changes, mark the UI cached-result badge stale.
- manual Run bypasses a stale visible result but still stores a new result.
- Reset clears org state and invalidates cache entries that depend on the old seed hash.

## UI Requirements

Use shadcn/ui style and behavior:
- app shell with top command bar
- fixed three-pane grid layout
- left workspace pane with example selector and class, trigger, anonymous, data, and metadata files
- textarea-backed editor with Prism Apex highlighting for source files
- a dedicated execute-anonymous editor with the Run button beside it
- output tabs for Logs, Vars, Problems, Limits, and Trace
- compact metric tiles with status, total time, DML count, and row count
- auto-run toggle with debounce
- command palette for Run, Save, Load Example, New Class, Seed Data, Reset Org, and Theme
- keyboard shortcuts:
  - `Cmd/Ctrl+Enter`: run
  - `Cmd/Ctrl+S`: save active file
  - `Cmd/Ctrl+K`: command palette
  - `Cmd/Ctrl+Shift+R`: reset org
- inline status badges for saved, running, cache hit, compile error, and runtime error
- no marketing hero, no decorative cards, no oversized type
- dense, useful, polished developer surface

Visual tone:
- dark-first slate shell with a restrained purple accent
- dark editor
- clear pass, fail, and amber stale-cache states
- compact metrics
- no one-note purple/blue gradient theme
- no nested cards inside cards

## Task Plan

### Task 1: Playground Workspace Package

**Files:**
- Create: `internal/playground/types.go`
- Create: `internal/playground/workspace.go`
- Test: `internal/playground/workspace_test.go`

- [ ] Add request/response structs for files, workspace metadata, run metadata, and errors.
- [ ] Add `Workspace` with root, id, project root, and data root.
- [ ] Add safe path validation that accepts `.cls`, `.trigger`, `.apex`, `.json`, `.xml`, `.yml`, and `.yaml`.
- [ ] Add default scratch workspace creation with `sfdx-project.json` and `force-app/main/default/classes/AccountPlayground.cls`.
- [ ] Test that absolute paths, `..`, unsupported extensions, and oversized files are rejected.
- [ ] Test that a default workspace writes an SFDX-shaped tree.

Run:

```bash
go test ./internal/playground -run 'TestWorkspace'
```

Expected: workspace tests pass.

### Task 2: Runner Package

**Files:**
- Create: `internal/playground/runner.go`
- Create: `internal/playground/orgdiff.go`
- Test: `internal/playground/runner_test.go`

- [ ] Build a runner that loads the workspace project and schema.
- [ ] Register project runtime through `apextest.RegisterProjectRuntimeForRequest`.
- [ ] Compile execute-anonymous body through `vm.CompileAnonymous`.
- [ ] Execute with context timeout, trace enabled, selected limit mode, and cloned org state.
- [ ] Return logs, vars, limits, trace, diagnostics, and org diff.
- [ ] Test a class file called from anonymous Apex.
- [ ] Test compile error returns `status: compile_error` and no org commit.
- [ ] Test runtime error returns `status: runtime_error` and includes debug logs already emitted.

Run:

```bash
go test ./internal/playground -run 'TestRunner'
```

Expected: runner tests pass.

### Task 3: Result Cache

**Files:**
- Create: `internal/playground/cache.go`
- Test: `internal/playground/cache_test.go`

- [ ] Hash workspace files, anonymous body, seed, limit mode, run mode, project root, and version.
- [ ] Store run results under `.oaer/playground/cache/<hash>.json`.
- [ ] Store latest pointer under `.oaer/playground/runs/latest.json`.
- [ ] Return `cacheHit: true` when the same request repeats.
- [ ] Invalidate cache when class file, anonymous body, seed, or limit mode changes.

Run:

```bash
go test ./internal/playground -run 'TestCache'
```

Expected: cache hit and stale-cache tests pass.

### Task 4: HTTP API

**Files:**
- Create: `internal/playground/server.go`
- Test: `internal/playground/server_test.go`

- [ ] Add routes for workspace load, file save, file delete, run, reset, seed, latest run, and static UI.
- [ ] Serialize runs per workspace with a mutex.
- [ ] Cancel stale auto-runs with request context.
- [ ] Cap request bodies.
- [ ] Return stable JSON errors.
- [ ] Test route shapes with `httptest`.
- [ ] Test stale file version conflict.
- [ ] Test run route returns pass, cache hit, compile error, and runtime error shapes.

Run:

```bash
go test ./internal/playground
```

Expected: all playground package tests pass.

### Task 5: CLI Command

**Files:**
- Modify: `internal/oaercli/cli.go`
- Modify: `internal/oaercli/cli_test.go`

- [ ] Add `playground` to usage.
- [ ] Parse `--project`, `--project-ref`, `--addr`, `--db`, `--workspace`, `--limit-mode`, `--open`, `--no-open`, and `--once`.
- [ ] Wire the command to `internal/playground`.
- [ ] Keep `--open` opt-in.
- [ ] Print `oaer playground: http://127.0.0.1:<port>/playground/`.
- [ ] Test unknown flags.
- [ ] Test `--once` route setup.
- [ ] Test bad project path returns an error.

Run:

```bash
go test ./internal/oaercli -run 'TestRunPlayground'
```

Expected: CLI playground tests pass.

### Task 6: Frontend Foundation

**Files:**
- Create: `internal/playground/web/package.json`
- Create: `internal/playground/web/vite.config.ts`
- Create: `internal/playground/web/tsconfig.json`
- Create: `internal/playground/web/src/index.css`
- Create: `internal/playground/web/src/App.tsx`
- Create: `internal/playground/web/src/lib/save-state.ts`

- [ ] Add Vite React TypeScript app.
- [ ] Add Tailwind and shadcn-style component setup.
- [ ] Add API client with typed responses matching Go structs.
- [ ] Add app state for workspace, active file, anonymous body, dirty state, auto-run, run state, cache state, and selected output tab.
- [ ] Add tests for stale save/run handling.

Run:

```bash
cd internal/playground/web
npm install
npm test
```

Expected: frontend state tests pass.

### Task 7: Slick UI Components

**Files:**
- Create: `internal/playground/web/src/components/CodeEditor.tsx`
- Create: `internal/playground/web/src/components/ui/*.tsx`

- [ ] Build the three-pane grid layout.
- [ ] Use Prism-highlighted editors for class files and anonymous Apex.
- [ ] Use shadcn-style buttons, tabs, switches, command palette, and select menus.
- [ ] Keep cards out of cards.
- [ ] Add keyboard shortcuts.
- [ ] Add auto-run debounce.
- [ ] Add cached-result badge and stale state.
- [ ] Add empty, running, pass, compile error, runtime error, and cache hit states.

Run:

```bash
cd internal/playground/web
npm run build
```

Expected: frontend build passes.

### Task 8: Embed And Serve UI

**Files:**
- Create: `internal/playground/static.go`
- Create or update: `internal/playground/static/`
- Modify: `internal/playground/web/vite.config.ts`
- Modify: `scripts/smoke.sh`

- [ ] Build the frontend into `internal/playground/static`.
- [ ] Embed the static assets with Go `embed`.
- [ ] Serve `/playground/` and fallback nested UI routes to `index.html`.
- [ ] Add a smoke check that starts `oaer playground --once`.

Run:

```bash
cd internal/playground/web && npm run build
cd ../../..
go test ./internal/playground ./internal/oaercli
```

Expected: UI assets build and Go tests pass.

### Task 9: End-To-End Local Check

- [ ] Start `go run ./cmd/oaer playground --addr 127.0.0.1:1789 --db <tempdb>`, or another known free localhost port.
- [ ] Open `/playground/`.
- [ ] Confirm file tree appears.
- [ ] Edit class file.
- [ ] Confirm auto-run queues and completes.
- [ ] Confirm cached result appears on repeated run.
- [ ] Confirm compile error appears when bad Apex is typed.
- [ ] Confirm reset returns org counts to seed values.

Run:

```bash
go test ./internal/playground ./internal/oaercli
cd internal/playground/web && npm test
```

Expected: local browser flow passes.

### Task 10: Docs And Release Notes

**Files:**
- Modify: `docs/INSTALL.md`
- Modify: `docs/EDITOR.md`
- Modify: `docs/RELEASE_NOTES.md`
- Modify: `docs/plans/2026-05-21-oaer-apex-playground-plan.md`

- [ ] Document `oaer playground`.
- [ ] Document local-only binding and security posture.
- [ ] Document workspace storage under `.oaer/playground`.
- [ ] Document cache behavior.
- [ ] Document common keyboard shortcuts.
- [ ] Add release note.

Run:

```bash
go test ./internal/playground ./internal/oaercli
```

Expected: final touched Go packages pass.

## Acceptance Criteria

- `go run ./cmd/oaer playground --project <root> --db .oaer/playground/org.sqlite --open` serves the UI.
- A user can create or edit at least one Apex class file.
- A user can run execute-anonymous Apex that calls that class.
- The output pane shows debug logs, variables, limits, trace events, diagnostics, org diff, and cache metadata.
- Auto-run debounces edits and cancels stale runs.
- A repeated unchanged run returns a cache hit.
- Reset restores the local org seed.
- Compile and runtime failures do not commit org changes.
- The UI is polished enough for daily developer use: shadcn-style controls, Prism-backed editing, keyboard shortcuts, clear status, and no marketing page.
- The existing `oaer server` Salesforce-shaped API remains unchanged.

## Validation Ladder

Run focused checks first:

```bash
go test ./internal/playground
go test ./internal/oaercli -run 'TestRunPlayground'
```

Then frontend checks:

```bash
cd internal/playground/web
npm test
npm run build
```

Then integration:

```bash
go test ./internal/playground ./internal/oaercli
go run ./cmd/oaer playground --once
```

Only after the local feature is stable, run broader checks:

```bash
go test ./...
scripts/smoke.sh
```

## Risks

- Project class registration currently lives in `internal/apextest`; the playground runner should reuse it first and only extract a shared runtime compiler if tests force that move.
- Embedded frontend assets add a Node build step. Keep Go runtime free of Node, and keep Node only in development/build.
- React, Prism, and Radix components can grow the UI bundle. Local-first can tolerate a larger bundle, but startup should remain fast.
- Org diff can become expensive on large orgs. Start with object-level counts and changed IDs, then add record previews behind limits.
- Full Apex parity is not the goal of this feature. Unsupported runtime behavior should surface as explicit diagnostics.

## Future Work

- Import/export playground workspaces.
- Shareable local `.oaer/playground` bundles.
- Electron or Tauri wrapper after the web UI proves itself.
- LSP-powered completions and diagnostics inside the web editor.
- Debug stepping with the existing DAP machinery.
- Fixture browser and record inspector.
- Package dependency browser for managed-package-like stubs.
