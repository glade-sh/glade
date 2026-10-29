package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestInvocableFlowSubflowRouteValidation(t *testing.T) {
	for _, shape := range []string{"local-child", "missing-child", "inactive-child", "hosted-child", "cyclic-child", "collection-child"} {
		t.Run(shape, func(t *testing.T) {
			child := storage.FlowRule{Name: "CedarChild", Active: true, ProcessType: "AutoLaunchedFlow",
				Variables: []storage.FlowVariable{{Name: "result", DataType: "String", IsOutput: true}},
				Steps:     []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{Target: "result", Operator: "Assign", LiteralValue: "Cedar first"}}},
			}
			parent := storage.FlowRule{Name: "CedarParent", Active: true, ProcessType: "AutoLaunchedFlow",
				Variables: []storage.FlowVariable{{Name: "result", DataType: "String", IsOutput: true}},
				Steps: []storage.FlowStep{{Kind: "subflow", Subflow: storage.FlowSubflow{Name: "CallChild", FlowName: "CedarChild",
					OutputAssignments: []storage.FlowAssignment{{Target: "result", SourceField: "result"}}}}},
			}
			switch shape {
			case "missing-child":
				parent.Steps[0].Subflow.FlowName = "AbsentChild"
			case "inactive-child":
				child.Active = false
			case "hosted-child":
				child.Steps = []storage.FlowStep{{Kind: "action", Action: storage.FlowAction{ActionType: "apex", ActionName: "HostedAction"}}}
			case "cyclic-child":
				child.Steps = []storage.FlowStep{{Kind: "subflow", Subflow: storage.FlowSubflow{FlowName: "CedarParent"}}}
			case "collection-child":
				child.Variables[0].IsCollection = true
			}
			org := storage.NewOrgState()
			org.Metadata.Flows = []storage.FlowRule{parent, child}
			machine := New(nil)
			machine.SetOrg(&org)
			action := Object("Invocable.Action")
			action.Fields["name"] = String("CedarParent")
			output, success, err := machine.invokeInvocableFlowAction(action, Map(), &Result{})
			if shape == "local-child" {
				// The project-level native route matrix pins this child result.
				if err != nil || !success || output.Map[mapKey(String("result"))].Text != "Cedar first" {
					t.Fatalf("local invocable subflow: success=%v output=%v error=%v", success, output, err)
				}
			} else if err == nil || success {
				t.Fatalf("unsupported child must retain the local boundary: success=%v error=%v", success, err)
			}
		})
	}
}

func TestFlowInterviewAfterTriggerScalarInputSnapshot(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Contact")
	org.Metadata.Flows = []storage.FlowRule{{Name: "CedarParent", Variables: []storage.FlowVariable{{Name: "target", DataType: "SObject", ObjectType: "Contact", IsInput: true}}}}
	machine := New(nil)
	machine.SetOrg(&org)
	row := Object("Contact")
	row.Fields["LastName"] = String("Cedar contact")
	markTriggerSObject(&row)
	row.Fields[sobjectReadOnlyField] = String("trigger record")
	variables := Map()
	key := mapKey(String("target"))
	variables.Map[key] = row
	variables.MapKeys[key] = String("target")
	interview := Object("Flow.Interview")
	interview.Fields["flowName"] = String("CedarParent")
	interview.Fields["variables"] = variables
	input, _, err := machine.flowInterviewInput(interview)
	if err != nil || input.Records["target"].Fields["LastName"].String != "Cedar contact" {
		t.Fatalf("native after-trigger interview accepts scalar record input: input=%v error=%v", input, err)
	}
	if _, protected := sobjectReadOnlyReason(row); !protected {
		t.Fatal("input snapshot removed the caller's read-only guard")
	}
	if _, err := machine.recordFromValue(&row); err == nil {
		t.Fatal("caller DML on the original trigger record must remain forbidden")
	}
}
