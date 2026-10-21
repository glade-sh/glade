package automation

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// Each source decision routes all of its alternatives to one shared suffix.
// Adding source nodes must not materialize every possible execution path in
// the model, its clones, or its serialized cache representation.
func TestSharedFlowTargetsStayCompact(t *testing.T) {
	for _, linked := range []bool{false, true} {
		t.Run(fmt.Sprint("linked=", linked), func(t *testing.T) {
			var sizes []int
			for _, depth := range []int{3, 5, 9, 30} {
				flow := loadSharedFlowFixtureWithLinks(t, depth, false, linked)
				rule := flow.Rules[0]
				org := storage.NewOrgState()
				org.Metadata.Flows = []storage.FlowRule{rule}
				clone := org.Clone()
				encoded, err := json.Marshal(clone.Metadata.Flows)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("%d decisions: %d cloned JSON bytes", depth, len(encoded))
				if len(rule.StepTargets) > 2*depth+1 {
					t.Fatalf("too many target nodes: %d", len(rule.StepTargets))
				}
				if len(sizes) > 0 && len(encoded) > sizes[0]*(depth+1)/2 {
					t.Fatalf("nonlinear serialized graph: %d bytes for %d decisions", len(encoded), depth)
				}
				sizes = append(sizes, len(encoded))
			}
			if sizes[1] > 3*sizes[0] {
				t.Fatalf("two additional shared decisions multiplied cloned/serialized model: %d -> %d bytes", sizes[0], sizes[1])
			}
		})
	}
}

func TestSharedFlowTargetsExecuteSelectedPathOnce(t *testing.T) {
	flow := loadSharedFlowFixture(t, 3, true)
	org := storage.NewOrgState()
	engine := dml.NewEngine(&org)
	for mask := 0; mask < 8; mask++ {
		t.Run(fmt.Sprint(mask), func(t *testing.T) {
			input := dml.FlowInterviewInput{Scalars: map[string]storage.Value{
				"A":     storage.BooleanValue(mask&1 != 0),
				"B":     storage.BooleanValue(mask&2 != 0),
				"C":     storage.BooleanValue(mask&4 != 0),
				"Count": storage.IntegerValue(0),
			}}
			for _, roundtrip := range []bool{false, true} {
				rule := flow.Rules[0]
				if roundtrip {
					raw, err := json.Marshal(rule)
					if err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(raw, &rule); err != nil {
						t.Fatal(err)
					}
				}
				calls := 0
				engine.FlowActionInvoker = func(storage.FlowAction, storage.Record) error { calls++; return nil }
				output, err := engine.RunAutolaunchedFlowWithOutput(rule, input)
				if err != nil {
					t.Fatal(err)
				}
				if calls != 1 {
					t.Fatalf("shared path executed %d actions", calls)
				}
				if got := output.Scalars["result"]; got.String != "done" {
					t.Fatalf("shared frame output: %#v", got)
				}
			}
		})
	}
}

func loadSharedFlowFixture(t *testing.T, depth int, scalar bool) Flow {
	t.Helper()
	return loadSharedFlowFixtureWithLinks(t, depth, scalar, false)
}

func loadSharedFlowFixtureWithLinks(t *testing.T, depth int, scalar, linked bool) Flow {
	t.Helper()
	path := filepath.Join(t.TempDir(), "SharedPath.flow-meta.xml")
	var text strings.Builder
	text.WriteString(`<Flow xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><processType>AutoLaunchedFlow</processType><status>Active</status>`)
	if scalar {
		for _, name := range []string{"A", "B", "C"} {
			fmt.Fprintf(&text, `<variables><name>%s</name><dataType>Boolean</dataType><isInput>true</isInput></variables>`, name)
		}
		text.WriteString(`<variables><name>Count</name><dataType>Number</dataType><isInput>true</isInput><isOutput>true</isOutput></variables><variables><name>Result</name><dataType>String</dataType><isOutput>true</isOutput></variables><start><connector><targetReference>Route0</targetReference></connector></start>`)
	} else {
		text.WriteString(`<start><object>Account</object><triggerType>RecordBeforeSave</triggerType><recordTriggerType>Create</recordTriggerType><connector><targetReference>Route0</targetReference></connector></start>`)
	}
	for i := 0; i < depth; i++ {
		next := "Finish"
		if i+1 < depth {
			next = fmt.Sprint("Route", i+1)
			if linked {
				next = fmt.Sprint("Link", i)
			}
		}
		fmt.Fprintf(&text, `<decisions><name>Route%d</name><defaultConnector><targetReference>%s</targetReference></defaultConnector><rules><name>Match</name><conditionLogic>or</conditionLogic>`, i, next)
		for _, name := range []string{"A", "B", "C"} {
			if scalar {
				fmt.Fprintf(&text, `<conditions><leftValueReference>%s</leftValueReference><operator>EqualTo</operator><rightValue><booleanValue>true</booleanValue></rightValue></conditions>`, name)
			} else {
				fmt.Fprintf(&text, `<conditions><leftValueReference>$Record.Name</leftValueReference><operator>EqualTo</operator><rightValue><stringValue>%s</stringValue></rightValue></conditions>`, name)
			}
		}
		fmt.Fprintf(&text, `<connector><targetReference>%s</targetReference></connector></rules></decisions>`, next)
	}
	if linked {
		for i := 0; i+1 < depth; i++ {
			fmt.Fprintf(&text, `<assignments><name>Link%d</name><assignmentItems><assignToReference>$Record.Description</assignToReference><operator>Assign</operator><value><stringValue>passing</stringValue></value></assignmentItems><connector><targetReference>Route%d</targetReference></connector></assignments>`, i, i+1)
		}
	}
	if scalar {
		text.WriteString(`<assignments><name>Finish</name><assignmentItems><assignToReference>Count</assignToReference><operator>Assign</operator><value><numberValue>1</numberValue></value></assignmentItems><assignmentItems><assignToReference>Result</assignToReference><operator>Assign</operator><value><stringValue>done</stringValue></value></assignmentItems><connector><targetReference>Notify</targetReference></connector></assignments><actionCalls><name>Notify</name><actionType>apex</actionType><actionName>Notice.run</actionName></actionCalls>`)
	} else {
		text.WriteString(`<assignments><name>Finish</name><assignmentItems><assignToReference>$Record.Description</assignToReference><operator>Assign</operator><value><stringValue>done</stringValue></value></assignmentItems></assignments>`)
	}
	if scalar {
		text.WriteString(`<customErrors><name>UnusedError</name><customErrorMessages><errorMessage>unused</errorMessage></customErrorMessages></customErrors>`)
	}
	text.WriteString(`</Flow>`)
	writeWorkflowTestFile(t, path, text.String())
	flow, diags, err := loadFlow(path)
	if err != nil || len(diags) != 0 || len(flow.Rules) != 1 {
		t.Fatalf("owned flow: error=%v diagnostics=%v ruleCount=%d", err, diags, len(flow.Rules))
	}
	return flow
}

// An empty target must remain empty, rather than becoming a reference that
// changes the existing branch selection or suppresses the default outcome.
func TestSharedFlowTargetsPreserveEmptyBranches(t *testing.T) {
	raw := flowXML{
		Assignments: []flowAssignmentXML{{Name: "Empty"}},
		ActionCalls: []flowActionCallXML{{Name: "Finish", ActionType: "apex", ActionName: "Notice.run"}},
		Decisions: []flowDecisionXML{
			{Name: "Entry", Rules: []flowDecisionRuleXML{
				{Name: "EmptyOutcome", Connector: flowConnectorXML{TargetReference: "Empty"}},
				{Name: "Continue", Connector: flowConnectorXML{TargetReference: "Tail"}},
			}, DefaultConnector: flowConnectorXML{TargetReference: "Finish"}},
			{Name: "Other", DefaultConnector: flowConnectorXML{TargetReference: "Tail"}},
			{Name: "Tail", DefaultConnector: flowConnectorXML{TargetReference: "Finish"}},
		},
	}
	legacy, diags := flowPopulateBranchFromTarget(storage.FlowBranch{}, "Entry", raw, nil, nil)
	if len(diags) != 0 {
		t.Fatal(diags)
	}
	raw.graph = newFlowTargetGraph(raw)
	if raw.graph == nil {
		t.Fatal("shared decision fixture did not use target graph")
	}
	compact, diags := flowPopulateBranchFromTarget(storage.FlowBranch{}, "Entry", raw, nil, nil)
	if len(diags) != 0 {
		t.Fatal(diags)
	}
	compactSteps := raw.graph.targets[compact.Steps[0].Target]
	want, got := legacy.Steps[0].Branches, compactSteps[0].Branches
	if len(got) != len(want) {
		t.Fatalf("empty outcome retained: got %d branches, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Name != want[i].Name || got[i].Default != want[i].Default {
			t.Fatalf("branch %d changed", i)
		}
	}
	unknown, diags := flowPopulateBranchFromTarget(storage.FlowBranch{}, "Missing", raw, nil, nil)
	if len(unknown.Steps) != 0 || len(diags) != 1 {
		t.Fatalf("unknown target acquired effects: %#v, %v", unknown, diags)
	}
}

func TestSharedFlowTargetsKeepDecisionOrder(t *testing.T) {
	connector := func(target string) flowConnectorXML { return flowConnectorXML{TargetReference: target} }
	outcome := func(name, input, target string) flowDecisionRuleXML {
		return flowDecisionRuleXML{Name: name, Connector: connector(target), Conditions: []flowConditionXML{{
			LeftValueReference: input, Operator: "EqualTo", RightValue: flowValueXML{BooleanValue: "true"},
		}}}
	}
	raw := flowXML{Decisions: []flowDecisionXML{
		{Name: "Entry", Rules: []flowDecisionRuleXML{outcome("First", "A", "FirstAction"), outcome("Second", "B", "SecondAction")}, DefaultConnector: connector("DefaultAction")},
		{Name: "Shared", Rules: []flowDecisionRuleXML{outcome("Yes", "C", "YesAction")}, DefaultConnector: connector("NoAction")},
	}}
	for _, name := range []string{"First", "Second", "Default", "Yes", "No"} {
		action := flowActionCallXML{Name: name + "Action", ActionType: "apex", ActionName: "Notice.run"}
		if name != "Yes" && name != "No" {
			action.Connector = connector("Shared")
		}
		raw.ActionCalls = append(raw.ActionCalls, action)
	}
	for _, compact := range []bool{false, true} {
		if compact {
			raw.graph = newFlowTargetGraph(raw)
			if raw.graph == nil {
				t.Fatal("indirect shared continuation not compacted")
			}
		}
		compiled, diags := flowPopulateBranchFromTarget(storage.FlowBranch{}, "Entry", raw, nil, nil)
		if len(diags) != 0 {
			t.Fatal(diags)
		}
		rule := storage.FlowRule{Name: "OrderedInterview", Steps: compiled.Steps}
		if compact {
			rule.StepTargets = raw.graph.targets
		}
		for mask := 0; mask < 8; mask++ {
			org := storage.NewOrgState()
			engine := dml.NewEngine(&org)
			var actions []string
			engine.FlowActionInvoker = func(action storage.FlowAction, _ storage.Record) error {
				actions = append(actions, action.Name)
				return nil
			}
			_, err := engine.RunAutolaunchedFlowWithOutput(rule, dml.FlowInterviewInput{Scalars: map[string]storage.Value{
				"a": storage.BooleanValue(mask&1 != 0), "b": storage.BooleanValue(mask&2 != 0), "c": storage.BooleanValue(mask&4 != 0),
			}})
			if err != nil {
				t.Fatal(err)
			}
			first := "DefaultAction"
			if mask&1 != 0 {
				first = "FirstAction"
			} else if mask&2 != 0 {
				first = "SecondAction"
			}
			last := "NoAction"
			if mask&4 != 0 {
				last = "YesAction"
			}
			if got, want := strings.Join(actions, ","), first+","+last; got != want {
				t.Fatalf("compact=%t mask=%d: actions %q, want %q", compact, mask, got, want)
			}
		}
	}
}

func TestSharedFlowTargetsBeforeSaveRoundTrip(t *testing.T) {
	flow := loadSharedFlowFixture(t, 5, false)
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Account", Fields: map[string]storage.Field{
			"Name":        {APIName: "Name", Type: storage.FieldString},
			"Description": {APIName: "Description", Type: storage.FieldString},
		}, FlowRules: flow.Rules,
	}}
	encoded, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	var decoded storage.OrgState
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, state := range []storage.OrgState{org.Clone(), decoded} {
		engine := dml.NewEngine(&state)
		for _, name := range []string{"A", "B", "C", "Unmatched"} {
			records := []storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue(name)}}}
			if err := engine.ApplyBeforeSaveFlows(records); err != nil {
				t.Fatal(err)
			}
			if got := records[0].Fields["Description"].String; got != "done" {
				t.Fatalf("name=%s: description %q", name, got)
			}
		}
	}
}

func TestSharedFlowTargetsKeepStructuredControlFlow(t *testing.T) {
	fixture := func() flowXML {
		return flowXML{
			Assignments: []flowAssignmentXML{{Name: "Finish"}},
			Decisions: []flowDecisionXML{
				{Name: "Entry", DefaultConnector: flowConnectorXML{TargetReference: "Tail"}, Rules: []flowDecisionRuleXML{{Name: "Match", Connector: flowConnectorXML{TargetReference: "Tail"}}}},
				{Name: "Tail", DefaultConnector: flowConnectorXML{TargetReference: "Finish"}},
			},
		}
	}
	if newFlowTargetGraph(fixture()) == nil {
		t.Fatal("control is not a shared graph")
	}
	for _, kind := range []string{"loop", "node-fault", "start-fault", "cycle"} {
		t.Run(kind, func(t *testing.T) {
			raw := fixture()
			switch kind {
			case "loop":
				raw.Loops = []flowLoopXML{{Name: "Repeat"}}
			case "node-fault":
				raw.Assignments[0].FaultConnector.TargetReference = "Tail"
			case "start-fault":
				raw.Start.FaultConnector.TargetReference = "Tail"
			case "cycle":
				raw.Decisions[1].DefaultConnector.TargetReference = "Entry"
			}
			if newFlowTargetGraph(raw) != nil {
				t.Fatal("structured control-flow graph compacted")
			}
		})
	}
}

func loadSharedLoopFixture(t *testing.T, depth int, second bool) Flow {
	t.Helper()
	flow := loadSharedFlowFixture(t, depth, true)
	text, err := os.ReadFile(flow.File)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.Replace(string(text), `<start><connector><targetReference>Route0</targetReference>`, `<start><connector><targetReference>Repeat</targetReference>`, 1)
	body = strings.Replace(body, `<actionName>Notice.run</actionName></actionCalls>`, `<actionName>Notice.run</actionName><connector><targetReference>Repeat</targetReference></connector></actionCalls>`, 1)
	order := "Asc"
	loop := fmt.Sprintf(`<variables><name>Items</name><dataType>SObject</dataType><objectType>Account</objectType><isCollection>true</isCollection><isInput>true</isInput></variables><loops><name>Repeat</name><collectionReference>Items</collectionReference><iterationOrder>%s</iterationOrder><assignNextValueToReference>Item</assignNextValueToReference><nextValueConnector><targetReference>Route0</targetReference></nextValueConnector><noMoreValuesConnector><targetReference>AfterLoop</targetReference></noMoreValuesConnector></loops><actionCalls><name>AfterLoop</name><actionType>apex</actionType><actionName>Notice.run</actionName></actionCalls>`, order)
	if second {
		loop = strings.Replace(loop, `<noMoreValuesConnector><targetReference>AfterLoop</targetReference>`, `<noMoreValuesConnector><targetReference>RepeatAgain</targetReference>`, 1)
		loop += `<loops><name>RepeatAgain</name><collectionReference>Items</collectionReference><iterationOrder>Asc</iterationOrder><assignNextValueToReference>Item</assignNextValueToReference><nextValueConnector><targetReference>NotifyAgain</targetReference></nextValueConnector><noMoreValuesConnector><targetReference>AfterLoop</targetReference></noMoreValuesConnector></loops><actionCalls><name>NotifyAgain</name><actionType>apex</actionType><actionName>Notice.run</actionName><connector><targetReference>RepeatAgain</targetReference></connector></actionCalls>`
	}
	body = strings.Replace(body, "</Flow>", loop+"</Flow>", 1)
	writeWorkflowTestFile(t, flow.File, body)
	flow, diags, err := loadFlow(flow.File)
	if err != nil || len(diags) != 0 || len(flow.Rules) != 1 {
		t.Fatalf("loop fixture: %v %v", err, diags)
	}
	return flow
}

func TestSharedLoopTargetsStayCompact(t *testing.T) {
	var sizes []int
	for _, depth := range []int{3, 5, 9, 30} {
		flow := loadSharedLoopFixture(t, depth, false)
		org := storage.NewOrgState()
		org.Metadata.Flows = flow.Rules
		encoded, err := json.Marshal(org.Clone().Metadata.Flows)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%d decisions in loop: %d bytes", depth, len(encoded))
		if len(sizes) > 0 && len(encoded) > sizes[0]*(depth+1)/2 {
			t.Fatalf("nonlinear loop graph: %d bytes at depth%d", len(encoded), depth)
		}
		sizes = append(sizes, len(encoded))
	}
	if sizes[1] > 3*sizes[0] {
		t.Fatalf("shared loop body expanded: %d -> %d", sizes[0], sizes[1])
	}
}

func TestSharedLoopTargetsPreserveIterations(t *testing.T) {
	for _, second := range []bool{false, true} {
		flow := loadSharedLoopFixture(t, 3, second)
		for _, count := range []int{0, 1, 2} {
			for _, roundtrip := range []bool{false, true} {
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
					calls = append(calls, action.Name+":"+ctx.Records["item"].Fields["Name"].String)
					return nil
				}
				items := []storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("First")}}, {Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Second")}}}
				_, err := engine.RunAutolaunchedFlowWithOutput(rule, dml.FlowInterviewInput{Collections: map[string][]storage.Record{"Items": items[:count]}})
				if err != nil {
					t.Fatal(err)
				}
				want := "AfterLoop:"
				if count != 0 {
					want = "Notify:First,"
					if count == 2 {
						want += "Notify:Second,"
					}
					if second {
						want += "NotifyAgain:First,"
						if count == 2 {
							want += "NotifyAgain:Second,"
						}
					}
					want += "AfterLoop:"
				}
				if got := strings.Join(calls, ","); got != want {
					t.Fatalf("second=%t count=%d roundtrip=%t got %q want %q", second, count, roundtrip, got, want)
				}
			}
		}
	}
}

func TestSharedLoopTargetsKeepUnstructuredGraphsLegacy(t *testing.T) {
	fixture := loadSharedLoopFixture(t, 3, false)
	text, err := os.ReadFile(fixture.File)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"nested", "overlap", "external-entry", "start-entry", "continuation-overlap", "cycle", "fault", "unknown-start", "missing-collection", "missing-next"} {
		t.Run(kind, func(t *testing.T) {
			var raw flowXML
			if err := xml.Unmarshal(text, &raw); err != nil {
				t.Fatal(err)
			}
			if newFlowTargetGraph(raw) == nil {
				t.Fatal("control graph not compacted")
			}
			switch kind {
			case "nested", "overlap":
				raw.Loops = append(raw.Loops, flowLoopXML{Name: "Nested", CollectionReference: "Items", NextValueConnector: flowConnectorXML{TargetReference: "Route0"}})
				if kind == "nested" {
					raw.Loops[0].NextValueConnector.TargetReference = "Nested"
				} else {
					raw.ActionCalls[0].Connector.TargetReference = ""
				}
			case "external-entry":
				raw.ActionCalls = append(raw.ActionCalls, flowActionCallXML{Name: "Outside", Connector: flowConnectorXML{TargetReference: "Route1"}})
				raw.Start.Connector.TargetReference = "Outside"
			case "start-entry":
				raw.Start.Connector.TargetReference = "Route0"
			case "continuation-overlap":
				raw.Loops[0].NoMoreValuesConnector.TargetReference = "Route1"
			case "cycle":
				raw.Decisions[2].DefaultConnector.TargetReference = "Route0"
			case "fault":
				raw.ActionCalls[0].FaultConnector.TargetReference = "AfterLoop"
			case "unknown-start":
				raw.Start.Connector.TargetReference = "Unknown"
			case "missing-collection":
				raw.Loops[0].CollectionReference = ""
			case "missing-next":
				raw.Loops[0].NextValueConnector.TargetReference = ""
			}
			if newFlowTargetGraph(raw) != nil {
				t.Fatal("path-dependent loop graph compacted")
			}
		})
	}
}
