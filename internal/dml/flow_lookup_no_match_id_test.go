package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native rows (scratch org, API 62.0, 67.0 and 68.0): a before-save flow ran
// Get Records (first record, store output automatically, assignNullValues
// false) with no match and then assigned Get.Id to a lookup field. The lookup
// field ended null, including when it held a value before the flow ran, and
// the flow's other assignment still applied. The flow did not fail.
func TestFlowBeforeSaveLookupNoMatchAssignsNullId(t *testing.T) {
	for _, preset := range []bool{false, true} {
		name := "unset"
		if preset {
			name = "preset"
		}
		t.Run(name, func(t *testing.T) {
			org := testOrg()
			account := org.Objects["Account"]
			account.Definition.Fields["Partner__c"] = storage.Field{APIName: "Partner__c", Type: storage.FieldReference, ReferenceTo: []string{"Target__c"}, RelationshipName: "Partner__r"}
			account.Definition.Fields["Description"] = storage.Field{APIName: "Description", Type: storage.FieldString}
			account.Definition.FlowRules = []storage.FlowRule{{
				Name:              "AssignMissingTarget",
				Active:            true,
				TriggerType:       "RecordBeforeSave",
				RecordTriggerType: "CreateAndUpdate",
				Steps: []storage.FlowStep{
					{Kind: "recordLookup", RecordLookup: storage.FlowRecordLookup{
						Name:                     "FindTarget",
						ObjectName:               "Target__c",
						GetFirstRecordOnly:       true,
						StoreOutputAutomatically: true,
						Criteria:                 []storage.WorkflowCriteriaItem{{Field: "Name", Operation: "equals", Value: "No Such Target"}},
					}},
					{Kind: "fieldUpdate", FieldUpdates: []storage.WorkflowFieldUpdate{
						{Name: "SetPartner", Field: "Partner__c", SourceField: "FindTarget.Id"},
						{Name: "SetDescription", Field: "Description", LiteralValue: "flow-set"},
					}},
				},
			}}
			org.Objects["Account"] = account
			org.Objects["Target__c"] = storage.ObjectState{
				Definition: storage.ObjectDefinition{
					APIName:   "Target__c",
					KeyPrefix: "a01",
					Fields:    map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
				},
				Records: map[storage.ID]storage.Record{
					"a01000000000001AAA": {Object: "Target__c", ID: "a01000000000001AAA", Fields: map[string]storage.Value{"Name": storage.StringValue("Existing")}},
				},
			}
			fields := map[string]storage.Value{"Name": storage.StringValue("Acme")}
			if preset {
				fields["Partner__c"] = storage.IDValue("a01000000000001AAA")
			}
			engine := NewEngine(&org)
			records := []storage.Record{{Object: "Account", Fields: fields}}
			if err := engine.ApplyBeforeSaveFlows(records); err != nil {
				t.Fatalf("ApplyBeforeSaveFlows = %v", err)
			}
			stored := records[0]
			if got := stored.Fields["Description"].String; got != "flow-set" {
				t.Fatalf("Description = %q, want flow-set", got)
			}
			if value, ok := stored.Fields["Partner__c"]; ok && value.Kind != storage.ValueNull {
				t.Fatalf("Partner__c = %#v, want null", value)
			}
		})
	}
}
