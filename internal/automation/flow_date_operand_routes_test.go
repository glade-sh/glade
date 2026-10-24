package automation

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowDateNullOperandDirectBeforeAndAfterSaveLoweringRoutes(t *testing.T) {
	for _, name := range []string{"Cedar_Operands", "Cedar_Before", "Cedar_After"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "apextest", "testdata", "conformance", "flow_operand_routes", "api62", "force-app", "main", "default", "flows", name+".flow-meta.xml")
			flow, diagnostics, err := loadFlow(path)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			var walk func([]storage.FlowStep)
			check := func(branch storage.FlowBranch) {
				if branch.Name != "Match1" {
					return
				}
				found = true
				if len(branch.Criteria) != 1 || branch.Criteria[0].Field != "vDate" || branch.Criteria[0].Operation != "isNull" || branch.Criteria[0].Value != "true" {
					t.Errorf("native null-Date condition lowered as %+v", branch)
				}
			}
			walk = func(steps []storage.FlowStep) {
				for _, step := range steps {
					for _, branch := range step.Branches {
						check(branch)
						walk(branch.Steps)
					}
				}
			}
			for _, rule := range flow.Rules {
				if len(rule.Steps) == 0 || len(rule.Branches) != 0 {
					t.Errorf("declared Date decision must execute inside a frame: steps=%d outer branches=%d", len(rule.Steps), len(rule.Branches))
				}
				walk(rule.Steps)
				for _, steps := range rule.StepTargets {
					walk(steps)
				}
				for _, branch := range rule.Branches {
					check(branch)
					walk(branch.Steps)
				}
			}
			if !found {
				t.Fatalf("Date null branch missing; diagnostics=%v", diagnostics)
			}
		})
	}
}

func TestFlowScalarDecisionBeforeAndAfterSaveEntryRoutes(t *testing.T) {
	for _, trigger := range []string{"RecordBeforeSave", "RecordAfterSave"} {
		for _, dataType := range []string{"Date", "String", "Number", "Boolean"} {
			t.Run(trigger+"/"+dataType, func(t *testing.T) {
				raw := flowXML{
					Start:     flowStartXML{Object: "Contact", TriggerType: trigger, RecordTriggerType: "Create"},
					Variables: []flowVariableXML{{Name: "operand", DataType: dataType}},
					Decisions: []flowDecisionXML{{Name: "First", Rules: []flowDecisionRuleXML{{Name: "Match", Conditions: []flowConditionXML{{LeftValueReference: "operand", Operator: "EqualTo"}}}}}},
				}
				if !flowRequiresOrderedGraph(raw) {
					t.Fatal("scalar decision must enter its variable frame")
				}
				raw.Decisions[0].Rules[0].Conditions[0].LeftValueReference = "$Record.Title"
				if flowRequiresOrderedGraph(raw) {
					t.Fatal("ordinary record-field decisions must retain their existing route")
				}
			})
		}
	}
}

func TestFlowScalarSaveEntryGraphBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, trigger, dataType, operator, source string
		collection                                bool
	}{
		{name: "number_prefix_before_save", trigger: "RecordBeforeSave", dataType: "Number", operator: "StartsWith"},
		{name: "unsupported_text_after_save", trigger: "RecordAfterSave", dataType: "String", operator: "DoesNotContain"},
		{name: "before_delete", trigger: "RecordBeforeDelete", dataType: "Date", operator: "EqualTo"},
		{name: "before_delete_casefold", trigger: "recordbeforedelete", dataType: "Date", operator: "EqualTo"},
		{name: "record_alias_before_save", trigger: "RecordBeforeSave", dataType: "String", operator: "IsNull", source: "$Record.Name"},
		{name: "record_alias_after_save", trigger: "RecordAfterSave", dataType: "String", operator: "IsNull", source: "$Record.Name"},
		{name: "collection", trigger: "RecordAfterSave", dataType: "String", operator: "EqualTo", collection: true},
		{name: "record_variable", trigger: "RecordBeforeSave", dataType: "SObject", operator: "IsNull"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := flowXML{
				Start: flowStartXML{Object: "Contact", TriggerType: tc.trigger},
				Variables: []flowVariableXML{{Name: "operand", DataType: tc.dataType, IsCollection: tc.collection,
					Value: flowValueXML{ElementReference: tc.source}}},
				Decisions: []flowDecisionXML{{Rules: []flowDecisionRuleXML{{Conditions: []flowConditionXML{{LeftValueReference: "operand", Operator: tc.operator}}}}}},
			}
			if flowRequiresOrderedGraph(raw) {
				t.Fatal("outside the captured scalar save-entry routes; retain existing graph selection")
			}
		})
	}
}
