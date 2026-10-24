package compile_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
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

// TestL11SalesforceConformance replays all 285 captured rows at API 59/67/68:
// 64 LWC compile, eight ListView metadata and 213 native DOM observations.
// Runtime inputs retain the original API 67 null/empty related-record reads.
// API 68 uses capture-repair's 72 compiler and 213 browser answers.
// Fresh ctrl_metadata_child_edit and user context bind the layout/locale inputs.
// Enumerated API 68 request IDs and release URLs are qualified separately from
// exact matches; every other response field remains exact.
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
	versions := []string{"59.0", "67.0", "68.0"}
	context := l11ReadAPI68Context(t, cases)
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
	qualifiedRows, qualifiedIDs, qualifiedURLs := 0, 0, 0
	// The original first compile initializes any missing shared toolchain before
	// concurrent runtime compiles read those assets, with the same fatal errors.
	firstCompiled := l11CompileCases(t, versions[0], compileCases)
	parallel := browserAPIParallelism(len(versions))
	// Each observation owns its temp project/output, org, source metadata, VM,
	// server port and browser context. Captured layouts are read-only; list view
	// and preference mutations write only that observation's temporary project.
	// No environment is mutated. Keep source/group order within each API pass.
	observe := func(t *testing.T, pass int) []l11DOMObservation {
		api := versions[pass]
		var observations []l11DOMObservation
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
			groups := [][]l11Case{selected}
			if api == "68.0" {
				for index, c := range selected {
					if c.ID == "r_relatedRecords_normal" {
						// This retained row predates the legacy audit snapshot.
						// Separate orgs keep concurrent browser reads isolated.
						others := append([]l11Case{}, selected[:index]...)
						others = append(others, selected[index+1:]...)
						groups = [][]l11Case{{c}, others}
						break
					}
				}
			}
			for _, rows := range groups {
				values, failures, compileErr, browserErr := l11ObserveDOM(t, api, source, rows, selected, dependencyRoot, parallel == 1)
				if parallel == 1 {
					// Stop immediately, before observing another group or API.
					l07CheckCompilerEnvironment(t, compileErr)
					if browserErr != nil {
						t.Fatalf("L11 browser environment: %v", browserErr)
					}
				}
				observations = append(observations, l11DOMObservation{
					Rows: rows, Values: values, Errors: failures,
					CompileErr: compileErr, BrowserErr: browserErr,
				})
				if browserErr != nil {
					return observations
				}
			}
		}
		return observations
	}
	var observedPasses [][]l11DOMObservation
	if parallel > 1 {
		observedPasses = runBrowserPasses(t, versions, parallel, observe)
	}
	for pass, api := range versions {
		compiled := firstCompiled
		if pass != 0 {
			compiled = l11CompileCases(t, api, compileCases)
		}
		var observations []l11DOMObservation
		if parallel == 1 {
			// Keep compile/observe/report interleaved in the legacy capped mode.
			observations = observe(t, pass)
		} else {
			observations = observedPasses[pass]
		}
		dom, domErrors, runtimeCompileErrors := map[string]json.RawMessage{}, map[string]string{}, map[string]error{}
		for _, observed := range observations {
			// Fail and report on the test goroutine, in the original API/group order.
			l07CheckCompilerEnvironment(t, observed.CompileErr)
			if observed.BrowserErr != nil {
				t.Fatalf("L11 browser environment: %v", observed.BrowserErr)
			}
			for id, value := range observed.Values {
				dom[id] = value
			}
			for id, failure := range observed.Errors {
				domErrors[id] = failure
			}
			for _, c := range observed.Rows {
				runtimeCompileErrors[c.ID] = observed.CompileErr
			}
		}
		apiMatches, apiDOMRows, apiQualified := 0, 0, 0
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
			} else {
				if api == "68.0" && (len(context.ErrorIDPaths[c.ID]) != 0 || len(context.ReleaseURLPaths[c.ID]) != 0) {
					ids, urls, err := context.compare(c.ID, got, want)
					if err == nil {
						status, reason = "QUALIFIED", "L11 API 68: enumerated request-ID presence/type and release URL prefix; all remaining JSON exact"
						qualifiedRows++
						apiQualified++
						qualifiedIDs += ids
						qualifiedURLs += urls
					} else {
						reason += "; " + err.Error()
					}
				}
				if status == "MISMATCH" && !capture {
					t.Errorf("%s API %s expected <%s> actual <%s>: %s", c.ID, api, want, got, l07OneLine(reason))
				}
			}
			fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, c.Kind, status, l07OneLine(got), l07OneLine(want), l07OneLine(reason), diagnosticStatus, l07OneLine(diagnosticGot), l07OneLine(diagnosticWant), l07OneLine(diagnosticReason), c.Oracle[api])
		}
		fmt.Fprintf(&report, "API_TOTAL\t%s\t%d/%d\nAPI_QUALIFIED\t%s\t%d\nLOCAL_DOM_ROWS\t%s\t%d/%d\n", api, apiMatches, len(cases), api, apiQualified, api, apiDOMRows, len(runtimeCases))
		t.Logf("L11 API %s exact matches %d/%d; qualified %d; observed local DOM %d/%d", api, apiMatches, len(cases), apiQualified, apiDOMRows, len(runtimeCases))
	}
	fmt.Fprintf(&report, "TOTAL\t%d/%d\nCOMPILE_TOTAL\t%d/%d\nMETADATA_TOTAL\t%d/%d\nDOM_TOTAL\t%d/%d\nDIAGNOSTIC_TOTAL\t%d/%d\nLOCAL_DOM_ROWS\t%d/%d\n", matches, len(cases)*len(versions), compileMatches, len(compileCases)*len(versions), metadataMatches, metadataRows*len(versions), domMatches, len(runtimeCases)*len(versions), diagnosticMatches, diagnosticRows*len(versions), localDOMRows, len(runtimeCases)*len(versions))
	t.Logf("L11 matches %d/%d; compile/metadata %d/%d; DOM %d/%d; diagnostics %d/%d", matches, len(cases)*len(versions), compileMatches, len(compileCases)*len(versions), domMatches, len(runtimeCases)*len(versions), diagnosticMatches, diagnosticRows*len(versions))
	fmt.Fprintf(&report, "QUALIFIED_TOTAL\t%d\nQUALIFIED_ERROR_ID_PATHS\t%d\nQUALIFIED_RELEASE_URL_PATHS\t%d\n", qualifiedRows, qualifiedIDs, qualifiedURLs)
	t.Logf("L11 qualified %d rows separately from exact matches; request-ID paths %d; release URL paths %d", qualifiedRows, qualifiedIDs, qualifiedURLs)
	if path := os.Getenv("GLADE_L11_REPORT"); path != "" {
		l07Write(t, path, []byte(report.String()))
	}
}

type l11API68Context struct {
	API                 string              `json:"api"`
	NativeReleasePrefix string              `json:"native_release_url_prefix"`
	LocalReleasePrefix  string              `json:"local_release_url_prefix"`
	ErrorIDPaths        map[string][]string `json:"error_id_paths"`
	ReleaseURLPaths     map[string][]string `json:"release_url_paths"`
}

func l11ReadAPI68Context(t *testing.T, cases []l11Case) l11API68Context {
	t.Helper()
	data, err := os.ReadFile("testdata/l11_api68_context.json")
	if err != nil {
		t.Fatal(err)
	}
	var context l11API68Context
	if err := json.Unmarshal(data, &context); err != nil {
		t.Fatal(err)
	}
	if context.API != "68.0" || context.NativeReleasePrefix != "/services/data/v68.0/" || context.LocalReleasePrefix != "/services/data/v67.0/" || len(context.ErrorIDPaths) != 34 || len(context.ReleaseURLPaths) != 10 {
		t.Fatal("invalid native L11 API 68 context scope")
	}
	runtime := map[string]bool{}
	for _, c := range cases {
		runtime[c.ID] = c.Kind == "runtime"
	}
	for kind, rows := range map[string]map[string][]string{"request-ID": context.ErrorIDPaths, "release URL": context.ReleaseURLPaths} {
		count := 0
		for id, paths := range rows {
			if !runtime[id] || len(paths) == 0 || (kind == "request-ID" && len(context.ReleaseURLPaths[id]) != 0) {
				t.Fatalf("invalid L11 API 68 context row %s", id)
			}
			seen := map[string]bool{}
			for _, path := range paths {
				if seen[path] || !strings.HasPrefix(path, "/") || (kind == "request-ID" && !strings.HasSuffix(path, "/body/id")) || (kind == "release URL" && !l11API68ReleaseURLPath(path)) {
					t.Fatalf("invalid L11 API 68 %s path %s: %s", kind, id, path)
				}
				seen[path] = true
				count++
			}
		}
		if count != 49 {
			t.Fatalf("L11 API 68 requires 49 observed %s paths, got %d", kind, count)
		}
	}
	return context
}

func l11API68ReleaseURLPath(path string) bool {
	return strings.HasPrefix(path, "/") && (strings.HasSuffix(path, "/relatedListInfoUrl") || strings.HasSuffix(path, "/url") || strings.HasSuffix(path, "/listUiUrl"))
}

var l11NativeRequestID = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func TestL11API68ContextGuards(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"/returned/data/relatedLists/0/relatedListInfoUrl", true},
		{"/returned/data/listViews/0/url", true},
		{"/returned/data/listUiUrl", true},
		{"/returned/data/resourceUrl", false},
	} {
		t.Run(tc.path, func(t *testing.T) {
			if got := l11API68ReleaseURLPath(tc.path); got != tc.want {
				t.Fatalf("release URL path allowed=%t, want %t", got, tc.want)
			}
		})
	}
	context := l11API68Context{ErrorIDPaths: map[string][]string{"row": {"/returned/error/body/id"}}}
	actual := `BROWSER|{"returned":{"error":{"body":{"id":"local-short-id"}}}}`
	for _, tc := range []struct {
		id   string
		want bool
	}{
		{"af93662b-a647-42c2-a405-f4b5263c8afb", true},
		{"native-non-uuid", false},
	} {
		t.Run(tc.id, func(t *testing.T) {
			expected := `BROWSER|{"returned":{"error":{"body":{"id":"` + tc.id + `"}}}}`
			ids, urls, err := context.compare("row", actual, expected)
			if (err == nil) != tc.want || (tc.want && (ids != 1 || urls != 0)) {
				t.Fatalf("request-ID guard: paths=%d/%d error=%v", ids, urls, err)
			}
		})
	}
}

// compare visits complete JSON without rewriting either captured or local text.
// Only the enumerated leaves have qualified assertions; keys, arrays, nulls and
// every other leaf must agree exactly. Missing context paths fail the row.
func (c l11API68Context) compare(id, actual, expected string) (int, int, error) {
	decode := func(text string) (any, error) {
		if !strings.HasPrefix(text, "BROWSER|") {
			return nil, fmt.Errorf("missing browser observation")
		}
		decoder := json.NewDecoder(strings.NewReader(strings.TrimPrefix(text, "BROWSER|")))
		decoder.UseNumber()
		var value any
		err := decoder.Decode(&value)
		return value, err
	}
	got, err := decode(actual)
	if err != nil {
		return 0, 0, err
	}
	want, err := decode(expected)
	if err != nil {
		return 0, 0, err
	}
	paths := map[string]string{}
	for _, path := range c.ErrorIDPaths[id] {
		paths[path] = "request-ID"
	}
	for _, path := range c.ReleaseURLPaths[id] {
		paths[path] = "release URL"
	}
	ids, urls := 0, 0
	var visit func(any, any, string) error
	visit = func(got, want any, path string) error {
		if kind := paths[path]; kind != "" {
			g, gok := got.(string)
			w, wok := want.(string)
			if !gok || !wok || g == "" || w == "" {
				return fmt.Errorf("%s must be a present nonempty string at %s", kind, path)
			}
			if kind == "request-ID" {
				if !l11NativeRequestID.MatchString(w) {
					return fmt.Errorf("native request-ID must have UUID shape at %s", path)
				}
				ids++
				return nil
			}
			if !strings.HasPrefix(g, c.LocalReleasePrefix) || !strings.HasPrefix(w, c.NativeReleasePrefix) || strings.TrimPrefix(g, c.LocalReleasePrefix) != strings.TrimPrefix(w, c.NativeReleasePrefix) {
				return fmt.Errorf("release URL prefix or exact suffix/query differs at %s", path)
			}
			urls++
			return nil
		}
		switch w := want.(type) {
		case map[string]any:
			g, ok := got.(map[string]any)
			if !ok || len(g) != len(w) {
				return fmt.Errorf("object keys differ at %s", path)
			}
			keys := make([]string, 0, len(w))
			for key := range w {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				value, ok := g[key]
				if !ok {
					return fmt.Errorf("missing key at %s/%s", path, key)
				}
				pointer := strings.NewReplacer("~", "~0", "/", "~1").Replace(key)
				if err := visit(value, w[key], path+"/"+pointer); err != nil {
					return err
				}
			}
		case []any:
			g, ok := got.([]any)
			if !ok || len(g) != len(w) {
				return fmt.Errorf("array differs at %s", path)
			}
			for i := range w {
				if err := visit(g[i], w[i], fmt.Sprintf("%s/%d", path, i)); err != nil {
					return err
				}
			}
		default:
			if !reflect.DeepEqual(got, want) {
				return fmt.Errorf("value differs at %s", path)
			}
		}
		return nil
	}
	if err := visit(got, want, ""); err != nil {
		return 0, 0, err
	}
	if ids+urls != len(paths) {
		return 0, 0, fmt.Errorf("qualified context path missing")
	}
	return ids, urls, nil
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

// l11DOMObservation carries failures back to the test goroutine without FailNow.
type l11DOMObservation struct {
	Rows       []l11Case
	Values     map[string]json.RawMessage
	Errors     map[string]string
	CompileErr error
	BrowserErr error
}

// Match l07Copy/l07Write's file traversal and modes without failing a worker.
func l11CopyRuntime(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
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
		return l11WriteRuntimeFile(filepath.Join(target, rel), data)
	})
}

func l11WriteRuntimeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func l11ObserveDOM(t *testing.T, api, sourceDirectory string, cases, fixtureCases []l11Case, dependencyRoot string, inline bool) (map[string]json.RawMessage, map[string]string, error, error) {
	t.Helper()
	root := t.TempDir()
	if err := l11CopyRuntime(filepath.Join("testdata", "l11_runtime", sourceDirectory), root); err != nil {
		if inline {
			t.Fatal(err)
		}
		return nil, nil, nil, err
	}
	if err := l11SeedNativeLayout(root, api); err != nil {
		if inline {
			t.Fatal(err)
		}
		return nil, nil, nil, err
	}
	p, err := project.Load(root)
	if err != nil {
		return nil, nil, nil, err
	}
	manifest, err := compile.Compile(p, compile.Options{OutDir: filepath.Join(root, "dist")})
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
		if err := l11SeedNativeObjectPrefixes(&org, api, fixtureCases); err != nil {
			return nil, nil, nil, err
		}
	}
	org.Metadata = registry
	if err := l11SeedNativeNavigationIdentity(&org.Metadata, api, fixtureCases); err != nil {
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
		if err := l11SeedCapturedAuditFields(&org, api, fixtureCases, cases); err != nil {
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
func l11SeedNativeLayout(root, api string) error {
	data, err := os.ReadFile("testdata/l11_native_layouts/provenance.json")
	if err != nil {
		return err
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
		return err
	}
	snapshot, ok := versions[api]
	if !ok || len(versions) != 3 || snapshot.Oracle == "" ||
		snapshot.Layout.ObjectAPIName != "FamilyL11Child__c" || snapshot.Layout.Mode != "Edit" || snapshot.Layout.LayoutType != "Full" {
		return fmt.Errorf("invalid native L11 layout input API %s", api)
	}
	name := filepath.Base(snapshot.Fixture)
	if name != snapshot.Layout.ObjectAPIName+"-Captured Full Edit.layout-meta.xml" {
		return fmt.Errorf("invalid captured layout fixture path")
	}
	layout, err := os.ReadFile(filepath.Join("testdata", "l11_native_layouts", "api"+api[:2], name))
	if err != nil {
		return err
	}
	if fmt.Sprintf("%x", sha256.Sum256(layout)) != snapshot.FixtureSHA256 {
		return fmt.Errorf("captured layout fixture hash changed")
	}
	return l11WriteRuntimeFile(filepath.Join(root, "force-app", "main", "default", "layouts", name), layout)
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
		if !storage.IDsEqual(id, storage.DefaultUserID) {
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
// Read them from the unchanged native projections and seed the matching fixture
// records after replaying setup. The isolated earlier API 68 related-record row
// retains its own systemModstamp; other reads use the legacy audit snapshot.
func l11SeedCapturedAuditFields(org *storage.OrgState, api string, cases, replay []l11Case) error {
	var expected string
	for _, c := range cases {
		if c.ID == "r_legacy_normal" {
			expected = c.Expected[api]
			break
		}
	}
	type auditRecord struct {
		APIName        string `json:"apiName"`
		SystemModstamp string `json:"systemModstamp"`
		Fields         map[string]struct {
			Value any `json:"value"`
		} `json:"fields"`
	}
	var native struct {
		Returned struct {
			Data struct {
				Records struct {
					Records []auditRecord `json:"records"`
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
	contextStamps := map[string]string{}
	if api == "68.0" && len(replay) == 1 && replay[0].ID == "r_relatedRecords_normal" {
		var earlier struct {
			Returned struct {
				Data struct {
					Records []auditRecord `json:"records"`
				} `json:"data"`
			} `json:"returned"`
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(replay[0].Expected[api], "BROWSER|")), &earlier); err != nil {
			return fmt.Errorf("L11 earlier audit fixture: %w", err)
		}
		for _, row := range earlier.Returned.Data.Records {
			name, ok := row.Fields["Name"].Value.(string)
			if !ok || row.APIName != "FamilyL11Child__c" || row.SystemModstamp == "" || contextStamps[name] != "" {
				return fmt.Errorf("L11 earlier audit fixture missing or duplicate values")
			}
			contextStamps[name] = row.SystemModstamp
		}
		if len(contextStamps) != 3 {
			return fmt.Errorf("L11 earlier audit fixture missing records")
		}
	}
	for _, row := range native.Returned.Data.Records.Records {
		name, nameOK := row.Fields["Name"].Value.(string)
		created, createdOK := row.Fields["CreatedDate"].Value.(string)
		modified, modifiedOK := row.Fields["LastModifiedDate"].Value.(string)
		if !nameOK || !createdOK || !modifiedOK || row.SystemModstamp == "" {
			return fmt.Errorf("L11 captured audit fixture missing values")
		}
		if len(contextStamps) != 0 {
			stamp, ok := contextStamps[name]
			if !ok {
				return fmt.Errorf("L11 earlier audit fixture has no matching record %s", name)
			}
			row.SystemModstamp = stamp
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
