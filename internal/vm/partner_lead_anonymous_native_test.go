package vm

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// NATIVE testdata/partner_lead_anonymous/org.tsv:2-5, authenticated anonymous.
func TestPartnerLeadAnonymousNativeConformance(t *testing.T) {
	base := filepath.Join("testdata", "partner_lead_anonymous")
	f, err := os.Open(filepath.Join(base, "org.tsv"))
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
	if len(rows) != 5 {
		t.Fatalf("rows=%d", len(rows)-1)
	}
	source, err := os.ReadFile(filepath.Join(base, "query.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			org := storage.NewOrgState()
			org.APIVersion = row[1]
			storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, "Communities"})
			for _, name := range []string{"Lead", "LeadShare", "User", "Profile", "UserLicense", "PermissionSet", "PermissionSetAssignment", "ObjectPermissions"} {
				storage.EnsureStandardObject(&org, name)
			}
			profile := org.Objects["Profile"]
			profile.Records = map[storage.ID]storage.Record{"00e000000000001": {ID: "00e000000000001", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("System Administrator")}}, "00e000000000002": {ID: "00e000000000002", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Partner API User"), "UserLicenseId": storage.IDValue("100000000000001"), "PermissionsApiEnabled": storage.BooleanValue(true)}}}
			org.Objects["Profile"] = profile
			license := org.Objects["UserLicense"]
			license.Records = map[storage.ID]storage.Record{"100000000000001": {ID: "100000000000001", Object: "UserLicense", Fields: map[string]storage.Value{"Name": storage.StringValue("Partner Community"), "LicenseDefinitionKey": storage.StringValue("PID_Partner_Community")}}}
			org.Objects["UserLicense"] = license
			user := storage.Record{ID: "005000000000002", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000002"), "UserType": storage.StringValue("PowerPartner")}}
			users := org.Objects["User"]
			users.Records = map[storage.ID]storage.Record{user.ID: user, "005000000000001": {ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000001"), "UserType": storage.StringValue("Standard")}}}
			org.Objects["User"] = users
			sets := org.Objects["PermissionSet"]
			sets.Records = map[storage.ID]storage.Record{"0PS000000000001": {ID: "0PS000000000001", Object: "PermissionSet", Fields: map[string]storage.Value{"PermissionsApiEnabled": storage.BooleanValue(true)}}}
			org.Objects["PermissionSet"] = sets
			assignments := org.Objects["PermissionSetAssignment"]
			assignments.Records = map[storage.ID]storage.Record{"0Pa000000000001": {ID: "0Pa000000000001", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(user.ID), "PermissionSetId": storage.IDValue("0PS000000000001")}}}
			org.Objects["PermissionSetAssignment"] = assignments
			permissions := org.Objects["ObjectPermissions"]
			permissions.Records = map[storage.ID]storage.Record{"110000000000001": {ID: "110000000000001", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("0PS000000000001"), "SObjectType": storage.StringValue("Lead"), "PermissionsRead": storage.BooleanValue(true)}}}
			org.Objects["ObjectPermissions"] = permissions
			lead := org.Objects["Lead"]
			lead.Definition.SharingModel = "ReadWriteTransfer"
			lead.Records = map[storage.ID]storage.Record{}
			for id, last := range map[storage.ID]string{"00Q000000000001": "Route First", "00Q000000000002": "Route Second"} {
				lead.Records[id] = storage.Record{ID: id, Object: "Lead", System: storage.SystemFields{OwnerID: "005000000000001", CreatedByID: "005000000000001"}, Fields: map[string]storage.Value{"LastName": storage.StringValue(last), "Company": storage.StringValue("Synthetic")}}
			}
			org.Objects["Lead"] = lead
			shares := org.Objects["LeadShare"]
			shares.Records = map[storage.ID]storage.Record{}
			if row[0] == "partner_after" {
				shares.Records["00T000000000001"] = storage.Record{ID: "00T000000000001", Object: "LeadShare", Fields: map[string]storage.Value{"LeadId": storage.IDValue("00Q000000000001"), "UserOrGroupId": storage.IDValue(user.ID), "LeadAccessLevel": storage.StringValue("Read"), "RowCause": storage.StringValue("Manual")}}
			}
			org.Objects["LeadShare"] = shares
			machine := New(nil)
			machine.SetOrg(&org)
			machine.SetCurrentUser(user)
			program, err := CompileAnonymousWithOptions(string(source), CompileOptions{APIVersion: row[1]})
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			want := "OBS|" + row[3]
			if len(result.Debug) != 1 || result.Debug[0] != want {
				t.Fatalf("Glade=%v native=%q", result.Debug, want)
			}
		})
	}
}
