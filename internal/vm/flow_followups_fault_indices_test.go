package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native system-mode controls: flow_followups_fault_indices/org.tsv.
func TestFlowFollowupsActionFaultNativeIndices(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name, records string
			count         int
		}{
			{"singleton", "new Account(Name = 'throw')", 1},
			{"bulk", "new Account(Name = 'value'), new Account(Name = 'throw'), new Account(Name = 'value')", 3},
			{"two", "new Account(Name = 'value'), new Account(Name = 'throw')", 2},
			{"limit", strings.Repeat("new Account(Name = 'value'), ", 100) + "new Account(Name = 'throw'), " + strings.TrimSuffix(strings.Repeat("new Account(Name = 'value'), ", 99), ", "), 200},
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
    System.assertEquals(%d, failure.getNumDml());
    for (Integer i = 0; i < failure.getNumDml(); i++) {
        System.assertEquals(i, failure.getDmlIndex(i));
        System.assertEquals('FIELD_CUSTOM_VALIDATION_EXCEPTION', failure.getDmlStatusCode(i));
        System.assertEquals(true, failure.getDmlMessage(i).contains('synthetic action fault'));
    }
}
System.assertEquals(true, caught);
System.assertEquals(%d, FaultBoundaryAction.InputCount);
`, tc.records, tc.count, tc.count), CompileOptions{APIVersion: api})
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
