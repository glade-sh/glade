package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPermsetMutingMultipleFieldsNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/permset_muting_multiple"
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
	if len(rows) != 8 {
		t.Fatalf("native rows=%d, want seven", len(rows)-1)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			root := copyPermsetMutingProject(t, "testdata/conformance/permset_muting/project", row[0])
			put := func(source, target string) {
				t.Helper()
				b, err := os.ReadFile(filepath.Join(dir, source))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default", target), strings.ReplaceAll(string(b), "67.0", row[0]))
			}
			put("Other.field-meta.xml", "objects/SynthMute__c/fields/Other__c.field-meta.xml")
			put("MuteGrant.permissionset-meta.xml", "permissionsets/MuteGrant.permissionset-meta.xml")
			put("PermMuteOther.cls", "classes/PermMuteOther.cls")
			put("PermMuteOther.cls-meta.xml", "classes/PermMuteOther.cls-meta.xml")
			put(row[1]+".xml", "mutingpermissionsets/M01_FieldEditMute.mutingpermissionset-meta.xml")
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatalf("project diagnostics: %v", index.Diagnostics)
			}
			class := "PermMuteObserve"
			if row[2] == "Other__c" {
				class = "PermMuteOther"
			}
			run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "M01_FieldEdit_actor", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("missing test result: %+v", run.Summary())
			}
			problem := run.Suites[0].Cases[0].Problem
			if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
				t.Fatalf("missing observation: %+v", problem)
			}
			_, payload, ok := strings.Cut(problem.Message, "OBS|")
			if !ok {
				t.Fatal("missing OBS payload")
			}
			var got map[string]any
			var want any
			if err := json.Unmarshal([]byte(payload), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got["fieldDescribe"], want) {
				t.Errorf("native=%v Glade=%v", want, got["fieldDescribe"])
			}
		})
	}
}
