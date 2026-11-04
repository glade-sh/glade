package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/visualforce"
)

// Exercise the HTTP Remote Objects route with an authenticated synthetic view
// state; the native DML controls are in case2692_direct_flow/org.tsv:11-12.
func TestFlowDirectVisualforceHTTPProvenance(t *testing.T) {
	for _, api := range []string{"59.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			srv := newVisualforceRemotingFixtureServer(t, "RemoteObjects.page", `<apex:page><apex:remoteObjects><apex:remoteObjectModel name="Account" fields="Id,LastName"/></apex:remoteObjects></apex:page>`, "")
			projectRoot := srv.Source.Project.Root
			writeServerTestFile(t, filepath.Join(projectRoot, "force-app/main/default/objects/Account/Account.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Account</label><pluralLabel>Accounts</pluralLabel><fields><fullName>LastName</fullName><label>Last Name</label><type>Text</type></fields></CustomObject>`)
			p, err := project.Load(projectRoot)
			if err != nil {
				t.Fatal(err)
			}
			srv.Source.Project = p
			page := filepath.Join(projectRoot, "force-app/main/default/pages/RemoteObjects.page-meta.xml")
			writeServerTestFile(t, page, fmt.Sprintf(`<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><label>RemoteObjects</label></ApexPage>`, api))
			org := testOrg()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			storage.EnsureDeterministicPlatformData(&org)
			account := org.Objects["Account"]
			account.Definition.Fields["LastName"] = storage.Field{APIName: "LastName", Type: storage.FieldString, Createable: storage.BoolFlag(true)}
			account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000101", DeveloperName: "Business", Active: true, Available: true, Default: true}, {ID: "012000000000103", DeveloperName: "Person", Active: true, Available: true, PersonAccount: true}}
			account.Definition.FlowRules = []storage.FlowRule{{Name: "Probe", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "Create", FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Phone", LiteralValue: "changed"}}}}
			org.Objects["Account"] = account
			srv.Org = &org
			secret, err := srv.visualforceViewStateSecretBytes()
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := visualforce.EncodeViewState(visualforce.ViewStatePayload{PageName: "RemoteObjects", CSRF: "synthetic-token"}, secret)
			if err != nil {
				t.Fatal(err)
			}
			request := visualforce.RemoteObjectCRUDRequest{Operation: "create", ObjectName: "Account", Fields: map[string]any{"LastName": nil}, ViewState: encoded, CSRF: "synthetic-token"}
			data, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/apex/RemoteObjects/remoteObjects", strings.NewReader(string(data)))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "REQUIRED_FIELD_MISSING") {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}
