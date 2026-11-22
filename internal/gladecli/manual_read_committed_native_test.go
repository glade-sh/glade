package gladecli

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE the named reader sees 0, 1, 0 records and has a present false,
// true, false UserRecordAccess row after real commits. See
// manual_read_lifecycle/org.tsv:4-9 (test checkpoints, APIs 62.0 and 67.0).
func TestManualReadCommittedNativeConformance(t *testing.T) {
	base := filepath.Join(criteriaSharingFixtures, "manual_read_lifecycle")
	rows := readCriteriaTSV(t, filepath.Join(base, "org.tsv"))
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(base, "project")
			if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(source, path)
				if err != nil {
					return err
				}
				writeTestFile(t, filepath.Join(root, "force-app/main/default", rel), strings.ReplaceAll(string(b), "67.0", api))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(filepath.Join(base, "project-scratch-def.json"))
			if err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(b))
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			db := filepath.Join(root, "state.sqlite")
			act := func(name string) {
				t.Helper()
				script, err := os.ReadFile(filepath.Join(base, name+".apex"))
				if err != nil {
					t.Fatal(err)
				}
				runCriteriaExec(t, root, db, string(script)+"\nSystem.debug('OBS|done');")
			}
			observe := func() string {
				t.Helper()
				store, org, err := openDBStore(db, root)
				if err != nil {
					t.Fatal(err)
				}
				if err := store.Close(); err != nil {
					t.Fatal(err)
				}
				state, err := awaitDAPStartup(context.Background(), root)
				if err != nil {
					t.Fatal(err)
				}
				machine := vm.New(nil)
				machine.SetOrg(&org)
				machine.SetCurrentNamespace(org.Namespace)
				machine.EnableTestContext()
				if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, state.runtime); err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions(`
User u=[SELECT Id FROM User WHERE Alias='shr' LIMIT 1];
ShareProbe__c r=[SELECT Id FROM ShareProbe__c WHERE Name='commit-control' LIMIT 1];
List<UserRecordAccess> a=[SELECT RecordId,HasReadAccess FROM UserRecordAccess WHERE UserId=:u.Id AND RecordId=:r.Id];
Integer visible; System.runAs(u) { visible=ShareProbeReader.countRows(r.Id); }
System.debug('row='+a.size()+':read='+(a.isEmpty() ? 'missing' : String.valueOf(a[0].HasReadAccess))+':query='+visible);`, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				if len(result.Debug) != 1 {
					t.Fatalf("expected one observation, got %v", result.Debug)
				}
				return result.Debug[0]
			}
			act("setup")
			act("insert")
			for _, phase := range []string{"denied", "granted", "revoked"} {
				if phase == "granted" {
					act("grant")
				}
				if phase == "revoked" {
					act("revoke")
				}
				want := ""
				for _, row := range rows {
					if row["id"] == "manual_read_committed_"+phase && row["api"] == api {
						_, want, _ = strings.Cut(row["outcome"], "Assertion Failed: ")
					}
				}
				if want == "" {
					t.Fatalf("missing native row for %s %s", api, phase)
				}
				if got := observe(); got != want {
					t.Errorf("%s: Glade=%q native=%q", phase, got, want)
				}
				anonymousWant := ""
				for _, row := range rows {
					if row["id"] == "manual_read_anonymous_"+phase && row["api"] == api {
						_, anonymousWant, _ = strings.Cut(row["outcome"], "Assertion Failed: ")
					}
				}
				if anonymousWant == "" {
					t.Fatalf("missing anonymous native row for %s %s", api, phase)
				}
				query := `
User u=[SELECT Id FROM User WHERE Alias='shr' LIMIT 1];
ShareProbe__c r=[SELECT Id FROM ShareProbe__c WHERE Name='commit-control' LIMIT 1];
List<UserRecordAccess> a=[SELECT RecordId,HasReadAccess FROM UserRecordAccess WHERE UserId=:u.Id AND RecordId=:r.Id];
System.debug('OBS|row='+a.size()+':read='+(a.isEmpty() ? 'missing' : String.valueOf(a[0].HasReadAccess)));`
				got := runCriteriaExec(t, root, db, query)
				if got != anonymousWant {
					t.Errorf("anonymous %s: Glade=%q native=%q", phase, got, anonymousWant)
				}
			}
		})
	}
}

// These anonymous projections have no matching native row. Preserve their
// pre-existing empty named projection.
func TestManualReadUncapturedProjection(t *testing.T) {
	for _, tc := range []struct{ name, api, model string }{
		{"api68", "68.0", "Private"},
		{"public", "67.0", "ReadWrite"},
		{"extra-field", "67.0", "Private"},
		{"view-all", "67.0", "Private"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := filepath.Join(criteriaSharingFixtures, "manual_read_lifecycle")
			root := t.TempDir()
			source := filepath.Join(base, "project")
			if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(source, path)
				if err != nil {
					return err
				}
				content := strings.ReplaceAll(string(b), "67.0", tc.api)
				content = strings.ReplaceAll(content, "<sharingModel>Private</sharingModel>", "<sharingModel>"+tc.model+"</sharingModel>")
				if tc.name == "view-all" {
					content = strings.ReplaceAll(content, "<viewAllRecords>false</viewAllRecords>", "<viewAllRecords>true</viewAllRecords>")
				}
				writeTestFile(t, filepath.Join(root, "force-app/main/default", rel), content)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(filepath.Join(base, "project-scratch-def.json"))
			if err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(b))
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+tc.api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			db := filepath.Join(root, "state.sqlite")
			for _, name := range []string{"setup", "insert"} {
				script, err := os.ReadFile(filepath.Join(base, name+".apex"))
				if err != nil {
					t.Fatal(err)
				}
				runCriteriaExec(t, root, db, string(script)+"\nSystem.debug('OBS|done');")
			}
			query := `
User u=[SELECT Id FROM User WHERE Alias='shr' LIMIT 1];
ShareProbe__c r=[SELECT Id FROM ShareProbe__c WHERE Name='commit-control' LIMIT 1];
List<UserRecordAccess> a=[SELECT RecordId,HasReadAccess FROM UserRecordAccess WHERE UserId=:u.Id AND RecordId=:r.Id];
System.debug('OBS|row='+a.size()+':read='+(a.isEmpty() ? 'missing' : String.valueOf(a[0].HasReadAccess)));`
			if tc.name == "extra-field" {
				query = strings.Replace(query, "RecordId,HasReadAccess FROM", "RecordId,HasReadAccess,HasEditAccess FROM", 1)
			}
			got := runCriteriaExec(t, root, db, query)
			if got != "row=0:read=missing" {
				t.Fatalf("uncaptured projection changed: %s", got)
			}
		})
	}
}
