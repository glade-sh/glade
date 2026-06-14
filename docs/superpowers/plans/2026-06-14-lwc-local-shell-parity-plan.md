# LWC Local Shell And Visualforce Lightning Out Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan with parallel subagent squads wherever ownership is disjoint. Use superpowers:executing-plans only for serial integration checkpoints. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build first-class local Lightning Web Component rendering in Glade across both supported hosts: the Salesforce-like local Lightning shell and Visualforce-hosted Lightning Out pages. Direct components, record pages, app pages, home pages, custom tabs, URL-addressable components, Visualforce `$Lightning.use()` / `$Lightning.createComponent()` mounts, real local Apex controller calls, LDS-like data, and shared browser runtime services all need checked scratch-org parity.

**Architecture:** Add an `internal/lwcshell` product package that turns Salesforce project metadata into a shell page model, and make it share the same `internal/lwcbrowser`, `lwcruntime`, wire, Apex, LDS, base component, and service shims used by Visualforce Lightning Out. Keep `internal/server` as the HTTP bridge to local org state, Visualforce `/apex` pages, Lightning shell routes, and the Apex VM. Use `glade-tools` only for scratch-org probes, generated support ledgers, and corpus dashboards.

**Tech Stack:** Go server and metadata parser, existing Glade project loader, existing LWC compiler/toolchain, browser ESM import maps, Visualforce renderer, Lightning Out parser, `lwcruntime` JavaScript shims, local `storage.OrgState`, Glade Apex VM, Playwright via `lwcruntime`, and the `oaer-probe-max` scratch org as the Salesforce oracle.

---

## Research Findings

The official Salesforce Live Preview feature now covers single-component, Lightning app, and Experience LWR site preview with browser or VS Code hot reload. That proves the desired local loop is aligned with Salesforce's own direction, but it does not give Glade local org state, local Apex VM execution, checked support reporting, or a page shell controlled by project metadata. Source: [Run a Live Component Preview](https://developer.salesforce.com/docs/platform/lwc/guide/get-started-test-components.html).

LWC exposure comes from `componentName.js-meta.xml`. It defines `isExposed`, `targets`, `targetConfigs`, builder properties, objects, and form factors. Sources: [XML Configuration File Elements](https://developer.salesforce.com/docs/platform/lwc/guide/reference-configuration-tags.html), [`lightning__RecordPage`](https://developer.salesforce.com/docs/platform/lwc/guide/targets-lightning-record-page.html), [`lightning__AppPage`](https://developer.salesforce.com/docs/platform/lwc/guide/targets-lightning-app-page.html), [`lightning__HomePage`](https://developer.salesforce.com/docs/platform/lwc/guide/targets-lightning-home-page.html), and [Configure Components for Custom Tabs](https://developer.salesforce.com/docs/platform/lwc/guide/use-config-custom-tab.html).

Lightning pages are FlexiPages in Metadata API. A FlexiPage has a page type, template, regions, component instances, component properties, visibility rules, and dynamic form field instances. Local docs confirm the same contract at `/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run/metadata-api/meta_flexipage.md`.

Lightning Data Service is a shared record and metadata cache for base record components plus `lightning/ui*Api` wire adapters and functions. It bulkifies, dedupes, invalidates, and re-emits values. Sources: [Lightning Data Service](https://developer.salesforce.com/docs/platform/lwc/guide/data-ui-api.html), [`lightning/ui*Api`](https://developer.salesforce.com/docs/platform/lwc/guide/reference-ui-api.html), [`getRecord`](https://developer.salesforce.com/docs/platform/lwc/guide/reference-wire-adapters-record.html), and [`getRelatedListRecords`](https://developer.salesforce.com/docs/platform/lwc/guide/reference-wire-adapters-get-related-list-records.html).

Runtime modules split into org-scoped `@salesforce/*` modules and universal `lightning/*` modules. Apex imports must support wire and imperative calls. Apex params are object-shaped. `null` invokes and `undefined` suppresses an Apex call. Source: [`@salesforce` Modules](https://developer.salesforce.com/docs/platform/lwc/guide/reference-salesforce-modules.html).

Navigation is PageReference-based. The shell must support app, nav item, object page, record page, record relationship page, quick action, URL-addressable component, standard named page, and web page references enough for local development. Sources: [Navigation](https://developer.salesforce.com/docs/platform/lightning-component-reference/guide/lightning-navigation.html) and [PageReference Types](https://developer.salesforce.com/docs/platform/lwc/guide/reference-page-reference-type.html).

Record context is automatic only in explicit record containers. The local shell must set `recordId` and `objectApiName` for record pages and actions, and it must leave them absent in shells where Salesforce leaves them absent. Source: [Make a Component Aware of Its Record Context](https://developer.salesforce.com/docs/platform/lwc/guide/use-record-context.html).

Lightning Out through Visualforce is a first-class LWC host in this roadmap. Visualforce pages that call `$Lightning.use()` and `$Lightning.createComponent()` must mount LWCs through the same compiled modules, import maps, Apex wire routes, LDS/UI API shims, base components, and service modules as the local Lightning shell. Host-specific context must stay distinct: Lightning record pages inject record context from FlexiPage and route state, while Visualforce pages inject Visualforce page params, standard controller context when present, and explicit `$Lightning.createComponent()` attributes.

## Parity Boundary

Full useful parity is practical. Literal internal Salesforce parity is not.

Build:
- Local shell parity for public LWC contracts, public metadata, public PageReference types, public LDS and UI API shapes, public base component contracts, and real local Apex controllers.
- Visualforce Lightning Out parity for public `$Lightning.use()` / `$Lightning.createComponent()` contracts, including LWC rendering inside `/apex/<PageName>` routes.
- Cross-host parity for shared runtime capabilities. If Apex wire, imperative Apex, LDS/UI API, base components, navigation, toasts, message service, or modals are marked supported, they must pass in both `/lwc/preview/*` and Visualforce Lightning Out hosts unless the support ledger names a host-specific exception.
- Checked differences against `oaer-probe-max`, recorded as support data, screenshots, DOM snapshots, wire payloads, and console errors.
- Fast local development that starts from project metadata and local data, not from hand-written harness HTML.

Do not promise:
- Salesforce's proprietary Lightning Experience implementation details.
- Pixel-exact internal chrome, animation timing, or private CSS.
- Exact Lightning Web Security or Locker internals.
- Full mobile offline runtime.
- Lightning App Builder authoring UI.
- Experience Cloud LWR site runtime in this LWC-only plan.

The product claim after this roadmap should be:

```text
Glade renders and tests Lightning Web Components locally in Salesforce-like record, app, home, tab, action, and URL-addressable shells. It runs project Apex controllers in the local VM, serves LDS-style data from local org state, and reports checked differences from Salesforce scratch-org captures.
Glade also renders LWCs embedded in Visualforce pages through Lightning Out, using the same local runtime services and checked support ledger.
```

## Current State From The Patch

Useful work already exists:
- `internal/lwc/meta.go` parses `isExposed` and top-level `targets`.
- `internal/lwcbrowser/project.go` compiles LWC bundles and applies Aura passthrough aliases.
- `internal/lwcbrowser/salesforce_modules.go` maps several `@salesforce/*` modules and `lightning/uiRecordApi`.
- `internal/server/lightning_wire.go` serves Apex wire, `getRecord`, and `getObjectInfo`.
- `lwcruntime/src/shims/wire-adapter.mjs` has fetch-backed wire adapters.
- `lwcruntime/test/wire.test.mjs` and `record-wire.test.mjs` prove browser wire flow.
- `internal/lightningout/parse.go`, `internal/server/visualforce.go`, `internal/server/lightning*.go`, and `testdata/local-tests/lightning-out-vf/` already prove a Visualforce Lightning Out host.
- `lwcruntime/test/visualforce-dev-server.test.mjs` proves a Visualforce page can boot Lightning Out components through the local dev server.

Gaps:
- No first-class `glade dev lwc` command.
- No local shell route model for direct component, record page, app page, home page, custom tab, or URL-addressable component.
- No explicit requirement that every shared LWC runtime feature pass in both the Lightning shell and Visualforce Lightning Out host.
- No FlexiPage parser or CustomTab resolver.
- No full `targetConfig` parser for properties, objects, form factors, quick action types, or dynamic interaction events.
- `lightning/navigation` currently throws.
- UI API coverage is limited to `getRecord` and `getObjectInfo`.
- No LDS cache, refresh, mutation, related lists, list views, layout, picklists, GraphQL, or object-info batching.
- No base component library beyond existing static rendering helpers.
- No scratch-org oracle for LWC shell behavior.

What about that: the pieces are good, but the bench is still missing a vise.

## Current Developer Experience

Commands supported by the merged product surface:

```bash
glade dev lwc --project .
glade dev lwc --project . --port 8080
glade dev lwc --project . --addr 127.0.0.1:0 --ready-file /tmp/glade-lwc-ready.json
glade dev vf --project .
glade dev vf --project . --port 8080
glade dev vf --project . --addr 127.0.0.1:0 --ready-file /tmp/glade-vf-ready.json
cd ../glade-tools
go run ./cmd/glade-plugin-compat lwc capture --target-org oaer-probe-max --project ../glade/testdata/local-tests/lwc-shell --targets record-page --out /tmp/glade-lwc-shell-fixture-manifest.json
```

Routes served by the local dev server:

```text
/lwc/preview/component/c/accountPanel?recordId=001000000000001AAA
/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page
/lwc/preview/app/Sales_Dashboard
/lwc/preview/home/Custom_Home
/lwc/preview/tab/My_Custom_Tab
/apex/WidgetHost
```

Selector flags, browser-test CLI commands, and scratch-org DOM comparison are
later phases in this plan. Today, the LWC and Visualforce dev commands start
servers and developers open the route that matches the target.

Future phases add a debug JSON endpoint:

```text
/lightning/local/context.json?url=/lightning/r/Account/001000000000001AAA/view
```

The JSON endpoint lets browser tests, agents, and users inspect the resolved target, page reference, record context, app context, component instances, wire support, and unsupported surfaces.

## File Map

Create:
- `internal/lwcshell/types.go` - shared enums and value objects for targets, form factors, page references, shell pages, shell components, and support diagnostics.
- `internal/lwcshell/meta.go` - full LWC `*.js-meta.xml` parser.
- `internal/lwcshell/flexipage.go` - FlexiPage metadata parser for API 48 and API 49+ shapes.
- `internal/lwcshell/tabs.go` - CustomTab and CustomApplication navigation resolver.
- `internal/lwcshell/context.go` - shell context builder for record, app, user, locale, theme, form factor, route, and state.
- `internal/lwcshell/resolver.go` - direct component, record page, app page, home page, tab, URL-addressable, and PageReference resolver.
- `internal/lwcshell/render.go` - shell HTML renderer and JSON context serializer.
- `internal/lwcshell/support.go` - support codes and explicit diagnostics.
- `internal/lwcshell/*_test.go` - unit tests for each parser and resolver.
- `internal/gladecli/dev_lwc_command.go` and `internal/gladecli/dev_lwc_command_test.go` - product CLI.
- `internal/gladecli/lwc_test_command.go` and `internal/gladecli/lwc_probe_command.go` - local browser test and scratch probe entrypoints.
- `internal/server/lwc_shell.go` and `internal/server/lwc_shell_test.go` - HTTP routes and context endpoint.
- `internal/server/lightning_apex.go` - imperative Apex endpoint.
- `internal/server/lightning_lds.go` - LDS/UI API route expansion.
- `lwcruntime/src/shell/context.mjs` - client-side shell context provider.
- `lwcruntime/src/shell/hot-reload.mjs` - browser reload channel.
- `lwcruntime/src/shims/navigation.mjs` - PageReference and NavigationMixin shim.
- `lwcruntime/src/shims/lds-cache.mjs` - client LDS cache and invalidation helper.
- `lwcruntime/src/shims/message-service.mjs` - Lightning Message Service shim.
- `lwcruntime/src/base-components/*.mjs` - base Lightning component modules.
- `lwcruntime/test/lwc-shell.test.mjs` - Playwright shell tests.
- `lwcruntime/test/visualforce-lightning-out.test.mjs` - Playwright tests for LWC runtime behavior inside Visualforce Lightning Out.
- `testdata/local-tests/lwc-shell/...` - full fixture project.
- `testdata/local-tests/lightning-out-vf/...` - Visualforce Lightning Out fixture project.
- `docs/LWC_LOCAL_SHELL.md` - user docs.
- `docs/LWC_SUPPORT.md` - checked support table.

Modify:
- `internal/lwcbrowser/salesforce_modules.go` - import map and module resolution.
- `internal/lwcbrowser/wire.go` - request and response contracts.
- `internal/lwcbrowser/manifest.go` - base component and context module entries.
- `internal/server/lightning.go` - route dispatch.
- `internal/server/lightning_wire.go` - UI API wire expansion.
- `internal/server/visualforce.go` - keep `/apex/<PageName>` Lightning Out host on the same runtime services as the LWC shell.
- `internal/lightningout/parse.go` - preserve and extend `$Lightning.use()` / `$Lightning.createComponent()` discovery only where Visualforce fixture evidence requires it.
- `internal/gladecli/dev_command.go` or equivalent dev command registry - add `lwc`.
- `internal/cliui/help.go` - discoverable but compact LWC command descriptions.
- `internal/gladehome/root.go` and `internal/gladehome/install.go` - install shell and shim assets.
- `scripts/release-build.sh` - include new `lwcruntime/src/shell` and base components.
- `docs/LOCAL_TESTING.md` and `site/docs-src/guide/local-testing.md` - link the new LWC page without turning public help into a support matrix.
- Sibling `../glade-tools` - add scratch capture and support-ledger generators only.

## Parallel Subagent Squad Model

Run this with parallel subagent squads. Use `superpowers:subagent-driven-development`; dispatch one fresh subagent per disjoint ownership boundary, then integrate serially. Each squad gets one package boundary, one host boundary when possible, and one evidence command. Do not run multiple broad gates at the same time; cap heavy verification to one wide Go or browser suite at a time.

- Metadata squad: `internal/lwcshell/meta.go`, `flexipage.go`, `tabs.go`.
- Shell squad: `internal/lwcshell/context.go`, `resolver.go`, `render.go`, `internal/server/lwc_shell.go`.
- Visualforce host squad: `internal/server/visualforce.go`, `internal/lightningout/parse.go`, and `testdata/local-tests/lightning-out-vf/`, proving LWC runtime behavior inside `/apex/<PageName>`.
- Runtime squad: `internal/lwcbrowser`, `lwcruntime/src/shell`, Visualforce bootstrap reuse, import maps, hot reload.
- LDS squad: `internal/server/lightning_lds.go`, `lwcruntime/src/shims/lds-cache.mjs`, UI API modules, with Lightning shell and Visualforce Lightning Out tests.
- Apex squad: `internal/server/lightning_apex.go`, VM invocation, JSON shaping, error shaping, with wire and imperative coverage in both hosts.
- Base component squad: `lwcruntime/src/base-components`, SLDS-like contracts, form components, with both-host browser tests.
- CLI squad: `internal/gladecli/dev_lwc_command.go`, `lwc_test_command.go`, help and docs hooks.
- Oracle squad: sibling `../glade-tools` scratch-org capture against `oaer-probe-max`.
- Review squad: tests, screenshots, support map, docs, release packaging.

Every phase review must answer two host questions before support is claimed:
- Does this feature pass in the Lightning shell host?
- Does this feature pass in the Visualforce Lightning Out host, or does the support ledger name the host-specific gap?

## Phase 0: Baseline And Fixture

**Goal:** Lock the target behavior before building. Use official docs, local docs, the patch, and a scratch org as inputs.

**Files:**
- Create: `testdata/local-tests/lwc-shell/`
- Create: `testdata/local-tests/lwc-shell/force-app/main/default/classes/AccountPanelController.cls`
- Create: `testdata/local-tests/lwc-shell/force-app/main/default/lwc/accountPanel/accountPanel.js`
- Create: `testdata/local-tests/lwc-shell/force-app/main/default/lwc/accountPanel/accountPanel.html`
- Create: `testdata/local-tests/lwc-shell/force-app/main/default/lwc/accountPanel/accountPanel.js-meta.xml`
- Create: `testdata/local-tests/lwc-shell/force-app/main/default/flexipages/Account_Record_Page.flexipage-meta.xml`
- Create: `testdata/local-tests/lwc-shell/force-app/main/default/flexipages/Sales_Dashboard.flexipage-meta.xml`
- Create: `testdata/local-tests/lwc-shell/force-app/main/default/flexipages/Custom_Home.flexipage-meta.xml`
- Create: `testdata/local-tests/lwc-shell/force-app/main/default/tabs/My_Custom_Tab.tab-meta.xml`
- Create: `testdata/local-tests/lwc-shell/force-app/main/default/applications/Sales.app-meta.xml`
- Create: `testdata/local-tests/lwc-shell/seed.json`
- Create: `testdata/local-tests/lwc-shell/README.md`

- [ ] **Step 0.1: Build the fixture project**

The fixture must contain one LWC that uses all first-mile contracts:

```javascript
import { LightningElement, api, wire } from 'lwc';
import getSummary from '@salesforce/apex/AccountPanelController.getSummary';
import updateName from '@salesforce/apex/AccountPanelController.updateName';
import ACCOUNT_NAME from '@salesforce/schema/Account.Name';
import { getRecord, getObjectInfo, updateRecord, notifyRecordUpdateAvailable } from 'lightning/uiRecordApi';
import { CurrentPageReference, NavigationMixin } from 'lightning/navigation';

export default class AccountPanel extends NavigationMixin(LightningElement) {
  @api recordId;
  @api objectApiName;
  @api title = 'Account Panel';

  fields = [ACCOUNT_NAME];

  @wire(CurrentPageReference) pageRef;
  @wire(getRecord, { recordId: '$recordId', fields: '$fields' }) record;
  @wire(getObjectInfo, { objectApiName: '$objectApiName' }) objectInfo;
  @wire(getSummary, { accountId: '$recordId' }) summary;

  async rename() {
    await updateName({ accountId: this.recordId, name: 'Local Rename' });
    await updateRecord({ fields: { Id: this.recordId, Name: 'Local Rename' } });
    await notifyRecordUpdateAvailable([{ recordId: this.recordId }]);
  }
}
```

The fixture must prove:
- Automatic `recordId` and `objectApiName` on record pages.
- Target property defaults from `targetConfigs`.
- App page and home page render without record context.
- Custom tab navigation uses `standard__navItemPage`.
- Apex wire and imperative Apex invoke the real local controller.
- `getRecord`, `getObjectInfo`, `updateRecord`, and `notifyRecordUpdateAvailable` share data.

- [ ] **Step 0.2: Prepare scratch fixture manifest**

Use `../glade-tools` and `oaer-probe-max` to deploy the fixture and write the
stable manifest that the later browser oracle consumes:

```bash
cd ../glade-tools
go run ./cmd/glade-plugin-compat lwc capture \
  --project ../glade/testdata/local-tests/lwc-shell \
  --target-org oaer-probe-max \
  --targets record-page,app-page,home-page,custom-tab \
  --include-hosts lightning-shell,visualforce-lightning-out \
  --out /tmp/glade-lwc-shell-fixture-manifest.json
```

Expected:
- `/tmp/glade-lwc-shell-fixture-manifest.json` has `mode:"fixture-manifest"`.
- Each target row has `status:"prepared"` and a `fixture://lwc/...` URL.
- Browser DOM, screenshot, wire payload, console, and context capture belongs to Phase 12.

- [ ] **Step 0.3: Run current baseline tests**

```bash
go test ./internal/lwc ./internal/lwcbrowser ./internal/server ./internal/gladecli -count=1
go test ./internal/lwcruntime -count=1
```

Expected:
- Existing LWC and runtime tests pass before new work starts.
- Any failure is copied into the phase notes with the exact package and test name.

- [ ] **Step 0.4: Commit**

```bash
git add testdata/local-tests/lwc-shell
git commit -m "test: add LWC shell parity fixture"
```

## Phase 1: Metadata Model

**Goal:** Parse the public metadata that decides where an LWC can run and how it is configured.

**Files:**
- Create: `internal/lwcshell/types.go`
- Create: `internal/lwcshell/meta.go`
- Create: `internal/lwcshell/meta_test.go`
- Modify: `internal/lwc/meta.go`

- [ ] **Step 1.1: Define shell metadata types**

Add these exported types in `internal/lwcshell/types.go`:

```go
package lwcshell

type TargetKind string

const (
	TargetAppPage        TargetKind = "lightning__AppPage"
	TargetHomePage       TargetKind = "lightning__HomePage"
	TargetRecordPage     TargetKind = "lightning__RecordPage"
	TargetRecordAction   TargetKind = "lightning__RecordAction"
	TargetGlobalAction   TargetKind = "lightning__GlobalAction"
	TargetTab            TargetKind = "lightning__Tab"
	TargetUrlAddressable TargetKind = "lightning__UrlAddressable"
	TargetUtilityBar     TargetKind = "lightning__UtilityBar"
)

type FormFactor string

const (
	FormFactorLarge FormFactor = "Large"
	FormFactorSmall FormFactor = "Small"
)

type ComponentMeta struct {
	BundleName    string
	Namespace     string
	APIName       string
	APIVersion    string
	IsExposed     bool
	MasterLabel   string
	Description   string
	Targets       []TargetKind
	Capabilities  []string
	TargetConfigs []TargetConfig
}

type TargetConfig struct {
	Targets              []TargetKind
	Properties           []DesignProperty
	Objects              []string
	SupportedFormFactors []FormFactor
	ActionType           string
	Events               []DesignEvent
}

type DesignProperty struct {
	Name        string
	Type        string
	Label       string
	Description string
	Default     string
	Required    bool
	DataSource  string
	Min         string
	Max         string
	Placeholder string
}

type DesignEvent struct {
	Name        string
	Label       string
	Description string
	SchemaJSON  string
}
```

- [ ] **Step 1.2: Write metadata parser tests**

Add tests that parse:
- One component exposed to record, app, home, tab, action, and URL-addressable targets.
- One record page `targetConfig` with `objects`.
- One app page `targetConfig` with `supportedFormFactors`.
- One property using `Boolean`, `Integer`, `String`, `default`, `required`, and `datasource`.
- One tab target with no properties.

Run:

```bash
go test ./internal/lwcshell -run 'TestParseComponentMeta' -count=1
```

Expected:
- The first run fails because `ParseComponentMetaFile` does not exist.

- [ ] **Step 1.3: Implement full `*.js-meta.xml` parsing**

Implement `ParseComponentMetaFile(path string, namespace string) (ComponentMeta, error)` in `internal/lwcshell/meta.go`. Use `encoding/xml` structs. Preserve unknown target strings in diagnostics rather than dropping them silently.

Behavior:
- Default namespace to `c` when project namespace is empty.
- Compute `APIName` as `namespace + "/" + bundleName`.
- Parse `targetConfig targets="lightning__RecordPage,lightning__AppPage"` into two `TargetKind` values.
- Parse `objects>object`, `property`, `event`, `schema`, and `supportedFormFactors>supportedFormFactor type="Large"`.
- Reject malformed XML with file path in the error.
- Return an empty meta with `IsExposed=false` for missing meta files only when the caller asks through `ParseOptionalComponentMetaFile`.

- [ ] **Step 1.4: Keep existing `internal/lwc/meta.go` stable**

Either delegate `internal/lwc.ParseComponentMeta` to `internal/lwcshell.ParseComponentMetaFile` or leave it in place and add a comment naming `internal/lwcshell` as the full product parser. Do not break existing callers.

- [ ] **Step 1.5: Run tests**

```bash
go test ./internal/lwc ./internal/lwcshell -count=1
```

Expected:
- Existing `internal/lwc` tests still pass.
- New metadata tests pass.

- [ ] **Step 1.6: Commit**

```bash
git add internal/lwcshell internal/lwc/meta.go
git commit -m "feat: parse LWC shell metadata"
```

## Phase 2: FlexiPage, CustomTab, And App Metadata

**Goal:** Resolve record pages, app pages, home pages, and tabs from Salesforce metadata.

**Files:**
- Create: `internal/lwcshell/flexipage.go`
- Create: `internal/lwcshell/flexipage_test.go`
- Create: `internal/lwcshell/tabs.go`
- Create: `internal/lwcshell/tabs_test.go`
- Modify: `internal/project/project.go` only if more source file lists are needed.

- [ ] **Step 2.1: Define page metadata types**

Add to `internal/lwcshell/types.go`:

```go
type FlexiPage struct {
	Name        string
	Label       string
	Type        string
	SObjectType string
	Template    string
	Regions     []FlexiRegion
	Events      []FlexiEvent
}

type FlexiRegion struct {
	Name       string
	Type       string
	Components []FlexiComponent
	Fields     []FlexiField
}

type FlexiComponent struct {
	Identifier     string
	ComponentName  string
	Properties     map[string]FlexiPropertyValue
	VisibilityRule *VisibilityRule
}

type FlexiPropertyValue struct {
	Value     string
	ValueList []string
}

type FlexiField struct {
	FieldItem string
	Label     string
}

type VisibilityRule struct {
	BooleanFilter string
	Criteria      []VisibilityCriterion
}

type VisibilityCriterion struct {
	LeftValue  string
	Operator   string
	RightValue string
}

type CustomTab struct {
	Name      string
	Label     string
	Type      string
	Content   string
	Component string
}

type CustomApplication struct {
	Name     string
	Label    string
	NavItems []string
}
```

- [ ] **Step 2.2: Write FlexiPage parser tests**

Tests must cover:
- API 48 `componentInstances`.
- API 49+ `itemInstances>componentInstance`.
- `RecordPage`, `AppPage`, `HomePage`, and `UtilityBar`.
- Component properties with `value` and `valueList`.
- `componentName` values like `c:accountPanel`, `force:recordData`, `flexipage:tab`, and `flexipage:fieldSection`.
- A visibility rule that depends on record field values.

Run:

```bash
go test ./internal/lwcshell -run 'TestParseFlexiPage' -count=1
```

Expected:
- Fails before parser implementation.

- [ ] **Step 2.3: Implement FlexiPage parser**

Implement `ParseFlexiPageFile(path string) (FlexiPage, error)`.

Behavior:
- Use the filename as fallback `Name`.
- Preserve `sobjectType`.
- Convert standard tab titles such as `Standard.Tab.detail` into stable labels only for shell display; keep raw values in properties.
- Include unsupported component instances with `ComponentName` and a diagnostic, instead of dropping them.
- Preserve component `identifier` when present. Generate a deterministic identifier from region name and index when absent.

- [ ] **Step 2.4: Write tab and app tests**

Tests must cover:
- A Lightning Component custom tab that points to an LWC.
- A Lightning Page custom tab that points to a FlexiPage.
- A CustomApplication with nav items where a tab appears in the nav bar.

Run:

```bash
go test ./internal/lwcshell -run 'TestParseCustom(Tab|Application)' -count=1
```

Expected:
- Fails before parser implementation.

- [ ] **Step 2.5: Implement tab and app parsers**

Implement:

```go
func ParseCustomTabFile(path string) (CustomTab, error)
func ParseCustomApplicationFile(path string) (CustomApplication, error)
func LoadShellMetadata(p project.Project) (ProjectShellMetadata, error)
```

`ProjectShellMetadata` must contain component metas, FlexiPages, CustomTabs, CustomApplications, and diagnostics.

- [ ] **Step 2.6: Run tests**

```bash
go test ./internal/lwcshell ./internal/project ./internal/metadata -count=1
```

Expected:
- New parser tests pass.
- Existing project and metadata indexing tests still pass.

- [ ] **Step 2.7: Commit**

```bash
git add internal/lwcshell internal/project internal/metadata
git commit -m "feat: parse LWC page and tab metadata"
```

## Phase 3: Shell Context And Resolver

**Goal:** Convert command flags or Lightning-style URLs into a complete shell page model.

**Files:**
- Create: `internal/lwcshell/context.go`
- Create: `internal/lwcshell/resolver.go`
- Create: `internal/lwcshell/resolver_test.go`
- Create: `internal/lwcshell/context_test.go`

- [ ] **Step 3.1: Define the shell page model**

Add:

```go
type ShellTargetKind string

const (
	ShellTargetComponent      ShellTargetKind = "component"
	ShellTargetRecordPage     ShellTargetKind = "recordPage"
	ShellTargetAppPage        ShellTargetKind = "appPage"
	ShellTargetHomePage       ShellTargetKind = "homePage"
	ShellTargetTab            ShellTargetKind = "tab"
	ShellTargetQuickAction    ShellTargetKind = "quickAction"
	ShellTargetUrlAddressable ShellTargetKind = "urlAddressable"
)

type PageReference struct {
	Type       string            `json:"type"`
	Attributes map[string]string `json:"attributes,omitempty"`
	State      map[string]string `json:"state,omitempty"`
}

type ShellContext struct {
	TargetKind    ShellTargetKind `json:"targetKind"`
	PageReference PageReference   `json:"pageReference"`
	Namespace     string          `json:"namespace"`
	AppName       string          `json:"appName,omitempty"`
	TabAPIName    string          `json:"tabApiName,omitempty"`
	RecordID      string          `json:"recordId,omitempty"`
	ObjectAPIName string          `json:"objectApiName,omitempty"`
	ActionName    string          `json:"actionName,omitempty"`
	FormFactor    FormFactor      `json:"formFactor"`
	UserID        string          `json:"userId"`
	Locale        string          `json:"locale"`
	Theme         string          `json:"theme"`
}

type ShellPage struct {
	Context     ShellContext
	Title       string
	Chrome      ShellChrome
	Regions     []ShellRegion
	Diagnostics []Diagnostic
}

type ShellRegion struct {
	Name       string
	Kind       string
	Components []ShellComponent
	Fields     []FlexiField
}

type ShellComponent struct {
	Identifier    string
	Namespace     string
	Name          string
	TagName       string
	Properties    map[string]any
	RecordContext bool
	Diagnostics   []Diagnostic
}
```

- [ ] **Step 3.2: Write resolver tests**

Tests must cover:
- Direct component target with explicit `--record-id`.
- `/lightning/r/Account/001.../view`.
- `/lightning/page/Sales_Dashboard`.
- `/lightning/home/Custom_Home`.
- `/lightning/n/My_Custom_Tab`.
- `/lightning/cmp/c__accountPanel?c__mode=compact`.
- Missing record ID on a record page returns a diagnostic and non-200 CLI error.
- Component not exposed for a target returns a diagnostic that names the missing target.
- Record-page object restriction rejects unsupported objects.

Run:

```bash
go test ./internal/lwcshell -run 'TestResolveShellPage' -count=1
```

Expected:
- Fails before resolver implementation.

- [ ] **Step 3.3: Implement context builder**

Implement:

```go
type ResolveOptions struct {
	ProjectRoot    string
	Component      string
	RecordPage     string
	AppPage        string
	HomePage       string
	Tab            string
	URL            string
	RecordID       string
	ObjectAPIName  string
	AppName        string
	UserID         string
	Locale         string
	Theme          string
	FormFactor     FormFactor
}

func ResolveShellPage(meta ProjectShellMetadata, org *storage.OrgState, opts ResolveOptions) (ShellPage, error)
```

Resolution rules:
- `--url` wins over all target flags.
- `--record-page`, `--app-page`, `--home-page`, `--tab`, and `--component` are mutually exclusive.
- Record pages require a record ID unless a seed record can be selected by object type with one clear match.
- `objectApiName` comes from record ID lookup first, then FlexiPage `sobjectType`, then explicit option.
- Target properties start from `targetConfig` defaults and then merge FlexiPage component instance properties and URL state.
- Form factor defaults to `Large`.

- [ ] **Step 3.4: Add diagnostics**

Diagnostics must include stable codes:

```text
GLADELWC001 component metadata missing
GLADELWC002 target not exposed
GLADELWC003 record id required
GLADELWC004 record not found
GLADELWC005 unsupported flexipage component
GLADELWC006 unsupported page reference
GLADELWC007 unsupported form factor
GLADELWC008 unsupported base component
```

- [ ] **Step 3.5: Run tests**

```bash
go test ./internal/lwcshell -count=1
```

Expected:
- All resolver, context, metadata, and page parser tests pass.

- [ ] **Step 3.6: Commit**

```bash
git add internal/lwcshell
git commit -m "feat: resolve local LWC shell targets"
```

## Phase 4: Shell Rendering And Server Routes

**Goal:** Serve Salesforce-like shell pages for direct components, record pages, app pages, home pages, tabs, and URL-addressable components.

**Files:**
- Create: `internal/lwcshell/render.go`
- Create: `internal/lwcshell/render_test.go`
- Create: `internal/server/lwc_shell.go`
- Create: `internal/server/lwc_shell_test.go`
- Modify: `internal/server/lightning.go`
- Modify: `internal/server/glade_handlers.go` if route registration belongs there.

- [ ] **Step 4.1: Write render tests**

Tests must assert:
- HTML includes one import map.
- HTML includes shell context JSON with escaped content.
- Record page HTML includes app chrome, record header, action bar, and named regions.
- App page HTML includes app nav and configured regions.
- Custom tab HTML includes tab label and resolved component or page.
- Unsupported FlexiPage components render diagnostics in the debug panel and do not break sibling components.

Run:

```bash
go test ./internal/lwcshell -run 'TestRenderShell' -count=1
```

Expected:
- Fails before renderer exists.

- [ ] **Step 4.2: Implement shell renderer**

`RenderShellPage(page ShellPage, cfg lwcbrowser.PageConfig) (string, error)` must output:
- Minimal Salesforce-like app chrome.
- Header with app name, nav item, page title, and form factor class.
- Record page header with object label, record name when present, record ID, and common actions.
- Region containers with stable `data-region`, `data-component-id`, `data-component-name`, and `data-glade-diagnostic` attributes.
- Direct component pages without extra card wrappers around the component.
- Debug panel hidden behind a query flag: `?gladeDebug=1`.

Use restrained CSS. Do not attempt pixel-exact Salesforce styling. Use SLDS class names where useful and local CSS where needed.

- [ ] **Step 4.3: Write server route tests**

Tests must hit:

```text
/lightning/local/component/c/accountPanel
/lightning/r/Account/001000000000001AAA/view
/lightning/page/Sales_Dashboard
/lightning/home/Custom_Home
/lightning/n/My_Custom_Tab
/lightning/cmp/c__accountPanel?c__mode=compact
/lightning/local/context.json?url=/lightning/r/Account/001000000000001AAA/view
```

Run:

```bash
go test ./internal/server -run 'TestLWCShellRoutes' -count=1
```

Expected:
- Fails before route implementation.

- [ ] **Step 4.4: Implement routes**

Add server routing:
- `GET /lightning/local/component/{namespace}/{component}`
- `GET /lightning/r/{objectApiName}/{recordId}/{actionName}`
- `GET /lightning/page/{pageName}`
- `GET /lightning/home/{pageName}`
- `GET /lightning/n/{tabApiName}`
- `GET /lightning/cmp/{namespace}__{component}`
- `GET /lightning/local/context.json`

Routes must share the same resolver. HTML and JSON must match for the same URL.

- [ ] **Step 4.5: Run tests**

```bash
go test ./internal/lwcshell ./internal/server -count=1
```

Expected:
- Shell render and route tests pass.

- [ ] **Step 4.6: Commit**

```bash
git add internal/lwcshell internal/server
git commit -m "feat: serve local LWC shell routes"
```

## Phase 5: CLI Developer Loop

**Goal:** Give users one command that starts the shell, opens the right page, watches files, and prints usable routes.

**Files:**
- Create: `internal/gladecli/dev_lwc_command.go`
- Create: `internal/gladecli/dev_lwc_command_test.go`
- Create: `internal/gladecli/lwc_test_command.go`
- Create: `internal/gladecli/lwc_probe_command.go`
- Modify: `internal/gladecli/dev_command.go`
- Modify: `internal/cliui/help.go`
- Modify: `docs/LOCAL_TESTING.md`
- Modify: `site/docs-src/guide/local-testing.md`

- [ ] **Step 5.1: Write CLI parser tests**

Tests must cover:
- `glade dev lwc --component c/accountPanel`
- `glade dev lwc --record-page Account_Record_Page --record-id 001...`
- `glade dev lwc --app-page Sales_Dashboard`
- `glade dev lwc --home-page Custom_Home`
- `glade dev lwc --tab My_Custom_Tab`
- `glade dev lwc --url /lightning/r/Account/001.../view`
- Mutually exclusive target flags.
- Missing `--record-id` for record page.
- `--port`, `--addr`, `--open`, `--watch`, `--seed`, `--form-factor`, `--user`.

Run:

```bash
go test ./internal/gladecli -run 'TestRunDevLWC' -count=1
```

Expected:
- Fails before command exists.

- [ ] **Step 5.2: Implement `glade dev lwc`**

Behavior:
- Load the project.
- Load shell metadata.
- Load seed data into `storage.OrgState` when `--seed` is present.
- Prepare LWC browser config.
- Set project runtime for Apex VM calls.
- Print the server URL, selected target, resolved local URL, context JSON URL, and watched root.
- Watch `.js`, `.html`, `.css`, `.js-meta.xml`, `.cls`, `.labels`, static resources, FlexiPages, tabs, and applications.
- Reset LWC bundle cache and shell metadata on relevant changes.

Startup text shape:

```text
LWC dev server: http://127.0.0.1:8080
Target: record-page Account_Record_Page
Open: http://127.0.0.1:8080/lightning/r/Account/001000000000001AAA/view
Context: http://127.0.0.1:8080/lightning/local/context.json?url=/lightning/r/Account/001000000000001AAA/view
Watching /path/to/project for LWC, Apex, metadata, and static resource changes.
```

- [ ] **Step 5.3: Implement `glade lwc test`**

`glade lwc test` must start a temporary server, launch the runtime browser tests, and fail on browser console errors unless the support map marks them as expected diagnostics.

Required flags:

```text
--project
--target
--record-id
--browser
--headed
--screenshot-dir
--seed
```

- [ ] **Step 5.4: Keep scratch probing out of base `glade`**

Document the compat command in help and docs, but do not add a base product wrapper. The capture logic and scratch-org dependency stay in `glade-tools`.

- [ ] **Step 5.5: Run tests**

```bash
go test ./internal/gladecli ./internal/server ./internal/lwcshell -count=1
```

Expected:
- CLI parser tests pass.
- Shell server still passes.

- [ ] **Step 5.6: Commit**

```bash
git add internal/gladecli internal/cliui docs/LOCAL_TESTING.md site/docs-src/guide/local-testing.md
git commit -m "feat: add local LWC dev command"
```

## Phase 6: Runtime Boot, Hot Reload, And Module Map

**Goal:** Boot shell pages in the browser with live component modules, context, and refresh on save.

**Files:**
- Create: `lwcruntime/src/shell/context.mjs`
- Create: `lwcruntime/src/shell/hot-reload.mjs`
- Create: `lwcruntime/src/shell/boot.mjs`
- Modify: `lwcruntime/esbuild.config.mjs`
- Modify: `internal/lwcbrowser/bootstrap.go`
- Modify: `internal/lwcbrowser/manifest.go`
- Modify: `internal/lwcbrowser/salesforce_modules.go`
- Modify: `internal/gladehome/root.go`
- Modify: `internal/gladehome/install.go`
- Modify: `scripts/release-build.sh`

- [ ] **Step 6.1: Write browser boot test**

Add `lwcruntime/test/lwc-shell.test.mjs` with a test that:
- Starts a test server.
- Serves shell context JSON.
- Loads a compiled component in a region.
- Asserts the component received `recordId`, `objectApiName`, and target property defaults.

Run:

```bash
cd lwcruntime
npm test
```

Expected:
- Fails before boot modules exist.

- [ ] **Step 6.2: Implement context provider**

`context.mjs` must export:

```javascript
export function getShellContext()
export function setShellContext(context)
export function subscribeShellContext(listener)
```

Behavior:
- Read initial context from `<script type="application/json" id="glade-lwc-context">`.
- Freeze context values before handing them to components.
- Notify subscribers after navigation or hot reload.

- [ ] **Step 6.3: Implement shell boot**

`boot.mjs` must:
- Read `window.__GLADE_LWC_PAGE__` or the context script.
- For each `[data-component-name]`, load the component module from the manifest.
- Set public props from shell component properties and record context.
- Preserve sibling component rendering when one component fails.
- Render an inline diagnostic for failed components and emit a console error with the diagnostic code.

- [ ] **Step 6.4: Implement hot reload**

Use server-sent events:

```text
GET /lightning/local/events
event: reload
data: {"reason":"lwc","paths":["force-app/main/default/lwc/accountPanel/accountPanel.js"]}
```

The browser reloads the page on metadata changes and hot-swaps component modules on component-only changes when the import URL cache key changes.

- [ ] **Step 6.5: Expand import map**

Map:
- `@salesforce/apex/`
- `@salesforce/apexContinuation/`
- `@salesforce/client/formFactor`
- `@salesforce/community/`
- `@salesforce/contentAssetUrl/`
- `@salesforce/customPermission/`
- `@salesforce/i18n/`
- `@salesforce/label/`
- `@salesforce/messageChannel/`
- `@salesforce/resourceUrl/`
- `@salesforce/schema/`
- `@salesforce/user/`
- `lightning/navigation`
- `lightning/pageReferenceUtils`
- `lightning/platformShowToastEvent`
- `lightning/uiRecordApi`
- `lightning/uiObjectInfoApi`
- `lightning/uiRelatedListApi`
- `lightning/uiListsApi`
- `lightning/messageService`

- [ ] **Step 6.6: Run tests**

```bash
cd lwcruntime && npm test
cd .. && go test ./internal/lwcbrowser ./internal/gladehome ./internal/server -count=1
```

Expected:
- Runtime and Go tests pass.

- [ ] **Step 6.7: Commit**

```bash
git add lwcruntime internal/lwcbrowser internal/gladehome internal/server scripts/release-build.sh
git commit -m "feat: boot LWC shell runtime"
```

## Phase 7: Apex Controller Bridge

**Goal:** Run actual project Apex controller methods from LWC wire and imperative calls.

**Files:**
- Create: `internal/server/lightning_apex.go`
- Create: `internal/server/lightning_apex_test.go`
- Modify: `internal/server/lightning_wire.go`
- Modify: `internal/lwcbrowser/salesforce_modules.go`
- Modify: `lwcruntime/src/shims/wire-adapter.mjs`

- [ ] **Step 7.1: Write Apex bridge tests**

Tests must cover:
- Apex wire calls `@AuraEnabled(cacheable=true)` method.
- Imperative Apex call invokes the same method with object-shaped params.
- `undefined` params suppress the call on the client.
- `null` params call the method.
- Apex exception returns `{ body: { message, stackTrace, exceptionType } }`.
- SObject, List<SObject>, Map<String,Object>, Date, Datetime, Decimal, Id, and Boolean serialize in Salesforce-like JSON shape.

Run:

```bash
go test ./internal/server -run 'TestLightningApex' -count=1
```

Expected:
- Fails before imperative endpoint and full shaping exist.

- [ ] **Step 7.2: Implement imperative endpoint**

Route:

```text
POST /lightning/apex/invoke
```

Request:

```json
{"className":"AccountPanelController","method":"updateName","params":{"accountId":"001000000000001AAA","name":"Local Rename"}}
```

Response on success:

```json
{"data":{"id":"001000000000001AAA","name":"Local Rename"}}
```

Response on Apex error:

```json
{"error":{"body":{"message":"boom","exceptionType":"System.QueryException","stackTrace":"AccountPanelController.updateName: line 7"},"status":500}}
```

- [ ] **Step 7.3: Enforce `@AuraEnabled` method visibility**

Local calls must reject methods not annotated for Aura/LWC use. Cacheable methods may be wired. Non-cacheable methods may be called imperatively.

- [ ] **Step 7.4: Implement client Apex modules**

Generated module for `@salesforce/apex/Class.method` must export a default function that:
- Detects wire registration path when used through `@wire`.
- Calls `/lightning/apex/invoke` when called imperatively.
- Returns a Promise.
- Preserves Salesforce's object-shaped params rule.

- [ ] **Step 7.5: Run tests**

```bash
go test ./internal/server ./internal/lwcbrowser ./internal/vm -count=1
cd lwcruntime && npm test
```

Expected:
- Apex bridge, existing VM, and runtime tests pass.

- [ ] **Step 7.6: Commit**

```bash
git add internal/server internal/lwcbrowser lwcruntime
git commit -m "feat: run LWC Apex controllers locally"
```

## Phase 8: LDS And UI API

**Goal:** Give local LWC pages a real data loop: read, mutate, refresh, invalidate, and re-emit records and metadata.

**Files:**
- Create: `internal/server/lightning_lds.go`
- Create: `internal/server/lightning_lds_test.go`
- Modify: `internal/server/lightning_wire.go`
- Modify: `internal/lwcbrowser/wire.go`
- Modify: `internal/lwcbrowser/salesforce_modules.go`
- Create: `lwcruntime/src/shims/lds-cache.mjs`
- Modify: `lwcruntime/src/shims/wire-adapter.mjs`

- [ ] **Step 8.1: Write LDS tests**

Tests must cover:
- `getRecord`
- `getRecords`
- `getFieldValue`
- `getFieldDisplayValue`
- `createRecord`
- `updateRecord`
- `deleteRecord`
- `notifyRecordUpdateAvailable`
- `getObjectInfo`
- `getObjectInfos`
- `getPicklistValues`
- `getPicklistValuesByRecordType`
- `getRelatedListInfo`
- `getRelatedListRecords`
- `getRelatedListsInfo`
- `getListUi` as deprecated but supported when local data exists.

Run:

```bash
go test ./internal/server -run 'TestLightningLDS' -count=1
```

Expected:
- Fails before LDS expansion exists.

- [ ] **Step 8.2: Implement server UI API routes**

Routes:

```text
POST /lightning/wire/getRecord
POST /lightning/wire/getRecords
POST /lightning/wire/getObjectInfo
POST /lightning/wire/getObjectInfos
POST /lightning/wire/getPicklistValues
POST /lightning/wire/getPicklistValuesByRecordType
POST /lightning/wire/getRelatedListInfo
POST /lightning/wire/getRelatedListRecords
POST /lightning/wire/getRelatedListsInfo
POST /lightning/wire/getListUi
POST /lightning/lds/createRecord
POST /lightning/lds/updateRecord
POST /lightning/lds/deleteRecord
```

Use `storage.OrgState` and existing describe data. Return a stable unsupported diagnostic when the local org lacks enough metadata.

- [ ] **Step 8.3: Implement client LDS cache**

`lds-cache.mjs` must:
- Deduplicate equivalent wire requests in one tick.
- Cache record and object-info responses by user, app, and shell context.
- Re-emit immutable snapshots to subscribers.
- Invalidate on create, update, delete, and `notifyRecordUpdateAvailable`.
- Support `refreshApex` for Apex wire values and warn when used for non-Apex wire values.

- [ ] **Step 8.4: Run browser data loop test**

`lwcruntime/test/lwc-shell.test.mjs` must:
- Load a record page.
- Read Account name through `getRecord`.
- Click a button that calls Apex and `updateRecord`.
- Assert all components showing the same record update without a full reload.

Run:

```bash
cd lwcruntime && npm test
```

Expected:
- Data changes render in the browser test.

- [ ] **Step 8.5: Run tests**

```bash
go test ./internal/server ./internal/lwcbrowser ./internal/lwcshell -count=1
cd lwcruntime && npm test
```

Expected:
- LDS and shell tests pass.

- [ ] **Step 8.6: Commit**

```bash
git add internal/server internal/lwcbrowser internal/lwcshell lwcruntime
git commit -m "feat: add local LDS and UI API support"
```

## Phase 9: Base Lightning Components

**Goal:** Support the base components most LWC pages need for local development.

**Files:**
- Create: `lwcruntime/src/base-components/`
- Create: `lwcruntime/test/base-components.test.mjs`
- Modify: `internal/lwcbrowser/salesforce_modules.go`
- Modify: `internal/lwcbrowser/manifest.go`
- Modify: `internal/lwcshell/support.go`

- [ ] **Step 9.1: Build Tier 1 base components**

Tier 1 must be complete before the phase closes:
- `lightning-card`
- `lightning-button`
- `lightning-button-icon`
- `lightning-input`
- `lightning-combobox`
- `lightning-textarea`
- `lightning-layout`
- `lightning-layout-item`
- `lightning-tabset`
- `lightning-tab`
- `lightning-badge`
- `lightning-icon`
- `lightning-spinner`
- `lightning-formatted-text`
- `lightning-formatted-number`
- `lightning-formatted-date-time`
- `lightning-helptext`

Each component needs:
- Public properties used by Salesforce docs.
- Basic slots.
- Events with Salesforce names.
- Disabled and error states when applicable.
- Tests that verify DOM, props, events, and accessibility labels.

- [ ] **Step 9.2: Build Tier 2 record and table components**

Tier 2 must be complete before the phase closes:
- `lightning-record-form`
- `lightning-record-view-form`
- `lightning-record-edit-form`
- `lightning-output-field`
- `lightning-input-field`
- `lightning-messages`
- `lightning-datatable`
- `lightning-tree-grid`
- `lightning-related-list`

Each component must use local LDS. Record edit must save to `storage.OrgState`.

- [ ] **Step 9.3: Build Tier 3 overlay and feedback components**

Tier 3 must be complete before the phase closes:
- `lightning/platformShowToastEvent`
- `lightning-modal`
- `lightning-quick-action-panel`
- `lightning-navigation`
- `lightning-file-upload` with local diagnostic for actual upload persistence.

- [ ] **Step 9.4: Test base components**

```bash
cd lwcruntime && npm test
go test ./internal/lwcbrowser ./internal/lwcshell -count=1
```

Expected:
- Base component tests pass.
- Unsupported base components render `GLADELWC008` with component name.

- [ ] **Step 9.5: Commit**

```bash
git add lwcruntime internal/lwcbrowser internal/lwcshell
git commit -m "feat: add local Lightning base components"
```

## Phase 10: Navigation, Toasts, LMS, And Actions

**Goal:** Make shell services feel like Lightning Experience for local workflows.

**Files:**
- Create: `lwcruntime/src/shims/navigation.mjs`
- Create: `lwcruntime/src/shims/message-service.mjs`
- Create: `lwcruntime/test/navigation.test.mjs`
- Create: `lwcruntime/test/message-service.test.mjs`
- Modify: `internal/server/lwc_shell.go`
- Modify: `internal/lwcshell/resolver.go`
- Modify: `internal/lwcshell/context.go`

- [ ] **Step 10.1: Implement PageReference support**

Support:
- `standard__app`
- `standard__navItemPage`
- `standard__objectPage`
- `standard__recordPage`
- `standard__recordRelationshipPage`
- `standard__quickAction`
- `standard__component`
- `standard__namedPage`
- `standard__webPage`

`NavigationMixin.Navigate` must update the shell route for supported local PageReferences. `NavigationMixin.GenerateUrl` must return a local URL.

- [ ] **Step 10.2: Implement `CurrentPageReference`**

`CurrentPageReference` must be a wire adapter backed by shell context. It re-emits after local navigation.

- [ ] **Step 10.3: Implement toast and modal services**

The shell owns one toast region and one modal region. Toast events and `LightningModal.open()` render there.

- [ ] **Step 10.4: Implement Lightning Message Service**

Support:
- `@salesforce/messageChannel/*`
- `createMessageContext`
- `releaseMessageContext`
- `publish`
- `subscribe`
- `unsubscribe`
- application scope in one shell window.

- [ ] **Step 10.5: Run tests**

```bash
cd lwcruntime && npm test
go test ./internal/lwcshell ./internal/server -count=1
```

Expected:
- Navigation and message service tests pass.

- [ ] **Step 10.6: Commit**

```bash
git add lwcruntime internal/lwcshell internal/server
git commit -m "feat: add LWC shell services"
```

## Phase 11: Browser Test Runner

**Goal:** Let users test LWCs against real local controllers without hand-written harnesses.

**Files:**
- Modify: `internal/gladecli/lwc_test_command.go`
- Create: `internal/gladecli/lwc_test_command_test.go`
- Create: `internal/lwcshell/testspec.go`
- Create: `internal/lwcshell/testspec_test.go`
- Create: `testdata/local-tests/lwc-shell/glade-lwc-test.json`

- [ ] **Step 11.1: Define test spec**

Support this file:

```json
{
  "seed": "seed.json",
  "tests": [
    {
      "name": "account record page",
      "target": "record-page:Account_Record_Page",
      "recordId": "001000000000001AAA",
      "assertText": [
        {"selector": "c-account-panel [data-testid='account-name']", "text": "Acme"}
      ],
      "click": [
        {"selector": "c-account-panel button[data-action='rename']"}
      ],
      "assertTextAfter": [
        {"selector": "c-account-panel [data-testid='account-name']", "text": "Local Rename"}
      ]
    }
  ]
}
```

- [ ] **Step 11.2: Implement runner**

`glade lwc test --project . --spec glade-lwc-test.json` must:
- Start a local server on an available port.
- Load seed data.
- Run each target in Playwright.
- Fail on console errors unless they match support diagnostics in the spec.
- Save screenshots when `--screenshot-dir` is present.
- Print one line per test with elapsed time.

- [ ] **Step 11.3: Run tests**

```bash
go test ./internal/gladecli ./internal/lwcshell -run 'TestLWCTest' -count=1
glade lwc test --project testdata/local-tests/lwc-shell --spec glade-lwc-test.json
```

Expected:
- Unit tests pass.
- CLI run prints one passing test and exits 0.

- [ ] **Step 11.4: Commit**

```bash
git add internal/gladecli internal/lwcshell testdata/local-tests/lwc-shell
git commit -m "feat: test LWC shell pages locally"
```

## Phase 12: Scratch Org Oracle In glade-tools

**Goal:** Verify local behavior against Salesforce with `oaer-probe-max`.

**Files in sibling repo `../glade-tools`:**
- Create: `internal/lwcprobe/capture.go`
- Create: `internal/lwcprobe/compare.go`
- Create: `internal/lwcprobe/capture_test.go`
- Modify: `cmd/glade-tools/main.go`
- Create: `docs/lwc-probe.md`

**Files in this repo:**
- Create: `docs/LWC_SUPPORT.md`
- Modify: `internal/gladecli/lwc_probe_command.go`

- [ ] **Step 12.1: Extend capture command in `glade-tools`**

The existing `lwc capture` command prepares fixture-manifest rows. This phase
extends it, or adds a subcommand under `lwc`, so Salesforce pages open in a
browser and produce oracle artifacts. Do not treat fixture-manifest output as
parity evidence.

Command:

```bash
cd ../glade-tools
go run ./cmd/glade-plugin-compat lwc capture \
  --project ../glade/testdata/local-tests/lwc-shell \
  --target-org oaer-probe-max \
  --targets record-page,app-page,home-page,custom-tab \
  --include-hosts lightning-shell,visualforce-lightning-out \
  --out /tmp/glade-lwc-shell-oracle
```

Capture:
- Salesforce URL.
- DOM snapshot after network idle.
- Screenshot.
- Console messages.
- Wire payloads when observable.
- Current page reference.
- Exposed `recordId`, `objectApiName`, and target properties.

- [ ] **Step 12.2: Implement compare command in `glade-tools`**

Command:

```bash
cd ../glade-tools
go run ./cmd/glade-plugin-compat lwc compare \
  --local /tmp/glade-lwc-shell-local \
  --oracle /tmp/glade-lwc-shell-oracle \
  --out /tmp/glade-lwc-shell-diff
```

Compare:
- DOM semantic markers.
- Record context.
- PageReference shape.
- Wire payload keys and required values.
- Screenshots with configurable threshold.
- Console errors.
- Unsupported local diagnostics.

- [ ] **Step 12.3: Keep product boundary clean**

Base `glade` must not add scratch-org capture code. Product docs may point to the first-party compat plugin command.

- [ ] **Step 12.4: Run probe**

```bash
cd ../glade-tools
go run ./cmd/glade-plugin-compat lwc capture \
  --project ../glade/testdata/local-tests/lwc-shell \
  --target-org oaer-probe-max \
  --targets record-page \
  --out /tmp/glade-lwc-shell-probe
```

Expected:
- Browser oracle capture exists, not just a fixture manifest.
- Compare report lists pass/fail per contract.
- Known differences have stable support codes.

- [ ] **Step 12.5: Commit**

In `../glade-tools`:

```bash
git add internal/lwcprobe cmd docs
git commit -m "feat: capture LWC shell parity probes"
```

In this repo:

```bash
git add internal/gladecli docs/LWC_SUPPORT.md
git commit -m "feat: add LWC parity probe wrapper"
```

## Phase 13: Documentation And Support Map

**Goal:** Tell users exactly what works, how to run it, and where support stops.

**Files:**
- Create: `docs/LWC_LOCAL_SHELL.md`
- Create: `docs/LWC_SUPPORT.md`
- Modify: `docs/LOCAL_TESTING.md`
- Modify: `site/docs-src/guide/local-testing.md`
- Modify: `site/docs-src/guide/support-map.md` if present.

- [ ] **Step 13.1: Write local shell docs**

`docs/LWC_LOCAL_SHELL.md` must include:
- One-command quick start.
- Direct component preview.
- Record page preview.
- App page preview.
- Home page preview.
- Custom tab preview.
- URL-addressable component preview.
- Seeding local data.
- Running against actual Apex controllers.
- Running browser tests.
- Running `oaer-probe-max` probe.
- Reading diagnostics.

- [ ] **Step 13.2: Write support table**

`docs/LWC_SUPPORT.md` must include tables:
- Page targets.
- Metadata tags.
- PageReference types.
- `@salesforce/*` modules.
- `lightning/*` modules.
- LDS/UI API adapters and functions.
- Base Lightning components.
- Shell services.
- Known non-goals.

Each row has:

```text
surface | status | evidence command | notes
```

- [ ] **Step 13.3: Update public guide**

The public guide should point to the workflow first:

```text
glade dev lwc --record-page Account_Record_Page --record-id 001...
```

Keep generated ledgers out of public CLI help. Put detailed support in docs.

- [ ] **Step 13.4: Run docs checks**

```bash
rg -n "glade dev lwc|LWC local shell|oaer-probe-max" docs site/docs-src
git diff --check
```

Expected:
- Commands appear in docs.
- Whitespace check passes.

- [ ] **Step 13.5: Commit**

```bash
git add docs site/docs-src
git commit -m "docs: document local LWC shell support"
```

## Phase 14: Hardening, Performance, And Release

**Goal:** Make the feature strong enough for daily local use.

**Files:**
- Modify: touched packages only.
- Modify: `docs/LWC_SUPPORT.md`.
- Modify: release packaging files if tests expose gaps.

- [ ] **Step 14.1: Add performance budget tests**

Budgets on the fixture project:
- Dev server cold start under 3 seconds after Go binary exists.
- LWC rebuild after one component edit under 750 ms.
- Browser hot reload under 1.5 seconds.
- Record page with 10 components renders under 2 seconds after server start.

Command:

```bash
go test ./internal/gladecli ./internal/server -run 'TestLWC.*Performance' -count=1
```

Expected:
- Tests pass on the development machine or print measured values with failure threshold.

- [ ] **Step 14.2: Run full focused suite**

```bash
go test ./internal/lwc ./internal/lwcbrowser ./internal/lwcshell ./internal/server ./internal/gladecli ./internal/vm ./internal/lwcruntime -count=1
cd lwcruntime && npm test
```

Expected:
- All focused tests pass.

- [ ] **Step 14.3: Run scratch fixture-manifest suite**

```bash
cd ../glade-tools
go run ./cmd/glade-plugin-compat lwc capture \
  --project ../glade/testdata/local-tests/lwc-shell \
  --target-org oaer-probe-max \
  --targets record-page,app-page,home-page,custom-tab \
  --include-hosts lightning-shell,visualforce-lightning-out \
  --out /tmp/glade-lwc-shell-fixture-manifest.json
```

Expected:
- Report has `mode:"fixture-manifest"` and prepared target rows.
- This proves fixture deploy and target enumeration only. Browser DOM, console, screenshot, and wire payload parity capture is future oracle work in `glade-tools`.

- [ ] **Step 14.4: Run broader checks**

```bash
go test ./...
scripts/smoke.sh
git diff --check
```

Expected:
- Broad checks pass.
- If `scripts/smoke.sh` is too slow for the machine, run the documented focused replacement and record the reason in the final handoff.

- [ ] **Step 14.5: Commit**

```bash
git add .
git commit -m "feat: harden local LWC shell support"
```

## Phase Completion Gates

A phase is not done until:
- Its fixture proves the user-visible behavior.
- Its unit tests fail before implementation and pass after implementation.
- Browser tests cover any rendered shell or runtime behavior.
- Docs or support rows exist for every public behavior added.
- Scratch-org comparison exists for target behavior when Salesforce behavior matters.
- The final response names every command run and its result.

## Agent Handoff Prompt

Use this prompt when assigning a phase:

```text
You are implementing Phase <N> of docs/superpowers/plans/2026-06-14-lwc-local-shell-parity-plan.md.
Use superpowers:subagent-driven-development.
Work only on the files named in the phase unless tests prove a narrow supporting change is required.
Start by writing the failing tests named in the phase.
Run the exact expected failure command.
Implement the smallest product code that passes the tests.
Run the phase verification commands.
Return changed files, command output summaries, and any support diagnostics added.
Do not change Visualforce behavior unless a shared server route demands a narrow refactor with tests.
```

## Final Parity Definition

This roadmap reaches practical LWC parity when all of these pass:
- `glade dev lwc` renders direct component, record page, app page, home page, tab, quick action, and URL-addressable routes.
- Components receive Salesforce-like context: `recordId`, `objectApiName`, app/nav state, PageReference, user, locale, theme, and form factor.
- LWC metadata and FlexiPage metadata drive the shell. No hand-written harness is required for normal use.
- Apex wire and imperative calls invoke project Apex in the Glade VM against local org state.
- LDS-style record reads, writes, cache invalidation, object info, picklists, related lists, and list views work for local data.
- Tier 1, Tier 2, and Tier 3 base Lightning components support the fixture suite.
- Navigation, toasts, modals, and Lightning Message Service work inside one shell window.
- Future `glade lwc test` runs browser tests against the real local shell and controllers.
- `go run ./cmd/glade-plugin-compat lwc capture --target-org oaer-probe-max` in `../glade-tools` prepares fixture-manifest targets, and the later browser/oracle command produces checked parity evidence.
- `docs/LWC_SUPPORT.md` lists every supported, partial, and intentionally unsupported LWC surface with evidence.
