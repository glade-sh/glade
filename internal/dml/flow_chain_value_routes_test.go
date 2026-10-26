package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native decision-shape rows VARS, OUT and PARENT pin the save routes;
// direct interviews share the frame and retain their existing operand behavior.
func TestFlowChainVariableOperandsDirectBeforeAndAfterSaveRoutes(t *testing.T) {
	for _, route := range []string{"direct_interview", "before_save", "after_save"} {
		for _, tc := range []struct {
			name, field, op, operand string
			want                     bool
		}{
			{"null_boolean_not_true", "flag", "notEqual", "{!truth}", true},
			{"false_not_true", "falsehood", "notEqual", "{!truth}", true},
			{"same_null_boolean", "flag", "equals", "{!flag}", true},
			{"different_null_boolean", "flag", "notEqual", "{!flag}", false},
			{"unassigned_date", "dateValue", "isNull", "true", true},
			{"missing_record_field", "accountValue.Industry", "notEqual", "Energy", true},
		} {
			t.Run(route+"/"+tc.name, func(t *testing.T) {
				org := testOrg()
				account := org.Objects["Account"]
				account.Definition.Fields["Industry"] = storage.Field{APIName: "Industry", Type: storage.FieldString}
				org.Objects["Account"] = account
				engine := NewEngine(&org)
				engine.InvocableFlow = route == "direct_interview"
				frame := newFlowFrame()
				frame.saveDecisions = route != "direct_interview"
				frame.beforeSaveDecisions = route == "before_save"
				flowCoreSeedScalarDefaults([]storage.FlowVariable{{Name: "flag", DataType: "Boolean"}, {Name: "truth", DataType: "Boolean", DefaultValue: flowChainValuePtr(storage.BooleanValue(true))}, {Name: "falsehood", DataType: "Boolean", DefaultValue: flowChainValuePtr(storage.BooleanValue(false))}, {Name: "dateValue", DataType: "Date"}, {Name: "accountValue", DataType: "SObject", ObjectType: "Account"}}, frame)
				branch := storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{{Field: tc.field, Operation: tc.op, Value: tc.operand}}}
				got, supported := engine.evaluateFlowFrameBranch(branch, storage.Record{}, storage.ObjectDefinition{}, frame)
				if !supported || got != tc.want {
					t.Fatalf("matches=%t supported=%t; native=%t", got, supported, tc.want)
				}
			})
		}
	}
}

func TestFlowChainScalarFieldUpdateBeforeSaveCreateRoutes(t *testing.T) {
	for _, mode := range []string{"Create", "CreateAndUpdate"} {
		for _, tc := range []struct {
			name, datatype, field, literal string
			fieldType                      storage.FieldType
			want                           storage.Value
		}{
			{"text", "String", "Description", "Not Applicable", storage.FieldString, storage.StringValue("Not Applicable")},
			{"number", "Number", "NumberOfEmployees", "42", storage.FieldInteger, storage.IntegerValue(42)},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				org := testOrg()
				object := org.Objects["Account"]
				object.Definition.Fields[tc.field] = storage.Field{APIName: tc.field, Type: tc.fieldType}
				object.Definition.FlowRules = []storage.FlowRule{{Name: "Copy", TriggerType: "RecordBeforeSave", RecordTriggerType: mode}}
				org.Objects["Account"] = object
				engine := NewEngine(&org)
				record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Probe")}, ExplicitNulls: map[string]bool{}}
				steps := []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{Name: "Prepare", Target: "scalar", Operator: "Assign", LiteralValue: tc.literal}}, {Kind: "fieldUpdate", FieldUpdates: []storage.WorkflowFieldUpdate{{Name: "Write", Field: tc.field, SourceField: "scalar"}}}}
				if tc.datatype == "Number" {
					// Numeric metadata literals lower to typed formula values.
					steps[0].Assignment.Formula = tc.literal
					steps[0].Assignment.LiteralValue = ""
				}
				_, err := engine.applyFlowSteps("Copy", "Account", &record, object.Definition, steps, nil, []storage.FlowVariable{{Name: "scalar", DataType: tc.datatype}})
				if err != nil {
					t.Fatal(err)
				}
				if got := record.Fields[tc.field]; !storage.ValuesEqual(got, tc.want) {
					t.Fatalf("got=%#v; native=%#v", got, tc.want)
				}
			})
		}
	}
}

func TestFlowChainParentAuditDateRecordUpdateRoute(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	frame := newFlowFrame()
	source := storage.Record{Object: "Contact", System: storage.SystemFields{LastModifiedDate: "2026-10-19T01:02:03.000Z"}}
	seedFlowFrame(&engine, &source, frame)
	for _, kind := range []storage.FieldType{storage.FieldDate, storage.FieldDateTime} {
		t.Run(string(kind), func(t *testing.T) {
			// parent=true: the parent-relationship Update Records path of
			// flow_decision_shapes/org.tsv PARENT:Open (API 62.0/67.0,
			// anonymous and istest, dt=set|dd=gmt). Assertions unchanged.
			value, _, ok := engine.flowRecordUpdateAssignmentValue(storage.Field{Type: kind}, source, storage.WorkflowFieldUpdate{SourceField: "$Record.LastModifiedDate"}, storage.ObjectDefinition{}, frame, true)
			want := storage.DateTimeValue(source.System.LastModifiedDate)
			if kind == storage.FieldDate {
				want = storage.DateValue("2026-10-19")
			}
			if !ok || !storage.ValuesEqual(value, want) {
				t.Fatalf("value=%#v supported=%t; native=%#v", value, ok, want)
			}
		})
	}
}

func flowChainValuePtr(value storage.Value) *storage.Value { return &value }

func TestFlowChainLookupFormulaAndParentUpdateSaveRoutes(t *testing.T) {
	for _, route := range []string{"direct_interview", "before_save", "after_save"} {
		t.Run(route+"/lookup_formula", func(t *testing.T) {
			org := testOrg()
			engine := NewEngine(&org)
			engine.InvocableFlow = route == "direct_interview"
			frame := newFlowFrame()
			frame.saveDecisions = route != "direct_interview"
			frame.beforeSaveDecisions = route == "before_save"
			definition := storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"Choices__c": {APIName: "Choices__c", Type: storage.FieldString}}}
			frame.lookupOutputs["lookup"] = flowLookupOutput{record: storage.Record{Object: "Contact", Fields: map[string]storage.Value{"Choices__c": storage.StringValue("First;Second")}}, definition: definition}
			for _, tc := range []struct {
				formula string
				want    bool
			}{
				{`INCLUDES(Lookup.Choices__c, "first")`, true},
				{`INCLUDES(Lookup.Choices__c, "Third")`, false},
				{`"Lookup.Choices__c" = "Lookup.Choices__c"`, true},
			} {
				got, ok := engine.evaluateFlowFrameBranch(storage.FlowBranch{Formula: tc.formula}, storage.Record{}, definition, frame)
				if !ok || got != tc.want {
					t.Errorf("%s: matches=%t supported=%t; native=%t", tc.formula, got, ok, tc.want)
				}
			}
		})
	}
	t.Run("after_save/parent_relationship_update", func(t *testing.T) {
		org := testOrg()
		engine := NewEngine(&org)
		inserted := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Open")}}})
		if !inserted[0].Success {
			t.Fatal(inserted)
		}
		definition := storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Parent__r"}}}
		source := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"Parent__c": storage.IDValue(inserted[0].ID)}}
		frame := newFlowFrame()
		seedFlowFrame(&engine, &source, frame)
		update := storage.FlowRecordUpdate{Name: "WriteParent", ObjectName: "Parent__r", InputReference: "$Record.Parent__r", InputAssignments: []storage.WorkflowFieldUpdate{{Field: "Name", LiteralValue: "Changed"}}}
		count, err := engine.executeFlowRecordUpdate(update, source, definition, frame)
		if err != nil || count != 1 {
			t.Fatalf("updated=%d error=%v", count, err)
		}
		_, parent, found := storage.LookupRecordByID(org.Objects["Account"].Records, inserted[0].ID)
		if !found || parent.Fields["Name"].String != "Changed" {
			t.Fatalf("parent=%+v found=%t", parent, found)
		}
	})
}

func TestFlowChainAfterSaveCreateTextFieldUpdateRoute(t *testing.T) {
	org := testOrg()
	object := org.Objects["Account"]
	object.Definition.Fields["Description"] = storage.Field{APIName: "Description", Type: storage.FieldString}
	object.Definition.FlowRules = []storage.FlowRule{{Name: "Copy", TriggerType: "RecordAfterSave", RecordTriggerType: "Create"}}
	org.Objects["Account"] = object
	engine := NewEngine(&org)
	record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Probe")}, ExplicitNulls: map[string]bool{}}
	steps := []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{Name: "Prepare", Target: "scalar", Operator: "Assign", LiteralValue: "1111001001"}}, {Kind: "fieldUpdate", FieldUpdates: []storage.WorkflowFieldUpdate{{Name: "Write", Field: "Description", SourceField: "scalar"}}}}
	_, err := engine.applyFlowSteps("Copy", "Account", &record, object.Definition, steps, nil, []storage.FlowVariable{{Name: "scalar", DataType: "String"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := record.Fields["Description"]; got.Kind != storage.ValueString || got.String != "1111001001" {
		t.Fatalf("native operand result not written: %#v", got)
	}
}
