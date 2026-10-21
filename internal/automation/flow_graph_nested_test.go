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

// The inner body returns only to its inner header. Its no-more continuation
// returns to the outer header; neither body has an outside entry or fault path.
func loadNestedSharedLoopFixture(t *testing.T, depth int) Flow {
	t.Helper()
	flow := loadSharedLoopFixture(t, depth, false)
	text, err := os.ReadFile(flow.File)
	if err != nil {
		t.Fatal(err)
	}
	body := string(text)
	body = strings.Replace(body, `<start><connector><targetReference>Repeat</targetReference>`, `<start><connector><targetReference>OuterRepeat</targetReference>`, 1)
	body = strings.Replace(body, `<name>Items</name>`, `<name>InnerItems</name>`, 1)
	body = strings.Replace(body, `<collectionReference>Items</collectionReference>`, `<collectionReference>InnerItems</collectionReference>`, 1)
	body = strings.Replace(body, `<assignNextValueToReference>Item</assignNextValueToReference>`, `<assignNextValueToReference>InnerItem</assignNextValueToReference>`, 1)
	body = strings.Replace(body,
		`<actionCalls><name>AfterLoop</name><actionType>apex</actionType><actionName>Notice.run</actionName></actionCalls>`,
		`<actionCalls><name>AfterLoop</name><actionType>apex</actionType><actionName>Notice.run</actionName><connector><targetReference>OuterRepeat</targetReference></connector></actionCalls>`, 1)
	outer := `<variables><name>OuterItems</name><dataType>SObject</dataType><objectType>Account</objectType><isCollection>true</isCollection><isInput>true</isInput></variables><loops><name>OuterRepeat</name><collectionReference>OuterItems</collectionReference><iterationOrder>Asc</iterationOrder><assignNextValueToReference>OuterItem</assignNextValueToReference><nextValueConnector><targetReference>Repeat</targetReference></nextValueConnector><noMoreValuesConnector><targetReference>AllDone</targetReference></noMoreValuesConnector></loops><actionCalls><name>AllDone</name><actionType>apex</actionType><actionName>Notice.run</actionName></actionCalls>`
	body = strings.Replace(body, "</Flow>", outer+"</Flow>", 1)
	writeWorkflowTestFile(t, flow.File, body)
	flow, diags, err := loadFlow(flow.File)
	if err != nil || len(diags) != 0 || len(flow.Rules) != 1 {
		t.Fatalf("nested fixture: %v %v", err, diags)
	}
	return flow
}

func TestNestedSharedLoopTargetsStayCompact(t *testing.T) {
	var sizes []int
	for _, depth := range []int{3, 5} {
		flow := loadNestedSharedLoopFixture(t, depth)
		org := storage.NewOrgState()
		org.Metadata.Flows = flow.Rules
		encoded, err := json.Marshal(org.Clone().Metadata.Flows)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%d shared decisions in strict nested loops: %d cloned JSON bytes", depth, len(encoded))
		sizes = append(sizes, len(encoded))
	}
	if sizes[1] > 3*sizes[0] {
		t.Fatalf("shared nested body expanded: %d -> %d bytes", sizes[0], sizes[1])
	}
}

func TestNestedSharedLoopTargetsPreserveIterations(t *testing.T) {
	flow := loadNestedSharedLoopFixture(t, 3)
	items := func(prefix string, count int) []storage.Record {
		var out []storage.Record
		for i := 0; i < count; i++ {
			out = append(out, storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue(fmt.Sprintf("%s%d", prefix, i+1))}})
		}
		return out
	}
	for _, outerCount := range []int{0, 1, 2} {
		for _, innerCount := range []int{0, 1, 2} {
			for _, roundtrip := range []bool{false, true} {
				t.Run(fmt.Sprintf("outer%d/inner%d/json%t", outerCount, innerCount, roundtrip), func(t *testing.T) {
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
					var calls []string
					engine.FlowActionInvokerWithContext = func(action storage.FlowAction, _ storage.Record, ctx dml.FlowActionContext) error {
						name := action.Name
						if name == "Notify" || name == "AfterLoop" {
							name += ":" + ctx.Records["outeritem"].Fields["Name"].String
						}
						if action.Name == "Notify" {
							name += "/" + ctx.Records["inneritem"].Fields["Name"].String
						}
						calls = append(calls, name)
						return nil
					}
					_, err := engine.RunAutolaunchedFlowWithOutput(rule, dml.FlowInterviewInput{Collections: map[string][]storage.Record{
						"OuterItems": items("Outer", outerCount), "InnerItems": items("Inner", innerCount),
					}})
					if err != nil {
						t.Fatal(err)
					}
					var want []string
					for outer := 1; outer <= outerCount; outer++ {
						for inner := 1; inner <= innerCount; inner++ {
							want = append(want, fmt.Sprintf("Notify:Outer%d/Inner%d", outer, inner))
						}
						want = append(want, fmt.Sprintf("AfterLoop:Outer%d", outer))
					}
					want = append(want, "AllDone")
					if got := strings.Join(calls, ","); got != strings.Join(want, ",") {
						t.Fatalf("nested action order = %q, want %q", got, strings.Join(want, ","))
					}
				})
			}
		}
	}
}

func TestNestedSharedLoopTargetsKeepUnstructuredGraphsLegacy(t *testing.T) {
	flow := loadNestedSharedLoopFixture(t, 3)
	text, err := os.ReadFile(flow.File)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"overlapping siblings", "external inner entry", "cross ancestor return", "inner continuation overlap", "fault"} {
		t.Run(kind, func(t *testing.T) {
			var raw flowXML
			if err := xml.Unmarshal(text, &raw); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "overlapping siblings":
				raw.Loops = append(raw.Loops, flowLoopXML{Name: "Sibling", CollectionReference: "InnerItems", NextValueConnector: flowConnectorXML{TargetReference: "Route0"}, NoMoreValuesConnector: flowConnectorXML{TargetReference: "AllDone"}})
			case "external inner entry":
				raw.ActionCalls = append(raw.ActionCalls, flowActionCallXML{Name: "Outside", Connector: flowConnectorXML{TargetReference: "Route1"}})
				raw.Start.Connector.TargetReference = "Outside"
			case "cross ancestor return":
				for i := range raw.ActionCalls {
					if raw.ActionCalls[i].Name == "Notify" {
						raw.ActionCalls[i].Connector.TargetReference = "OuterRepeat"
					}
				}
			case "inner continuation overlap":
				for i := range raw.Loops {
					if raw.Loops[i].Name == "Repeat" {
						raw.Loops[i].NoMoreValuesConnector.TargetReference = "Route1"
					}
				}
			case "fault":
				raw.ActionCalls[0].FaultConnector.TargetReference = "AllDone"
			}
			if newFlowTargetGraph(raw) != nil {
				t.Fatal("unstructured nested graph was compacted")
			}
		})
	}
}
