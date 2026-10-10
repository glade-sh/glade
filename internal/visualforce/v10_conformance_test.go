package visualforce_test

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/url"
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

type v10Input struct {
	Name  string            `json:"name"`
	Files map[string]string `json:"files"`
}

type v10Diagnostic struct {
	Resource string `json:"resource"`
	Type     string `json:"type"`
	Message  string `json:"message"`
}

type v10Case struct {
	ID             string                     `json:"id"`
	Group          string                     `json:"group"`
	Kind           string                     `json:"kind"`
	Query          string                     `json:"query"`
	Fills          map[string]any             `json:"fills"`
	FillOrder      []string                   `json:"fill_order"`
	FillAll        bool                       `json:"fill_all"`
	SubmitRow      string                     `json:"submit_row,omitempty"`
	SubmitPosition string                     `json:"submit_position,omitempty"`
	Submissions    int                        `json:"submissions"`
	Inputs         map[string]v10Input        `json:"inputs"`
	Expected       map[string]string          `json:"expected"`
	Diagnostics    map[string][]v10Diagnostic `json:"diagnostics"`
	Owner          string                     `json:"owner,omitempty"`
	Reason         string                     `json:"reason,omitempty"`
}

// TestV10SalesforceConformance compares every captured compile/metadata and
// GET/postback DOM row at API 59/67, including exact diagnostic text. Owned
// fixtures include repeat, identity, region and ordinary-command controls.
// Request runtime, form binding, view state and partial rendering use product
// code. The adapter observes textContent and controls and submits only rendered
// successful controls; it supplies no Apex getters, setters, validation or
// action behavior. CI reads testdata without credentials or a browser.
// GLADE_V10_CAPTURE=1 writes differences without failing on row mismatches;
// GLADE_V10_REPORT selects the per-row TSV output path.
func TestV10SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_V10_CAPTURE") != ""
	data, err := os.ReadFile("testdata/v10_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Source string    `json:"source"`
		Cases  []v10Case `json:"cases"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if table.Source != "Owned form-lifecycle observations and review controls at API 59.0 and 67.0." || len(table.Cases) != 307 {
		t.Fatalf("Form lifecycle requires 216 original cases plus 91 captured review controls, got %d", len(table.Cases))
	}
	versions := []string{"59.0", "67.0"}
	counts, seen := map[string]int{}, map[string]bool{}
	for _, c := range table.Cases {
		if c.ID == "" || seen[c.ID] || (c.Kind != "compile" && c.Kind != "runtime") {
			t.Fatalf("invalid or duplicate Form lifecycle case %q (%s)", c.ID, c.Kind)
		}
		seen[c.ID] = true
		if (c.Owner == "") != (c.Reason == "") || strings.Contains(c.Owner, "Form lifecycle") {
			t.Fatalf("%s: invalid carry owner/reason", c.ID)
		}
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
				var stages []json.RawMessage
				if len(native) != 1 || json.Unmarshal(native["stages"], &stages) != nil || len(stages) == 0 {
					t.Fatalf("%s: incomplete native stages at API %s", c.ID, api)
				}
			}
		}
	}
	if counts["compile"] != 164 || counts["runtime"] != 143 {
		t.Fatalf("Form lifecycle requires 164 compile/metadata and 143 DOM rows, got %v", counts)
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
			got, diagnostics := v10Execute(t, root, input.Name, c)
			want := c.Expected[api]
			actualDiagnostic := v10Encode(t, diagnostics)
			expectedDiagnostic := v10Encode(t, c.Diagnostics[api])
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
			owner, reason := "", ""
			if (!acceptanceMatch || !diagnosticMatch) && c.Owner != "" {
				owner, reason = c.Owner, c.Reason
				t.Logf("API %s %s carry %s: %s", api, c.ID, owner, reason)
			}
			// Equality above retains every observed string. Carries remain
			// mismatches in the denominator, with a named active owner.
			writeRow(api, c.ID, status, got, want, c.Group, "ApexPage", strings.ReplaceAll(actualDiagnostic, root, "<fixture>"), owner, reason, acceptanceStatus, diagnosticStatus, expectedDiagnostic)
			if !capture && owner == "" && (!acceptanceMatch || !diagnosticMatch) {
				t.Errorf("API %s %s: actual <%s> expected <%s>; diagnostics <%s> expected <%s>", api, c.ID, got, want, actualDiagnostic, expectedDiagnostic)
			}
		}
		writeRow("API_TOTAL", api, fmt.Sprintf("%d/%d", apiMatches, len(table.Cases)))
		writeRow("COMPILE_API_TOTAL", api, fmt.Sprintf("%d/%d", apiCompileMatches, counts["compile"]))
		writeRow("COMPILE_ACCEPTANCE_API_TOTAL", api, fmt.Sprintf("%d/%d", apiAcceptanceMatches, counts["compile"]))
		writeRow("DIAGNOSTIC_API_TOTAL", api, fmt.Sprintf("%d/%d", apiDiagnosticMatches, counts["compile"]))
		writeRow("DOM_API_TOTAL", api, fmt.Sprintf("%d/%d", apiDOMMatches, counts["runtime"]))
		t.Logf("Form lifecycle API %s exact %d/%d; compile %d/%d, acceptance %d/%d, diagnostics %d/%d; DOM %d/%d", api, apiMatches, len(table.Cases), apiCompileMatches, counts["compile"], apiAcceptanceMatches, counts["compile"], apiDiagnosticMatches, counts["compile"], apiDOMMatches, counts["runtime"])
		matches += apiMatches
		compileMatches += apiCompileMatches
		acceptanceMatches += apiAcceptanceMatches
		diagnosticMatches += apiDiagnosticMatches
		domMatches += apiDOMMatches
	}
	writeRow("COMPILE_TOTAL", fmt.Sprintf("%d/%d", compileMatches, len(versions)*counts["compile"]))
	writeRow("COMPILE_ACCEPTANCE_TOTAL", fmt.Sprintf("%d/%d", acceptanceMatches, len(versions)*counts["compile"]))
	writeRow("DIAGNOSTIC_TOTAL", fmt.Sprintf("%d/%d", diagnosticMatches, len(versions)*counts["compile"]))
	writeRow("DOM_TOTAL", fmt.Sprintf("%d/%d", domMatches, len(versions)*counts["runtime"]))
	writeRow("TOTAL", fmt.Sprintf("%d/%d", matches, len(versions)*len(table.Cases)))
	t.Logf("Form lifecycle exact matches %d/%d; compile %d/%d; DOM %d/%d", matches, len(versions)*len(table.Cases), compileMatches, len(versions)*counts["compile"], domMatches, len(versions)*counts["runtime"])
	tsv.Flush()
	if err := tsv.Error(); err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("GLADE_V10_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func v10Execute(t *testing.T, root, name string, c v10Case) (string, []v10Diagnostic) {
	t.Helper()
	diagnostics := []v10Diagnostic{}
	fail := func(err error, resource string) (string, []v10Diagnostic) {
		diagnostics = append(diagnostics, v10Diagnostic{Resource: resource, Type: "Error", Message: err.Error()})
		if c.Kind == "runtime" {
			return v10DOM(t, map[string]any{"stages": []any{map[string]any{"phase": "get", "value": v10Error(err)}}}), diagnostics
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
				diagnostics = append(diagnostics, v10Diagnostic{Resource: "ApexClass", Type: "Error", Message: d.Message})
			}
		}
	}
	loader := visualforce.LoadProject
	if c.Kind == "runtime" {
		loader = visualforce.LoadProjectForRender
	}
	vf, err := loader(p)
	if err != nil {
		diagnostics = append(diagnostics, v10Diagnostic{Resource: "ApexPage", Type: "Error", Message: err.Error()})
	}
	v10SortDiagnostics(diagnostics)
	if c.Kind == "compile" {
		if len(diagnostics) != 0 {
			return "COMPILE_ERROR", diagnostics
		}
		return "COMPILE_OK", diagnostics
	}
	if err != nil {
		return v10DOM(t, map[string]any{"stages": []any{map[string]any{"phase": "get", "value": v10Error(err)}}}), diagnostics
	}
	org := storage.NewOrgState()
	request := visualforce.PageRenderRequest{Project: p, VFIndex: vf, Org: &org, PageName: name, PageURL: "/apex/" + name + c.Query, ViewStateSecret: []byte("Form lifecycle owned conformance view-state secret")}
	// Each request uses a fresh VM, as the server does. State crosses requests
	// only via the product's encoded view state and submitted DOM controls.
	render := func() (visualforce.PageRenderResult, error) {
		machine := vm.New(nil)
		machine.Org = &org
		if err := apextest.RegisterProjectRuntimeForRequest(machine, index); err != nil {
			return visualforce.PageRenderResult{}, err
		}
		request.Machine = machine
		result, err := visualforce.RenderPage(request)
		if err == nil && result.Error != nil {
			err = result.Error
		}
		return result, err
	}
	result, err := render()
	var doc *html.Node
	observe := func() map[string]any {
		if err != nil {
			return v10Error(err)
		}
		if result.RedirectURL != "" {
			// Follow the actual local navigation once, just like a native full postback.
			target, parseErr := url.Parse(result.RedirectURL)
			if parseErr != nil {
				return v10Error(parseErr)
			}
			if !strings.HasPrefix(target.Path, "/apex/") {
				return map[string]any{"redirectURL": result.RedirectURL}
			}
			request.PageURL = result.RedirectURL
			request.PageName = strings.TrimPrefix(target.Path, "/apex/")
			request.Action, request.FormValues, request.ViewState = "", nil, nil
			result, err = render()
			if err != nil {
				return v10Error(err)
			}
			if result.RedirectURL != "" {
				return map[string]any{"redirectURL": result.RedirectURL}
			}
		}
		doc, err = html.Parse(strings.NewReader(result.HTML))
		if err != nil {
			return v10Error(err)
		}
		return v10Observe(doc, c.ID)
	}
	stages := []any{map[string]any{"phase": "get", "value": observe()}}
	for submission := 1; submission <= c.Submissions && err == nil && doc != nil; submission++ {
		root := v10Find(doc, func(n *html.Node) bool { return v10Attr(n, "data-v10-case") == c.ID })
		if root == nil {
			break
		}
		filled := v10Fill(root, c)
		submitRoot := root
		if c.SubmitRow != "" {
			submitRoot = v10Find(root, func(n *html.Node) bool { return v10Attr(n, "data-v10-repeat-row") == c.SubmitRow })
		}
		if c.SubmitPosition != "" {
			submitRoot = v10Find(root, func(n *html.Node) bool { return v10Attr(n, "data-v10-command-position") == c.SubmitPosition })
		}
		buttonRoot := v10Find(submitRoot, func(n *html.Node) bool { return v10Attr(n, "data-v10-submit") == "owned" })
		button := v10Find(buttonRoot, func(n *html.Node) bool { return n.Type == html.ElementNode && (n.Data == "input" || n.Data == "a") })
		if button == nil {
			stages = append(stages, map[string]any{"phase": "postback", "submission": submission, "filled": filled, "value": map[string]any{"localError": "rendered submit control missing"}})
			break
		}
		form := v10Ancestor(button, func(n *html.Node) bool { return n.Data == "form" })
		if form == nil {
			form = v10Find(doc, func(n *html.Node) bool { return n.Data == "form" })
		}
		if form == nil {
			stages = append(stages, map[string]any{"phase": "postback", "submission": submission, "filled": filled, "value": map[string]any{"localError": "rendered submit form missing"}})
			break
		}
		// Native required_blank/region_*_required never send a request: the
		// browser checks all enabled required controls in the submitted form,
		// including controls outside an actionRegion and immediate actions.
		if button.Data != "a" && !v10HasAttr(form, "novalidate") && !v10HasAttr(button, "formnovalidate") && v10Find(form, func(n *html.Node) bool {
			return v10Control(n) && !v10HasAttr(n, "disabled") && v10HasAttr(n, "required") && v10ControlValue(n) == ""
		}) != nil {
			stages = append(stages, map[string]any{"phase": "postback", "submission": submission, "filled": filled, "value": v10Observe(doc, c.ID)})
			continue
		}
		hook := v10Attr(button, "onclick")
		action := v10Attr(button, "data-action")
		targets := ""
		ajax := v10AjaxHook.FindStringSubmatch(hook)
		if len(ajax) == 3 {
			action, targets = ajax[1], ajax[2]
		} else if action == "" {
			if match := v10ActionHook.FindStringSubmatch(hook); len(match) == 2 {
				action = match[1]
			}
		}
		scope := form
		if len(ajax) == 3 && strings.Contains(hook, "[data-vf-region]") {
			if region := v10Ancestor(button, func(n *html.Node) bool { return v10HasAttr(n, "data-vf-region") }); region != nil {
				scope = region
			}
		}
		v10SelectCommand(t, doc, form, button, hook)
		values := v10FormValues(scope, button, len(ajax) != 3)
		if scope != form {
			v10AppendAjaxFormControls(t, doc, form, values)
		}
		values[visualforce.ViewStateActionFieldName()] = action
		if len(ajax) == 3 {
			values["__vf_ajax"], values["__vf_rerender"] = "1", targets
		}
		payload, decodeErr := visualforce.DecodeViewState(values[visualforce.ViewStateFormFieldName()], request.ViewStateSecret)
		if decodeErr == nil {
			decodeErr = visualforce.VerifyViewStateCSRF(payload, values["__vf_csrf"])
		}
		if decodeErr != nil {
			err = decodeErr
		} else {
			parsed := visualforce.ParseAjaxPayload(values)
			request.ViewState, request.FormValues, request.Action = &payload, parsed.SubmittedFields, parsed.Action
			request.PageURL = v10Attr(form, "action")
			result, err = render()
		}
		var value map[string]any
		if err != nil {
			value = v10Error(err)
		} else if len(ajax) == 3 && result.RedirectURL == "" {
			partial := visualforce.NewPartialResponse(result.HTML, result.ViewState, visualforce.ParseRerenderTargets(targets))
			v10ApplyPartial(t, doc, partial.Targets)
			v10Walk(form, func(n *html.Node) {
				if v10Attr(n, "name") == visualforce.ViewStateFormFieldName() {
					v10SetAttr(n, "value", partial.ViewState)
				}
			})
			value = v10Observe(doc, c.ID)
		} else {
			value = observe()
		}
		stages = append(stages, map[string]any{"phase": "postback", "submission": submission, "filled": filled, "value": value})
	}
	return v10DOM(t, map[string]any{"stages": stages}), diagnostics
}

func v10SortDiagnostics(values []v10Diagnostic) {
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

func v10Encode(t *testing.T, value any) string {
	t.Helper()
	var out strings.Builder
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(out.String(), "\n")
}

var v10FixtureVersion = regexp.MustCompile(`FamilyV10(?:Controller\d+|P\d{5})`)
var v10AjaxHook = regexp.MustCompile(`GLADEVF\.submit\(f,'([^']*)','([^']*)'`)
var v10ActionHook = regexp.MustCompile(`\.value='([^']*)'`)
var v10NativeError = regexp.MustCompile(`(?i)V10_(?:ACTION|PAGE_ACTION|SET_[AB])|Invalid (?:integer|decimal|date|id)|value is required|Validation Error|System\.\w+Exception|common\.apex|Error occurred while loading`)

func v10Error(err error) map[string]any {
	// The captured browser observer returns sorted unique trimmed lines matching
	// vf_forms_cases.py's NATIVE pattern. Keep every selected line's exact text,
	// including unsupported-call prefixes, before normalizing fixture names.
	lines := strings.Split(err.Error(), "\n")
	selected := map[string]bool{}
	for _, line := range lines {
		if v10NativeError.MatchString(line) {
			selected[strings.TrimSpace(line)] = true
		}
	}
	if len(selected) != 0 {
		lines = nil
		for line := range selected {
			lines = append(lines, line)
		}
		sort.Strings(lines)
	}
	for i, line := range lines {
		lines[i] = v10FixtureVersion.ReplaceAllString(line, "<owned>")
	}
	return map[string]any{"nativeError": lines}
}

func v10Attr(n *html.Node, key string) string {
	if n != nil {
		for _, a := range n.Attr {
			if a.Key == key {
				return a.Val
			}
		}
	}
	return ""
}
func v10HasAttr(n *html.Node, key string) bool {
	if n != nil {
		for _, a := range n.Attr {
			if a.Key == key {
				return true
			}
		}
	}
	return false
}
func v10SetAttr(n *html.Node, key, value string) {
	for i := range n.Attr {
		if n.Attr[i].Key == key {
			n.Attr[i].Val = value
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: value})
}
func v10Walk(n *html.Node, visit func(*html.Node)) {
	if n == nil {
		return
	}
	visit(n)
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		v10Walk(child, visit)
	}
}
func v10Find(n *html.Node, predicate func(*html.Node) bool) *html.Node {
	if n == nil {
		return nil
	}
	if predicate(n) {
		return n
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := v10Find(child, predicate); found != nil {
			return found
		}
	}
	return nil
}
func v10Ancestor(n *html.Node, predicate func(*html.Node) bool) *html.Node {
	for ; n != nil; n = n.Parent {
		if predicate(n) {
			return n
		}
	}
	return nil
}
func v10Text(n *html.Node) string {
	var out strings.Builder
	v10Walk(n, func(n *html.Node) {
		if n.Type == html.TextNode {
			out.WriteString(n.Data)
		}
	})
	return out.String()
}
func v10Control(n *html.Node) bool {
	return n.Type == html.ElementNode && (n.Data == "textarea" || n.Data == "select" || (n.Data == "input" && v10Attr(n, "type") != "hidden"))
}
func v10ControlType(n *html.Node) string {
	if n.Data == "textarea" {
		return "textarea"
	}
	if n.Data == "select" {
		if v10HasAttr(n, "multiple") {
			return "select-multiple"
		}
		return "select-one"
	}
	if kind := v10Attr(n, "type"); kind != "" {
		return kind
	}
	return "text"
}
func v10ControlValue(n *html.Node) string {
	if n.Data == "textarea" {
		return v10Text(n)
	}
	if n.Data == "select" {
		var first, selected *html.Node
		v10Walk(n, func(o *html.Node) {
			if o.Data == "option" {
				if first == nil {
					first = o
				}
				if v10HasAttr(o, "selected") {
					selected = o
				}
			}
		})
		if selected == nil {
			selected = first
		}
		if selected == nil {
			return ""
		}
		if v10HasAttr(selected, "value") {
			return v10Attr(selected, "value")
		}
		return v10Text(selected)
	}
	if (v10ControlType(n) == "checkbox" || v10ControlType(n) == "radio") && !v10HasAttr(n, "value") {
		return "on"
	}
	return v10Attr(n, "value")
}
func v10Fill(root *html.Node, c v10Case) []string {
	affected := []string{}
	for _, field := range c.FillOrder {
		value := c.Fills[field]
		controls := []*html.Node{}
		v10Walk(root, func(n *html.Node) {
			if v10Attr(n, "data-v10-field") == field {
				v10Walk(n, func(control *html.Node) {
					if v10Control(control) {
						controls = append(controls, control)
					}
				})
			}
		})
		if !c.FillAll && len(controls) > 1 {
			controls = controls[:1]
		}
		for _, control := range controls {
			if v10HasAttr(control, "disabled") {
				continue
			}
			if v10ControlType(control) == "checkbox" {
				if value == true {
					v10SetAttr(control, "checked", "")
				} else {
					attrs := control.Attr[:0]
					for _, a := range control.Attr {
						if a.Key != "checked" {
							attrs = append(attrs, a)
						}
					}
					control.Attr = attrs
				}
			} else if control.Data == "textarea" {
				for control.FirstChild != nil {
					control.RemoveChild(control.FirstChild)
				}
				control.AppendChild(&html.Node{Type: html.TextNode, Data: fmt.Sprint(value)})
			} else {
				v10SetAttr(control, "value", fmt.Sprint(value))
			}
			affected = append(affected, field)
		}
	}
	return affected
}

// Interpret the rendered selectCommand hook's DOM mutation before collecting
// successful controls. The marker is never added directly to the payload.
func v10SelectCommand(t *testing.T, doc, form, button *html.Node, hook string) {
	t.Helper()
	if !strings.Contains(hook, "GLADEVF.selectCommand(this);") {
		return
	}
	script := v10Find(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "script" && strings.Contains(v10Text(n), "GLADEVF.selectCommand=function(e)")
	})
	match := v10CommandFieldHook.FindStringSubmatch(v10Text(script))
	if len(match) != 3 {
		t.Fatal("rendered selectCommand hook has no supported hidden-field mutation")
	}
	field := v10Find(form, func(n *html.Node) bool {
		return n.Type == html.ElementNode && v10Attr(n, "name") == match[1]
	})
	if field == nil {
		field = &html.Node{Type: html.ElementNode, Data: "input", Attr: []html.Attribute{{Key: "type", Val: "hidden"}, {Key: "name", Val: match[1]}}}
		form.AppendChild(field)
	}
	v10SetAttr(field, "value", v10Attr(button, match[2]))
}

// Read the shipped region transport's field list rather than maintaining a
// second list in the adapter. Removing a field from the script removes it from
// this request, even when selectCommand created it elsewhere in the form.
func v10AppendAjaxFormControls(t *testing.T, doc, form *html.Node, values map[string]string) {
	t.Helper()
	script := v10Find(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "script" && strings.Contains(v10Text(n), "function appendFormControlFields(data,form)")
	})
	match := v10AjaxFormFields.FindStringSubmatch(v10Text(script))
	var fields []string
	if len(match) != 2 || json.Unmarshal([]byte(match[1]), &fields) != nil {
		t.Fatal("rendered AJAX transport has no supported form-control field list")
	}
	for _, name := range fields {
		field := v10Find(form, func(n *html.Node) bool {
			return n.Type == html.ElementNode && v10Attr(n, "name") == name
		})
		for key, value := range v10FormValues(field, nil, false) {
			values[key] = value
		}
	}
}

var v10CommandFieldHook = regexp.MustCompile(`c\.name='([^']+)';f\.appendChild\(c\);\}c\.value=e\.getAttribute\('([^']+)'\)`)
var v10AjaxFormFields = regexp.MustCompile(`function appendFormControlFields\(data,form\)\{\s*(\[[^\]]*\])\.forEach\(function\(name\)\{`)

func v10FormValues(root, button *html.Node, normalizeNewlines bool) map[string]string {
	values := map[string]string{}
	v10Walk(root, func(n *html.Node) {
		if n.Type != html.ElementNode || (n.Data != "input" && n.Data != "select" && n.Data != "textarea" && n.Data != "button") {
			return
		}
		name := v10Attr(n, "name")
		if name == "" || v10HasAttr(n, "disabled") {
			return
		}
		kind := v10ControlType(n)
		if (kind == "checkbox" || kind == "radio") && !v10HasAttr(n, "checked") {
			return
		}
		if (kind == "submit" || kind == "button" || kind == "reset") && n != button {
			return
		}
		value := v10ControlValue(n)
		if normalizeNewlines && n.Data == "textarea" {
			value = strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
			value = strings.ReplaceAll(value, "\n", "\r\n")
		}
		values[name] = value
	})
	return values
}
func v10Observe(doc *html.Node, id string) map[string]any {
	result := v10Find(doc, func(n *html.Node) bool { return v10Attr(n, "data-v10-result") == id })
	if result == nil {
		return map[string]any{"localError": "rendered result marker missing"}
	}
	var observed any
	if err := json.Unmarshal([]byte(v10Text(result)), &observed); err != nil {
		return map[string]any{"localError": err.Error(), "text": v10Text(result)}
	}
	root := v10Find(doc, func(n *html.Node) bool { return v10Attr(n, "data-v10-case") == id })
	controls := []any{}
	v10Walk(root, func(n *html.Node) {
		field := v10Attr(n, "data-v10-field")
		if field == "" {
			return
		}
		v10Walk(n, func(control *html.Node) {
			if !v10Control(control) {
				return
			}
			var checked any
			kind := v10ControlType(control)
			if kind == "checkbox" {
				checked = v10HasAttr(control, "checked")
			}
			controls = append(controls, map[string]any{"field": field, "tag": control.Data, "type": kind, "value": v10ControlValue(control), "checked": checked, "disabled": v10HasAttr(control, "disabled")})
		})
	})
	messages := []string{}
	v10Walk(doc, func(n *html.Node) {
		for _, class := range strings.Fields(v10Attr(n, "class")) {
			if class == "messageText" || class == "errorMsg" || class == "errorMessage" {
				if text := strings.TrimSpace(v10Text(n)); text != "" {
					messages = append(messages, text)
				}
				break
			}
		}
	})
	value := map[string]any{"result": observed, "controls": controls, "messages": messages}
	if v10HasAttr(root, "data-v10-repeat-bindings") {
		commands := []any{}
		v10Walk(root, func(n *html.Node) {
			if v10Attr(n, "data-v10-submit") != "owned" {
				return
			}
			var child *html.Node
			for e := n.FirstChild; e != nil; e = e.NextSibling {
				if e.Type == html.ElementNode {
					child = e
					break
				}
			}
			row := v10Ancestor(n, func(e *html.Node) bool { return v10HasAttr(e, "data-v10-repeat-row") })
			var rowKey, tag, kind, disabled, href, aria any
			if row != nil {
				rowKey = v10Attr(row, "data-v10-repeat-row")
			}
			if child != nil {
				tag = child.Data
				if v10HasAttr(child, "type") {
					kind = v10Attr(child, "type")
				}
				disabled, href = v10HasAttr(child, "disabled"), v10HasAttr(child, "href")
				if v10HasAttr(child, "aria-disabled") {
					aria = v10Attr(child, "aria-disabled")
				}
			}
			commands = append(commands, map[string]any{"row": rowKey, "tag": tag, "type": kind, "disabled": disabled, "href": href, "ariaDisabled": aria})
		})
		value["commands"] = commands
	}
	if v10HasAttr(root, "data-v10-component-ids") {
		attribute := func(n *html.Node, key string) any {
			if v10HasAttr(n, key) {
				return v10Attr(n, key)
			}
			return nil
		}
		rowKey := func(n *html.Node) any {
			return attribute(v10Ancestor(n, func(e *html.Node) bool {
				return v10HasAttr(e, "data-v10-repeat-row")
			}), "data-v10-repeat-row")
		}
		forms, fields, commands := []any{}, []any{}, []any{}
		v10Walk(root, func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "form" {
				forms = append(forms, map[string]any{"id": attribute(n, "id"), "name": attribute(n, "name")})
			}
			if field := v10Attr(n, "data-v10-field"); field != "" {
				v10Walk(n, func(e *html.Node) {
					if v10Control(e) {
						fields = append(fields, map[string]any{"field": field, "row": rowKey(e), "id": attribute(e, "id"), "name": attribute(e, "name")})
					}
				})
			}
			if v10Attr(n, "data-v10-submit") == "owned" {
				e := v10Find(n, func(e *html.Node) bool {
					return e.Type == html.ElementNode && (e.Data == "input" || e.Data == "a")
				})
				commands = append(commands, map[string]any{"row": rowKey(n), "id": attribute(e, "id"), "name": attribute(e, "name")})
			}
		})
		value["componentIds"] = map[string]any{"forms": forms, "fields": fields, "commands": commands}
	}
	return value
}
func v10ApplyPartial(t *testing.T, doc *html.Node, targets map[string]string) {
	t.Helper()
	ids := make([]string, 0, len(targets))
	for id := range targets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		current := v10Find(doc, func(n *html.Node) bool { return v10Attr(n, "id") == id })
		if current == nil || current.Parent == nil {
			continue
		}
		nodes, err := html.ParseFragment(strings.NewReader(targets[id]), current.Parent)
		if err != nil {
			t.Fatal(err)
		}
		parent := current.Parent
		for _, node := range nodes {
			parent.InsertBefore(node, current)
		}
		parent.RemoveChild(current)
	}
}

// The captured TSV uses Python ensure_ascii=True for its outer JSON framing.
// Escape framing identically while preserving the decoded observed text.
func v10DOM(t *testing.T, value any) string {
	t.Helper()
	encoded := v10FixtureVersion.ReplaceAllString(v10Encode(t, value), "<owned>")
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
