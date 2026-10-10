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

// The API 62.0/67.0 anonymous captures supply identical exact answers.
// SC021 is excluded until a native capture settles the getPopulatedFieldsAsMap /
// marker-map question: whether mutating the returned map throws or leaves the
// record unchanged. Native @IsTest capture remains follow-up work.
func TestSObjectContainersOrgConformance(t *testing.T) {
	var data struct {
		APIVersions []string          `json:"apiVersions"`
		Files       map[string]string `json:"files"`
		Cases       []struct {
			ID       string `json:"id"`
			OracleID string `json:"oracleId"`
			Code     string `json:"code"`
			Expected string `json:"expected"`
		} `json:"cases"`
	}
	raw, err := os.ReadFile("testdata/sobject_containers.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 39 || len(data.APIVersions) != 2 || data.APIVersions[0] != "62.0" || data.APIVersions[1] != "67.0" {
		t.Fatalf("incomplete conformance export: %d rows, APIs %v", len(data.Cases), data.APIVersions)
	}
	quote := func(s string) string {
		return "'" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "'", "\\'") + "'"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			var files []string
			for name, source := range data.Files {
				path := filepath.Join(root, name)
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				files = append(files, path)
			}
			idx := typesys.Build(project.Project{Root: root, ApexFiles: files}, gladeschema.Schema{})
			if idx.HasErrors() {
				t.Fatal(idx.Diagnostics)
			}
			org := orgFromIndex(idx)
			runner := newConformanceRunner(t, idx, conformanceRunnerOptions{LinkProject: true, Org: &org})
			var class strings.Builder
			class.WriteString("@IsTest private class SObjectContainersConformance {\n")
			expectedMethods := map[string]bool{}
			for _, tc := range data.Cases {
				if tc.ID != tc.OracleID || tc.ID == "SC021" || tc.ID == "SC029" || expectedMethods[tc.ID] {
					t.Fatalf("invalid or duplicate oracle ID %s (%s)", tc.ID, tc.OracleID)
				}
				expectedMethods[tc.ID] = true
				comparison := "expectedText.equals(observedText)"
				if tc.Expected == "null" {
					comparison += " && observedNull"
				}
				body := "String expectedText=" + quote(tc.Expected) + "; String observedText; Boolean observedNull=false; try { Object r; " + tc.Code + " observedNull=(r==null); observedText=observedNull ? 'null' : String.valueOf(r); } catch(Exception e) { observedText='EXC|'+e.getTypeName()+'|'+e.getMessage(); } System.assert(" + comparison + ", " + quote(tc.ID) + " + ' expected <' + expectedText + '> actual <' + observedText + '>');"
				t.Run(tc.ID+"/anonymous", func(t *testing.T) {
					if analysis := sema.AnalyzeAnonymous(idx, body, api); analysis.HasErrors() {
						t.Fatalf("analysis: %v", analysis.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := runner.execute(program); err != nil {
						t.Fatal(err)
					}
				})
				fmt.Fprintf(&class, "@IsTest static void %s() { %s }\n", tc.ID, body)
			}
			class.WriteString("}\n")
			path := filepath.Join(root, "SObjectContainersConformance.cls")
			writeFile(t, path, class.String())
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			files = append(files, path)
			idx = typesys.Build(project.Project{Root: root, ApexFiles: files}, gladeschema.Schema{})
			if idx.HasErrors() {
				t.Fatal(idx.Diagnostics)
			}
			if analysis := sema.Analyze(idx); analysis.HasErrors() {
				t.Fatal(analysis.Diagnostics)
			}
			run := Run(idx, Options{NoDiskCache: true, Parallelism: 1})
			for _, suite := range run.Suites {
				for _, c := range suite.Cases {
					if !expectedMethods[c.MethodName] {
						t.Fatalf("unexpected/repeated @IsTest method %s", c.MethodName)
					}
					delete(expectedMethods, c.MethodName)
					t.Run(c.MethodName+"/isTest", func(t *testing.T) {
						if c.Status != testreport.StatusPass {
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
