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
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native order read acceptance with the setting on: standard_enhanced_commerce_orders/org.tsv:12,14,36,38.
func TestEnhancedOrdersHostedReadRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			definition, err := os.ReadFile("../apextest/testdata/conformance/standard_enhanced_commerce_orders/definitions/setting-only.json")
			if err != nil {
				t.Fatal(err)
			}
			writeLightningFixtureFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(definition))
			ctrl := `public class OwnedOrdersCtrl {
    @AuraEnabled public static String ping() { Order o = null; OrderItem i = null; if (o != null) return String.valueOf(o.OrderedDate); if (i != null) return String.valueOf(i.TotalLineAmount); return 'ok'; }
    public String getValue() { return ping(); }
   }`
			path := filepath.Join(root, "force-app/main/default/classes/OwnedOrdersCtrl.cls")
			writeLightningFixtureFile(t, path, ctrl)
			writeLightningFixtureFile(t, path+"-meta.xml", fmt.Sprintf(`<ApexClass><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			page := filepath.Join(root, "force-app/main/default/pages/OwnedOrders.page")
			writeLightningFixtureFile(t, page, `<apex:page controller="OwnedOrdersCtrl"><apex:outputText value="{!value}"/></apex:page>`)
			writeLightningFixtureFile(t, page+"-meta.xml", fmt.Sprintf(`<ApexPage><apiVersion>%s</apiVersion><label>OwnedOrders</label></ApexPage>`, api))
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			objects, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, objects)
			if result := sema.Analyze(index); index.HasErrors() || result.HasErrors() {
				t.Fatalf("compile: %v %v", index.Diagnostics, result.Diagnostics)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := testOrg()
			lightning := newLightningTestServer(t, &org, source)
			lightning.SetProjectIndex(index)
			vf := newVisualforceHTMLTestServer(t, &org, source)
			vf.SetProjectIndex(index)
			for pass := 0; pass < 2; pass++ {
				for _, route := range []struct {
					h                  http.Handler
					method, path, body string
				}{
					{lightning, http.MethodPost, "/lightning/apex/OwnedOrdersCtrl/ping", `{}`},
					{vf, http.MethodGet, "/apex/OwnedOrders", ""},
				} {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
					req.Header.Set("Content-Type", "application/json")
					route.h.ServeHTTP(rec, req)
					if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ok") {
						t.Fatalf("%s pass %d: %d %s", route.path, pass, rec.Code, rec.Body.String())
					}
				}
			}
		})
	}
}

// The disabled setting rejects both reads at API 62 and 67 (native
// standard_enhanced_commerce_orders/org.tsv:4,6,28,30).
func TestEnhancedOrdersToolingDisabledReadRoutes(t *testing.T) {
	for _, api := range []string{"67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			definition, err := os.ReadFile("../apextest/testdata/conformance/standard_enhanced_commerce_orders/definitions/feature-only.json")
			if err != nil {
				t.Fatal(err)
			}
			writeLightningFixtureFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(definition))
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
			org := testOrg()
			handler := newLightningTestServer(t, &org, source)
			handler.SetProjectIndex(index)
			for _, body := range []string{`{"anonymousBody":"Order o; Object result=o.OrderedDate;"}`, `{"anonymousBody":"OrderItem i; Object result=i.TotalLineAmount;"}`} {
				for pass := 0; pass < 2; pass++ {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(http.MethodPost, serverTestDataPath+"/tooling/executeAnonymous", strings.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					handler.ServeHTTP(rec, req)
					if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"compiled":false`) || !strings.Contains(rec.Body.String(), "Variable does not exist") {
						t.Fatalf("pass %d: %d %s", pass, rec.Code, rec.Body.String())
					}
				}
			}
		})
	}
}
