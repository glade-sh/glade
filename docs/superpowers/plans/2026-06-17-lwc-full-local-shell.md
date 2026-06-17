# LWC Full Local Shell Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` for implementation. The implementing model should be GPT-5.5 High. Dispatch parallel subagents by squad, keep file ownership disjoint, and integrate through a main conductor after each phase. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a full local Salesforce-like shell for running LWCs with real Glade context, Apex controller calls, LDS/UI API services, navigation, base components, SLDS styling, Visualforce Lightning Out parity, and easy context selection.

**Architecture:** Keep the POC from `codex/lwc-vf-preview-feature` as the baseline. That branch is already an ancestor of `main`; do not cherry-pick it. Build a Glade-owned Lightning shell inspired by `salesforce-ux/design-system-2-starter-kit`, but generate routes, context, page placement, and data from Salesforce metadata and local org state instead of static starter-kit config.

**Tech Stack:** Go 1.26, `internal/lwcshell`, `internal/lwcbrowser`, `internal/server`, `internal/gladecli`, `lwcruntime`, third-party LWC compiler/engine, local Apex VM, local storage/DML/schema, Playwright, VitePress docs, sibling `glade-tools`, Salesforce CLI, scratch org alias `oaer-probe-max`.

---

## Design Boundary

Do not fork `salesforce-ux/design-system-2-starter-kit` whole.

Use it as a reference for:

- App shell shape.
- Global header.
- App navigation.
- Standard app and console app modes.
- Theme loading.
- SLDS 1 and SLDS 2 coexistence.
- Synthetic shadow expectations.
- Base component assumptions.
- Local router ergonomics.

Glade must own:

- Metadata-driven route discovery.
- FlexiPage placement.
- Custom tab resolution.
- Context presets.
- Local record state.
- Apex execution through the VM.
- LDS/UI API service routes.
- Visualforce Lightning Out shared runtime.
- Support diagnostics and oracle evidence.

Primary references:

- `https://github.com/salesforce-ux/design-system-2-starter-kit`
- `https://developer.salesforce.com/docs/platform/lwc/guide/get-started-test-components.html`
- `https://developer.salesforce.com/docs/platform/lwc/guide/reference-configuration-tags.html`
- `https://developer.salesforce.com/docs/platform/lwc/guide/reference-page-reference-type.html`
- `https://developer.salesforce.com/docs/platform/lwc/guide/reference-ui-api.html`
- `https://developer.salesforce.com/docs/platform/lwc/guide/reference-salesforce-modules.html`
- `/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run/lwc`
- `/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run/metadata-api/meta_flexipage.md`

## Starting Baseline

The POC branch is already in `main`:

```bash
git merge-base --is-ancestor codex/lwc-vf-preview-feature HEAD
# expected exit code: 0
```

Use these existing pieces:

- `/Users/matt/Dev/glade/internal/gladecli/dev_lwc_command.go`
- `/Users/matt/Dev/glade/internal/server/lwc_shell.go`
- `/Users/matt/Dev/glade/internal/lwcshell/model.go`
- `/Users/matt/Dev/glade/internal/lwcshell/resolve.go`
- `/Users/matt/Dev/glade/internal/lwcshell/flexipage.go`
- `/Users/matt/Dev/glade/internal/lwcshell/tab.go`
- `/Users/matt/Dev/glade/internal/lwcbrowser/bootstrap.go`
- `/Users/matt/Dev/glade/internal/lwcbrowser/salesforce_modules.go`
- `/Users/matt/Dev/glade/internal/server/lightning_wire.go`
- `/Users/matt/Dev/glade/internal/server/lightning_shims.go`
- `/Users/matt/Dev/glade/lwcruntime/src/glade.out.mjs`
- `/Users/matt/Dev/glade/lwcruntime/src/shims/wire-adapter.mjs`
- `/Users/matt/Dev/glade/lwcruntime/test/lwc-dev-server.test.mjs`
- `/Users/matt/Dev/glade/testdata/local-tests/lwc-shell`
- `/Users/matt/Dev/glade/testdata/local-tests/lightning-out-vf`
- `/Users/matt/Dev/glade/site/docs-src/guide/lwc-local-shell.md`

Current routes stay supported:

```text
/lwc/preview/component/<namespace>/<component>
/lwc/preview/record/<Object>/<recordId>?page=<FlexiPage>
/lwc/preview/app/<Page>
/lwc/preview/home/<Page>
/lwc/preview/tab/<Tab>
/apex/<PageName>
```

The new work adds the porcelain on top:

```bash
glade dev lwc --project . --open
glade dev lwc --project . --context accountDemo --open
glade dev lwc --project . --target record-page --object Account --record 001000000000001AAA --page Account_Record_Page --open
```

## Worktree Layout

Use paired worktrees only when `glade-tools` oracle work is active. Product shell work can use one Glade worktree.

Create the product worktree:

```bash
git -C /Users/matt/Dev/glade worktree add /Users/matt/Dev/lwc-full-shell/glade -b codex/lwc-full-local-shell
```

Create the oracle worktree when Phase 10 starts:

```bash
git -C /Users/matt/Dev/glade-tools worktree add /Users/matt/Dev/lwc-full-shell/glade-tools -b codex/lwc-full-local-shell-tools
```

Rules:

- Product code stays in `/Users/matt/Dev/lwc-full-shell/glade`.
- Oracle and generated compatibility reports stay in `/Users/matt/Dev/lwc-full-shell/glade-tools`.
- Do not add a product dependency from Glade to `glade-tools`.
- Use one branch per subagent squad only when two squads must write at the same time.
- Merge squad branches into the product worktree through the conductor.

## Global Gates

Run focused gates after each phase:

```bash
cd /Users/matt/Dev/lwc-full-shell/glade
go test ./internal/lwcshell ./internal/lwcbrowser ./internal/server ./internal/gladecli -run 'LWC|Lightning|Shell|Context|Navigation|Wire|Base|Dev' -count=1
npm --prefix lwcruntime test
```

Run broad gates after integrated feature phases:

```bash
cd /Users/matt/Dev/lwc-full-shell/glade
go test ./...
scripts/smoke.sh
npm --prefix site test
npm --prefix site run build
```

Run oracle gates when `glade-tools` changes:

```bash
cd /Users/matt/Dev/lwc-full-shell/glade-tools
go test ./...
go run ./cmd/glade-plugin-compat lwc capture \
  --target-org oaer-probe-max \
  --project /Users/matt/Dev/lwc-full-shell/glade/testdata/local-tests/lwc-shell \
  --include-hosts lightning-shell,visualforce-lightning-out \
  --out /tmp/glade-lwc-full-shell-capture.json
```

Expected after final phase:

```text
all claimed hosts pass
no unknown support rows
no browser console errors for claimed fixtures
```

## Parallel Squad Model

Use these squads. Do not let two squads edit the same files at the same time.

- **Shell UI squad:** `lwcruntime/src/shell`, shell CSS, shell browser behavior.
- **Server shell squad:** `internal/server/lwc_shell*.go`, shell HTML and endpoints.
- **Context squad:** `internal/lwcshell/context*.go`, context presets, target resolution.
- **Metadata squad:** LWC meta, FlexiPage, CustomTab, CustomApplication parsing.
- **CLI squad:** `internal/gladecli/dev_lwc_command*.go`, help, open behavior, ready file.
- **Runtime shim squad:** `internal/lwcbrowser`, import maps, bootstrap, shared config.
- **Apex/LDS squad:** `internal/server/lightning_wire.go`, Apex invocation, UI API routes, local LDS cache contract.
- **Base component squad:** `lwcruntime/src/lightning`, base component modules, events.
- **SLDS/theme squad:** SLDS assets, shell theme loader, icon paths.
- **Visualforce host squad:** `/apex` Lightning Out parity and shared service behavior.
- **Oracle squad:** sibling `glade-tools`, scratch capture, support ledger.
- **Docs/support squad:** docs, site, support tables.
- **Review squad:** read-only review after each integrated phase.

---

## Phase 0: Baseline Audit And Starter-Kit Extraction Notes

**Goal:** Freeze the current POC baseline and record exactly what to reproduce from the starter kit.

**Files:**

- Create: `/Users/matt/Dev/lwc-full-shell/glade/reports/lwc-full-shell-baseline.md`
- Modify only if stale: `/Users/matt/Dev/lwc-full-shell/glade/docs/LWC_LOCAL_SHELL.md`
- Modify only if stale: `/Users/matt/Dev/lwc-full-shell/glade/site/docs-src/guide/lwc-local-shell.md`

**Steps:**

- [ ] Confirm the POC branch is merged:

```bash
cd /Users/matt/Dev/lwc-full-shell/glade
git merge-base --is-ancestor codex/lwc-vf-preview-feature HEAD
echo $?
```

Expected:

```text
0
```

- [ ] Run current focused shell tests:

```bash
go test ./internal/lwcshell ./internal/lwcbrowser ./internal/server ./internal/gladecli -run 'LWC|Lightning|Shell|Dev' -count=1
npm --prefix lwcruntime test -- --test-name-pattern='lwc|visualforce|wire|mount|events'
```

Expected: pass, or record named baseline failures in the report.

- [ ] Start the current shell:

```bash
go run ./cmd/glade dev lwc --project testdata/local-tests/lwc-shell --addr 127.0.0.1:0 --ready-file /tmp/glade-lwc-ready.json
```

Expected: ready file contains base URL and current preview routes.

- [ ] In `reports/lwc-full-shell-baseline.md`, record the starter-kit pieces to reproduce:

```text
global header
app launcher
standard app tab bar
console mode side rail
theme switcher
client router
route-level page components
SLDS 1/SLDS 2 loader shape
synthetic shadow expectation
base component package strategy
```

**Done Gate:**

- Current POC tests are measured.
- The starter-kit extraction notes name concrete shell pieces.
- No implementation starts from unknown shell breakage.

## Phase 1: Shell Runtime And Workbench UI

**Goal:** Replace the small inline shell HTML with a real Glade Lightning Shell workbench.

**Files:**

- Create: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcshell/workbench.go`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcshell/workbench_test.go`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/internal/server/lwc_shell_assets.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/server/lwc_shell.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcbrowser/bootstrap.go`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/shell/app.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/shell/router.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/shell/context-panel.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/shell/diagnostics.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/shell/glade-shell.css`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/esbuild.config.mjs`
- Test: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/test/lwc-shell-workbench.test.mjs`

**Steps:**

- [x] Add `WorkbenchModel` in `internal/lwcshell/workbench.go`:

```go
type WorkbenchModel struct {
    Title       string       `json:"title"`
    Mode        string       `json:"mode"`
    Apps        []ShellApp   `json:"apps"`
    Routes      []ShellRoute `json:"routes"`
    ActiveRoute string       `json:"activeRoute"`
    Active      ShellPage    `json:"active"`
    Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

type ShellApp struct {
    Name       string   `json:"name"`
    Label      string   `json:"label"`
    Mode       string   `json:"mode"`
    NavItems   []string `json:"navItems"`
    DefaultURL string   `json:"defaultUrl"`
}

type ShellRoute struct {
    Label       string            `json:"label"`
    URL         string            `json:"url"`
    Kind        RenderTargetKind  `json:"kind"`
    PageName    string            `json:"pageName,omitempty"`
    Component   string            `json:"component,omitempty"`
    ObjectName  string            `json:"objectApiName,omitempty"`
    RecordID    string            `json:"recordId,omitempty"`
    TabName     string            `json:"tabName,omitempty"`
    Diagnostics []Diagnostic      `json:"diagnostics,omitempty"`
    State       map[string]string `json:"state,omitempty"`
}
```

- [x] Add `BuildWorkbenchModel(project.Project, ShellPage, []string) WorkbenchModel`.
- [x] Refactor `renderLWCShellHTML` so HTML skeleton lives in `internal/server/lwc_shell_assets.go`.
- [x] Serve shell assets:

```text
/lightning/runtime/shell/app.js
/lightning/runtime/shell/router.js
/lightning/runtime/shell/context-panel.js
/lightning/runtime/shell/diagnostics.js
/lightning/runtime/shell/glade-shell.css
```

- [x] Add `/lwc` and `/lwc/preview` workbench roots. They must render the workbench with discovered routes and no mounted component selected.
- [x] Add a Salesforce-like shell frame:

```text
top global header
app launcher button
app name
tab bar
optional console side rail
main content region
right context panel
diagnostics drawer
```

- [x] Add tests proving `/lwc` renders route picker, `/lwc/preview/record/...` renders the same frame plus mounted components, and diagnostics appear in the context panel.

**Focused Verification:**

```bash
go test ./internal/lwcshell ./internal/server -run 'Workbench|LWCShell|ShellAssets' -count=1
npm --prefix lwcruntime test -- --test-name-pattern='lwc-shell-workbench'
```

**Done Gate:**

- The shell no longer depends on a long inline CSS string inside `internal/server/lwc_shell.go`.
- `/lwc` is a usable workbench entry point.
- Existing raw preview routes still work.

## Phase 2: Context Presets And Porcelain CLI

**Goal:** Let users choose a Salesforce-like context without hand-building URLs.

**Files:**

- Create: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcshell/context_preset.go`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcshell/context_preset_test.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/gladecli/dev_lwc_command.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/gladecli/dev_lwc_command_test.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/server/lwc_shell.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/testdata/local-tests/lwc-shell/glade.lwc.json`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/testdata/local-tests/lwc-shell/README.md`

**Context File Contract:**

Create project-root `glade.lwc.json`:

```json
{
  "defaultContext": "accountRecord",
  "contexts": {
    "accountRecord": {
      "target": "recordPage",
      "objectApiName": "Account",
      "recordId": "001000000000001AAA",
      "page": "Account_Record_Page",
      "app": "Sales",
      "tab": "Accounts",
      "formFactor": "Large",
      "state": {
        "c__mode": "demo"
      }
    },
    "salesDashboard": {
      "target": "appPage",
      "page": "Sales_Dashboard",
      "app": "Sales",
      "formFactor": "Large"
    },
    "home": {
      "target": "homePage",
      "page": "Custom_Home",
      "app": "Sales",
      "formFactor": "Large"
    },
    "tab": {
      "target": "tab",
      "tab": "Lwc_Probe",
      "app": "Sales",
      "formFactor": "Large"
    }
  }
}
```

**Steps:**

- [x] Add `LoadContextPresets(root string) (ContextPresetFile, error)`.
- [x] Search for `glade.lwc.json` at project root. Do not overload `glade.yml` in this phase.
- [x] Add stable diagnostics:

```text
GLADELWC020 context preset file invalid
GLADELWC021 context preset not found
GLADELWC022 context target unsupported
GLADELWC023 context record required
GLADELWC024 context page required
```

- [x] Add `ContextPreset.ToPageContext()` mapping:

```text
recordPage -> RenderTargetRecordPage
appPage -> RenderTargetAppPage
homePage -> RenderTargetHomePage
component -> RenderTargetComponent
tab -> RenderTargetTab
```

- [x] Add `glade dev lwc` flags:

```text
--open
--context <name>
--context-file <path>
--target component|record-page|app-page|home-page|tab
--component c:accountPanel
--object Account
--record 001000000000001AAA
--page Account_Record_Page
--tab Lwc_Probe
--app Sales
--form-factor Large|Medium|Small
--state key=value
```

- [x] Make `--context` choose the first browser path in startup output and ready file.
- [x] Make explicit flags override preset fields.
- [x] Make `--open` launch the browser with the selected URL through the existing platform opener pattern used elsewhere in Glade.
- [x] Extend the ready file:

```json
{
  "url": "http://127.0.0.1:8080",
  "selectedUrl": "http://127.0.0.1:8080/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page",
  "selectedContext": "accountRecord",
  "routes": []
}
```

- [x] Add `/lightning/local/context.json` that returns active context, PageReference, apps, routes, mounted components, diagnostics, and supported services.

**Focused Verification:**

```bash
go test ./internal/lwcshell ./internal/gladecli ./internal/server -run 'ContextPreset|DevLWC|LocalContext' -count=1
go run ./cmd/glade dev lwc --project testdata/local-tests/lwc-shell --context accountRecord --addr 127.0.0.1:0 --ready-file /tmp/glade-lwc-ready.json
jq '.selectedContext,.selectedUrl' /tmp/glade-lwc-ready.json
```

Expected:

```text
"accountRecord"
"http://127.0.0.1:<port>/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page"
```

**Done Gate:**

- Users can start the right shell context without composing URLs.
- The workbench UI lists contexts and routes.
- Agents can read `/lightning/local/context.json` for exact current state.

## Phase 3: Metadata Resolver Completion

**Goal:** Make placement and properties match Salesforce metadata contracts.

**Files:**

- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwc/meta.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwc/meta_test.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcshell/flexipage.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcshell/flexipage_test.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcshell/resolve.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcshell/resolve_test.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcshell/tab.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcshell/tab_test.go`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcshell/application.go`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcshell/application_test.go`

**Steps:**

- [x] Parse `targetConfigs`.
- [x] Parse `targetConfig targets`.
- [x] Parse property fields: `name`, `type`, `label`, `description`, `default`, `required`, `placeholder`, `min`, `max`, `datasource`, and `role`.
- [x] Parse supported objects.
- [x] Parse supported form factors.
- [x] Parse quick action `actionType`.
- [x] Parse URL-addressable target.
- [x] Parse FlexiPage API 48 legacy `componentInstances`.
- [x] Parse FlexiPage API 49+ `itemInstances`.
- [x] Parse `valueList` properties and preserve string lists.
- [x] Parse component visibility rules for simple field equality and boolean expressions.
- [x] Parse CustomApplication metadata enough for app label, nav items, default landing tab, and console flag.
- [x] Make resolver choose:

```text
direct component route from LWC target metadata
record page route from RecordPage FlexiPage
app page route from AppPage FlexiPage
home page route from HomePage FlexiPage
tab route from CustomTab
app navigation from CustomApplication
URL-addressable component route from lightning__UrlAddressable
quick action route from lightning__RecordAction
```

- [x] Add diagnostics:

```text
GLADELWC030 target config invalid
GLADELWC031 component target mismatch
GLADELWC032 form factor unsupported
GLADELWC033 property required
GLADELWC034 visibility rule approximated
GLADELWC035 application metadata unsupported
```

**Focused Verification:**

```bash
go test ./internal/lwc ./internal/lwcshell -run 'Meta|TargetConfig|FlexiPage|Application|Resolve|Tab|Visibility' -count=1
```

**Done Gate:**

- FlexiPage placement uses metadata instead of route guesses.
- Component properties use XML defaults and context overrides.
- App navigation is metadata-driven.

## Phase 4: Navigation, Router, And Shell Services

**Goal:** Make local navigation feel like small Salesforce navigation.

**Files:**

- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcbrowser/salesforce_modules.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcbrowser/salesforce_modules_test.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/server/lwc_shell.go`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/shell/navigation-service.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/shell/toast-service.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/shell/message-service.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/test/lwc-navigation-services.test.mjs`

**Steps:**

- [x] Replace direct `window.location.assign` in the navigation shim with a shell navigation service.
- [x] Keep `NavigationMixin.Navigate`.
- [x] Keep `NavigationMixin.GenerateUrl`.
- [x] Keep `CurrentPageReference` as a wire adapter.
- [x] Support PageReference types:

```text
standard__recordPage
standard__objectPage
standard__recordRelationshipPage
standard__navItemPage
standard__app
standard__namedPage
standard__component
standard__quickAction
standard__webPage
```

- [x] Generate local URLs under `/lwc/preview/*` where possible.
- [x] Redirect Visualforce tabs and Visualforce page references to `/apex/<Page>`.
- [x] Add unsupported diagnostics for Salesforce-only destinations:

```text
GLADELWC040 navigation target unsupported
GLADELWC041 quick action context missing
GLADELWC042 object page unsupported
```

- [x] Implement `lightning/platformShowToastEvent` with visible shell toasts and captured test events.
- [x] Implement `lightning/messageService` with local in-page channels.
- [x] Implement `lightning/platformResourceLoader` for local static resources and scripts.
- [x] Add browser tests for GenerateUrl, Navigate, CurrentPageReference re-emit, toast display, LMS publish/subscribe, and resource loading.

**Focused Verification:**

```bash
go test ./internal/lwcbrowser ./internal/server -run 'Navigation|PageReference|Toast|MessageService|ResourceLoader' -count=1
npm --prefix lwcruntime test -- --test-name-pattern='navigation|services'
```

**Done Gate:**

- Components can navigate between supported local shell targets.
- PageReference stays correct after navigation.
- Toasts, LMS, and resource loading work in shell browser tests.

## Phase 5: Apex Controller Bridge And Wire Fidelity

**Goal:** Make local LWC Apex calls behave like Salesforce enough for real development.

**Files:**

- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/server/lightning_wire.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/server/lightning_test.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcbrowser/wire.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcbrowser/salesforce_modules.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/shims/wire-adapter.mjs`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/test/wire.test.mjs`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/test/imperative-apex.test.mjs`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/testdata/local-tests/lwc-shell/force-app/main/default/classes/LwcProbeController.cls`

**Steps:**

- [x] Preserve existing Apex wire route.
- [x] Add imperative Apex call helper from `@salesforce/apex/Class.method`.
- [x] Enforce object-shaped Apex params.
- [x] Treat `undefined` wire params as suppressed invocation.
- [x] Treat `null` wire params as explicit invocation values.
- [x] Preserve array, object, string, number, boolean, date string, and null JSON shapes.
- [x] Run Apex through the local VM with current user and local org state.
- [x] Return Salesforce-shaped error bodies:

```json
{
  "body": {
    "message": "message",
    "exceptionType": "Type",
    "stackTrace": "..."
  },
  "status": 500
}
```

- [x] Add cacheable wire behavior with deterministic local cache keys.
- [x] Add `refreshApex` for Apex wires.
- [x] Add tests for success, thrown exception, validation error, missing class, missing method, wrong params, undefined suppression, and null invocation.
- [x] Add the same Apex wire and imperative calls inside Visualforce Lightning Out fixture pages.

**Focused Verification:**

```bash
go test ./internal/server ./internal/lwcbrowser -run 'Apex|Wire|Imperative|Lightning' -count=1
npm --prefix lwcruntime test -- --test-name-pattern='wire|imperative'
npm --prefix lwcruntime test -- --test-name-pattern='visualforce'
```

**Done Gate:**

- Apex wire and imperative calls work in the shell.
- The same calls work inside Visualforce Lightning Out.
- Error shape is stable and tested.

## Phase 6: LDS And UI API Services

**Goal:** Make record-backed LWCs work against local org state.

**Files:**

- Create: `/Users/matt/Dev/lwc-full-shell/glade/internal/server/lightning_lds.go`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/internal/server/lightning_lds_test.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/server/lightning_wire.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcbrowser/wire.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcbrowser/salesforce_modules.go`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/shims/lds-cache.mjs`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/test/record-wire.test.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/test/lds-services.test.mjs`

**Steps:**

- [x] Keep existing `getRecord`.
- [x] Keep existing `getObjectInfo`.
- [x] Add `getFieldValue`.
- [x] Add `getFieldDisplayValue`.
- [x] Add `getRecordNotifyChange` compatibility if still used by older LWC code.
- [x] Add `notifyRecordUpdateAvailable`.
- [x] Add `refreshApex` invalidation for LDS-backed wires.
- [x] Add `createRecord`.
- [x] Add `updateRecord`.
- [x] Add `deleteRecord`.
- [x] Add `getPicklistValues`.
- [x] Add `getPicklistValuesByRecordType`.
- [x] Add `getRelatedListRecords` for deterministic child relationships in local schema.
- [x] Add `getListUi` diagnostic:

```text
GLADELWC050 getListUi unsupported locally; use getRelatedListRecords or local SOQL-backed Apex
```

- [x] Add local LDS cache keyed by record ID, fields, optional fields, layout types, modes, object info, and related list params.
- [x] Re-emit wire values after create/update/delete/notify when cache keys overlap.
- [x] Use local DML/storage paths for mutations.
- [x] Add tests for field helper functions, mutation invalidation, picklists, related lists, missing record, missing field, readonly field, required field, and deleted record.

**Focused Verification:**

```bash
go test ./internal/server ./internal/lwcbrowser -run 'LDS|UIAPI|Record|Picklist|RelatedList' -count=1
npm --prefix lwcruntime test -- --test-name-pattern='lds|record'
```

**Done Gate:**

- Record-page LWCs can read, create, update, delete, and refresh local records.
- LDS cache behavior is deterministic.
- Unsupported UI API calls fail with named diagnostics.

## Phase 7: SLDS, Icons, And Base Components

**Goal:** Make common `lightning-*` components render and behave well in the shell and Visualforce Lightning Out.

**Files:**

- Modify: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/package.json`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/package-lock.json`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/esbuild.config.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/slds/slds-loader.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/slds/glade-slds.css`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/lightning/button.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/lightning/buttonIcon.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/lightning/card.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/lightning/input.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/lightning/combobox.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/lightning/layout.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/lightning/tabset.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/lightning/datatable.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/lightning/recordForm.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/lightning/modal.mjs`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcbrowser/salesforce_modules.go`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcbrowser/base_components.go`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcbrowser/base_components_test.go`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/test/base-components.test.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/test/visualforce-base-components.test.mjs`

**Dependency Policy:**

- Add `@salesforce-ux/design-system` for SLDS 1 CSS assets.
- Add `@salesforce-ux/design-system-2` for SLDS 2 CSS assets.
- Add `lightning-base-components` only if it can be bundled into Glade runtime without requiring Vite-specific project structure.
- If `lightning-base-components` cannot be bundled cleanly, implement Glade-owned base component modules in `lwcruntime/src/lightning` and keep the package out of runtime dependencies.

**Steps:**

- [x] Add SLDS 2 as the default shell theme.
- [x] Add SLDS 1 fallback switch.
- [x] Serve CSS from local assets. Do not fetch CDN assets.
- [x] Generate icon assets or serve stable local icon placeholders for utility, action, standard, and custom icons.
- [x] Support Tier 1 base components:

```text
lightning-button
lightning-button-icon
lightning-card
lightning-input
lightning-textarea
lightning-combobox
lightning-layout
lightning-layout-item
lightning-tabset
lightning-tab
lightning-spinner
lightning-icon
```

- [x] Support Tier 2 data components:

```text
lightning-datatable
lightning-record-form
lightning-record-view-form
lightning-record-edit-form
lightning-output-field
lightning-input-field
lightning-messages
```

- [x] Dispatch common events:

```text
click
change
submit
success
error
cancel
rowaction
active
```

- [x] Add diagnostics:

```text
GLADELWC060 base component unsupported
GLADELWC061 base component attribute unsupported
GLADELWC062 SLDS asset missing
```

- [x] Add screenshot or DOM shape tests for card, inputs, datatable, record form, tabs, modal, and toast.
- [x] Run the same base component probes inside Visualforce Lightning Out.

**Focused Verification:**

```bash
go test ./internal/lwcbrowser ./internal/server -run 'Base|Lightning|SLDS|Icon' -count=1
npm --prefix lwcruntime test -- --test-name-pattern='base|visualforce-base'
```

**Done Gate:**

- Common `lightning-*` components render in the shell.
- Data components use local LDS.
- Visualforce Lightning Out gets the same base component support unless a support row says otherwise.

## Phase 8: Quick Actions, URL-Addressable Components, And App Modes

**Goal:** Cover the shell contexts users hit in real Salesforce navigation.

**Files:**

- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcshell/model.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcshell/resolve.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/server/lwc_shell.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/shell/app.mjs`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/shell/router.mjs`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/test/lwc-actions-url-addressable.test.mjs`
- Add fixtures under: `/Users/matt/Dev/lwc-full-shell/glade/testdata/local-tests/lwc-shell/force-app/main/default/quickActions`

**Steps:**

- [ ] Add URL-addressable component route:

```text
/lwc/preview/cmp/<namespace>/<component>?c__name=value
```

- [ ] Add record action route:

```text
/lwc/preview/action/<Object>/<recordId>/<ActionName>
```

- [ ] Add global action route:

```text
/lwc/preview/action/global/<ActionName>
```

- [ ] Parse quick action metadata enough to identify action name, object, component, and action type.
- [ ] Inject action context:

```json
{
  "recordId": "001000000000001AAA",
  "objectApiName": "Account",
  "actionName": "Account.Update_Status",
  "actionType": "ScreenAction"
}
```

- [ ] Add standard app mode and console mode:

```text
standard app: top tabs
console app: side rail plus workspace tabs approximation
```

- [ ] Add diagnostics:

```text
GLADELWC070 quick action unsupported
GLADELWC071 URL-addressable state invalid
GLADELWC072 console API approximated
```

- [ ] Keep full workspace API unsupported, but provide stable approximations for active tab label and current route.

**Focused Verification:**

```bash
go test ./internal/lwcshell ./internal/server -run 'QuickAction|UrlAddressable|Console|Application' -count=1
npm --prefix lwcruntime test -- --test-name-pattern='actions|url-addressable'
```

**Done Gate:**

- URL-addressable LWCs receive state.
- Quick action LWCs receive record/action context.
- Standard and console shell modes are visible and documented.

## Phase 9: Visualforce Lightning Out Shared Runtime Parity

**Goal:** Keep Visualforce-hosted LWCs equal to shell-hosted LWCs for shared services.

**Files:**

- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/server/visualforce.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/server/lightning.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/server/lightning_shims.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/internal/lwcbrowser/bootstrap.go`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/src/glade.out.mjs`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/test/visualforce-dev-server.test.mjs`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/test/visualforce-lightningout.test.mjs`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/lwcruntime/test/visualforce-base-components.test.mjs`
- Modify fixtures under: `/Users/matt/Dev/lwc-full-shell/glade/testdata/local-tests/lightning-out-vf`

**Steps:**

- [ ] Ensure Visualforce bootstrap uses the same import map as the shell.
- [ ] Ensure Visualforce bootstrap gets a host-specific PageReference:

```json
{
  "type": "standard__webPage",
  "attributes": {
    "url": "/apex/WidgetHost"
  },
  "state": {}
}
```

- [ ] If the Visualforce page has standard controller record context, pass record-like attributes only through explicit `$Lightning.createComponent` attrs or documented Visualforce bridge context.
- [ ] Do not leak record-page route context into `/apex`.
- [ ] Run Apex wire, imperative Apex, LDS, labels, resources, navigation diagnostics, toast, LMS, and base component probes inside Visualforce Lightning Out.
- [ ] Add diagnostics:

```text
GLADELWC080 Lightning Out app missing
GLADELWC081 Lightning Out dependency missing
GLADELWC082 Lightning Out service unsupported in Visualforce host
```

**Focused Verification:**

```bash
go test ./internal/server -run 'Visualforce.*Lightning|LightningOut|LWC' -count=1
npm --prefix lwcruntime test -- --test-name-pattern='visualforce'
```

**Done Gate:**

- Shell and Visualforce hosts share Apex, LDS, labels, resources, base components, and service shims.
- Host-specific context is correct.
- Missing dependencies fail with named diagnostics.

## Phase 10: Scratch-Org Oracle And Support Ledger

**Goal:** Prove the shell against Salesforce where public behavior can be observed.

**Files:**

- Modify or create in `/Users/matt/Dev/lwc-full-shell/glade-tools/internal/compat/lwc_capture.go`
- Modify or create in `/Users/matt/Dev/lwc-full-shell/glade-tools/internal/compat/lwc_capture_test.go`
- Modify or create in `/Users/matt/Dev/lwc-full-shell/glade-tools/internal/compat/lwc_diff.go`
- Modify or create in `/Users/matt/Dev/lwc-full-shell/glade-tools/internal/toolcli/compat_lwc_command.go`
- Modify fixtures under `/Users/matt/Dev/lwc-full-shell/glade/testdata/local-tests/lwc-shell`
- Create: `/Users/matt/Dev/lwc-full-shell/glade/docs/generated/LWC_SHELL_SUPPORT.md`

**Steps:**

- [ ] Extend LWC capture to deploy or validate the fixture project against `oaer-probe-max`.
- [ ] Capture Salesforce browser DOM for:

```text
direct component
record page
app page
home page
custom tab
URL-addressable component
record quick action
Visualforce Lightning Out
Apex wire
imperative Apex
LDS read
LDS mutation
navigation
toast
LMS
base components
```

- [ ] Capture local browser DOM for the same targets.
- [ ] Capture console errors and page errors.
- [ ] Normalize volatile IDs, private Salesforce script URLs, session-specific values, timestamps, and generated DOM noise.
- [ ] Diff visible text, public attributes, PageReference JSON, wire payload shape, and mounted component count.
- [ ] Generate support rows with fields:

```json
{
  "feature": "lwc.service.navigation",
  "host": "lightning-shell",
  "status": "supported",
  "evidence": "/tmp/glade-lwc-full-shell-capture.json",
  "notes": "PageReference and GenerateUrl match local contract"
}
```

- [ ] Keep public docs sourced from support facts.

**Focused Verification:**

```bash
cd /Users/matt/Dev/lwc-full-shell/glade-tools
go test ./internal/compat ./internal/toolcli -run 'LWC|lwc' -count=1
go run ./cmd/glade-plugin-compat lwc capture \
  --target-org oaer-probe-max \
  --project /Users/matt/Dev/lwc-full-shell/glade/testdata/local-tests/lwc-shell \
  --include-hosts lightning-shell,visualforce-lightning-out \
  --out /tmp/glade-lwc-full-shell-capture.json
jq '.ok,.hosts,.counts' /tmp/glade-lwc-full-shell-capture.json
```

**Done Gate:**

- Every supported shell feature has local and Salesforce evidence or a named local-only reason.
- No support row remains unknown.
- Salesforce-only/private behavior is marked unsupported or partial with exact wording.

## Phase 11: Docs, Site, And User Workflow

**Goal:** Make the feature easy to use without knowing the raw routes.

**Files:**

- Modify: `/Users/matt/Dev/lwc-full-shell/glade/docs/LWC_LOCAL_SHELL.md`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/docs/LWC_SUPPORT.md`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/docs/LOCAL_TESTING.md`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/docs/COMPATIBILITY.md`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/site/docs-src/guide/lwc-local-shell.md`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/site/docs-src/guide/support-map.md`
- Modify: `/Users/matt/Dev/lwc-full-shell/glade/site/.vitepress/config.ts` when the LWC shell page needs a new sidebar or nav entry

**Steps:**

- [ ] Document the simple start:

```bash
glade dev lwc --project . --open
```

- [ ] Document context presets:

```bash
glade dev lwc --project . --context accountRecord --open
```

- [ ] Document direct flags:

```bash
glade dev lwc --project . --target record-page --object Account --record 001000000000001AAA --page Account_Record_Page --open
```

- [ ] Document `glade.lwc.json`.
- [ ] Document `/lightning/local/context.json`.
- [ ] Document supported shell hosts:

```text
lwc.host.lightning-shell
lwc.host.visualforce-lightning-out
```

- [ ] Document Apex, LDS, navigation, services, base components, and SLDS support.
- [ ] Keep preview wording until Phase 12 final gate passes.
- [ ] Add screenshots or short route examples only from fixtures.
- [ ] Verify rendered site routes.

**Focused Verification:**

```bash
npm --prefix site test
npm --prefix site run build
```

**Done Gate:**

- Docs explain the porcelain first and raw routes second.
- Docs name supported, partial, and unsupported behavior.
- Site builds.

## Phase 12: Final Review, Performance, And Claim Gate

**Goal:** Finish the branch and decide whether public wording can move beyond preview.

**Files:**

- Modify only from findings: product, runtime, docs, and test files touched by earlier phases.

**Steps:**

- [ ] Run focused Go shell tests:

```bash
go test ./internal/lwcshell ./internal/lwcbrowser ./internal/server ./internal/gladecli -count=1
```

- [ ] Run all runtime browser tests:

```bash
npm --prefix lwcruntime test
```

- [ ] Run full Go tests:

```bash
go test ./...
```

- [ ] Run smoke:

```bash
scripts/smoke.sh
```

- [ ] Run docs/site:

```bash
npm --prefix site test
npm --prefix site run build
```

- [ ] Run oracle:

```bash
cd /Users/matt/Dev/lwc-full-shell/glade-tools
go test ./...
go run ./cmd/glade-plugin-compat lwc capture \
  --target-org oaer-probe-max \
  --project /Users/matt/Dev/lwc-full-shell/glade/testdata/local-tests/lwc-shell \
  --include-hosts lightning-shell,visualforce-lightning-out \
  --out /tmp/glade-lwc-full-shell-capture.json
```

- [ ] Profile startup and first render:

```bash
time go run ./cmd/glade dev lwc --project testdata/local-tests/lwc-shell --addr 127.0.0.1:0 --ready-file /tmp/glade-lwc-ready.json
```

Record:

```text
server startup time
first shell route time
first component mount time
wire response time
browser console error count
```

- [ ] Dispatch Review squad for read-only review. Findings first. No code edits.
- [ ] Fix findings.
- [ ] Re-run affected gates.
- [ ] Decide claim wording:

```text
If all final gates pass:
  "LWC local shell support" may replace "preview" only for supported rows.
If any feature lacks oracle or broad tests:
  keep "preview feature" wording and list exact remaining rows.
```

**Done Gate:**

- The shell is usable from `glade dev lwc --open`.
- Context presets work.
- LWCs call local Apex.
- LWCs use LDS/UI API services.
- Navigation, toasts, LMS, resources, labels, schema, user, i18n, and base components have tested support.
- Visualforce Lightning Out shares the same runtime services.
- Docs and site match shipped behavior.
- Support ledger has no unknown rows.

---

## Execution Handoff Prompt

Use this prompt when starting implementation:

```text
Implement /Users/matt/Dev/glade/docs/superpowers/plans/2026-06-17-lwc-full-local-shell.md in full.

You are GPT-5.5 High.
Use superpowers:subagent-driven-development.
Use parallel subagents by squad where file ownership is disjoint.
Treat codex/lwc-vf-preview-feature as merged baseline. Do not cherry-pick it.
Build the full Glade-owned LWC shell and porcelain.
Use salesforce-ux/design-system-2-starter-kit as a reference, not as a direct fork.
Keep product code in /Users/matt/Dev/lwc-full-shell/glade.
Keep oracle work in /Users/matt/Dev/lwc-full-shell/glade-tools.
Use oaer-probe-max for Salesforce evidence.
Do not remove preview wording until the final claim gate says it is justified.

After each phase, return:
- squads dispatched
- changed files
- commands run
- browser routes checked
- scratch-org artifacts when relevant
- support rows changed
- remaining explicit gaps
```

## Final Claim

The branch is done when a user can run:

```bash
glade dev lwc --project . --context accountRecord --open
```

and get a Salesforce-like local shell where:

- the chosen context is visible,
- metadata-driven routes are selectable,
- LWCs mount in the right page, app, tab, action, or Visualforce host,
- Apex calls execute through Glade,
- LDS/UI API calls use local org data,
- navigation and services work locally,
- common base components render with SLDS styling,
- diagnostics are visible and machine-readable,
- support docs match the tested contract.
