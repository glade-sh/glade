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
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
	"golang.org/x/net/html"
)

type v02Diagnostic struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type v02Input struct {
	Name     string `json:"name"`
	Markup   string `json:"markup"`
	Metadata string `json:"metadata"`
}

type v02Case struct {
	ID               string                   `json:"id"`
	Group            string                   `json:"group"`
	Kind             string                   `json:"kind"`
	Inputs           map[string]v02Input      `json:"inputs"`
	Expected         map[string]string        `json:"expected"`
	Diagnostics      map[string]v02Diagnostic `json:"diagnostics"`
	Owner            string                   `json:"remaining_owner,omitempty"`
	Reason           string                   `json:"remaining_reason,omitempty"`
	DiagnosticOwner  string                   `json:"remaining_diagnostic_owner,omitempty"`
	DiagnosticReason string                   `json:"remaining_diagnostic_reason,omitempty"`
}

type v02Table struct {
	Cases    []v02Case                    `json:"cases"`
	Fixtures map[string]map[string]string `json:"fixtures"`
	Getters  []struct {
		Name       string `json:"name"`
		ReturnType string `json:"return_type"`
		Body       string `json:"body"`
	} `json:"runtime_getters"`
}

type v02DOM struct {
	Error []string `json:"error,omitempty"`
	HTML  *string  `json:"innerHTML,omitempty"`
	Text  *string  `json:"text,omitempty"`
}

// TestV02SalesforceConformance compares all 286 native rows at API 59/67,
// exported from the native expression capture. Compile acceptance
// and diagnostic type/message, DOM text/innerHTML, and native error arrays are
// compared exactly. JSON framing is shared; observed strings are never folded,
// trimmed, or coerced. The DOM observer removes only descendant IDs, matching
// the native capture. Null-returning helpers retain their captured Apex bodies.
// CI reads owned testdata without credentials or a browser.
// GLADE_V02_CAPTURE reports mismatches without failing; GLADE_V02_REPORT writes
// the optional complete TSV, including match counts at each API and overall.
func TestV02SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_V02_CAPTURE") != ""
	versions := []string{"59.0", "67.0"}
	data, err := os.ReadFile("testdata/v02_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table v02Table
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if len(table.Cases) != 286 {
		t.Fatalf("Expression evaluation requires all 286 native cases, got %d", len(table.Cases))
	}
	counts := map[string]int{}
	seen := map[string]bool{}
	for _, c := range table.Cases {
		if c.ID == "" || seen[c.ID] || (c.Kind != "compile" && c.Kind != "runtime") {
			t.Fatalf("invalid or duplicate Expression evaluation case %q (%s)", c.ID, c.Kind)
		}
		seen[c.ID] = true
		counts[c.Kind]++
		if (c.Owner == "") != (c.Reason == "") || (c.DiagnosticOwner == "") != (c.DiagnosticReason == "") {
			t.Fatalf("%s: remaining differences require both owner and reason", c.ID)
		}
		if c.Owner == "Expression evaluation" || c.DiagnosticOwner == "Expression evaluation" {
			t.Fatalf("%s: Expression evaluation-owned behavior must be enforced", c.ID)
		}
		for _, api := range versions {
			input, ok := c.Inputs[api]
			if !ok || input.Name == "" || input.Markup == "" || input.Metadata == "" {
				t.Fatalf("%s: missing captured input at API %s", c.ID, api)
			}
			answer := c.Expected[api]
			if c.Kind == "compile" {
				diagnostic, ok := c.Diagnostics[api]
				if !ok || (answer != "COMPILE_OK" && answer != "COMPILE_ERROR") ||
					(answer == "COMPILE_OK" && diagnostic != (v02Diagnostic{})) ||
					(answer == "COMPILE_ERROR" && (diagnostic.Type == "" || diagnostic.Message == "")) {
					t.Fatalf("%s: missing or inconsistent native compile answer at API %s", c.ID, api)
				}
			} else {
				var native v02DOM
				if !strings.HasPrefix(answer, "DOM|") || json.Unmarshal([]byte(answer[4:]), &native) != nil ||
					(len(native.Error) == 0 && (native.HTML == nil || native.Text == nil)) ||
					(len(native.Error) != 0 && (native.HTML != nil || native.Text != nil)) ||
					v02EncodeDOM(t, native) != answer {
					t.Fatalf("%s: missing or inconsistent native DOM/error answer at API %s", c.ID, api)
				}
			}
		}
	}
	if counts["compile"] != 160 || counts["runtime"] != 126 {
		t.Fatalf("Expression evaluation requires 160 compile/metadata and 126 DOM/error cases, got %v", counts)
	}

	var report strings.Builder
	tsv := csv.NewWriter(&report)
	tsv.Comma = '\t'
	writeRow := func(fields ...string) {
		if err := tsv.Write(fields); err != nil {
			t.Fatal(err)
		}
	}
	writeRow("api", "id", "status", "actual", "expected", "group", "resource", "diagnostic", "owner", "reason", "acceptance_status", "diagnostic_status", "diagnostic_type", "expected_diagnostic_type", "expected_diagnostic", "diagnostic_owner", "diagnostic_reason")
	matches, compileMatches, acceptanceMatches, diagnosticMatches, domMatches := 0, 0, 0, 0, 0
	for _, api := range versions {
		if len(table.Fixtures[api]) != 8 {
			t.Fatalf("API %s: requires all eight captured supporting files", api)
		}
		controller := v02RuntimeController(t, table, api)
		apiMatches, apiCompileMatches, apiAcceptanceMatches, apiDiagnosticMatches, apiDOMMatches := 0, 0, 0, 0, 0
		for _, c := range table.Cases {
			root := t.TempDir()
			for path, body := range table.Fixtures[api] {
				writeFile(t, filepath.Join(root, path), body)
			}
			input := c.Inputs[api]
			path := filepath.Join(root, "force-app/main/default/pages", input.Name+".page")
			writeFile(t, path, input.Markup)
			writeFile(t, path+"-meta.xml", input.Metadata)
			p, compileErr := project.Load(root)
			var index Index
			if compileErr == nil {
				index, compileErr = LoadProject(p)
			}
			got := "COMPILE_OK"
			diagnostic := v02Diagnostic{}
			if compileErr != nil {
				got = "COMPILE_ERROR"
				diagnostic = v02Diagnostic{Type: "Error", Message: compileErr.Error()}
			}
			if c.Kind == "runtime" && compileErr == nil {
				machine := testRunner(t)
				storage.EnsureDeterministicPlatformData(machine.Org)
				storage.EnsureStandardObject(machine.Org, "Account")
				machine.SetCurrentUser(machine.Org.Objects["User"].Records[storage.ID("005000000000001")])
				if err := resource.ApplyProject(machine.Org, p); err != nil {
					t.Fatalf("%s: load owned label/resource fixtures: %v", c.ID, err)
				}
				if err := machine.RegisterClass(controller); err != nil {
					t.Fatal(err)
				}
				result, renderErr := RenderPage(PageRenderRequest{Project: p, VFIndex: index, Machine: machine, PageName: input.Name, PageURL: "/apex/" + input.Name + "?owned=OWNED_PARAM"})
				if renderErr != nil {
					diagnostic = v02Diagnostic{Type: "Error", Message: renderErr.Error()}
					messages := strings.Split(strings.ReplaceAll(renderErr.Error(), input.Name, "<owned-page>"), "\n")
					got = v02EncodeDOM(t, v02DOM{Error: messages})
				} else {
					got = v02ObserveDOM(t, result.HTML, c.ID)
				}
			}
			want := c.Expected[api]
			acceptanceMatch := got == want
			diagnosticMatch := c.Kind != "compile" || diagnostic == c.Diagnostics[api]
			acceptanceStatus, diagnosticStatus, status := "MISMATCH", "NOT_APPLICABLE", "MISMATCH"
			if acceptanceMatch {
				acceptanceStatus = "MATCH"
				if c.Kind == "compile" {
					apiAcceptanceMatches++
				}
			}
			if c.Kind == "compile" {
				diagnosticStatus = "MISMATCH"
				if diagnosticMatch {
					diagnosticStatus = "MATCH"
					apiDiagnosticMatches++
				}
			}
			if acceptanceMatch && diagnosticMatch {
				status = "MATCH"
				apiMatches++
				if c.Kind == "compile" {
					apiCompileMatches++
				} else {
					apiDOMMatches++
				}
			}
			// Only the report hides the temporary root; comparisons retain the
			// original exact text, including native diagnostic case and length.
			owner, reason := c.Owner, c.Reason
			if acceptanceMatch && !diagnosticMatch {
				owner, reason = c.DiagnosticOwner, c.DiagnosticReason
			}
			writeRow(api, c.ID, status, got, want, c.Group, "ApexPage", strings.ReplaceAll(diagnostic.Message, root, "<fixture>"), owner, reason, acceptanceStatus, diagnosticStatus, diagnostic.Type, c.Diagnostics[api].Type, c.Diagnostics[api].Message, c.DiagnosticOwner, c.DiagnosticReason)
			if !capture && !acceptanceMatch {
				if c.Owner == "" {
					t.Errorf("API %s %s: expected <%s> actual <%s>", api, c.ID, want, got)
				} else {
					t.Logf("API %s %s: remaining mismatch owned by %s: %s; expected <%s> actual <%s>", api, c.ID, c.Owner, c.Reason, want, got)
				}
			}
			if !capture && !diagnosticMatch {
				if c.DiagnosticOwner == "" {
					t.Errorf("API %s %s: expected diagnostic <%#v> actual <%#v>", api, c.ID, c.Diagnostics[api], diagnostic)
				} else {
					t.Logf("API %s %s: remaining diagnostic owned by %s: %s; expected <%#v> actual <%#v>", api, c.ID, c.DiagnosticOwner, c.DiagnosticReason, c.Diagnostics[api], diagnostic)
				}
			}
		}
		writeRow("API_TOTAL", api, fmt.Sprintf("%d/%d", apiMatches, len(table.Cases)))
		writeRow("COMPILE_API_TOTAL", api, fmt.Sprintf("%d/%d", apiCompileMatches, counts["compile"]))
		writeRow("COMPILE_ACCEPTANCE_API_TOTAL", api, fmt.Sprintf("%d/%d", apiAcceptanceMatches, counts["compile"]))
		writeRow("DIAGNOSTIC_API_TOTAL", api, fmt.Sprintf("%d/%d", apiDiagnosticMatches, counts["compile"]))
		writeRow("DOM_API_TOTAL", api, fmt.Sprintf("%d/%d", apiDOMMatches, counts["runtime"]))
		t.Logf("Expression evaluation API %s matches %d/286; compile %d/160, acceptance %d/160, diagnostic %d/160; DOM/error %d/126", api, apiMatches, apiCompileMatches, apiAcceptanceMatches, apiDiagnosticMatches, apiDOMMatches)
		matches += apiMatches
		compileMatches += apiCompileMatches
		acceptanceMatches += apiAcceptanceMatches
		diagnosticMatches += apiDiagnosticMatches
		domMatches += apiDOMMatches
	}
	writeRow("COMPILE_TOTAL", fmt.Sprintf("%d/320", compileMatches))
	writeRow("COMPILE_ACCEPTANCE_TOTAL", fmt.Sprintf("%d/320", acceptanceMatches))
	writeRow("DIAGNOSTIC_TOTAL", fmt.Sprintf("%d/320", diagnosticMatches))
	writeRow("DOM_TOTAL", fmt.Sprintf("%d/252", domMatches))
	writeRow("TOTAL", fmt.Sprintf("%d/572", matches))
	t.Logf("Expression evaluation matches %d/572; compile %d/320; DOM/error %d/252", matches, compileMatches, domMatches)
	tsv.Flush()
	if err := tsv.Error(); err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("GLADE_V02_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func v02RuntimeController(t *testing.T, table v02Table, api string) vm.Class {
	t.Helper()
	if len(table.Getters) != 10 {
		t.Fatalf("Expression evaluation requires all ten captured controller getters, got %d", len(table.Getters))
	}
	class := vm.Class{Name: "FamilyV02Controller", APIVersion: api, Access: "public", Fields: map[string]vm.Field{}, Methods: map[string]vm.Method{}}
	for _, getter := range table.Getters {
		program, err := vm.CompileAnonymousWithOptions(getter.Body, vm.CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatalf("compile captured helper %s: %v", getter.Name, err)
		}
		method := vm.Method{Name: class.Name + "." + getter.Name, ClassName: class.Name, ReturnType: getter.ReturnType, Access: "public", APIVersion: api, Program: program}
		class.Methods[strings.ToLower(getter.Name)] = method
		property := strings.TrimPrefix(getter.Name, "get")
		class.Fields[strings.ToLower(property)] = vm.Field{Name: property, Type: getter.ReturnType, Access: "public", Getter: &method}
	}
	return class
}

func v02EncodeDOM(t *testing.T, value v02DOM) string {
	t.Helper()
	var encoded strings.Builder
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return "DOM|" + strings.TrimSuffix(encoded.String(), "\n")
}

func v02ObserveDOM(t *testing.T, markup, id string) string {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		return "LOCAL_OBSERVATION_ERROR|" + err.Error()
	}
	var root *html.Node
	var find func(*html.Node)
	find = func(n *html.Node) {
		for _, attr := range n.Attr {
			if attr.Key == "data-case" && attr.Val == id {
				root = n
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			find(child)
		}
	}
	find(doc)
	if root == nil {
		return "LOCAL_OBSERVATION_ERROR|missing marker " + id
	}
	var text, innerHTML strings.Builder
	var collect func(*html.Node)
	collect = func(n *html.Node) {
		if n.Type == html.TextNode {
			text.WriteString(n.Data)
		}
		if n != root {
			attrs := n.Attr[:0]
			for _, attr := range n.Attr {
				if attr.Key != "id" {
					attrs = append(attrs, attr)
				}
			}
			n.Attr = attrs
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			collect(child)
		}
	}
	collect(root)
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		v02SerializeDOM(&innerHTML, child)
	}
	textValue, htmlValue := text.String(), innerHTML.String()
	return v02EncodeDOM(t, v02DOM{Text: &textValue, HTML: &htmlValue})
}

func v02SerializeDOM(out *strings.Builder, n *html.Node) {
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
			v02SerializeDOM(out, child)
		}
		out.WriteString("</" + n.Data + ">")
	}
}
