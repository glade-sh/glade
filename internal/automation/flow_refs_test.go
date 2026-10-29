package automation

import "testing"

func TestFlowRefsDecisionNeedsLookupTriggerContexts(t *testing.T) {
	for _, test := range []struct {
		name           string
		process        string
		trigger        string
		leftReference  string
		operation      string
		getFirstRecord bool
		wantRoute      bool
	}{
		{name: "before_save_field", process: "AutoLaunchedFlow", trigger: "RecordBeforeSave", leftReference: "Get_Contact.Title", operation: "Contains", getFirstRecord: true, wantRoute: true},
		{name: "after_save_field", process: "AutoLaunchedFlow", trigger: "RecordAfterSave", leftReference: "Get_Contact.Title", operation: "Contains", getFirstRecord: true, wantRoute: true},
		{name: "autolaunched_field", process: "AutoLaunchedFlow", leftReference: "Get_Contact.Title", operation: "Contains", getFirstRecord: true, wantRoute: true},
		{name: "before_delete_field", process: "AutoLaunchedFlow", trigger: "RecordBeforeDelete", leftReference: "Get_Contact.Title", operation: "Contains", getFirstRecord: true, wantRoute: false},
		{name: "unobserved_field_operator", process: "AutoLaunchedFlow", trigger: "RecordBeforeSave", leftReference: "Get_Contact.Title", operation: "GreaterThan", getFirstRecord: true, wantRoute: false},
		{name: "unobserved_multi_record_field", process: "AutoLaunchedFlow", trigger: "RecordBeforeSave", leftReference: "Get_Contact.Title", operation: "Contains", wantRoute: false},
		{name: "lookup_collection_empty", process: "AutoLaunchedFlow", trigger: "RecordBeforeSave", leftReference: "Get_Contact", operation: "IsEmpty", wantRoute: true},
		{name: "lookup_collection_null", process: "AutoLaunchedFlow", trigger: "RecordBeforeSave", leftReference: "Get_Contact", operation: "IsNull", wantRoute: true},
		{name: "unobserved_lookup_collection_blank", process: "AutoLaunchedFlow", trigger: "RecordBeforeSave", leftReference: "Get_Contact", operation: "IsBlank", wantRoute: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw := flowXML{
				ProcessType: test.process,
				Start:       flowStartXML{TriggerType: test.trigger},
				Decisions: []flowDecisionXML{{Rules: []flowDecisionRuleXML{{Conditions: []flowConditionXML{{
					LeftValueReference: test.leftReference,
					Operator:           test.operation,
				}}}}}},
				RecordLookups: []flowRecordLookupXML{{
					Name:                     "Get_Contact",
					GetFirstRecordOnly:       test.getFirstRecord,
					StoreOutputAutomatically: true,
				}},
			}
			if got := flowRefsDecisionNeedsLookup(raw); got != test.wantRoute {
				t.Fatalf("flowRefsDecisionNeedsLookup() = %v, want %v", got, test.wantRoute)
			}
		})
	}
}
