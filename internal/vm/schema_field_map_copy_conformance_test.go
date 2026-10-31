package vm_test

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
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

// Results C01-C07 are recorded in testdata/conformance/schema_field_map_copy/org.tsv.
func TestSchemaFieldMapCopyConformance(t *testing.T) {
	const setup = `
Map<String, Schema.SObjectField> d = Schema.SObjectType.Account.fields.getMap();
Map<String, Schema.SObjectField> c = new Map<String, Schema.SObjectField>(d);
Map<String, Schema.SObjectField> p = new Map<String, Schema.SObjectField>(); p.putAll(d);
Map<String, Schema.SObjectField> l = d.clone();
Map<String, Schema.SObjectField> ordinary = new Map<String, Schema.SObjectField>{'Name' => Account.Name};
`
	cases := []struct {
		id, expr string
	}{
		{"C01", "d.get('NAME') == Account.Name"},
		{"C02", "c.get('NAME') == Account.Name"},
		{"C03", "p.get('NAME') == Account.Name"},
		{"C04", "l.get('NAME') == Account.Name"},
		{"C05", "c.get('MissingField__c') == null"},
		{"C06", "ordinary.get('NAME') == null"},
		{"C07", "c.get('Name') == Account.Name"},
	}
	f, err := os.Open("../../testdata/conformance/schema_field_map_copy/org.tsv")
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
		if row[1] == "anonymous" {
			expected[row[0]+"/"+row[2]] = row[3]
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		var named strings.Builder
		named.WriteString("@IsTest private class FieldMapCopyProbe {\n")
		for _, tc := range cases {
			want := expected[api+"/"+tc.id]
			if want != "true" && want != "false" {
				t.Fatalf("missing native row %s/%s", api, tc.id)
			}
			assertion := fmt.Sprintf("System.assertEquals(%s, %s);", want, tc.expr)
			fmt.Fprintf(&named, "@IsTest static void observed%s(){%s%s}\n", tc.id, setup, assertion)
			t.Run(api+"/"+tc.id, func(t *testing.T) {
				program, err := vm.CompileAnonymousWithOptions(setup+assertion, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				machine := vm.New(nil)
				machine.SetOrg(&org)
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
		named.WriteString("}\n")
		root := t.TempDir()
		path := filepath.Join(root, "FieldMapCopyProbe.cls")
		if err := os.WriteFile(path, []byte(named.String()), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path+"-meta.xml", []byte("<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>"), 0600); err != nil {
			t.Fatal(err)
		}
		index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}, SourceAPIVersion: api}, schema.Schema{})
		run := apextest.Run(index, apextest.Options{NoDiskCache: true, Parallelism: 1})
		count := 0
		for _, suite := range run.Suites {
			for _, result := range suite.Cases {
				count++
				if result.Status != testreport.StatusPass {
					t.Errorf("%s @IsTest %s: %v", api, result.MethodName, result.Problem)
				}
			}
		}
		if count != len(cases) {
			t.Errorf("%s @IsTest: got %d cases, want %d", api, count, len(cases))
		}
	}
}
