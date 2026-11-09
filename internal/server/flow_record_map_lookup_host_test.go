package server

import (
	"encoding/xml"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Host twins for the captured Flow observations in the record-map and lookup
// conformance fixtures. Each route gets an independent org.
func TestFlowRecordMapLookupHostRoutes(t *testing.T) {
	const operation = `
        Flow.Interview transform=Flow.Interview.createInterview('TransformProbe',new Map<String,Object>{'InputText'=>'Map sample'});
        transform.start(); Contact mapped=(Contact)transform.getVariableValue('OutputRecord');
        System.assertEquals('Map sample',mapped.LastName); System.assertEquals(null,mapped.Title);
        System.assertEquals(true,mapped.getPopulatedFieldsAsMap().containsKey('Title'));
        Account b=new Account(Name='Capture Collection Seed B'); insert b;
        Map<String,Object> populated=new Map<String,Object>{'ids'=>new List<String>{String.valueOf(b.Id)}};
        Flow.Interview excluded=Flow.Interview.createInterview('Capture_Filter_NotIn',populated);
        excluded.start(); System.assertEquals(null,excluded.getVariableValue('outcome'));
        Account a=new Account(Name='Capture Collection Seed A'); insert a;
        for(String op:new List<String>{'In','NotIn'}) {
            for(String mode:new List<String>{'populated','empty','unassigned'}) {
                Map<String,Object> inputs=new Map<String,Object>();
                if(mode=='populated') inputs.put('ids',new List<String>{String.valueOf(b.Id)});
                if(mode=='empty') inputs.put('ids',new List<String>());
                Flow.Interview f=Flow.Interview.createInterview('Capture_Filter_'+op,inputs); f.start();
                String expected=op=='NotIn'?'Capture Collection Seed A':(mode=='populated'?'Capture Collection Seed B':null);
                System.assertEquals(expected,f.getVariableValue('outcome'));
            }
        }
    `
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			for _, route := range []string{"vf", "lwc", "soap"} {
				t.Run(route, func(t *testing.T) {
					root := t.TempDir()
					writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
					dir := "api" + strings.TrimSuffix(api, ".0")
					fixtures := map[string]string{
						"TransformProbe":       filepath.Join("..", "apextest", "testdata", "conformance", "flow_transform_record_map", dir, "TransformProbe.flow-meta.xml"),
						"Capture_Filter_In":    filepath.Join("..", "apextest", "testdata", "conformance", "pr163_flow_lookup_collection", dir, "Capture_Filter_In.flow-meta.xml"),
						"Capture_Filter_NotIn": filepath.Join("..", "apextest", "testdata", "conformance", "pr163_flow_lookup_collection", dir, "Capture_Filter_NotIn.flow-meta.xml"),
					}
					for name, path := range fixtures {
						data, err := os.ReadFile(path)
						if err != nil {
							t.Fatal(err)
						}
						writeServerTestFile(t, filepath.Join(root, "force-app/main/default/flows", name+".flow-meta.xml"), string(data))
					}
					writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/FlowRouteHost.cls"), `public class FlowRouteHost { @AuraEnabled public static String observe(){`+operation+`return 'PASS';} public String getOutput(){return observe();} }`)
					writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/FlowRouteHost.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
					writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/FlowRouteHost.page"), `<apex:page controller="FlowRouteHost"><apex:outputText value="{!output}"/></apex:page>`)
					p, err := project.Load(root)
					if err != nil {
						t.Fatal(err)
					}
					flows, err := automation.LoadProject(p)
					if err != nil || len(flows.Diagnostics) != 0 {
						t.Fatalf("flows: %v %v", err, flows.Diagnostics)
					}
					org := storage.NewOrgState()
					org.APIVersion = api
					automation.ApplyToOrg(&org, flows)
					if route == "soap" {
						handler := New(&org)
						var escaped strings.Builder
						if err := xml.EscapeText(&escaped, []byte(operation)); err != nil {
							t.Fatal(err)
						}
						rec := httptest.NewRecorder()
						handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/Soap/s/"+api+"/example", strings.NewReader(soapEnvelope("<apex:executeAnonymous><apex:String>"+escaped.String()+"</apex:String></apex:executeAnonymous>"))))
						if api == "62.0" {
							if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "sf:INVALID_VERSION") {
								t.Fatalf("SOAP version boundary: %d %s", rec.Code, rec.Body.String())
							}
							return
						}
						if rec.Code != http.StatusOK {
							t.Fatalf("SOAP: %d %s", rec.Code, rec.Body.String())
						}
						result := decodeSOAPResult(t, rec.Body.String())
						if result["compiled"] != "true" || result["success"] != "true" {
							t.Fatalf("SOAP result: %v", result)
						}
						return
					}
					source, err := NewSourceMetadataFromProject(p)
					if err != nil {
						t.Fatal(err)
					}
					schema, err := gladeschema.LoadProject(p)
					if err != nil {
						t.Fatal(err)
					}
					srv := newVisualforceHTMLTestServer(t, &org, source)
					srv.SetProjectIndex(typesys.Build(p, schema))
					if srv.runtimeErr != nil {
						t.Fatal(srv.runtimeErr)
					}
					rec := httptest.NewRecorder()
					if route == "vf" {
						srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/FlowRouteHost", nil))
					} else {
						req := httptest.NewRequest(http.MethodPost, "/lightning/apex/FlowRouteHost/observe", strings.NewReader(`{}`))
						req.Header.Set("Content-Type", "application/json")
						srv.ServeHTTP(rec, req)
					}
					if rec.Code != http.StatusOK || !strings.Contains(html.UnescapeString(rec.Body.String()), "PASS") {
						t.Fatalf("%s: %d %s", route, rec.Code, rec.Body.String())
					}
				})
			}
		})
	}
}
