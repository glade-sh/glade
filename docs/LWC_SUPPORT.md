# LWC Local Support

> Repo maintainer reference. User-facing setup lives in
> <https://glade.sh/guide/lwc-local-shell>. Maintainer oracle capture belongs in
> <https://glade.sh/maintainer/glade-tools>.

This page lists the user-facing LWC preview feature surface for the local shell.
The feature is centered on `glade dev lwc`, the home routes `/` and `/lwc`, the
builder route `/lwc/builder`, and `/lwc/preview/*` routes. Visualforce appears
here only where a Visualforce-backed tab or shared Lightning Out runtime
affects an LWC.

The local shell UI is the Workbench Console. It is designed for developer
preview and debugging of local LWCs in component, record page, builder, tab, and
app contexts. It exposes local diagnostics for Apex, LDS, navigation,
PageReference, and runtime issues.

Generated capture rows live in
[generated/LWC_SHELL_SUPPORT.md](generated/LWC_SHELL_SUPPORT.md). They are
prepared from the `glade-tools` LWC capture command. Two-sided browser capture
is available through the sibling `glade-tools` command when you provide a
Salesforce target org.

## Hosts

| Host | Status | Support key | Notes |
| --- | --- | --- | --- |
| LWC workbench | Preview feature | `lwc.host.lightning-shell` | The printed base URL at `/` and `/lwc` opens a local home page with tabs and discovered routes. `/lwc/builder` opens the draft page composer with a filterable available-LWC catalog, drag-and-drop placement, layout switching, seeded record context, active context, and diagnostics. |
| Direct component shell | Preview feature | `lwc.host.lightning-shell` | `/lwc/preview/component/<namespace>/<component>` mounts one exposed component for local development. |
| Record page shell | Preview feature | `lwc.host.lightning-shell` | `/lwc/preview/record/<Object>/<recordId>?page=<FlexiPage>` resolves FlexiPage regions and record context. |
| App page shell | Preview feature | `lwc.host.lightning-shell` | `/lwc/preview/app/<Page>` resolves app-page FlexiPage metadata. |
| Home page shell | Preview feature | `lwc.host.lightning-shell` | `/lwc/preview/home/<Page>` resolves home-page FlexiPage metadata. |
| Custom tab shell | Preview feature with limits | `lwc.host.lightning-shell` | LWC tabs and FlexiPage tabs render locally. Visualforce tabs redirect to `/apex/<Page>`. Web and object tabs are reported as unsupported LWC-shell targets. |
| URL-addressable component shell | Preview feature | `lwc.host.lightning-shell` | `/lwc/preview/cmp/<namespace>/<component>` mounts `lightning__UrlAddressable` LWCs and passes `c__*` state. |
| Quick action shell | Preview feature with limits | `lwc.host.lightning-shell` | `/lwc/preview/action/...` mounts LWC-backed record and global actions with local action context. Full workspace APIs are approximated. |
| Utility bar shell | Preview feature with limits | `lwc.host.lightning-shell` | `/lwc/preview/utility/<UtilityBar>` resolves `UtilityBar` FlexiPage metadata and passes a local workspace utility list. |
| Flow screen shell | Preview feature with limits | `lwc.host.lightning-shell` | `/lwc/preview/flow/<FlowApiName>` resolves `flowScreen` presets, mounts `lightning__FlowScreen` components, and passes local Flow context. |
| Flow action shell | Preview feature with limits | `lwc.host.lightning-shell` | Flow actions use `/lwc/preview/action/...` and require `lightning__FlowAction` metadata. Hosted Flow runtime behavior remains a Salesforce check. |
| Experience Cloud page shell | Preview feature with limits | `lwc.host.lightning-shell` | `/lwc/preview/community/<site>/<page>` resolves `communityPage` presets, mounts `lightningCommunity__Page` components, and applies a local theme-layout boundary when a `lightningCommunity__Theme_Layout` component exists. |
| Experience Cloud component shell | Preview feature with limits | `lwc.host.lightning-shell` | `/lwc/preview/community/<site>/cmp/<namespace>/<component>` mounts `lightningCommunity__Default` components with community context and a `/s` base-path fallback. |
| Visualforce Lightning Out | Preview feature with limits | `lwc.host.visualforce-lightning-out` | `/apex/<PageName>` can host LWCs through `$Lightning.use()` and `$Lightning.createComponent()` using the shared local runtime. |

## Starting The Shell

```bash
glade toolchain install
glade dev lwc --project . --open
glade dev lwc --project . --context accountRecord --open
glade dev lwc --project . --context-file config/lwc-contexts.json --context accountRecord --open
glade dev lwc --project . --target record-page --object Account --record 001000000000001AAA --page Account_Record_Page --open
glade dev lwc --project . --port 8080 --open
```

`glade.lwc.json` presets support `component`, `urlAddressable`, `recordPage`,
`appPage`, `homePage`, `tab`, `recordAction`, `globalAction`, `utilityBar`,
`flowScreen`, `flowAction`, and `communityPage` targets.
Community routes are preset-backed; select them with `--context` or
`--context-file`. Pass `--context-file` to use a different preset file. The
ready file records `selectedUrl` and `selectedContext` when a preset or direct
target selects the first browser route.

`/lightning/local/context.json` returns the active route, PageReference, route
context, mounted components, discovered apps, named context presets,
diagnostics, route list, and service support.

## Runtime Services

| Service | Status | Local coverage |
| --- | --- | --- |
| LWC module compilation | Local support | LWC shell and shared Lightning Out runtime. |
| Import maps | Local support | LWC shell and shared Lightning Out runtime. |
| Context presets in `glade.lwc.json` | Local support | LWC shell startup, ready file, and local context JSON. |
| Local context JSON | Local support | LWC shell at `/lightning/local/context.json`. |
| Apex wire and imperative Apex imports | Local support with VM limits | LWC shell and Visualforce Lightning Out. Request user context is passed to the VM. Cacheable wire calls use deterministic local cache keys and `refreshApex` forces a fresh invocation. Raw imperative calls can also use `/lightning/apex/<Class>/<method>`. Overloaded `@AuraEnabled` controller methods return `GLADELWC013` instead of picking an arbitrary overload. |
| `lightning/uiRecordApi` `getRecord` and `getRecords` | Local support with data limits | LWC shell and Visualforce Lightning Out. Batch results preserve request order and return per-record status rows. `optionalFields` are accepted; missing required fields return a wire error while missing optional fields are skipped. Soft-deleted records read as not found. |
| `lightning/uiObjectInfoApi` `getObjectInfo` and `getObjectInfos` | Local support with schema limits | LWC shell and Visualforce Lightning Out. Batch object info results preserve request order and return per-object status rows. Compatibility exports remain available from `lightning/uiRecordApi`. |
| `lightning/uiRecordApi` `getRecordCreateDefaults` | Local support with schema/layout limits | LWC shell. Returns a local default record, object info, and create-mode layout. Project `.layout-meta.xml` field sections are used when present; otherwise Glade generates a local full layout from createable fields. Profile/app layout assignment and non-field layout widgets remain Salesforce-only. |
| `lightning/uiLayoutApi` `getLayout` | Local support with layout limits | LWC shell. Returns the same local Record Layout shape used by create defaults: parsed project field sections when present, otherwise a generated full layout from createable fields. Form factor is accepted but does not choose a distinct local variant yet. |
| REST SObject layout metadata | Local support with layout limits | `/sobjects/<Object>/describe/layouts` and `/sobjects/<Object>/namedLayouts/<Name>` return parsed project `.layout-meta.xml` field sections when present. Profile/app layout assignment and non-field layout widgets remain Salesforce-only. |
| `lightning/uiRecordApi` create, update, and delete helpers | Local support with DML limits | LWC shell and Visualforce Lightning Out. Mutations use the local DML engine for supported objects, including ID sequences, required-field checks, audit fields, explicit nulls, and soft deletes. |
| `lightning/uiObjectInfoApi` picklist adapters | Local support with schema limits | LWC shell. `getPicklistValues` and `getPicklistValuesByRecordType` read local field picklist metadata. Compatibility exports remain available from `lightning/uiRecordApi`. |
| `lightning/uiRelatedListApi` `getRelatedListRecords` | Local support with relationship limits | LWC shell. Reads deterministic child rows from local relationship metadata and record state. Related-list metadata adapters remain a Salesforce check. Compatibility export remains available from `lightning/uiRecordApi`. |
| `lightning/uiAppsApi` | Local support with shell metadata limits | LWC shell. `getNavItems`, `getAppMenuItems`, and `getAppMenuItem` read the active local app, route, tab, and component catalog metadata. Hosted app launcher assignments remain a Salesforce check. |
| `lightning/uiListsApi` | Local support with list-view limits | LWC shell. `getListInfosByObjectName`, `getListInfoByName`, `getListRecordsByName`, and `getListPreferences` return deterministic local list shapes. List mutations return a Salesforce-shaped local unsupported error until local list-view metadata writes exist. |
| `lightning/graphql` and `lightning/uiGraphQLApi` | Local support with read envelope limits | LWC shell. `gql`, `graphql`, and `query` resolve locally and return the UI API GraphQL response envelope. Deep GraphQL semantics such as fragments, aliases, pagination, and nullability remain oracle-gated. |
| `lightning/uiRecordApi` field and record-input helper functions | Local support for record shapes | LWC shell. Covers `getFieldValue`, `getFieldDisplayValue`, `generateRecordInputForCreate`, `generateRecordInputForUpdate`, and `createRecordInputFilteredByEditedFields`. |
| `@salesforce/schema` object and field tokens | Local support | LWC shell and shared runtime imports. |
| `@salesforce/label` | Local support with metadata limits | LWC shell and shared runtime imports. |
| `@salesforce/resourceUrl` | Local support with metadata limits | LWC shell and shared runtime imports. |
| `@salesforce/contentAssetUrl` | Local support with metadata limits | LWC shell and shared runtime imports. |
| `@salesforce/client/formFactor` | Local support from shell context | Exports the active form factor, defaulting to `Large` when no route context sets one. |
| `@salesforce/customPermission/*` and `@salesforce/userPermission/*` | Local development permissions | Exports deterministic permission values from shell context when provided, with local defaults for package LWCs that branch on permissions. Permission assignment parity remains a hosted Salesforce check. |
| `@salesforce/user` | Local support for `Id` and `isGuest` | LWC shell and shared runtime imports. `isGuest` reads the active community context and stays false outside guest community routes. |
| `@salesforce/i18n` | Local support for checked values | LWC shell and shared runtime imports. |
| `@salesforce/community` and `@salesforce/site` | Local support with context limits | LWC shell community routes. Supports `@salesforce/community/basePath`, `@salesforce/community/Id`, `@salesforce/site/Id`, and `@salesforce/site/activeLanguages`; missing IDs export empty strings and report `GLADELWC102`. |
| `@salesforce/apexContinuation` | Local simulated support | LWC shell and shared runtime imports. Continuation helpers return Promise-shaped local results with explicit simulated diagnostics; hosted servlet scheduling remains Salesforce-only. |
| Experience Cloud context | Local support with builder limits | LWC shell community routes. Supports site, base path, site ID, network ID, guest mode, language, `lightningCommunity__Page`, `lightningCommunity__Default`, and `lightningCommunity__Theme_Layout` wrapper boundaries from local metadata and `glade.lwc.json`. |
| `lightning/navigation` | Local support with route limits | LWC shell and Visualforce Lightning Out. Includes standard local route generation plus `standard__flow`, `standard__externalRecordPage`, `standard__externalRecordRelationshipPage`, `standard__knowledgeArticlePage`, `comm__namedPage`, `comm__loginPage`, `comm__managedContentPage`, `comm__recordPage`, and `comm__recordRelationshipPage`; unsupported PageReferences report named diagnostics such as `GLADELWC042` or `GLADELWC103`. |
| `lightning/messageService` | Local support for in-page publish and subscribe | LWC shell and Visualforce Lightning Out. Message channel metadata imports resolve to local channel tokens. |
| `lightning/flow` and `lightning/flowSupport` | Local Flow screen support with limits | The shell can host local Flow screen components and dispatch Flow navigation/attribute events. It is not Flow Builder and does not execute hosted Flow interviews. |
| `lightning/platformResourceLoader` | Local support for local scripts and styles | LWC shell and Visualforce Lightning Out. |
| `lightning/platformShowToastEvent` | Local support as a browser event shim | LWC shell and Visualforce Lightning Out. |
| `lightning/showToastEvent` | Local support as a browser event shim | Alias for the package-exposed toast event contract. Exports `SHOW_TOAST_EVENT_NAME`, `ShowToastEvent`, and the local toast recorder hook. |
| `lightning/toast` | Local support as a browser event shim | `LightningToast.show()` dispatches the local toast event so shell toast UI and tests observe the same path. |
| `lightning/platformWorkspaceApi` | Local console tab model | Returns the active local route, tab info, and console wire values; supports local no-op tab open, close, focus, refresh, highlight, label, and icon helpers. Marked with `GLADELWC072`; full console workspace behavior stays hosted-only. |
| `lightning/platformUtilityBarApi` | Local utility-bar model | Returns deterministic utility item state for local utility routes, with open, close, minimize, focus, label, icon, highlighted state, and callback helpers. Hosted utility-bar chrome remains a Salesforce check. |
| `lightning/actions` | Local event shim | Exports local `CloseActionScreenEvent` for quick-action flows. |
| `lightning/alert` | Local overlay shim | `LightningAlert.open()` dispatches a browser event and resolves locally for package flows. |
| `lightning/confirm` | Local confirm shim | `LightningConfirm.open()` dispatches a browser event and resolves `true` for local flows. |
| `lightning/prompt` | Local overlay shim | `LightningPrompt.open()` dispatches a browser event and resolves the provided default value for local flows. |
| `lightning/configProvider` | Local icon and localization token shim | Exports path-prefix, icon sprite token, one-config, and localization helpers used by package base-component utilities. |
| `lightning/flowSupport` | Local event shim | Exports Flow screen attribute and navigation events used by flow-screen LWCs. |
| `lightning/pageReferenceUtils` | Local helper shim | Exports default field value encode/decode helpers for navigation flows. |
| `lightning/refresh` | Local in-page refresh registry | Exports `RefreshEvent`, handler/container registration, unregister helpers, and a local dispatch hook for shell tests. |
| `lightning/empApi` | Local in-page event bus | Exports subscribe, unsubscribe, onError, debug flag, and enablement helpers without connecting to Salesforce streaming services. Tests can publish locally through the Glade test hook. |
| `lightning-base-components` package expose list | Local module support | All 118 public `lightning/*` names exposed by `lightning-base-components@1.28.19-alpha` resolve through `/lightning/shims/lightning/<name>.js` in the local server. Renderable base components use practical local implementations; service and helper modules use stable local shims. |
| Common and expanded `lightning-*` base components | Practical local rendering | LWC shell and Visualforce Lightning Out where modules are served by the shared runtime. Coverage includes common inputs, button and icon-button variants, card title/actions/footer slots, layout alignment/sizing classes, formatted number styles, tabs, LDS-backed record forms, input/output fields, datatable row actions, tab active events, messages, icons, spinner, modal/body/header/footer, alert/prompt/toast surfaces, display/input/container components, barcode scanner, picklist, grouped combobox, lookup/address/name/location helpers, progress, tree/tree grid, map, carousel, vertical navigation variants, stacked tabs, and local overlay/popover containers. The source-backed allowlist includes `lightning-badge`, `lightning-breadcrumb`, `lightning-breadcrumbs`, `lightning-button-group`, `lightning-datatable`, `lightning-input-field`, `lightning-messages`, `lightning-output-field`, `lightning-record-edit-form`, `lightning-record-form`, `lightning-record-picker`, `lightning-record-view-form`, `lightning-menu-subheader`, and `lightning-vertical-navigation-item`. |
| Packaged SLDS styling | Supported local | LWC shell and shared runtime pages load packaged SLDS 2 Cosmos by default. The runtime also serves SLDS 2 Lightning Blue, classic SLDS 1 CSS, local icon sprites, fonts, and referenced images from `/lightning/runtime/slds/`. |
| Static resource subpaths | Local support with metadata limits | Root files, directories, and nested subpaths can be served from local static resources and loaded through resource URL imports or the platform resource loader. |

## Unsupported Or Limited

| Area | Local behavior |
| --- | --- |
| Web custom tabs | Unsupported in the LWC shell. |
| Object custom tabs | Unsupported in the LWC shell. |
| Unsupported quick action metadata | Reports `GLADELWC070`; unsupported action types report `GLADELWC015`. |
| Invalid URL-addressable state | Reports `GLADELWC071`. |
| Console workspace API | Approximated locally and marked with `GLADELWC072`. |
| Local Flow shell | Hosts Flow screen LWCs and events. It is not Flow Builder and does not execute hosted Flow runtime behavior. |
| Local EMP API | Deterministic in-page event simulation. It does not connect to Salesforce streaming services. |
| Missing community context | Reports `GLADELWC100`. |
| Unsupported local Experience Builder feature | Reports `GLADELWC101`. |
| Missing community site or network ID | Reports `GLADELWC102`; related shims export empty strings. |
| Unsupported community PageReference | Reports `GLADELWC103`. |
| Unsupported local base-component attributes | Reports `GLADELWC061`. |
| Missing LWC, FlexiPage, app, or tab metadata | Returns a named `GLADELWC` diagnostic. |
| Invalid or missing context preset fields | Returns a named `GLADELWC` diagnostic. |
| Full Lightning Experience | Not modeled. Use Salesforce for hosted chrome, app state, console APIs, live auth, permissions, and final gates. |
| Full Experience Cloud runtime | Not modeled. Menus, managed content delivery, personalization, builder data sources, auth flows, and exact hosted chrome stay Salesforce checks. |
| `lightning/uiListApi` `getListUi` | Unsupported locally. Reports `GLADELWC050`; use `getRelatedListRecords` or local SOQL-backed Apex. |
| Full UI API | The local shell has selected LDS/UI API shims and local field layout sections. It is not broad UI API parity. |
| Full base component parity | Use the supported modules in this build. Keep a Salesforce browser check for exact hosted base-component behavior. |
| Exact Visualforce Lightning Out parity | The local host mounts LWCs and shares runtime services. Hosted lifecycle timing and every Lightning Out edge are not promised. |

## Diagnostics

Unsupported or approximate behavior reports stable `GLADELWC` diagnostics. The
workbench context panel, browser console, and `/lightning/local/context.json`
are the user-facing places to inspect them. Notable local-shell diagnostics
include `GLADELWC034` for approximated FlexiPage visibility rules,
`GLADELWC013` for overloaded `@AuraEnabled` controller methods,
`GLADELWC061` for unsupported local base-component attributes,
`GLADELWC100` through `GLADELWC103` for community context and navigation
limits, and `GLADELWC080` through `GLADELWC082` for Lightning Out app,
dependency, and host-service issues.

## Browser Oracle

Use `glade-tools` from the sibling tools repository when you need an
authenticated Salesforce browser check and a matching local shell browser check:

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

Use a local-only browser capture for fixture routes without a stable Salesforce
URL:

```bash
go run ./cmd/glade-plugin-compat lwc capture \
  --target-org <target-org> \
  --project ../glade/testdata/local-tests/lwc-shell \
  --targets community-page,package-phase1-base-components,phase3-base-components \
  --skip-deploy \
  --local-browser-capture \
  --glade-bin /tmp/glade-lwc-shell-bin \
  --out /tmp/glade-lwc-local-only-browser-check.json
```

`--local-browser-capture` starts or uses a local `glade dev lwc` shell and
stores `local-browser-dom` evidence. Use `--glade-bin` to let the tool start
the shell, or `--local-base-url` to point at a shell that is already running.
`--browser-capture` uses `sf org open --url-only` in memory and stores
`salesforce-browser-dom` evidence. The report writes stable local and
Salesforce paths only; frontdoor login URLs and one-time tokens are not written.
When both sides are captured, each case also includes a `comparison` block with
the scoped component selector, normalized visible text, project LWC component
names, project LWC component counts, and any current diffs.

The current browser oracle lane has two-sided browser proof for the app page,
custom tab, and URL-addressable component targets: local DOM captured,
Salesforce DOM captured, selector-scoped comparison passed, and zero browser
console or page errors. The app-page and custom-tab proofs deploy
`Lwc_Shell.app-meta.xml`, assign `Lwc_Shell_Access`, and open
`/lightning/app/c__Lwc_Shell/n/Lwc_Probe` so Salesforce resolves the same
FlexiPage-backed tab. Record pages need a real org record id and page
activation before they can become a
strict browser oracle. Quick actions need modal routing proof. Visualforce
Lightning Out browser capture needs the Visualforce fixture pages deployed to
the same org. Expanded base-component capture can start from the base-component
fixture context and the local direct component route.

For project corpus planning, use the sibling compat plugin:

```bash
cd ../glade-tools
go run ./cmd/glade-tools compat lwc corpus \
  --root /path/to/salesforce-repos \
  --out /path/to/lwc-corpus.json \
  --include-repos repo-a,repo-b,repo-c \
  --json
```
