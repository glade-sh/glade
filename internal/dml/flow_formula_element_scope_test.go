package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The captured formulas observe an unset String, but ordinary frame references
// have no native row and retain the shipping-base behavior.
func TestFlowFormulaElementUnsetDoesNotSeedFrame(t *testing.T) {
	for _, trigger := range []string{"RecordBeforeSave", "RecordAfterSave"} {
		t.Run(trigger, func(t *testing.T) {
			org := testOrg()
			engine := NewEngine(&org)
			def := storage.ObjectDefinition{APIName: "Account", FlowRules: []storage.FlowRule{{Name: "Probe", TriggerType: trigger, RecordTriggerType: "Create"}}}
			rec := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Other": storage.StringValue("record")}}
			frame := engine.newFlowStepFrame("Probe", &rec, def, nil, nil, []storage.FlowVariable{{Name: "Other", DataType: "String"}, {Name: "Unused", DataType: "String"}})
			for _, name := range []string{"Other", "Unused"} {
				if _, found := frame.scalars[flowFrameKey(name)]; found {
					t.Errorf("%s leaked into frame", name)
				}
			}
		})
	}
}

func TestFlowCustomErrorUnrelatedFormulaMergePriorBehavior(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	def := storage.ObjectDefinition{APIName: "Account", FlowRules: []storage.FlowRule{{Name: "Probe", TriggerType: "RecordBeforeSave", RecordTriggerType: "Create"}}}
	rec := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("record")}}
	frame := engine.newFlowStepFrame("Probe", &rec, def, nil, nil, []storage.FlowVariable{{Name: "Site", DataType: "String"}, {Name: "Name", DataType: "String"}})
	// A record-only formula merge was not evaluated on the shipping base,
	// including when another formula in the flow reads a String variable.
	got := engine.renderFlowCustomErrorText("{!RecordOnly}", frame, map[string]string{"recordonly": "$Record.Name", "variable": "Site"})
	if got != "{!RecordOnly}" {
		t.Fatalf("record-only merge = %q", got)
	}
}
