package lwcbrowser

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
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/project"
)

type l16Case struct {
	ID           string            `json:"id"`
	Group        string            `json:"group"`
	Kind         string            `json:"kind"`
	JS           string            `json:"js"`
	Template     string            `json:"template"`
	MetaFragment string            `json:"meta_fragment"`
	Control      bool              `json:"control"`
	Spec         json.RawMessage   `json:"spec"`
	Expected     map[string]string `json:"expected"`
}

type l16Diagnostic struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

// TestL16SalesforceConformance replays the owned API 59/67 RefreshView captures.
// The native capture contains 222 rows/API.
// Six direct registration controls/API are exported separately.
// Every row uses exact text assertions. The supplemental diagnostic export
// retains the full native deployment text; only the identified L23 hosted
// envelope differences can be carried. CI never calls Salesforce or glade-tools.
func TestL16SalesforceConformance(t *testing.T) {
	data, err := os.ReadFile("testdata/l16_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []l16Case
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 222 {
		t.Fatalf("L16 requires the unchanged 222 core rows, got %d", len(cases))
	}
	data, err = os.ReadFile("testdata/l16_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var controlExport struct {
		Source string    `json:"source"`
		Rows   []l16Case `json:"rows"`
	}
	if err := json.Unmarshal(data, &controlExport); err != nil {
		t.Fatal(err)
	}
	if controlExport.Source != "4688101856a5a9181c615c7b4cd2161e1feee523" || len(controlExport.Rows) != 6 {
		t.Fatal("L16 requires six direct controls from the committed native capture")
	}
	for _, row := range controlExport.Rows {
		if !row.Control || row.Kind != "runtime" || !strings.HasPrefix(row.ID, "r_control_") {
			t.Fatalf("invalid L16 direct control %s", row.ID)
		}
	}
	cases = append(cases, controlExport.Rows...)
	data, err = os.ReadFile("testdata/l16_diagnostics.json")
	if err != nil {
		t.Fatal(err)
	}
	var diagnosticExport struct {
		Source string                     `json:"source"`
		Rows   map[string][]l16Diagnostic `json:"rows"`
	}
	if err := json.Unmarshal(data, &diagnosticExport); err != nil {
		t.Fatal(err)
	}
	if diagnosticExport.Source != "417d543b912d4da510ca0c603f8cd0b36954d597" {
		t.Fatal("unexpected L16 diagnostic source")
	}
	diagnostics := map[string]map[string]string{}
	for _, api := range []string{"59.0", "67.0"} {
		diagnostics[api] = map[string]string{}
		for _, row := range diagnosticExport.Rows[api] {
			if row.ID == "" || row.Message == "" || diagnostics[api][row.ID] != "" {
				t.Fatalf("invalid native diagnostic %s API %s", row.ID, api)
			}
			diagnostics[api][row.ID] = row.Message
		}
		count := 4
		if api == "59.0" {
			count = 8
		}
		if len(diagnostics[api]) != count {
			t.Fatalf("L16 API %s requires %d native rejection diagnostics, got %d", api, count, len(diagnostics[api]))
		}
	}
	versions := []string{"59.0", "67.0"}
	seen, groups := map[string]bool{}, map[string]bool{}
	var compilerCases, runtimeCases []l16Case
	for _, c := range cases {
		if c.ID == "" || seen[c.ID] || (c.Kind != "compile" && c.Kind != "runtime") {
			t.Fatalf("invalid or duplicate L16 row %q (%s)", c.ID, c.Kind)
		}
		seen[c.ID], groups[c.Group] = true, true
		for _, api := range versions {
			want, ok := c.Expected[api]
			if !ok || (c.Kind == "compile" && want != "COMPILE_OK" && want != "COMPILE_ERROR") || (c.Kind == "runtime" && !strings.HasPrefix(want, "BROWSER|")) {
				t.Fatalf("missing native %s answer: %s API %s", c.Kind, c.ID, api)
			}
			if (want == "COMPILE_ERROR") != (diagnostics[api][c.ID] != "") {
				t.Fatalf("native rejection/diagnostic disagreement: %s API %s", c.ID, api)
			}
			if c.Kind == "runtime" {
				var native struct {
					ID          string          `json:"id"`
					Stage       string          `json:"stage"`
					Observation json.RawMessage `json:"observation"`
				}
				if err := json.Unmarshal([]byte(strings.TrimPrefix(want, "BROWSER|")), &native); err != nil || native.ID != c.ID || native.Stage != "done" || len(native.Observation) == 0 {
					t.Fatalf("invalid native DOM answer: %s API %s", c.ID, api)
				}
			}
		}
		if c.Kind == "compile" {
			if c.JS == "" || c.Template == "" {
				t.Fatalf("missing owned compile input: %s", c.ID)
			}
			compilerCases = append(compilerCases, c)
		} else {
			var spec struct {
				ID      string `json:"id"`
				Control bool   `json:"control"`
			}
			if err := json.Unmarshal(c.Spec, &spec); err != nil || spec.ID != c.ID || spec.Control != c.Control {
				t.Fatalf("invalid owned browser input: %s", c.ID)
			}
			runtimeCases = append(runtimeCases, c)
		}
	}
	if len(cases) != 228 || len(compilerCases) != 64 || len(runtimeCases) != 164 {
		t.Fatalf("L16 requires 64 compile/metadata and 164 DOM rows, got %d/%d", len(compilerCases), len(runtimeCases))
	}
	for api, rows := range diagnostics {
		for id := range rows {
			if !seen[id] {
				t.Fatalf("unexpected native diagnostic %s API %s", id, api)
			}
		}
	}
	for _, group := range []string{"dispatch", "registration-handles", "tree-traversal-pruning", "completion"} {
		if !groups[group] {
			t.Fatalf("missing L16 group %s", group)
		}
	}

	// A missing compiler must never masquerade as a native compile rejection.
	repo, err := gladehome.SourceRoot()
	if err != nil {
		t.Fatal(err)
	}
	dependencies, err := gladehome.EnsureRoot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{
		filepath.Join(repo, "third_party/lwc/compile.mjs"),
		filepath.Join(dependencies, "third_party/lwc/node_modules/@lwc/compiler/package.json"),
	} {
		if _, err := os.Stat(file); err != nil {
			t.Fatal(err)
		}
	}

	var report strings.Builder
	report.WriteString("API\tID\tKIND\tSTATUS\tACTUAL\tEXPECTED\tREASON\n")
	matches, total, compileMatches, domMatches, localDOMRows := 0, 0, 0, 0, 0
	coreMatches, controlMatches := 0, 0
	diagnosticMatches, diagnosticTotal, diagnosticCarried := 0, 0, 0
	for _, api := range versions {
		compiled := l16CompileCases(t, api, compilerCases)
		dom, domErrors, domErr := l16ObserveDOM(t, api, runtimeCases, repo, dependencies)
		compileIndex, apiMatches := 0, 0
		for _, c := range cases {
			got, want, reason := "", c.Expected[api], ""
			if c.Kind == "compile" {
				compileErr := compiled[compileIndex].Err
				compileIndex++
				got = "COMPILE_OK"
				if compileErr != nil {
					got = "COMPILE_ERROR"
				}
				if got != want {
					reason = "L01: compile/metadata acceptance differs from native"
					if compileErr != nil {
						reason += ": " + compileErr.Error()
					}
				}
				if expected := diagnostics[api][c.ID]; expected != "" {
					actual := l16RejectionDiagnostic(compileErr)
					status, carry := "MATCH", ""
					diagnosticTotal++
					if actual == expected {
						diagnosticMatches++
					} else if got == want && l16DiagnosticEnvelopeCarry(c.ID, api, expected, actual) {
						status = "CARRIED"
						carry = "L23: local compiler omits the captured hosted Metadata API source-location envelope"
						if strings.HasSuffix(c.ID, "_bad_syntax") {
							carry = "L23: local parser diagnostic lacks the captured hosted Metadata API LWC1503 envelope and source location"
						}
						diagnosticCarried++
						t.Logf("%s API %s %s expected <%s> actual <%s>", c.ID, api, carry, expected, actual)
					} else {
						status = "MISMATCH"
						t.Errorf("%s API %s diagnostic expected <%s> actual <%s>", c.ID, api, expected, actual)
					}
					fmt.Fprintf(&report, "%s\t%s\tdiagnostic\t%s\t%s\t%s\t%s\n", api, c.ID, status, l16OneLine(actual), expected, carry)
				}
			} else if value, ok := dom[c.ID]; ok {
				got, err = l16BrowserText(value)
				if err != nil {
					t.Fatalf("%s API %s: %v", c.ID, api, err)
				}
				localDOMRows++
				if got != want {
					reason = "L16: native DOM differs at " + l16BrowserDifference(want, got)
				}
			} else {
				got, reason = "BROWSER_ERROR", "L16: local DOM observation missing"
				if domErrors[c.ID] != "" {
					reason += ": " + domErrors[c.ID]
				} else if domErr != nil {
					reason += ": " + domErr.Error()
				}
			}
			status := "MISMATCH"
			if got == want {
				status = "MATCH"
				matches++
				apiMatches++
				if c.Control {
					controlMatches++
				} else {
					coreMatches++
				}
				if c.Kind == "compile" {
					compileMatches++
				} else {
					domMatches++
				}
			}
			total++
			fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, c.Kind, status, got, want, l16OneLine(reason))
			if got != want {
				t.Errorf("%s API %s expected <%s> actual <%s>: %s", c.ID, api, want, got, l16OneLine(reason))
			}
		}
		fmt.Fprintf(&report, "API_TOTAL\t%s\t%d/%d\n", api, apiMatches, len(cases))
		t.Logf("L16 API %s matches %d/%d", api, apiMatches, len(cases))
	}
	fmt.Fprintf(&report, "TOTAL\t%d/%d\nCOMPILE_TOTAL\t%d/%d\nDOM_TOTAL\t%d/%d\nLOCAL_DOM_ROWS\t%d/%d\n", matches, total, compileMatches, len(compilerCases)*len(versions), domMatches, len(runtimeCases)*len(versions), localDOMRows, len(runtimeCases)*len(versions))
	fmt.Fprintf(&report, "CORE_TOTAL\t%d/444\nCONTROL_TOTAL\t%d/12\n", coreMatches, controlMatches)
	fmt.Fprintf(&report, "DIAGNOSTIC_TOTAL\t%d/%d\nDIAGNOSTIC_CARRIED\t%d\n", diagnosticMatches, diagnosticTotal, diagnosticCarried)
	t.Logf("L16 matches %d/%d; compile/metadata %d/%d; native-vs-local DOM %d/%d; observed DOM rows %d/%d", matches, total, compileMatches, len(compilerCases)*len(versions), domMatches, len(runtimeCases)*len(versions), localDOMRows, len(runtimeCases)*len(versions))
	t.Logf("L16 core matches %d/444; direct controls %d/12", coreMatches, controlMatches)
	t.Logf("L16 rejection diagnostics %d/%d; carried %d (separate from the %d-row denominator)", diagnosticMatches, diagnosticTotal, diagnosticCarried, total)
	if path := os.Getenv("GLADE_L16_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func l16RejectionDiagnostic(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if _, output, ok := strings.Cut(message, "\n"); ok {
		message = output
	}
	return strings.TrimPrefix(strings.Split(message, "\n")[0], "Error: ")
}

// Keep the full observations in the report. A carry permits only these known
// wrappers around the identical captured cause, never a new parser rejection.
func l16DiagnosticEnvelopeCarry(id, api, expected, actual string) bool {
	if strings.HasSuffix(id, "_bad_expression") && api == "59.0" {
		return actual == strings.TrimPrefix(expected, "[Line: 1, Col: 17] ")
	}
	if strings.HasSuffix(id, "_bad_syntax") {
		return actual == "SyntaxError: Unexpected token (1:337)" && expected == "[Line: 1, Col: 337] LWC1503: Parsing error: Unexpected token (1:337)"
	}
	return false
}

func l16WriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func l16WriteBundle(t *testing.T, root, name, js, template, fragment, api string) string {
	t.Helper()
	bundle := filepath.Join(root, "force-app/main/default/lwc", name)
	js = strings.ReplaceAll(js, "FamilyRefreshView", strings.ToUpper(name[:1])+name[1:])
	if fragment == "" {
		fragment = "<isExposed>false</isExposed>"
	}
	l16WriteFile(t, filepath.Join(bundle, name+".js"), js)
	l16WriteFile(t, filepath.Join(bundle, name+".html"), template)
	l16WriteFile(t, filepath.Join(bundle, name+".js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion>`+fragment+`</LightningComponentBundle>`)
	return bundle
}

func l16WriteProject(t *testing.T, root, directory, api string) {
	t.Helper()
	l16WriteFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"`+directory+`","default":true}],"namespace":"","sourceApiVersion":"`+api+`"}`)
}

func l16CompileCases(t *testing.T, api string, cases []l16Case) []compile.BundleResult {
	t.Helper()
	root := t.TempDir()
	keys := make([]string, len(cases))
	for index, c := range cases {
		bundle := l16WriteBundle(t, root, fmt.Sprintf("familyL16%03d", index), c.JS, c.Template, c.MetaFragment, api)
		key, err := filepath.Rel(root, bundle)
		if err != nil {
			t.Fatal(err)
		}
		keys[index] = filepath.ToSlash(key)
	}
	l16WriteProject(t, root, "force-app", api)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := compile.CompileBatch(p, compile.Options{OutDir: filepath.Join(root, "dist"), Namespace: "c"})
	if err != nil {
		t.Fatal(err)
	}
	results := make([]compile.BundleResult, len(cases))
	for index, key := range keys {
		result, ok := compiled[key]
		if !ok {
			t.Fatalf("missing L16 batch compiler result %s", key)
		}
		results[index] = result
	}
	return results
}

func l16WriteRuntime(t *testing.T, root, api, fixture, prefix string, cases []l16Case) (string, error) {
	t.Helper()
	nodeJS, err := os.ReadFile("testdata/" + fixture + "_node.js")
	if err != nil {
		return "", err
	}
	runtimeJS, err := os.ReadFile("testdata/" + fixture + "_runtime.js")
	if err != nil {
		return "", err
	}
	specs := make([]json.RawMessage, len(cases))
	for index, c := range cases {
		specs[index] = c.Spec
	}
	specJSON, err := json.Marshal(specs)
	if err != nil {
		return "", err
	}
	js := strings.ReplaceAll(string(runtimeJS), "__SPECS__", string(specJSON))
	for _, expression := range []struct{ key, token, fallback string }{
		{"event_expr", "__EVENT_EXPRESSION__", "undefined"},
		{"value_expr", "__VALUE_EXPRESSION__", "undefined"},
		{"context_expr", "__CONTEXT_EXPRESSION__", "this"},
		{"provider_expr", "__PROVIDER_EXPRESSION__", "null"},
	} {
		value := expression.fallback
		for index := len(cases) - 1; index >= 0; index-- {
			var spec map[string]json.RawMessage
			if err := json.Unmarshal(cases[index].Spec, &spec); err != nil {
				return "", err
			}
			if raw, ok := spec[expression.key]; ok {
				var source string
				if err := json.Unmarshal(raw, &source); err != nil {
					return "", err
				}
				id, _ := json.Marshal(cases[index].ID)
				value = "(spec.id === " + string(id) + " ? (" + source + ") : " + value + ")"
			}
		}
		js = strings.ReplaceAll(js, expression.token, value)
	}
	// Preserve the native case-selection method while replacing hosted routing
	// with a public local control. Every refresh observation uses the same body.
	js = strings.ReplaceAll(js, "import { LightningElement, wire } from 'lwc';", "import { LightningElement, api } from 'lwc';")
	js = strings.ReplaceAll(js, "import { CurrentPageReference } from 'lightning/navigation';\n", "")
	js = strings.ReplaceAll(js, "@wire(CurrentPageReference) page(value)", "@api page(value)")
	suffix := strings.TrimSuffix(api, ".0")
	leaf, branch, tree, page := prefix+"Leaf"+suffix, prefix+"Branch"+suffix, prefix+"Tree"+suffix, prefix+"Runtime"+suffix
	l16WriteBundle(t, root, leaf, string(nodeJS), `<template><span data-leaf>Owned leaf</span></template>`, "", api)
	l16WriteBundle(t, root, branch, string(nodeJS), `<template><section><`+l16Tag(leaf)+` data-child></`+l16Tag(leaf)+`><`+l16Tag(leaf)+` data-child></`+l16Tag(leaf)+`></section></template>`, "", api)
	l16WriteBundle(t, root, tree, string(nodeJS), `<template><section><`+l16Tag(branch)+` data-child></`+l16Tag(branch)+`><`+l16Tag(branch)+` data-child></`+l16Tag(branch)+`></section></template>`, "", api)
	l16WriteBundle(t, root, page, js, `<template><section data-l16-host><template lwc:if={ready}><button data-run onclick={execute}>Capture case</button><pre data-result>{result}</pre><`+l16Tag(tree)+` data-root ontrace={recordTrace}></`+l16Tag(tree)+`><`+l16Tag(leaf)+` data-outside ontrace={recordTrace}></`+l16Tag(leaf)+`></template></section></template>`, "<isExposed>true</isExposed><targets><target>lightning__Tab</target></targets>", api)
	return page, nil
}

func l16ObserveDOM(t *testing.T, api string, cases []l16Case, repo, dependencies string) (map[string]json.RawMessage, map[string]string, error) {
	t.Helper()
	root := t.TempDir()
	pages := map[string]string{}
	ids := make([]string, len(cases))
	for index, c := range cases {
		ids[index] = c.ID
	}
	// The original and control harnesses keep their captured bodies, while one
	// compile and browser batch per API replays every row on its own page.
	for _, control := range []bool{false, true} {
		var rows []l16Case
		for _, c := range cases {
			if c.Control == control {
				rows = append(rows, c)
			}
		}
		fixture, prefix := "l16", "familyL16"
		if control {
			fixture, prefix = "l16_controls", "familyL16Controls"
		}
		page, err := l16WriteRuntime(t, root, api, fixture, prefix, rows)
		if err != nil {
			return nil, nil, err
		}
		for _, c := range rows {
			pages[c.ID] = page
		}
	}
	l16WriteProject(t, root, "force-app", api)
	p, err := project.Load(root)
	if err != nil {
		return nil, nil, err
	}
	compiled, err := compile.Compile(p, compile.Options{OutDir: filepath.Join(root, "dist"), Namespace: "c"})
	if err != nil {
		return nil, nil, err
	}
	for _, page := range pages {
		if _, ok := compiled.Modules["c:"+page]; !ok {
			return nil, nil, fmt.Errorf("L16 runtime page %s missing from compiler output", page)
		}
	}
	imports := map[string]string{"lwc": "/engine.js", "@lwc/synthetic-shadow": "/shadow.js", "lightning/refresh": "/refresh.js"}
	for _, module := range compiled.Modules {
		rel, err := filepath.Rel(compiled.OutDir, module.File)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, nil, fmt.Errorf("L16 module escapes compiler output: %s", module.File)
		}
		imports[module.ModuleKey] = "/modules/" + filepath.ToSlash(rel)
	}
	importJSON, err := json.Marshal(imports)
	if err != nil {
		return nil, nil, err
	}
	engine := filepath.Join(dependencies, "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js")
	shadow := filepath.Join(dependencies, "third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js")
	for _, file := range []string{engine, shadow} {
		if _, err := os.Stat(file); err != nil {
			return nil, nil, err
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			page, ok := pages[r.URL.Query().Get("case")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			entry := compiled.Modules["c:"+page]
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><script>window.process={env:{NODE_ENV:"production"}}</script><script type="importmap">{"imports":%s}</script><main></main><script type="module">import "@lwc/synthetic-shadow";import {createElement} from "lwc";import Page from "c/%s";const page=createElement("%s",{is:Page});document.querySelector("main").appendChild(page);page.page({state:{c__case:new URL(location.href).searchParams.get("case")}});</script>`, importJSON, page, entry.Tag)
			return
		}
		if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		if r.URL.Path == "/refresh.js" {
			_, _ = w.Write([]byte(RefreshModuleJS()))
			return
		}
		file := ""
		switch {
		case r.URL.Path == "/engine.js":
			file = engine
		case r.URL.Path == "/shadow.js":
			file = shadow
		case strings.HasPrefix(r.URL.Path, "/modules/"):
			file = filepath.Join(compiled.OutDir, filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/modules/")))
			rel, err := filepath.Rel(compiled.OutDir, file)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				http.NotFound(w, r)
				return
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
	// Neither expected text nor Salesforce credentials reach the local browser.
	config, err := json.Marshal(map[string]any{"url": server.URL, "ids": ids, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE"), "concurrency": browserRowConcurrency})
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", filepath.Join(repo, "internal/lwcbrowser/testdata/l16_browser.mjs"))
	cmd.Stdin = bytes.NewReader(config)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, nil, fmt.Errorf("L16 local browser: %w: %s", err, l16OneLine(stderr.String()))
	}
	var observation struct {
		Values map[string]json.RawMessage `json:"values"`
		Errors map[string]string          `json:"errors"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &observation); err != nil {
		return nil, nil, err
	}
	return observation.Values, observation.Errors, nil
}

func l16Tag(name string) string {
	var tag strings.Builder
	tag.WriteString("c-")
	for _, r := range name {
		if r >= 'A' && r <= 'Z' {
			tag.WriteByte('-')
			r += 'a' - 'A'
		}
		tag.WriteRune(r)
	}
	return tag.String()
}

// Match the native json.dumps(sort_keys=True, separators=(',', ':')) exactly.
// Serialize only observed JSON: raw null, array order, text case and length stay
// intact. In particular, no status, missing export, or handle is synthesized.
func l16BrowserText(raw json.RawMessage) (string, error) {
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

func l16BrowserDifference(want, got string) string {
	var expected, actual any
	if json.Unmarshal([]byte(strings.TrimPrefix(want, "BROWSER|")), &expected) != nil || json.Unmarshal([]byte(strings.TrimPrefix(got, "BROWSER|")), &actual) != nil {
		return "$"
	}
	if difference := l16DifferencePath(expected, actual, "$"); difference != "" {
		return difference
	}
	return "$.serialization"
}

func l16DifferencePath(expected, actual any, path string) string {
	switch want := expected.(type) {
	case map[string]any:
		got, ok := actual.(map[string]any)
		if !ok || len(want) != len(got) {
			return path + ".keys"
		}
		keys := make([]string, 0, len(want))
		for key := range want {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			other, ok := got[key]
			if !ok {
				return path + "." + key
			}
			if difference := l16DifferencePath(want[key], other, path+"."+key); difference != "" {
				return difference
			}
		}
	case []any:
		got, ok := actual.([]any)
		if !ok || len(want) != len(got) {
			return path + ".length"
		}
		for index, value := range want {
			if difference := l16DifferencePath(value, got[index], fmt.Sprintf("%s[%d]", path, index)); difference != "" {
				return difference
			}
		}
	default:
		if !reflect.DeepEqual(expected, actual) {
			return path
		}
	}
	return ""
}

func l16OneLine(value string) string {
	return strings.NewReplacer("\t", " ", "\r", " ", "\n", " ").Replace(value)
}
