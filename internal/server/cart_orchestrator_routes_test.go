package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

// The subclass declaration is backed by org.tsv:2,7. These are local host
// route checks; the direct call uses a null receiver and does not invoke a hosted calculator.
func TestCartOrchestratorServerAndLightningCompileRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			dir := filepath.Join(root, "force-app/main/default/classes")
			files := map[string]string{
				"ProbeCtrl.cls": `public class ProbeCtrl extends CartExtension.CartCalculate {
 public override void calculate(CartExtension.CartCalculateOrchestratorRequest r) { shippingPromotions((CartExtension.CartCalculateCalculatorRequest)null); }
 @AuraEnabled public static String ping() { CartExtension.CartCalculate value = null; try { value.shippingPromotions((CartExtension.CartCalculateCalculatorRequest)null); return 'miss'; } catch (NullPointerException expected) { return 'ok'; } }
}`,
				"ProbePageCtrl.cls": `public class ProbePageCtrl {
 public String getValue() { CartExtension.CartCalculate value = null; try { value.shippingPromotions((CartExtension.CartCalculateCalculatorRequest)null); return 'miss'; } catch (NullPointerException expected) { return 'ok'; } }
}`,
				"ProbeRest.cls": `@RestResource(urlMapping='/probe-cart/*') global class ProbeRest extends CartExtension.CartCalculate {
 global override void calculate(CartExtension.CartCalculateOrchestratorRequest r) { shippingPromotions((CartExtension.CartCalculateCalculatorRequest)null); }
 @HttpGet global static String ping() { CartExtension.CartCalculate value = null; try { value.shippingPromotions((CartExtension.CartCalculateCalculatorRequest)null); return 'miss'; } catch (NullPointerException expected) { return 'ok'; } }
}`,
			}
			for name, body := range files {
				writeLightningFixtureFile(t, filepath.Join(dir, name), body)
				writeLightningFixtureFile(t, filepath.Join(dir, name+"-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			}
			page := filepath.Join(root, "force-app/main/default/pages/Probe.page")
			writeLightningFixtureFile(t, page, `<apex:page controller="ProbePageCtrl"><apex:outputText value="{!value}"/></apex:page>`)
			writeLightningFixtureFile(t, page+"-meta.xml", fmt.Sprintf(`<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><label>Probe</label></ApexPage>`, api))
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			objects, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, objects)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if result := sema.Analyze(index); result.HasErrors() {
				t.Fatal(result.Diagnostics)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := testOrg()
			handler := newLightningTestServer(t, &org, source)
			handler.SetProjectIndex(index)
			for _, route := range []struct{ method, path, body string }{
				{http.MethodPost, "/lightning/apex/ProbeCtrl/ping", `{}`},
				{http.MethodGet, "/services/apexrest/probe-cart/1", ""},
			} {
				for i := 0; i < 2; i++ {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
					req.Header.Set("Content-Type", "application/json")
					handler.ServeHTTP(rec, req)
					if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ok") {
						t.Fatalf("%s pass %d: %d %s", route.path, i, rec.Code, rec.Body.String())
					}
				}
			}
			vf := newVisualforceHTMLTestServer(t, &org, source)
			vf.SetProjectIndex(index)
			for i := 0; i < 2; i++ {
				rec := httptest.NewRecorder()
				vf.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/Probe", nil))
				if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ok") {
					t.Fatalf("VF pass %d: %d %s", i, rec.Code, rec.Body.String())
				}
			}
		})
	}
}
