package server

import (
	"github.com/glade-sh/glade/internal/storage"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFlowDirectInsertInertProvenanceServer(t *testing.T) {
	for _, version := range []string{"62.0", "67.0"} {
		for _, mode := range []string{"noop", "unmatched", "update", "mutating"} {
			t.Run(version+"/"+mode, func(t *testing.T) {
				org := testOrg()
				storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
				storage.EnsureDeterministicPlatformData(&org)
				account := org.Objects["Account"]
				account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000101", DeveloperName: "Business", Active: true, Available: true, Default: true}, {ID: "012000000000103", DeveloperName: "Person", Active: true, Available: true, PersonAccount: true}}
				flow := storage.FlowRule{Name: "Probe", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "Create"}
				if mode == "unmatched" {
					flow.Criteria = []storage.WorkflowCriteriaItem{{Field: "Name", Operation: "equals", Value: "unmatched"}}
					flow.FieldUpdates = []storage.WorkflowFieldUpdate{{Field: "Phone", LiteralValue: "updated"}}
				}
				if mode == "mutating" {
					flow.FieldUpdates = []storage.WorkflowFieldUpdate{{Field: "Phone", LiteralValue: "changed"}}
				}
				if mode == "update" {
					flow.RecordTriggerType = "Update"
					flow.FieldUpdates = []storage.WorkflowFieldUpdate{{Field: "Phone", LiteralValue: "updated"}}
				}
				account.Definition.FlowRules = []storage.FlowRule{flow}
				org.Objects["Account"] = account
				req := httptest.NewRequest(http.MethodPost, "/services/data/v"+version+"/sobjects/Account", strings.NewReader(`{"LastName":null}`))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				New(&org).ServeHTTP(rec, req)
				if version == "62.0" {
					if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "unsupported API version") {
						t.Fatalf("status=%d body=%s, want unsupported REST API version", rec.Code, rec.Body.String())
					}
					return
				}
				if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "REQUIRED_FIELD_MISSING") {
					t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
				}
			})
		}
	}
}
