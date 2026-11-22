package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// No native Lightning row covers this projection; preserve origin/main.
func TestPartnerLeadLightningUncapturedProjection(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "visualforce", "testdata", "partner_lead_named_access", "project", "classes", "RouteProjection67.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		api := api
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			body := strings.Replace(string(source), "public String getObservation()", "@AuraEnabled(cacheable=true) public static String observe()", 1)
			if body == string(source) {
				t.Fatal("missing controller method")
			}
			dir := filepath.Join(root, "force-app/main/default/classes")
			writeServerTestFile(t, filepath.Join(dir, "RouteProjection67.cls"), body)
			writeServerTestFile(t, filepath.Join(dir, "RouteProjection67.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			sch, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			idx := typesys.Build(p, sch)
			if idx.HasErrors() {
				t.Fatal(idx.Diagnostics)
			}
			metadata, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := partnerLightningScopeOrg(api)
			host := newLightningTestServer(t, &org, metadata)
			host.SetProjectIndex(idx)
			for _, route := range []struct{ name, path, body string }{{"imperative", "/lightning/apex/RouteProjection67/observe", `{}`}, {"wire", "/lightning/wire/apex", `{"className":"RouteProjection67","method":"observe","params":{}}`}} {
				route := route
				t.Run(route.name, func(t *testing.T) {
					req := httptest.NewRequest(http.MethodPost, route.path, strings.NewReader(route.body))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("X-GLADE-User-Id", "005000000000001")
					rec := httptest.NewRecorder()
					host.ServeHTTP(rec, req)
					want := "first=0:missing:missing|second=0:missing:missing"
					if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
						t.Fatalf("HTTP %d body=%s, want %s", rec.Code, rec.Body.String(), want)
					}
				})
			}
		})
	}
}
func partnerLightningScopeOrg(api string) storage.OrgState {
	org := storage.NewOrgState()
	org.APIVersion = api
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, "Communities"})
	for _, name := range []string{"User", "Profile", "Lead", "LeadShare", "PermissionSet", "PermissionSetAssignment", "ObjectPermissions"} {
		storage.EnsureStandardObject(&org, name)
	}
	profiles := org.Objects["Profile"]
	profiles.Records = map[storage.ID]storage.Record{"00e000000000001": {ID: "00e000000000001", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("System Administrator")}}, "00e000000000002": {ID: "00e000000000002", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Partner Community User")}}}
	org.Objects["Profile"] = profiles
	users := org.Objects["User"]
	users.Records = map[storage.ID]storage.Record{"005000000000001": {ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000001"), "UserType": storage.StringValue("Standard")}}, "005000000000002": {ID: "005000000000002", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000002"), "Alias": storage.StringValue("rtprt"), "UserType": storage.StringValue("PowerPartner")}}, "005000000000003": {ID: "005000000000003", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000001"), "UserType": storage.StringValue("Standard")}}}
	org.Objects["User"] = users
	leads := org.Objects["Lead"]
	leads.Definition.SharingModel = "ReadWriteTransfer"
	leads.Records = map[storage.ID]storage.Record{"00Q000000000001": {ID: "00Q000000000001", Object: "Lead", System: storage.SystemFields{OwnerID: "005000000000003", CreatedByID: "005000000000001"}, Fields: map[string]storage.Value{"LastName": storage.StringValue("Route First")}}, "00Q000000000002": {ID: "00Q000000000002", Object: "Lead", System: storage.SystemFields{OwnerID: "005000000000003", CreatedByID: "005000000000001"}, Fields: map[string]storage.Value{"LastName": storage.StringValue("Route Second")}}}
	org.Objects["Lead"] = leads
	shares := org.Objects["LeadShare"]
	shares.Records = map[storage.ID]storage.Record{"00T000000000001": {ID: "00T000000000001", Object: "LeadShare", Fields: map[string]storage.Value{"LeadId": storage.IDValue("00Q000000000001"), "UserOrGroupId": storage.IDValue("005000000000002"), "LeadAccessLevel": storage.StringValue("Read"), "RowCause": storage.StringValue("Manual")}}}
	org.Objects["LeadShare"] = shares
	sets := org.Objects["PermissionSet"]
	sets.Records = map[storage.ID]storage.Record{"0PS000000000001": {ID: "0PS000000000001", Object: "PermissionSet"}}
	org.Objects["PermissionSet"] = sets
	assign := org.Objects["PermissionSetAssignment"]
	assign.Records = map[storage.ID]storage.Record{"0Pa000000000001": {ID: "0Pa000000000001", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue("005000000000002"), "PermissionSetId": storage.IDValue("0PS000000000001")}}}
	org.Objects["PermissionSetAssignment"] = assign
	perms := org.Objects["ObjectPermissions"]
	perms.Records = map[storage.ID]storage.Record{"110000000000001": {ID: "110000000000001", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("0PS000000000001"), "SObjectType": storage.StringValue("Lead"), "PermissionsRead": storage.BooleanValue(true)}}}
	org.Objects["ObjectPermissions"] = perms
	return org
}
