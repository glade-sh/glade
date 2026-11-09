package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestApprovalHostRoutes(t *testing.T) {
	fixture := filepath.Join("..", "apextest", "testdata", "conformance", "approval_project_capture", "project", "force-app", "main", "default", "approvalProcesses", "Account.Owned_Active.approvalProcess-meta.xml")
	metadata, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/approvalProcesses/Account.Owned_Active.approvalProcess-meta.xml"), string(metadata))
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/ApprovalHost.cls"), `public class ApprovalHost {
   @AuraEnabled public static String run() {
    Account a=new Account(Name='Probe'); insert a;
    Approval.ProcessSubmitRequest r=new Approval.ProcessSubmitRequest();r.setObjectId(a.Id);r.setProcessDefinitionNameOrId('Owned_Active');
    Approval.ProcessResult p=Approval.process(r,false);return String.valueOf(p.isSuccess())+':'+p.getInstanceStatus()+':'+String.valueOf(Approval.isLocked(a.Id));
   }
   public String getOutcome() { return run(); }
  }`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/ApprovalHost.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/ApprovalHost.page"), `<apex:page controller="ApprovalHost" showHeader="false" sidebar="false"><apex:outputText value="{!outcome}"/></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/ApprovalHost.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>59.0</apiVersion><label>Approval host</label></ApexPage>`)
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
			index := typesys.Build(p, schema)
			org := apextest.OrgFromIndex(index)
			handler := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, handler)
			handler.SetProjectIndex(index)
			if handler.runtimeErr != nil {
				t.Fatal(handler.runtimeErr)
			}
			for _, phase := range []string{"cold", "warm"} {
				t.Run(phase, func(t *testing.T) {
					rec := httptest.NewRecorder()
					handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/ApprovalHost", nil))
					if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), ">true:Pending:true<") {
						t.Fatalf("VF status=%d body=%s", rec.Code, rec.Body.String())
					}
					rec = httptest.NewRecorder()
					req := httptest.NewRequest(http.MethodPost, "/lightning/apex/ApprovalHost/run", strings.NewReader(`{}`))
					req.Header.Set("Content-Type", "application/json")
					handler.ServeHTTP(rec, req)
					var out lwcbrowser.WireResponse
					if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
						t.Fatal(err)
					}
					if rec.Code != http.StatusOK || out.Error != nil || out.Data != "true:Pending:true" {
						t.Fatalf("LWC status=%d response=%+v", rec.Code, out)
					}
				})
			}
		})
	}
}
