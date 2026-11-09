package server

import (
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

func TestV035ExposedProjectExecution(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "secret")
	t.Setenv("GLADE_SERVER_ALLOWED_HOSTS", "")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			dir := filepath.Join(root, "force-app/main/default")
			writeServerTestFile(t, filepath.Join(dir, "classes/ExposeProbe.cls"), `public class ExposeProbe { public String getValue() { return 'hello'; } @AuraEnabled public static String run() { return 'hello'; } }`)
			writeServerTestFile(t, filepath.Join(dir, "classes/ExposeProbe.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, filepath.Join(dir, "classes/ExposeRest.cls"), `@RestResource(urlMapping='/expose-probe') global class ExposeRest { @HttpGet global static String getValue() { return 'hello'; } }`)
			writeServerTestFile(t, filepath.Join(dir, "classes/ExposeRest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, filepath.Join(dir, "pages/ExposePage.page"), `<apex:page controller="ExposeProbe"><apex:outputText value="{!value}"/></apex:page>`)
			writeServerTestFile(t, filepath.Join(dir, "pages/ExposePage.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><label>Expose</label></ApexPage>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			storage.EnsureDeterministicPlatformData(&org)
			s := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, s)
			s.SetProjectIndex(typesys.Build(p, schema))
			if s.runtimeErr != nil {
				t.Fatal(s.runtimeErr)
			}
			token := case340OwnedToken(t, s)
			h := GuardHTTPAuthority(s, "0.0.0.0:1789", "example.test:1789")
			for _, route := range []struct{ name, method, path, body string }{
				{"vf", "GET", "/apex/ExposePage", ""},
				{"lwc", "POST", "/lightning/apex/ExposeProbe/run", "{}"},
				{"wire", "POST", "/lightning/wire/apex", `{"className":"ExposeProbe","method":"run","params":{}}`},
				{"apex-rest", "GET", "/services/apexrest/expose-probe", ""},
			} {
				t.Run(route.name, func(t *testing.T) {
					for _, authorized := range []bool{false, true, true} {
						req := httptest.NewRequest(route.method, "http://example.test:1789"+route.path, strings.NewReader(route.body))
						req.Header.Set("Content-Type", "application/json")
						req.Header.Set("X-Glade-Client", "1")
						if authorized {
							req.Header.Set("Authorization", "Bearer "+token)
						}
						rec := httptest.NewRecorder()
						h.ServeHTTP(rec, req)
						if !authorized && rec.Code != http.StatusForbidden {
							t.Fatalf("unauthorized %d %s", rec.Code, rec.Body.String())
						}
						if authorized && (rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "hello")) {
							t.Fatalf("authorized %d %s", rec.Code, rec.Body.String())
						}
					}
				})
			}
		})
	}
}
