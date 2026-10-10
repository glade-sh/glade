package visualforce_test

import (
	"encoding/csv"
	"encoding/json"
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

type v03Input struct {
	Name  string            `json:"name"`
	Files map[string]string `json:"files"`
}

type v03Diagnostic struct {
	Resource string `json:"resource"`
	Type     string `json:"type"`
	Message  string `json:"message"`
}

type v03Case struct {
	ID          string                     `json:"id"`
	Group       string                     `json:"group"`
	Kind        string                     `json:"kind"`
	Query       string                     `json:"query"`
	Inputs      map[string]v03Input        `json:"inputs"`
	Expected    map[string]string          `json:"expected"`
	Diagnostics map[string][]v03Diagnostic `json:"diagnostics"`
}

// TestV03SalesforceConformance exports every Page messages compile/metadata and DOM row
// at API 59/67 from the owned oracle, including its original rows. The captured Apex sources
// use the existing project compiler and request runtime; no replacement getters
// or controller behavior are supplied by this adapter. DOM observation matches
// the native textContent/JSON-key projection, without running a browser.
// Compile acceptance and all diagnostic type/message strings compare exactly.
// Native errors retain the capture's fixture-name/stack-position projection.
// CI reads only owned testdata.
// GLADE_V03_CAPTURE=1 reports all differences without failing on the unchanged
// accepted base; GLADE_V03_REPORT selects the per-row TSV output path.
func TestV03SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_V03_CAPTURE") != ""
	data, err := os.ReadFile("testdata/v03_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Source string    `json:"source"`
		Cases  []v03Case `json:"cases"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if table.Source != "Owned page-message observations at API 59.0 and 67.0." || len(table.Cases) != 299 {
		t.Fatalf("Page messages requires all 299 cases from the final native oracle, got %d at %s", len(table.Cases), table.Source)
	}
	versions := []string{"59.0", "67.0"}
	counts, seen := map[string]int{}, map[string]bool{}
	for _, c := range table.Cases {
		if c.ID == "" || seen[c.ID] || (c.Kind != "compile" && c.Kind != "runtime") {
			t.Fatalf("invalid or duplicate Page messages case %q (%s)", c.ID, c.Kind)
		}
		seen[c.ID] = true
		counts[c.Kind]++
		for _, api := range versions {
			input := c.Inputs[api]
			if input.Name == "" || len(input.Files) < 5 || input.Files["sfdx-project.json"] == "" ||
				input.Files["force-app/main/default/pages/"+input.Name+".page"] == "" ||
				input.Files["force-app/main/default/pages/"+input.Name+".page-meta.xml"] == "" {
				t.Fatalf("%s: incomplete captured input at API %s", c.ID, api)
			}
			answer := c.Expected[api]
			diagnostics, ok := c.Diagnostics[api]
			if !ok {
				t.Fatalf("%s: missing diagnostic list at API %s", c.ID, api)
			}
			if c.Kind == "compile" {
				if (answer != "COMPILE_OK" && answer != "COMPILE_ERROR") ||
					(answer == "COMPILE_OK" && len(diagnostics) != 0) ||
					(answer == "COMPILE_ERROR" && len(diagnostics) == 0) {
					t.Fatalf("%s: inconsistent native compile answer at API %s", c.ID, api)
				}
				for _, d := range diagnostics {
					if d.Resource == "" || d.Type == "" || d.Message == "" {
						t.Fatalf("%s: incomplete native diagnostic at API %s", c.ID, api)
					}
				}
			} else {
				var native map[string]json.RawMessage
				if !strings.HasPrefix(answer, "DOM|") || json.Unmarshal([]byte(strings.TrimPrefix(answer, "DOM|")), &native) != nil || len(diagnostics) != 0 {
					t.Fatalf("%s: invalid native DOM answer at API %s", c.ID, api)
				}
				var present bool
				var text string
				var errors []string
				if json.Unmarshal(native["rootPresent"], &present) != nil ||
					(present && (len(native) != 2 || json.Unmarshal(native["text"], &text) != nil)) ||
					(!present && (len(native) != 2 || json.Unmarshal(native["nativeRenderError"], &errors) != nil || len(errors) == 0)) {
					t.Fatalf("%s: incomplete native DOM/error observation at API %s", c.ID, api)
				}
			}
		}
	}
	if counts["compile"] != 165 || counts["runtime"] != 134 {
		t.Fatalf("Page messages requires 165 compile/metadata and 134 DOM rows, got %v", counts)
	}

	var report strings.Builder
	tsv := csv.NewWriter(&report)
	tsv.Comma = '\t'
	writeRow := func(fields ...string) {
		if err := tsv.Write(fields); err != nil {
			t.Fatal(err)
		}
	}
	writeRow("api", "id", "status", "actual", "expected", "group", "resource", "diagnostic", "owner", "reason", "acceptance_status", "diagnostic_status", "expected_diagnostic")
	matches, compileMatches, acceptanceMatches, diagnosticMatches, domMatches := 0, 0, 0, 0, 0
	for _, api := range versions {
		apiMatches, apiCompileMatches, apiAcceptanceMatches, apiDiagnosticMatches, apiDOMMatches := 0, 0, 0, 0, 0
		for _, c := range table.Cases {
			root := t.TempDir()
			input := c.Inputs[api]
			for path, source := range input.Files {
				if filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "..") {
					t.Fatalf("%s: invalid fixture path %q", c.ID, path)
				}
				fullPath := filepath.Join(root, path)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(fullPath, []byte(source), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, diagnostics := v03Execute(t, root, input.Name, c)
			want := c.Expected[api]
			actualDiagnostic := v03Encode(t, diagnostics)
			expectedDiagnostic := v03Encode(t, c.Diagnostics[api])
			acceptanceMatch := got == want
			diagnosticMatch := c.Kind != "compile" || actualDiagnostic == expectedDiagnostic
			status, acceptanceStatus, diagnosticStatus := "MISMATCH", "MISMATCH", "NOT_APPLICABLE"
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
			// Only reporting hides temporary paths. Equality above retains every
			// observed string; phase 1 supplies no carries or inferred reasons.
			writeRow(api, c.ID, status, got, want, c.Group, "ApexPage", strings.ReplaceAll(actualDiagnostic, root, "<fixture>"), "", "", acceptanceStatus, diagnosticStatus, expectedDiagnostic)
			if !capture && (!acceptanceMatch || !diagnosticMatch) {
				t.Errorf("API %s %s: actual <%s> expected <%s>; diagnostics <%s> expected <%s>", api, c.ID, got, want, actualDiagnostic, expectedDiagnostic)
			}
		}
		writeRow("API_TOTAL", api, fmt.Sprintf("%d/299", apiMatches))
		writeRow("COMPILE_API_TOTAL", api, fmt.Sprintf("%d/165", apiCompileMatches))
		writeRow("COMPILE_ACCEPTANCE_API_TOTAL", api, fmt.Sprintf("%d/165", apiAcceptanceMatches))
		writeRow("DIAGNOSTIC_API_TOTAL", api, fmt.Sprintf("%d/165", apiDiagnosticMatches))
		writeRow("DOM_API_TOTAL", api, fmt.Sprintf("%d/134", apiDOMMatches))
		t.Logf("Page messages API %s exact %d/299; compile %d/165, acceptance %d/165, diagnostics %d/165; DOM %d/134", api, apiMatches, apiCompileMatches, apiAcceptanceMatches, apiDiagnosticMatches, apiDOMMatches)
		matches += apiMatches
		compileMatches += apiCompileMatches
		acceptanceMatches += apiAcceptanceMatches
		diagnosticMatches += apiDiagnosticMatches
		domMatches += apiDOMMatches
	}
	writeRow("COMPILE_TOTAL", fmt.Sprintf("%d/330", compileMatches))
	writeRow("COMPILE_ACCEPTANCE_TOTAL", fmt.Sprintf("%d/330", acceptanceMatches))
	writeRow("DIAGNOSTIC_TOTAL", fmt.Sprintf("%d/330", diagnosticMatches))
	writeRow("DOM_TOTAL", fmt.Sprintf("%d/268", domMatches))
	writeRow("TOTAL", fmt.Sprintf("%d/598", matches))
	t.Logf("Page messages exact matches %d/598; compile %d/330; DOM %d/268", matches, compileMatches, domMatches)
	tsv.Flush()
	if err := tsv.Error(); err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("GLADE_V03_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func v03Execute(t *testing.T, root, name string, c v03Case) (string, []v03Diagnostic) {
	t.Helper()
	diagnostics := []v03Diagnostic{}
	fail := func(err error, resource string) (string, []v03Diagnostic) {
		diagnostics = append(diagnostics, v03Diagnostic{Resource: resource, Type: "Error", Message: err.Error()})
		if c.Kind == "runtime" {
			return v03ObserveError(t, err), diagnostics
		}
		return "COMPILE_ERROR", diagnostics
	}
	p, err := project.Load(root)
	if err != nil {
		return fail(err, "ApexPage")
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		return fail(err, "ApexClass")
	}
	index := typesys.Build(p, sch)
	if c.Kind == "compile" {
		analysis := sema.AnalyzeWithOptions(apextest.SemanticAnalysisIndex(index), sema.AnalyzeOptions{Diagnostics: true, SuppressPerformanceDiagnostics: true})
		for _, d := range analysis.Diagnostics {
			if d.Severity == diagnostic.Error {
				diagnostics = append(diagnostics, v03Diagnostic{Resource: "ApexClass", Type: "Error", Message: d.Message})
			}
		}
	}
	loader := visualforce.LoadProject
	if c.Kind == "runtime" {
		loader = visualforce.LoadProjectForRender
	}
	vf, err := loader(p)
	if err != nil {
		diagnostics = append(diagnostics, v03Diagnostic{Resource: "ApexPage", Type: "Error", Message: err.Error()})
	}
	v03SortDiagnostics(diagnostics)
	if c.Kind == "compile" {
		if len(diagnostics) != 0 {
			return "COMPILE_ERROR", diagnostics
		}
		return "COMPILE_OK", diagnostics
	}
	if err != nil {
		return v03ObserveError(t, err), diagnostics
	}
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	machine := vm.New(nil)
	machine.Org = &org
	if err := apextest.RegisterProjectRuntimeForRequest(machine, index); err != nil {
		return fail(err, "ApexClass")
	}
	result, err := visualforce.RenderPage(visualforce.PageRenderRequest{Project: p, VFIndex: vf, Machine: machine, PageName: name, PageURL: "/apex/" + name + c.Query})
	if err != nil {
		return fail(err, "ApexPage")
	}
	if result.Error != nil {
		return fail(result.Error, "ApexPage")
	}
	if result.RedirectURL != "" {
		// Retain navigation as an observed local result rather than pretending
		// that the current page's markup was rendered after a redirect.
		return v03DOM(t, map[string]any{"rootPresent": false, "redirectURL": result.RedirectURL, "redirect": result.Redirect}), diagnostics
	}
	return v03ObserveDOM(t, result.HTML, c.ID), diagnostics
}

func v03SortDiagnostics(values []v03Diagnostic) {
	sort.SliceStable(values, func(i, j int) bool {
		a, b := values[i], values[j]
		if a.Resource != b.Resource {
			return a.Resource < b.Resource
		}
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		return a.Message < b.Message
	})
}

func v03Encode(t *testing.T, value any) string {
	t.Helper()
	var out strings.Builder
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(out.String(), "\n")
}

var v03FixtureVersion = regexp.MustCompile(`FamilyV03([CR]\d+V)(59|6[0-7])`)
var v03StackPosition = regexp.MustCompile(`(line )\d+(, column )\d+`)

func v03ObserveError(t *testing.T, err error) string {
	t.Helper()
	// Apply only the two documented native observer replacements. Preserve
	// casing, text and each error line; never substitute an expected answer.
	text := v03FixtureVersion.ReplaceAllString(err.Error(), "FamilyV03${1}<API>")
	text = v03StackPosition.ReplaceAllString(text, "${1}<N>${2}<N>")
	return v03DOM(t, map[string]any{"nativeRenderError": strings.Split(text, "\n"), "rootPresent": false})
}

func v03ObserveDOM(t *testing.T, markup, id string) string {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		return v03ObserveError(t, err)
	}
	var root *html.Node
	var find func(*html.Node)
	find = func(n *html.Node) {
		for _, attr := range n.Attr {
			if attr.Key == "data-v03-case" && attr.Val == id {
				if root != nil {
					t.Fatalf("duplicate local Page messages DOM row %s", id)
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
		return v03DOM(t, map[string]any{"rootPresent": false})
	}
	var text strings.Builder
	var collect func(*html.Node)
	collect = func(n *html.Node) {
		if n.Type == html.TextNode {
			text.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			collect(child)
		}
	}
	collect(root)
	value := text.String()
	var parsed any
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.UseNumber()
	if decoder.Decode(&parsed) == nil {
		// Like the native observer, canonicalize JSON objects/arrays only.
		// json.Valid prevents accepting a partial JSON prefix or trailing text.
		if json.Valid([]byte(value)) {
			switch parsed.(type) {
			case map[string]any, []any:
				value = v03Encode(t, parsed)
			}
		}
	}
	return v03DOM(t, map[string]any{"rootPresent": true, "text": value})
}

// The captured TSV uses Python ensure_ascii=True for its outer JSON framing.
// Escape framing identically while preserving the decoded observed text.
func v03DOM(t *testing.T, value any) string {
	t.Helper()
	encoded := v03Encode(t, value)
	var out strings.Builder
	out.WriteString("DOM|")
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
