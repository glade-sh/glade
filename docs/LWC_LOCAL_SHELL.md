# Local LWC Shell

Glade can serve Lightning Web Components from an SFDX project without deploying
to Salesforce. The local shell loads the project on disk, compiles LWC bundles,
reads Lightning page and custom tab metadata, starts the shared Lightning
runtime, and serves browser routes that look like Salesforce page targets.

Run the toolchain install before opening LWC routes:

```bash
glade toolchain install
```

Then start the local shell from the project root:

```bash
glade dev lwc --project . --port 8080
```

Use an ephemeral port and ready file for scripts:

```bash
glade dev lwc --project . --addr 127.0.0.1:0 --ready-file /tmp/glade-lwc-ready.json
```

The startup banner lists discovered preview routes and watches LWC, FlexiPage,
custom tab, Visualforce, Apex, and static resource changes.

## Preview Routes

Open these routes from the local server:

```text
/lwc/preview/component/<namespace>/<component>
/lwc/preview/record/<Object>/<recordId>?page=<FlexiPage>
/lwc/preview/app/<Page>
/lwc/preview/home/<Page>
/lwc/preview/tab/<Tab>
```

Examples:

```text
http://127.0.0.1:8080/lwc/preview/component/c/contextProbe
http://127.0.0.1:8080/lwc/preview/record/Account/001000000000001?page=Account_Record_Page
http://127.0.0.1:8080/lwc/preview/app/Sales_Dashboard
http://127.0.0.1:8080/lwc/preview/home/Custom_Home
http://127.0.0.1:8080/lwc/preview/tab/Lwc_Probe
```

Direct component preview mounts one component with URL context. Record, app,
home, and tab routes resolve Salesforce metadata from the project:

- `*.js-meta.xml` controls exposure, targets, target configs, objects, form
  factors, and builder properties.
- FlexiPage metadata controls record pages, app pages, home pages, regions, and
  component properties.
- Custom tab metadata controls LWC tabs, FlexiPage tabs, and Visualforce tabs.

Visualforce-backed custom tabs redirect to the Visualforce page route:

```text
/lwc/preview/tab/Visualforce_Tab -> /apex/<Page>
```

## Visualforce Lightning Out

`glade dev lwc` and `glade dev vf` share the same local Lightning runtime. A
Visualforce page that includes Lightning and calls `$Lightning.use()` and
`$Lightning.createComponent()` renders LWCs through `/apex/<PageName>`.

Use the Visualforce route when the real host is Visualforce:

```bash
glade dev vf --project . --port 8080
open http://127.0.0.1:8080/apex/WidgetHost
```

Use the LWC tab route when you want the Lightning shell to resolve tab metadata:

```text
http://127.0.0.1:8080/lwc/preview/tab/Visualforce_Tab
```

That route redirects to `/apex/<Page>`. The LWC modules, import map, Apex wire
endpoint, LDS/UI API shims, labels, resources, navigation basics, and browser
runtime are the same pieces used by the direct LWC shell.

## Local Context

The shell builds a page reference for each route:

| Route | PageReference shape |
| --- | --- |
| `/lwc/preview/component/<ns>/<component>` | `standard__component` |
| `/lwc/preview/record/<Object>/<recordId>?page=<FlexiPage>` | `standard__recordPage` |
| `/lwc/preview/app/<Page>` | `standard__app` |
| `/lwc/preview/home/<Page>` | `standard__namedPage` with `home` |
| `/lwc/preview/tab/<Tab>` | `standard__navItemPage` when the tab route is the active target |

Query parameters named `state.<key>` become PageReference state values:

```text
/lwc/preview/component/c/contextProbe?state.c__mode=review
```

Record routes pass `recordId` and `objectApiName` to page components. FlexiPage
component properties are passed as component attributes.

## Apex, LDS, Labels, And Resources

The local LWC runtime supports the first-mile Salesforce module paths used by
common local development loops:

- Apex wire and imperative imports through `@salesforce/apex/<Class>.<method>`.
- Local Apex controller execution for project methods that the local VM can
  invoke.
- `lightning/uiRecordApi` `getRecord` and `getObjectInfo` against local org
  state.
- `lightning/uiRecordApi` `createRecord`, `updateRecord`, `deleteRecord`,
  `getFieldValue`, and `getFieldDisplayValue` against local record shapes.
- `@salesforce/schema/<Object>` and `@salesforce/schema/<Object>.<Field>`
  tokens.
- `@salesforce/label/<namespace>.<name>` from project custom labels and known
  platform fallbacks.
- `@salesforce/resourceUrl/<name>` and
  `@salesforce/contentAssetUrl/<name>` for local static resources and content
  assets.
- `@salesforce/user/Id`, `@salesforce/user/isGuest`, and checked
  `@salesforce/i18n/*` values.
- `lightning/navigation` basics for `CurrentPageReference`,
  `NavigationMixin.Navigate`, and `NavigationMixin.GenerateUrl`.
- `lightning/messageService` in-page publish and subscribe,
  `lightning/platformResourceLoader` local script and style loading, and
  `lightning/platformShowToastEvent` browser toast events.

These services work in both `/lwc/preview/*` routes and Visualforce Lightning
Out pages unless a support row names a host-specific limit.

## Local Data

`glade dev lwc` starts with local org state inferred from the project schema and
loads fixture files from `data/*.json` when they use the Glade storage fixture
format. That is the same fixture behavior used by the Visualforce dev server.

Use fixtures for records that record-page LWCs and LDS wires should see:

```text
my-project/
  data/accounts.json
  force-app/main/default/lwc/
  force-app/main/default/flexipages/
  force-app/main/default/tabs/
  sfdx-project.json
```

For persistent local data loops, seed a database separately with `glade db seed`
and use the local API or test commands that target that database.

## Current Limits

This is local development support, not a hosted Salesforce replacement.

| Area | Current limit |
| --- | --- |
| Hosted Lightning Experience | Glade serves a local shell, not Salesforce chrome, app navigation, workspace API, or console behavior. |
| Metadata | The shell resolves LWC bundle metadata, FlexiPages, and custom tabs needed for preview routes. It does not implement every builder rule or Experience Cloud runtime. |
| Base components | Common custom LWCs can mount. Full `lightning-*` base component parity and SLDS fidelity depend on the supported runtime modules in this build. |
| LDS/UI API | `getRecord` and `getObjectInfo` are local shims. Full UI API, layouts, picklists, permissions, record edit flows, and server-side validation are not complete hosted parity. |
| Apex controllers | Supported local Apex executes in the Glade VM. Unsupported Apex surfaces return diagnostics instead of calling Salesforce. |
| Navigation | `CurrentPageReference`, `Navigate`, and `GenerateUrl` cover local page targets. Full router history, console navigation, named app behavior, and hosted URL generation remain limited. |
| Visualforce Lightning Out | LWC mounting through `$Lightning.use()` and `$Lightning.createComponent()` is supported for local pages. Exact hosted lifecycle timing and every Lightning Out edge remain outside the local contract. |

Use Salesforce for live auth, hosted permissions, org-only services, exact
Lightning Experience behavior, and final deployment gates.
