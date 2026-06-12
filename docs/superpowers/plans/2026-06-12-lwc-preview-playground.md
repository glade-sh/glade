# LWC Preview Playground Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a local Salesforce-like LWC runner to `glade playground`, where project LWCs load with realistic Salesforce context, seeded local data, base Lightning components, and backing Apex through the Glade VM.

**Architecture:** Keep rendering in the browser with the official LWC engine and keep Apex execution in Go. The playground server indexes LWC bundles, compiles a local preview module with Salesforce import shims, persists preview scenarios with page context and public properties, and exposes Apex and UI API endpoints backed by the existing `Runner` runtime template and org store.

**Tech Stack:** Go 1.26, existing `internal/playground` server, existing React/Vite playground UI, `@lwc/compiler`, `@lwc/engine-dom`, `@lwc/wire-service`, `@lwc/synthetic-shadow`, `lightning-base-components`, `@salesforce-ux/design-system`, SQLite-backed `storage.OrgState`, `vm.InvokeLWCMethod`.

---

## Recommendation

Build this as a new local-only mode inside `glade playground`, not as a continuation of the Visualforce rendering path.

The earlier Visualforce rendering lane was phase-partial. Its hard part is the Visualforce request lifecycle: component tree, controller hydration, view state, postback, partial rerender, and `PageReference.getContent`. LWC has a cleaner local split. The browser can run LWC modules. Glade can supply the platform modules that Salesforce supplies in an org.

The first working feature should support these cases:

- Show every local LWC bundle, with clear badges for exposed, target-compatible, unresolved Apex, and unsupported platform imports.
- Open a bundle in a focused preview runner rather than a Lightning App Builder clone.
- Pick a preview scenario: Record Page, App Page, Home Page, or bare component.
- Set required context: `recordId`, `objectApiName`, URL state, form factor, user, locale, timezone, theme, and public `@api` properties.
- Edit properties declared in `targetConfigs`, and also expose detected public `@api` properties when metadata is missing.
- Call imported Apex methods through Glade with wire and imperative styles.
- Read and mutate the same local org used by execute anonymous, SOQL, DML, and the database browser.
- Support the local data services LWCs commonly need: current record, object info, picklist values, labels, schema imports, static resources, navigation, toasts, `refreshApex`, and cache invalidation.
- Wrap preview in a Lightning-like record/app shell with SLDS and real `lightning-base-components` modules where they can run outside Salesforce.

Out of scope for the first slice:

- Full Lightning App Builder fidelity.
- Drag-and-drop page building as a primary workflow.
- Full Lightning Data Service.
- Locker or Lightning Web Security emulation.
- Salesforce data-backed `lightning/*` components that the npm package does not include, such as `lightning-record-form` and `lightning-input-field`.
- Visualforce rendering or Visualforce/LWC composition.
- Hosted public playground support.

## Compelling MVP

The MVP is a local component workbench. A user opens `glade playground --project . --db .glade/playground/org.sqlite --open`, clicks `LWC Preview`, chooses a component, chooses a scenario, and sees it run with data. The first screen should answer three questions without setup work:

- What context is this LWC running under?
- What data is backing it?
- What Apex and platform services did it call?

The runner should feel like a Salesforce record page only where that helps the component work. It should have a record header, current record picker, object selector, form factor switch, public property inspector, seed/reset controls, network/Apex call log, and database tab. It should not spend MVP effort on free-form layout regions, App Builder chrome, or page metadata export.

The strongest demo:

- Seed Account, Contact, Opportunity, Case, User, Profile, and RecordType data.
- Select `Account` and `001000000000001` as current context.
- Load an Account summary LWC that imports `@salesforce/apex`, `@salesforce/schema`, `@salesforce/label`, and `lightning/uiRecordApi`.
- Render `lightning-card`, `lightning-button`, `lightning-input`, `lightning-datatable`, `lightning-formatted-*`, and `lightning-toast` behavior.
- Click a button that calls local Apex, writes local DML, refreshes a wire, and shows the changed database state.

If that works clean, the tool is useful before it ever grows layout editing.

## Required LWC Context

Treat context as a first-class input. Most useful LWCs do not just render markup. They expect Salesforce to hand them a page, a record, a user, schema, labels, resources, and service adapters.

MVP context support:

| Need | Local source | MVP behavior |
| --- | --- | --- |
| `recordId` | Scenario current record picker | Set as component public property and runtime context. |
| `objectApiName` | Scenario object picker or record ID prefix lookup | Set as component public property and runtime context. |
| `@salesforce/client/formFactor` | Scenario form factor control | Return `Large`, `Medium`, or `Small`. |
| `@salesforce/user/Id` | Scenario user picker, default local User | Return selected user ID. |
| `@salesforce/i18n/*` | Scenario locale and timezone | Return deterministic locale, timezone, direction, and currency defaults. |
| `@salesforce/label/*` | `internal/metadata` custom label index | Return label value or a clear missing-label diagnostic. |
| `@salesforce/resourceUrl/*` | Static resource index and server route | Return local `/resource/...` or `/playground/lwc/resource/...` URL. |
| `@salesforce/schema/*` | Project schema and storage definitions | Return object and field descriptor with API name and relationship path. |
| `@salesforce/apex/*` | Existing VM `InvokeLWCMethod` | Support wire and imperative calls with typed JSON params. |
| `refreshApex` | Runtime call registry | Rerun the matching Apex or UI API adapter and log the refresh. |
| `notifyRecordUpdateAvailable` | Current org and wire registry | Refresh affected `getRecord` wires by record ID. |
| `lightning/uiRecordApi.getRecord` | `storage.OrgState` current record | Return LDS-like `{ id, apiName, fields }`. |
| `lightning/uiRecordApi.updateRecord` | Glade DML path or focused storage update | Update existing local records, refresh affected wires, and log changed fields. |
| `lightning/uiObjectInfoApi.getObjectInfo` | Project schema and storage definitions | Return object API name, fields, record type infos, and default record type ID. |
| `lightning/uiObjectInfoApi.getPicklistValues` | Value sets and field metadata | Return local values when present; return empty values with diagnostic when unknown. |
| `NavigationMixin` | Preview runtime | Record navigation request and show it in the call log. |
| `ShowToastEvent` | Preview runtime | Show toast in parent shell and call log. |
| Lightning Message Service | Preview runtime | MVP can stub local publish/subscribe in one iframe; cross-page messaging is out of scope. |

The context bar should expose the parts a developer changes often: page type, object, record, user, form factor, data mode, and seed. The inspector should expose public properties. Advanced context such as URL state, locale, timezone, and theme can live behind a collapsible section.

## Base Component Sources

Use two component sources, with clear roles:

- `lightning-base-components` npm package: primary runtime source for ordinary `lightning-*` components. It is published as an npm package, has an MIT license in package metadata, carries an LWC module map, and includes current component source under `src/lightning`.
- `jerry-wang12/lightning-demo`: project-approved source for copied data-backed base component code and tests. GitHub metadata observed on 2026-06-12 showed 10 commits, no releases, and last push on 2019-03-26. The inspected commit was `2f1c6ea4078fd584aea245256073be086d743650`.

The `lightning-demo` repo is valuable because it contains 164 `src/lightning` directories, including data-backed components not present in `lightning-base-components`: `recordForm`, `recordEditForm`, `recordViewForm`, `inputField`, `outputField`, `uiRecordApi`, `uiObjectInfoApi`, `uiListApi`, `uiActionsApi`, and `uiLookupsApi`. Those files mostly depend on a `force/lds` module that the repo does not include. That is the contract Glade should implement.

MVP strategy:

- Runtime component source: use `lightning-base-components`.
- Data-backed component source: copy selected `lightning-demo` modules and tests into the LWC runtime vendor area, then run a mechanical formatting pass. Do not mix formatting changes with behavior changes.
- LDS backing: implement Glade's own `force/lds` module so copied record forms, `lightning/uiRecordApi`, and `lightning/uiObjectInfoApi` talk to the local playground server.
- Tests: copy upstream tests where useful, then adapt the harness to Glade's runtime. Keep assertion intent intact when imports or mocks need local replacements.

The useful seam is `force/lds`. Build one Glade module behind `lightning/uiRecordApi`, `lightning/uiObjectInfoApi`, `lightning/uiListApi`, `lightning/uiActionsApi`, and `lightning/uiLookupsApi`. The LWC runtime should call that module, and the module should call the playground server.

## Current Repo Fit

Useful timber already stands in the repo:

- `internal/playground/server.go` owns `/playground/api/*`, seed, reset, database, run, and static asset routes.
- `internal/playground/runner.go` already builds a cached VM runtime from the project, merges local org schema, persists org state, and has `Seed`.
- `internal/playground/workspace.go` loads project references but currently allows only `.cls`, `.trigger`, `.apex`, `.json`, `.xml`, `.yml`, and `.yaml`.
- `internal/metadata/metadata.go` identifies LWC bundle names from `project.Project.LWCFiles`, but only `.js` files are collected today.
- `internal/uicontroller/index.go` already parses LWC Apex imports and `@wire` references, then resolves only static `@AuraEnabled` Apex methods.
- `internal/vm/ui_invocation.go` already has `InvokeLWCMethod(className, methodName, params)`.
- `internal/playground/web/src/App.tsx` already has the three-pane workbench, run controls, source tabs, seed/reset, and database browser.

The first changes should reuse those pieces.

## Phase 0: One-page spike

Purpose: prove that a real LWC bundle can render in the existing playground browser and call a real local Apex method.

Definition of done:

- A fixture project with one exposed LWC renders a button and text.
- The preview scenario sets `pageType=lightning__RecordPage`, `objectApiName=Account`, `recordId=001000000000001`, `formFactor=Large`, `locale=en-US`, and `theme=Lightning`.
- The component receives `recordId` and `objectApiName` as public properties when it declares or targets a record page context.
- The component imports `@salesforce/apex/PreviewController.summary`.
- The component imports `@salesforce/schema/Account.Name` and receives a local schema descriptor.
- The component calls a local `getRecord` wire adapter for the current record.
- Clicking the button calls `/playground/api/lwc/invoke`.
- The Apex method queries seeded `Account` data from the current org.
- The UI shows data from Apex, record wire data, and the trace tab shows `framework=lwc`.

Do not build page layout editing until this passes.

## File Map

Create:

- `internal/lwcpreview/catalog.go`
  Parse LWC bundle directories and metadata files into exposed bundle descriptors.

- `internal/lwcpreview/catalog_test.go`
  Test `isExposed`, targets, target configs, property defaults, and hidden bundles.

- `internal/lwcpreview/scenario.go`
  Define preview scenarios, context defaults, public property values, and validation.

- `internal/lwcpreview/scenario_test.go`
  Test current record context, target compatibility, public property coercion, URL state, form factor, and persisted scenarios.

- `internal/lwcpreview/build.go`
  Go wrapper around the Node compile script. Cache output under `.glade/playground/lwc-cache`.

- `internal/lwcpreview/build_test.go`
  Test cache keys and missing Node/dependency error messages without requiring Node.

- `internal/playground/lwc_preview.go`
  Add playground server handlers for catalog, scenarios, compile, module assets, Apex invocation, and UI API data.

- `internal/playground/lwc_preview_test.go`
  Test HTTP contracts with a temp SFDX project.

- `internal/playground/lwc-runtime/package.json`
  Pin local preview compiler dependencies.

- `internal/playground/lwc-runtime/package-lock.json`
  Commit exact dependency tree after `npm install`.

- `internal/playground/lwc-runtime/build.mjs`
  Compile LWC bundles and generate shims.

- `internal/playground/lwc-runtime/shims/gladeRuntime.js`
  Browser runtime bridge for Apex, wire adapters, labels, schema, resources, and page context.

- `internal/playground/lwc-runtime/shims/lightning-data/*.js`
  Glade-owned shims for Salesforce data-backed modules that `lightning-base-components` does not provide.

- `internal/playground/web/src/lib/lwc-preview.ts`
  Frontend types and API helpers.

- `internal/playground/web/src/lib/lwc-preview.test.ts`
  Unit tests for property coercion, scenario updates, and context derivation.

- `internal/playground/web/src/components/LWCPreview.tsx`
  Preview shell: component shelf, scenario/context bar, property inspector, iframe runner, call log, status, and diagnostics.

- `internal/playground/web/src/components/LWCPreview.test.tsx`
  Component tests for catalog render, add/remove, property edit, and invoke error display.

- `testdata/playground/lwc-preview-basic/`
  Fixture project with Apex, LWC, `seed.json`, and expected catalog shape.

Modify:

- `internal/project/project.go`
  Collect all LWC bundle files, not only `.js`.

- `internal/project/project_test.go`
  Prove `.html`, `.css`, `.js-meta.xml`, and `.svg` stay attached to a bundle.

- `internal/metadata/metadata.go`
  Preserve bundle directory metadata while keeping existing public JSON stable.

- `internal/playground/workspace.go`
  Allow local project LWC files in managed workspace loads and hashing.

- `internal/playground/server.go`
  Route new `/playground/api/lwc/*` and `/playground/lwc/*` paths.

- `internal/playground/runner.go`
  Add a method for UI Apex invocation that shares runtime template and org state.

- `internal/playground/types.go`
  Add preview API response types only if they belong in the playground package.

- `internal/playground/web/src/App.tsx`
  Add a top-level tab or segmented view for `Apex` and `LWC Preview`.

- `internal/playground/web/src/index.css`
  Add Salesforce-like shell styling and SLDS import integration.

- `internal/playground/web/package.json` and `package-lock.json`
  Add UI-only test helpers only if component tests require them.

- `site/docs-src/guide/playground.md`
  Document local LWC preview and its limits.

- `site/docs-src/guide/cli-reference.md`
  Add flags only if CLI behavior changes.

## Data Contracts

Add these Go types in `internal/lwcpreview/catalog.go`:

```go
package lwcpreview

type Catalog struct {
	Bundles []Bundle `json:"bundles"`
}

type Bundle struct {
	Name        string         `json:"name"`
	Dir         string         `json:"dir"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	APIVersion  string         `json:"apiVersion,omitempty"`
	IsExposed   bool           `json:"isExposed"`
	Targets     []string       `json:"targets,omitempty"`
	Properties  []Property     `json:"properties,omitempty"`
	ApexMethods []ApexMethodRef `json:"apexMethods,omitempty"`
	Files       []BundleFile   `json:"files,omitempty"`
}

type Property struct {
	Name        string   `json:"name"`
	Label       string   `json:"label,omitempty"`
	Type        string   `json:"type"`
	Default     string   `json:"default,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Description string   `json:"description,omitempty"`
	Datasource  string   `json:"datasource,omitempty"`
	Values      []string `json:"values,omitempty"`
	Targets     []string `json:"targets,omitempty"`
}

type ApexMethodRef struct {
	ClassName  string `json:"className"`
	MethodName string `json:"methodName"`
	Resolved   bool   `json:"resolved"`
	ReturnType string `json:"returnType,omitempty"`
	File       string `json:"file,omitempty"`
	Line       int    `json:"line,omitempty"`
}

type BundleFile struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}
```

Add these Go types in `internal/lwcpreview/scenario.go`:

```go
package lwcpreview

type Scenario struct {
	Version       string            `json:"version"`
	Name          string            `json:"name"`
	BundleName    string            `json:"bundleName"`
	PageType      string            `json:"pageType"`
	ObjectAPIName string            `json:"objectApiName,omitempty"`
	RecordID      string            `json:"recordId,omitempty"`
	FormFactor    string            `json:"formFactor"`
	UserID        string            `json:"userId,omitempty"`
	ProfileID     string            `json:"profileId,omitempty"`
	Locale        string            `json:"locale"`
	TimeZone      string            `json:"timeZone"`
	Theme         string            `json:"theme"`
	URLState      map[string]string `json:"urlState,omitempty"`
	Properties    map[string]string `json:"properties,omitempty"`
	SeedPath      string            `json:"seedPath,omitempty"`
	DataMode      string            `json:"dataMode"`
	UpdatedUnix   int64             `json:"updatedUnix,omitempty"`
}

type ScenarioDefaults struct {
	PageType   string `json:"pageType"`
	FormFactor string `json:"formFactor"`
	Locale     string `json:"locale"`
	TimeZone   string `json:"timeZone"`
	Theme      string `json:"theme"`
	DataMode   string `json:"dataMode"`
}

const ScenarioVersion = "glade.lwc-preview.scenario.v1"
```

Valid `pageType` values:

```text
lightning__RecordPage
lightning__AppPage
lightning__HomePage
bare
```

Valid `formFactor` values:

```text
Large
Medium
Small
```

Valid `dataMode` values:

```text
scratch
persist
```

Persist local scenarios to:

```text
.glade/playground/lwc-scenarios/default.json
```

When the playground points at a real project, keep preview scenario state under `.glade/`. Do not write Salesforce metadata by default. Add export later if needed.

## HTTP Contracts

Add local-only routes:

```text
GET  /playground/api/lwc/catalog
GET  /playground/api/lwc/scenario
PUT  /playground/api/lwc/scenario
GET  /playground/api/lwc/context
POST /playground/api/lwc/build
POST /playground/api/lwc/invoke
POST /playground/api/lwc/ui-record
POST /playground/api/lwc/object-info
POST /playground/api/lwc/picklist-values
GET  /playground/lwc/assets/{cacheKey}/{file}
```

`GET /playground/api/lwc/catalog` response:

```json
{
  "bundles": [
    {
      "name": "accountSummary",
      "title": "Account Summary",
      "description": "Shows account and contact counts.",
      "apiVersion": "65.0",
      "isExposed": true,
      "targets": ["lightning__RecordPage"],
      "properties": [
        {
          "name": "heading",
          "label": "Heading",
          "type": "String",
          "default": "Summary",
          "targets": ["lightning__RecordPage"]
        }
      ],
      "apexMethods": [
        {
          "className": "AccountPreviewController",
          "methodName": "summary",
          "resolved": true,
          "returnType": "Map<String,Object>"
        }
      ]
    }
  ]
}
```

`GET /playground/api/lwc/context` response:

```json
{
  "defaults": {
    "pageType": "lightning__RecordPage",
    "formFactor": "Large",
    "locale": "en-US",
    "timeZone": "America/Los_Angeles",
    "theme": "Lightning",
    "dataMode": "scratch"
  },
  "objects": [
    {
      "name": "Account",
      "label": "Account",
      "keyPrefix": "001",
      "recordCount": 2
    }
  ],
  "records": {
    "Account": [
      {
        "id": "001000000000001",
        "label": "Twin Lakes Supply"
      }
    ]
  }
}
```

`POST /playground/api/lwc/invoke` request:

```json
{
  "className": "AccountPreviewController",
  "methodName": "summary",
  "params": {
    "recordId": "001000000000001"
  },
  "mode": "scratch"
}
```

Response uses the existing `vm.UIInvocationResult` shape:

```json
{
  "framework": "lwc",
  "className": "AccountPreviewController",
  "methodName": "summary",
  "success": true,
  "returnValue": {
    "name": "Twin Lakes Supply"
  },
  "trace": []
}
```

`POST /playground/api/lwc/ui-record` request:

```json
{
  "operation": "getRecord",
  "recordId": "001000000000001",
  "fields": ["Account.Id", "Account.Name"]
}
```

Response:

```json
{
  "id": "001000000000001",
  "apiName": "Account",
  "fields": {
    "Id": { "value": "001000000000001", "displayValue": "001000000000001" },
    "Name": { "value": "Twin Lakes Supply", "displayValue": "Twin Lakes Supply" }
  }
}
```

`POST /playground/api/lwc/ui-record` update request:

```json
{
  "operation": "updateRecord",
  "fields": {
    "Id": "001000000000001",
    "Name": "Twin Lakes Supply Updated"
  }
}
```

Response:

```json
{
  "id": "001000000000001",
  "apiName": "Account",
  "updatedFields": ["Name"]
}
```

`POST /playground/api/lwc/build` response:

```json
{
  "cacheKey": "sha256:...",
  "entry": "/playground/lwc/assets/sha256.../entry.js",
  "css": [
    "/playground/lwc/assets/sha256.../preview.css"
  ],
  "diagnostics": []
}
```

In public playground mode, return `403` for build and invoke:

```json
{"error":"LWC preview is disabled in public playground mode"}
```

## Implementation Tasks

### Task 1: Collect complete LWC bundle files

**Files:**

- Modify: `internal/project/project.go`
- Modify: `internal/project/project_test.go`
- Modify: `internal/playground/workspace.go`

- [ ] **Step 1: Write the failing project loader test**

Add a test case under `internal/project/project_test.go` that creates this tree:

```text
force-app/main/default/lwc/accountSummary/accountSummary.js
force-app/main/default/lwc/accountSummary/accountSummary.html
force-app/main/default/lwc/accountSummary/accountSummary.css
force-app/main/default/lwc/accountSummary/accountSummary.js-meta.xml
force-app/main/default/lwc/accountSummary/icon.svg
```

Assert `p.LWCFiles` contains all five files in sorted order.

- [ ] **Step 2: Run the focused test**

Run:

```bash
go test ./internal/project -run TestLoadProjectIndexesLWCBundleFiles -count=1
```

Expected before implementation:

```text
FAIL
```

The failure should show that only `.js` is indexed.

- [ ] **Step 3: Expand LWC file collection**

In `internal/project/project.go`, replace the LWC branch with a helper:

```go
case isLWCPath(lower) && isLWCSourceFile(lower):
	p.LWCFiles = append(p.LWCFiles, path)
```

Add:

```go
func isLWCSourceFile(path string) bool {
	switch {
	case strings.HasSuffix(path, ".js"):
		return true
	case strings.HasSuffix(path, ".html"):
		return true
	case strings.HasSuffix(path, ".css"):
		return true
	case strings.HasSuffix(path, ".svg"):
		return true
	case strings.HasSuffix(path, ".js-meta.xml"):
		return true
	default:
		return false
	}
}
```

- [ ] **Step 4: Allow playground workspace reads**

In `internal/playground/workspace.go`, update `isAllowedPlaygroundExtension` so managed workspace loads can carry LWC assets:

```go
case ".cls", ".trigger", ".apex", ".json", ".xml", ".yml", ".yaml", ".js", ".html", ".css", ".svg":
	return true
```

Keep `SaveFile` size limits unchanged.

- [ ] **Step 5: Run tests**

Run:

```bash
go test ./internal/project ./internal/playground -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/project
ok  	github.com/glade-sh/glade/internal/playground
```

Commit:

```bash
git add internal/project/project.go internal/project/project_test.go internal/playground/workspace.go
git commit -m "feat(playground): index complete lwc bundles"
```

### Task 2: Build the LWC catalog package

**Files:**

- Create: `internal/lwcpreview/catalog.go`
- Create: `internal/lwcpreview/catalog_test.go`
- Modify: `internal/uicontroller/index.go` only if absolute paths need normalization.

- [ ] **Step 1: Write catalog tests**

Create test fixtures in `catalog_test.go` using `t.TempDir()`. Include:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <apiVersion>65.0</apiVersion>
  <isExposed>true</isExposed>
  <masterLabel>Account Summary</masterLabel>
  <description>Shows seeded account data.</description>
  <targets>
    <target>lightning__RecordPage</target>
    <target>lightning__AppPage</target>
  </targets>
  <targetConfigs>
    <targetConfig targets="lightning__RecordPage">
      <property name="heading" type="String" label="Heading" default="Summary"/>
      <property name="showContacts" type="Boolean" label="Show Contacts" default="true"/>
    </targetConfig>
  </targetConfigs>
</LightningComponentBundle>
```

Test these facts:

- Hidden bundles are omitted from `Catalog.ExposedBundles("lightning__RecordPage")`.
- `Account Summary` becomes `Title`.
- Properties attach only to matching target configs.
- Public `@api recordId;`, `@api objectApiName;`, and `@api heading;` properties are detected from the component JavaScript when metadata omits them.
- Apex imports from `@salesforce/apex/AccountPreviewController.summary` are reported with `Resolved=true` when the class has `@AuraEnabled static`.

- [ ] **Step 2: Run the failing test**

Run:

```bash
go test ./internal/lwcpreview -run TestCatalogFromProject -count=1
```

Expected:

```text
FAIL
```

Package does not exist yet.

- [ ] **Step 3: Implement metadata parsing**

Use `encoding/xml`. Define XML structs with explicit fields for:

- `apiVersion`
- `isExposed`
- `masterLabel`
- `description`
- `targets.target`
- `targetConfigs.targetConfig`
- `property`
- public `@api` fields in the component JavaScript

Property XML struct:

```go
type propertyXML struct {
	Name        string `xml:"name,attr"`
	Label       string `xml:"label,attr"`
	Type        string `xml:"type,attr"`
	Default     string `xml:"default,attr"`
	Required    string `xml:"required,attr"`
	Description string `xml:"description,attr"`
	Datasource  string `xml:"datasource,attr"`
}
```

Parse `datasource="A,B,C"` into `Values`.

For public `@api` detection, keep the parser conservative:

```go
var lwcAPIPropertyRe = regexp.MustCompile(`(?m)@api\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*(?:=|;|\n)`)
```

When a property is detected in JS but not metadata, add it as `Type: "String"` and `Targets: bundle.Targets`. Do not infer type from the property name.

- [ ] **Step 4: Join UI controller Apex references**

Reuse `uicontroller.Build(p, apexIndex)` instead of reparsing JS. For each LWC bundle, attach `ApexMethodRef` values from `idx.ApexMethods` where `Framework == "lwc"` and the source file lives under the bundle directory.

- [ ] **Step 5: Run tests**

Run:

```bash
go test ./internal/lwcpreview ./internal/uicontroller -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/lwcpreview
ok  	github.com/glade-sh/glade/internal/uicontroller
```

Commit:

```bash
git add internal/lwcpreview internal/uicontroller
git commit -m "feat(playground): catalog exposed lwc bundles"
```

### Task 3: Add scenario context and persistence

**Files:**

- Create: `internal/lwcpreview/scenario.go`
- Create: `internal/lwcpreview/scenario_test.go`
- Create: `internal/playground/lwc_preview.go`
- Create: `internal/playground/lwc_preview_test.go`
- Modify: `internal/playground/server.go`

- [ ] **Step 1: Write scenario tests**

Test:

- New scenario defaults to `pageType=lightning__RecordPage`, `formFactor=Large`, `locale=en-US`, `timeZone=America/Los_Angeles`, `theme=Lightning`, and `dataMode=scratch`.
- A Record Page scenario requires `objectApiName` and `recordId` before build.
- App Page, Home Page, and bare scenarios do not require `recordId`.
- Scenario rejects a bundle not compatible with the selected page type unless `pageType=bare`.
- Boolean and numeric property values stay string-backed in JSON but validate against property type.
- Missing metadata still permits detected public `@api` properties with `type=String`.
- URL state keys and values are trimmed and persisted.
- Scenario data persists at `.glade/playground/lwc-scenarios/default.json`.

- [ ] **Step 2: Implement scenario helpers**

Add:

```go
func DefaultScenario(bundleName string) Scenario
func ValidateScenario(scenario Scenario, catalog Catalog) error
func CoercePropertyValue(prop Property, value string) (string, error)
func ScenarioPath(dataRoot string) string
func LoadScenario(dataRoot string, catalog Catalog) (Scenario, error)
func SaveScenario(dataRoot string, scenario Scenario, catalog Catalog) error
func ScenarioProperties(bundle Bundle, scenario Scenario) map[string]string
```

Use `os.MkdirAll(filepath.Dir(path), 0o755)` and indented JSON.

- [ ] **Step 3: Add server routes**

In `Server.ServeHTTP`, add cases before the static file catch-all:

```go
case r.Method == http.MethodGet && r.URL.Path == "/playground/api/lwc/catalog":
	s.handleLWCCatalog(w, r)
case r.Method == http.MethodGet && r.URL.Path == "/playground/api/lwc/scenario":
	s.handleLWCScenario(w, r)
case r.Method == http.MethodPut && r.URL.Path == "/playground/api/lwc/scenario":
	s.handleSaveLWCScenario(w, r)
case r.Method == http.MethodGet && r.URL.Path == "/playground/api/lwc/context":
	s.handleLWCContext(w, r)
```

- [ ] **Step 4: Build catalog and context from current workspace**

In `internal/playground/lwc_preview.go`, load project and schema the same way `Runner.loadRuntimeTemplate` does. Do not duplicate parsing logic beyond what `lwcpreview` needs.

Context response rules:

- Use `runner.CurrentOrg()` for current records.
- Include objects with at least one record first, sorted by name.
- Derive record labels from `Name`, then `Subject`, then `CaseNumber`, then ID.
- Include deterministic platform `User`, `Profile`, and `RecordType` records when present.
- Never infer field behavior from field names beyond display labels for the selector.

- [ ] **Step 5: Test routes**

Run:

```bash
go test ./internal/lwcpreview ./internal/playground -run 'TestLWC|TestServerLWC' -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/lwcpreview
ok  	github.com/glade-sh/glade/internal/playground
```

Commit:

```bash
git add internal/lwcpreview internal/playground
git commit -m "feat(playground): persist lwc preview scenarios"
```

### Task 4: Add local Apex invocation endpoint

**Files:**

- Modify: `internal/playground/runner.go`
- Modify: `internal/playground/lwc_preview.go`
- Modify: `internal/playground/lwc_preview_test.go`
- Modify: `internal/vm/ui_invocation.go` only if error shape needs a small extension.

- [ ] **Step 1: Write endpoint tests**

Create a temp project with:

```apex
public with sharing class AccountPreviewController {
  @AuraEnabled(cacheable=true)
  public static Map<String, Object> summary(String recordId) {
    Account account = [
      SELECT Id, Name
      FROM Account
      WHERE Id = :recordId
      LIMIT 1
    ];
    return new Map<String, Object>{ 'id' => account.Id, 'name' => account.Name };
  }
}
```

Seed:

```json
{
  "version": "glade.storage.v0",
  "objects": [
    {
      "name": "Account",
      "records": [
        {
          "id": "001000000000001",
          "fields": {
            "Name": { "kind": "string", "string": "Twin Lakes Supply" }
          }
        }
      ]
    }
  ]
}
```

Post to `/playground/api/lwc/invoke` and assert:

- HTTP 200.
- `success=true`.
- `returnValue.name == "Twin Lakes Supply"`.
- `framework == "lwc"`.

- [ ] **Step 2: Add Runner method**

Add this method to `internal/playground/runner.go`:

```go
func (r *Runner) InvokeLWC(ctx context.Context, className, methodName string, params map[string]any, mode RunMode, limitMode vm.LimitMode) (vm.UIInvocationResult, error)
```

Implementation rules:

- Lock `r.mu`.
- Compute `runtimeSourceHash`.
- Load runtime template.
- Merge current store org with template org.
- Clone runtime and attach `runOrg`.
- Set context, trace, and limit mode.
- Call `machine.InvokeLWCMethod(className, methodName, params)`.
- Persist org only when `mode == RunModePersist`.
- Update `lastOrg` with the run org so the database browser reflects scratch calls.

- [ ] **Step 3: Add handler**

Request struct:

```go
type lwcInvokeRequest struct {
	ClassName  string         `json:"className"`
	MethodName string         `json:"methodName"`
	Params     map[string]any `json:"params"`
	Scenario   lwcpreview.Scenario `json:"scenario,omitempty"`
	Mode       RunMode        `json:"mode"`
	LimitMode  vm.LimitMode   `json:"limitMode"`
}
```

Handler rules:

- Public mode returns 403.
- Empty `mode` defaults to scratch.
- Empty `limitMode` defaults to effective playground mode.
- If `Scenario.RecordID` is set, merge it into params as `recordId` only when the caller did not already provide `recordId`.
- If `Scenario.ObjectAPIName` is set, merge it into params as `objectApiName` only when the caller did not already provide `objectApiName`.
- Use scenario user, locale, timezone, and form factor for Glade runtime shims even when Apex receives no matching method parameter.
- Context deadline uses `s.runTimeout`.
- Write the `vm.UIInvocationResult` JSON on success.

- [ ] **Step 4: Run tests**

Run:

```bash
go test ./internal/playground ./internal/vm -run 'TestServerLWCInvoke|Test.*LWC' -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/playground
ok  	github.com/glade-sh/glade/internal/vm
```

Commit:

```bash
git add internal/playground internal/vm
git commit -m "feat(playground): invoke lwc apex locally"
```

### Task 5: Add LWC compile sidecar with real base components

**Files:**

- Create: `internal/lwcpreview/build.go`
- Create: `internal/lwcpreview/build_test.go`
- Create: `internal/playground/lwc-runtime/package.json`
- Create: `internal/playground/lwc-runtime/package-lock.json`
- Create: `internal/playground/lwc-runtime/build.mjs`
- Create: `internal/playground/lwc-runtime/shims/gladeRuntime.js`
- Create: `internal/playground/lwc-runtime/shims/lightning-data/uiRecordApi.js`
- Create: `internal/playground/lwc-runtime/shims/lightning-data/uiObjectInfoApi.js`
- Create: `internal/playground/lwc-runtime/shims/lightning-data/navigation.js`
- Create: `internal/playground/lwc-runtime/shims/lightning-data/platformShowToastEvent.js`
- Modify: `internal/playground/lwc_preview.go`
- Modify: `internal/playground/lwc_preview_test.go`

- [ ] **Step 1: Pin runtime dependencies**

In `internal/playground/lwc-runtime/package.json`:

```json
{
  "name": "@glade/lwc-preview-runtime",
  "private": true,
  "type": "module",
  "scripts": {
    "build": "node build.mjs"
  },
  "dependencies": {
    "@lwc/compiler": "9.3.4",
    "@lwc/engine-dom": "9.3.4",
    "@lwc/module-resolver": "9.3.4",
    "@lwc/synthetic-shadow": "9.3.4",
    "@lwc/wire-service": "9.3.4",
    "lightning-base-components": "1.28.19-alpha",
    "@salesforce-ux/design-system": "2.30.4"
  }
}
```

Run:

```bash
cd internal/playground/lwc-runtime
npm install
```

The package versions above were current on 2026-06-12. `lightning-base-components` is still published with an `alpha` version suffix, but it is the Salesforce-published package for base Lightning components outside the platform. If npm reports a newer stable version during implementation, update all `@lwc/*` packages together and keep the lockfile exact. Do not float these ranges.

- [ ] **Step 2: Write build-wrapper tests**

In `build_test.go`, test:

- Cache key changes when bundle source changes.
- Missing Node returns: `LWC preview requires Node.js and npm dependencies. Run npm install in internal/playground/lwc-runtime.`
- Build command receives project root, cache root, catalog JSON, and scenario JSON paths.

Use a fake command runner interface so the test does not require Node.

- [ ] **Step 3: Implement Go build wrapper**

Add:

```go
type BuildRequest struct {
	ProjectRoot string
	DataRoot    string
	Catalog     Catalog
	Scenario    Scenario
}

type BuildResult struct {
	CacheKey    string       `json:"cacheKey"`
	Entry       string       `json:"entry"`
	CSS         []string     `json:"css,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}
```

Run `node internal/playground/lwc-runtime/build.mjs --project <root> --out <cache> --catalog <json> --scenario <json>`.

- [ ] **Step 4: Implement `build.mjs` with package-backed `lightning/*` resolution**

Compiler behavior:

- Read catalog and scenario.
- Import the selected scenario bundle by bundle name.
- Generate `entry.js` that imports `@lwc/engine-dom`, registers runtime globals, creates one focused component instance, and assigns scenario public properties before insertion.
- Load `node_modules/lightning-base-components/package.json`, read its `lwc.modules` map, and add those modules to the compiler module resolver.
- Resolve normal `lightning/*` imports from `lightning-base-components/src/lightning/*` instead of Glade-owned stubs.
- Import `@lwc/synthetic-shadow` before creating components. The `lightning-base-components` README says synthetic shadow is required.
- Rewrite only the Salesforce platform imports that Glade must own:
  - `@salesforce/apex/Class.method`
  - `@salesforce/label/name`
  - `@salesforce/resourceUrl/name`
  - `@salesforce/schema/Object.Field`
  - `@salesforce/client/formFactor`
  - `@salesforce/user/Id`
  - `@salesforce/i18n/*`
  - `lightning/uiRecordApi`
  - `lightning/uiObjectInfoApi`
  - `lightning/navigation`
  - `lightning/platformShowToastEvent`
- Copy `@salesforce-ux/design-system/assets/styles/salesforce-lightning-design-system.min.css` into the output as `slds.css`.
- Emit diagnostics as JSON rather than raw stderr.

- [ ] **Step 5: Add a resolver smoke test for base components**

Create a Node-side fixture in `internal/playground/lwc-runtime/testdata/base-component-smoke` with:

```html
<template>
  <lightning-card title="Smoke">
    <lightning-button label="Tap" onclick={handleTap}></lightning-button>
    <lightning-input label="Name" value={name} onchange={handleName}></lightning-input>
    <lightning-formatted-text value={name}></lightning-formatted-text>
  </lightning-card>
</template>
```

```js
import { LightningElement } from 'lwc';

export default class BaseComponentSmoke extends LightningElement {
  name = 'Twin Lakes';

  handleTap() {
    this.name = 'Button tapped';
  }

  handleName(event) {
    this.name = event.detail.value;
  }
}
```

The smoke command should compile this fixture and assert the generated module contains imports resolved from `lightning-base-components`, not from `internal/playground/lwc-runtime/shims/lightning`.

- [ ] **Step 6: Add Glade-owned shims only for platform data modules**

Implement:

- `internal/playground/lwc-runtime/shims/lightning-data/uiRecordApi.js`
- `internal/playground/lwc-runtime/shims/lightning-data/uiObjectInfoApi.js`
- `internal/playground/lwc-runtime/shims/lightning-data/navigation.js`
- `internal/playground/lwc-runtime/shims/lightning-data/platformShowToastEvent.js`

Rules:

- `getRecord` reads through `/playground/api/lwc/ui-record`.
- `getObjectInfo` reads the current database/schema snapshot.
- `getPicklistValues` reads local schema value sets when available and returns an empty value list with a diagnostic when unavailable.
- `refreshApex` reruns the matching Apex or UI API adapter and posts a call-log entry.
- `NavigationMixin` records navigation requests in the preview diagnostics panel.
- `ShowToastEvent` posts a toast message to the parent preview shell.
- Unsupported exports throw an `Error` with the module and export name.

- [ ] **Step 7: Add build endpoint**

Route:

```go
case r.Method == http.MethodPost && r.URL.Path == "/playground/api/lwc/build":
	s.handleLWCBuild(w, r)
case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/playground/lwc/assets/"):
	s.serveLWCAsset(w, r)
```

Asset serving rules:

- Resolve only inside `.glade/playground/lwc-cache`.
- Reject `..`, absolute paths, and backslashes.
- Set `Content-Type` from extension.
- Set `Cache-Control: no-cache`.

- [ ] **Step 8: Run tests**

Run:

```bash
go test ./internal/lwcpreview ./internal/playground -count=1
cd internal/playground/lwc-runtime && npm test --if-present
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/lwcpreview
ok  	github.com/glade-sh/glade/internal/playground
```

Commit:

```bash
git add internal/lwcpreview internal/playground/lwc-runtime internal/playground
git commit -m "feat(playground): compile local lwc preview modules"
```

### Task 6: Import data-backed base component source

**Files:**

- Create: `internal/playground/lwc-runtime/contracts/lightning-demo-contracts.md`
- Create: `internal/playground/lwc-runtime/contracts/lds-contract.json`
- Create: `internal/playground/lwc-runtime/contracts/record-form-fixtures.json`
- Create: `internal/playground/lwc-runtime/vendor/lightning-demo/README.md`
- Create: `internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/uiRecordApi/`
- Create: `internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/uiObjectInfoApi/`
- Create: `internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/recordForm/`
- Create: `internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/recordEditForm/`
- Create: `internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/recordViewForm/`
- Create: `internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/inputField/`
- Create: `internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/outputField/`
- Create: `internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/recordEditUtils/`
- Create: `internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/fieldUtils/`
- Create: `internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/recordUtils/`
- Create: `internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/fieldDependencyManager/`
- Create: `internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/inputUtils/`
- Create: `internal/playground/lwc-runtime/shims/force/lds.js`
- Create: `internal/playground/lwc-runtime/shims/force/lds.test.mjs`
- Modify: `internal/playground/lwc-runtime/build.mjs`
- Modify: `internal/playground/lwc-runtime/package.json`
- Modify: `internal/playground/lwc_preview.go`
- Modify: `internal/playground/lwc_preview_test.go`

- [ ] **Step 1: Record reference corpus facts**

Write `internal/playground/lwc-runtime/contracts/lightning-demo-contracts.md` with these facts and paths:

```markdown
# lightning-demo Reference Contracts

Source: https://github.com/jerry-wang12/lightning-demo
Checked: 2026-06-12
Commit: 2f1c6ea4078fd584aea245256073be086d743650
Status: project-approved source for copied code and tests.
Import rule: copy selected files from this commit, then apply mechanical formatting only.

Useful paths:
- `src/lightning/uiRecordApi/uiRecordApi.js`
- `src/lightning/uiObjectInfoApi/uiObjectInfoApi.js`
- `src/lightning/recordForm/recordForm.js`
- `src/lightning/recordEditForm/recordEditForm.js`
- `src/lightning/recordViewForm/recordViewForm.js`
- `src/lightning/inputField/inputField.js`
- `src/lightning/outputField/outputField.js`
- `src/lightning/recordEditUtils/recordEditUtils.js`
- `src/lightning/fieldUtils/fieldUtils.js`
- `src/lightning/recordUtils/recordUtils.js`
- `src/lightning/fieldDependencyManager/fieldDependencyManager.js`
- `src/lightning/inputUtils/inputUtils.js`

Observed contract:
- `lightning/uiRecordApi` re-exports `getRecord`, `getRecordCreateDefaults`, `updateRecord`, `createRecord`, `deleteRecord`, `generateRecordInputForCreate`, `generateRecordInputForUpdate`, `createRecordInputFilteredByEditedFields`, `getRecordUi`, `getFieldValue`, and `getFieldDisplayValue` from `force/lds`.
- `lightning/uiObjectInfoApi` re-exports `getObjectInfo`, `getPicklistValues`, and `getPicklistValuesByRecordType` from `force/lds`.
- `lightning-record-form`, `lightning-record-edit-form`, and `lightning-record-view-form` consume `getRecordUi` and pass normalized data into `lightning-input-field` and `lightning-output-field`.
```

- [ ] **Step 2: Copy and format selected source**

Create `internal/playground/lwc-runtime/vendor/lightning-demo/README.md`:

```markdown
# lightning-demo Vendor Source

Source: https://github.com/jerry-wang12/lightning-demo
Commit: 2f1c6ea4078fd584aea245256073be086d743650
Imported: 2026-06-12

This directory contains copied source and tests for selected Salesforce base component behavior that is not present in `lightning-base-components`.

Allowed edits:

- Mechanical formatting.
- Import rewrites needed to point at Glade shims.
- Test harness changes needed to run under Glade's local LWC runtime.

Do not change component behavior in the same commit as the import. Add a failing test first for any later behavior fix.
```

Copy these directories from `lightning-demo/src/lightning` into the matching `vendor/lightning-demo/src/lightning` path:

```text
uiRecordApi
uiObjectInfoApi
recordForm
recordEditForm
recordViewForm
inputField
outputField
recordEditUtils
fieldUtils
recordUtils
fieldDependencyManager
inputUtils
```

Run a formatting command from `internal/playground/lwc-runtime` after the copy:

```bash
npm run format:vendor
```

Add this script to `internal/playground/lwc-runtime/package.json` if it does not exist:

```json
{
  "scripts": {
    "format:vendor": "prettier --write \"vendor/lightning-demo/src/lightning/**/*.{js,html,css,json,md}\""
  },
  "devDependencies": {
    "prettier": "latest"
  }
}
```

Expected:

```text
vendor/lightning-demo/src/lightning/uiRecordApi/uiRecordApi.js
vendor/lightning-demo/src/lightning/recordForm/recordForm.js
```

and similar Prettier output for copied files.

- [ ] **Step 3: Write the LDS contract JSON**

Create `internal/playground/lwc-runtime/contracts/lds-contract.json`:

```json
{
  "modules": {
    "force/lds": {
      "exports": [
        "RESPONSES",
        "getRecord",
        "getRecordCreateDefaults",
        "getRecordUi",
        "getObjectInfo",
        "getPicklistValues",
        "getPicklistValuesByRecordType",
        "getFieldValue",
        "getFieldDisplayValue",
        "generateRecordInputForCreate",
        "generateRecordInputForUpdate",
        "createRecordInputFilteredByEditedFields",
        "createRecord",
        "updateRecord",
        "deleteRecord"
      ]
    }
  },
  "recordUiShape": {
    "records": "map of recordId to record",
    "objectInfos": "map of objectApiName to objectInfo",
    "layouts": "map of objectApiName to layout metadata"
  }
}
```

- [ ] **Step 4: Implement `force/lds.js`**

Implement these exports first:

```js
export const RESPONSES = {
  SUCCESS: 'SUCCESS',
  ERROR: 'ERROR'
};

export function getFieldValue(record, field) {
  const fieldName = typeof field === 'string' ? field.split('.').pop() : field?.fieldApiName;
  return record?.fields?.[fieldName]?.value;
}

export function getFieldDisplayValue(record, field) {
  const fieldName = typeof field === 'string' ? field.split('.').pop() : field?.fieldApiName;
  return record?.fields?.[fieldName]?.displayValue ?? record?.fields?.[fieldName]?.value;
}
```

For networked functions, call the existing Glade runtime bridge:

```js
export function getRecord(config) {
  return window.__gladeLwcRuntime.uiRecord({ operation: 'getRecord', ...config });
}

export function getRecordUi(config) {
  return window.__gladeLwcRuntime.uiRecord({ operation: 'getRecordUi', ...config });
}

export function updateRecord(recordInput) {
  return window.__gladeLwcRuntime.uiRecord({ operation: 'updateRecord', ...recordInput });
}
```

Unsupported exports must throw:

```js
throw new Error(`force/lds ${name} is not supported by Glade LWC preview yet`);
```

- [ ] **Step 5: Add shim tests**

In `lds.test.mjs`, test:

- `getFieldValue` reads `record.fields.Name.value`.
- `getFieldDisplayValue` prefers `displayValue`.
- `getRecord` calls `window.__gladeLwcRuntime.uiRecord` with `operation=getRecord`.
- `getRecordUi` calls `operation=getRecordUi`.
- `updateRecord` calls `operation=updateRecord`.
- unsupported exports throw a message containing the export name.

- [ ] **Step 6: Route data-backed modules through `force/lds`**

In `build.mjs`, add module rewrites:

```text
force/lds -> internal/playground/lwc-runtime/shims/force/lds.js
lightning/uiRecordApi -> internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/uiRecordApi/uiRecordApi.js
lightning/uiObjectInfoApi -> internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/uiObjectInfoApi/uiObjectInfoApi.js
lightning/recordForm -> internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/recordForm/recordForm.js
lightning/recordEditForm -> internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/recordEditForm/recordEditForm.js
lightning/recordViewForm -> internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/recordViewForm/recordViewForm.js
lightning/inputField -> internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/inputField/inputField.js
lightning/outputField -> internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/outputField/outputField.js
lightning/recordEditUtils -> internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/recordEditUtils/recordEditUtils.js
lightning/fieldUtils -> internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/fieldUtils/fieldUtils.js
lightning/recordUtils -> internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/recordUtils/recordUtils.js
lightning/fieldDependencyManager -> internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/fieldDependencyManager/fieldDependencyManager.js
lightning/inputUtils -> internal/playground/lwc-runtime/vendor/lightning-demo/src/lightning/inputUtils/inputUtils.js
```

Keep `lightning-base-components` as the primary source for components it already provides. Use the copied `lightning-demo` modules for data-backed components missing from that package.

- [ ] **Step 7: Extend server UI API handlers**

Add support for:

- `operation=getRecordUi`
- `operation=getRecordCreateDefaults`
- `operation=createRecord`
- `operation=deleteRecord`

Minimum MVP behavior:

- `getRecordUi` returns `records`, `objectInfos`, and a generated layout containing requested fields or current object fields.
- `getRecordCreateDefaults` returns object info and empty default field values for create mode.
- `createRecord` inserts a local record through storage and returns `{ id, fields }`.
- `deleteRecord` deletes or marks the local record deleted according to existing storage behavior.

- [ ] **Step 8: Run tests**

Run:

```bash
cd internal/playground/lwc-runtime && node --test shims/force/lds.test.mjs
go test ./internal/playground -run 'TestServerLWC.*Record|TestServerLWC.*Object' -count=1
```

Expected:

```text
ok
ok  	github.com/glade-sh/glade/internal/playground
```

Commit:

```bash
git add internal/playground/lwc-runtime internal/playground
git commit -m "feat(playground): add local lds contract for lwc preview"
```

### Task 7: Build the preview UI

**Files:**

- Create: `internal/playground/web/src/lib/lwc-preview.ts`
- Create: `internal/playground/web/src/lib/lwc-preview.test.ts`
- Create: `internal/playground/web/src/components/LWCPreview.tsx`
- Create: `internal/playground/web/src/components/LWCPreview.test.tsx`
- Modify: `internal/playground/web/src/App.tsx`
- Modify: `internal/playground/web/src/index.css`
- Modify: `internal/playground/web/package.json`
- Modify: `internal/playground/web/package-lock.json`

- [ ] **Step 1: Write frontend model tests**

Test these functions:

```ts
export function defaultPropertyValues(bundle: LWCBundle, pageType: string): Record<string, string>
export function defaultScenario(bundle: LWCBundle, context: LWCContext): LWCScenario
export function setScenarioProperty(scenario: LWCScenario, name: string, value: string): LWCScenario
export function setCurrentRecord(scenario: LWCScenario, objectApiName: string, recordId: string): LWCScenario
export function scenarioBuildKey(scenario: LWCScenario): string
```

Expected:

- Defaults are copied from catalog properties.
- Record Page scenario receives `recordId` and `objectApiName`.
- App Page and Home Page scenarios clear record context.
- Property edit updates only the selected scenario.
- Build key changes when component, record context, form factor, or public properties change.

- [ ] **Step 2: Add preview API helper**

In `lwc-preview.ts`, add:

```ts
export async function loadLWCCatalog(): Promise<LWCCatalog>
export async function loadLWCContext(): Promise<LWCContext>
export async function loadLWCScenario(): Promise<LWCScenario>
export async function saveLWCScenario(scenario: LWCScenario): Promise<LWCScenario>
export async function buildLWCPreview(): Promise<LWCBuildResult>
```

Use the existing `api<T>` pattern from `App.tsx`; if needed, move that helper to `internal/playground/web/src/lib/api.ts`.

- [ ] **Step 3: Add `LWCPreview` component**

UI structure:

- Left: component shelf with target filter and search.
- Center: Salesforce-like runner shell with page type, current record selector, object selector, form factor switch, seed/reset buttons, and the live iframe.
- Right: inspector with public property controls, build diagnostics, Apex/UI API call log, toast log, navigation log, and selected database rows.

Clicking a component loads it into the runner. Do not implement drag/drop in the MVP unless all scenario, data, and Apex flows already pass.

- [ ] **Step 4: Add property controls**

Map metadata types:

```text
String -> input text
Boolean -> switch
Integer -> input number step 1
Double/Decimal -> input number step any
Picklist/datasource values -> select
Unsupported -> read-only text with warning badge
```

Do not infer types from property names.

- [ ] **Step 5: Add iframe renderer**

The preview should render compiled LWC in an iframe:

- Isolate SLDS and component CSS from the React playground.
- Inject `slds.css`, generated CSS, `gladeRuntime.js`, and `entry.js`.
- Pass scenario and page context through `postMessage`.
- Receive Apex invocation summaries, UI API reads, refreshes, toasts, navigation requests, and diagnostics through `postMessage`.

- [ ] **Step 6: Wire it into `App.tsx`**

Add a segmented control near the topbar:

```text
Apex Workbench | LWC Preview
```

Keep the current Apex workbench as the default. Store the selected mode in `localStorage` as `glade-playground-mode`.

- [ ] **Step 7: Style the Salesforce shell**

Use SLDS classes inside the iframe. Outside the iframe, keep Glade’s existing playground style.

The iframe shell should include:

- `.slds-scope`
- Lightning record/app header
- One focused component mount point
- Current context strip showing page type, object, record ID, form factor, and data mode
- Empty-state panel when context is missing
- Visible unsupported-import diagnostics

- [ ] **Step 8: Run UI tests**

Run:

```bash
cd internal/playground/web
npm test -- --runInBand
npm run build
```

Expected:

```text
PASS
```

and Vite build succeeds.

Commit:

```bash
git add internal/playground/web
git commit -m "feat(playground): add lwc preview workbench"
```

### Task 8: Add seed recipes and sample dataset support

**Files:**

- Modify: `internal/playground/examples.go`
- Modify: `internal/playground/workspace.go`
- Modify: `internal/playground/server.go`
- Modify: `internal/playground/server_test.go`
- Create: `testdata/playground/lwc-preview-basic/seed.json`
- Create: `testdata/playground/lwc-preview-basic/force-app/main/default/classes/AccountPreviewController.cls`
- Create: `testdata/playground/lwc-preview-basic/force-app/main/default/lwc/accountSummary/accountSummary.js`
- Create: `testdata/playground/lwc-preview-basic/force-app/main/default/lwc/accountSummary/accountSummary.html`
- Create: `testdata/playground/lwc-preview-basic/force-app/main/default/lwc/accountSummary/accountSummary.js-meta.xml`

- [ ] **Step 1: Keep seed model simple**

Do not invent a second database format. Use `storage.Fixture`.

Add one richer built-in example named:

```text
lwc-account-summary-preview
```

It should seed:

- 2 Accounts
- 3 Contacts
- 2 Opportunities
- 1 Case
- 1 User tied to an existing Profile
- 2 RecordType records, one for Account and one for Case

- [ ] **Step 2: Add seed status**

Extend the seed response:

```json
{
  "seeded": true,
  "objects": {
    "Account": 2,
    "Contact": 3,
    "Opportunity": 2,
    "Case": 1,
    "User": 1,
    "RecordType": 2
  }
}
```

Keep old clients safe. A response with extra fields must not break existing UI.

- [ ] **Step 3: Add record context selector**

In `LWCPreview`, let the user choose:

- object API name
- record ID from current database snapshot
- user ID from local `User` records
- form factor: Large, Medium, Small
- data mode: scratch or persist

For Record Page previews, pass `recordId` and `objectApiName` as public properties before component insertion. Also expose them through the Glade runtime context so wire adapters, navigation, and Apex shims read the same values.

- [ ] **Step 4: Add docs for seeding**

Document:

```bash
glade playground --project . --db .glade/playground/org.sqlite
```

Then in the UI:

- edit `seed.json`
- press Seed
- open LWC Preview

- [ ] **Step 5: Run tests**

Run:

```bash
go test ./internal/playground ./internal/storage -run 'TestServerSeed|TestFixture|TestLWC' -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/playground
ok  	github.com/glade-sh/glade/internal/storage
```

Commit:

```bash
git add internal/playground internal/storage testdata/playground/lwc-preview-basic
git commit -m "feat(playground): add lwc preview seed example"
```

### Task 9: Add documentation and guardrails

**Files:**

- Modify: `site/docs-src/guide/playground.md`
- Modify: `site/docs-src/guide/cli-reference.md`
- Modify: `README.md` only if the public feature list changes.
- Modify: `scripts/smoke.sh`

- [ ] **Step 1: Document the feature**

Add a section to `site/docs-src/guide/playground.md`:

```markdown
## LWC preview

`glade playground --project . --db .glade/playground/org.sqlite --open`
can run Lightning web components from your local SFDX project with local data
and Salesforce-like context. The preview reads `componentName.js-meta.xml`,
lists local bundles, lets you choose a preview scenario, sets current record
context, edits supported public properties, and invokes static `@AuraEnabled`
Apex through the same local VM used by Apex runs.

The first version supports App, Home, and Record page targets. It does not
emulate Locker, Lightning Web Security, full Lightning Data Service, or the full
Lightning App Builder. It focuses on running a component well with realistic
context before adding page layout editing.
```

- [ ] **Step 2: Document setup**

Include:

```bash
cd internal/playground/lwc-runtime
npm install
cd ../../..
glade playground --project . --db .glade/playground/org.sqlite --open
```

If runtime dependencies are vendored into the release later, remove this setup note in the same commit that changes packaging.

- [ ] **Step 3: Add smoke coverage**

In `scripts/smoke.sh`, add a local-only smoke that:

- starts playground with a temp LWC fixture project
- calls `/playground/api/lwc/catalog`
- asserts one exposed bundle
- calls `/playground/api/lwc/context`
- asserts an Account record context
- posts `/playground/api/lwc/invoke`
- asserts `success:true`

Do not run a Node build in smoke until release packaging includes runtime dependencies.

- [ ] **Step 4: Run docs and smoke checks**

Run:

```bash
go test ./internal/playground ./internal/lwcpreview -count=1
scripts/smoke.sh
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/playground
ok  	github.com/glade-sh/glade/internal/lwcpreview
```

Smoke should exit 0.

Commit:

```bash
git add site/docs-src/guide/playground.md site/docs-src/guide/cli-reference.md README.md scripts/smoke.sh
git commit -m "docs(playground): describe local lwc preview"
```

## Alternative Designs Considered

### Option A: Extend Visualforce local rendering

Rejected for this feature. It pulls in view state, Visualforce postback, component renderers, and `PageReference.getContent`. LWC preview needs none of that to produce value.

### Option B: Use the Salesforce local development server

Not recommended as the core. It would make Glade a wrapper around another tool and would not run Apex against Glade’s local VM and database. It may still serve as a comparison oracle during manual testing.

### Option C: Build a Storybook-style standalone app

Useful later. Not first. A separate app would split the developer flow from `glade playground`, duplicate seed/database controls, and weaken the Apex connection.

## Verification Matrix

Focused Go:

```bash
go test ./internal/project ./internal/uicontroller ./internal/lwcpreview ./internal/playground ./internal/vm -count=1
```

Frontend:

```bash
cd internal/playground/web
npm test
npm run build
```

Runtime sidecar:

```bash
cd internal/playground/lwc-runtime
npm install
node build.mjs --help
```

Smoke:

```bash
scripts/smoke.sh
```

Manual:

```bash
glade playground --project testdata/playground/lwc-preview-basic --db /tmp/glade-lwc-preview.sqlite --open
```

Manual pass criteria:

- Catalog lists `Account Summary`.
- Clicking `Account Summary` opens a focused scenario.
- Scenario bar shows Record Page, Account, `001000000000001`, Large form factor, and scratch mode.
- Heading property changes visible text.
- `recordId` and `objectApiName` arrive as public properties before render.
- `getRecord` returns `Twin Lakes Supply` from the local database.
- Apex wire and imperative calls return local VM data.
- `refreshApex` reruns the backing call after a DML mutation.
- Toast and navigation events appear in the preview call log.
- Database tab reflects DML if the Apex method mutates state.
- Reset clears org state.
- Seed restores sample records.

## Release and Packaging Notes

First release should mark this as local preview. The feature depends on Node-side LWC compilation unless the release build later embeds compiled runtime dependencies. `lightning-base-components` adds about 12.9 MB unpacked at version `1.28.19-alpha`, so release packaging must make an explicit choice: install runtime dependencies on demand, vendor the runtime package in the archive, or disable LWC preview with a clear setup message. If Node or npm dependencies are missing, the UI must show a precise setup command and keep the rest of playground usable.

Do not add plugin dependencies. Compat fixtures and broad support ledgers stay in `glade-tools`. This feature belongs in base `glade` because it is product runtime behavior: local UI preview, local Apex invocation, local org state, and developer workflow.

## Sources

- Salesforce LWC configuration docs: `componentName.js-meta.xml` defines targets and builder properties; `isExposed=false` hides a component from builders; exposed builder use requires `isExposed=true` and at least one target.
- Salesforce LWC Apex docs: LWC imports Apex methods and calls them through `@wire` or imperative JavaScript; limits apply per Apex invocation.
- Salesforce wire docs: wire provisions immutable data streams and supports reactive `$` variables.
- Salesforce SLDS docs: SLDS gives LWC a Lightning Experience look; base components in the `lightning` namespace use SLDS styling.
- Salesforce SLDS version docs: SLDS 2 exists alongside SLDS 1; component styling hooks differ by version.
- Salesforce LWC repository: source code for LWC engine and compiler; latest release observed on 2026-06-12 was `v9.3.4`.
- npm `lightning-base-components`: latest observed on 2026-06-12 was `1.28.19-alpha`; README says SLDS styles must be globally defined and `@lwc/synthetic-shadow` is required; package includes many `lightning-*` components but excludes Salesforce data-backed components such as `lightning-record-form`, `lightning-input-field`, and `lightning-file-upload`.
- GitHub `jerry-wang12/lightning-demo`: project-approved copy source inspected on 2026-06-12 at commit `2f1c6ea4078fd584aea245256073be086d743650`; GitHub metadata showed a public repo with 10 commits, no releases, and last push on 2019-03-26; the clone contained 164 `src/lightning` directories and 153 Lightning component test files.
