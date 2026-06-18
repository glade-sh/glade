# Priority LWC Project Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the local LWC shell fully support the priority project set by closing resource, UI API, form, datatable, Flow, Community, console, message, streaming, Apex, and DX gaps.

**Architecture:** Keep product code generic. Use the priority project set as a private verification corpus only, never as product special cases. Move complex base component behavior into source-backed runtime modules, keep server APIs in `internal/server`, keep shell modeling in `internal/lwcshell`, and keep private corpus verification outside public docs/site language.

**Tech Stack:** Go server/runtime, `internal/lwcshell`, `internal/lwcbrowser`, local LWC runtime modules in `lwcruntime/src`, SLDS 2 assets, Playwright runtime tests, Go unit tests, private corpus smoke commands.

---

## Scope

Priority project set:

- `priority-project-a`
- `priority-project-b`
- `priority-project-c`
- `priority-project-d`
- `priority-project-e`
- `priority-project-f`
- `priority-project-g`
- `priority-project-h`
- `priority-project-i`

Current static audit:

- Four projects have no LWC files in the local corpus: `priority-project-b`, `priority-project-f`, `priority-project-g`, `priority-project-h`.
- Five projects contain LWC bundles: `priority-project-a`, `priority-project-c`, `priority-project-d`, `priority-project-e`, `priority-project-i`.
- No missing `lightning-*` tags or missing `lightning/*` imports appear in this project set.
- The real gaps are behavior: static resource subpaths, datatable fidelity, record forms, UI API, Flow shell, Community shell, console workspace API, utility bar, message service, EMP API, Apex controller execution, and easy local context setup.

## Acceptance Criteria

- `glade dev lwc --project <repo> --port <port>` opens the LWC landing page at `/`.
- Every discovered LWC bundle in the five LWC projects can be selected from the landing page.
- Every supported target type in the five LWC projects has a generated context route: app page, record page, home page, tab, record action, utility bar, Community page, Flow screen, Flow action, and URL addressable.
- The browser console has no uncaught import errors, missing static-resource 404s, or shell service crashes on generated routes.
- Project-owned Apex controller imports call the local Apex runtime. Unsupported Apex language/runtime gaps surface as Apex diagnostics with class and method names, not browser module failures.
- `priority-project-i` loads `Fixture_Assets/css/main.css`, `Fixture_Assets/js/chart.min.js`, and image subpaths from local static resources.
- `priority-project-c` Community pages and `lightning-flow` usage render in a Community shell with usable base path, guest/user state, route params, and Flow navigation events.
- `priority-project-d` UI API, datatable, Flow support, and platform resource loader usage render without shell-level failures.
- `priority-project-a` app/tab pages and Apex-backed components render through generated app/tab contexts.
- `priority-project-e` app pages and navigation usage render through generated app contexts.
- Public docs and site describe product capabilities without naming private corpus repos.

## Parallel Squad Layout

- **Squad A: Server Data and Resources** handles Tasks 1, 4, and 9.
- **Squad B: Base Components** handles Tasks 2 and 3.
- **Squad C: Shell Services** handles Tasks 5, 6, 7, and 8.
- **Squad D: DX, Verification, Docs** handles Tasks 0, 10, 11, and 12.

Each squad works in its own worktree. Merge through the main integration worktree only after focused tests pass.

## File Structure

- `internal/project/project.go`: discovers static resource content files and directories under Salesforce package roots.
- `internal/project/project_test.go`: verifies nonstandard package static resource discovery.
- `internal/resource/resource.go`: records directory static resource roots and subpath resolution metadata.
- `internal/resource/resource_test.go`: validates resource URL and metadata behavior.
- `internal/server/static_resource.go`: serves static resource files and nested directory/zip subpaths.
- `internal/server/lightning_shims.go`: serves `@salesforce/resourceUrl/*`, Community, site, user, and Lightning service modules.
- `internal/server/lightning_wire.go`: implements LDS/UI API wire and imperative endpoints.
- `internal/server/lightning_test.go`: tests UI API, resource, Apex, and wire endpoints.
- `internal/server/lwc_shell.go`: resolves shell routes and target contexts.
- `internal/server/lwc_shell_assets.go`: renders the landing page, diagnostics, context presets, and builder controls.
- `internal/server/lwc_shell_test.go`: tests landing routes, static assets, context presets, shell capabilities, and generated route metadata.
- `internal/lwcshell/model.go`: owns shell context structs.
- `internal/lwcshell/context_preset.go`: parses `glade.lwc.json`.
- `internal/lwcshell/workbench.go`: discovers routes and workbench navigation models.
- `internal/lwcshell/resolve.go`: resolves pages, tabs, actions, utility bars, and Community targets.
- `internal/lwcbrowser/base_components.go`: keeps simple generated base components and contracts.
- `internal/lwcbrowser/source_backed_components.go`: maps complex base components to runtime modules.
- `internal/lwcbrowser/salesforce_modules.go`: emits Salesforce module shims.
- `lwcruntime/src/lightning/datatable.mjs`: source-backed datatable implementation.
- `lwcruntime/src/lightning/recordForm.mjs`: source-backed record form implementation.
- `lwcruntime/src/lightning/recordEditForm.mjs`: source-backed record edit form implementation.
- `lwcruntime/src/lightning/recordViewForm.mjs`: source-backed record view form implementation.
- `lwcruntime/src/lightning/inputField.mjs`: source-backed input field implementation.
- `lwcruntime/src/lightning/outputField.mjs`: source-backed output field implementation.
- `lwcruntime/src/lightning/messages.mjs`: form and LDS message rendering.
- `lwcruntime/src/lightning/flow.mjs`: embedded Flow host approximation.
- `lwcruntime/src/shell/workspace-service.mjs`: console tab and utility-bar state.
- `lwcruntime/src/shell/message-service.mjs`: Lightning message service bus.
- `lwcruntime/src/shell/emp-service.mjs`: local EMP event bus.
- `lwcruntime/src/shell/flow-service.mjs`: local Flow state and events.
- `lwcruntime/src/shell/community-service.mjs`: Community shell state, route params, menus, and managed content stubs.
- `lwcruntime/src/shell/workbench-builder.mjs`: landing page context builder.
- `lwcruntime/test/*.test.mjs`: browser/runtime tests.
- `docs/LWC_LOCAL_SHELL.md`: product docs.
- `docs/LWC_SUPPORT.md`: supported and limited capabilities.
- `docs/generated/LWC_SHELL_SUPPORT.md`: generated support map.
- `site/docs-src/guide/lwc-local-shell.md`: public guide.
- `site/docs-src/guide/support-map.md`: public support page.

## Task 0: Baseline Private Corpus Gate

**Files:**
- Create: `scripts/dev/lwc-priority-corpus-audit.mjs`
- Create: `scripts/dev/lwc-priority-corpus-smoke.mjs`
- Create: `docs/superpowers/plans/lwc-local-shell/priority-corpus-gate.md`

- [ ] **Step 1: Add the static audit script**

Create `scripts/dev/lwc-priority-corpus-audit.mjs` with this shape:

```js
#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";

const corpusRoot = process.env.GLADE_LWC_CORPUS || "<local-corpus-root>";
const repos = [
  "priority-project-a",
  "priority-project-b",
  "priority-project-c",
  "priority-project-d",
  "priority-project-e",
  "priority-project-f",
  "priority-project-g",
  "priority-project-h",
  "priority-project-i",
];

function walk(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if ([".git", ".sf", ".sfdx", "node_modules"].includes(entry.name)) continue;
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(full, out);
    else out.push(full);
  }
  return out;
}

function count(files, pattern) {
  return files.filter((file) => pattern.test(file)).length;
}

const result = {};
for (const repo of repos) {
  const root = path.join(corpusRoot, repo);
  const files = fs.existsSync(root) ? walk(root) : [];
  result[repo] = {
    files: files.length,
    lwcFiles: count(files, /\/lwc\/[^/]+\/[^/]+\.(js|html|css|js-meta\.xml)$/),
    apexClasses: count(files, /\.cls$/),
    staticResources: count(files, /\/staticresources\//),
  };
}
console.log(JSON.stringify(result, null, 2));
```

- [ ] **Step 2: Run the static audit**

Run:

```bash
node scripts/dev/lwc-priority-corpus-audit.mjs
```

Expected: JSON with nonzero `lwcFiles` for the five LWC projects and zero `lwcFiles` for the four non-LWC projects.

- [ ] **Step 3: Add the smoke runner shell**

Create `scripts/dev/lwc-priority-corpus-smoke.mjs`:

```js
#!/usr/bin/env node
import { spawn } from "node:child_process";
import path from "node:path";

const corpusRoot = process.env.GLADE_LWC_CORPUS || "<local-corpus-root>";
const projects = ["priority-project-a", "priority-project-c", "priority-project-d", "priority-project-e", "priority-project-i"];
const basePort = Number(process.env.GLADE_LWC_SMOKE_PORT || 18080);

function run(project, port) {
  const cwd = path.join(corpusRoot, project);
  const child = spawn("go", ["run", ".", "dev", "lwc", "--project", cwd, "--port", String(port)], {
    cwd: "/Users/matt/Dev/glade",
    stdio: ["ignore", "pipe", "pipe"],
  });
  return { project, port, child };
}

for (const [index, project] of projects.entries()) {
  const proc = run(project, basePort + index);
  console.log(`${proc.project} http://127.0.0.1:${proc.port}/`);
  setTimeout(() => proc.child.kill("SIGTERM"), 15000);
}
```

- [ ] **Step 4: Commit**

Run:

```bash
git add scripts/dev/lwc-priority-corpus-audit.mjs scripts/dev/lwc-priority-corpus-smoke.mjs docs/superpowers/plans/lwc-local-shell/priority-corpus-gate.md
git commit -m "test: add priority lwc corpus gate"
```

## Task 1: Static Resource Subpath Support

**Files:**
- Modify: `internal/project/project.go`
- Modify: `internal/project/project_test.go`
- Modify: `internal/resource/resource.go`
- Modify: `internal/resource/resource_test.go`
- Modify: `internal/server/static_resource.go`
- Modify: `internal/server/lightning_test.go`

- [ ] **Step 1: Write project discovery tests**

Add a test in `internal/project/project_test.go`:

```go
func TestDiscoverProjectIncludesDirectoryStaticResourceContent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/fixture-app/main/staticresources/Bundle.resource-meta.xml"), "<StaticResource/>")
	writeFile(t, filepath.Join(root, "force-app/fixture-app/main/staticresources/Bundle/css/main.css"), "body{}")

	p, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(p.StaticResourceFiles, func(file string) bool {
		return strings.HasSuffix(filepath.ToSlash(file), "staticresources/Bundle/css/main.css")
	}) {
		t.Fatalf("StaticResourceFiles = %#v, want nested directory content", p.StaticResourceFiles)
	}
}
```

- [ ] **Step 2: Run the failing test**

Run:

```bash
go test ./internal/project -run TestDiscoverProjectIncludesDirectoryStaticResourceContent -count=1
```

Expected: FAIL because directory static resource content is not recorded for subpath lookup.

- [ ] **Step 3: Implement static resource directory discovery**

In `internal/project/project.go`, change static resource vendor handling so files under `staticresources/<ResourceName>/...` are retained as static resource content, while Apex and metadata scanners still ignore vendor files for class/object parsing.

Use this helper shape:

```go
func isStaticResourceContentFile(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i, part := range parts {
		if part != "staticresources" || i >= len(parts)-1 {
			continue
		}
		name := strings.ToLower(parts[len(parts)-1])
		if strings.HasSuffix(name, "-meta.xml") || strings.HasSuffix(name, ".xml") {
			return false
		}
		return true
	}
	return false
}
```

- [ ] **Step 4: Add resource metadata tests**

Add a test in `internal/resource/resource_test.go`:

```go
func TestLoadStaticResourcesKeepsDirectoryContentPath(t *testing.T) {
	root := t.TempDir()
	content := filepath.Join(root, "force-app/fixture-app/main/staticresources/Bundle/css/main.css")
	meta := filepath.Join(root, "force-app/fixture-app/main/staticresources/Bundle.resource-meta.xml")
	writeFile(t, content, "body{}")
	writeFile(t, meta, `<StaticResource xmlns="http://soap.sforce.com/2006/04/metadata"><contentType>text/css</contentType></StaticResource>`)

	p := project.Project{Root: root, StaticResourceFiles: []string{content}, StaticResourceMetas: []string{meta}}
	registry, err := LoadProjectMetadata(p)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := URLForStaticResource(registry, "Bundle", "css/main.css")
	if !ok || got != "/resource/Bundle/css/main.css" {
		t.Fatalf("URLForStaticResource = %q, %t", got, ok)
	}
}
```

- [ ] **Step 5: Implement resource subpath metadata**

In `internal/resource/resource.go`, group content files by resource root:

```go
func staticResourceNameAndSubpath(file string) (name string, subpath string, ok bool) {
	parts := strings.Split(filepath.ToSlash(file), "/")
	for i, part := range parts {
		if part == "staticresources" && i+1 < len(parts) {
			name = strings.TrimSuffix(parts[i+1], ".resource")
			if i+2 < len(parts) {
				subpath = strings.Join(parts[i+2:], "/")
			}
			return name, subpath, name != ""
		}
	}
	return "", "", false
}
```

Add a `Files map[string]string` or equivalent to `storage.StaticResourceMetadata` only if one does not already exist. If adding storage shape is too broad, keep a package-local map during metadata load and use `ContentPath` for directory root plus `URLForStaticResource` for URL creation.

- [ ] **Step 6: Add server route test**

Add a test in `internal/server/lightning_test.go`:

```go
func TestStaticResourceServesDirectorySubpathFromPackageRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/fixture-app/main/staticresources/Fixture_Assets.resource-meta.xml"), "<StaticResource/>")
	writeFile(t, filepath.Join(root, "force-app/fixture-app/main/staticresources/Fixture_Assets/css/main.css"), ".fixture{}")

	srv := newTestServerWithProject(t, root)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/resource/Fixture_Assets/css/main.css", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), ".fixture") {
		t.Fatalf("status/body = %d %q", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 7: Implement server lookup**

In `internal/server/static_resource.go`, update `lookupStaticResource` so `subpath != ""` resolves against metadata content roots and discovered project static resource files. Preserve zip behavior through `visualforce.ResolveStaticResourceFile`.

- [ ] **Step 8: Run tests and commit**

Run:

```bash
go test ./internal/project ./internal/resource ./internal/server -run 'StaticResource|ResourceURL|Lightning' -count=1
git add internal/project/project.go internal/project/project_test.go internal/resource/resource.go internal/resource/resource_test.go internal/server/static_resource.go internal/server/lightning_test.go
git commit -m "fix: serve lwc static resource subpaths"
```

## Task 2: Source-Backed Datatable

**Files:**
- Modify: `internal/lwcbrowser/source_backed_components.go`
- Modify: `internal/lwcbrowser/base_components.go`
- Modify: `internal/lwcbrowser/base_components_test.go`
- Modify: `lwcruntime/src/lightning/datatable.mjs`
- Modify: `lwcruntime/test/base-components.test.mjs`

- [ ] **Step 1: Add datatable contract tests**

In `lwcruntime/test/base-components.test.mjs`, add browser assertions for:

```js
const datatable = createElement("lightning-datatable", { is: Datatable });
datatable.keyField = "Id";
datatable.columns = [
  { label: "Name", fieldName: "Name", type: "text", editable: true, sortable: true },
  { label: "Status", fieldName: "Status__c", type: "badge" },
  { type: "action", typeAttributes: { rowActions: [{ label: "View", name: "view" }] } },
];
datatable.data = [{ Id: "001", Name: "Acme", Status__c: "Ready" }];
datatable.addEventListener("rowaction", (event) => events.push(event.detail));
```

Assert rendered headers, row text, action event detail `{ action: { name: "view" }, row: { Id: "001" } }`, sorted event, selection event, and draft value event.

- [ ] **Step 2: Run the failing runtime test**

Run:

```bash
npm --prefix lwcruntime test -- --grep datatable
```

Expected: FAIL on missing sort, selection, editable cell, or event details.

- [ ] **Step 3: Move datatable to source-backed mapping**

In `internal/lwcbrowser/source_backed_components.go`, add:

```go
"datatable": "datatable",
```

In `internal/lwcbrowser/base_components.go`, keep `datatable` registered as supported, but let the source-backed path win in `serveLightningAPIShim`.

- [ ] **Step 4: Implement datatable runtime**

In `lwcruntime/src/lightning/datatable.mjs`, implement:

```js
import { LightningElement, api, track } from "lwc";

export default class LightningDatatable extends LightningElement {
  @api keyField = "id";
  @api columns = [];
  @api data = [];
  @api draftValues = [];
  @api selectedRows = [];
  @api hideCheckboxColumn = false;
  @api sortedBy = "";
  @api sortedDirection = "asc";
  @track localDraftValues = [];

  @api getSelectedRows() {
    const selected = new Set(this.selectedRows || []);
    return (this.data || []).filter((row) => selected.has(row[this.keyField]));
  }

  handleRowAction(event) {
    const rowIndex = Number(event.currentTarget.dataset.rowIndex);
    const actionIndex = Number(event.currentTarget.dataset.actionIndex);
    const column = this.columns[Number(event.currentTarget.dataset.columnIndex)];
    const action = (column?.typeAttributes?.rowActions || [])[actionIndex];
    this.dispatchEvent(new CustomEvent("rowaction", {
      detail: { action, row: this.data[rowIndex] },
      bubbles: true,
      composed: true,
    }));
  }

  handleSort(event) {
    const fieldName = event.currentTarget.dataset.fieldName;
    const sortedDirection = this.sortedBy === fieldName && this.sortedDirection === "asc" ? "desc" : "asc";
    this.dispatchEvent(new CustomEvent("sort", {
      detail: { fieldName, sortedBy: fieldName, sortDirection: sortedDirection },
      bubbles: true,
      composed: true,
    }));
  }
}
```

Add template support for text, number, currency, date, boolean, url, email, phone, badge, button, action, editable text cells, checkbox selection, and `loadmore`.

- [ ] **Step 5: Run focused tests**

Run:

```bash
go test ./internal/lwcbrowser ./internal/server -run 'Datatable|BaseComponent|LightningShim' -count=1
npm --prefix lwcruntime test -- --grep datatable
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/lwcbrowser/source_backed_components.go internal/lwcbrowser/base_components.go internal/lwcbrowser/base_components_test.go lwcruntime/src/lightning/datatable.mjs lwcruntime/test/base-components.test.mjs
git commit -m "feat: deepen local lightning datatable"
```

## Task 3: Source-Backed Record Forms and Fields

**Files:**
- Modify: `internal/lwcbrowser/source_backed_components.go`
- Modify: `lwcruntime/src/lightning/inputField.mjs`
- Modify: `lwcruntime/src/lightning/outputField.mjs`
- Modify: `lwcruntime/src/lightning/recordForm.mjs`
- Modify: `lwcruntime/src/lightning/recordEditForm.mjs`
- Modify: `lwcruntime/src/lightning/recordViewForm.mjs`
- Modify: `lwcruntime/src/lightning/messages.mjs`
- Create: `lwcruntime/src/lightning/lds-form.mjs`
- Modify: `lwcruntime/test/base-components.test.mjs`

- [ ] **Step 1: Add form tests**

Add runtime tests covering:

```js
const form = createElement("lightning-record-edit-form", { is: RecordEditForm });
form.objectApiName = "Provider__c";
form.recordId = "a01000000000001AAA";
form.addEventListener("submit", (event) => events.push(["submit", event.detail.fields]));
form.addEventListener("success", (event) => events.push(["success", event.detail.id]));
form.addEventListener("error", (event) => events.push(["error", event.detail.message]));
```

Assert `lightning-input-field` supports `reportValidity`, `checkValidity`, `setCustomValidity`, `reset`, `focus`, `blur`, `value`, picklist options, checkbox values, number values, date values, and required field errors.

- [ ] **Step 2: Run failing form tests**

Run:

```bash
npm --prefix lwcruntime test -- --grep 'record form|input field'
```

Expected: FAIL on at least one missing form method, field type, or event detail.

- [ ] **Step 3: Add shared LDS form helper**

Create `lwcruntime/src/lightning/lds-form.mjs`:

```js
export function normalizeFieldName(fieldName) {
  if (fieldName && typeof fieldName === "object") {
    return fieldName.fieldApiName || String(fieldName);
  }
  return String(fieldName || "");
}

export function readRecordValue(record, fieldName) {
  const name = normalizeFieldName(fieldName);
  return record?.fields?.[name]?.value ?? record?.fields?.[name]?.displayValue ?? "";
}

export function collectFormFields(root) {
  return [...root.querySelectorAll("lightning-input-field")].reduce((fields, field) => {
    fields[normalizeFieldName(field.fieldName)] = field.value;
    return fields;
  }, {});
}

export function validityMessage({ required, value, customError }) {
  if (customError) return customError;
  if (required && (value === "" || value === null || value === undefined)) return "Complete this field.";
  return "";
}
```

- [ ] **Step 4: Implement source-backed components**

Add these mappings to `internal/lwcbrowser/source_backed_components.go`:

```go
"inputfield":     "inputField",
"outputfield":    "outputField",
"messages":       "messages",
"recordform":     "recordForm",
"recordeditform": "recordEditForm",
"recordviewform": "recordViewForm",
```

Implement each runtime module with `@api` properties and public methods used by real base components. Keep events named `load`, `submit`, `success`, `error`, and `cancel`.

- [ ] **Step 5: Wire forms to UI API endpoints**

`recordEditForm.mjs` must call:

- `/lightning/wire/getRecord`
- `/lightning/wire/getRecordCreateDefaults`
- `/lightning/wire/createRecord`
- `/lightning/wire/updateRecord`

Use `fetch` with JSON bodies that match existing `lightning/uiRecordApi` shims.

- [ ] **Step 6: Run focused tests**

Run:

```bash
npm --prefix lwcruntime test -- --grep 'record form|input field|messages'
go test ./internal/lwcbrowser ./internal/server -run 'RecordForm|InputField|UIRecord|LightningWire' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

Run:

```bash
git add internal/lwcbrowser/source_backed_components.go lwcruntime/src/lightning/lds-form.mjs lwcruntime/src/lightning/inputField.mjs lwcruntime/src/lightning/outputField.mjs lwcruntime/src/lightning/recordForm.mjs lwcruntime/src/lightning/recordEditForm.mjs lwcruntime/src/lightning/recordViewForm.mjs lwcruntime/src/lightning/messages.mjs lwcruntime/test/base-components.test.mjs
git commit -m "feat: deepen local lightning record forms"
```

## Task 4: UI API and LDS Depth

**Files:**
- Modify: `internal/server/lightning_wire.go`
- Modify: `internal/server/lightning_test.go`
- Modify: `internal/lwcbrowser/wire.go`
- Modify: `internal/lwcbrowser/salesforce_modules.go`
- Modify: `internal/lwcbrowser/salesforce_modules_test.go`

- [ ] **Step 1: Add UI API tests for priority shapes**

Add Go tests that cover:

```go
func TestLightningWireGetRecordIncludesOptionalRelationshipAndDisplayValues(t *testing.T) {}
func TestLightningWireGetObjectInfoIncludesRecordTypesPicklistsAndFieldPermissions(t *testing.T) {}
func TestLightningWireCreateDefaultsUsesSourceLayoutSections(t *testing.T) {}
func TestLightningWireUpdateRecordReturnsLDSRecordShape(t *testing.T) {}
func TestLightningWireRecordPickerSearchFiltersByObjectAndFields(t *testing.T) {}
```

- [ ] **Step 2: Run failing tests**

Run:

```bash
go test ./internal/server -run 'LightningWire(GetRecord|ObjectInfo|CreateDefaults|UpdateRecord|RecordPicker)' -count=1
```

Expected: FAIL on missing payload fields or unsupported record picker search.

- [ ] **Step 3: Extend response shapes**

In `internal/server/lightning_wire.go`, make these payloads closer to UI API:

- `getRecord`: `apiName`, `childRelationships`, `fields`, `id`, `lastModifiedById`, `lastModifiedDate`, `recordTypeId`.
- Field payload: `displayValue`, `value`, `dataType`, `relationshipName`, `referenceToInfos`.
- `getObjectInfo`: `apiName`, `childRelationships`, `fields`, `keyPrefix`, `label`, `labelPlural`, `recordTypeInfos`, `themeInfo`.
- `getRecordCreateDefaults`: layout sections, editable fields, default values, record type ID.
- CRUD results: full record object, not only ID.

- [ ] **Step 4: Add search endpoint for `lightning-record-picker`**

Add endpoint routing in `internal/server/lightning_wire.go`:

```go
case "recordPickerSearch":
	s.handleLightningWireRecordPickerSearch(w, r)
```

Implement request fields:

```go
type WireRecordPickerSearchRequest struct {
	ObjectAPIName string   `json:"objectApiName"`
	SearchTerm    string   `json:"searchTerm"`
	Fields        []string `json:"fields"`
	MatchingFields []string `json:"matchingFields"`
}
```

- [ ] **Step 5: Export record picker fetch in module shim**

In `internal/lwcbrowser/salesforce_modules.go`, add a helper export consumed by `recordPicker.mjs`:

```js
export async function __gladeRecordPickerSearch(config = {}) {
  const response = await fetch("/lightning/wire/recordPickerSearch", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(config),
  });
  return response.json();
}
```

- [ ] **Step 6: Run focused tests and commit**

Run:

```bash
go test ./internal/server ./internal/lwcbrowser -run 'LightningWire|UIRecord|RecordPicker|UIObjectInfo' -count=1
git add internal/server/lightning_wire.go internal/server/lightning_test.go internal/lwcbrowser/wire.go internal/lwcbrowser/salesforce_modules.go internal/lwcbrowser/salesforce_modules_test.go
git commit -m "feat: deepen local ui api support"
```

## Task 5: Console Workspace and Utility Bar Shell

**Files:**
- Modify: `internal/lwcshell/model.go`
- Modify: `internal/lwcshell/flexipage.go`
- Modify: `internal/lwcshell/flexipage_test.go`
- Modify: `internal/lwcshell/workbench.go`
- Modify: `internal/server/lwc_shell.go`
- Modify: `internal/server/lwc_shell_assets.go`
- Modify: `internal/server/lwc_shell_test.go`
- Create: `lwcruntime/src/shell/workspace-service.mjs`
- Modify: `internal/lwcbrowser/salesforce_modules.go`
- Modify: `internal/lwcbrowser/salesforce_modules_test.go`

- [ ] **Step 1: Add utility-bar route tests**

In `internal/lwcshell/flexipage_test.go`, assert `UtilityBar` flexipages load component instances and region names.

In `internal/server/lwc_shell_test.go`, assert `/lwc/preview/utility/<name>` resolves a shell context with utility components.

- [ ] **Step 2: Run failing tests**

Run:

```bash
go test ./internal/lwcshell ./internal/server -run 'UtilityBar|Workspace|Console' -count=1
```

Expected: FAIL where utility-bar routes and workspace state are shallow.

- [ ] **Step 3: Extend shell model**

Add to `internal/lwcshell/model.go`:

```go
const RenderTargetUtilityBar RenderTargetKind = "utilityBar"

type WorkspaceContext struct {
	Console bool            `json:"console,omitempty"`
	FocusedTabID string      `json:"focusedTabId,omitempty"`
	Tabs []WorkspaceTab      `json:"tabs,omitempty"`
	Utilities []UtilityItem  `json:"utilities,omitempty"`
}
```

Add `Workspace WorkspaceContext` to `PageContext`.

- [ ] **Step 4: Implement runtime workspace service**

Create `lwcruntime/src/shell/workspace-service.mjs`:

```js
const state = {
  console: false,
  focusedTabId: "workspace-tab-1",
  tabs: [{ tabId: "workspace-tab-1", label: "Local", url: window.location.href, workspaceTab: true }],
  utilities: [],
};

export function configureWorkspace(next = {}) {
  Object.assign(state, next);
}

export async function getFocusedTabInfo() {
  return state.tabs.find((tab) => tab.tabId === state.focusedTabId) || state.tabs[0];
}

export async function getAllTabInfo() {
  return state.tabs.slice();
}

export async function openTab(options = {}) {
  const tab = { tabId: `workspace-tab-${state.tabs.length + 1}`, label: options.label || options.url || "Tab", url: options.url || "", workspaceTab: true };
  state.tabs.push(tab);
  state.focusedTabId = tab.tabId;
  return tab.tabId;
}
```

Move existing `platformWorkspaceApi` exports to call this service.

- [ ] **Step 5: Render utility chrome**

In `internal/server/lwc_shell_assets.go`, add utility bar markup when `shell.Context.Workspace.Utilities` has entries. Use SLDS utility bar classes and route links to selected utilities.

- [ ] **Step 6: Run focused tests and commit**

Run:

```bash
go test ./internal/lwcshell ./internal/server -run 'UtilityBar|Workspace|Console|LWC' -count=1
npm --prefix lwcruntime test -- --grep workspace
git add internal/lwcshell internal/server/lwc_shell.go internal/server/lwc_shell_assets.go internal/server/lwc_shell_test.go lwcruntime/src/shell/workspace-service.mjs internal/lwcbrowser/salesforce_modules.go internal/lwcbrowser/salesforce_modules_test.go
git commit -m "feat: add local console workspace shell"
```

## Task 6: Message Service and EMP API

**Files:**
- Modify: `lwcruntime/src/shell/message-service.mjs`
- Create: `lwcruntime/src/shell/emp-service.mjs`
- Modify: `internal/lwcbrowser/salesforce_modules.go`
- Modify: `internal/lwcbrowser/salesforce_modules_test.go`
- Modify: `lwcruntime/test/lwc-navigation-services.test.mjs`

- [ ] **Step 1: Add message and EMP tests**

Add tests that assert:

```js
const context = createMessageContext();
const unsubscribe = subscribe(context, channel, (message) => received.push(message), { scope: APPLICATION_SCOPE });
publish(context, channel, { id: "one" });
unsubscribe();
```

Add EMP tests:

```js
const sub = await subscribe("/event/Local__e", -1, (event) => received.push(event));
__gladePublish("/event/Local__e", { payload: { Name__c: "Probe" }, replayId: 1 });
await unsubscribe(sub);
```

- [ ] **Step 2: Run failing tests**

Run:

```bash
npm --prefix lwcruntime test -- --grep 'message service|emp api'
```

Expected: FAIL on scoped channels, unsubscribe behavior, replay handling, or service state.

- [ ] **Step 3: Implement scoped LMS**

In `lwcruntime/src/shell/message-service.mjs`, model:

- context IDs
- channel names from `messageChannelName`
- `APPLICATION_SCOPE`
- component scope default
- subscribe/unsubscribe cleanup
- publish ordering

- [ ] **Step 4: Implement EMP bus**

Create `lwcruntime/src/shell/emp-service.mjs` with:

```js
const subscriptions = new Map();
const errors = new Set();

export async function subscribe(channel, replayId, callback) {
  const id = `emp-${subscriptions.size + 1}`;
  subscriptions.set(id, { id, channel, replayId, callback });
  return { id, channel, replayId };
}

export async function unsubscribe(subscription, callback) {
  subscriptions.delete(subscription?.id);
  if (typeof callback === "function") callback({ successful: true });
}

export function __gladePublish(channel, payload) {
  for (const sub of subscriptions.values()) {
    if (sub.channel === channel && typeof sub.callback === "function") sub.callback(payload);
  }
}
```

Wire `EmpAPIModuleJS()` to export from this runtime file.

- [ ] **Step 5: Run tests and commit**

Run:

```bash
npm --prefix lwcruntime test -- --grep 'message service|emp api'
go test ./internal/lwcbrowser ./internal/server -run 'MessageService|EmpAPI|LightningShim' -count=1
git add lwcruntime/src/shell/message-service.mjs lwcruntime/src/shell/emp-service.mjs internal/lwcbrowser/salesforce_modules.go internal/lwcbrowser/salesforce_modules_test.go lwcruntime/test/lwc-navigation-services.test.mjs
git commit -m "feat: deepen local messaging services"
```

## Task 7: Flow Shell

**Files:**
- Modify: `internal/lwcshell/model.go`
- Modify: `internal/lwcshell/context_preset.go`
- Modify: `internal/lwcshell/context_preset_test.go`
- Modify: `internal/lwcshell/resolve.go`
- Modify: `internal/server/lwc_shell.go`
- Modify: `internal/server/lwc_shell_assets.go`
- Modify: `internal/server/lwc_shell_test.go`
- Modify: `lwcruntime/src/lightning/flow.mjs`
- Create: `lwcruntime/src/shell/flow-service.mjs`
- Modify: `internal/lwcbrowser/salesforce_modules.go`

- [ ] **Step 1: Add Flow context tests**

Add context preset tests:

```go
preset := ContextPreset{
	Target: "flowScreen",
	Component: "c:communityFlow2",
	Flow: FlowContext{APIName: "Membership_Flow", InputVariables: map[string]any{"recordId": "001000000000001AAA"}},
}
```

Assert target becomes `RenderTargetFlowScreen` and Flow input variables survive JSON parsing.

- [ ] **Step 2: Run failing tests**

Run:

```bash
go test ./internal/lwcshell ./internal/server -run 'Flow' -count=1
```

Expected: FAIL on missing Flow target or missing Flow context fields.

- [ ] **Step 3: Extend model**

Add:

```go
const RenderTargetFlowScreen RenderTargetKind = "flowScreen"

type FlowContext struct {
	APIName string `json:"apiName,omitempty"`
	InputVariables map[string]any `json:"inputVariables,omitempty"`
	AvailableActions []string `json:"availableActions,omitempty"`
}
```

Add `Flow FlowContext` to `PageContext` and `ContextPreset`.

- [ ] **Step 4: Implement Flow service**

Create `lwcruntime/src/shell/flow-service.mjs`:

```js
export function readFlowContext() {
  const node = document.getElementById("glade-lwc-context");
  if (!node) return {};
  try {
    return JSON.parse(node.textContent || "{}").flow || {};
  } catch (_err) {
    return {};
  }
}

export function dispatchStatusChange(element, detail) {
  element.dispatchEvent(new CustomEvent("statuschange", { detail, bubbles: true, composed: true }));
}
```

- [ ] **Step 5: Update `lightning-flow`**

In `lwcruntime/src/lightning/flow.mjs`, support:

- `@api flowApiName`
- `@api flowInputVariables`
- `@api startFlow(flowApiName, inputVariables)`
- statuschange events with `FINISHED`, `FINISHED_SCREEN`, and `ERROR`
- visible input/output variable table in local shell

- [ ] **Step 6: Capture Flow navigation events**

In shell boot code, listen for:

- `flowattributechange`
- `flownavigationnext`
- `flownavigationback`
- `flownavigationpause`
- `flownavigationfinish`

Record events in diagnostics/context panel so Flow components can be tested.

- [ ] **Step 7: Run tests and commit**

Run:

```bash
go test ./internal/lwcshell ./internal/server -run 'Flow|LWC' -count=1
npm --prefix lwcruntime test -- --grep flow
git add internal/lwcshell internal/server/lwc_shell.go internal/server/lwc_shell_assets.go internal/server/lwc_shell_test.go lwcruntime/src/lightning/flow.mjs lwcruntime/src/shell/flow-service.mjs internal/lwcbrowser/salesforce_modules.go
git commit -m "feat: add local lwc flow shell"
```

## Task 8: Community Shell Depth

**Files:**
- Modify: `internal/lwcshell/model.go`
- Modify: `internal/lwcshell/context_preset.go`
- Modify: `internal/lwcshell/workbench.go`
- Modify: `internal/server/lwc_shell.go`
- Modify: `internal/server/lwc_shell_assets.go`
- Modify: `internal/server/lwc_shell_test.go`
- Create: `lwcruntime/src/shell/community-service.mjs`
- Modify: `lwcruntime/src/shell/community-host.mjs`
- Modify: `lwcruntime/src/shell/navigation-service.mjs`
- Modify: `lwcruntime/src/shims/community.mjs`

- [ ] **Step 1: Add Community behavior tests**

Add tests for:

- `@salesforce/community/basePath`
- `@salesforce/community/Id`
- `@salesforce/site/Id`
- guest state
- language
- route params
- `comm__namedPage`
- `comm__recordPage`
- `comm__managedContentPage`

- [ ] **Step 2: Run failing tests**

Run:

```bash
go test ./internal/lwcshell ./internal/server -run 'Community' -count=1
npm --prefix lwcruntime test -- --grep community
```

Expected: FAIL on route params, managed content, or incomplete context state.

- [ ] **Step 3: Extend Community context**

Add fields to `CommunityContext`:

```go
RouteParams map[string]string `json:"routeParams,omitempty"`
Menus map[string][]CommunityMenuItem `json:"menus,omitempty"`
ManagedContent map[string]CommunityContentItem `json:"managedContent,omitempty"`
```

Add `CommunityMenuItem` and `CommunityContentItem` structs with label, target, type, content key, title, body, and image URL fields.

- [ ] **Step 4: Implement Community runtime service**

Create `lwcruntime/src/shell/community-service.mjs`:

```js
export function readCommunityShell() {
  const node = document.getElementById("glade-lwc-context");
  if (!node) return {};
  try {
    return JSON.parse(node.textContent || "{}").community || {};
  } catch (_err) {
    return {};
  }
}

export function readRouteParam(name, fallback = "") {
  return readCommunityShell().routeParams?.[name] || fallback;
}

export function readManagedContent(key) {
  return readCommunityShell().managedContent?.[key] || null;
}
```

- [ ] **Step 5: Wire navigation and shims**

Update `navigation-service.mjs` to preserve Community site, base path, route params, record ID, object API name, and managed content key in local URLs.

Update `community.mjs` to read the new service and keep current diagnostics for missing IDs.

- [ ] **Step 6: Render Community chrome**

In `lwc_shell_assets.go`, render a Community shell header and menu when Community context exists. Use route links to local Community routes.

- [ ] **Step 7: Run tests and commit**

Run:

```bash
go test ./internal/lwcshell ./internal/server -run 'Community|LWC' -count=1
npm --prefix lwcruntime test -- --grep community
git add internal/lwcshell internal/server/lwc_shell.go internal/server/lwc_shell_assets.go internal/server/lwc_shell_test.go lwcruntime/src/shell/community-service.mjs lwcruntime/src/shell/community-host.mjs lwcruntime/src/shell/navigation-service.mjs lwcruntime/src/shims/community.mjs
git commit -m "feat: deepen local community shell"
```

## Task 9: Apex Controller Readiness Loop

**Files:**
- Modify: `internal/server/lightning_wire.go`
- Modify: `internal/server/lightning_test.go`
- Modify: `internal/vm` package files as failures identify real Apex runtime gaps.
- Create: `scripts/dev/lwc-apex-import-inventory.mjs`

- [ ] **Step 1: Add Apex import inventory**

Create `scripts/dev/lwc-apex-import-inventory.mjs`:

```js
#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";

const root = process.argv[2];
if (!root) throw new Error("usage: node scripts/dev/lwc-apex-import-inventory.mjs <project>");

function walk(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if ([".git", ".sf", ".sfdx", "node_modules"].includes(entry.name)) continue;
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(full, out);
    else out.push(full);
  }
  return out;
}

const imports = new Map();
for (const file of walk(root).filter((file) => /\/lwc\/.*\.js$/.test(file))) {
  const text = fs.readFileSync(file, "utf8");
  for (const match of text.matchAll(/@salesforce\/apex\/([A-Za-z0-9_]+)\.([A-Za-z0-9_]+)/g)) {
    const key = `${match[1]}.${match[2]}`;
    imports.set(key, (imports.get(key) || 0) + 1);
  }
}
console.log([...imports.entries()].sort((a, b) => b[1] - a[1]).map(([name, count]) => ({ name, count })));
```

- [ ] **Step 2: Inventory all five LWC projects**

Run:

```bash
for repo in priority-project-a priority-project-c priority-project-d priority-project-e priority-project-i; do
  node scripts/dev/lwc-apex-import-inventory.mjs "<local-corpus-root>/$repo" > "/tmp/$repo.apex-imports.json"
done
```

Expected: JSON files listing Apex controller methods. `priority-project-i` should be the largest.

- [ ] **Step 3: Harden Apex invocation diagnostics**

In `internal/server/lightning_wire.go`, include class, method, params, exception type, and stack trace when local Apex fails. Browser code must receive an LDS-compatible error body.

Expected error shape:

```json
{
  "error": {
    "status": 500,
    "body": {
      "message": "Class.method failed: <message>",
      "exceptionType": "ApexException",
      "stackTrace": "<stack>"
    }
  }
}
```

- [ ] **Step 4: Run controller-focused smoke**

Run:

```bash
go test ./internal/server -run 'LightningApex|WireApex' -count=1
```

Expected: PASS.

- [ ] **Step 5: Fix real VM gaps found by corpus smoke**

For each controller failure:

- Reproduce with `glade exec` or the LWC route that invokes the method.
- Add a focused VM test in the owning `internal/vm` package.
- Implement the missing Apex language/runtime behavior.
- Re-run the single failing test.
- Re-run the LWC route smoke.

- [ ] **Step 6: Commit**

Run:

```bash
git add scripts/dev/lwc-apex-import-inventory.mjs internal/server/lightning_wire.go internal/server/lightning_test.go internal/vm
git commit -m "fix: harden apex-backed lwc controller calls"
```

## Task 10: DX Landing Page, Context Builder, and Generated Presets

**Files:**
- Modify: `internal/server/lwc_shell_assets.go`
- Modify: `internal/server/lwc_shell.go`
- Modify: `internal/server/lwc_shell_test.go`
- Modify: `internal/lwcshell/workbench.go`
- Modify: `internal/lwcshell/workbench_test.go`
- Modify: `lwcruntime/src/shell/workbench-builder.mjs`
- Modify: `lwcruntime/src/shell/glade-shell.css`

- [ ] **Step 1: Add route tests for `/`**

In `internal/server/lwc_shell_test.go`, assert `glade dev lwc` serves the LWC landing page at `/` without requiring `/lwc`.

Expected checks:

```go
rec := httptest.NewRecorder()
srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "data-glade-lwc-workbench") {
	t.Fatalf("landing page missing")
}
```

- [ ] **Step 2: Run failing route test**

Run:

```bash
go test ./internal/server -run 'LWCShell.*Root|LWC.*Landing' -count=1
```

Expected: FAIL if `/` is not the shell landing page for `glade dev lwc`.

- [ ] **Step 3: Generate context presets from metadata**

In `internal/lwcshell/workbench.go`, generate routes for:

- direct component
- app page
- record page with sample record ID
- home page
- tab
- record action
- utility bar
- Community page
- Flow screen
- Flow action
- URL addressable

Each generated route needs a display label, target kind, selected URL, and missing-context diagnostics.

- [ ] **Step 4: Improve builder controls**

In `lwcruntime/src/shell/workbench-builder.mjs`, add:

- component picker
- target picker
- app selector
- object selector
- record ID field with sample button
- Community selector
- Flow input variable editor
- console mode toggle
- form factor segmented control
- state key/value editor

- [ ] **Step 5: Style the shell**

In `lwcruntime/src/shell/glade-shell.css`, use SLDS classes first and add only namespaced `.glade-*` rules. Do not override `.slds-button`, `.slds-badge`, or other SLDS global classes directly.

- [ ] **Step 6: Run tests and commit**

Run:

```bash
go test ./internal/lwcshell ./internal/server -run 'Workbench|LWCShell|Context' -count=1
npm --prefix lwcruntime test -- --grep workbench
git add internal/server/lwc_shell_assets.go internal/server/lwc_shell.go internal/server/lwc_shell_test.go internal/lwcshell/workbench.go internal/lwcshell/workbench_test.go lwcruntime/src/shell/workbench-builder.mjs lwcruntime/src/shell/glade-shell.css
git commit -m "feat: improve local lwc shell dx"
```

## Task 11: Priority Project Browser Verification

**Files:**
- Modify: `scripts/dev/lwc-priority-corpus-smoke.mjs`
- Create: `scripts/dev/lwc-priority-browser-smoke.mjs`
- Create: `docs/superpowers/plans/lwc-local-shell/priority-corpus-results.md`

- [ ] **Step 1: Add browser smoke script**

Create `scripts/dev/lwc-priority-browser-smoke.mjs`:

```js
#!/usr/bin/env node
import { chromium } from "playwright";

const targets = process.argv.slice(2);
if (!targets.length) throw new Error("usage: node scripts/dev/lwc-priority-browser-smoke.mjs <url>...");

const browser = await chromium.launch();
let failed = false;
for (const url of targets) {
  const page = await browser.newPage();
  const messages = [];
  page.on("console", (msg) => {
    if (["error", "warning"].includes(msg.type())) messages.push(`${msg.type()}: ${msg.text()}`);
  });
  const response = await page.goto(url, { waitUntil: "networkidle" });
  const missing = await page.locator("[data-glade-diagnostic-severity='error']").count();
  if (!response?.ok() || messages.some((line) => /GLADELWC081|404|failed to load resource/i.test(line)) || missing > 0) {
    failed = true;
    console.error(JSON.stringify({ url, status: response?.status(), messages, diagnostics: missing }, null, 2));
  }
  await page.close();
}
await browser.close();
process.exit(failed ? 1 : 0);
```

- [ ] **Step 2: Run smoke against five LWC projects**

Start servers:

```bash
node scripts/dev/lwc-priority-corpus-smoke.mjs
```

Open root pages:

```bash
node scripts/dev/lwc-priority-browser-smoke.mjs \
  http://127.0.0.1:18080/ \
  http://127.0.0.1:18081/ \
  http://127.0.0.1:18082/ \
  http://127.0.0.1:18083/ \
  http://127.0.0.1:18084/
```

Expected: PASS on root landing pages.

- [ ] **Step 3: Run generated route smoke**

For each running server, scrape links from `[data-glade-route-link]` and run the browser smoke over each route.

Expected: no import errors, no missing static resources, no shell service crashes.

- [ ] **Step 4: Record results**

Write `docs/superpowers/plans/lwc-local-shell/priority-corpus-results.md` with:

- date
- git commit
- project
- bundle count
- route count
- failures
- fixed issue links or commit hashes

- [ ] **Step 5: Commit**

Run:

```bash
git add scripts/dev/lwc-priority-corpus-smoke.mjs scripts/dev/lwc-priority-browser-smoke.mjs docs/superpowers/plans/lwc-local-shell/priority-corpus-results.md
git commit -m "test: add priority lwc browser smoke"
```

## Task 12: Docs, Site, and Public Boundary

**Files:**
- Modify: `docs/LWC_LOCAL_SHELL.md`
- Modify: `docs/LWC_SUPPORT.md`
- Modify: `docs/generated/LWC_SHELL_SUPPORT.md`
- Modify: `site/docs-src/guide/lwc-local-shell.md`
- Modify: `site/docs-src/guide/support-map.md`
- Modify: `docs/COMPATIBILITY.md`

- [ ] **Step 1: Update support docs**

Document these as supported:

- LWC landing page at `/`
- app pages
- record pages
- home pages
- tabs
- URL addressable components
- record actions
- utility bar
- Community pages
- Flow screen contexts
- local Apex controller calls
- LDS/UI API record and object APIs
- datatable, record forms, input fields
- message service
- EMP API local event bus
- console workspace API local tab model
- static resource subpaths

- [ ] **Step 2: Document limits**

Document limits in product terms:

- Local Flow shell is not Salesforce Flow Builder.
- Local EMP API is deterministic in-page event simulation.
- Local workspace API models console state for development.
- Apex behavior depends on current Glade Apex runtime coverage.
- Private customer/project corpus names do not appear in public docs.

- [ ] **Step 3: Run docs checks**

Run:

```bash
rg -n 'private-corpus-markers' docs site/docs-src
```

Expected: no matches outside internal superpowers plan files.

- [ ] **Step 4: Run tests**

Run:

```bash
go test ./internal/lwcbrowser ./internal/lwcshell ./internal/server ./internal/project ./internal/resource
npm --prefix lwcruntime test
git diff --check
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add docs/LWC_LOCAL_SHELL.md docs/LWC_SUPPORT.md docs/generated/LWC_SHELL_SUPPORT.md site/docs-src/guide/lwc-local-shell.md site/docs-src/guide/support-map.md docs/COMPATIBILITY.md
git commit -m "docs: update local lwc shell support"
```

## Final Integration Gate

- [ ] **Step 1: Run Go focused tests**

```bash
go test ./internal/lwcbrowser ./internal/lwcshell ./internal/server ./internal/project ./internal/resource
```

- [ ] **Step 2: Run LWC runtime tests**

```bash
npm --prefix lwcruntime test
```

- [ ] **Step 3: Run broad build**

```bash
go test ./...
```

- [ ] **Step 4: Run priority browser smoke**

```bash
node scripts/dev/lwc-priority-corpus-audit.mjs
node scripts/dev/lwc-priority-corpus-smoke.mjs
node scripts/dev/lwc-priority-browser-smoke.mjs \
  http://127.0.0.1:18080/ \
  http://127.0.0.1:18081/ \
  http://127.0.0.1:18082/ \
  http://127.0.0.1:18083/ \
  http://127.0.0.1:18084/
```

- [ ] **Step 5: Run whitespace and public-boundary checks**

```bash
git diff --check
rg -n 'private-corpus-markers' docs site/docs-src --glob '!docs/superpowers/plans/**'
```

Expected: no output from `git diff --check`; no public docs/site matches for private corpus names.

## Project-by-Project Exit Criteria

### `priority-project-a`

- App page route loads.
- Tab route loads.
- Navigation calls produce local URLs.
- All 8 unique Apex methods either return data or show Apex-runtime diagnostics with class/method names.

### `priority-project-b`

- Static audit reports zero LWC bundles.
- No LWC support work remains for this project.

### `priority-project-c`

- Community pages load under `/lwc/preview/community/<site>/<page>`.
- `@salesforce/community/basePath`, `@salesforce/user/Id`, and resource URLs resolve.
- `lightning-flow` tags render a local Flow host.
- Flow navigation events appear in diagnostics.
- Product grid/product detail Apex calls reach local Apex runtime.

### `priority-project-d`

- Record pages, app pages, home pages, Flow screens, Flow action, and Community pages have generated routes.
- `lightning/uiRecordApi` and `lightning/uiObjectInfoApi` calls return LDS-shaped data.
- `PackageAmsAnimations.css` and `NPSTodayPopupBundle.zip` subpaths serve where referenced.
- Datatables render row actions and editable cells.

### `priority-project-e`

- App page routes load.
- Navigation calls produce local URLs.
- All Apex imports reach local Apex runtime.

### `priority-project-f`

- Static audit reports zero LWC bundles.
- No LWC support work remains for this project.

### `priority-project-g`

- Static audit reports zero LWC bundles.
- No LWC support work remains for this project.

### `priority-project-h`

- Static audit reports zero LWC bundles.
- No LWC support work remains for this project.

### `priority-project-i`

- Landing page lists all 253 LWC bundles.
- Record pages, record actions, tabs, app pages, home pages, URL addressable routes, and utility-bar routes load.
- `Fixture_Assets/css/main.css`, `Fixture_Assets/js/chart.min.js`, SVG images, `fixtureLogo`, and `fixtureIcon` serve without 404.
- Record forms and input fields render with LDS data, validation, reset, and save behavior.
- Datatables render custom columns, row actions, selection, inline draft values, and sorted events.
- `messageService`, `empApi`, `platformWorkspaceApi`, and `platformResourceLoader` calls run without shell-level crashes.
- Apex controller calls reach local Apex runtime and surface useful diagnostics for any remaining VM gaps.

## Self-Review

- Spec coverage: every measured gap has a task: resources Task 1, datatable Task 2, record forms Task 3, UI API Task 4, workspace/utility Task 5, messaging/EMP Task 6, Flow Task 7, Community Task 8, Apex Task 9, DX Task 10, verification Task 11, docs Task 12.
- Placeholder scan: this plan contains no deferred implementation slots.
- Type consistency: shell contexts stay in `internal/lwcshell`; browser modules stay in `lwcruntime/src`; server endpoints stay in `internal/server`.
