package dml

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Null, absent, and non-String output behavior here records Glade's current
// inferred behavior, not a Salesforce capture; coercion remains unverified.
func TestFlowActionAssignOutputsSingleAndBatch(t *testing.T) {
	action := storage.FlowAction{Name: "Return", ActionType: "apex", Outputs: []storage.FlowActionOutput{
		{Name: "text", AssignTo: "TextResult"}, {Name: "empty", AssignTo: "NullResult"},
		{Name: "missing", AssignTo: "PriorResult"}, {Name: "number", AssignTo: "NumberResult"},
		{Name: "date", AssignTo: "DateResult"}, {Name: "items", AssignTo: "ItemsResult"},
	}}
	prior := storage.StringValue("prior")
	vars := []storage.FlowVariable{
		{Name: "TextResult", DataType: "String", IsOutput: true}, {Name: "NullResult", DataType: "String", IsOutput: true, DefaultValue: &prior},
		{Name: "PriorResult", DataType: "String", IsOutput: true, DefaultValue: &prior}, {Name: "NumberResult", DataType: "Number", IsOutput: true},
		{Name: "DateResult", DataType: "Date", IsOutput: true}, {Name: "ItemsResult", DataType: "String", IsCollection: true, IsOutput: true},
	}
	values := func(text string) map[string]storage.Value {
		return map[string]storage.Value{
			"text": storage.StringValue(text), "empty": storage.NullValue(), "number": storage.DecimalValue("12.5"),
			"date": storage.DateValue("2026-11-03"), "items": storage.ListValue(storage.StringValue(text), storage.StringValue("other")),
		}
	}
	t.Run("single", func(t *testing.T) {
		org := testOrg()
		engine := NewEngine(&org)
		engine.FlowActionInvokerWithContext = func(_ storage.FlowAction, _ storage.Record, context FlowActionContext) error {
			context.OutputFields(values("one"))
			return nil
		}
		rule := storage.FlowRule{Name: "SingleOutput", Variables: vars, Steps: []storage.FlowStep{{Kind: "action", Action: action}}}
		out, err := engine.RunAutolaunchedFlowWithOutput(rule, FlowInterviewInput{})
		if err != nil {
			t.Fatal(err)
		}
		want := values("one")
		want["missing"] = prior
		for source, dest := range map[string]string{"text": "textresult", "empty": "nullresult", "missing": "priorresult", "number": "numberresult", "date": "dateresult", "items": "itemsresult"} {
			if got := out.Scalars[dest]; !reflect.DeepEqual(got, want[source]) {
				t.Errorf("%s = %#v, want %#v", dest, got, want[source])
			}
		}
	})
	t.Run("batch", func(t *testing.T) {
		org := testOrg()
		obj := org.Objects["Account"]
		obj.Definition.Fields["Description"] = storage.Field{APIName: "Description", Type: storage.FieldString}
		obj.Definition.FlowRules = []storage.FlowRule{{Name: "BatchOutput", Active: true, TriggerType: "RecordAfterSave", RecordTriggerType: "Create", Variables: vars,
			Steps: []storage.FlowStep{{Kind: "action", Action: action}, {Kind: "fieldUpdate", FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Name", SourceField: "TextResult"}, {Field: "Description", SourceField: "NullResult"}}}},
		}}
		records := []storage.Record{{Object: "Account", ID: "001000000000001", Fields: map[string]storage.Value{"Name": storage.StringValue("before1"), "Description": storage.StringValue("old")}}, {Object: "Account", ID: "001000000000002", Fields: map[string]storage.Value{"Name": storage.StringValue("before2"), "Description": storage.StringValue("old")}}}
		for _, record := range records {
			obj.Records[record.ID] = record
		}
		org.Objects["Account"] = obj
		engine := NewEngine(&org)
		for _, record := range records {
			engine.markCreatedAutomation(record.ID)
		}
		engine.FlowActionBatchInvoker = func(_ storage.FlowAction, requests []FlowActionRequest) error {
			if len(requests) != 2 {
				t.Fatalf("requests = %d, want 2", len(requests))
			}
			requests[0].Context.OutputFields(values("first"))
			requests[1].Context.OutputFields(values("second"))
			return nil
		}
		handled, err := engine.ApplyFlowActionBatch(records, nil)
		if err != nil || !handled {
			t.Fatalf("handled=%v error=%v", handled, err)
		}
		for i, want := range []string{"first", "second"} {
			if got := org.Objects["Account"].Records[records[i].ID].Fields["Description"]; got.Kind != storage.ValueNull && got.Kind != "" {
				t.Errorf("record %d Description = %#v, want null", i, got)
			}
			if got := org.Objects["Account"].Records[records[i].ID].Fields["Name"].String; got != want {
				t.Errorf("record %d Name = %q, want %q", i, got, want)
			}
		}
	})
}
