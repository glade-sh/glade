package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// The owned API 62/67 oracle is exported verbatim; CI needs neither an org nor
// glade-tools. Domain-abort rows assert construction only, as captured natively.
func TestFormulaEvaluationOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected string
		Compile            bool
	}
	var data struct {
		APIVersions            []string `json:"apiVersions"`
		Declarations, Metadata map[string]string
		Cases                  []familyCase
		Controls               []familyCase
		BuildOnlyRows          []string
	}
	raw, err := os.ReadFile("testdata/conformance/formula_evaluation.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 295 || strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle rows=%d versions=%v", len(data.Cases), data.APIVersions)
	}
	if strings.Join(data.BuildOnlyRows, ",") != "R028,R031,R044,R097" {
		t.Fatalf("domain-abort observations: %v", data.BuildOnlyRows)
	}
	if len(data.Controls) != 12 {
		t.Fatalf("formula controls: %d", len(data.Controls))
	}
	rows := append(data.Cases, data.Controls...)
	codeFor := func(row familyCase) string {
		code := row.Code
		if row.Compile {
			marker := " pq.out('" + row.ID + "',r);"
			if !strings.HasSuffix(code, marker) {
				t.Fatalf("compile observation %s", row.ID)
			}
			code = strings.TrimSuffix(code, marker)
		} else {
			code = "Object r; " + code
		}
		return code
	}
	body := func(row familyCase) string {
		code := codeFor(row)
		prefix := "String expectedText=" + conformanceApexString(row.Expected) + "; String conformanceObservedText; Boolean observedNull=false;\n"
		observation := "conformanceObservedText=String.valueOf(r); observedNull=(r==null);"
		comparison := "expectedText.equals(conformanceObservedText)"
		if row.Expected == "null" {
			comparison = "observedNull"
		}
		return prefix + "try {" + code + observation + "} catch(Exception e) {conformanceObservedText='EXC|'+e.getTypeName()+'|'+e.getMessage();}\n" +
			"System.assert(" + comparison + ", " + conformanceApexString(row.ID) + "+' expected <'+expectedText+'> actual <'+conformanceObservedText+'>');"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "formula evaluation", api)
			root := t.TempDir()
			var declarationPaths []string
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for rel, source := range data.Metadata {
				writeFile(t, filepath.Join(root, rel), source)
			}
			for name, source := range data.Declarations {
				rel := "force-app/main/default/classes/" + name + ".cls"
				if _, ok := data.Metadata[rel+"-meta.xml"]; !ok {
					t.Fatalf("missing captured class metadata for %s", name)
				}
				writeFile(t, filepath.Join(root, rel), source)
				declarationPaths = append(declarationPaths, filepath.Join(root, rel))
			}
			baseIndex := loadTestIndex(t, root)
			if baseIndex.HasErrors() {
				t.Fatal(baseIndex.Diagnostics)
			}
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest private class FormulaEvaluationConformance {\n")
			seen := map[string]bool{}
			accepted := 0
			for _, row := range rows {
				if seen[row.ID] {
					t.Fatalf("repeated oracle row %s", row.ID)
				}
				seen[row.ID] = true
				if strings.HasPrefix(row.Expected, "COMPILE_ERROR") {
					continue
				}
				accepted++
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", row.ID, body(row))
			}
			namedSource.WriteString("}\n")
			path := filepath.Join(root, "force-app/main/default/classes/FormulaEvaluationConformance.cls")
			writeFile(t, path, namedSource.String())
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if result := sema.Analyze(index); result.HasErrors() {
				t.Fatal(result.Diagnostics)
			}
			org := orgFromIndex(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			// Project linking excludes void @IsTest entries. Compile and register
			// them once on the unexecuted base before cloning either row route.
			namedCases := Discover(index, Options{SelectedClasses: []string{"FormulaEvaluationConformance"}})
			if len(namedCases) != accepted {
				t.Fatalf("named discovery: %d expected %d", len(namedCases), accepted)
			}
			testMethods, methodErrors := compileTestMethods(namedCases)
			for _, namedCase := range namedCases {
				key := testCaseKey(namedCase)
				if err := methodErrors[key]; err != nil {
					t.Fatal(err)
				}
				if err := runner.base.RegisterMethod(testMethods[key]); err != nil {
					t.Fatal(err)
				}
			}
			for _, row := range rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range []string{"anonymous", "isTest"} {
						countRoute := route
						if route == "isTest" {
							countRoute = "@IsTest"
						}
						counts.run(t, route, countRoute, "exact", func(t *testing.T) {
							if strings.HasPrefix(row.Expected, "COMPILE_ERROR") {
								// The native compile probe puts the unchanged row on
								// line 6. Check its full diagnostic, including that line.
								code := codeFor(row)
								result := sema.AnalyzeAnonymous(baseIndex, "\n\n\n\n\n"+code, api)
								if route == "isTest" {
									rejectedPath := filepath.Join(root, "rejected", "FormulaEvaluationRejected.cls")
									writeFile(t, rejectedPath, "@IsTest private class FormulaEvaluationRejected {\n@IsTest static void observed(){\n\n\n\n"+code+"\n}\n}\n")
									writeFile(t, rejectedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
									rejectedProject := project.Project{Root: root, SourceAPIVersion: api, ApexFiles: append(append([]string{}, declarationPaths...), rejectedPath)}
									rejectedIndex := typesys.Build(rejectedProject, gladeschema.Schema{Objects: baseIndex.Objects})
									if rejectedIndex.HasErrors() {
										t.Fatal(rejectedIndex.Diagnostics)
									}
									result = sema.Analyze(rejectedIndex)
								}
								observedText := "ACCEPTED"
								for _, item := range result.Diagnostics {
									if item.Severity == diagnostic.Error && item.Range != nil {
										observedText = fmt.Sprintf("COMPILE_ERROR\tline %d: %s", item.Range.Start.Line, item.Message)
										break
									}
								}
								if observedText != row.Expected {
									t.Fatalf("%s expected <%s> actual <%s>; diagnostics=%v", row.ID, row.Expected, observedText, result.Diagnostics)
								}
								return
							}
							source := body(row)
							if route == "anonymous" {
								if result := sema.AnalyzeAnonymous(index, source, api); result.HasErrors() {
									t.Fatal(result.Diagnostics)
								}
							} else {
								source = "FormulaEvaluationConformance.observed" + row.ID + "();"
							}
							program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							if _, err := runner.execute(program, func(machine *vm.VM) {
								if route == "isTest" {
									machine.EnableTestContext()
								}
							}); err != nil {
								t.Fatal(err)
							}
						})
					}
				})
			}
		})
	}
}
