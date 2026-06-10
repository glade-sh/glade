# LWC Rendering Outside Lightning Experience: Technical Research & Methodology

## 1. Current State of LWC in Glade

### 1.1 What Exists

Glade has **indexing-only** support for LWC, with no rendering capability:

- `internal/project/` collects `.js` files from `lwc/` directories during project discovery.
- `internal/metadata/` catalogs LWC component names as `NamedAsset` entries.
- `internal/uicontroller/` uses regex to parse LWC JavaScript files for `@wire` decorators,
  `@salesforce/apex/` imports (wiring Apex methods to LWC), label/resource/schema imports,
  and local component imports. This feeds the performance scanner and type system to
  understand LWC-to-Apex dependencies.
- `internal/uicontroller/` also indexes `js-meta.xml` configuration files to determine
  component targets (`lightning__AppPage`, `lightning__RecordPage`, `lightning__Tab`, etc.),
  exposed properties, and capabilities.

### 1.2 What Is Missing

- No HTML template compilation (`*.html` files are not parsed for component markup).
- No CSS processing (`*.css` files are not processed or scoped).
- No `LightningElement` base class or component lifecycle (constructor, connectedCallback,
  renderedCallback, disconnectedCallback, errorCallback).
- No reactive property system (`@track`, `@api`, `@wire`).
- No template directive processing (`if:true`, `if:false`, `for:each`, `iterator:*`,
  `lwc:dom`, `lwc:spread`, `lwc:ref`, `lwc:inner-html`).
- No `@salesforce/*` module resolution at runtime (`@salesforce/apex`, `@salesforce/label`,
  `@salesforce/resourceUrl`, `@salesforce/schema`, `@salesforce/user`, `@salesforce/i18n`,
  `@salesforce/contentAssetUrl`).
- No Lightning base component registry (no `lightning-button`, `lightning-input`,
  `lightning-datatable`, etc.).
- No SLDS integration for LWC components.
- No Locker/LWS security sandbox.
- No wire adapter service (no `@wire(getRecord, ...)`, Apex wire proxying, etc.).
- No Lightning Message Service or pubsub.

### 1.3 Design Context

Glade's LWC support is currently an **import-graph analyzer**, not a renderer. It exists
to feed the performance scanner with knowledge of which Apex methods are wired into which
LWC components, and to power the type system with schema/label/resource references. This
is distinct from Visualforce in Glade, which has a full HTML rendering pipeline for ~40
standard components plus expression evaluation and view state management.

---

## 2. Approaches to Rendering LWC Outside Lightning Experience

Seven distinct approaches exist for rendering LWCs outside the standard Lightning
Experience container. These range from fully Salesforce-hosted (Lightning Out, Visualforce
embedding) to fully independent (open-source LWC via LWR).

### 2.1 Summary Matrix

| Approach | Hosting | Auth Required | Aura Required | Standard Components | Maturity |
|---|---|---|---|---|---|
| **Lightning Out (Beta) + Visualforce** | Salesforce | Yes (org session) | Yes (dependency app) | Limited (via Aura) | Beta, deprecated path |
| **Lightning Out 2.0** | Salesforce | Yes (OAuth/session) | No | Custom LWC only; standard via custom wrapper | GA (API v65+) |
| **Custom Tab (`lightning__Tab`)** | Salesforce | Yes | No | Yes (within Lightning shell) | GA |
| **Lightning App Builder pages** | Salesforce | Yes | No | Yes (within Lightning shell) | GA |
| **Experience Builder (LWR Sites)** | Salesforce | Optional | No | Yes (with target config) | GA |
| **LWR on Node.js (OSS)** | Self-hosted | Optional (no Salesforce) | No | Via `lightning-base-components` npm | Developer Preview |
| **Open-source LWC (`lwc` npm package)** | Self-hosted | N/A | No | Build your own | GA (v9.x) |

---

## 3. Lightning Out (Beta) — Visualforce & External Embedding

### 3.1 Architecture

Lightning Out (beta) embeds Lightning components into a non-Lightning container page using
a standalone Aura dependency app. The Aura app acts as a bootstrap layer that loads the
LWC engine into the DOM of the host page. Components run **directly on the host page DOM**
(no iframe isolation).

The Aura dependency app must:
- Extend `ltng:outApp` (the Lightning Out interface).
- Declare each LWC and its namespace as a dependency.
- Be named uniquely (the name is passed to `$Lightning.use()`).

```
Browser loads host page (VF or external)
  → JavaScript loads lightning.out.js from Salesforce domain
  → $Lightning.use("namespace:appName", callback, endpoint, authToken)
    → Aura framework bootstraps in-page
    → Creates Lightning Out app
  → $Lightning.createComponent("namespace:componentName", attributes, domElement, callback)
    → Factory instantiates LWC in target DOM element
    → component lifecycle runs (connectedCallback, renderedCallback)
```

### 3.2 Visualforce Integration

The `apex:includeLightning` component loads `lightning.out.js` into a Visualforce page:

```html
<apex:page standardController="Account">
    <apex:includeLightning />
    <div id="lwcContainer" />
    <script>
        $Lightning.use("c:myVFApp", function() {
            $Lightning.createComponent(
                "c:myComponent",
                { recordId: "{!Account.Id}" },
                "lwcContainer",
                function(cmp) { /* rendered */ }
            );
        });
    </script>
</apex:page>
```

Key constraints:
- The Lightning Out app must exist in the **same org** as the Visualforce page.
- Visualforce session context determines the API endpoint and authentication.
- Only custom LWCs are supported (standard base components have limited functionality).
- `$Lightning.use()` requires a callback; rendering is asynchronous.
- No `lightning/navigation` support; no flexipage context.

### 3.3 External Hosting (Non-Visualforce)

Using Lightning Out (beta) from an external page (Node.js, Heroku, internal server):

1. Configure **CORS** in Salesforce Setup (allow origin).
2. Load `lightning.out.js` via script tag pointing to the Salesforce instance CDN.
3. Obtain a valid Salesforce session ID or OAuth token.
4. Call `$Lightning.use()` with the endpoint URL and session token.
5. Create components via `$Lightning.createComponent()`.

This approach is subject to Beta Service Terms and has been **superseded by Lightning Out 2.0**.

### 3.4 Limitations

- **Security**: Components share the host page DOM. No iframe isolation.
- **Requires Lightning Locker** (LWS not supported for beta version).
- **Aura dependency**: Requires a listed Aura dependency app in the org.
- **Cookie requirements**: Requires browser third-party cookie support for cross-origin.
- **No standard component support**: Only custom Aura and LWC components.
- **Beta status**: Not recommended for new development.

---

## 4. Lightning Out 2.0 — Modern External Embedding

### 4.1 Architecture

Lightning Out 2.0 is a complete redesign built on **Lightning Web Runtime (LWR)**.
Instead of mounting components directly on the host page, each embedded LWC loads in an
**iframe encapsulated within a closed shadow DOM**. The host page creates custom web
components (`lightning-out-application` and per-component wrappers) that manage the
iframes.

```
Browser loads host page (any external site)
  → Script loads Lightning Out 2.0 JavaScript library
  → <lightning-out-application> custom element initializes
    Attributes: frontdoor-url, app-id, components
  → Uses frontdoor URL to establish Salesforce session
  → Fires lo.application.ready event
  → For each component in "components" list:
    → Creates custom element (e.g. <c-my-lwc>)
    → Creates iframe inside closed shadow DOM
    → Loads actual LWC inside iframe (Salesforce context)
    → Fires lo.component.ready event
```

### 4.2 Setup Process

1. **Org preparation**: Enable cross-domain Salesforce session cookies.
2. **Create Lightning Out 2.0 App** in Setup → Lightning Out 2.0 App Manager.
   - Declarative configuration (add components, set properties).
   - Generates an 18-character app ID (since Spring '26).
   - Produces HTML markup snippet for host page.
3. **Host page markup** from App Manager output:
```html
<script src="https://your-instance.lightning.force.com/lightning/lightning.out.2.js"></script>
<lightning-out-application
    frontdoor-url="<dynamic>"
    app-id="1Usfi200000006TCAQ"
    components="c-my-lwc,c-another-lwc">
</lightning-out-application>
<c-my-lwc style="--custom-color: brown;"></c-my-lwc>
<c-another-lwc></c-another-lwc>
```
4. **Authentication**: Exchange access token/session ID for a frontdoor URL via the UI
   Bridge API (`/secur/frontdoor_single_access.jsp`).
5. **Lifecycle events**:
   - `lo.application.ready` — session established.
   - `lo.application.error` — session failed.
   - `lo.component.ready` — component rendered inside iframe.
   - `lo.component.error` — component render/run-time failure.

### 4.3 Comparison with Lightning Out (Beta)

| Feature | Lightning Out 2.0 | Lightning Out (Beta) |
|---|---|---|
| Runtime base | LWR (Lightning Web Runtime) | Aura framework |
| Isolation | iframe + shadow DOM | Host page DOM (shared) |
| Security | LWS auto-applied | Lightning Locker required |
| Config | Declarative + programmatic | Programmatic only |
| Component types | Custom LWC only | Custom LWC + Aura |
| Aura dependency | **Not required** | Required |
| Events | Supported (via postMessage) | Not supported |
| Unauthenticated | Roadmap | Supported (with Digital Experiences) |
| Styling | CSS custom properties / SLDS hooks | SLDS or unstyled |
| GA status | GA (API v65+) | Beta |

### 4.4 Limitations

- Only custom LWC components. To use a standard base component, wrap it in a custom LWC
  (styling/behavior may differ from documented behavior).
- No Aura component embedding.
- Requires third-party (cross-origin) cookies enabled in browser.
- Cannot load the Lightning Out 2.0 JS library from within an LWC itself (LWS blocks
  script insertion).
- No page navigation support (`lightning/navigation` not supported).
- Authenticated users only (unauth on roadmap).
- No OAuth 2.0 client credentials flow (requires user context).
- Tooling API: `LightningOutApp` SObject available via API v65+ with fields:
  `ApplicationName`, `DeveloperName`, `IsEnabled`, `Language`, `MasterLabel`.

---

## 5. Standalone LWC Rendering Within Salesforce

### 5.1 Custom Tabs (`lightning__Tab`)

LWCs can be rendered as a full-page custom tab in Lightning Experience or the
Salesforce mobile app. This is the simplest in-platform "standalone" approach.

**Configuration** (`component.js-meta.xml`):
```xml
<?xml version="1.0" encoding="UTF-8"?>
<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
    <apiVersion>62.0</apiVersion>
    <isExposed>true</isExposed>
    <targets>
        <target>lightning__Tab</target>
    </targets>
</LightningComponentBundle>
```

- The component receives the full page width. No surrounding record context.
- Works within the Lightning Experience shell (header, nav bar present).
- Standard base components (`lightning-*`) are fully available.
- `@salesforce/*` modules work as in any Lightning context.
- Access to `lightning/navigation`, `lightning/messageService`, etc.

### 5.2 URL-Addressable Components (`lightning__UrlAddressable`)

Enables direct navigation to a component via a URL pattern:
`/lightning/cmp/{namespace}__{componentName}`

- Supports URL parameters for passing data.
- Uses `CurrentPageReference` wire to read URL parameters.
- No need for an Aura wrapper or containing app page.

### 5.3 Lightning App Builder Targets

LWCs configured with these targets render within the Lightning Experience shell, on
specific page types:

| Target | Context |
|---|---|
| `lightning__AppPage` | Custom Lightning app page; full-page or dashboard layout |
| `lightning__HomePage` | Home page; column-based layout |
| `lightning__RecordPage` | Record detail page; receives `recordId` |
| `lightning__UtilityBar` | Utility bar item; footer/hidden popup model |

All provide full access to standard base components, `@salesforce/*` modules,
and Lightning Data Service (LDS).

### 5.4 Flow Screen Components (`lightning__FlowScreen`)

LWCs can be embedded as screens within Salesforce Flows:

- Receive flow variable values as `@api` properties.
- Can emit `FlowAttributeChangeEvent` to pass data back to the flow.
- Render within the Flow runtime (Lightning or Classic context).

### 5.5 Quick Actions (`lightning__RecordAction`, `lightning__GlobalAction`)

LWCs can serve as quick actions, rendered in a modal dialog overlaying the current page:

| Target | Context |
|---|---|
| `lightning__RecordAction` | Record-page quick action; receives `recordId` |
| `lightning__GlobalAction` | Global quick action from utility bar or global actions menu |

- Access to standard `@salesforce/*` modules and LDS.
- Use `CloseActionScreenEvent` to dismiss the modal.
- Can invoke Apex methods and process results before closing.
- Render within the Lightning Experience shell context.

### 5.6 Email Content (`lightningStatic__Email`)

Enables an LWC to be used as email content in Email Content Builder:

- Components render server-side to HTML for email delivery.
- Supports `@salesforce/label` for localized strings.
- Limited interactivity (email constraints: no JavaScript execution in most clients).

### 5.7 Custom Property Editor (`lightning__PropertyEditor`)

Admin-facing component used as a custom property editor in Experience Builder:

- Referenced by a `LightningTypeBundle` or another component's `js-meta.xml`.
- Modifies the properties of other components at design time.

---

## 6. LWR (Lightning Web Runtime) Framework

### 6.1 Overview

LWR is an **open-source, Node.js-based web application framework** built by Salesforce
that uses Lightning Web Components as its component model. It provides an alternative to
the full Salesforce Lightning Experience for building web applications.

Key products powered by LWR:
- **Experience Cloud LWR Sites** — Managed B2B/B2C sites on Salesforce infrastructure.
- **Lightning Out 2.0** — External component embedding.
- **developer.salesforce.com** — The Salesforce developer documentation site itself.
- **LWR on Node.js** — Self-hosted local development and deployment.

### 6.2 LWR on Node.js Architecture

```
npm init lwr
  → Scaffolds project with:
    lwr.config.json    (routing, modules, server config)
    package.json       (npm dependencies)
    src/
      content/         (content templates: .md, .html, .njk)
      layouts/         (layout templates: .njk, .html)
      modules/         (custom LWC components)
      data/            (global context data: .json)
```

**lwr.config.json** key sections:
```json
{
  "lwc": {
    "modules": [
      { "dir": "$rootDir/src/modules" },
      { "npm": "lightning-base-components" }
    ]
  },
  "routes": [
    {
      "id": "home",
      "path": "/",
      "rootComponent": "app/app",
      "layoutTemplate": "$layoutsDir/main_layout.njk"
    }
  ],
  "port": 3000,
  "serverMode": "dev",
  "serverType": "express",
  "locker": { "enable": true, "trust": [...components...] }
}
```

### 6.3 Server Modes

| Mode | Command | Module Format | File Watch | Bundling | Minification |
|---|---|---|---|---|---|
| dev | `npm run dev` | ESM | ✅ | ❌ | ❌ |
| prod | `npm run start` | ESM | ❌ | ✅ | ✅ |
| compat | `npm run dev --mode compat` | AMD | ✅ | ❌ | ❌ |
| prod-compat | `npm run start --mode prod-compat` | AMD | ❌ | ✅ | ✅ |

### 6.4 Rendering Pipeline

1. **Request** arrives at route (e.g., `/`).
2. **Route resolution**: LWR matches URL to route config (`lwr.config.json`).
3. **Content template** rendered: Markdown/HTML/Nunjucks template compiled.
4. **Layout template** renders: Wraps content with `{{ body | safe }}` and
   `{{ lwr_resources | safe }}` (injects LWC engine scripts).
5. **LWC compilation**: Components referenced in templates or `rootComponent` are
   compiled by `@lwc/compiler` (template → JS, CSS → scoped CSS, JS → transformed).
6. **Module resolution**: `@salesforce/*` imports resolved from filesystem or npm.
7. **Bundle assembly**: In prod mode, modules bundled; singletons (`lwc`, `lwr/navigation`,
   `@lwc/synthetic-shadow`) excluded into shared bundles.
8. **Response**: HTML document with embedded `lwr_resources` script block, CSS, and
   hydrated component markup.

### 6.5 Templating

LWR supports three templating languages:
- **HTML**: Standard HTML with context interpolation via `{{ }}`.
- **Markdown**: Content-focused; supports YAML front matter; context only via route handlers.
- **Nunjucks**: Full templating (conditionals, loops, filters, includes).

Embedding LWC components in templates:
```html
<!-- In Nunjucks layout -->
<example-button label="Click Me"></example-button>
{{ lwr_resources | safe }}
```

### 6.6 Capabilities

- **SSR + Hydration**: With `lightning__ServerRenderable` and
  `lightning__ServerRenderableWithHydration` capabilities (for Experience Cloud LWR sites).
  Components render server-side for fast initial paint, then hydrate for interactivity.
- **Lightning Web Security (LWS)**: Optional sandbox via `locker.enable` in config.
- **SLDS integration**: `@salesforce-ux/design-system` npm package.
- **Lightning Base Components**: Via `lightning-base-components` npm package.
  ⚠️ **Currently unavailable** via npm (as of latest docs: "working on bringing them back").
- **Routing**: Server-side routes, client-side navigate via `lwr/navigation` module.
- **Global data**: JSON data files in `src/data/` merged into template context.
- **Route handlers**: Custom Node.js functions to manipulate context per-route.
- **Deployment**: Heroku, any Node.js host. Standard npm workflow.

### 6.7 LWR CLI

The `lwr` npm package provides:
- `lwr build` — Production build.
- `lwr dev` — Development server with HMR.
- `lwr start` — Production server.
- `lwr preview` — Preview built output.

### 6.8 Limitations

- **Developer Preview** status: Not GA for self-hosted production use. Breaking changes
  possible.
- Lightning base components **not currently available** via npm.
- No Apex wire support locally (LWR on Node.js doesn't connect to Salesforce orgs for
  data unless you build custom REST integration).
- No Lightning Data Service.
- No `@salesforce/apex` module resolution (no Apex proxy).
- No Salesforce authentication integration out of box.

---

## 7. Open-Source LWC (`lwc` npm Package)

### 7.1 Overview

The `lwc` npm package (v9.3.4 as of June 2026) is the core open-source Lightning Web
Components framework from Salesforce. It is the foundation upon which both Salesforce
platform LWC and LWR are built.

Package structure:
- `lwc` — Convenience package re-exporting all `@lwc/*` packages.
- `@lwc/engine-dom` — DOM rendering engine.
- `@lwc/engine-server` — Server-side rendering engine.
- `@lwc/compiler` — Compiles `.html` templates and `.js` modules into LWC internals.
- `@lwc/synthetic-shadow` — Shadow DOM polyfill for older browsers.
- `@lwc/features` — Feature flags (experimental signals, etc.).
- `@lwc/ssr-compiler` / `@lwc/ssr-runtime` — Experimental SSR packages.
- `@lwc/signals` — Experimental reactive signals.

### 7.2 Component Model

```js
// counter.js
import { LightningElement } from 'lwc';

export default class Counter extends LightningElement {
    count = 0;

    increaseCounter() {
        this.count += 1;
    }
}
```

```html
<!-- counter.html -->
<template>
    <p>Counter: {count}</p>
    <button onclick={increaseCounter}>Add</button>
</template>
```

Component model features:
- `LightningElement` base class (extends `HTMLElement`).
- Reactive properties: `@api` (public), `@track` (private reactive), `@wire` (data binding).
- Template directives: `if:true`, `if:false`, `for:each`, `iterator:*`, `lwc:dom`,
  `lwc:spread`, `lwc:ref`, `lwc:inner-html`, `lwc:external`.
- Lifecycle hooks: `constructor()`, `connectedCallback()`, `renderedCallback()`,
  `disconnectedCallback()`, `errorCallback()`.
- CSS scoping: styles auto-scoped to component shadow DOM.
- Slot-based composition (native `<slot>`).
- Event system: `CustomEvent` with `bubbles: true, composed: true`.

### 7.3 Using Open-Source LWC Without Salesforce

```bash
npm init lwr
# Select "Single Page App" → "LWC"
npm install
npm run start
# → Served at http://localhost:3000
```

Without the Salesforce platform:
- No `@salesforce/*` modules (must build or mock these).
- No Lightning base components (unless `lightning-base-components` npm package is
  available — currently unavailable).
- No Apex wire.
- Full SLDS via `@salesforce-ux/design-system` npm package.
- Full native web component standards (Custom Elements, Shadow DOM, HTML Templates).

### 7.4 Compilation Pipeline

```
.html template  →  @lwc/compiler  →  compiled JS (template function)
.js module       →  @lwc/compiler  →  transformed JS (decorator desugaring, wire transform)
.css             →  @lwc/compiler  →  scoped CSS string
                                 →  combined component module
                                 →  evaluated by @lwc/engine-dom
                                 →  custom element registered
                                 →  rendered in browser
```

---

## 8. Salesforce CLI Plugins & Development Tools

### 8.1 Current First-Party CLI Commands (`sf` CLI)

The modern `sf` CLI includes built-in Lightning development commands:

**`sf lightning dev component`**
- Preview a single LWC in isolation.
- Launches a local dev server with hot module replacement (HMR).
- Auto-detects changes to HTML, CSS, and non-API JavaScript.
- Requires a target org (scratch org or sandbox).
- No deployment needed for HTML/CSS changes.
- JavaScript changes that don't modify `@api` properties or public methods auto-refresh.

**`sf lightning dev app`**
- Preview an entire Lightning Experience app locally.
- Supports desktop, iOS, and Android device simulation.
- Real-time HMR for HTML/CSS and non-API JS changes.
- Must deploy component additions, removed components, or API signature changes.
- Enables Local Dev in the target org if not already enabled.

**`sf lightning dev site`**
- Preview an Experience Builder site locally.
- Same HMR capabilities as `lightning dev app`.
- Must deploy structural changes and republish the site.

### 8.2 Legacy CLI Plugin (`@salesforce/lwc-dev-server`)

The original LWC local development server, now **deprecated** (last published ~5 years
ago, v2.11.0):

```bash
sfdx plugins:install @salesforce/lwc-dev-server
sfdx force:lightning:lwc:start
```

Supported:
- `@salesforce/resourceUrl` — Served from local filesystem.
- `@salesforce/label` — Resolved from `labels/CustomLabels.labels-meta.xml`.
- `@salesforce/apex` — Proxied to connected scratch org.
- `@salesforce/schema` — Standard behavior.
- `@salesforce/i18n` — Hardcoded en-US defaults.
- `@salesforce/user` — `isGuest` always true; `Id` undefined.

Unsupported:
- `@salesforce/contentAssetUrl`.
- `@salesforce/apexContinuation`.
- Aura components.
- Flexipages.
- Lightning Locker.
- Design tokens and custom tokens in CSS.

This plugin has been **replaced** by the built-in `sf lightning dev *` commands.

### 8.3 LWR CLI (`lwr` npm package)

For LWR-based projects (both self-hosted and Experience Cloud):

```bash
npm install lwr
npx lwr dev     # Development server
npx lwr build   # Production build
npx lwr start   # Production server
```

- Configuration via `lwr.config.json`.
- Hot module replacement in dev mode.
- Module bundling and minification in production mode.
- Compat mode for older browsers (AMD format).

### 8.4 Creation Scaffolding

```bash
# LWR project scaffold
npm init lwr@latest

# SFDX project scaffold (includes LWC directory structure)
sf project generate --name myProject
```

---

## 9. Aura Compatibility Layer Requirements

### 9.1 When Aura Is Required

Aura serves as a compatibility wrapper for LWC in specific scenarios:

| Scenario | Aura Required? | Notes |
|---|---|---|
| Lightning Out (Beta) | ✅ **Yes** — dependency app extends `ltng:outApp` | Aura app bootstraps LWC engine in non-Lightning context |
| Lightning Out 2.0 | ❌ **No** — LWR-based | Built on LWR; no Aura dependency |
| Visualforce + LWC | ✅ **Yes** — via Lightning Out (Beta) or `apex:includeLightning` + Aura dependency app | Cannot embed LWC directly in VF without Aura bridge |
| Custom Tab | ❌ No | Native LWC support |
| Lightning App Builder | ❌ No | Native LWC support |
| Experience Builder | ❌ No | Native LWC support |
| Flow Screen | ❌ No | Native LWC support |
| Utility Bar | ❌ No | Native LWC support |
| Quick Actions (Record + Global) | ❌ No | Native LWC support |
| Email Content Builder | ❌ No | Native LWC support |
| URL-Addressable | ❌ No | Direct LWC navigation |
| Custom Property Editor | ❌ No | Admin-facing; Experience Builder integration |
| LWR on Node.js | ❌ No | LWR-based |
| Open-source LWC | ❌ No | No Salesforce platform at all |

### 9.2 Aura Dependency App Pattern (Legacy)

For Lightning Out (Beta), the Aura app acts as a named bridge:

```xml
<!-- myVFApp.app -->
<aura:application access="GLOBAL" extends="ltng:outApp">
    <aura:dependency resource="c:myLwcComponent"/>
    <aura:dependency resource="c:anotherLwc"/>
</aura:application>
```

Then referenced by name in JavaScript:
```js
$Lightning.use("c:myVFApp", function() { ... });
```

Without this Aura app, `$Lightning.use()` resolves to nothing — there is no named
application to attach the LWC engine to.

### 9.3 Evolution Away from Aura

Salesforce is actively reducing Aura dependency:
- Lightning Out 2.0 eliminates the Aura dependency entirely (LWR-based).
- `lightning:recordForm` and other Aura-exclusive components now have LWC equivalents.
- The `lightning:flow` Aura component has an LWC equivalent (`lightning-flow`).
- New platform capabilities target LWC first.
- Aura is in maintenance mode — no new features. All innovation is in LWC.

The only remaining critical Aura dependency is **Lightning Out (Beta) for Visualforce**.
For new development, this path is discouraged in favor of:
- Migrating Visualforce pages to Lightning pages.
- Using Lightning Out 2.0 for external embedding.
- Using `lightning__Tab` or URL-addressable components for standalone rendering.

### 9.4 LWC Inside Aura (Composition)

The reverse direction — embedding LWC inside Aura components — is fully supported and
was the original migration path from Aura to LWC:

```xml
<!-- Aura component -->
<aura:component>
    <c:myLwcComponent recordId="{!v.recordId}" onchange="{!c.handleChange}" />
</aura:component>
```

LWCs inside Aura:
- Can receive Aura attributes via `@api` properties.
- Can fire events that Aura handlers catch.
- Run in the LWC engine (not Aura's rendering engine).
- Are isolated: no shared reactivity, separate lifecycle.

---

## 10. Architectural Implications for Glade

### 10.1 Rendering Complexity Comparison

| Layer | Complexity | Dependencies |
|---|---|---|
| LWC template parsing | Medium | HTML parser, template directive transformer |
| LWC engine (`@lwc/engine-dom`) | Very High | Custom element registry, shadow DOM, reactive system, wire protocol |
| `@salesforce/*` module resolution | High | Apex proxy, schema metadata, label catalog, resource serving |
| Lightning base components | High | Individual component implementations, SLDS dependency |
| Locker/LWS security sandbox | Very High | Membrane-based access control, CSP, namespace isolation |
| Wire adapter service | High | Reactive wire protocol, LDS wire adapters, Apex wire proxy |
| LWR compilation pipeline | High | Module resolution, bundling, SSR/hydration |

For Glade, implementing a full LWC rendering pipeline would require a JavaScript runtime
(v8go or similar) to execute the `@lwc/engine-dom` and compiled component modules, or a
native Go reimplementation of the LWC engine and template compiler.

### 10.2 Incremental Approach Options

**Option A: Template-to-HTML Static Compilation** (lowest complexity)

Parse LWC HTML templates, resolve `{property}` bindings with known values, and emit
static HTML. No reactivity, no event handling, no wire. This is analogous to what Glade
already does for Visualforce — parse markup, resolve `${expression}`, emit HTML.

- Parse `for:each` / `iterator:*` for iteration of known data arrays.
- Resolve `if:true` / `if:false` for conditional rendering.
- Resolve `{property}` for top-level `@api` properties with default values.
- No `@wire`, no `@salesforce/apex` resolution.
- No event handling.

**Option B: JS Runtime with LWC Engine** (medium complexity)

Embed a JavaScript runtime (e.g., v8go) and load the `@lwc/engine-dom` package. Compile
LWC component bundles using `@lwc/compiler`, then evaluate them in the JS runtime.

- Requires v8go or equivalent JS runtime in Go.
- Requires `@lwc/*` npm packages to be bundled/cached.
- Enables full reactive property system, wire protocol, and lifecycle hooks.
- Requires mock/proxy implementations of `@salesforce/*` modules.
- Does NOT require an actual browser DOM — `@lwc/engine-server` could be used for SSR.

**Option C: Go-native LWC Engine Reimplementation** (highest complexity)

Re-implement the LWC reactive engine, template compiler, and wire protocol in pure Go.

- No external JS dependency.
- Full control over rendering pipeline.
- Enormous engineering effort (tens of thousands of lines of reactive framework logic).
- Must track upstream LWC changes.

### 10.3 Minimum Viable LWC Rendering Path

For a pragmatic first step aligned with Glade's Visualforce philosophy:

1. **Parse LWC HTML templates** into a template AST (similar to `visualforce/compiler.go`).
2. **Resolve static data bindings**: Given a component class with default `@api` property
   values and a mock `@wire` result, resolve `{property}` expressions in templates.
3. **Execute template directives**: `if:true/false`, `for:each`/`iterator:*`.
4. **Resolve component composition**: `<c-child-component>` → inline child template.
5. **Emit HTML**: Generate complete HTML output for the component tree.

This would enable:
- Snapshot tests: Verify component HTML output given known data.
- UI regression testing: Compare rendered output against baselines.
- Documentation generation: Auto-render component examples.

It would NOT enable:
- Interactive preview (no event handling).
- Real-time data binding.
- `@wire` data fetching.
- Form submission or navigation.

### 10.4 Key Dependencies for Any LWC Rendering

| Dependency | Source | License | Purpose |
|---|---|---|---|
| `lwc` | npm | MIT | Core framework (compiler + engine) |
| `@lwc/compiler` | npm | MIT | Template & JS compilation |
| `@lwc/engine-dom` | npm | MIT | Browser DOM rendering engine |
| `@lwc/engine-server` | npm | MIT | Server-side rendering (SSR) |
| `lightning-base-components` | npm | MIT | Standard base component implementations |
| `@salesforce-ux/design-system` | npm | MIT | SLDS CSS framework |
| v8go | Go module | BSD-3 | V8 JavaScript engine binding for Go |

### 10.5 Licensing & Distribution Considerations

All key LWC dependencies (lwc, @lwc/*, lightning-base-components) are MIT-licensed and
can be embedded or bundled. Glade could ship a pre-compiled bundle of the LWC engine
and compiler, or download them on-demand.

---

## 11. References

### 11.1 Salesforce Documentation

- Lightning Out 2.0 Developer Guide: `/docs/platform/lwc/guide/lightning-out-intro.html`
- Lightning Out 2.0 Architecture: `/docs/platform/lwc/guide/lightning-out-architecture.html`
- Lightning Out 2.0 Limitations: `/docs/platform/lwc/guide/lightning-out-limitations.html`
- Lightning Out (Beta) for Visualforce: `apex:includeLightning` component reference
- LWR on Node.js Developer Guide: `/docs/platform/lwr/guide/`
- LWR Project Configuration: `/docs/platform/lwr/guide/lwr-configure-project.html`
- LWR Templates: `/docs/platform/lwr/guide/lwr-templates.html`
- LWR Compile-Time Data: `/docs/platform/lwr/guide/lwr-compile-data.html`
- Lightning Base Components in LWR: `/docs/platform/lwr/guide/lwr-lwc.html`
- LWC Configuration Tags: `/docs/platform/lwc/guide/reference-configuration-tags.html`
- LWC Targets: `lightning__Tab`, `lightning__UrlAddressable`, etc.
- Local Dev (sf CLI): `sf lightning dev component|app|site`
- Tooling API: `LightningOutApp` object (API v65+)
- Open-Source LWC: https://lwc.dev

### 11.2 Package Registries

- `lwc` npm package: https://www.npmjs.com/package/lwc (v9.3.4, MIT)
- `lightning-base-components` npm package: https://www.npmjs.com/package/lightning-base-components (currently unavailable)
- `@salesforce/lwc-dev-server` npm package: https://www.npmjs.com/package/@salesforce/lwc-dev-server (v2.11.0, deprecated)
- `create-lwr` npm package: https://www.npmjs.com/package/create-lwr
- `lwr` npm package: https://www.npmjs.com/package/lwr

### 11.3 Source Repositories

- Open-source LWC: https://github.com/salesforce/lwc
- LWC Dev Server (legacy): https://github.com/forcedotcom/lwc-dev-server-feedback
- LWR: https://github.com/salesforce/lwr (internal)

### 11.4 Local Documentation

Scraped Salesforce documentation available at:
`example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run/`

Relevant directories:
- `lwc/` — LWC reference documentation (API modules, config tags, targets)
- `lightning/` — Aura reference (component model, JS API, event system)
- `visualforce/` — VF component reference (including `apex:includeLightning`)
- `cli-reference/` — CLI commands (including `sf lightning dev *`)
- `tooling-api/` — Tooling API objects (including `LightningOutApp`)
- `site-references/platform/lwr/` — LWR reference docs (CLI, CSR, SSR routing)

---

## 12. Implementation Status (Glade)

**Phase 1 — static template rendering (shipped):**

- `internal/lwc/` — bundle index, template compiler, static renderer, `@api` default extraction, minimal `lightning-*` stubs
- `internal/project/` — discovers `.html`, `.css`, and `js-meta.xml` under `lwc/` bundles
- `glade render lwc <component> [--project <root>] [--props <json>]` — emits static HTML for snapshot and regression use
- `lwc.RenderComponent` / `lwc.RenderComponentForTest` — programmatic render API with property overrides and wire mocks
- Fixtures: `testdata/local-tests/lwc-rendering/`

**Phase 2 — Lightning Out on Visualforce (partial):**

- `third_party/lwc/` + `compile.mjs` — `@lwc/compiler` batch compile to ESM modules
- `internal/lwcbrowser/` — manifest, bootstrap HTML, `PreparePageConfig`
- `internal/lwcruntime/embed/glade.out.js` — browser `$Lightning.use` / `createComponent` shim
- `internal/aura/` — `ltng:outApp` dependency app indexing
- `internal/lightningout/` — VF script parser for `$Lightning` calls
- `GET /lightning/glade.out.js`, `/lightning/modules/*`, `/lightning/vendor/lwc.js`
- VF `<apex:includeLightning />` emits bootstrap when server compiles LWC (via `glade dev vf` or local server)
- Fixture: `testdata/local-tests/lightning-out-vf/`

**Not yet implemented (see `docs/superpowers/plans/2026-06-10-lightning-out-vf-lwc-runtime.md`):**

- Real `@wire` adapter execution (Apex wire, `getRecord` LDS)
- Event bridging tests in browser CI
- `GET /lightning/cmp/...` SSR route
- Locker/LWS
- Full `lightning-base-components` parity
