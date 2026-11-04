package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native direct REST controls: testdata/conformance/case2692_direct_flow/org.tsv.
func TestDirectInsertMutatingFlowPreservesPriorRecordTypeBoundary(t *testing.T) {
	for _, field := range []string{"Name", "Phone", "Description", "RecordTypeId"} {
		t.Run(field, func(t *testing.T) {
			org := testOrg()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			storage.EnsureDeterministicPlatformData(&org)
			account := org.Objects["Account"]
			account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000101", DeveloperName: "Business", Active: true, Available: true, Default: true}, {ID: "012000000000103", DeveloperName: "Person", Active: true, Available: true, PersonAccount: true}}
			value := "updated"
			if field == "RecordTypeId" {
				value = "012000000000101"
			}
			account.Definition.FlowRules = []storage.FlowRule{{Name: "Change", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "Create", FieldUpdates: []storage.WorkflowFieldUpdate{{Field: field, LiteralValue: value}}}}
			org.Objects["Account"] = account
			engine := NewEngine(&org)
			got := engine.Insert([]storage.Record{{Object: "Account", ExplicitNulls: map[string]bool{"LastName": true}}})[0]
			want := "REQUIRED_FIELD_MISSING"
			if field == "Name" {
				want = "INVALID_FIELD_FOR_INSERT_UPDATE"
			}
			if got.StatusCode != want {
				t.Fatalf("got %+v", got)
			}
		})
	}
}
