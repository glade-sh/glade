# Local Playground Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the local `glade playground` a clean, fast, multi-file Apex workbench that shows Glade's runtime power without relying on a hosted playground.

**Architecture:** Keep the local playground permissive and file-backed. Use the existing Go server, runner, workspace, React app, and embedded static assets. Fix source-change runtime invalidation first. Then improve the source editor shape, built-in examples, command flags, and docs. Anonymous Apex stays visible below the source files.

**Tech Stack:** Go 1.26, `internal/playground`, `internal/gladecli`, React 19, TypeScript, Vitest, Vite, existing shadcn components, lucide icons, embedded playground static assets.

---

## Current Findings

- `glade playground` is already local-first. The default address is `127.0.0.1:1789`.
- Local mode can use built-in examples, an SFDX project root, project references, permissive limits, scratch runs, persist runs, SQLite state, seeding, reset, cache, database browsing, logs, limits, trace, and org diff.
- `--public` exists, but public publishing is paused. Do not add hosted links while this plan is in flight.
- The runner has two cache layers: the playground result cache and the project runtime template cache in `internal/playground/runner.go`.
- Apex test runtime caches can be cleared with `apextest.InvalidateRuntimeCaches()`, but the playground server does not call it after source saves, deletes, or example loads.
- The UI keeps anonymous Apex visible in the normal editor grid. In advanced mode, the Apex/database tabs sit above the full editor area, so database can replace both source and anonymous panes.
- Source files are chosen from the left workspace tree. There are no source tabs above the code editor.
- Built-in examples already cover DML, SOQL, triggers, org diff, limits, maps, Account, Contact, and multi-class flows. They need a few stronger multi-file examples that read like a Salesforce developer's day job.

## Boundaries

In scope:

- Local playground command and UI.
- Built-in local examples.
- Local docs and local command help.
- Runtime cache correctness after workspace changes.
- Local persistence controls.

Out of scope:

- Hosted deployment.
- Public domain links.
- Batch Apex or async showcase flows.
- OAuth, org login, or live Salesforce connectivity.
- Compatibility scanner or first-party plugin work.

## File Map

- Modify `internal/playground/runner.go`: add explicit source-runtime invalidation.
- Modify `internal/playground/server.go`: invalidate runtime caches after source saves, source deletes, and source loads.
- Modify `internal/playground/server_test.go`: add server-level cache invalidation tests.
- Modify `internal/playground/runner_test.go`: keep anonymous-only save coverage and add any runner-only source cache coverage needed.
- Modify `internal/playground/examples.go`: add richer multi-file examples.
- Modify `internal/playground/types.go`: add fields only if the UI needs example "touches" metadata or initial command defaults.
- Modify `internal/gladecli/server_command.go`: add local command flags.
- Modify `internal/gladecli/cli_test.go`: cover new local command flags and wizard output.
- Modify `internal/playground/web/src/App.tsx`: add source tabs, keep anonymous below source, and improve local state controls.
- Modify `internal/playground/web/src/App.test.tsx`: cover source tabs, anonymous pane visibility, docs link, and database placement.
- Modify `internal/playground/web/src/index.css`: add tab strip, responsive editor sizing, and local state badges.
- Regenerate `internal/playground/static/index.html` and `internal/playground/static/assets/*` with the web build.
- Modify `site/docs-src/guide/playground.md`: document the improved local-only flow.
- Modify `site/docs-src/guide/cli-reference.md`: document new local flags.

---

### Task 1: Source Runtime Invalidation

**Purpose:** A saved source file must affect the next run without restarting the playground.

- [ ] Add `func (r *Runner) InvalidateSourceRuntime()` in `internal/playground/runner.go`.

Implementation details:

- Lock `r.mu`.
- Set `r.runtimeTemplate = nil`.
- Set `r.lastOrgCacheKey = ""`.
- Keep `r.lastOrg` intact only if the next run must still allow database browsing of the last result. Do not allow cache hits to reuse it.
- Call `apextest.InvalidateRuntimeCaches()` outside or inside the same method. Keep the method as the one place that knows source changed.

- [ ] Add `func shouldInvalidateRuntimeForFile(path string) bool` near the server save/delete handlers.

It should return true for these workspace file kinds:

- `class`
- `trigger`
- `metadata`
- `other`, only if the path is under `force-app/`

It should return false for:

- `anonymous`
- `data`

- [ ] Call `s.runner.InvalidateSourceRuntime()` after successful source save in `handleSaveFile`.
- [ ] Call it after successful source delete in `handleDeleteFile`.
- [ ] Call it after successful `handleLoadExample` and local project reference loads.
- [ ] Do not invalidate after anonymous-only saves.

Tests:

- [ ] Add `TestServerSaveSourceInvalidatesProjectRuntime`.
- [ ] Add `TestServerDeleteSourceInvalidatesProjectRuntime`.
- [ ] Add `TestServerLoadExampleInvalidatesProjectRuntime`.
- [ ] Keep `TestRunnerKeepsProjectRuntimeAfterAnonymousFileSave` passing.

Focused gate:

```bash
go test ./internal/playground -run 'Test(ServerSaveSourceInvalidatesProjectRuntime|ServerDeleteSourceInvalidatesProjectRuntime|ServerLoadExampleInvalidatesProjectRuntime|RunnerKeepsProjectRuntimeAfterAnonymousFileSave)' -count=1
```

Full package gate:

```bash
go test ./internal/playground -count=1
```

### Task 2: Source Tabs Above Code

**Purpose:** Multiple classes should read like a small Salesforce project, not a single-file scratch pad.

- [ ] Add source tabs above the source `CodeEditor`.
- [ ] Keep the anonymous Apex `CodeEditor` as a pane below source. Do not hide it in a tab.
- [ ] Keep the left tree for discovery and large projects.
- [ ] Open a source tab when a class, trigger, or metadata source is selected from the tree.
- [ ] Keep 6 to 8 recent source tabs visible, with overflow handled by horizontal scroll.
- [ ] Add close buttons to non-active tabs. Do not close the last source tab.
- [ ] Show a dirty marker on any tab whose path is in `dirtyPaths`.
- [ ] Use `FileCode2`, `Trash2`, and existing button components. Do not add hand-drawn icons.
- [ ] Preserve read-only handling for project references.

Tests:

- [ ] Add `renders source tabs above the source editor`.
- [ ] Add `selecting a source tab changes the active source file`.
- [ ] Add `execute anonymous stays visible in advanced mode`.
- [ ] Add `dirty source tab shows a marker after edit`.

Focused web gate:

```bash
cd internal/playground/web
npm test -- --run
npm run build
```

### Task 3: Local Output And Database Layout

**Purpose:** Runs should show cause and effect without making the user hunt.

- [ ] Keep logs, vars, limits, trace, problems, and org diff in the right output pane.
- [ ] Move database browsing into the right output pane as a result tab, or keep it visible beside output. Do not make it replace both source and anonymous panes.
- [ ] Show scratch/persist mode where a user can see it without opening a hidden section.
- [ ] Keep local default as scratch.
- [ ] Allow local persist mode.
- [ ] Show the active DB path when a DB is configured.
- [ ] Add a small memory-only state when no DB path is configured.
- [ ] Keep seed and reset controls available in local mode.

Tests:

- [ ] Add `database browser does not replace anonymous editor`.
- [ ] Add `persist mode selector is visible in local mode`.
- [ ] Add `memory-only state appears when db path is empty`.

Focused gate:

```bash
cd internal/playground/web
npm test -- --run
```

### Task 4: Stronger Multi-File Examples

**Purpose:** The local gallery should show Glade touching DML, SOQL, triggers, selectors, services, limits, org diff, and database state in one short loop.

Do not add batch examples.

- [ ] Add `deal-desk-discount-guard`.

Files:

- `AccountDealScenario.cls`
- `DiscountPolicy.cls`
- `DealAccountSelector.cls`
- `DealDeskReport.cls`
- `AccountDealTrigger.trigger`
- `anonymous.apex`

Runtime story:

- Insert several Accounts with Industries and annual revenue fields that Glade supports.
- Trigger stamps `AccountNumber`.
- Service applies a discount bucket.
- Selector queries rows back.
- Report prints count, top bucket, DML rows, SOQL queries, and a row name.

- [ ] Add `renewal-health-scorecard`.

Files:

- `RenewalSeeder.cls`
- `RenewalAccountSelector.cls`
- `RenewalHealthRules.cls`
- `RenewalScorecard.cls`
- `anonymous.apex`

Runtime story:

- Insert Accounts and Contacts.
- Query Contacts by AccountId.
- Compute a health score in Apex.
- Print limits and selected database rows.

- [ ] Add `org-diff-review-loop`.

Files:

- `ReviewScenario.cls`
- `ReviewSelector.cls`
- `ReviewDecision.cls`
- `ReviewReport.cls`
- `anonymous.apex`

Runtime story:

- Insert an Account.
- Update it through a decision class.
- Query the final row.
- Make org diff show inserted and updated counts.

- [ ] Extend `TestComplexExampleProjectsRunAnonymous` with expected log fragments for each new example.
- [ ] Verify each example has at least three source files plus `anonymous.apex`.
- [ ] Keep descriptions short. Use tags for capability signals.

Focused gate:

```bash
go test ./internal/playground -run 'Test(ExampleProjectsRunAnonymous|ComplexExampleProjectsRunAnonymous)' -count=1
```

### Task 5: Local Command Improvements

**Purpose:** The command should make local demos repeatable without a hosted link.

- [ ] Add `--list-examples`.

Behavior:

- Prints built-in example ids, names, file counts, and tags.
- Includes project refs when `--project-ref` is supplied.
- Exits without starting the server.

- [ ] Add `--example <id>`.

Behavior:

- Implies `--examples` for built-in example ids.
- Starts or prints the URL with `?example=<id>`.
- Works with `--open`, `--once`, and `--wizard`.
- Errors if the id is unknown.

- [ ] Add `--no-db`.

Behavior:

- Sets `DBPath` to empty.
- Prints or exposes a memory-only state in the UI.
- Does not write `.glade/playground/org.sqlite`.

- [ ] Add `--reset-on-start`.

Behavior:

- Clears the selected workspace source and local org state before serving.
- Refuses to run with `--project <path>` unless `--force` already exists or a new explicit `--force-reset` is added. Do not delete a user's project source.
- Works for scratch workspaces and built-in example workspaces.

Tests:

- [ ] Add `TestRunPlaygroundListExamples`.
- [ ] Add `TestRunPlaygroundExampleFlagPrintsDeepLocalURL`.
- [ ] Add `TestRunPlaygroundExampleFlagImpliesExamples`.
- [ ] Add `TestRunPlaygroundNoDB`.
- [ ] Add `TestRunPlaygroundResetOnStartRefusesProjectRoot`.
- [ ] Extend wizard tests for `--example`, `--no-db`, and `--reset-on-start`.

Focused gate:

```bash
go test ./internal/gladecli -run 'TestRunPlayground(ListExamples|ExampleFlag|NoDB|ResetOnStart|Wizard)' -count=1
```

### Task 6: Local Docs And Help

**Purpose:** The docs should point to the local workbench and never imply a hosted playground exists.

- [ ] Update `site/docs-src/guide/playground.md`.
- [ ] Update `site/docs-src/guide/cli-reference.md`.
- [ ] Add a local examples table with id, name, and command.
- [ ] Document `--example`, `--list-examples`, `--no-db`, and `--reset-on-start`.
- [ ] Keep public publishing paused in `docs/PLAYGROUND_HOSTING.md`.
- [ ] Do not add `play.glade.sh`.

Verification:

```bash
rg -n 'play\.glade\.sh|https://play\.glade\.sh|Open playground|Open hosted' site docs Dockerfile .do README.md -g '!docs/superpowers/**'
```

Expected result:

```text
no matches
```

### Task 7: End-To-End Local Check

**Purpose:** Prove the local playground still works after the UI and command changes.

Run:

```bash
go test ./internal/playground ./internal/gladecli -count=1
cd internal/playground/web
npm test -- --run
npm run build
cd /Users/matt/Dev/glade
go run ./cmd/glade playground --examples --example deal-desk-discount-guard --once
git diff --check
```

Manual browser check:

```bash
go run ./cmd/glade playground --examples --example deal-desk-discount-guard --addr 127.0.0.1:1789 --open
```

Check these points:

- Source tabs appear above the source editor.
- Anonymous Apex is visible below source on desktop and mobile widths.
- Run produces logs, limits, org diff, and database rows.
- Persist mode works locally and shows its DB path.
- `--no-db` starts memory-only and writes no SQLite file.
- Reloading after a source save uses the new source without server restart.

## Done Criteria

- No public playground links remain in docs site sources or deploy config.
- Local playground still starts from `glade playground`.
- Built-in examples run through `internal/playground` tests.
- The UI shows multiple source files as tabs above code.
- Anonymous Apex stays visible below code.
- Source saves, deletes, and example loads invalidate runtime caches.
- New local flags have focused CLI tests.
- Web tests pass and embedded static assets are regenerated.
