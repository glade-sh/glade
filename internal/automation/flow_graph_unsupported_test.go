package automation

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func TestRejectedFlowRuleKeepsSharedTargetsCompact(t *testing.T) {
	var sizes []int
	for _, depth := range []int{3, 5} {
		flow := loadSharedFlowWithRejectedRule(t, depth)
		org := storage.NewOrgState()
		org.Metadata.Flows = flow.Rules
		encoded, err := json.Marshal(org.Clone().Metadata.Flows)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%d shared decisions with rejected rule: %d cloned JSON bytes", depth, len(encoded))
		sizes = append(sizes, len(encoded))
	}
	if sizes[1] > 3*sizes[0] {
		t.Fatalf("rejected rule disabled bounded shared targets: %d -> %d bytes", sizes[0], sizes[1])
	}
}

func TestRejectedFlowRuleKeepsSupportedBehavior(t *testing.T) {
	flow := loadSharedFlowWithRejectedRule(t, 3)
	for mask := 0; mask < 8; mask++ {
		for _, roundtrip := range []bool{false, true} {
			t.Run(fmt.Sprintf("mask%d/json%t", mask, roundtrip), func(t *testing.T) {
				rule := flow.Rules[0]
				if roundtrip {
					encoded, err := json.Marshal(rule)
					if err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(encoded, &rule); err != nil {
						t.Fatal(err)
					}
				}
				org := storage.NewOrgState()
				engine := dml.NewEngine(&org)
				var actions []string
				engine.FlowActionInvoker = func(action storage.FlowAction, _ storage.Record) error {
					actions = append(actions, action.Name)
					return nil
				}
				output, err := engine.RunAutolaunchedFlowWithOutput(rule, dml.FlowInterviewInput{Scalars: map[string]storage.Value{
					"A": storage.BooleanValue(mask&1 != 0), "B": storage.BooleanValue(mask&2 != 0), "C": storage.BooleanValue(mask&4 != 0),
					"Count": storage.IntegerValue(0),
				}})
				if err != nil {
					t.Fatal(err)
				}
				if got := strings.Join(actions, ","); got != "Notify" {
					t.Fatalf("supported action changed or rejected rule executed: %q", got)
				}
				if got := output.Scalars["result"]; got.String != "done" {
					t.Fatalf("supported output changed: %#v", got)
				}
			})
		}
	}
}

func TestRejectedFlowRuleWithoutDefaultKeepsLegacyFallback(t *testing.T) {
	flow := loadSharedFlowWithRejectedRule(t, 3)
	text, err := os.ReadFile(flow.File)
	if err != nil {
		t.Fatal(err)
	}
	var raw flowXML
	if err := xml.Unmarshal(text, &raw); err != nil {
		t.Fatal(err)
	}
	raw.Decisions[0].DefaultConnector = flowConnectorXML{}
	raw.Decisions[0].Rules = raw.Decisions[0].Rules[:1]
	raw.Decisions[0].Rules[0].Connector.TargetReference = "Route0"
	if newFlowTargetGraph(raw) != nil {
		t.Fatal("rejected connector with path-dependent fallback was compacted")
	}
	compiled, diags := flowPopulateBranchFromTarget(storage.FlowBranch{}, "Route0", raw, flowFormulaMap(raw.Formulas), flowVariableMap(raw.Variables))
	if len(compiled.Steps) != 0 || len(diags) != 1 || !strings.Contains(diags[0].Message, `condition logic "NOT (1)" is not supported`) {
		t.Fatalf("legacy rejected-connector fallback changed: %#v, %#v", compiled, diags)
	}
}

func loadSharedFlowWithRejectedRule(t *testing.T, depth int) Flow {
	t.Helper()
	flow := loadSharedFlowFixture(t, depth, true)
	text, err := os.ReadFile(flow.File)
	if err != nil {
		t.Fatal(err)
	}
	// NOT remains unsupported. Rejecting one branch must retain its diagnostic
	// without expanding all supported shared continuations in the same Flow.
	rejected := `<rules><name>Rejected</name><conditionLogic>NOT (1)</conditionLogic><conditions><leftValueReference>A</leftValueReference><operator>EqualTo</operator><rightValue><booleanValue>true</booleanValue></rightValue></conditions><connector><targetReference>RejectedAction</targetReference></connector></rules>`
	body := strings.Replace(string(text), `<decisions><name>Route0</name>`, `<decisions><name>Route0</name>`+rejected, 1)
	body = strings.Replace(body, `</Flow>`, `<actionCalls><name>RejectedAction</name><actionType>apex</actionType><actionName>Notice.run</actionName></actionCalls></Flow>`, 1)
	writeWorkflowTestFile(t, flow.File, body)
	flow, diags, err := loadFlow(flow.File)
	if err != nil || len(flow.Rules) != 1 {
		t.Fatalf("rejected-rule fixture: error=%v ruleCount=%d", err, len(flow.Rules))
	}
	if len(diags) != 1 || !strings.Contains(diags[0].Message, `branch "Rejected" condition logic "NOT (1)" is not supported`) {
		t.Fatalf("rejected-rule diagnostic changed: %#v", diags)
	}
	return flow
}
