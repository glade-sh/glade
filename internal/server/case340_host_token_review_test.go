package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestCase340HostedUserMutationRevokesOwnedToken(t *testing.T) {
	for _, route := range []string{"vf", "lwc"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			class := filepath.Join(root, "force-app/main/default/classes/SyntheticCtrl.cls")
			writeLightningFixtureFile(t, class, `public class SyntheticCtrl { @AuraEnabled public static String changeUser() { User u = [SELECT Id, LastName FROM User WHERE Id = '005000000000123']; u.LastName = 'Changed'; update u; return 'ok'; } public String getValue() { return changeUser(); } }`)
			writeLightningFixtureFile(t, class+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
			page := filepath.Join(root, "force-app/main/default/pages/Synthetic.page")
			writeLightningFixtureFile(t, page, `<apex:page controller="SyntheticCtrl"><apex:outputText value="{!value}"/></apex:page>`)
			writeLightningFixtureFile(t, page+"-meta.xml", `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><label>Synthetic</label></ApexPage>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			objects, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			idx := typesys.Build(p, objects)
			if idx.HasErrors() {
				t.Fatal(idx.Diagnostics)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := testOrg()
			storage.EnsureDeterministicPlatformData(&org)
			addUser(&org, "005000000000123", "user@example.test", "user.alias@example.test", "Test User")
			var s *Server
			if route == "vf" {
				s = newVisualforceHTMLTestServer(t, &org, source)
				// This principal updates a different user, so grant it the
				// authority required for that mutation.
				users := s.Org.Objects["User"]
				principal := users.Records[s.VisualforceHTMLUserID]
				principal.Fields["PermissionsManageInternalUsers"] = storage.BooleanValue(true)
				users.Records[s.VisualforceHTMLUserID] = principal
			} else {
				s = newLightningTestServer(t, &org, source)
			}
			s.SetProjectIndex(idx)
			token := case340OwnedTokenFor(t, s, "005000000000123")
			var req *http.Request
			if route == "vf" {
				req = httptest.NewRequest("GET", "/apex/Synthetic", nil)
			} else {
				req = httptest.NewRequest("POST", "/lightning/apex/SyntheticCtrl/changeUser", strings.NewReader(`{}`))
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			s.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ok") {
				t.Fatalf("host status %d: %s", rec.Code, rec.Body.String())
			}
			user := s.Org.Objects["User"].Records["005000000000123"]
			if user.Fields["LastName"].String != "Changed" {
				t.Fatal("hosted user mutation did not persist")
			}
			if _, ok := issuedTokenUserFor(token, s); ok {
				t.Fatal("hosted user mutation retained token")
			}
		})
	}
}
