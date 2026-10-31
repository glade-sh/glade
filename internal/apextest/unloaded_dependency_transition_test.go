package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

// The captured static call rejects without the library and passes with it:
// testdata/conformance/unloaded_dependency/api{62,67}/org.tsv:2,14.
// Re-load the same consumer after its declared package becomes available and
// unavailable again, to guard project discovery against stale source state.
func TestUnloadedDependencySourceTransition(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			workspace := t.TempDir()
			root := filepath.Join(workspace, "consumer")
			supplier := filepath.Join(workspace, "supplier")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"cedar-library"}]}]}`)
			name := "CedarLibrary" + api[:2]
			path := filepath.Join(root, "force-app/main/default/classes/CedarTransitionTest.cls")
			writeFile(t, path, `@IsTest private class CedarTransitionTest { @IsTest static void observe() { System.assertEquals(7, `+name+`.value()); } }`)
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			body, err := os.ReadFile(filepath.Join("testdata/conformance/unloaded_dependency/sources/CedarLibrary.cls"))
			if err != nil {
				t.Fatal(err)
			}
			for _, state := range []string{"absent", "available", "absent_again"} {
				if state == "available" {
					writeFile(t, filepath.Join(supplier, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"package":"cedar-library"}]}`)
					file := filepath.Join(supplier, "force-app/main/default/classes", name+".cls")
					writeFile(t, file, strings.ReplaceAll(string(body), "CedarLibrary", name))
					writeFile(t, file+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
				} else if state == "absent_again" {
					if err := os.RemoveAll(supplier); err != nil {
						t.Fatal(err)
					}
				}
				t.Run(state, func(t *testing.T) {
					run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
						t.Fatalf("unexpected cases: %+v", run.Summary())
					}
					got := run.Suites[0].Cases[0]
					if state == "available" {
						if got.Status != testreport.StatusPass || got.Problem != nil {
							t.Fatalf("available source: %+v", got)
						}
					} else if got.Status != testreport.StatusCompileError || got.Problem == nil || got.Problem.Type != "dependency_missing" {
						t.Fatalf("missing source: %+v", got)
					}
				})
			}
		})
	}
}
