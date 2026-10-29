package dml

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Uncaptured lookup operators belong to the existing decision dispatcher.
// These checks preserve release behavior; they do not assert native parity.
func TestFlowLookupUncapturedOperatorsPreserveDispatcher(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, operation := range []string{"wasSet", "in", "notIn", "GreaterThan", "GreaterThanOrEqualTo", "LessThan", "LessThanOrEqualTo", "hasError", "isChanged", "isEmpty", "unknown"} {
			for _, left := range []storage.Value{storage.StringValue("Owned operator seed"), storage.IntegerValue(23), storage.NullValue()} {
				for _, invocable := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/%s/invocable=%t", api, operation, left.Kind, invocable), func(t *testing.T) {
						org := testOrg()
						org.APIVersion = api
						definition := org.Objects["Account"].Definition
						frame := newFlowFrame()
						frame.lookupOutputs["find"] = flowLookupOutput{definition: definition, record: storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": left}}}
						engine := NewEngine(&org)
						engine.InvocableFlow = invocable
						item := storage.WorkflowCriteriaItem{Field: "Find.Name", Operation: operation, Value: "true"}
						if _, _, handled := engine.flowVariableOperandCriterion(item, frame); handled {
							t.Error("uncaptured lookup operator intercepted before the existing dispatcher")
						}
						if invocable {
							// Invocable decisions have their own pre-existing field reader.
							return
						}
						branch := storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{item}}
						matched, supported := engine.evaluateFlowFrameBranch(branch, storage.Record{}, definition, frame)
						wantSupported := operation == "wasSet" || operation == "hasError"
						if matched || supported != wantSupported {
							t.Fatalf("release dispatcher changed: matched=%v supported=%v; want false, %v", matched, supported, wantSupported)
						}
						if operation == "wasSet" {
							frame.assigned["find.name"] = true
							item.Value = "false"
							branch.Criteria[0] = item
							if matched, supported := engine.evaluateFlowFrameBranch(branch, storage.Record{}, definition, frame); !matched || !supported {
								t.Fatalf("WasSet must consult assignment provenance: matched=%v supported=%v", matched, supported)
							}
						}
					})
				}
			}
		}
	}
}
