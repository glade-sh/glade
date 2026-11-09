package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Supplemental host routes for b3_auto_response/active native lead_auto_true.
func TestB3AutoResponseHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("../apextest/testdata/conformance/b3_auto_response/project")); err != nil {
				t.Fatal(err)
			}
			if err := os.CopyFS(filepath.Join(root, "force-app/main/default"), os.DirFS("../apextest/testdata/conformance/b3_auto_response/assignment-metadata")); err != nil {
				t.Fatal(err)
			}
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/B3AssignmentEmail.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page"), `<apex:page controller="B3HostController"></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/B3HostController.cls"), `public class B3HostController { @AuraEnabled @RemoteAction public static String run() { String observed=B3AssignmentEmail.run();Database.DMLOptions a=new Database.DMLOptions();a.EmailHeader.triggerUserEmail=true;Account account=new Account(Name='Synthetic');account.setOptions(a);insert account;Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.triggerOtherEmail=true;Database.SaveResult r=Database.insert(new Case(Subject='Synthetic',Status='New',Origin='Phone'),o);return observed+'|'+(account.Id!=null)+'|'+r.isSuccess()+'|'+r.getErrors()[0].getStatusCode(); } }`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/B3HostController.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
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
			storage.EnsureDeterministicPlatformData(&org)
			for _, name := range []string{"Lead", "Case", "Task", "Contact"} {
				storage.EnsureStandardObject(&org, name)
			}
			if err := resource.ApplyProject(&org, p); err != nil {
				t.Fatal(err)
			}
			if err := resource.ApplyProjectQueues(&org, p); err != nil {
				t.Fatal(err)
			}
			policies, err := automation.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			automation.ApplyToOrg(&org, policies)
			newHost := func() *Server {
				copy := org.Clone()
				srv := newLightningTestServer(t, &copy, source)
				configureVisualforceTestPrincipal(t, srv)
				srv.SetProjectIndex(typesys.Build(p, schema))
				if srv.runtimeErr != nil {
					t.Fatal(srv.runtimeErr)
				}
				return srv
			}
			t.Run("vf", func(t *testing.T) {
				srv := newHost()
				responses := postVisualforceRemotingWithViewState(t, srv, `[{"action":"B3HostController","method":"run","data":[],"type":"rpc","tid":3}]`)
				if len(responses) != 1 || !responses[0].Status || responses[0].Result != "lead_assigned=true,true,1|case_assigned=true,true,1|true|false|FIELD_INTEGRITY_EXCEPTION" {
					t.Fatalf("remoting: %+v", responses)
				}
			})
			t.Run("lwc", func(t *testing.T) {
				srv := newHost()
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(`{"className":"B3HostController","method":"run","params":{}}`))
				req.Header.Set("Content-Type", "application/json")
				srv.ServeHTTP(rec, req)
				var body struct {
					Data  any `json:"data"`
					Error any `json:"error"`
				}
				if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &body) != nil || body.Error != nil || body.Data != "lead_assigned=true,true,1|case_assigned=true,true,1|true|false|FIELD_INTEGRITY_EXCEPTION" {
					t.Fatalf("wire: %s", rec.Body.String())
				}
			})
			t.Run("soap", func(t *testing.T) {
				srv := newHost()
				body := soapEnvelope(`<apex:executeAnonymous><apex:String>Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.triggerAutoResponseEmail=true;Lead l=new Lead(LastName='Synthetic',Company='Synthetic',Email='synthetic@example.invalid');Database.SaveResult r=Database.insert(l,o);System.assertEquals(true,r.isSuccess());Id recordId=r.Id;System.assertEquals(1,[SELECT COUNT() FROM Task WHERE WhoId=:recordId]);</apex:String></apex:executeAnonymous>`)
				rec := httptest.NewRecorder()
				srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, strings.Replace(soapApexTestPath, "65.0", api, 1), strings.NewReader(body)))
				if api == "62.0" {
					if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "unsupported API version 62.0") {
						t.Fatalf("SOAP API 62 boundary: %d %s", rec.Code, rec.Body.String())
					}
					return
				}
				if rec.Code != http.StatusOK {
					t.Fatalf("HTTP %d: %s", rec.Code, rec.Body.String())
				}
				result := decodeSOAPResult(t, rec.Body.String())
				if result["compiled"] != "true" || result["success"] != "true" {
					t.Fatalf("SOAP result: %#v", result)
				}
			})
		})
	}
}
