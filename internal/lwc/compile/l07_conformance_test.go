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

type l07Case struct {
	ID                 string            `json:"id"`
	Group              string            `json:"group"`
	Kind               string            `json:"kind"`
	Basis              string            `json:"basis"`
	JS                 string            `json:"js"`
	Sources            map[string]string `json:"sources,omitempty"`
	Template           string            `json:"template"`
	MetaFragment       string            `json:"meta_fragment"`
	Adapter            string            `json:"adapter"`
	Expected           map[string]string `json:"expected"`
	DiagnosticExpected map[string]string `json:"diagnostic_expected"`
	Carry              *l07Carry         `json:"carry,omitempty"`
	DiagnosticCarry    *l07Carry         `json:"diagnostic_carry,omitempty"`
}

type l07Carry struct {
	Owner  string `json:"owner"`
	Reason string `json:"reason"`
}

// TestL07SalesforceConformance compares every exported org.tsv row exactly.
// Native API 59/67 answers and runtime sources come from the owned wire
// capture (204 rows per API).
// GLADE_L07_CAPTURE=1 records differences without failing them;
// GLADE_L07_REPORT selects the per-row TSV. CI needs no Salesforce or capture tools.
func TestL07SalesforceConformance(t *testing.T) {
	// The explicit browser selector lets guarded queues supply their prepared
	// Playwright executable when this conformance job moves between hosts.
	t.Run("BrowserRuntime", l07SalesforceConformance)
}

func l07SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_L07_CAPTURE") == "1"
	data, err := os.ReadFile("testdata/l07_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []l07Case
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	versions := []string{"59.0", "67.0"}
	seen, groups := map[string]bool{}, map[string]bool{}
	var compileCases, runtimeCases []l07Case
	diagnosticRows := 0
	for _, c := range cases {
		if c.ID == "" || seen[c.ID] || c.Basis != "org" {
			t.Fatalf("invalid or duplicate L07 row %q", c.ID)
		}
		seen[c.ID], groups[c.Group] = true, true
		if len(c.DiagnosticExpected) != 0 {
			diagnosticRows++
		}
		for _, carry := range []*l07Carry{c.Carry, c.DiagnosticCarry} {
			if carry != nil && (carry.Owner == "" || strings.HasPrefix(carry.Owner, "L07") || carry.Reason == "" ||
				strings.ContainsAny(carry.Owner+carry.Reason, "\r\n\t")) {
				t.Fatalf("invalid L07 carry %s", c.ID)
			}
		}
		if (c.Carry != nil && c.Kind != "runtime") || (c.DiagnosticCarry != nil && len(c.DiagnosticExpected) == 0) {
			t.Fatalf("invalid L07 carry kind %s", c.ID)
		}
		switch c.Kind {
		case "compile":
			if c.JS == "" || c.Template == "" {
				t.Fatalf("missing L07 compile input %s", c.ID)
			}
			compileCases = append(compileCases, c)
		case "runtime":
			if c.Adapter != "apex" && c.Adapter != "lds" {
				t.Fatalf("invalid L07 adapter %s: %s", c.ID, c.Adapter)
			}
			runtimeCases = append(runtimeCases, c)
		default:
			t.Fatalf("invalid L07 row kind %s: %s", c.ID, c.Kind)
		}
		for _, api := range versions {
			want, ok := c.Expected[api]
			if !ok || (c.Kind == "compile" && want != "COMPILE_OK" && want != "COMPILE_ERROR") ||
				(c.Kind == "runtime" && (!strings.HasPrefix(want, "BROWSER|") || !json.Valid([]byte(strings.TrimPrefix(want, "BROWSER|"))))) {
				t.Fatalf("missing or invalid native L07 row %s API %s", c.ID, api)
			}
			if len(c.DiagnosticExpected) != 0 && (c.Kind != "compile" || want != "COMPILE_ERROR" || c.DiagnosticExpected[api] == "") {
				t.Fatalf("missing or invalid native L07 diagnostic %s API %s", c.ID, api)
			}
			if want == "COMPILE_ERROR" && c.DiagnosticExpected[api] == "" {
				t.Fatalf("missing captured rejection diagnostic %s API %s", c.ID, api)
			}
		}
	}
	if len(cases) != 204 || len(compileCases) != 94 || len(runtimeCases) != 110 {
		t.Fatalf("L07 requires 94 compile and 110 DOM rows, got %d/%d", len(compileCases), len(runtimeCases))
	}
	if diagnosticRows != 17 {
		t.Fatalf("L07 requires all 17 captured rejection diagnostics, got %d", diagnosticRows)
	}
	for _, group := range []string{"consumer-forms", "reactive-config", "suppression", "immutability", "reconnect-stale"} {
		if !groups[group] {
			t.Fatalf("missing L07 group %s", group)
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
	if !t.Run("RuntimeBuild", l07CheckRuntimeBuild) {
		t.FailNow()
	}
	if !t.Run("AdapterContract", func(t *testing.T) {
		root, err := gladehome.SourceRoot()
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "node", "--test", "lwcruntime/test/wire-adapter-contract.test.mjs")
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("L07 adapter contracts: %v\n%s", err, output)
		}
	}) {
		t.FailNow()
	}
	var reviewReport string
	if !t.Run("ReviewCompiler", func(t *testing.T) {
		reviewReport = l07ReviewCompiler(t, capture)
	}) {
		t.FailNow()
	}

	var report strings.Builder
	fmt.Fprintln(&report, "api\tid\tkind\tstatus\tactual\texpected\treason\tdiagnostic_status\tdiagnostic_actual\tdiagnostic_expected\tdiagnostic_reason")
	report.WriteString(reviewReport)
	matches, total, compileMatches, domMatches, localDOMRows := 0, 0, 0, 0, 0
	carriedRows, diagnosticMatches, diagnosticTotal, diagnosticCarries := 0, 0, 0, 0
	for _, api := range versions {
		compiled := l07CompileCases(t, api, compileCases)
		dom, domErrors, runtimeCompileErr, browserErr := l07ObserveDOM(t, api, runtimeCases, dependencyRoot)
		if browserErr != nil {
			t.Fatalf("L07 browser environment: %v", browserErr)
		}
		apiMatches := 0
		for _, c := range cases {
			want, got, reason := c.Expected[api], "", ""
			diagnosticWant, diagnosticGot, diagnosticStatus, diagnosticReason := c.DiagnosticExpected[api], "", "", ""
			observedDOM := false
			if c.Kind == "compile" {
				result := compiled[c.ID]
				compileErr := result.Err
				got = "COMPILE_OK"
				if compileErr != nil {
					got = "COMPILE_ERROR"
				}
				if diagnosticWant != "" {
					diagnosticGot = "MISSING_DIAGNOSTIC"
					if len(result.Diagnostics) == 1 {
						diagnosticGot = result.Diagnostics[0].Message
					} else if len(result.Diagnostics) > 1 {
						encoded, err := json.Marshal(result.Diagnostics)
						if err != nil {
							t.Fatal(err)
						}
						diagnosticGot = string(encoded)
					}
					diagnosticTotal++
					diagnosticStatus = "MISMATCH"
					// Check the complete native message separately from org.tsv's
					// rejection status, including source positions and hosted paths.
					if diagnosticGot == diagnosticWant {
						diagnosticStatus = "MATCH"
						diagnosticMatches++
					} else if c.DiagnosticCarry != nil && got == want && len(result.Diagnostics) != 0 {
						diagnosticStatus = "CARRIED"
						diagnosticReason = c.DiagnosticCarry.Owner + ": " + c.DiagnosticCarry.Reason
						diagnosticCarries++
						t.Logf("%s API %s diagnostic carry %s", c.ID, api, diagnosticReason)
					} else {
						diagnosticReason = "compiler diagnostic differs from native"
						if !capture {
							t.Errorf("%s API %s diagnostic expected <%s> actual <%s>", c.ID, api, diagnosticWant, diagnosticGot)
						}
					}
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
				got, err = l07BrowserText(raw)
				if err != nil {
					t.Fatal(err)
				}
				localDOMRows++
				observedDOM = true
				if got == want {
					domMatches++
				} else {
					reason = "wire DOM observation differs from native"
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
			} else if c.Carry != nil && observedDOM {
				status = "CARRIED"
				reason = c.Carry.Owner + ": " + c.Carry.Reason
				carriedRows++
				t.Logf("%s API %s carry %s", c.ID, api, reason)
			}
			total++
			fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, c.Kind, status, l07OneLine(got), l07OneLine(want), l07OneLine(reason), diagnosticStatus, l07OneLine(diagnosticGot), l07OneLine(diagnosticWant), l07OneLine(diagnosticReason))
			if !capture && status == "MISMATCH" {
				t.Errorf("%s API %s expected <%s> actual <%s>: %s", c.ID, api, want, got, l07OneLine(reason))
			}
		}
		fmt.Fprintf(&report, "API_TOTAL\t%s\t%d/%d\n", api, apiMatches, len(cases))
		t.Logf("L07 API %s matches %d/%d", api, apiMatches, len(cases))
	}
	fmt.Fprintf(&report, "TOTAL\t%d/%d\nCOMPILE_TOTAL\t%d/%d\nDOM_TOTAL\t%d/%d\nLOCAL_DOM_ROWS\t%d/%d\n", matches, total, compileMatches, len(compileCases)*len(versions), domMatches, len(runtimeCases)*len(versions), localDOMRows, len(runtimeCases)*len(versions))
	fmt.Fprintf(&report, "CARRIED_ROWS\t%d\nDIAGNOSTIC_TOTAL\t%d/%d\nCARRIED_DIAGNOSTICS\t%d\n", carriedRows, diagnosticMatches, diagnosticTotal, diagnosticCarries)
	t.Logf("L07 matches %d/%d; compile %d/%d; DOM %d/%d; observed local DOM %d/%d", matches, total, compileMatches, len(compileCases)*len(versions), domMatches, len(runtimeCases)*len(versions), localDOMRows, len(runtimeCases)*len(versions))
	t.Logf("L07 carries %d DOM rows; diagnostics %d/%d with %d carries", carriedRows, diagnosticMatches, diagnosticTotal, diagnosticCarries)
	if path := os.Getenv("GLADE_L07_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

// D7 fixed-source controls at every API 59-67 and adopted L09 compiler rows.
// The apparent 59/67 column difference came from generated class names. Sources
// and complete native diagnostics are replayed verbatim, without API offsets.
func l07ReviewCompiler(t *testing.T, capture bool) string {
	t.Helper()
	data, err := os.ReadFile("testdata/l07_review_compiler.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []l07Case
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 17 {
		t.Fatalf("L07 requires all 17 review compiler sources, got %d", len(cases))
	}
	seen := map[string]bool{}
	for _, c := range cases {
		if c.ID == "" || seen[c.ID] || c.Basis != "org" || c.Kind != "compile" || c.Template == "" || c.Carry != nil || c.DiagnosticCarry != nil {
			t.Fatalf("invalid L07 review compiler row %q", c.ID)
		}
		seen[c.ID] = true
		for api, want := range c.Expected {
			if c.Sources[api] == "" || (want != "COMPILE_OK" && want != "COMPILE_ERROR") ||
				(want == "COMPILE_ERROR" && c.DiagnosticExpected[api] == "") {
				t.Fatalf("missing native L07 review input/diagnostic %s API %s", c.ID, api)
			}
		}
	}
	var report strings.Builder
	matches, total := 0, 0
	for version := 59; version <= 67; version++ {
		api := fmt.Sprintf("%d.0", version)
		var selected []l07Case
		for _, c := range cases {
			if _, ok := c.Expected[api]; ok {
				selected = append(selected, c)
			}
		}
		results := l07CompileCases(t, api, selected)
		for _, c := range selected {
			result := results[c.ID]
			got := "COMPILE_OK"
			if result.Err != nil {
				got = "COMPILE_ERROR"
			}
			diagnostic := ""
			for _, d := range result.Diagnostics {
				if diagnostic != "" {
					diagnostic += "\n"
				}
				diagnostic += d.Message
			}
			want, diagnosticWant := c.Expected[api], c.DiagnosticExpected[api]
			status, diagnosticStatus := "MATCH", ""
			if diagnosticWant != "" {
				diagnosticStatus = "MATCH"
				if diagnostic != diagnosticWant {
					diagnosticStatus = "MISMATCH"
				}
			}
			if got != want || diagnosticStatus == "MISMATCH" {
				status = "MISMATCH"
				if !capture {
					t.Errorf("%s API %s expected <%s> actual <%s>; diagnostic expected <%s> actual <%s>", c.ID, api, want, got, diagnosticWant, diagnostic)
				}
			} else {
				matches++
			}
			total++
			fmt.Fprintf(&report, "%s\t%s\tcompile\t%s\t%s\t%s\t\t%s\t%s\t%s\t\n", api, c.ID, status, got, want, diagnosticStatus, l07OneLine(diagnostic), l07OneLine(diagnosticWant))
		}
	}
	if total != 62 {
		t.Fatalf("L07 requires all 62 review compiler observations, got %d", total)
	}
	fmt.Fprintf(&report, "REVIEW_COMPILER_TOTAL\t%d/%d\n", matches, total)
	t.Logf("L07 review compiler matches %d/%d (D7 APIs 59-67; L09 APIs 59/67)", matches, total)
	return report.String()
}

func l07CheckRuntimeBuild(t *testing.T) {
	t.Helper()
	sourceRoot, err := gladehome.SourceRoot()
	if err != nil {
		t.Fatal(err)
	}
	buildRoot := t.TempDir()
	runtimeDir := filepath.Join(buildRoot, "lwcruntime")
	script, err := os.ReadFile(filepath.Join(sourceRoot, "lwcruntime", "esbuild.config.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	l07Write(t, filepath.Join(runtimeDir, "esbuild.config.mjs"), script)
	for _, rel := range []string{"src", "node_modules"} {
		if err := os.Symlink(filepath.Join(sourceRoot, "lwcruntime", rel), filepath.Join(runtimeDir, rel)); err != nil {
			t.Fatal(err)
		}
	}
	// Run the authoritative generator in a temporary output tree. Guarded jobs
	// verify the committed asset without changing the checkout or source files.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", filepath.Join(runtimeDir, "esbuild.config.mjs"))
	cmd.Dir = runtimeDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("L07 runtime build: %v\n%s", err, output)
	}
	rel := filepath.Join("internal", "lwcruntime", "embed", "glade.out.js")
	generated, err := os.ReadFile(filepath.Join(buildRoot, rel))
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(filepath.Join(sourceRoot, rel))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, committed) {
		t.Fatal("glade.out.js differs from the authoritative runtime build")
	}
}

func l07CompileCases(t *testing.T, api string, cases []l07Case) map[string]compile.BundleResult {
	t.Helper()
	root := t.TempDir()
	keys := map[string]string{}
	for index, c := range cases {
		name := fmt.Sprintf("familyL07Compile%03d", index)
		rel := filepath.Join("cases", fmt.Sprintf("%04d", index), "force-app", "main", "default", "lwc", name)
		keys[c.ID] = filepath.ToSlash(rel)
		bundle := filepath.Join(root, rel)
		// Replay the native deployment's bundle class name and source columns.
		source := c.Sources[api]
		if source == "" {
			source = strings.ReplaceAll(c.JS, "FamilyWire", "FamilyL07Compile"+fmt.Sprintf("%03d", index))
		}
		l07Write(t, filepath.Join(bundle, name+".js"), []byte(source))
		l07Write(t, filepath.Join(bundle, name+".html"), []byte(c.Template))
		fragment := c.MetaFragment
		if fragment == "" {
			fragment = "<isExposed>false</isExposed>"
		}
		l07Write(t, filepath.Join(bundle, name+".js-meta.xml"), []byte(`<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion>`+fragment+`</LightningComponentBundle>`))
	}
	fixtureAPI := api[:2]
	if fixtureAPI != "59" && fixtureAPI != "67" {
		fixtureAPI = "59" // Unchanged owned Apex fixture; D7 controls use no Apex imports.
	}
	l07Copy(t, filepath.Join("testdata", "l07_runtime", "api"+fixtureAPI, "force-app", "main", "default", "classes"), filepath.Join(root, "cases", "common", "force-app", "main", "default", "classes"))
	l07Write(t, filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"cases","default":true}],"sourceApiVersion":"`+api+`"}`))
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
			t.Fatalf("missing L07 batch row %s", c.ID)
		}
		l07CheckCompilerEnvironment(t, result.Err)
		results[c.ID] = result
	}
	return results
}

func l07ObserveDOM(t *testing.T, api string, cases []l07Case, dependencyRoot string) (map[string]json.RawMessage, map[string]string, error, error) {
	t.Helper()
	root := t.TempDir()
	l07Copy(t, filepath.Join("testdata", "l07_runtime", "api"+api[:2]), root)
	p, err := project.Load(root)
	if err != nil {
		return nil, nil, nil, err
	}
	manifest, err := compile.Compile(p, compile.Options{OutDir: filepath.Join(root, "dist")})
	l07CheckCompilerEnvironment(t, err)
	if err != nil {
		return nil, nil, err, nil
	}
	page := "familyL07Runtime" + api[:2]
	_, ok := manifest.Modules["c:"+page]
	if !ok {
		return nil, nil, fmt.Errorf("runtime entry %s missing", page), nil
	}
	localManifest := lwcbrowser.Manifest{Modules: map[string]lwcbrowser.ModuleEntry{}}
	for qualified, module := range manifest.Modules {
		rel, err := filepath.Rel(manifest.OutDir, module.File)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, nil, nil, fmt.Errorf("L07 module escapes output: %s", module.File)
		}
		localManifest.Modules[qualified] = lwcbrowser.ModuleEntry{URL: "/lightning/modules/" + filepath.ToSlash(rel), Tag: module.Tag}
	}
	bootstrap := lwcbrowser.BootstrapHTML(lwcbrowser.PageConfig{Namespace: "c", Manifest: localManifest})
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
	org.Metadata = registry
	product := server.NewWithSource(&org, source)
	product.SetProjectIndex(typesys.Build(p, schema))
	mux := http.NewServeMux()
	mux.Handle("/lightning/", product) // Real Apex, LDS, errors and adapter modules; no answer mocks.
	mux.Handle("/lightning/modules/", http.StripPrefix("/lightning/modules/", http.FileServer(http.Dir(manifest.OutDir))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><script>window.__l07Error=null;</script><script id="glade-lwc-context" type="application/json">{}</script><main id="l07-host"></main>%s<script>window.$Lightning.createComponent(%q,{},"l07-host",(_el,status,message)=>{if(status!=="SUCCESS")window.__l07Error={name:"Error",message};});</script>`, bootstrap, "c:"+page)
	})
	local := httptest.NewServer(mux)
	defer local.Close()
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(dependencyRoot, "lwcruntime", "node_modules", "playwright")
	}
	observer, err := filepath.Abs("testdata/l07_browser.mjs")
	if err != nil {
		return nil, nil, nil, err
	}
	// Preserve native source order: all Apex rows share page 0's storable cache;
	// LDS rows alternate between pages 1/2 and use distinct owned records.
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", observer)
	cmd.Stdin = bytes.NewReader(config)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("local observer: %w: %s", err, l07OneLine(stderr.String()))
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
			return nil, nil, nil, fmt.Errorf("unexpected or conflicting local L07 row %s", id)
		}
	}
	for id := range observed.Errors {
		if !ids[id] {
			return nil, nil, nil, fmt.Errorf("unexpected local L07 error row %s", id)
		}
	}
	return observed.Values, observed.Errors, nil, nil
}

func l07CheckCompilerEnvironment(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	for _, marker := range []string{"Cannot find module", "ERR_MODULE_NOT_FOUND", "decode compile result:", "could not find glade", "signal: killed"} {
		if strings.Contains(err.Error(), marker) {
			t.Fatalf("L07 compiler environment: %v", err)
		}
	}
}

func l07Copy(t *testing.T, source, target string) {
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
		l07Write(t, filepath.Join(target, rel), data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func l07Write(t *testing.T, path string, data []byte) {
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
func l07BrowserText(raw json.RawMessage) (string, error) {
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

func l07OneLine(value string) string {
	return strings.NewReplacer("\t", `\t`, "\r", `\r`, "\n", `\n`).Replace(value)
}
