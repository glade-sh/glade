package visualforce_test

import (
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
	"golang.org/x/net/html"
)

type v11Case struct {
	ID                    string                     `json:"id"`
	Group                 string                     `json:"group"`
	Kind                  string                     `json:"kind"`
	Tamper                string                     `json:"tamper"`
	CrossPageNames        map[string]string          `json:"cross_page_names,omitempty"`
	Inputs                map[string]v10Input        `json:"inputs"`
	Expected              map[string]string          `json:"expected"`
	Diagnostics           map[string][]v10Diagnostic `json:"diagnostics"`
	Owner                 string                     `json:"owner,omitempty"`
	Reason                string                     `json:"reason,omitempty"`
	ExcludedReason        string                     `json:"excluded_reason,omitempty"`
	ExcludedOutcomeReason string                     `json:"excluded_outcome_reason,omitempty"`
	SizeCapture           bool                       `json:"size_capture,omitempty"`
	MinimalSizeControl    bool                       `json:"minimal_size_control,omitempty"`
}

// TestV11SalesforceConformance compares every captured compile/metadata and
// view-state GET/postback DOM row at API 59/67. Source and native text are
// retained as owned fixtures. Compile fixtures
// were checked against captured source hashes; runtime inputs are deployed sources.
// The Form lifecycle compiler adapter and DOM/form helpers supply the shared test paths.
// Fresh request VMs, controller restoration, state encoding and validation use
// product code. No Apex behavior or native token fields are supplied by the test.
// Every row compares exact text, including raw null values and diagnostics.
// GLADE_V11_CAPTURE=1 reports mismatches without failing on the unchanged base;
// GLADE_V11_REPORT selects the per-row TSV output path. CI needs no credentials.
func TestV11SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_V11_CAPTURE") != ""
	data, err := os.ReadFile("testdata/v11_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Source string    `json:"source"`
		Cases  []v11Case `json:"cases"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if table.Source != "Owned view-state observations at API 59.0 and 67.0." || len(table.Cases) != 240 {
		t.Fatalf("View state requires 240 captured cases, got %d", len(table.Cases))
	}

	versions := []string{"59.0", "67.0"}
	counts, seen := map[string]int{}, map[string]bool{}
	for _, c := range table.Cases {
		if c.ID == "" || seen[c.ID] || (c.Kind != "compile" && c.Kind != "runtime") {
			t.Fatalf("invalid or duplicate View state case %q (%s)", c.ID, c.Kind)
		}
		seen[c.ID] = true
		if (c.Owner == "") != (c.Reason == "") || strings.Contains(c.Owner, "View state") {
			t.Fatalf("%s: invalid carry owner/reason", c.ID)
		}
		if c.ExcludedOutcomeReason != "" && (c.Kind != "runtime" || c.Tamper == "" || c.Owner != "" || c.ExcludedReason != "") {
			t.Fatalf("%s: invalid hosted-outcome exclusion", c.ID)
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
				var steps []json.RawMessage
				if json.Unmarshal(native["steps"], &steps) != nil || (len(steps) == 0 && len(native["outcome"]) == 0) {
					t.Fatalf("%s: incomplete native steps at API %s", c.ID, api)
				}
				var decoded any
				decoder := json.NewDecoder(strings.NewReader(strings.TrimPrefix(answer, "DOM|")))
				decoder.UseNumber()
				if err := decoder.Decode(&decoded); err != nil || v11DOM(t, decoded) != answer {
					t.Fatalf("%s: native DOM framing changed at API %s", c.ID, api)
				}
			}
		}
	}
	if counts["compile"] != 130 || counts["runtime"] != 110 {
		t.Fatalf("View state requires 130 compile/metadata and 110 DOM rows, got %v", counts)
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
	total, compileTotal, domTotal := 0, 0, 0
	for _, api := range versions {
		apiMatches, apiCompileMatches, apiAcceptanceMatches, apiDiagnosticMatches, apiDOMMatches := 0, 0, 0, 0, 0
		apiCompileTotal, apiDOMTotal := 0, 0
		apiSecurityMatches, apiSecurityTotal := 0, 0
		t.Run(api, func(t *testing.T) {
			for _, c := range table.Cases {
				t.Run(c.ID, func(t *testing.T) {
					if c.ExcludedReason != "" {
						t.Logf("API %s %s retired exact row: %s", api, c.ID, c.ExcludedReason)
						return
					}
					if c.Kind == "compile" {
						apiCompileTotal++
					} else {
						apiDOMTotal++
					}
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
					got, diagnostics := v11Execute(t, root, input.Name, api, c)
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
					if c.ExcludedOutcomeReason != "" {
						apiSecurityTotal++
						// The hosted response body/status are an excluded boundary,
						// not an exemption for state validation or any preceding step.
						observed := v11SecurityComparable(t, got, false)
						expected := v11SecurityComparable(t, want, true)
						if observed == expected && actualDiagnostic == expectedDiagnostic {
							apiSecurityMatches++
							status = "MATCH_SCOPED"
						} else if !capture {
							t.Errorf("API %s %s state rejection: expected <%s %s> actual <%s %s>", api, c.ID, expected, expectedDiagnostic, observed, actualDiagnostic)
						}
						if acceptanceMatch {
							t.Errorf("API %s %s now matches full native output; remove stale hosted-outcome exclusion", api, c.ID)
						}
						reason = c.ExcludedOutcomeReason
						t.Logf("API %s %s excluded hosted outcome: %s; exact rejection/steps expected <%s> actual <%s>", api, c.ID, reason, expected, observed)
					} else if (!acceptanceMatch || !diagnosticMatch) && c.Owner != "" {
						owner, reason = c.Owner, c.Reason
						t.Logf("API %s %s carry %s: %s; actual <%s> expected <%s>; diagnostics <%s> expected <%s>", api, c.ID, owner, reason, got, want, actualDiagnostic, expectedDiagnostic)
					} else if c.Owner != "" {
						t.Errorf("API %s %s now matches native; remove stale %s carry", api, c.ID, c.Owner)
					}
					writeRow(api, c.ID, status, got, want, c.Group, "ApexPage", strings.ReplaceAll(actualDiagnostic, root, "<fixture>"), owner, reason, acceptanceStatus, diagnosticStatus, expectedDiagnostic)
					if !capture && owner == "" && c.ExcludedOutcomeReason == "" && (!acceptanceMatch || !diagnosticMatch) {
						t.Errorf("API %s %s: actual <%s> expected <%s>; diagnostics <%s> expected <%s>", api, c.ID, got, want, actualDiagnostic, expectedDiagnostic)
					}
				})
			}
		})
		apiTotal := apiCompileTotal + apiDOMTotal
		if apiTotal == 0 {
			continue
		}
		writeRow("API_TOTAL", api, fmt.Sprintf("%d/%d", apiMatches, apiTotal))
		writeRow("COMPILE_API_TOTAL", api, fmt.Sprintf("%d/%d", apiCompileMatches, apiCompileTotal))
		writeRow("COMPILE_ACCEPTANCE_API_TOTAL", api, fmt.Sprintf("%d/%d", apiAcceptanceMatches, apiCompileTotal))
		writeRow("DIAGNOSTIC_API_TOTAL", api, fmt.Sprintf("%d/%d", apiDiagnosticMatches, apiCompileTotal))
		writeRow("DOM_API_TOTAL", api, fmt.Sprintf("%d/%d", apiDOMMatches, apiDOMTotal))
		writeRow("SECURITY_SCOPED_API_TOTAL", api, fmt.Sprintf("%d/%d", apiSecurityMatches, apiSecurityTotal))
		t.Logf("View state API %s exact %d/%d; compile %d/%d, acceptance %d/%d, diagnostics %d/%d; DOM %d/%d", api, apiMatches, apiTotal, apiCompileMatches, apiCompileTotal, apiAcceptanceMatches, apiCompileTotal, apiDiagnosticMatches, apiCompileTotal, apiDOMMatches, apiDOMTotal)
		t.Logf("View state API %s security rejection/steps exact %d/%d; hosted outcome excluded; counted separately from raw exact matches", api, apiSecurityMatches, apiSecurityTotal)
		total += apiTotal
		compileTotal += apiCompileTotal
		domTotal += apiDOMTotal
		matches += apiMatches
		compileMatches += apiCompileMatches
		acceptanceMatches += apiAcceptanceMatches
		diagnosticMatches += apiDiagnosticMatches
		domMatches += apiDOMMatches
	}
	writeRow("COMPILE_TOTAL", fmt.Sprintf("%d/%d", compileMatches, compileTotal))
	writeRow("COMPILE_ACCEPTANCE_TOTAL", fmt.Sprintf("%d/%d", acceptanceMatches, compileTotal))
	writeRow("DIAGNOSTIC_TOTAL", fmt.Sprintf("%d/%d", diagnosticMatches, compileTotal))
	writeRow("DOM_TOTAL", fmt.Sprintf("%d/%d", domMatches, domTotal))
	writeRow("TOTAL", fmt.Sprintf("%d/%d", matches, total))
	t.Logf("View state exact matches %d/%d; compile %d/%d; DOM %d/%d", matches, total, compileMatches, compileTotal, domMatches, domTotal)
	tsv.Flush()
	if err := tsv.Error(); err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("GLADE_V11_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("size-controls", v11SizeControlConformance)
}

// Native tamper captures retain the hosted error page in outcome.frames, with
// no_data=true and the failing stage. The local boundary returns a state
// rejection instead. Compare every preceding observation and the rejection stage
// exactly; exclude only the hosted response rendering/transport representation.
func v11SecurityComparable(t *testing.T, text string, native bool) string {
	t.Helper()
	var value map[string]any
	decoder := json.NewDecoder(strings.NewReader(strings.TrimPrefix(text, "DOM|")))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	outcome, _ := value["outcome"].(map[string]any)
	rejected, _ := outcome["state_rejection"].(bool)
	if native {
		var ok bool
		rejected, ok = outcome["no_data"].(bool)
		if !ok || !rejected || outcome["stage"] == nil {
			t.Fatal("hosted-outcome exclusion requires a captured no-data rejection and stage")
		}
	}
	value["outcome"] = map[string]any{"stage": outcome["stage"], "no_data": rejected}
	return v11DOM(t, value)
}

// Native wire lengths and the measured-KB suffix are explicitly retired
// observables, not carries or inferred local expectations. The export keeps all
// three raw native samples. Compare their remaining observable behavior exactly
// and require every sample to agree before comparing it with product execution.
func v11SizeControlConformance(t *testing.T) {
	data, err := os.ReadFile("testdata/v11_size_controls_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Source              string                          `json:"source"`
		ExcludedObservables []struct{ Name, Reason string } `json:"excluded_observables"`
		Cases               []struct {
			v11Case
			NativeCompile map[string]string            `json:"native_compile"`
			Samples       map[string][]json.RawMessage `json:"samples"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if table.Source != "Owned view-state size controls at API 59.0 and 67.0." || len(table.Cases) != 39 || len(table.ExcludedObservables) != 2 {
		t.Fatalf("incomplete View state native size control export")
	}
	for _, excluded := range table.ExcludedObservables {
		if excluded.Name == "" || excluded.Reason == "" {
			t.Fatal("missing retired observable reason")
		}
		t.Logf("retired size observable %s: %s", excluded.Name, excluded.Reason)
	}
	for _, api := range []string{"59.0", "67.0"} {
		matches, carried := 0, 0
		t.Run(api, func(t *testing.T) {
			for _, row := range table.Cases {
				t.Run(row.ID, func(t *testing.T) {
					if row.Owner != "" || row.Reason != "" {
						if strings.TrimSpace(row.Owner) == "" || strings.TrimSpace(row.Reason) == "" || strings.Contains(row.Owner, "View state") {
							t.Fatal("invalid size control carry")
						}
					}
					if len(row.Samples[api]) != 3 || row.NativeCompile[api] != "COMPILE_OK" {
						t.Fatal("missing native samples/compile answer")
					}
					want := v11SizeComparable(t, string(row.Samples[api][0]))
					for _, sample := range row.Samples[api][1:] {
						if v11SizeComparable(t, string(sample)) != want {
							t.Fatal("native stable observations vary; capture controls required")
						}
					}
					root := t.TempDir()
					input := row.Inputs[api]
					for path, source := range input.Files {
						if filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "..") {
							t.Fatalf("invalid source path %q", path)
						}
						full := filepath.Join(root, path)
						if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(full, []byte(source), 0o600); err != nil {
							t.Fatal(err)
						}
					}
					compiled, diagnostics := v10Execute(t, root, input.Name, v10Case{ID: row.ID, Kind: "compile"})
					if compiled != row.NativeCompile[api] || len(diagnostics) != 0 {
						t.Fatalf("native compile <%s []> actual <%s %s>", row.NativeCompile[api], compiled, v10Encode(t, diagnostics))
					}
					actual, diagnostics := v11Execute(t, root, input.Name, api, row.v11Case)
					got := v11SizeComparable(t, strings.TrimPrefix(actual, "DOM|"))
					if len(diagnostics) != 0 {
						t.Fatalf("unexpected local diagnostics %s", v10Encode(t, diagnostics))
					}
					if got == want {
						matches++
						if row.Owner != "" {
							t.Errorf("API %s %s now matches native; remove stale %s carry", api, row.ID, row.Owner)
						}
						return
					}
					if row.Owner != "" {
						carried++
						t.Logf("API %s %s carry %s: %s; expected <%s> actual <%s>", api, row.ID, row.Owner, row.Reason, want, got)
						return
					}
					t.Errorf("API %s %s stable size control expected <%s> actual <%s>", api, row.ID, want, got)
				})
			}
		})
		t.Logf("View state size controls API %s exact stable behavior %d/39; carried %d/39; native compile 39/39; wire/KB metadata excluded", api, matches, carried)
	}
}

func v11SizeComparable(t *testing.T, text string) string {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		t.Fatal(err)
	}
	if steps, ok := value["steps"].([]any); ok {
		for _, raw := range steps {
			step := raw.(map[string]any)
			if tokens, ok := step["tokens"].(map[string]any); ok {
				if fields, ok := tokens["fields"].([]any); ok {
					for _, rawField := range fields {
						field := rawField.(map[string]any)
						delete(field, "encodedCharacters")
						delete(field, "decodedBytes")
					}
				}
			}
		}
	}
	if outcome, ok := value["outcome"].(map[string]any); ok {
		if lines, ok := outcome["native_error"].([]any); ok {
			for i, line := range lines {
				if text, ok := line.(string); ok && strings.HasPrefix(text, "Maximum view state size limit (170KB) exceeded. Actual view state size for this page was ") && strings.HasSuffix(text, "KB") {
					lines[i] = "Maximum view state size limit (170KB) exceeded."
				}
			}
		}
	}
	return v11DOM(t, value)
}

func v11Execute(t *testing.T, root, name, api string, c v11Case) (string, []v10Diagnostic) {
	t.Helper()
	if c.Kind == "compile" {
		return v10Execute(t, root, name, v10Case{ID: c.ID, Kind: c.Kind})
	}
	diagnostics := []v10Diagnostic{}
	fail := func(err error, resource string) (string, []v10Diagnostic) {
		diagnostics = append(diagnostics, v10Diagnostic{Resource: resource, Type: "Error", Message: err.Error()})
		return v11DOM(t, map[string]any{"steps": []any{}, "outcome": v11Error("initial", err)}), diagnostics
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
	vf, err := visualforce.LoadProjectForRender(p)
	if err != nil {
		return fail(err, "ApexPage")
	}
	runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
	if err != nil {
		return fail(err, "ApexClass")
	}
	org := storage.NewOrgState()
	request := visualforce.PageRenderRequest{Project: p, VFIndex: vf, Org: &org, PageName: name, PageURL: "/apex/" + name, ViewStateSecret: []byte("View state owned conformance view-state secret")}
	var doc *html.Node
	var httpServer *server.Server
	var response *httptest.ResponseRecorder
	if c.Tamper == "remove" || c.Tamper == "duplicate" {
		// These captures exercise missing and repeated successful controls. Send
		// them to the same multivalue request boundary as actual form posts;
		// decoding or choosing one value here would mask request-handler fixes.
		org.APIVersion = api
		storage.EnsureStandardObject(&org, "User")
		userID := storage.ID("005000000000011")
		users := org.Objects["User"]
		users.Records[userID] = storage.Record{ID: userID, Object: "User", Fields: map[string]storage.Value{
			"Name": storage.StringValue("View state Owned User"), "Username": storage.StringValue("v11@example.test"), "IsActive": storage.BooleanValue(true),
		}}
		org.Objects["User"] = users
		httpServer = server.NewWithSource(&org, server.SourceMetadata{Project: p})
		httpServer.VisualforceHTMLUserID = userID
		machine := vm.New(nil)
		if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
			return fail(err, "ApexClass")
		}
		httpServer.SetProjectRuntime(index, machine, nil)
	}
	httpRender := func(method, target string, values url.Values) error {
		incoming := httptest.NewRequest(method, target, strings.NewReader(values.Encode()))
		if method == http.MethodPost {
			incoming.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		response = httptest.NewRecorder()
		httpServer.ServeHTTP(response, incoming)
		if response.Code >= http.StatusBadRequest {
			return fmt.Errorf("local HTTP %d: %s", response.Code, strings.TrimSpace(response.Body.String()))
		}
		if response.Code >= http.StatusMultipleChoices {
			return fmt.Errorf("local redirect: %s", response.Header().Get("Location"))
		}
		doc, err = html.Parse(strings.NewReader(response.Body.String()))
		return err
	}
	// Each request starts with a fresh VM. Only rendered form controls and the
	// product's encoded state carry controller data into the next request.
	render := func() error {
		if httpServer != nil {
			return httpRender(http.MethodGet, request.PageURL, nil)
		}
		machine := vm.New(nil)
		machine.Org = &org
		if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
			return err
		}
		request.Machine = machine
		result, err := visualforce.RenderPage(request)
		if err != nil {
			return err
		}
		if result.Error != nil {
			return result.Error
		}
		if result.RedirectURL != "" {
			return fmt.Errorf("local redirect: %s", result.RedirectURL)
		}
		doc, err = html.Parse(strings.NewReader(result.HTML))
		return err
	}
	steps := []any{}
	finish := func(stage string, err error) (string, []v10Diagnostic) {
		var noData *v11NoDataResponse
		if errors.As(err, &noData) {
			return v11DOM(t, map[string]any{"steps": steps, "outcome": noData.outcome(stage)}), diagnostics
		}
		return v11DOM(t, map[string]any{"steps": steps, "outcome": v11Error(stage, err)}), diagnostics
	}
	var saved []v11Field
	if c.Tamper == "cross_page" {
		// The oracle visits its control page first and replays its actual fields.
		controlName := c.CrossPageNames[api]
		if controlName == "" {
			return finish("cross-page-source", fmt.Errorf("cross-page source fixture missing"))
		}
		request.PageName, request.PageURL = controlName, "/apex/"+controlName
		if err := render(); err != nil {
			return finish("cross-page-source", err)
		}
		saved = v11Fields(doc)
		steps = append(steps, map[string]any{"stage": "cross-page-source", "mutation": v11Mutation(doc, "save_cross", &saved)})
		request.PageName, request.PageURL = name, "/apex/"+name
	}
	if err := render(); err != nil {
		return finish("initial", err)
	}
	observe := func(stage string, decode bool) error {
		marker := v10Find(doc, func(n *html.Node) bool { return v10Attr(n, "data-v11-case") == c.ID })
		if marker == nil {
			return fmt.Errorf("local owned row marker missing")
		}
		text := v10Text(marker)
		if httpServer != nil && decode && (text == "INITIAL" || text == "PREPARED") {
			// Native bounded_ready requires a changed restore observation. Keep
			// a real restarted page as the captured no-data outcome, not a
			// fabricated restored observation or an adapter decode failure.
			return &v11NoDataResponse{doc: doc, status: response.Code}
		}
		var observation any = text
		if decode {
			decoder := json.NewDecoder(strings.NewReader(text))
			decoder.UseNumber()
			if err := decoder.Decode(&observation); err != nil {
				observation = map[string]any{"text": text}
			} else {
				observation = v11ObservationNumbers(observation)
			}
		}
		if c.SizeCapture {
			step := map[string]any{"stage": stage, "tokens": v11SizeTokens(doc)}
			if c.MinimalSizeControl {
				step["observation"] = text
			} else {
				step["observationCharacters"] = len([]rune(text))
			}
			steps = append(steps, step)
		} else {
			steps = append(steps, map[string]any{"stage": stage, "observation": observation, "tokens": v11Tokens(doc)})
		}
		return nil
	}
	if err := observe("initial", false); err != nil {
		return finish("initial", err)
	}
	if c.Tamper == "replay_initial" {
		saved = v11Fields(doc)
	}
	post := func(label string) error {
		button := v10Find(doc, func(n *html.Node) bool {
			return n.Type == html.ElementNode && (n.Data == "input" || n.Data == "button") && v10ControlValue(n) == label
		})
		if button == nil {
			return fmt.Errorf("local rendered submit control missing: %s", label)
		}
		form := v10Ancestor(button, func(n *html.Node) bool { return n.Data == "form" })
		if form == nil {
			return fmt.Errorf("local rendered submit form missing")
		}
		action := v10Attr(button, "data-action")
		if action == "" {
			if match := v10ActionHook.FindStringSubmatch(v10Attr(button, "onclick")); len(match) == 2 {
				action = match[1]
			}
		}
		if httpServer != nil {
			values := v11SuccessfulControls(form, button)
			values.Set(visualforce.ViewStateActionFieldName(), action)
			return httpRender(http.MethodPost, v10Attr(form, "action"), values)
		}
		values := v10FormValues(form, button, true)
		values[visualforce.ViewStateActionFieldName()] = action
		encoded := values[visualforce.ViewStateFormFieldName()]
		payload, err := visualforce.DecodeViewState(encoded, request.ViewStateSecret)
		if err != nil {
			return err
		}
		parsed := visualforce.ParseAjaxPayload(values)
		request.ViewState, request.FormValues, request.Action = &payload, parsed.SubmittedFields, parsed.Action
		// Let RenderPage enforce page binding and public CSRF fields. Checking
		// them in the adapter would hide a regression in the request boundary.
		return render()
	}
	if err := post("V11_PREPARE"); err != nil {
		return finish("prepare", err)
	}
	if err := observe("prepare", false); err != nil {
		return finish("prepare", err)
	}
	if c.Tamper != "" {
		steps = append(steps, map[string]any{"stage": "tamper", "mutation": v11Mutation(doc, c.Tamper, &saved)})
	}
	stages := []string{"restore", "repeat"}
	if c.MinimalSizeControl {
		stages = []string{"restore"}
	}
	for _, stage := range stages {
		if err := post("V11_OBSERVE"); err != nil {
			return finish(stage, err)
		}
		if err := observe(stage, true); err != nil {
			return finish(stage, err)
		}
	}
	return v11DOM(t, map[string]any{"steps": steps}), diagnostics
}

// Match the browser's successful-control collection without reducing repeated
// names. In particular, removal stays absent and duplicate state tokens remain
// ordered until Server.ServeHTTP applies its own request policy.
func v11SuccessfulControls(form, button *html.Node) url.Values {
	values := url.Values{}
	v10Walk(form, func(n *html.Node) {
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
		if kind == "select-multiple" {
			v10Walk(n, func(option *html.Node) {
				if option.Data == "option" && v10HasAttr(option, "selected") && !v10HasAttr(option, "disabled") {
					value := v10Attr(option, "value")
					if !v10HasAttr(option, "value") {
						value = v10Text(option)
					}
					values.Add(name, value)
				}
			})
			return
		}
		value := v10ControlValue(n)
		if n.Data == "textarea" {
			value = strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
			value = strings.ReplaceAll(value, "\n", "\r\n")
		}
		values.Add(name, value)
	})
	return values
}

type v11NoDataResponse struct {
	doc    *html.Node
	status int
}

func (*v11NoDataResponse) Error() string { return "owned page has no changed observation" }

func (response *v11NoDataResponse) outcome(stage string) map[string]any {
	var body strings.Builder
	var visibleText func(*html.Node)
	visibleText = func(n *html.Node) {
		if n == nil || n.Data == "script" || n.Data == "style" || v10HasAttr(n, "hidden") || strings.Contains(strings.ReplaceAll(v10Attr(n, "style"), " ", ""), "display:none") {
			return
		}
		if n.Type == html.TextNode {
			body.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visibleText(child)
		}
	}
	visibleText(v10Find(response.doc, func(n *html.Node) bool { return n.Data == "body" }))
	title := v10Find(response.doc, func(n *html.Node) bool { return n.Data == "title" })
	messages := []string{}
	v10Walk(response.doc, func(n *html.Node) {
		selected := v10Attr(n, "id") == "errorTitle" || v10Attr(n, "id") == "errorBody"
		for _, class := range strings.Fields(v10Attr(n, "class")) {
			selected = selected || class == "messageText" || class == "errorMsg" || class == "errorMessage"
		}
		if text := strings.TrimSpace(v10Text(n)); selected && text != "" {
			messages = append(messages, text)
		}
	})
	return map[string]any{
		"stage": stage, "no_data": true, "http_status": response.status,
		"frames": []any{map[string]any{"title": v10Text(title), "messages": messages, "body": strings.TrimSpace(body.String())}},
	}
}

func TestV11SuccessfulControlsPreservesMissingAndDuplicateState(t *testing.T) {
	for _, mode := range []string{"remove", "duplicate"} {
		t.Run(mode, func(t *testing.T) {
			doc, err := html.Parse(strings.NewReader(`<form><input type="hidden" name="com.salesforce.visualforce.ViewState" value="original"><input type="submit" name="observe" value="V11_OBSERVE"></form>`))
			if err != nil {
				t.Fatal(err)
			}
			var saved []v11Field
			v11Mutation(doc, mode, &saved)
			form := v10Find(doc, func(n *html.Node) bool { return n.Data == "form" })
			button := v10Find(doc, func(n *html.Node) bool { return v10Attr(n, "name") == "observe" })
			values := v11SuccessfulControls(form, button)
			encoded, err := url.ParseQuery(values.Encode())
			if err != nil {
				t.Fatal(err)
			}
			state, present := encoded[visualforce.ViewStateFormFieldName()]
			if mode == "remove" && present {
				t.Errorf("removed state was submitted: %v", state)
			}
			if mode == "duplicate" && (len(state) != 2 || state[0] != "original" || state[1] != "V11_INVALID") {
				t.Errorf("state values = %v, want ordered original and V11_INVALID", state)
			}
			if encoded.Get("observe") != "V11_OBSERVE" {
				t.Errorf("successful submit control missing: %v", encoded)
			}
		})
	}
}

func v11SizeTokens(doc *html.Node) map[string]any {
	fields := []any{}
	for _, n := range v11Nodes(doc) {
		parts := strings.Split(v10Attr(n, "name"), ":")
		fields = append(fields, map[string]any{"name": parts[len(parts)-1], "nonempty": v10Attr(n, "value") != ""})
	}
	length := func(attribute string) any {
		marker := v10Find(doc, func(n *html.Node) bool { return v10Attr(n, attribute) != "" })
		if marker == nil {
			return nil
		}
		return v10Text(marker)
	}
	return map[string]any{"fields": fields, "valueLength": length("data-v11-value-length"), "cacheLength": length("data-v11-cache-length")}
}

// The native observer uses Python json.loads/json.dumps on the Apex JSON text.
// Integer tokens retain exact precision; floating tokens use the shortest
// float representation instead of preserving Apex's lexical decimal scale.
func v11ObservationNumbers(value any) any {
	switch value := value.(type) {
	case json.Number:
		if !strings.ContainsAny(value.String(), ".eE") {
			return value
		}
		number, err := value.Float64()
		if err != nil {
			return value
		}
		format := byte('f')
		if number != 0 && (number > -0.0001 && number < 0.0001 || number <= -1e16 || number >= 1e16) {
			format = 'e'
		}
		text := strconv.FormatFloat(number, format, -1, 64)
		if !strings.ContainsAny(text, ".eE") {
			text += ".0"
		}
		return json.Number(text)
	case map[string]any:
		for key, item := range value {
			value[key] = v11ObservationNumbers(item)
		}
	case []any:
		for i, item := range value {
			value[i] = v11ObservationNumbers(item)
		}
	}
	return value
}

var v11FixtureVersion = regexp.MustCompile(`FamilyV11\d{2}_\d{3}[PC]`)
var v11StateName = regexp.MustCompile(`(?i)com\.salesforce\.visualforce\.ViewState`)
var v11NativeError = regexp.MustCompile(`(?i)Error (?:is in|evaluating) expression|(?:System\.)?(?:QueryException|SObjectException|NullPointerException|VisualforceException|DmlException|IllegalArgumentException)|SObject row was retrieved via SOQL|(?:Invalid|No such) (?:field|column)|unexpected token|Didn.t understand relationship|Attempt to de-reference a null object|Argument cannot be null|List index out of bounds|view ?state|invalid session|CSRF|An internal server error has occurred`)

func v11Error(stage string, err error) map[string]any {
	// Select the native observer's error lines, retaining the original text.
	// Local transport failures remain explicit mismatches, never native bodies.
	selected := map[string]bool{}
	for _, line := range strings.Split(err.Error(), "\n") {
		if v11NativeError.MatchString(line) {
			selected[strings.TrimSpace(line)] = true
		}
	}
	outcome := map[string]any{"stage": stage}
	// Page binding currently has an exact product error rather than a sentinel.
	// It is returned by RenderPage, never synthesized by this observer.
	if errors.Is(err, visualforce.ErrViewStateInvalid) || errors.Is(err, visualforce.ErrViewStateTampered) || errors.Is(err, visualforce.ErrViewStateCSRF) || errors.Is(err, visualforce.ErrViewStateExpired) || err.Error() == "view state page mismatch" {
		outcome["state_rejection"] = true
	}
	if len(selected) == 0 {
		outcome["local_error"] = err.Error()
		return outcome
	}
	lines := []string{}
	for line := range selected {
		lines = append(lines, line)
	}
	sort.Strings(lines)
	outcome["native_error"] = lines
	return outcome
}

type v11Field struct {
	Name, Value string
}

func v11Nodes(doc *html.Node) []*html.Node {
	fields := []*html.Node{}
	v10Walk(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "input" && v10Attr(n, "type") == "hidden" && v11StateName.MatchString(v10Attr(n, "name")) {
			fields = append(fields, n)
		}
	})
	return fields
}

func v11Fields(doc *html.Node) []v11Field {
	fields := []v11Field{}
	for _, n := range v11Nodes(doc) {
		fields = append(fields, v11Field{v10Attr(n, "name"), v10Attr(n, "value")})
	}
	return fields
}

func v11TokenNodes(doc *html.Node) (fields []*html.Node, token, csrf *html.Node) {
	fields = v11Nodes(doc)
	for _, n := range fields {
		name := v10Attr(n, "name")
		if token == nil && strings.HasSuffix(name, ".ViewState") {
			token = n
		}
		if csrf == nil && strings.HasSuffix(name, ".ViewStateCSRF") {
			csrf = n
		}
	}
	return
}

func v11Tokens(doc *html.Node) map[string]any {
	nodes, token, _ := v11TokenNodes(doc)
	fields := []any{}
	for _, n := range nodes {
		name := strings.Split(v10Attr(n, "name"), ":")
		fields = append(fields, map[string]any{"name": name[len(name)-1], "nonempty": v10Attr(n, "value") != ""})
	}
	var plaintext, decoded any
	if token != nil {
		value := v10Attr(token, "value")
		plaintext = strings.Contains(value, "V11_CANARY_PRIVATE_7e4a")
		// atob accepts standard base64 with optional padding and whitespace.
		compact := strings.Map(func(r rune) rune {
			if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' {
				return -1
			}
			return r
		}, value)
		bytes, err := base64.StdEncoding.DecodeString(compact)
		if err != nil && !strings.Contains(compact, "=") {
			bytes, err = base64.RawStdEncoding.DecodeString(compact)
		}
		decoded = err == nil && strings.Contains(string(bytes), "V11_CANARY_PRIVATE_7e4a")
	}
	return map[string]any{"fields": fields, "plaintextContainsCanary": plaintext, "decodedContainsCanary": decoded}
}

func v11Mutation(doc *html.Node, mode string, saved *[]v11Field) map[string]any {
	fields, token, csrf := v11TokenNodes(doc)
	targets := 0
	change := func(n *html.Node, apply func(*html.Node)) {
		if n != nil {
			apply(n)
			targets++
		}
	}
	set := func(n *html.Node, value string) { v10SetAttr(n, "value", value) }
	flip := func(n *html.Node) {
		value := v10Attr(n, "value")
		first := "A"
		if strings.HasPrefix(value, "A") {
			first = "B"
		}
		if value != "" {
			value = value[1:]
		}
		set(n, first+value)
	}
	remove := func(n *html.Node) { n.Parent.RemoveChild(n) }
	switch mode {
	case "empty":
		change(token, func(n *html.Node) { set(n, "") })
	case "remove":
		change(token, remove)
	case "truncate":
		change(token, func(n *html.Node) { value := v10Attr(n, "value"); set(n, value[:len(value)/2]) })
	case "flip":
		change(token, flip)
	case "append":
		change(token, func(n *html.Node) { set(n, v10Attr(n, "value")+"AAAA") })
	case "invalid_base64":
		change(token, func(n *html.Node) { set(n, "%%%V11_INVALID%%%") })
	case "duplicate":
		change(token, func(n *html.Node) {
			copy := &html.Node{Type: n.Type, Data: n.Data, DataAtom: n.DataAtom, Attr: append([]html.Attribute(nil), n.Attr...)}
			set(copy, "V11_INVALID")
			n.Parent.InsertBefore(copy, n.NextSibling)
		})
	case "remove_version":
		for _, n := range fields {
			if strings.HasSuffix(v10Attr(n, "name"), ".ViewStateVersion") {
				change(n, remove)
				break
			}
		}
	case "remove_csrf":
		change(csrf, remove)
	case "flip_csrf":
		change(csrf, flip)
	case "empty_csrf":
		change(csrf, func(n *html.Node) { set(n, "") })
	case "save_initial", "save_cross":
		*saved = v11Fields(doc)
		targets = len(fields)
	case "replay_initial", "cross_page":
		for _, item := range *saved {
			for _, n := range fields {
				if v10Attr(n, "name") == item.Name {
					change(n, func(n *html.Node) { set(n, item.Value) })
					break
				}
			}
		}
	}
	// The native mutator reports references captured before removal.
	return map[string]any{"mode": mode, "targets": targets, "tokenPresent": token != nil, "csrfPresent": csrf != nil}
}

func v11DOM(t *testing.T, value any) string {
	t.Helper()
	// Python's oracle framing sorts object keys and uses comma/colon spaces
	// and ensure_ascii. Preserve nulls, numbers and all observed text exactly.
	encoded := v11FixtureVersion.ReplaceAllString(v10Encode(t, value), "<owned>")
	var out strings.Builder
	out.WriteString("DOM|")
	inString, escaped := false, false
	for _, r := range encoded {
		if r > 127 {
			for _, unit := range utf16.Encode([]rune{r}) {
				fmt.Fprintf(&out, "\\u%04x", unit)
			}
		} else {
			out.WriteRune(r)
		}
		if inString {
			if escaped {
				escaped = false
			} else if r == '\\' {
				escaped = true
			} else if r == '"' {
				inString = false
			}
		} else if r == '"' {
			inString = true
		} else if r == ',' || r == ':' {
			out.WriteByte(' ')
		}
	}
	return out.String()
}
