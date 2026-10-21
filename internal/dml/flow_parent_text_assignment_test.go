package dml

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowBeforeSaveParentTextAssignment(t *testing.T) {
	// The owned native matrix captures a one-parent Text reference assigned
	// to a scalar String in a before-save CreateAndUpdate Flow. Guard cases
	// bound local support; compiler-to-runtime controls separately verify the
	// retained unsupported boundary without claiming native conversion rules.
	for _, tc := range []struct {
		name, source, trigger, mode, dataType, target, formula string
		parentID                                               storage.ID
		parentValue                                            storage.Value
		want                                                   storage.Value
		collection, reject                                     bool
	}{
		{name: "populated_parent", parentID: "001000000000001", parentValue: storage.StringValue("Marker Alpha"), want: storage.StringValue("Marker Alpha")},
		{name: "null_parent_text", parentID: "001000000000001", parentValue: storage.NullValue(), want: storage.NullValue()},
		{name: "null_lookup", parentValue: storage.StringValue("Marker Alpha"), want: storage.NullValue()},
		{name: "changed_parent", parentID: "001000000000002", parentValue: storage.StringValue("Marker Alpha"), want: storage.StringValue("Marker Beta")},
		{name: "direct_field_control", source: "$Record.LastName", want: storage.StringValue("Meadow direct")},
		// The integer-to-String native rows preserve null and format integers
		// as text; other source types retain their unsupported boundary.
		{name: "direct_unknown_field_control", source: "$Record.Missing", reject: true},
		{name: "direct_integer", source: "$Record.Counter__c", want: storage.StringValue("9")},
		{name: "direct_formula_control", source: "$Record.Formula__c", reject: true},
		{name: "unknown_relationship_control", source: "$Record.Missing.Description", reject: true},
		{name: "unknown_parent_field_control", source: "$Record.Account.Missing", reject: true},
		{name: "nontext_parent_control", source: "$Record.Account.NumberOfEmployees", reject: true},
		{name: "mixed_formula_control", formula: `"Keep"`, reject: true},
		{name: "formula_parent_control", source: "$Record.Account.Formula__c", reject: true},
		{name: "multihop_control", source: "$Record.Account.Parent.Description", reject: true},
		{name: "collection_control", collection: true, reject: true},
		{name: "after_save_control", trigger: "RecordAfterSave", want: storage.NullValue()},
		{name: "update_only_control", mode: "Update", want: storage.NullValue()},
		{name: "record_target_control", target: "pending.Description", dataType: "SObject", reject: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			account := org.Objects["Account"]
			account.Definition.Fields["Description"] = storage.Field{APIName: "Description", Type: storage.FieldString}
			account.Definition.Fields["NumberOfEmployees"] = storage.Field{APIName: "NumberOfEmployees", Type: storage.FieldInteger}
			account.Definition.Fields["Formula__c"] = storage.Field{APIName: "Formula__c", Type: storage.FieldString, Formula: "Name"}
			account.Definition.Fields["ParentId"] = storage.Field{APIName: "ParentId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Parent"}
			account.Records["001000000000001"] = storage.Record{Object: "Account", ID: "001000000000001", Fields: map[string]storage.Value{
				"Name": storage.StringValue("Meadow A"), "Description": tc.parentValue, "NumberOfEmployees": storage.IntegerValue(7),
			}}
			account.Records["001000000000002"] = storage.Record{Object: "Account", ID: "001000000000002", Fields: map[string]storage.Value{
				"Name": storage.StringValue("Meadow B"), "Description": storage.StringValue("Marker Beta"),
			}}
			org.Objects["Account"] = account
			contact := storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Contact", KeyPrefix: "003", Fields: map[string]storage.Field{
				"AccountId":  {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
				"LastName":   {APIName: "LastName", Type: storage.FieldString},
				"Counter__c": {APIName: "Counter__c", Type: storage.FieldInteger},
				"Formula__c": {APIName: "Formula__c", Type: storage.FieldString, Formula: "LastName"},
				"Email":      {APIName: "Email", Type: storage.FieldString},
			}}, Records: make(map[storage.ID]storage.Record)}
			trigger, mode := tc.trigger, tc.mode
			if trigger == "" {
				trigger = "RecordBeforeSave"
			}
			if mode == "" {
				mode = "CreateAndUpdate"
			}
			rule := storage.FlowRule{Name: "CopyParentText", TriggerType: trigger, RecordTriggerType: mode}
			contact.Definition.FlowRules = []storage.FlowRule{rule}
			org.Objects["Contact"] = contact
			source := storage.Record{Object: "Contact", Fields: map[string]storage.Value{
				"LastName": storage.StringValue("Meadow direct"), "Counter__c": storage.IntegerValue(9), "Formula__c": storage.StringValue("Meadow direct"), "Email": storage.StringValue("keep@example.invalid"), "AccountId": storage.NullValue(),
			}}
			if tc.parentID != "" {
				source.Fields["AccountId"] = storage.IDValue(tc.parentID)
			}
			sourceBefore := source.Clone()
			parentBefore := make(map[storage.ID]storage.Record, len(account.Records))
			for id, parent := range account.Records {
				parentBefore[id] = parent.Clone()
			}
			engine := NewEngine(&org)
			frame := newFlowFrame()
			frame.saveDecisions, frame.priorDecisions, frame.beforeSaveDecisions = flowSaveDecisionMode(rule.Name, contact.Definition)
			target, dataType := tc.target, tc.dataType
			if target == "" {
				target = "parentText"
			}
			if dataType == "" {
				dataType = "String"
			}
			variable := storage.FlowVariable{Name: "parentText", DataType: dataType, IsCollection: tc.collection}
			if dataType == "SObject" {
				variable.Name, variable.ObjectType = "pending", "Contact"
			}
			flowCoreSeedScalarDefaults([]storage.FlowVariable{variable}, frame)
			seedFlowFrame(&engine, &source, frame)
			reference := tc.source
			if reference == "" {
				reference = "$Record.Account.Description"
			}
			err := engine.applyFlowAssignmentStep(storage.FlowAssignment{Name: "ReadParent", Target: target, Operator: "Assign", SourceField: reference, Formula: tc.formula}, frame)
			if tc.reject {
				if err == nil {
					t.Fatal("unsupported assignment unexpectedly succeeded")
				}
				if frame.assigned[flowFrameKey(target)] {
					t.Fatal("unsupported assignment marked its target assigned")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				value, present := frame.scalars[flowFrameKey(target)]
				if !present || !storage.ValuesEqual(value, tc.want) {
					t.Fatalf("assigned value=%#v, present=%t; want %#v", value, present, tc.want)
				}
				if !frame.assigned[flowFrameKey(target)] {
					t.Fatal("successful assignment did not mark its target assigned")
				}
			}
			if !reflect.DeepEqual(source, sourceBefore) || !reflect.DeepEqual(frame.records[flowFrameKey("$Record")], sourceBefore) {
				t.Fatal("scalar assignment mutated its triggering record")
			}
			if !reflect.DeepEqual(org.Objects["Account"].Records, parentBefore) {
				t.Fatal("scalar assignment mutated a parent record")
			}
		})
	}
}

func TestFlowBeforeSaveTextScalarFieldUpdate(t *testing.T) {
	// The same native Flow assigns its captured String scalar to a Text
	// field. Other declared resource and destination shapes retain their
	// existing local behavior; these controls do not assert native rejection.
	for _, tc := range []struct {
		name, source, field, trigger, mode, dataType, formula string
		value                                                 storage.Value
		unassigned, undeclared, collection, reject, collision bool
	}{
		{name: "assigned_text", value: storage.StringValue("Marker Alpha")},
		{name: "assigned_null", value: storage.NullValue()},
		{name: "direct_source_control", source: "Name", value: storage.StringValue("Meadow direct")},
		{name: "field_name_collision_control", source: "Name", value: storage.StringValue("Meadow direct"), collision: true},
		{name: "unknown_scalar_control", source: "missing", reject: true},
		{name: "unassigned_scalar_control", unassigned: true, reject: true},
		{name: "undeclared_scalar_control", undeclared: true, reject: true},
		{name: "numeric_scalar_control", dataType: "Number", value: storage.IntegerValue(3), reject: true},
		{name: "collection_control", collection: true, reject: true},
		{name: "nontext_destination_control", field: "Counter__c", reject: true},
		{name: "unknown_destination_control", field: "Missing", reject: true},
		{name: "calculated_destination_control", field: "Formula__c", reject: true},
		{name: "identity_destination_control", field: "Id", reject: true},
		{name: "after_save_control", trigger: "RecordAfterSave", reject: true},
		{name: "update_only_control", mode: "Update", reject: true},
		{name: "mixed_formula_control", formula: `"Keep"`, reject: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			object := org.Objects["Account"]
			object.Definition.Fields["Description"] = storage.Field{APIName: "Description", Type: storage.FieldString}
			object.Definition.Fields["Counter__c"] = storage.Field{APIName: "Counter__c", Type: storage.FieldInteger}
			object.Definition.Fields["Formula__c"] = storage.Field{APIName: "Formula__c", Type: storage.FieldString, Formula: "Name"}
			trigger, mode := tc.trigger, tc.mode
			if trigger == "" {
				trigger = "RecordBeforeSave"
			}
			if mode == "" {
				mode = "CreateAndUpdate"
			}
			rule := storage.FlowRule{Name: "CopyScalarText", TriggerType: trigger, RecordTriggerType: mode}
			object.Definition.FlowRules = []storage.FlowRule{rule}
			org.Objects["Account"] = object
			record := storage.Record{Object: "Account", Fields: map[string]storage.Value{
				"Name": storage.StringValue("Meadow direct"), "Description": storage.StringValue("Before"),
			}, ExplicitNulls: make(map[string]bool)}
			before := record.Clone()
			engine := NewEngine(&org)
			frame := newFlowFrame()
			frame.saveDecisions, frame.priorDecisions, frame.beforeSaveDecisions = flowSaveDecisionMode(rule.Name, object.Definition)
			dataType := tc.dataType
			if dataType == "" {
				dataType = "String"
			}
			variableName, scalar := "textValue", tc.value
			if tc.collision {
				variableName, scalar = "Name", storage.StringValue("Shadow scalar")
			}
			if !tc.undeclared {
				flowCoreSeedScalarDefaults([]storage.FlowVariable{{Name: variableName, DataType: dataType, IsCollection: tc.collection}}, frame)
			}
			frame.scalars[flowFrameKey(variableName)] = scalar
			frame.assigned[flowFrameKey(variableName)] = !tc.unassigned
			seedFlowFrame(&engine, &record, frame)
			source, field := tc.source, tc.field
			if source == "" {
				source = "textValue"
			}
			if field == "" {
				field = "Description"
			}
			step := storage.FlowStep{Kind: "fieldUpdate", FieldUpdates: []storage.WorkflowFieldUpdate{{Name: "WriteText", Field: field, SourceField: source, Formula: tc.formula}}}
			changed, err := engine.applyFlowStep(rule.Name, "Account", &record, object.Definition, step, frame)
			if tc.reject {
				if err == nil || changed {
					t.Fatalf("unsupported scalar update: changed=%t, err=%v", changed, err)
				}
				if !reflect.DeepEqual(record, before) {
					t.Fatal("unsupported scalar update changed the record")
				}
				return
			}
			if err != nil || !changed {
				t.Fatalf("scalar update: changed=%t, err=%v", changed, err)
			}
			value, present := record.Fields["Description"]
			if tc.value.Kind == storage.ValueNull {
				if present || !record.ExplicitNulls["Description"] {
					t.Fatalf("null scalar did not clear the Text field: %#v", record)
				}
			} else if !present || !storage.ValuesEqual(value, tc.value) {
				t.Fatalf("updated value=%#v, present=%t; want %#v", value, present, tc.value)
			}
			if !storage.ValuesEqual(record.Fields["Name"], before.Fields["Name"]) {
				t.Fatal("scalar update changed an unassigned field")
			}
		})
	}
}
