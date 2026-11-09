package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestDatetimeStoragePrecisionNativeProject(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("testdata", "conformance", "case159_datetime", "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	// ORG-RUN.txt: "Field read and edit permission was granted to the scratch
	// user." admin_field_grant_sources (2026-11-05) shows the administrator
	// needs that grant through an assigned permission set (anonymous Apex at
	// every API, named code from API 67), so the copied project replays it.
	grant := func(t *testing.T, root string) {
		writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets/Case159TimeAccess.permissionset-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><PermissionSet xmlns="http://soap.sforce.com/2006/04/metadata"><label>Case159 Time Access</label><fieldPermissions><editable>true</editable><field>RoadTime__c.Moment__c</field><readable>true</readable></fieldPermissions></PermissionSet>`)
		writeCapturedAssignments(t, root, "Case159TimeAccess")
	}
	runCase159ProjectVersionsPrepared(t, "case159_datetime", grant, func(t *testing.T, index typesys.Index) {
		t.Run("named", func(t *testing.T) {
			run := Run(index, Options{SelectedClasses: []string{"Case159TimeTest"}, NoDiskCache: true})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("datetime storage precision: summary=%#v problem=%s", summary, firstRunProblem(run))
			}
		})
		t.Run("anonymous", func(t *testing.T) {
			org := capturedFixtureOrg(t, index, index.Project.SourceAPIVersion)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: index.Project.SourceAPIVersion})
			if err != nil {
				t.Fatal(err)
			}
			result, err := runner.newMachine().Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			log := strings.Join(result.Debug, "\n")
			for _, want := range []string{"CASE159|insert|1777723200007|1777723200007|1777723200000", "CASE159|update|1777723200999|1777723200000"} {
				if !strings.Contains(log, want) {
					t.Errorf("missing native observation %q in %q", want, log)
				}
			}
		})
	})
}
