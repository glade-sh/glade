package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Salesforce's record-triggered controls and deployment rejections are in
// testdata/conformance/flow_record_action_dependency/org.tsv. Salesforce did
// not execute the rejected flows; Glade's runtime errors below are local pins.
func TestFlowRecordActionDependencyNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_record_action_dependency"
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 13 {
		t.Fatalf("native rows=%d, error=%v", len(rows), err)
	}
	native := map[string][]string{}
	for _, row := range rows[1:] {
		if len(row) != 9 {
			t.Fatalf("invalid native row: %v", row)
		}
		key := row[1] + "/" + row[0]
		if native[key] != nil {
			t.Fatalf("duplicate native row %q", key)
		}
		native[key] = row
	}
	variants := []struct {
		name, flowFile, target, nativeCase string
	}{
		{"loaded", "force-app/main/default/flows/HarborFlowControl.flow-meta.xml", "", ""},
		{"missing", "deploy-checks/missing.flow-meta.xml", "AbsentFlowAction", "missing_action"},
		{"noninvocable", "deploy-checks/noninvocable.flow-meta.xml", "PlainFlowAction", "noninvocable_action"},
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, variant := range variants {
			t.Run(api+"/"+variant.name, func(t *testing.T) {
				if variant.target == "" {
					anon, named := native[api+"/control_insert"], native[api+"/control_test"]
					if anon == nil || anon[3] != "True" || anon[4] != "True" || anon[5] != "ok" || named == nil || named[3] != "true" || named[4] != "Passed" {
						t.Fatalf("missing native control rows: %v %v", anon, named)
					}
				} else {
					row := native[api+"/"+variant.nativeCase]
					if row == nil || row[2] != "metadata-deploy-checkonly" || row[4] != "False" || !strings.Contains(row[5], "can't find the "+variant.target+" action") {
						t.Fatalf("missing native deployment rejection: %v", row)
					}
				}
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
				base := filepath.Join(root, "force-app/main/default")
				flow := strings.ReplaceAll(read(variant.flowFile), "__API__", api)
				writeFile(t, filepath.Join(base, "flows/HarborFlowControl.flow-meta.xml"), flow)
				for _, name := range []string{"HarborFlowAction", "PlainFlowAction"} {
					path := "force-app/main/default/classes/" + name
					writeFile(t, filepath.Join(base, "classes", name+".cls"), read(path+".cls"))
					writeFile(t, filepath.Join(base, "classes", name+".cls-meta.xml"), strings.ReplaceAll(read(path+".cls-meta.xml"), "__API__", api))
				}
				testSource := read("force-app/main/default/classes/HarborFlowControlTest.cls")
				anonSource := read("control.apex")
				if variant.target != "" {
					testSource = `@IsTest private class HarborFlowControlTest { @IsTest static void recordActionRuns() { insert new Account(Name='Control'); } }`
					anonSource = `insert new Account(Name='Control');`
				}
				writeFile(t, filepath.Join(base, "classes/HarborFlowControlTest.cls"), testSource)
				writeFile(t, filepath.Join(base, "classes/HarborFlowControlTest.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
				index := loadTestIndex(t, root)
				run := Run(index, Options{SelectedClasses: []string{"HarborFlowControlTest"}, SelectedMethod: "recordActionRuns", NoDiskCache: true, Parallelism: 1})
				if variant.target == "" {
					if run.Summary().Total != 1 || run.Summary().Passed != 1 {
						t.Fatalf("native test control passed; Glade: %s", firstRunProblem(run))
					}
				} else if run.Summary().Total != 1 || run.Summary().Passed != 0 || !strings.Contains(firstRunProblem(run), "no static @InvocableMethod found") {
					t.Fatalf("local missing-action test route: %s", firstRunProblem(run))
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				program, err := vm.CompileAnonymousWithOptions(anonSource, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				_, err = runner.execute(program)
				if variant.target == "" && err != nil {
					t.Fatalf("native anonymous control passed; Glade: %v", err)
				}
				if variant.target != "" && (err == nil || !strings.Contains(err.Error(), "no static @InvocableMethod found")) {
					t.Fatalf("local missing-action anonymous route: %v", err)
				}
			})
		}
	}
}
