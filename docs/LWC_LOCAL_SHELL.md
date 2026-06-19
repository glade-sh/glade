# Local LWC Shell

The LWC local shell is a preview feature. Glade can serve Lightning Web
Components from an SFDX project without deploying to Salesforce, but it does
not replace hosted Lightning Experience. The shell reads the project on disk,
compiles LWC bundles, resolves Lightning page, custom tab, quick action, and
community context metadata, starts a Glade-owned Lightning runtime, and opens
local routes for component, record, app, home, tab, action, and Experience
Cloud contexts.

Run the toolchain install before opening LWC routes:

```bash
glade toolchain install
```

Then start the local workbench from the project root:

```bash
glade dev lwc --project . --open
```

The printed base URL opens the LWC home page at `/`. The same home page is
also available at `/lwc` for stable links. It groups formal tabs, app pages,
record pages, direct components, utilities, flows, and community routes. Use
`/lwc/builder` or the **Open builder** link when you want to compose a draft
app, home, record, tab, URL-addressable, quick-action, or community page
context. The builder lists and filters available LWCs, lets you drag or add
target-compatible components into canvas regions, switches page layouts, and
shows active context and local diagnostics.

Use an ephemeral port and ready file for scripts:

```bash
glade dev lwc --project . --addr 127.0.0.1:0 --ready-file /tmp/glade-lwc-ready.json
```

Use `--port` for the common localhost shortcut when you want a fixed port:

```bash
glade dev lwc --project . --port 8080 --open
```

The ready file includes `url`, `selectedUrl`, `selectedContext`, and the route
list. The startup banner lists the same routes and watches LWC, FlexiPage,
custom tab, Visualforce, Apex, and static resource changes.

The VS Code extension does not manage this server yet. Start, stop, and route
selection stay in the terminal while this preview feature is still being
ironed out.

## Context Presets

Create `glade.lwc.json` at the project root when a component needs a stable
Salesforce-like page context:

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

Open a preset:

```bash
glade dev lwc --project . --context accountRecord --open
```

Use a context file outside the default project-root `glade.lwc.json` when a
script or workspace needs a separate set of presets:

```bash
glade dev lwc --project . --context-file config/lwc-contexts.json --context accountRecord --open
```

Use direct flags when a one-off route is enough:

```bash
glade dev lwc --project . --target record-page --object Account --record 001000000000001AAA --page Account_Record_Page --open
```

Supported context targets are `component`, `urlAddressable`, `recordPage`,
`appPage`, `homePage`, `tab`, `recordAction`, `globalAction`, `utilityBar`,
`flowScreen`, `flowAction`, and `communityPage`. Community presets use
`community.site`, `basePath`, `siteId`, `networkId`, `guest`, `language`, and an
optional configured `pageReference`. Flow presets use `flow.apiName` and
optional `flow.inputVariables`. Direct flags include `--component`, `--object`,
`--record`, `--page`, `--tab`, `--action`, `--app`, `--flow`,
`--flow-input key=value`, `--form-factor`, and repeated `--state key=value`.
Explicit flags override preset fields for non-community routes.

## Preview Routes

Open these routes from the local server:

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
http://127.0.0.1:8080/lwc/preview/tab/Lwc_Probe
http://127.0.0.1:8080/lwc/preview/utility/Support_Utility
http://127.0.0.1:8080/lwc/preview/flow/Membership_Flow
http://127.0.0.1:8080/lwc/preview/action/Account/001000000000001AAA/Update_Status
http://127.0.0.1:8080/lwc/preview/action/global/Global_Status
http://127.0.0.1:8080/lwc/preview/community/Partner_Portal/Account
http://127.0.0.1:8080/lwc/preview/community/Partner_Portal/cmp/c/communityProbe
```

Direct component preview mounts one component with URL context. Record, app,
home, and tab routes resolve Salesforce metadata from the project:

- `*.js-meta.xml` controls exposure, targets, target configs, objects, form
  factors, and builder properties.
- FlexiPage metadata controls record pages, app pages, home pages, regions, and
  component properties.
- Custom tab metadata controls LWC tabs, FlexiPage tabs, and Visualforce tabs.
- URL-addressable routes mount LWCs exposed through `lightning__UrlAddressable`
  and pass `c__*` state through the local PageReference.
- Quick action routes mount LWC-backed action metadata and pass `recordId`,
  `objectApiName`, `actionName`, `actionType`, state, and PageReference
  attributes. Unsupported action metadata returns `GLADELWC070`; unsupported
  action types return `GLADELWC015`.
- Utility-bar routes resolve `UtilityBar` FlexiPage metadata, mount utility
  items, and pass a local workspace utility list.
- Flow-screen routes resolve `flowScreen` presets, mount
  `lightning__FlowScreen` components, and pass local Flow input variables and
  navigation events.
- Flow-action routes use the quick-action route shape and require
  `lightning__FlowAction` metadata.
- Community page routes resolve `communityPage` presets, mount
  `lightningCommunity__Page` components, apply a local
  `lightningCommunity__Theme_Layout` wrapper boundary when one exists, and pass
  community base path, IDs, guest mode, language, state, and configured
  `comm__*` PageReference data.
- Direct community component routes mount `lightningCommunity__Default`
  components with the site from the route and a `/s` base-path fallback.
- Generated workbench routes include direct components, app pages, record
  pages with a sample record ID, home pages, tabs, URL-addressable components,
  utility bars, Flow screens, LWC-backed actions, and configured community
  pages.

Visualforce-backed custom tabs redirect to the Visualforce page route when the
project defines one. That redirect is the main Visualforce mention in the LWC
workflow. Use the LWC shell for LWC contexts. Use the Visualforce route only
when the real host is a Visualforce page or a Visualforce-backed tab.

## Local Context JSON

The shell exposes local state for tools and browser tests:

```text
/lightning/local/context.json
```

The response includes the active route, PageReference, route context, mounted
components, route list, diagnostics when present, and service support such as
Apex, LDS, UI API shims, navigation, labels, resources, LMS, quick actions,
Experience Cloud context, base components, Visualforce host support, and toast.

`CurrentPageReference` uses the same local PageReference shapes:

| Route | PageReference shape |
| --- | --- |
| `/lwc/preview/component/<ns>/<component>` | `standard__component` |
| `/lwc/preview/cmp/<ns>/<component>?c__name=value` | `standard__component` with state |
| `/lwc/preview/record/<Object>/<recordId>?page=<FlexiPage>` | `standard__recordPage` |
| `/lwc/preview/app/<Page>` | `standard__app` |
| `/lwc/preview/home/<Page>` | `standard__namedPage` with `home` |
| `/lwc/preview/tab/<Tab>` | `standard__navItemPage` when the tab route is the active target |
| `/lwc/preview/utility/<UtilityBar>` | `standard__component` with local workspace utility context |
| `/lwc/preview/flow/<FlowApiName>` | `standard__component` with local Flow context |
| `/lwc/preview/action/<Object>/<recordId>/<ActionName>` | `standard__quickAction` |
| `/lwc/preview/action/global/<ActionName>` | `standard__quickAction` |
| `/lwc/preview/community/<site>/<page>` | configured `comm__*`, defaulting to `comm__namedPage` |
| `/lwc/preview/community/<site>/cmp/<ns>/<component>` | `standard__component` with community context |

Query parameters named `state.<key>` become PageReference state values:

```text
/lwc/preview/component/c/contextProbe?state.c__mode=review
```

Record routes pass `recordId` and `objectApiName` to page components. Direct
component contexts can pass the same values through `glade.lwc.json` or direct
flags, which makes LDS and UI API probes useful without a FlexiPage wrapper.
FlexiPage component properties are passed as component attributes.

## Apex, LDS, Labels, And Resources

The local LWC runtime supports the first-mile Salesforce module paths used by
common local development loops:

- Local static resources resolve root files and nested subpaths used by
  `@salesforce/resourceUrl/*` and `lightning/platformResourceLoader`.
- Datatable, record form, record edit/view form, input field, output field,
  and message surfaces use practical local renderers backed by LDS/UI API
  shims.
- `lightning/flow` and `lightning/flowSupport` provide a local Flow screen
  host and navigation events. This is not Salesforce Flow Builder.
- `lightning/empApi` provides a deterministic in-page event bus. It does not
  connect to Salesforce streaming services.
- `lightning/platformWorkspaceApi` models local console tab state for
  development. It is not the hosted console workspace engine.

- Apex wire and imperative imports through `@salesforce/apex/<Class>.<method>`
  in the LWC shell and Visualforce Lightning Out.
- Local Apex controller execution for project methods that the local VM can
  invoke. Request user context is passed through the local server, cacheable
  Apex wires use deterministic local cache keys, and `refreshApex` forces a
  fresh local invocation.
- `lightning/uiRecordApi` `getRecord`, `getRecords`, and
  `getRecordCreateDefaults` against local org state. `optionalFields` are
  accepted; unknown required fields return a wire error and unknown optional
  fields are skipped. Soft-deleted records read as not found.
  Create defaults include object info, record defaults, and a create-mode layout
  from project field sections when `.layout-meta.xml` is available, with a
  generated full layout from createable fields as the local fallback.
- `lightning/uiObjectInfoApi` `getObjectInfo`, `getObjectInfos`,
  `getPicklistValues`, and `getPicklistValuesByRecordType` against local
  schema metadata. `getObjectInfo` is browser-checked in the LWC shell and
  Visualforce Lightning Out. Compatibility exports remain available from
  `lightning/uiRecordApi`.
- `lightning/uiLayoutApi` `getLayout` for the same local Record Layout shape.
  `formFactor` is accepted, but distinct mobile/tablet layout variants remain a
  Salesforce check.
- `lightning/uiRecordApi` `createRecord`, `updateRecord`, `deleteRecord`,
  `getFieldValue`, `getFieldDisplayValue`, `generateRecordInputForCreate`,
  `generateRecordInputForUpdate`, and
  `createRecordInputFilteredByEditedFields` against local record shapes.
  Create, update, and delete are browser-checked in the LWC shell and
  Visualforce Lightning Out.
  Mutations use the local DML engine for supported objects, so ID sequences,
  required-field checks, audit fields, explicit nulls, and soft deletes follow
  the same local rules as Apex DML.
- `notifyRecordUpdateAvailable`, `getRecordNotifyChange`, and `refreshApex`
  re-emit matching local record wires through the browser LDS cache.
- `lightning/uiRelatedListApi` `getRelatedListRecords` for deterministic local
  child-relationship data. Compatibility export remains available from
  `lightning/uiRecordApi`; related-list metadata adapters remain a Salesforce
  check.
- Deprecated `lightning/uiListApi` `getListUi` reports `GLADELWC050`; use
  `getRelatedListRecords` or a local Apex controller.
- `@salesforce/schema/<Object>` and `@salesforce/schema/<Object>.<Field>`
  tokens.
- `@salesforce/label/<namespace>.<name>` from project custom labels and known
  platform fallbacks.
- `@salesforce/resourceUrl/<name>` and
  `@salesforce/contentAssetUrl/<name>` for local static resources and content
  assets.
- `@salesforce/client/formFactor` from the active route context, defaulting to
  `Large`.
- `@salesforce/customPermission/*` as a truthy local development default for
  permission-gated package LWCs. Permission assignment parity remains a hosted
  Salesforce check.
- `@salesforce/user/Id`, `@salesforce/user/isGuest`, and checked
  `@salesforce/i18n/*` values. `isGuest` reads the active community context
  and remains `false` outside guest community routes.
- `@salesforce/community/basePath`, `@salesforce/community/Id`, and
  `@salesforce/site/Id` from the active local community context. Missing IDs
  export empty strings and report `GLADELWC102`.
- `lightning/navigation` basics for `CurrentPageReference`,
  `NavigationMixin.Navigate`, and `NavigationMixin.GenerateUrl`, including
  local `comm__namedPage`, `comm__loginPage`, `comm__managedContentPage`,
  `comm__recordPage`, and `comm__recordRelationshipPage` URL generation.
- `lightning/messageService` in-page publish and subscribe,
  `lightning/platformResourceLoader` local script and style loading,
  `lightning/platformShowToastEvent`, `lightning/showToastEvent`, and
  `lightning/toast` browser toast paths.
- `lightning/platformWorkspaceApi` active-route and tab label/icon
  approximation for console apps, including local tab info, open/close/focus,
  refresh, highlight, and console wire values. Full console workspace behavior
  remains a Salesforce check and is marked with `GLADELWC072`.
- `lightning/alert`, `lightning/confirm`, `lightning/prompt`,
  `lightning/configProvider`, and `lightning/pageReferenceUtils` local shims
  for package flows, icon token lookup, localization helpers, and default
  field value encode/decode helpers.
- `lightning/actions`, `lightning/flowSupport`, `lightning/refresh`, and
  `lightning/empApi` practical local shims. They provide browser events,
  refresh handler dispatch, and in-page pub/sub contracts without live
  Salesforce action, flow runtime, or streaming service execution.

These services work in LWC shell routes. Shared runtime services also support
Lightning Out pages where the support table names that host.

## Base Components And SLDS

The shell resolves every public `lightning/*` name exposed by
`lightning-base-components@1.28.19-alpha`: 118 module names checked from the
npm package. Renderable `lightning-*` components use practical local
implementations. Service and helper modules use stable local shims. New shell
pages load SLDS 2 Cosmos from
`/lightning/runtime/slds/design-system-2/dist/css/bundled/slds2.cosmos.css` by
default. The local runtime also serves the SLDS 2 Lightning Blue bundle,
classic SLDS 1 styles, icon sprites, fonts, and referenced image assets from
the same `/lightning/runtime/slds/` tree. The legacy
`/lightning/runtime/slds/glade-slds.css` path remains as a compatibility import
to the default SLDS 2 bundle.

This is meant to make real project LWCs usable in the browser without network
access. It is not a claim of full hosted Lightning Experience chrome or exact
hosted base-component behavior.

The practical event model includes normal `click`, `change`, and `submit`
events, datatable `rowaction` events for local action columns, record-form
`success` and `error` events for local LDS-backed submits, and tab `active`
events when a rendered tab label is selected. `lightning-record-form`,
`lightning-record-view-form`, and `lightning-record-edit-form` read field
values through the local `getRecord` endpoint and submit edits through the
local `updateRecord` endpoint.

The broader base-component set includes modal body/header/footer,
alert/prompt/toast surfaces, display and formatted value helpers,
name/location/address inputs, picklist and grouped combobox, barcode scanner,
progress, tree and tree grid, map, carousel, vertical navigation variants,
stacked tabs, overlay/popover containers, and the package helper modules such
as `lightning/iconUtils`, `lightning/i18nService`, `lightning/routingService`,
`lightning/messageDispatcher`, and icon template modules. The practical
renderers cover common button/icon-button variants, `lightning-card`
title/actions/footer slots, `lightning-layout` alignment and sizing classes,
and `lightning-formatted-number` decimal, currency, percent, and percent-fixed
formatting. These local modules are meant to keep real project LWCs rendering
and testable. Exact hosted keyboard behavior, overlay stacking, accessibility
edge cases, and full Salesforce chrome still belong to a Salesforce browser
check.

A small allowlist uses compiled or source-backed base component
implementations instead of generated shims: `lightning-badge`,
`lightning-breadcrumb`, `lightning-breadcrumbs`, `lightning-button-group`,
`lightning-datatable`, `lightning-input-field`, `lightning-messages`,
`lightning-output-field`, `lightning-record-edit-form`,
`lightning-record-form`, `lightning-record-picker`,
`lightning-record-view-form`, `lightning-menu-subheader`, and
`lightning-vertical-navigation-item`. Additional source-backed components
should be added one slice at a time after their compiled imports are checked
for local-only browser-safe dependencies.

The checked `lwc-shell` fixture exposes an expanded base-component direct
component context through `c:baseComponentHost` and `phase3BaseComponents`.
The local browser capture for that fixture passes with no browser console
errors or page errors. Live Salesforce parity evidence still belongs to a
hosted capture report.

Unsupported base component imports, unsupported local base-component
attributes, and missing SLDS assets report named `GLADELWC` diagnostics. Keep a
Salesforce browser check for exact styling, keyboard behavior, accessibility
edge cases, and hosted base-component details.

## Local Data

`glade dev lwc` starts with local org state inferred from the project schema and
loads fixture files from `data/*.json` when they use the Glade storage fixture
format.

Use fixtures for records that record-page LWCs and LDS wires should see:

```text
my-project/
  glade.lwc.json
  data/accounts.json
  force-app/main/default/lwc/
  force-app/main/default/flexipages/
  force-app/main/default/tabs/
  sfdx-project.json
```

For persistent local data loops, seed a database separately with `glade db seed`
and use the local API or test commands that target that database.

## Diagnostics

Known unsupported or approximate behavior returns stable `GLADELWC` diagnostics
instead of falling through to Salesforce. Examples include missing components,
unsupported targets, invalid context presets, unsupported navigation
destinations, approximated FlexiPage visibility rules (`GLADELWC034`),
unsupported base components, unsupported local base-component attributes
(`GLADELWC061`), missing community context (`GLADELWC100`), unsupported
Experience Builder features (`GLADELWC101`), missing community site or network
IDs (`GLADELWC102`), unsupported community PageReferences (`GLADELWC103`),
missing SLDS assets, and Lightning Out host issues (`GLADELWC080` through
`GLADELWC082`). The workbench context panel and
`/lightning/local/context.json` are the first places to look.

## Browser Oracle

Use the sibling `glade-tools` compatibility command when local behavior needs a
live Salesforce browser check beside the local shell:

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

`--local-browser-capture` captures DOM, console errors, and page errors from
the local Glade shell. Pass `--glade-bin` to start `glade dev lwc` for the run,
or pass `--local-base-url` for an existing local server. `--browser-capture`
uses `sf org open --url-only` in memory and passes the frontdoor URL straight to
Playwright. The written report keeps stable local and Salesforce paths. It does
not write one-time login URLs. When both sides are present, each case includes a
`comparison` block with the scoped component selector, normalized visible text,
project LWC component names, project LWC component counts, and diffs.

The current browser oracle lane proves the app page, custom tab, and
URL-addressable component targets on both sides: local shell DOM and Salesforce
DOM, selector-scoped comparison, zero browser console errors, and zero page
errors. The app-page and custom-tab oracle deploys the fixture app, assigns the
fixture permission set, and opens the fixture Lightning app route. The latest
external capture lane recorded 3 browser pass rows and 0 fail rows for the
two-sided target set. A broader deploy/prepared capture recorded 35 prepared
targets across `lightning-shell` and `visualforce-lightning-out`.
Record pages need a real org record id and page activation. Quick actions need
modal routing proof. Visualforce Lightning Out needs the Visualforce fixture
pages deployed to the same org. Expanded base-component and community captures
use local-only browser evidence because those fixture routes have no direct
stable Salesforce URL:

```bash
cd ../glade-tools
go run ./cmd/glade-plugin-compat lwc capture \
  --target-org <target-org> \
  --project ../glade/testdata/local-tests/lwc-shell \
  --targets community-page,package-phase1-base-components,phase3-base-components \
  --skip-deploy \
  --local-browser-capture \
  --glade-bin /tmp/glade-lwc-shell-bin \
  --out /tmp/glade-lwc-local-only-browser-check.json
```

The project corpus scanner lives in the sibling compat plugin as
`glade-tools compat lwc corpus`; it writes repo, target, import,
base-component tag, and property-type counts for selected Salesforce projects.

## Current Limits

This is local development support, not a hosted Salesforce replacement.

| Area | Current limit |
| --- | --- |
| Hosted Lightning Experience | Glade serves a local shell, not Salesforce chrome, live auth, permissions, workspace API, or exact console behavior. |
| Metadata | The shell resolves LWC bundle metadata, FlexiPages, custom applications, custom tabs, quick actions, and configured community contexts needed for preview routes. It does not implement every builder rule or full Experience Builder runtime. |
| Base components | All public `lightning/*` names exposed by `lightning-base-components@1.28.19-alpha` resolve locally. Renderable `lightning-*` modules have practical local implementations, including datatable row actions, LDS-backed record form reads and submits, tab active events, dual listbox/select/slider changes, rich text input changes, record picker changes, file upload events, overlay/toast/modal surfaces, and display/input/container helpers. Packaged SLDS 2 and classic SLDS assets are served locally, but exact hosted base-component behavior remains outside the local contract. Unsupported local attributes report `GLADELWC061`. |
| Apex params and errors | Params must be object-shaped. `undefined` wire params suppress invocation; `null` is passed as an explicit value. Errors use a Salesforce-shaped `body.message`, `body.exceptionType`, `body.stackTrace`, and `status` envelope. |
| LDS/UI API | Selected LDS/UI API shims use local schema and local records, including batch records, optional fields, batch object info, create-default field layouts, REST layout field sections, picklists, related-list rows, mutation refresh, and matching record-wire re-emits. Full UI API, profile layout assignment, non-field layout widgets, permissions, record edit flows, broad cross-adapter coalescing, and hosted validation parity are not complete. |
| Apex controllers | Supported local Apex executes in the Glade VM. Unsupported Apex surfaces return diagnostics instead of calling Salesforce. |
| Navigation | `CurrentPageReference`, `Navigate`, and `GenerateUrl` cover supported local page targets, including quick action, URL-addressable component, and configured community routes. Full router history, full console navigation, named app behavior, full Experience Cloud routing, and hosted URL generation remain limited. Console workspace APIs are approximated and marked with `GLADELWC072`. |
| Experience Cloud | Community routes provide local site context, base path, IDs, guest flag, language, supported `comm__*` PageReferences, and a theme-layout boundary. Menus, managed content delivery, personalization, builder data sources, auth flows, and exact hosted Experience Cloud chrome remain Salesforce checks. |
| Visualforce Lightning Out | Shared runtime services can mount LWCs through Lightning Out pages. Exact hosted lifecycle timing and every Lightning Out edge remain outside the local contract. |

Use Salesforce for live auth, hosted permissions, org-only services, exact
Lightning Experience behavior, and final deployment gates.
