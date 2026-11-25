package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestPartialExternalIDUpsertKeepsRowOrderOnProcessRetry(t *testing.T) {
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Key__c"] = storage.Field{
		APIName: "Key__c", Type: storage.FieldString, Length: 4, ExternalID: true, Unique: true,
	}
	account.Definition.ValidationRules = []storage.ValidationRule{{
		Name:                  "RejectBlockedDescription",
		Active:                true,
		ErrorConditionFormula: `Description = "Blocked"`,
		ErrorMessage:          "Blocked description",
	}}
	account.Definition.FlowRules = []storage.FlowRule{{
		Name:              "SetDescription",
		Active:            true,
		ProcessType:       "Workflow",
		TriggerType:       "RecordAfterSave",
		RecordTriggerType: "Create",
		Criteria:          []storage.WorkflowCriteriaItem{{Field: "Name", Operation: "equals", Value: "Reject"}},
		Steps: []storage.FlowStep{{
			Kind: "recordUpdate",
			RecordUpdate: storage.FlowRecordUpdate{
				Name:             "SetDescription",
				ObjectName:       "Account",
				Criteria:         []storage.WorkflowCriteriaItem{{Field: "Name", Operation: "equals", Value: "Reject"}},
				InputAssignments: []storage.WorkflowFieldUpdate{{Field: "Description", LiteralValue: "Blocked"}},
			},
		}},
	}}
	org.Objects["Account"] = account

	machine := New(nil)
	machine.SetOrg(&org)
	program, err := CompileAnonymous(`
List<Account> rows = new List<Account>{
    new Account(Name = 'Oversized', Key__c = 'long-key'),
    new Account(Name = 'Reject', Key__c = 'bad'),
    new Account(Name = 'Keep', Key__c = 'good')
};
Database.UpsertResult[] results = Database.upsert(rows, Account.Fields.Key__c, false);
System.assertEquals(3, results.size());
System.assertEquals(false, results[0].isSuccess());
System.assertEquals(StatusCode.FIELD_INTEGRITY_EXCEPTION, results[0].getErrors()[0].getStatusCode());
System.assertEquals(false, results[1].isSuccess());
System.assertEquals(StatusCode.CANNOT_EXECUTE_FLOW_TRIGGER, results[1].getErrors()[0].getStatusCode());
System.assertEquals(true, results[2].isSuccess());
System.assertEquals(true, results[2].isCreated());
List<Account> saved = [SELECT Id, Name FROM Account];
System.assertEquals(1, saved.size());
System.assertEquals('Keep', saved[0].Name);
System.assertEquals(saved[0].Id, results[2].getId());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
