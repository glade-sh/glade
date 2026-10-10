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

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
)

type l22Case struct {
	ID                 string              `json:"id"`
	Group              string              `json:"group"`
	Kind               string              `json:"kind"`
	Basis              string              `json:"basis"`
	JS                 string              `json:"js"`
	Template           string              `json:"template"`
	Meta               string              `json:"meta_fragment"`
	OmitVersion        bool                `json:"omit_version"`
	Spec               json.RawMessage     `json:"spec"`
	Expected           map[string]string   `json:"expected"`
	DiagnosticExpected map[string][]string `json:"diagnostic_expected"`
	BundleName         string              `json:"bundle_name"`
	NativeControl      string              `json:"native_control"`
}

type l22Runtime struct {
	Page  string            `json:"page"`
	Files map[string]string `json:"files"`
}

// TestL22SalesforceConformance compares every exported API 59/67 org.tsv row
// the three ruled observer timeout-message strings are excluded from equality.
// Capture mode measures the accepted tree without failing on behavior gaps.
// Only owned inputs and local connection settings reach the browser; CI never
// this adapter use the product browser modules without a compiler import cycle.
func TestL22SalesforceConformance(t *testing.T) {
	t.Run("HostRegistryAdmission", l22CheckHostRegistryAdmission)
	capture := os.Getenv("GLADE_L22_CAPTURE") != ""
	var cases []l22Case
	l22Read(t, "testdata/l22_salesforce.json", &cases)
	if len(cases) != 262 {
		t.Fatalf("L22 requires 262 primary native rows, got %d", len(cases))
	}
	var controls []l22Case
	l22Read(t, "testdata/l22_native_controls.json", &controls)
	if len(controls) != 15 {
		t.Fatalf("L22 requires 15 isolated native controls, got %d", len(controls))
	}
	controlIDs := map[string]bool{}
	for _, c := range controls {
		controlIDs[c.ID] = true
	}
	cases = append(cases, controls...)
	var fixtures map[string]l22Runtime
	l22Read(t, "testdata/l22_runtime.json", &fixtures)
	seen, groups := map[string]bool{}, map[string]bool{}
	var compilerCases, runtimeCases []l22Case
	for _, c := range cases {
		if c.ID == "" || seen[c.ID] || c.Basis != "org" {
			t.Fatalf("invalid or duplicate L22 row %q", c.ID)
		}
		seen[c.ID], groups[c.Group] = true, true
		for _, api := range []string{"59.0", "67.0"} {
			want, ok := c.Expected[api]
			if !ok {
				t.Fatalf("missing native L22 answer %s API %s", c.ID, api)
			}
			if c.Kind == "compile" {
				if want != "COMPILE_OK" && want != "COMPILE_ERROR" {
					t.Fatalf("invalid native compile answer %s API %s", c.ID, api)
				}
				diagnostics, ok := c.DiagnosticExpected[api]
				if !ok || (want == "COMPILE_ERROR") != (len(diagnostics) != 0) {
					t.Fatalf("missing native rejection diagnostic %s API %s", c.ID, api)
				}
			} else if c.Kind == "runtime" {
				var native struct {
					ID    string `json:"id"`
					Stage string `json:"stage"`
				}
				if !strings.HasPrefix(want, "BROWSER|") || json.Unmarshal([]byte(strings.TrimPrefix(want, "BROWSER|")), &native) != nil || native.ID != c.ID || native.Stage != "done" {
					t.Fatalf("invalid native DOM answer %s API %s", c.ID, api)
				}
			} else {
				t.Fatalf("unknown L22 kind %q", c.Kind)
			}
		}
		if c.Kind == "compile" {
			if c.JS == "" || c.Template == "" {
				t.Fatalf("missing compile input %s", c.ID)
			}
			compilerCases = append(compilerCases, c)
		} else {
			var spec struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(c.Spec, &spec) != nil || spec.ID != c.ID {
				t.Fatalf("invalid runtime input %s", c.ID)
			}
			runtimeCases = append(runtimeCases, c)
		}
	}
	if len(compilerCases) != 72 || len(runtimeCases) != 205 {
		t.Fatalf("L22 requires 60+12 compile/metadata and 202+3 DOM rows, got %d/%d", len(compilerCases), len(runtimeCases))
	}
	for _, group := range []string{"modal lifecycle", "alert/confirm/prompt", "toast"} {
		if !groups[group] {
			t.Fatalf("missing L22 group %s", group)
		}
	}
	repo, err := gladehome.SourceRoot()
	if err != nil {
		t.Fatal(err)
	}
	dependencies, err := gladehome.EnsureRoot()
	if err != nil {
		t.Fatal(err)
	}
	// Toolchain failures are infrastructure errors, never compile rejections.
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{
		filepath.Join(repo, "third_party/lwc/compile.mjs"),
		filepath.Join(dependencies, "third_party/lwc/node_modules/@lwc/compiler/package.json"),
		filepath.Join(dependencies, "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js"),
		filepath.Join(dependencies, "third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js"),
	} {
		if _, err := os.Stat(file); err != nil {
			t.Fatal(err)
		}
	}
	var report strings.Builder
	fmt.Fprintln(&report, "api\tid\tkind\tstatus\tactual\texpected\treason")
	matches, compileMatches, domMatches, domRows := 0, 0, 0, 0
	diagnosticMatches, diagnosticRows := 0, 0
	controlMatches := 0
	for _, api := range []string{"59.0", "67.0"} {
		compiled := l22CompileCases(t, api, compilerCases)
		fixture, ok := fixtures[api]
		if !ok || fixture.Page != "familyL22Runtime"+api[:2] {
			t.Fatalf("missing captured runtime fixture API %s", api)
		}
		// Append only captured control inputs to the otherwise unchanged native
		// runtime protocol. Expected output never reaches the browser.
		fixture = l22WithControlSpecs(t, fixture, controls)
		dom, errors, runtimeErr := l22ObserveDOM(t, api, runtimeCases, fixture, repo, dependencies)
		compileIndex, apiMatches, apiDOMRows := 0, 0, 0
		for _, c := range cases {
			want, got, reason := c.Expected[api], "COMPILE_OK", ""
			if c.Kind == "compile" {
				result := compiled[compileIndex]
				if compileErr := result.Err; compileErr != nil {
					got, reason = "COMPILE_ERROR", compileErr.Error()
				}
				{
					native := c.DiagnosticExpected[api]
					if len(native) != 0 {
						diagnosticRows++
					}
					observed := append([]string{}, result.DeploymentDiagnostics...)
					for i := range observed {
						// The native capture transport redacts URLs. Apply the same
						// transport rule; retain every diagnostic character and position.
						observed[i] = l22DiagnosticURL.ReplaceAllString(observed[i], "<URL>")
					}
					actualText, expectedText := l22DiagnosticText(t, observed), l22DiagnosticText(t, native)
					status, diagnosticReason := "MISMATCH", "L22: rejection diagnostic differs from native"
					if actualText == expectedText {
						status, diagnosticReason = "MATCH", ""
						if len(native) != 0 {
							diagnosticMatches++
						}
					} else if !capture {
						t.Errorf("%s API %s diagnostic expected <%s> actual <%s>", c.ID, api, expectedText, actualText)
					}
					fmt.Fprintf(&report, "%s\t%s\tdiagnostic\t%s\t%s\t%s\t%s\n", api, c.ID, status, l22OneLine(actualText), l22OneLine(expectedText), diagnosticReason)
				}
				compileIndex++
			} else if value, ok := dom[c.ID]; ok {
				got = value // Browser serializes observed JSON exactly as native org.tsv.
				if !controlIDs[c.ID] {
					domRows++
					apiDOMRows++
				}
			} else {
				got, reason = "BROWSER_ERROR", errors[c.ID]
				if runtimeErr != nil {
					reason = runtimeErr.Error()
				}
			}
			status := "MISMATCH"
			matching, observerReason := l22CompareRows(c.ID, got, want)
			if observerReason != "" {
				reason = observerReason
				t.Logf("L22 observer exclusion %s API %s: %s", c.ID, api, observerReason)
			}
			if matching {
				status = "MATCH"
				if controlIDs[c.ID] {
					controlMatches++
				} else {
					matches++
					apiMatches++
					if c.Kind == "compile" {
						compileMatches++
					} else {
						domMatches++
					}
				}
			} else {
				if observerReason != "" {
					reason += "; remaining raw row differs from native"
				} else if reason == "" {
					reason = "L22: local observation differs from native"
				}
				if !capture {
					t.Errorf("%s API %s expected <%s> actual <%s>: %s", c.ID, api, want, got, reason)
				}
			}
			fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, c.Kind, status, l22OneLine(got), l22OneLine(want), l22OneLine(reason))
		}
		fmt.Fprintf(&report, "API_TOTAL\t%s\t%d/262\nLOCAL_DOM_ROWS\t%s\t%d/202\n", api, apiMatches, api, apiDOMRows)
		fmt.Fprintf(&report, "API_CARRIED\t%s\t%d/262\n", api, 0)
		t.Logf("L22 API %s matches %d/262; local DOM rows %d/202; carried %d/262", api, apiMatches, apiDOMRows, 0)
	}
	fmt.Fprintf(&report, "TOTAL\t%d/524\nCOMPILE_TOTAL\t%d/120\nDOM_TOTAL\t%d/404\nLOCAL_DOM_ROWS\t%d/404\n", matches, compileMatches, domMatches, domRows)
	fmt.Fprintf(&report, "DIAGNOSTIC_TOTAL\t%d/%d\nCARRIED_TOTAL\t%d/524\nCONTROL_TOTAL\t%d/30\nCONTROL_CARRIED\t%d/30\n", diagnosticMatches, diagnosticRows, 0, controlMatches, 0)
	t.Logf("L22 matches %d/524; compile/metadata %d/120; native-vs-local DOM %d/404; observed DOM rows %d/404", matches, compileMatches, domMatches, domRows)
	t.Logf("L22 exact native rejection diagnostics %d/%d (primary plus isolated controls)", diagnosticMatches, diagnosticRows)
	t.Logf("L22 carried %d/524; isolated controls %d/30 matching, %d/30 carried", 0, controlMatches, 0)
	if path := os.Getenv("GLADE_L22_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

var l22DiagnosticURL = regexp.MustCompile(`https?://[^\s"'<>]+`)

// Only these observer TimeoutError.message values are excluded from exact
// comparison; their complete, unmodified answers remain in the fixture and row
// report.
var l22ObserverTimeoutReasons = map[string]string{
	"r_modal_disable_true_dismiss":         "observer TimeoutError.message excluded: native API 59 row 104 has 5 final retries; API 67 row 104 has 6",
	"r_modal_disable_string_false_dismiss": "observer TimeoutError.message excluded: native API 59 row 110 has 6 final retries; API 67 row 110 has 5",
	"r_modal_disable_number_dismiss":       "observer TimeoutError.message excluded: native API 59 row 112 has 6 final retries; API 67 row 112 has 5",
}

func l22CompareRows(id, actual, expected string) (bool, string) {
	reason, allowed := l22ObserverTimeoutReasons[id]
	if !allowed {
		return actual == expected, ""
	}
	actualStart, actualEnd, actualOK := l22ObserverTimeoutMessageSpan(actual)
	expectedStart, expectedEnd, expectedOK := l22ObserverTimeoutMessageSpan(expected)
	if !actualOK || !expectedOK {
		return actual == expected, ""
	}
	// Compare original bytes on both sides of only the JSON string value. Do
	// not reserialize, normalize text, remove other fields, or substitute null.
	return actual[:actualStart] == expected[:expectedStart] &&
		actual[actualEnd:] == expected[expectedEnd:], reason
}

func l22ObserverTimeoutMessageSpan(row string) (int, int, bool) {
	if !strings.HasPrefix(row, "BROWSER|") {
		return 0, 0, false
	}
	var browser struct {
		Observation struct {
			BrowserActions []struct {
				ActionError json.RawMessage `json:"actionError"`
			} `json:"browserActions"`
		} `json:"observation"`
	}
	if json.Unmarshal([]byte(strings.TrimPrefix(row, "BROWSER|")), &browser) != nil || len(browser.Observation.BrowserActions) != 1 {
		return 0, 0, false
	}
	actionError := browser.Observation.BrowserActions[0].ActionError
	var failure struct {
		Name    string          `json:"name"`
		Message json.RawMessage `json:"message"`
	}
	if json.Unmarshal(actionError, &failure) != nil || failure.Name != "TimeoutError" || len(failure.Message) == 0 || failure.Message[0] != '"' {
		return 0, 0, false
	}
	// Native and local observers emit compact JSON. Locate this exact parsed
	// actionError object once, then its exact message member once. Ambiguous or
	// differently formatted rows receive no exception and use full equality.
	errorMember := `"actionError":` + string(actionError)
	messageMember := `"message":` + string(failure.Message)
	if strings.Count(row, errorMember) != 1 || strings.Count(string(actionError), messageMember) != 1 {
		return 0, 0, false
	}
	start := strings.Index(row, errorMember) + len(`"actionError":`) +
		strings.Index(string(actionError), messageMember) + len(`"message":`)
	return start, start + len(failure.Message), true
}

func TestL22ObserverTimeoutComparison(t *testing.T) {
	var cases []l22Case
	l22Read(t, "testdata/l22_salesforce.json", &cases)
	checked := 0
	for _, c := range cases {
		if _, allowed := l22ObserverTimeoutReasons[c.ID]; !allowed {
			continue
		}
		checked++
		t.Run(c.ID, func(t *testing.T) {
			floor, ceiling := c.Expected["59.0"], c.Expected["67.0"]
			if floor == ceiling {
				t.Fatal("native timeout captures must disagree before the exception")
			}
			if matching, reason := l22CompareRows(c.ID, ceiling, floor); !matching || reason == "" {
				t.Fatal("the two captured native rows must match except for the observer timeout message")
			}
			for _, change := range []struct{ name, before, after string }{
				{"action", `"performed":false`, `"performed":true`},
				{"modal state", `"stillOpen":true`, `"stillOpen":false`},
				{"DOM", "Processing", "PROCESSING"},
				{"error type", `"name":"TimeoutError"`, `"name":"Error"`},
				{"raw formatting", `"performed":false`, `"performed": false`},
			} {
				t.Run(change.name, func(t *testing.T) {
					changed := strings.Replace(ceiling, change.before, change.after, 1)
					if changed == ceiling {
						t.Fatal("native row lacks the field this regression check changes")
					}
					if matching, _ := l22CompareRows(c.ID, changed, floor); matching {
						t.Fatal("a field outside the timeout message must still differ exactly")
					}
				})
			}
			start, end, ok := l22ObserverTimeoutMessageSpan(ceiling)
			if !ok {
				t.Fatal("native observer timeout message is missing")
			}
			if matching, _ := l22CompareRows(c.ID, ceiling[:start]+"null"+ceiling[end:], floor); matching {
				t.Fatal("raw null is not an observer timeout message string")
			}
			if matching, reason := l22CompareRows("r_modal_small_close_string", ceiling, floor); matching || reason != "" {
				t.Fatal("the timeout exception must not apply to another row")
			}
		})
	}
	if checked != 3 {
		t.Fatalf("requires exactly three native observer timeout pairs, got %d", checked)
	}
}

func l22DiagnosticText(t *testing.T, diagnostics []string) string {
	t.Helper()
	text, err := json.Marshal(diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	return string(text)
}

func l22Read(t *testing.T, path string, dst any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatal(err)
	}
}

func l22Write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

// L12's captured import was admitted without a host registry in its compiler
// adapter. Exercise the page host itself so native's registry guard cannot hide a
func l22CheckHostRegistryAdmission(t *testing.T) {
	t.Helper()
	var cases []l22Case
	l22Read(t, "testdata/l22_registry_admission.json", &cases)
	if len(cases) != 1 {
		t.Fatalf("requires one captured host-registry admission row, got %d", len(cases))
	}
	for _, api := range []string{"59.0", "67.0"} {
		for _, c := range cases {
			t.Run(api+"/"+c.ID, func(t *testing.T) {
				want, ok := c.Expected[api]
				if !ok || want != "COMPILE_OK" || c.Basis != "org" || c.BundleName == "" {
					t.Fatalf("invalid captured registry admission %s API %s", c.ID, api)
				}
				root := t.TempDir()
				bundle := filepath.Join(root, "force-app/main/default/lwc", c.BundleName)
				l22Write(t, filepath.Join(bundle, c.BundleName+".js"), strings.ReplaceAll(c.JS, "FamilyUiGraphql", "Family"+strings.TrimPrefix(c.BundleName, "family")))
				l22Write(t, filepath.Join(bundle, c.BundleName+".html"), c.Template)
				l22Write(t, filepath.Join(bundle, c.BundleName+".js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)
				l22Write(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				_, compiled, err := lwcbrowser.PreparePageConfig(p, filepath.Join(root, "cache"))
				got := "COMPILE_OK"
				if err != nil {
					got = "COMPILE_ERROR"
				}
				if got != want {
					t.Fatalf("%s API %s expected <%s> actual <%s>: %v", c.ID, api, want, got, err)
				}
				if _, ok := compiled.Modules["c:"+c.BundleName]; !ok {
					t.Fatalf("host compiler omitted captured bundle %s", c.BundleName)
				}
			})
		}
	}
}

func l22CompileCases(t *testing.T, api string, cases []l22Case) []compile.BundleResult {
	t.Helper()
	root := t.TempDir()
	keys := make([]string, len(cases))
	for index, c := range cases {
		// Keep the captured identity, including its source spelling and length;
		// native module-resolution diagnostics echo it and parser columns depend on it.
		name := fmt.Sprintf("familyL22%03d", index)
		bundle := filepath.Join(root, "force-app/main/default/lwc", name)
		if c.BundleName != "" {
			name = c.BundleName
			// Isolated native deployments all used this identity. Separate paths
			// preserve it without collisions in the shared batch runner.
			bundle = filepath.Join(root, "force-app", c.ID, "main/default/lwc", name)
		}
		l22Write(t, filepath.Join(bundle, name+".js"), strings.ReplaceAll(c.JS, "FamilyOverlays", "Family"+strings.TrimPrefix(name, "family")))
		l22Write(t, filepath.Join(bundle, name+".html"), c.Template)
		version := "<apiVersion>" + api + "</apiVersion>"
		if c.OmitVersion {
			version = ""
		}
		fragment := c.Meta
		if fragment == "" {
			fragment = "<isExposed>false</isExposed>"
		}
		l22Write(t, filepath.Join(bundle, name+".js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">`+version+fragment+`</LightningComponentBundle>`)
		key, err := filepath.Rel(root, bundle)
		if err != nil {
			t.Fatal(err)
		}
		keys[index] = filepath.ToSlash(key)
	}
	l22Write(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := compile.CompileBatch(p, compile.Options{OutDir: filepath.Join(root, "dist"), Namespace: "c", LightningModules: lwcbrowser.SalesforceImportMap()})
	if err != nil {
		t.Fatal(err)
	}
	results := make([]compile.BundleResult, len(cases))
	for index, key := range keys {
		result, ok := batch[key]
		if !ok {
			t.Fatalf("missing L22 compiler result %s", key)
		}
		results[index] = result
	}
	return results
}

func l22WithControlSpecs(t *testing.T, fixture l22Runtime, controls []l22Case) l22Runtime {
	t.Helper()
	var specs []json.RawMessage
	for _, c := range controls {
		if c.Kind == "runtime" {
			specs = append(specs, c.Spec)
		}
	}
	text, err := json.Marshal(specs)
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string]string, len(fixture.Files))
	found := false
	for rel, content := range fixture.Files {
		if filepath.Base(rel) == fixture.Page+".js" {
			marker := "\nexport default class "
			if strings.Count(content, marker) != 1 {
				t.Fatal("missing L22 native runtime declaration")
			}
			content = strings.Replace(content, marker, "\nSPECS.push(..."+string(text)+");"+marker, 1)
			found = true
		}
		files[rel] = content
	}
	if !found {
		t.Fatal("missing L22 runtime control host")
	}
	fixture.Files = files
	return fixture
}

func l22ObserveDOM(t *testing.T, api string, cases []l22Case, fixture l22Runtime, repo, dependencies string) (map[string]string, map[string]string, error) {
	t.Helper()
	root := t.TempDir()
	for rel, content := range fixture.Files {
		if filepath.IsAbs(rel) || filepath.Clean(rel) != rel || strings.HasPrefix(rel, "../") {
			t.Fatalf("L22 fixture escapes project: %s", rel)
		}
		l22Write(t, filepath.Join(root, filepath.FromSlash(rel)), content)
	}
	p, err := project.Load(root)
	if err != nil {
		return nil, nil, err
	}
	if p.SourceAPIVersion != api {
		t.Fatalf("L22 fixture API %s differs from %s", p.SourceAPIVersion, api)
	}
	compiled, err := compile.Compile(p, compile.Options{OutDir: filepath.Join(root, "dist"), Namespace: "c", LightningModules: lwcbrowser.SalesforceImportMap()})
	if err != nil {
		return nil, nil, fmt.Errorf("L22 runtime source compile: %w", err)
	}
	entry, ok := compiled.Modules["c:"+fixture.Page]
	if !ok {
		return nil, nil, fmt.Errorf("missing L22 runtime page %s", fixture.Page)
	}
	imports := lwcbrowser.SalesforceImportMap()
	imports["lwc"], imports["@lwc/synthetic-shadow"] = "/engine.js", "/shadow.js"
	for _, module := range compiled.Modules {
		rel, err := filepath.Rel(compiled.OutDir, module.File)
		if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
			return nil, nil, fmt.Errorf("L22 module escapes output: %s", module.File)
		}
		imports[module.ModuleKey] = "/modules/" + filepath.ToSlash(rel)
	}
	importJSON, err := json.Marshal(imports)
	if err != nil {
		return nil, nil, err
	}
	ids, allowed, specs := []string{}, map[string]bool{}, []json.RawMessage{}
	for _, c := range cases {
		ids, specs = append(ids, c.ID), append(specs, c.Spec)
		allowed[c.ID] = true
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		if r.URL.Path == "/" {
			id := r.URL.Query().Get("c__case")
			if !allowed[id] {
				http.NotFound(w, r)
				return
			}
			ref, _ := json.Marshal(map[string]any{"pageReference": map[string]any{"type": "standard__navItemPage", "attributes": map[string]string{"apiName": "FamilyL22Oracle" + api[:2]}, "state": map[string]string{"c__case": id}}})
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><script>window.process={env:{NODE_ENV:"production"}}</script><script type="importmap">{"imports":%s}</script><script type="application/json" id="glade-lightning-config">%s</script><main></main><script type="module">import "@lwc/synthetic-shadow";import {createElement} from "lwc";import {installToastService} from "/lightning/runtime/shell/toast-service.js";import Page from "c/%s";installToastService();document.querySelector("main").appendChild(createElement("%s",{is:Page}));</script>`, importJSON, ref, fixture.Page, entry.Tag)
			return
		}
		if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		file := ""
		switch {
		case r.URL.Path == "/engine.js":
			file = filepath.Join(dependencies, "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js")
		case r.URL.Path == "/shadow.js":
			file = filepath.Join(dependencies, "third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js")
		case strings.HasPrefix(r.URL.Path, "/modules/"):
			file = filepath.Join(compiled.OutDir, filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/modules/")))
			rel, err := filepath.Rel(compiled.OutDir, file)
			if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
				http.NotFound(w, r)
				return
			}
		case strings.HasPrefix(r.URL.Path, "/lightning/shims/lightning/"):
			token := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/lightning/shims/lightning/"), ".js")
			js := ""
			switch token {
			case "navigation":
				js = lwcbrowser.NavigationModuleJS()
			case "alert":
				js = lwcbrowser.AlertModuleJS()
			case "confirm":
				js = lwcbrowser.ConfirmModuleJS()
			case "prompt":
				js = lwcbrowser.PromptModuleJS()
			case "platformShowToastEvent":
				js = lwcbrowser.ShowToastEventModuleJS()
			default:
				if lwcbrowser.IsLightningBaseComponentModule(token) {
					js = lwcbrowser.LightningBaseComponentModuleJS(token)
				}
			}
			if js != "" {
				fmt.Fprint(w, js)
				return
			}
		case strings.HasPrefix(r.URL.Path, "/lightning/runtime/shell/"):
			name := strings.TrimPrefix(r.URL.Path, "/lightning/runtime/shell/")
			if filepath.Base(name) == name && (strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".mjs")) {
				name = strings.TrimSuffix(name, ".js")
				if !strings.HasSuffix(name, ".mjs") {
					name += ".mjs"
				}
				file = filepath.Join(repo, "lwcruntime/src/shell", name)
			}
		}
		content, err := os.ReadFile(file)
		if file == "" || err != nil {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(content)
	}))
	defer server.Close()
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(repo, "lwcruntime/node_modules/playwright")
	}
	config, err := json.Marshal(map[string]any{"url": server.URL, "specs": specs, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE"), "concurrency": 3})
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", filepath.Join(repo, "internal/lwc/compile/testdata/l22_browser.mjs"))
	cmd.WaitDelay = 5 * time.Second
	cmd.Stdin = bytes.NewReader(config)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	t.Logf("L22 API %s: runtime compiled; observing %d DOM rows", api, len(ids))
	if err := cmd.Run(); err != nil {
		t.Fatalf("L22 browser infrastructure API %s: %v (context %v): %s", api, err, ctx.Err(), stderr.String())
	}
	var observed struct {
		Values map[string]string `json:"values"`
		Errors map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &observed); err != nil {
		t.Fatal(err)
	}
	for id := range observed.Values {
		if !allowed[id] {
			t.Fatalf("unknown L22 observation %s", id)
		}
	}
	for _, id := range ids {
		if _, ok := observed.Values[id]; !ok && observed.Errors[id] == "" {
			t.Fatalf("missing L22 observation/error %s", id)
		}
	}
	return observed.Values, observed.Errors, nil
}

func l22OneLine(text string) string {
	return strings.NewReplacer("\t", `\t`, "\r", `\r`, "\n", `\n`).Replace(text)
}
