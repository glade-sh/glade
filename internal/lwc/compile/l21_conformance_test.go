package compile_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
)

type l21Case struct {
	ID           string            `json:"id"`
	Group        string            `json:"group"`
	Kind         string            `json:"kind"`
	Basis        string            `json:"basis"`
	BundleName   string            `json:"bundle_name"`
	JS           string            `json:"js"`
	Template     string            `json:"template"`
	MetaFragment string            `json:"meta_fragment"`
	Actions      json.RawMessage   `json:"actions"`
	Expected     map[string]string `json:"expected"`
	RawExpected  map[string]string `json:"raw_expected"`
}

type l21Bundle struct {
	Name     string `json:"name"`
	JS       string `json:"js"`
	Template string `json:"template"`
	Metadata string `json:"metadata"`
}

type l21Table struct {
	FixtureSchemaVersion  int                              `json:"fixture_schema_version"`
	ActionTargetText      bool                             `json:"action_target_text"`
	ObservationAttributes []string                         `json:"observation_attributes"`
	ObservationNote       string                           `json:"observation_note"`
	OriginalDOMSurface    bool                             `json:"original_dom_surface"`
	Cases                 []l21Case                        `json:"cases"`
	RuntimeBundles        map[string]l21Bundle             `json:"runtime_bundles"`
	NativeDiagnostics     map[string][]l21NativeDiagnostic `json:"native_diagnostics"`
}

type l21NativeDiagnostic struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

// TestL21SalesforceConformance compares the exact exported org.tsv text at
// source API 59/67. The committed native oracle has 260 rows at each endpoint:
// 53 compiler/metadata outcomes and 207 full native DOM observations.
// GLADE_L21_CAPTURE=1 records the unchanged product's before behavior without
// failing mismatches; GLADE_L21_REPORT selects its per-row TSV destination.
// Compiler rows compare their native COMPILE_OK/COMPILE_ERROR verdicts. Every
// rejection also compares the exact native diagnostic in separate report fields;
// compiler bisection evidence remains in the export.
// The isolated API 59/67 unknown-target control supersedes the original failed
// deployment batch's component-success entry; both observations remain exported.
func TestL21SalesforceConformance(t *testing.T) {
	// Expose the browser dependency in the subtest selector used for retries.
	t.Run("BrowserRuntime", func(t *testing.T) {
		l21SalesforceConformance(t)
		t.Run("LegacyControls", l21LegacyControls)
		t.Run("RichTextControls", l21RichTextControls)
		t.Run("RichTextSrcControls", l21RichTextSrcControls)
		t.Run("TabRemovalControls", l21TabRemovalControls)
	})
}

// Replay the exact captured legacy fixtures separately, preserving the original
// 260-row denominator. The complete DOM, event order, flags and actions are
// compared at both endpoints, including canceled menu/navigation interactions.
func l21LegacyControls(t *testing.T) {
	l21CapturedControls(t, "legacy", "familyL21LegacyRuntime", 3, true)
}

// Replay the full native rich-text observations, including their additional
// attribute surface. Image src was not observed by this capture; these rows
// assert its other fields; the separate image controls below observe src.
func l21RichTextControls(t *testing.T) {
	l21CapturedControls(t, "rich_text", "familyL21RichControlsRuntime", 27, false)
}

func l21RichTextSrcControls(t *testing.T) {
	l21CapturedControls(t, "rich_text_src", "familyL21RichSrcControlsRuntime", 9, false)
}

func l21TabRemovalControls(t *testing.T) {
	l21CapturedControls(t, "tab_removal", "familyL21RichSrcControlsRuntime", 2, false)
}

func l21CapturedControls(t *testing.T, name, bundlePrefix string, count int, actionTargetText bool) {
	t.Helper()
	data, err := os.ReadFile("testdata/l21_" + name + "_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var table l21Table
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if table.FixtureSchemaVersion != 1 || len(table.Cases) != count || table.ActionTargetText != actionTargetText {
		t.Fatalf("invalid native %s control export", name)
	}
	if table.OriginalDOMSurface && (name != "tab_removal" || len(table.ObservationAttributes) != 0) {
		t.Fatal("only tab-removal controls use the original L21 DOM surface")
	}
	if table.ObservationNote != "" {
		t.Log(table.ObservationNote)
	}
	seen := map[string]bool{}
	for _, c := range table.Cases {
		if c.ID == "" || seen[c.ID] || c.Kind != "runtime" || c.Basis != "org" {
			t.Fatalf("invalid or duplicate native %s control %q", name, c.ID)
		}
		seen[c.ID] = true
	}
	dependencyRoot, err := gladehome.Root()
	if err != nil {
		t.Fatal(err)
	}
	capture := os.Getenv("GLADE_L21_CAPTURE") == "1"
	var report strings.Builder
	for _, api := range []string{"59.0", "67.0"} {
		bundle := table.RuntimeBundles[api]
		if bundle.Name != bundlePrefix+api[:2] || bundle.JS == "" || bundle.Template == "" || bundle.Metadata == "" {
			t.Fatalf("missing native %s runtime bundle at API %s", name, api)
		}
		values, errors, compileErr := l21ObserveDOM(t, table, api, dependencyRoot)
		if compileErr != nil {
			t.Fatal(compileErr)
		}
		matches := 0
		for _, c := range table.Cases {
			want := c.Expected[api]
			if !strings.HasPrefix(want, "BROWSER|") || !json.Valid([]byte(strings.TrimPrefix(want, "BROWSER|"))) {
				t.Fatalf("invalid native %s row %s at API %s", name, c.ID, api)
			}
			if table.OriginalDOMSurface {
				projected, err := l21OriginalDOMText(c.RawExpected[api])
				if err != nil || projected != want {
					t.Fatalf("%s API %s original DOM projection does not match its untouched native source: %v", c.ID, api, err)
				}
			} else if raw := c.RawExpected[api]; raw != "" && raw != want {
				t.Fatalf("%s API %s expected text differs from the complete captured row", c.ID, api)
			}
			got, err := l21BrowserText(values[c.ID])
			if err != nil {
				t.Fatalf("missing %s observation %s at API %s: %s: %v", name, c.ID, api, errors[c.ID], err)
			}
			status := "MISMATCH"
			if got == want {
				status = "MATCH"
				matches++
			} else if !capture {
				t.Errorf("%s API %s expected <%s> actual <%s>", c.ID, api, want, got)
			}
			fmt.Fprintf(&report, "%s\t%s\truntime\t%s\t%s\t%s\t\t\t\t\n", api, c.ID, status, l21Cell(got), l21Cell(want))
		}
		t.Logf("L21 %s controls API %s matches %d/%d", name, api, matches, count)
		fmt.Fprintf(&report, "%s_CONTROL_TOTAL\t%s\t%d/%d\n", strings.ToUpper(name), api, matches, count)
	}
	if path := os.Getenv("GLADE_L21_REPORT"); path != "" {
		file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if _, err := file.WriteString(report.String()); err != nil {
			t.Fatal(err)
		}
	}
}

// The follow-up observer expanded attributes for the image URL questions. Tab
// removal retains the original native DOM surface rather than adding inferred
// hosted IDs or layout policy. Full native answers remain in raw_expected;
// verify the exported projection without changing any observed field values.
func l21OriginalDOMText(raw string) (string, error) {
	if !strings.HasPrefix(raw, "BROWSER|") {
		return "", fmt.Errorf("missing full native browser answer")
	}
	var value map[string]any
	decoder := json.NewDecoder(strings.NewReader(strings.TrimPrefix(raw, "BROWSER|")))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	attributes := map[string]bool{}
	for _, name := range []string{"class", "role", "title", "name", "type", "value", "disabled", "checked", "href", "target", "alt", "tabindex", "aria-label", "aria-selected", "aria-expanded", "aria-hidden", "aria-valuemin", "aria-valuemax", "aria-valuenow", "aria-valuetext"} {
		attributes[name] = true
	}
	var projectDOM func(any)
	projectDOM = func(node any) {
		switch node := node.(type) {
		case map[string]any:
			if attrs, ok := node["attrs"].(map[string]any); ok {
				for name := range attrs {
					if !attributes[name] {
						delete(attrs, name)
					}
				}
			}
			for _, child := range node {
				projectDOM(child)
			}
		case []any:
			for _, child := range node {
				projectDOM(child)
			}
		}
	}
	projectDOM(value["renderedBefore"])
	projectDOM(value["renderedAfter"])
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return l21BrowserText(encoded)
}

func l21SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_L21_CAPTURE") == "1"
	data, err := os.ReadFile("testdata/l21_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table l21Table
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	versions := []string{"59.0", "67.0"}
	seen, groups := map[string]bool{}, map[string]bool{}
	compileCount, runtimeCount := 0, 0
	for _, c := range table.Cases {
		if c.ID == "" || seen[c.ID] || c.Basis != "org" || c.BundleName == "" {
			t.Fatalf("invalid or duplicate L21 row %q", c.ID)
		}
		seen[c.ID], groups[c.Group] = true, true
		switch c.Kind {
		case "compile":
			compileCount++
			if c.JS == "" || c.Template == "" {
				t.Fatalf("missing L21 source input for %s", c.ID)
			}
		case "runtime":
			runtimeCount++
		default:
			t.Fatalf("invalid L21 kind for %s: %s", c.ID, c.Kind)
		}
		for _, api := range versions {
			want, present := c.Expected[api]
			if !present || (c.Kind == "compile" && want != "COMPILE_OK" && want != "COMPILE_ERROR") ||
				(c.Kind == "runtime" && (!strings.HasPrefix(want, "BROWSER|") || !json.Valid([]byte(strings.TrimPrefix(want, "BROWSER|"))))) {
				t.Fatalf("missing or invalid native L21 answer: %s API %s", c.ID, api)
			}
		}
	}
	if len(table.Cases) != 260 || compileCount != 53 || runtimeCount != 207 {
		t.Fatalf("L21 needs all 53 compile/metadata + 207 DOM rows, got %d + %d", compileCount, runtimeCount)
	}
	for _, group := range []string{"formatted values", "actions and icons", "containers and navigation", "progress and media"} {
		if !groups[group] {
			t.Fatalf("missing L21 group %s", group)
		}
	}
	if table.FixtureSchemaVersion != 1 {
		t.Fatal("invalid native fixture schema version")
	}
	for _, api := range versions {
		bundle := table.RuntimeBundles[api]
		if bundle.Name != "familyL21Runtime"+api[:2] || bundle.JS == "" || bundle.Template == "" || bundle.Metadata == "" {
			t.Fatalf("missing native L21 runtime fixture at API %s", api)
		}
	}

	// Infrastructure failures must never count as native compiler rejections.
	dependencyRoot, err := gladehome.Root()
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
	report.WriteString("api\tid\tkind\tstatus\tactual\texpected\treason\tdiagnostic_status\tdiagnostic_actual\tdiagnostic_expected\n")
	matches, total := 0, 0
	for _, api := range versions {
		compiled := l21CompileCases(t, table.Cases, api)
		dom, domErrors, runtimeCompileErr := l21ObserveDOM(t, table, api, dependencyRoot)
		nativeDiagnostics := map[string]string{}
		for _, diagnostic := range table.NativeDiagnostics[api] {
			if !seen[diagnostic.ID] || diagnostic.Message == "" || nativeDiagnostics[diagnostic.ID] != "" {
				t.Fatalf("invalid or duplicate native L21 diagnostic %s at API %s", diagnostic.ID, api)
			}
			nativeDiagnostics[diagnostic.ID] = diagnostic.Message
		}
		if len(nativeDiagnostics) != 9 {
			t.Fatalf("L21 needs nine native rejection diagnostics at API %s, got %d", api, len(nativeDiagnostics))
		}
		apiMatches, compileMatches, domMatches, domRows := 0, 0, 0, 0
		diagnosticMatches, diagnosticTotal := 0, 0
		for _, c := range table.Cases {
			want, got, reason := c.Expected[api], "COMPILE_OK", ""
			diagnosticStatus, diagnosticGot, diagnosticWant := "", "", ""
			if c.Kind == "compile" {
				result := compiled[c.ID]
				if err := result.Err; err != nil {
					got = "COMPILE_ERROR"
					reason = "L21: local compiler rejected source: " + err.Error()
				}
				if want == "COMPILE_ERROR" {
					diagnosticWant = nativeDiagnostics[c.ID]
					if diagnosticWant == "" {
						t.Fatalf("missing native L21 rejection diagnostic %s at API %s", c.ID, api)
					}
					diagnosticGot = l21CompileDiagnostic(result)
					diagnosticStatus = "MISMATCH"
					diagnosticTotal++
					if diagnosticGot == diagnosticWant {
						diagnosticStatus = "MATCH"
						diagnosticMatches++
					} else {
						t.Logf("L21 unresolved diagnostic %s API %s expected <%s> actual <%s>", c.ID, api, diagnosticWant, diagnosticGot)
						if !capture {
							t.Errorf("%s API %s diagnostic expected <%s> actual <%s>", c.ID, api, diagnosticWant, diagnosticGot)
						}
					}
				}
				if got != want && reason == "" {
					reason = "L21: local compiler accepted native-rejected source"
				}
			} else if raw, present := dom[c.ID]; present {
				got, err = l21BrowserText(raw)
				if err != nil {
					t.Fatalf("invalid local DOM answer for %s: %v", c.ID, err)
				}
				domRows++
				if got != want {
					reason = "L21: local display/layout DOM, public property or event differs from native"
				}
			} else {
				got = "DOM_ERROR"
				reason = "L21: local DOM observation missing: " + domErrors[c.ID]
				if runtimeCompileErr != nil {
					got = "COMPILE_ERROR"
					reason = "L21: runtime fixture compile failed: " + runtimeCompileErr.Error()
				}
			}
			// Exact Go text equality preserves case, Id length and raw JSON null.
			// No assertEquals/== Apex route participates in the row comparison.
			status := "MISMATCH"
			if got == want {
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
			fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, c.Kind, status, l21Cell(got), l21Cell(want), l21Cell(reason), diagnosticStatus, l21Cell(diagnosticGot), l21Cell(diagnosticWant))
			if !capture && status == "MISMATCH" {
				t.Errorf("%s API %s expected <%s> actual <%s>: %s", c.ID, api, want, got, l21Cell(reason))
			}
		}
		fmt.Fprintf(&report, "API_TOTAL\t%s\t%d/260\nCOMPILE_TOTAL\t%s\t%d/53\nDOM_TOTAL\t%s\t%d/207\nLOCAL_DOM_ROWS\t%s\t%d/207\n", api, apiMatches, api, compileMatches, api, domMatches, api, domRows)
		t.Logf("L21 API %s matches %d/260; compile/metadata %d/53; DOM %d/207; local DOM observations %d/207", api, apiMatches, compileMatches, domMatches, domRows)
		fmt.Fprintf(&report, "DIAGNOSTIC_TOTAL\t%s\t%d/%d\n", api, diagnosticMatches, diagnosticTotal)
		t.Logf("L21 API %s exact rejection diagnostics %d/%d", api, diagnosticMatches, diagnosticTotal)
	}
	fmt.Fprintf(&report, "TOTAL\t%d/%d\n", matches, total)
	t.Logf("L21 matches %d/%d", matches, total)
	if path := os.Getenv("GLADE_L21_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func l21CompileDiagnostic(result compile.BundleResult) string {
	if result.Err == nil {
		return ""
	}
	if len(result.DeploymentDiagnostics) != 0 {
		return strings.Join(result.DeploymentDiagnostics, "\n")
	}
	if len(result.Diagnostics) != 0 {
		messages := make([]string, 0, len(result.Diagnostics))
		for _, diagnostic := range result.Diagnostics {
			messages = append(messages, diagnostic.Message)
		}
		return strings.Join(messages, "\n")
	}
	if cause := errors.Unwrap(result.Err); cause != nil {
		return cause.Error()
	}
	return result.Err.Error()
}

func l21Write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func l21WriteBundle(t *testing.T, root string, bundle l21Bundle) {
	t.Helper()
	path := filepath.Join(root, "force-app", "main", "default", "lwc", bundle.Name, bundle.Name)
	l21Write(t, path+".js", bundle.JS)
	l21Write(t, path+".html", bundle.Template)
	l21Write(t, path+".js-meta.xml", bundle.Metadata)
}

func l21CompileCases(t *testing.T, cases []l21Case, api string) map[string]compile.BundleResult {
	t.Helper()
	root := t.TempDir()
	paths := map[string]string{}
	for index, c := range cases {
		if c.Kind != "compile" {
			continue
		}
		caseRoot := filepath.Join(root, "cases", fmt.Sprintf("%04d", index))
		fragment := c.MetaFragment
		if fragment == "" {
			fragment = "<isExposed>false</isExposed>"
		}
		metadata := `<?xml version="1.0" encoding="UTF-8"?><LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion>` + fragment + `</LightningComponentBundle>`
		js := strings.ReplaceAll(c.JS, "FamilyDisplayLayout", strings.ToUpper(c.BundleName[:1])+c.BundleName[1:])
		l21WriteBundle(t, caseRoot, l21Bundle{Name: c.BundleName, JS: js, Template: c.Template, Metadata: metadata})
		path, err := filepath.Rel(root, filepath.Join(caseRoot, "force-app", "main", "default", "lwc", c.BundleName))
		if err != nil {
			t.Fatal(err)
		}
		paths[c.ID] = filepath.ToSlash(path)
	}
	l21Write(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"cases","default":true}],"namespace":"","sourceApiVersion":"`+api+`"}`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := compile.CompileBatch(p, compile.Options{OutDir: filepath.Join(root, "dist")})
	if err != nil {
		t.Fatal(err)
	}
	results := map[string]compile.BundleResult{}
	for id, path := range paths {
		result, present := batch[path]
		if !present {
			t.Fatalf("missing L21 compiler result for %s", id)
		}
		results[id] = result
	}
	return results
}

func l21ObserveDOM(t *testing.T, table l21Table, api, dependencyRoot string) (map[string]json.RawMessage, map[string]string, error) {
	t.Helper()
	root := t.TempDir()
	bundle := table.RuntimeBundles[api]
	l21WriteBundle(t, root, bundle)
	l21Write(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":"`+api+`"}`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := compile.Compile(p, compile.Options{OutDir: filepath.Join(root, "dist")})
	if err != nil {
		return nil, nil, err
	}
	entry, present := compiled.Modules["c:"+bundle.Name]
	if !present {
		t.Fatalf("missing L21 runtime entry for API %s", api)
	}
	imports := lwcbrowser.SalesforceImportMap()
	imports["lwc"], imports["@lwc/synthetic-shadow"] = "/engine.js", "/shadow.js"
	for _, module := range compiled.Modules {
		rel, err := filepath.Rel(compiled.OutDir, module.File)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("L21 compiled module escapes its output: %s", module.File)
		}
		imports[module.ModuleKey] = "/modules/" + filepath.ToSlash(rel)
	}
	importMap, err := json.Marshal(map[string]any{"imports": imports})
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	registry, err := resource.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org.Metadata = registry
	product := server.NewWithSource(&org, server.SourceMetadata{Project: p})
	mux := http.NewServeMux()
	// Use the product handlers for base components, their runtime assets and
	// Salesforce imports; no component implementation is replaced by the test.
	mux.Handle("/lightning/", product)
	mux.Handle("/modules/", http.StripPrefix("/modules/", http.FileServer(http.Dir(compiled.OutDir))))
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
		pageReference := map[string]any{"type": "standard__navItemPage", "attributes": map[string]string{"apiName": "FamilyL21Oracle" + api[:2]}, "state": map[string]string{"c__case": r.URL.Query().Get("c__case")}}
		pageConfig, _ := json.Marshal(map[string]any{"pageReference": pageReference})
		moduleName, _ := json.Marshal("c/" + bundle.Name)
		tag, _ := json.Marshal(entry.Tag)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// These locale/time-zone inputs reproduce the captured user's context.
		// Native expected rows are never included in the page or browser config.
		fmt.Fprintf(w, `<!doctype html><html lang="en-US"><head><script>window.process={env:{NODE_ENV:"production"}};window.__l21Error=null;window.addEventListener("error",e=>{window.__l21Error={name:e.error?.name||"Error",message:e.message};});</script><script id="glade-lightning-config" type="application/json">%s</script><script id="glade-lwc-context" type="application/json">{"i18n":{"timeZone":"America/Los_Angeles"}}</script><script type="importmap">%s</script></head><body><main></main><script type="module">try{await import("@lwc/synthetic-shadow");const {createElement}=await import("lwc");const {default:Page}=await import(%s);document.querySelector("main").appendChild(createElement(%s,{is:Page}));}catch(e){window.__l21Error={name:e.name,message:e.message};}</script></body></html>`, pageConfig, importMap, moduleName, tag)
	})
	local := httptest.NewServer(mux)
	defer local.Close()
	rows := []map[string]any{}
	for _, c := range table.Cases {
		if c.Kind == "runtime" {
			rows = append(rows, map[string]any{"id": c.ID, "actions": c.Actions})
		}
	}
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(dependencyRoot, "lwcruntime", "node_modules", "playwright")
	}
	config, err := json.Marshal(map[string]any{"url": local.URL, "rows": rows, "actionTargetText": table.ActionTargetText, "observationAttributes": table.ObservationAttributes, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")})
	if err != nil {
		t.Fatal(err)
	}
	observer, err := filepath.Abs("testdata/l21_browser.mjs")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", observer)
	cmd.Stdin = bytes.NewReader(config)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("L21 local browser environment: %v: %s", err, l21Cell(stderr.String()))
	}
	var observation struct {
		Values map[string]json.RawMessage `json:"values"`
		Errors map[string]string          `json:"errors"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &observation); err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, c := range table.Cases {
		if c.Kind == "runtime" {
			known[c.ID] = true
		}
	}
	for _, ids := range []map[string]bool{l21ObservedIDs(observation.Values), l21ObservedIDs(observation.Errors)} {
		for id := range ids {
			if !known[id] {
				t.Fatalf("unexpected L21 browser row %s", id)
			}
		}
	}
	for id := range known {
		_, valuePresent := observation.Values[id]
		_, errorPresent := observation.Errors[id]
		if valuePresent == errorPresent {
			t.Fatalf("L21 browser must return one value or error for %s", id)
		}
	}
	return observation.Values, observation.Errors, nil
}

func l21ObservedIDs[T any](rows map[string]T) map[string]bool {
	ids := map[string]bool{}
	for id := range rows {
		ids[id] = true
	}
	return ids
}

// Match Python's sorted, compact, ensure_ascii native serialization. This only
// orders object keys and escapes text: all fields, array order, case, numeric
// text and null values remain part of the exact row comparison.
func l21BrowserText(raw json.RawMessage) (string, error) {
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
		switch {
		case r < 0x7f:
			text.WriteRune(r)
		case r <= 0xffff:
			fmt.Fprintf(&text, `\u%04x`, r)
		default:
			hi, lo := utf16.EncodeRune(r)
			fmt.Fprintf(&text, `\u%04x\u%04x`, hi, lo)
		}
	}
	return text.String(), nil
}

func l21Cell(text string) string {
	return strings.NewReplacer("\t", `\t`, "\r", `\r`, "\n", `\n`).Replace(text)
}
