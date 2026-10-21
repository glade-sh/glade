package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Native controls and distribution-policy rows are deliberately separate:
// the clean org does not expose the feature-specific catalog fields.
func TestStandardPicklistCatalogDescribeConformance(t *testing.T) {
	file, err := os.Open("testdata/conformance/standard_picklist_catalog/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 33 {
		t.Fatalf("native rows = %d, want 32", len(rows)-1)
	}
	controls := 0
	for _, row := range rows[1:] {
		if row[3] == "UNAVAILABLE" {
			continue
		}
		controls++
		if row[1] != "anonymous" || row[2] != "Account.Industry" || row[3] != "0" || row[4] != "32" {
			t.Fatal("native control contract changed")
		}
	}
	if controls != 2 {
		t.Fatalf("native controls = %d, want 2", controls)
	}
	fieldsRaw, err := os.ReadFile("testdata/conformance/standard_picklist_catalog/policy-fields.txt")
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(fieldsRaw))
	if len(fields) != 15 {
		t.Fatalf("policy fields = %d, want 15", len(fields))
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/CatalogControl.cls"), "public class CatalogControl {}")
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			check := func(t *testing.T, field string, total string) {
				t.Helper()
				parts := strings.Split(field, ".")
				code := fmt.Sprintf(`Integer customCount=0; Integer standardCount=0;
for (Schema.PicklistEntry entry : Schema.getGlobalDescribe().get('%s').getDescribe().fields.getMap().get('%s').getDescribe().getPicklistValues()) {
 String value=entry.getValue();
 if (value.endsWith('__c') || value.endsWith('__e') || value.endsWith('__mdt') || value.endsWith('__x')) { customCount++; } else { standardCount++; }
}
System.assertEquals(0,customCount,'custom picklist entry count');`, parts[0], parts[1])
				if total != "" {
					code += "System.assertEquals(" + total + ",standardCount,'standard picklist entry count');"
				}
				if a := sema.AnalyzeAnonymous(index, code, api); a.HasErrors() {
					t.Fatal(a.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(code, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := runner.execute(program); err != nil {
					t.Fatal(err)
				}
			}
			t.Run("native-control", func(t *testing.T) { check(t, "Account.Industry", "32") })
			for _, field := range fields {
				t.Run("policy/"+field, func(t *testing.T) { check(t, field, "") })
			}
		})
	}
}
