package compile_test

import (
	"bytes"
	"context"
	"crypto/sha256"
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

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/lwcshell"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type l11Case struct {
	ID                 string              `json:"id"`
	Group              string              `json:"group"`
	Kind               string              `json:"kind"`
	Basis              string              `json:"basis"`
	JS                 string              `json:"js"`
	Template           string              `json:"template"`
	MetaFragment       string              `json:"meta_fragment"`
	FixturePath        string              `json:"fixture_path"`
	FixtureXML         string              `json:"fixture_xml"`
	Adapter            string              `json:"adapter"`
	Expected           map[string]string   `json:"expected"`
	DiagnosticExpected map[string][]string `json:"diagnostic_expected"`
	Oracle             map[string]string   `json:"oracle"`
	RuntimeSource      map[string]string   `json:"runtime_source"`
}

type l11CompileResult struct {
	Err         error
	Diagnostics []string
}

// TestL11SalesforceConformance replays all 285 captured rows at API 59/67:
// 64 LWC compile, eight ListView metadata and 213 native DOM observations.
// Runtime inputs retain the original API 67 null/empty related-record reads.
// GLADE_L11_CAPTURE=1 reports mismatches without failing; GLADE_L11_REPORT
// selects the TSV destination. CI needs only these credential-free exports.
func TestL11SalesforceConformance(t *testing.T) {
	t.Run("BrowserRuntime", l11SalesforceConformance)
}

func l11SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_L11_CAPTURE") == "1"
	data, err := os.ReadFile("testdata/l11_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []l11Case
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cases); err != nil {
		t.Fatal(err)
	}
	versions := []string{"59.0", "67.0"}
	seen, groups := map[string]bool{}, map[string]bool{}
	var compileCases, runtimeCases []l11Case
	metadataRows, diagnosticRows := 0, 0
	for _, c := range cases {
		if c.ID == "" || seen[c.ID] || c.Basis != "org" || len(c.Expected) != len(versions) || len(c.Oracle) != len(versions) {
			t.Fatalf("invalid or duplicate L11 row %q", c.ID)
		}
		seen[c.ID], groups[c.Group] = true, true
		switch c.Kind {
		case "compile":
			if c.JS == "" || c.Template == "" || len(c.DiagnosticExpected) != len(versions) {
				t.Fatalf("missing L11 compile input or diagnostics %s", c.ID)
			}
			if c.FixtureXML != "" {
				if !strings.HasPrefix(c.FixturePath, "objects/FamilyL11Child__c/listViews/") || strings.Contains(c.FixturePath, "..") || !strings.HasSuffix(c.FixturePath, ".listView-meta.xml") {
					t.Fatalf("invalid L11 metadata path %s", c.ID)
				}
				metadataRows++
			}
			if len(c.DiagnosticExpected["67.0"]) != 0 {
				diagnosticRows++
			}
			compileCases = append(compileCases, c)
		case "runtime":
			if c.Adapter == "" || len(c.RuntimeSource) != len(versions) {
				t.Fatalf("missing L11 DOM input %s", c.ID)
			}
			runtimeCases = append(runtimeCases, c)
		default:
			t.Fatalf("invalid L11 row kind %s: %s", c.ID, c.Kind)
		}
		for _, api := range versions {
			want, ok := c.Expected[api]
			if !ok || c.Oracle[api] == "" || (c.Kind == "compile" && want != "COMPILE_OK" && want != "COMPILE_ERROR") ||
				(c.Kind == "runtime" && (!strings.HasPrefix(want, "BROWSER|") || !json.Valid([]byte(strings.TrimPrefix(want, "BROWSER|"))))) {
				t.Fatalf("missing or invalid native L11 row %s API %s", c.ID, api)
			}
			if c.Kind == "compile" && (want == "COMPILE_ERROR") != (len(c.DiagnosticExpected[api]) != 0) {
				t.Fatalf("missing or unexpected native L11 diagnostic %s API %s", c.ID, api)
			}
			if c.Kind == "runtime" {
				source := c.RuntimeSource[api]
				if source != "api"+api[:2] && !(api == "67.0" && source == "api67_original" && (c.ID == "r_relatedRecords_null" || c.ID == "r_relatedRecords_empty")) {
					t.Fatalf("invalid L11 native source %s API %s: %s", c.ID, api, source)
				}
			}
		}
	}
	if len(cases) != 285 || len(compileCases) != 72 || metadataRows != 8 || len(runtimeCases) != 213 || diagnosticRows != 32 {
		t.Fatalf("L11 requires 72 compile/metadata (eight ListViews), 213 DOM and 32 diagnostic rows, got %d/%d/%d/%d", len(compileCases), metadataRows, len(runtimeCases), diagnosticRows)
	}
	for _, group := range []string{"related-list-reads", "list-info", "list-records", "legacy-getListUi", "list-view-mutations", "nav-items", "preferences"} {
		if !groups[group] {
			t.Fatalf("missing L11 group %s", group)
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
			t.Fatal(err) // Toolchain failures are never native compiler rejections.
		}
	}

	var report strings.Builder
	fmt.Fprintln(&report, "api\tid\tkind\tstatus\tactual\texpected\treason\tdiagnostic_status\tdiagnostic_actual\tdiagnostic_expected\tdiagnostic_reason\toracle")
	matches, compileMatches, metadataMatches, domMatches, localDOMRows, diagnosticMatches := 0, 0, 0, 0, 0, 0
	for _, api := range versions {
		compiled := l11CompileCases(t, api, compileCases)
		dom, domErrors, runtimeCompileErrors := map[string]json.RawMessage{}, map[string]string{}, map[string]error{}
		for _, source := range []string{"api" + api[:2], "api67_original"} {
			var selected []l11Case
			for _, c := range runtimeCases {
				if c.RuntimeSource[api] == source {
					selected = append(selected, c)
				}
			}
			if len(selected) == 0 {
				continue
			}
			values, failures, compileErr, browserErr := l11ObserveDOM(t, api, source, selected, dependencyRoot)
			if browserErr != nil {
				t.Fatalf("L11 browser environment: %v", browserErr)
			}
			for id, value := range values {
				dom[id] = value
			}
			for id, failure := range failures {
				domErrors[id] = failure
			}
			for _, c := range selected {
				runtimeCompileErrors[c.ID] = compileErr
			}
		}
		apiMatches, apiDOMRows := 0, 0
		for _, c := range cases {
			want, got, reason := c.Expected[api], "", ""
			diagnosticGot, diagnosticWant, diagnosticStatus := "", "", ""
			diagnosticReason := ""
			if c.Kind == "compile" {
				result := compiled[c.ID]
				got = "COMPILE_OK"
				if result.Err != nil {
					got = "COMPILE_ERROR"
				}
				reason = "local compilation or metadata loading differs from native"
				if result.Err != nil {
					reason += ": " + result.Err.Error()
				}
				if len(c.DiagnosticExpected[api]) != 0 {
					diagnosticGot = l11DiagnosticsText(t, result.Diagnostics)
					diagnosticWant = l11DiagnosticsText(t, c.DiagnosticExpected[api])
					diagnosticStatus = "MISMATCH"
					if diagnosticGot == diagnosticWant {
						diagnosticStatus = "MATCH"
						diagnosticMatches++
					} else if !capture {
						t.Errorf("%s API %s diagnostic expected <%s> actual <%s>", c.ID, api, diagnosticWant, diagnosticGot)
					}
				}
			} else if raw, ok := dom[c.ID]; ok {
				// Shared native serializer preserves every field and the raw null.
				got, err = l07BrowserText(raw)
				if err != nil {
					t.Fatal(err)
				}
				localDOMRows++
				apiDOMRows++
				reason = "local DOM observation differs from native"
			} else {
				got, reason = "BROWSER_ERROR", "local DOM observation missing"
				if err := runtimeCompileErrors[c.ID]; err != nil {
					got = "BROWSER_COMPILE_ERROR"
					reason += ": " + err.Error()
				} else if domErrors[c.ID] != "" {
					reason += ": " + domErrors[c.ID]
				}
			}
			status := "MISMATCH"
			// Compare complete text, including nested nulls and native-null rows.
			if got == want {
				status, reason = "MATCH", ""
				matches++
				apiMatches++
				if c.Kind == "runtime" {
					domMatches++
				} else {
					compileMatches++
					if c.FixtureXML != "" {
						metadataMatches++
					}
				}
			} else if !capture {
				t.Errorf("%s API %s expected <%s> actual <%s>: %s", c.ID, api, want, got, l07OneLine(reason))
			}
			fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, c.Kind, status, l07OneLine(got), l07OneLine(want), l07OneLine(reason), diagnosticStatus, l07OneLine(diagnosticGot), l07OneLine(diagnosticWant), l07OneLine(diagnosticReason), c.Oracle[api])
		}
		fmt.Fprintf(&report, "API_TOTAL\t%s\t%d/%d\nLOCAL_DOM_ROWS\t%s\t%d/%d\n", api, apiMatches, len(cases), api, apiDOMRows, len(runtimeCases))
		t.Logf("L11 API %s matches %d/%d; observed local DOM %d/%d", api, apiMatches, len(cases), apiDOMRows, len(runtimeCases))
	}
	fmt.Fprintf(&report, "TOTAL\t%d/%d\nCOMPILE_TOTAL\t%d/%d\nMETADATA_TOTAL\t%d/%d\nDOM_TOTAL\t%d/%d\nDIAGNOSTIC_TOTAL\t%d/%d\nLOCAL_DOM_ROWS\t%d/%d\n", matches, len(cases)*len(versions), compileMatches, len(compileCases)*len(versions), metadataMatches, metadataRows*len(versions), domMatches, len(runtimeCases)*len(versions), diagnosticMatches, diagnosticRows*len(versions), localDOMRows, len(runtimeCases)*len(versions))
	t.Logf("L11 matches %d/%d; compile/metadata %d/%d; DOM %d/%d; diagnostics %d/%d", matches, len(cases)*len(versions), compileMatches, len(compileCases)*len(versions), domMatches, len(runtimeCases)*len(versions), diagnosticMatches, diagnosticRows*len(versions))
	if path := os.Getenv("GLADE_L11_REPORT"); path != "" {
		l07Write(t, path, []byte(report.String()))
	}
}

func l11DiagnosticsText(t *testing.T, messages []string) string {
	t.Helper()
	if messages == nil {
		messages = []string{}
	}
	data, err := json.Marshal(messages)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func l11CompileCases(t *testing.T, api string, cases []l11Case) map[string]l11CompileResult {
	t.Helper()
	root := t.TempDir()
	keys := map[string]string{}
	for _, directory := range []string{"objects", "layouts", "classes"} {
		l07Copy(t, filepath.Join("testdata", "l11_runtime", "api"+api[:2]+"_original", "force-app", "main", "default", directory), filepath.Join(root, "fixtures", "force-app", "main", "default", directory))
	}
	for index, c := range cases {
		if c.FixtureXML != "" {
			continue
		}
		name := fmt.Sprintf("familyL11Compile%03d", index)
		rel := filepath.Join("cases", fmt.Sprintf("%04d", index), "force-app", "main", "default", "lwc", name)
		keys[c.ID] = filepath.ToSlash(rel)
		bundle := filepath.Join(root, rel)
		// The native bundle writer materializes the generated class name before
		// deployment. Replay it so diagnostic positions use identical source.
		js := strings.ReplaceAll(c.JS, "FamilyCompile", strings.ToUpper(name[:1])+name[1:])
		l07Write(t, filepath.Join(bundle, name+".js"), []byte(js))
		l07Write(t, filepath.Join(bundle, name+".html"), []byte(c.Template))
		fragment := c.MetaFragment
		if fragment == "" {
			fragment = "<isExposed>false</isExposed>"
		}
		l07Write(t, filepath.Join(bundle, name+".js-meta.xml"), []byte(`<?xml version="1.0" encoding="UTF-8"?><LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion>`+fragment+`</LightningComponentBundle>`))
	}
	l07Write(t, filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"cases","default":true},{"path":"fixtures"}],"namespace":"","sourceApiVersion":"`+api+`"}`))
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := compile.CompileBatch(p, compile.Options{OutDir: filepath.Join(root, "dist")})
	if err != nil {
		t.Fatal(err)
	}
	results := map[string]l11CompileResult{}
	for _, c := range cases {
		if c.FixtureXML != "" {
			// Exercise the product metadata loader, rather than interpreting the
			// native outcome or validating XML in the conformance adapter.
			metadataRoot := t.TempDir()
			l07Copy(t, filepath.Join("testdata", "l11_runtime", "api"+api[:2]+"_original"), metadataRoot)
			l07Write(t, filepath.Join(metadataRoot, "force-app", "main", "default", filepath.FromSlash(c.FixturePath)), []byte(c.FixtureXML))
			metadataProject, metadataErr := project.Load(metadataRoot)
			if metadataErr == nil {
				_, metadataErr = server.NewSourceMetadataFromProject(metadataProject)
			}
			results[c.ID] = l11CompileResult{Err: metadataErr, Diagnostics: l11ErrorMessages(metadataErr)}
			continue
		}
		result, ok := batch[keys[c.ID]]
		if !ok {
			t.Fatalf("missing L11 batch row %s", c.ID)
		}
		l07CheckCompilerEnvironment(t, result.Err)
		messages := append([]string{}, result.DeploymentDiagnostics...)
		if len(messages) == 0 {
			for _, diagnostic := range result.Diagnostics {
				messages = append(messages, diagnostic.Message)
			}
		}
		if len(messages) == 0 {
			messages = l11ErrorMessages(result.Err)
		}
		results[c.ID] = l11CompileResult{Err: result.Err, Diagnostics: messages}
	}
	return results
}

func l11ErrorMessages(err error) []string {
	if err == nil {
		return []string{}
	}
	for errors.Unwrap(err) != nil {
		err = errors.Unwrap(err)
	}
	return []string{err.Error()}
}

func l11ObserveDOM(t *testing.T, api, sourceDirectory string, cases []l11Case, dependencyRoot string) (map[string]json.RawMessage, map[string]string, error, error) {
	t.Helper()
	root := t.TempDir()
	l07Copy(t, filepath.Join("testdata", "l11_runtime", sourceDirectory), root)
	l11SeedNativeLayout(t, root, api)
	p, err := project.Load(root)
	if err != nil {
		return nil, nil, nil, err
	}
	manifest, err := compile.Compile(p, compile.Options{OutDir: filepath.Join(root, "dist")})
	l07CheckCompilerEnvironment(t, err)
	if err != nil {
		return nil, nil, err, nil
	}
	page := "familyL11Runtime" + api[:2]
	if _, ok := manifest.Modules["c:"+page]; !ok {
		return nil, nil, fmt.Errorf("runtime entry %s missing", page), nil
	}
	localManifest := lwcbrowser.Manifest{Modules: map[string]lwcbrowser.ModuleEntry{}}
	for qualified, module := range manifest.Modules {
		rel, err := filepath.Rel(manifest.OutDir, module.File)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, nil, nil, fmt.Errorf("L11 module escapes output: %s", module.File)
		}
		localManifest.Modules[qualified] = lwcbrowser.ModuleEntry{URL: "/lightning/modules/" + filepath.ToSlash(rel), Tag: module.Tag}
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
	index := typesys.Build(p, schema)
	org := apextest.OrgFromIndex(index)
	storage.EnsureDeterministicPlatformData(&org)
	if err := l11SeedNativeUserContext(&org, api); err != nil {
		return nil, nil, nil, err
	}
	if !strings.HasSuffix(sourceDirectory, "_original") {
		if err := l11SeedNativeObjectPrefixes(&org, api, cases); err != nil {
			return nil, nil, nil, err
		}
	}
	org.Metadata = registry
	if err := l11SeedNativeNavigationIdentity(&org.Metadata, api, cases); err != nil {
		return nil, nil, nil, err
	}
	product := server.NewWithSource(&org, source)
	runtime := vm.New(nil)
	if err := apextest.RegisterProjectRuntimeForRequest(runtime, index); err != nil {
		return nil, nil, nil, err
	}
	product.SetProjectRuntime(index, runtime, nil)
	if !strings.HasSuffix(sourceDirectory, "_original") {
		if err := l11SetUpNativeFixture(runtime, &org, root, api); err != nil {
			return nil, nil, nil, err
		}
		if err := l11SeedCapturedAuditFields(&org, api, cases); err != nil {
			return nil, nil, nil, err
		}
	}
	tab := "FamilyL11Oracle" + api[:2]
	shell := lwcshell.ShellPage{Context: lwcshell.PageContext{Kind: lwcshell.RenderTargetTab, TabName: tab, AppName: "FamilyL11App"}}
	workbench, err := json.Marshal(lwcshell.BuildWorkbenchModel(p, shell, "/lwc/preview/tab/"+tab, org.Metadata))
	if err != nil {
		return nil, nil, nil, err
	}
	known := map[string]bool{}
	specs := make([]map[string]string, len(cases))
	for index, c := range cases {
		known[c.ID] = true
		specs[index] = map[string]string{"id": c.ID, "adapter": c.Adapter}
	}
	mux := http.NewServeMux()
	mux.Handle("/lightning/", product) // Actual product Apex, wire and module routes.
	mux.Handle("/lightning/modules/", http.StripPrefix("/lightning/modules/", http.FileServer(http.Dir(manifest.OutDir))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("c__case")
		if r.URL.Path != "/" || !known[id] {
			http.NotFound(w, r)
			return
		}
		bootstrap := lwcbrowser.BootstrapHTML(lwcbrowser.PageConfig{Namespace: "c", Manifest: localManifest, PageReference: map[string]any{"type": "standard__navItemPage", "attributes": map[string]string{"apiName": tab}, "state": map[string]string{"c__case": id}}})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><script>window.__l11Error=null;</script><script id="glade-lwc-context" type="application/json">{}</script><script id="glade-lwc-workbench" type="application/json">%s</script><main id="l11-host"></main>%s<script>window.$Lightning.createComponent(%q,{},"l11-host",(_el,status,message)=>{if(status!=="SUCCESS")window.__l11Error={name:"Error",message};});</script>`, workbench, bootstrap, "c:"+page)
	})
	local := httptest.NewServer(mux)
	defer local.Close()
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(dependencyRoot, "lwcruntime", "node_modules", "playwright")
	}
	observer, err := filepath.Abs("testdata/l11_browser.mjs")
	if err != nil {
		return nil, nil, nil, err
	}
	config, err := json.Marshal(map[string]any{"url": local.URL, "specs": specs, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")})
	if err != nil {
		return nil, nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
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
	for id, raw := range observed.Values {
		if !known[id] || observed.Errors[id] != "" || !json.Valid(raw) {
			return nil, nil, nil, fmt.Errorf("unexpected or conflicting local L11 row %s", id)
		}
	}
	for id := range observed.Errors {
		if !known[id] {
			return nil, nil, nil, fmt.Errorf("unexpected local L11 error row %s", id)
		}
	}
	return observed.Values, observed.Errors, nil, nil
}

// Component-tab identities are org-generated inputs absent from source XML.
// Seed only the opaque identity from the captured normal navigation row; type,
// motif, URLs, selection and the complete response still come from product code.
func l11SeedNativeNavigationIdentity(metadata *storage.MetadataRegistry, api string, cases []l11Case) error {
	for _, c := range cases {
		if c.ID != "r_nav_normal" {
			continue
		}
		var native struct {
			Returned struct {
				Data struct {
					NavItems []struct {
						ObjectAPIName string `json:"objectApiName"`
					} `json:"navItems"`
				} `json:"data"`
			} `json:"returned"`
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(c.Expected[api], "BROWSER|")), &native); err != nil {
			return err
		}
		items := native.Returned.Data.NavItems
		if len(items) != 1 || items[0].ObjectAPIName == "" {
			return fmt.Errorf("L11 native tab identity missing at API %s", api)
		}
		tabName := "FamilyL11Oracle" + api[:2]
		for i := range metadata.Tabs {
			if metadata.Tabs[i].Name == tabName {
				metadata.Tabs[i].NavigationIdentity = items[0].ObjectAPIName
				return nil
			}
		}
		return fmt.Errorf("L11 captured tab %s missing from source metadata", tabName)
	}
	// The separate original-source replay contains only two related-record rows.
	return nil
}

// The native object has an org-generated child layout absent from the deployed
// project. Seed its captured Full Edit field membership/behavior as org input,
// just like the captured key prefix, user context and system timestamps.
func l11SeedNativeLayout(t *testing.T, root, api string) {
	t.Helper()
	data, err := os.ReadFile("testdata/l11_native_layouts/provenance.json")
	if err != nil {
		t.Fatal(err)
	}
	var versions map[string]struct {
		Oracle        string `json:"oracle"`
		Fixture       string `json:"fixture"`
		FixtureSHA256 string `json:"fixture_sha256"`
		Layout        struct {
			ObjectAPIName string `json:"objectApiName"`
			Mode          string `json:"mode"`
			LayoutType    string `json:"layoutType"`
		} `json:"layout"`
	}
	if err := json.Unmarshal(data, &versions); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := versions[api]
	if !ok || len(versions) != 2 || snapshot.Oracle == "" ||
		snapshot.Layout.ObjectAPIName != "FamilyL11Child__c" || snapshot.Layout.Mode != "Edit" || snapshot.Layout.LayoutType != "Full" {
		t.Fatalf("invalid native L11 layout input API %s", api)
	}
	name := filepath.Base(snapshot.Fixture)
	if name != snapshot.Layout.ObjectAPIName+"-Captured Full Edit.layout-meta.xml" {
		t.Fatal("invalid captured layout fixture path")
	}
	layout, err := os.ReadFile(filepath.Join("testdata", "l11_native_layouts", "api"+api[:2], name))
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(layout)) != snapshot.FixtureSHA256 {
		t.Fatal("captured layout fixture hash changed")
	}
	l07Write(t, filepath.Join(root, "force-app", "main", "default", "layouts", name), layout)
}

func l11SeedNativeObjectPrefixes(org *storage.OrgState, api string, cases []l11Case) error {
	var expected string
	for _, c := range cases {
		if c.ID == "r_relatedInfos_normal" {
			expected = c.Expected[api]
			break
		}
	}
	var native struct {
		Returned struct {
			Data struct {
				RelatedLists []struct {
					ObjectAPIName string `json:"objectApiName"`
					KeyPrefix     string `json:"keyPrefix"`
				} `json:"relatedLists"`
			} `json:"data"`
		} `json:"returned"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(expected, "BROWSER|")), &native); err != nil {
		return err
	}
	if len(native.Returned.Data.RelatedLists) == 0 {
		return fmt.Errorf("L11 native object identity fixture missing")
	}
	for _, entry := range native.Returned.Data.RelatedLists {
		object, ok := org.Objects[entry.ObjectAPIName]
		if !ok || len(entry.KeyPrefix) != 3 {
			return fmt.Errorf("L11 native object identity invalid for %s", entry.ObjectAPIName)
		}
		for name, other := range org.Objects {
			if name != entry.ObjectAPIName && other.Definition.KeyPrefix == entry.KeyPrefix {
				if storage.StandardKeyPrefix(name) != "" || len(other.Records) != 0 {
					return fmt.Errorf("L11 native object prefix collides with seeded or standard object %s", name)
				}
				// This empty object's prefix is a local allocation, not captured
				// org identity. Reserve the native prefix before reallocating it.
				other.Definition.KeyPrefix = ""
				org.Objects[name] = other
			}
		}
		object.Definition.KeyPrefix = entry.KeyPrefix
		org.Objects[entry.ObjectAPIName] = object
	}
	org.ClearRuntimeSchemaStamp()
	storage.EnsureUniqueKeyPrefixes(org)
	for _, entry := range native.Returned.Data.RelatedLists {
		if org.Objects[entry.ObjectAPIName].Definition.KeyPrefix != entry.KeyPrefix {
			return fmt.Errorf("L11 native object prefix allocation changed for %s", entry.ObjectAPIName)
		}
	}
	return nil
}

func TestL11NativeObjectPrefixReservation(t *testing.T) {
	data, err := os.ReadFile("testdata/l11_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []l11Case
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"59.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			// The fixture identities below come from r_relatedInfos_normal;
			// the unrelated placeholder has no asserted Salesforce identity.
			var native struct {
				Returned struct {
					Data struct {
						RelatedLists []struct {
							ObjectAPIName string `json:"objectApiName"`
							KeyPrefix     string `json:"keyPrefix"`
						} `json:"relatedLists"`
					} `json:"data"`
				} `json:"returned"`
			}
			for _, c := range cases {
				if c.ID == "r_relatedInfos_normal" {
					if err := json.Unmarshal([]byte(strings.TrimPrefix(c.Expected[api], "BROWSER|")), &native); err != nil {
						t.Fatal(err)
					}
				}
			}
			if len(native.Returned.Data.RelatedLists) != 1 {
				t.Fatal("missing native child identity")
			}
			entry := native.Returned.Data.RelatedLists[0]
			for _, mode := range []string{"empty", "seeded", "standard"} {
				t.Run(mode, func(t *testing.T) {
					org := storage.NewOrgState()
					org.Objects[entry.ObjectAPIName] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: entry.ObjectAPIName, KeyPrefix: "a00"}}
					name := "LocalUnseededPlaceholder"
					if mode == "standard" {
						name = "Account"
						if storage.StandardKeyPrefix(name) == "" {
							t.Fatal("standard-prefix guard fixture missing")
						}
					}
					placeholder := storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name, KeyPrefix: entry.KeyPrefix}}
					if mode == "seeded" {
						id := storage.ID(entry.KeyPrefix + "000000000001")
						placeholder.Records = map[storage.ID]storage.Record{id: {ID: id, Object: name}}
					}
					org.Objects[name] = placeholder
					err := l11SeedNativeObjectPrefixes(&org, api, cases)
					if mode != "empty" {
						if err == nil || org.Objects[name].Definition.KeyPrefix != entry.KeyPrefix {
							t.Fatalf("protected collision changed: %v", err)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					if org.Objects[entry.ObjectAPIName].Definition.KeyPrefix != entry.KeyPrefix ||
						org.Objects[name].Definition.KeyPrefix == entry.KeyPrefix || len(org.Objects[name].Definition.KeyPrefix) != 3 {
						t.Fatal("captured identity was not reserved during local placeholder allocation")
					}
				})
			}
		})
	}
}

func l11SeedNativeUserContext(org *storage.OrgState, api string) error {
	data, err := os.ReadFile("testdata/l11_user_context.json")
	if err != nil {
		return err
	}
	var captured map[string]struct {
		Settings map[string]string `json:"settings"`
	}
	if err := json.Unmarshal(data, &captured); err != nil {
		return err
	}
	context, ok := captured[api]
	if !ok || len(context.Settings) != 3 {
		return fmt.Errorf("L11 native user context missing at API %s", api)
	}
	user := org.Objects["User"]
	matches := 0
	for id, record := range user.Records {
		if !storage.IDsEqual(id, "005000000000001") {
			continue
		}
		for key, value := range context.Settings {
			if value == "" {
				return fmt.Errorf("L11 native user setting %s missing at API %s", key, api)
			}
			record.Fields[key] = storage.StringValue(value)
		}
		user.Records[id] = record
		matches++
	}
	if matches != 1 {
		return fmt.Errorf("L11 native user context matched %d local default users", matches)
	}
	org.Objects["User"] = user
	return nil
}

// Capture-time system values are fixture inputs, not comparison exemptions.
// Read them from the unchanged native legacy projection and seed the matching
// fixture records after replaying the exported setup invocation.
func l11SeedCapturedAuditFields(org *storage.OrgState, api string, cases []l11Case) error {
	var expected string
	for _, c := range cases {
		if c.ID == "r_legacy_normal" {
			expected = c.Expected[api]
			break
		}
	}
	var native struct {
		Returned struct {
			Data struct {
				Records struct {
					Records []struct {
						APIName        string `json:"apiName"`
						SystemModstamp string `json:"systemModstamp"`
						Fields         map[string]struct {
							Value any `json:"value"`
						} `json:"fields"`
					} `json:"records"`
				} `json:"records"`
			} `json:"data"`
		} `json:"returned"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(expected, "BROWSER|")), &native); err != nil {
		return fmt.Errorf("L11 captured audit fixture: %w", err)
	}
	if len(native.Returned.Data.Records.Records) != 3 {
		return fmt.Errorf("L11 captured audit fixture missing records")
	}
	for _, row := range native.Returned.Data.Records.Records {
		name, nameOK := row.Fields["Name"].Value.(string)
		created, createdOK := row.Fields["CreatedDate"].Value.(string)
		modified, modifiedOK := row.Fields["LastModifiedDate"].Value.(string)
		if !nameOK || !createdOK || !modifiedOK || row.SystemModstamp == "" {
			return fmt.Errorf("L11 captured audit fixture missing values")
		}
		object := org.Objects[row.APIName]
		matches := 0
		for id, record := range object.Records {
			if record.Fields["Name"].String != name {
				continue
			}
			record.System.CreatedDate = created
			record.System.LastModifiedDate = modified
			record.System.SystemModstamp = row.SystemModstamp
			object.Records[id] = record
			matches++
		}
		if matches != 1 {
			return fmt.Errorf("L11 audit fixture %s.%s matched %d records", row.APIName, name, matches)
		}
		org.Objects[row.APIName] = object
	}
	return nil
}

func l11SetUpNativeFixture(runtime *vm.VM, org *storage.OrgState, root, api string) error {
	// The retry capture set up its records once before opening parallel pages.
	// Replay that exported call in the shared project runtime; prepare stays read-only.
	setup, err := os.ReadFile(filepath.Join(root, "fixture-actions", "setup.apex"))
	if err != nil {
		return err
	}
	setupAPI := api
	if api == "59.0" {
		setupAPI = "62.0" // The native setup invocation used the Apex floor.
	}
	program, err := vm.CompileAnonymousWithOptions(string(setup), vm.CompileOptions{APIVersion: setupAPI})
	if err != nil {
		return fmt.Errorf("L11 native fixture setup compile: %w", err)
	}
	machine := runtime.CloneRuntime(nil)
	machine.SetOrg(org)
	machine.SetToolingExecuteAnonymous(true)
	result, err := machine.Execute(program)
	if err != nil {
		return fmt.Errorf("L11 native fixture setup execution: %w", err)
	}
	if err := machine.DrainAsync(&result); err != nil {
		return fmt.Errorf("L11 native fixture setup async: %w", err)
	}
	return nil
}
