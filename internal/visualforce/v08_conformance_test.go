package visualforce

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
	"golang.org/x/net/html"
)

type v08Diagnostic struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type v08Case struct {
	ID     string `json:"id"`
	Group  string `json:"group"`
	Kind   string `json:"kind"`
	Inputs map[string]struct {
		Name     string `json:"name"`
		Markup   string `json:"markup"`
		Metadata string `json:"metadata"`
	} `json:"inputs"`
	Expected    map[string]string        `json:"expected"`
	Diagnostics map[string]v08Diagnostic `json:"diagnostics"`
}

type v08Table struct {
	Support map[string]map[string]string `json:"support"`
	Getters []struct {
		Name       string `json:"name"`
		ReturnType string `json:"return_type"`
		Body       string `json:"body"`
	} `json:"runtime_getters"`
	Cases []v08Case `json:"cases"`
}

// TestV08SalesforceConformance compares all 152 compile/metadata and 124 native
// rendered rows at API 59/67, exported from the native repetition capture.
// Inputs retain captured page names, metadata and controller bodies.
// The Go HTML observer follows the native script/style exclusion and measures
// exact text, table headers/cells, list items, semantic order and nullable spans.
// JSON transport whitespace is canonicalized; no observed field is dropped.
// All comparisons use exact Go equality, never Apex assertEquals or ==.
// CI reads testdata without Salesforce, glade-tools imports or browser launches.
// GLADE_V08_CAPTURE=1 records every mismatch without failing on the unchanged
// accepted base; GLADE_V08_REPORT selects the optional per-row TSV output.
func TestV08SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_V08_CAPTURE") != ""
	data, err := os.ReadFile("testdata/v08_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table v08Table
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	versions := []string{"59.0", "67.0"}
	counts, seen, groups := map[string]int{}, map[string]bool{}, map[string]bool{}
	for _, c := range table.Cases {
		if c.ID == "" || seen[c.ID] || (c.Kind != "compile" && c.Kind != "runtime") {
			t.Fatalf("invalid or duplicate V08 case %q (%s)", c.ID, c.Kind)
		}
		seen[c.ID], groups[c.Group] = true, true
		counts[c.Kind]++
		for _, api := range versions {
			input, ok := c.Inputs[api]
			if !ok || input.Name == "" || input.Markup == "" || input.Metadata == "" || len(table.Support[api]) != 3 {
				t.Fatalf("%s: missing native input/support at API %s", c.ID, api)
			}
			answer, ok := c.Expected[api]
			if !ok {
				t.Fatalf("%s: missing native answer at API %s", c.ID, api)
			}
			if c.Kind == "compile" {
				d, exists := c.Diagnostics[api]
				if !exists || (answer != "COMPILE_OK" && answer != "COMPILE_ERROR") ||
					(answer == "COMPILE_ERROR" && (d.Type == "" || d.Message == "")) ||
					(answer == "COMPILE_OK" && d != (v08Diagnostic{})) {
					t.Fatalf("%s: inconsistent native compile answer/diagnostic at API %s", c.ID, api)
				}
			} else {
				var native map[string]any
				if !strings.HasPrefix(answer, "DOM|") || json.Unmarshal([]byte(strings.TrimPrefix(answer, "DOM|")), &native) != nil ||
					(native["status"] != "DOM" && native["status"] != "NATIVE_RENDER_ERROR") || "DOM|"+v08JSON(t, native) != answer {
					t.Fatalf("%s: invalid native DOM answer/serialization at API %s", c.ID, api)
				}
			}
		}
	}
	if len(table.Cases) != 276 || counts["compile"] != 152 || counts["runtime"] != 124 || len(table.Getters) != 22 ||
		len(groups) != 4 || !groups["repeat"] || !groups["tables and lists"] || !groups["generated columns"] || !groups["row variables"] {
		t.Fatalf("V08 requires 276 cases (152 compile/metadata + 124 DOM), 22 getters and all four groups; got %d, %v, %d, %v", len(table.Cases), counts, len(table.Getters), groups)
	}
	var report strings.Builder
	tsv := csv.NewWriter(&report)
	tsv.Comma = '\t'
	write := func(fields ...string) {
		if err := tsv.Write(fields); err != nil {
			t.Fatal(err)
		}
	}
	write("api", "id", "status", "actual", "expected", "group", "kind", "acceptance_status", "diagnostic_status", "diagnostic_type", "diagnostic", "expected_diagnostic_type", "expected_diagnostic")
	matches, compileMatches, acceptanceMatches, diagnosticMatches, domMatches := 0, 0, 0, 0, 0
	for _, api := range versions {
		controllers := v08Controllers(t, table, api)
		apiMatches, apiCompile, apiAcceptance, apiDiagnostic, apiDOM := 0, 0, 0, 0, 0
		for _, c := range table.Cases {
			root, p, index, compileErr := v08LoadCase(t, table, c, api)
			actual := "COMPILE_OK"
			diagnostic := v08Diagnostic{}
			if compileErr != nil {
				actual = "COMPILE_ERROR"
				diagnostic = v08Diagnostic{Type: "Error", Message: compileErr.Error()}
			} else if c.Kind == "runtime" {
				machine := testRunner(t)
				storage.EnsureStandardObject(machine.Org, "Account")
				for _, controller := range controllers {
					if err := machine.RegisterClass(controller); err != nil {
						t.Fatalf("API %s %s: register captured controller: %v", api, c.ID, err)
					}
				}
				input := c.Inputs[api]
				result, renderErr := RenderPage(PageRenderRequest{Project: p, VFIndex: index, Machine: machine, PageName: input.Name, PageURL: "/apex/" + input.Name})
				if renderErr != nil {
					diagnostic = v08Diagnostic{Type: "Error", Message: renderErr.Error()}
					// Retain the local error text exactly. Do not substitute the
					// native exception or manufacture native diagnostic context.
					actual = "DOM|" + v08JSON(t, map[string]any{"status": "NATIVE_RENDER_ERROR", "diagnostics": strings.Split(renderErr.Error(), "\n")})
				} else {
					actual = v08ObserveDOM(t, result.HTML, c.ID)
				}
			}
			want := c.Expected[api]
			acceptanceMatch := actual == want
			diagnosticMatch := c.Kind != "compile" || diagnostic == c.Diagnostics[api]
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
					apiDiagnostic++
				}
			}
			if acceptanceMatch && diagnosticMatch {
				status = "MATCH"
				apiMatches++
				if c.Kind == "compile" {
					apiCompile++
				} else {
					apiDOM++
				}
			}
			nativeDiagnostic := c.Diagnostics[api]
			// Only the report hides the temporary root; comparison uses raw text.
			write(api, c.ID, status, actual, want, c.Group, c.Kind, acceptanceStatus, diagnosticStatus, diagnostic.Type,
				strings.ReplaceAll(diagnostic.Message, root, "<fixture>"), nativeDiagnostic.Type, nativeDiagnostic.Message)
			if !capture && (!acceptanceMatch || !diagnosticMatch) {
				t.Errorf("API %s %s: expected <%s> actual <%s>; diagnostic expected <%#v> actual <%#v>", api, c.ID, want, actual, nativeDiagnostic, diagnostic)
			}
		}
		matches += apiMatches
		compileMatches += apiCompile
		acceptanceMatches += apiAcceptance
		diagnosticMatches += apiDiagnostic
		domMatches += apiDOM
		write("API_TOTAL", api, fmt.Sprintf("%d/276", apiMatches))
		write("COMPILE_API_TOTAL", api, fmt.Sprintf("%d/152", apiCompile))
		write("COMPILE_ACCEPTANCE_API_TOTAL", api, fmt.Sprintf("%d/152", apiAcceptance))
		write("DIAGNOSTIC_API_TOTAL", api, fmt.Sprintf("%d/152", apiDiagnostic))
		write("DOM_API_TOTAL", api, fmt.Sprintf("%d/124", apiDOM))
		t.Logf("V08 API %s exact matches %d/276; compile/diagnostics %d/152; DOM %d/124", api, apiMatches, apiCompile, apiDOM)
	}
	write("TOTAL", fmt.Sprintf("%d/552", matches))
	write("COMPILE_TOTAL", fmt.Sprintf("%d/304", compileMatches))
	write("COMPILE_ACCEPTANCE_TOTAL", fmt.Sprintf("%d/304", acceptanceMatches))
	write("DIAGNOSTIC_TOTAL", fmt.Sprintf("%d/304", diagnosticMatches))
	write("DOM_TOTAL", fmt.Sprintf("%d/248", domMatches))
	t.Logf("V08 exact matches %d/552; compile/diagnostics %d/304; compile acceptance %d/304; DOM %d/248", matches, compileMatches, acceptanceMatches, domMatches)
	tsv.Flush()
	if err := tsv.Error(); err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("GLADE_V08_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// Native review controls capture relationship paths on typed
// Account rows in all four containers, alongside c_invalid_unknown_row_field,
// at both APIs.
func TestLoadProjectV08TypedRowRelationships(t *testing.T) {
	data, err := os.ReadFile("testdata/v08_relationship_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var table v08Table
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if len(table.Cases) != 5 {
		t.Fatalf("expected five captured V08 review controls, got %d", len(table.Cases))
	}
	for _, api := range []string{"59.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			for _, c := range table.Cases {
				t.Run(c.ID, func(t *testing.T) {
					_, _, _, compileErr := v08LoadCase(t, table, c, api)
					actual, diagnostic := "COMPILE_OK", v08Diagnostic{}
					if compileErr != nil {
						actual = "COMPILE_ERROR"
						diagnostic = v08Diagnostic{Type: "Error", Message: compileErr.Error()}
					}
					if actual != c.Expected[api] || diagnostic != c.Diagnostics[api] {
						t.Errorf("expected <%s> diagnostic <%#v>; actual <%s> diagnostic <%#v>", c.Expected[api], c.Diagnostics[api], actual, diagnostic)
					}
				})
			}
		})
	}
}

func v08LoadCase(t *testing.T, table v08Table, c v08Case, api string) (string, project.Project, Index, error) {
	t.Helper()
	root := t.TempDir()
	for path, source := range table.Support[api] {
		writeFile(t, filepath.Join(root, filepath.FromSlash(path)), source)
	}
	input := c.Inputs[api]
	path := filepath.Join(root, "force-app/main/default/pages", input.Name+".page")
	writeFile(t, path, input.Markup)
	writeFile(t, path+"-meta.xml", input.Metadata)
	p, err := project.Load(root)
	var index Index
	if err == nil {
		index, err = LoadProject(p)
	}
	return root, p, index, err
}

func v08Controllers(t *testing.T, table v08Table, api string) []vm.Class {
	t.Helper()
	name := "FamilyV08C" + strings.TrimSuffix(api, ".0")
	class := vm.Class{Name: name, APIVersion: api, Access: "public", Fields: map[string]vm.Field{}, Methods: map[string]vm.Method{}}
	for _, getter := range table.Getters {
		program, err := vm.CompileAnonymousWithOptions(getter.Body, vm.CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatalf("compile captured getter %s API %s: %v", getter.Name, api, err)
		}
		method := vm.Method{Name: name + "." + getter.Name, ClassName: name, ReturnType: getter.ReturnType, Access: "public", APIVersion: api, Program: program}
		class.Methods[strings.ToLower(getter.Name)] = method
		property := strings.TrimPrefix(getter.Name, "get")
		class.Fields[strings.ToLower(property)] = vm.Field{Name: property, Type: getter.ReturnType, Access: "public", Getter: &method}
	}
	return []vm.Class{{Name: name + ".V08Exception", SuperClass: "Exception", Access: "public", APIVersion: api}, class}
}

func v08ObserveDOM(t *testing.T, markup, id string) string {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		t.Fatal(err)
	}
	var root *html.Node
	for _, n := range v08Descendants(doc) {
		if n.Type == html.ElementNode && v08DOMAttribute(n, "data-case") != nil && *v08DOMAttribute(n, "data-case") == id {
			if root != nil {
				return "DOM|" + v08JSON(t, map[string]any{"status": "LOCAL_ROW_MARKER_DUPLICATE"})
			}
			root = n
		}
	}
	if root == nil {
		return "DOM|" + v08JSON(t, map[string]any{"status": "LOCAL_ROW_MARKER_MISSING"})
	}
	tables, listItems, structure := []any{}, []string{}, []any{}
	for _, n := range v08Descendants(root) {
		if n.Type != html.ElementNode {
			continue
		}
		if n.Data == "table" {
			headers, rows := []string{}, [][]string{}
			for _, child := range v08Descendants(n) {
				if child.Type != html.ElementNode {
					continue
				}
				if child.Data == "th" && v08DOMAncestor(child, "thead") {
					headers = append(headers, v08TextContent(child))
				}
				if child.Data == "tr" && v08DOMAncestor(child, "tbody") {
					cells := []string{}
					for _, cell := range v08Descendants(child) {
						if cell.Type == html.ElementNode && (cell.Data == "th" || cell.Data == "td") {
							cells = append(cells, v08TextContent(cell))
						}
					}
					rows = append(rows, cells)
				}
			}
			tables = append(tables, map[string]any{"headers": headers, "rows": rows})
		}
		if n.Data == "li" {
			listItems = append(listItems, v08TextContent(n))
		}
		switch n.Data {
		case "table", "thead", "tbody", "tr", "th", "td", "ul", "ol", "li":
			// getAttribute returns raw null for an absent span, not an empty
			// string or the DOM property's default numeric value.
			structure = append(structure, map[string]any{"tag": strings.ToUpper(n.Data), "rowSpan": v08DOMAttribute(n, "rowspan"), "colSpan": v08DOMAttribute(n, "colspan")})
		}
	}
	return "DOM|" + v08JSON(t, map[string]any{"status": "DOM", "text": v08TextContent(root), "tables": tables, "listItems": listItems, "structure": structure})
}

// querySelectorAll traverses descendants, excluding the root. The native clone
// removes script/style nodes with their entire subtrees before every query.
func v08Descendants(root *html.Node) []*html.Node {
	var nodes []*html.Node
	var walk func(*html.Node)
	walk = func(parent *html.Node) {
		for child := parent.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode && (child.Data == "script" || child.Data == "style") {
				continue
			}
			nodes = append(nodes, child)
			walk(child)
		}
	}
	walk(root)
	return nodes
}

func v08TextContent(root *html.Node) string {
	var text strings.Builder
	if root.Type == html.TextNode {
		text.WriteString(root.Data)
	}
	for _, n := range v08Descendants(root) {
		if n.Type == html.TextNode {
			text.WriteString(n.Data)
		}
	}
	return text.String()
}

func v08DOMAttribute(node *html.Node, name string) *string {
	for _, attr := range node.Attr {
		if attr.Key == name {
			value := attr.Val
			return &value
		}
	}
	return nil
}

func v08DOMAncestor(node *html.Node, tag string) bool {
	for parent := node.Parent; parent != nil; parent = parent.Parent {
		if parent.Type == html.ElementNode && parent.Data == tag {
			return true
		}
	}
	return false
}

func v08JSON(t *testing.T, value any) string {
	t.Helper()
	var encoded strings.Builder
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	// Match the native ensure_ascii transport, including pageBlock NBSPs.
	var ascii strings.Builder
	for _, r := range strings.TrimSuffix(encoded.String(), "\n") {
		if r <= 127 {
			ascii.WriteRune(r)
		} else if r <= 0xffff {
			fmt.Fprintf(&ascii, `\u%04x`, r)
		} else {
			r -= 0x10000
			fmt.Fprintf(&ascii, `\u%04x\u%04x`, 0xd800+(r>>10), 0xdc00+(r&0x3ff))
		}
	}
	return ascii.String()
}
