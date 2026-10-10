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

// Anonymous r2 and named r1 captures at APIs 62.0/67.0 agree on all 18 rows.
// Compare exact observed text on both local execution routes.
func TestPlatformCacheCopyConformance(t *testing.T) {
	var data struct {
		APIVersions []string          `json:"apiVersions"`
		Files       map[string]string `json:"files"`
		Metadata    map[string]string `json:"metadata"`
		Cases       []struct {
			ID       string `json:"id"`
			OracleID string `json:"oracleId"`
			Code     string `json:"code"`
			Expected string `json:"expected"`
		} `json:"cases"`
	}
	raw, err := os.ReadFile("testdata/platform_cache_copy.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 18 || len(data.APIVersions) != 2 || data.APIVersions[0] != "62.0" || data.APIVersions[1] != "67.0" {
		t.Fatalf("incomplete conformance export: %d rows, APIs %v", len(data.Cases), data.APIVersions)
	}
	for i, tc := range data.Cases {
		want := i + 1
		if i >= 9 {
			want += 91
		}
		if tc.ID != fmt.Sprintf("PC%03d", want) {
			t.Fatalf("unexpected row %s at %d", tc.ID, i)
		}
	}
	quote := func(s string) string {
		return "'" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "'", "\\'") + "'"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			for name, source := range data.Metadata {
				writeFile(t, filepath.Join(root, name), source)
			}
			var files []string
			for name, source := range data.Files {
				path := filepath.Join(root, name)
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				files = append(files, path)
			}
			buildIndex := func() typesys.Index {
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				p.ApexFiles, p.SourceAPIVersion = files, api
				return typesys.Build(p, gladeschema.Schema{})
			}
			idx := buildIndex()
			if idx.HasErrors() {
				t.Fatal(idx.Diagnostics)
			}
			org := orgFromIndex(idx)
			runner := newConformanceRunner(t, idx, conformanceRunnerOptions{LinkProject: true, Org: &org})
			var class strings.Builder
			class.WriteString("@IsTest private class PlatformCacheCopyConformance {\n")
			expectedMethods := map[string]bool{}
			expectedTexts := map[string]string{}
			for _, tc := range data.Cases {
				if tc.ID != tc.OracleID || expectedMethods[tc.ID] {
					t.Fatalf("invalid or duplicate oracle ID %s (%s)", tc.ID, tc.OracleID)
				}
				expectedMethods[tc.ID] = true
				expectedTexts[tc.ID] = tc.Expected
				comparison := "expectedText.equals(observedText)"
				if tc.Expected == "null" {
					comparison += " && observedNull"
				}
				body := "String expectedText=" + quote(tc.Expected) + "; String observedText; Boolean observedNull=false; try { Object r; " + tc.Code + " observedNull=(r==null); observedText=observedNull ? 'null' : String.valueOf(r); } catch(Exception e) { observedText='EXC|'+e.getTypeName()+'|'+e.getMessage(); } System.assert(" + comparison + ", " + quote(tc.ID) + " + ' expected <' + expectedText + '> actual <' + observedText + '>'); System.debug('PC_RESULT|'+" + quote(tc.ID) + "+'|'+observedText);"
				t.Run(tc.ID+"/anonymous", func(t *testing.T) {
					if analysis := sema.AnalyzeAnonymous(idx, body, api); analysis.HasErrors() {
						t.Fatalf("analysis: %v", analysis.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					result, err := runner.execute(program)
					if err != nil {
						t.Fatal(err)
					}
					for _, line := range result.Debug {
						t.Log(line)
					}
				})
				fmt.Fprintf(&class, "@IsTest static void %s() { %s }\n", tc.ID, body)
			}
			class.WriteString("}\n")
			path := filepath.Join(root, "PlatformCacheCopyConformance.cls")
			writeFile(t, path, class.String())
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			files = append(files, path)
			idx = buildIndex()
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
						} else {
							t.Logf("verbatim assertion matched: %s", expectedTexts[c.MethodName])
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
