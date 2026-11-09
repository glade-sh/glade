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
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Synthetic host-route twins of csnpe_capture/org.tsv:3,9,38,21,27,40.
func csnpeHostServer(t *testing.T, api string) *Server {
	t.Helper()
	root := t.TempDir()
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
	base := filepath.Join(root, "force-app/main/default")
	writeServerTestFile(t, filepath.Join(base, "objects/ProbeList__c/ProbeList__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Probe List</label><customSettingsType>List</customSettingsType><visibility>Public</visibility></CustomObject>`)
	writeServerTestFile(t, filepath.Join(base, "objects/ProbeList__c/fields/Text__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Text__c</fullName><label>Text</label><length>80</length><type>Text</type></CustomField>`)
	writeServerTestFile(t, filepath.Join(base, "namedCredentials/ProbeEndpoint.namedCredential-meta.xml"), `<NamedCredential xmlns="http://soap.sforce.com/2006/04/metadata"><label>Probe endpoint</label><endpoint>https://example.invalid</endpoint><protocol>NoAuthentication</protocol><principalType>Anonymous</principalType></NamedCredential>`)
	writeServerTestFile(t, filepath.Join(base, "classes/ProbeAdapter.cls"), `global class ProbeAdapter implements commercepayments.PaymentGatewayAdapter { global commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) { return null; } }`)
	writeServerTestFile(t, filepath.Join(base, "paymentGatewayProviders/ProbeGateway.paymentGatewayProvider-meta.xml"), `<PaymentGatewayProvider xmlns="http://soap.sforce.com/2006/04/metadata"><apexAdapter>ProbeAdapter</apexAdapter><masterLabel>Probe Gateway</masterLabel><idempotencySupported>Yes</idempotencySupported></PaymentGatewayProvider>`)
	writeServerTestFile(t, filepath.Join(base, "classes/AjaxController.cls"), `public class AjaxController {
 private static String value() {
  String message = 'no exception';
  try { ProbeList__c setting = ProbeList__c.getValues('absent'); String ignored = setting.Text__c; }
  catch (NullPointerException e) { message = e.getMessage(); }
  Integer credentials = [SELECT count() FROM NamedCredential WHERE DeveloperName='ProbeEndpoint'];
  Integer providers = [SELECT count() FROM PaymentGatewayProvider WHERE DeveloperName='ProbeGateway'];
  return message+'|'+credentials+'|'+providers;
 }
 @AuraEnabled public static String lightning() { return value(); }
 @RemoteAction public static String remote() { return value(); }
}`)
	writeServerTestFile(t, filepath.Join(base, "classes/AjaxController.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
	writeServerTestFile(t, filepath.Join(base, "pages/Remote.page"), `<apex:page controller="AjaxController"></apex:page>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
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
	srv := newLightningTestServer(t, &org, source)
	configureVisualforceTestPrincipal(t, srv)
	// The adapter source is consumed by the metadata loader above. Only the
	// controller needs to be compiled for these request routes.
	controllerProject := p
	controllerProject.ApexFiles = []string{filepath.Join(base, "classes/AjaxController.cls")}
	srv.SetProjectIndex(typesys.Build(controllerProject, s))
	if srv.runtimeErr != nil {
		t.Fatal(srv.runtimeErr)
	}
	return srv
}

func TestCsnpeHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			srv := csnpeHostServer(t, api)
			const want = "Attempt to de-reference a null object|1|1"
			for i := 0; i < 2; i++ {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lightning/apex/AjaxController/lightning", strings.NewReader(`{}`))
				req.Header.Set("Content-Type", "application/json")
				srv.ServeHTTP(rec, req)
				var out lwcbrowser.WireResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
					t.Fatal(err)
				}
				if rec.Code != http.StatusOK || out.Error != nil || out.Data != want {
					t.Fatalf("lightning: status %d response %+v, want %q", rec.Code, out, want)
				}
				result := postVisualforceRemotingWithViewState(t, srv, `[{"action":"AjaxController","method":"remote","data":[],"type":"rpc","tid":1}]`)
				if len(result) != 1 || !result[0].Status || result[0].Result != want {
					t.Fatalf("visualforce: %+v, want %q", result, want)
				}
			}
		})
	}
}
