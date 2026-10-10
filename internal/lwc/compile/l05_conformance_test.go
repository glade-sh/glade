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
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/glade-sh/glade/internal/project"
)

type l05Case struct {
	ID                 string            `json:"id"`
	Group              string            `json:"group"`
	Kind               string            `json:"kind"`
	Basis              string            `json:"basis"`
	JS                 string            `json:"js"`
	Template           string            `json:"template"`
	MetaFragment       string            `json:"meta_fragment"`
	Action             string            `json:"action"`
	ParentAttrs        string            `json:"parent_attrs"`
	ListenerAction     string            `json:"listener_action"`
	EventType          string            `json:"event_type"`
	Expected           map[string]string `json:"expected"`
	RuntimeExpected    map[string]string `json:"runtime_expected"`
	DiagnosticExpected map[string]string `json:"diagnostic_expected"`
	RejectionExpected  map[string]string `json:"rejection_expected"`
	DiagnosticCarried  map[string]string `json:"diagnostic_carried"`
	Carried            map[string]string `json:"carried"`
	CarriedPaths       []string          `json:"carried_paths"`
}

// TestL05SalesforceConformance replays owned fixtures against exported native
// API 59/67 observations. Capture mode produces the before report without
// failing on differences. Normal mode asserts matching rows exactly and logs
// only declared external-owner carries. It never fetches Salesforce or imports
// glade-tools.
func TestL05SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_L05_CAPTURE") != ""
	data, err := os.ReadFile("testdata/l05_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []l05Case
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	seen, groups := map[string]bool{}, map[string]bool{}
	var runtimeCases []l05Case
	var compileIndices []int
	for index, c := range cases {
		if c.ID == "" || seen[c.ID] || c.Basis != "org" || (c.Kind != "compile" && c.Kind != "runtime") {
			t.Fatalf("invalid L05 row %q", c.ID)
		}
		seen[c.ID], groups[c.Group] = true, true
		for _, api := range []string{"59.0", "67.0"} {
			if c.Expected[api] != "COMPILE_OK" && c.Expected[api] != "COMPILE_ERROR" {
				t.Fatalf("missing native compile answer: %s API %s", c.ID, api)
			}
			if c.Kind == "compile" && c.Expected[api] == "COMPILE_ERROR" && c.RejectionExpected[api] == "" {
				t.Fatalf("missing native rejection diagnostic: %s API %s", c.ID, api)
			}
			for _, carry := range []string{c.Carried[api], c.DiagnosticCarried[api]} {
				if carry != "" && (!strings.HasPrefix(carry, "L07: ") && !strings.HasPrefix(carry, "L23: ")) {
					t.Fatalf("L05 carry needs another family's explicit owner: %s API %s", c.ID, api)
				}
			}
			if c.Kind == "runtime" {
				want, ok := c.RuntimeExpected[api]
				if !ok || !strings.HasPrefix(want, "DOM|") || !json.Valid([]byte(strings.TrimPrefix(want, "DOM|"))) {
					t.Fatalf("missing native DOM answer: %s API %s", c.ID, api)
				}
			}
		}
		if c.Kind == "runtime" {
			runtimeCases = append(runtimeCases, c)
		} else {
			compileIndices = append(compileIndices, index)
		}
	}
	if len(cases) != 223 || len(runtimeCases) != 128 {
		t.Fatalf("L05 requires 95 compile/metadata and 128 DOM rows, got %d/%d", len(cases)-len(runtimeCases), len(runtimeCases))
	}
	for _, group := range []string{"api-properties", "api-declarations", "api-accessors", "api-methods", "mutation-source", "event-compiler", "api-metadata", "api-attribute-conversion", "mutation-errors", "event-propagation-retargeting", "dispatchEvent-errors-cancellation"} {
		if !groups[group] {
			t.Fatalf("missing L05 group %s", group)
		}
	}
	// Missing dependencies are infrastructure errors, never compiler rejections.
	roots, err := compileToolchainRoots()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{
		filepath.Join(roots.ScriptRoot, "third_party/lwc/compile.mjs"),
		filepath.Join(roots.DependencyRoot, "third_party/lwc/node_modules/@lwc/compiler/package.json"),
		filepath.Join(roots.DependencyRoot, "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js"),
		filepath.Join(roots.DependencyRoot, "third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js"),
	} {
		if _, err := os.Stat(file); err != nil {
			t.Fatal(err)
		}
	}

	var report strings.Builder
	fmt.Fprintln(&report, "api\tid\tkind\tstatus\tactual\texpected\treason")
	matches, total, compileMatches, domMatches, carried := 0, 0, 0, 0, 0
	rejectionMatches, rejectionTotal, rejectionCarried := 0, 0, 0
	record := func(c l05Case, api, got, want, reason, carry string) string {
		match := got == want // Exact Go string comparison preserves case, lengths and raw JSON null.
		status := "MISMATCH"
		if match {
			status = "MATCH"
		} else if carry != "" {
			status, reason = "CARRIED", carry
			t.Logf("%s API %s %s expected <%s> actual <%s>", c.ID, api, carry, want, got)
		} else if reason == "" {
			reason = "L05: observed row differs from native"
		}
		fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, c.Kind, status, l05OneLine(got), l05OneLine(want), l05OneLine(reason))
		if !capture && status == "MISMATCH" {
			t.Errorf("%s API %s expected <%s> actual <%s>: %s", c.ID, api, want, got, l05OneLine(reason))
		}
		return status
	}
	for _, api := range []string{"59.0", "67.0"} {
		compiled := compileConformanceBatch(t, api, len(compileIndices), func(root string, index int) {
			caseIndex := compileIndices[index]
			l05WriteBundle(t, root, fmt.Sprintf("familyL05Compile%03d", caseIndex), cases[caseIndex], api)
		})
		dom, domErrors, runtimeCompileErr, browserErr := l05ObserveDOM(t, api, runtimeCases, roots.DependencyRoot)
		apiMatches, localDOMRows, apiCarried := 0, 0, 0
		compileIndex := 0
		for _, c := range cases {
			got, want, reason, carry := "", c.Expected[api], "", ""
			if c.Kind == "compile" {
				compileErr := compiled[compileIndex].Err
				compileIndex++
				got = "COMPILE_OK"
				if compileErr != nil {
					got = "COMPILE_ERROR"
				}
				if got != want {
					reason = "L05: source compile differs from native"
					if c.Group == "api-metadata" {
						reason = "L05: public property metadata validation differs from native"
					}
					if compileErr != nil {
						reason += ": " + compileErr.Error()
					}
				}
				if got == want {
					compileMatches++
				}
				if signature := c.RejectionExpected[api]; signature != "" {
					actual := l05RejectionDiagnostic(compileErr, api)
					diagnosticCase := c
					diagnosticCase.Kind = "diagnostic"
					diagnosticCarry := ""
					if got == c.Expected[api] {
						diagnosticCarry = c.DiagnosticCarried[api]
					}
					rejectionTotal++
					switch record(diagnosticCase, api, actual, signature, "L05: rejection diagnostic differs from native", diagnosticCarry) {
					case "MATCH":
						rejectionMatches++
					case "CARRIED":
						rejectionCarried++
					}
				}
				if signature := c.DiagnosticExpected[api]; signature != "" {
					got += "|" + l05Diagnostic(compileErr, api)
					want += "|" + signature
					if reason == "" && got != want {
						reason = "L05: c_event_call compiler diagnostic differs at the captured API 66 boundary"
					}
				}
			} else {
				want = c.RuntimeExpected[api]
				if value, ok := dom[c.ID]; ok {
					got, err = l05DOMText(value)
					if err != nil {
						t.Fatal(err)
					}
					localDOMRows++
					if c.Carried[api] != "" && l05DOMCarryAllowed(want, got, c.CarriedPaths) {
						carry = c.Carried[api]
					}
				} else {
					got = "DOM_ERROR"
					reason = "L05: local DOM observation missing"
					if domErrors[c.ID] != "" {
						reason += ": " + domErrors[c.ID]
					} else if runtimeCompileErr != nil {
						reason += ": source compile: " + runtimeCompileErr.Error()
					} else if browserErr != nil {
						reason += ": " + browserErr.Error()
					}
				}
				if got != want && reason == "" {
					reason = "L05: public API/event DOM observation differs from native"
				}
			}
			total++
			status := record(c, api, got, want, reason, carry)
			if status == "CARRIED" {
				carried++
				apiCarried++
			}
			if status == "MATCH" {
				matches++
				apiMatches++
				if c.Kind == "runtime" {
					domMatches++
				}
			}
		}
		fmt.Fprintf(&report, "API_TOTAL\t%s\t%d/223\nLOCAL_DOM_ROWS\t%s\t%d/128\nAPI_CARRIED\t%s\t%d/223\n", api, apiMatches, api, localDOMRows, api, apiCarried)
		t.Logf("L05 API %s matches %d/223; local DOM observations %d/128; carried %d/223", api, apiMatches, localDOMRows, apiCarried)
	}
	// Native bisection answers are extra diagnostic checks, outside the 446-row
	// floor/ceiling denominator. No intermediate answer is inferred from endpoints.
	gateMatches, gateTotal := 0, 0
	for index, c := range cases {
		if c.ID != "c_event_call" {
			continue
		}
		for _, api := range []string{"63.0", "65.0", "66.0"} {
			if c.Expected[api] != "COMPILE_ERROR" || c.DiagnosticExpected[api] == "" {
				t.Fatalf("missing captured L05 diagnostic gate API %s", api)
			}
			compiled := compileConformanceBatch(t, api, 1, func(root string, _ int) {
				l05WriteBundle(t, root, fmt.Sprintf("familyL05Compile%03d", index), c, api)
			})
			compileErr := compiled[0].Err
			verdict := "COMPILE_OK"
			if compileErr != nil {
				verdict = "COMPILE_ERROR"
			}
			got, want := verdict+"|"+l05Diagnostic(compileErr, api), c.Expected[api]+"|"+c.DiagnosticExpected[api]
			gateTotal++
			if record(c, api, got, want, "L05: captured c_event_call compiler diagnostic gate differs", "") == "MATCH" {
				gateMatches++
			}
		}
	}
	fmt.Fprintf(&report, "TOTAL\t%d/%d\nSOURCE_COMPILE_TOTAL\t%d/190\nDOM_TOTAL\t%d/256\nDIAGNOSTIC_GATE_TOTAL\t%d/%d\n", matches, total, compileMatches, domMatches, gateMatches, gateTotal)
	fmt.Fprintf(&report, "CARRIED\t%d/%d\nREJECTION_DIAGNOSTIC_TOTAL\t%d/%d\nREJECTION_DIAGNOSTIC_CARRIED\t%d/%d\n", carried, total, rejectionMatches, rejectionTotal, rejectionCarried, rejectionTotal)
	t.Logf("L05 matches %d/%d; source compile %d/190; DOM %d/256; diagnostic gates %d/%d; carried %d/%d", matches, total, compileMatches, domMatches, gateMatches, gateTotal, carried, total)
	t.Logf("L05 rejection diagnostic matches %d/%d; carried %d/%d", rejectionMatches, rejectionTotal, rejectionCarried, rejectionTotal)
	if path := os.Getenv("GLADE_L05_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

var l05KebabRE = regexp.MustCompile(`([a-z0-9])([A-Z])`)

func l05Tag(name string) string {
	return "c-" + strings.ToLower(l05KebabRE.ReplaceAllString(name, "${1}-${2}"))
}
func l05WriteBundle(t *testing.T, root, name string, c l05Case, api string) {
	t.Helper()
	bundle := filepath.Join(root, "force-app", "main", "default", "lwc", name)
	js := strings.ReplaceAll(c.JS, "FamilyApi", strings.ToUpper(name[:1])+name[1:])
	fragment := c.MetaFragment
	if fragment == "" {
		fragment = "<isExposed>false</isExposed>"
	}
	meta := `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion>` + fragment + `</LightningComponentBundle>`
	writeCompileFixtureFile(t, filepath.Join(bundle, name+".js"), js)
	writeCompileFixtureFile(t, filepath.Join(bundle, name+".html"), c.Template)
	writeCompileFixtureFile(t, filepath.Join(bundle, name+".js-meta.xml"), meta)
}
func l05WriteProject(t *testing.T, root, api string) {
	t.Helper()
	writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":"`+api+`"}`)
}
func l05Compile(root string) (Manifest, error) {
	p, err := project.Load(root)
	if err != nil {
		return Manifest{}, err
	}
	return Compile(p, Options{OutDir: filepath.Join(root, "dist"), Namespace: "c"})
}

// Native c_event_call/c_method_constructor carry source locations. The capture's
// signatures export removes that header; retain the complete diagnostic body.
var l05DiagnosticRE = regexp.MustCompile(`(?m)^Error: (?:\[Line: [0-9]+, Col: [0-9]+\] )?((?:LWC[0-9]+: |The |More than one |You specified )[^\r\n]+)\r?$`)
var l05MetadataDiagnosticRE = regexp.MustCompile(`(?m)\.js-meta\.xml: ((?:The |More than one |You specified )[^\r\n]+)$`)
var l05DiagnosticSourceRE = regexp.MustCompile(`/[^\s:]+\.js`)

func l05RejectionDiagnostic(err error, api string) string {
	if err == nil {
		return ""
	}
	message := ""
	if match := l05DiagnosticRE.FindStringSubmatch(err.Error()); match != nil {
		message = match[1]
	} else if match := l05MetadataDiagnosticRE.FindStringSubmatch(err.Error()); match != nil {
		message = match[1]
	} else {
		// Preserve a backend parser rejection's actual first line even when its
		// diagnostic envelope differs. Never substitute an expected diagnostic.
		_, output, ok := strings.Cut(err.Error(), "\n")
		if !ok {
			output = err.Error()
		}
		message = strings.Split(output, "\n")[0]
	}
	message = l05DiagnosticSourceRE.ReplaceAllString(message, "<source>")
	return strings.ReplaceAll(message, "current component API version ("+strings.TrimSuffix(api, ".0")+")", "current component API version (<source-api>)")
}

func l05Diagnostic(err error, api string) string {
	return l05RejectionDiagnostic(err, api)
}
func l05OneLine(text string) string {
	return strings.NewReplacer("\t", " ", "\r", " ", "\n", " ").Replace(text)
}

// Serialize only the observation in the native TSV's JSON text format. No
// fields are removed or renamed; arrays retain order and null stays raw null.
func l05DOMText(raw json.RawMessage) (string, error) {
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

func l05ObserveDOM(t *testing.T, api string, cases []l05Case, dependencyRoot string) (map[string]json.RawMessage, map[string]string, error, error) {
	t.Helper()
	parentJS, err := os.ReadFile("testdata/l05_parent.js")
	if err != nil {
		return nil, nil, nil, err
	}
	root := t.TempDir()
	var children strings.Builder
	ids := make([]string, len(cases))
	for index, c := range cases {
		child := fmt.Sprintf("familyL05Child%03dV%s", index, api[:2])
		parent := fmt.Sprintf("familyL05Row%03dV%s", index, api[:2])
		tag := l05Tag(child)
		l05WriteBundle(t, root, child, c, api)
		eventJSON, _ := json.Marshal(c.EventType)
		idJSON, _ := json.Marshal(c.ID)
		js := strings.NewReplacer("__CHILD__", tag, "__ACTION__", c.Action, "__LISTENER_ACTION__", c.ListenerAction, "__EVENT__", string(eventJSON), "__ID__", string(idJSON)).Replace(string(parentJS))
		template := `<template><section data-case="` + c.ID + `"><div data-node="wrapper"><` + tag + ` ` + c.ParentAttrs + `></` + tag + `></div><button data-run onclick={run}>Capture</button><pre data-result>{result}</pre></section></template>`
		l05WriteBundle(t, root, parent, l05Case{JS: js, Template: template}, api)
		fmt.Fprintf(&children, "<%s></%s>\n", l05Tag(parent), l05Tag(parent))
		ids[index] = c.ID
	}
	page := "familyL05Runtime" + api[:2]
	l05WriteBundle(t, root, page, l05Case{JS: "import {LightningElement} from 'lwc';export default class FamilyApi extends LightningElement{}", Template: "<template>" + children.String() + "</template>"}, api)
	l05WriteProject(t, root, api)
	compiled, err := l05Compile(root)
	if err != nil {
		return nil, nil, err, nil
	}
	entry, ok := compiled.Modules["c:"+page]
	if !ok {
		return nil, nil, nil, fmt.Errorf("L05 runtime page missing from compile output")
	}
	imports := map[string]string{"lwc": "/engine.js", "@lwc/synthetic-shadow": "/shadow.js"}
	for _, module := range compiled.Modules {
		rel, err := filepath.Rel(compiled.OutDir, module.File)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, nil, nil, fmt.Errorf("L05 module escapes output: %s", module.File)
		}
		imports[module.ModuleKey] = "/modules/" + filepath.ToSlash(rel)
	}
	importJSON, err := json.Marshal(imports)
	if err != nil {
		return nil, nil, nil, err
	}
	engine := filepath.Join(dependencyRoot, "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js")
	shadow := filepath.Join(dependencyRoot, "third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js")
	for _, file := range []string{engine, shadow} {
		if _, err := os.Stat(file); err != nil {
			return nil, nil, nil, err
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><script>window.process={env:{NODE_ENV:"production"}}</script><script type="importmap">{"imports":%s}</script><main></main><script type="module">import "@lwc/synthetic-shadow";import {createElement} from "lwc";import Page from "c/%s";document.querySelector("main").appendChild(createElement("%s",{is:Page}));</script>`, importJSON, page, entry.Tag)
			return
		case "/favicon.ico":
			w.WriteHeader(http.StatusNoContent)
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
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write(content)
	}))
	defer server.Close()
	repo, err := FindRepoRoot()
	if err != nil {
		return nil, nil, nil, err
	}
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(repo, "lwcruntime/node_modules/playwright")
	}

	// Only IDs and local connection settings enter the browser, never native answers.
	config, err := json.Marshal(map[string]any{"url": server.URL, "ids": ids, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")})
	if err != nil {
		return nil, nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", filepath.Join(repo, "internal/lwc/compile/testdata/l05_browser.mjs"))
	cmd.Stdin = bytes.NewReader(config)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, nil, nil, fmt.Errorf("L05 local browser: %w: %s", err, l05OneLine(stderr.String()))
	}
	var observation struct {
		Values map[string]json.RawMessage `json:"values"`
		Errors map[string]string          `json:"errors"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &observation); err != nil {
		return nil, nil, nil, err
	}
	for id := range observation.Values {
		found := false
		for _, known := range ids {
			if id == known {
				found = true
				break
			}
		}
		if !found {
			return nil, nil, nil, fmt.Errorf("unexpected L05 browser row %s", id)
		}
	}
	return observation.Values, observation.Errors, nil, nil
}

// A carry is allowed only for the fields identified in the before report.
// The full native/local row stays exact in the report and outside matches.
// Any additional difference is a new mismatch, never hidden behind a carry.
func l05DOMCarryAllowed(want, got string, paths []string) bool {
	if len(paths) == 0 || !strings.HasPrefix(want, "DOM|") || !strings.HasPrefix(got, "DOM|") {
		return false
	}
	var expected, actual any
	for _, input := range []struct {
		text string
		dst  *any
	}{{want, &expected}, {got, &actual}} {
		decoder := json.NewDecoder(strings.NewReader(strings.TrimPrefix(input.text, "DOM|")))
		decoder.UseNumber()
		if decoder.Decode(input.dst) != nil {
			return false
		}
	}
	allowed := map[string]bool{}
	for _, path := range paths {
		allowed[path] = true
	}
	var compare func(any, any, string) bool
	compare = func(want, got any, path string) bool {
		if allowed[path] {
			return true
		}
		switch value := want.(type) {
		case map[string]any:
			other, ok := got.(map[string]any)
			if !ok || len(value) != len(other) {
				return false
			}
			for key, item := range value {
				actual, present := other[key]
				if !present || !compare(item, actual, path+"."+key) {
					return false
				}
			}
			return true
		case []any:
			other, ok := got.([]any)
			if !ok || len(value) != len(other) {
				return false
			}
			for index, item := range value {
				if !compare(item, other[index], fmt.Sprintf("%s[%d]", path, index)) {
					return false
				}
			}
			return true
		default:
			return reflect.DeepEqual(want, got)
		}
	}
	return compare(expected, actual, "$")
}
