package apextest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

type locationCase struct {
	ID            string `json:"id"`
	Code          string `json:"code"`
	Compile       bool   `json:"compile"`
	Expected      string `json:"expected"`
	FloorExpected string `json:"floorExpected"`
	ExpectedNull  bool   `json:"expectedNull,omitempty"`
}

func locationQuote(text string) string {
	return "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'", "\n", "\\n", "\r", "\\r", "\t", "\\t").Replace(text) + "'"
}

func locationCode(row locationCase) string {
	code := row.Code
	if row.Compile {
		code = strings.Split(code, "pq.out(")[0]
		if !strings.HasPrefix(row.Expected, "COMPILE_ERROR") {
			code += "r = 'compiled';"
		}
	}
	if !strings.Contains(code, ";") {
		code = "r = " + code + ";"
	}
	return "Object r; " + code
}

func locationAssertion(row locationCase, expected string) string {
	if row.ExpectedNull {
		// Assert the raw null value; the text 'null' must still fail.
		return "Object observed; try { " + locationCode(row) + " observed = r; } catch(Exception e) { observed = 'EXC|' + e.getTypeName() + '|' + e.getMessage(); } System.assert(observed == null, " + locationQuote(row.ID+": exact null") + ");\n"
	}
	quoted := locationQuote(expected)
	return "String observed; try { " + locationCode(row) + " observed = String.valueOf(r); } catch(Exception e) { observed = 'EXC|' + e.getTypeName() + '|' + e.getMessage(); } String expectedText = " + quoted + "; System.assert(expectedText.equals(observed), " + locationQuote(row.ID) + " + ' expected <' + expectedText + '> actual <' + observed + '>');\n"
}

// API62/API67 answers are owned Salesforce observations, including exact Double
// text and exceptions that escape catch(Exception). Every row runs through both
// anonymous execution and a named @IsTest method without live calls.
func TestLocationOrgConformance(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/conformance/location/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []locationCase
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 182 {
		t.Fatalf("rows=%d, want=182", len(rows))
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "Location", api)
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			anonymousRunner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org})
			var source, uncaughtSource strings.Builder
			source.WriteString("@IsTest private class LocationOracleTest {\n")
			count, methods, rejections := 0, 0, 0
			uncaught := map[string]string{}
			seen := map[string]bool{}
			for _, row := range rows {
				if seen[row.ID] {
					t.Fatalf("duplicate row %s", row.ID)
				}
				seen[row.ID] = true
				expected := row.Expected
				if api == "62.0" {
					expected = row.FloorExpected
				}
				if strings.HasPrefix(expected, "COMPILE_ERROR") {
					rejections++
					passed := t.Run(row.ID+"/compile", func(t *testing.T) {
						code := locationCode(row)
						if result := sema.AnalyzeAnonymous(index, code, api); !result.HasErrors() {
							t.Fatalf("native rejected anonymous source: %s", code)
						}
						invalidRoot := t.TempDir()
						writeFile(t, filepath.Join(invalidRoot, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
						path := filepath.Join(invalidRoot, "force-app/main/default/classes/RejectedLocation.cls")
						writeFile(t, path, "@IsTest private class RejectedLocation { @IsTest static void rejected() {"+code+"} }")
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						idx := loadTestIndex(t, invalidRoot)
						if !idx.HasErrors() && !sema.Analyze(idx).HasErrors() {
							t.Fatalf("native rejected named source: %s", code)
						}
					})
					counts.record("anonymous", "category", 1, passed)
					counts.record("@IsTest", "category", 1, passed)
					continue
				}
				body := locationAssertion(row, expected)
				counts.run(t, row.ID+"/anonymous", "anonymous", "exact", func(t *testing.T) {
					if result := sema.AnalyzeAnonymous(index, body, api); result.HasErrors() {
						t.Fatalf("analysis: %v", result.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = anonymousRunner.execute(program)
					if strings.HasPrefix(expected, "UNCAUGHT|") {
						var runtimeErr *vm.RuntimeError
						if !errors.As(err, &runtimeErr) || "UNCAUGHT|"+runtimeErr.Error() != expected {
							t.Fatalf("uncaught=%v, want=%q", err, expected)
						}
					} else if err != nil {
						t.Fatal(err)
					}
				})
				if strings.HasPrefix(expected, "UNCAUGHT|") {
					name := "uncaught_" + row.ID
					uncaught[name] = expected
					fmt.Fprintf(&uncaughtSource, "@IsTest static void %s() {\n%s}\n", name, body)
					continue
				}
				if count%25 == 0 {
					if count > 0 {
						source.WriteString("}\n")
					}
					fmt.Fprintf(&source, "@IsTest static void batch%d() {\n", methods)
					methods++
				}
				count++
				source.WriteString("{\n" + body + "}\n")
			}
			source.WriteString("}\n" + uncaughtSource.String() + "}\n")
			if count != 162 || rejections != 6 || len(uncaught) != 14 {
				t.Fatalf("successful=%d rejected=%d uncaught=%d, want=162/6/14", count, rejections, len(uncaught))
			}
			path := filepath.Join(root, "force-app/main/default/classes/LocationOracleTest.cls")
			writeFile(t, path, source.String())
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			passed := t.Run("isTest", func(t *testing.T) {
				run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
				if got := run.Summary(); got.Total != methods+len(uncaught) || got.Passed != methods || got.Failed != len(uncaught) {
					t.Fatalf("summary=%#v, want=%d passing batches and %d native uncaught errors; first problem: %s", got, methods, len(uncaught), firstRunProblem(run))
				}
				checked := map[string]bool{}
				for _, suite := range run.Suites {
					for _, result := range suite.Cases {
						expected, ok := uncaught[result.MethodName]
						if !ok {
							if result.Status != testreport.StatusPass {
								t.Errorf("%s: unexpected %s: %#v", result.MethodName, result.Status, result.Problem)
							}
							continue
						}
						if checked[result.MethodName] {
							t.Errorf("duplicate method %s", result.MethodName)
						}
						checked[result.MethodName] = true
						if result.Status != testreport.StatusFail || result.Problem == nil || "UNCAUGHT|"+result.Problem.Type+": "+result.Problem.Message != expected {
							t.Errorf("%s: status=%s problem=%#v, want=%q", result.MethodName, result.Status, result.Problem, expected)
						}
					}
				}
				if len(checked) != len(uncaught) {
					t.Fatalf("checked uncaught methods=%d, want=%d", len(checked), len(uncaught))
				}
			})
			counts.record("@IsTest", "exact", count+len(uncaught), passed)
		})
	}
}
