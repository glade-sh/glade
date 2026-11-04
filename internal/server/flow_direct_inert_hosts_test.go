package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/visualforce"
)

func TestFlowDirectInertProvenanceHostedRoutes(t *testing.T) {
	for _, route := range []string{"composite", "lightning", "visualforce"} {
		t.Run(route, func(t *testing.T) {
			org := testOrg()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			storage.EnsureDeterministicPlatformData(&org)
			account := org.Objects["Account"]
			account.Definition.Fields["LastName"] = storage.Field{APIName: "LastName", Type: storage.FieldString, Createable: storage.BoolFlag(true)}
			account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000101", DeveloperName: "Business", Active: true, Available: true, Default: true}, {ID: "012000000000103", DeveloperName: "Person", Active: true, Available: true, PersonAccount: true}}
			account.Definition.FlowRules = []storage.FlowRule{{Name: "Probe", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "Create", FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Phone", LiteralValue: "changed"}}}}
			org.Objects["Account"] = account
			var srv *Server
			var path, body string
			switch route {
			case "composite":
				srv = New(&org)
				path = "/services/data/v67.0/composite/sobjects"
				body = `{"allOrNone":false,"records":[{"attributes":{"type":"Account"},"LastName":null}]}`
			case "lightning":
				srv = New(&org)
				path = "/lightning/wire/createRecord"
				body = `{"apiName":"Account","fields":{"LastName":null}}`
			case "visualforce":
				// The connected page currently rejects Account.LastName before
				// DML; exercise the shared Remote Objects dispatcher directly.
				descriptor := visualforce.RemoteObjectsDescriptor{Models: []visualforce.RemoteObjectModelDescriptor{{Name: "Account", Fields: []visualforce.RemoteObjectFieldDescriptor{{Name: "LastName"}}}}}
				result := visualforce.DispatchRemoteObjectCRUD(&org, descriptor, visualforce.RemoteObjectCRUDRequest{Operation: "create", ObjectName: "Account", Fields: map[string]any{"LastName": nil}})
				if result.Success || len(result.Errors) != 1 || result.Errors[0].StatusCode != "REQUIRED_FIELD_MISSING" {
					t.Fatalf("result=%+v", result)
				}
				return
			}
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if !strings.Contains(rec.Body.String(), "REQUIRED_FIELD_MISSING") {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}
