# Local LWC Shell

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Local Lightning</p>
  <p>Serve Lightning Web Components from an SFDX project with Salesforce-like page context, local data, and the same runtime used by Visualforce Lightning Out.</p>
  <ul>
    <li>Start <code>glade dev lwc</code>.</li>
    <li>Open component, record, app, home, and tab routes.</li>
    <li>Keep Salesforce for hosted Lightning Experience parity.</li>
  </ul>
</div>

The LWC local shell is a preview feature. Glade can render LWCs from local
source without a deploy, but it does not replace hosted Lightning Experience.
It reads LWC bundle metadata, FlexiPages, custom tabs, Apex classes, labels,
resources, and local fixtures from the project on disk.

## Setup

Install the local LWC toolchain:

```bash
glade toolchain install
```

Start the shell:

```bash
glade dev lwc --project . --port 8080
```

For scripts:

```bash
glade dev lwc --project . --addr 127.0.0.1:0 --ready-file /tmp/glade-lwc-ready.json
```

The banner prints the local URL and discovered routes.

The VS Code extension does not manage this server yet. Start, stop, and route
selection stay in the terminal while this preview feature is still being
ironed out.

## Routes

Open one of these routes:

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

The shell uses `*.js-meta.xml` exposure and targets, FlexiPage regions and
properties, and custom tab metadata. Visualforce-backed custom tabs redirect to
`/apex/<Page>`.

## Visualforce Lightning Out

Visualforce-hosted LWCs use the same preview runtime. A page served at
`/apex/<PageName>` can include Lightning and call `$Lightning.use()` and
`$Lightning.createComponent()`.

```bash
glade dev vf --project . --port 8080
```

Open the Visualforce route when the real host is Visualforce. Open
`/lwc/preview/tab/<Tab>` when you want Glade to resolve tab metadata first. A
Visualforce tab redirects to `/apex/<Page>`.

## Data And Services

The LWC shell and Visualforce Lightning Out host share:

- Apex wire and imperative imports through `@salesforce/apex/<Class>.<method>`.
- Local Apex controller execution through the Glade VM.
- `lightning/uiRecordApi` `getRecord` and `getObjectInfo`.
- `lightning/uiRecordApi` create, update, delete, and field helper functions.
  Mutations use the local DML engine for supported objects, so ID sequences,
  required-field checks, audit fields, explicit nulls, and soft deletes match
  the local Apex DML path.
- `@salesforce/schema` object and field tokens.
- `@salesforce/label`, `@salesforce/resourceUrl`, and
  `@salesforce/contentAssetUrl`.
- `@salesforce/user`, checked `@salesforce/i18n` values, and
  `lightning/navigation` basics.
- `lightning/messageService`, `lightning/platformResourceLoader`, and
  `lightning/platformShowToastEvent` shims.

`CurrentPageReference` reports the local route context. `NavigationMixin`
supports local URL generation and navigation for supported page targets.

## Fixtures

At startup, Glade infers local org state from the project schema and loads Glade
storage fixtures from `data/*.json`. Use those fixtures for records that
record-page LWCs and LDS wires should read.

```text
my-project/
  data/accounts.json
  force-app/main/default/lwc/
  force-app/main/default/flexipages/
  force-app/main/default/tabs/
  sfdx-project.json
```

For persistent local data loops, seed a database with `glade db seed` and use
commands that target that database.

## Current Limits

Glade serves a local Lightning shell. It does not replace hosted Lightning
Experience, live auth, permissions, console APIs, full UI API, every
`lightning-*` base component, exact SLDS fidelity, hosted validation parity, or
every Lightning Out edge.

Unsupported metadata and unsupported Salesforce services return named
diagnostics instead of calling Salesforce. Keep a Salesforce gate for hosted
behavior and final deployment checks.
