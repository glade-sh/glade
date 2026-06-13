# VS Code Extension UI Research For Glade

**Goal:** Shape the Glade VS Code extension into a daily Salesforce development tool, not a command shelf.

**Audience:** Salesforce developers who already live in VS Code with Salesforce CLI, scratch orgs, Apex tests, source tracking, Code Analyzer, SOQL Builder, Git, and a terminal.

**Mockups:**

- `/tmp/picture-it/glade-vscode-extension-research-mockups.html`
- `/tmp/picture-it/glade-vscode-extension-research-mockups.svg`
- `/tmp/picture-it/glade-vscode-extension-brand-iteration.html`
- `/tmp/picture-it/glade-vscode-extension-brand-iteration.svg`

---

## Summary

The best VS Code extensions do not ask the user to move into a new product.
They make the current editor sharper.

The pattern is consistent across Python, GitLens, GitHub Pull Requests, ESLint, Prettier, Error Lens, Container Tools, and Thunder Client:

1. Put state where the user already looks.
2. Put actions on the thing they act on.
3. Use native VS Code surfaces before webviews.
4. Keep the sidebar for durable workspace state.
5. Keep the editor for code-specific action.
6. Keep notifications scarce.
7. Keep full logs out of the way until needed.

For Glade, that means:

- A single Glade Activity Bar item.
- A useful `Start Here` view at the top.
- Native Testing API for local Apex tests.
- Status Bar for the active local data environment and last local result.
- CodeLens, hovers, gutter actions, and diagnostics in Apex files.
- Tree views for local runs, data environments, local org state, and debug state.
- Webviews only for a future record/data browser or rich run report.

The first release should feel native and quiet. A good tool does not wave its arms.

---

## Sources Read

Official VS Code guidance:

- [UX Guidelines overview](https://code.visualstudio.com/api/ux-guidelines/overview)
- [Activity Bar guidelines](https://code.visualstudio.com/api/ux-guidelines/activity-bar)
- [Views guidelines](https://code.visualstudio.com/api/ux-guidelines/views)
- [Status Bar guidelines](https://code.visualstudio.com/api/ux-guidelines/status-bar)
- [Notifications guidelines](https://code.visualstudio.com/api/ux-guidelines/notifications)
- [Quick Picks guidelines](https://code.visualstudio.com/api/ux-guidelines/quick-picks)
- [Command Palette guidelines](https://code.visualstudio.com/api/ux-guidelines/command-palette)
- [Context Menus guidelines](https://code.visualstudio.com/api/ux-guidelines/context-menus)
- [Webviews guidelines](https://code.visualstudio.com/api/ux-guidelines/webviews)
- [Walkthroughs guidelines](https://code.visualstudio.com/api/ux-guidelines/walkthroughs)
- [Testing API guide](https://code.visualstudio.com/api/extension-guides/testing)
- [Extension capabilities overview](https://code.visualstudio.com/api/extension-capabilities/overview)

Comparable extensions:

- [Python extension](https://marketplace.visualstudio.com/items?itemName=ms-python.python)
- [Python testing in VS Code](https://code.visualstudio.com/docs/python/testing)
- [Python environments in VS Code](https://code.visualstudio.com/docs/python/environments)
- [GitLens](https://marketplace.visualstudio.com/items?itemName=eamodio.gitlens)
- [GitHub Pull Requests](https://marketplace.visualstudio.com/items?itemName=GitHub.vscode-pull-request-github)
- [ESLint](https://marketplace.visualstudio.com/items?itemName=dbaeumer.vscode-eslint)
- [Prettier](https://marketplace.visualstudio.com/items?itemName=esbenp.prettier-vscode)
- [Error Lens](https://marketplace.visualstudio.com/items?itemName=usernamehw.errorlens)
- [Container Tools overview](https://code.visualstudio.com/docs/containers/overview)
- [Thunder Client](https://marketplace.visualstudio.com/items?itemName=rangav.vscode-thunder-client)

Salesforce-adjacent surfaces:

- [Salesforce CLI project commands](https://developer.salesforce.com/docs/platform/salesforce-cli-reference/guide/cli_reference_project.html)
- [Salesforce SOQL Builder](https://developer.salesforce.com/docs/platform/sfvscode-extensions/guide/soql-builder.html)
- [Salesforce Code Analyzer in VS Code](https://developer.salesforce.com/docs/platform/salesforce-code-analyzer/guide/analyze-vscode.html)

---

## What Popular Extensions Teach

### Python

The Python extension has a broad surface: IntelliSense, debugging, linting, formatting, refactoring, unit tests, environment management, and Test Explorer integration.

It works because it does not make one giant Python dashboard. It fans work into native VS Code places:

- Interpreter/env selection in the Status Bar.
- Tests in the native Testing view.
- Debug through VS Code debug surfaces.
- Lint output in Problems and diagnostics.
- Commands in the Command Palette.

**Glade lesson:** Local Apex can be broad if each piece lands in the native surface. Do not force tests, debug, data, and logs into one custom panel.

### Python Testing

Python testing uses the native Testing view, supports configuration flows, refresh, run, debug, failed tests, and last-run style commands.

**Glade lesson:** Apex tests should appear in the native Testing view. The Glade sidebar should show summaries and next actions, not duplicate the full test tree.

### Python Environments

Python environment work has three parts:

- Active environment visible.
- Environment creation, deletion, and switching.
- Environment manager views grouped by type.

**Glade lesson:** Local data environments should be first-class state. The active environment belongs in the Status Bar and `Start Here`. The environment list belongs in a tree view with row actions.

### GitLens

GitLens is useful because it adds context at the point of work:

- Inline blame.
- CodeLens.
- Hovers.
- Status Bar blame.
- Home view for workflow state.
- Deeper commit graph only when needed.

**Glade lesson:** Put local Apex context in editor hovers and CodeLens. Show last local run, affected tests, debug actions, and data environment without sending the user into a separate control room.

### GitHub Pull Requests

The GitHub PR extension keeps PR work inside VS Code:

- Activity Bar view.
- List and browse PRs.
- In-editor comments.
- Hover cards.
- Terminal integration so UI and CLI can co-exist.

**Glade lesson:** Glade should make `glade` CLI output and VS Code UI co-exist. Every UI action should reveal the exact CLI command and output when it fails.

### ESLint, Prettier, Error Lens

These tools win by being quiet until they have something useful.

- Prettier formats without a sidebar.
- ESLint reports diagnostics and code actions.
- Error Lens makes existing diagnostics easier to see.

**Glade lesson:** Parser, semantic, and local-test failures should use Problems, diagnostics, CodeLens, and editor decorations. Do not invent a second Problems panel.

### Container Tools

Container Tools uses a resource explorer for assets: containers, images, volumes, networks, registries. Common work sits on right-click menus for each asset.

**Glade lesson:** Data Environments should feel like a local asset explorer. A row should carry `switch`, `clone`, `seed`, `reset`, `export`, `inspect`, and `reveal DB`.

### Thunder Client

Thunder Client proves that custom in-editor tools can work when the product needs them. It focuses on local storage, environments, collections, and a clean API testing flow.

**Glade lesson:** A future data browser webview can work, but only when tree views and Quick Picks are too small for the job. Use webviews for tabular records, diffs, and run reports. Not for the first sidebar.

---

## VS Code UX Rules That Matter Most

### Activity Bar

Use one Glade item. It needs a clear icon and a clear name.

Do not add separate icons for tests, data, debug, or org. VS Code already has core icons for Explorer, Search, Source Control, Run and Debug, and Testing.

### Tree Views

Tree views can show flat lists or nested state. The guidelines call out descriptive labels, product icons, limited nesting, and no more than three actions per item.

For Glade:

- Top-level views should stay shallow.
- Data environments can have one nested level.
- Local Org can have object summaries under an inspected environment.
- Debug can have breakpoints and last failure under one compact tree.

### Welcome Views

Use Welcome content when a view is empty.

For Glade:

- No SFDX project: one link to open project docs, one action to run doctor.
- No local DB: one action to inspect or create environment.
- No tests: one action to refresh local tests.
- No debug state: one action to debug current test.

Keep welcome text short. Use one primary action. No sales copy.

### Status Bar

The Status Bar is crowded. The VS Code guideline says to limit items, use short labels, and avoid custom colors except for special blocking cases.

For Glade, use one item:

- `Glade: dev`
- `Glade: dev 3 fail`
- `Glade: seeding...`
- `Glade: no DB`

Click opens a Quick Pick:

1. Switch data environment.
2. Inspect active data.
3. Run local proof.
4. Open Glade output.

### Notifications

Notifications should be rare.

For Glade:

- No success popups after normal runs.
- No popup on extension activation.
- No popup after every refresh.
- Use progress in the relevant view or Status Bar.
- Use error notifications only when action failed and the user needs a button.

Good error actions:

- `Show Output`
- `Run Doctor`
- `Open Settings`
- `Retry`

### Quick Picks

Use Quick Picks for selection and short setup. Use descriptions and brief details. Use separators when choices have groups.

For Glade:

- Switch environment.
- Pick test target.
- Pick affected-test base ref.
- Pick seed file.
- Pick debug target.

Do not build a long wizard. If a flow has more than three steps, it wants a command, a generated config file, or a webview.

### Command Palette

Commands need clear names and one category.

Use:

- `Glade: Run Local Proof`
- `Glade: Debug Current Apex Test`
- `Glade: Switch Local Data Environment`
- `Glade: Inspect Active Local Data`
- `Glade: Open Output`

Avoid:

- `Refresh`
- `Run`
- `Inspect`
- `Start`

### Context Menus

Context menus should appear only when they apply.

For Glade:

- Apex file context: run local tests in file, debug test in file, show local dependencies.
- Apex test method CodeLens: run local, debug local.
- Environment row: switch, clone, seed, reset, inspect, export, reveal DB.
- Local object row: inspect records, export records, clear object records.

### Webviews

The guidance is blunt: use webviews only when needed.

Use a webview later for:

- Local record browser.
- SOQL result table over local DB.
- Run report with failure stack, covered methods, affected dependencies, and fixture diffs.
- Environment diff/clone preview.

Do not use a webview for:

- Start Here.
- Simple environment list.
- Run buttons.
- Settings.
- Empty states.

---

## What People Tend To Like

This is an inference from popular extension design and install-heavy patterns, not a formal survey.

Developers keep extensions that:

- Save context switches.
- Give one obvious next action.
- Put details behind hover, row action, or output.
- Show exact local state.
- Respect existing VS Code muscle memory.
- Work from the editor, not only from a sidebar.
- Fail with the exact command and log visible.
- Stay quiet when work succeeds.
- Let terminal and UI co-exist.
- Do not create a new mental model for work VS Code already handles.

Developers remove or ignore extensions that:

- Open webviews on startup.
- Notify too often.
- Put too many buttons in a sidebar.
- Require setup before showing any value.
- Duplicate built-in views.
- Hide the command they ran.
- Show empty panels without actions.

---

## Information Density Rules For Glade

### Start Here

Show 5 to 7 rows. No more.

Recommended rows:

1. `Ready for local Apex` or blocking state.
2. Active project/package dir.
3. Active local data environment.
4. Local DB summary.
5. Last local proof.
6. Watch or daemon state.
7. Primary action.

Full paths belong in tooltips. Full logs belong in Output.

### Local Runs

Show current and recent run state.

Rows:

- `Run changed since origin/main` with estimated test count.
- `Run failed local tests` with failure count.
- `Run current file` when an Apex file is active.
- `Start watch` or `Stop watch`.
- Last 3 local runs.

Avoid a long run history. Use Output or future run report for that.

### Data Environments

Each environment row should show:

- Name.
- Active marker.
- Record count, if inspected.
- Fixture or DB hint.
- Last seed/reset time, if known.

Tooltip should show:

- DB path.
- Fixture path.
- Exact seed command.
- Exact inspect command.

Row actions should be:

- Inline: switch, clone, inspect.
- Context menu: seed, reset, export, reveal DB, delete.

### Local Org

Before inspect:

- Active env.
- DB path.
- `Not inspected yet`.
- One action: inspect.

After inspect:

- Total records.
- Object count.
- Top changed objects.
- Objects with local data.

Do not show every object by default if the list is long. Start with changed/non-empty objects.

### Debug

Show:

- Breakpoint count.
- Active environment.
- Last failed test.
- Current Apex file target.
- Debug current test.
- Debug last failure.
- Open trace output.

The debugger itself belongs in VS Code Run and Debug. This view should prepare the launch and expose local Apex state.

### Status Bar

One item.

Use:

- `Glade: dev`
- `Glade: dev 18ms`
- `Glade: dev 2 fail`
- `Glade: seeding`
- `Glade: no DB`

Do not show package dir, branch, API version, DB path, and failure count all at once.

### Notifications

Use no notification for normal success.

Use a warning/error notification when:

- Glade CLI is missing.
- The command failed before producing output.
- A local data operation would delete data.
- A debug session cannot attach.

Always include `Show Output` when command output exists.

---

## Recommended Glade Shape

### Sidebar

Final sidebar order:

1. `Start Here`
2. `Local Runs`
3. `Data Environments`
4. `Local Org`
5. `Debug`

Do not keep a standalone `Apex Tests` sidebar view unless it has a distinct job. The native Testing view should own the local Apex tree.

### Editor

For Apex tests:

- CodeLens above test class: `Run Local Tests | Debug Local Tests`
- CodeLens above test method: `Run Local | Debug Local`
- Hover on local failure: failure message, top stack frame, active environment.
- Gutter breakpoint support through DAP.

For Apex implementation files:

- CodeLens: `Run Affected Local Tests`
- Hover: local dependency summary if cheap.
- Diagnostics: parser and semantic issues through Problems.

### Testing View

Use one Glade test controller:

- Label: `Glade Local Apex`
- Items: Apex classes and test methods.
- Tags: `local`, `affected`, `failed`, maybe `requiresData`.
- Profiles: `Run Local`, `Debug Local`.

This is how Glade separates local runs from org-backed runs.

### Data

Make the active data environment impossible to miss but not loud:

- Status Bar: `Glade: dev`
- Start Here row: `Data env: dev`
- Data Environments row: active marker.
- Debug row: `Env: dev`
- Run output header: `Environment: dev`

This prevents a bad local run where the developer used the wrong data.

### Org Boundary

Glade should not copy org-backed jobs.

Do not build:

- Org deploy/retrieve UI.
- Scratch org test runner clone.
- Org SOQL Builder clone.
- Code Analyzer clone.

Build:

- Local run proof before deploy.
- Local affected test loop.
- Local data state manager.
- Local debug session.
- Local failure explanation.
---

## Mockup Notes

The mockups use the current Glade VS Code logo from:

```text
contrib/vscode-glade/media/glade.svg
```

The second iteration also follows the current site brand guide:

- Glade green `#9BE870` for local-ready state, primary local actions, and successful proof.
- Glade strong `#B7FF8A` for active focus and strong selected state.
- Near-black `#070B0D`, surface `#10191E`, raised surface `#152229`, and line `#26363D` for Glade-owned visual material.
- Warning `#F5C95F`, danger `#FF6B61`, and info `#7DB7FF` only for named runtime states.

Inside VS Code, use brand color with restraint. The Activity Bar icon should stay legible in the user theme. Tree rows, labels, Testing, Debug, Output, and Status Bar should look native first. Glade green should mean "local state is active or proven," not decoration.

They show three states:

1. **Daily Local Loop**
   - Start Here ready state.
   - Active data environment.
   - CodeLens in Apex file.
   - Native Testing view present.
   - Output panel with exact command.

2. **Data Environment Manager**
   - Environments as assets.
   - Active marker.
   - Row context menu.
   - Local Org summary.
   - Future record browser as a tab, not a sidebar replacement.

3. **Debug Last Failure**
   - Breakpoint visible in editor.
   - Debug view with last failure.
   - Call stack and variables in native debug panel.
   - Glade output showing DAP launch command.

This is the shape to build toward.

---

## Build Recommendations

### Must Do First

1. Fix activation/package tests so view providers cannot go missing again.
2. Rename `Project` to `Start Here`.
3. Remove or hide the weak standalone `Apex Tests` sidebar.
4. Add one Status Bar item.
5. Add Welcome content for every empty view.
6. Add row context menus for environments.
7. Wire local Apex tests into the native Testing API.

### Next

1. Add CodeLens for Apex test classes and methods.
2. Add `Run Local Proof` as the first primary workflow.
3. Add `Debug Current Test` and `Debug Last Failure`.
4. Persist last run, last failure, active env, and last inspect state.
5. Show exact command and logs in the Glade Output channel.
6. Add Quick Picks for switch env, seed env, and pick debug target.

### Stretch

1. Local record browser webview.
2. Local SOQL result table.
3. Run report webview.
4. Environment diff and clone preview.
5. Affected-test explanation view.
6. Test fixture coverage hints.
7. Status Bar latency and warm-daemon signal.

---

## Final Position

Glade should feel like the local half of Salesforce development.

Org-backed tools answer: what is in the org, what deploys, what validates in Salesforce.

Glade answers: what happens here, with this code, this data, and this local Apex run.

That is enough work for one sharp tool.
