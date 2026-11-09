package server

import (
	"fmt"
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

// API 62/67 native null insertion and update: standard_webstore_preserve_null/org.tsv:4,7.
func TestWebStoreForcedInsertControllerRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			cfg, err := os.ReadFile("../apextest/testdata/conformance/standard_webstore_options/definition.json")
			if err != nil {
				t.Fatal(err)
			}
			writeLightningFixtureFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(cfg))
			ctrl := `public class OwnedStoreCtrl {
    @AuraEnabled public static String ping() {
     WebStore s = new WebStore(Name='Hosted store'); s.put('OptionsPreserveGuestCartEnabled',null); insert s;
     Boolean inserted=[SELECT OptionsPreserveGuestCartEnabled FROM WebStore WHERE Id=:s.Id].OptionsPreserveGuestCartEnabled;
     s.put('OptionsPreserveGuestCartEnabled',false); update s;
     Boolean updated=[SELECT OptionsPreserveGuestCartEnabled FROM WebStore WHERE Id=:s.Id].OptionsPreserveGuestCartEnabled;
     return String.valueOf(inserted)+':'+String.valueOf(updated);
    }
    public String getValue() { return ping(); }
   }`
			path := filepath.Join(root, "force-app/main/default/classes/OwnedStoreCtrl.cls")
			writeLightningFixtureFile(t, path, ctrl)
			writeLightningFixtureFile(t, path+"-meta.xml", fmt.Sprintf(`<ApexClass><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			page := filepath.Join(root, "force-app/main/default/pages/OwnedStore.page")
			writeLightningFixtureFile(t, page, `<apex:page controller="OwnedStoreCtrl"><apex:outputText value="{!value}"/></apex:page>`)
			writeLightningFixtureFile(t, page+"-meta.xml", fmt.Sprintf(`<ApexPage><apiVersion>%s</apiVersion><label>OwnedStore</label></ApexPage>`, api))
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			objects, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, objects)
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			definition, ok := storage.StandardObjectDefinition("WebStore")
			if !ok {
				t.Fatal("missing WebStore")
			}
			storage.EnsureStandardObjectFieldsForFeatures(&definition, []string{"B2BCommerce", "Communities", "CommerceEnabled"})
			org := testOrg()
			org.Objects["WebStore"] = storage.ObjectState{Definition: definition, Records: map[storage.ID]storage.Record{}}
			lightning := newLightningTestServer(t, &org, source)
			lightning.SetProjectIndex(index)
			vf := newVisualforceHTMLTestServer(t, &org, source)
			vf.SetProjectIndex(index)
			for pass := 0; pass < 2; pass++ {
				for _, route := range []struct {
					h                  http.Handler
					method, path, body string
				}{
					{lightning, http.MethodPost, "/lightning/apex/OwnedStoreCtrl/ping", `{}`}, {vf, http.MethodGet, "/apex/OwnedStore", ""},
				} {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
					req.Header.Set("Content-Type", "application/json")
					route.h.ServeHTTP(rec, req)
					if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "true:false") {
						t.Fatalf("%s pass %d: %d %s", route.path, pass, rec.Code, rec.Body.String())
					}
				}
			}
		})
	}
}
