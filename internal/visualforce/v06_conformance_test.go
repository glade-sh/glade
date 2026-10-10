package visualforce_test

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
	"golang.org/x/net/html"
)

type v06Input struct {
	Name  string            `json:"name"`
	Files map[string]string `json:"files"`
}

type v06Diagnostic struct {
	Resource string `json:"resource"`
	Type     string `json:"type"`
	Message  string `json:"message"`
}

type v06UserContext struct {
	Locale   string `json:"locale"`
	Timezone string `json:"timezone"`
	Language string `json:"language"`
}

type v06Case struct {
	ID          string                     `json:"id"`
	Group       string                     `json:"group"`
	Kind        string                     `json:"kind"`
	Inputs      map[string]v06Input        `json:"inputs"`
	Expected    map[string]string          `json:"expected"`
	Diagnostics map[string][]v06Diagnostic `json:"diagnostics"`
	Owner       string                     `json:"owner,omitempty"`
	Reason      string                     `json:"reason,omitempty"`
}

// TestV06SalesforceConformance uses every captured UI components row at API 59/67:
// 130 metadata rows and 104 runtime rows (100 DOM, two render errors and two
// native metadata rejections). Inputs and answers are exported from the native
// oracle. Captured Apex bodies run through the existing project
// compiler and request VM. The DOM adapter follows the native owned-subtree
// observer, including text, innerHTML, controls and descendant tag order.
// All observed strings and metadata diagnostic type/text compare exactly.
// Only the native runtime observer's fixture-name replacement is applied; JSON
// inside observed text is left untouched. CI needs no browser or Salesforce.
// The request user matches the separately captured org B context. The form
// markup row carries to active Form lifecycle; all matching rows and diagnostics assert.
// GLADE_V06_CAPTURE=1 reports every row without failing on semantic differences;
// GLADE_V06_REPORT selects the complete per-row TSV and match-count output.
func TestV06SalesforceConformance(t *testing.T) {
	t.Run("preserve-body-formula-source", v06PreserveBodyFormulaSource)
	t.Run("preserve-scope-before-attribute", v06PreserveScopeBeforeAttribute)
	capture := os.Getenv("GLADE_V06_CAPTURE") != ""
	data, err := os.ReadFile("testdata/v06_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Source              string                    `json:"source"`
		ContextSource       string                    `json:"context_source"`
		UserContext         map[string]v06UserContext `json:"user_context"`
		VersionGates        []any                     `json:"version_gates"`
		RuntimeVersionGates []any                     `json:"runtime_version_gates"`
		Cases               []v06Case                 `json:"cases"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if table.Source != "Owned UI component observations at API 59.0 and 67.0." || len(table.Cases) != 234 ||
		table.VersionGates == nil || table.RuntimeVersionGates == nil || len(table.VersionGates)+len(table.RuntimeVersionGates) != 0 {
		t.Fatalf("UI components requires all 234 final native cases and both empty version gates, got %d at %s", len(table.Cases), table.Source)
	}
	versions := []string{"59.0", "67.0"}
	if table.ContextSource != "Owned locale, timezone and language observations at API 59.0 and 67.0." || len(table.UserContext) != len(versions) {
		t.Fatal("UI components requires the captured API59/67 org B user context")
	}
	counts, seen := map[string]int{}, map[string]bool{}
	nativeKinds := map[string]map[string]int{}
	for _, api := range versions {
		actor := table.UserContext[api]
		if actor.Locale == "" || actor.Timezone == "" || actor.Language == "" {
			t.Fatalf("UI components API %s requires all captured user context fields", api)
		}
		nativeKinds[api] = map[string]int{}
	}
	for _, c := range table.Cases {
		if c.ID == "" || seen[c.ID] || (c.Kind != "compile" && c.Kind != "runtime") || c.Group == "" {
			t.Fatalf("invalid or duplicate UI components case %q (%s)", c.ID, c.Kind)
		}
		seen[c.ID] = true
		counts[c.Kind]++
		if c.Owner != "" || c.Reason != "" {
			if c.ID != "composition_body_form_runtime" || c.Kind != "runtime" || c.Owner != "Form lifecycle" || c.Reason == "" {
				t.Fatalf("%s: only the active Form lifecycle form markup difference can carry", c.ID)
			}
		}
		for _, api := range versions {
			input := c.Inputs[api]
			if input.Name == "" || len(input.Files) < 9 || input.Files["sfdx-project.json"] == "" ||
				input.Files["force-app/main/default/pages/"+input.Name+".page"] == "" ||
				input.Files["force-app/main/default/pages/"+input.Name+".page-meta.xml"] == "" {
				t.Fatalf("%s: incomplete native input at API %s", c.ID, api)
			}
			answer := c.Expected[api]
			diagnostics, ok := c.Diagnostics[api]
			if !ok || diagnostics == nil {
				t.Fatalf("%s: missing diagnostic list at API %s", c.ID, api)
			}
			if c.Kind == "compile" {
				if (answer != "COMPILE_OK" && answer != "COMPILE_ERROR") ||
					(answer == "COMPILE_OK" && len(diagnostics) != 0) || (answer == "COMPILE_ERROR" && len(diagnostics) == 0) {
					t.Fatalf("%s: inconsistent metadata answer at API %s", c.ID, api)
				}
				for _, d := range diagnostics {
					if d.Resource == "" || d.Type == "" || d.Message == "" {
						t.Fatalf("%s: incomplete native diagnostic at API %s", c.ID, api)
					}
				}
			} else {
				var native map[string]any
				if !strings.HasPrefix(answer, "NATIVE|") || json.Unmarshal([]byte(strings.TrimPrefix(answer, "NATIVE|")), &native) != nil ||
					v06Native(t, native) != answer || len(diagnostics) != 0 {
					t.Fatalf("%s: invalid exact native answer at API %s", c.ID, api)
				}
				kind, _ := native["kind"].(string)
				nativeKinds[api][kind]++
				switch kind {
				case "dom":
					if len(native) != 5 || native["text"] == nil || native["html"] == nil || native["controls"] == nil || native["tags"] == nil {
						t.Fatalf("%s: incomplete native DOM at API %s", c.ID, api)
					}
				case "native-render-error", "native-compile-rejection":
					values, ok := native["diagnostics"].([]any)
					if !ok || len(values) == 0 || (kind == "native-compile-rejection" && native["value"] != "COMPILE_ERROR") {
						t.Fatalf("%s: incomplete native error at API %s", c.ID, api)
					}
				default:
					t.Fatalf("%s: unsupported native outcome %q", c.ID, kind)
				}
			}
		}
	}
	if counts["compile"] != 130 || counts["runtime"] != 104 {
		t.Fatalf("UI components requires 130 metadata and 104 runtime rows, got %v", counts)
	}
	for _, api := range versions {
		kinds := nativeKinds[api]
		if kinds["dom"] != 100 || kinds["native-render-error"] != 2 || kinds["native-compile-rejection"] != 2 {
			t.Fatalf("UI components API %s requires all 100 DOM/2 render error/2 rejection rows, got %v", api, kinds)
		}
	}

	var report strings.Builder
	tsv := csv.NewWriter(&report)
	tsv.Comma = '\t'
	writeRow := func(fields ...string) {
		if err := tsv.Write(fields); err != nil {
			t.Fatal(err)
		}
	}
	writeRow("api", "id", "status", "actual", "expected", "group", "diagnostic", "expected_diagnostic", "acceptance_status", "diagnostic_status", "owner", "reason")
	matches, compileMatches, runtimeMatches := 0, 0, 0
	for _, api := range versions {
		apiMatches, apiCompile, apiRuntime, apiAcceptance, apiDiagnostics := 0, 0, 0, 0, 0
		for _, c := range table.Cases {
			root := t.TempDir()
			input := c.Inputs[api]
			for path, body := range input.Files {
				if filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "..") {
					t.Fatalf("%s: invalid fixture path %q", c.ID, path)
				}
				fullPath := filepath.Join(root, path)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(fullPath, []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, diagnostics := v06Execute(t, root, input.Name, c, table.UserContext[api])
			want := c.Expected[api]
			actualDiagnostic, expectedDiagnostic := v06Encode(t, diagnostics), v06Encode(t, c.Diagnostics[api])
			acceptanceMatch := got == want
			diagnosticMatch := c.Kind != "compile" || actualDiagnostic == expectedDiagnostic
			status, acceptanceStatus, diagnosticStatus := "MISMATCH", "MISMATCH", "NOT_APPLICABLE"
			if acceptanceMatch {
				acceptanceStatus = "MATCH"
				if c.Kind == "compile" {
					apiAcceptance++
				}
			}
			if c.Kind == "compile" {
				diagnosticStatus = "MISMATCH"
				if diagnosticMatch {
					diagnosticStatus = "MATCH"
					apiDiagnostics++
				}
			}
			if acceptanceMatch && diagnosticMatch {
				status = "MATCH"
				apiMatches++
				if c.Kind == "compile" {
					apiCompile++
				} else {
					apiRuntime++
				}
			}
			// Temporary paths are hidden only in the report, after exact equality.
			writeRow(api, c.ID, status, strings.ReplaceAll(got, root, "<fixture>"), want, c.Group,
				strings.ReplaceAll(actualDiagnostic, root, "<fixture>"), expectedDiagnostic, acceptanceStatus, diagnosticStatus, c.Owner, c.Reason)
			if c.Owner != "" {
				if acceptanceMatch && diagnosticMatch {
					if !capture {
						t.Errorf("API %s %s now matches; remove the %s carry", api, c.ID, c.Owner)
					}
				} else {
					t.Logf("API %s %s carry %s: %s", api, c.ID, c.Owner, c.Reason)
				}
			} else if !capture && (!acceptanceMatch || !diagnosticMatch) {
				t.Errorf("API %s %s: actual <%s> expected <%s>; diagnostics <%s> expected <%s>", api, c.ID, got, want, actualDiagnostic, expectedDiagnostic)
			}
		}
		writeRow("API_TOTAL", api, fmt.Sprintf("%d/234", apiMatches))
		writeRow("COMPILE_API_TOTAL", api, fmt.Sprintf("%d/130", apiCompile))
		writeRow("COMPILE_ACCEPTANCE_API_TOTAL", api, fmt.Sprintf("%d/130", apiAcceptance))
		writeRow("DIAGNOSTIC_API_TOTAL", api, fmt.Sprintf("%d/130", apiDiagnostics))
		writeRow("RUNTIME_API_TOTAL", api, fmt.Sprintf("%d/104", apiRuntime))
		t.Logf("UI components API %s exact %d/234; metadata %d/130, acceptance %d/130, diagnostics %d/130; runtime %d/104", api, apiMatches, apiCompile, apiAcceptance, apiDiagnostics, apiRuntime)
		matches += apiMatches
		compileMatches += apiCompile
		runtimeMatches += apiRuntime
	}
	writeRow("COMPILE_TOTAL", fmt.Sprintf("%d/260", compileMatches))
	writeRow("RUNTIME_TOTAL", fmt.Sprintf("%d/208", runtimeMatches))
	writeRow("TOTAL", fmt.Sprintf("%d/468", matches))
	t.Logf("UI components exact matches %d/468; metadata %d/260; runtime %d/208", matches, compileMatches, runtimeMatches)
	tsv.Flush()
	if err := tsv.Error(); err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("GLADE_V06_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func v06Execute(t *testing.T, root, name string, c v06Case, actor v06UserContext) (string, []v06Diagnostic) {
	t.Helper()
	diagnostics := []v06Diagnostic{}
	metadata := []map[string]any{}
	addDiagnostic := func(message, diagnosticType, resource, path string) {
		diagnostics = append(diagnostics, v06Diagnostic{Resource: resource, Type: diagnosticType, Message: message})
		fullName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		fileName, relErr := filepath.Rel(filepath.Join(root, "force-app/main/default"), path)
		if relErr != nil {
			t.Fatal(relErr)
		}
		metadata = append(metadata, map[string]any{"id": c.ID, "componentType": resource, "problemType": diagnosticType,
			"problem": message, "fileName": filepath.ToSlash(fileName), "fullName": fullName,
			"success": false, "lineNumber": nil, "columnNumber": nil})
	}
	pagePath := filepath.Join(root, "force-app/main/default/pages", name+".page")
	rejected := func() (string, []v06Diagnostic) {
		sort.SliceStable(diagnostics, func(i, j int) bool {
			a, b := diagnostics[i], diagnostics[j]
			if a.Resource != b.Resource {
				return a.Resource < b.Resource
			}
			if a.Type != b.Type {
				return a.Type < b.Type
			}
			return a.Message < b.Message
		})
		if c.Kind == "runtime" {
			return v06Native(t, map[string]any{"kind": "native-compile-rejection", "value": "COMPILE_ERROR", "diagnostics": metadata}), diagnostics
		}
		return "COMPILE_ERROR", diagnostics
	}
	p, err := project.Load(root)
	if err != nil {
		addDiagnostic(err.Error(), "Error", "ApexPage", pagePath)
		return rejected()
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		addDiagnostic(err.Error(), "Error", "ApexClass", pagePath)
		return rejected()
	}
	index := typesys.Build(p, sch)
	analysis := sema.AnalyzeWithOptions(apextest.SemanticAnalysisIndex(index), sema.AnalyzeOptions{Diagnostics: true, SuppressPerformanceDiagnostics: true})
	for _, d := range analysis.Diagnostics {
		if d.Severity == diagnostic.Error {
			addDiagnostic(d.Message, "Error", "ApexClass", d.File)
		}
	}
	// All runtime inputs pass the same admission check, including the native
	// rejected rows. No oracle answer controls local compilation or rendering.
	vf, err := visualforce.LoadProject(p)
	if err != nil {
		var componentError *visualforce.ComponentValidationError
		if errors.As(err, &componentError) {
			for _, d := range componentError.Diagnostics {
				addDiagnostic(d.Message, d.Type, d.Resource, d.File)
			}
		} else {
			resource, path := "ApexPage", pagePath
			for _, componentPath := range p.VisualforceComponentFiles {
				if strings.Contains(err.Error(), componentPath) {
					resource, path = "ApexComponent", componentPath
					break
				}
			}
			addDiagnostic(err.Error(), "Error", resource, path)
		}
	}
	if len(diagnostics) != 0 {
		return rejected()
	}
	if c.Kind == "compile" {
		return "COMPILE_OK", diagnostics
	}
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	machine := vm.New(nil)
	machine.Org = &org
	if err := apextest.RegisterProjectRuntimeForRequest(machine, index); err != nil {
		return v06RenderError(t, err), diagnostics
	}
	machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{
		"LocaleSidKey":      storage.StringValue(actor.Locale),
		"TimeZoneSidKey":    storage.StringValue(actor.Timezone),
		"LanguageLocaleKey": storage.StringValue(actor.Language),
	}})
	result, err := visualforce.RenderPage(visualforce.PageRenderRequest{Project: p, VFIndex: vf, Machine: machine, PageName: name, PageURL: "/apex/" + name})
	if err != nil {
		return v06RenderError(t, err), diagnostics
	}
	if result.Error != nil {
		return v06RenderError(t, result.Error), diagnostics
	}
	if result.RedirectURL != "" {
		return v06Native(t, map[string]any{"kind": "redirect", "url": result.RedirectURL}), diagnostics
	}
	return v06ObserveDOM(t, result.HTML, c.ID), diagnostics
}

func v06RenderError(t *testing.T, err error) string {
	return v06Native(t, map[string]any{"kind": "native-render-error", "diagnostics": strings.Split(err.Error(), "\n")})
}

func v06Encode(t *testing.T, value any) string {
	t.Helper()
	var out strings.Builder
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(out.String(), "\n")
}

var v06FixtureName = regexp.MustCompile(`(?i)FamilyV06[ACHPL]\d+_\d+`)

// Match the native observer's owned-name replacement and ensure_ascii framing.
// This does not fold, trim, parse or reorder any observed text or innerHTML.
func v06Native(t *testing.T, value any) string {
	t.Helper()
	encoded := v06FixtureName.ReplaceAllString(v06Encode(t, value), "<owned>")
	var out strings.Builder
	out.WriteString("NATIVE|")
	for _, r := range encoded {
		if r <= 127 {
			out.WriteRune(r)
			continue
		}
		for _, unit := range utf16.Encode([]rune{r}) {
			fmt.Fprintf(&out, "\\u%04x", unit)
		}
	}
	return out.String()
}

func v06ObserveDOM(t *testing.T, markup, id string) string {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		return v06RenderError(t, err)
	}
	var root *html.Node
	var find func(*html.Node)
	find = func(n *html.Node) {
		for _, attr := range n.Attr {
			if attr.Key == "data-case" && attr.Val == id {
				if root != nil {
					t.Fatalf("duplicate local UI components marker %s", id)
				}
				root = n
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			find(child)
		}
	}
	find(doc)
	if root == nil {
		return v06Native(t, map[string]any{"kind": "missing-root"})
	}
	controls := []map[string]string{}
	tags := []string{}
	var clean func(*html.Node)
	clean = func(n *html.Node) {
		if n.Type == html.ElementNode {
			attrs := map[string]string{}
			for _, attr := range n.Attr {
				attrs[attr.Key] = attr.Val
			}
			if n != root && (n.Data == "script" || n.Data == "style" || (n.Data == "input" && attrs["type"] == "hidden")) {
				n.Parent.RemoveChild(n)
				return
			}
			if n != root {
				tags = append(tags, n.Data)
			}
			if n != root && (n.Data == "input" || n.Data == "textarea" || n.Data == "select") {
				value, typ := attrs["value"], attrs["type"]
				switch n.Data {
				case "input":
					if typ == "" {
						typ = "text"
					}
				case "textarea":
					typ, value = "textarea", v06TextContent(n)
				case "select":
					typ, value = "select-one", v06SelectValue(n)
					if _, multiple := attrs["multiple"]; multiple {
						typ = "select-multiple"
					}
				}
				controls = append(controls, map[string]string{"tag": n.Data, "type": typ, "value": value})
			}
			kept := n.Attr[:0]
			for _, attr := range n.Attr {
				if attr.Key != "id" && attr.Key != "for" && attr.Key != "name" && attr.Key != "action" {
					kept = append(kept, attr)
				}
			}
			n.Attr = kept
		}
		for child := n.FirstChild; child != nil; {
			next := child.NextSibling
			clean(child)
			child = next
		}
	}
	clean(root)
	var innerHTML strings.Builder
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		v06SerializeDOM(&innerHTML, child)
	}
	return v06Native(t, map[string]any{"kind": "dom", "text": v06TextContent(root), "html": innerHTML.String(), "controls": controls, "tags": tags})
}

func v06TextContent(n *html.Node) string {
	var out strings.Builder
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.TextNode {
			out.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(n)
	return out.String()
}

func v06SelectValue(n *html.Node) string {
	first, selected := "", false
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "option" {
			value, chosen := v06TextContent(node), false
			for _, attr := range node.Attr {
				if attr.Key == "value" {
					value = attr.Val
				}
				if attr.Key == "selected" {
					chosen = true
				}
			}
			// First option is the browser default for a single select.
			if !selected {
				first, selected = value, true
			}
			if chosen {
				first = value
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(n)
	return first
}

// Browser innerHTML serialization, following the Component structure harness: preserve text,
// whitespace, attributes and order, with HTML void tags and entity escaping.
func v06SerializeDOM(out *strings.Builder, n *html.Node) {
	textEscape := strings.NewReplacer("&", "&amp;", "\u00a0", "&nbsp;", "<", "&lt;", ">", "&gt;")
	attrEscape := strings.NewReplacer("&", "&amp;", "\u00a0", "&nbsp;", `"`, "&quot;")
	switch n.Type {
	case html.TextNode:
		out.WriteString(textEscape.Replace(n.Data))
	case html.CommentNode:
		out.WriteString("<!--" + n.Data + "-->")
	case html.ElementNode:
		out.WriteString("<" + n.Data)
		for _, attr := range n.Attr {
			name := attr.Key
			if attr.Namespace != "" {
				name = attr.Namespace + ":" + name
			}
			out.WriteString(" " + name + `="` + attrEscape.Replace(attr.Val) + `"`)
		}
		out.WriteString(">")
		switch n.Data {
		case "area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr":
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			v06SerializeDOM(out, child)
		}
		out.WriteString("</" + n.Data + ">")
	}
}
