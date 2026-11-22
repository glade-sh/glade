package visualforce_test

import (
	"encoding/csv"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE testdata/community_case_page/org.tsv:2-3, rendered Visualforce action.
func TestCommunityCasePageNativeConformance(t *testing.T) {
	base := filepath.Join("testdata", "community_case_page")
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
	if len(rows) != 3 {
		t.Fatalf("rows=%d", len(rows)-1)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[1], func(t *testing.T) {
			root := t.TempDir()
			write := func(path string, data []byte) {
				t.Helper()
				path = filepath.Join(root, path)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			write("sfdx-project.json", []byte(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`))
			src := filepath.Join(base, "project")
			if err := filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(src, path)
				if err != nil {
					return err
				}
				write(filepath.Join("force-app/main/default", rel), data)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			sch, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(typesys.Build(p, sch), nil)
			if err != nil {
				t.Fatal(err)
			}
			org := communityCasePageOrg(row[1])
			machine := vm.New(nil)
			machine.SetOrg(&org)
			machine.SetCurrentUser(org.Objects["User"].Records["005000000000002"])
			if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
				t.Fatal(err)
			}
			visualforce.SetVMRenderEnvironment(machine, p)
			defer visualforce.ClearVMRenderEnvironment(machine)
			page := "RouteCasePage" + strings.TrimSuffix(row[1], ".0")
			program, err := vm.CompileAnonymousWithOptions("System.debug(new PageReference('/apex/"+page+"').getContent().toString());", vm.CompileOptions{APIVersion: row[1]})
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Debug) != 1 {
				t.Fatalf("debug rows=%d", len(result.Debug))
			}
			body := html.UnescapeString(result.Debug[0])
			start := strings.Index(body, "OBS|")
			end := strings.Index(body, "|END")
			if start < 0 || end < start {
				t.Fatalf("observation missing in %q", body)
			}
			got := body[start+4 : end]
			if got != row[3] {
				t.Fatalf("Glade=%q native=%q", got, row[3])
			}
		})
	}
}

func communityCasePageOrg(api string) storage.OrgState {
	org := storage.NewOrgState()
	org.APIVersion = api
	for _, object := range []string{"Case", "User", "Profile", "UserLicense", "PermissionSet", "PermissionSetAssignment", "ObjectPermissions", "FieldPermissions"} {
		storage.EnsureStandardObject(&org, object)
	}
	license := org.Objects["UserLicense"]
	license.Records = map[storage.ID]storage.Record{"100000000000001": {ID: "100000000000001", Object: "UserLicense", Fields: map[string]storage.Value{"Name": storage.StringValue("Customer Community"), "LicenseDefinitionKey": storage.StringValue("PID_Customer_Community")}}}
	org.Objects["UserLicense"] = license
	profiles := org.Objects["Profile"]
	profiles.Records = map[storage.ID]storage.Record{"00e000000000002": {ID: "00e000000000002", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Community API Reader"), "UserLicenseId": storage.IDValue("100000000000001"), "PermissionsApiEnabled": storage.BooleanValue(true)}}}
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
	org.Objects["FieldPermissions"] = fields
	return org
}
