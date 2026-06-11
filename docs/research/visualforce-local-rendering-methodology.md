# Visualforce Local Rendering: Technical Deep Dive & Methodology for Glade

## 1. Current State of Visualforce in Glade

### 1.1 What Exists

Glade has three tiers of Visualforce support, each at a different maturity level:

**Tier 1 — Metadata Parsing & Indexing (`internal/visualforce/`)**

- `visualforce.go` parses `.page` and `.component` files using regex-based token extraction (tagRE + attrRE) rather than XML parsing. This handles broken/lenient markup.
- Extracts: controller, standardController, extensions, action, component attributes, and merge references (`{! ... }`).
- Classifies merge references by type: `URLFOR`, `$Label`, `$ObjectType`, `$Resource`, `$Site`, `ControllerExpression`.
- `component_references.go` maintains a catalog of ~173 standard Visualforce component names (for metadata purposes, not rendering).

**Tier 2 — Controller Logic Runtime (VM — `internal/vm/`)**

- `platform_apexpages_formula.go` (807 lines): Full `ApexPages.StandardController` (getId, getRecord, save, delete, view/edit/cancel, reset, addFields) and `ApexPages.StandardSetController` (getRecords, pagination, save, delete, cancel, filterId, addFields, listViewOptions).
- `ui_invocation.go` (322 lines): `InvokeVisualforceAction()` — constructs a controller instance, sets current page context, invokes a zero-arg instance method, collects page messages, emits trace events.
- `vm.go`: `PageReference` objects with full member dispatch (getUrl, setRedirect, getParameters, getHeaders, getCookies).
- `dispatch.go`: `ApexPages.currentPage()`, `ApexPages.hasMessages()`, `ApexPages.addMessage()`, `Test.setCurrentPage()`, `Test.invokePage()`.
- `visualforce_validation_runtime.go`: Automatic `ApexPages.Message` generation from DML errors when a page context is active.

**Tier 3 — Server & Test Integration**

- `internal/server/`: Tooling API exposes `ApexPage` (type `066`) and `ApexComponent` (type `099`) objects with Markup, ApiVersion, MasterLabel, ControllerKey.
- `internal/apextest/`: Registers Visualforce page names on the VM during test setup so `Page.` prefix references resolve.
- `internal/resource/`: Reads `.page` files and stores `Markup` in the org state for the `ApexPage` SObject.
- `glade-tools/internal/perfscan/`: Treats Visualforce pages as entry points (`EntryVisualforce` kind) and scans for `apex.visualforce.action` performance findings through the performance plugin.

### 1.2 What Is Explicitly Missing

The critical gap, verbatim from `vm/platform_passive_members.go`:

```
getContent() → UnsupportedFeature
  "PageReference.getContent local Visualforce page rendering surface"
getContentAsPDF() → UnsupportedFeature
  "PageReference.getContentAsPDF local Visualforce page rendering surface"
```

And from `platform_apexpages_formula.go`:

```
ApexPages.Action.invoke() → UnsupportedFeature
  "ApexPages.Action.invoke requires bound Visualforce controller lifecycle"
```

The following are NOT implemented:
- HTML rendering of `<apex:page>`, `<apex:component>`, or any VF markup to HTML
- Visualforce expression language evaluation (`{! ... }`) beyond static resource resolution
- Component tree construction, composition, and inclusion
- View state management (encryption, serialization, deserialization)
- Visualforce request lifecycle (POST-back processing, controller state hydration)
- `apex:composition`, `apex:include`, `apex:insert`, `apex:define`, `apex:dynamicComponent`, `apex:facet`
- Iteration components: `apex:repeat`, `apex:dataTable`, `apex:dataList`, `apex:pageBlockTable`

### 1.3 Design Philosophy

Glade's Visualforce support is intentionally scoped as a **controller contract layer**, not a rendering surface. The goal is to let developers test controller logic (DML, SOQL, page redirects, messages) without needing a full page render. This is stated in the docs:

> `STDLIB_COVERAGE.md`: "ApexPages.Message — partial — Constructor and getters; no Visualforce rendering lifecycle."
> `RELEASE_NOTES.md`: Visualforce trace events were added for controller action testing.

---

## 2. Visualforce Rendering Lifecycle (Salesforce Reference)

Understanding the full lifecycle is essential for designing a local equivalent.

### 2.1 Page Request Flow

```
Browser GET /apex/MyPage
  ↓
Salesforce App Server
  ↓
1. Session validation & user resolution
2. Lookup Visualforce page by name → fetch Markup from ApexPage metadata
3. Parse markup into component tree (apex:page root, nesting of standard/custom components)
4. Instantiate StandardController or custom controller + extensions
5. Execute constructor chain → controller state initialized
6. Resolve expressions in markup ({!controllerVar}, {!$Label.X}, etc.) using expression language
7. Walk component tree, emitting HTML for each component based on its renderer
8. Generate and encrypt view state (serialized controller state + component state)
9. Insert hidden <input id="com.salesforce.visualforce.ViewState" value="...encrypted...">
10. Return HTML document
```

### 2.2 POST-back (Action) Flow

```
Browser POST /apex/MyPage (form submit or JS remoting)
  ↓
1. Decrypt and deserialize view state → restore controller & component state
2. Identify which action (commandButton, commandLink, actionFunction, actionSupport) triggered POST
3. Execute action method on controller
4. Re-render component tree if needed (reRender targets)
5. (for apex:actionFunction/apex:actionSupport with reRender) return partial HTML fragment
6. (for full postback) re-render entire page
7. Re-encrypt view state, return full HTML
```

### 2.3 Standard Component Catalog

Salesforce provides ~250 standard Visualforce components. Key categories:

| Category | Components |
|----------|-----------|
| Page structure | `page`, `component`, `composition`, `include`, `define`, `insert`, `attribute`, `facet` |
| Data display | `outputText`, `outputField`, `outputLabel`, `outputLink`, `outputPanel`, `outputFormat`, `detail`, `listViews`, `relatedList`, `enhancedList` |
| Data input | `inputText`, `inputField`, `inputHidden`, `inputSecret`, `inputCheckbox`, `inputFile`, `selectList`, `selectCheckboxes`, `selectRadio`, `selectOption`, `selectOptions` |
| Data iteration | `repeat`, `dataTable`, `dataList`, `pageBlockTable`, `pageBlock`, `pageBlockSection`, `pageBlockSectionItem`, `pageBlockButtons`, `column` |
| Actions | `commandButton`, `commandLink`, `actionFunction`, `actionSupport`, `actionPoller`, `actionStatus`, `actionRegion` |
| UI chrome | `form`, `stylesheet`, `includeScript`, `image`, `iframe`, `flash`, `tabPanel`, `tab`, `panelBar`, `panelBarItem`, `toolbar`, `toolbarGroup` |
| Messages | `pageMessages`, `pageMessage`, `message` |
| Parameters | `param` |
| Variables | `variable` |
| Email | `emailPublisher`, `emailTemplate`, `emailBody` |
| Charting | `chart`, `chartData`, `chartLabel`, `chartSeries` |
| Dynamic | `dynamicComponent`, `dynamicBinding` |

The `component_references.go` file already catalogs 173 of these.

---

## 3. Methodology for Local Visualforce Rendering in Glade

### 3.1 Architecture Overview

The proposed architecture has three layers, each buildable independently:

```
┌──────────────────────────────────────────────────────────┐
│ Layer 1: Visualforce Compiler                            │
│ ────────────────────────────────────                     │
│ .page/.component Markup → Component Tree                 │
│ Expression parsing and AST building                      │
│ Controller binding resolution                            │
├──────────────────────────────────────────────────────────┤
│ Layer 2: Visualforce Renderer                            │
│ ────────────────────────────────────                     │
│ Component Tree → HTML output                             │
│ Expression evaluation (against VM state)                 │
│ Standard component HTML renderers                        │
├──────────────────────────────────────────────────────────┤
│ Layer 3: Visualforce Server Integration                  │
│ ────────────────────────────────────                     │
│ GET /apex/{page} → full-page render return HTML          │
│ View state serialization/deserialization                 │
│ POST-back handling & controller rehydration              │
│ Static resource serving                                  │
│ Label resolution and metadata-driven field rendering     │
└──────────────────────────────────────────────────────────┘
```

### 3.2 Layer 1: Visualforce Compiler

**Package: `internal/visualforce/compiler/`**

#### 3.2.1 Markup Parser Rewrite

The current regex-based parser (`parseMarkup`, `tagRE`, `attrRE`) is sufficient for metadata extraction but cannot produce a component tree. A proper HTML/XML parser is needed.

**Approach: Use Go's `golang.org/x/net/html` (stdlib-compatible)**

The `x/net/html` package is a fully spec-compliant HTML5 parser that handles lenient markup (unclosed tags, attribute anomalies, embedded CSS/JS, bare `&` characters). This handles the same problem space as the current regex parser but produces a parse tree.

```go
// Replace parseMarkup with:
func parseMarkupTree(path string) (*html.Node, error) {
    content, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    doc, err := html.Parse(strings.NewReader(string(content)))
    if err != nil {
        return nil, err
    }
    return doc, nil
}
```

The `x/net/html` dependency is already indirect through Go 1.26's stdlib. Check `go.mod` — if not present, add it. Given Go 1.26, this may already be folded into stdlib.

**Risk:** `x/net/html` auto-inserts `<html>`, `<head>`, `<body>` wrappers for fragments. Visualforce pages are fragments (no `<html>` root). Solution: parse as fragment, or extract body children post-parse.

#### 3.2.2 Component Tree Construction

Walk the HTML parse tree and build a typed component tree. Each node maps to a VF component.

```go
type VFNode struct {
    Component  string            // "page", "form", "outputText", "c:MyBadge", etc.
    Attributes map[string]string // controller="...", value="{!...}", etc.
    Body       []VFNode          // child text and elements
    Source     html.Node         // original source location
}

type VFPage struct {
    Name       string
    Controller string
    Extensions []string
    StdCtrl    string
    Root       VFNode
}

type VFComponent struct {
    Name       string
    Controller string
    Attributes []VFAttribute
    Root       VFNode
}
```

**Namespace resolution rules:**
- `apex:*` → standard component (check `component_references.go` catalog)
- `c:*` → custom component (resolve from project index)
- Other namespace prefixes → managed package components
- Unprefixed names with `:` are custom namespace components

**Attribute processing:**
- `assignTo` on component attributes binds a page value to a component controller property
- `rendered`, `id`, `styleClass`, `style` are pass-through HTML attributes
- `value`, `selectedValue`, `var`, `items`, `label`, `title` are component-specific

#### 3.2.3 Expression Parser

Build a proper expression grammar for `{! ... }` expressions. The current `classifyMergeExpression()` is a prefix-based classifier — it needs to become a recursive-descent expression parser.

**Grammar subset needed:**

```
expression    = accessor | functionCall | literal | globalRef
accessor      = identifier ("." identifier)*
functionCall  = identifier "(" argList ")"
argList       = expression ("," expression)*
literal       = stringLiteral | numberLiteral | booleanLiteral | nullLiteral
stringLiteral = "'" [^']* "'"
globalRef     = "$" globalVar ("." accessor)*
globalVar     = "Label" | "Resource" | "ObjectType" | "Site" | "Component" |
                "User" | "Profile" | "Organization" | "Setup" | "Api" |
                "Action" | "CurrentPage" | "Page" | "Request"
```

**Expression evaluation context:**

```go
type ExpressionContext struct {
    Controller       Value        // the page's controller object
    Extensions       []Value      // extension controller objects
    StdController    Value        // standard controller (if any)
    Record           Value        // the SObject being viewed/edited
    PageMessages     []Value      // current page messages
    ComponentContext *VFNode      // the component being rendered
    Labels           LabelsRegistry    // $Label map
    StaticResources  ResourceRegistry  // $Resource URLs
    Schema           *typesys.Index     // $ObjectType resolution
    User             UserInfo          // $User, $Profile
}
```

The expression evaluator runs against the live VM state, querying controller fields, standard controller record data, and global variables.

**Key difference from Salesforce:** Salesforce evaluates expressions in a specific precedence chain (controller fields, then extensions in order, then label/resource globals). Glade already has the controller construction logic in `ui_invocation.go` — expression evaluation should follow the same priority.

### 3.3 Layer 2: Visualforce Renderer

**Package: `internal/visualforce/renderer/` (or new integrated package)**

#### 3.3.1 Renderer Interface

Each standard component gets a renderer function:

```go
type Renderer func(node VFNode, ctx *RenderContext) (string, error)

type RenderContext struct {
    Vm       *vm.VM
    Page     *VFPage
    ExprCtx  *ExpressionContext
    // Output builder and state
}
```

#### 3.3.2 Component Renderer Registry

A registry mapping component names to renderer functions, with fallback for custom components:

```go
var StandardRenderers = map[string]Renderer{
    "page":        renderPage,
    "form":        renderForm,
    "outputText":  renderOutputText,
    "outputField": renderOutputField,
    "inputText":   renderInputText,
    "inputField":  renderInputField,
    "inputHidden": renderInputHidden,
    "commandButton":  renderCommandButton,
    "commandLink":    renderCommandLink,
    "actionFunction": renderActionFunction,
    "actionSupport":  renderActionSupport,
    "pageBlock":      renderPageBlock,
    "pageBlockTable": renderPageBlockTable,
    "pageBlockSection": renderPageBlockSection,
    "pageBlockSectionItem": renderPageBlockSectionItem,
    "pageBlockButtons": renderPageBlockButtons,
    "pageMessages":  renderPageMessages,
    "pageMessage":   renderPageMessage,
    "dataTable":     renderDataTable,
    "dataList":      renderDataList,
    "repeat":        renderRepeat,
    "column":        renderColumn,
    "outputLabel":   renderOutputLabel,
    "outputLink":    renderOutputLink,
    "outputPanel":   renderOutputPanel,
    "selectList":    renderSelectList,
    "selectOption":  renderSelectOption,
    "selectOptions": renderSelectOptions,
    "selectCheckboxes": renderSelectCheckboxes,
    "selectRadio":   renderSelectRadio,
    "stylesheet":    renderStyleSheet,
    "includeScript": renderIncludeScript,
    "image":         renderImage,
    "variable":      renderVariable,
    "param":         renderParam,
    "detail":        renderDetail,
    "relatedList":   renderRelatedList,
    "enhancedList":  renderEnhancedList,
    "attribute":     renderAttribute,
    "include":       renderInclude,
    "composition":   renderComposition,
    "define":        renderDefine,
    "insert":        renderInsert,
    "facet":         renderFacet,
    "component":     renderComponent,
    "dynamicComponent": renderDynamicComponent,
    "actionPoller":  renderActionPoller,
    "actionStatus":  renderActionStatus,
    "actionRegion":  renderActionRegion,
    "tabPanel":      renderTabPanel,
    "tab":           renderTab,
    "panelBar":      renderPanelBar,
    "panelBarItem":  renderPanelBarItem,
    "toolbar":       renderToolbar,
    "toolbarGroup":  renderToolbarGroup,
    "iframe":        renderIframe,
    "flash":         renderFlash,
    "inlineEditSupport": renderInlineEditSupport,
    // Fallback for unknown/standard components
    "*":             renderPassthroughTag,
}
```

#### 3.3.3 Minimum Viable Renderers (MVP Priority)

The first pass should support the top components that cover ~80% of real Visualforce pages:

1. **`apex:page`** — root wrapper: emits `<!DOCTYPE html>`, `<html>`, `<head>` with view state hidden input, `<body>` with children.
2. **`apex:form`** — `<form method="post">` with `action` pointing to `/apex/{pageName}`, includes view state.
3. **`apex:outputText`** — evaluates `{!value}` expression, HTML-encodes result, wraps in `<span>` (or bare text if no `style`/`styleClass`).
4. **`apex:outputField`** — reads SObject field definition, formats value (date, currency, etc.), renders with label.
5. **`apex:inputText`** — `<input type="text">` with name/value binding.
6. **`apex:inputField`** — `<input>` or `<select>` based on field type (picklist → select, checkbox → checkbox, etc.).
7. **`apex:inputHidden`** — `<input type="hidden">`.
8. **`apex:commandButton`** — `<button type="submit">` or `<input type="submit">`.
9. **`apex:commandLink`** — `<a href="...">` with JavaScript postback.
10. **`apex:pageBlock`** — Salesforce-styled `<div class="bPageBlock">`.
11. **`apex:pageBlockSection`** — `<div class="pbSubsection">` with title and columns.
12. **`apex:pageBlockSectionItem`** — two-column row (label + value/input).
13. **`apex:pageBlockButtons`** — button bar.
14. **`apex:pageBlockTable`** — `<table>` with column headers, iterates over records.
15. **`apex:dataTable`** — simplified table without page block styling.
16. **`apex:repeat`** — `for` loop over a list variable.
17. **`apex:pageMessages`** — renders `ApexPages.Message` list as styled error/warning/info blocks.
18. **`apex:outputLink`** — `<a href="...">`.
19. **`apex:outputPanel`** — `<div>` or `<span>` wrapper.
20. **`apex:selectList`** / **`apex:selectOption`** — `<select>` with `<option>`s.
21. **`apex:dataList`** — definition list `<dl>`.
22. **`apex:stylesheet`** — `<link rel="stylesheet">`.
23. **`apex:includeScript`** — `<script src="...">`.
24. **`apex:variable`** — declares a local variable (no HTML output).
25. **`apex:param`** — passes a parameter to parent component.
26. **`apex:attribute`** — component attribute declaration (no HTML output).
27. **`apex:image`** — `<img>` tag.
28. **`c:CustomComponent`** — resolves to `.component` file, instantiates its controller, evaluates its body.

#### 3.3.4 Expression Evaluation in Render Context

For each `{! ... }` expression encountered during rendering:
1. Parse into AST using expression grammar
2. Walk AST against `ExpressionContext`:
   - Leaf identifiers → look up in controller fields, then extension fields, then std controller, then globals
   - Dot chains → walk object field hierarchy
   - Function calls → resolve known VF functions (`URLFOR`, `IF`, `CASESAFEID`, `TEXT`, `ISBLANK`, `ISNULL`, `NULLVALUE`, `JSENCODE`, `JSINHTMLENCODE`, `HTMLENCODE`, `SUBSTITUTE`, `BEGINS`, `CONTAINS`, etc.)
   - Global references → `$Label.c.xxx` (labels table), `$Resource.xxx` (static resource URL), `$ObjectType.xxx` (schema metadata), `$User.xxx` (user record)
3. Convert result to string (with appropriate formatting for the output context)

### 3.4 Layer 3: Visualforce Server Integration

**Integration with existing `internal/server/` and `internal/playground/`**

#### 3.4.1 Page Serving Endpoint

Add to `internal/server/server.go` routing:

```go
// GET /apex/{pageName}  → serve rendered Visualforce page
// POST /apex/{pageName} → handle Visualforce postback
if len(parts) >= 2 && parts[0] == "apex" {
    s.handleVisualforcePage(w, r, parts[1:])
    return
}
```

**GET handler:**

```go
func (s *Server) handleVisualforcePage(w http.ResponseWriter, r *http.Request) {
    pageName := strings.Join(parts, "/")

    // 1. Lookup page from project index or org state
    page, ok := s.visualforceIndex.Page(pageName)
    if !ok {
        writeSalesforceError(w, errNotFound, "Visualforce page not found")
        return
    }

    // 2. Load page markup from file
    markup, err := os.ReadFile(page.File)
    if err != nil {
        writeSalesforceError(w, errInternalError, err.Error())
        return
    }

    // 3. Compile page markup to component tree
    compiled, err := compilePage(string(markup), s.visualforceIndex)
    if err != nil {
        writeSalesforceError(w, errInternalError, err.Error())
        return
    }

    // 4. Create VM instance with org state
    machine := s.runtime.Clone()
    machine.Org = s.Org
    machine.currentPage = machine.newPageReference(fmt.Sprintf("/apex/%s", pageName))

    // 5. Construct controller (standard + custom + extensions)
    controllerCtx, err := s.buildVisualforceController(machine, compiled)
    if err != nil {
        writeSalesforceError(w, errInternalError, err.Error())
        return
    }

    // 6. Build expression context
    exprCtx := s.buildExpressionContext(machine, controllerCtx, compiled)

    // 7. Render component tree to HTML
    renderCtx := &RenderContext{
        Vm:         machine,
        Page:       compiled,
        ExprCtx:    exprCtx,
        IsPostBack: false,
    }
    rendered, err := renderComponentTree(compiled.Root, renderCtx)
    if err != nil {
        writeSalesforceError(w, errInternalError, err.Error())
        return
    }

    // 8. Serialize and encrypt view state
    viewState, err := serializeViewState(controllerCtx, machine)
    if err != nil {
        writeSalesforceError(w, errInternalError, err.Error())
        return
    }

    // 9. Inject view state into rendered HTML
    finalHTML := injectViewState(rendered, viewState, pageName)

    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.Write([]byte(finalHTML))
}
```

**POST handler:**

```go
func (s *Server) handleVisualforcePostBack(w http.ResponseWriter, r *http.Request) {
    // 1. Parse multipart form
    // 2. Extract view state from hidden input
    // 3. Deserialize and decrypt view state → restore VM state + controller state
    // 4. Identify action (which button/link was clicked) from request parameters
    // 5. Execute action method on restored controller
    // 6. Determine reRender targets
    // 7. Re-render component tree (full or partial)
    // 8. Re-serialize view state, inject, return HTML
}
```

#### 3.4.2 View State Implementation

View state is the most complex piece. On Salesforce it's an encrypted, compressed, base64-encoded blob of serialized controller state. For Glade:

```go
type ViewState struct {
    PageName      string            `json:"pn"`
    ControllerJSON string           `json:"c"`   // serialized controller + extensions
    StdControllerJSON string        `json:"sc"`  // serialized std controller record
    PageMessagesJSON string         `json:"pm"`  // serialized page messages
    ComponentState map[string]string `json:"cs"`  // per-component state by ID
    CSRF          string            `json:"csrf"` // CSRF token
    Timestamp    int64             `json:"ts"`
    Signature    []byte            `json:"sig"`  // HMAC signature
}

func serializeViewState(ctx *ControllerContext, machine *vm.VM) (string, error) {
    vs := ViewState{
        PageName:       ctx.Page.Name,
        ControllerJSON: jsonSerializeValue(ctx.Controller),
        CSRF:           generateCSRF(),
        Timestamp:     time.Now().Unix(),
    }
    payload, _ := json.Marshal(vs)
    compressed := gzipCompress(payload)
    mac := hmacSHA256(compressed, viewStateSecret)
    encoded := base64Encode(append(compressed, mac...))
    return encoded, nil
}
```

**State that must survive round-trips:**
- Controller field values (including transient fields — they survive view state, excluding `transient` keyword)
- Standard controller record state (including unsaved edits)
- Component state (collapsed sections, selected tabs, pagination state)
- Page messages
- Extension state

**State NOT in view state:**
- `transient` fields (the VM already handles this via the Apex compiler)
- Static variables (reset per-request — handled by VM cloning)

#### 3.4.3 CSRF Protection

Salesforce's Visualforce view state includes a CSRF token. Glade should:

1. Generate a random CSRF token on page load, embed in view state
2. On POST, verify the token in the view state matches
3. Use HMAC to prevent view state tampering

#### 3.4.4 Static Resource & Label Serving

The existing `internal/resource/` package handles static resource URLs. For local rendering:

- `{!URLFOR($Resource.Bundle, 'styles.css')}` → `/resource/Bundle/styles.css`
- Need a `GET /resource/{name}/{path}` handler in the server that serves files from static resource directories
- `{!$Label.MyLabel}` → lookup from CustomLabels metadata
- Can serve labels as an in-page `<script>` block or via API call

### 3.5 Label, Resource, and Schema Resolution

All rendering is driven by local metadata. No external service calls are needed.

**Static resources** are resolved from the project's `staticresources/` directory. The existing `internal/resource/` package provides `URLForStaticResource()` for `{!URLFOR($Resource.Bundle, 'x.css')}` resolution. A `/resource/{name}/{path}` handler on the server serves the actual files.

**Custom labels** (`{!$Label.c.Mylabel}`) come from the project's `labels/` directory (`CustomLabels.labels-meta.xml`). The VM already has label infrastructure used by `internal/vm/label_resolver.go`. The expression evaluator queries this directly.

**Schema metadata** (`{!$ObjectType.Account.fields.Name.label}`) is resolved from `internal/storage/` org object definitions. The `typesys.Index` and `storage.OrgState.Objects[].Definition` provide field metadata (label, type, picklist values, etc.). No network call — the metadata is already loaded at startup via `gladeschema.LoadProject()`.

**User context** (`{!$User.FirstName}`, `{!$Profile.Name}`) comes from the local org state's user table. The server sets the current user from headers or defaults to the first platform user.

### 3.6 Development Command: `glade dev vf`

A new development mode entry point:

```bash
# Start Visualforce dev server (fully local)
glade dev vf --port 8080 --project .
```

**Features:**
- Hot reload: `fsnotify` on `.page`, `.component`, `.cls` files → auto-recompile and re-render
- In-browser error overlay showing compilation/rendering errors (like Next.js/Vite error overlays)
- Debug mode: view state inspector panel, expression evaluation trace, controller state snapshot
- Fully self-contained: controllers execute in the local VM against local org state, all labels/resources/schema resolved from local metadata

---

## 4. Implementation Roadmap

### Phase 1: Foundation (Markup → Tree → Text) — 2-3 weeks

**Objective:** Build a deterministic compiler path from Visualforce markup to renderable component nodes.

**Exit conditions**
- `.page` and `.component` compile into typed trees.
- Expression parse/eval runs for fields, labels, global vars, and simple functions.
- `apex:page`, `apex:outputText`, and `apex:outputField` render to HTML.
- `GET /apex/{name}` returns HTML for read-only pages.

**Subagent lanes**
1. **Parser lane:** replace regex extraction with tree parser and metadata-preserving node model.
2. **Expression lane:** build recursive-descent parser and evaluator, wired to VM state.
3. **Renderer lane:** build registry, root wrappers, and baseline text outputs.
4. **Server lane:** wire GET path and controller bootstrap.

| Task | Description | Package |
|------|-------------|---------|
| 1.1 | HTML parser integration (`x/net/html`) to build typed component tree | `internal/visualforce/compiler/` |
| 1.2 | `VFNode`/`VFPage`/`VFComponent` model with source-location metadata | `internal/visualforce/compiler/` |
| 1.3 | Expression parser: recursive-descent grammar for `{! ... }` | `internal/visualforce/expression/` |
| 1.4 | Evaluation context: controller, extensions, std controller, globals, schema, labels, resources | `internal/visualforce/expression/` |
| 1.5 | Renderer interface + registry for known component names | `internal/visualforce/renderer/` |
| 1.6 | `apex:page`, `apex:outputText`, `apex:outputField` renderers + baseline escaping | `internal/visualforce/renderer/` |
| 1.7 | Server endpoint `GET /apex/{name}` returns rendered page HTML | `internal/server/` |
| 1.8 | Replace `PageReference.getContent` `UnsupportedFeature` with compile+render path | `internal/vm/platform_passive_members.go` |

**Validation**
- Render `testdata/local-tests/visualforce-pages/force-app/main/default/pages/OrderList.page`.
- Confirm no panic on malformed tags and component lookup.
- Add a smoke assertion for output containing controller field values and `$Label` rendering.

**Success checks**
- No `UnsupportedFeature` for `ApexPages.PageReference.getContent` in happy paths.
- `glade test` still isolates org state per test run.
- Existing perf scan still records `EntryVisualforce` findings.

### Phase 2: Interaction (Forms + Postback) — 2-3 weeks

**Objective:** Add full request lifecycle for page actions and controller round-trip.

**Exit conditions**
- POST request can deserialize valid view state.
- Action method can execute through `ApexPages` controller lifecycle.
- Inputs and command controls round-trip values into controller fields.
- Message flow from action execution re-renders into page output.

**Subagent lanes**
1. **State lane:** view state model, signing, and safe decode/encode.
2. **Postback lane:** action target mapping, parameter binding, action invocation.
3. **Form/field lane:** input and command component set.
4. **Message lane:** page-message renderers and validation.

| Task | Description | Package |
|------|-------------|---------|
| 2.1 | View state serialization/deserialization + HMAC verification | `internal/visualforce/viewstate/` |
| 2.2 | `POST /apex/{name}` handler: decode view state → rehydrate controller → dispatch action | `internal/server/` |
| 2.3 | `apex:form` renderer with hidden state and action route binding | `internal/visualforce/renderer/` |
| 2.4 | `apex:inputText`, `apex:inputHidden`, `apex:inputCheckbox` | `internal/visualforce/renderer/` |
| 2.5 | `apex:inputField` renderers for text, picklist, checkbox, date | `internal/visualforce/renderer/` |
| 2.6 | `apex:commandButton`, `apex:commandLink` action dispatch semantics | `internal/visualforce/renderer/` |
| 2.7 | `apex:pageMessages` rendering from `ApexPages.Message` stack | `internal/visualforce/renderer/` |
| 2.8 | `apex:selectList`, `apex:selectOption`, `apex:selectOptions` | `internal/visualforce/renderer/` |
| 2.9 | Postback regression set: full-submit and action-polling style partial responses | `internal/visualforce/` |

**Validation**
- Add form round-trip fixture with `PageReference.getContent` and a controller action that changes state.
- Verify action re-render updates one bound field and retains view state integrity on second submit.
- Confirm failed action restores prior view state and writes page error messages.

**Success checks**
- Action invocations can survive two or more postbacks.
- Invalid or tampered view-state returns controlled failure, no crash.
- CSRF token mismatch fails closed with explicit error path.

### Phase 3: Data Components (Tables, Repeats, Detail) — 2-3 weeks

**Objective:** Cover data-heavy pages and reusable listing layouts used by sales pages.

**Exit conditions**
- Repeating components iterate correctly over list and map-backed collections.
- Table components support row/column output without custom render logic.
- Detail components can show common SObject forms.

**Subagent lanes**
1. **Iteration lane:** `apex:repeat` and local variable scope behavior.
2. **Table lane:** list rendering, column extraction, and paging-safe structure.
3. **Block lane:** pageBlock family and sectioning semantics.
4. **Utility lane:** detail/list related components and simple actions.

| Task | Description | Package |
|------|-------------|---------|
| 3.1 | `apex:repeat` renderer with `var` scoping and index support | `internal/visualforce/renderer/` |
| 3.2 | `apex:dataTable`, `apex:column` renderers (records + headers) | `internal/visualforce/renderer/` |
| 3.3 | `apex:pageBlock`, `apex:pageBlockSection`, `apex:pageBlockSectionItem` | `internal/visualforce/renderer/` |
| 3.4 | `apex:pageBlockTable` renderer and action column handling | `internal/visualforce/renderer/` |
| 3.5 | `apex:pageBlockButtons` renderer and button alignment rules | `internal/visualforce/renderer/` |
| 3.6 | `apex:detail`, `apex:relatedList`, `apex:enhancedList` simplified mode | `internal/visualforce/renderer/` |
| 3.7 | `apex:outputLink`, `apex:outputPanel`, `apex:outputLabel` | `internal/visualforce/renderer/` |
| 3.8 | `apex:actionSupport` partial update behavior (best-effort) | `internal/visualforce/renderer/` |
| 3.9 | Data component snapshot tests and malformed input hardening | `internal/visualforce/` |

**Validation**
- Build fixtures for repeat + dataTable + pageBlock combinations.
- Verify variable scope does not leak across nested repeats.
- Verify row-level rendering works with empty collections and null fields.

**Success checks**
- At least one fixture of each family renders deterministic HTML.
- No hidden panics when component receives non-list for repeat-like nodes.
- No unsupported-feature bleed into runtime paths used by core pages.

### Phase 4: Component Composition & Static Resources — 2-3 weeks

**Objective:** Close the gap between isolated markup and project-scale page composition.

**Exit conditions**
- Custom component includes render with nested controller context.
- Label and resource functions are available in expressions and URL rendering.
- Include/define/insert style composition works for simple templates.

**Subagent lanes**
1. **Component lane:** custom component discovery, args, and lifecycle.
2. **Template lane:** composition/define/insert resolution.
3. **Resource lane:** static resource serving and URL mapping.
4. **Globals lane:** labels, schema refs, dynamic components.

| Task | Description | Package |
|------|-------------|---------|
| 4.1 | `c:MyBadge` resolution: parse `.component` and render nested body | `internal/visualforce/compiler/`, `internal/visualforce/renderer/` |
| 4.2 | `apex:composition`, `apex:define`, `apex:insert` template chain | `internal/visualforce/renderer/` |
| 4.3 | `apex:include` support for page and component fragments | `internal/visualforce/renderer/` |
| 4.4 | `apex:attribute` declaration and assignTo binding | `internal/visualforce/compiler/` |
| 4.5 | `GET /resource/{name}/{path}` with cache-safe static serving | `internal/server/` |
| 4.6 | `$Label` and `$Resource` expression resolution from local metadata | `internal/visualforce/expression/`, `internal/resource/` |
| 4.7 | `apex:dynamicComponent` scaffolded with graceful fallback path | `internal/visualforce/renderer/` |
| 4.8 | `apex:variable` scope stack for iteration and component trees | `internal/visualforce/expression/` |
| 4.9 | Namespace and managed-package component test coverage | `internal/visualforce/` |

**Validation**
- Composition fixture with `apex:composition` + `apex:insert` should render parent and child values.
- Custom component should inherit and isolate scope correctly.
- Verify resource endpoint resolves `URLFOR($Resource.Bundle,'x.css')` and serves file.

**Success checks**
- Label/text renders correctly across nested component trees.
- Dynamic component render fallback returns diagnostic marker rather than generic panic.
- Static file responses include correct MIME and cache headers for repeat calls.

### Phase 5: Developer Experience — 1-2 weeks

**Objective:** Turn rendering work into usable local workflow and test hooks.

**Exit conditions**
- Local dev command has hot reload with clear compile/run errors.
- Test API exposes page rendering assertions.
- Observability exists for render cost and expression behavior.

**Subagent lanes**
1. **CLI lane:** `glade dev vf` and command wiring.
2. **Error UX lane:** overlay, source mapping, and action traces.
3. **Test lane:** rendering assertions and deterministic fixtures.
4. **Metrics lane:** render-time and expression-cost telemetry.

| Task | Description | Package |
|------|-------------|---------|
| 5.1 | `glade dev vf` command with file watching and hot reload | `internal/gladecli/`, `internal/cli/` |
| 5.2 | Error overlay with file + line context for compile/render failures | `internal/server/`, `internal/playground/` |
| 5.3 | View state inspector panel and action trace list | `internal/server/`, `internal/playground/` |
| 5.4 | Render metrics: time per component and expression cache hit-rate | `internal/visualforce/renderer/` |
| 5.5 | `glade test` hooks: assert rendered HTML fragments and lifecycle events | `internal/apextest/`, `internal/gladecli/` |
| 5.6 | Documentation polish and runbook for unsupported components | `docs/research/`, `docs/STDLIB_COVERAGE.md` |

**Validation**
- Start `glade dev vf`, edit `.page` file, and observe hot-reload without server restart.
- Trigger an action with missing variable and confirm overlay points at exact expression.
- Profile output records component and expression hotspots.

**Success checks**
- New command appears in help and CLI docs.
- Typical page tests can assert against output and action side effects.
- DX path supports local page iteration without browser/manual steps.

### Phase execution and worktree process

This project is documented for worktree execution:

1. Phase 1 changes land in worktree `.../glade-phase-1` first.
2. Each phase lands only if previous phase gates pass.
3. Before the next phase, branch is clean and evidence is recorded.
4. Each phase checkpoint should be committed with message `docs(vf): complete phase N local rendering`.

If a phase is blocked by unknowns, leave the next phase untouched and freeze that checkpoint.

---

## 5. Key Technical Decisions

### 5.1 HTML Parser: `golang.org/x/net/html` vs Custom

**Recommendation: `golang.org/x/net/html`**

- Handles malformed markup (the same problem that drove the current regex approach)
- Produces a proper parse tree with whitespace preservation
- Zero new dependencies (already in Go extended library)
- Risk: auto-wrapping of fragments. Mitigation: use `html.ParseFragment` or extract from `<body>`.

### 5.2 Template Engine vs Manual Rendering

**Recommendation: Manual component renderers over a template engine**

- Each component has specific rendering logic (e.g., `apex:inputField` renders a `<select>` for picklists, `<input type="checkbox">` for booleans, etc.)
- No Go template engine can handle conditionals based on runtime schema lookups
- The component model is more like React/Vue's render functions than HTML templates
- Use `strings.Builder` for output and `fmt.Fprintf` or template snippets for repetitive HTML patterns

### 5.3 View State Encryption

**Recommendation: HMAC-SHA256, not full encryption (for dev mode)**

- Production Visualforce view state is encrypted on Salesforce because it may contain sensitive data
- In local dev mode, HMAC tamper protection is sufficient
- Secret derived from `glade.yml` or auto-generated per-session
- Option for full AES-GCM encryption if needed for parity testing

### 5.4 Iteration Variables and Scope

**Recommendation: Lexical scoping via render context stack**

- `apex:repeat` with `var="item"` pushes a new scope frame for each child, shadowing outer variables
- `apex:variable` with `var="x" value="{!...}"` declares a local template variable
- Component attributes with `assignTo="{!prop}"` bind to component controller fields, not the render scope
- The render context maintains a stack of scope frames consulted during expression evaluation

---

## 6. Integration with Existing Glade Architecture

### 6.1 Server Layer

The `internal/server/server.go` `serveHTTPLocked` method already routes by URL path prefix. Adding `/apex/` routing is a single case in the switch:

```go
if len(parts) >= 2 && parts[0] == "apex" {
    s.handleVisualforcePage(w, r, parts[1:])
    return
}
```

The server already holds `Org *storage.OrgState` and an optional `Index *typesys.Index` with full type information. Visualforce needs both.

### 6.2 VM Layer

The VM already has:
- `currentPage` field for page context
- `pageMessages` for `ApexPages.Message` collection
- Full StandardController and StandardSetController dispatch
- DML/validation → page message integration

New VM additions needed:
- `cloneWithOrg(org)` method for per-request VM instances
- `serializeState()` / `restoreState(bytes)` for view state persistence
- Per-component state registry

### 6.3 Test Runner

The test runner (`internal/apextest/`) already registers Visualforce page names and supports `InvokeVisualforceAction`. Extending it:

- `Test.renderPage(PageReference)` → renders page to string for assertion
- `Test.setCurrentPageAndController(pageName)` → full page context setup
- Assertions on rendered output (e.g., verify specific HTML content, verify page messages appear)

### 6.4 Playground Integration

The playground (`internal/playground/server.go`) already:
- Has a React frontend with an editor and run/reset controls
- Compiles and executes anonymous Apex
- Uses a SQLite-backed org state

Visualforce integration would add:
- A "Pages" tab showing project Visualforce pages
- Click a page → render it in an iframe or inline preview
- Edit markup → auto-rerender on save
- Trace view showing controller execution and expression evaluation

---

## 7. Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Component coverage is too large (250+ standard components) | Scope creep | Phase approach; 28 core components cover ~80% of real pages |
| Expression language complexity (Salesforce has hundreds of VF functions) | Render errors on complex expressions | Graceful fallback: unrenderable expressions render as `{!expr}` text with warning |
| View state compatibility (existing tests may rely on its absence) | Test failures | Feature-gate view state behind `--vf-view-state` flag initially |
| HTML parser diverges from Salesforce's VF markup parser behavior | Visual mismatches | Document known differences; match Salesforce's HTML output for common patterns |
| Performance of expression evaluation on large pages | Slow page loads | Cache expression parse results per component template |

---

## 8. Success Criteria

1. **Controller Logic Parity:** All existing Visualforce controller tests pass without changes
2. **Page Render:** `GET /apex/{name}` returns valid HTML with controller expression evaluation
3. **Form Postback:** Submitting a form with view state executes the action method and re-renders
4. **Standard Components:** 28 core components render with Salesforce-compatible HTML structure
5. **Custom Components:** `c:MyComponent` resolves `.component` files and instantiates controllers
6. **Test Integration:** `glade test` can assert on rendered page output
7. **Dev Mode:** `glade dev vf` provides hot-reload Visualforce development with error overlay

---

## Appendix A: Key Files Reference

| File | Role in Visualforce Support |
|------|---------------------------|
| `internal/visualforce/visualforce.go` | Page/component metadata parsing, merge reference extraction |
| `internal/visualforce/component_references.go` | Standard component name catalog (173 entries) |
| `internal/vm/ui_invocation.go` | `InvokeVisualforceAction()` — controller action testing |
| `internal/vm/platform_apexpages_formula.go` | StandardController, StandardSetController, ApexPages.Action, FormulaBuilder |
| `internal/vm/visualforce_validation_runtime.go` | DML error → ApexPages.Message conversion |
| `internal/vm/vm.go` | PageReference construction, page message helpers |
| `internal/vm/dispatch.go` | `ApexPages.currentPage()`, `Test.setCurrentPage()`, `Test.invokePage()` |
| `internal/vm/platform_passive_members.go` | PageReference member dispatch; `getContent` → UnsupportedFeature |
| `internal/vm/runtime_state.go` | VM state fields: currentPage, pageMessages, pageReferences |
| `internal/server/server.go` | REST API routing (where `/apex/` endpoint would go) |
| `internal/server/rest_apex.go` | Apex REST dispatch pattern (template for Visualforce action dispatch) |
| `internal/server/source_metadata.go` | ApexPage/ApexComponent Tooling API types |
| `internal/resource/resource.go` | ApexPage SObject creation, markup loading |
| `internal/playground/server.go` | Web playground server (pattern for Visualforce dev UI) |
| `internal/apextest/runner.go` | Test runner with Visualforce page registration |
| `glade-tools/internal/perfscan/metadata_scan.go` | Performance plugin scanner — VF entry points |
| `internal/project/project.go` | File discovery for `.page` and `.component` |
| `internal/ir/ir.go` | Intermediate representation (for expression compilation if needed) |
| `docs/STDLIB_COVERAGE.md` | ApexPages coverage status |
| `docs/POST_PARITY_TODO.md` | Visualforce as listed future work item |
