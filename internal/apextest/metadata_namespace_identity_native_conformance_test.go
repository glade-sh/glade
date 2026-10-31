package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestMetadataNamespaceIdentityNativeConformance(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(metadataNamespaceFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var data metadataNamespaceData
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open("testdata/conformance/metadata_namespace_identity/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 17 {
		t.Fatalf("native rows=%d err=%v", len(rows), err)
	}
	expected := map[string]string{}
	for _, row := range rows[1:] {
		if len(row) != 4 {
			t.Fatalf("bad native row %v", row)
		}
		key := strings.Join(row[:3], "/")
		if _, ok := expected[key]; ok {
			t.Fatalf("duplicate native row %s", key)
		}
		expected[key] = row[3]
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(route, func(t *testing.T) {
					get := func(id string) string {
						v, ok := expected[api+"/"+route+"/"+id]
						if !ok {
							t.Fatalf("missing row %s", id)
						}
						return v
					}
					body := fmt.Sprintf(`System.assertEquals('%s', gladens001__Ns113bFlag__mdt.gladens001__Enabled__c.getDescribe().getName());
System.assertEquals('%s', gladens001__Ns113bFlag__mdt.SObjectType.getDescribe().getName());
List<FieldDefinition> found = [SELECT QualifiedApiName FROM FieldDefinition WHERE EntityDefinition.QualifiedApiName = 'gladens001__Ns113bFlag__mdt' AND QualifiedApiName = 'gladens001__Enabled__c'];
System.assertEquals('%s', found.size() + ':' + found[0].QualifiedApiName);
List<FieldDefinition> missing = [SELECT QualifiedApiName FROM FieldDefinition WHERE EntityDefinition.QualifiedApiName = 'gladens001__Ns113bFlag__mdt' AND QualifiedApiName = 'gladens001__NoSuchField__c'];
System.assertEquals(%s, missing.size());`, get("field-describe"), get("object-describe"), get("field-definition"), get("missing"))
					classes := map[string]string{}
					if route == "istest" {
						classes["NamespaceIdentityTest"] = "@IsTest private class NamespaceIdentityTest { @IsTest static void observe(){" + body + "}}"
					}
					index := metadataNamespaceIndex(t, api, classes, data)
					org := orgFromIndex(index)
					org.APIVersion = api
					if route == "anonymous" {
						if got := sema.AnalyzeAnonymous(index, body, api); got.HasErrors() {
							t.Fatalf("native anonymous accepted: %+v", got.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						if _, err = runner.execute(program); err != nil {
							t.Fatalf("native anonymous assertions: %v", err)
						}
						return
					}
					cases := Discover(index, Options{SelectedClasses: []string{"NamespaceIdentityTest"}, SelectedMethod: "observe"})
					if len(cases) != 1 {
						t.Fatalf("expected one case, got %d", len(cases))
					}
					methods, errs := compileTestMethods(cases)
					programs, perrs := compileTestInvokePrograms(cases)
					key := testCaseKey(cases[0])
					if errs[key] != nil || perrs[key] != nil {
						t.Fatalf("compile: %v %v", errs[key], perrs[key])
					}
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					machine := runner.newMachine()
					machine.EnableTestContext()
					if err := machine.RegisterMethod(methods[key]); err != nil {
						t.Fatal(err)
					}
					if _, err := machine.ExecuteInClass(programs[key], cases[0].ClassName); err != nil {
						t.Fatalf("native named assertions: %v", err)
					}
				})
			}
		})
	}
}
