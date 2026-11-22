package server

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// NATIVE ../vm/testdata/community_case_anonymous/org.tsv:4-5 anonymous 62.0 67.0.
func TestCommunityCaseAnonymousCompileNativeConformance(t *testing.T) {
	base := filepath.Join("..", "vm", "testdata", "community_case_anonymous")
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
	for _, row := range rows[1:] {
		// The server advertises 67.0; Apex 62.0 has no matching REST route.
		if row[2] != "anonymous" || row[3] != "false" || row[1] != "67.0" {
			continue
		}
		row := row
		for _, route := range []string{"rest", "soap"} {
			for _, variant := range []string{"native", "uncaptured-no-api", "uncaptured-no-origin", "uncaptured-delete-grant"} {
				if row[0] != "case_delete" && variant != "native" {
					continue
				}
				t.Run(row[0]+"/"+row[1]+"/"+route+"/"+variant, func(t *testing.T) {
					org := communityCaseCompileOrg(row[1])
					if variant == "uncaptured-no-api" {
						profiles := org.Objects["Profile"]
						profile := profiles.Records["00e000000000002"]
						profile.Fields["PermissionsApiEnabled"] = storage.BooleanValue(false)
						profiles.Records[profile.ID] = profile
						org.Objects["Profile"] = profiles
					}
					if variant == "uncaptured-no-origin" {
						fields := org.Objects["FieldPermissions"]
						delete(fields.Records, "0FP000000000002")
						org.Objects["FieldPermissions"] = fields
					}
					if variant == "uncaptured-delete-grant" {
						objects := org.Objects["ObjectPermissions"]
						permission := objects.Records["110000000000001"]
						permission.Fields["PermissionsDelete"] = storage.BooleanValue(true)
						objects.Records[permission.ID] = permission
						org.Objects["ObjectPermissions"] = objects
					}
					handler := New(&org)
					name := row[0]
					if name == "case_delete" {
						name = "delete"
					}
					source, err := os.ReadFile(filepath.Join(base, name+".apex"))
					if err != nil {
						t.Fatal(err)
					}
					code := strings.ReplaceAll(string(source), "__API__", strings.TrimSuffix(row[1], ".0"))
					var req *http.Request
					if route == "rest" {
						body, err := json.Marshal(map[string]string{"anonymousBody": code})
						if err != nil {
							t.Fatal(err)
						}
						req = httptest.NewRequest(http.MethodPost, "/services/data/v"+row[1]+"/tooling/executeAnonymous", bytes.NewReader(body))
					} else {
						var escaped bytes.Buffer
						if err := xml.EscapeText(&escaped, []byte(code)); err != nil {
							t.Fatal(err)
						}
						req = httptest.NewRequest(http.MethodPost, "/services/Soap/s/"+row[1]+"/local", strings.NewReader(soapEnvelope("<apex:executeAnonymous><apex:String>"+escaped.String()+"</apex:String></apex:executeAnonymous>")))
					}
					req.Header.Set("X-GLADE-User-Id", "005000000000002")
					rec := httptest.NewRecorder()
					handler.ServeHTTP(rec, req)
					if rec.Code != http.StatusOK {
						t.Fatalf("status=%d", rec.Code)
					}
					if route == "soap" {
						result := decodeSOAPResult(t, rec.Body.String())
						if variant != "native" {
							if result["compiled"] != "true" {
								t.Fatalf("uncaptured compiled=%s", result["compiled"])
							}
							return
						}
						if result["compiled"] != row[3] || result["success"] != row[4] || result["compileProblem"] != row[5] {
							t.Fatalf("compiled=%s success=%s problem=%q; native=%s,%s,%q", result["compiled"], result["success"], result["compileProblem"], row[3], row[4], row[5])
						}
						return
					}
					var result map[string]any
					if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					if variant != "native" {
						if result["compiled"] != true {
							t.Fatalf("uncaptured compiled=%v", result["compiled"])
						}
						return
					}
					if result["compiled"] != false || result["success"] != false || result["compileProblem"] != row[5] {
						t.Fatalf("compiled=%v success=%v problem=%q; native=%s,%s,%q", result["compiled"], result["success"], result["compileProblem"], row[3], row[4], row[5])
					}
				})
			}
		}
	}
}

func communityCaseCompileOrg(api string) storage.OrgState {
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

func TestCommunityCaseCompilePolicyTracksRequestPermission(t *testing.T) {
	org := communityCaseCompileOrg("67.0")
	handler := New(&org)
	source, err := os.ReadFile(filepath.Join("..", "vm", "testdata", "community_case_anonymous", "delete.apex"))
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]string{"anonymousBody": strings.ReplaceAll(string(source), "__API__", "67")})
	if err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{true, false, true} {
		profiles := org.Objects["Profile"]
		profile := profiles.Records["00e000000000002"]
		profile.Fields["PermissionsApiEnabled"] = storage.BooleanValue(enabled)
		profiles.Records[profile.ID] = profile
		org.Objects["Profile"] = profiles
		req := httptest.NewRequest(http.MethodPost, "/services/data/v67.0/tooling/executeAnonymous", bytes.NewReader(body))
		req.Header.Set("X-GLADE-User-Id", "005000000000002")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		var result map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result["compiled"] != !enabled {
			t.Fatalf("API permission=%v compiled=%v", enabled, result["compiled"])
		}
	}
}
