# VS Code UI Polish Deep Dive

**Goal:** Make the Glade VS Code extension feel useful on first open, clear during normal Salesforce development, and hard to misuse in a full VS Code setup.

**Audience:** Salesforce developers who already use VS Code, Salesforce CLI, scratch orgs, source tracking, Apex tests, SOQL Builder, and Code Analyzer.

**Current Source Read:** `contrib/vscode-glade/package.json`, `src/extension.ts`, `src/views/*`, `src/localOrg.ts`, `src/projectContext.ts`, and the current installed sidebar screenshot.

**Visual Map:** `/tmp/picture-it/glade-vscode-ui-polish-map.html`

---

## Summary

The extension has the right raw boards. It has a Glade Activity Bar, project context, recommended runs, Apex Tests, Data Environments, Local Org, Debug And Logs, CodeLens, Test Explorer wiring, DAP, and local DB commands.

The UI still reads like a command inventory. It does not yet read like a daily tool.

The biggest polish win is to make the top of the Glade sidebar answer three questions:

1. Is this project ready for local Glade work?
2. What local data state am I using?
3. What should I run next?

The second win is to stop presenting environment operations as a flat command list. A developer expects row actions on the environment itself: switch, clone, seed, reset, export, inspect, reveal DB.

The third win is to remove dead or duplicate-looking surfaces. The current `Apex Tests` view is weaker than the native Testing API. If it is not a real tree, it should either become a compact local-test status view or disappear.

---

## Design Fences

Glade should not compete with Salesforce’s official surfaces.

Salesforce owns:

- Org deploy and retrieve.
- Org source tracking and conflict preview.
- Org SOQL Builder.
- Code Analyzer scans and quick fixes.
- Scratch-org and sandbox test runs.
- Salesforce Apex language features.

Glade owns:

- Local Apex proof.
- Local data state.
- Local VM debug.
- Local affected-test reruns.
- Trace or failure to local repro.
- A clean handoff back to Salesforce deploy and validate.

This split matches official product seams. Salesforce CLI project commands cover deploy, retrieve, preview, validate, and source tracking. Salesforce SOQL Builder is built around authenticated org query work. Salesforce Code Analyzer already has a VS Code scan loop, diagnostics, severities, quick fixes, and settings.

---

## Current UI Problems

### 1. First Open Looks Broken

The screenshot shows all view containers but every view says:

```text
There is no data provider registered that can provide view data.
```

That specific bug was packaging. Still, the product lesson stands. The sidebar needs intentional empty states.

Bad first impression:

- It looks like VS Code is broken.
- The user does not know whether they opened the wrong folder.
- The user does not know whether extension activation failed.
- There is no action in the empty state.

Target first impression:

- If no SFDX project: show `Open an SFDX project`.
- If Glade binary missing: show `Install Glade CLI` or `Run glade editor doctor vscode`.
- If local DB has not been inspected: show `Inspect active data`.
- If no tests are known: show `Refresh local tests`.
- If all ready: show `Run local proof`.

### 2. The Sidebar Has Too Many Equal-Weight Views

Today:

- Project
- Recommended Runs
- Apex Tests
- Data Environments
- Local Org
- Debug And Logs

All sit at the same level. That makes minor and major things look equal.

Better order:

1. Start Here
2. Local Runs
3. Data Environments
4. Local Org
5. Debug

`Apex Tests` should not be a separate view unless it displays a real useful local test tree. VS Code already has the native Testing view. Glade should contribute tests there, then use the sidebar for summaries and next actions.

### 3. Project View Is Informational, Not Operational

The current Project view shows root, namespace, API version, package dirs, and refresh.

That is useful, but not enough. A daily first panel should also show:

- Active environment.
- DB path and record count.
- Local proof action.
- Last run status.
- Watch status.
- LSP diagnostics state.

The user should not have to open five panels to know whether they can run a local test.

### 4. Recommended Runs Has No State

It currently shows commands:

- Run changed since origin/main.
- Run failed tests.
- Start watch.
- Stop watch.

A developer wants state:

- Changed since which ref?
- How many tests would run?
- When did it last run?
- What failed?
- Is watch already running?
- Is the daemon warm?

Commands are secondary. State comes first.

### 5. Data Environments Are Too Flat

The current environment panel has labels and global commands.

It should feel like a small local-org switcher:

- Active marker.
- Record count.
- Fixture path.
- Last seeded time.
- DB path.
- Row context actions.
- Inline action icons where they fit.

Global commands are still useful, but row-level actions are where the work belongs.

### 6. Local Org Is Invisible Until Manual Inspect

The Local Org view stays command-heavy until inspect runs. A user may not realize that inspect is the key step.

Better:

- Show active environment at the top even before inspect.
- Show DB path.
- Show `Not inspected yet`.
- Provide one primary row: `Inspect active data`.
- After inspect, show record/object counts.

### 7. Debug And Logs Is Too Thin

Debug is day-to-day useful for local Apex. The view should show:

- Breakpoint count.
- Active launch target.
- Active environment.
- Last failed test.
- `Debug last failure`.
- `Debug current test`.
- `Open trace/log output`.

If there is no last failure, say so. Empty state should teach the next action.

### 8. Commands Are Discoverable Only If You Already Know Them

Command Palette entries exist, but many users will not search `Glade`.

Need three command layers:

- View title buttons for primary actions.
- Row context menus for object-specific work.
- Command Palette for advanced or keyboard-driven users.

The menu text should be specific:

- `Glade: Run Local Proof`
- `Glade: Inspect Active Local Data`
- `Glade: Clone Local Data Environment`
- `Glade: Debug Last Local Failure`

Avoid generic titles like `Refresh local tests` when the user cannot see which surface it refreshes.

---

## Recommended Information Architecture

### Activity Bar

Keep one Activity Bar item:

- Title: `Glade`
- Icon: current `media/glade.svg`, but test it at 16px and 24px in light/dark themes.

Do not add more Activity Bar icons.

### Sidebar Views

Use this order:

1. `Start Here`
2. `Local Runs`
3. `Data Environments`
4. `Local Org`
5. `Debug`

Remove or hide:

- `Apex Tests` as a standalone sidebar view, unless it becomes a real local test tree. Native Testing API already owns the test tree.

Rename:

- `Recommended Runs` -> `Local Runs`
- `Debug And Logs` -> `Debug`
- `Project` -> `Start Here`

Keep existing view ids where possible to avoid VS Code layout churn:

- `glade.project` can render `Start Here`.
- `glade.recommendedRuns` can render `Local Runs`.
- `glade.debugLogs` can render `Debug`.

### Start Here Rows

Recommended rows:

```text
Ready
  Glade CLI found · SFDX root loaded

Project
  enterprise-composed · API 63.0 · namespace acme

Active data
  dev · 48 records · .glade/envs/dev.sqlite

Run local proof
  changed since origin/main

Last run
  Changed tests · 8 passed · 1 failed · 1.4s

Watch
  stopped · click to start

Salesforce handoff
  validate deploy after local proof
```

Only two or three rows should be clickable by default:

- Run local proof.
- Inspect active data.
- Start or stop watch.

The rest should be status.

### Local Runs Rows

Recommended rows:

```text
Changed tests
  origin/main · last run 1m ago · 8 passed · 1 failed

Failed tests
  1 failing · Debug first failure

Watch
  stopped · Start watch

Warm daemon
  unavailable or running
```

Add toolbar buttons:

- Refresh.
- Run changed.
- Start/stop watch.

### Data Environments Rows

Recommended rows:

```text
dev                         active · 48 records
bug-4821                    fixture: fixtures/bug-4821.json
migration                   0 records · not seeded
```

Row context actions:

- Switch.
- Clone.
- Inspect.
- Seed.
- Reset.
- Export.
- Reveal DB.
- Delete.

The active row should use a check icon. Other rows use a database icon.

### Local Org Rows

Recommended rows:

```text
Active: dev
DB: .glade/envs/dev.sqlite
Objects: 4
Records: 48
Account: 12
Contact: 34
User: 1
Profile: 1
```

View toolbar:

- Inspect.
- Seed.
- Reset.
- Export.

### Debug Rows

Recommended rows:

```text
Breakpoints
  3 Apex breakpoints

Last failure
  InvoiceServiceTest.updatesTotals

Debug current test
  available in Apex editor

Active data
  dev · .glade/envs/dev.sqlite

Trace/log
  Open Glade output
```

Primary action:

- `Debug last failure`, if one exists.

---

## Discoverability Work

### View Welcome Content

Add `contributes.viewsWelcome` entries.

Useful cases:

- No SFDX project.
- No local DB inspected.
- No test run yet.
- No data environments configured.
- No breakpoints.

Example copy:

```text
Open an SFDX project to use local Apex commands.
[Open Folder]
```

```text
No local data has been inspected yet.
[Inspect Active Data] [Create Environment]
```

This is better than a blank tree, and it is native VS Code.

### View Title Actions

Add `menus.view/title` actions:

- Start Here: refresh, run local proof.
- Local Runs: run changed, start/stop watch.
- Data Environments: create, inspect active.
- Local Org: inspect, seed, export.
- Debug: debug current test, open output.

Keep icons from VS Code product icons:

- `refresh`
- `run`
- `debug-start`
- `database`
- `add`
- `save`
- `bug`

### Row Context Actions

Use `TreeItem.contextValue`.

Examples:

- `gladeEnvironmentActive`
- `gladeEnvironment`
- `gladeFailedTest`
- `gladeBreakpoint`
- `gladeLocalOrgObject`

Context menus should be specific to the row. No global command pile.

### Status Bar

Add one left-side or right-side status item:

```text
Glade: dev · local
```

On click:

- Open Glade sidebar.
- Or run `Glade: Run Local Proof` if ready.

When there is a failure:

```text
Glade: 1 local fail
```

Do not make it noisy. No animations. No color unless failing.

### Command Palette Polish

Use consistent names:

- `Glade: Run Local Proof`
- `Glade: Run Local Changed Tests`
- `Glade: Debug Local Current Test`
- `Glade: Inspect Active Local Data`
- `Glade: Create Local Data Environment`
- `Glade: Clone Local Data Environment`

Avoid:

- `Run failed tests` without `Local`.
- `Inspect Local Org` when the user thinks in environments.
- `Debug And Logs`.

### First-Run Doctor

After activation, silently compute readiness:

- SFDX project found.
- `glade` found on PATH.
- VSIX version.
- Active DB path exists.

Do not pop notifications unless an action fails. Show readiness in Start Here.

### Error Surfaces

Every failed CLI command should offer:

- `Open Glade Output`
- `Copy Command`
- `Run Doctor`

The output should include the exact command. That saves time.

---

## Visual Polish

### Icons

Tree rows need icons.

Use product icons, not custom art:

- Project: `root-folder`
- Ready: `pass`
- Warning: `warning`
- Data environment: `database`
- Active environment: `check`
- Run: `run`
- Watch: `eye` or `sync`
- Debug: `debug-alt`
- Failure: `error`
- Output: `output`

### Labels

Use labels that start with the object, not the action.

Better:

```text
dev
  active · 48 records
```

Worse:

```text
Active: dev
```

Use `description` for secondary data. Use `tooltip` for full paths and command details.

### Density

Tree views should stay compact. Avoid paragraphs. If a row needs more than one short line, move detail to tooltip or output.

Good row:

```text
Changed tests        8 passed, 1 failed
```

Bad row:

```text
Run the tests that Glade thinks are affected by changes since origin/main.
```

### Empty States

Each view needs one empty-state row with one action.

Examples:

- `Open an SFDX project`
- `Inspect active data`
- `Run local proof`
- `Start watch`
- `Create environment`

Do not show five commands in an empty state.

### Loading States

Use temporary rows:

```text
Inspecting dev...
Running changed tests...
Starting watch...
```

This matters because many Glade commands run through child processes. Silence feels broken.

---

## Feature Polish By Surface

### Start Here

Highest value.

Build it as the daily dashboard. It should have no more than seven rows. It should be the one view a developer can leave open during normal work.

Must have:

- Readiness.
- Project root.
- Active environment.
- Local proof.
- Last run.
- Watch.

Nice to have:

- Handoff row after proof passes.
- `Copy local proof command`.
- `Run doctor`.

### Local Runs

Rename from `Recommended Runs`.

The view should show state first, commands second.

Must have:

- Changed since ref.
- Last changed-test result.
- Last failed count.
- Watch status.
- Start/stop watch action.

Nice to have:

- Changed since picker.
- Slowest tests.
- Recently failed tests.

### Data Environments

This should feel like a local scratch-org shelf.

Must have:

- Active marker.
- Record count after inspect.
- Row action menus.
- Clone from row.
- Seed/reset/export from row or active row.
- Reveal DB.

Nice to have:

- Last seed fixture.
- Last seed time.
- Branch-derived quick create.
- Archive/delete DB file option with confirmation.

### Local Org

This view should show facts about the active environment.

Must have:

- Active environment name.
- DB path.
- Inspect button.
- Object and record counts.
- Per-object counts.

Nice to have:

- Copy fixture for object.
- Open DB in external tool.
- Object filter.

### Debug

This should bridge tests, breakpoints, and active data.

Must have:

- Breakpoint count.
- Active data environment.
- Last failure.
- Debug current test.
- Debug last failure.

Nice to have:

- Variables snapshot after debug session.
- Link from failed test to debug config.
- Trace/log analysis row.

---

## Recommended Build Order

### Pass 1: Make It Stop Looking Empty

1. Add `viewsWelcome`.
2. Rename visible views.
3. Add icons to tree rows.
4. Add one action per empty state.
5. Add view title refresh and run buttons.

This is the fastest visible improvement.

### Pass 2: Build Start Here

1. Replace `ProjectView` with `StartHereView`.
2. Add `StartHereState`.
3. Feed it project, active env, last run, watch state, and inspect summary.
4. Add `Run Local Proof`.

This turns the extension into a daily tool.

### Pass 3: Build Environment Rows

1. Add row context values.
2. Add switch/clone/inspect/reveal/delete row commands.
3. Add active marker and icons.
4. Add DB inspect summaries beside environment names.

This makes local data feel concrete.

### Pass 4: Debug Polish

1. Track last failure.
2. Add `Debug last failure`.
3. Show active environment in debug row.
4. Show breakpoint count and file links.

This makes local DAP visible and useful.

### Pass 5: Ship Quality

1. Package test must assert runtime dependencies ship.
2. Manifest tests must assert view names and command contributions.
3. Screenshot smoke after install.
4. Extension host activation smoke in a sample SFDX project.

This prevents the blank-provider failure from coming back.

---

## Implementation Notes

### Keep The Native Tree For Now

Do not jump to a webview dashboard yet.

Tree views are enough for:

- Status rows.
- Command rows.
- Row context menus.
- View title actions.
- Welcome content.

A webview buys visual freedom, but costs accessibility, theme consistency, testing, keyboard behavior, and native VS Code feel. Save it for a real local DB record browser.

### Use Webview Only For A Future DB Browser

The first real webview candidate is `Local Record Browser`:

- Table grid.
- Search/filter.
- Export selected object.
- Copy selected rows as seed fixture.

That should be separate from the basic environment shelf.

### Treat Output As A Detail Drawer

Every CLI-backed action should write:

```text
$ glade ...
```

Then the JSON or human result.

Views should stay short. Output carries detail.

### Use Workspace Settings As The Source Of Truth

Keep environment definitions in workspace settings:

- `glade.environments`
- `glade.activeEnvironment`
- `glade.changedSince`

Do not invent a side database for UI state. Runtime state like last run and watch status can live in memory for now.

---

## Acceptance Criteria

### First Open

- Opening Glade in a non-SFDX folder shows a clear `Open an SFDX project` row.
- Opening Glade in an SFDX project shows `Start Here` with project and active data rows.
- No view displays VS Code’s generic `no data provider` message.

### Daily Loop

- A developer can run local proof from the first panel.
- A developer can see which DB is active without opening settings.
- A developer can inspect, seed, reset, and export the active environment from visible UI.
- A developer can start and stop watch from visible UI.

### Environment Work

- Environment rows show active state.
- Context menu has clone, inspect, reveal, and delete.
- Cloning copies the DB file when it exists.
- Deleting `dev` is blocked.
- Deleting another environment asks for confirmation.

### Debug Work

- Debug view shows active environment.
- Debug view shows breakpoint count.
- Debug view offers debug current test and debug last failure when available.

### Packaging

- VSIX includes runtime dependencies.
- Tests fail if `node_modules/**` is blanket-excluded again.
- Install command still works from source checkout and release archive.

---

## Source Notes

- VS Code UX guidance says views can be Tree Views, Welcome Views, or Webview Views, and view-specific actions can live in View Toolbars. Use those native surfaces before reaching for a webview.
- VS Code Activity Bar guidance favors clear names and icons matching native style.
- VS Code sidebar guidance favors grouping related views and clear descriptive names.
- VS Code Tree View guidance favors descriptive labels and product icons.
- Salesforce CLI project commands own deploy, retrieve, preview, validate, and source tracking.
- Salesforce SOQL Builder is already a visual, authenticated-org query workflow.
- Salesforce Code Analyzer already owns scan, diagnostic, severity, and quick-fix loops in VS Code.

## Priority Table

| Priority | Change | Why |
| --- | --- | --- |
| P0 | Prevent no-provider view states | First impression and activation confidence |
| P0 | Package runtime dependencies | Sidebar cannot work without activation |
| P1 | Rename and build Start Here | Gives developer one front door |
| P1 | Add viewsWelcome | Empty states become action states |
| P1 | Add environment row actions | Makes local data useful |
| P1 | Add view title actions | Makes commands discoverable |
| P2 | Add status bar item | Keeps active env visible outside sidebar |
| P2 | Debug last failure | Turns local DAP into daily workflow |
| P3 | Webview DB browser | Useful later, not needed for first polish |
