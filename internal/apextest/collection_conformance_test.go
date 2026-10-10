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

// The data is a credential-free export of owned API62 and API67 scratch-org probes.
// Both executeAnonymous and the real @IsTest runner consume the same rows.
func TestCollectionConformanceAPI67(t *testing.T) {
	var data struct {
		APIVersion  string   `json:"apiVersion"`
		APIVersions []string `json:"apiVersions"`
		Cases       []struct {
			ID            string `json:"id"`
			Code          string `json:"code"`
			Expected      string `json:"expected"`
			FloorExpected string `json:"floorExpected"`
		} `json:"cases"`
	}
	raw, err := os.ReadFile("../vm/testdata/conformance/list_set_map.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
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
	mismatches := newFloorCompilerMismatches(t, "collections", nativeCompilerRows, "anonymous", "@IsTest")
	checkFloorCompiler := func(t *testing.T, id, route, expected, observed string) {
		t.Helper()
		mismatches.Check(t, id, route, expected, observed)
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "collections", api)
			root := t.TempDir()
			classPath := filepath.Join(root, "CollectionConformance.cls")
			var class strings.Builder
			class.WriteString("@IsTest private class CollectionConformance {\n")
			expectedErrors := map[string]string{}
			expectedMethods := map[string]bool{}
			quote := func(s string) string {
				return "'" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "'", "\\'") + "'"
			}
			for _, row := range data.Cases {
				if api == "62.0" {
					if row.FloorExpected == "" {
						t.Fatalf("missing floor oracle row %s", row.ID)
					}
					row.Expected = row.FloorExpected
				}
				code := row.Code
				if !strings.Contains(code, ";") {
					code = "r=" + code + ";"
				}
				body := "Object r; " + code
				if strings.HasPrefix(row.Expected, "COMPILE_ERROR") {
					t.Run(row.ID+"/compile", func(t *testing.T) {
						anonymousKind, namedKind := "category", "category"
						if api == "62.0" {
							anonymousKind = mismatches.Kind(row.ID, "anonymous")
							namedKind = mismatches.Kind(row.ID, "@IsTest")
						}
						counts.TrackKind(t, "anonymous", anonymousKind, 1)
						counts.TrackKind(t, "@IsTest", namedKind, 1)
						result := sema.AnalyzeAnonymous(typesys.Index{}, body, api)
						if !result.HasErrors() {
							t.Errorf("org rejected anonymous source: %s", body)
						}
						if api == "62.0" {
							checkFloorCompiler(t, row.ID, "anonymous", row.Expected, conformanceCompilerText(result.Diagnostics, nil))
						}
						path := filepath.Join(t.TempDir(), "InvalidCollection.cls")
						writeFile(t, path, "@IsTest private class InvalidCollection { @IsTest static void run() {"+body+"} }")
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						idx := typesys.Build(project.Project{Root: filepath.Dir(path), ApexFiles: []string{path}}, gladeschema.Schema{})
						if !idx.HasErrors() && !sema.Analyze(idx).HasErrors() {
							t.Errorf("org rejected named source: %s", body)
						}
						if api == "62.0" {
							analysis := sema.Analyze(idx)
							checkFloorCompiler(t, row.ID, "@IsTest", row.Expected, conformanceCompilerText(append(idx.Diagnostics, analysis.Diagnostics...), nil))
						}
					})
					continue
				}
				if strings.HasPrefix(row.Expected, "UNCAUGHT|") {
					expectedErrors[row.ID] = strings.TrimPrefix(row.Expected, "UNCAUGHT|")
				} else {
					body = "String observed; try {" + body + " observed=''+String.valueOf(r); } catch(Exception e) { observed='EXC|'+e.getTypeName()+'|'+e.getMessage(); } System.assertEquals(" + quote(row.Expected) + ",observed);"
					if api == "62.0" {
						body = strings.TrimSuffix(body, "System.assertEquals("+quote(row.Expected)+",observed);") + "String expectedText=" + quote(row.Expected) + "; System.assert(expectedText.equals(observed), " + quote(row.ID) + "+' expected <'+expectedText+'> actual <'+observed+'>');"
					}
				}
				kind := "exact"
				if api == "67.0" && !strings.HasPrefix(row.Expected, "UNCAUGHT|") {
					kind = "legacy"
				}
				t.Run(row.ID+"/anonymous", func(t *testing.T) {
					counts.TrackKind(t, "anonymous", kind, 1)
					if analysis := sema.AnalyzeAnonymous(typesys.Index{}, body, api); analysis.HasErrors() {
						t.Fatalf("analysis: %v", analysis.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = vm.New(nil).Execute(program)
					if want, ok := expectedErrors[row.ID]; ok {
						if err == nil || err.Error() != want {
							t.Errorf("error=%v, want %s", err, want)
						}
					} else if err != nil {
						t.Fatal(err)
					}
				})
				fmt.Fprintf(&class, "@IsTest static void %s() { %s }\n", row.ID, body)
				expectedMethods[row.ID] = true
			}
			class.WriteString("}\n")
			writeFile(t, classPath, class.String())
			writeFile(t, classPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			idx := typesys.Build(project.Project{Root: root, ApexFiles: []string{classPath}}, gladeschema.Schema{})
			if idx.HasErrors() {
				t.Fatal(idx.Diagnostics)
			}
			run := Run(idx, Options{NoDiskCache: true})
			for _, suite := range run.Suites {
				for _, c := range suite.Cases {
					if !expectedMethods[c.MethodName] {
						t.Fatalf("unexpected or repeated @IsTest method %s", c.MethodName)
					}
					delete(expectedMethods, c.MethodName)
					t.Run(c.MethodName+"/isTest", func(t *testing.T) {
						kind := "exact"
						if _, boundary := expectedErrors[c.MethodName]; api == "67.0" && !boundary {
							kind = "legacy"
						}
						counts.TrackKind(t, "@IsTest", kind, 1)
						if want, ok := expectedErrors[c.MethodName]; ok {
							if c.Problem == nil || c.Problem.Message != want {
								t.Errorf("problem=%v, want %s", c.Problem, want)
							}
						} else if c.Status != testreport.StatusPass {
							t.Errorf("status=%s problem=%v", c.Status, c.Problem)
						}
					})
				}
			}
			if len(expectedMethods) != 0 {
				t.Fatalf("@IsTest methods did not execute: %v", expectedMethods)
			}
		})
	}
}
