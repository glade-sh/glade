package visualforce_test

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
	"golang.org/x/net/html"
)

type v04Input struct {
	Name  string            `json:"name"`
	Files map[string]string `json:"files"`
}

type v04Diagnostic struct {
	Resource string `json:"resource"`
	Type     string `json:"type"`
	Message  string `json:"message"`
}

type v04Carry struct {
	Owner  string `json:"owner"`
	Reason string `json:"reason"`
	Scope  string `json:"scope,omitempty"`
}

type v04Case struct {
	ID          string                     `json:"id"`
	Group       string                     `json:"group"`
	Kind        string                     `json:"kind"`
	Inputs      map[string]v04Input        `json:"inputs"`
	Expected    map[string]string          `json:"expected"`
	Diagnostics map[string][]v04Diagnostic `json:"diagnostics"`
	Carries     map[string]v04Carry        `json:"carries,omitempty"`
	HTTPStatus  map[string]int             `json:"http_status,omitempty"`
}

// TestV04SalesforceConformance compares all 130 compile/metadata and 100 DOM
// rows at API 59/67, exported from the final native oracle.
// Captured sources run through the shared project compiler and real HTTP handler.
// GET and postback use fresh VMs, with state carried only by product view state
// and rendered form controls, including the server's CSRF verification. The DOM
// observer mirrors the native parsed result and script-free bound text/inputs.
// Equality retains every observed value and raw null; only JSON transport key
// order/spacing and the oracle's ID/token/fixture-name projection are normalized.
// CI reads only owned testdata without credentials or a browser.
// GLADE_V04_CAPTURE=1 records all differences without failing on row mismatches;
// GLADE_V04_REPORT selects the optional per-row TSV output path.
func TestV04SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_V04_CAPTURE") != ""
	data, err := os.ReadFile("testdata/v04_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Source string    `json:"source"`
		Cases  []v04Case `json:"cases"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if table.Source != "Owned standard-controller observations at API 59.0 and 67.0." || len(table.Cases) != 230 {
		t.Fatalf("Standard controller requires all 230 cases from the final native oracle, got %d at %s", len(table.Cases), table.Source)
	}
	versions := []string{"59.0", "67.0"}
	counts, seen, groups := map[string]int{}, map[string]bool{}, map[string]bool{}
	for _, c := range table.Cases {
		if c.ID == "" || seen[c.ID] || (c.Kind != "compile" && c.Kind != "runtime") {
			t.Fatalf("invalid or duplicate Standard controller case %q (%s)", c.ID, c.Kind)
		}
		seen[c.ID] = true
		groups[c.Group] = true
		counts[c.Kind]++
		for _, api := range versions {
			input := c.Inputs[api]
			if input.Name == "" || len(input.Files) < 5 || input.Files["sfdx-project.json"] == "" ||
				input.Files["force-app/main/default/pages/"+input.Name+".page"] == "" ||
				input.Files["force-app/main/default/pages/"+input.Name+".page-meta.xml"] == "" {
				t.Fatalf("%s: incomplete captured input at API %s", c.ID, api)
			}
			if carry, exists := c.Carries[api]; exists {
				if c.Kind != "runtime" || strings.TrimSpace(carry.Owner) == "" || strings.TrimSpace(carry.Reason) == "" || strings.HasPrefix(carry.Owner, "Standard controller") {
					t.Fatalf("%s: invalid carry at API %s: %+v", c.ID, api, carry)
				}
				if carry.Scope != "" && (carry.Scope != "confirmation-token" || carry.Owner != "Hosted navigation" || !strings.HasPrefix(c.ID, "r_action_delete_")) {
					t.Fatalf("%s: invalid carry scope at API %s: %+v", c.ID, api, carry)
				}
			}
			if status, boundary := c.HTTPStatus[api]; boundary && (c.ID != "r_add_ctor_null_member" || status != http.StatusInternalServerError) {
				t.Fatalf("%s: unsupported native HTTP boundary at API %s: %d", c.ID, api, status)
			}
			answer, exists := c.Expected[api]
			if !exists {
				t.Fatalf("%s: missing native answer at API %s", c.ID, api)
			}
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
				if len(native) != 2 || native["result"] == nil || native["binding"] == nil {
					t.Fatalf("%s: incomplete native result/binding at API %s", c.ID, api)
				}
			}
		}
	}
	if counts["compile"] != 130 || counts["runtime"] != 100 || len(groups) != 4 ||
		!groups["projection"] || !groups["addFields/reset"] || !groups["actions"] || !groups["visibility"] {
		t.Fatalf("Standard controller requires 130 compile/metadata and 100 DOM rows in all four groups, got %v, %v", counts, groups)
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
			got, diagnostics := v04Execute(t, root, input.Name, c)
			want := c.Expected[api]
			actualDiagnostic := v04Encode(t, diagnostics)
			expectedDiagnostic := v04Encode(t, c.Diagnostics[api])
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
			if status != "MATCH" {
				if carry, exists := c.Carries[api]; exists {
					owner, reason = carry.Owner, carry.Reason
					if carry.Scope == "confirmation-token" && v04ConfirmationContract(got) != v04ConfirmationContract(want) && !capture {
						t.Errorf("API %s %s: delete contract differs beyond the carried token; actual <%s> expected <%s>", api, c.ID, got, want)
					}
					t.Logf("Standard controller API %s carry %s owner=%s: %s; actual <%s> expected <%s>", api, c.ID, owner, reason, got, want)
				}
			}
			// Every row is compared exactly. Matching rows retain the normal
			// assertion even when an earlier export assigned them a carry.
			writeRow(api, c.ID, status, got, want, c.Group, "ApexPage", strings.ReplaceAll(actualDiagnostic, root, "<fixture>"), owner, reason, acceptanceStatus, diagnosticStatus, expectedDiagnostic)
			if !capture && owner == "" && (!acceptanceMatch || !diagnosticMatch) {
				t.Errorf("API %s %s: actual <%s> expected <%s>; diagnostics <%s> expected <%s>", api, c.ID, got, want, actualDiagnostic, expectedDiagnostic)
			}
		}
		writeRow("API_TOTAL", api, fmt.Sprintf("%d/230", apiMatches))
		writeRow("COMPILE_API_TOTAL", api, fmt.Sprintf("%d/130", apiCompileMatches))
		writeRow("COMPILE_ACCEPTANCE_API_TOTAL", api, fmt.Sprintf("%d/130", apiAcceptanceMatches))
		writeRow("DIAGNOSTIC_API_TOTAL", api, fmt.Sprintf("%d/130", apiDiagnosticMatches))
		writeRow("DOM_API_TOTAL", api, fmt.Sprintf("%d/100", apiDOMMatches))
		t.Logf("Standard controller API %s exact %d/230; compile %d/130, acceptance %d/130, diagnostics %d/130; DOM %d/100", api, apiMatches, apiCompileMatches, apiAcceptanceMatches, apiDiagnosticMatches, apiDOMMatches)
		matches += apiMatches
		compileMatches += apiCompileMatches
		acceptanceMatches += apiAcceptanceMatches
		diagnosticMatches += apiDiagnosticMatches
		domMatches += apiDOMMatches
	}
	writeRow("COMPILE_TOTAL", fmt.Sprintf("%d/260", compileMatches))
	writeRow("COMPILE_ACCEPTANCE_TOTAL", fmt.Sprintf("%d/260", acceptanceMatches))
	writeRow("DIAGNOSTIC_TOTAL", fmt.Sprintf("%d/260", diagnosticMatches))
	writeRow("DOM_TOTAL", fmt.Sprintf("%d/200", domMatches))
	writeRow("TOTAL", fmt.Sprintf("%d/460", matches))
	t.Logf("Standard controller exact matches %d/460; compile %d/260; DOM %d/200", matches, compileMatches, domMatches)
	tsv.Flush()
	if err := tsv.Error(); err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("GLADE_V04_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func v04Execute(t *testing.T, root, name string, c v04Case) (string, []v04Diagnostic) {
	t.Helper()
	diagnostics := []v04Diagnostic{}
	fail := func(err error, resource string) (string, []v04Diagnostic) {
		diagnostics = append(diagnostics, v04Diagnostic{Resource: resource, Type: "Error", Message: err.Error()})
		if c.Kind == "runtime" {
			return v04ObserveError(t, err), diagnostics
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
				diagnostics = append(diagnostics, v04Diagnostic{Resource: "ApexClass", Type: "Error", Message: d.Message})
			}
		}
	}
	loader := visualforce.LoadProject
	if c.Kind == "runtime" {
		loader = visualforce.LoadProjectForRender
	}
	vf, err := loader(p)
	if err != nil {
		diagnostics = append(diagnostics, v04Diagnostic{Resource: "ApexPage", Type: "Error", Message: err.Error()})
	}
	v04SortDiagnostics(diagnostics)
	if c.Kind == "compile" {
		if len(diagnostics) != 0 {
			return "COMPILE_ERROR", diagnostics
		}
		return "COMPILE_OK", diagnostics
	}
	if err != nil {
		return v04ObserveError(t, err), diagnostics
	}
	return v04Render(t, p, vf, index, name, c), diagnostics
}

func v04SortDiagnostics(values []v04Diagnostic) {
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

// The capture used the same administrator-owned Account and current principal
// for every row. Local IDs are synthetic; no org values or credentials are used.
func v04Org() (storage.OrgState, storage.Record) {
	org := storage.NewOrgState()
	for _, object := range []string{"Account", "Contact", "User", "Profile", "ObjectPermissions", "FieldPermissions"} {
		storage.EnsureStandardObject(&org, object)
	}
	const userID storage.ID = "005000000000004AAA"
	const profileID storage.ID = "00e000000000004AAA"
	user := storage.Record{ID: userID, Object: "User", Fields: map[string]storage.Value{
		"Name": storage.StringValue("Standard controller Current Principal"), "Username": storage.StringValue("v04@example.test"),
		"UserType": storage.StringValue("Standard"), "ProfileId": storage.IDValue(profileID),
	}}
	users := org.Objects["User"]
	users.Records[userID] = user
	org.Objects["User"] = users
	profiles := org.Objects["Profile"]
	profiles.Records[profileID] = storage.Record{ID: profileID, Object: "Profile", Fields: map[string]storage.Value{
		"Name": storage.StringValue("System Administrator"), "PermissionsViewAllData": storage.BooleanValue(true),
		"PermissionsModifyAllData": storage.BooleanValue(true),
	}}
	org.Objects["Profile"] = profiles
	account := org.Objects["Account"]
	const seedID storage.ID = "001000000000004AAA"
	account.Records[seedID] = storage.Record{ID: seedID, Object: "Account", System: storage.SystemFields{OwnerID: userID}, Fields: map[string]storage.Value{
		"Name": storage.StringValue("V04_OWNED"), "Phone": storage.StringValue("4155550104"),
		"Description": storage.StringValue("V04_DESCRIPTION"), "NumberOfEmployees": storage.IntegerValue(7),
		"AnnualRevenue": storage.DecimalValue("12.50"), "BillingCity": storage.NullValue(), "OwnerId": storage.IDValue(userID),
	}}
	org.Objects["Account"] = account
	objects, fields := org.Objects["ObjectPermissions"], org.Objects["FieldPermissions"]
	for i, object := range []string{"Account", "Contact", "User"} {
		id := storage.ID(fmt.Sprintf("11000000000%04d", i+1))
		objects.Records[id] = storage.Record{ID: id, Object: "ObjectPermissions", Fields: map[string]storage.Value{
			"ParentId": storage.IDValue(profileID), "SObjectType": storage.StringValue(object),
			"PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true),
			"PermissionsEdit": storage.BooleanValue(true), "PermissionsDelete": storage.BooleanValue(true),
		}}
		// This is the capture principal's positive access profile, independent
		// of expected row answers. Denied-access oracles are outside this table.
		names := make([]string, 0, len(org.Objects[object].Definition.Fields))
		for field := range org.Objects[object].Definition.Fields {
			names = append(names, field)
		}
		sort.Strings(names)
		for j, field := range names {
			id := storage.ID(fmt.Sprintf("120000000%02d%04d", i+1, j+1))
			fields.Records[id] = storage.Record{ID: id, Object: "FieldPermissions", Fields: map[string]storage.Value{
				"ParentId": storage.IDValue(profileID), "SObjectType": storage.StringValue(object),
				"Field": storage.StringValue(object + "." + field), "PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true),
			}}
		}
	}
	org.Objects["ObjectPermissions"], org.Objects["FieldPermissions"] = objects, fields
	return org, user
}

func v04Render(t *testing.T, p project.Project, vf visualforce.Index, index typesys.Index, name string, c v04Case) string {
	t.Helper()
	org, user := v04Org()
	page, _ := vf.Page(name)
	srv := server.NewWithSource(&org, server.SourceMetadata{Project: p})
	srv.VisualforceHTMLUserID = user.ID
	machine := vm.New(nil)
	runtimeErr := apextest.RegisterUncachedProjectRuntimeForRequest(machine, index)
	srv.SetProjectRuntime(index, machine, runtimeErr)
	response := httptest.NewRecorder()
	srv.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/apex/"+name+"?id=001000000000004AAA", nil))
	if response.Code >= http.StatusBadRequest || c.HTTPStatus[page.APIVersion] != 0 {
		return v04ObserveHTTPError(t, response, c.HTTPStatus[page.APIVersion], "render")
	}
	if response.Code >= http.StatusMultipleChoices && response.Code < http.StatusBadRequest {
		return v04DOM(t, map[string]any{"redirectURL": response.Header().Get("Location"), "redirect": response.Code == http.StatusFound})
	}
	doc, err := html.Parse(strings.NewReader(response.Body.String()))
	if err != nil {
		return v04ObserveError(t, err)
	}
	marker := v04Find(doc, func(n *html.Node) bool { return v04Attr(n, "data-case") == c.ID })
	if marker != nil && strings.TrimSpace(v04Text(marker)) == "PENDING" {
		// Submit the actual rendered V04_RUN button and successful controls.
		// Never call runProbe directly or supply controller fields/actions.
		button := v04Find(doc, func(n *html.Node) bool { return n.Data == "input" && v04Attr(n, "value") == "V04_RUN" })
		var form *html.Node
		for parent := button; parent != nil; parent = parent.Parent {
			if parent.Data == "form" {
				form = parent
				break
			}
		}
		if button == nil || form == nil {
			return v04DOM(t, map[string]any{"localError": "rendered submit control/form missing"})
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
		encoded := url.Values{}
		for name, value := range values {
			encoded.Set(name, value)
		}
		request := httptest.NewRequest(http.MethodPost, v04Attr(form, "action"), strings.NewReader(encoded.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response = httptest.NewRecorder()
		srv.ServeHTTP(response, request)
		if response.Code >= http.StatusBadRequest {
			return v04ObserveHTTPError(t, response, 0, "postback-render")
		}
		if response.Code >= http.StatusMultipleChoices && response.Code < http.StatusBadRequest {
			return v04DOM(t, map[string]any{"redirectURL": response.Header().Get("Location"), "redirect": response.Code == http.StatusFound})
		}
		doc, err = html.Parse(strings.NewReader(response.Body.String()))
		if err != nil {
			return v04ObserveError(t, err)
		}
	}
	return v04ObserveDOM(t, doc, c.ID)
}

// Observe the whole-page HTTP boundary, including the captured null-member
// status. Other failures retain their HTTP text/status as a visible mismatch.
func v04ObserveHTTPError(t *testing.T, response *httptest.ResponseRecorder, expectedStatus int, stage string) string {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(response.Body.String()))
	if err != nil {
		return v04ObserveError(t, err)
	}
	result := map[string]any{"stage": stage, "diagnostics": []string{strings.TrimSpace(v04Text(doc))}}
	if response.Code != expectedStatus {
		result["httpStatus"] = response.Code
	}
	return v04DOM(t, map[string]any{"result": result, "binding": []any{}})
}

var v04FixtureVersion = regexp.MustCompile(`FamilyV04[PE]\d{5}`)
var v04ID = regexp.MustCompile(`(?:001|003|005)[A-Za-z0-9]{12}(?:[A-Za-z0-9]{3})?`)
var v04Token = regexp.MustCompile(`(?i)(_CONFIRMATIONTOKEN(?:=|%3D))[^&\s"<>\\]+`)
var v04ContractToken = regexp.MustCompile(`(_CONFIRMATIONTOKEN=)[^&"]*`)
var v04ActionHook = regexp.MustCompile(`\.value='([^']*)'`)
var v04URL = regexp.MustCompile(`https?://[^\s"'<>]+`)

// A token-only carry cannot hide changes to the route, ID, return target,
// redirect flag or any other observed value. Full-row equality still determines
// the match count; this additional assertion checks every other byte exactly.
func v04ConfirmationContract(text string) string {
	return v04ContractToken.ReplaceAllString(text, "${1}<TOKEN>")
}

func v04Stable(text string) string {
	text = v04URL.ReplaceAllString(text, "<URL>")
	text = v04ID.ReplaceAllStringFunc(text, func(id string) string { return "<ID:" + id[:3] + ">" })
	text = v04Token.ReplaceAllString(text, "${1}<REDACTED>")
	return v04FixtureVersion.ReplaceAllString(text, "<owned>")
}

func v04ObserveError(t *testing.T, err error) string {
	return v04DOM(t, map[string]any{"result": map[string]any{"stage": "render", "diagnostics": strings.Split(err.Error(), "\n")}, "binding": []any{}})
}

func v04Attr(n *html.Node, key string) string {
	if n != nil {
		for _, a := range n.Attr {
			if a.Key == key {
				return a.Val
			}
		}
	}
	return ""
}

func v04HasAttr(n *html.Node, key string) bool {
	if n != nil {
		for _, a := range n.Attr {
			if a.Key == key {
				return true
			}
		}
	}
	return false
}

func v04Walk(n *html.Node, visit func(*html.Node)) {
	if n == nil {
		return
	}
	visit(n)
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		v04Walk(child, visit)
	}
}

func v04Find(n *html.Node, predicate func(*html.Node) bool) *html.Node {
	if n == nil {
		return nil
	}
	if predicate(n) {
		return n
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := v04Find(child, predicate); found != nil {
			return found
		}
	}
	return nil
}

func v04Text(n *html.Node) string {
	var out strings.Builder
	var collect func(*html.Node)
	collect = func(n *html.Node) {
		if n == nil || (n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style")) {
			return
		}
		if n.Type == html.TextNode {
			out.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			collect(child)
		}
	}
	collect(n)
	return out.String()
}

func v04ControlValue(n *html.Node) string {
	if n.Data == "textarea" {
		return v04Text(n)
	}
	if n.Data == "select" {
		var first, selected *html.Node
		v04Walk(n, func(o *html.Node) {
			if o.Data == "option" {
				if first == nil {
					first = o
				}
				if v04HasAttr(o, "selected") {
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
		if v04HasAttr(selected, "value") {
			return v04Attr(selected, "value")
		}
		return v04Text(selected)
	}
	return v04Attr(n, "value")
}

func v04ObserveDOM(t *testing.T, doc *html.Node, id string) string {
	t.Helper()
	marker := v04Find(doc, func(n *html.Node) bool { return v04Attr(n, "data-case") == id })
	if marker == nil {
		return v04DOM(t, map[string]any{"localError": "rendered result marker missing"})
	}
	text := strings.TrimSpace(v04Text(marker))
	var observed any
	// The native browser capture parses the result JSON before exporting it;
	// numeric scale is lost there, while binding text remains byte-exact.
	if err := json.Unmarshal([]byte(text), &observed); err != nil {
		return v04DOM(t, map[string]any{"localError": "rendered result JSON invalid", "text": text})
	}
	bindings := []any{}
	v04Walk(doc, func(n *html.Node) {
		if v04Attr(n, "data-binding") != "owned" {
			return
		}
		inputs := []any{}
		v04Walk(n, func(control *html.Node) {
			if control.Type != html.ElementNode || (control.Data != "input" && control.Data != "textarea" && control.Data != "select") {
				return
			}
			kind := v04Attr(control, "type")
			if control.Data == "input" && kind == "" {
				kind = "text"
			} else if control.Data == "textarea" {
				kind = "textarea"
			} else if control.Data == "select" {
				kind = "select-one"
				if v04HasAttr(control, "multiple") {
					kind = "select-multiple"
				}
			}
			inputs = append(inputs, map[string]any{"tag": strings.ToUpper(control.Data), "type": kind, "value": v04ControlValue(control)})
		})
		bindings = append(bindings, map[string]any{"text": strings.TrimSpace(v04Text(n)), "inputs": inputs})
	})
	return v04DOM(t, map[string]any{"result": observed, "binding": bindings})
}

func v04Encode(t *testing.T, value any) string {
	t.Helper()
	var out strings.Builder
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(out.String(), "\n")
}

// The captured TSV uses Python ensure_ascii=True for its outer JSON framing.
// Escape framing identically while preserving the decoded observed text.
func v04DOM(t *testing.T, value any) string {
	t.Helper()
	encoded := v04Stable(v04Encode(t, value))
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
