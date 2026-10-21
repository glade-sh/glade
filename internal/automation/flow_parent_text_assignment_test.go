package automation

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowParentTextScalarAssignmentKeepsRecordSource(t *testing.T) {
	// Scalar String sources retain the triggering record in every save mode.
	// Other resource kinds and deeper paths retain their existing lowering.
	for _, tc := range []struct {
		name, trigger, mode, target, dataType, source, operator, want string
		collection                                                    bool
	}{
		{name: "parent_text", source: "$Record.Parent__r.Marker__c", want: "$Record.Parent__r.Marker__c"},
		{name: "direct_text", source: "$Record.LastName", want: "$Record.LastName"},
		{name: "after_save_control", trigger: "RecordAfterSave", source: "$Record.Parent__r.Marker__c", want: "$Record.Parent__r.Marker__c"},
		{name: "update_only_control", mode: "Update", source: "$Record.Parent__r.Marker__c", want: "$Record.Parent__r.Marker__c"},
		{name: "create_only_control", mode: "Create", source: "$Record.Parent__r.Marker__c", want: "$Record.Parent__r.Marker__c"},
		{name: "collection_control", collection: true, source: "$Record.Parent__r.Marker__c", want: "Parent__r.Marker__c"},
		{name: "record_target_control", target: "pending.Note__c", dataType: "SObject", source: "$Record.Parent__r.Marker__c", want: "Parent__r.Marker__c"},
		{name: "numeric_target_control", dataType: "Number", source: "$Record.Parent__r.Marker__c", want: "Parent__r.Marker__c"},
		{name: "operator_control", operator: "Add", source: "$Record.Parent__r.Marker__c", want: "$Record.Parent__r.Marker__c"},
		{name: "deeper_path_control", source: "$Record.Parent__r.Parent__r.Marker__c", want: "Parent__r.Parent__r.Marker__c"},
		{name: "other_frame_control", source: "lookup.Marker__c", want: "lookup.Marker__c"},
		{name: "record_direct_control", target: "pending.Note__c", dataType: "SObject", source: "$Record.LastName", want: "$Record.LastName"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			trigger, mode, target, dataType, operator := tc.trigger, tc.mode, tc.target, tc.dataType, tc.operator
			if trigger == "" {
				trigger = "RecordBeforeSave"
			}
			if mode == "" {
				mode = "CreateAndUpdate"
			}
			if target == "" {
				target = "textValue"
			}
			if dataType == "" {
				dataType = "String"
			}
			if operator == "" {
				operator = "Assign"
			}
			variables := map[string]flowVariableXML{
				"textvalue": {Name: "textValue", DataType: dataType, IsCollection: tc.collection},
				"pending":   {Name: "pending", DataType: "SObject", ObjectType: "Contact"},
			}
			raw := flowXML{
				Start: flowStartXML{Object: "Contact", TriggerType: trigger, RecordTriggerType: mode},
				Assignments: []flowAssignmentXML{{Name: "ReadText", Items: []flowAssignmentItemXML{{
					AssignToReference: target, Operator: operator, Value: flowValueXML{ElementReference: tc.source},
				}}}},
			}
			branch, diagnostics := flowPopulateBranchFromTarget(storage.FlowBranch{}, "ReadText", raw, nil, variables)
			if len(diagnostics) != 0 || len(branch.Steps) != 1 || branch.Steps[0].Kind != "assignment" {
				t.Fatalf("assignment lowering: %#v %v", branch.Steps, diagnostics)
			}
			if got := branch.Steps[0].Assignment.SourceField; got != tc.want {
				t.Fatalf("source=%q, want %q", got, tc.want)
			}
		})
	}
}

func TestFlowScalarStringAssignmentKeepsUncapturedDirectFieldsUnsupported(t *testing.T) {
	// These compiler-to-runtime controls preserve Glade's unsupported boundary;
	// they do not assert Salesforce rejects these uncaptured conversions.
	for _, tc := range []struct {
		name, field string
		typeOf      storage.FieldType
		formula     string
		value       storage.Value
	}{
		{name: "boolean", field: "Ready__c", typeOf: storage.FieldBoolean, value: storage.BooleanValue(true)},
		{name: "unknown", field: "Missing__c"},
		{name: "formula", field: "Derived__c", typeOf: storage.FieldString, formula: "LastName", value: storage.StringValue("Meadow")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := flowXML{
				Start: flowStartXML{Object: "Contact", TriggerType: "RecordBeforeSave", RecordTriggerType: "CreateAndUpdate"},
				Assignments: []flowAssignmentXML{{Name: "ReadText", Items: []flowAssignmentItemXML{{
					AssignToReference: "textValue", Operator: "Assign", Value: flowValueXML{ElementReference: "$Record." + tc.field},
				}}}},
			}
			branch, diagnostics := flowPopulateBranchFromTarget(storage.FlowBranch{}, "ReadText", raw, nil, map[string]flowVariableXML{
				"textvalue": {Name: "textValue", DataType: "String"},
			})
			if len(diagnostics) != 0 || len(branch.Steps) != 1 {
				t.Fatalf("assignment lowering: %#v %v", branch.Steps, diagnostics)
			}
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "Contact")
			object := org.Objects["Contact"]
			fields := map[string]storage.Value{"LastName": storage.StringValue("Meadow")}
			if tc.typeOf != "" {
				object.Definition.Fields[tc.field] = storage.Field{APIName: tc.field, Type: tc.typeOf, Formula: tc.formula}
				fields[tc.field] = tc.value
			}
			object.Definition.FlowRules = []storage.FlowRule{{
				Name: "CopyText", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "CreateAndUpdate",
				Variables: []storage.FlowVariable{{Name: "textValue", DataType: "String"}}, Steps: branch.Steps,
			}}
			org.Objects["Contact"] = object
			engine := dml.NewEngine(&org)
			err := engine.ApplyBeforeSaveFlows([]storage.Record{{Object: "Contact", Fields: fields}})
			if err == nil || !strings.Contains(err.Error(), "unsupported value") {
				t.Fatalf("uncaptured direct source must retain unsupported boundary, got %v", err)
			}
		})
	}
}
