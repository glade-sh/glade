package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestLightningApexFormulaPermissionUsesRequestUser(t *testing.T) {
	for _, version := range []string{"62.0", "67.0"} {
		t.Run(version, func(t *testing.T) {
			root := t.TempDir()
			writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/classes/Q1Controller.cls"), `public class Q1Controller { @AuraEnabled public static String save() { Boolean has=FeatureManagement.checkPermission('Bypass'); Database.SaveResult r=Database.insert(new Account(Name='q1'),false); return String.valueOf(has)+'|'+String.valueOf(r.isSuccess()); } }`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/classes/Q1Controller.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+version+`</apiVersion><status>Active</status></ApexClass>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := testOrg()
			account := org.Objects["Account"]
			account.Definition.ValidationRules = []storage.ValidationRule{{Name: "Guard", Active: true, ErrorConditionFormula: "NOT($Permission.Bypass)", ErrorMessage: "denied"}}
			org.Objects["Account"] = account
			const granted storage.ID = "005000000000777AAA"
			const denied storage.ID = "005000000000778AAA"
			const set storage.ID = "0PS000000000001"
			const cp storage.ID = "0CP000000000001"
			addUser(&org, granted, "granted@example.invalid", "granted@example.invalid", "Granted")
			addUser(&org, denied, "denied@example.invalid", "denied@example.invalid", "Denied")
			org.Objects["PermissionSet"] = storage.ObjectState{Records: map[storage.ID]storage.Record{set: {ID: set, Fields: map[string]storage.Value{}}}}
			org.Objects["CustomPermission"] = storage.ObjectState{Records: map[storage.ID]storage.Record{cp: {ID: cp, Fields: map[string]storage.Value{"DeveloperName": storage.StringValue("Bypass")}}}}
			org.Objects["SetupEntityAccess"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0SE000000000001": {Fields: map[string]storage.Value{"ParentId": storage.IDValue(set), "SetupEntityId": storage.IDValue(cp)}}}}
			org.Objects["PermissionSetAssignment"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0Pa000000000001": {Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(granted), "PermissionSetId": storage.IDValue(set)}}}}
			handler := newLightningTestServer(t, &org, source)
			handler.SetProjectIndex(typesys.Build(p, schema))
			for _, tc := range []struct {
				id   storage.ID
				want string
			}{{denied, "false|false"}, {granted, "true|true"}} {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lightning/apex/Q1Controller/save", strings.NewReader(`{}`))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-GLADE-User-Id", string(tc.id))
				handler.ServeHTTP(rec, req)
				var body struct {
					Data string `json:"data"`
				}
				err := json.Unmarshal(rec.Body.Bytes(), &body)
				if rec.Code != http.StatusOK || err != nil || body.Data != tc.want {
					t.Errorf("user=%s status=%d body=%s err=%v", tc.id, rec.Code, rec.Body.String(), err)
				}
			}
		})
	}
}
