# Full Visualforce Rendering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring Glade from scaffold-level local Visualforce rendering to a full product-supported Visualforce renderer: GET render, POST lifecycle, controller and extension binding, view state, standard and custom components, partial page refresh, Lightning Out/LWC embedding, static resources, `PageReference.getContent()`, and PDF rendering.

**Architecture:** Keep Visualforce rendering in `internal/visualforce` and request integration in `internal/server`. Build a table-driven component registry, a stronger Visualforce expression evaluator, a lifecycle engine that reuses the Apex VM, and browser/runtime support only where Visualforce requires browser behavior. Keep parity capture and large corpus bookkeeping in first-party plugins sourced from `/Users/matt/Dev/glade-tools`; this repo must not depend on glade-tools.

**Tech Stack:** Go 1.26, `golang.org/x/net/html`, existing `internal/vm`, existing `internal/server`, existing `internal/lwcbrowser`, existing `lwcruntime` Node test lane, optional local Chromium/Playwright toolchain for PDF proof.

---

## Research Findings

Salesforce defines a Visualforce page as markup plus a controller. The markup lives inside one `<apex:page>` tag, and controllers provide the behavior behind user interaction and displayed data. See the latest [Visualforce Developer Guide PDF](https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/salesforce_pages_developers_guide.pdf) and [What is Visualforce?](https://developer.salesforce.com/docs/atlas.en-us.pages.meta/pages/pages_intro_what_is_it.htm).

Full rendering is not one feature. It is a request lifecycle:

- GET `/apex/Page`: resolve page metadata, parse markup, instantiate controllers, execute page action when present, evaluate expressions, render component tree, write view state, return HTML.
- POST `/apex/Page`: decode view state, restore controller and component state, apply submitted values through setters, dispatch the bound action, render full page or partial `reRender` regions, write fresh view state.
- `PageReference.getContent()`: render the referenced page and return a Blob. Salesforce documents this as content dependent on the page render mode. See [PageReference](https://developer.salesforce.com/docs/atlas.en-us.pages.meta/pages/apex_system_pagereference.htm) and [`getContent()`](https://developer.salesforce.com/docs/atlas.en-us.pages.meta/pages/apex_System_PageReference_getContent.htm).
- Styling/chrome is part of the render contract: `showHeader`, `sidebar`, `standardStylesheets`, `lightningStylesheets`, and page-level resources change output. The guide notes that standard Visualforce styling and scripts are normally included unless suppressed.
- Visualforce custom components need `apex:component`, `apex:attribute`, `assignTo`, body insertion, facets, and controller binding. See [custom component attributes](https://developer.salesforce.com/docs/atlas.en-us.pages.meta/pages/pages_comp_cust_elements_attributes.htm).
- Templates use `apex:composition`, `apex:define`, and `apex:insert`. See [`apex:composition`](https://developer.salesforce.com/docs/atlas.en-us.pages.meta/pages/pages_compref_composition.htm).
- AJAX components require a client bridge. Salesforce documents `reRender` on command links/buttons and `apex:actionSupport` for asynchronous refresh. See [partial page updates](https://developer.salesforce.com/docs/atlas.en-us.pages.meta/pages/pages_quick_start_ajax_partial_page_updates.htm) and [`apex:actionSupport`](https://developer.salesforce.com/docs/atlas.en-us.pages.meta/pages/pages_compref_actionSupport.htm).
- Visualforce-hosted LWC uses Lightning Components for Visualforce, based on Lightning Out beta: add `<apex:includeLightning/>`, define an Aura app extending `ltng:outApp`, add `aura:dependency` entries, and call `$Lightning.use()` plus `$Lightning.createComponent(...)`. See [Use Components in Visualforce Pages](https://developer.salesforce.com/docs/platform/lwc/guide/use-visualforce.html) and [`apex:includeLightning`](https://developer.salesforce.com/docs/atlas.en-us.pages.meta/pages/pages_compref_includeLightning.htm).

Local reference packet:

- Use `/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run` as the local docs reference for the implementation pass. The scrape is 140 MB and contains 15,684 files.
- Use `visualforce/pages_compref.md` and the 161 `visualforce/pages_compref_*.md` files as the component catalog source. The plan must cover every component file with `supported`, `partial`, or `unsupported` status before any public "full support" claim.
- Use `visualforce/pages_compref_page.md` for page-level behavior: `action`, `apiVersion`, `applyHtmlTag`, `applyBodyTag`, `cache`, `contentType`, `cspHeader`, `docType`, `extensions`, `language`, `lightningStylesheets`, `readOnly`, `recordSetVar`, `renderAs`, `showHeader`, `sidebar`, `standardController`, and `standardStylesheets`.
- Use `visualforce/pages_compref_component.md`, `visualforce/pages_compref_attribute.md`, and `visualforce/pages_compref_componentBody.md` for custom component access, DML allowance, `assignTo`, typed attributes, required attributes, body insertion, and component controller binding.
- Use `visualforce/pages_compref_actionFunction.md`, `pages_compref_actionSupport.md`, `pages_compref_actionRegion.md`, `pages_compref_actionStatus.md`, and `pages_compref_actionPoller.md` for AJAX behavior. These docs add parameter-order binding, region processing, status facets, poller reset behavior, interval minimums, and timeout attributes.
- Use `apex-guide/apex_invoking_javascript_remoting.md` and `apex-guide/apex_classes_annotation_RemoteAction.md` for JavaScript remoting. `@RemoteAction` methods must be `static` and `public` or `global`; adding a page controller or extension exposes every such method in that class.
- Use `visualforce/pages_compref_remoteObjects.md`, `pages_compref_remoteObjectModel.md`, and `pages_compref_remoteObjectField.md` for Visualforce Remote Objects. These components generate client-side models for create, retrieve, update, and delete against declared sObjects and fields.
- Use `visualforce/pages_compref_inputFile.md` for upload behavior. Visualforce file uploads bind `Blob` value, file name, content type, and size, and the documented upload limit is 10 MB.
- Use `visualforce/pages_compref_flow_interview.md` for `flow:interview`. It binds `Flow.Interview.<name>`, supports finish navigation, rerender, paused interviews, and flow variables.
- Use `apex/apex_System_PageReference_getContent.md`, `apex/apex_System_PageReference_getContentAsPDF.md`, and `apex/apex_pages_action.md` for Apex APIs. `getContent()` returns HTML or PDF depending on page render mode; `getContentAsPDF()` forces PDF; both are forbidden in triggers, Apex test methods, and email services.
- Use `apex-guide/pages_security_tips_csrf.md`, `apex-guide/pages_security_tips_xss.md`, and `apex-guide/pages_security_tips_scontrols.md` for security behavior. Standard Visualforce components escape by default, `escape="false"` disables that protection, raw formulas outside safe components remain risky, and standard form posts include anti-CSRF state.
- Use `limits-reference/salesforce_app_limits_platform_vf.md` for runtime guardrails: response size below 15 MB, view state 170 KB, upload 10 MB, PDF 60 MB, PDF images 30 MB, header 8,192 bytes, JavaScript remoting request 4 MB, remoting default timeout 30 seconds, remoting max timeout 120 seconds, normal query rows 50,000, read-only query rows 1,000,000, iteration items 1,000, read-only iteration items 10,000, and StandardSetController records 10,000.

## Current Glade Baseline

The saved patch `/Users/matt/Desktop/glade-2e7f7ca-visualforce and lwc rendering.patch` is mostly present on `main`. The commit object `2e7f7ca5371929c13fbfcc93f708495f05336d91` exists locally. `git apply --check` fails because many files already exist or have moved on.

Current code already has these beams set:

- `internal/visualforce/compiler.go`: parses fragments with `golang.org/x/net/html` and normalizes self-closing Visualforce tags.
- `internal/visualforce/render.go`: renders common `apex:*` tags, custom components, templates, static resources, view-state injection, and Lightning bootstrap.
- `internal/visualforce/viewstate.go`: HMAC-signed local view state with CSRF and expiry.
- `internal/server/visualforce.go`: handles GET and POST under `/apex/*`.
- `internal/vm/page_render.go`: registers `PageReference.getContent()` hook through `internal/visualforce/bridge.go`.
- `internal/lwcbrowser` and `lwcruntime`: compile and boot LWC in browser, with `@salesforce/*` shims and wire routes.

The focused baseline passed:

```bash
go test ./internal/visualforce ./internal/server ./internal/lwcbrowser ./internal/lwcruntime
```

Result:

```text
ok github.com/glade-sh/glade/internal/visualforce 2.263s
ok github.com/glade-sh/glade/internal/server 9.954s
ok github.com/glade-sh/glade/internal/lwcbrowser 2.380s
ok github.com/glade-sh/glade/internal/lwcruntime 1.621s
```

The current gaps are specific:

- Expression support is narrow: identifiers, strings, simple functions, `$Label`, `$Resource`, getters. It lacks full Visualforce expression/formula behavior.
- View state stores string field snapshots. It does not restore arbitrary Apex object graphs or component state.
- POST action dispatch reconstructs controllers after invoking actions. It does not run the exact setter/action/getter lifecycle.
- `apex:actionSupport` currently renders a span with `data-rerender`; it is not an AJAX client/server partial refresh.
- `ApexPages.Action.invoke()` still needs bound lifecycle semantics.
- Many standard components in `component_references.go` are cataloged but not rendered.
- PDF rendering remains unsupported through `RenderPageURL(..., asPDF=true)`.
- Salesforce chrome and standard stylesheet/script output is not modeled with versioned fidelity.

## Product Boundary

Keep in `glade`:

- Visualforce parser, expression evaluator, renderer, view state, lifecycle, server routes, CLI/dev support, docs, and product tests.
- LWC browser runtime needed by Visualforce pages using `<apex:includeLightning/>`.
- Small fixture projects that prove product behavior.

Keep in `/Users/matt/Dev/glade-tools`:

- Salesforce black-box capture harnesses.
- Large Visualforce component catalogs and parity scoreboards.
- Surface ledger refreshes and compatibility evidence packets.
- Generated maintenance artifacts.

The product repo may read fixture outputs checked into this repo only when they are stable and hand-curated. It must not import glade-tools packages.

## Done Definition

- GET render covers the standard component set Glade claims in a generated support table.
- POST render preserves controller state, applies form setters, dispatches actions, and refreshes view state.
- AJAX partial refresh supports `reRender`, `actionFunction`, `actionSupport`, `actionRegion`, and `actionStatus` for local deterministic pages.
- Custom components support `apex:attribute`, `assignTo`, facets, body insertion, controllers, extensions, and namespaces.
- `PageReference.getContent()` returns rendered HTML for local Visualforce pages.
- `PageReference.getContentAsPDF()` and `<apex:page renderAs="pdf">` either return PDF through the Glade toolchain or remain a documented blocker that prevents "full support" from being claimed.
- JavaScript remoting, Visualforce Remote Objects, `apex:inputFile`, `flow:interview`, XSS escaping, CSRF, CSP headers, page cache/content-type behavior, and documented Visualforce limits have product tests or explicit unsupported diagnostics.
- The generated component support table is seeded from the local scraped docs catalog and accounts for all 161 `pages_compref_*.md` files.
- `apex:includeLightning` works with existing LWC runtime paths and browser tests.
- Salesforce reference output is captured from the `oaer-probe-max` scratch org for the same Visualforce fixtures before support wording changes.
- Public docs stop saying "full Visualforce rendering unsupported" only after the gates pass.

## File Structure

| File | Action | Responsibility |
| --- | --- | --- |
| `internal/visualforce/component_registry.go` | Create | Table of standard components, status, renderer function, supported attributes |
| `internal/visualforce/component_catalog.go` | Create | Checked component catalog generated from local docs reference |
| `internal/visualforce/component_catalog_test.go` | Create | Confirms registry accounts for the generated catalog |
| `internal/visualforce/component_registry_test.go` | Create | Coverage tests against `StandardComponentReferenceNames()` |
| `internal/visualforce/compiler.go` | Modify | Source positions, raw attribute case, script/style safety |
| `internal/visualforce/expression.go` | Split/modify | Parser facade for current expression API |
| `internal/visualforce/expression_parser.go` | Create | Visualforce expression grammar |
| `internal/visualforce/expression_eval.go` | Create | Evaluation against VM/controller/scope/globals |
| `internal/visualforce/lifecycle.go` | Create | GET/POST lifecycle, controller construction, setters/actions/getters |
| `internal/visualforce/form_binding.go` | Create | Submitted value binding to controller fields, SObjects, lists |
| `internal/visualforce/partial.go` | Create | `reRender` target rendering and partial response model |
| `internal/visualforce/ajax.go` | Create | Client script emitted for AJAX Visualforce controls |
| `internal/visualforce/security.go` | Create | Escaping, CSRF, CSP, cache, content type, headers |
| `internal/visualforce/limits.go` | Create | Documented Visualforce limits and local enforcement |
| `internal/visualforce/remoting.go` | Create | JavaScript remoting route and `@RemoteAction` dispatch |
| `internal/visualforce/remote_objects.go` | Create | Remote Objects model generation and CRUD dispatch |
| `internal/visualforce/upload.go` | Create | Multipart upload binding for `apex:inputFile` |
| `internal/visualforce/flow.go` | Create | `flow:interview` adapter or explicit blocker diagnostic |
| `internal/visualforce/pdf.go` | Create | PDF rendering adapter and toolchain checks |
| `internal/visualforce/render.go` | Modify | Delegate to registry, add missing core component renderers |
| `internal/visualforce/page.go` | Modify | Route through lifecycle and support render modes |
| `internal/visualforce/viewstate.go` | Modify | Typed state model, versioning, compression, HMAC |
| `internal/server/visualforce.go` | Modify | GET/POST/AJAX/PDF response handling |
| `internal/vm/page_render.go` | Modify | Return HTML/PDF Blob through renderer hook |
| `internal/vm/platform_apexpages_formula.go` | Modify | Bind `ApexPages.Action.invoke()` to current lifecycle |
| `internal/gladecli/dev_vf_command.go` | Modify | Error overlay, reload diagnostics, browser target logging |
| `docs/STDLIB_COVERAGE.md` | Modify | Generated support wording after gates |
| `site/docs-src/guide/support-map.md` | Modify | Public support wording after gates |
| `testdata/local-tests/visualforce-rendering/` | Create | Product fixtures for renderer behavior |

## Phase 0: Coverage Harness

**Purpose:** Measure what "full" means before adding code. The harness prevents new renderer work from turning into a loose pile of cases.

**Files:**
- Create: `internal/visualforce/component_registry.go`
- Create: `internal/visualforce/component_catalog.go`
- Create: `internal/visualforce/component_catalog_test.go`
- Create: `internal/visualforce/component_registry_test.go`
- Create: `testdata/local-tests/visualforce-rendering/component_coverage.page`

- [ ] **Step 0.0: Generate the checked component catalog from local docs**

Read the local docs scrape and generate a compact checked-in catalog:

```bash
docs_root='/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run'
find "$docs_root/visualforce" -maxdepth 1 -name 'pages_compref_*.md' -type f | wc -l
```

Expected:

```text
161
```

Create `internal/visualforce/component_catalog.go` from the component index and per-component attribute tables. Keep the source path in a comment, but do not read the local scrape at runtime:

```go
type ComponentCatalogEntry struct {
	Name string
	SourceFile string
	Attributes []ComponentAttribute
	RequiredChildren []string
	DocAPIVersion string
}

type ComponentAttribute struct {
	Name string
	Type string
	Required bool
	APIVersion string
}

func StandardComponentCatalog() []ComponentCatalogEntry {
	return generatedStandardComponentCatalog()
}
```

Use the component reference index names, not current renderer names, as the ground truth. Preserve non-`apex` namespaces such as `flow:interview`, `chatter:*`, `liveAgent:*`, `messaging:*`, `site:*`, `support:*`, and `wave:*`.

- [ ] **Step 0.1: Add renderer status model**

Create `internal/visualforce/component_registry.go`:

```go
package visualforce

type ComponentStatus string

const (
	ComponentSupported ComponentStatus = "supported"
	ComponentPartial   ComponentStatus = "partial"
	ComponentUnsupported ComponentStatus = "unsupported"
)

type ComponentSpec struct {
	Name       string
	Namespace  string
	Status     ComponentStatus
	Reason     string
	Attributes []string
	DocSource  string
	Render     func(*MarkupNode, *RenderContext) (string, error)
}

func StandardComponentSpecs() map[string]ComponentSpec {
	specs := map[string]ComponentSpec{}
	for name, spec := range currentStandardComponentSpecs() {
		spec.Name = name
		specs[name] = spec
	}
	return specs
}
```

- [ ] **Step 0.2: Move current switch cases into the registry**

Create `currentStandardComponentSpecs()` in the same file and wire current renderers by name:

```go
func currentStandardComponentSpecs() map[string]ComponentSpec {
	return map[string]ComponentSpec{
		"apex:page": {Namespace: "apex", Status: ComponentPartial, Render: renderApexPage, Attributes: []string{"action", "apiVersion", "applyBodyTag", "applyHtmlTag", "cache", "contentType", "cspHeader", "controller", "docType", "extensions", "language", "lightningStylesheets", "readOnly", "recordSetVar", "renderAs", "showHeader", "sidebar", "standardController", "standardStylesheets"}},
		"apex:form": {Namespace: "apex", Status: ComponentPartial, Render: renderApexForm, Attributes: []string{"id", "prependId"}},
		"apex:outputText": {Namespace: "apex", Status: ComponentPartial, Render: func(n *MarkupNode, c *RenderContext) (string, error) { return renderApexOutput(n, c, false) }, Attributes: []string{"value", "escape"}},
		"apex:outputField": {Namespace: "apex", Status: ComponentPartial, Render: func(n *MarkupNode, c *RenderContext) (string, error) { return renderApexOutput(n, c, true) }, Attributes: []string{"value"}},
		"apex:actionSupport": {Namespace: "apex", Status: ComponentPartial, Render: renderApexActionSupport, Attributes: []string{"event", "action", "reRender", "status", "immediate", "timeout"}},
	}
}
```

Keep existing switch behavior while the registry grows. Then replace the switch arm lookup with:

```go
key := namespace + ":" + node.Name
if spec, ok := StandardComponentSpecs()[key]; ok && spec.Render != nil {
		return spec.Render(node, ctx)
}
return renderUnsupportedComponent(node, ctx)
```

- [ ] **Step 0.3: Add the failing coverage test**

Create `internal/visualforce/component_registry_test.go`:

```go
func TestStandardComponentSpecsCoverReferenceCatalog(t *testing.T) {
	specs := StandardComponentSpecs()
	missing := []string{}
	for _, entry := range StandardComponentCatalog() {
		name := strings.TrimSpace(entry.Name)
		if name == "" || strings.Contains(entry.SourceFile, "additional_") {
			continue
		}
		if _, ok := specs[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("missing Visualforce component specs: %v", missing)
	}
}
```

Create `internal/visualforce/component_catalog_test.go`:

```go
func TestGeneratedComponentCatalogMatchesLocalDocsCount(t *testing.T) {
	got := len(StandardComponentCatalog())
	if got != 161 {
		t.Fatalf("component catalog count = %d, want 161 from local docs scrape", got)
	}
}
```

- [ ] **Step 0.4: Run the test and record the real count**

Run:

```bash
go test ./internal/visualforce -run TestStandardComponentSpecsCoverReferenceCatalog -count=1
```

Expected now: FAIL with a concrete missing component list.

- [ ] **Step 0.5: Add explicit unsupported specs**

For components that require Salesforce-only services in the first pass, add `ComponentUnsupported` with a concrete reason:

```go
"analytics:reportChart": {Namespace: "analytics", Status: ComponentUnsupported, Reason: "requires live Analytics chart service"},
"apex:canvasApp": {Namespace: "apex", Status: ComponentUnsupported, Reason: "requires Canvas signed request and hosted app frame"},
"flow:interview": {Namespace: "flow", Status: ComponentUnsupported, Reason: "requires Flow runtime adapter before full support can be claimed"},
```

Run the test again. Expected: PASS because every reference has a stated status.

- [ ] **Step 0.6: Commit**

```bash
git add internal/visualforce testdata/local-tests/visualforce-rendering
git commit -m "test: add Visualforce renderer coverage registry"
```

## Phase 1: Parser and Source Model

**Purpose:** Make the markup tree strong enough for error overlays, facets, templates, and stable component ids.

**Files:**
- Modify: `internal/visualforce/compiler.go`
- Modify: `internal/visualforce/render.go`
- Create: `internal/visualforce/compiler_source_test.go`

- [ ] **Step 1.1: Add source and raw-name fields**

Extend `MarkupNode`:

```go
type MarkupNode struct {
	Type       MarkupNodeType
	Namespace  string
	Name       string
	RawName    string
	Attributes map[string]string
	RawAttributes map[string]string
	Text       string
	Children   []*MarkupNode
	Line       int
	Column     int
}
```

- [ ] **Step 1.2: Write parser tests**

Create `internal/visualforce/compiler_source_test.go`:

```go
func TestParseMarkupTreePreservesScriptStyleAndRawAttributes(t *testing.T) {
	source := `<apex:page><script>if (a < b) { console.log("{!Name}"); }</script><apex:outputText styleClass="x" value="{!Name}"/></apex:page>`
	root, err := ParseMarkupTree(source)
	if err != nil {
		t.Fatal(err)
	}
	page := root.Children[0]
	if page.RawName != "apex:page" {
		t.Fatalf("raw page name = %q", page.RawName)
	}
	out := findFirstNode(page, "apex", "outputtext")
	if out == nil || out.RawAttributes["styleClass"] != "x" {
		t.Fatalf("raw attributes = %#v", out)
	}
	script := findFirstHTMLNode(page, "script")
	if script == nil || !strings.Contains(renderTextChildren(script), "console.log") {
		t.Fatalf("script node = %#v", script)
	}
}
```

- [ ] **Step 1.3: Run the parser test**

```bash
go test ./internal/visualforce -run TestParseMarkupTreePreservesScriptStyleAndRawAttributes -count=1
```

Expected now: FAIL until `RawName` and `RawAttributes` are populated.

- [ ] **Step 1.4: Implement stable component ids**

Add a helper in `render.go`:

```go
func componentClientID(ctx *RenderContext, node *MarkupNode) string {
	if id := strings.TrimSpace(node.Attribute("id")); id != "" {
		return strings.Join(append(ctx.ClientIDStack(), id), ":")
	}
	return ctx.NextAutoID(node.Name)
}
```

Extend `RenderContext` with deterministic counters:

```go
clientIDStack []string
autoIDs map[string]int
```

- [ ] **Step 1.5: Commit**

```bash
git add internal/visualforce
git commit -m "feat: strengthen Visualforce markup source model"
```

## Phase 2: Expression Language

**Purpose:** Support the Visualforce expression language used in real pages, not only identifier substitution.

**Files:**
- Modify: `internal/visualforce/expression.go`
- Create: `internal/visualforce/expression_parser.go`
- Create: `internal/visualforce/expression_eval.go`
- Create: `internal/visualforce/expression_formula_test.go`

- [ ] **Step 2.1: Add failing expression tests**

Create `internal/visualforce/expression_formula_test.go`:

```go
func TestEvaluateVisualforceExpressionOperatorsAndGlobals(t *testing.T) {
	machine := vm.New(nil)
	machine.SetCurrentPageURL("/apex/Edit?id=001000000000001")
	ctx := &ExpressionContext{
		VM: machine,
		Controller: vm.Object("EditController"),
		Scope: NewScopeStack(),
		ProjectNamespace: "pkg",
	}
	ctx.Controller.Fields["amount"] = vm.Int(42)
	ctx.Controller.Fields["name"] = vm.String("Acme")

	cases := map[string]string{
		"amount + 8": "50",
		"IF(amount > 40, 'big', 'small')": "big",
		"NOT(ISBLANK(name))": "true",
		"$CurrentPage.parameters.id": "001000000000001",
		"namespace": "pkg",
	}
	for expr, want := range cases {
		got, err := EvaluateExpression(expr, ctx)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if got != want {
			t.Fatalf("%s = %q, want %q", expr, got, want)
		}
	}
}
```

- [ ] **Step 2.2: Run the failing test**

```bash
go test ./internal/visualforce -run TestEvaluateVisualforceExpressionOperatorsAndGlobals -count=1
```

Expected now: FAIL on arithmetic, comparison, `NOT`, `ISBLANK`, and `$CurrentPage`.

- [ ] **Step 2.3: Implement expression AST**

Create node types:

```go
type binaryExpr struct {
	op string
	left Expression
	right Expression
}

type unaryExpr struct {
	op string
	value Expression
}

type indexExpr struct {
	target Expression
	key Expression
}
```

Parser precedence:

```text
or:        a || b, OR(a,b)
and:       a && b, AND(a,b)
equality:  ==, !=
compare:   <, <=, >, >=
add:       +, -
multiply:  *, /
unary:     !, NOT(...)
primary:   literal, identifier, call, parenthesized, index
```

- [ ] **Step 2.4: Add global resolvers**

Support these first:

```go
var supportedVisualforceGlobals = []string{
	"$Action",
	"$Component",
	"$CurrentPage",
	"$Label",
	"$ObjectType",
	"$Organization",
	"$Page",
	"$Profile",
	"$Request",
	"$Resource",
	"$Site",
	"$User",
}
```

Map `$Page.Name` to `PageReference`, `$CurrentPage.parameters.*` to the current page parameter map, and `$ObjectType.Account.fields.Name.label` through existing schema describe helpers.

- [ ] **Step 2.5: Commit**

```bash
git add internal/visualforce
git commit -m "feat: expand Visualforce expression evaluation"
```

## Phase 3: Controller Lifecycle and View State

**Purpose:** Render with Salesforce-like lifecycle order instead of ad hoc construction.

**Files:**
- Create: `internal/visualforce/lifecycle.go`
- Create: `internal/visualforce/form_binding.go`
- Modify: `internal/visualforce/page.go`
- Modify: `internal/visualforce/viewstate.go`
- Modify: `internal/server/visualforce.go`
- Modify: `internal/vm/platform_apexpages_formula.go`
- Create: `internal/visualforce/lifecycle_test.go`

- [ ] **Step 3.1: Add lifecycle model**

Create `internal/visualforce/lifecycle.go`:

```go
type RequestKind string

const (
	RequestGET RequestKind = "GET"
	RequestPOST RequestKind = "POST"
	RequestAjax RequestKind = "AJAX"
)

type LifecycleRequest struct {
	Page Page
	PageURL string
	Kind RequestKind
	ViewState *ViewStatePayload
	FormValues map[string]string
	Action string
	PartialTargets []string
}

type LifecycleState struct {
	Controller vm.Value
	Extensions []vm.Value
	StandardController vm.Value
	PageMessages []vm.Value
	Redirect *vm.Value
}
```

- [ ] **Step 3.2: Add failing lifecycle test**

Create `internal/visualforce/lifecycle_test.go` with a controller that records constructor, setter, action, getter order:

```go
func TestVisualforcePostLifecycleAppliesSettersBeforeActionAndGettersAfter(t *testing.T) {
	root := makeVisualforceLifecycleProject(t)
	machine := compileLifecycleController(t, root)
	html, err := RenderPageForTest(machine, root, "Lifecycle")
	if err != nil {
		t.Fatal(err)
	}
	viewState := extractViewState(t, html)

	p, _ := project.Load(root)
	idx, _ := LoadProject(p)
	result, err := RenderPage(PageRenderRequest{
		Project: p,
		VFIndex: idx,
		Machine: machine,
		PageName: "Lifecycle",
		PageURL: "/apex/Lifecycle",
		ViewState: &viewState,
		FormValues: map[string]string{
			"form:name": "changed",
			ViewStateActionFieldName(): "{!save}",
		},
		Action: "{!save}",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, result.HTML, "setter:changed")
	assertContains(t, result.HTML, "action:save")
	assertContains(t, result.HTML, "getter:after")
}
```

- [ ] **Step 3.3: Version view state**

Change `ViewStatePayload`:

```go
type ViewStatePayload struct {
	Version int `json:"v"`
	PageName string `json:"pn"`
	CSRF string `json:"csrf"`
	Timestamp int64 `json:"ts"`
	ControllerType string `json:"ct,omitempty"`
	ControllerState json.RawMessage `json:"cstate,omitempty"`
	ExtensionState []json.RawMessage `json:"estate,omitempty"`
	ComponentState map[string]json.RawMessage `json:"cs,omitempty"`
	PageMessages []string `json:"pm,omitempty"`
}
```

Keep decoder compatibility with the old `ControllerFields` map for one release cycle.

- [ ] **Step 3.4: Bind `ApexPages.Action.invoke()`**

In `internal/vm/platform_apexpages_formula.go`, replace the current unsupported path with a lifecycle callback stored on the VM. The callback signature should carry action expression and current page URL:

```go
type VisualforceActionInvoker func(actionExpr string, pageURL string) (Value, error)
```

The action object created from `new ApexPages.Action('{!save}')` must invoke through the current render context.

- [ ] **Step 3.5: Commit**

```bash
git add internal/visualforce internal/server internal/vm
git commit -m "feat: add Visualforce lifecycle engine"
```

## Phase 4: Core Component Registry

**Purpose:** Make all high-use standard components render through one registry with explicit support status.

**Files:**
- Modify: `internal/visualforce/component_registry.go`
- Modify: `internal/visualforce/render.go`
- Create: `internal/visualforce/core_components_test.go`

- [ ] **Step 4.1: Add core component fixture**

Create `testdata/local-tests/visualforce-rendering/force-app/main/default/pages/Core.page`:

```xml
<apex:page controller="CoreController" title="Core">
  <apex:pageMessages/>
  <apex:form id="form">
    <apex:pageBlock title="Account">
      <apex:pageBlockButtons>
        <apex:commandButton value="Save" action="{!save}" reRender="summary"/>
      </apex:pageBlockButtons>
      <apex:pageBlockSection columns="2">
        <apex:pageBlockSectionItem>
          <apex:outputLabel value="Name"/>
          <apex:inputText id="name" value="{!name}"/>
        </apex:pageBlockSectionItem>
      </apex:pageBlockSection>
    </apex:pageBlock>
    <apex:outputPanel id="summary">
      <apex:outputText value="{!name}"/>
    </apex:outputPanel>
  </apex:form>
</apex:page>
```

- [ ] **Step 4.2: Add test**

Create `internal/visualforce/core_components_test.go`:

```go
func TestRenderCoreVisualforceComponents(t *testing.T) {
	root := copyFixtureProject(t, "testdata/local-tests/visualforce-rendering")
	machine := compileProjectController(t, root, "CoreController")
	html, err := RenderPageForTest(machine, root, "Core")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<form`,
		`class="bPageBlock"`,
		`name="form:name"`,
		`data-rerender="form:summary"`,
		`com.salesforce.visualforce.ViewState`,
	} {
		assertContains(t, html, want)
	}
}
```

- [ ] **Step 4.3: Expand components**

Bring these to `ComponentSupported` or `ComponentPartial` with tests:

```text
page, form, outputText, outputField, outputLabel, outputPanel, outputLink,
outputFormat, param, pageMessages, pageMessage, message,
pageBlock, pageBlockButtons, pageBlockSection, pageBlockSectionItem,
pageBlockTable, column, dataTable, dataList, repeat,
inputText, inputTextarea, inputSecret, inputHidden, inputCheckbox, inputField,
selectList, selectCheckboxes, selectRadio, selectOption, selectOptions,
commandButton, commandLink, actionFunction, actionSupport, actionRegion,
actionStatus, actionPoller, variable, stylesheet, includeScript, image,
iframe, facet
```

The `apex:page` renderer must cover the local docs attribute set before promotion beyond `ComponentPartial`:

```text
action, apiVersion, applyBodyTag, applyHtmlTag, cache, contentType,
cspHeader, docType, expires, extensions, language, lightningStylesheets,
manifest, readOnly, recordSetVar, renderAs, rendered, showHeader, sidebar,
standardController, standardStylesheets, tabStyle, title
```

Keep deprecated attributes in the catalog with inert behavior and diagnostics where Salesforce documents no effect.

- [ ] **Step 4.4: Commit**

```bash
git add internal/visualforce testdata/local-tests/visualforce-rendering
git commit -m "feat: expand core Visualforce component rendering"
```

## Phase 5: Data-Aware Components

**Purpose:** Make field rendering depend on schema and records, not string formatting.

**Files:**
- Modify: `internal/visualforce/render.go`
- Create: `internal/visualforce/field_rendering.go`
- Create: `internal/visualforce/field_rendering_test.go`
- Modify: `internal/schema/schema.go` only if existing describe helpers cannot answer field display questions

- [ ] **Step 5.1: Add field fixture**

Create `testdata/local-tests/visualforce-rendering/force-app/main/default/pages/Fields.page`:

```xml
<apex:page standardController="Account">
  <apex:form id="f">
    <apex:outputField value="{!Account.Name}"/>
    <apex:inputField id="industry" value="{!Account.Industry}"/>
    <apex:selectList id="rating" value="{!Account.Rating}">
      <apex:selectOptions value="{!ratingOptions}"/>
    </apex:selectList>
  </apex:form>
</apex:page>
```

- [ ] **Step 5.2: Add test**

```go
func TestRenderInputFieldUsesLocalSchemaAndRecordValues(t *testing.T) {
	root := copyFixtureProject(t, "testdata/local-tests/visualforce-rendering")
	machine := vmWithAccountRecord(t, root, map[string]storage.Value{
		"Name": storage.StringValue("Acme"),
		"Industry": storage.StringValue("Manufacturing"),
		"Rating": storage.StringValue("Hot"),
	})
	html, err := RenderPageForTest(machine, root, "Fields")
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `Acme`)
	assertContains(t, html, `name="f:industry"`)
	assertContains(t, html, `<option value="Hot" selected="selected">Hot</option>`)
}
```

- [ ] **Step 5.3: Implement field renderers**

Add `field_rendering.go`:

```go
type FieldRenderKind string

const (
	FieldText FieldRenderKind = "text"
	FieldCheckbox FieldRenderKind = "checkbox"
	FieldTextarea FieldRenderKind = "textarea"
	FieldSelect FieldRenderKind = "select"
	FieldDate FieldRenderKind = "date"
	FieldDatetime FieldRenderKind = "datetime"
)

func renderFieldInput(ctx *RenderContext, binding FieldBinding, id string) (string, error) {
	switch binding.Kind {
	case FieldCheckbox:
		return renderCheckboxInput(binding, id), nil
	case FieldSelect:
		return renderSelectInput(binding, id), nil
	default:
		return renderTextInput(binding, id), nil
	}
}
```

Use existing org/schema metadata. Do not infer behavior from field names.

- [ ] **Step 5.4: Commit**

```bash
git add internal/visualforce testdata/local-tests/visualforce-rendering
git commit -m "feat: render Visualforce fields from local schema"
```

## Phase 6: Custom Components, Facets, Templates

**Purpose:** Make real componentized pages work.

**Files:**
- Modify: `internal/visualforce/render.go`
- Create: `internal/visualforce/component_lifecycle.go`
- Create: `internal/visualforce/custom_component_test.go`

- [ ] **Step 6.1: Add fixture**

Create `testdata/local-tests/visualforce-rendering/force-app/main/default/components/Card.component`:

```xml
<apex:component controller="CardController">
  <apex:attribute name="title" type="String" assignTo="{!heading}" required="true" description="Card heading"/>
  <apex:facet name="actions"/>
  <section class="card">
    <h2>{!heading}</h2>
    <div class="actions"><apex:insert name="actions"/></div>
    <apex:componentBody/>
  </section>
</apex:component>
```

Create `testdata/local-tests/visualforce-rendering/force-app/main/default/pages/CardHost.page`:

```xml
<apex:page controller="CardHostController">
  <c:Card title="{!title}">
    <apex:facet name="actions">
      <apex:commandLink value="Edit" action="{!edit}"/>
    </apex:facet>
    <apex:outputText value="{!body}"/>
  </c:Card>
</apex:page>
```

- [ ] **Step 6.2: Add test**

```go
func TestRenderCustomComponentAssignToFacetAndBody(t *testing.T) {
	root := copyFixtureProject(t, "testdata/local-tests/visualforce-rendering")
	machine := compileCardControllers(t, root)
	html, err := RenderPageForTest(machine, root, "CardHost")
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<h2>Deal Summary</h2>`)
	assertContains(t, html, `Edit`)
	assertContains(t, html, `Quarterly renewal`)
}
```

- [ ] **Step 6.3: Implement component lifecycle**

Create a `ComponentInvocation`:

```go
type ComponentInvocation struct {
	Component Component
	Attributes map[string]vm.Value
	Facets map[string]*MarkupNode
	Body []*MarkupNode
	Controller vm.Value
}
```

Rules:

- Evaluate parent attributes before constructing child body.
- Apply `assignTo` after child controller construction and before child getters.
- Support `<apex:componentBody/>` by rendering the caller body in the parent scope.
- Support `<apex:facet name="x">` and `<apex:insert name="x">`.
- Keep namespace lookup compatible with `pkg__Component` and `pkg:Component`.

- [ ] **Step 6.4: Commit**

```bash
git add internal/visualforce testdata/local-tests/visualforce-rendering
git commit -m "feat: support Visualforce custom component lifecycle"
```

## Phase 7: Forms and Partial Page Refresh

**Purpose:** Make user interaction work without full browser reloads when Visualforce asks for AJAX.

**Files:**
- Create: `internal/visualforce/partial.go`
- Create: `internal/visualforce/ajax.go`
- Modify: `internal/visualforce/render.go`
- Modify: `internal/server/visualforce.go`
- Create: `internal/server/visualforce_ajax_test.go`
- Create: `lwcruntime/test/vf-ajax.test.mjs` only if the shared browser helper is needed

- [ ] **Step 7.1: Add server test**

```go
func TestHandleVisualforceAjaxPartialRefresh(t *testing.T) {
	srv := newVisualforceFixtureServer(t, "Ajax.page", `<apex:page controller="AjaxController">
<apex:form id="f">
  <apex:outputPanel id="count"><apex:outputText value="{!count}"/></apex:outputPanel>
  <apex:commandButton id="inc" value="Inc" action="{!increment}" reRender="count"/>
</apex:form>
</apex:page>`)

	first := httptest.NewRecorder()
	srv.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/apex/Ajax", nil))
	viewState := extractHTMLInput(first.Body.String(), ViewStateFormFieldName())

	form := url.Values{}
	form.Set(ViewStateFormFieldName(), viewState)
	form.Set("__vf_csrf", extractHTMLInput(first.Body.String(), "__vf_csrf"))
	form.Set(ViewStateActionFieldName(), "{!increment}")
	form.Set("__vf_ajax", "1")
	form.Set("__vf_rerender", "f:count")
	req := httptest.NewRequest(http.MethodPost, "/apex/Ajax", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	second := httptest.NewRecorder()
	srv.ServeHTTP(second, req)
	if second.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", second.Code, second.Body.String())
	}
	assertContains(t, second.Body.String(), `"targets"`)
	assertContains(t, second.Body.String(), `"f:count"`)
	assertContains(t, second.Body.String(), `1`)
}
```

- [ ] **Step 7.2: Implement partial response**

Create `internal/visualforce/partial.go`:

```go
type PartialResponse struct {
	Targets map[string]string `json:"targets"`
	ViewState string `json:"viewState"`
	Messages []string `json:"messages,omitempty"`
	Redirect string `json:"redirect,omitempty"`
}
```

- [ ] **Step 7.3: Emit AJAX client**

Create `internal/visualforce/ajax.go`:

```go
func VisualforceAjaxScript() string {
	return `<script>
window.GLADEVF=window.GLADEVF||{};
window.GLADEVF.submit=function(form,action,targets){
  var data=new FormData(form);
  data.set("__vf_action",action||"");
  data.set("__vf_ajax","1");
  data.set("__vf_rerender",targets||"");
  fetch(form.action,{method:"POST",body:new URLSearchParams(data)})
    .then(function(r){return r.json();})
    .then(function(p){Object.keys(p.targets||{}).forEach(function(id){
      var el=document.getElementById(id); if(el){el.outerHTML=p.targets[id];}
    });});
  return false;
};
</script>`
}
```

- [ ] **Step 7.4: Support these controls**

Bring these controls to tested `ComponentPartial` or `ComponentSupported`:

```text
commandButton reRender
commandLink reRender
actionFunction
actionSupport
actionRegion
actionStatus
actionPoller with interval and enabled flags
```

- [ ] **Step 7.5: Commit**

```bash
git add internal/visualforce internal/server lwcruntime/test testdata/local-tests/visualforce-rendering
git commit -m "feat: add Visualforce partial page refresh"
```

## Phase 7A: Security, Remoting, Remote Objects, Flow, Uploads, and Limits

**Purpose:** Cover the local docs scrape items that are easy to miss if the renderer only follows common page examples.

**Files:**
- Create: `internal/visualforce/security.go`
- Create: `internal/visualforce/security_test.go`
- Create: `internal/visualforce/limits.go`
- Create: `internal/visualforce/limits_test.go`
- Create: `internal/visualforce/remoting.go`
- Create: `internal/visualforce/remoting_test.go`
- Create: `internal/visualforce/remote_objects.go`
- Create: `internal/visualforce/remote_objects_test.go`
- Create: `internal/visualforce/upload.go`
- Create: `internal/visualforce/upload_test.go`
- Create: `internal/visualforce/flow.go`
- Create: `internal/visualforce/flow_test.go`
- Modify: `internal/server/visualforce.go`
- Modify: `internal/vm`
- Modify: `testdata/local-tests/visualforce-rendering/`

- [ ] **Step 7A.1: Add security tests from local docs**

Create tests for default escaping, explicit unescaped output, raw formula output, CSRF, CSP, cache, and content type:

```go
func TestVisualforceOutputEscapingMatchesDocs(t *testing.T) {
	html := renderVisualforceForTest(t, `<apex:page><apex:outputText value="{!payload}"/></apex:page>`, map[string]any{
		"payload": `<script>alert(1)</script>`,
	})
	assertContains(t, html, `&lt;script&gt;alert(1)&lt;/script&gt;`)
	assertNotContains(t, html, `<script>alert(1)</script>`)
}

func TestVisualforceEscapeFalseRendersRawOutput(t *testing.T) {
	html := renderVisualforceForTest(t, `<apex:page><apex:outputText escape="false" value="{!payload}"/></apex:page>`, map[string]any{
		"payload": `<b>raw</b>`,
	})
	assertContains(t, html, `<b>raw</b>`)
}

func TestVisualforcePostRejectsInvalidCSRF(t *testing.T) {
	srv := newVisualforceFixtureServer(t, "Post.page", `<apex:page controller="PostController"><apex:form><apex:commandButton action="{!save}" value="Save"/></apex:form></apex:page>`)
	form := url.Values{}
	form.Set(ViewStateFormFieldName(), "tampered")
	form.Set("__vf_csrf", "wrong")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/apex/Post", strings.NewReader(form.Encode())))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}
```

Implement:

- Standard Visualforce components escape HTML by default.
- `escape="false"` disables escaping only on components that document it.
- Raw formulas in HTML text nodes follow Salesforce's risky behavior and receive a diagnostic in dev mode.
- POST requests validate view state and CSRF before setter/action dispatch.
- `<apex:page cspHeader="true">` emits a local CSP with `script-src 'self'`.
- `cache`, `expires`, `contentType`, and `contentType#filename` set response headers.

- [ ] **Step 7A.2: Add documented limit constants**

Create `internal/visualforce/limits.go`:

```go
const (
	MaxVisualforceResponseBytes = 15 * 1000 * 1000
	MaxVisualforceViewStateBytes = 170 * 1024
	MaxVisualforceUploadBytes = 10 * 1000 * 1000
	MaxVisualforcePDFBytes = 60 * 1000 * 1000
	MaxVisualforcePDFImageBytes = 30 * 1000 * 1000
	MaxVisualforceHeaderBytes = 8192
	MaxVisualforceRemotingRequestBytes = 4 * 1000 * 1000
	DefaultVisualforceRemotingTimeout = 30 * time.Second
	MaxVisualforceRemotingTimeout = 120 * time.Second
	MaxVisualforceQueryRows = 50000
	MaxVisualforceReadOnlyQueryRows = 1000000
	MaxVisualforceIterationItems = 1000
	MaxVisualforceReadOnlyIterationItems = 10000
	MaxVisualforceStandardSetControllerRecords = 10000
)
```

Tests must cover:

- View state over 170 KB gets a stable diagnostic.
- `readOnly="true"` raises query and iteration limits for the request.
- `actionPoller interval` below 5 seconds normalizes or errors according to Salesforce behavior captured in parity tests.
- Remoting requests over 4 MB fail before Apex dispatch.
- Upload requests over 10 MB fail before Blob binding.

- [ ] **Step 7A.3: Implement JavaScript remoting**

Add a server route for Visualforce remoting calls and generate page JavaScript for every `@RemoteAction` method in the page controller and extensions:

```go
type RemotingRequest struct {
	Action string `json:"action"`
	Method string `json:"method"`
	Data []json.RawMessage `json:"data"`
	Type string `json:"type,omitempty"`
	TID int `json:"tid,omitempty"`
	CTX map[string]any `json:"ctx,omitempty"`
}
```

Rules from the local docs:

- A remote method must be `static` and `public` or `global`.
- Adding a controller or extension exposes all `@RemoteAction` methods on that class, even when the page does not call them in markup.
- Arguments can be primitives, collections, typed sObjects, generic sObjects with `Id` or `sobjectType`, user-defined Apex classes, and interfaces with `apexType`.
- Return values can be primitives, sObjects, collections, Apex classes, enums, `SaveResult`, `UpsertResult`, `DeleteResult`, `SelectOption`, or `PageReference`.
- The callback receives the result and an event/status object.
- The configuration object can control response escaping.
- The default timeout is 30 seconds and the maximum timeout is 120 seconds.

Add a test that calls both forms:

```js
Visualforce.remoting.Manager.invokeAction("{!$RemoteAction.RemoteController.echo}", "x", callback);
RemoteController.echo("x", callback, { escape: true, timeout: 30000 });
```

- [ ] **Step 7A.4: Implement Visualforce Remote Objects**

Render `<apex:remoteObjects>`, `<apex:remoteObjectModel>`, and `<apex:remoteObjectField>` into local JavaScript models backed by Glade storage:

```xml
<apex:remoteObjects jsNamespace="Remote">
  <apex:remoteObjectModel name="Account" jsShorthand="Acct" fields="Name,Industry">
    <apex:remoteObjectField name="Rating" jsShorthand="rate"/>
  </apex:remoteObjectModel>
</apex:remoteObjects>
```

Required behavior:

- Generate the configured JavaScript namespace and shorthand names.
- Restrict client CRUD to declared objects and fields.
- Support create, retrieve, update, and delete over local storage.
- Honor method overrides through `$RemoteAction` attributes when present.
- Return explicit diagnostics for sharing, FLS, or server services Glade cannot model.

- [ ] **Step 7A.5: Implement `apex:inputFile`**

Add multipart form parsing and Blob binding:

```go
func TestVisualforceInputFileBindsBlobAndMetadata(t *testing.T) {
	srv := newUploadFixtureServer(t)
	rec := postMultipartVisualforce(t, srv, "/apex/Upload", "f:file", "invoice.txt", "text/plain", []byte("hello"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	assertContains(t, rec.Body.String(), "invoice.txt")
	assertContains(t, rec.Body.String(), "text/plain")
	assertContains(t, rec.Body.String(), "5")
}
```

Rules:

- Bind `value` to a `Blob`.
- Bind `fileName`, `contentType`, and `fileSize` through setters or properties.
- Enforce 10 MB before loading into memory.
- Do not serialize file bodies into view state.

- [ ] **Step 7A.6: Implement or block `flow:interview` explicitly**

The local docs make `flow:interview` part of the Visualforce component catalog. Full Visualforce support cannot ignore it.

Implement if a Glade Flow runtime is available:

- Resolve `name` to local flow metadata.
- Instantiate `Flow.Interview.<name>` and bind the `interview` attribute.
- Render screen steps with local form controls.
- Support `finishLocation`, `rerender`, `pausedInterviewId`, navigation buttons, and output variable access through the interview object.

If no Flow runtime exists when this task starts, keep `flow:interview` as `ComponentUnsupported` with this diagnostic:

```text
flow:interview requires local Flow runtime support and blocks full Visualforce component support
```

Then create a separate follow-on plan for Flow runtime support before changing public wording to "full".

- [ ] **Step 7A.7: Commit**

```bash
git add internal/visualforce internal/server internal/vm testdata/local-tests/visualforce-rendering
git commit -m "feat: cover Visualforce docs edge services"
```

## Phase 8: Lightning Out and Browser Runtime Completion

**Purpose:** Finish the Visualforce/LWC bridge that the patch started.

**Files:**
- Modify: `internal/lwcbrowser/manifest.go`
- Modify: `internal/lwcbrowser/salesforce_modules.go`
- Modify: `internal/lwcbrowser/wire.go`
- Modify: `internal/server/lightning.go`
- Modify: `internal/server/lightning_wire.go`
- Modify: `lwcruntime/src/glade.out.mjs`
- Modify: `lwcruntime/test/*.test.mjs`
- Modify: `testdata/local-tests/lightning-out-vf/**`

- [ ] **Step 8.1: Add callback/status browser test**

Add or extend `lwcruntime/test/callback.test.mjs`:

```js
test("$Lightning.createComponent callback matches Salesforce status contract", async () => {
  const page = await mountLightningOutFixture({
    script: `
      $Lightning.use("c:lightningOut", function() {
        $Lightning.createComponent("c:counter", { label: "Count" }, "host", function(cmp, status, message) {
          window.__callback = { tag: cmp && cmp.tagName, status, message };
        });
      });
    `
  });
  await page.waitForFunction(() => window.__callback && window.__callback.status === "SUCCESS");
  const callback = await page.evaluate(() => window.__callback);
  assert.equal(callback.status, "SUCCESS");
  assert.equal(callback.message, undefined);
});
```

- [ ] **Step 8.2: Expand wire and modules**

Add tests and support for:

```text
@salesforce/user/*
@salesforce/i18n/*
@salesforce/resourceUrl/*
@salesforce/contentAssetUrl/*
lightning/uiObjectInfoApi getObjectInfo, getPicklistValues
lightning/messageService local pubsub subset
lightning/navigation explicit unsupported diagnostic
```

- [ ] **Step 8.3: Keep Aura rendering out of scope**

Do not build an Aura renderer. For Visualforce Lightning Out, index `ltng:outApp` and `aura:dependency`, alias simple Aura wrappers to LWC when they are passthrough wrappers, and produce explicit diagnostics for true Aura component render attempts.

- [ ] **Step 8.4: Run browser tests**

```bash
npm --prefix lwcruntime test
go test ./internal/lwcbrowser ./internal/server -run 'TestLightning|TestVFPageBootstrapsLightningOut' -count=1
```

- [ ] **Step 8.5: Commit**

```bash
git add internal/lwcbrowser internal/server lwcruntime testdata/local-tests/lightning-out-vf
git commit -m "feat: complete Visualforce Lightning Out runtime bridge"
```

## Phase 9: PageReference Content and PDF

**Purpose:** Make Apex APIs render the same content path as the server.

**Files:**
- Modify: `internal/visualforce/page.go`
- Create: `internal/visualforce/pdf.go`
- Modify: `internal/vm/page_render.go`
- Create: `internal/vm/page_render_pdf_test.go`
- Modify: `internal/gladehome/install.go`
- Modify: `internal/gladecli/toolchain_command.go`

- [ ] **Step 9.1: Add `getContent()` test**

Create `internal/vm/page_render_pdf_test.go`:

```go
func TestPageReferenceGetContentRendersLocalVisualforcePage(t *testing.T) {
	root := makePageReferenceContentProject(t)
	machine := compileContentProject(t, root)
	visualforce.SetVMRenderEnvironment(machine, mustLoadProject(t, root))
	program, err := CompileAnonymous(`
Blob body = Page.Invoice.getContent();
System.assert(body.toString().contains('Invoice Total'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 9.2: Add PDF adapter**

Create `internal/visualforce/pdf.go`:

```go
type PDFRenderer interface {
	RenderPDF(ctx context.Context, html string, baseURL string) ([]byte, error)
}

type PDFToolchain struct {
	Command string
}

func (p PDFToolchain) RenderPDF(ctx context.Context, html string, baseURL string) ([]byte, error) {
	if strings.TrimSpace(p.Command) == "" {
		return nil, vm.UnsupportedFeature("PageReference.getContentAsPDF requires the Glade PDF toolchain")
	}
	return runPDFCommand(ctx, p.Command, html, baseURL)
}
```

Use a Glade-managed toolchain path, not a system command guessed from `$PATH`. The first implementation may install or locate Chromium through `glade toolchain install`; tests can use Playwright's browser when available.

- [ ] **Step 9.3: Add render mode support**

In `RenderPageURL(machine, pageURL, asPDF bool)`, render HTML first. If `asPDF` or page metadata has `renderAs="pdf"`, pass the HTML through the PDF adapter and return `vm.NewBlobValueBytes(pdfBytes)`.

- [ ] **Step 9.4: Add unsupported diagnostic test**

When the PDF toolchain is absent, the error must be stable:

```go
func TestPageReferenceGetContentAsPDFRequiresToolchain(t *testing.T) {
	machine := vm.New(nil)
	_, err := visualforce.RenderPageURL(machine, "/apex/Missing", true)
	if err == nil || !strings.Contains(err.Error(), "requires the Glade PDF toolchain") {
		t.Fatalf("err = %v", err)
	}
}
```

This test remains until the install path is available in CI.

- [ ] **Step 9.5: Commit**

```bash
git add internal/visualforce internal/vm internal/gladehome internal/gladecli
git commit -m "feat: render Visualforce PageReference content"
```

## Phase 10: Dev Server, Error Overlay, and Diagnostics

**Purpose:** Make the feature usable without reading Go errors.

**Files:**
- Modify: `internal/gladecli/dev_vf_command.go`
- Modify: `internal/server/visualforce.go`
- Modify: `internal/visualforce/page.go`
- Create: `internal/server/visualforce_error_test.go`
- Modify: `docs/LOCAL_TESTING.md`
- Modify: `site/docs-src/guide/local-testing.md`

- [ ] **Step 10.1: Add error overlay test**

```go
func TestVisualforceRenderErrorOverlayIncludesFileLineAndExpression(t *testing.T) {
	srv := newVisualforceFixtureServer(t, "Broken.page", `<apex:page><apex:outputText value="{!missing + }"/></apex:page>`)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/Broken", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	assertContains(t, rec.Body.String(), "Visualforce render error")
	assertContains(t, rec.Body.String(), "Broken.page")
	assertContains(t, rec.Body.String(), "missing +")
}
```

- [ ] **Step 10.2: Add dev output**

`glade dev vf` should print:

```text
Visualforce dev server: http://127.0.0.1:8080
Pages:
  /apex/Core
  /apex/CardHost
Watching ... for .page, .component, .cls, aura, lwc, and static resource changes.
```

- [ ] **Step 10.3: Add route for component support JSON**

Add `/services/data/vXX.X/glade/visualforce/support` returning:

```json
{
  "components": [
    {"name":"page","status":"partial","reason":"chrome not fully modeled"},
    {"name":"form","status":"supported"}
  ]
}
```

Keep this under the local `glade` namespace in the local server only.

- [ ] **Step 10.4: Commit**

```bash
git add internal/gladecli internal/server internal/visualforce docs site
git commit -m "feat: improve Visualforce dev diagnostics"
```

## Phase 11: Scratch-Org Parity Capture and Product Support Wording

**Purpose:** Compare with Salesforce using the `oaer-probe-max` scratch org without making the product depend on the maintenance harness.

**Files in `/Users/matt/Dev/glade-tools`:**
- Create or modify: `docs/fixtures/visualforce-rendering-parity.json`
- Create or modify: `internal/capability/visualforce_rendering.go`
- Create or modify: `cmd/glade-tools` command wiring only if no capture command exists
- Create or modify: `internal/salesforce/oracle_visualforce.go` or the nearest existing oracle package

**Files in `/Users/matt/Dev/glade`:**
- Modify: `docs/STDLIB_COVERAGE.md`
- Modify: `docs/COMPATIBILITY.md`
- Modify: `site/docs-src/guide/support-map.md`
- Modify: `site/docs-src/guide/local-api-server.md`

- [ ] **Step 11.0: Confirm the scratch org**

Run a bounded Salesforce CLI check before doing any org-backed work. Do not let an auth prompt hang the worker:

```bash
python3 - <<'PY'
import json
import subprocess
import sys

cmd = ["sf", "org", "display", "--target-org", "oaer-probe-max", "--json"]
try:
    proc = subprocess.run(cmd, text=True, capture_output=True, timeout=30)
except subprocess.TimeoutExpired:
    sys.exit("sf org display timed out for oaer-probe-max; refresh Salesforce CLI auth before continuing")

if proc.returncode != 0:
    sys.stderr.write(proc.stderr)
    sys.exit(proc.returncode)

data = json.loads(proc.stdout)
result = data.get("result", {})
print("status=%s alias=%s username=%s orgId=%s" % (
    data.get("status"),
    result.get("alias") or "oaer-probe-max",
    result.get("username", ""),
    result.get("orgId") or result.get("id", ""),
))
PY
```

Expected: status `0` and a current username/org id. If this check times out or fails, stop this phase. Do not mark Visualforce rendering verified by Salesforce until the org check passes.

- [ ] **Step 11.1: Build and deploy the parity fixture to `oaer-probe-max`**

Create `/tmp/vf-parity-project` from the product fixtures and include pages that exercise the full renderer:

```text
Core
Fields
CardHost
Ajax
Security
Remoting
RemoteObjects
Upload
FlowHost
LightningHost
PDFInvoice
```

Deploy the fixture:

```bash
sf project deploy start \
  --target-org oaer-probe-max \
  --source-dir /tmp/vf-parity-project/force-app \
  --json
```

Expected: deploy status `Succeeded`. If Salesforce rejects the fixture, fix the fixture or product assumptions before capturing local output.

- [ ] **Step 11.2: Capture Salesforce reference output in glade-tools**

Run from `/Users/matt/Dev/glade-tools` after adding the fixture. The command must deploy or verify the deployed source, fetch page HTML through the scratch org, invoke `PageReference.getContent()`/`getContentAsPDF()` probes, and write a redacted fixture with no access token or session id:

```bash
go run ./cmd/glade-tools compat visualforce capture \
  --target-org oaer-probe-max \
  --project /tmp/vf-parity-project \
  --pages Core,Fields,CardHost,Ajax,Security,Remoting,RemoteObjects,Upload,FlowHost,LightningHost,PDFInvoice \
  --out /tmp/glade-vf-parity-capture.json
```

Expected output:

```text
confirmed target org oaer-probe-max
captured 11 Visualforce pages
captured PageReference.getContent probes
captured PageReference.getContentAsPDF probes
wrote /tmp/glade-vf-parity-capture.json
```

The capture fixture must include:

```json
{
  "targetOrg": "oaer-probe-max",
  "capturedAt": "RFC3339 timestamp",
  "orgId": "redacted or non-secret org id",
  "pages": [],
  "apexProbes": [],
  "limits": {
    "viewStateBytes": 174080,
    "uploadBytes": 10000000,
    "remotingRequestBytes": 4000000
  }
}
```

If `glade-tools` does not already have this command, add it there. Do not add Salesforce CLI orchestration to the product `glade` CLI.

- [ ] **Step 11.3: Compare local output**

Run from `/Users/matt/Dev/glade`:

```bash
go test ./internal/visualforce ./internal/server -run 'TestVisualforceParity' -count=1
```

Expected: PASS with diff budget printed in test logs for each page and each Apex probe.

- [ ] **Step 11.4: Update public docs only from measured support**

Do not claim byte-identical Salesforce output. Use wording like:

```text
Visualforce rendering: local HTML rendering, controller lifecycle, postback, common standard components, custom components, static resources, and Lightning Out/LWC embedding are supported for local development and tests. Salesforce chrome and PDF output require the Visualforce support table; unsupported components report explicit diagnostics.
```

- [ ] **Step 11.5: Commit both repos separately**

```bash
cd /Users/matt/Dev/glade-tools
git add docs/fixtures internal/capability cmd
git commit -m "test: add Visualforce rendering parity capture"

cd /Users/matt/Dev/glade
git add docs site internal/visualforce internal/server
git commit -m "docs: publish Visualforce rendering support table"
```

## Final Verification

Run focused gates first:

```bash
go test ./internal/visualforce -count=1
go test ./internal/server -run 'TestHandleVisualforce|TestLightning|TestStaticResource' -count=1
go test ./internal/vm -run 'Test.*(PageReference|ApexPages|Visualforce)' -count=1
go test ./internal/apextest -run 'TestRunResolves.*Visualforce|Test.*InvokePage' -count=1
npm --prefix lwcruntime test
```

Run broad gates when the feature is ready to publish:

```bash
go test ./...
npm --prefix site test
npm --prefix site run build
scripts/smoke.sh
git diff --check
```

Run browser proof for `glade dev vf`:

```bash
go build -o /tmp/glade ./cmd/glade
/tmp/glade toolchain install
/tmp/glade dev vf --project testdata/local-tests/visualforce-rendering --addr 127.0.0.1:18080
```

Open:

```text
http://127.0.0.1:18080/apex/Core
http://127.0.0.1:18080/apex/CardHost
http://127.0.0.1:18080/apex/Ajax
```

Run scratch-org proof before changing support wording:

```bash
python3 - <<'PY'
import json, subprocess, sys
cmd = ["sf", "org", "display", "--target-org", "oaer-probe-max", "--json"]
try:
    proc = subprocess.run(cmd, text=True, capture_output=True, timeout=30)
except subprocess.TimeoutExpired:
    sys.exit("oaer-probe-max org check timed out")
if proc.returncode != 0:
    sys.stderr.write(proc.stderr)
    sys.exit(proc.returncode)
print(json.loads(proc.stdout).get("status"))
PY

cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools compat visualforce capture \
  --target-org oaer-probe-max \
  --project /tmp/vf-parity-project \
  --pages Core,Fields,CardHost,Ajax,Security,Remoting,RemoteObjects,Upload,FlowHost,LightningHost,PDFInvoice \
  --out /tmp/glade-vf-parity-capture.json
```

Stop rule:

- Do not change public docs from "unsupported" to "supported" until focused gates, broad gates, and browser proof pass.
- Do not change public docs from "unsupported" to "supported" until `oaer-probe-max` scratch-org parity capture passes and the checked fixture has no token, session id, or org secret.
- Do not claim PDF support until `getContentAsPDF()` returns a valid `%PDF-` Blob in a product test.
- Do not import or depend on glade-tools from this repo.

## Visual Aid

The companion lifecycle diagram was written to:

```text
/tmp/picture-it/visualforce-rendering-lifecycle.html
```
