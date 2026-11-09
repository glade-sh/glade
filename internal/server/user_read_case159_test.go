package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestDefaultUserReadApexHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/classes/ReadController.cls"), `public class ReadController {
                @AuraEnabled public static String readable() { return String.valueOf(Schema.SObjectType.User.isAccessible()); }
                public String getValue() { return readable(); }
            }`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/classes/ReadController.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/pages/ReadPage.page"), `<apex:page controller="ReadController">{!value}</apex:page>`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/pages/ReadPage.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><label>Read Page</label></ApexPage>`)
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
			org := storage.NewOrgState()
			storage.EnsureDeterministicPlatformData(&org)
			id := storage.ID("005000000000777AAA")
			addUser(&org, id, "minimal@example.test", "minimal@example.test", "Synthetic User")
			users := org.Objects["User"]
			user := users.Records[id]
			user.Fields["ProfileId"] = storage.IDValue("00e000000000002")
			users.Records[id] = user
			org.Objects["User"] = users
			handler := newLightningTestServer(t, &org, source)
			handler.VisualforceHTMLUserID = id
			handler.SetProjectIndex(typesys.Build(p, schema))
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/lightning/apex/ReadController/readable", strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-GLADE-User-Id", string(id))
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("Lightning status %d: %s", rec.Code, rec.Body.String())
			}
			var out lwcbrowser.WireResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			if out.Error != nil || out.Data != "true" {
				t.Fatalf("Lightning result %#v error %#v", out.Data, out.Error)
			}
			page := httptest.NewRecorder()
			handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/apex/ReadPage", nil))
			if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "true") {
				t.Fatalf("Visualforce status %d body=%s", page.Code, page.Body.String())
			}
		})
	}
}
