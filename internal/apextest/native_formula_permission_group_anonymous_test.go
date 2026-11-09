package apextest

import (
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
	"path/filepath"
	"strings"
	"testing"
)

// Supplemental org.tsv rows 6-7: assign the captured group to the current
// user, then compare FeatureManagement and validation-rule DML in anonymous Apex.
func TestNativeFormulaPermissionGroupAnonymous(t *testing.T) {
	root := t.TempDir()
	copyDiskRuntimeScopeProject(t, filepath.Join("testdata", "conformance", "native_formula_permission_regex"), root)
	index := loadTestIndex(t, root)
	for _, row := range readNativeRows(t, filepath.Join(root, "supplemental"), 6) {
		if row[1] != "anonymous:group" {
			continue
		}
		t.Run(row[0], func(t *testing.T) {
			source := `Database.SaveResult r=Database.insert(new ProbeRecord__c(Name='guarded'),false);
System.debug('Q1ANON|hasBypass='+FeatureManagement.checkPermission('Probe_Bypass')+';guarded='+(r.isSuccess()?'ok':'denied:'+r.getErrors()[0].getMessage()));`
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			group, found := recordFieldID(org.Objects["PermissionSetGroup"], "DeveloperName", "Probe_Group")
			if !found {
				t.Fatal("permission set group missing from project metadata")
			}
			// The native assignment was made in a separate transaction.
			setup := dml.NewEngine(&org)
			assigned := setup.Insert([]storage.Record{{Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(storage.DefaultUserID), "PermissionSetGroupId": storage.IDValue(group)}}})
			if len(assigned) != 1 || !assigned[0].Success {
				t.Fatalf("group setup: %+v", assigned)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			result, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			var got string
			for _, line := range result.Debug {
				if strings.HasPrefix(line, "Q1ANON|") {
					got = strings.TrimPrefix(line, "Q1ANON|")
				}
			}
			if got != row[2] {
				t.Fatalf("native %q local %q", row[2], got)
			}
		})
	}
}
