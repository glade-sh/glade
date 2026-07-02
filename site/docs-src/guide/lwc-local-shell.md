# Local LWC Shell

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Local Lightning</p>
  <p>Serve Lightning Web Components from an SFDX project with the Workbench Console, route discovery, page context, local data, diagnostics, and a Glade-owned Lightning preview canvas.</p>
  <ul>
    <li>Start <code>glade dev lwc --open</code>.</li>
    <li>Select contexts from <code>glade.lwc.json</code>.</li>
    <li>Open tabs from home or compose test pages at <code>/lwc/builder</code>.</li>
    <li>Keep Salesforce for hosted Lightning Experience behavior.</li>
  </ul>
</div>

Use [Preview LWC locally](/guide/workflows/lwc-preview) for the task path. This page is the deeper route, context, service, fixture, and diagnostic reference.

The LWC local shell is a preview feature. Glade can render LWCs from local
source without a deploy, but it does not replace hosted Lightning Experience.
It reads LWC bundle metadata, FlexiPages, custom apps, custom tabs, quick
actions, community contexts, Apex classes, labels, resources, and local
fixtures from the project on disk.

## Setup

Install the local LWC toolchain:

```bash
glade toolchain install
```

Start the Workbench Console:

```bash
glade dev lwc --project . --open
```

Use a DB-backed local org when record-page LWCs, LDS, Apex, or the builder's
record picker should use persisted data:

```bash
glade db seed --db .glade/envs/lwc-preview.sqlite --project . data/lwc-preview-db.json
glade dev lwc --project . --db .glade/envs/lwc-preview.sqlite --open
```

The printed base URL opens the Workbench Console at `/`; `/lwc` is the stable
home link. The console has route discovery, a preview canvas, editable context,
and debug panes for Apex, LDS, network calls, navigation/events, and runtime
issues. Use the **Open builder** link or `/lwc/builder` to compose a local page,
drag LWCs into canvas regions, switch layouts, keep record fields out of app and
home contexts, search objects and records from the active local DB for record
pages, and tune app, community, Flow, and PageReference context. Mobile preview
uses the main canvas viewport control and does not reserve a side-by-side phone panel.

Open a named context:

```bash
glade dev lwc --project . --context accountRecord --open
```

Open one route with flags:

```bash
glade dev lwc --project . --target record-page --object Account --record 001000000000001AAA --page Account_Record_Page --open
```

For scripts:

```bash
glade dev lwc --project . --addr 127.0.0.1:0 --ready-file /tmp/glade-lwc-ready.json
```

Use a fixed local port when browser bookmarks or local tools expect one:

```bash
glade dev lwc --project . --port 8080 --open
```

The ready file includes `url`, `selectedUrl`, `selectedContext`, and the route
list. The VS Code extension does not manage this server yet. Start, stop, and
route selection stay in the terminal while this preview workflow matures.

## Context presets

Put reusable contexts in `glade.lwc.json` at the project root:

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
    "ldsRecord": {
      "target": "component",
      "component": "c:recordProbe",
      "objectApiName": "Account",
      "recordId": "001000000000001AAA",
      "app": "Sales",
      "formFactor": "Large"
    },
    "recordAction": {
      "target": "recordAction",
      "objectApiName": "Account",
      "recordId": "001000000000001AAA",
      "action": "Update_Status",
      "app": "Sales",
      "formFactor": "Large"
    },
    "utilityBar": {
      "target": "utilityBar",
      "page": "Support_Utility",
      "app": "Support",
      "formFactor": "Large"
    },
    "membershipFlow": {
      "target": "flowScreen",
      "component": "c:communityFlow2",
      "app": "Support",
      "formFactor": "Large",
      "flow": {
        "apiName": "Membership_Flow",
        "inputVariables": {
          "recordId": "001000000000001AAA"
        }
      }
    },
    "phase3BaseComponents": {
      "target": "component",
      "component": "c:baseComponentHost",
      "app": "Sales",
      "formFactor": "Large",
      "state": {
        "c__lane": "phase3"
      }
    },
    "communityAccount": {
      "target": "communityPage",
      "component": "c:communityProbe",
      "page": "Account",
      "formFactor": "Large",
      "community": {
        "site": "Partner_Portal",
        "basePath": "/partners",
        "siteId": "0DM000000000001",
        "networkId": "0DB000000000001",
        "guest": true,
        "language": "en-US"
      },
      "pageReference": {
        "type": "comm__namedPage",
        "attributes": {
          "name": "Account"
        },
        "state": {
          "c__view": "summary"
        }
      }
    }
  }
}
```

Supported targets are `component`, `urlAddressable`, `recordPage`, `appPage`,
`homePage`, `tab`, `recordAction`, `globalAction`, `utilityBar`, `flowScreen`,
`flowAction`, and `communityPage`. Community routes are preset-backed; use
`--context` or `--context-file` to select them. Community presets carry site,
base path, site ID, network ID, guest mode, language, and optional `comm__*`
PageReference data. Flow presets carry `flow.apiName` and optional
`flow.inputVariables`. Direct flags include `--component`, `--object`,
`--record`, `--page`, `--tab`, `--action`, `--app`, `--flow`,
`--flow-input key=value`, `--form-factor`, and repeated `--state key=value` for
non-community routes. Use `--context-file` when the presets live somewhere
other than the project-root `glade.lwc.json`:

```bash
glade dev lwc --project . --context-file config/lwc-contexts.json --context accountRecord --open
```

Direct flags override preset fields.

## Routes

The printed base URL opens the Workbench Console at `/`. `/lwc` opens the same
console for stable links. It lists available LWCs, filters by search text and
selected target, lets you place components into draft app, home, record, tab,
URL-addressable, action, and community contexts, and keeps active context and
diagnostics visible. Raw preview routes remain stable for scripts and bookmarks:

```text
/
/lwc
/lwc/preview/component/<namespace>/<component>
/lwc/preview/cmp/<namespace>/<component>?c__name=value
/lwc/preview/record/<Object>/<recordId>?page=<FlexiPage>
/lwc/preview/app/<Page>
/lwc/preview/home/<Page>
/lwc/preview/tab/<Tab>
/lwc/preview/utility/<UtilityBar>
/lwc/preview/flow/<FlowApiName>
/lwc/preview/action/<Object>/<recordId>/<ActionName>
/lwc/preview/action/global/<ActionName>
/lwc/preview/community/<site>/<page>
/lwc/preview/community/<site>/cmp/<namespace>/<component>
```

Examples:

```text
http://127.0.0.1:8080/
http://127.0.0.1:8080/lwc
http://127.0.0.1:8080/lwc/preview/component/c/contextProbe
http://127.0.0.1:8080/lwc/preview/cmp/c/actionProbe?c__name=value
http://127.0.0.1:8080/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page
http://127.0.0.1:8080/lwc/preview/app/Sales_Dashboard
http://127.0.0.1:8080/lwc/preview/home/Custom_Home
http://127.0.0.1:8080/lwc/preview/tab/Lwc_Probe
http://127.0.0.1:8080/lwc/preview/utility/Support_Utility
http://127.0.0.1:8080/lwc/preview/flow/Membership_Flow
http://127.0.0.1:8080/lwc/preview/action/Account/001000000000001AAA/Update_Status
http://127.0.0.1:8080/lwc/preview/action/global/Global_Status
http://127.0.0.1:8080/lwc/preview/community/Partner_Portal/Account
http://127.0.0.1:8080/lwc/preview/community/Partner_Portal/cmp/c/communityProbe
```

The shell uses `*.js-meta.xml` exposure and targets, FlexiPage regions and
properties, custom application navigation, and custom tab metadata.
Visualforce-backed custom tabs redirect to `/apex/<Page>` when the project
defines one.
URL-addressable routes pass `c__*` state through the local PageReference.
Quick action routes mount LWC-backed action metadata and pass action context
attributes. Unsupported action metadata returns `GLADELWC070`; unsupported
action types return `GLADELWC015`.
Utility-bar routes resolve `UtilityBar` FlexiPages, mount utility items, and
pass local workspace utility context.
Flow-screen routes resolve `flowScreen` presets, mount `lightning__FlowScreen`
components, and pass Flow input variables and local navigation events.
Flow-action routes use the quick-action route shape and require
`lightning__FlowAction` metadata.
Community routes resolve `communityPage` presets, mount
`lightningCommunity__Page` components, mount `lightningCommunity__Default`
direct components, expose community base path, site ID, network ID, guest mode,
language, and use a local `lightningCommunity__Theme_Layout` wrapper boundary
when one is present.
The generated route list includes direct components, app pages, record pages
with a sample record ID, home pages, tabs, URL-addressable components,
utility bars, Flow screens, LWC-backed actions, and configured community pages.

## Local context JSON

Tools can read the current local shell state:

```text
/lightning/local/context.json
/lightning/local/objects.json?q=acc
/lightning/local/records.json?object=Account&q=local
```

The JSON includes active route, PageReference, route context, mounted
components, discovered apps, route list, named context presets, default and
selected context names, diagnostics when present, and service support for Apex,
LDS, UI API shims, navigation, labels, resources, LMS, quick actions, base
components, Experience Cloud context, Visualforce host support, and toast. The
object and record endpoints expose the active local org data used by the
builder's record-page pickers.

## Data and services

The LWC shell supports:

- Apex wire and imperative imports through `@salesforce/apex/CLASS_NAME.methodName`
  in the LWC shell and Visualforce Lightning Out.
- Local Apex controller execution through the Glade VM, with request user
  context, deterministic cache keys for cacheable Apex wires, and
  `refreshApex` forced refreshes. Overloaded `@AuraEnabled` controller methods
  return `GLADELWC013` instead of picking an arbitrary overload.
- Raw imperative Apex calls can also post params to
  `/lightning/apex/<Class>/<method>`.
- `lightning/uiRecordApi` `getRecord`, `getRecords`, `getRecordCreateDefaults`,
  create, update, delete, field helper functions, and record-input helper
  functions against local records. `getRecord` plus create, update, and delete
  are browser-checked in the LWC shell and Visualforce Lightning Out.
  `optionalFields` are accepted; unknown required fields return a wire error and
  unknown optional fields are skipped. Soft-deleted records read as not found.
  Create defaults include object info, record defaults, and a create-mode
  layout from project field sections when `.layout-meta.xml` is available, with
  a generated full layout from createable fields as the local fallback.
- `lightning/uiObjectInfoApi` `getObjectInfo`, `getObjectInfos`,
  `getPicklistValues`, and `getPicklistValuesByRecordType` against local schema
  metadata. `getObjectInfo` is browser-checked in the LWC shell and Visualforce
  Lightning Out. Compatibility exports remain available from
  `lightning/uiRecordApi`.
- `lightning/uiLayoutApi` `getLayout` returns the same local Record Layout
  shape. `formFactor` is accepted, but distinct mobile/tablet layout variants
  remain a Salesforce check.
- `lightning/uiRelatedListApi` `getRelatedListRecords` for deterministic local
  child-relationship data. Compatibility export remains available from
  `lightning/uiRecordApi`; related-list metadata adapters remain a Salesforce
  check.
- `lightning/uiAppsApi` for local app, tab, and route metadata.
- `lightning/uiListsApi` for deterministic local list info, list rows, and list
  preferences. Local list metadata writes return a named unsupported error.
- `lightning/graphql` and `lightning/uiGraphQLApi` for the local UI API GraphQL
  response envelope. Deep GraphQL query semantics remain oracle-gated.
- `notifyRecordUpdateAvailable`, `getRecordNotifyChange`, and `refreshApex`
  refresh matching local record wires through the browser LDS cache.
- Deprecated `lightning/uiListApi` `getListUi` reports `GLADELWC050`; use
  `getRelatedListRecords` or local Apex.
- `@salesforce/schema` object and field tokens.
- `@salesforce/label`, `@salesforce/resourceUrl`, and
  `@salesforce/contentAssetUrl`.
- Static resource subpaths for local scripts, styles, and images.
- `@salesforce/client/formFactor`, `@salesforce/customPermission/*`, and
  `@salesforce/userPermission/*` shims for package components that branch on
  form factor or permissions.
- `@salesforce/user`, checked `@salesforce/i18n` values, and
  `lightning/navigation` basics. `@salesforce/user/isGuest` reads active
  community guest context and remains false on non-community routes.
- `@salesforce/community/basePath`, `@salesforce/community/Id`,
  `@salesforce/site/Id`, and `@salesforce/site/activeLanguages` for active
  community routes. Missing IDs export empty strings and report `GLADELWC102`.
- `@salesforce/apexContinuation` as a simulated local continuation helper for
  Promise-shaped controller flows. Hosted servlet continuation scheduling stays
  Salesforce-only.
- `comm__namedPage`, `comm__loginPage`, `comm__managedContentPage`,
  `comm__recordPage`, and `comm__recordRelationshipPage` local URL generation.
- `lightning/messageService`, `lightning/platformResourceLoader`,
  `lightning/platformShowToastEvent`, `lightning/showToastEvent`, and
  `lightning/toast` shims in the shell and Visualforce Lightning Out where the
  support table names that host.
- `lightning/platformWorkspaceApi` console approximation for local tab info,
  open/close/focus, refresh, highlight, label/icon helpers, and console wire
  values.
- `lightning/platformUtilityBarApi` utility item discovery and local
  open/close/minimize/focus, label, icon, highlighted-state, and callback
  helpers for utility-bar route tests.
- `lightning/alert`, `lightning/confirm`, `lightning/prompt`,
  `lightning/configProvider`, and `lightning/pageReferenceUtils` shims for
  overlay flows, icon token lookup, localization helpers, and default field
  value encode/decode helpers.
- `lightning/actions`, `lightning/flowSupport`, `lightning/refresh`, and
  `lightning/empApi` practical local shims for quick-action events, flow-screen
  events, in-page refresh handlers, and deterministic in-page EMP pub/sub.
- `lightning/flow` and `lightning/flowSupport` for local Flow screen hosts and
  navigation events. This is not Flow Builder.
- Datatable, record forms, input fields, output fields, and local LDS/UI API
  form messages.
- Practical local implementations for every public `lightning/*` name exposed
  by `lightning-base-components@1.28.19-alpha`: 118 module names checked from
  the npm package. Renderable components get local renderers; service and
  helper modules get local shims.
- Source-backed local implementations for an allowlist of base components:
  `lightning-badge`, `lightning-breadcrumb`, `lightning-breadcrumbs`,
  `lightning-button-group`, `lightning-datatable`, `lightning-input-field`,
  `lightning-messages`, `lightning-output-field`,
  `lightning-record-edit-form`, `lightning-record-form`,
  `lightning-record-picker`, `lightning-record-view-form`,
  `lightning-menu-subheader`, and `lightning-vertical-navigation-item`.
- Expanded checked `lightning-*` modules used by the checked LWC fixture,
  mounted through `c:baseComponentHost` and the `phase3BaseComponents`
  context. The set includes modal body/header/footer, alert/prompt/toast
  surfaces, display and formatted value helpers, name/location/address inputs,
  picklist and grouped combobox, barcode scanner, progress, tree and tree
  grid, map, carousel, vertical navigation variants, stacked tabs, and local
  overlay/popover containers.
- Deeper practical base-component contracts for common local development:
  button and icon-button variants, `lightning-card` title/actions/footer slots,
  `lightning-layout` alignment and sizing classes, and
  `lightning-formatted-number` decimal, currency, percent, and percent-fixed
  output.
- Packaged local SLDS 2 styling for shell and base-component previews, with
  classic SLDS assets available from the same runtime asset tree.

`CurrentPageReference` reports the local route context. `NavigationMixin`
supports local URL generation and navigation for supported page targets.

Checked capture rows are generated in `docs/generated/LWC_SHELL_SUPPORT.md`.
They record the local support matrix for the LWC shell.

Maintainers can run browser oracle captures from the `glade-tools` lane. See
[glade-tools maintainer guide](/maintainer/glade-tools).

That capture stores stable local and Salesforce paths and browser evidence
only. It does not write frontdoor login URLs or one-time tokens. When both
sides are captured, each case includes a selector-scoped comparison block for
normalized visible text and project LWC component counts. The current oracle lane
proves the app page, custom tab, and URL-addressable component targets on both
sides. The app-page and custom-tab proof deploys the fixture app, assigns the
fixture permission set, and opens the fixture Lightning app route. Record pages,
quick actions, and Visualforce Lightning Out need their org setup completed
before they are strict browser-oracle targets. The latest external capture lane
recorded 3 browser pass rows and 0 fail rows for the two-sided target set, plus
35 prepared targets across `lightning-shell` and `visualforce-lightning-out`. The
expanded base-component target has local browser proof through
`test/base-components-expanded.test.mjs`; Salesforce DOM comparison remains a
hosted follow-up. Community routes have local Go and Playwright runtime
coverage in this build; a refreshed compatibility capture should add the
external support row for the community target.

Maintainers can also use that lane for local-only browser capture of community
and expanded base-component fixture routes.

## Fixtures

At startup, Glade infers local org state from the project schema and loads Glade
storage fixtures from `data/*.json`. Use those fixtures for records that
record-page LWCs and LDS wires should read.

```text
macrodata-apex/
  glade.lwc.json
  data/file-rows.json
  force-app/main/default/lwc/
  force-app/main/default/flexipages/
  force-app/main/default/tabs/
  sfdx-project.json
```

For persistent local data loops, seed a database with `glade db seed` and use
commands that target that database.

## Diagnostics

Unsupported metadata, invalid context presets, unsupported navigation targets,
unsupported base components, unsupported local base-component attributes,
missing SLDS assets, and unsupported Salesforce services return named
`GLADELWC` diagnostics instead of calling Salesforce. Visibility rules use
`GLADELWC034`, overloaded `@AuraEnabled` controller methods use
`GLADELWC013`, unsupported local base-component attributes use `GLADELWC061`,
community context and navigation issues use `GLADELWC100` through
`GLADELWC103`, and Lightning Out host issues use `GLADELWC080` through
`GLADELWC082`.
Look in the Workbench Console debug panes for Apex, LDS, network calls,
navigation/events, and runtime issues. The workbench context panel, browser
console, and `/lightning/local/context.json` expose the same local state for
tools and deeper checks.

## Current Limits

Glade serves a local Lightning shell. It does not replace hosted Lightning
Experience, live auth, permissions, full UI API, broad LDS cross-adapter
coalescing, every `lightning-*` base component edge, exact SLDS fidelity, full
Experience Cloud menus and managed content, hosted validation behavior, or
every Lightning Out edge. The workspace API models console state for
development; exact hosted console behavior remains a Salesforce check. The
Flow shell is not Flow Builder, and local EMP API is deterministic in-page event
simulation, not live streaming. Record forms use local `getRecord` and
`updateRecord` support, not full hosted edit-flow behavior. Apex params must be
object-shaped; `undefined` wire params suppress invocation and `null` is passed
as an explicit value.

Visualforce appears in this workflow only when a Visualforce-backed tab
redirects to `/apex/<Page>` or when a page shares the Lightning Out runtime.
Keep a Salesforce gate for hosted behavior and final deployment checks.
