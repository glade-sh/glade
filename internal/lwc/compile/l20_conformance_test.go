package compile_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
)

type l20Case struct {
	ID                string              `json:"id"`
	Group             string              `json:"group"`
	Kind              string              `json:"kind"`
	Basis             string              `json:"basis"`
	JS                string              `json:"js"`
	Template          string              `json:"template"`
	MetaFragment      string              `json:"meta_fragment"`
	Fixture           json.RawMessage     `json:"fixture"`
	Expected          map[string]string   `json:"expected"`
	NativeDiagnostics map[string][]string `json:"native_diagnostics"`
	NativeError       bool                `json:"native_error"`
}

// TestL20SalesforceConformance replays the owned inputs and exact org.tsv text
// exported from the API 59/67 data presentation capture.
// An individual native control supersedes meta_bad_target with identical
// inputs and captured rejection text.
// The capture records 41 compiler/metadata and 161 browser observations at each
// API. The null-location mount error is retained and counted separately from
// the 201 ordinary rows. No observed fields, including raw null, are removed.
// Compiler diagnostics from diagnostics.json are compared separately as exact
// text, preserving native positions, deployment envelopes and API echoes.
// Documentation URLs use the native capture's identical URL redaction.
// GLADE_L20_CAPTURE=1 reports differences without failing mismatched rows;
// GLADE_L20_REPORT optionally writes the complete per-row TSV and counts.
// Browser execution belongs to the campaign test host, never Salesforce/CI auth.
func TestL20SalesforceConformance(t *testing.T) {
	// Keep the browser requirement explicit in scoped campaign test selections.
	t.Run("Chromium", l20SalesforceConformance)
}

func l20SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_L20_CAPTURE") != ""
	data, err := os.ReadFile("testdata/l20_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []l20Case
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	versions := []string{"59.0", "67.0"}
	seen, groups := map[string]bool{}, map[string]bool{}
	var compilerCases, runtimeCases []l20Case
	nativeErrors := 0
	for _, c := range cases {
		if c.ID == "" || seen[c.ID] || c.Basis != "org" || (c.Kind != "compile" && c.Kind != "runtime") {
			t.Fatalf("invalid or duplicate L20 row %q", c.ID)
		}
		seen[c.ID], groups[c.Group] = true, true
		for _, api := range versions {
			want, ok := c.Expected[api]
			if !ok || want == "" {
				t.Fatalf("missing native L20 answer: %s API %s", c.ID, api)
			}
			if c.Kind == "compile" {
				if want != "COMPILE_OK" && want != "COMPILE_ERROR" {
					t.Fatalf("invalid native compile answer: %s API %s", c.ID, api)
				}
				diagnostics, ok := c.NativeDiagnostics[api]
				if !ok || diagnostics == nil || (want == "COMPILE_ERROR" && len(diagnostics) == 0) {
					t.Fatalf("missing native compiler diagnostic text: %s API %s", c.ID, api)
				}
			} else {
				if !strings.HasPrefix(want, "BROWSER|") {
					t.Fatalf("missing native browser answer: %s API %s", c.ID, api)
				}
				var native map[string]json.RawMessage
				if err := json.Unmarshal([]byte(strings.TrimPrefix(want, "BROWSER|")), &native); err != nil {
					t.Fatal(err)
				}
				_, hasError := native["nativeError"]
				if hasError != c.NativeError {
					t.Fatalf("native error classification differs: %s API %s", c.ID, api)
				}
			}
		}
		if c.Kind == "compile" {
			if c.JS == "" || c.Template == "" || c.NativeError {
				t.Fatalf("incomplete L20 compiler input: %s", c.ID)
			}
			compilerCases = append(compilerCases, c)
		} else {
			var input map[string]json.RawMessage
			if err := json.Unmarshal(c.Fixture, &input); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"id", "group", "kind", "operation", "category"} {
				if len(input[key]) == 0 {
					t.Fatalf("missing L20 browser input %s: %s", key, c.ID)
				}
			}
			// An oracle must never enter the local application or browser driver.
			for _, key := range []string{"expected", "runtime_expected", "native_error"} {
				if _, ok := input[key]; ok {
					t.Fatalf("L20 browser input contains oracle field %s: %s", key, c.ID)
				}
			}
			runtimeCases = append(runtimeCases, c)
			if c.NativeError {
				nativeErrors++
			}
		}
	}
	if len(cases) != 202 || len(compilerCases) != 41 || len(runtimeCases) != 161 || nativeErrors != 1 {
		t.Fatalf("L20 needs 41 compiler + 161 browser rows, including one native error; got %d/%d/%d", len(compilerCases), len(runtimeCases), nativeErrors)
	}
	for _, group := range []string{"datatable", "tree", "tree-grid", "dual-listbox", "lookup-style-lists"} {
		if !groups[group] {
			t.Fatalf("missing L20 group %s", group)
		}
	}

	// Infrastructure errors must not be mistaken for native compiler rejections.
	dependencyRoot, err := gladehome.EnsureRoot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"third_party/lwc/node_modules/@lwc/compiler/package.json",
		"third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js",
		"third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js",
	} {
		if _, err := os.Stat(filepath.Join(dependencyRoot, filepath.FromSlash(rel))); err != nil {
			t.Fatal(err)
		}
	}

	var report strings.Builder
	fmt.Fprintln(&report, "api\tid\tkind\tscope\tstatus\tactual\texpected\treason")
	matches, total, errorMatches, errorTotal := 0, 0, 0, 0
	diagnosticMatches, diagnosticTotal := 0, 0
	for _, api := range versions {
		compiled := l20CompileCases(t, api, compilerCases)
		values, errors, runtimeCompileErr, browserErr := l20ObserveDOM(t, api, runtimeCases, dependencyRoot)
		if browserErr != nil {
			t.Fatal(browserErr)
		}
		apiMatches, apiErrorMatches, compileIndex := 0, 0, 0
		for _, c := range cases {
			want, got, reason := c.Expected[api], "", ""
			if c.Kind == "compile" {
				result := compiled[compileIndex]
				compileErr := result.Err
				compileIndex++
				got = "COMPILE_OK"
				if compileErr != nil {
					got = "COMPILE_ERROR"
				}
				if got != want {
					reason = "L20: source compile differs from native"
					if compileErr != nil {
						reason += ": " + compileErr.Error()
					}
				}
				diagnosticWant, diagnosticGot := l20DiagnosticText(t, c.NativeDiagnostics[api]), l20CompileDiagnosticText(t, result)
				diagnosticStatus, diagnosticReason := "MATCH", ""
				diagnosticTotal++
				if diagnosticGot == diagnosticWant {
					diagnosticMatches++
				} else {
					diagnosticStatus, diagnosticReason = "MISMATCH", "L20: local compiler diagnostic text differs from native; unresolved, no carry"
				}
				fmt.Fprintf(&report, "%s\t%s\tcompile\tdiagnostic\t%s\t%s\t%s\t%s\n", api, c.ID, diagnosticStatus, l20Cell(diagnosticGot), l20Cell(diagnosticWant), diagnosticReason)
				if !capture && diagnosticGot != diagnosticWant {
					t.Errorf("%s API %s diagnostic expected <%s> actual <%s>", c.ID, api, diagnosticWant, diagnosticGot)
				}
			} else if runtimeCompileErr != nil {
				got, reason = "COMPILE_ERROR", "L20: runtime fixture compile failed: "+runtimeCompileErr.Error()
			} else if raw, ok := values[c.ID]; ok {
				got, err = l20BrowserText(raw)
				if err != nil {
					t.Fatalf("%s API %s invalid browser JSON: %v", c.ID, api, err)
				}
			} else {
				got, reason = "BROWSER_ERROR", "L20: local browser row missing"
				if errors[c.ID] != "" {
					reason += ": " + errors[c.ID]
				}
			}
			status, scope := "MISMATCH", "ordinary"
			if c.NativeError {
				scope = "native-error"
				errorTotal++
			} else {
				total++
			}
			if got == want { // Exact text: case, Id length, array order and raw null survive.
				status = "MATCH"
				if c.NativeError {
					errorMatches++
					apiErrorMatches++
				} else {
					matches++
					apiMatches++
				}
			} else if reason == "" {
				reason = "L20: local DOM, API return or event text differs from native"
			}
			fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, c.Kind, scope, status, l20Cell(got), l20Cell(want), l20Cell(reason))
			if !capture && got != want {
				t.Errorf("%s API %s expected <%s> actual <%s>: %s", c.ID, api, want, got, l20Cell(reason))
			}
		}
		fmt.Fprintf(&report, "API_TOTAL\t%s\t%d/201\nNATIVE_ERROR_TOTAL\t%s\t%d/1\n", api, apiMatches, api, apiErrorMatches)
		t.Logf("L20 API %s matches %d/201; native-error %d/1; all observations %d/202", api, apiMatches, apiErrorMatches, apiMatches+apiErrorMatches)
	}
	fmt.Fprintf(&report, "TOTAL\t%d/%d\nNATIVE_ERROR_TOTAL\t%d/%d\nALL_OBSERVATIONS_TOTAL\t%d/%d\n", matches, total, errorMatches, errorTotal, matches+errorMatches, total+errorTotal)
	fmt.Fprintf(&report, "DIAGNOSTIC_TOTAL\t%d/%d\n", diagnosticMatches, diagnosticTotal)
	t.Logf("L20 matches %d/%d; native-error %d/%d; all observations %d/%d", matches, total, errorMatches, errorTotal, matches+errorMatches, total+errorTotal)
	t.Logf("L20 exact compiler diagnostics %d/%d", diagnosticMatches, diagnosticTotal)
	if path := os.Getenv("GLADE_L20_REPORT"); path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(report.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

var l20DiagnosticURL = regexp.MustCompile(`https?://[^\s"'<>]+`)

func l20DiagnosticText(t *testing.T, messages []string) string {
	t.Helper()
	// diagnostics.json redacts URLs with this exact pattern. No diagnostic
	// wording, position, API echo, case or whitespace is normalized.
	redacted := make([]string, len(messages))
	for i, message := range messages {
		redacted[i] = l20DiagnosticURL.ReplaceAllString(message, "<URL>")
	}
	data, err := json.Marshal(redacted)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func l20CompileDiagnosticText(t *testing.T, result compile.BundleResult) string {
	t.Helper()
	if result.ReportedDiagnostics == nil || (result.Err != nil && len(result.ReportedDiagnostics) == 0) {
		t.Fatalf("local compiler result lacks reported diagnostics: %v", result.Err)
	}
	return l20DiagnosticText(t, result.ReportedDiagnostics)
}

func l20Write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func l20WriteBundle(t *testing.T, root, name string, c l20Case, api string) {
	t.Helper()
	bundle := filepath.Join(root, "force-app", "main", "default", "lwc", name)
	class := strings.ToUpper(name[:1]) + name[1:]
	l20Write(t, filepath.Join(bundle, name+".js"), strings.ReplaceAll(c.JS, "FamilyPresentation", class))
	l20Write(t, filepath.Join(bundle, name+".html"), c.Template)
	fragment := c.MetaFragment
	if fragment == "" {
		fragment = "<isExposed>false</isExposed>"
	}
	l20Write(t, filepath.Join(bundle, name+".js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion>`+fragment+`</LightningComponentBundle>`)
}

func l20CompileCases(t *testing.T, api string, cases []l20Case) []compile.BundleResult {
	t.Helper()
	root := t.TempDir()
	keys := make([]string, len(cases))
	for index, c := range cases {
		name := fmt.Sprintf("familyL20Compile%03d", index)
		caseRoot := filepath.Join(root, "cases", fmt.Sprintf("%04d", index))
		l20WriteBundle(t, caseRoot, name, c, api)
		keys[index] = fmt.Sprintf("cases/%04d/force-app/main/default/lwc/%s", index, name)
	}
	l20Write(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"cases","default":true}],"sourceApiVersion":"`+api+`"}`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := compile.CompileBatch(p, compile.Options{OutDir: filepath.Join(root, "dist")})
	if err != nil {
		t.Fatal(err)
	}
	if len(batch) != len(cases) {
		t.Fatalf("L20 batch returned %d/%d results", len(batch), len(cases))
	}
	results := make([]compile.BundleResult, len(cases))
	for index, key := range keys {
		result, ok := batch[key]
		if !ok {
			t.Fatalf("missing L20 compiler result %s", key)
		}
		results[index] = result
	}
	return results
}

func l20ObserveDOM(t *testing.T, api string, cases []l20Case, dependencyRoot string) (map[string]json.RawMessage, map[string]string, error, error) {
	t.Helper()
	js, err := os.ReadFile("testdata/l20_runtime.js")
	if err != nil {
		return nil, nil, nil, err
	}
	template, err := os.ReadFile("testdata/l20_runtime.html")
	if err != nil {
		return nil, nil, nil, err
	}
	specs := make([]json.RawMessage, len(cases))
	known := map[string]bool{}
	for index, c := range cases {
		specs[index], known[c.ID] = c.Fixture, true
	}
	inputs, err := json.Marshal(specs)
	if err != nil {
		return nil, nil, nil, err
	}
	root, name := t.TempDir(), "familyL20Runtime"+api[:2]
	l20WriteBundle(t, root, name, l20Case{JS: strings.ReplaceAll(string(js), "__SPECS__", string(inputs)), Template: string(template), MetaFragment: "<isExposed>true</isExposed><targets><target>lightning__Tab</target></targets>"}, api)
	l20Write(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
	p, err := project.Load(root)
	if err != nil {
		return nil, nil, nil, err
	}
	manifest, compileErr := compile.Compile(p, compile.Options{OutDir: filepath.Join(root, "dist")})
	if compileErr != nil {
		return nil, nil, compileErr, nil
	}
	entry, ok := manifest.Modules["c:"+name]
	if !ok {
		return nil, nil, nil, fmt.Errorf("L20 runtime entry missing")
	}
	imports := lwcbrowser.SalesforceImportMap()
	imports["lwc"], imports["@lwc/synthetic-shadow"] = "/engine.js", "/shadow.js"
	for _, module := range manifest.Modules {
		rel, err := filepath.Rel(manifest.OutDir, module.File)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, nil, nil, fmt.Errorf("L20 compiled module escapes output: %s", module.File)
		}
		imports[module.ModuleKey] = "/modules/" + filepath.ToSlash(rel)
	}
	importJSON, err := json.Marshal(map[string]any{"imports": imports})
	if err != nil {
		return nil, nil, nil, err
	}
	org := storage.NewOrgState()
	product := server.New(&org)
	mux := http.NewServeMux()
	// Serve the real product shims and embedded base components. The harness
	// does not supply alternate component classes, events or expected values.
	mux.Handle("/lightning/", product)
	mux.Handle("/modules/", http.StripPrefix("/modules/", http.FileServer(http.Dir(manifest.OutDir))))
	for route, rel := range map[string]string{
		"/engine.js": "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js",
		"/shadow.js": "third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js",
	} {
		file := filepath.Join(dependencyRoot, filepath.FromSlash(rel))
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			http.ServeFile(w, r, file)
		})
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		// Supply the same c__case input to the product CurrentPageReference
		// adapter that the native fixture gets from its Lightning tab URL.
		config, err := json.Marshal(map[string]any{"pageReference": map[string]any{
			"type": "standard__navItemPage", "attributes": map[string]string{"apiName": "FamilyL20Oracle" + api[:2]},
			"state": map[string]string{"c__case": r.URL.Query().Get("c__case")},
		}})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><script>window.process={env:{NODE_ENV:"production"}};window.__l20Error=null;</script><script id="glade-lightning-config" type="application/json">%s</script><script id="glade-lwc-context" type="application/json">{}</script><script type="importmap">%s</script><main></main><script type="module">try {await import("@lwc/synthetic-shadow");const {createElement}=await import("lwc");const {default:Page}=await import("c/%s");document.querySelector("main").appendChild(createElement("%s",{is:Page}));} catch(error) {window.__l20Error={name:error.name,message:error.message};}</script>`, config, importJSON, name, entry.Tag)
	})
	local := httptest.NewServer(mux)
	defer local.Close()
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(dependencyRoot, "lwcruntime", "node_modules", "playwright")
	}
	config, err := json.Marshal(map[string]any{"url": local.URL, "specs": specs, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")})
	if err != nil {
		return nil, nil, nil, err
	}
	observer, err := filepath.Abs("testdata/l20_browser.mjs")
	if err != nil {
		return nil, nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", observer)
	cmd.Stdin = bytes.NewReader(config)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("L20 local browser environment: %w: %s", err, l20Cell(stderr.String()))
	}
	var observed struct {
		Values map[string]json.RawMessage `json:"values"`
		Errors map[string]string          `json:"errors"`
	}
	if err := json.Unmarshal(output, &observed); err != nil {
		return nil, nil, nil, err
	}
	for id := range observed.Values {
		if !known[id] {
			return nil, nil, nil, fmt.Errorf("unexpected L20 browser row %s", id)
		}
	}
	for id := range observed.Errors {
		if !known[id] {
			return nil, nil, nil, fmt.Errorf("unexpected L20 browser error %s", id)
		}
	}
	return observed.Values, observed.Errors, nil, nil
}

// Match Python's sort_keys=True, separators=(',', ':'), ensure_ascii=True JSON
// serialization without altering any observed field, text, number or null.
func l20BrowserText(raw json.RawMessage) (string, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	var text strings.Builder
	text.WriteString("BROWSER|")
	for _, r := range strings.TrimSuffix(encoded.String(), "\n") {
		if r < 0x7f {
			text.WriteRune(r)
		} else if r <= 0xffff {
			fmt.Fprintf(&text, `\u%04x`, r)
		} else {
			hi, lo := utf16.EncodeRune(r)
			fmt.Fprintf(&text, `\u%04x\u%04x`, hi, lo)
		}
	}
	return text.String(), nil
}

func l20Cell(text string) string {
	return strings.NewReplacer("\t", `\t`, "\r", `\r`, "\n", `\n`).Replace(text)
}
