package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Child steps belong to the shared interpreter, not the top-level action
// signature whitelist. Project routes separately pin the observable effects.
func TestInvocableFlowChildInterpreterAdmission(t *testing.T) {
	for _, shape := range []string{"empty", "field-update", "collection-lookup", "record-input-create", "contact-create", "record-delete", "email-alert", "email-simple", "subtract", "loop", "custom-error", "transform", "collection-processor", "fault-branch", "target"} {
		t.Run(shape, func(t *testing.T) {
			child := storage.FlowRule{Name: "OwnedChild", Active: true, ProcessType: "AutoLaunchedFlow"}
			switch shape {
			case "field-update":
				child.Steps = []storage.FlowStep{{Kind: "fieldUpdate"}}
			case "collection-lookup":
				child.Steps = []storage.FlowStep{{Kind: "recordLookup", RecordLookup: storage.FlowRecordLookup{Name: "Found", ObjectName: "Account", StoreOutputAutomatically: true}}}
			case "record-input-create":
				child.Steps = []storage.FlowStep{{Kind: "recordCreate", RecordCreate: storage.FlowRecordCreate{Name: "Create", InputReference: "candidate"}}}
			case "contact-create":
				child.Steps = []storage.FlowStep{{Kind: "recordCreate", RecordCreate: storage.FlowRecordCreate{Name: "Create", ObjectName: "Contact"}}}
			case "record-delete":
				child.Steps = []storage.FlowStep{{Kind: "recordDelete"}}
			case "email-alert":
				child.Steps = []storage.FlowStep{{Kind: "action", Action: storage.FlowAction{ActionType: "emailAlert", ActionName: "Account.Alert"}}}
			case "email-simple":
				child.Steps = []storage.FlowStep{{Kind: "action", Action: storage.FlowAction{ActionType: "emailSimple", ActionName: "emailSimple"}}}
			case "subtract":
				child.Steps = []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{Target: "amount", Operator: "Subtract", LiteralValue: "1"}}}
			case "loop":
				child.Steps = []storage.FlowStep{{Kind: "loop"}}
			case "custom-error":
				child.Steps = []storage.FlowStep{{Kind: "customError"}}
			case "transform":
				child.Steps = []storage.FlowStep{{Kind: "transform"}}
			case "collection-processor":
				child.Steps = []storage.FlowStep{{Kind: "collectionProcessor"}}
			case "fault-branch":
				child.Steps = []storage.FlowStep{{Kind: "recordLookup", FaultBranch: []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{Target: "message", Operator: "Assign", LiteralValue: "caught"}}}}}
			case "target":
				child.Steps = []storage.FlowStep{{Kind: "reference", Target: "entry"}}
				child.StepTargets = map[string][]storage.FlowStep{"entry": {{Kind: "loop"}}}
			}
			parent := storage.FlowRule{Name: "OwnedParent", Active: true, ProcessType: "AutoLaunchedFlow", Variables: []storage.FlowVariable{{Name: "token", DataType: "String", IsInput: true}}, Steps: []storage.FlowStep{{Kind: "subflow", Subflow: storage.FlowSubflow{FlowName: child.Name}}}}
			org := storage.NewOrgState()
			org.Metadata.Flows = []storage.FlowRule{parent, child}
			machine := New(nil)
			machine.SetOrg(&org)
			if !machine.invocableFlowActionTreeSupported(parent) {
				t.Fatal("child interpreter shape was rejected by action admission")
			}
		})
	}
}
