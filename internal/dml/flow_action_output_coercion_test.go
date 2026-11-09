package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native OutputProbeCoercion at API 62 and 67 maps Decimal 12.5 into a String variable.
func TestFlowActionOutputDecimalToText(t *testing.T) {
	for _, tc := range []struct{ input, want string }{{"12.5", "12.5"}, {"-7.25", "-7.25"}, {"0", "0"}} {
		t.Run(tc.input, func(t *testing.T) {
			org := testOrg()
			engine := NewEngine(&org)
			engine.FlowActionInvokerWithContext = func(_ storage.FlowAction, _ storage.Record, ctx FlowActionContext) error {
				ctx.OutputFields(map[string]storage.Value{"amount": storage.DecimalValue(tc.input)})
				return nil
			}
			rule := storage.FlowRule{Name: "OutputCoercion", Variables: []storage.FlowVariable{{Name: "result", DataType: "String", IsOutput: true}}, Steps: []storage.FlowStep{{Kind: "action", Action: storage.FlowAction{Name: "Return", ActionType: "apex", Outputs: []storage.FlowActionOutput{{Name: "amount", AssignTo: "result"}}}}}}
			out, err := engine.RunAutolaunchedFlowWithOutput(rule, FlowInterviewInput{})
			if err != nil {
				t.Fatal(err)
			}
			if got := out.Scalars["result"]; got.Kind != storage.ValueString || got.String != tc.want {
				t.Fatalf("result = %#v, want String %q", got, tc.want)
			}
		})
	}
}

// The prepared batch route uses the same mapping separately for each record.
func TestFlowActionOutputDecimalToTextBatch(t *testing.T) {
	org := testOrg()
	object := org.Objects["Account"]
	action := storage.FlowAction{Name: "Return", ActionType: "apex", Outputs: []storage.FlowActionOutput{{Name: "amount", AssignTo: "result"}}}
	object.Definition.FlowRules = []storage.FlowRule{{Name: "BatchCoercion", Active: true, TriggerType: "RecordAfterSave", RecordTriggerType: "Create", Variables: []storage.FlowVariable{{Name: "result", DataType: "String"}}, Steps: []storage.FlowStep{{Kind: "action", Action: action}, {Kind: "fieldUpdate", FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Name", SourceField: "result"}}}}}}
	records := []storage.Record{{Object: "Account", ID: "001000000000001", Fields: map[string]storage.Value{"Name": storage.StringValue("before")}}, {Object: "Account", ID: "001000000000002", Fields: map[string]storage.Value{"Name": storage.StringValue("before")}}}
	for _, record := range records {
		object.Records[record.ID] = record
	}
	org.Objects["Account"] = object
	engine := NewEngine(&org)
	for _, record := range records {
		engine.markCreatedAutomation(record.ID)
	}
	engine.FlowActionBatchInvoker = func(_ storage.FlowAction, requests []FlowActionRequest) error {
		if len(requests) != 2 {
			t.Fatalf("got %d requests", len(requests))
		}
		requests[0].Context.OutputFields(map[string]storage.Value{"amount": storage.DecimalValue("12.5")})
		requests[1].Context.OutputFields(map[string]storage.Value{"amount": storage.DecimalValue("-7.25")})
		return nil
	}
	handled, err := engine.ApplyFlowActionBatch(records, nil)
	if err != nil || !handled {
		t.Fatalf("handled=%v error=%v", handled, err)
	}
	for i, want := range []string{"12.5", "-7.25"} {
		got := org.Objects["Account"].Records[records[i].ID].Fields["Name"]
		if got.Kind != storage.ValueString || got.String != want {
			t.Errorf("record %d = %#v, want %q", i, got, want)
		}
	}
}
