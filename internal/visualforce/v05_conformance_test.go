package visualforce_test

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
	"golang.org/x/net/html"
)

// TestV05SalesforceConformance compares 125 compile/metadata and 106 observable
// DOM rows at API 59/67, plus 42 compile/37 DOM mutation controls,
// 8 compile/7 DOM default-ordering controls, 8 compile/7 DOM clone controls,
// 8 compile/7 DOM native-page controls, and 10 compile/9 DOM bound-page controls.
// Removed callback-state rows retain their raw hosted errors and reasons in
// testdata; they are not matches. CI reads exported sources and observations.
// Standard-controller helpers supply the compiler adapter, principal fixture
// and DOM controls. Runtime rows use the product request runner and fresh GET/postback
// VMs, submitting only rendered controls and authenticated product view state.
// Equality retains every observed string and raw null. Only outer JSON key
// order/spacing and the native observer's ID/component-name projection change.
// GLADE_V05_CAPTURE=1 reports all differences without failing on mismatches;
// GLADE_V05_REPORT, GLADE_V05_CONTROLS_REPORT, GLADE_V05_DEFAULT_REPORT,
// GLADE_V05_CLONE_REPORT, GLADE_V05_NATIVE_PAGE_REPORT and
// GLADE_V05_BOUND_PAGE_REPORT select per-row TSV outputs.
func TestV05SalesforceConformance(t *testing.T) {
	v05RunConformance(t, "testdata/v05_salesforce.json", "Owned standard-set controller observations at API 59.0 and 67.0.", "Standard set controller", 125, 106, 3, []string{"paging", "filters", "selection", "construction", "save"}, "GLADE_V05_REPORT")
	t.Run("controls", func(t *testing.T) {
		v05RunConformance(t, "testdata/v05_controls_salesforce.json", "Owned standard-set controller mutation controls at API 59.0 and 67.0.", "Standard set controller controls", 42, 37, 4, []string{"paging", "filters", "construction"}, "GLADE_V05_CONTROLS_REPORT")
	})
	t.Run("defaults", func(t *testing.T) {
		v05RunConformance(t, "testdata/v05_default_salesforce.json", "Owned standard-set controller default-ordering controls at API 59.0 and 67.0.", "Standard set controller defaults", 8, 7, 5, []string{"paging", "filters"}, "GLADE_V05_DEFAULT_REPORT")
	})
	t.Run("clones", func(t *testing.T) {
		v05RunConformance(t, "testdata/v05_clone_salesforce.json", "Owned standard-set controller clone controls at API 59.0 and 67.0.", "Standard set controller clones", 8, 7, 4, []string{"paging", "construction", "filters"}, "GLADE_V05_CLONE_REPORT")
	})
	t.Run("native_page", func(t *testing.T) {
		v05RunConformance(t, "testdata/v05_native_page_salesforce.json", "Owned standard-set controller native-page controls at API 59.0 and 67.0.", "Standard set controller native page", 8, 7, 4, []string{"paging", "filters"}, "GLADE_V05_NATIVE_PAGE_REPORT")
	})
	t.Run("bound_page", func(t *testing.T) {
		v05RunConformance(t, "testdata/v05_bound_page_salesforce.json", "Owned standard-set controller bound-page controls at API 59.0 and 67.0.", "Standard set controller bound page", 10, 9, 5, []string{"paging", "filters"}, "GLADE_V05_BOUND_PAGE_REPORT")
	})
}

type v05Case struct {
	v04Case
	FatalMessage       string            `json:"fatal_message"`
	FatalMessageSource string            `json:"fatal_message_source"`
	MessageOnly        bool              `json:"message_only"`
	NativeExpected     map[string]string `json:"native_expected"`
	UnobservableReason string            `json:"unobservable_reason"`
}

type v05RemovedCase struct {
	ID             string            `json:"id"`
	Reason         string            `json:"reason"`
	NativeExpected map[string]string `json:"native_expected"`
}

func v05RunConformance(t *testing.T, fixture, source, label string, compileRows, runtimeRows, metadataFiles int, requiredGroups []string, reportVariable string) {
	capture := os.Getenv("GLADE_V05_CAPTURE") != ""
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Source                string                       `json:"source"`
		Cases                 []v05Case                    `json:"cases"`
		RuntimeFiles          map[string]map[string]string `json:"runtime_files"`
		RemovedCases          []v05RemovedCase             `json:"removed_cases"`
		ListViewCreationOrder []string                     `json:"list_view_creation_order"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if table.Source != source || len(table.Cases) != compileRows+runtimeRows {
		t.Fatalf("%s requires %d captured cases, got %d at %s", label, compileRows+runtimeRows, len(table.Cases), table.Source)
	}
	versions := []string{"59.0", "67.0"}
	for _, removed := range table.RemovedCases {
		if removed.ID == "" || removed.Reason == "" || removed.NativeExpected["59.0"] == "" || removed.NativeExpected["67.0"] == "" {
			t.Fatal("removed native row requires both raw observations and a reason")
		}
		t.Logf("%s removed %s: %s", label, removed.ID, removed.Reason)
	}
	counts, seen, groups := map[string]int{}, map[string]bool{}, map[string]bool{}
	for _, api := range versions {
		if len(table.RuntimeFiles[api]) != metadataFiles {
			t.Fatalf("Standard set controller requires captured list-view, tab and permission metadata at API %s", api)
		}
		ordered := map[string]bool{}
		for _, path := range table.ListViewCreationOrder {
			if ordered[path] || !strings.HasSuffix(path, ".listView-meta.xml") || table.RuntimeFiles[api][path] == "" {
				t.Fatalf("invalid captured list-view creation order at API %s: %s", api, path)
			}
			ordered[path] = true
		}
		for path := range table.RuntimeFiles[api] {
			if strings.HasSuffix(path, ".listView-meta.xml") && !ordered[path] {
				t.Fatalf("missing captured list-view creation order at API %s: %s", api, path)
			}
		}
	}
	for _, c := range table.Cases {
		if c.UnobservableReason != "" {
			if c.Kind != "runtime" || c.FatalMessage == "" || c.NativeExpected["59.0"] == "" || c.NativeExpected["67.0"] == "" {
				t.Fatal("partial native observation requires both raw responses and an asserted fatal message")
			}
			t.Logf("%s %s: asserting observable native error fields; unobservable: %s", label, c.ID, c.UnobservableReason)
		}
		if c.ID == "" || seen[c.ID] || (c.Kind != "compile" && c.Kind != "runtime") {
			t.Fatalf("invalid or duplicate Standard set controller case %q (%s)", c.ID, c.Kind)
		}
		seen[c.ID], groups[c.Group] = true, true
		counts[c.Kind]++
		for _, api := range versions {
			input := c.Inputs[api]
			if input.Name == "" || len(input.Files) < 5 || input.Files["sfdx-project.json"] == "" ||
				input.Files["force-app/main/default/pages/"+input.Name+".page"] == "" ||
				input.Files["force-app/main/default/pages/"+input.Name+".page-meta.xml"] == "" {
				t.Fatalf("%s: incomplete captured input at API %s", c.ID, api)
			}
			answer, exists := c.Expected[api]
			diagnostics, haveDiagnostics := c.Diagnostics[api]
			if !exists || !haveDiagnostics {
				t.Fatalf("%s: missing native answer/diagnostics at API %s", c.ID, api)
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
				if !strings.HasPrefix(answer, "DOM|") || json.Unmarshal([]byte(answer[4:]), &native) != nil || len(diagnostics) != 0 ||
					(native["result"] == nil && native["native_error"] == nil && native["no_data"] == nil && (!c.MessageOnly || native["fatal_message"] == nil)) {
					t.Fatalf("%s: missing or invalid native DOM observation at API %s", c.ID, api)
				}
			}
		}
	}
	if counts["compile"] != compileRows || counts["runtime"] != runtimeRows || len(groups) != len(requiredGroups) {
		t.Fatalf("%s requires %d compile and %d DOM rows, got %v, %v", label, compileRows, runtimeRows, counts, groups)
	}
	for _, group := range requiredGroups {
		if !groups[group] {
			t.Fatalf("%s: missing captured group %s", label, group)
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
	writeRow("api", "id", "status", "actual", "expected", "group", "resource", "diagnostic", "owner", "reason", "acceptance_status", "diagnostic_status", "expected_diagnostic")
	totals := map[string]int{}
	totalRows := map[string]int{}
	for _, api := range versions {
		matches := map[string]int{}
		rows := map[string]int{}
		for _, c := range table.Cases {
			t.Run(api+"/"+c.ID, func(t *testing.T) {
				rows["all"]++
				rows[c.Kind]++
				root := t.TempDir()
				input := c.Inputs[api]
				if c.Kind == "runtime" {
					v05WriteFiles(t, root, table.RuntimeFiles[api])
				}
				v05WriteFiles(t, root, input.Files)
				got, diagnostics := v05Execute(t, root, input.Name, c, table.ListViewCreationOrder)
				want := c.Expected[api]
				if c.Kind == "runtime" {
					// The native observer already parsed result JSON. Canonicalize only
					// its outer transport, retaining every value, string and raw null.
					var native any
					if err := json.Unmarshal([]byte(want[4:]), &native); err != nil {
						t.Fatal(err)
					}
					want = v05FrameDOM(t, native)
				}
				actualDiagnostic, expectedDiagnostic := v04Encode(t, diagnostics), v04Encode(t, c.Diagnostics[api])
				acceptanceMatch := got == want
				diagnosticMatch := c.Kind != "compile" || actualDiagnostic == expectedDiagnostic
				status, acceptanceStatus, diagnosticStatus := "MISMATCH", "MISMATCH", "NOT_APPLICABLE"
				if acceptanceMatch {
					acceptanceStatus = "MATCH"
					if c.Kind == "compile" {
						matches["acceptance"]++
					}
				}
				if c.Kind == "compile" {
					diagnosticStatus = "MISMATCH"
					if diagnosticMatch {
						diagnosticStatus = "MATCH"
						matches["diagnostic"]++
					}
				}
				if acceptanceMatch && diagnosticMatch {
					status = "MATCH"
					matches["all"]++
					matches[c.Kind]++
				}
				// Temporary paths change only in the report, after exact comparison.
				// Every remaining row is compared exactly; removals are logged above.
				writeRow(api, c.ID, status, got, want, c.Group, "ApexPage", strings.ReplaceAll(actualDiagnostic, root, "<fixture>"), "", "", acceptanceStatus, diagnosticStatus, expectedDiagnostic)
				if !capture && (!acceptanceMatch || !diagnosticMatch) {
					t.Errorf("API %s %s: actual <%s> expected <%s>; diagnostics <%s> expected <%s>", api, c.ID, got, want, actualDiagnostic, expectedDiagnostic)
				}
			})
		}
		for _, summary := range []struct {
			name string
			key  string
			rows int
		}{
			{"API_TOTAL", "all", rows["all"]}, {"COMPILE_API_TOTAL", "compile", rows["compile"]},
			{"COMPILE_ACCEPTANCE_API_TOTAL", "acceptance", rows["compile"]}, {"DIAGNOSTIC_API_TOTAL", "diagnostic", rows["compile"]},
			{"DOM_API_TOTAL", "runtime", rows["runtime"]},
		} {
			writeRow(summary.name, api, fmt.Sprintf("%d/%d", matches[summary.key], summary.rows))
			totals[summary.key] += matches[summary.key]
			totalRows[summary.key] += summary.rows
		}
		t.Logf("%s API %s exact %d/%d; compile %d/%d, acceptance %d/%d, diagnostics %d/%d; DOM %d/%d", label, api, matches["all"], rows["all"], matches["compile"], rows["compile"], matches["acceptance"], rows["compile"], matches["diagnostic"], rows["compile"], matches["runtime"], rows["runtime"])
	}
	writeRow("COMPILE_TOTAL", fmt.Sprintf("%d/%d", totals["compile"], totalRows["compile"]))
	writeRow("COMPILE_ACCEPTANCE_TOTAL", fmt.Sprintf("%d/%d", totals["acceptance"], totalRows["acceptance"]))
	writeRow("DIAGNOSTIC_TOTAL", fmt.Sprintf("%d/%d", totals["diagnostic"], totalRows["diagnostic"]))
	writeRow("DOM_TOTAL", fmt.Sprintf("%d/%d", totals["runtime"], totalRows["runtime"]))
	writeRow("TOTAL", fmt.Sprintf("%d/%d", totals["all"], totalRows["all"]))
	t.Logf("%s exact matches %d/%d; compile %d/%d; DOM %d/%d", label, totals["all"], totalRows["all"], totals["compile"], totalRows["compile"], totals["runtime"], totalRows["runtime"])
	tsv.Flush()
	if err := tsv.Error(); err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv(reportVariable); path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(report.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func v05WriteFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for path, source := range files {
		if filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "..") {
			t.Fatalf("invalid Standard set controller fixture path %q", path)
		}
		fullPath := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func v05Execute(t *testing.T, root, name string, c v05Case, creationOrder []string) (string, []v04Diagnostic) {
	t.Helper()
	if c.Kind == "compile" {
		return v04Execute(t, root, name, c.v04Case)
	}
	diagnostics := []v04Diagnostic{}
	fail := func(err error) (string, []v04Diagnostic) {
		return v05DOM(t, map[string]any{"stage": "initial", "local_error": err.Error()}), diagnostics
	}
	p, err := project.Load(root)
	if err != nil {
		return fail(err)
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		return fail(err)
	}
	index := typesys.Build(p, sch)
	vf, err := visualforce.LoadProjectForRender(p)
	if err != nil {
		return fail(err)
	}
	return v05Render(t, p, vf, index, name, c, creationOrder), diagnostics
}

func v05Org(t *testing.T, p project.Project, creationOrder []string) (storage.OrgState, storage.Record) {
	t.Helper()
	org, user := v04Org()
	// The native setup requires an empty V05_ Account prefix. Every action
	// creates its owned rows and rolls them back; no Standard controller seed enters this table.
	accounts := org.Objects["Account"]
	accounts.Records = map[storage.ID]storage.Record{}
	org.Objects["Account"] = accounts
	paths := map[string]string{}
	for _, path := range p.ListViewFiles {
		relative, err := filepath.Rel(p.Root, path)
		if err != nil {
			t.Fatal(err)
		}
		paths[filepath.ToSlash(relative)] = path
	}
	if len(paths) != len(creationOrder) {
		t.Fatal("captured list-view creation order does not cover the project")
	}
	// Replay the native fixture's metadata creation sequence with the product
	// loader. Generated 18-character identities and postback stability are part
	// of the conformance check; no expected identity is installed by the test.
	machine := vm.New(nil)
	machine.SetOrg(&org)
	for _, path := range creationOrder {
		fullPath, exists := paths[path]
		if !exists {
			t.Fatalf("captured list-view source missing: %s", path)
		}
		if err := machine.LoadStandardSetListViews([]string{fullPath}); err != nil {
			t.Fatal(err)
		}
	}
	return org, user
}

func v05Render(t *testing.T, p project.Project, vf visualforce.Index, index typesys.Index, name string, c v05Case, creationOrder []string) string {
	id := c.ID
	t.Helper()
	org, user := v05Org(t, p, creationOrder)
	request := visualforce.PageRenderRequest{Project: p, VFIndex: vf, Org: &org, PageName: name,
		PageURL: "/apex/" + name, ViewStateSecret: []byte("Standard set controller owned conformance view-state secret")}
	render := func() (visualforce.PageRenderResult, error) {
		machine := vm.New(nil)
		machine.SetOrg(&org)
		machine.SetCurrentUser(user)
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
	fail := func(stage string, err error) string {
		return v05DOM(t, map[string]any{"stage": stage, "local_error": err.Error()})
	}
	result, err := render()
	if err != nil {
		return fail("initial", err)
	}
	if result.RedirectURL != "" {
		return v05DOM(t, map[string]any{"redirectURL": result.RedirectURL, "redirect": result.Redirect})
	}
	doc, err := html.Parse(strings.NewReader(result.HTML))
	if err != nil {
		return fail("initial", err)
	}
	marker := v04Find(doc, func(n *html.Node) bool { return v04Attr(n, "data-v05-case") == id })
	if marker == nil || strings.TrimSpace(v04Text(marker)) != "PENDING" {
		return v05DOM(t, map[string]any{"stage": "initial", "local_error": "rendered PENDING result marker missing", "text": v04Text(marker)})
	}
	button := v04Find(doc, func(n *html.Node) bool { return n.Data == "input" && v04Attr(n, "value") == "V05_RUN" })
	var form *html.Node
	for parent := button; parent != nil; parent = parent.Parent {
		if parent.Data == "form" {
			form = parent
			break
		}
	}
	if button == nil || form == nil {
		return v05DOM(t, map[string]any{"local_error": "rendered submit control/form missing"})
	}
	values := map[string]string{}
	v04Walk(form, func(n *html.Node) {
		if n.Type != html.ElementNode || (n.Data != "input" && n.Data != "textarea" && n.Data != "select") || v04HasAttr(n, "disabled") {
			return
		}
		kind := v04Attr(n, "type")
		if ((kind == "checkbox" || kind == "radio") && !v04HasAttr(n, "checked")) || ((kind == "submit" || kind == "button" || kind == "reset") && n != button) {
			return
		}
		if name := v04Attr(n, "name"); name != "" {
			value := v04ControlValue(n)
			if n.Data == "textarea" {
				value = strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
				value = strings.ReplaceAll(value, "\n", "\r\n")
			}
			values[name] = value
		}
	})
	action := v04Attr(button, "data-action")
	if action == "" {
		if match := v04ActionHook.FindStringSubmatch(v04Attr(button, "onclick")); len(match) == 2 {
			action = match[1]
		}
	}
	values[visualforce.ViewStateActionFieldName()] = action
	payload, err := visualforce.DecodeViewState(values[visualforce.ViewStateFormFieldName()], request.ViewStateSecret)
	if err == nil {
		err = visualforce.VerifyViewStateCSRF(payload, values["__vf_csrf"])
	}
	if err != nil {
		return fail("postback", err)
	}
	parsed := visualforce.ParseAjaxPayload(values)
	request.ViewState, request.FormValues, request.Action = &payload, parsed.SubmittedFields, parsed.Action
	request.PageURL = v04Attr(form, "action")
	result, err = render()
	if err != nil {
		if c.MessageOnly {
			// These hosted responses expose only the message. Preserve exact
			// text, using the same ID projection as the native observer.
			var cause *vm.UIActionError
			if c.FatalMessageSource == "" || !errors.As(err, &cause) {
				return fail("postback", err)
			}
			return v05DOM(t, map[string]any{"stage": "postback", "fatal_message": cause.Message})
		}
		var diagnostic interface{ ActionDiagnostic() string }
		if errors.As(err, &diagnostic) && diagnostic.ActionDiagnostic() != "" {
			if c.FatalMessage != "" {
				var cause *vm.UIActionError
				if c.FatalMessageSource == "" || !errors.As(err, &cause) || cause.Message != c.FatalMessage {
					return fail("postback", fmt.Errorf("fatal action message: %w; expected <%s> from %s", err, c.FatalMessage, c.FatalMessageSource))
				}
			}
			return v05DOM(t, map[string]any{"stage": "postback", "native_error": []string{diagnostic.ActionDiagnostic()}})
		}
		return fail("postback", err)
	}
	if result.RedirectURL != "" {
		return v05DOM(t, map[string]any{"redirectURL": result.RedirectURL, "redirect": result.Redirect})
	}
	doc, err = html.Parse(strings.NewReader(result.HTML))
	if err != nil {
		return fail("postback", err)
	}
	marker = v04Find(doc, func(n *html.Node) bool { return v04Attr(n, "data-v05-case") == id })
	if marker == nil {
		return v05DOM(t, map[string]any{"local_error": "rendered result marker missing"})
	}
	text := strings.TrimSpace(v04Text(marker))
	var observed any
	if err := json.Unmarshal([]byte(text), &observed); err != nil {
		return v05DOM(t, map[string]any{"local_error": "rendered result JSON invalid", "text": text})
	}
	return v05DOM(t, map[string]any{"result": observed})
}

var v05FixtureName = regexp.MustCompile(`FamilyV05(?:[PC]\d{5}|(?:Tab|Access)\d{2})`)
var v05ID = regexp.MustCompile(`(?:001|003|005|00B|00D)[A-Za-z0-9]{12}(?:[A-Za-z0-9]{3})?`)
var v05ErrorID = regexp.MustCompile(`Error ID: [A-Za-z0-9()_+-]+`)

func v05DOM(t *testing.T, value any) string {
	t.Helper()
	text := v05FrameDOM(t, value)
	text = v04URL.ReplaceAllString(text, "<URL>")
	text = v05FixtureName.ReplaceAllString(text, "<owned>")
	text = v05ErrorID.ReplaceAllString(text, "Error ID: <volatile>")
	// Match the native observer's ASCII word boundaries; embedded strings
	// that merely contain an ID-shaped substring retain their complete text.
	word := func(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' }
	var out strings.Builder
	start := 0
	for _, span := range v05ID.FindAllStringIndex(text, -1) {
		if (span[0] > 0 && word(text[span[0]-1])) || (span[1] < len(text) && word(text[span[1]])) {
			continue
		}
		out.WriteString(text[start:span[0]])
		out.WriteString("<ID:" + text[span[0]:span[0]+3] + ">")
		start = span[1]
	}
	out.WriteString(text[start:])
	return out.String()
}

func v05FrameDOM(t *testing.T, value any) string {
	t.Helper()
	encoded := v04Encode(t, value)
	var out strings.Builder
	out.WriteString("DOM|")
	// Mirror Python ensure_ascii=True framing without changing decoded text.
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
