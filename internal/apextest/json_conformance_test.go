package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

// Both routes consume the owned Salesforce observations at the source API floor
// and ceiling. CI reads only this export, including its custom Number field.
// The API 68 primary anonymous capture at native-capture
// (SHA-256 d2beeb3a92c4918011f673aa077ef8ffb6bf2c189b682608cb305d85a0680b07)
// matches the API 67 oracle on all 299 answered rows and is missing R303. The
// native UNCAUGHT recapture answers R303 with a volatile internal error ID; it
// stays carried. The review, ordering and diagnostic controls have no API 68
// capture, so API 68 leaves them unasserted. API 68 capture is anonymous-only,
// so @IsTest checks at 68 count as legacy local regressions.
func TestJSONOrgConformance(t *testing.T) {
	type row struct {
		ID, Code, Expected, FloorExpected, Carry string
		Compile, NativeNull                      bool
	}
	var data struct {
		APIVersions     []string `json:"apiVersions"`
		API68Uncaptured []string `json:"api68Uncaptured"`
		API68Recaptured struct {
			Source   string
			Expected map[string]string
		} `json:"api68Recaptured"`
		Declarations       map[string]string
		Cases              []row
		ReviewControls     []row
		OrderingControls   []row
		DiagnosticControls []row
	}
	raw, err := os.ReadFile("testdata/conformance/json/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 300 || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" {
		t.Fatalf("incomplete JSON oracle: %d rows, APIs %v", len(data.Cases), data.APIVersions)
	}
	if len(data.ReviewControls) != 26 {
		t.Fatalf("incomplete JSON review controls: %d rows", len(data.ReviewControls))
	}
	if len(data.OrderingControls) != 34 {
		t.Fatalf("incomplete JSON ordering controls: %d rows", len(data.OrderingControls))
	}
	if len(data.DiagnosticControls) != 4 {
		t.Fatalf("incomplete JSON diagnostic controls: %d rows", len(data.DiagnosticControls))
	}
	if len(data.API68Uncaptured) != 0 || len(data.API68Recaptured.Expected) != 1 || data.API68Recaptured.Expected["R303"] == "" || data.API68Recaptured.Source == "" {
		t.Fatalf("API 68 uncaptured=%v recaptured=%v, want R303 recaptured only", data.API68Uncaptured, data.API68Recaptured.Expected)
	}
	// Rows without a native API 68 answer are not asserted at API 68.
	uncaptured68 := map[string]string{}
	for _, id := range data.API68Uncaptured {
		uncaptured68[id] = "no API 68 answer in the primary capture"
	}
	for _, controls := range [][]row{data.ReviewControls, data.OrderingControls, data.DiagnosticControls} {
		for _, tc := range controls {
			uncaptured68[tc.ID] = "control has no API 68 capture"
		}
	}
	data.Cases = append(data.Cases, data.ReviewControls...)
	data.Cases = append(data.Cases, data.OrderingControls...)
	data.Cases = append(data.Cases, data.DiagnosticControls...)
	seen := map[string]bool{}
	carried := 0
	for _, tc := range data.Cases {
		if tc.ID == "" || seen[tc.ID] {
			t.Fatalf("empty or duplicate row ID %q", tc.ID)
		}
		seen[tc.ID] = true
		if tc.Carry != "" {
			carried++
			t.Logf("%s carried: %s; native=%s", tc.ID, tc.Carry, tc.Expected)
		}
	}
	if carried != 11 {
		t.Fatalf("JSON carries: %d, want 11", carried)
	}
	bodyFor := func(tc row, expected string) string {
		code := tc.Code
		if tc.Compile {
			// Retain the focal source; replace only its observation emitter.
			prefix := "pq.out('" + tc.ID + "',"
			at := strings.LastIndex(code, prefix)
			if at < 0 || !strings.HasSuffix(code, ");") {
				t.Fatalf("invalid compile-row emitter: %s", tc.ID)
			}
			code = code[:at] + "r=" + code[at+len(prefix):len(code)-2] + ";"
		} else if !strings.Contains(code, ";") {
			code = "r=" + code + ";"
		}
		if strings.HasPrefix(expected, "COMPILE_ERROR") {
			return "Object r; " + code
		}
		declarations := "String expectedText=" + conformanceApexString(expected) + "; String observedText;"
		observation := " observedText=''+String.valueOf(r);"
		comparison := "expectedText.equals(observedText)"
		if tc.NativeNull {
			declarations += " Boolean observedNull=false;"
			observation += " observedNull=(r==null);"
			comparison = "observedNull"
		}
		return declarations + " try { Object r; " + code + observation + " } catch(Exception e) { observedText='EXC|'+e.getTypeName()+'|'+e.getMessage(); } System.assert(" + comparison + ", " + conformanceApexString(tc.ID) + " + ' expected <' + expectedText + '> actual <' + observedText + '>');"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCounts()
			defer counts.Log(t, "JSON", api, "")
			if api == "68.0" {
				t.Log("A18 API 68.0: native capture is anonymous-only; @IsTest checks keep anonymous-backed local assertions and count as legacy, not native named-route evidence")
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			fixtureRoot := "testdata/conformance/json/schema"
			if err := filepath.WalkDir(fixtureRoot, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				rel, err := filepath.Rel(fixtureRoot, path)
				if err != nil {
					return err
				}
				contents, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), string(contents))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			names := make([]string, 0, len(data.Declarations))
			for name := range data.Declarations {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, data.Declarations[name])
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			}
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatal(analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			namedResults := map[string]testreport.Case{}
			namedReady := false
			prepareNamed := func(t *testing.T) {
				t.Helper()
				if namedReady {
					return
				}
				// Compile and run the accepted named class once per API. Each method
				// still receives its own runner state; row subtests inspect its result.
				var source strings.Builder
				source.WriteString("@IsTest private class JSONConformance {\n")
				namedMethods := map[string]bool{}
				for _, tc := range data.Cases {
					expected := tc.Expected
					if api == "62.0" {
						expected = tc.FloorExpected
					}
					if tc.Carry != "" || strings.HasPrefix(expected, "COMPILE_ERROR") {
						continue
					}
					if _, ok := uncaptured68[tc.ID]; ok && api == "68.0" {
						continue
					}
					namedMethods["observed"+tc.ID] = true
					fmt.Fprintf(&source, "@IsTest static void observed%s() {\n%s\n}\n", tc.ID, bodyFor(tc, expected))
				}
				source.WriteString("}\n")
				path := filepath.Join(root, "force-app/main/default/classes/JSONConformance.cls")
				writeFile(t, path, source.String())
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				namedIndex := loadTestIndex(t, root)
				if namedIndex.HasErrors() {
					t.Fatal(namedIndex.Diagnostics)
				}
				if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
					t.Fatal(analysis.Diagnostics)
				}
				run := Run(namedIndex, Options{NoDiskCache: true, Parallelism: 1, SelectedClasses: []string{"JSONConformance"}})
				for _, suite := range run.Suites {
					for _, result := range suite.Cases {
						if result.ClassName != "JSONConformance" || !namedMethods[result.MethodName] || namedResults[result.MethodName].MethodName != "" {
							t.Fatalf("unexpected/repeated named row: %#v", result)
						}
						namedResults[result.MethodName] = result
					}
				}
				if run.Summary().Total != len(namedMethods) || len(namedResults) != len(namedMethods) {
					t.Fatalf("named results: %#v, unique %d expected %d: %s", run.Summary(), len(namedResults), len(namedMethods), firstRunProblem(run))
				}
				namedReady = true
			}
			for _, tc := range data.Cases {
				expected := tc.Expected
				if api == "62.0" {
					expected = tc.FloorExpected
				}
				if reason, ok := uncaptured68[tc.ID]; ok && api == "68.0" {
					t.Logf("%s excluded at API 68: %s", tc.ID, reason)
					continue
				}
				if recaptured, ok := data.API68Recaptured.Expected[tc.ID]; ok && api == "68.0" {
					if tc.Carry == "" {
						t.Fatalf("%s API 68 recapture must stay carried", tc.ID)
					}
					expected = recaptured
				}
				if tc.Carry != "" {
					for _, route := range []string{"anonymous", "isTest"} {
						t.Run(tc.ID+"/"+route, func(t *testing.T) {
							t.Logf("carried: %s; native=%s", tc.Carry, expected)
						})
					}
					continue
				}
				body := bodyFor(tc, expected)
				rejected := strings.HasPrefix(expected, "COMPILE_ERROR")
				kind := "exact"
				if rejected {
					kind = "category"
				}
				counts.RunKind(t, "anonymous", tc.ID, tc.ID+"/anonymous", kind, func(t *testing.T) {
					analysis := sema.AnalyzeAnonymous(index, body, api)
					program, compileErr := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if rejected {
						if !analysis.HasErrors() && compileErr == nil {
							t.Fatal("Salesforce rejects compilation; anonymous source accepted")
						}
						return
					}
					if analysis.HasErrors() {
						t.Fatal(analysis.Diagnostics)
					}
					if compileErr != nil {
						t.Fatal(compileErr)
					}
					if _, err := runner.execute(program); err != nil {
						t.Fatal(err)
					}
				})
				namedKind := kind
				if api == "68.0" {
					// API 68 has no native named-route capture.
					namedKind = "legacy"
				}
				counts.RunKind(t, "isTest", tc.ID, tc.ID+"/isTest", namedKind, func(t *testing.T) {
					methodName := "observed" + tc.ID
					if rejected {
						// Rejected source must not change the accepted matrix generation.
						path := filepath.Join(root, "force-app/main/default/classes/JSONRejected.cls")
						writeFile(t, path, "@IsTest private class JSONRejected { @IsTest static void "+methodName+"() {"+body+"} }")
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						// Keep rejected source available through analysis and compilation.
						t.Cleanup(func() {
							for _, file := range []string{path, path + "-meta.xml"} {
								if err := os.Remove(file); err != nil {
									t.Error(err)
								}
							}
						})
						idx := loadTestIndex(t, root)
						analysis := sema.Analyze(idx)
						if !idx.HasErrors() && !analysis.HasErrors() {
							run := Run(idx, Options{NoDiskCache: true, Parallelism: 1, SelectedClasses: []string{"JSONRejected"}, SelectedMethod: methodName})
							if run.Summary().CompileErrors == 0 {
								t.Fatalf("Salesforce rejects compilation; @IsTest source accepted: %#v %s", run.Summary(), firstRunProblem(run))
							}
						}
						return
					}
					prepareNamed(t)
					result, ok := namedResults[methodName]
					if !ok {
						t.Fatalf("missing named row %s", methodName)
					}
					if result.Status != testreport.StatusPass {
						t.Fatalf("status=%s problem=%v", result.Status, result.Problem)
					}
				})
			}
		})
	}
}
