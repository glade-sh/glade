package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// The native unset-variable decision is before-save. After-save retains the
// shipping record-field fallback, even while after-save create bindings remain enabled.
func TestFlowAfterSaveDecisionUsesPriorUnsetFallback(t *testing.T) {
	for _, name := range []string{"Site", "Description"} {
		for _, trigger := range []string{"RecordBeforeSave", "RecordAfterSave"} {
			t.Run(name+"/"+trigger, func(t *testing.T) {
				org := storage.NewOrgState()
				storage.EnsureStandardObject(&org, "Account")
				engine := NewEngine(&org)
				def := org.Objects["Account"].Definition
				def.FlowRules = []storage.FlowRule{{Name: "Probe", TriggerType: trigger, RecordTriggerType: "Create"}}
				rec := storage.Record{Object: "Account", Fields: map[string]storage.Value{name: storage.StringValue("record")}}
				frame := engine.newFlowStepFrame("Probe", &rec, def, nil, nil, []storage.FlowVariable{{Name: name, DataType: "String"}})
				got, ok := engine.evaluateFlowDecisionFormula("ISBLANK("+name+")", rec, def, frame)
				if !ok || got != (trigger == "RecordBeforeSave") {
					t.Fatalf("decision=%v supported=%v", got, ok)
				}
				if trigger == "RecordAfterSave" && !frame.capturedFormulaBindings {
					t.Fatal("after-save create bindings lost")
				}
			})
		}
	}
}
