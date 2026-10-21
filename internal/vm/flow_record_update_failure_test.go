package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func flowRecordUpdateFailureMachine(t *testing.T, steps []storage.FlowStep) *VM {
	t.Helper()
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	org.Metadata.Flows = []storage.FlowRule{{
		Name: "MeadowRecordUpdate", Active: true, ProcessType: "AutoLaunchedFlow",
		Variables: []storage.FlowVariable{
			{Name: "targetId", DataType: "String", IsInput: true},
			{Name: "pendingRecord", DataType: "SObject", ObjectType: "Account", IsInput: true, IsOutput: true},
		},
		Steps: steps,
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{Name: "Flow.Interview"}); err != nil {
		t.Fatal(err)
	}
	return machine
}

func TestFlowInterviewRecordVariableMissingIdentityIsCatchable(t *testing.T) {
	// The owned record-binding captures catch the populated scalar's missing-ID
	// failure. Native anonymous logs identify it as System.FlowException; the
	// exact diagnostic text is deliberately outside these assertions.
	for _, populatedID := range []bool{false, true} {
		name := "missing_id"
		steps := []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{
			Name: "SetDescription", Target: "pendingRecord.Description", Operator: "Assign", LiteralValue: "After",
		}}}
		assertions := `System.assert(caught); System.assertEquals('System.FlowException', failureType); System.assertEquals('Before', stored.Description);`
		if populatedID {
			name = "populated_id_control"
			steps = append(steps, storage.FlowStep{Kind: "assignment", Assignment: storage.FlowAssignment{
				Name: "SetIdentity", Target: "pendingRecord.Id", Operator: "Assign", SourceField: "targetId",
			}})
			assertions = `System.assert(!caught); System.assertEquals(null, failureType); System.assertEquals('After', stored.Description);`
		}
		steps = append(steps, storage.FlowStep{Kind: "recordUpdate", RecordUpdate: storage.FlowRecordUpdate{
			Name: "PersistPending", ObjectName: "Account", InputReference: "pendingRecord",
		}})
		t.Run(name, func(t *testing.T) {
			machine := flowRecordUpdateFailureMachine(t, steps)
			program, err := CompileAnonymous(`
Account target = new Account(Name='Meadow retained name', Description='Before');
insert target;
Flow.Interview interview = Flow.Interview.createInterview('MeadowRecordUpdate', new Map<String,Object>{'targetId'=>String.valueOf(target.Id)});
Boolean caught = false;
String failureType;
try {
    interview.start();
} catch (Exception failure) {
    caught = true;
    failureType = failure.getTypeName();
}
Account stored = [SELECT Name, Description FROM Account WHERE Id=:target.Id];
System.assertEquals('Meadow retained name', stored.Name);
` + assertions)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFlowInterviewUnsupportedStepRetainsExistingFailure(t *testing.T) {
	// Preserve the current local unsupported-step boundary. Missing identity
	// must not turn unrelated engine errors into catchable FlowExceptions.
	machine := flowRecordUpdateFailureMachine(t, []storage.FlowStep{{Kind: "splitLog"}})
	program, err := CompileAnonymous(`
Flow.Interview interview = Flow.Interview.createInterview('MeadowRecordUpdate', new Map<String,Object>());
Boolean caught = false;
try {
    interview.start();
} catch (Exception failure) {
    caught = true;
}
System.assertEquals(false, caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), `flow step kind "splitLog" is not supported`) {
		t.Fatalf("expected preserved unsupported-step failure, got %v", err)
	}
}

func TestFlowInterviewEmptyRecordInputRetainsExistingFailure(t *testing.T) {
	// Explicit empty record inputs were not observed in the native captures.
	// Preserve their existing engine failure instead of applying the exception
	// conversion established for a populated scalar record without an Id.
	machine := flowRecordUpdateFailureMachine(t, []storage.FlowStep{{
		Kind: "recordUpdate", RecordUpdate: storage.FlowRecordUpdate{
			Name: "PersistPending", ObjectName: "Account", InputReference: "pendingRecord",
		},
	}})
	program, err := CompileAnonymous(`
Flow.Interview interview = Flow.Interview.createInterview('MeadowRecordUpdate', new Map<String,Object>{'pendingRecord'=>new Account()});
Boolean caught = false;
try {
    interview.start();
} catch (Exception failure) {
    caught = true;
}
System.assertEquals(false, caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "update requires id") {
		t.Fatalf("expected preserved empty-record update failure, got %v", err)
	}
}
