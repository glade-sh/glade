package apextest

import (
	"encoding/csv"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestR6FlowUnrelatedPermissionNativeConformance(t *testing.T) {
	fixture := filepath.Join("testdata", "conformance", "r6flow_unrelated")
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 {
		t.Fatalf("rows=%d error=%v", len(rows), err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := t.TempDir()
			copyDiskRuntimeScopeProject(t, filepath.Join(fixture, "api"+strings.TrimSuffix(row[0], ".0")), root)
			index := loadTestIndex(t, root)
			if row[1] == "isTest" {
				run := Run(index, Options{SelectedClasses: []string{"R6UnrelatedTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				if run.Summary().Total != 1 || run.Summary().Passed != 1 {
					t.Fatalf("native=%s local=%s", row[3], firstRunProblem(run))
				}
				return
			}
			org := orgFromIndex(index)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			p.Test.PermissionSets = []string{"R6_Other", "R6_Bypass"}
			if err := applyConfiguredTestPermissions(&org, p); err != nil {
				t.Fatal(err)
			}
			state := org.Objects["PermissionSetAssignment"]
			bypassID, ok := recordFieldID(org.Objects["PermissionSet"], "Name", "R6_Bypass")
			if !ok {
				t.Fatal("missing bypass permission set")
			}
			setID, ok := recordFieldID(org.Objects["PermissionSet"], "Name", "R6_Other")
			if !ok {
				t.Fatal("missing permission set")
			}
			found := false
			for id, record := range state.Records {
				v := record.Fields["PermissionSetId"]
				value := v.ID
				if v.Kind == storage.ValueString {
					value = storage.ID(v.String)
				}
				if storage.IDsEqual(value, bypassID) {
					record.Fields["IsRevoked"] = storage.BooleanValue(true)
					state.Records[id] = record
				}
				if storage.IDsEqual(value, setID) {
					record.Fields["ExpirationDate"] = storage.DateTimeValue(time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC).Format(time.RFC3339))
					state.Records[id] = record
					found = true
				}
			}
			if !found {
				t.Fatal("missing assignment")
			}
			org.Objects["PermissionSetAssignment"] = state
			program, err := vm.CompileAnonymousWithOptions("System.debug('R6='+FeatureManagement.checkPermission('R6_Bypass')+'|'+R6Probe.observe('R6Perm'));", vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			result, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range result.Debug {
				if line == "R6="+row[3] {
					return
				}
			}
			t.Fatalf("native=%s local=%v", row[3], result.Debug)
		})
	}
}
