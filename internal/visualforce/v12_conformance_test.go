package visualforce_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type v12Case struct {
	ID            string                     `json:"id"`
	Group         string                     `json:"group"`
	Kind          string                     `json:"kind"`
	Component     string                     `json:"component"`
	Value         json.RawMessage            `json:"value"`
	Steps         int                        `json:"steps"`
	ParameterName string                     `json:"parameter_name,omitempty"`
	ErrorDocument bool                       `json:"error_document,omitempty"`
	MixedControl  *v12MixedControl           `json:"mixed_control,omitempty"`
	Inputs        map[string]v10Input        `json:"inputs"`
	Expected      map[string]string          `json:"expected"`
	Diagnostics   map[string][]v10Diagnostic `json:"diagnostics"`
}

type v12MixedControl struct {
	Order         string               `json:"order"`
	LinkTransport string               `json:"link_transport"`
	UniqueCommand string               `json:"unique_command,omitempty"`
	Submissions   []v12MixedSubmission `json:"submissions"`
}

type v12MixedSubmission struct {
	Control string `json:"control"`
	Value   string `json:"value"`
}

type v12Result struct {
	Text        string
	Diagnostics []v10Diagnostic
}

// TestV12SalesforceConformance compares all captured Metadata API and DOM/AJAX
// rows at API 59/67 as exact text, including diagnostic text and raw nulls.
// Owned fixtures and native answers are exported from the capture tools, with
// mixed-control, parameter/error and unique-action review captures recorded
// separately. Compile rows reuse the V10 adapter; DOM rows exercise the shared
// product HTTP server, controller runtime, view state, partial response and
// emitted JavaScript.
// Only fixture controls go to Chromium; native answers never go to the browser.
// CI uses exported testdata without Salesforce or glade-tools.
// GLADE_V12_CAPTURE=1 reports row differences without failing on mismatches;
// GLADE_V12_REPORT selects the per-row TSV output path.
func TestV12SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_V12_CAPTURE") != ""
	data, err := os.ReadFile("testdata/v12_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		OracleCommit               string    `json:"oracle_commit"`
		MixedControlOracleCommit   string    `json:"mixed_control_oracle_commit"`
		ParameterErrorOracleCommit string    `json:"parameter_error_oracle_commit"`
		UniqueActionOracleCommit   string    `json:"unique_action_oracle_commit"`
		Cases                      []v12Case `json:"cases"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if table.OracleCommit != "capture-revision-038994833800834840608883342634468529964973339481374197801731892579899416501695" || table.MixedControlOracleCommit != "capture-revision-017579274598624121181200487284224553621291546491292008348903503118259261196875" || table.ParameterErrorOracleCommit != "capture-revision-030784572789262464482397954069695350064343716144593654102480696193698042291231" || table.UniqueActionOracleCommit != "capture-revision-090827034908303855622090522814871962279188938241957268231173061579449906875864" || len(table.Cases) != 262 {
		t.Fatalf("V12 requires 196 original, 16 mixed-control, 18 parameter/error and 32 unique-action captured cases, got %d at %s / %s / %s / %s", len(table.Cases), table.OracleCommit, table.MixedControlOracleCommit, table.ParameterErrorOracleCommit, table.UniqueActionOracleCommit)
	}
	versions := []string{"59.0", "67.0"}
	counts, seen, groups := map[string]int{}, map[string]bool{}, map[string]bool{}
	for _, c := range table.Cases {
		if c.ID == "" || seen[c.ID] || (c.Kind != "compile" && c.Kind != "runtime") {
			t.Fatalf("invalid or duplicate V12 case %q (%s)", c.ID, c.Kind)
		}
		seen[c.ID], groups[c.Group] = true, true
		counts[c.Kind]++
		if c.ParameterName != "" {
			counts["parameter"]++
			if c.ParameterName != "n" && c.ParameterName != "f" && c.ParameterName != "o" && c.ParameterName != "r" {
				t.Fatalf("%s: unknown captured parameter name %q", c.ID, c.ParameterName)
			}
		}
		if c.ErrorDocument {
			counts["error_document"]++
		}
		if c.MixedControl == nil && c.ParameterName == "" && !c.ErrorDocument {
			counts["original"]++
		}
		if c.MixedControl != nil {
			counts["mixed"]++
			control := c.MixedControl
			if (control.Order != "function_first" && control.Order != "link_first") ||
				(control.LinkTransport != "full" && control.LinkTransport != "ajax") ||
				len(control.Submissions) != c.Steps {
				t.Fatalf("%s: incomplete mixed-control fixture", c.ID)
			}
			if control.UniqueCommand != "" {
				counts["unique"]++
				if control.UniqueCommand != "link" && control.UniqueCommand != "button" {
					t.Fatalf("%s: unknown unique command %q", c.ID, control.UniqueCommand)
				}
			}
			for _, submission := range control.Submissions {
				if submission.Control != "function" && submission.Control != "link" && (submission.Control != "unique" || control.UniqueCommand == "") {
					t.Fatalf("%s: unknown mixed-control submission %q", c.ID, submission.Control)
				}
			}
		}
		if len(c.Inputs) != 2 || len(c.Expected) != 2 || len(c.Diagnostics) != 2 || !json.Valid(c.Value) || c.Steps < 1 {
			t.Fatalf("%s: incomplete captured inputs or API answers", c.ID)
		}
		for _, api := range versions {
			input := c.Inputs[api]
			if input.Name == "" || len(input.Files) != 5 || input.Files["sfdx-project.json"] == "" ||
				input.Files["force-app/main/default/pages/"+input.Name+".page"] == "" ||
				input.Files["force-app/main/default/pages/"+input.Name+".page-meta.xml"] == "" {
				t.Fatalf("%s: incomplete captured input at API %s", c.ID, api)
			}
			answer, ok := c.Expected[api]
			diagnostics, hasDiagnostics := c.Diagnostics[api]
			if !ok || !hasDiagnostics || diagnostics == nil {
				t.Fatalf("%s: missing native answer or diagnostic list at API %s", c.ID, api)
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
				if !strings.HasPrefix(answer, "DOM|") || len(diagnostics) != 0 {
					t.Fatalf("%s: invalid native DOM answer at API %s", c.ID, api)
				}
				native := v12Decode(t, []byte(strings.TrimPrefix(answer, "DOM|")))
				// A captured null stays null. Never replace it with a shape or a
				// local expectation; exact comparison below includes the raw null.
				if native != nil {
					fields, ok := native.(map[string]any)
					if !ok || fields["before"] == nil || fields["outcomes"] == nil ||
						(fields["after"] == nil && fields["nativeError"] == nil) {
						t.Fatalf("%s: incomplete native DOM fields at API %s", c.ID, api)
					}
				}
				if v12DOM(t, native) != answer {
					t.Fatalf("%s: native DOM framing changed at API %s", c.ID, api)
				}
			}
		}
	}
	if counts["compile"] != 153 || counts["runtime"] != 109 || counts["mixed"] != 48 || counts["unique"] != 32 || counts["parameter"] != 16 || counts["error_document"] != 2 || counts["original"] != 196 || len(groups) != 4 ||
		!groups["action-components"] || !groups["rerender-targets"] || !groups["callbacks"] || !groups["errors"] {
		t.Fatalf("V12 requires 153 compile/metadata and 109 DOM rows, including 16 mixed-control, 18 parameter/error and 32 unique-action rows, across all four groups, got %v/%v", counts, groups)
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
		results, runtimeRoots := map[string]v12Result{}, map[string]string{}
		for _, c := range table.Cases {
			root := v12WriteFixture(t, c.Inputs[api])
			if c.Kind == "compile" {
				text, diagnostics := v10Execute(t, root, c.Inputs[api].Name, v10Case{Kind: "compile"})
				results[c.ID] = v12Result{Text: text, Diagnostics: diagnostics}
			} else {
				runtimeRoots["/apex/"+c.Inputs[api].Name] = root
			}
		}
		observed := v12BrowserObservations(t, table.Cases, api, runtimeRoots)
		for id, raw := range observed {
			results[id] = v12Result{Text: v12DOM(t, v12Decode(t, raw)), Diagnostics: []v10Diagnostic{}}
		}
		apiMatches, apiCompileMatches, apiAcceptanceMatches, apiDiagnosticMatches, apiDOMMatches := 0, 0, 0, 0, 0
		apiOriginalMatches := 0
		for _, c := range table.Cases {
			result, ok := results[c.ID]
			if !ok {
				t.Fatalf("V12 omitted %s at API %s", c.ID, api)
			}
			actualDiagnostic := v10Encode(t, result.Diagnostics)
			expectedDiagnostic := v10Encode(t, c.Diagnostics[api])
			acceptanceMatch := result.Text == c.Expected[api]
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
				if c.MixedControl == nil && c.ParameterName == "" && !c.ErrorDocument {
					apiOriginalMatches++
				}
				if c.Kind == "compile" {
					apiCompileMatches++
				} else {
					apiDOMMatches++
				}
			}
			writeRow(api, c.ID, status, result.Text, c.Expected[api], c.Group, "ApexPage", actualDiagnostic, "", "", acceptanceStatus, diagnosticStatus, expectedDiagnostic)
			if !capture && (!acceptanceMatch || !diagnosticMatch) {
				t.Errorf("API %s %s: actual <%s> expected <%s>; diagnostics <%s> expected <%s>", api, c.ID, result.Text, c.Expected[api], actualDiagnostic, expectedDiagnostic)
			}
		}
		writeRow("API_TOTAL", api, fmt.Sprintf("%d/262", apiMatches))
		writeRow("ORIGINAL_API_TOTAL", api, fmt.Sprintf("%d/196", apiOriginalMatches))
		writeRow("COMPILE_API_TOTAL", api, fmt.Sprintf("%d/153", apiCompileMatches))
		writeRow("COMPILE_ACCEPTANCE_API_TOTAL", api, fmt.Sprintf("%d/153", apiAcceptanceMatches))
		writeRow("DIAGNOSTIC_API_TOTAL", api, fmt.Sprintf("%d/153", apiDiagnosticMatches))
		writeRow("DOM_API_TOTAL", api, fmt.Sprintf("%d/109", apiDOMMatches))
		t.Logf("V12 API %s exact %d/262; original %d/196; compile %d/153, acceptance %d/153, diagnostics %d/153; DOM %d/109", api, apiMatches, apiOriginalMatches, apiCompileMatches, apiAcceptanceMatches, apiDiagnosticMatches, apiDOMMatches)
		matches += apiMatches
		compileMatches += apiCompileMatches
		acceptanceMatches += apiAcceptanceMatches
		diagnosticMatches += apiDiagnosticMatches
		domMatches += apiDOMMatches
	}
	writeRow("TOTAL", fmt.Sprintf("%d/524", matches))
	writeRow("COMPILE_TOTAL", fmt.Sprintf("%d/306", compileMatches))
	writeRow("COMPILE_ACCEPTANCE_TOTAL", fmt.Sprintf("%d/306", acceptanceMatches))
	writeRow("DIAGNOSTIC_TOTAL", fmt.Sprintf("%d/306", diagnosticMatches))
	writeRow("DOM_TOTAL", fmt.Sprintf("%d/218", domMatches))
	t.Logf("V12 Salesforce conformance: exact %d/524; compile %d/306, acceptance %d/306, diagnostics %d/306; DOM %d/218", matches, compileMatches, acceptanceMatches, diagnosticMatches, domMatches)
	tsv.Flush()
	if err := tsv.Error(); err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("GLADE_V12_REPORT"); path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(report.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func v12WriteFixture(t *testing.T, input v10Input) string {
	t.Helper()
	root := t.TempDir()
	for path, source := range input.Files {
		if filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "..") {
			t.Fatalf("V12 invalid fixture path %q", path)
		}
		fullPath := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func v12BrowserObservations(t *testing.T, cases []v12Case, api string, roots map[string]string) map[string]json.RawMessage {
	t.Helper()
	specs := []map[string]any{}
	files := map[string]string{}
	for _, c := range cases {
		if c.Kind == "runtime" {
			specs = append(specs, map[string]any{"id": c.ID, "path": "/apex/" + c.Inputs[api].Name, "component": c.Component, "value": c.Value, "steps": c.Steps, "mixed_control": c.MixedControl, "error_document": c.ErrorDocument})
			for path, source := range c.Inputs[api].Files {
				if previous, exists := files[path]; exists && previous != source {
					t.Fatalf("V12 conflicting shared runtime source %s at API %s", path, api)
				}
				files[path] = source
			}
		}
	}
	// Compile the byte-identical captured controllers once per API. The base
	// never executes: the shared server clones it for each actual HTTP request.
	// Each row still loads its own source project, page metadata and type index.
	root := v12WriteFixture(t, v10Input{Files: files})
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.APIVersion = api
	storage.EnsureDeterministicPlatformData(&org)
	base := vm.New(nil)
	base.SetOrg(&org)
	if err := apextest.RegisterProjectRuntimeForRequest(base, typesys.Build(p, sch)); err != nil {
		t.Fatalf("V12 API %s shared request runtime: %v", api, err)
	}
	base.FreezeClassLookup()
	orgTemplate := storage.NewRuntimeTemplate(org)
	t.Logf("V12 API %s linked shared request runtime for %d isolated DOM rows", api, len(specs))
	// Retain only the active row's server and private org, without compiling a
	// second request-runtime cache entry on every navigation or AJAX postback.
	var mu sync.Mutex
	var active *server.Server
	var activePath string
	var setupErr error
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if root, ok := roots[r.URL.Path]; ok && activePath != r.URL.Path {
			activePath, active, setupErr = r.URL.Path, nil, nil
			p, err := project.Load(root)
			if err != nil {
				setupErr = err
			} else if sch, err := schema.LoadProject(p); err != nil {
				setupErr = err
			} else if source, err := server.NewSourceMetadataFromProject(p); err != nil {
				setupErr = err
			} else {
				org := orgTemplate.CloneRuntimeOrg()
				active = server.NewWithSource(&org, source)
				active.VisualforceHTMLUserID = storage.ID(storage.DefaultUserID)
				active.SetProjectRuntime(typesys.Build(p, sch), base, nil)
			}
		}
		if setupErr != nil {
			http.Error(w, setupErr.Error(), http.StatusInternalServerError)
		} else if active != nil {
			active.ServeHTTP(w, r)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer httpServer.Close()
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(repo, "lwcruntime/node_modules/playwright")
	}
	observer, err := filepath.Abs("testdata/v12_browser.mjs")
	if err != nil {
		t.Fatal(err)
	}
	config := map[string]any{"url": httpServer.URL, "specs": specs, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", observer)
	cmd.Stdin = strings.NewReader(v10Encode(t, config))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("V12 API %s browser adapter: %v", api, err)
	}
	observed := map[string]json.RawMessage{}
	decoder := json.NewDecoder(stdout)
	var decodeErr error
	for {
		var row struct {
			ID    string          `json:"id"`
			Value json.RawMessage `json:"value"`
		}
		decodeErr = decoder.Decode(&row)
		if decodeErr != nil {
			break
		}
		if row.ID == "" || !json.Valid(row.Value) || observed[row.ID] != nil {
			decodeErr = fmt.Errorf("invalid or duplicate V12 browser row %q", row.ID)
			break
		}
		observed[row.ID] = row.Value
		t.Logf("V12 API %s observed %d/%d DOM rows: %s", api, len(observed), len(specs), row.ID)
	}
	if decodeErr != io.EOF {
		cancel()
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("V12 API %s browser adapter after %d/%d rows: %v: %s", api, len(observed), len(specs), err, stderr.String())
	}
	if decodeErr != io.EOF {
		t.Fatalf("V12 browser JSON: %v", decodeErr)
	}
	if len(observed) != len(specs) {
		t.Fatalf("V12 browser returned %d/%d rows at API %s", len(observed), len(specs), api)
	}
	for _, spec := range specs {
		if _, ok := observed[spec["id"].(string)]; !ok {
			t.Fatalf("V12 browser omitted %s at API %s", spec["id"], api)
		}
	}
	return observed
}

func v12Decode(t *testing.T, raw []byte) any {
	t.Helper()
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

var v12FixtureVersion = regexp.MustCompile(`FamilyV12[PC]\d{5}`)
var v12NativeErrorID = regexp.MustCompile(`Error ID: [^\s<]+`)

func v12DOM(t *testing.T, value any) string {
	t.Helper()
	// V11 shares the oracle's Python JSON framing (sorted keys, spaces and
	// ensure_ascii). Apply only V12's captured, case-sensitive name/Error ID
	// substitutions. Lowercase API-specific names in r_errors_throw stay raw.
	return v11DOM(t, v12StableValue(value))
}

func v12StableValue(value any) any {
	switch value := value.(type) {
	case string:
		value = v12FixtureVersion.ReplaceAllString(value, "<owned>")
		return v12NativeErrorID.ReplaceAllString(value, "Error ID: <native>")
	case []any:
		for i, item := range value {
			value[i] = v12StableValue(item)
		}
		return value
	case map[string]any:
		for key, item := range value {
			value[key] = v12StableValue(item)
		}
		return value
	default:
		return value
	}
}
