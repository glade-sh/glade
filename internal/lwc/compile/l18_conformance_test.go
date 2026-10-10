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

type l18Case struct {
	ID                 string            `json:"id"`
	Group              string            `json:"group"`
	Kind               string            `json:"kind"`
	Basis              string            `json:"basis"`
	JS                 string            `json:"js"`
	Template           string            `json:"template"`
	MetaFragment       string            `json:"meta_fragment"`
	Component          string            `json:"component"`
	Operation          string            `json:"operation"`
	Interaction        string            `json:"interaction"`
	Typed              json.RawMessage   `json:"typed"`
	Expected           map[string]string `json:"expected"`
	DiagnosticExpected map[string]string `json:"diagnostic_expected"`
	Carried            map[string]string `json:"carried,omitempty"`
}

type l18BrowserCase struct {
	ID          string          `json:"id"`
	Component   string          `json:"component"`
	Operation   string          `json:"operation"`
	Interaction string          `json:"interaction"`
	Typed       json.RawMessage `json:"typed,omitempty"`
}

type l18BrowserResults struct {
	Values map[string]string `json:"values"`
	Errors map[string]string `json:"errors"`
}

// TestL18SalesforceConformance replays owned compiler fixtures and the unchanged
// native DOM harness against Glade. Answers were exported from native API 59/67
// org.tsv observations; CI has no Salesforce dependency.
// Capture mode reports the entire before denominator without failing differences.
func TestL18SalesforceConformance(t *testing.T) {
	t.Run("BrowserRuntime", testL18SalesforceConformance)
}

func testL18SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_L18_CAPTURE") != ""
	data, err := os.ReadFile("testdata/l18_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []l18Case
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	seen, groups := map[string]bool{}, map[string]bool{}
	var compilerCases, runtimeCases []l18Case
	for _, c := range cases {
		if c.ID == "" || seen[c.ID] || c.Basis != "org" || (c.Kind != "compile" && c.Kind != "runtime") {
			t.Fatalf("invalid L18 row %q", c.ID)
		}
		seen[c.ID], groups[c.Group] = true, true
		for _, api := range []string{"59.0", "67.0"} {
			want, ok := c.Expected[api]
			if !ok || (c.Kind == "compile" && want != "COMPILE_OK" && want != "COMPILE_ERROR") ||
				(c.Kind == "runtime" && (!strings.HasPrefix(want, "BROWSER|") || !json.Valid([]byte(strings.TrimPrefix(want, "BROWSER|"))))) {
				t.Fatalf("missing native L18 answer for %s API %s", c.ID, api)
			}
			if reason := c.Carried[api]; reason != "" && (c.Kind != "runtime" || !strings.HasPrefix(reason, "owner L23:")) {
				t.Fatalf("invalid L18 host carry for %s API %s", c.ID, api)
			}
		}
		if c.Kind == "compile" {
			compilerCases = append(compilerCases, c)
		} else {
			runtimeCases = append(runtimeCases, c)
		}
	}
	if len(cases) != 285 || len(compilerCases) != 45 || len(runtimeCases) != 240 {
		t.Fatalf("L18 requires 45 compiler and 240 browser rows, got %d/%d", len(compilerCases), len(runtimeCases))
	}
	for _, group := range []string{"input types", "choice components", "validity API", "messages"} {
		if !groups[group] {
			t.Fatalf("missing L18 case group %s", group)
		}
	}

	// Missing tools/assets are infrastructure failures, never matching rejections.
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal(err)
	}
	repo, err := gladehome.SourceRoot()
	if err != nil {
		t.Fatal(err)
	}
	toolchain, err := gladehome.LWCToolchainDir()
	if err != nil {
		t.Fatal(err)
	}
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(repo, "lwcruntime/node_modules/playwright")
	}
	for _, file := range []string{
		filepath.Join(repo, "third_party/lwc/compile.mjs"),
		filepath.Join(toolchain, "node_modules/@lwc/compiler/package.json"),
		filepath.Join(toolchain, "node_modules/@lwc/engine-dom/dist/index.js"),
		filepath.Join(toolchain, "node_modules/@lwc/synthetic-shadow/dist/index.js"),
		filepath.Join(playwright, "package.json"),
		filepath.Join(repo, "lwcruntime/src/shell/navigation-service.mjs"),
		filepath.Join(repo, "lwcruntime/src/shell/diagnostics.mjs"),
		filepath.Join(repo, "lwcruntime/src/slds/design-system-2/dist/css/bundled/slds2.cosmos.css"),
	} {
		if _, err := os.Stat(file); err != nil {
			t.Fatal(err)
		}
	}

	var report strings.Builder
	fmt.Fprintln(&report, "api\tid\tkind\tstatus\tactual\texpected\treason")
	matches, total, compilerMatches, browserMatches := 0, 0, 0, 0
	runtimeCarried := 0
	diagnosticMatches, diagnosticTotal, gateMatches, gateTotal := 0, 0, 0, 0
	record := func(c l18Case, api, kind, got, want, reason string) bool {
		// Exact text retains case, lengths, array order, omitted fields and raw null.
		match := got == want
		status := "MATCH"
		if !match {
			status = "MISMATCH"
			if carry := l18HostCarryReason(c, api, kind, got, want); carry != "" {
				status, reason = "CARRIED", carry
				runtimeCarried++
				t.Logf("%s API %s CARRIED expected <%s> actual <%s>: %s", c.ID, api, want, got, reason)
			} else if reason == "" {
				reason = "L18: local observation differs from native"
			}
		}
		fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, kind, status, l18OneLine(got), l18OneLine(want), l18OneLine(reason))
		if !capture && !match && status != "CARRIED" {
			t.Errorf("%s API %s %s expected <%s> actual <%s>: %s", c.ID, api, kind, want, got, l18OneLine(reason))
		}
		return match
	}
	writeReport := func() {
		if path := os.Getenv("GLADE_L18_REPORT"); path != "" {
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(report.String()), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, api := range []string{"59.0", "67.0"} {
		compiled := l18CompileCases(t, api, compilerCases)
		browser := l18ObserveBrowser(t, api, runtimeCases, repo, toolchain, playwright)
		compilerIndex, apiMatches, localBrowserRows := 0, 0, 0
		for _, c := range cases {
			got, want, reason := "", c.Expected[api], ""
			if c.Kind == "compile" {
				compileErr := compiled[compilerIndex].Err
				compilerIndex++
				got = "COMPILE_OK"
				if compileErr != nil {
					got = "COMPILE_ERROR"
				}
				if got != want {
					reason = fmt.Sprintf("L18: local compiler outcome differs from native: %v", compileErr)
				}
				if signature, ok := c.DiagnosticExpected[api]; ok {
					diagnosticTotal++
					if record(c, api, "diagnostic", l18LocalDiagnostic(compileErr, api), l18DiagnosticText(signature, api), "L18: local rejection diagnostic differs from native") {
						diagnosticMatches++
					}
				}
			} else {
				var ok bool
				got, ok = browser.Values[c.ID]
				if ok {
					localBrowserRows++
				} else {
					got = "BROWSER_ERROR"
					reason = "L18: local browser row unavailable: " + browser.Errors[c.ID]
				}
			}
			total++
			if record(c, api, c.Kind, got, want, reason) {
				matches++
				apiMatches++
				if c.Kind == "compile" {
					compilerMatches++
				} else {
					browserMatches++
				}
			}
		}
		fmt.Fprintf(&report, "API_TOTAL\t%s\t%d/285\nLOCAL_BROWSER_ROWS\t%s\t%d/240\n", api, apiMatches, api, localBrowserRows)
		t.Logf("L18 API %s matches %d/285; local browser observations %d/240", api, apiMatches, localBrowserRows)
		writeReport()
	}
	// Reuse captured intermediate compiler answers; no endpoint interpolation.
	// These checks stay outside the 570-row floor/ceiling denominator.
	for _, api := range []string{"60.0", "61.0", "62.0", "63.0", "64.0", "65.0", "66.0"} {
		var gateCases []l18Case
		for _, c := range compilerCases {
			if _, ok := c.Expected[api]; ok {
				gateCases = append(gateCases, c)
			}
		}
		if len(gateCases) != 4 {
			t.Fatalf("L18 requires four captured compiler gates at API %s, got %d", api, len(gateCases))
		}
		compiled := l18CompileCases(t, api, gateCases)
		for index, c := range gateCases {
			compileErr := compiled[index].Err
			got, want := "COMPILE_OK", c.Expected[api]
			if compileErr != nil {
				got = "COMPILE_ERROR"
			}
			if signature, ok := c.DiagnosticExpected[api]; ok {
				got += "|" + l18LocalDiagnostic(compileErr, api)
				want += "|" + l18DiagnosticText(signature, api)
			}
			gateTotal++
			if record(c, api, "compiler-gate", got, want, "L18: captured compiler version gate differs") {
				gateMatches++
			}
		}
	}
	fmt.Fprintf(&report, "TOTAL\t%d/%d\nSOURCE_COMPILE_TOTAL\t%d/90\nBROWSER_TOTAL\t%d/480\nDIAGNOSTIC_TOTAL\t%d/%d\nCOMPILER_GATE_TOTAL\t%d/%d\n", matches, total, compilerMatches, browserMatches, diagnosticMatches, diagnosticTotal, gateMatches, gateTotal)
	fmt.Fprintf(&report, "RUNTIME_CARRIED_TOTAL\t%d/480\n", runtimeCarried)
	t.Logf("L18 matches %d/%d; compiler %d/90; browser %d/480; diagnostics %d/%d; compiler gates %d/%d", matches, total, compilerMatches, browserMatches, diagnosticMatches, diagnosticTotal, gateMatches, gateTotal)
	t.Logf("L18 host carries %d/480 browser rows", runtimeCarried)
	writeReport()
}

// Carries do not count as matches. Recognize only the measured host difference;
// any other byte difference remains a failure, including in these same rows.
func l18HostCarryReason(c l18Case, api, kind, got, want string) string {
	if kind != "runtime" || c.Carried[api] == "" {
		return ""
	}
	var local, native string
	switch c.ID {
	case "r_input_datetime_normal":
		local, native = `"value":"12:30 PM"`, `"value":"4:30 AM"`
	case "r_validity_input_missing_method", "r_validity_textarea_missing_method",
		"r_validity_combobox_missing_method", "r_validity_radio_group_missing_method",
		"r_validity_select_missing_method", "r_validity_native_select_missing_method":
		local, native = `"message":"c[name] is not a function"`, `"message":"e[t] is not a function"`
	default:
		return ""
	}
	if strings.Count(got, local) == 1 && strings.Replace(got, local, native, 1) == want {
		return c.Carried[api]
	}
	return ""
}

func l18WriteFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}

func l18Project(t *testing.T, root, api, packages string) project.Project {
	t.Helper()
	l18WriteFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"`+packages+`","default":true}],"namespace":"","sourceApiVersion":"`+api+`"}`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func l18CompileCases(t *testing.T, api string, cases []l18Case) []compile.BundleResult {
	t.Helper()
	root := t.TempDir()
	keys := make([]string, len(cases))
	for index, c := range cases {
		name := fmt.Sprintf("familyL18%03d", index)
		key := filepath.Join("cases", fmt.Sprintf("%04d", index), "force-app/main/default/lwc", name)
		keys[index] = filepath.ToSlash(key)
		bundle := filepath.Join(root, key)
		fragment := c.MetaFragment
		if fragment == "" {
			fragment = "<isExposed>false</isExposed>"
		}
		l18WriteFile(t, filepath.Join(bundle, name+".js"), strings.ReplaceAll(c.JS, "FamilyInputs", strings.ToUpper(name[:1])+name[1:]))
		l18WriteFile(t, filepath.Join(bundle, name+".html"), c.Template)
		l18WriteFile(t, filepath.Join(bundle, name+".js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion>`+strings.ReplaceAll(fragment, "{api}", api)+`</LightningComponentBundle>`)
	}
	compiled, err := compile.CompileBatch(l18Project(t, root, api, "cases"), compile.Options{OutDir: filepath.Join(root, "dist")})
	if err != nil {
		t.Fatal(err)
	}
	results := make([]compile.BundleResult, len(cases))
	for index, key := range keys {
		result, ok := compiled[key]
		if !ok {
			t.Fatalf("missing L18 batch result %s", key)
		}
		results[index] = result
	}
	return results
}

var l18DiagnosticLineRE = regexp.MustCompile(`(?m)^Error: (LWC[0-9]+:[^\r\n]+)`)
var l18DiagnosticPositionRE = regexp.MustCompile(`\[Line: [0-9]+, Col: [0-9]+\]\s*`)
var l18DiagnosticURLRE = regexp.MustCompile(`https?://[^\s<>"']+`)

func l18DiagnosticText(text, api string) string {
	text = l18DiagnosticPositionRE.ReplaceAllString(text, "")
	// The captured deployment transport exports documentation links as <URL>.
	text = l18DiagnosticURLRE.ReplaceAllString(text, "<URL>")
	return strings.ReplaceAll(text, "current component API version ("+strings.TrimSuffix(api, ".0")+")", "current component API version (<source-api>)")
}

func l18LocalDiagnostic(err error, api string) string {
	if err == nil {
		return ""
	}
	// A shared deployment formatter may prefix the message with its position.
	// Strip only the same transport fields removed from the captured diagnostic.
	text := l18DiagnosticText(err.Error(), api)
	if match := l18DiagnosticLineRE.FindStringSubmatch(text); match != nil {
		text = match[1]
	}
	return text
}

func l18OneLine(text string) string {
	return strings.NewReplacer("\t", `\t`, "\r", `\r`, "\n", `\n`).Replace(text)
}

func l18ObserveBrowser(t *testing.T, api string, cases []l18Case, repo, toolchain, playwright string) l18BrowserResults {
	t.Helper()
	root := t.TempDir()
	name := "familyL18Runtime" + strings.TrimSuffix(api, ".0")
	for _, extension := range []string{"js", "html", "js-meta.xml"} {
		text, err := os.ReadFile("testdata/l18_runtime" + api[:2] + "." + extension)
		if err != nil {
			t.Fatal(err)
		}
		l18WriteFile(t, filepath.Join(root, "force-app/main/default/lwc", name, name+"."+extension), string(text))
	}
	compiled, err := compile.Compile(l18Project(t, root, api, "force-app"), compile.Options{OutDir: filepath.Join(root, "dist"), Namespace: "c"})
	if err != nil {
		result := l18BrowserResults{Values: map[string]string{}, Errors: map[string]string{}}
		for _, c := range cases {
			result.Errors[c.ID] = "runtime fixture compilation: " + err.Error()
		}
		return result
	}
	entry, ok := compiled.Modules["c:"+name]
	if !ok {
		t.Fatal("L18 runtime entry missing from compiled output")
	}
	imports := lwcbrowser.SalesforceImportMap()
	imports["lwc"] = "/engine.js"
	imports["@lwc/synthetic-shadow"] = "/shadow.js"
	for _, module := range compiled.Modules {
		rel, err := filepath.Rel(compiled.OutDir, module.File)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("L18 compiled module escapes output: %s", module.File)
		}
		imports[module.ModuleKey] = "/modules/" + filepath.ToSlash(rel)
	}
	importJSON, err := json.Marshal(imports)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{}
	inputs := make([]l18BrowserCase, len(cases))
	for index, c := range cases {
		allowed[c.ID] = true
		inputs[index] = l18BrowserCase{c.ID, c.Component, c.Operation, c.Interaction, c.Typed}
	}
	slds := http.StripPrefix("/lightning/runtime/slds/", http.FileServer(http.Dir(filepath.Join(repo, "lwcruntime/src/slds"))))
	// Entry modules import generated template/style siblings not in the manifest.
	modules := http.StripPrefix("/modules/", http.FileServer(http.Dir(compiled.OutDir)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			id := r.URL.Query().Get("c__case")
			if !allowed[id] {
				http.Error(w, "unknown L18 case", http.StatusBadRequest)
				return
			}
			configJSON, _ := json.Marshal(map[string]any{"pageReference": map[string]any{"type": "standard__component", "attributes": map[string]string{"componentName": "c:" + name}, "state": map[string]string{"c__case": id}}})
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><script>window.process={env:{NODE_ENV:"production"}}</script><script type="importmap">{"imports":%s}</script><script type="application/json" id="glade-lightning-config">%s</script><main></main><script type="module">import "@lwc/synthetic-shadow";import {createElement} from "lwc";import {loadSLDS} from "@glade/slds";import Page from "c/%s";const style=await loadSLDS();if(!style.ok)throw new Error("L18 local stylesheet unavailable");document.querySelector("main").appendChild(createElement("%s",{is:Page}));</script>`, importJSON, configJSON, name, entry.Tag)
			return
		}
		if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/lightning/runtime/slds/") && r.URL.Path != "/lightning/runtime/slds/slds-loader.js" {
			slds.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/modules/") {
			modules.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/lightning/shims/i18n/") {
			property := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/lightning/shims/i18n/"), ".js")
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			_, _ = w.Write([]byte(lwcbrowser.I18nModuleJS(property)))
			return
		}
		file := ""
		switch r.URL.Path {
		case "/engine.js":
			file = filepath.Join(toolchain, "node_modules/@lwc/engine-dom/dist/index.js")
		case "/shadow.js":
			file = filepath.Join(toolchain, "node_modules/@lwc/synthetic-shadow/dist/index.js")
		case "/lightning/shims/lightning/navigation.js":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			_, _ = w.Write([]byte(lwcbrowser.NavigationModuleJS()))
			return
		case "/lightning/runtime/slds/slds-loader.js":
			file = filepath.Join(repo, "lwcruntime/src/slds/slds-loader.mjs")
		case "/lightning/runtime/shell/navigation-service.js":
			file = filepath.Join(repo, "lwcruntime/src/shell/navigation-service.mjs")
		case "/lightning/runtime/shell/diagnostics.js", "/lightning/runtime/shell/diagnostics.mjs":
			file = filepath.Join(repo, "lwcruntime/src/shell/diagnostics.mjs")
		}
		if strings.HasPrefix(r.URL.Path, "/lightning/shims/lightning/") {
			component := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/lightning/shims/lightning/"), ".js")
			if lwcbrowser.IsLightningBaseComponentModule(component) {
				w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
				_, _ = w.Write([]byte(lwcbrowser.LightningBaseComponentModuleJS(component)))
				return
			}
		}
		if file == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		http.ServeFile(w, r, file)
	}))
	defer server.Close()
	// No oracle answers enter the browser process. Canonical text is serialized
	// there so JavaScript's lone surrogates and raw null survive unchanged.
	config, err := json.Marshal(map[string]any{"url": server.URL, "cases": inputs, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", filepath.Join(repo, "internal/lwc/compile/testdata/l18_browser.mjs"))
	cmd.Stdin = bytes.NewReader(config)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("L18 browser transport: %v: %s", err, stderr.String())
	}
	var result l18BrowserResults
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("L18 browser output: %v: %s", err, stderr.String())
	}
	if len(result.Values)+len(result.Errors) != len(cases) {
		t.Fatalf("L18 browser returned %d answers and %d errors for %d rows", len(result.Values), len(result.Errors), len(cases))
	}
	for _, c := range cases {
		value, observed := result.Values[c.ID]
		_, failed := result.Errors[c.ID]
		if observed == failed || (observed && (!strings.HasPrefix(value, "BROWSER|") || !json.Valid([]byte(strings.TrimPrefix(value, "BROWSER|"))))) {
			t.Fatalf("invalid L18 local browser answer for %s", c.ID)
		}
	}
	return result
}
