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
	"github.com/glade-sh/glade/internal/vm"
	"golang.org/x/net/html"
)

type v01Case struct {
	ID               string                   `json:"id"`
	Group            string                   `json:"group"`
	Kind             string                   `json:"kind"`
	Resource         string                   `json:"resource"`
	Markup           string                   `json:"markup"`
	Metadata         map[string]string        `json:"metadata"`
	Expected         map[string]string        `json:"expected"`
	Owner            string                   `json:"remaining_owner,omitempty"`
	Reason           string                   `json:"remaining_reason,omitempty"`
	Diagnostics      map[string]v01Diagnostic `json:"diagnostics,omitempty"`
	DiagnosticOwner  string                   `json:"remaining_diagnostic_owner,omitempty"`
	DiagnosticReason string                   `json:"remaining_diagnostic_reason,omitempty"`
}

type v01Diagnostic struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type v01Table struct {
	Cases   []v01Case `json:"cases"`
	Getters []struct {
		Name       string `json:"name"`
		ReturnType string `json:"return_type"`
		Body       string `json:"body"`
	} `json:"runtime_getters"`
}

type v01DOM struct {
	Elements []string `json:"elements"`
	HTML     string   `json:"html"`
	Text     string   `json:"text"`
}

// TestV01SalesforceConformance uses all 305 owned rows at API 59/67, exported
// from the native structure capture: compile inputs and DOM observations,
// plus the repeat and raw-text controls at both APIs.
// Compile rows compare acceptance and exact diagnostic type/text from the native
// diagnostics.json files. Every Component structure-owned diagnostic is enforced; the three Expression evaluation
// expression rows carry their acceptance and diagnostic differences separately.
// DOM rows compare exact text, element order and
// normalized innerHTML using the native observer's exclusions and traversal-local
// ID replacement.
// CI reads only testdata and never calls Salesforce or launches a browser.
// GLADE_V01_CAPTURE reports every mismatch without failing on the pre-fix tree;
// GLADE_V01_REPORT selects the optional TSV output path.
func TestV01SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_V01_CAPTURE") != ""
	versions := []string{"59.0", "67.0"}
	data, err := os.ReadFile("testdata/v01_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table v01Table
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	cases := table.Cases
	if len(cases) != 305 {
		t.Fatalf("Component structure conformance requires all 305 native cases, got %d", len(cases))
	}
	counts := map[string]int{}
	seen := make(map[string]bool, len(cases))
	for _, c := range cases {
		if c.ID == "" || seen[c.ID] {
			t.Fatalf("empty or duplicate Component structure case ID %q", c.ID)
		}
		seen[c.ID] = true
		if (c.Kind != "compile" && c.Kind != "runtime") || (c.Resource != "ApexPage" && c.Resource != "ApexComponent") {
			t.Fatalf("%s: unsupported case kind/resource %s/%s", c.ID, c.Kind, c.Resource)
		}
		counts[c.Kind]++
		if (c.Owner == "") != (c.Reason == "") {
			t.Fatalf("%s: a remaining row requires both owner and reason", c.ID)
		}
		if (c.DiagnosticOwner == "") != (c.DiagnosticReason == "") {
			t.Fatalf("%s: a remaining diagnostic requires both owner and reason", c.ID)
		}
		if c.Owner == "Component structure" || c.DiagnosticOwner == "Component structure" {
			t.Fatalf("%s: Component structure-owned behavior must be enforced", c.ID)
		}
		for _, api := range versions {
			validAnswer := c.Expected[api] == "COMPILE_OK" || c.Expected[api] == "COMPILE_ERROR"
			if c.Kind == "runtime" {
				var native v01DOM
				validAnswer = strings.HasPrefix(c.Expected[api], "DOM|") && json.Unmarshal([]byte(strings.TrimPrefix(c.Expected[api], "DOM|")), &native) == nil
			}
			if c.Metadata[api] == "" || !validAnswer {
				t.Fatalf("%s: missing native input/answer at API %s", c.ID, api)
			}
			if c.Kind == "compile" {
				diagnostic, ok := c.Diagnostics[api]
				if !ok || (c.Expected[api] == "COMPILE_ERROR" && (diagnostic.Type == "" || diagnostic.Message == "")) || (c.Expected[api] == "COMPILE_OK" && diagnostic != (v01Diagnostic{})) {
					t.Fatalf("%s: missing or inconsistent native diagnostic at API %s", c.ID, api)
				}
			}
		}
	}
	if counts["compile"] != 285 || counts["runtime"] != 20 {
		t.Fatalf("Component structure requires 285 compile/metadata and 20 DOM cases, got %v", counts)
	}
	controller := v01RuntimeController(t, table)

	var report strings.Builder
	tsv := csv.NewWriter(&report)
	tsv.Comma = '\t'
	writeRow := func(fields ...string) {
		if err := tsv.Write(fields); err != nil {
			t.Fatal(err)
		}
	}
	writeRow("api", "id", "status", "actual", "expected", "group", "resource", "diagnostic", "owner", "reason", "acceptance_status", "diagnostic_status", "diagnostic_type", "expected_diagnostic_type", "expected_diagnostic", "diagnostic_owner", "diagnostic_reason")
	matches, total := 0, 0
	domMatches := 0
	compileAcceptanceMatches, diagnosticMatches := 0, 0
	for _, api := range versions {
		apiMatches := 0
		apiDOMMatches := 0
		apiCompileAcceptanceMatches, apiDiagnosticMatches := 0, 0
		for i, c := range cases {
			root := t.TempDir()
			name := fmt.Sprintf("FamilyV01Case%03d", i)
			folder, extension := "pages", ".page"
			if c.Resource == "ApexComponent" {
				folder, extension = "components", ".component"
			}
			path := filepath.Join(root, "force-app", "main", "default", folder, name+extension)
			markup := c.Markup
			if c.Kind == "runtime" {
				markup = `<apex:page controller="` + controller.Name + `" showHeader="false" sidebar="false" standardStylesheets="false" docType="html-5.0"><div data-v01-case="` + c.ID + `">` + markup + `</div></apex:page>`
			}
			writeFile(t, path, markup)
			writeFile(t, path+"-meta.xml", c.Metadata[api])
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			p, compileErr := project.Load(root)
			var index Index
			if compileErr == nil {
				index, compileErr = LoadProject(p)
			}
			got := "COMPILE_OK"
			diagnostic := v01Diagnostic{}
			if compileErr != nil {
				got = "COMPILE_ERROR"
				diagnostic = v01Diagnostic{Type: "Error", Message: compileErr.Error()}
			}
			if c.Kind == "runtime" && compileErr == nil {
				machine := testRunner(t)
				if err := machine.RegisterClass(controller); err != nil {
					t.Fatal(err)
				}
				result, renderErr := RenderPage(PageRenderRequest{Project: p, VFIndex: index, Machine: machine, PageName: name})
				if renderErr != nil {
					got = "RENDER_ERROR"
					diagnostic = v01Diagnostic{Type: "Error", Message: renderErr.Error()}
				} else {
					got = v01ObserveDOM(t, result.HTML, c.ID)
				}
			}
			want := c.Expected[api]
			acceptanceMatch := got == want
			diagnosticMatch := true
			acceptanceStatus, diagnosticStatus := "MISMATCH", "NOT_APPLICABLE"
			if acceptanceMatch {
				acceptanceStatus = "MATCH"
				if c.Kind == "compile" {
					compileAcceptanceMatches++
					apiCompileAcceptanceMatches++
				}
			}
			if c.Kind == "compile" {
				diagnosticMatch = diagnostic == c.Diagnostics[api]
				diagnosticStatus = "MISMATCH"
				if diagnosticMatch {
					diagnosticStatus = "MATCH"
					diagnosticMatches++
					apiDiagnosticMatches++
				}
			}
			status := "MISMATCH"
			total++
			if acceptanceMatch && diagnosticMatch {
				matches++
				apiMatches++
				status = "MATCH"
				if c.Kind == "runtime" {
					domMatches++
					apiDOMMatches++
				}
			}
			owner, reason := c.Owner, c.Reason
			if acceptanceMatch && !diagnosticMatch {
				owner, reason = c.DiagnosticOwner, c.DiagnosticReason
			}
			// Only the report replaces the temporary root. The comparison above
			// uses the original diagnostic text without trimming or normalization.
			writeRow(api, c.ID, status, got, want, c.Group, c.Resource, strings.ReplaceAll(diagnostic.Message, root, "<fixture>"), owner, reason, acceptanceStatus, diagnosticStatus, diagnostic.Type, c.Diagnostics[api].Type, c.Diagnostics[api].Message, c.DiagnosticOwner, c.DiagnosticReason)
			if !capture && !acceptanceMatch && c.Owner == "" {
				t.Errorf("API %s %s: got %s, want %s (local diagnostic: %s)", api, c.ID, got, want, diagnostic.Message)
			}
			if !capture && !acceptanceMatch && c.Owner != "" {
				t.Logf("API %s %s: remaining mismatch owned by %s: %s; actual <%s> expected <%s>", api, c.ID, c.Owner, c.Reason, got, want)
			}
			if !capture && !diagnosticMatch {
				if c.DiagnosticOwner == "" {
					t.Errorf("API %s %s: diagnostic got %#v, want %#v", api, c.ID, diagnostic, c.Diagnostics[api])
				} else {
					t.Logf("API %s %s: remaining diagnostic owned by %s: %s; actual <%#v> expected <%#v>", api, c.ID, c.DiagnosticOwner, c.DiagnosticReason, diagnostic, c.Diagnostics[api])
				}
			}
		}
		writeRow("API_TOTAL", api, fmt.Sprintf("%d/%d", apiMatches, len(cases)))
		writeRow("COMPILE_API_TOTAL", api, fmt.Sprintf("%d/%d", apiMatches-apiDOMMatches, counts["compile"]))
		writeRow("COMPILE_ACCEPTANCE_API_TOTAL", api, fmt.Sprintf("%d/%d", apiCompileAcceptanceMatches, counts["compile"]))
		writeRow("DIAGNOSTIC_API_TOTAL", api, fmt.Sprintf("%d/%d", apiDiagnosticMatches, counts["compile"]))
		writeRow("DOM_API_TOTAL", api, fmt.Sprintf("%d/%d", apiDOMMatches, counts["runtime"]))
		t.Logf("Component structure API %s matches %d/%d", api, apiMatches, len(cases))
	}
	writeRow("COMPILE_TOTAL", fmt.Sprintf("%d/%d", matches-domMatches, counts["compile"]*len(versions)))
	writeRow("COMPILE_ACCEPTANCE_TOTAL", fmt.Sprintf("%d/%d", compileAcceptanceMatches, counts["compile"]*len(versions)))
	writeRow("DIAGNOSTIC_TOTAL", fmt.Sprintf("%d/%d", diagnosticMatches, counts["compile"]*len(versions)))
	writeRow("DOM_TOTAL", fmt.Sprintf("%d/%d", domMatches, counts["runtime"]*len(versions)))
	t.Logf("Component structure compile acceptance matches %d/%d; exact diagnostic matches %d/%d", compileAcceptanceMatches, counts["compile"]*len(versions), diagnosticMatches, counts["compile"]*len(versions))
	t.Logf("Component structure compile matches %d/%d; DOM matches %d/%d", matches-domMatches, counts["compile"]*len(versions), domMatches, counts["runtime"]*len(versions))
	writeRow("TOTAL", fmt.Sprintf("%d/%d", matches, total))
	t.Logf("Component structure matches %d/%d", matches, total)
	tsv.Flush()
	if err := tsv.Error(); err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("GLADE_V01_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func v01RuntimeController(t *testing.T, table v01Table) vm.Class {
	t.Helper()
	if len(table.Getters) != 5 {
		t.Fatalf("Component structure runtime requires the five captured helper getters, got %d", len(table.Getters))
	}
	class := vm.Class{Name: "FamilyV01RuntimeController", APIVersion: "67.0", Access: "public", Fields: map[string]vm.Field{}, Methods: map[string]vm.Method{}}
	for _, getter := range table.Getters {
		program, err := vm.CompileAnonymousWithOptions(getter.Body, vm.CompileOptions{APIVersion: "67.0"})
		if err != nil {
			t.Fatalf("compile owned helper %s: %v", getter.Name, err)
		}
		method := vm.Method{Name: class.Name + "." + getter.Name, ClassName: class.Name, ReturnType: getter.ReturnType, Access: "public", APIVersion: "67.0", Program: program}
		class.Methods[strings.ToLower(getter.Name)] = method
		// Visualforce exposes Apex getX() methods as properties. Execute the
		// captured bodies, including raw null returns and the caught exception.
		property := strings.TrimPrefix(getter.Name, "get")
		class.Fields[strings.ToLower(property)] = vm.Field{Name: property, Type: getter.ReturnType, Access: "public", Getter: &method}
	}
	return class
}

func v01ObserveDOM(t *testing.T, markup, id string) string {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		t.Fatal(err)
	}
	var root *html.Node
	var find func(*html.Node)
	find = func(n *html.Node) {
		for _, attr := range n.Attr {
			if attr.Key == "data-v01-case" && attr.Val == id {
				if root != nil {
					t.Fatalf("duplicate DOM row %s", id)
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
		t.Fatalf("missing local DOM row %s", id)
	}
	var elements []*html.Node
	var collect func(*html.Node)
	collect = func(n *html.Node) {
		elements = append(elements, n)
		for child := n.FirstChild; child != nil; {
			next := child.NextSibling
			if child.Type == html.ElementNode && (child.Data == "script" || child.Data == "style" || child.Data == "input" || child.Data == "form") {
				n.RemoveChild(child)
			} else if child.Type == html.ElementNode {
				collect(child)
			}
			child = next
		}
	}
	collect(root)
	tokens := map[string]string{}
	nextID := 0
	for _, el := range elements {
		for i, attr := range el.Attr {
			if attr.Key == "id" && attr.Val != "" {
				tokens[attr.Val] = fmt.Sprintf("v01-id-%d", nextID)
				el.Attr[i].Val = tokens[attr.Val]
				nextID++
			}
		}
	}
	for _, el := range elements {
		attrs := el.Attr[:0]
		for _, attr := range el.Attr {
			if strings.HasPrefix(strings.ToLower(attr.Key), "on") {
				continue
			}
			if attr.Key == "for" || attr.Key == "aria-labelledby" || attr.Key == "aria-describedby" {
				refs := strings.Split(attr.Val, " ")
				for i, ref := range refs {
					if token, exists := tokens[ref]; exists {
						refs[i] = token
					}
				}
				attr.Val = strings.Join(refs, " ")
			}
			attrs = append(attrs, attr)
		}
		el.Attr = attrs
	}
	value := v01DOM{Elements: []string{}}
	for _, el := range elements[1:] {
		value.Elements = append(value.Elements, el.Data)
	}
	var text, innerHTML strings.Builder
	var textContent func(*html.Node)
	textContent = func(n *html.Node) {
		if n.Type == html.TextNode {
			text.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			textContent(child)
		}
	}
	textContent(root)
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		v01SerializeDOM(&innerHTML, child)
	}
	value.Text, value.HTML = text.String(), innerHTML.String()
	var encoded strings.Builder
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return "DOM|" + strings.TrimSuffix(encoded.String(), "\n")
}

// Serialize innerHTML with HTML void tags and entity escaping, preserving all
// text whitespace, attribute order and element order from the parsed DOM.
func v01SerializeDOM(out *strings.Builder, n *html.Node) {
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
		if isVoidHTMLElement(n.Data) {
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			v01SerializeDOM(out, child)
		}
		out.WriteString("</" + n.Data + ">")
	}
}
