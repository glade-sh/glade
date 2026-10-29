package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The base interpreter selects a branch before resolving its subflow. An
// untaken dependency must not become an action-wide admission failure.
func TestInvocableFlowGuardedChildDependenciesPreserveRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, shape := range []string{"missing", "inactive", "recursive"} {
			t.Run(api+"/"+shape, func(t *testing.T) {
				parent := storage.FlowRule{Name: "GuardedParent", Active: true, ProcessType: "AutoLaunchedFlow", APIVersion: api,
					Variables: []storage.FlowVariable{{Name: "executeChild", DataType: "Boolean", IsInput: true}, {Name: "outcome", DataType: "String", IsOutput: true}},
					Steps: []storage.FlowStep{
						{Kind: "assignment", Assignment: storage.FlowAssignment{Target: "outcome", Operator: "Assign", LiteralValue: "kept"}},
						{Kind: "decision", Branches: []storage.FlowBranch{
							{Criteria: []storage.WorkflowCriteriaItem{{Field: "executeChild", Operation: "equals", Value: "true"}}, Steps: []storage.FlowStep{{Kind: "subflow", Subflow: storage.FlowSubflow{FlowName: "GuardedChild"}}}},
							{Default: true},
						}},
					},
				}
				child := storage.FlowRule{Name: "GuardedChild", Active: shape != "inactive", ProcessType: "AutoLaunchedFlow", APIVersion: api,
					Steps: []storage.FlowStep{{Kind: "subflow", Subflow: storage.FlowSubflow{FlowName: parent.Name}}},
				}
				org := storage.NewOrgState()
				org.APIVersion = api
				org.Metadata.Flows = []storage.FlowRule{parent}
				if shape != "missing" {
					org.Metadata.Flows = append(org.Metadata.Flows, child)
				}
				machine := New(nil)
				machine.SetOrg(&org)
				action := Object("Invocable.Action")
				action.Fields["name"] = String(parent.Name)
				invocation := Map()
				key := mapKey(String("executeChild"))
				invocation.Map[key], invocation.MapKeys[key] = Bool(false), String("executeChild")
				output, success, err := machine.invokeInvocableFlowAction(action, invocation, &Result{})
				if err != nil || !success || output.Map[mapKey(String("outcome"))].Text != "kept" {
					t.Fatalf("untaken %s child changed the base route: success=%v output=%v error=%v", shape, success, output, err)
				}
			})
		}
	}
}
