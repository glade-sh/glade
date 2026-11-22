package server

import (
	"encoding/json"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func explicitCaseDenialOrg(api string) storage.OrgState {
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

func TestCommunityCaseExplicitDeleteDenialNeighbor(t *testing.T) {
	root := t.TempDir()
	fixture := "testdata/community_case_rest/project"
	if err := filepath.WalkDir(fixture, func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if e.Name() == "RouteCaseRest67.cls" {
			data = []byte(strings.Replace(string(data), "try{delete as user c;", "c.Description='extra';try{delete as user c;", 1))
		}
		rel, err := filepath.Rel(fixture, path)
		if err != nil {
			return err
		}
		target := filepath.Join(root, rel)
		if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
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
	idx := typesys.Build(p, sch)
	org := explicitCaseDenialOrg("67.0")
	objects := org.Objects["ObjectPermissions"]
	objects.Records["110000000000002"] = storage.Record{ID: "110000000000002", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("00e000000000002"), "SObjectType": storage.StringValue("Case"), "PermissionsDelete": storage.BooleanValue(false)}}
	org.Objects["ObjectPermissions"] = objects
	host := New(&org)
	host.SetProjectIndex(idx)
	req := httptest.NewRequest(http.MethodPost, "/services/apexrest/RouteCaseRest67/", strings.NewReader("{}"))
	req.Header.Set("X-GLADE-User-Id", "005000000000002")
	rec := httptest.NewRecorder()
	host.ServeHTTP(rec, req)
	var got string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("HTTP=%d body=%s err=%v", rec.Code, rec.Body.String(), err)
	}
	if !strings.Contains(got, "delete=System.SecurityException") {
		t.Fatalf("expected explicit profile denial, got %q", got)
	}
}
