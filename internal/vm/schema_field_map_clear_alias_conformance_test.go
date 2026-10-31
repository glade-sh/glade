package vm_test

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestSchemaFieldMapClearAliasConformance(t *testing.T) {
	const dir = "../../testdata/conformance/schema_field_map_clear_alias"
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	observed := map[string]string{}
	for _, row := range rows[1:] {
		observed[row[0]+"/"+row[1]+"/"+row[2]] = row[3]
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			for _, route := range []string{"anonymous", "isTest"} {
				for _, id := range []string{"A01", "A02", "A03", "A04", "A05", "A06", "A07", "A08"} {
					if observed[api+"/"+route+"/"+id] != "true" {
						t.Fatalf("missing native row %s %s %s", api, route, id)
					}
				}
			}
			root := t.TempDir()
			var files []string
			for _, name := range []string{"AliasProbe.cls", "AliasProbeTest.cls"} {
				data, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, name)
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path+"-meta.xml", []byte("<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>"), 0600); err != nil {
					t.Fatal(err)
				}
				files = append(files, path)
			}
			index := typesys.Build(project.Project{Root: root, ApexFiles: files, SourceAPIVersion: api}, schema.Schema{})
			t.Run("anonymous", func(t *testing.T) {
				runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
				if err != nil {
					t.Fatal(err)
				}
				machine := vm.New(nil)
				if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
					t.Fatal(err)
				}
				org := apextest.OrgFromIndex(index)
				org.APIVersion = api
				machine.SetOrg(&org)
				program, err := vm.CompileAnonymousWithOptions("AliasProbe.run();", vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
			t.Run("isTest", func(t *testing.T) {
				run := apextest.Run(index, apextest.Options{NoDiskCache: true, Parallelism: 1})
				count := 0
				for _, suite := range run.Suites {
					for _, result := range suite.Cases {
						count++
						if result.Status != testreport.StatusPass {
							t.Errorf("%s: %v", result.MethodName, result.Problem)
						}
					}
				}
				if count != 1 {
					t.Fatalf("got %d test cases, want 1", count)
				}
			})
		})
	}
}
