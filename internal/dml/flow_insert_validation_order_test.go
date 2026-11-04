package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Preserve the pre-Flow Engine.Insert rejection when a direct input contains
// a non-createable field; this is a base comparison, not a native Flow claim.
func TestDirectInsertWriteabilityPrecedesRejectingFlow(t *testing.T) {
	for _, tc := range []struct{ object, field string }{{"User", "Name"}, {"Account", "Locked__c"}, {"Account", "OtherLocked__c"}} {
		t.Run(tc.object+"/"+tc.field, func(t *testing.T) {
			org := testOrg()
			storage.EnsureStandardObject(&org, "User")
			object := org.Objects[tc.object]
			if tc.object == "Account" {
				object.Definition.Fields[tc.field] = storage.Field{APIName: tc.field, Type: storage.FieldString, Createable: storage.BoolFlag(false)}
			}
			object.Definition.FlowRules = []storage.FlowRule{{Name: "Reject", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "Create", Steps: []storage.FlowStep{{Kind: "customError", CustomError: storage.FlowCustomError{Name: "Reject", Messages: []storage.FlowCustomErrorMessage{{Message: "flow rejected"}}}}}}}
			org.Objects[tc.object] = object
			engine := NewEngine(&org)
			result := engine.Insert([]storage.Record{{Object: tc.object, Fields: map[string]storage.Value{tc.field: storage.StringValue("explicit")}}})[0]
			if result.StatusCode != "INVALID_FIELD_FOR_INSERT_UPDATE" {
				t.Fatalf("result=%+v, want pre-Flow writeability error", result)
			}
		})
	}
}
