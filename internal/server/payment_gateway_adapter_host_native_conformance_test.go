package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// NATIVE dispatch ../apextest/testdata/conformance/payment_gateway_adapter/org.tsv:2 anonymous 62.0
// NATIVE dispatch ../apextest/testdata/conformance/payment_gateway_adapter/org.tsv:3 anonymous 67.0
func TestPaymentGatewayAdapterHostNativeConformance(t *testing.T) {
	fixture := filepath.Join("..", "apextest", "testdata", "conformance", "payment_gateway_adapter")
	adapter, err := os.ReadFile(filepath.Join(fixture, "AdapterProbe.cls"))
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(fixture, "dispatch.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "force-app/main/default")
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeServerTestFile(t, filepath.Join(base, "classes/AdapterProbe.cls"), string(adapter))
			meta := `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><status>Active</status></ApexClass>`
			writeServerTestFile(t, filepath.Join(base, "classes/AdapterProbe.cls-meta.xml"), meta)
			writeServerTestFile(t, filepath.Join(base, "classes/AdapterController.cls"), `public class AdapterController {
 private static String run() {
  commercepayments.PaymentGatewayAdapter adapter = new AdapterProbe();
  return adapter.processRequest(null) instanceof commercepayments.CaptureResponse ? 'ok' : 'wrong';
 }
 @AuraEnabled public static String lightning() { return run(); }
 @RemoteAction public static String remote() { return run(); }
}`)
			writeServerTestFile(t, filepath.Join(base, "classes/AdapterController.cls-meta.xml"), meta)
			writeServerTestFile(t, filepath.Join(base, "pages/Remote.page"), `<apex:page controller="AdapterController"></apex:page>`)
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
			srv := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, srv)
			srv.SetProjectIndex(typesys.Build(p, s))
			if srv.runtimeErr != nil {
				t.Fatal(srv.runtimeErr)
			}
			payload, err := json.Marshal(map[string]string{"anonymousBody": string(body)})
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v"+api+"/tooling/executeAnonymous", strings.NewReader(string(payload))))
			if api == "62.0" {
				// HTTP Execute Anonymous admits 65.0-68.0; source API is independent.
				// Local contract: docs/RELEASE_POLICY.md, Salesforce API Versions.
				if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "unsupported API version") {
					t.Fatalf("anonymous API boundary: status %d, body %s", rec.Code, rec.Body.String())
				}
				rec = httptest.NewRecorder()
				srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v67.0/tooling/executeAnonymous", strings.NewReader(string(payload))))
			}
			var result struct {
				Compiled         bool   `json:"compiled"`
				Success          bool   `json:"success"`
				CompileProblem   string `json:"compileProblem"`
				ExceptionMessage string `json:"exceptionMessage"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusOK || !result.Compiled || !result.Success {
				t.Fatalf("anonymous: status %d, result %+v", rec.Code, result)
			}
			rec = httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/lightning/apex/AdapterController/lightning", strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			srv.ServeHTTP(rec, req)
			var wire lwcbrowser.WireResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &wire); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusOK || wire.Error != nil || wire.Data != "ok" {
				t.Fatalf("lightning: status %d, response %+v", rec.Code, wire)
			}
			vf := postVisualforceRemotingWithViewState(t, srv, `[{"action":"AdapterController","method":"remote","data":[],"type":"rpc","tid":1}]`)
			if len(vf) != 1 || !vf[0].Status || vf[0].Result != "ok" {
				t.Fatalf("visualforce: %+v", vf)
			}
		})
	}
}
