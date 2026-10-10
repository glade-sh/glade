package compile_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
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
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

type l10Case struct {
	ID                 string              `json:"id"`
	Group              string              `json:"group"`
	Kind               string              `json:"kind"`
	Basis              string              `json:"basis"`
	JS                 string              `json:"js"`
	Template           string              `json:"template"`
	MetaFragment       string              `json:"meta_fragment"`
	Adapter            string              `json:"adapter"`
	Expected           map[string]string   `json:"expected"`
	DiagnosticExpected map[string][]string `json:"diagnostic_expected"`
	Carry              *l10Carry           `json:"carry,omitempty"`
}

type l10Carry struct {
	Owner  string `json:"owner"`
	Reason string `json:"reason"`
}

// TestL10SalesforceConformance compares every exported org.tsv row exactly.
// Every matching row and rejection diagnostic is asserted as exact text.
// Observed carries retain their scoped reason. GLADE_L10_REPORT selects the TSV;
// CI needs no Salesforce or capture tools and capture mode cannot bypass checks.
func TestL10SalesforceConformance(t *testing.T) {
	// The explicit browser selector lets guarded queues supply their prepared
	// Playwright executable when this conformance job moves between hosts.
	t.Run("BrowserRuntime", l10SalesforceConformance)
}

func l10SalesforceConformance(t *testing.T) {
	data, err := os.ReadFile("testdata/l10_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Cases               []l10Case         `json:"cases"`
		VersionGates        []json.RawMessage `json:"version_gates"`
		RuntimeVersionGates []json.RawMessage `json:"runtime_version_gates"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if table.VersionGates == nil || table.RuntimeVersionGates == nil || len(table.VersionGates) != 0 || len(table.RuntimeVersionGates) != 0 {
		t.Fatal("L10 native floor/ceiling capture has no version differences")
	}
	cases := table.Cases
	versions := []string{"59.0", "67.0"}
	seen, groups := map[string]bool{}, map[string]bool{}
	var compileCases, runtimeCases []l10Case
	for _, c := range cases {
		if c.ID == "" || seen[c.ID] || c.Basis != "org" {
			t.Fatalf("invalid or duplicate L10 row %q", c.ID)
		}
		seen[c.ID], groups[c.Group] = true, true
		if c.Carry != nil && (c.Kind != "runtime" || (c.Carry.Owner != "L07" && c.Carry.Owner != "L09") || c.Carry.Reason == "" || strings.ContainsAny(c.Carry.Reason, "\r\n\t")) {
			t.Fatalf("invalid L10 carry %s", c.ID)
		}
		switch c.Kind {
		case "compile":
			if c.JS == "" || c.Template == "" {
				t.Fatalf("missing L10 compile input %s", c.ID)
			}
			compileCases = append(compileCases, c)
		case "runtime":
			if c.Adapter != "info" && c.Adapter != "infos" && c.Adapter != "pick" && c.Adapter != "pickall" && c.Adapter != "defaults" && c.Adapter != "layout" {
				t.Fatalf("invalid L10 adapter %s: %s", c.ID, c.Adapter)
			}
			runtimeCases = append(runtimeCases, c)
		default:
			t.Fatalf("invalid L10 row kind %s: %s", c.ID, c.Kind)
		}
		for _, api := range versions {
			want, ok := c.Expected[api]
			if !ok || (c.Kind == "compile" && want != "COMPILE_OK" && want != "COMPILE_ERROR") ||
				(c.Kind == "runtime" && (!strings.HasPrefix(want, "BROWSER|") || !json.Valid([]byte(strings.TrimPrefix(want, "BROWSER|"))))) {
				t.Fatalf("missing or invalid native L10 row %s API %s", c.ID, api)
			}
			if c.Kind == "compile" {
				diagnostics, present := c.DiagnosticExpected[api]
				if !present || (want == "COMPILE_ERROR") != (len(diagnostics) > 0) {
					t.Fatalf("missing native L10 rejection diagnostic %s API %s", c.ID, api)
				}
			}
		}
		if c.Expected["59.0"] != c.Expected["67.0"] {
			t.Fatalf("uncaptured L10 version difference for %s", c.ID)
		}
	}
	if len(cases) != 231 || len(compileCases) != 54 || len(runtimeCases) != 177 {
		t.Fatalf("L10 requires 54 compile and 177 DOM rows, got %d/%d", len(compileCases), len(runtimeCases))
	}
	for _, group := range []string{"object-info-shape", "picklists", "defaults", "layouts", "record-types"} {
		if !groups[group] {
			t.Fatalf("missing L10 group %s", group)
		}
	}
	dependencyRoot, err := gladehome.Root()
	if err != nil {
		t.Fatal(err)
	}
	toolchain, err := gladehome.LWCToolchainDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"@lwc/compiler/package.json", "@lwc/engine-dom/dist/index.js", "@lwc/synthetic-shadow/dist/index.js"} {
		if _, err := os.Stat(filepath.Join(toolchain, "node_modules", filepath.FromSlash(rel))); err != nil {
			t.Fatal(err) // An absent toolchain must never count as a native rejection.
		}
	}

	var report strings.Builder
	fmt.Fprintln(&report, "api\tid\tkind\tstatus\tactual\texpected\treason")
	matches, total, compileMatches, domMatches, localDOMRows, carries := 0, 0, 0, 0, 0, 0
	diagnosticMatches, diagnosticTotal := 0, 0
	for _, api := range versions {
		compiled := l10CompileCases(t, api, compileCases)
		dom, domErrors, runtimeCompileErr, browserErr := l10ObserveDOM(t, api, runtimeCases, dependencyRoot, toolchain)
		if browserErr != nil {
			t.Fatalf("L10 browser environment: %v", browserErr)
		}
		apiMatches := 0
		for _, c := range cases {
			want, got, reason := c.Expected[api], "", ""
			observed := false
			if c.Kind == "compile" {
				compileErr := compiled[c.ID].Err
				got = "COMPILE_OK"
				if compileErr != nil {
					got = "COMPILE_ERROR"
				}
				if got == want {
					compileMatches++
				} else {
					reason = "source compile differs from native"
					if compileErr != nil {
						reason += ": " + compileErr.Error()
					}
				}
			} else if raw, ok := dom[c.ID]; ok {
				got, err = l10BrowserText(raw)
				if err != nil {
					t.Fatal(err)
				}
				localDOMRows++
				observed = true
				if got == want {
					domMatches++
				} else {
					reason = "metadata DOM observation differs from native"
				}
			} else {
				got, reason = "BROWSER_ERROR", "local DOM observation missing"
				if runtimeCompileErr != nil {
					got = "BROWSER_COMPILE_ERROR"
					reason += ": " + runtimeCompileErr.Error()
				} else if domErrors[c.ID] != "" {
					reason += ": " + domErrors[c.ID]
				}
			}
			status := "MISMATCH"
			if got == want { // Exact text: case, Id length, undefined markers and raw null survive.
				status = "MATCH"
				matches++
				apiMatches++
			} else if c.Carry != nil && observed {
				status, reason = "CARRIED", c.Carry.Owner+": "+c.Carry.Reason
				carries++
				t.Logf("%s API %s carry %s", c.ID, api, reason)
			}
			total++
			fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, c.Kind, status, l10OneLine(got), l10OneLine(want), l10OneLine(reason))
			if c.Kind == "compile" && want == "COMPILE_ERROR" {
				diagnosticTotal++
				actualDiagnostics, err := json.Marshal(l10RejectionDiagnostics(compiled[c.ID]))
				if err != nil {
					t.Fatal(err)
				}
				expectedDiagnostics, err := json.Marshal(c.DiagnosticExpected[api])
				if err != nil {
					t.Fatal(err)
				}
				diagnosticStatus, diagnosticReason := "MISMATCH", "exact rejection diagnostic differs from native"
				if string(actualDiagnostics) == string(expectedDiagnostics) {
					diagnosticMatches++
					diagnosticStatus, diagnosticReason = "MATCH", ""
				} else {
					t.Errorf("%s API %s rejection expected <%s> actual <%s>", c.ID, api, expectedDiagnostics, actualDiagnostics)
				}
				fmt.Fprintf(&report, "%s\t%s@diagnostic\tdiagnostic\t%s\t%s\t%s\t%s\n", api, c.ID, diagnosticStatus, actualDiagnostics, expectedDiagnostics, diagnosticReason)
			}
			if status == "MISMATCH" {
				t.Errorf("%s API %s expected <%s> actual <%s>: %s", c.ID, api, want, got, l10OneLine(reason))
			}
		}
		fmt.Fprintf(&report, "API_TOTAL\t%s\t%d/%d\n", api, apiMatches, len(cases))
		t.Logf("L10 API %s matches %d/%d", api, apiMatches, len(cases))
		if path := os.Getenv("GLADE_L10_REPORT"); path != "" {
			if err := os.WriteFile(path, []byte(report.String()), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	fmt.Fprintf(&report, "TOTAL\t%d/%d\nCOMPILE_TOTAL\t%d/%d\nDOM_TOTAL\t%d/%d\nLOCAL_DOM_ROWS\t%d/%d\n", matches, total, compileMatches, len(compileCases)*len(versions), domMatches, len(runtimeCases)*len(versions), localDOMRows, len(runtimeCases)*len(versions))
	t.Logf("L10 matches %d/%d; compile %d/%d; DOM %d/%d; observed local DOM %d/%d", matches, total, compileMatches, len(compileCases)*len(versions), domMatches, len(runtimeCases)*len(versions), localDOMRows, len(runtimeCases)*len(versions))
	fmt.Fprintf(&report, "DIAGNOSTIC_TOTAL\t%d/%d\n", diagnosticMatches, diagnosticTotal)
	t.Logf("L10 exact rejection diagnostics %d/%d", diagnosticMatches, diagnosticTotal)
	fmt.Fprintf(&report, "CARRIED_ROWS\t%d\n", carries)
	t.Logf("L10 carries %d rows", carries)
	if path := os.Getenv("GLADE_L10_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func l10RejectionDiagnostics(result compile.BundleResult) []string {
	messages := make([]string, 0, len(result.Diagnostics))
	for _, diagnostic := range result.Diagnostics {
		messages = append(messages, diagnostic.Message)
	}
	if len(messages) == 0 && result.Err != nil {
		// Metadata validation errors carry a local filename before their native
		// message. Remove that transport prefix alone; preserve all diagnostic text.
		message := result.Err.Error()
		if _, native, ok := strings.Cut(message, ".js-meta.xml: "); ok {
			message = native
		}
		messages = append(messages, message)
	}
	return messages
}

func l10CompileCases(t *testing.T, api string, cases []l10Case) map[string]compile.BundleResult {
	t.Helper()
	root := t.TempDir()
	keys := map[string]string{}
	for index, c := range cases {
		name := fmt.Sprintf("familyL10Compile%03d", index)
		rel := filepath.Join("cases", fmt.Sprintf("%04d", index), "force-app", "main", "default", "lwc", name)
		keys[c.ID] = filepath.ToSlash(rel)
		bundle := filepath.Join(root, rel)
		l10Write(t, filepath.Join(bundle, name+".js"), []byte(strings.ReplaceAll(c.JS, "FamilyCompile", "FamilyL10Compile"+fmt.Sprintf("%03d", index))))
		l10Write(t, filepath.Join(bundle, name+".html"), []byte(c.Template))
		fragment := c.MetaFragment
		if fragment == "" {
			fragment = "<isExposed>false</isExposed>"
		}
		l10Write(t, filepath.Join(bundle, name+".js-meta.xml"), []byte(`<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion>`+fragment+`</LightningComponentBundle>`))
	}
	l10Write(t, filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"cases","default":true}],"sourceApiVersion":"`+api+`"}`))
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := compile.CompileBatch(p, compile.Options{OutDir: filepath.Join(root, "dist")})
	if err != nil {
		t.Fatal(err)
	}
	results := map[string]compile.BundleResult{}
	for _, c := range cases {
		result, ok := batch[keys[c.ID]]
		if !ok {
			t.Fatalf("missing L10 batch row %s", c.ID)
		}
		l10CheckCompilerEnvironment(t, result.Err)
		results[c.ID] = result
	}
	return results
}

func l10ObserveDOM(t *testing.T, api string, cases []l10Case, dependencyRoot, toolchain string) (map[string]json.RawMessage, map[string]string, error, error) {
	t.Helper()
	root := t.TempDir()
	l10Copy(t, filepath.Join("testdata", "l10_runtime", "api"+api[:2]), root)
	p, err := project.Load(root)
	if err != nil {
		return nil, nil, nil, err
	}
	manifest, err := compile.Compile(p, compile.Options{OutDir: filepath.Join(root, "dist")})
	l10CheckCompilerEnvironment(t, err)
	if err != nil {
		return nil, nil, err, nil
	}
	page := "familyL10Runtime" + api[:2]
	entry, ok := manifest.Modules["c:"+page]
	if !ok {
		return nil, nil, fmt.Errorf("runtime entry %s missing", page), nil
	}
	imports := lwcbrowser.SalesforceImportMap()
	imports["lwc"], imports["@lwc/synthetic-shadow"] = "/engine.js", "/shadow.js"
	for _, module := range manifest.Modules {
		rel, err := filepath.Rel(manifest.OutDir, module.File)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, nil, nil, fmt.Errorf("L10 module escapes output: %s", module.File)
		}
		imports[module.ModuleKey] = "/modules/" + filepath.ToSlash(rel)
	}
	importJSON, err := json.Marshal(map[string]any{"imports": imports})
	if err != nil {
		return nil, nil, nil, err
	}
	registry, err := resource.LoadProject(p)
	if err != nil {
		return nil, nil, nil, err
	}
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		return nil, nil, nil, err
	}
	source, err := server.NewSourceMetadataFromProject(p)
	if err != nil {
		return nil, nil, nil, err
	}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	l10SetupOrgMetadata(t, &org)
	org.Metadata = registry
	product := server.NewWithSource(&org, source)
	product.SetProjectIndex(typesys.Build(p, schema))
	mux := http.NewServeMux()
	mux.Handle("/lightning/", product) // Product metadata endpoints and adapters; no answer mocks.
	mux.Handle("/modules/", http.StripPrefix("/modules/", http.FileServer(http.Dir(manifest.OutDir))))
	for route, rel := range map[string]string{"/engine.js": "@lwc/engine-dom/dist/index.js", "/shadow.js": "@lwc/synthetic-shadow/dist/index.js"} {
		file := filepath.Join(toolchain, "node_modules", filepath.FromSlash(rel))
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
		pageContext, err := json.Marshal(map[string]any{
			"pageReference": map[string]any{
				"type":       "standard__navItemPage",
				"attributes": map[string]string{"apiName": "FamilyL10Oracle" + api[:2]},
				"state":      map[string]string{"c__case": r.URL.Query().Get("c__case")},
			},
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><script>window.process={env:{NODE_ENV:"production"}};window.__l10Error=null;</script><script id="glade-lwc-context" type="application/json">{}</script><script id="glade-lightning-config" type="application/json">%s</script><script type="importmap">%s</script><main></main><script type="module">try {await import("@lwc/synthetic-shadow");const {createElement}=await import("lwc");const {default:Ctor}=await import("c/%s");document.querySelector("main").appendChild(createElement(%q,{is:Ctor}));} catch(e) {window.__l10Error={name:e.name,message:e.message};}</script>`, pageContext, importJSON, page, entry.Tag)
	})
	local := httptest.NewServer(mux)
	defer local.Close()
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(dependencyRoot, "lwcruntime", "node_modules", "playwright")
	}
	observer, err := filepath.Abs("testdata/l10_browser.mjs")
	if err != nil {
		return nil, nil, nil, err
	}
	// Replay the native requested row through CurrentPageReference, then click
	// the unchanged component control and read only its DOM acknowledgement.
	specs := make([]map[string]string, len(cases))
	ids := map[string]bool{}
	for index, c := range cases {
		specs[index] = map[string]string{"id": c.ID, "adapter": c.Adapter}
		ids[c.ID] = true
	}
	config, err := json.Marshal(map[string]any{"url": local.URL, "specs": specs, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")})
	if err != nil {
		return nil, nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", observer)
	cmd.Stdin = bytes.NewReader(config)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("local observer: %w: %s", err, l10OneLine(stderr.String()))
	}
	var observed struct {
		Values map[string]json.RawMessage `json:"values"`
		Errors map[string]string          `json:"errors"`
	}
	if err := json.Unmarshal(output, &observed); err != nil {
		return nil, nil, nil, err
	}
	for id := range observed.Values {
		if !ids[id] || observed.Errors[id] != "" {
			return nil, nil, nil, fmt.Errorf("unexpected or conflicting local L10 row %s", id)
		}
	}
	for id := range observed.Errors {
		if !ids[id] {
			return nil, nil, nil, fmt.Errorf("unexpected local L10 error row %s", id)
		}
	}
	return observed.Values, observed.Errors, nil, nil
}

func l10CheckCompilerEnvironment(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	for _, marker := range []string{"Cannot find module", "ERR_MODULE_NOT_FOUND", "decode compile result:", "could not find glade", "signal: killed"} {
		if strings.Contains(err.Error(), marker) {
			t.Fatalf("L10 compiler environment: %v", err)
		}
	}
}

func l10Copy(t *testing.T, source, target string) {
	t.Helper()
	if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		l10Write(t, filepath.Join(target, rel), data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func l10Write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

// Match the native compact, sorted, ensure_ascii JSON serialization only.
// No field is removed or supplied, and null is never changed to a sentinel.
func l10BrowserText(raw json.RawMessage) (string, error) {
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

func l10OneLine(value string) string {
	return strings.NewReplacer("\t", `\t`, "\r", `\r`, "\n", `\n`).Replace(value)
}
