package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These are structural admission tests, not new Salesforce observations. Graphs
// beyond the native action-first fixture must keep their existing execution path.
func TestFlowActionBatchGraphAdmission(t *testing.T) {
	action := storage.FlowStep{Kind: "action", Action: storage.FlowAction{Name: "Result", ActionType: "apex"}}
	for _, tc := range []struct {
		name    string
		steps   []storage.FlowStep
		targets map[string][]storage.FlowStep
		want    bool
	}{
		{"direct", []storage.FlowStep{action, {Kind: "fieldUpdate"}}, nil, true},
		{"reference", []storage.FlowStep{{Kind: "reference", Target: "entry"}}, map[string][]storage.FlowStep{"entry": {action, {Kind: "recordUpdate"}}}, true},
		{"fault", []storage.FlowStep{{Kind: "action", Action: action.Action, FaultBranch: []storage.FlowStep{{Kind: "customError"}}}}, nil, true},
		{"earlier-effect", []storage.FlowStep{{Kind: "fieldUpdate"}, action}, nil, false},
		{"two-actions", []storage.FlowStep{action, action}, nil, false},
		{"fault-action", []storage.FlowStep{{Kind: "action", Action: action.Action, FaultBranch: []storage.FlowStep{action}}}, nil, false},
		{"decision", []storage.FlowStep{action, {Kind: "decision"}}, nil, false},
		{"subflow", []storage.FlowStep{action, {Kind: "subflow"}}, nil, false},
		{"loop", []storage.FlowStep{action, {Kind: "loop"}}, nil, false},
		{"missing", []storage.FlowStep{{Kind: "reference", Target: "absent"}}, nil, false},
		{"cycle", []storage.FlowStep{{Kind: "reference", Target: "entry"}}, map[string][]storage.FlowStep{"entry": {action, {Kind: "reference", Target: "entry"}}}, false},
		{"repeated-reference", []storage.FlowStep{{Kind: "reference", Target: "entry"}, {Kind: "reference", Target: "entry"}}, map[string][]storage.FlowStep{"entry": {action}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, supported := flowActionBatchEntry(storage.FlowRule{Steps: tc.steps, StepTargets: tc.targets})
			if supported != tc.want || supported && got.Name != action.Action.Name {
				t.Fatalf("action=%+v supported=%t; want supported=%t", got, supported, tc.want)
			}
		})
	}
}

func TestFlowActionAutomaticOutputNullAndUnsupportedTypes(t *testing.T) {
	for _, kind := range []storage.FieldType{storage.FieldDate, storage.FieldDateTime} {
		value, ok := flowActionOutputFieldValue(storage.NullValue(), storage.Field{Type: kind})
		if !ok || value.Kind != storage.ValueNull {
			t.Fatalf("native null output for %s = %+v, %t", kind, value, ok)
		}
		if _, ok := flowActionOutputFieldValue(storage.StringValue("unmodeled"), storage.Field{Type: kind}); ok {
			t.Fatal("unsupported output type silently accepted")
		}
	}
	frame := newFlowFrame()
	frame.actionOutputReceiver("Result")(storage.DateTimeValue("2031-03-11T12:00:00Z"))
	receive := frame.actionOutputReceiver("Result")
	if _, stale := frame.actionOutputs["result"]; stale {
		t.Fatal("previous invocation leaked into the next output")
	}
	receive(storage.NullValue())
	if got, found := frame.actionOutputs["result"]; !found || got.Kind != storage.ValueNull {
		t.Fatalf("missing positional output was not stored as null: %+v, %t", got, found)
	}
}
