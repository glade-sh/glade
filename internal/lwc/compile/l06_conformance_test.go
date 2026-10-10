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
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	lwcembed "github.com/glade-sh/glade/internal/lwcruntime/embed"
	"github.com/glade-sh/glade/internal/project"
)

type l06Bundle struct {
	JS           string `json:"js"`
	Template     string `json:"template"`
	MetaFragment string `json:"meta_fragment"`
}

type l06Case struct {
	l06Bundle
	ID                string              `json:"id"`
	Group             string              `json:"group"`
	Kind              string              `json:"kind"`
	BundleName        string              `json:"bundle_name"`
	Child             *l06Bundle          `json:"child"`
	Frames            []int               `json:"frames"`
	Expected          map[string]string   `json:"expected"`
	NativeDiagnostics map[string][]string `json:"native_diagnostics"`
	Carried           map[string]l06Carry `json:"carried"`
}

type l06Carry struct {
	Owner          string `json:"owner"`
	Reason         string `json:"reason"`
	Observed       string `json:"observed"`
	ObservedReason string `json:"observed_reason"`
}

// TestL06SalesforceConformance compares the exact exported native row text.
// identify the compiler and browser observations. The recorded table has 184
// compile/metadata and 56 DOM rows per API. Compiler rows compare the captured
// outcome and diagnostic text, while DOM rows include every observed frame,
// error and console entry. Compiler diagnostics retain source positions and
// the echoed source API. Known differences owned by other families are logged
// only while the observed text and cause still match the canonical capture.
// GLADE_L06_CAPTURE reports mismatches without failing row checks, allowing a
// before measurement on an unchanged product tree. CI needs no Salesforce auth.
func TestL06SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_L06_CAPTURE") != ""
	data, err := os.ReadFile("testdata/l06_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []l06Case
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	versions := []string{"59.0", "67.0"}
	seen, groups := map[string]bool{}, map[string]bool{}
	var compilerCases, runtimeCases []l06Case
	for _, c := range cases {
		if c.ID == "" || seen[c.ID] || c.JS == "" || c.Template == "" {
			t.Fatalf("invalid or duplicate L06 input %q", c.ID)
		}
		seen[c.ID], groups[c.Group] = true, true
		for _, api := range versions {
			want, ok := c.Expected[api]
			if !ok {
				t.Fatalf("missing native L06 row %s API %s", c.ID, api)
			}
			if carry, ok := c.Carried[api]; ok {
				if carry.Owner != "component compiler" || carry.Reason == "" || carry.Observed == "" || carry.ObservedReason == "" {
					t.Fatalf("invalid L06 carry %s API %s: needs another family's owner, reason and captured observation", c.ID, api)
				}
			}
			switch c.Kind {
			case "compile":
				if want != "COMPILE_OK" && want != "COMPILE_ERROR" {
					t.Fatalf("invalid native compiler answer %s API %s: %q", c.ID, api, want)
				}
				messages, observed := c.NativeDiagnostics[api]
				if !observed || (len(messages) != 0) != (want == "COMPILE_ERROR") {
					t.Fatalf("missing native diagnostic answer %s API %s", c.ID, api)
				}
			case "runtime":
				if !strings.HasPrefix(want, "DOM|") {
					t.Fatalf("missing native DOM answer %s API %s", c.ID, api)
				}
				text, err := l06DOMText(json.RawMessage(strings.TrimPrefix(want, "DOM|")))
				if err != nil || text != want {
					t.Fatalf("invalid native DOM text %s API %s: %v", c.ID, api, err)
				}
			default:
				t.Fatalf("unknown L06 row kind %q", c.Kind)
			}
		}
		if c.Kind == "compile" {
			if c.BundleName == "" {
				t.Fatalf("missing captured bundle identity for %s", c.ID)
			}
			compilerCases = append(compilerCases, c)
		} else {
			if len(c.Frames) == 0 {
				t.Fatalf("missing DOM frames for %s", c.ID)
			}
			for index, frame := range c.Frames {
				if frame != index {
					t.Fatalf("invalid DOM frame sequence for %s", c.ID)
				}
			}
			runtimeCases = append(runtimeCases, c)
		}
	}
	if len(compilerCases) != 184 || len(runtimeCases) != 56 {
		t.Fatalf("L06 requires 184 compiler/metadata and 56 DOM rows, got %d/%d", len(compilerCases), len(runtimeCases))
	}
	for _, group := range []string{"shadow-light", "slots", "scoped-slots", "refs", "spread", "manual-dom", "dynamic-components", "dom-metadata"} {
		if !groups[group] {
			t.Fatalf("missing L06 case group %s", group)
		}
	}
	// A missing toolchain must fail as an environment error, never match a
	// native compiler rejection or turn missing browser output into DOM parity.
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
	matches, total, compilerMatches, domMatches, observedDOM, carried := 0, 0, 0, 0, 0, 0
	sourceCompileMatches, diagnosticMatches, diagnosticTotal := 0, 0, 0
	for _, api := range versions {
		compiled := compileConformanceBatch(t, api, len(compilerCases), func(root string, index int) {
			c := compilerCases[index]
			l06PrepareCase(t, root, c.BundleName, c, api)
		})
		dom, domErrors, domErr := l06ObserveDOM(t, api, runtimeCases, roots.ScriptRoot, roots.DependencyRoot)
		compilerIndex := 0
		for _, c := range cases {
			want := c.Expected[api]
			got, reason := "", ""
			if c.Kind == "compile" {
				result := compiled[compilerIndex]
				compilerIndex++
				outcome := "COMPILE_OK"
				if result.Err != nil {
					outcome = "COMPILE_ERROR"
				}
				if outcome == c.Expected[api] {
					sourceCompileMatches++
				} else {
					reason = "L06: native compiler/metadata outcome differs"
					if result.Err != nil {
						reason += ": " + result.Err.Error()
					}
				}
				got = outcome + "|" + l06DiagnosticText(t, l06LocalDiagnostics(result))
				want += "|" + l06DiagnosticText(t, c.NativeDiagnostics[api])
				if c.Expected[api] == "COMPILE_ERROR" {
					diagnosticTotal++
					if got == want {
						diagnosticMatches++
					}
				}
				if got != want && reason == "" {
					reason = "L06: native compiler diagnostic text differs"
				}
			} else if value, ok := dom[c.ID]; ok {
				got, err = l06DOMText(value)
				if err != nil {
					t.Fatalf("invalid local DOM JSON for %s API %s: %v", c.ID, api, err)
				}
				observedDOM++
				if got != want {
					reason = "L06: " + l04DOMDifference(want, got)
				}
			} else {
				got = "DOM_ERROR"
				reason = "L06: local DOM observation missing"
				if rowError := domErrors[c.ID]; rowError != "" {
					reason = rowError
				} else if domErr != nil {
					reason += ": " + domErr.Error()
				}
			}
			status := "MISMATCH"
			// Go string equality is exact: case, length and JSON null all matter.
			// Carried fixture reasons omit the family label the built reason starts with.
			if got == want {
				status = "MATCH"
				matches++
				if c.Kind == "compile" {
					compilerMatches++
				} else {
					domMatches++
				}
			} else if carry, ok := c.Carried[api]; ok && got == carry.Observed && strings.TrimPrefix(l06OneLine(reason), "L06: ") == carry.ObservedReason {
				status, reason = "CARRIED", carry.Owner+": "+carry.Reason
				carried++
				t.Logf("%s API %s %s expected <%s> actual <%s>", c.ID, api, reason, want, got)
			}
			total++
			fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, c.ID, c.Kind, status, got, want, l06OneLine(reason))
			if !capture && status != "MATCH" && status != "CARRIED" {
				t.Errorf("%s API %s expected <%s> actual <%s>: %s", c.ID, api, want, got, l06OneLine(reason))
			}
		}
	}
	fmt.Fprintf(&report, "TOTAL\t%d/%d\nCOMPILE_TOTAL\t%d/%d\nDOM_TOTAL\t%d/%d\nLOCAL_DOM_ROWS\t%d/%d\n", matches, total, compilerMatches, len(compilerCases)*len(versions), domMatches, len(runtimeCases)*len(versions), observedDOM, len(runtimeCases)*len(versions))
	fmt.Fprintf(&report, "SOURCE_COMPILE_TOTAL\t%d/%d\nDIAGNOSTIC_TOTAL\t%d/%d\n", sourceCompileMatches, len(compilerCases)*len(versions), diagnosticMatches, diagnosticTotal)
	fmt.Fprintf(&report, "CARRIED\t%d/%d\n", carried, total)
	t.Logf("L06 matches %d/%d; compiler/metadata %d/%d; DOM %d/%d; local DOM observations %d/%d", matches, total, compilerMatches, len(compilerCases)*len(versions), domMatches, len(runtimeCases)*len(versions), observedDOM, len(runtimeCases)*len(versions))
	t.Logf("L06 source compiler outcomes %d/%d; exact rejection diagnostics %d/%d; carried %d/%d", sourceCompileMatches, len(compilerCases)*len(versions), diagnosticMatches, diagnosticTotal, carried, total)
	if path := os.Getenv("GLADE_L06_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

var l06ErrorLineRE = regexp.MustCompile(`(?m)^Error: ([^\r\n]+)\r?$`)
var l06MetadataPathRE = regexp.MustCompile(`^[^\r\n]+\.js-meta\.xml: `)

func l06LocalDiagnostics(result BundleResult) []string {
	if result.Err == nil {
		return []string{}
	}
	if len(result.DeploymentDiagnostics) != 0 {
		return result.DeploymentDiagnostics
	}
	// The Error line retains the deployment envelope around expression errors.
	// Structured compiler diagnostics retain the underlying error separately.
	if len(result.Diagnostics) <= 1 {
		if match := l06ErrorLineRE.FindStringSubmatch(result.Err.Error()); match != nil {
			return []string{match[1]}
		}
	}
	if len(result.Diagnostics) != 0 {
		messages := make([]string, 0, len(result.Diagnostics))
		for _, diagnostic := range result.Diagnostics {
			messages = append(messages, diagnostic.Message)
		}
		return messages
	}
	// A metadata validator prefixes its own fixture path to the actual message.
	return []string{l06MetadataPathRE.ReplaceAllString(result.Err.Error(), "")}
}

func l06DiagnosticText(t *testing.T, messages []string) string {
	t.Helper()
	text := append([]string{}, messages...)
	sort.Strings(text)
	return l03JSON(t, text)
}

var l06KebabRE = regexp.MustCompile(`([a-z0-9])([A-Z])`)

func l06Tag(name string) string {
	return "c-" + strings.ToLower(l06KebabRE.ReplaceAllString(name, "${1}-${2}"))
}

func l06WriteBundle(t *testing.T, root, name string, bundle l06Bundle, api string) {
	t.Helper()
	dir := filepath.Join(root, "force-app", "main", "default", "lwc", name)
	js := strings.ReplaceAll(bundle.JS, "FamilyDom", strings.ToUpper(name[:1])+name[1:])
	writeCompileFixtureFile(t, filepath.Join(dir, name+".js"), js)
	writeCompileFixtureFile(t, filepath.Join(dir, name+".html"), bundle.Template)
	fragment := bundle.MetaFragment
	if fragment == "" {
		fragment = "<isExposed>false</isExposed>"
	}
	meta := `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion>` + fragment + `</LightningComponentBundle>`
	writeCompileFixtureFile(t, filepath.Join(dir, name+".js-meta.xml"), meta)
}

func l06PrepareCase(t *testing.T, root, name string, c l06Case, api string) {
	t.Helper()
	child := name + "Child"
	if strings.Contains(c.JS+c.Template, "__CHILD_") {
		if c.Child == nil {
			t.Fatalf("missing owned child fixture for %s", c.ID)
		}
		l06WriteBundle(t, root, child, *c.Child, api)
	}
	prepared := c.l06Bundle
	replace := strings.NewReplacer("__CHILD_TAG__", strings.TrimPrefix(l06Tag(child), "c-"), "__CHILD_NAME__", child)
	prepared.JS, prepared.Template = replace.Replace(prepared.JS), replace.Replace(prepared.Template)
	l06WriteBundle(t, root, name, prepared, api)
}

func l06ObserveDOM(t *testing.T, api string, cases []l06Case, sourceRoot, dependencyRoot string) (map[string]json.RawMessage, map[string]string, error) {
	t.Helper()
	boundaryJS, err := os.ReadFile("testdata/l06_boundary.js")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	rowBundles := make([][]string, len(cases))
	for index, c := range cases {
		child := fmt.Sprintf("familyL06Dom%03dV%s", index, api[:2])
		boundary := fmt.Sprintf("familyL06Boundary%03dV%s", index, api[:2])
		l06PrepareCase(t, root, child, c, api)
		rowBundles[index] = []string{child, boundary}
		if strings.Contains(c.JS+c.Template, "__CHILD_") {
			rowBundles[index] = append(rowBundles[index], child+"Child")
		}
		id, err := json.Marshal(c.ID)
		if err != nil {
			t.Fatal(err)
		}
		js := strings.NewReplacer("__CASE__", string(id), "__CHILD__", l06Tag(child)).Replace(string(boundaryJS))
		template := `<template><section data-case="` + c.ID + `"><` + l06Tag(child) + ` onobserve={handleObserve}></` + l06Tag(child) + `><button data-next onclick={advance}>Next</button><pre data-result>{result}</pre></section></template>`
		l06WriteBundle(t, root, boundary, l06Bundle{JS: js, Template: template}, api)
	}
	writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":"`+api+`"}`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	outputRoot := filepath.Join(root, "dist")
	bundles, err := CompileBatch(p, Options{OutDir: outputRoot, Namespace: "c"})
	if err != nil {
		t.Fatal(err)
	}
	// A rejected fixture stays a failed row. Only its own bundles are excluded
	// from the browser page, so it cannot suppress unrelated native DOM rows.
	compiled := Manifest{OutDir: outputRoot, Modules: map[string]ModuleEntry{}}
	rowErrors := map[string]string{}
	activeCases := make([]l06Case, 0, len(cases))
	var children strings.Builder
	for index, c := range cases {
		for _, name := range rowBundles[index] {
			key := "force-app/main/default/lwc/" + name
			result, ok := bundles[key]
			if !ok {
				t.Fatalf("L06 batch omitted bundle %s", key)
			}
			if result.Err != nil {
				rowErrors[c.ID] = "L06: native-deployed DOM fixture rejected by local compiler: " + l06OneLine(result.Err.Error())
				break
			}
		}
		if rowErrors[c.ID] != "" {
			continue
		}
		for _, name := range rowBundles[index] {
			for key, module := range bundles["force-app/main/default/lwc/"+name].Manifest.Modules {
				compiled.Modules[key] = module
			}
		}
		activeCases = append(activeCases, c)
		boundary := rowBundles[index][1]
		fmt.Fprintf(&children, "<%s></%s>", l06Tag(boundary), l06Tag(boundary))
	}
	if len(activeCases) == 0 {
		return map[string]json.RawMessage{}, rowErrors, nil
	}
	page := "familyL06Runtime" + api[:2]
	l06WriteBundle(t, root, page, l06Bundle{JS: "import { LightningElement } from 'lwc'; export default class FamilyDom extends LightningElement {}", Template: "<template>" + children.String() + "</template>"}, api)
	p, err = project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	pageRoot := filepath.Join(root, "force-app", "main", "default", "lwc", page)
	pageManifest, err := Compile(compileCaseProject(p, pageRoot), Options{OutDir: filepath.Join(outputRoot, "entry"), Namespace: "c"})
	if err != nil {
		t.Fatalf("L06 observer page compilation: %v", err)
	}
	for key, module := range pageManifest.Modules {
		compiled.Modules[key] = module
	}
	entry, ok := compiled.Modules["c:"+page]
	if !ok {
		t.Fatal("L06 runtime page missing from compile output")
	}
	imports := map[string]string{"lwc": "/engine.js", "@lwc/synthetic-shadow": "/shadow.js"}
	for _, module := range compiled.Modules {
		rel, err := filepath.Rel(compiled.OutDir, module.File)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("L06 module escapes output: %s", module.File)
		}
		imports[module.ModuleKey] = "/modules/" + filepath.ToSlash(rel)
	}
	importJSON, err := json.Marshal(imports)
	if err != nil {
		t.Fatal(err)
	}
	engine := filepath.Join(dependencyRoot, "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js")
	shadow := filepath.Join(dependencyRoot, "third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><script>window.process={env:{NODE_ENV:"production"}}</script><script type="importmap">{"imports":%s}</script><main></main><script type="module">%s import {createElement} from "lwc";import Page from "c/%s";document.querySelector("main").appendChild(createElement("%s",{is:Page}));</script>`, importJSON, lwcembed.ManualDOMJS, page, entry.Tag)
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
		case r.URL.Path == "/lightning/runtime/shims/component-dom.mjs":
			// Compiled shadowRoot reads use the production component boundary.
			// Serve its dependency without changing the native DOM observations.
			file = filepath.Join(sourceRoot, "lwcruntime/src/shims/component-dom.mjs")
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
		t.Fatal(err)
	}
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(dependencyRoot, "lwcruntime/node_modules/playwright")
	}
	// Only IDs and frame counts reach the observer; native answers cannot alter
	// the actual DOM, error messages, console entries, or requested mutations.
	specs := make([]map[string]any, len(activeCases))
	for index, c := range activeCases {
		specs[index] = map[string]any{"id": c.ID, "frames": len(c.Frames)}
	}
	config, err := json.Marshal(map[string]any{"url": server.URL, "specs": specs, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", filepath.Join(repo, "internal/lwc/compile/testdata/l06_browser.mjs"))
	cmd.Stdin = bytes.NewReader(config)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("L06 local browser environment: %v: %s", err, l06OneLine(stderr.String()))
	}
	var observation struct {
		Values map[string]json.RawMessage `json:"values"`
		Errors map[string]string          `json:"errors"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &observation); err != nil {
		t.Fatal(err)
	}
	for id, message := range observation.Errors {
		rowErrors[id] = "L06: local DOM observation failed: " + message
	}
	for _, c := range cases {
		if _, ok := observation.Values[c.ID]; !ok && rowErrors[c.ID] == "" {
			t.Fatalf("L06 observer omitted both value and error for %s", c.ID)
		}
	}
	return observation.Values, rowErrors, nil
}

// Serialize observed JSON in the native TSV's sorted, compact, ensure_ascii
// format. Do not normalize identities, strings, errors, arrays or null values.
func l06DOMText(raw json.RawMessage) (string, error) {
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

func l06OneLine(text string) string {
	return strings.NewReplacer("\t", " ", "\r", " ", "\n", " ").Replace(text)
}
