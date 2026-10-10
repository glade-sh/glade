package compile_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
)

type l14Case struct {
	ID       string            `json:"id"`
	Group    string            `json:"group"`
	Kind     string            `json:"kind"`
	Input    json.RawMessage   `json:"input"`
	Expected map[string]string `json:"expected"`
}

type l14Input struct {
	ID           string            `json:"id"`
	NativeID     string            `json:"native_id"`
	Host         string            `json:"host"`
	Group        string            `json:"group"`
	Kind         string            `json:"kind"`
	Basis        string            `json:"basis"`
	JS           string            `json:"js"`
	Template     string            `json:"template"`
	MetaFragment string            `json:"meta_fragment"`
	InitialState map[string]string `json:"initial_state"`
}

// TestL14SalesforceConformance replays the owned navigation inputs and exact
// native answers from the API 59/67 navigation capture.
// The 28 metadata answers are superseded by one-component native dry-runs:
// failed batch deployments had reported successes before target validation ran.
// The original 214 inputs remain: 72 compile/metadata and 142 native browser
// observations, plus 13 VF-hosted controls/API captured in a second org. Their
// exact inputs and native answers are exported under testdata/l14_vf_runtime/api{59,67}.
// No native answer is sent to the browser.
// Rejections also compare the captured diagnostics.json messages byte for byte,
// including positions, code frames, order and repeated messages.
// GLADE_L14_CAPTURE=1 reports mismatches without failing;
// GLADE_L14_REPORT selects the report destination. CI needs no Salesforce
// connection or glade-tools import. Product navigation and wire code is used
// unchanged, including the real tab route, history and page-reference bootstrap.
func TestL14SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_L14_CAPTURE") == "1"
	data, err := os.ReadFile("testdata/l14_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []l14Case
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile("testdata/l14_diagnostics.json")
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics map[string]map[string][]string
	if err := json.Unmarshal(data, &diagnostics); err != nil {
		t.Fatal(err)
	}
	versions := []string{"59.0", "67.0"}
	seen, groups := map[string]bool{}, map[string]bool{}
	var compileCases, runtimeCases, vfCases []l14Case
	for _, c := range cases {
		var input l14Input
		if err := json.Unmarshal(c.Input, &input); err != nil {
			t.Fatal(err)
		}
		if c.ID == "" || seen[c.ID] || input.ID != c.ID || input.Kind != c.Kind || input.Group != c.Group || input.Basis != "org" {
			t.Fatalf("invalid or duplicate L14 input %q", c.ID)
		}
		seen[c.ID], groups[c.Group] = true, true
		switch c.Kind {
		case "compile":
			if input.JS == "" || input.Template == "" {
				t.Fatalf("missing compile source for %s", c.ID)
			}
			compileCases = append(compileCases, c)
		case "runtime":
			if input.Host == "Visualforce Lightning Out" {
				vfCases = append(vfCases, c)
			} else {
				runtimeCases = append(runtimeCases, c)
			}
		default:
			t.Fatalf("invalid L14 kind for %s: %s", c.ID, c.Kind)
		}
		for _, api := range versions {
			want, ok := c.Expected[api]
			if !ok || (c.Kind == "compile" && want != "COMPILE_OK" && want != "COMPILE_ERROR") ||
				(c.Kind == "runtime" && (!strings.HasPrefix(want, "BROWSER|") || !json.Valid([]byte(strings.TrimPrefix(want, "BROWSER|"))))) {
				t.Fatalf("missing or invalid native answer for %s API %s", c.ID, api)
			}
			if c.Kind == "compile" {
				messages, ok := diagnostics[api][c.ID]
				if !ok || messages == nil || (want == "COMPILE_ERROR") != (len(messages) > 0) {
					t.Fatalf("missing or invalid native diagnostics for %s API %s", c.ID, api)
				}
				for _, message := range messages {
					if message == "" {
						t.Fatalf("empty native diagnostic for %s API %s", c.ID, api)
					}
				}
			}
		}
	}
	if len(cases) != 227 || len(compileCases) != 72 || len(runtimeCases) != 142 || len(vfCases) != 13 {
		t.Fatalf("L14 requires 72 compile/metadata + 142 tab DOM + 13 VF DOM rows, got %d + %d + %d", len(compileCases), len(runtimeCases), len(vfCases))
	}
	if len(diagnostics) != len(versions) {
		t.Fatalf("L14 requires diagnostics at both native APIs, got %d APIs", len(diagnostics))
	}
	for _, api := range versions {
		if len(diagnostics[api]) != len(compileCases) {
			t.Fatalf("L14 API %s requires diagnostics for all %d compile/metadata rows, got %d", api, len(compileCases), len(diagnostics[api]))
		}
	}
	for _, group := range []string{"page-reference-types", "GenerateUrl", "state", "CurrentPageReference", "component-targets", "VF-hosted GenerateUrl"} {
		if !groups[group] {
			t.Fatalf("missing L14 group %s", group)
		}
	}
	// Infrastructure failures must never satisfy native compiler-rejection rows.
	dependencyRoot, err := gladehome.Root()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"third_party/lwc/node_modules/@lwc/compiler/package.json", "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js", "third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js"} {
		if _, err := os.Stat(filepath.Join(dependencyRoot, filepath.FromSlash(rel))); err != nil {
			t.Fatal(err)
		}
	}

	var report strings.Builder
	report.WriteString("api\tid\tkind\tstatus\tactual\texpected\treason\tactual_diagnostics\texpected_diagnostics\tdiagnostic_status\n")
	matches, total, outcomeMatches, diagnosticMatches, diagnosticTotal := 0, 0, 0, 0, 0
	for _, api := range versions {
		compiler := l14CompileRows(t, api, compileCases)
		dom := l14ObserveDOM(t, api, runtimeCases, dependencyRoot, false)
		for id, value := range l14ObserveDOM(t, api, vfCases, dependencyRoot, true) {
			dom[id] = value
		}
		apiMatches, compileMatches, domMatches, apiOutcomeMatches, apiDiagnosticMatches, apiDiagnosticTotal := 0, 0, 0, 0, 0, 0
		for _, c := range cases {
			got, reason := "", "L14: native-vs-local navigation DOM output differs"
			gotDiagnostics, wantDiagnostics, diagnosticStatus := "", "", ""
			diagnosticsMatch := true
			if c.Kind == "compile" {
				result := compiler[c.ID]
				got, reason = "COMPILE_OK", "L01: native compile/metadata outcome differs"
				if result.Err != nil {
					got = "COMPILE_ERROR"
					reason += ": " + result.Err.Error()
				}
				// Keep the compiler's raw diagnostic messages. Metadata preflight
				// errors have no Diagnostics entry, so retain their raw error text.
				// No path, location, wording or diagnostic envelope is rewritten.
				messages := make([]string, 0, len(result.Diagnostics))
				for _, diagnostic := range result.Diagnostics {
					messages = append(messages, diagnostic.Message)
				}
				if result.Err != nil && len(messages) == 0 {
					messages = append(messages, result.Err.Error())
				}
				gotDiagnostics = l14JSON(t, messages)
				wantDiagnostics = l14JSON(t, diagnostics[api][c.ID])
				diagnosticsMatch = gotDiagnostics == wantDiagnostics
				diagnosticStatus = "MISMATCH"
				if diagnosticsMatch {
					diagnosticStatus = "MATCH"
				}
				if c.Expected[api] == "COMPILE_ERROR" {
					diagnosticTotal++
					apiDiagnosticTotal++
					if diagnosticsMatch {
						diagnosticMatches++
						apiDiagnosticMatches++
					}
				}
			} else {
				got = dom[c.ID]
				if got == "" {
					t.Fatalf("L14 observer omitted %s API %s", c.ID, api)
				}
				if strings.HasPrefix(got, "BROWSER_ERROR|") {
					reason = "L14: local navigation DOM observation failed: " + strings.TrimPrefix(got, "BROWSER_ERROR|")
				}
			}
			want := c.Expected[api]
			// Exact Go text equality preserves case, Id length, value types and
			// raw JSON null. No Apex assertEquals or coercive comparison is used.
			outcomeMatched := got == want
			if outcomeMatched {
				outcomeMatches++
				apiOutcomeMatches++
				if !diagnosticsMatch {
					reason = "L01: native compile/metadata diagnostic text differs"
				}
			}
			matched := outcomeMatched && diagnosticsMatch
			status := "MISMATCH"
			if matched {
				status, reason = "MATCH", ""
				matches++
				apiMatches++
				if c.Kind == "compile" {
					compileMatches++
				} else {
					domMatches++
				}
			}
			total++
			fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, c.Kind, status, l14Cell(got), l14Cell(want), l14Cell(reason), l14Cell(gotDiagnostics), l14Cell(wantDiagnostics), diagnosticStatus)
			if !capture {
				if !outcomeMatched {
					t.Errorf("%s API %s expected <%s> actual <%s>: %s", c.ID, api, want, got, l14Cell(reason))
				}
				if !diagnosticsMatch {
					t.Errorf("%s API %s expected diagnostics <%s> actual <%s>", c.ID, api, wantDiagnostics, gotDiagnostics)
				}
			}
		}
		fmt.Fprintf(&report, "API_TOTAL\t%s\t%d/%d\nAPI_COMPILE_TOTAL\t%s\t%d/%d\nAPI_DOM_TOTAL\t%s\t%d/%d\n", api, apiMatches, len(cases), api, compileMatches, len(compileCases), api, domMatches, len(runtimeCases)+len(vfCases))
		fmt.Fprintf(&report, "API_OUTCOME_TOTAL\t%s\t%d/%d\nAPI_REJECTION_DIAGNOSTIC_TOTAL\t%s\t%d/%d\n", api, apiOutcomeMatches, len(cases), api, apiDiagnosticMatches, apiDiagnosticTotal)
		t.Logf("L14 API %s exact matches %d/%d; compile/metadata %d/%d; native-vs-local DOM %d/%d; outcomes %d/%d; rejection diagnostics %d/%d", api, apiMatches, len(cases), compileMatches, len(compileCases), domMatches, len(runtimeCases)+len(vfCases), apiOutcomeMatches, len(cases), apiDiagnosticMatches, apiDiagnosticTotal)
	}
	fmt.Fprintf(&report, "TOTAL\t%d/%d\nOUTCOME_TOTAL\t%d/%d\nREJECTION_DIAGNOSTIC_TOTAL\t%d/%d\n", matches, total, outcomeMatches, total, diagnosticMatches, diagnosticTotal)
	t.Logf("L14 exact matches %d/%d; outcomes %d/%d; rejection diagnostics %d/%d", matches, total, outcomeMatches, total, diagnosticMatches, diagnosticTotal)
	if path := os.Getenv("GLADE_L14_REPORT"); path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(report.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func l14CompileRows(t *testing.T, api string, cases []l14Case) map[string]compile.BundleResult {
	t.Helper()
	root := t.TempDir()
	paths := map[string]string{}
	for index, c := range cases {
		var input l14Input
		if err := json.Unmarshal(c.Input, &input); err != nil {
			t.Fatal(err)
		}
		name := fmt.Sprintf("familyL14%03d", index)
		rel := filepath.Join("cases", fmt.Sprintf("%04d", index), "force-app", "main", "default", "lwc", name)
		paths[c.ID] = filepath.ToSlash(rel)
		bundle := filepath.Join(root, rel)
		js := strings.ReplaceAll(input.JS, "FamilyNavigation", "FamilyL14"+fmt.Sprintf("%03d", index))
		l14Write(t, filepath.Join(bundle, name+".js"), []byte(js))
		l14Write(t, filepath.Join(bundle, name+".html"), []byte(input.Template))
		fragment := input.MetaFragment
		if fragment == "" {
			fragment = "<isExposed>false</isExposed>"
		}
		l14Write(t, filepath.Join(bundle, name+".js-meta.xml"), []byte(`<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion>`+fragment+`</LightningComponentBundle>`))
	}
	l14Write(t, filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"cases","default":true}],"namespace":"","sourceApiVersion":"`+api+`"}`))
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := compile.CompileBatch(p, compile.Options{OutDir: filepath.Join(root, "dist"), Namespace: "c"})
	if err != nil {
		t.Fatal(err)
	}
	results := map[string]compile.BundleResult{}
	for id, path := range paths {
		result, ok := compiled[path]
		if !ok {
			t.Fatalf("L14 batch omitted %s", id)
		}
		if result.Err != nil {
			for _, infrastructure := range []string{"Cannot find module", "ERR_MODULE_NOT_FOUND", "decode compile result:", "could not find glade", "signal: killed"} {
				if strings.Contains(result.Err.Error(), infrastructure) {
					t.Fatalf("L14 compiler environment failure: %v", result.Err)
				}
			}
		}
		results[id] = result
	}
	return results
}

func l14ObserveDOM(t *testing.T, api string, cases []l14Case, dependencyRoot string, visualforce bool) map[string]string {
	t.Helper()
	root := t.TempDir()
	fixture := "l14_runtime"
	if visualforce {
		fixture = "l14_vf_runtime"
	}
	source := filepath.Join("testdata", fixture, "api"+strings.TrimSuffix(api, ".0"))
	if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		l14Write(t, filepath.Join(root, rel), data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.LWCFiles) != 1 || len(p.LWCHTMLFiles) != 1 || len(p.LWCMetaFiles) != 1 {
		t.Fatalf("L14 API %s runtime fixture requires one component: discovered %d JS, %d HTML, %d metadata files", api, len(p.LWCFiles), len(p.LWCHTMLFiles), len(p.LWCMetaFiles))
	}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	host := server.NewWithSource(&org, server.SourceMetadata{Project: p})
	if visualforce {
		host.VisualforceHTMLUserID = storage.ID("005000000000001")
	}
	defer host.ResetLightningCache()
	local := httptest.NewServer(host)
	defer local.Close()
	// Use the real tab or Visualforce Lightning Out host and query parsing.
	// Do not inject page references, emit wire values, replace navigation
	// or simulate history.
	rows := []map[string]string{}
	ids := map[string]string{}
	for _, c := range cases {
		var input l14Input
		if err := json.Unmarshal(c.Input, &input); err != nil {
			t.Fatal(err)
		}
		query := url.Values{"c__case": {c.ID}}
		for key, value := range input.InitialState {
			query.Set(key, value)
		}
		id, route, selector := c.ID, "/lwc/preview/tab/FamilyL14Oracle", "[data-l14-host]"
		if visualforce {
			if input.NativeID == "" {
				t.Fatalf("missing native VF id for %s", c.ID)
			}
			id, route, selector = input.NativeID, "/apex/FamilyL14VFHost", "[data-l14-vf-control]"
			query = url.Values{"l14case": {id}}
		}
		ids[id] = c.ID
		rows = append(rows, map[string]string{"id": id, "selector": selector, "url": local.URL + route + strings.TrimSuffix(api, ".0") + "?" + query.Encode()})
	}
	// The tab host compiles its project lazily. Complete that real request
	// before Playwright starts so their Node subprocesses run sequentially.
	// A failed host warmup
	// is an infrastructure failure, never a native navigation mismatch.
	response, err := local.Client().Get(rows[0]["url"])
	if err != nil {
		t.Fatalf("L14 API %s tab host warmup: %v", api, err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatalf("L14 API %s tab host response: %v", api, err)
	}
	if response.StatusCode != 200 {
		t.Fatalf("L14 API %s tab host HTTP %d: %s", api, response.StatusCode, body)
	}
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(dependencyRoot, "lwcruntime", "node_modules", "playwright")
	}
	observer, err := filepath.Abs("testdata/l14_browser.mjs")
	if err != nil {
		t.Fatal(err)
	}
	config := l14JSON(t, map[string]any{"rows": rows, "origin": local.URL, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", observer)
	cmd.Stdin = strings.NewReader(config)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("L14 local browser environment: %v: %s", err, stderr.String())
	}
	var observed struct {
		Values map[string]string `json:"values"`
	}
	if err := json.Unmarshal(output, &observed); err != nil {
		t.Fatal(err)
	}
	values := make(map[string]string, len(observed.Values))
	for id, value := range observed.Values {
		if strings.HasPrefix(value, "BROWSER|") {
			// Match the captured TSV serialization only: sorted JSON keys and
			// ASCII escapes. Every field, scalar type, case and null is retained.
			var raw any
			decoder := json.NewDecoder(strings.NewReader(strings.TrimPrefix(value, "BROWSER|")))
			decoder.UseNumber()
			if err := decoder.Decode(&raw); err != nil {
				t.Fatalf("L14 invalid browser JSON for %s: %v", id, err)
			}
			var canonical strings.Builder
			for _, r := range l14JSON(t, raw) {
				if r < 0x7f {
					canonical.WriteRune(r)
				} else if r <= 0xffff {
					fmt.Fprintf(&canonical, `\u%04x`, r)
				} else {
					hi, lo := utf16.EncodeRune(r)
					fmt.Fprintf(&canonical, `\u%04x\u%04x`, hi, lo)
				}
			}
			value = "BROWSER|" + canonical.String()
		}
		if ids[id] == "" {
			t.Fatalf("unexpected L14 browser row %s", id)
		}
		values[ids[id]] = value
	}
	return values
}

func l14Write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func l14JSON(t *testing.T, value any) string {
	t.Helper()
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(out.String(), "\n")
}

func l14Cell(value string) string {
	return strings.NewReplacer("\t", `\t`, "\r", `\r`, "\n", `\n`).Replace(value)
}
