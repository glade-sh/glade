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
	"github.com/glade-sh/glade/internal/resource"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestComponentTabIdentityApexHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/tabs/RouteTab.tab-meta.xml"), `<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata"><label>Route Tab</label><lwcComponent>routePanel</lwcComponent></CustomTab>`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/applications/RouteApp.app-meta.xml"), `<CustomApplication xmlns="http://soap.sforce.com/2006/04/metadata"><label>Route App</label><tabs>RouteTab</tabs></CustomApplication>`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/classes/RouteController.cls"), `public class RouteController {
                @AuraEnabled public static String tabToken() {
                    for(Schema.DescribeTabSetResult a:Schema.describeTabs()) for(Schema.DescribeTabResult b:a.getTabs()) if(b.getName()=='RouteTab') return b.getSObjectName();
                    return null;
                }
                public String getToken() { return tabToken(); }
            }`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/classes/RouteController.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/pages/RoutePage.page"), `<apex:page controller="RouteController">{!token}</apex:page>`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/pages/RoutePage.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><label>Route Page</label></ApexPage>`)
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
			if err := resource.ApplyProject(&org, p); err != nil {
				t.Fatal(err)
			}
			handler := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, handler)
			handler.SetProjectIndex(typesys.Build(p, schema))
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/lightning/apex/RouteController/tabToken", strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("Lightning status %d: %s", rec.Code, rec.Body.String())
			}
			var out lwcbrowser.WireResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			token, ok := out.Data.(string)
			if out.Error != nil || !ok || len(token) != 15 || !strings.HasPrefix(token, "0Rb") {
				t.Fatalf("Lightning result %#v error %#v", out.Data, out.Error)
			}
			page := httptest.NewRecorder()
			handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/apex/RoutePage", nil))
			if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), token) {
				t.Fatalf("Visualforce status %d body=%s want %s", page.Code, page.Body.String(), token)
			}
		})
	}
}
