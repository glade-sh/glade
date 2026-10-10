package compile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
)

type l03Case struct {
	ID                string                           `json:"id"`
	Kind              string                           `json:"kind"`
	Template          string                           `json:"template"`
	JS                string                           `json:"js"`
	MetaFragment      string                           `json:"meta_fragment"`
	Frames            []map[string]any                 `json:"frames"`
	Expected          map[string]string                `json:"expected"`
	CompileExpected   map[string]string                `json:"compile_expected"`
	NativeDiagnostics map[string][]l03NativeDiagnostic `json:"native_diagnostics"`
	VersionGate       *l03VersionGate                  `json:"version_gate"`
	DiagnosticCarry   map[string]l03CarryReason        `json:"diagnostic_carry"`
}

type l03CarryReason struct {
	Owner  string `json:"owner"`
	Reason string `json:"reason"`
}

type l03NativeDiagnostic struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

type l03CompileObservation struct {
	Value       string   `json:"value"`
	Diagnostics []string `json:"diagnostics"`
}

type l03VersionGate struct {
	ID                     string                           `json:"id"`
	LastFloorBehavior      int                              `json:"last_floor_behavior"`
	FirstDifferentBehavior int                              `json:"first_different_behavior"`
	Observations           map[string]l03CompileObservation `json:"observations"`
}

type l03DiagnosticSignature struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// TestL03SalesforceConformance compares exact exported native row text at the
// floor, ceiling and captured 65/66 boundaries. Capture mode reports mismatches.
// DOM answers come from the compiled browser engine and the owned fixture's
// WeakMap of actual Node references, including all frames and removed nodes.
func TestL03SalesforceConformance(t *testing.T) {
	capture := os.Getenv("GLADE_L03_CAPTURE") != ""
	data, err := os.ReadFile("testdata/l03_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []l03Case
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 276 {
		t.Fatalf("L03 requires all 276 native cases, got %d", len(cases))
	}
	versions := []string{"59.0", "65.0", "66.0", "67.0"}
	seen, runtimeCases, boundaryCases := map[string]bool{}, 0, 0
	for _, c := range cases {
		if c.ID == "" || seen[c.ID] {
			t.Fatalf("empty or duplicate L03 case ID %q", c.ID)
		}
		seen[c.ID] = true
		if c.Kind == "runtime" {
			runtimeCases++
			if len(c.Frames) == 0 {
				t.Fatalf("missing input frames for %s", c.ID)
			}
		} else if c.Kind != "compile" {
			t.Fatalf("unknown L03 case kind %q for %s", c.Kind, c.ID)
		}
		caseVersions := []string{"59.0", "67.0"}
		if c.VersionGate != nil {
			boundaryCases++
			if c.Kind != "compile" || c.VersionGate.ID != c.ID || c.VersionGate.LastFloorBehavior != 65 || c.VersionGate.FirstDifferentBehavior != 66 {
				t.Fatalf("invalid captured version boundary for %s", c.ID)
			}
			caseVersions = versions
		}
		for _, api := range caseVersions {
			compileWant := c.CompileExpected[api]
			if compileWant != "COMPILE_OK" && compileWant != "COMPILE_ERROR" {
				t.Fatalf("missing native compile answer for %s at API %s", c.ID, api)
			}
			want, observed := c.Expected[api]
			if !observed || (c.Kind == "compile" && want != compileWant) || (c.Kind == "runtime" && !strings.HasPrefix(want, "DOM|")) {
				t.Fatalf("missing or inconsistent native row for %s at API %s", c.ID, api)
			}
			signatures := l03NativeSignatures(t, c, api)
			if (compileWant == "COMPILE_ERROR") != (len(signatures) != 0) {
				t.Fatalf("missing or inconsistent native diagnostic for %s at API %s", c.ID, api)
			}
			if c.VersionGate != nil {
				observation, ok := c.VersionGate.Observations[strings.TrimSuffix(api, ".0")]
				messages := make([]string, 0, len(signatures))
				for _, signature := range signatures {
					messages = append(messages, signature.Message)
				}
				sort.Strings(messages)
				if !ok || observation.Value != compileWant || l03JSON(t, messages) != l03JSON(t, observation.Diagnostics) {
					t.Fatalf("missing or inconsistent captured boundary observation for %s at API %s", c.ID, api)
				}
			}
		}
		changed := c.CompileExpected["59.0"] != c.CompileExpected["67.0"] || l03JSON(t, l03NativeSignatures(t, c, "59.0")) != l03JSON(t, l03NativeSignatures(t, c, "67.0"))
		if changed != (c.VersionGate != nil) {
			t.Fatalf("native version difference lacks its captured boundary for %s", c.ID)
		}
		for api, carry := range c.DiagnosticCarry {
			if c.VersionGate == nil || (api != "59.0" && api != "65.0") || c.CompileExpected[api] != "COMPILE_ERROR" || carry.Owner == "" || carry.Reason == "" {
				t.Fatalf("invalid carried diagnostic reason for %s at API %s", c.ID, api)
			}
		}
	}
	if runtimeCases != 72 {
		t.Fatalf("L03 requires all 72 native DOM cases, got %d", runtimeCases)
	}
	if boundaryCases != 42 {
		t.Fatalf("L03 requires all 42 captured version boundaries, got %d", boundaryCases)
	}
	// Missing tooling is an environment failure, never a native rejection match.
	roots, err := compileToolchainRoots()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(roots.DependencyRoot, "third_party", "lwc", "node_modules", "@lwc", "compiler", "package.json")); err != nil {
		t.Fatal(err)
	}
	var report strings.Builder
	report.WriteString("API\tID\tCHECK\tSTATUS\tACTUAL\tEXPECTED\tREASON\n")
	matches, total, compileMatches, compileTotal, domMatches, domTotal := 0, 0, 0, 0, 0, 0
	boundaryMatches, boundaryTotal, diagnosticMatches, diagnosticTotal, diagnosticCarried, causeMatches, causeTotal := 0, 0, 0, 0, 0, 0, 0
	causeCarried := 0
	record := func(t *testing.T, api, id, check, got, want, reason string) bool {
		t.Helper()
		matched := got == want // Exact Go string equality preserves case, length and JSON nulls.
		status := "MISMATCH"
		if matched {
			status, reason = "MATCH", ""
		}
		fmt.Fprintf(&report, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", api, id, check, status, l03ReportCell(got), l03ReportCell(want), l03ReportCell(reason))
		if !capture && !matched {
			t.Errorf("%s %s %s expected <%s> actual <%s>: %s", api, id, check, want, got, reason)
		}
		return matched
	}
	carried := func(t *testing.T, api, id, check, got, want, reason string) {
		t.Helper()
		fmt.Fprintf(&report, "%s\t%s\t%s\tCARRIED\t%s\t%s\t%s\n", api, id, check, l03ReportCell(got), l03ReportCell(want), l03ReportCell(reason))
		t.Logf("%s %s %s CARRIED: %s; expected <%s> actual <%s>", api, id, check, reason, want, got)
	}
	for _, api := range versions {
		boundary := api == "65.0" || api == "66.0"
		var dom map[string]string
		if !boundary {
			dom = l03BrowserDOM(t, cases, api, roots)
		}
		compiled := compileConformanceBatch(t, api, len(cases), func(root string, index int) {
			l03WriteCompileBundle(t, root, cases[index], api)
		})
		for index, c := range cases {
			if boundary && c.VersionGate == nil {
				continue // Intermediate APIs contain only captured compilation rows.
			}
			t.Run(api+"/"+c.ID, func(t *testing.T) {
				compileErr := compiled[index].Err
				diagnostics := l03BatchSignatures(t, compiled[index])
				got, reason := "COMPILE_OK", "L03: local compiler accepted the native-rejected source"
				if compileErr != nil {
					got, reason = "COMPILE_ERROR", "L03: "+compileErr.Error()
				}
				check := "compile"
				if c.Kind == "runtime" {
					check = "compile-source"
				} else if boundary {
					check = "compile-boundary"
				}
				compileTotal++
				matched := record(t, api, c.ID, check, got, c.CompileExpected[api], reason)
				if matched {
					compileMatches++
				}
				if compileErr != nil || c.CompileExpected[api] == "COMPILE_ERROR" {
					diagnosticTotal++
					want := l03NativeSignatures(t, c, api)
					gotText, wantText := l03JSON(t, diagnostics), l03JSON(t, want)
					const pluginPrefix = "LWC1535: Unexpected plugin compilation error: Plugin - lwc, Hook - transform, Cause - "
					if gotText != wantText && c.VersionGate != nil && len(want) == 1 && want[0].Code == 1535 && strings.HasPrefix(want[0].Message, pluginPrefix) {
						// Compare the complete captured cause separately from the wrapper;
						// known legacy-parser differences have exported reasons and owners.
						cause := strings.TrimPrefix(want[0].Message, pluginPrefix)
						causeWant := []l03DiagnosticSignature{l03Signature(t, cause)}
						causeWantText := l03JSON(t, causeWant)
						carry, hasCarry := c.DiagnosticCarry[api]
						carryCause := hasCarry && gotText != causeWantText
						wrapperReason := "owner L01: Metadata API wraps the template expression diagnostic in LWC1535; the local compiler exposes its cause directly"
						if carryCause {
							wrapperReason += "; " + carry.Reason
						}
						diagnosticCarried++
						carried(t, api, c.ID, "diagnostic", gotText, wantText, wrapperReason)
						causeTotal++
						if carryCause {
							causeCarried++
							carried(t, api, c.ID, "diagnostic-cause", gotText, causeWantText, "owner "+carry.Owner+": "+carry.Reason)
						} else if record(t, api, c.ID, "diagnostic-cause", gotText, causeWantText, "owner L03: local expression diagnostic code or message differs from the captured cause") {
							causeMatches++
						}
					} else if record(t, api, c.ID, "diagnostic", gotText, wantText, "owner L03: local compiler diagnostic code or message differs from the native signature") {
						diagnosticMatches++
					}
				}
				if boundary {
					boundaryTotal++
					if matched {
						boundaryMatches++
					}
					return
				}
				total++
				if c.Kind == "compile" {
					if matched {
						matches++
					}
					return
				}
				if compileErr == nil {
					got = dom[c.ID]
					reason = "L03: compiled browser DOM differs from the complete native row"
					if !strings.HasPrefix(got, "DOM|") {
						reason = "L03: " + got
					}
				}
				domTotal++
				if record(t, api, c.ID, "dom", got, c.Expected[api], reason) {
					matches++
					domMatches++
				}
			})
		}
	}
	// TOTAL follows org.tsv: 204 compiler/metadata plus 72 DOM rows per API.
	// DOM-source admissions are supplemental and cannot inflate that denominator.
	fmt.Fprintf(&report, "TOTAL\t%d/%d\nCOMPILE_TOTAL\t%d/%d\nDOM_TOTAL\t%d/%d\n", matches, total, compileMatches, compileTotal, domMatches, domTotal)
	fmt.Fprintf(&report, "BOUNDARY_TOTAL\t%d/%d\nDIAGNOSTIC_TOTAL\t%d/%d\nDIAGNOSTIC_CARRIED\t%d\nDIAGNOSTIC_CAUSE_TOTAL\t%d/%d\n", boundaryMatches, boundaryTotal, diagnosticMatches, diagnosticTotal, diagnosticCarried, causeMatches, causeTotal)
	fmt.Fprintf(&report, "DIAGNOSTIC_CAUSE_CARRIED\t%d\n", causeCarried)
	t.Logf("L03 matches %d/%d; compile checks %d/%d; DOM rows %d/%d", matches, total, compileMatches, compileTotal, domMatches, domTotal)
	t.Logf("L03 boundary checks %d/%d; diagnostic signatures %d/%d; carried wrappers %d; exact diagnostic causes %d/%d; carried causes %d", boundaryMatches, boundaryTotal, diagnosticMatches, diagnosticTotal, diagnosticCarried, causeMatches, causeTotal, causeCarried)
	if path := os.Getenv("GLADE_L03_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

// Observe error objects before Node's console inspection adds paths and stacks.
// This test-only preload preserves the compiler call, original output and exit.
const l03DiagnosticObserverJS = `const original = console.error;
console.error = function (...args) {
 for (const error of args) {
  if (error instanceof Error) {
   const errors = Array.isArray(error.errors) ? error.errors : [error];
   for (const diagnostic of errors) {
    process.stderr.write('GLADE_L03_DIAGNOSTIC|' + JSON.stringify({code:typeof diagnostic.code === 'number' ? diagnostic.code : 0, message:diagnostic.message}) + '\n');
   }
  }
 }
 return original.apply(console, args);
};
`

var (
	l03DiagnosticPosition = regexp.MustCompile(`\[Line: \d+, Col: \d+\]\s*`)
	l03DiagnosticAPIEcho  = regexp.MustCompile(`current component API version \(\d+\)`)
	l03DiagnosticCode     = regexp.MustCompile(`\bLWC(\d{4}):`)
)

func l03NormalizeDiagnostic(message string) string {
	// Match the native bisect signature: omit only location and echoed API.
	message = l03DiagnosticPosition.ReplaceAllString(message, "")
	return l03DiagnosticAPIEcho.ReplaceAllString(message, "current component API version (<source-api>)")
}

func l03Signature(t *testing.T, message string) l03DiagnosticSignature {
	t.Helper()
	code := l03DiagnosticCode.FindStringSubmatch(message)
	if len(code) != 2 {
		t.Fatalf("native diagnostic lacks an LWC code: %q", message)
	}
	value, err := strconv.Atoi(code[1])
	if err != nil {
		t.Fatal(err)
	}
	return l03DiagnosticSignature{Code: value, Message: l03NormalizeDiagnostic(message)}
}

func l03NativeSignatures(t *testing.T, c l03Case, api string) []l03DiagnosticSignature {
	t.Helper()
	diagnostics, observed := c.NativeDiagnostics[api]
	if !observed {
		t.Fatalf("missing native diagnostics for %s at API %s", c.ID, api)
	}
	signatures := make([]l03DiagnosticSignature, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		if diagnostic.ID != c.ID || diagnostic.Type != "Error" {
			t.Fatalf("unexpected native diagnostic for %s at API %s: %+v", c.ID, api, diagnostic)
		}
		signatures = append(signatures, l03Signature(t, diagnostic.Message))
	}
	sort.Slice(signatures, func(i, j int) bool { return signatures[i].Message < signatures[j].Message })
	return signatures
}

// The boundary is the owned native capture fixture: it records actual child
// observations and catches errors without predicting reconciliation behavior.
const l03BoundaryJS = `import { LightningElement } from 'lwc';
export default class FamilyIteration extends LightningElement {
 result = ''; _requested = 0; _steps = []; _errors = [];
 _publish() { this.result = JSON.stringify({id:__CASE__, requested:this._requested, steps:this._steps, errors:this._errors}); }
 handleObserve(event) { this._steps.push(event.detail); this._publish(); }
 errorCallback(error) {
  this._errors.push({step:this._requested, name:error.name, message:error.message});
  this._publish();
 }
 advance() {
  this._requested += 1;
  const child = this.template.querySelector('__CHILD__');
  if (!child) { this._errors.push({step:this._requested, childPresent:false}); this._publish(); return; }
  try { child.advance(); }
  catch (error) { this._errors.push({step:this._requested, name:error.name, message:error.message}); this._publish(); }
 }
}
`

func l03BrowserDOM(t *testing.T, cases []l03Case, api string, roots compileRoots) map[string]string {
	t.Helper()
	root := t.TempDir()
	specs := []map[string]any{}
	var children strings.Builder
	kebab := regexp.MustCompile(`([a-z0-9])([A-Z])`)
	tag := func(name string) string { return "c-" + strings.ToLower(kebab.ReplaceAllString(name, "$1-$2")) }
	for _, c := range cases {
		if c.Kind != "runtime" {
			continue
		}
		child := fmt.Sprintf("familyL03Dom%03dV%s", len(specs), api[:2])
		boundary := fmt.Sprintf("familyL03Boundary%03dV%s", len(specs), api[:2])
		l03WriteBundle(t, root, child, c.JS, c.Template, api)
		boundaryJS := strings.ReplaceAll(strings.ReplaceAll(l03BoundaryJS, "__CASE__", l03JSON(t, c.ID)), "__CHILD__", tag(child))
		template := `<template><section data-case="` + html.EscapeString(c.ID) + `"><` + tag(child) + ` onobserve={handleObserve}></` + tag(child) + `><button data-next onclick={advance}>Next frame</button><pre data-result>{result}</pre></section></template>`
		l03WriteBundle(t, root, boundary, boundaryJS, template, api)
		fmt.Fprintf(&children, "<%s></%s>\n", tag(boundary), tag(boundary))
		specs = append(specs, map[string]any{"id": c.ID, "frames": len(c.Frames)})
	}
	host := "familyL03Runtime" + api[:2]
	l03WriteBundle(t, root, host, `import { LightningElement } from 'lwc'; export default class FamilyIteration extends LightningElement {}`, "<template>"+children.String()+"</template>", api)
	writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := Compile(p, Options{OutDir: filepath.Join(root, "dist")})
	if err != nil {
		t.Fatalf("L03 API %s runtime fixture compilation: %v", api, err)
	}
	entry, ok := manifest.Modules["c:"+host]
	if !ok || len(manifest.Modules) != 2*len(specs)+1 {
		t.Fatalf("L03 runtime fixture modules incomplete at API %s: %d", api, len(manifest.Modules))
	}
	relative, err := filepath.Rel(manifest.OutDir, entry.File)
	if err != nil {
		t.Fatal(err)
	}
	imports := map[string]string{"lwc": "/engine.js", "@lwc/synthetic-shadow": "/shadow.js"}
	for _, module := range manifest.Modules {
		file, err := filepath.Rel(manifest.OutDir, module.File)
		if err != nil {
			t.Fatal(err)
		}
		imports[module.ModuleKey] = "/modules/" + filepath.ToSlash(file)
	}
	page := `<!doctype html><script>window.process={env:{NODE_ENV:"production"}}</script><script type="importmap">` + l03JSON(t, map[string]any{"imports": imports}) + `</script><main></main><script type="module">import "@lwc/synthetic-shadow";import {createElement} from "lwc";import Ctor from ` + l03JSON(t, "/modules/"+filepath.ToSlash(relative)) + `;document.querySelector("main").appendChild(createElement(` + l03JSON(t, entry.Tag) + `,{is:Ctor}));</script>`
	mux := http.NewServeMux()
	for route, file := range map[string]string{
		"/engine.js": filepath.Join(roots.DependencyRoot, "third_party", "lwc", "node_modules", "@lwc", "engine-dom", "dist", "index.js"),
		"/shadow.js": filepath.Join(roots.DependencyRoot, "third_party", "lwc", "node_modules", "@lwc", "synthetic-shadow", "dist", "index.js"),
	} {
		if _, err := os.Stat(file); err != nil {
			t.Fatal(err)
		}
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/javascript")
			http.ServeFile(w, r, file)
		})
	}
	mux.Handle("/modules/", http.StripPrefix("/modules/", http.FileServer(http.Dir(manifest.OutDir))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, page)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	playwright := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwright == "" {
		playwright = filepath.Join(roots.DependencyRoot, "lwcruntime", "node_modules", "playwright")
	}
	if _, err := os.Stat(filepath.Join(playwright, "index.mjs")); err != nil {
		t.Fatal(err)
	}
	observer, err := filepath.Abs("testdata/l03_dom_observer.mjs")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", observer)
	cmd.Stdin = strings.NewReader(l03JSON(t, map[string]any{"url": server.URL, "specs": specs, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")}))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("L03 API %s browser observation: %v: %s", api, err, stderr.String())
	}
	var observed map[string]string
	if err := json.Unmarshal(output, &observed); err != nil {
		t.Fatalf("L03 browser returned invalid row JSON: %v", err)
	}
	if len(observed) != len(specs) {
		t.Fatalf("L03 API %s browser returned %d/%d rows", api, len(observed), len(specs))
	}
	for _, spec := range specs {
		if _, ok := observed[spec["id"].(string)]; !ok {
			t.Fatalf("L03 browser omitted %s at API %s", spec["id"], api)
		}
	}
	return observed
}

func l03WriteBundle(t *testing.T, root, name, js, template, api string) {
	t.Helper()
	bundle := filepath.Join(root, "force-app", "main", "default", "lwc", name)
	writeCompileFixtureFile(t, filepath.Join(bundle, name+".js"), js)
	writeCompileFixtureFile(t, filepath.Join(bundle, name+".html"), template)
	writeCompileFixtureFile(t, filepath.Join(bundle, name+".js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)
}

func l03JSON(t *testing.T, value any) string {
	t.Helper()
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(out.String(), "\n")
}

func l03ReportCell(value string) string {
	return strings.NewReplacer("\t", "\\t", "\r", "\\r", "\n", "\\n").Replace(value)
}

func l03WriteCompileBundle(t *testing.T, root string, c l03Case, api string) {
	t.Helper()
	bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "familyIteration")
	writeCompileFixtureFile(t, filepath.Join(bundle, "familyIteration.js"), c.JS)
	writeCompileFixtureFile(t, filepath.Join(bundle, "familyIteration.html"), c.Template)
	fragment := c.MetaFragment
	if fragment == "" {
		fragment = "<isExposed>false</isExposed>"
	}
	meta := `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion>` + fragment + `</LightningComponentBundle>`
	writeCompileFixtureFile(t, filepath.Join(bundle, "familyIteration.js-meta.xml"), meta)
}

func l03BatchSignatures(t *testing.T, result BundleResult) []l03DiagnosticSignature {
	t.Helper()
	diagnostics := []l03DiagnosticSignature{}
	if result.Err != nil {
		for _, diagnostic := range result.Diagnostics {
			if diagnostic.Code == 0 || diagnostic.Message == "" {
				t.Fatalf("local rejection is not an LWC diagnostic: %v", result.Err)
			}
			diagnostics = append(diagnostics, l03DiagnosticSignature{Code: diagnostic.Code, Message: l03NormalizeDiagnostic(diagnostic.Message)})
		}
		if len(diagnostics) == 0 {
			t.Fatalf("local rejection has no observed LWC diagnostic: %v", result.Err)
		}
	}
	sort.Slice(diagnostics, func(i, j int) bool { return diagnostics[i].Message < diagnostics[j].Message })
	return diagnostics
}
