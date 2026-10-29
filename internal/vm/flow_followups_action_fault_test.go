package vm

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native flow_followups AO11/AO12 capture this fault connector as a catchable
// FIELD_CUSTOM_VALIDATION_EXCEPTION for one-record and three-record insert lists.
func TestFlowFollowupsActionFaultStatementInsert(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name, records string
			count         int
		}{
			{"singleton", "new Account(Name = 'throw')", 1},
			{"bulk", "new Account(Name = 'value'), new Account(Name = 'throw'), new Account(Name = 'value')", 3},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				org := testDataOrg()
				org.APIVersion = api
				object := org.Objects["Account"]
				object.Definition.FlowRules = []storage.FlowRule{{
					Name: "FaultBoundary", APIVersion: api, Active: true,
					TriggerType: "RecordAfterSave", RecordTriggerType: "Create",
					Steps: []storage.FlowStep{{
						Kind:        "action",
						Action:      storage.FlowAction{Name: "Reject", ActionType: "apex", ClassName: "FaultBoundaryAction", Inputs: []storage.WorkflowFieldUpdate{{Name: "records", SourceField: "$Record"}}},
						FaultBranch: []storage.FlowStep{{Kind: "customError", CustomError: storage.FlowCustomError{Name: "Action_fault", Messages: []storage.FlowCustomErrorMessage{{Message: "synthetic action fault"}}}}},
					}},
				}}
				org.Objects["Account"] = object
				machine := New(nil)
				machine.SetOrg(&org)
				machine.EnableTestContext()
				body, err := CompileAnonymousWithOptions(`
InputCount = records.size();
for (Account subject : records) {
    if (subject.Name == 'throw') throw new IllegalArgumentException('synthetic rejection');
}
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterClass(Class{
					Name: "FaultBoundaryAction", APIVersion: api,
					StaticFields: map[string]Field{"InputCount": {Name: "InputCount", Type: "Integer", Static: true}},
					Methods: map[string]Method{"reject": {
						Name: "FaultBoundaryAction.reject", ClassName: "FaultBoundaryAction", ReturnType: "void",
						IsStatic: true, Access: "public", Modifiers: []string{"InvocableMethod"},
						Params:     []Param{{Name: "records", Type: "List<Account>"}},
						APIVersion: api, SourceContextBound: true, Program: body,
					}},
				}); err != nil {
					t.Fatal(err)
				}
				program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
List<Account> subjects = new List<Account>{%s};
Boolean caught = false;
try {
    insert subjects;
} catch (DmlException failure) {
    caught = true;
    System.assertEquals('FIELD_CUSTOM_VALIDATION_EXCEPTION', failure.getDmlStatusCode(0));
    System.assertEquals(true, failure.getDmlMessage(0).contains('synthetic action fault'));
}
System.assertEquals(true, caught);
System.assertEquals(%d, FaultBoundaryAction.InputCount);
`, tc.records, tc.count), CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestFlowFollowupsStatementErrorRetainsUnrelatedFailure(t *testing.T) {
	machine := New(nil)
	failure := errors.New("unrelated automation failure")
	for _, value := range []Value{Object("Account"), typedList("List<Account>")} {
		if got := machine.flowCoreStatementInsertError("insert", value, failure); got != failure {
			t.Fatalf("unrelated failure was changed: %v", got)
		}
	}
}
