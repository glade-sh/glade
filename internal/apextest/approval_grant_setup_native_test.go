package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// Native setup.apex succeeded at API 67 before the captured grant observations.
// This checks that the group and muting records can actually be constructed.
func TestApprovalGrantSetupNativeConformance(t *testing.T) {
	fixture := filepath.Join("testdata/conformance/approval_grant_activity/project", "setup.apex")
	source, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"name":"GrantSetup","sourceApiVersion":"67.0"}`)
	index := loadTestIndex(t, root)
	org := orgFromIndex(index)
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	machine := runner.newMachine()
	program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range result.Debug {
		if strings.Contains(line, "APPROVAL_GRANT_SETUP_OK") {
			return
		}
	}
	t.Fatalf("native setup succeeded; local observations: %v", result.Debug)
}

// Construct the captured group through Apex, assign it through Apex, then
// submit the captured locked-record update against that same runtime state.
func TestApprovalGrantSetupToMutation(t *testing.T) {
	for _, state := range []string{"muted", "unmuted", "direct", "revoked", "expired"} {
		t.Run(state, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"name":"GrantSetup","sourceApiVersion":"67.0"}`)
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			user := approvalGrantStandardUser(t, &org)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			execute := func(source string) vm.Result {
				t.Helper()
				p, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: "67.0"})
				if err != nil {
					t.Fatal(err)
				}
				result, err := machine.Execute(p)
				if err != nil {
					t.Fatal(err)
				}
				return result
			}
			source, err := os.ReadFile(filepath.Join("testdata/conformance/approval_grant_activity/project", "setup.apex"))
			if err != nil {
				t.Fatal(err)
			}
			setup := execute(string(source))
			if !strings.Contains(strings.Join(setup.Debug, "\n"), "APPROVAL_GRANT_SETUP_OK") {
				t.Fatalf("setup: %v", setup.Debug)
			}
			execute(`User u=[SELECT Id FROM User WHERE Profile.Name='Standard User' AND IsActive=true LIMIT 1]; PermissionSetGroup g=[SELECT Id FROM PermissionSetGroup WHERE DeveloperName='ApprovalGroup']; insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetGroupId=g.Id);`)
			if state == "unmuted" {
				execute(`MutingPermissionSet m=[SELECT Id FROM MutingPermissionSet WHERE DeveloperName='ApprovalMute']; PermissionSetGroup g=[SELECT Id FROM PermissionSetGroup WHERE DeveloperName='ApprovalGroup']; delete [SELECT Id FROM PermissionSetGroupComponent WHERE PermissionSetGroupId=:g.Id AND PermissionSetId=:m.Id];`)
			}
			if state == "direct" || state == "revoked" || state == "expired" {
				assignment := `User u=[SELECT Id FROM User WHERE Profile.Name='Standard User' AND IsActive=true LIMIT 1]; PermissionSet p=[SELECT Id FROM PermissionSet WHERE Name='ApprovalGrant']; PermissionSetAssignment a=new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=p.Id);`
				if state == "expired" {
					assignment += `a.ExpirationDate=System.now().addDays(-1);`
				}
				assignment += `insert a;`
				if state == "revoked" {
					assignment += `a.IsRevoked=true; update a;`
				}
				execute(assignment)
			}
			machine.SetCurrentUser(user)
			anon, err := os.ReadFile(filepath.Join("testdata/conformance/approval_grant_activity/project", "anonymous.apex"))
			if err != nil {
				t.Fatal(err)
			}
			result := execute(string(anon))
			want := "false:ENTITY_IS_LOCKED"
			if state == "unmuted" || state == "direct" {
				want = "true:NONE"
			}
			found := false
			for _, line := range result.Debug {
				if _, v, ok := strings.Cut(line, "PERMISSION_ROW|"); ok {
					found = true
					if v != want {
						t.Fatalf("got %s want %s", v, want)
					}
				}
			}
			if !found {
				t.Fatalf("missing result: %v", result.Debug)
			}
		})
	}
}

func approvalGrantStandardUser(t *testing.T, org *storage.OrgState) storage.Record {
	t.Helper()
	var profile storage.ID
	for id, row := range org.Objects["Profile"].Records {
		if row.Fields["Name"].String == "Standard User" {
			profile = id
			break
		}
	}
	if profile == "" {
		t.Fatal("standard profile missing")
	}
	id := storage.ID("005000000000991AAA")
	user := storage.Record{ID: id, Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue(id), "ProfileId": storage.IDValue(profile), "IsActive": storage.BooleanValue(true)}}
	users := org.Objects["User"]
	users.Records[id] = user
	org.Objects["User"] = users
	parent := profile
	for pid, row := range org.Objects["PermissionSet"].Records {
		if row.Fields["IsOwnedByProfile"].Boolean && storage.IDsEqual(row.Fields["ProfileId"].ID, profile) {
			parent = pid
			break
		}
	}
	storage.EnsureStandardObject(org, "ObjectPermissions")
	perms := org.Objects["ObjectPermissions"]
	for key, row := range perms.Records {
		object, _ := row.GetField("SObjectType")
		if storage.IDsEqual(row.Fields["ParentId"].ID, parent) && strings.EqualFold(object.String, "Account") {
			delete(perms.Records, key)
		}
	}
	perm := storage.ID("110000000000990AAA")
	perms.Records[perm] = storage.Record{ID: perm, Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(parent), "SObjectType": storage.StringValue("Account"), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true), "PermissionsDelete": storage.BooleanValue(true)}}
	org.Objects["ObjectPermissions"] = perms
	return user
}
