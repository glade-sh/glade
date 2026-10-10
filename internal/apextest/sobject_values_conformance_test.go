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

// Both source APIs consume the same independently captured Salesforce answers.
// Anonymous and @IsTest execution use the runner's ordinary standard-object org.
func TestSObjectValuesOrgConformance(t *testing.T) {
	type row struct {
		ID          string `json:"id"`
		Code        string `json:"code"`
		Expected    string `json:"expected"`
		CompileCase bool   `json:"compileCase"`
	}
	var data struct {
		APIVersions []string `json:"apiVersions"`
		Cases       []row    `json:"cases"`
		Controls    []row    `json:"controls"`
	}
	raw, err := os.ReadFile("testdata/sobject_values.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 299 || len(data.APIVersions) != 2 {
		t.Fatalf("incomplete conformance export: %d rows, APIs %v", len(data.Cases), data.APIVersions)
	}
	quote := func(s string) string {
		return "'" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "'", "\\'") + "'"
	}
	bodyFor := func(tc row) string {
		code := tc.Code
		if tc.CompileCase {
			// Replace only the probe's observation emitter; keep its focal statements.
			prefix := "pq.out('" + tc.ID + "',"
			at := strings.LastIndex(code, prefix)
			if at < 0 || !strings.HasSuffix(code, ");") {
				t.Fatalf("invalid compile-row emitter: %s", tc.ID)
			}
			code = code[:at] + "r=" + code[at+len(prefix):len(code)-2] + ";"
		}
		body := "Object r; " + code
		if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
			return body
		}
		// Compare text exactly; native-null rows check the raw result instead.
		declarations := "String expectedText=" + quote(tc.Expected) + "; String observedText;"
		observation := " observedText=String.valueOf(r);"
		comparison := "expectedText.equals(observedText)"
		if tc.Expected == "null" {
			declarations += " Boolean observedNull=false;"
			observation += " observedNull=(r==null);"
			comparison = "observedNull"
		}
		return declarations + " try {" + body + observation + " } catch(Exception e) { observedText='EXC|'+e.getTypeName()+'|'+e.getMessage(); } System.assert(" + comparison + ", " + quote(tc.ID) + " + ' expected <' + expectedText + '> actual <' + observedText + '>');"
	}
	rows := make([]row, 0, len(data.Cases)+len(data.Controls))
	rows = append(rows, data.Cases...)
	rows = append(rows, data.Controls...)
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "SObject values", api)
			emptyRoot := t.TempDir()
			emptyIndex := typesys.Build(project.Project{Root: emptyRoot}, gladeschema.Schema{})
			org := orgFromIndex(emptyIndex)
			anonymousRunner := newConformanceRunner(t, emptyIndex, conformanceRunnerOptions{Org: &org})
			var class strings.Builder
			class.WriteString("@IsTest private class SObjectValuesConformance {\n")
			expectedMethods := map[string]bool{}
			seen := map[string]bool{}
			for _, tc := range rows {
				if seen[tc.ID] {
					t.Fatalf("duplicate oracle ID %s", tc.ID)
				}
				seen[tc.ID] = true
				body := bodyFor(tc)
				rejected := strings.HasPrefix(tc.Expected, "COMPILE_ERROR")
				kind := "exact"
				if rejected {
					kind = "category"
				}
				t.Run(tc.ID+"/anonymous", func(t *testing.T) {
					counts.TrackKind(t, "anonymous", kind, 1)
					analysis := sema.AnalyzeAnonymous(emptyIndex, body, api)
					program, compileErr := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if rejected {
						if !analysis.HasErrors() && compileErr == nil {
							t.Fatal("Salesforce rejects compilation; anonymous source accepted")
						}
						return
					}
					if analysis.HasErrors() {
						t.Fatalf("analysis: %v", analysis.Diagnostics)
					}
					if compileErr != nil {
						t.Fatal(compileErr)
					}
					if _, err := anonymousRunner.execute(program); err != nil {
						t.Fatal(err)
					}
				})
				if rejected {
					t.Run(tc.ID+"/isTest", func(t *testing.T) {
						counts.TrackKind(t, "@IsTest", "category", 1)
						root := t.TempDir()
						path := filepath.Join(root, "InvalidSObjectValues.cls")
						writeFile(t, path, "@IsTest private class InvalidSObjectValues { @IsTest static void run() {"+body+"} }")
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						idx := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, gladeschema.Schema{})
						if !idx.HasErrors() && !sema.Analyze(idx).HasErrors() {
							t.Fatal("Salesforce rejects compilation; @IsTest source accepted")
						}
					})
					continue
				}
				fmt.Fprintf(&class, "@IsTest static void %s() { %s }\n", tc.ID, body)
				expectedMethods[tc.ID] = true
			}
			class.WriteString("}\n")
			root := t.TempDir()
			path := filepath.Join(root, "SObjectValuesConformance.cls")
			writeFile(t, path, class.String())
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			idx := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, gladeschema.Schema{})
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
						counts.Track(t, "@IsTest", 1)
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
