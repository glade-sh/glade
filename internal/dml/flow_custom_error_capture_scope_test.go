package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowCapturedCustomErrorFormulaScope(t *testing.T) {
	for _, tc := range []struct {
		name, trigger, message, expression string
		want                               bool
	}{
		{"captured_before", "RecordBeforeSave", "{!Value}", "Name", true},
		{"after_save_uncaptured", "RecordAfterSave", "{!Value}", "Name", false},
		{"record_only_uncaptured", "RecordBeforeSave", "{!Value}", "$Record.Name", false},
		{"unreferenced_formula", "RecordBeforeSave", "plain", "Name", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			engine := NewEngine(&org)
			def := storage.ObjectDefinition{APIName: "Account", FlowRules: []storage.FlowRule{{Name: "Probe", TriggerType: tc.trigger, RecordTriggerType: "Create"}}}
			rec := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("record")}}
			frame := engine.newFlowStepFrame("Probe", &rec, def, nil, nil, []storage.FlowVariable{{Name: "Name", DataType: "String"}})
			if got := flowCapturedCustomErrorFormula(tc.message, map[string]string{"value": tc.expression}, frame); got != tc.want {
				t.Fatalf("captured=%v, want %v", got, tc.want)
			}
		})
	}
}
