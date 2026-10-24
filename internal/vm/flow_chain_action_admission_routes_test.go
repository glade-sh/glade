package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowChainActionLookupAndLocalDateInlineAndReferenceRoutes(t *testing.T) {
	for _, shape := range []string{"explicit_lookup_output", "local_date_operand"} {
		for _, route := range []string{"inline", "reference"} {
			t.Run(shape+"/"+route, func(t *testing.T) {
				rule := storage.FlowRule{ProcessType: "AutoLaunchedFlow", Variables: []storage.FlowVariable{{Name: "Result", DataType: "String", IsOutput: true}}}
				step := storage.FlowStep{Kind: "assignment", Assignment: storage.FlowAssignment{Target: "Result", Operator: "Assign", LiteralValue: "ready"}}
				if shape == "explicit_lookup_output" {
					step = storage.FlowStep{Kind: "recordLookup", RecordLookup: storage.FlowRecordLookup{Name: "Find", ObjectName: "Contact", OutputAssignments: []storage.FlowAssignment{{Target: "Result", SourceField: "Title"}}}}
				} else {
					rule.Variables = append(rule.Variables, storage.FlowVariable{Name: "emptyDate", DataType: "Date"})
				}
				rule.Steps = []storage.FlowStep{step}
				if route == "reference" {
					rule.Steps = []storage.FlowStep{{Kind: "reference", Target: "entry"}}
					rule.StepTargets = map[string][]storage.FlowStep{"entry": {step}}
				}
				if !invocableFlowActionSupported(rule) {
					t.Fatal("captured local action route rejected before execution")
				}
			})
		}
	}
}

func TestFlowChainActionAdmissionBoundariesInlineAndReferenceRoutes(t *testing.T) {
	for _, shape := range []string{"date_input", "date_output", "date_collection", "automatic_many", "explicit_record_reference", "no_output_mode"} {
		for _, route := range []string{"inline", "reference"} {
			t.Run(shape+"/"+route, func(t *testing.T) {
				rule := storage.FlowRule{ProcessType: "AutoLaunchedFlow", Variables: []storage.FlowVariable{{Name: "Result", DataType: "String", IsOutput: true}}}
				step := storage.FlowStep{Kind: "assignment", Assignment: storage.FlowAssignment{Target: "Result", Operator: "Assign", LiteralValue: "ready"}}
				switch shape {
				case "date_input", "date_output", "date_collection":
					rule.Variables = append(rule.Variables, storage.FlowVariable{Name: "date", DataType: "Date", IsInput: shape == "date_input", IsOutput: shape == "date_output", IsCollection: shape == "date_collection"})
				default:
					step = storage.FlowStep{Kind: "recordLookup", RecordLookup: storage.FlowRecordLookup{Name: "Find", ObjectName: "Contact"}}
					if shape == "automatic_many" {
						step.RecordLookup.StoreOutputAutomatically = true
					} else if shape == "explicit_record_reference" {
						step.RecordLookup.OutputReference = "record"
						step.RecordLookup.OutputAssignments = []storage.FlowAssignment{{Target: "Result", SourceField: "Title"}}
					}
				}
				rule.Steps = []storage.FlowStep{step}
				if route == "reference" {
					rule.Steps = []storage.FlowStep{{Kind: "reference", Target: "entry"}}
					rule.StepTargets = map[string][]storage.FlowStep{"entry": {step}}
				}
				if invocableFlowActionSupported(rule) {
					t.Fatal("existing action boundary unexpectedly admitted")
				}
			})
		}
	}
}
