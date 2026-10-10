package server

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
	"golang.org/x/net/html"
)

type v16File struct {
	Text   string `json:"text"`
	Base64 string `json:"base64"`
}

type v16Input struct {
	Name  string             `json:"name"`
	Page  string             `json:"page"`
	Files map[string]v16File `json:"files"`
}

type v16Diagnostic struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type v16Case struct {
	ID               string                   `json:"id"`
	Group            string                   `json:"group"`
	Kind             string                   `json:"kind"`
	Resource         string                   `json:"resource"`
	Observe          string                   `json:"observe"`
	Finish           bool                     `json:"finish"`
	Inputs           map[string]v16Input      `json:"inputs"`
	Expected         map[string]string        `json:"expected"`
	Diagnostics      map[string]v16Diagnostic `json:"diagnostics"`
	Owner            string                   `json:"owner,omitempty"`
	Reason           string                   `json:"reason,omitempty"`
	DiagnosticOwner  string                   `json:"diagnostic_owner,omitempty"`
	DiagnosticReason string                   `json:"diagnostic_reason,omitempty"`
}

type v16Table struct {
	Oracle struct {
		Source     string   `json:"source"`
		Versions   []string `json:"api_versions"`
		RowsPerAPI int      `json:"rows_per_api"`
	} `json:"oracle"`
	Support map[string]map[string]v16File `json:"support"`
	Cases   []v16Case                     `json:"cases"`
}

type v16Observation struct {
	Text       string
	Diagnostic v16Diagnostic
	Unobserved string
	Root       string
}

type v16Environment struct {
	Root       string
	Index      typesys.Index
	Runtime    *vm.VM
	RuntimeErr error
}

// TestV16SalesforceConformance compares all 184 native rows at API 59/67 from
// the native alternate rendering capture. Its captured sources, metadata,
// support resources, diagnostics and org.tsv answers are exported to testdata.
// Local compile/metadata acceptance uses the product loaders; runtime rows use
// the server's HTTP/PDF response and the captured email controller unchanged.
// No row assertion uses Apex == or System.assertEquals: observed text and
// diagnostic type/text use exact Go equality. JSON nulls remain raw nulls.
//
// Like Component structure, DOM inspection uses Go's HTML parser, without a browser. Callback,
// page-error, layout and Finish observations requiring a hosted browser observation
// are explicitly UNOBSERVED and never counted as matches, even if a partial DOM
// resembles the native answer. Remaining Messaging and hosted observations are
// logged with unfinished owners; local compiler acceptance is always asserted.
// GLADE_V16_CAPTURE=1 records every row instead of failing on behavior differences;
// GLADE_V16_REPORT selects the optional per-row TSV output path.
// The captured support sources and Apex runtime are prepared once per API.
// Each row still loads only its own page/template and gets a fresh org, server
// and request runtime clone. TSV rows are flushed immediately; TOTAL is written
// only after both APIs finish, so interrupted captures remain visibly incomplete.
func TestV16SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_V16_CAPTURE") == "1"
	data, err := os.ReadFile("testdata/v16_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table v16Table
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	versions := []string{"59.0", "67.0"}
	counts, seen := map[string]int{}, map[string]bool{}
	if table.Oracle.Source != "Owned email-template observations at API 59.0 and 67.0." ||
		table.Oracle.RowsPerAPI != 184 || !reflect.DeepEqual(table.Oracle.Versions, versions) {
		t.Fatal("Email templates requires the complete, committed API 59/67 oracle")
	}
	for _, c := range table.Cases {
		if c.ID == "" || seen[c.ID] || (c.Kind != "compile" && c.Kind != "runtime") ||
			(c.Resource != "ApexPage" && c.Resource != "EmailTemplate") {
			t.Fatalf("invalid or duplicate Email templates case %q (%s/%s)", c.ID, c.Kind, c.Resource)
		}
		seen[c.ID] = true
		counts[c.Kind]++
		if (c.Owner == "") != (c.Reason == "") || (c.DiagnosticOwner == "") != (c.DiagnosticReason == "") {
			t.Fatalf("%s: every carried fact requires both owner and reason", c.ID)
		}
		for _, owner := range []string{c.Owner, c.DiagnosticOwner} {
			if owner != "" && owner != "Messaging" && owner != "Hosted navigation" {
				t.Fatalf("%s: unsupported carry owner %q; Email templates and Done families cannot own carries", c.ID, owner)
			}
		}
		if (c.Kind == "compile" && c.Owner != "") || (c.Kind != "compile" && c.DiagnosticOwner != "") {
			t.Fatalf("%s: compiler acceptance cannot be carried; diagnostic carries require a compile row", c.ID)
		}
		for _, api := range versions {
			input, want := c.Inputs[api], c.Expected[api]
			if input.Name == "" || input.Page == "" || len(input.Files) < 2 || len(table.Support[api]) != 11 {
				t.Fatalf("%s: incomplete captured inputs at API %s", c.ID, api)
			}
			if c.Kind == "compile" {
				d, ok := c.Diagnostics[api]
				if !ok || (want != "COMPILE_OK" && want != "COMPILE_ERROR") ||
					(want == "COMPILE_ERROR" && (d.Type == "" || d.Message == "")) ||
					(want == "COMPILE_OK" && d != (v16Diagnostic{})) {
					t.Fatalf("%s: missing/inconsistent compile answer at API %s", c.ID, api)
				}
			} else {
				if !strings.HasPrefix(want, "BROWSER|") {
					t.Fatalf("%s: missing native runtime answer at API %s", c.ID, api)
				}
				var value map[string]any
				if err := v16Decode(strings.TrimPrefix(want, "BROWSER|"), &value); err != nil || value == nil ||
					"BROWSER|"+v16JSON(t, value) != want {
					t.Fatalf("%s: native runtime text changed at API %s", c.ID, api)
				}
			}
		}
	}
	if len(table.Cases) != 184 || counts["compile"] != 102 || counts["runtime"] != 82 {
		t.Fatalf("Email templates requires 102 compile/metadata and 82 runtime rows, got %d, %v", len(table.Cases), counts)
	}

	// Keep PDF observation at the service boundary: the real local renderer,
	// signature and MIME type, with no exact hosted bytes or layout assertions.
	t.Cleanup(visualforce.SetPDFRendererForTest(visualforce.PDFToolchain{}))
	var report io.Writer = io.Discard
	if path := os.Getenv("GLADE_V16_REPORT"); path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := file.Close(); err != nil {
				t.Error(err)
			}
		})
		report = file
	}
	tsv := csv.NewWriter(report)
	tsv.Comma = '\t'
	write := func(fields ...string) {
		if err := tsv.Write(fields); err != nil {
			t.Fatal(err)
		}
		tsv.Flush()
		if err := tsv.Error(); err != nil {
			t.Fatal(err)
		}
	}
	write("api", "id", "status", "actual", "expected", "group", "resource", "kind", "diagnostic_type", "diagnostic", "expected_diagnostic_type", "expected_diagnostic", "acceptance_status", "diagnostic_status", "owner", "reason", "unobserved")
	matches, compileMatches, runtimeMatches, unobserved := 0, 0, 0, 0
	for _, api := range versions {
		environment := v16PrepareEnvironment(t, table.Support[api], api)
		apiMatches, apiCompile, apiRuntime, apiUnobserved := 0, 0, 0, 0
		for _, c := range table.Cases {
			local := v16ObserveCase(t, environment, c, api)
			want, wantDiagnostic := c.Expected[api], c.Diagnostics[api]
			acceptanceMatch := local.Text == want
			diagnosticMatch := c.Kind != "compile" || local.Diagnostic == wantDiagnostic
			diagnosticCarry := c.DiagnosticOwner != "" && acceptanceMatch &&
				local.Diagnostic.Type == wantDiagnostic.Type &&
				v16FlowDiagnosticCarry(local.Diagnostic.Message, wantDiagnostic.Message)
			acceptanceStatus, diagnosticStatus := "MISMATCH", "NOT_APPLICABLE"
			if acceptanceMatch {
				acceptanceStatus = "MATCH"
			}
			if c.Kind == "compile" {
				diagnosticStatus = "MISMATCH"
				if diagnosticMatch {
					diagnosticStatus = "MATCH"
				}
			}
			status, owner, reason := "MISMATCH", "Email templates", ""
			if local.Unobserved != "" {
				status, reason = "UNOBSERVED", local.Unobserved
				unobserved++
				apiUnobserved++
			} else if acceptanceMatch && diagnosticMatch {
				status, owner = "MATCH", ""
				matches++
				apiMatches++
				if c.Kind == "compile" {
					compileMatches++
					apiCompile++
				} else {
					runtimeMatches++
					apiRuntime++
				}
			} else if !diagnosticMatch && acceptanceMatch {
				reason = fmt.Sprintf("diagnostic expected <%s: %s> actual <%s: %s>", wantDiagnostic.Type, wantDiagnostic.Message, local.Diagnostic.Type, local.Diagnostic.Message)
			} else {
				reason = v16Difference(t, local.Text, want)
			}
			// Owners describe only remaining differences. A row which matches is
			// always enforced and counted, even if an old carry field remains.
			if status != "MATCH" {
				if c.Kind == "compile" && !diagnosticMatch && diagnosticCarry {
					owner, reason = c.DiagnosticOwner, c.DiagnosticReason+"; "+reason
				} else if c.Kind == "runtime" && c.Owner != "" {
					owner, reason = c.Owner, c.Reason+"; "+reason
				}
			}
			reason = strings.NewReplacer("\r", " ", "\n", " ").Replace(reason)
			// Comparisons above retain the exact original text; only displayed
			// temporary fixture paths are replaced in the report.
			display := func(s string) string { return strings.ReplaceAll(s, local.Root, "<fixture>") }
			write(api, c.ID, status, display(local.Text), want, c.Group, c.Resource, c.Kind,
				local.Diagnostic.Type, display(local.Diagnostic.Message), wantDiagnostic.Type, wantDiagnostic.Message,
				acceptanceStatus, diagnosticStatus, owner, display(reason), local.Unobserved)
			if !capture && status != "MATCH" {
				if c.Kind == "compile" {
					if !acceptanceMatch {
						t.Errorf("API %s %s acceptance expected <%s> actual <%s>", api, c.ID, want, local.Text)
					}
					if local.Diagnostic.Type != wantDiagnostic.Type {
						t.Errorf("API %s %s diagnostic type expected <%s> actual <%s>", api, c.ID, wantDiagnostic.Type, local.Diagnostic.Type)
					}
					if !diagnosticMatch {
						if diagnosticCarry {
							t.Logf("API %s %s diagnostic carried by %s: %s", api, c.ID, owner, reason)
						} else {
							t.Errorf("API %s %s: %s", api, c.ID, reason)
						}
					}
				} else if c.Owner != "" {
					t.Logf("API %s %s %s carried by %s: %s", api, c.ID, status, owner, reason)
				} else {
					t.Errorf("API %s %s: %s; expected <%s> actual <%s>", api, c.ID, reason, want, local.Text)
				}
			}
		}
		write("API_TOTAL", api, fmt.Sprintf("%d/%d", apiMatches, len(table.Cases)))
		write("COMPILE_API_TOTAL", api, fmt.Sprintf("%d/%d", apiCompile, counts["compile"]))
		write("RUNTIME_API_TOTAL", api, fmt.Sprintf("%d/%d", apiRuntime, counts["runtime"]))
		write("UNOBSERVED_API_TOTAL", api, fmt.Sprint(apiUnobserved))
		t.Logf("Email templates API %s exact matches %d/%d (compile/metadata %d/%d, runtime %d/%d, unobserved %d)",
			api, apiMatches, len(table.Cases), apiCompile, counts["compile"], apiRuntime, counts["runtime"], apiUnobserved)
	}
	write("COMPILE_TOTAL", fmt.Sprintf("%d/%d", compileMatches, counts["compile"]*len(versions)))
	write("RUNTIME_TOTAL", fmt.Sprintf("%d/%d", runtimeMatches, counts["runtime"]*len(versions)))
	write("UNOBSERVED_TOTAL", fmt.Sprint(unobserved))
	write("TOTAL", fmt.Sprintf("%d/%d", matches, len(table.Cases)*len(versions)))
	t.Logf("Email templates exact matches %d/%d; unobserved %d", matches, len(table.Cases)*len(versions), unobserved)
}

func v16WriteFiles(t *testing.T, root string, files map[string]v16File) {
	t.Helper()
	for name, file := range files {
		if filepath.IsAbs(name) || filepath.Clean(name) != name || !strings.HasPrefix(name, "force-app/") || strings.HasPrefix(name, "../") {
			t.Fatalf("invalid exported path %q", name)
		}
		content := []byte(file.Text)
		if file.Base64 != "" {
			var err error
			content, err = base64.StdEncoding.DecodeString(file.Base64)
			if err != nil {
				t.Fatal(err)
			}
		}
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func v16PrepareEnvironment(t *testing.T, support map[string]v16File, api string) v16Environment {
	t.Helper()
	root := t.TempDir()
	v16WriteFiles(t, root, support)
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
	environment := v16Environment{Root: root}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		environment.RuntimeErr = err
		return environment
	}
	template := &Server{}
	template.SetProjectIndex(typesys.Build(p, schema))
	environment.Index = *template.Index
	environment.Runtime, environment.RuntimeErr = template.runtime, template.runtimeErr
	return environment
}

func v16ObserveCase(t *testing.T, environment v16Environment, c v16Case, api string) v16Observation {
	t.Helper()
	root, input := environment.Root, c.Inputs[api]
	local := v16Observation{Text: "COMPILE_OK", Root: root}
	// The source root is stable so compiling an identical helper does not create
	// a new cache entry per row. Remove each row's files before loading the next:
	// rejected pages and templates must never contaminate a following case.
	for name := range input.Files {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("%s: case path %q is not isolated: %v", c.ID, name, err)
		}
	}
	v16WriteFiles(t, root, input.Files)
	defer func() {
		for name := range input.Files {
			if err := os.Remove(filepath.Join(root, name)); err != nil {
				t.Error(err)
			}
		}
	}()
	p, compileErr := project.Load(root)
	if compileErr == nil {
		_, compileErr = resource.LoadProject(p)
	}
	if compileErr == nil {
		_, compileErr = visualforce.LoadProject(p)
	}
	if compileErr != nil {
		local.Text = "COMPILE_ERROR"
		local.Diagnostic = v16Diagnostic{Type: "Error", Message: compileErr.Error()}
		if c.Kind == "runtime" {
			local.Text = "BROWSER|" + v16JSON(t, map[string]any{"kind": "native-deploy-error", "diagnostics": []string{compileErr.Error()}})
		}
		return local
	}
	if c.Kind == "compile" {
		return local
	}

	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		local.Text = "LOCAL_SETUP_ERROR|" + err.Error()
		return local
	}
	org := storage.NewOrgState()
	if err := resource.ApplyProject(&org, p); err != nil {
		local.Text = "LOCAL_SETUP_ERROR|" + err.Error()
		return local
	}
	srv := NewWithSource(&org, source)
	configureVisualforceTestPrincipal(t, srv)
	if environment.RuntimeErr != nil {
		local.Text = "LOCAL_SETUP_ERROR|" + environment.RuntimeErr.Error()
		return local
	}
	runtime := environment.Runtime.CloneRuntime(nil)
	runtime.RegisterPageReference(input.Page)
	srv.SetProjectRuntime(environment.Index, runtime, nil)
	pageURL := "/apex/" + input.Page
	if c.Resource == "EmailTemplate" {
		pageURL += "?ownedTemplate=" + url.QueryEscape(input.Name)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, pageURL, nil))
	if c.Observe == "http" {
		local.Text = "BROWSER|" + v16JSON(t, v16ObserveHTTP(rec))
		return local
	}
	doc, err := html.Parse(strings.NewReader(rec.Body.String()))
	if err != nil {
		local.Text = "LOCAL_DOM_ERROR|" + err.Error()
		return local
	}
	marker := v16Find(doc, "data-v16-case", c.ID)
	if marker == nil {
		message := v16HTTPText(rec.Body.String())
		if messages := v16RenderErrorMessages(doc); len(messages) > 0 {
			message = v16URLs.ReplaceAllString(strings.Join(messages, " "), "<URL>")
			units := utf16.Encode([]rune(message))
			if len(units) > 2048 {
				message = string(utf16.Decode(units[:2048]))
			}
		}
		local.Text = "BROWSER|" + v16JSON(t, map[string]any{"kind": "native-render-error", "status": rec.Code, "message": message})
		return local
	}
	if c.Observe == "email" {
		pre := v16Find(marker, "id", "v16-email")
		var result any
		if pre == nil {
			local.Text = "LOCAL_DOM_ERROR|missing owned email output"
		} else if err := v16Decode(v16Text(pre), &result); err != nil {
			local.Text = "LOCAL_DOM_ERROR|invalid owned email JSON: " + v16Text(pre)
		} else {
			local.Text = "BROWSER|" + v16JSON(t, v16StableValue(map[string]any{"kind": "email", "result": result}))
		}
		return local
	}
	// An actually empty Flow subtree has no layout, callback or Finish state to
	// infer. Every other dynamic embedding requires the hosted browser observer.
	if c.Observe == "flow" && marker.FirstChild == nil && !c.Finish {
		local.Text = "BROWSER|" + v16JSON(t, map[string]any{"kind": "flow", "buttons": []string{}, "display": []string{}, "frames": 0, "no_data": true, "text": ""})
		return local
	}
	local.Text = "LOCAL_DOM|" + v16JSON(t, v16DOM(marker))
	switch c.Observe {
	case "lightning":
		local.Unobserved = "Lightning Out callbacks, component output and page errors require a coordinator browser observation"
	case "flow":
		local.Unobserved = "Flow innerText, rendered controls and Finish state require a coordinator browser observation"
	case "canvas":
		local.Unobserved = "Canvas dynamic boundary text, frame state and page errors require a coordinator browser observation"
	default:
		t.Fatalf("%s: unknown captured observer %q", c.ID, c.Observe)
	}
	return local
}

var (
	v16Scripts         = regexp.MustCompile(`(?is)<script\b[^>]*>[\s\S]*?</script>`)
	v16Styles          = regexp.MustCompile(`(?is)<style\b[^>]*>[\s\S]*?</style>`)
	v16Inputs          = regexp.MustCompile(`(?i)<input\b[^>]*>`)
	v16Tags            = regexp.MustCompile(`<[^>]+>`)
	v16URLs            = regexp.MustCompile(`https?://[^\s"'<>]+`)
	v16Names           = regexp.MustCompile(`FamilyV16(?:_[0-9]+_[0-9]+(?:Host)?|Flow[0-9]+|Out[0-9]+|Text[0-9]+|Email[0-9]+)`)
	v16FlowGeneratedID = regexp.MustCompile(`"j_id[0-9]+(?::[A-Za-z0-9_]+)*"`)
)

func v16FlowDiagnosticCarry(actual, expected string) bool {
	// This does not change the exact row comparison or its match count. Only
	// a quoted generated Flow UI identifier may differ in a carried diagnostic;
	// an unrelated error or any other text difference still fails the row.
	if v16FlowGeneratedID.FindString(actual) == "" || v16FlowGeneratedID.FindString(expected) == "" {
		return false
	}
	return v16FlowGeneratedID.ReplaceAllString(actual, `"<hosted-flow-id>"`) ==
		v16FlowGeneratedID.ReplaceAllString(expected, `"<hosted-flow-id>"`)
}

func v16ObserveHTTP(rec *httptest.ResponseRecorder) map[string]any {
	content := rec.Body.Bytes()
	pdf := bytes.HasPrefix(content, []byte("%PDF-"))
	invalid := !pdf && !utf8.Valid(content)
	var text any
	if !pdf {
		raw := string(content)
		if invalid {
			var escaped strings.Builder
			for _, b := range content {
				if b < 128 {
					escaped.WriteByte(b)
				} else {
					fmt.Fprintf(&escaped, `\x%02x`, b)
				}
			}
			raw = escaped.String()
		}
		text = v16HTTPText(raw)
	}
	return v16StableValue(map[string]any{"kind": "http", "status": rec.Code, "mime": strings.ToLower(rec.Header().Get("Content-Type")), "pdf": pdf, "invalid_utf8": invalid, "text": text}).(map[string]any)
}

// Match the native HTTP observer's raw-text extraction, including entities;
// this is deliberately different from parsed DOM textContent.
func v16HTTPText(raw string) string {
	raw = v16Scripts.ReplaceAllString(raw, "")
	raw = v16Styles.ReplaceAllString(raw, "")
	raw = v16Inputs.ReplaceAllString(raw, "")
	raw = v16Tags.ReplaceAllString(raw, "")
	raw = v16URLs.ReplaceAllString(strings.TrimSpace(raw), "<URL>")
	units := utf16.Encode([]rune(raw))
	if len(units) > 4096 {
		raw = string(utf16.Decode(units[:4096]))
	}
	return raw
}

func v16Find(n *html.Node, key, value string) *html.Node {
	for _, attr := range n.Attr {
		if attr.Key == key && attr.Val == value {
			return n
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := v16Find(child, key, value); found != nil {
			return found
		}
	}
	return nil
}

func v16Text(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var out strings.Builder
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		out.WriteString(v16Text(child))
	}
	return out.String()
}

// The native browser observer reads textContent from this selector union. Go's
// parsed DOM provides the same entity decoding without inferring layout.
func v16RenderErrorMessages(root *html.Node) []string {
	var messages []string
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		matches := false
		if node.Type == html.ElementNode {
			for _, attr := range node.Attr {
				if attr.Key == "id" && (attr.Val == "errorTitle" || attr.Val == "errorBody") {
					matches = true
				}
				if attr.Key == "class" {
					for _, class := range strings.Fields(attr.Val) {
						if class == "errorMsg" || class == "messageText" {
							matches = true
						}
					}
				}
			}
		}
		if matches {
			messages = append(messages, v16Text(node))
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	return messages
}

func v16DOM(root *html.Node) map[string]any {
	var inner strings.Builder
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if err := html.Render(&inner, child); err != nil {
			return map[string]any{"serialization_error": err.Error()}
		}
	}
	return map[string]any{"html": inner.String(), "text_content": v16Text(root)}
}

func v16StableValue(value any) any {
	switch value := value.(type) {
	case string:
		return v16Names.ReplaceAllString(value, "OWNED")
	case []any:
		for i := range value {
			value[i] = v16StableValue(value[i])
		}
	case map[string]any:
		for key := range value {
			value[key] = v16StableValue(value[key])
		}
	}
	return value
}

func v16Decode(text string, value any) error {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	return decoder.Decode(value)
}

// org.tsv uses Python's sorted-key, compact, ensure_ascii JSON. Preserve that
// exact serialization without normalizing any compared fields or nulls.
func v16JSON(t *testing.T, value any) string {
	t.Helper()
	var raw strings.Builder
	encoder := json.NewEncoder(&raw)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	for _, r := range strings.TrimSuffix(raw.String(), "\n") {
		if r < 128 {
			out.WriteRune(r)
		} else if r <= 0xffff {
			fmt.Fprintf(&out, `\u%04x`, r)
		} else {
			high, low := utf16.EncodeRune(r)
			fmt.Fprintf(&out, `\u%04x\u%04x`, high, low)
		}
	}
	return out.String()
}

func v16Difference(t *testing.T, actual, expected string) string {
	if strings.HasPrefix(actual, "BROWSER|") && strings.HasPrefix(expected, "BROWSER|") {
		var a, e any
		if v16Decode(strings.TrimPrefix(actual, "BROWSER|"), &a) == nil &&
			v16Decode(strings.TrimPrefix(expected, "BROWSER|"), &e) == nil {
			return v16FirstDifference(t, "$", a, e)
		}
	}
	return fmt.Sprintf("expected <%s> actual <%s>", expected, actual)
}

func v16FirstDifference(t *testing.T, path string, actual, expected any) string {
	if reflect.DeepEqual(actual, expected) {
		return ""
	}
	if a, ok := actual.(map[string]any); ok {
		if e, ok := expected.(map[string]any); ok {
			keys := map[string]bool{}
			for k := range a {
				keys[k] = true
			}
			for k := range e {
				keys[k] = true
			}
			var ordered []string
			for k := range keys {
				ordered = append(ordered, k)
			}
			sort.Strings(ordered)
			for _, k := range ordered {
				av, aok := a[k]
				ev, eok := e[k]
				if aok != eok {
					return fmt.Sprintf("%s.%s: local field present %t, native field present %t", path, k, aok, eok)
				}
				if reason := v16FirstDifference(t, path+"."+k, av, ev); reason != "" {
					return reason
				}
			}
		}
	}
	return fmt.Sprintf("%s: expected <%s> actual <%s>", path, v16JSON(t, expected), v16JSON(t, actual))
}
