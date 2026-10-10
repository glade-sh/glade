package compile

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
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/glade-sh/glade/internal/project"
)

type l04Case struct {
	ID                 string            `json:"id"`
	Group              string            `json:"group"`
	Kind               string            `json:"kind"`
	JS                 string            `json:"js"`
	Template           string            `json:"template"`
	MetaFragment       string            `json:"meta_fragment"`
	Files              map[string]string `json:"files"`
	Leaf               string            `json:"leaf"`
	Controls           []string          `json:"controls"`
	Expected           map[string]string `json:"expected"`
	DiagnosticExpected map[string]string `json:"diagnostic_expected"`
	RuntimeExpected    map[string]string `json:"runtime_expected"`
	Carried            map[string]string `json:"carried"`
}

// TestL04SalesforceConformance replays owned API 59/67 compiler and DOM captures.
// GLADE_L04_CAPTURE reports every mismatch or missing oracle without accepting it.
// Sources and observations are exported from the native lifecycle capture; CI
// never accesses Salesforce or imports the capture tools.
// Native DOM answers come from the per-API runtime observations. The original
// source compilation answers are preserved separately for each API.
// Rejection signatures preserve the LWC code and message from diagnostics.json;
// source positions and local stack traces are outside that signature.
func TestL04SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_L04_CAPTURE") != ""
	data, err := os.ReadFile("testdata/l04_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []l04Case
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	versions := []string{"59.0", "67.0"}
	seen, groups := map[string]bool{}, map[string]bool{}
	runtimeCases := []l04Case{}
	runtimeIndices := map[string]int{}
	diagnosticCases := 0
	for _, c := range cases {
		if c.ID == "" || seen[c.ID] || (c.Kind != "compile" && c.Kind != "runtime") {
			t.Fatalf("invalid or duplicate L04 row %q (%s)", c.ID, c.Kind)
		}
		seen[c.ID], groups[c.Group] = true, true
		if len(c.DiagnosticExpected) != 0 {
			diagnosticCases++
		}
		for _, api := range versions {
			if c.Expected[api] != "COMPILE_OK" && c.Expected[api] != "COMPILE_ERROR" {
				t.Fatalf("missing native source-compile answer: %s API %s", c.ID, api)
			}
			if len(c.DiagnosticExpected) != 0 && (c.Kind != "compile" || c.Expected[api] != "COMPILE_ERROR" || !l04DiagnosticSignatureRE.MatchString(c.DiagnosticExpected[api])) {
				t.Fatalf("invalid or missing native rejection signature: %s API %s", c.ID, api)
			}
			if !capture && c.Kind == "runtime" {
				if want, ok := c.RuntimeExpected[api]; !ok || !strings.HasPrefix(want, "DOM|") {
					t.Fatalf("missing native DOM answer: %s API %s", c.ID, api)
				}
			}
		}
		if c.Kind == "runtime" {
			runtimeIndices[c.ID] = len(runtimeCases)
			runtimeCases = append(runtimeCases, c)
		}
	}
	if len(cases) != 230 || len(runtimeCases) != 123 {
		t.Fatalf("L04 requires 107 compile/metadata and 123 DOM rows, got %d/%d", len(cases)-len(runtimeCases), len(runtimeCases))
	}
	if diagnosticCases != 6 {
		t.Fatalf("L04 requires native signatures for six @track rejection rows, got %d", diagnosticCases)
	}
	for _, group := range []string{"reactivity-triggers", "callback-order", "errorCallback", "render-switching"} {
		if !groups[group] {
			t.Fatalf("missing L04 case group %s", group)
		}
	}
	// Dependency failures must not masquerade as native compiler rejections.
	roots, err := compileToolchainRoots()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/package.json"} {
		root := roots.DependencyRoot
		if rel == "third_party/lwc/compile.mjs" {
			root = roots.ScriptRoot
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatal(err)
		}
	}

	var report strings.Builder
	matches, total, compileMatches, nativeDOMRows, localDOMRows, carried := 0, 0, 0, 0, 0, 0
	diagnosticMatches, diagnosticTotal := 0, 0
	for _, api := range versions {
		dom, domErrors, domErr := l04ObserveDOM(t, api, runtimeCases, roots.DependencyRoot, roots.ScriptRoot)
		compiled := compileConformanceBatch(t, api, len(cases), func(root string, index int) {
			l04WriteBundle(t, root, fmt.Sprintf("familyL04%03d", index), cases[index], api)
		})
		for index, c := range cases {
			compileErr := compiled[index].Err
			compileGot := "COMPILE_OK"
			if compileErr != nil {
				compileGot = "COMPILE_ERROR"
			}
			compileWant := c.Expected[api]
			if compileGot == compileWant {
				compileMatches++
			}
			got, want, reason := compileGot, compileWant, ""
			if diagnosticWant := c.DiagnosticExpected[api]; diagnosticWant != "" {
				diagnosticGot := l04CompileDiagnostic(compileErr)
				diagnosticTotal++
				if diagnosticGot == diagnosticWant && compileGot == compileWant {
					diagnosticMatches++
				}
				got, want = compileGot+"|"+diagnosticGot, compileWant+"|"+diagnosticWant
			}
			hasOracle := true
			if c.Kind == "runtime" {
				want, hasOracle = c.RuntimeExpected[api]
				child := l04ChildName(runtimeIndices[c.ID], api)
				if hasOracle {
					nativeDOMRows++
					want, err = l04DOMText(json.RawMessage(strings.TrimPrefix(want, "DOM|")), child)
					if err != nil {
						t.Fatal(err)
					}
				}
				if value, ok := dom[c.ID]; ok {
					got, err = l04DOMText(value, child)
					if err != nil {
						t.Fatal(err)
					}
					localDOMRows++
				} else {
					got = "DOM_ERROR"
					reason = "L04: local DOM observation missing"
					if domErrors[c.ID] != "" {
						reason += ": " + domErrors[c.ID]
					} else if domErr != nil {
						reason += ": " + domErr.Error()
					}
				}
			}
			status := "MISMATCH"
			if !hasOracle {
				status = "UNKNOWN"
				pending := "L04: native DOM capture pending; source compilation is not a DOM answer"
				if reason != "" {
					pending += "; " + reason
				}
				if compileGot != compileWant {
					pending += "; L04: source compile differs from native"
				}
				reason = pending
			} else if got == want && compileGot == compileWant {
				status = "MATCH"
				matches++
			} else if compileGot != compileWant {
				reason = "L04: source compile differs from native"
				if compileErr != nil {
					reason += ": " + compileErr.Error()
				}
			} else if c.DiagnosticExpected[api] != "" {
				if c.Carried[api] != "" {
					// Explicit diagnostic carries retain both signatures and owner;
					// they never enter matches or assert the carried text.
					status, reason = "CARRIED", c.Carried[api]
					carried++
					t.Logf("%s API %s diagnostic expected <%s> actual <%s>", c.ID, api, want, got)
				} else {
					reason = "L04: native compiler diagnostic signature differs"
				}
			} else if c.Carried[api] != "" && strings.HasPrefix(got, "DOM|") && reason == "" {
				// The full row still compares exactly and remains outside matches.
				// Assert the initial snapshot and rejection message even when the
				// hosted container's later outcome is carried with its owner.
				if difference := l04CarriedDifference(want, got); difference != "" {
					reason = "L04: oracle-backed carried-row fact differs at " + difference
				} else {
					status, reason = "CARRIED", c.Carried[api]
					carried++
				}
			} else if reason == "" {
				reason = "L04: " + l04DOMDifference(want, got)
			}
			total++
			fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, c.Kind, status, got, want, l04OneLine(reason), compileGot, compileWant)
			// Go string equality is exact, including case, length, and raw null.
			if status == "CARRIED" {
				t.Logf("%s API %s %s", c.ID, api, reason)
			}
			if !capture && status != "MATCH" && status != "CARRIED" {
				t.Errorf("%s API %s expected <%s> actual <%s>: %s", c.ID, api, want, got, l04OneLine(reason))
			}
		}
	}
	fmt.Fprintf(&report, "TOTAL\t%d/%d\nSOURCE_COMPILE_TOTAL\t%d/%d\n", matches, total, compileMatches, total)
	fmt.Fprintf(&report, "COMPILE_DIAGNOSTIC_TOTAL\t%d/%d\n", diagnosticMatches, diagnosticTotal)
	fmt.Fprintf(&report, "NATIVE_DOM_ROWS\t%d/%d\nLOCAL_DOM_ROWS\t%d/%d\n", nativeDOMRows, len(runtimeCases)*len(versions), localDOMRows, len(runtimeCases)*len(versions))
	fmt.Fprintf(&report, "CARRIED\t%d/%d\n", carried, total)
	t.Logf("L04 matches %d/%d; source compilation %d/%d", matches, total, compileMatches, total)
	t.Logf("L04 compiler diagnostic signatures %d/%d", diagnosticMatches, diagnosticTotal)
	t.Logf("L04 DOM rows: native %d/%d; local %d/%d", nativeDOMRows, len(runtimeCases)*len(versions), localDOMRows, len(runtimeCases)*len(versions))
	t.Logf("L04 carried %d/%d", carried, total)
	if path := os.Getenv("GLADE_L04_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

var l04DiagnosticSignatureRE = regexp.MustCompile(`^LWC[0-9]+: [^\r\n]+$`)
var l04CompileDiagnosticRE = regexp.MustCompile(`(?m)^Error: (LWC[0-9]+: [^\r\n]+)\r?$`)

func l04CompileDiagnostic(err error) string {
	if err == nil {
		return ""
	}
	// Read the actual Error message, never a code excerpt or stack frame that
	// merely contains an LWC code. Preserve message case and punctuation.
	if match := l04CompileDiagnosticRE.FindStringSubmatch(err.Error()); match != nil {
		return match[1]
	}
	return "NO_LWC_DIAGNOSTIC"
}

var l04KebabRE = regexp.MustCompile(`([a-z0-9])([A-Z])`)

func l04Tag(name string) string {
	return "c-" + strings.ToLower(l04KebabRE.ReplaceAllString(name, "${1}-${2}"))
}

func l04ChildName(index int, api string) string {
	return fmt.Sprintf("familyL04Dom%03dV%s", index, api[:2])
}

func l04WriteBundle(t *testing.T, root, name string, c l04Case, api string) {
	t.Helper()
	bundle := filepath.Join(root, "force-app", "main", "default", "lwc", name)
	js, template := c.JS, c.Template
	if c.Kind == "runtime" {
		trace := name + "Trace"
		traceDir := filepath.Join(filepath.Dir(bundle), trace)
		writeCompileFixtureFile(t, filepath.Join(traceDir, trace+".js"), "export const trace = [];")
		writeCompileFixtureFile(t, filepath.Join(traceDir, trace+".js-meta.xml"), l04Meta("", api))
		js = strings.ReplaceAll(js, "__TRACE__", trace)
		if c.Leaf != "" {
			leaf := name + "Leaf"
			l04WriteBundle(t, root, leaf, l04Case{JS: strings.ReplaceAll(c.Leaf, "__TRACE__", trace), Template: "<template><small data-value>{output}</small></template>"}, api)
			template = strings.ReplaceAll(template, "__LEAF__", l04Tag(leaf))
		}
	}
	js = strings.ReplaceAll(js, "FamilyLifecycle", strings.ToUpper(name[:1])+name[1:])
	writeCompileFixtureFile(t, filepath.Join(bundle, name+".js"), js)
	writeCompileFixtureFile(t, filepath.Join(bundle, name+".html"), template)
	writeCompileFixtureFile(t, filepath.Join(bundle, name+".js-meta.xml"), l04Meta(c.MetaFragment, api))
	for filename, content := range c.Files {
		if filepath.Base(filename) != filename || !strings.HasSuffix(filename, ".html") {
			t.Fatalf("unowned L04 auxiliary template %q", filename)
		}
		writeCompileFixtureFile(t, filepath.Join(bundle, filename), content)
	}
}

func l04Meta(fragment, api string) string {
	if fragment == "" {
		fragment = "<isExposed>false</isExposed>"
	}
	return `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion>` + fragment + `</LightningComponentBundle>`
}

func l04WriteProject(t *testing.T, root, api string) {
	t.Helper()
	writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":"`+api+`"}`)
}

func l04Compile(root string) (Manifest, error) {
	p, err := project.Load(root)
	if err != nil {
		return Manifest{}, err
	}
	return Compile(p, Options{OutDir: filepath.Join(root, "dist"), Namespace: "c"})
}

func l04ObserveDOM(t *testing.T, api string, cases []l04Case, dependencyRoot, sourceRoot string) (map[string]json.RawMessage, map[string]string, error) {
	t.Helper()
	boundaryJS, err := os.ReadFile("testdata/l04_boundary.js")
	if err != nil {
		return nil, nil, err
	}
	root := t.TempDir()
	var children strings.Builder
	for index, c := range cases {
		child := l04ChildName(index, api)
		boundary := fmt.Sprintf("familyL04Boundary%03dV%s", index, api[:2])
		l04WriteBundle(t, root, child, c, api)
		js := strings.ReplaceAll(strings.ReplaceAll(string(boundaryJS), "__TRACE__", child+"Trace"), "__CHILD__", l04Tag(child))
		template := `<template><section data-case="` + c.ID + `"><template lwc:if={show}><` + l04Tag(child) + `></` + l04Tag(child) + `> </template><button data-mount onclick={mount}>Mount</button><button data-unmount onclick={unmount}>Unmount</button><button data-next onclick={advance}>Mutate</button><button data-fire onclick={fire}>Fire</button><button data-snapshot onclick={snapshot}>Snapshot</button><pre data-result>{result}</pre></section></template>`
		l04WriteBundle(t, root, boundary, l04Case{JS: js, Template: template}, api)
		fmt.Fprintf(&children, "<%s></%s>", l04Tag(boundary), l04Tag(boundary))
	}
	page := "familyL04Runtime" + api[:2]
	l04WriteBundle(t, root, page, l04Case{JS: "import {LightningElement} from 'lwc';export default class FamilyLifecycle extends LightningElement{}", Template: "<template>" + children.String() + "</template>"}, api)
	l04WriteProject(t, root, api)
	compiled, err := l04Compile(root)
	if err != nil {
		return nil, nil, err
	}
	entry, ok := compiled.Modules["c:"+page]
	if !ok {
		return nil, nil, fmt.Errorf("L04 runtime page missing from compile output")
	}
	imports := map[string]string{"lwc": "/engine.js", "@lwc/synthetic-shadow": "/shadow.js"}
	for _, module := range compiled.Modules {
		rel, err := filepath.Rel(compiled.OutDir, module.File)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, nil, fmt.Errorf("L04 module escapes output: %s", module.File)
		}
		imports[module.ModuleKey] = "/modules/" + filepath.ToSlash(rel)
	}
	importJSON, err := json.Marshal(imports)
	if err != nil {
		return nil, nil, err
	}
	engine := filepath.Join(dependencyRoot, "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js")
	shadow := filepath.Join(dependencyRoot, "third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js")
	bootstrap := filepath.Join(sourceRoot, "lwcruntime/src/shims/lwc-engine.mjs")
	for _, file := range []string{engine, shadow, bootstrap} {
		if _, err := os.Stat(file); err != nil {
			return nil, nil, err
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			// Resolve appendChild after root creation installs synthetic lifecycle hooks.
			fmt.Fprintf(w, `<!doctype html><script>window.process={env:{NODE_ENV:"production"}}</script><script type="importmap">{"imports":%s}</script><main></main><script type="module">import "@lwc/synthetic-shadow";import {createElement} from "lwc";import Page from "c/%s";const el=createElement("%s",{is:Page});document.querySelector("main").appendChild(el);</script>`, importJSON, page, entry.Tag)
			return
		case "/favicon.ico":
			w.WriteHeader(http.StatusNoContent)
			return
		}
		file := ""
		switch {
		case r.URL.Path == "/engine.js":
			file = bootstrap
		case r.URL.Path == "/lightning/vendor/engine-dom.js":
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
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write(content)
	}))
	defer server.Close()
	repo, err := FindRepoRoot()
	if err != nil {
		return nil, nil, err
	}
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(repo, "lwcruntime/node_modules/playwright")
	}
	// The browser receives controls only; native answers cannot affect observations.
	specs := make([]map[string]any, len(cases))
	for index, c := range cases {
		specs[index] = map[string]any{"id": c.ID, "controls": c.Controls}
	}
	config, err := json.Marshal(map[string]any{"url": server.URL, "specs": specs, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")})
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", filepath.Join(repo, "internal/lwc/compile/testdata/l04_browser.mjs"))
	cmd.Stdin = bytes.NewReader(config)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, nil, fmt.Errorf("L04 local browser: %w: %s", err, l04OneLine(stderr.String()))
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

// Match the native TSV's json.dumps(sort_keys=True, ensure_ascii=True) format.
// This serializes observed JSON; string values retain their exact case/length.
func l04DOMText(raw json.RawMessage, child string) (string, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	l04NormalizeGeneratedNames(value, child, false)
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	var text strings.Builder
	text.WriteString("DOM|")
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

// Normalize only this row's generated identities in diagnostic message fields.
// DOM text, attributes, callback names, order and all other fields stay exact.
func l04NormalizeGeneratedNames(value any, child string, diagnostic bool) any {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if key == "dom" {
				continue
			}
			v[key] = l04NormalizeGeneratedNames(item, child, key == "message" || key == "messages")
		}
	case []any:
		for index, item := range v {
			v[index] = l04NormalizeGeneratedNames(item, child, diagnostic)
		}
	case string:
		if diagnostic {
			upper := strings.ToUpper(child[:1]) + child[1:]
			v = strings.NewReplacer(child, "{generated-child}", upper, "{generated-child}", l04Tag(child), "{generated-child}").Replace(v)
			// Native render diagnostics use ./m.html in their import example;
			// the local diagnostic uses this generated fixture's class name.
			if strings.HasPrefix(v, "Invalid template returned by the render() method") {
				v = strings.ReplaceAll(v, `import html from "./m.html"`, `import html from "./{generated-child}.html"`)
			}
		}
		return v
	}
	return value
}

func l04DOMDifference(want, got string) string {
	var expected, actual any
	if !strings.HasPrefix(got, "DOM|") || json.Unmarshal([]byte(strings.TrimPrefix(want, "DOM|")), &expected) != nil || json.Unmarshal([]byte(strings.TrimPrefix(got, "DOM|")), &actual) != nil {
		return "DOM observation differs from native"
	}
	return "native DOM differs at " + l04DifferencePath(expected, actual, "$")
}

// The hosted Aura failure ends the native observation before snapshot 1. Keep
// its complete row in the report and assert only facts observed on both routes.
func l04CarriedDifference(want, got string) string {
	var expected, actual map[string]any
	if json.Unmarshal([]byte(strings.TrimPrefix(want, "DOM|")), &expected) != nil || json.Unmarshal([]byte(strings.TrimPrefix(got, "DOM|")), &actual) != nil {
		return "$"
	}
	if difference := l04DifferencePath(expected["console"], actual["console"], "$.console"); difference != "" {
		return difference
	}
	wantSteps, wantOK := expected["steps"].([]any)
	gotSteps, gotOK := actual["steps"].([]any)
	if !wantOK || !gotOK || len(wantSteps) == 0 || len(gotSteps) == 0 {
		return "$.steps[0]"
	}
	return l04DifferencePath(wantSteps[0], gotSteps[0], "$.steps[0]")
}

func l04DifferencePath(expected, actual any, path string) string {
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
			value := want[key]
			other, ok := got[key]
			if !ok {
				return path + "." + key
			}
			if difference := l04DifferencePath(value, other, path+"."+key); difference != "" {
				return difference
			}
		}
	case []any:
		got, ok := actual.([]any)
		if !ok || len(want) != len(got) {
			return path + ".length"
		}
		for index, value := range want {
			if difference := l04DifferencePath(value, got[index], fmt.Sprintf("%s[%d]", path, index)); difference != "" {
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

func l04OneLine(text string) string {
	return strings.NewReplacer("\t", " ", "\r", " ", "\n", " ").Replace(text)
}
