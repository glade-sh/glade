package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// The oracle is exported owned Apex and Salesforce output. This test has no
// dependency on the probe tools, an org, credentials, or a network connection.
func TestIntegerLongOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected string
		FloorExpected      string `json:"floorExpected"`
		Compile            bool
		RemainingReason    string `json:"remainingReason"`
	}
	var data struct {
		APIVersion   string       `json:"apiVersion"`
		APIVersions  []string     `json:"apiVersions"`
		Cases        []familyCase `json:"cases"`
		Preservation []familyCase `json:"preservation"`
	}
	raw, err := os.ReadFile("../vm/testdata/conformance/integer_long.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 300 {
		t.Fatalf("oracle has %d cases, want 300", len(data.Cases))
	}
	if len(data.Preservation) != 15 {
		t.Fatalf("preservation oracle has %d cases, want 15", len(data.Preservation))
	}
	prepare := func(tc familyCase, api string) (helpers, declarations, body string) {
		expected := conformanceApexString(tc.Expected)
		helpers = `class P {
 public void out(String id, Object v) { System.assertEquals(` + expected + `, '' + String.valueOf(v), id); }
 public void err(String id, Exception e) { System.assertEquals(` + expected + `, 'EXC|' + e.getTypeName() + '|' + e.getMessage(), id); }
}
class Ov {
 public String k(Integer x) { return 'Integer'; } public String k(Long x) { return 'Long'; }
 public String h(Object x) { return 'Object'; } public String h(Long x) { return 'Long'; }
 public String f(Long x) { return 'Long'; } public String f(Decimal x) { return 'Decimal'; }
 public String m(Object x) { return 'Object'; } public String m(Decimal x) { return 'Decimal'; }
}
`
		if api == "62.0" {
			// Floor runtime cells use exact String.equals. Keep the accepted
			// ceiling adapter and its assertions unchanged.
			helpers = `class P {
 public void out(String id, Object v) { String expectedText=` + expected + `; String observedText=''+String.valueOf(v); System.assert(expectedText.equals(observedText), id+' expected <'+expectedText+'> actual <'+observedText+'>'); }
 public void err(String id, Exception e) { String expectedText=` + expected + `; String observedText='EXC|'+e.getTypeName()+'|'+e.getMessage(); System.assert(expectedText.equals(observedText), id+' expected <'+expectedText+'> actual <'+observedText+'>'); }
}
` + helpers[strings.Index(helpers, "class Ov"):]
		}

		code := tc.Code
		declarations = ""
		if strings.HasPrefix(code, "class ") {
			depth, end := 0, 0
			for i, c := range code {
				if c == '{' {
					depth++
				}
				if c == '}' {
					depth--
					if depth == 0 {
						end = i + 1
						break
					}
				}
			}
			declarations, code = code[:end], code[end:]
		}
		body = "P pq = new P(); Ov ov = new Ov();\n" + code
		if !tc.Compile {
			if !strings.Contains(code, ";") {
				code = "r = " + code + ";"
			}
			body = "P pq = new P(); Ov ov = new Ov();\ntry { Object r; " + code + " pq.out('" + tc.ID + "', r); } catch (Exception e) { pq.err('" + tc.ID + "', e); }"
		}
		return helpers, declarations, body
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle API versions=%v, want 62.0,67.0", data.APIVersions)
	}
	nativeCompilerRows := map[string]string{}
	for _, row := range data.Cases {
		if strings.HasPrefix(row.FloorExpected, "COMPILE_ERROR") {
			nativeCompilerRows[row.ID] = row.FloorExpected
		}
	}
	mismatches := newFloorCompilerMismatches(t, "Integer and Long", nativeCompilerRows, "anonymous", "@IsTest")
	checkFloorCompiler := func(t *testing.T, id, route, expected, observed string) {
		t.Helper()
		mismatches.Check(t, id, route, expected, observed)
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "Integer and Long", api)
			type scopedCase struct {
				familyCase
				key string
			}
			allCases := make([]scopedCase, 0, len(data.Cases)+len(data.Preservation))
			for i, rows := range [][]familyCase{data.Cases, data.Preservation} {
				if api == "62.0" && i == 1 {
					continue // These preservation rows have ceiling captures only.
				}
				scope := "main"
				if i == 1 {
					scope = "preservation"
				}
				for _, tc := range rows {
					if api == "62.0" {
						if tc.FloorExpected == "" {
							t.Fatalf("missing floor oracle row %s", tc.ID)
						}
						tc.Expected = tc.FloorExpected
					}
					// Oracle IDs repeat across the two tables. Scope only internal
					// method/result keys; keep oracle IDs and Go subtest names intact.
					allCases = append(allCases, scopedCase{familyCase: tc, key: scope + "_" + tc.ID})
				}
			}
			// As in CollectionConformance, share named-class preparation, but give
			// every row its own @IsTest method and runner-isolated VM/org state.
			// Rejections and row-specific class declarations keep the isolated path.
			groupedKeys := map[string]bool{}
			seenKeys := map[string]bool{}
			for _, tc := range allCases {
				if seenKeys[tc.key] {
					t.Fatalf("duplicate scoped oracle case %s", tc.key)
				}
				seenKeys[tc.key] = true
				if tc.RemainingReason == "" && !strings.HasPrefix(tc.Expected, "COMPILE_ERROR") && !strings.HasPrefix(tc.Code, "class ") {
					groupedKeys[tc.key] = true
				}
			}
			var namedResults map[string]testreport.Case
			var namedBatchError string
			namedBatchRan := false
			runNamedBatch := func(t *testing.T) {
				t.Helper()
				if namedBatchRan {
					return
				}
				namedBatchRan = true
				var source strings.Builder
				source.WriteString("@IsTest private class IntegerLongProbe {\n")
				for _, tc := range allCases {
					if !groupedKeys[tc.key] {
						continue
					}
					helpers, _, body := prepare(tc.familyCase, api)
					if namedResults == nil {
						namedResults = map[string]testreport.Case{}
						// Expected values belong only to the assertion helper; the
						// observed value still comes from each unchanged case body.
						helpers = strings.Replace(helpers, "class P {", "class P { public String expected;", 1)
						helpers = strings.ReplaceAll(helpers, "System.assertEquals("+conformanceApexString(tc.Expected)+",", "System.assertEquals(expected,")
						if api == "62.0" {
							helpers = strings.ReplaceAll(helpers, "String expectedText="+conformanceApexString(tc.Expected)+";", "String expectedText=expected;")
						}
						source.WriteString(helpers)
					}
					body = strings.Replace(body, "P pq = new P();", "P pq = new P(); pq.expected = "+conformanceApexString(tc.Expected)+";", 1)
					fmt.Fprintf(&source, "@IsTest static void observed%s() {\n%s\n}\n", tc.key, body)
				}
				source.WriteString("}\n")
				root := t.TempDir()
				path := filepath.Join(root, "IntegerLongProbe.cls")
				writeFile(t, path, source.String())
				writeFile(t, path+"-meta.xml", fmt.Sprintf("<ApexClass><apiVersion>%s</apiVersion></ApexClass>", api))
				index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, gladeschema.Schema{})
				if index.HasErrors() {
					namedBatchError = fmt.Sprintf("named parser: %v", index.Diagnostics)
					return
				}
				if analysis := sema.Analyze(index); analysis.HasErrors() {
					namedBatchError = fmt.Sprintf("named semantics: %v", analysis.Diagnostics)
					return
				}
				run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
				for _, suite := range run.Suites {
					for _, result := range suite.Cases {
						key := strings.TrimPrefix(result.MethodName, "observed")
						if result.ClassName != "IntegerLongProbe" || !groupedKeys[key] || namedResults[key].MethodName != "" {
							namedBatchError = fmt.Sprintf("unexpected or repeated named result: %#v", result)
							return
						}
						namedResults[key] = result
					}
				}
				if run.Summary().Total != len(groupedKeys) || len(namedResults) != len(groupedKeys) {
					namedBatchError = fmt.Sprintf("named results: %#v, got %d unique rows, want %d", run.Summary(), len(namedResults), len(groupedKeys))
				}
			}
			// Row helpers embed different expected literals. Link the unchanged platform
			// classes once, then register only the row's source classes on a private clone.
			base := typesys.Build(project.Project{Root: t.TempDir()}, gladeschema.Schema{})
			runner := newConformanceRunner(t, base, conformanceRunnerOptions{LinkProject: true})
			for _, tc := range allCases {
				if tc.RemainingReason != "" {
					t.Logf("Documented org mismatch %s: %s", tc.ID, tc.RemainingReason)
					continue
				}
				t.Run(tc.ID, func(t *testing.T) {
					helpers, declarations, body := prepare(tc.familyCase, api)
					wantCompileError := strings.HasPrefix(tc.Expected, "COMPILE_ERROR")
					kind := "exact"
					if api == "67.0" {
						kind = "legacy"
						if wantCompileError {
							kind = "category"
						}
					}
					t.Run("anonymous", func(t *testing.T) {
						routeKind := kind
						if api == "62.0" && wantCompileError {
							routeKind = mismatches.Kind(tc.ID, "anonymous")
						}
						counts.TrackKind(t, "anonymous", routeKind, 1)
						root := t.TempDir()
						paths := []string{}
						for name, source := range map[string]string{"P": helpers[:strings.Index(helpers, "class Ov")], "Ov": helpers[strings.Index(helpers, "class Ov"):], "G": declarations} {
							if source == "" {
								continue
							}
							path := filepath.Join(root, name+".cls")
							writeFile(t, path, source)
							writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
							paths = append(paths, path)
						}
						source := body
						index := typesys.Build(project.Project{Root: root, ApexFiles: paths}, gladeschema.Schema{})
						analysis := sema.AnalyzeAnonymous(index, source, api)
						program, compileErr := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if wantCompileError {
							if compileErr == nil && !analysis.HasErrors() {
								t.Fatal("org rejects compilation; anonymous path accepted it")
							}
							if api == "62.0" {
								checkFloorCompiler(t, tc.ID, "anonymous", tc.Expected, conformanceCompilerText(analysis.Diagnostics, compileErr))
							}
							return
						}
						if analysis.HasErrors() {
							t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
						}
						if compileErr != nil {
							t.Fatal(compileErr)
						}
						rowRunner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: runner, LinkProject: true})
						if _, err := rowRunner.execute(program); err != nil {
							t.Fatal(err)
						}
					})
					t.Run("isTest", func(t *testing.T) {
						routeKind := kind
						if api == "62.0" && wantCompileError {
							routeKind = mismatches.Kind(tc.ID, "@IsTest")
						}
						counts.TrackKind(t, "@IsTest", routeKind, 1)
						if groupedKeys[tc.key] {
							runNamedBatch(t)
							if namedBatchError != "" {
								t.Fatal(namedBatchError)
							}
							result := namedResults[tc.key]
							if result.Status != testreport.StatusPass {
								t.Fatalf("named result %s: status=%s problem=%v", tc.ID, result.Status, result.Problem)
							}
							return
						}
						root := t.TempDir()
						path := filepath.Join(root, "IntegerLongProbe.cls")
						source := "@IsTest private class IntegerLongProbe {\n" + helpers + declarations + "\n@IsTest static void observed() {\n" + body + "\n}\n}"
						writeFile(t, path, source)
						writeFile(t, path+"-meta.xml", fmt.Sprintf("<ApexClass><apiVersion>%s</apiVersion></ApexClass>", api))
						index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, gladeschema.Schema{})
						analysis := sema.Analyze(index)
						if wantCompileError {
							if !index.HasErrors() && !analysis.HasErrors() {
								run := Run(index, Options{NoDiskCache: true})
								if run.Summary().CompileErrors == 0 {
									t.Fatalf("org rejects compilation; named class path accepted it: %#v %s", run.Summary(), firstRunProblem(run))
								}
							}
							if api == "62.0" {
								checkFloorCompiler(t, tc.ID, "@IsTest", tc.Expected, conformanceCompilerText(append(index.Diagnostics, analysis.Diagnostics...), nil))
							}
							return
						}
						if index.HasErrors() {
							t.Fatalf("named parser: %v", index.Diagnostics)
						}
						if analysis.HasErrors() {
							t.Fatalf("named semantics: %v", analysis.Diagnostics)
						}
						run := Run(index, Options{NoDiskCache: true})
						if summary := run.Summary(); summary.Passed != 1 || summary.Failed != 0 || summary.CompileErrors != 0 {
							t.Fatalf("named result: %#v: %s", summary, firstRunProblem(run))
						}
					})
				})
			}
		})
	}
}

func conformanceApexString(s string) string {
	r := strings.NewReplacer("\\", "\\\\", "'", "\\'", "\r", "\\r", "\n", "\\n", "\t", "\\t")
	return "'" + r.Replace(s) + "'"
}
