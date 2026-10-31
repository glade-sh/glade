package vm_test

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestSchemaFieldMapOriginalMutationConformance(t *testing.T) {
	const dir = "../../testdata/conformance/schema_field_map_copy"
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
	expected := map[string]string{}
	for _, row := range rows[1:] {
		expected[row[0]+"/"+row[1]+"/"+row[2]] = row[3]
	}
	for _, api := range []string{"62.0", "67.0"} {
		var named strings.Builder
		named.WriteString("@IsTest private class FieldMapOriginalProbe {\n")
		for i := 48; i <= 55; i++ {
			id := fmt.Sprintf("C%d", i)
			data, e := os.ReadFile(filepath.Join(dir, "readonly-"+id+".apex"))
			if e != nil {
				t.Fatal(e)
			}
			want := expected[api+"/anonymous/"+id]
			if want != expected[api+"/isTest/"+id] || (want != "success" && want != "FinalException") {
				t.Fatalf("missing oracle %s %s", api, id)
			}
			fmt.Fprintf(&named, "@IsTest static void observed%s(){%s}\n", id, data)
			t.Run(api+"/"+id, func(t *testing.T) {
				program, e := vm.CompileAnonymousWithOptions(string(data), vm.CompileOptions{APIVersion: api})
				if e != nil {
					t.Fatal(e)
				}
				org := storage.NewOrgState()
				machine := vm.New(nil)
				machine.SetOrg(&org)
				_, e = machine.Execute(program)
				if want == "success" && e != nil {
					t.Fatal(e)
				}
				if want == "FinalException" && (e == nil || !strings.Contains(e.Error(), "System.FinalException")) {
					t.Fatalf("expected FinalException: %v", e)
				}
			})
		}
		raw, e := os.ReadFile(filepath.Join(dir, "review2-clear.apex"))
		if e != nil {
			t.Fatal(e)
		}
		re := regexp.MustCompile(`System\.debug\('ROAD123\|(C[0-9]+)\|'\+(.+)\);`)
		source := re.ReplaceAllStringFunc(string(raw), func(line string) string {
			p := re.FindStringSubmatch(line)
			if expected[api+"/anonymous/"+p[1]] != "true" || expected[api+"/isTest/"+p[1]] != "true" {
				t.Fatalf("missing %s", p[1])
			}
			return fmt.Sprintf("System.assertEquals(true,%s,'%s');", p[2], p[1])
		})
		fmt.Fprintf(&named, "@IsTest static void observedClearCopy(){%s}\n}\n", source)
		t.Run(api+"/clearCopy", func(t *testing.T) {
			p, e := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
			if e != nil {
				t.Fatal(e)
			}
			o := storage.NewOrgState()
			m := vm.New(nil)
			m.SetOrg(&o)
			if _, e = m.Execute(p); e != nil {
				t.Fatal(e)
			}
		})
		root := t.TempDir()
		path := filepath.Join(root, "FieldMapOriginalProbe.cls")
		if e = os.WriteFile(path, []byte(named.String()), 0600); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path+"-meta.xml", []byte("<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>"), 0600); e != nil {
			t.Fatal(e)
		}
		index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}, SourceAPIVersion: api}, schema.Schema{})
		run := apextest.Run(index, apextest.Options{NoDiskCache: true, Parallelism: 1})
		count := 0
		for _, suite := range run.Suites {
			for _, result := range suite.Cases {
				count++
				if result.MethodName == "observedClearCopy" || result.MethodName == "observedC54" {
					if result.Status != testreport.StatusPass {
						t.Errorf("%s %s %v", api, result.MethodName, result.Problem)
					}
				} else if result.Status == testreport.StatusPass || result.Problem == nil || !strings.Contains(result.Problem.Message, "FinalException") {
					t.Errorf("%s %s expected FinalException: %v", api, result.MethodName, result.Problem)
				}
			}
		}
		if count != 9 {
			t.Errorf("%s: got %d methods", api, count)
		}
	}
}
