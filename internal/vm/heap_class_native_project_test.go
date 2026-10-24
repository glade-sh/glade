package vm_test

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestHeapOrdinaryClassProjectNativeAnonymousAndIsTestRoutes(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	const fixture = "testdata/conformance/blob_class_heap"
	raw, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("want four native route rows, got %d", len(rows)-1)
	}
	for _, row := range rows[1:] {
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := t.TempDir()
			write := func(path, body string) {
				path = filepath.Join(root, path)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}
			write("sfdx-project.json", `{"sourceApiVersion":"`+row[0]+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			files, err := os.ReadDir(fixture)
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range files {
				if strings.HasSuffix(f.Name(), ".cls") || strings.HasSuffix(f.Name(), ".xml") {
					raw, err := os.ReadFile(filepath.Join(fixture, f.Name()))
					if err != nil {
						t.Fatal(err)
					}
					if strings.HasSuffix(f.Name(), "Test.cls") {
						continue
					}
					write("force-app/main/default/classes/"+f.Name(), string(raw))
				}
			}
			var expected map[string]any
			if err := json.Unmarshal([]byte(row[2]), &expected); err != nil {
				t.Fatal(err)
			}
			if expected["userBlobCounted"] != true {
				t.Fatalf("native class payload row: %v", expected)
			}
			helper := "HeapIdentity" + strings.TrimSuffix(row[0], ".0")
			assertion := `System.assertEquals(JSON.deserializeUntyped('` + row[2] + `'),JSON.deserializeUntyped(` + helper + `.observe()));`
			write("force-app/main/default/classes/HeapPayloadTest.cls", `@IsTest private class HeapPayloadTest { @IsTest static void observed() { `+assertion+` } }`)
			write("force-app/main/default/classes/HeapPayloadTest.cls-meta.xml", `<ApexClass><apiVersion>`+row[0]+`</apiVersion></ApexClass>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, s)
			org := apextest.OrgFromIndex(index)
			org.APIVersion = row[0]
			machine := vm.New(nil)
			machine.SetOrg(&org)
			if err := apextest.RegisterProjectRuntime(machine, index); err != nil {
				t.Fatal(err)
			}
			source := assertion
			if row[1] == "isTest" {
				machine.EnableTestContext()
				if err := apextest.RegisterTestMethodForRequest(machine, index, "HeapPayloadTest", "observed"); err != nil {
					t.Fatal(err)
				}
				source = "HeapPayloadTest.observed();"
			}
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			if row[1] == "isTest" {
				_, err = machine.ExecuteTestInClass(program, "HeapPayloadTest")
			} else {
				_, err = machine.Execute(program)
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
