package apextest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Salesforce deployment rows establish whether the project source is valid;
// the clean class is also exercised through both local execution routes.
func TestMergeConflictProjectRoutes(t *testing.T) {
	fixture := filepath.Join("..", "apexast", "testdata", "merge_conflict_markers")
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name, path string
			valid      bool
		}{{"conflict", "MarkerProbe.cls", false}, {"clean", "clean/MarkerProbe.cls", true}} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
				classes := filepath.Join(root, "force-app", "main", "default", "classes")
				source, err := os.ReadFile(filepath.Join(fixture, tc.path))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(classes, "MarkerProbe.cls"), string(source))
				meta := `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><status>Active</status></ApexClass>`
				writeFile(t, filepath.Join(classes, "MarkerProbe.cls-meta.xml"), meta)
				writeFile(t, filepath.Join(classes, "MarkerRouteTest.cls"), `@IsTest private class MarkerRouteTest { @IsTest static void clean() { System.assertEquals(2, MarkerProbe.run()); } }`)
				writeFile(t, filepath.Join(classes, "MarkerRouteTest.cls-meta.xml"), meta)
				index := loadTestIndex(t, root)
				if !tc.valid {
					if !index.HasErrors() {
						t.Fatal("conflict input accepted into project")
					}
					return
				}
				if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("clean project: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				t.Run("anonymous", func(t *testing.T) {
					p, err := vm.CompileAnonymousWithOptions(`System.assertEquals(2, MarkerProbe.run());`, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					_, err = newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true}).execute(p)
					if err != nil {
						t.Fatal(err)
					}
				})
				t.Run("istest", func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"MarkerRouteTest"}, NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil || run.Suites[0].Cases[0].Status != "pass" {
						t.Fatalf("test route: %+v", run.Summary())
					}
				})
			})
		}
	}
}
