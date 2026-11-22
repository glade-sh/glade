package vm

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// NATIVE testdata/community_case_anonymous/org.tsv:2-3 anonymous 62.0 67.0.
func TestCommunityCaseAnonymousCreateUpdateNativeConformance(t *testing.T) {
	base := filepath.Join("testdata", "community_case_anonymous")
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
	source, err := os.ReadFile(filepath.Join(base, "create-update.apex"))
	if err != nil {
		t.Fatal(err)
	}
	selected := 0
	for _, row := range rows[1:] {
		if row[0] != "case_create_update" {
			continue
		}
		row := row
		selected++
		for _, variant := range []string{"native", "uncaptured-no-api", "uncaptured-no-origin"} {
			t.Run(row[1]+"/"+variant, func(t *testing.T) {
				org := storage.NewOrgState()
				org.APIVersion = row[1]
				for _, object := range []string{"Case", "User", "Profile", "UserLicense", "PermissionSet", "PermissionSetAssignment", "ObjectPermissions", "FieldPermissions"} {
					storage.EnsureStandardObject(&org, object)
				}
				license := org.Objects["UserLicense"]
				license.Records = map[storage.ID]storage.Record{"100000000000001": {ID: "100000000000001", Object: "UserLicense", Fields: map[string]storage.Value{"Name": storage.StringValue("Customer Community"), "LicenseDefinitionKey": storage.StringValue("PID_Customer_Community")}}}
				org.Objects["UserLicense"] = license
				profiles := org.Objects["Profile"]
				profiles.Records = map[storage.ID]storage.Record{"00e000000000002": {ID: "00e000000000002", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Community API Reader"), "UserLicenseId": storage.IDValue("100000000000001"), "PermissionsApiEnabled": storage.BooleanValue(true)}}}
				if variant == "uncaptured-no-api" {
					profile := profiles.Records["00e000000000002"]
					profile.Fields["PermissionsApiEnabled"] = storage.BooleanValue(false)
					profiles.Records[profile.ID] = profile
				}
				org.Objects["Profile"] = profiles
				user := storage.Record{ID: "005000000000002", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000002"), "UserType": storage.StringValue("CspLitePortal")}}
				users := org.Objects["User"]
				users.Records = map[storage.ID]storage.Record{user.ID: user}
				org.Objects["User"] = users
				sets := org.Objects["PermissionSet"]
				sets.Records = map[storage.ID]storage.Record{"0PS000000000001": {ID: "0PS000000000001", Object: "PermissionSet"}}
				org.Objects["PermissionSet"] = sets
				assign := org.Objects["PermissionSetAssignment"]
				assign.Records = map[storage.ID]storage.Record{"0Pa000000000001": {ID: "0Pa000000000001", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(user.ID), "PermissionSetId": storage.IDValue("0PS000000000001")}}}
				org.Objects["PermissionSetAssignment"] = assign
				objects := org.Objects["ObjectPermissions"]
				objects.Records = map[storage.ID]storage.Record{"110000000000001": {ID: "110000000000001", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("0PS000000000001"), "SObjectType": storage.StringValue("Case"), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true), "PermissionsDelete": storage.BooleanValue(false)}}}
				org.Objects["ObjectPermissions"] = objects
				fields := org.Objects["FieldPermissions"]
				fields.Records = map[storage.ID]storage.Record{}
				for id, field := range map[storage.ID]string{"0FP000000000001": "Case.Subject", "0FP000000000002": "Case.Origin"} {
					fields.Records[id] = storage.Record{ID: id, Object: "FieldPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("0PS000000000001"), "SObjectType": storage.StringValue("Case"), "Field": storage.StringValue(field), "PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true)}}
				}
				if variant == "uncaptured-no-origin" {
					delete(fields.Records, "0FP000000000002")
				}
				org.Objects["FieldPermissions"] = fields
				machine := New(nil)
				machine.SetOrg(&org)
				machine.SetCurrentUser(user)
				body := strings.ReplaceAll(string(source), "__API__", strings.TrimSuffix(row[1], ".0"))
				want := "OBS|" + row[5]
				if variant != "native" {
					body = "System.debug('OBS|'+Case.SObjectType.getDescribe().isDeletable());"
					want = "OBS|true"
				}
				program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: row[1]})
				if err != nil {
					t.Fatal(err)
				}
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				if len(result.Debug) != 1 || result.Debug[0] != want {
					t.Fatalf("Glade=%v want=%q", result.Debug, want)
				}
			})
		}
	}
	if selected != 2 {
		t.Fatalf("create/update rows=%d, want 2", selected)
	}
}
