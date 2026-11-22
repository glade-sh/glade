package vm

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// NATIVE lead_read_anonymous_update/org.tsv:2-3 anonymous APIs 62.0 and 67.0.
func TestLeadAnonymousUpdateUncapturedField(t *testing.T) {
	base := "../apextest/testdata/conformance/lead_read_anonymous_update"
	f, err := os.Open(filepath.Join(base, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(base, "update.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		row := row
		for _, tc := range []struct {
			name                            string
			queried, permission, ownerAdmin bool
		}{
			{"uncaptured-different-text-field", true, true, true},
		} {
			t.Run(row[1]+"/"+tc.name, func(t *testing.T) {
				org := stripInaccessibleTestOrg()
				storage.EnsureStandardObject(&org, "Lead")
				lead := org.Objects["Lead"]
				lead.Definition.SharingModel = "Read"
				lead.Records = map[storage.ID]storage.Record{
					"00Q000000000001": {ID: "00Q000000000001", Object: "Lead", System: storage.SystemFields{OwnerID: "005000000000001", CreatedByID: "005000000000001"}, Fields: map[string]storage.Value{"LastName": storage.StringValue("Route Lead"), "Company": storage.StringValue("Original")}},
				}
				org.Objects["Lead"] = lead
				profile := org.Objects["Profile"]
				reader := profile.Records["00e000000000002"]
				reader.Fields["Name"] = storage.StringValue("Standard User")
				profile.Records[reader.ID] = reader
				ownerProfile := "Standard User"
				if tc.ownerAdmin {
					ownerProfile = "System Administrator"
				}
				profile.Records["00e000000000001"] = storage.Record{ID: "00e000000000001", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue(ownerProfile)}}
				org.Objects["Profile"] = profile
				storage.EnsureStandardObject(&org, "User")
				users := org.Objects["User"]
				users.Records = map[storage.ID]storage.Record{"005000000000001": {ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000001")}}}
				org.Objects["User"] = users
				permissions := org.Objects["ObjectPermissions"]
				for id, p := range permissions.Records {
					p.Fields["SObjectType"] = storage.StringValue("Lead")
					permissions.Records[id] = p
				}
				org.Objects["ObjectPermissions"] = permissions
				storage.EnsureStandardObject(&org, "PermissionSet")
				storage.EnsureStandardObject(&org, "PermissionSetAssignment")
				sets := org.Objects["PermissionSet"]
				sets.Records = map[storage.ID]storage.Record{"0PS000000000001": {ID: "0PS000000000001", Object: "PermissionSet", Fields: map[string]storage.Value{
					"PermissionsApiEnabled": storage.BooleanValue(true), "PermissionsAuthorApex": storage.BooleanValue(true),
					"PermissionsViewSetup": storage.BooleanValue(true), "PermissionsModifyMetadata": storage.BooleanValue(true), "PermissionsViewRoles": storage.BooleanValue(true),
				}}}
				if !tc.permission {
					set := sets.Records["0PS000000000001"]
					set.Fields["PermissionsAuthorApex"] = storage.BooleanValue(false)
					sets.Records[set.ID] = set
				}
				org.Objects["PermissionSet"] = sets
				assignments := org.Objects["PermissionSetAssignment"]
				assignments.Records = map[storage.ID]storage.Record{"0Pa000000000001": {ID: "0Pa000000000001", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{
					"AssigneeId": storage.IDValue("005000000000002"), "PermissionSetId": storage.IDValue("0PS000000000001"),
				}}}
				org.Objects["PermissionSetAssignment"] = assignments
				machine := New(nil)
				machine.SetOrg(&org)
				machine.SetCurrentUser(storage.Record{ID: "005000000000002", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue(reader.ID), "UserType": storage.StringValue("Standard")}})
				body := strings.ReplaceAll(strings.ReplaceAll(string(source), "Id,Company", "Id,LastName"), "l.Company=", "l.LastName=")
				if !tc.queried {
					body = strings.Replace(body, "[SELECT Id,Company FROM Lead WHERE LastName='Route Lead' LIMIT 1]", "new Lead(Id='00Q000000000001')", 1)
				}
				program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: row[1]})
				if err != nil {
					t.Fatal(err)
				}
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				want := row[5]
				if tc.name != "native" {
					want = "update=ok"
					if row[1] == "67.0" {
						want = "update=System.SecurityException"
					}
				}
				if len(result.Debug) != 1 || result.Debug[0] != "OBS|"+want {
					t.Fatalf("Glade=%v want=%q", result.Debug, want)
				}
			})
		}
	}
}
