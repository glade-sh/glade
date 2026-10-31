package server

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// The serialization observation is captured at both Apex versions in
// ../apextest/testdata/conformance/assertion_map_order/org.tsv.
func TestJSONNullKeyHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, host := range []string{"59.0", "67.0"} {
			t.Run(api+"/host="+host, func(t *testing.T) {
				root := t.TempDir()
				writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
				classes := map[string]string{
					"NullKeyHost": `public class NullKeyHost {
      @AuraEnabled(cacheable=true) public static String observe() {
       Map<String,Object> m = new Map<String,Object>(); m.put(null,1);
       try { return JSON.serialize(m); } catch (Exception e) {
        Boolean scopeOk = JSON.serialize(m, true) == '{"null":1}' && JSON.serialize(m, false) == '{"null":1}';
        return e.getTypeName() + '|' + e.getMessage() + (scopeOk ? '|scope-ok' : '|scope-bad');
       }
      }
      public String getResult() { return observe(); }
    }`,
					"NullKeyResource": `@RestResource(urlMapping='/null-key') global class NullKeyResource {
      @HttpGet global static String getIt() { return NullKeyHost.observe(); }
    }`,
				}
				for name, source := range classes {
					path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
					writeServerTestFile(t, path, source)
					writeServerTestFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				}
				writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/NullKey.page"), `<apex:page controller="NullKeyHost"><apex:outputText value="{!result}"/></apex:page>`)
				writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/NullKey.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+host+`</apiVersion><label>Null Key</label></ApexPage>`)
				bundle := filepath.Join(root, "force-app/main/default/lwc/nullKey")
				writeServerTestFile(t, filepath.Join(bundle, "nullKey.js"), `import observe from '@salesforce/apex/NullKeyHost.observe'; export default observe;`)
				writeServerTestFile(t, filepath.Join(bundle, "nullKey.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+host+`</apiVersion><isExposed>true</isExposed></LightningComponentBundle>`)
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
				org.APIVersion = api
				srv := newVisualforceHTMLTestServer(t, &org, source)
				srv.SetProjectIndex(typesys.Build(p, schema))
				if srv.runtimeErr != nil {
					t.Fatal(srv.runtimeErr)
				}
				for _, route := range []struct{ name, method, path, body string }{
					{"rest", "GET", "/services/apexrest/null-key", ""},
					{"lightning_imperative", "POST", "/lightning/apex/NullKeyHost/observe", `{}`},
					{"lightning_wire", "POST", "/lightning/wire/apex", `{"className":"NullKeyHost","method":"observe","params":{},"cacheable":true}`},
					{"visualforce", "GET", "/apex/NullKey", ""},
				} {
					t.Run(route.name, func(t *testing.T) {
						for _, phase := range []string{"cold", "warm"} {
							req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
							req.Header.Set("Content-Type", "application/json")
							rec := httptest.NewRecorder()
							srv.ServeHTTP(rec, req)
							if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Unable to serialize a map with a null key.") || !strings.Contains(rec.Body.String(), "JSONException") || !strings.Contains(rec.Body.String(), "scope-ok") {
								t.Fatalf("%s status=%d body=%s", phase, rec.Code, rec.Body.String())
							}
						}
					})
				}
			})
		}
	}
}
