package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/config"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestR6FlowExpiryNativeConformance(t *testing.T) {
	fixture := filepath.Join("testdata", "conformance", "r6flow_expiry")
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 13 {
		t.Fatalf("rows=%d error=%v", len(rows), err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			root := t.TempDir()
			copyDiskRuntimeScopeProject(t, filepath.Join(fixture, "api"+strings.TrimSuffix(row[0], ".0")), root)
			index := loadTestIndex(t, root)
			if row[1] == "isTest" {
				if strings.HasPrefix(row[3], "SETUP|") {
					// Native rejected the setup before a Flow interview existed.
					if row[0] != "67.0" || row[2] != "revoked" || !strings.Contains(row[3], "fields being inaccessible") {
						t.Fatalf("unexpected native setup rejection: %q", row[3])
					}
					return
				}
				run := Run(index, Options{SelectedClasses: []string{"R6ExpiryTest"}, SelectedMethod: row[2], NoDiskCache: true, Parallelism: 1})
				if run.Summary().Total != 1 {
					t.Fatalf("selection: %+v", run.Summary())
				}
				message := firstRunProblem(run)
				_, got, ok := strings.Cut(message, "R6|")
				if !ok || got != row[3] {
					t.Fatalf("native=%q local=%q", row[3], message)
				}
				return
			}
			org := orgFromIndex(index)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			p.Test.PermissionSets = []string{"R6_Bypass"}
			if err := applyConfiguredTestPermissions(&org, p); err != nil {
				t.Fatal(err)
			}
			setID, ok := recordFieldID(org.Objects["PermissionSet"], "Name", "R6_Bypass")
			if !ok {
				t.Fatal("missing permission set")
			}
			state := org.Objects["PermissionSetAssignment"]
			// The conformance VM's initial clock is the documented default.
			now, err := time.Parse(time.RFC3339, config.DefaultTestClockStart)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for id, record := range state.Records {
				value := record.Fields["PermissionSetId"]
				idValue := value.ID
				if value.Kind == storage.ValueString {
					idValue = storage.ID(value.String)
				}
				if !storage.IDsEqual(idValue, setID) {
					continue
				}
				switch row[2] {
				case "future":
					record.Fields["ExpirationDate"] = storage.DateTimeValue(now.Add(48 * time.Hour).Format(time.RFC3339))
				case "past":
					record.Fields["ExpirationDate"] = storage.DateTimeValue(now.Add(-48 * time.Hour).Format(time.RFC3339))
				case "revoked":
					record.Fields["IsRevoked"] = storage.BooleanValue(true)
				}
				state.Records[id] = record
				found = true
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
