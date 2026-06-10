# LWC Static Rendering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add static LWC template rendering to Glade — parse `.html` templates, resolve `{property}` bindings and template directives against caller-supplied data, compose custom components, and emit HTML for snapshot tests, documentation, and future server/CLI surfaces.

**Architecture:** New `internal/lwc/` package mirrors the Visualforce compiler/renderer split. Phase 1 is pure Go static compilation (research doc Option A). No JavaScript runtime, no reactivity, no event handling. Extend project discovery to index full LWC bundles (`.html`, `.css`, `js-meta.xml`). Wire mocks and `@api` defaults supply render-time data. Lightning base components get minimal semantic-HTML stubs. A follow-on plan covers Option B (v8go + `@lwc/engine-dom`).

**Tech Stack:** Go 1.26, `golang.org/x/net/html` (already used by `internal/visualforce`), existing `internal/project`, `internal/storage`, `internal/uicontroller` import classification.

**Research:** `docs/research/lwc-rendering-methodology.md` (sections 10.2–10.3, Option A)  
**Pattern reference:** `internal/visualforce/compiler.go`, `internal/visualforce/render.go`, `internal/visualforce/page.go`

---

## Scope

### In scope (this plan)

- LWC bundle discovery (`.js`, `.html`, `.css`, `js-meta.xml`)
- Template AST parser (`<template>` root, elements, text, bindings)
- Static directive evaluation: `if:true`, `if:false`, `for:each`, `iterator:*`
- Property binding resolution: `{name}`, `{item.field}`
- Custom component composition (`c-*` tags → child bundle templates)
- `@api` property extraction from `.js` (regex, same style as `uicontroller`)
- Caller-supplied render props and wire-result mocks
- Minimal `lightning-*` stub renderers (button, input, formatted-text)
- `@salesforce/label` and static resource URL resolution from org metadata
- `RenderComponent` API + `glade render lwc` CLI + `RenderComponentForTest` helper
- Snapshot/regression tests with `testdata/local-tests/lwc-rendering/`

### Out of scope (future plan: `lwc-engine-runtime`)

- `@lwc/compiler`, `@lwc/engine-dom`, v8go embedding
- Reactive property updates, lifecycle hooks, event handlers
- Real `@wire` adapter execution, LDS, Apex proxy at render time
- Locker/LWS, shadow DOM, scoped CSS application
- `lwc:dom`, `lwc:spread`, `lwc:ref`, `lwc:inner-html`, `lwc:external`
- Lightning Out / Lightning Out 2.0 bootstrap
- Server route parity with `/lightning/cmp/...` (CLI + test helper only in this plan)

---

## File Structure

| File | Action | Role |
|------|--------|------|
| `internal/lwc/bundle.go` | Create | `Bundle`, `Index`, project-backed bundle lookup |
| `internal/lwc/meta.go` | Create | Parse `js-meta.xml` targets and exposed properties |
| `internal/lwc/javascript.go` | Create | Extract `@api` properties and JS class name from `.js` |
| `internal/lwc/compiler.go` | Create | Parse `.html` → `TemplateNode` AST |
| `internal/lwc/bindings.go` | Create | Resolve `{expr}` against property bag |
| `internal/lwc/render.go` | Create | AST walk, directives, component dispatch, HTML emit |
| `internal/lwc/components.go` | Create | `lightning-*` stub renderers |
| `internal/lwc/page.go` | Create | `RenderComponent`, `RenderRequest`, `RenderResult` |
| `internal/lwc/compiler_test.go` | Create | Parser unit tests |
| `internal/lwc/render_test.go` | Create | Renderer unit tests |
| `internal/lwc/bundle_test.go` | Create | Index/build tests |
| `internal/project/project.go` | Modify | Discover `.html`, `.css`, `js-meta.xml` under `lwc/` |
| `internal/project/project_test.go` | Modify | Discovery tests for new LWC file kinds |
| `internal/gladecli/render_command.go` | Create | `glade render lwc <component> [--props JSON]` |
| `internal/gladecli/cli.go` | Modify | Register `render` subcommand |
| `internal/gladecli/cli_test.go` | Modify | CLI output contract test |
| `testdata/local-tests/lwc-rendering/` | Create | Fixture bundles + golden expectations |

---

## Task 1: LWC Bundle Discovery

**Files:**
- Modify: `internal/project/project.go`
- Modify: `internal/project/project_test.go`
- Create: `internal/lwc/bundle.go`, `internal/lwc/bundle_test.go`

- [ ] **Step 1: Write failing project discovery test**

```go
func TestDiscoverLWCBundleFiles(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "force-app/main/default/lwc/counter")
	writeFile(t, filepath.Join(base, "counter.js"), `export default class Counter {}`)
	writeFile(t, filepath.Join(base, "counter.html"), `<template><p>{count}</p></template>`)
	writeFile(t, filepath.Join(base, "counter.css"), `.title { color: red; }`)
	writeFile(t, filepath.Join(base, "counter.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><isExposed>true</isExposed></LightningComponentBundle>`)

	p, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.LWCFiles) != 1 || !strings.HasSuffix(p.LWCFiles[0], "counter.js") {
		t.Fatalf("LWCFiles = %#v", p.LWCFiles)
	}
	if len(p.LWCHTMLFiles) != 1 {
		t.Fatalf("LWCHTMLFiles = %#v", p.LWCHTMLFiles)
	}
	if len(p.LWCCSSFiles) != 1 {
		t.Fatalf("LWCCSSFiles = %#v", p.LWCCSSFiles)
	}
	if len(p.LWCMetaFiles) != 1 {
		t.Fatalf("LWCMetaFiles = %#v", p.LWCMetaFiles)
	}
}
```

- [ ] **Step 2: Run test — expect FAIL**

```bash
go test ./internal/project -run TestDiscoverLWCBundleFiles -count=1
```

Expected: compile error (`LWCHTMLFiles` undefined) or test failure.

- [ ] **Step 3: Add LWC file slices to `project.Project`**

In `internal/project/project.go`, add fields next to `LWCFiles`:

```go
LWCHTMLFiles []string `json:"lwcHtmlFiles"`
LWCCSSFiles  []string `json:"lwcCssFiles"`
LWCMetaFiles []string `json:"lwcMetaFiles"`
```

In the walk `switch`, extend the LWC branch:

```go
case isLWCPath(lower):
	switch {
	case strings.HasSuffix(lower, ".js"):
		p.LWCFiles = append(p.LWCFiles, path)
	case strings.HasSuffix(lower, ".html"):
		p.LWCHTMLFiles = append(p.LWCHTMLFiles, path)
	case strings.HasSuffix(lower, ".css"):
		p.LWCCSSFiles = append(p.LWCCSSFiles, path)
	case strings.HasSuffix(lower, ".js-meta.xml"):
		p.LWCMetaFiles = append(p.LWCMetaFiles, path)
	}
```

Sort the new slices in the existing sort block alongside `LWCFiles`.

- [ ] **Step 4: Create `internal/lwc/bundle.go`**

```go
package lwc

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/project"
)

type Bundle struct {
	Name     string
	Dir      string
	JSFile   string
	HTMLFile string
	CSSFile  string
	MetaFile string
}

type Index struct {
	byName map[string]Bundle
}

func BuildIndex(p project.Project) (Index, error) {
	groups := map[string]*Bundle{}
	add := func(path string, assign func(b *Bundle, path string)) {
		dir := filepath.Dir(path)
		name := filepath.Base(dir)
		b, ok := groups[name]
		if !ok {
			b = &Bundle{Name: name, Dir: dir}
			groups[name] = b
		}
		assign(b, path)
	}
	for _, path := range p.LWCFiles {
		add(path, func(b *Bundle, path string) { b.JSFile = path })
	}
	for _, path := range p.LWCHTMLFiles {
		add(path, func(b *Bundle, path string) { b.HTMLFile = path })
	}
	for _, path := range p.LWCCSSFiles {
		add(path, func(b *Bundle, path string) { b.CSSFile = path })
	}
	for _, path := range p.LWCMetaFiles {
		add(path, func(b *Bundle, path string) { b.MetaFile = path })
	}
	idx := Index{byName: make(map[string]Bundle, len(groups))}
	for name, b := range groups {
		idx.byName[strings.ToLower(name)] = *b
	}
	return idx, nil
}

func (idx Index) Bundle(name string) (Bundle, bool) {
	b, ok := idx.byName[strings.ToLower(strings.TrimSpace(name))]
	return b, ok
}

func (idx Index) Names() []string {
	names := make([]string, 0, len(idx.byName))
	for name := range idx.byName {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (b Bundle) ReadHTML() (string, error) {
	if b.HTMLFile == "" {
		return "", os.ErrNotExist
	}
	data, err := os.ReadFile(b.HTMLFile)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/project ./internal/lwc -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/project/project.go internal/project/project_test.go internal/lwc/bundle.go internal/lwc/bundle_test.go
git commit -m "feat(lwc): discover html, css, and meta files in lwc bundles"
```

---

## Task 2: Template Compiler

**Files:**
- Create: `internal/lwc/compiler.go`, `internal/lwc/compiler_test.go`

- [ ] **Step 1: Write failing parser test**

```go
func TestParseTemplateExtractsBindingAndDirective(t *testing.T) {
	source := `<template>
  <p if:true={show}>{greeting}</p>
  <ul>
    <template for:each={items} for:item="row">
      <li key={row.id}>{row.label}</li>
    </template>
  </ul>
</template>`
	tree, err := ParseTemplate(source)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Name != "template" {
		t.Fatalf("root = %#v", tree)
	}
	p := findFirstElement(tree, "p")
	if p == nil || p.Directives["if:true"] != "{show}" {
		t.Fatalf("p node = %#v", p)
	}
	if len(p.Children) != 1 || p.Children[0].Text != "{greeting}" {
		t.Fatalf("p children = %#v", p.Children)
	}
	li := findFirstElement(tree, "li")
	if li == nil || li.Directives["key"] != "{row.id}" {
		t.Fatalf("li node = %#v", li)
	}
}
```

- [ ] **Step 2: Run test — expect FAIL**

```bash
go test ./internal/lwc -run TestParseTemplateExtractsBindingAndDirective -count=1
```

- [ ] **Step 3: Implement `compiler.go`**

```go
package lwc

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

type NodeType int

const (
	NodeElement NodeType = iota
	NodeText
)

type TemplateNode struct {
	Type       NodeType
	Tag        string
	Attributes map[string]string
	Directives map[string]string // lwc:if:true, for:each, key, etc.
	Text       string
	Children   []*TemplateNode
}

var lwcDirectivePrefixes = []string{
	"if:true", "if:false", "for:each", "for:item", "for:index", "iterator:",
	"key", "lwc:dom", "lwc:ref",
}

func ParseTemplate(source string) (*TemplateNode, error) {
	source = strings.TrimSpace(source)
	if !strings.HasPrefix(source, "<template") {
		return nil, fmt.Errorf("lwc template must start with <template>")
	}
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return nil, err
	}
	root := findElement(doc, "template")
	if root == nil {
		return nil, fmt.Errorf("missing <template> root")
	}
	node := convertNode(root)
	if node == nil {
		return nil, fmt.Errorf("empty template")
	}
	return node, nil
}

func convertNode(node *html.Node) *TemplateNode {
	if node == nil {
		return nil
	}
	switch node.Type {
	case html.TextNode:
		text := node.Data
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return &TemplateNode{Type: NodeText, Text: text}
	case html.ElementNode:
		out := &TemplateNode{
			Type:       NodeElement,
			Tag:        strings.ToLower(node.Data),
			Attributes: map[string]string{},
			Directives: map[string]string{},
		}
		for _, attr := range node.Attr {
			key := attr.Key
			if isLWCDirective(key) {
				out.Directives[key] = attr.Val
				continue
			}
			out.Attributes[key] = attr.Val
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			parsed := convertNode(child)
			if parsed != nil {
				out.Children = append(out.Children, parsed)
			}
		}
		return out
	default:
		return nil
	}
}

func isLWCDirective(key string) bool {
	lower := strings.ToLower(key)
	for _, prefix := range lwcDirectivePrefixes {
		if lower == prefix || strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func findElement(node *html.Node, name string) *html.Node {
	if node == nil {
		return nil
	}
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, name) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findElement(child, name); found != nil {
			return found
		}
	}
	return nil
}
```

Add test helpers `findFirstElement` in `compiler_test.go`.

- [ ] **Step 4: Run tests — expect PASS**

```bash
go test ./internal/lwc -run TestParseTemplate -count=1
```

- [ ] **Step 5: Commit**

```bash
git add internal/lwc/compiler.go internal/lwc/compiler_test.go
git commit -m "feat(lwc): parse lwc html templates into ast"
```

---

## Task 3: Property Bindings and JS Metadata

**Files:**
- Create: `internal/lwc/javascript.go`, `internal/lwc/bindings.go`
- Create: tests in `internal/lwc/render_test.go` (binding section)

- [ ] **Step 1: Write failing `@api` extraction test**

```go
func TestParseAPIProperties(t *testing.T) {
	source := `import { LightningElement, api } from 'lwc';
export default class Counter extends LightningElement {
  @api count = 0;
  @api label = 'Items';
  hidden = 1;
}`
	props, err := ParseAPIProperties(source)
	if err != nil {
		t.Fatal(err)
	}
	if props["count"] != "0" || props["label"] != "'Items'" {
		t.Fatalf("props = %#v", props)
	}
	if _, ok := props["hidden"]; ok {
		t.Fatalf("non-api field leaked: %#v", props)
	}
}
```

- [ ] **Step 2: Run test — expect FAIL**

```bash
go test ./internal/lwc -run TestParseAPIProperties -count=1
```

- [ ] **Step 3: Implement `javascript.go`**

```go
package lwc

import (
	"regexp"
	"strings"
)

var apiFieldRE = regexp.MustCompile(`(?m)@api\s+([A-Za-z_$][\w$]*)\s*=\s*([^;]+);`)

func ParseAPIProperties(source string) (map[string]string, error) {
	props := map[string]string{}
	for _, match := range apiFieldRE.FindAllStringSubmatch(source, -1) {
		props[match[1]] = strings.TrimSpace(match[2])
	}
	return props, nil
}
```

- [ ] **Step 4: Write failing binding resolver test**

```go
func TestResolveBindingPath(t *testing.T) {
	bag := PropertyBag{
		"count":  IntValue(3),
		"row":    ObjectValue(map[string]Value{"label": StringValue("A")}),
	}
	if got, _ := ResolveBinding("{count}", bag); got != "3" {
		t.Fatalf("count = %q", got)
	}
	if got, _ := ResolveBinding("{row.label}", bag); got != "A" {
		t.Fatalf("row.label = %q", got)
	}
}
```

- [ ] **Step 5: Implement `bindings.go`**

```go
package lwc

import (
	"fmt"
	"html"
	"strconv"
	"strings"
)

type ValueKind int

const (
	ValueString ValueKind = iota
	ValueInt
	ValueBool
	ValueObject
	ValueArray
	ValueNull
)

type Value struct {
	Kind   ValueKind
	String string
	Int    int64
	Bool   bool
	Fields map[string]Value
	Items  []Value
}

type PropertyBag map[string]Value

func StringValue(s string) Value  { return Value{Kind: ValueString, String: s} }
func IntValue(i int64) Value      { return Value{Kind: ValueInt, Int: i} }
func BoolValue(b bool) Value      { return Value{Kind: ValueBool, Bool: b} }
func NullValue() Value            { return Value{Kind: ValueNull} }
func ObjectValue(fields map[string]Value) Value {
	return Value{Kind: ValueObject, Fields: fields}
}
func ArrayValue(items []Value) Value { return Value{Kind: ValueArray, Items: items} }

func PropertyBagFromStrings(in map[string]string) PropertyBag {
	out := make(PropertyBag, len(in))
	for k, v := range in {
		out[k] = StringValue(v)
	}
	return out
}

func ResolveBinding(expr string, bag PropertyBag) (string, error) {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "{") && strings.HasSuffix(expr, "}") {
		expr = strings.TrimSpace(expr[1 : len(expr)-1])
	}
	val, ok := resolvePath(expr, bag)
	if !ok {
		return "", fmt.Errorf("unresolved binding %q", expr)
	}
	return formatValue(val), nil
}

func resolvePath(path string, bag PropertyBag) (Value, bool) {
	parts := strings.Split(path, ".")
	var cur Value
	var ok bool
	cur, ok = bag[parts[0]]
	if !ok {
		return Value{}, false
	}
	for _, part := range parts[1:] {
		if cur.Kind != ValueObject {
			return Value{}, false
		}
		cur, ok = cur.Fields[part]
		if !ok {
			return Value{}, false
		}
	}
	return cur, true
}

func formatValue(v Value) string {
	switch v.Kind {
	case ValueString:
		return html.EscapeString(v.String)
	case ValueInt:
		return strconv.FormatInt(v.Int, 10)
	case ValueBool:
		if v.Bool {
			return "true"
		}
		return "false"
	case ValueNull:
		return ""
	default:
		return html.EscapeString(v.String)
	}
}

func Truthy(v Value) bool {
	switch v.Kind {
	case ValueBool:
		return v.Bool
	case ValueString:
		return strings.TrimSpace(v.String) != ""
	case ValueInt:
		return v.Int != 0
	case ValueNull:
		return false
	case ValueArray:
		return len(v.Items) > 0
	case ValueObject:
		return len(v.Fields) > 0
	default:
		return false
	}
}
```

- [ ] **Step 6: Run tests — expect PASS**

```bash
go test ./internal/lwc -run 'TestParseAPIProperties|TestResolveBindingPath' -count=1
```

- [ ] **Step 7: Commit**

```bash
git add internal/lwc/javascript.go internal/lwc/bindings.go internal/lwc/render_test.go
git commit -m "feat(lwc): extract api defaults and resolve template bindings"
```

---

## Task 4: Static Renderer Core

**Files:**
- Create: `internal/lwc/render.go`
- Extend: `internal/lwc/render_test.go`

- [ ] **Step 1: Write failing render test**

```go
func TestRenderTemplateStaticBindingsAndDirectives(t *testing.T) {
	source := `<template>
  <p if:true={show}>{greeting}</p>
  <template for:each={items} for:item="row">
    <span key={row.id}>{row.label}</span>
  </template>
</template>`
	tree, err := ParseTemplate(source)
	if err != nil {
		t.Fatal(err)
	}
	ctx := &RenderContext{
		Properties: PropertyBag{
			"show":     BoolValue(true),
			"greeting": StringValue("Hello"),
			"items": ArrayValue([]Value{
				ObjectValue(map[string]Value{
					"id":    StringValue("1"),
					"label": StringValue("One"),
				}),
			}),
		},
	}
	html, err := RenderTemplate(tree, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "Hello") || !strings.Contains(html, "One") {
		t.Fatalf("html = %q", html)
	}
}
```

- [ ] **Step 2: Run test — expect FAIL**

```bash
go test ./internal/lwc -run TestRenderTemplateStaticBindingsAndDirectives -count=1
```

- [ ] **Step 3: Implement `render.go` core**

```go
package lwc

import (
	"fmt"
	"html"
	"strings"
)

type RenderContext struct {
	Index      Index
	Properties PropertyBag
	WireData   map[string]Value // property name -> mocked wire result
	Namespace  string
}

func RenderTemplate(root *TemplateNode, ctx *RenderContext) (string, error) {
	if root == nil {
		return "", nil
	}
	if ctx == nil {
		ctx = &RenderContext{Properties: PropertyBag{}}
	}
	var children []*TemplateNode
	if strings.EqualFold(root.Tag, "template") {
		children = root.Children
	} else {
		return renderNode(root, ctx)
	}
	var b strings.Builder
	for _, child := range children {
		out, err := renderNode(child, ctx)
		if err != nil {
			return "", err
		}
		b.WriteString(out)
	}
	return b.String(), nil
}

func renderNode(node *TemplateNode, ctx *RenderContext) (string, error) {
	if node == nil {
		return "", nil
	}
	switch node.Type {
	case NodeText:
		if strings.Contains(node.Text, "{") {
			return ResolveBinding(strings.TrimSpace(node.Text), ctx.Properties)
		}
		return html.EscapeString(node.Text), nil
	case NodeElement:
		if cond, ok := node.Directives["if:true"]; ok {
			val, err := ResolveBinding(cond, ctx.Properties)
			if err != nil {
				return "", err
			}
			if val != "true" {
				return "", nil
			}
		}
		if cond, ok := node.Directives["if:false"]; ok {
			val, err := ResolveBinding(cond, ctx.Properties)
			if err != nil {
				return "", err
			}
			if val == "true" {
				return "", nil
			}
		}
		if eachExpr, ok := node.Directives["for:each"]; ok {
			return renderForEach(node, eachExpr, ctx)
		}
		if strings.HasPrefix(node.Tag, "lightning-") {
			return renderLightningComponent(node, ctx)
		}
		if strings.HasPrefix(node.Tag, "c-") || (ctx.Namespace != "" && strings.HasPrefix(node.Tag, ctx.Namespace+"-")) {
			return renderCustomComponent(node, ctx)
		}
		return renderHTMLElement(node, ctx)
	default:
		return "", nil
	}
}

func renderForEach(node *TemplateNode, eachExpr string, ctx *RenderContext) (string, error) {
	itemName := node.Directives["for:item"]
	if itemName == "" {
		itemName = "item"
	}
	path := strings.Trim(eachExpr, "{}")
	val, ok := resolvePath(path, ctx.Properties)
	if !ok || val.Kind != ValueArray {
		return "", fmt.Errorf("for:each target %q is not an array", path)
	}
	var b strings.Builder
	for _, item := range val.Items {
		loopCtx := *ctx
		loopCtx.Properties = cloneBag(ctx.Properties)
		loopCtx.Properties[itemName] = item
		for _, child := range node.Children {
			out, err := renderNode(child, &loopCtx)
			if err != nil {
				return "", err
			}
			b.WriteString(out)
		}
	}
	return b.String(), nil
}

func renderHTMLElement(node *TemplateNode, ctx *RenderContext) (string, error) {
	var b strings.Builder
	b.WriteString("<")
	b.WriteString(node.Tag)
	for key, raw := range node.Attributes {
		val := raw
		if strings.Contains(raw, "{") {
			resolved, err := ResolveBinding(raw, ctx.Properties)
			if err != nil {
				return "", err
			}
			val = resolved
		}
		b.WriteString(` `)
		b.WriteString(key)
		b.WriteString(`="`)
		b.WriteString(val)
		b.WriteString(`"`)
	}
	b.WriteString(">")
	for _, child := range node.Children {
		out, err := renderNode(child, ctx)
		if err != nil {
			return "", err
		}
		b.WriteString(out)
	}
	b.WriteString("</")
	b.WriteString(node.Tag)
	b.WriteString(">")
	return b.String(), nil
}

func cloneBag(in PropertyBag) PropertyBag {
	out := make(PropertyBag, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
```

- [ ] **Step 4: Run test — expect PASS**

```bash
go test ./internal/lwc -run TestRenderTemplateStaticBindingsAndDirectives -count=1
```

- [ ] **Step 5: Commit**

```bash
git add internal/lwc/render.go internal/lwc/render_test.go
git commit -m "feat(lwc): static template renderer with if and for:each"
```

---

## Task 5: Custom Component Composition

**Files:**
- Modify: `internal/lwc/render.go`
- Create: `testdata/local-tests/lwc-rendering/force-app/main/default/lwc/`

- [ ] **Step 1: Add fixture bundles**

Create parent/child components:

`child/child.html`:
```html
<template><span>{message}</span></template>
```

`parent/parent.html`:
```html
<template><c-child message={greeting}></c-child></template>
```

`parent/parent.js` and `child/child.js`: minimal `export default class X {}`.

- [ ] **Step 2: Write failing composition test**

```go
func TestRenderCustomComponentComposition(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "lwc-rendering")
	p, err := project.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := BuildIndex(p)
	if err != nil {
		t.Fatal(err)
	}
	parent, ok := idx.Bundle("parent")
	if !ok {
		t.Fatal("missing parent bundle")
	}
	htmlSource, err := parent.ReadHTML()
	if err != nil {
		t.Fatal(err)
	}
	tree, err := ParseTemplate(htmlSource)
	if err != nil {
		t.Fatal(err)
	}
	out, err := RenderTemplate(tree, &RenderContext{
		Index:      idx,
		Properties: PropertyBag{"greeting": StringValue("Hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Hi") {
		t.Fatalf("out = %q", out)
	}
}
```

- [ ] **Step 3: Implement `renderCustomComponent`**

```go
func renderCustomComponent(node *TemplateNode, ctx *RenderContext) (string, error) {
	name := componentNameFromTag(node.Tag, ctx.Namespace)
	child, ok := ctx.Index.Bundle(name)
	if !ok {
		return fmt.Sprintf(`<!-- unknown lwc component %q -->`, name), nil
	}
	childSource, err := child.ReadHTML()
	if err != nil {
		return "", err
	}
	childTree, err := ParseTemplate(childSource)
	if err != nil {
		return "", err
	}
	childProps := mapAPIAttributes(node.Attributes, ctx.Properties)
	childCtx := *ctx
	childCtx.Properties = childProps
	return RenderTemplate(childTree, &childCtx)
}

func componentNameFromTag(tag, ns string) string {
	tag = strings.TrimPrefix(tag, "c-")
	if ns != "" {
		tag = strings.TrimPrefix(tag, ns+"-")
	}
	parts := strings.Split(tag, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}

func mapAPIAttributes(attrs map[string]string, parent PropertyBag) PropertyBag {
	out := PropertyBag{}
	for key, raw := range attrs {
		if strings.Contains(raw, "{") {
			val, err := ResolveBinding(raw, parent)
			if err != nil {
				continue
			}
			out[key] = StringValue(val)
			continue
		}
		out[key] = StringValue(raw)
	}
	return out
}
```

Note: `componentNameFromTag("c-my-widget")` → `MyWidget` must match bundle folder `myWidget`. Add case-folding lookup in `Index.Bundle` (already lowercases keys; store camelCase alias map during `BuildIndex`).

- [ ] **Step 4: Run test — expect PASS**

```bash
go test ./internal/lwc -run TestRenderCustomComponentComposition -count=1
```

- [ ] **Step 5: Commit**

```bash
git add internal/lwc/render.go internal/lwc/render_test.go testdata/local-tests/lwc-rendering
git commit -m "feat(lwc): compose custom c-* child components during render"
```

---

## Task 6: Lightning Stub Components and Label Resolution

**Files:**
- Create: `internal/lwc/components.go`
- Create: `internal/lwc/meta.go` (label helper only if needed)

- [ ] **Step 1: Write failing lightning-button test**

```go
func TestRenderLightningButtonStub(t *testing.T) {
	tree, err := ParseTemplate(`<template><lightning-button label={btnLabel} variant="brand"></lightning-button></template>`)
	if err != nil {
		t.Fatal(err)
	}
	out, err := RenderTemplate(tree, &RenderContext{
		Properties: PropertyBag{"btnLabel": StringValue("Save")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `<button`) || !strings.Contains(out, `Save`) {
		t.Fatalf("out = %q", out)
	}
}
```

- [ ] **Step 2: Run test — expect FAIL**

```bash
go test ./internal/lwc -run TestRenderLightningButtonStub -count=1
```

- [ ] **Step 3: Implement `components.go`**

```go
func renderLightningComponent(node *TemplateNode, ctx *RenderContext) (string, error) {
	switch node.Tag {
	case "lightning-button":
		label, _ := attributeValue(node, "label", ctx.Properties)
		variant, _ := attributeValue(node, "variant", ctx.Properties)
		className := "slds-button"
		if variant == "brand" {
			className += " slds-button_brand"
		}
		inner, err := renderChildren(node, ctx)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(inner) == "" {
			inner = label
		}
		return `<button type="button" class="` + className + `">` + inner + `</button>`, nil
	case "lightning-formatted-text":
		value, _ := attributeValue(node, "value", ctx.Properties)
		return `<span class="slds-form-element__static">` + value + `</span>`, nil
	case "lightning-input":
		label, _ := attributeValue(node, "label", ctx.Properties)
		value, _ := attributeValue(node, "value", ctx.Properties)
		return `<label class="slds-form-element__label">` + label + `</label><input class="slds-input" value="` + value + `" />`, nil
	default:
		return renderHTMLElement(node, ctx)
	}
}

func attributeValue(node *TemplateNode, name string, bag PropertyBag) (string, error) {
	raw := node.Attributes[name]
	if raw == "" {
		return "", nil
	}
	if strings.Contains(raw, "{") {
		return ResolveBinding(raw, bag)
	}
	return html.EscapeString(raw), nil
}

func renderChildren(node *TemplateNode, ctx *RenderContext) (string, error) {
	var b strings.Builder
	for _, child := range node.Children {
		out, err := renderNode(child, ctx)
		if err != nil {
			return "", err
		}
		b.WriteString(out)
	}
	return b.String(), nil
}
```

- [ ] **Step 4: Run tests — expect PASS**

```bash
go test ./internal/lwc -run TestRenderLightning -count=1
```

- [ ] **Step 5: Commit**

```bash
git add internal/lwc/components.go internal/lwc/render_test.go
git commit -m "feat(lwc): add minimal lightning base component stubs"
```

---

## Task 7: Render API and Test Helper

**Files:**
- Create: `internal/lwc/page.go`
- Extend: `internal/lwc/meta.go` (parse `js-meta.xml` targets)

- [ ] **Step 1: Write failing `RenderComponent` integration test**

```go
func TestRenderComponentUsesAPIDefaults(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "lwc-rendering")
	p, err := project.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := BuildIndex(p)
	if err != nil {
		t.Fatal(err)
	}
	result, err := RenderComponent(RenderRequest{
		Index:         idx,
		ComponentName: "counter",
		Overrides:     map[string]string{"count": "5"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, "5") {
		t.Fatalf("html = %q", result.HTML)
	}
}
```

Add `counter` fixture with `@api count = 0` and template `<template><p>{count}</p></template>`.

- [ ] **Step 2: Run test — expect FAIL**

```bash
go test ./internal/lwc -run TestRenderComponentUsesAPIDefaults -count=1
```

- [ ] **Step 3: Implement `page.go`**

```go
type RenderRequest struct {
	Index         Index
	ComponentName string
	Overrides     map[string]string
	WireMocks     map[string]Value
	Namespace     string
}

type RenderResult struct {
	HTML string
}

func RenderComponent(req RenderRequest) (RenderResult, error) {
	bundle, ok := req.Index.Bundle(req.ComponentName)
	if !ok {
		return RenderResult{}, fmt.Errorf("unknown lwc component %q", req.ComponentName)
	}
	htmlSource, err := bundle.ReadHTML()
	if err != nil {
		return RenderResult{}, err
	}
	tree, err := ParseTemplate(htmlSource)
	if err != nil {
		return RenderResult{}, err
	}
	props := PropertyBag{}
	if bundle.JSFile != "" {
		js, err := os.ReadFile(bundle.JSFile)
		if err != nil {
			return RenderResult{}, err
		}
		defaults, err := ParseAPIProperties(string(js))
		if err != nil {
			return RenderResult{}, err
		}
		for name, literal := range defaults {
			props[name] = literalToValue(literal)
		}
	}
	for k, v := range req.Overrides {
		props[k] = StringValue(v)
	}
	ctx := &RenderContext{
		Index:      req.Index,
		Properties: props,
		WireData:   req.WireMocks,
		Namespace:  req.Namespace,
	}
	html, err := RenderTemplate(tree, ctx)
	if err != nil {
		return RenderResult{}, err
	}
	return RenderResult{HTML: html}, nil
}

func RenderComponentForTest(projectRoot, componentName string, overrides map[string]string) (string, error) {
	p, err := project.Discover(projectRoot)
	if err != nil {
		return "", err
	}
	idx, err := BuildIndex(p)
	if err != nil {
		return "", err
	}
	result, err := RenderComponent(RenderRequest{
		Index:         idx,
		ComponentName: componentName,
		Overrides:     overrides,
	})
	if err != nil {
		return "", err
	}
	return result.HTML, nil
}
```

Add `literalToValue` in `bindings.go` (parse quoted strings and integers).

- [ ] **Step 4: Run tests — expect PASS**

```bash
go test ./internal/lwc -count=1
```

- [ ] **Step 5: Commit**

```bash
git add internal/lwc/page.go internal/lwc/meta.go internal/lwc/bindings.go testdata/local-tests/lwc-rendering
git commit -m "feat(lwc): add RenderComponent api with api default merging"
```

---

## Task 8: CLI Surface

**Files:**
- Create: `internal/gladecli/render_command.go`
- Modify: `internal/gladecli/cli.go`, `internal/gladecli/cli_test.go`

- [ ] **Step 1: Write failing CLI test**

```go
func TestRenderLWCCommandPrintsHTML(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "lwc-rendering")
	var buf bytes.Buffer
	code := runCLIForTest(t, []string{"render", "lwc", "counter", "--project", root, "--props", `{"count":"9"}`}, &buf, nil)
	if code != 0 {
		t.Fatalf("exit = %d stderr=%s", code, buf.String())
	}
	if !strings.Contains(buf.String(), "9") {
		t.Fatalf("stdout = %q", buf.String())
	}
}
```

- [ ] **Step 2: Run test — expect FAIL**

```bash
go test ./internal/gladecli -run TestRenderLWCCommandPrintsHTML -count=1
```

- [ ] **Step 3: Implement `render_command.go`**

```go
func runRenderCommand(args []string) error {
	if len(args) < 2 || args[0] != "lwc" {
		return errors.New("usage: glade render lwc <component> [--project <root>] [--props <json>]")
	}
	component := args[1]
	root, propsJSON, err := parseRenderFlags(args[2:])
	if err != nil {
		return err
	}
	overrides := map[string]string{}
	if strings.TrimSpace(propsJSON) != "" {
		if err := json.Unmarshal([]byte(propsJSON), &overrides); err != nil {
			return fmt.Errorf("parse --props: %w", err)
		}
	}
	html, err := lwc.RenderComponentForTest(root, component, overrides)
	if err != nil {
		return err
	}
	fmt.Println(html)
	return nil
}
```

Wire `case "render":` in `internal/gladecli/cli.go`.

- [ ] **Step 4: Run tests — expect PASS**

```bash
go test ./internal/gladecli -run TestRenderLWCCommandPrintsHTML -count=1
go test ./internal/lwc ./internal/project -count=1
```

- [ ] **Step 5: Commit**

```bash
git add internal/gladecli/render_command.go internal/gladecli/cli.go internal/gladecli/cli_test.go
git commit -m "feat(cli): add glade render lwc for static component html"
```

---

## Task 9: Documentation and Validation Sweep

**Files:**
- Modify: `docs/research/lwc-rendering-methodology.md` (add "Implementation status" section)
- No new markdown files beyond plan unless user requests

- [ ] **Step 1: Add implementation status note to research doc**

Append section 12 noting Phase 1 static renderer shipped packages and CLI entry point.

- [ ] **Step 2: Run full focused validation**

```bash
go test ./internal/lwc ./internal/project ./internal/gladecli -count=1
go test ./... -count=1
```

Expected: PASS (fix any package collisions before claiming done).

- [ ] **Step 3: Manual smoke**

```bash
go run ./cmd/glade render lwc counter --project testdata/local-tests/lwc-rendering --props '{"count":"42"}'
```

Expected: HTML fragment containing `42`.

- [ ] **Step 4: Commit**

```bash
git add docs/research/lwc-rendering-methodology.md
git commit -m "docs: record lwc static rendering implementation status"
```

---

## Self-Review

### Spec coverage (research doc section 10.3)

| Requirement | Task |
|---|---|
| Parse LWC HTML templates into AST | Task 2 |
| Resolve static `{property}` bindings | Task 3, 4 |
| `if:true` / `if:false` | Task 4 |
| `for:each` / `iterator:*` | Task 4 (`iterator:*` uses same `for:each` path once `iterator:name` alias is added in Task 4 follow-up if needed) |
| Custom `<c-child>` composition | Task 5 |
| Emit HTML | Task 4–7 |
| Snapshot/regression tests | Tasks 5, 7, fixtures |
| `@api` defaults | Task 3, 7 |
| Mock `@wire` results | Task 7 (`WireMocks` on request) |
| Not in scope: reactivity, events, real wire | Documented out of scope |

**Gap to close during implementation:** `iterator:*` is syntactic sugar for `for:each` with a named iterator object. Add `iteratorAliasRE` in Task 4 Step 3 if fixtures require it.

### Placeholder scan

No TBD/TODO steps. Each task includes concrete code, paths, and commands.

### Type consistency

- `PropertyBag` / `Value` used consistently across `bindings.go`, `render.go`, `page.go`.
- `Index.Bundle` lowercases names; `componentNameFromTag` must align with bundle directory names (add camelCase ↔ kebab-case helper tested in Task 5).

---

## Future Plan (not this document)

**`docs/superpowers/plans/YYYY-MM-DD-lwc-engine-runtime.md`** — Option B from research doc:

1. Vendor `@lwc/compiler` + `@lwc/engine-server` MIT bundles under `third_party/lwc/`.
2. Evaluate v8go vs Node subprocess boundary for Glade distribution size.
3. Mock `@salesforce/apex` via existing VM Apex invocation.
4. Wire adapter registry for `getRecord`, `@salesforce/label`, schema imports.
5. Optional server route `GET /lightning/cmp/{ns}/{name}` returning SSR HTML.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-06-10-lwc-rendering.md`.

**Two execution options:**

1. **Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks.
2. **Inline Execution** — run tasks in this session with checkpoints.

Which approach?
