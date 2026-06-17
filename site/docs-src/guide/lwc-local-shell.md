# Local LWC Shell

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Local Lightning</p>
  <p>Serve Lightning Web Components from an SFDX project with local page context, local data, diagnostics, and a Glade-owned Lightning workbench.</p>
  <ul>
    <li>Start <code>glade dev lwc --open</code>.</li>
    <li>Select contexts from <code>glade.lwc.json</code>.</li>
    <li>Open component, record, app, home, tab, action, and community routes.</li>
    <li>Keep Salesforce for hosted Lightning Experience behavior.</li>
  </ul>
</div>

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

Start the workbench:

```bash
glade dev lwc --project . --open
```

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
    "communityAccount": {
      "target": "communityPage",
      "component": "c:communityProbe",
      "page": "Account",
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
        }
      }
    }
  }
}
```

Supported targets are `component`, `urlAddressable`, `recordPage`, `appPage`,
`homePage`, `tab`, `recordAction`, `globalAction`, and `communityPage`.
Community presets carry site, base path, site ID, network ID, guest mode,
language, and optional `comm__*` PageReference data. Direct flags include
`--component`, `--object`, `--record`, `--page`, `--tab`, `--action`, `--app`,
`--form-factor`, and repeated `--state key=value` for non-community routes. Use
`--context-file` when the presets live somewhere other than the project-root
`glade.lwc.json`:

```bash
glade dev lwc --project . --context-file config/lwc-contexts.json --context accountRecord --open
```

Direct flags override preset fields.

## Routes

The `/lwc` workbench lists discovered routes, mounted components, active
context, and diagnostics. Raw preview routes remain stable for scripts and
bookmarks:

```text
/lwc
/lwc/preview/component/<namespace>/<component>
/lwc/preview/cmp/<namespace>/<component>?c__name=value
/lwc/preview/record/<Object>/<recordId>?page=<FlexiPage>
/lwc/preview/app/<Page>
/lwc/preview/home/<Page>
/lwc/preview/tab/<Tab>
/lwc/preview/action/<Object>/<recordId>/<ActionName>
/lwc/preview/action/global/<ActionName>
/lwc/preview/community/<site>/<page>
/lwc/preview/community/<site>/cmp/<namespace>/<component>
```

Examples:

```text
http://127.0.0.1:8080/lwc
http://127.0.0.1:8080/lwc/preview/component/c/contextProbe
http://127.0.0.1:8080/lwc/preview/cmp/c/actionProbe?c__name=value
http://127.0.0.1:8080/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page
http://127.0.0.1:8080/lwc/preview/app/Sales_Dashboard
http://127.0.0.1:8080/lwc/preview/home/Custom_Home
http://127.0.0.1:8080/lwc/preview/tab/Lwc_Probe
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
Community routes resolve `communityPage` presets, mount
`lightningCommunity__Page` components, mount `lightningCommunity__Default`
direct components, expose community base path, site ID, network ID, guest mode,
language, and use a local `lightningCommunity__Theme_Layout` wrapper boundary
when one is present.

## Local context JSON

Tools can read the current local shell state:

```text
/lightning/local/context.json
```

The JSON includes active route, PageReference, route context, mounted
components, discovered apps, route list, named context presets, default and
selected context names, diagnostics when present, and service support for Apex,
LDS, UI API shims, navigation, labels, resources, LMS, quick actions, base
components, Experience Cloud context, Visualforce host support, and toast.

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
  Create defaults include a create-mode layout from project field sections when
  `.layout-meta.xml` is available, with a generated full layout from createable
  fields as the local fallback.
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
- `notifyRecordUpdateAvailable`, `getRecordNotifyChange`, and `refreshApex`
  refresh matching local record wires through the browser LDS cache.
- Deprecated `lightning/uiListApi` `getListUi` reports `GLADELWC050`; use
  `getRelatedListRecords` or local Apex.
- `@salesforce/schema` object and field tokens.
- `@salesforce/label`, `@salesforce/resourceUrl`, and
  `@salesforce/contentAssetUrl`.
- `@salesforce/user`, checked `@salesforce/i18n` values, and
  `lightning/navigation` basics. `@salesforce/user/isGuest` reads active
  community guest context and remains false on non-community routes.
- `@salesforce/community/basePath`, `@salesforce/community/Id`, and
  `@salesforce/site/Id` for active community routes. Missing IDs export empty
  strings and report `GLADELWC102`.
- `comm__namedPage`, `comm__loginPage`, `comm__managedContentPage`,
  `comm__recordPage`, and `comm__recordRelationshipPage` local URL generation.
- `lightning/messageService`, `lightning/platformResourceLoader`, and
  `lightning/platformShowToastEvent` shims in the shell and Visualforce
  Lightning Out where the support table names that host.
- `lightning/actions`, `lightning/flowSupport`, `lightning/refresh`, and
  `lightning/empApi` practical local shims for quick-action events, flow-screen
  events, in-page refresh handlers, and local pub/sub.
- Practical local implementations for common `lightning-*` base components,
  including datatable row actions, LDS-backed record form reads and submits,
  record-form `success` and `error` events, and tab active events.
- Expanded checked `lightning-*` modules used by the checked LWC fixture,
  mounted through `c:baseComponentHost` and the `phase3BaseComponents`
  context. The set covers email, dual listbox, select, slider, rich text input,
  menu divider, progress, breadcrumbs, tree grid, map, carousel, record picker,
  file upload, and additional display/input/container components.
- SLDS-shaped local styling for shell and base-component previews.

`CurrentPageReference` reports the local route context. `NavigationMixin`
supports local URL generation and navigation for supported page targets.

Checked capture rows are generated in `docs/generated/LWC_SHELL_SUPPORT.md`.
They record the local support matrix for the LWC shell. The sibling
`glade-tools` command can also run a two-sided browser oracle: local Glade shell
DOM plus authenticated Salesforce DOM for deployed Lightning paths.

```bash
cd ../glade-tools
go run ./cmd/glade-plugin-compat lwc capture \
  --target-org <target-org> \
  --project ../glade/testdata/local-tests/lwc-shell \
  --targets app-page,custom-tab,url-addressable-component \
  --local-browser-capture \
  --glade-bin /tmp/glade-lwc-shell-bin \
  --browser-capture \
  --out /tmp/glade-lwc-two-sided-browser-check.json
```

That capture stores stable local and Salesforce paths and browser evidence
only. It does not write frontdoor login URLs or one-time tokens. When both
sides are captured, each case includes a selector-scoped comparison block for
normalized visible text and project LWC component counts. The current oracle lane
proves the app page, custom tab, and URL-addressable component targets on both
sides. The app-page and custom-tab proof deploys the `Lwc_Shell` app, assigns
`Lwc_Shell_Access`, and opens `/lightning/app/c__Lwc_Shell/n/Lwc_Probe`. Record
pages, quick actions, and Visualforce Lightning Out need their org setup
completed before they are strict browser-oracle targets. The expanded
base-component target has local browser proof through
`test/base-components-expanded.test.mjs`; Salesforce DOM comparison remains a
hosted follow-up. Community routes have local Go and Playwright runtime
coverage in this build; a refreshed compatibility capture should add the
external support row for the community target.

Use local-only browser capture for community and expanded base-component
fixture routes:

```bash
cd ../glade-tools
go run ./cmd/glade-plugin-compat lwc capture \
  --target-org <target-org> \
  --project ../glade/testdata/local-tests/lwc-shell \
  --targets community-page,phase3-base-components \
  --skip-deploy \
  --local-browser-capture \
  --glade-bin /tmp/glade-lwc-shell-bin \
  --out /tmp/glade-lwc-local-only-browser-check.json
```

## Fixtures

At startup, Glade infers local org state from the project schema and loads Glade
storage fixtures from `data/*.json`. Use those fixtures for records that
record-page LWCs and LDS wires should read.

```text
my-project/
  glade.lwc.json
  data/accounts.json
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
Look in the workbench context panel, browser console, and
`/lightning/local/context.json`.

## Current Limits

Glade serves a local Lightning shell. It does not replace hosted Lightning
Experience, live auth, permissions, console APIs, full UI API, broad LDS
cross-adapter coalescing, every `lightning-*` base component edge, exact SLDS
fidelity, full Experience Cloud menus and managed content, hosted validation
behavior, or every Lightning Out edge. Record forms use local `getRecord` and
`updateRecord` support, not full hosted edit-flow behavior. Apex params must be
object-shaped; `undefined` wire params suppress invocation and `null` is passed
as an explicit value.

Visualforce appears in this workflow only when a Visualforce-backed tab
redirects to `/apex/<Page>` or when a page shares the Lightning Out runtime.
Keep a Salesforce gate for hosted behavior and final deployment checks.
