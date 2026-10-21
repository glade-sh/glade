package dml

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowRecordUpdateDeclaredEmptyInputs(t *testing.T) {
	// Owned API 62 anonymous and named observations distinguish an untouched
	// record variable and an empty collection from a partial record without Id.
	// The malformed-resource controls preserve local boundaries, without
	// asserting native diagnostic wording for those uncaptured shapes.
	for _, tc := range []struct {
		name       string
		prepare    func(*flowFrame, *storage.FlowVariable)
		undeclared bool
		wantError  bool
		wantCount  int
	}{
		{name: "untouched_scalar"},
		{name: "untouched_collection", prepare: func(_ *flowFrame, variable *storage.FlowVariable) {
			variable.IsCollection = true
		}},
		{name: "partial_missing_id", wantError: true, prepare: func(frame *flowFrame, _ *storage.FlowVariable) {
			frame.records["pending"] = storage.Record{Object: "Account", Fields: map[string]storage.Value{"Description": storage.StringValue("After")}}
		}},
		{name: "undeclared", undeclared: true, wantError: true},
		{name: "primitive", wantError: true, prepare: func(_ *flowFrame, variable *storage.FlowVariable) {
			variable.DataType = "String"
		}},
		{name: "missing_object_type", wantError: true, prepare: func(_ *flowFrame, variable *storage.FlowVariable) {
			variable.ObjectType = ""
		}},
		{name: "non_sobject_type", wantError: true, prepare: func(_ *flowFrame, variable *storage.FlowVariable) {
			variable.DataType = "Apex"
		}},
		{name: "explicit_null_scalar", wantError: true, prepare: func(frame *flowFrame, _ *storage.FlowVariable) {
			frame.scalars["pending"] = storage.NullValue()
		}},
		{name: "explicit_text_scalar", wantError: true, prepare: func(frame *flowFrame, _ *storage.FlowVariable) {
			frame.scalars["pending"] = storage.StringValue("Not a record")
		}},
		{name: "initialized_empty_collection_control", prepare: func(frame *flowFrame, variable *storage.FlowVariable) {
			variable.IsCollection = true
			frame.collections["pending"] = []storage.Record{}
		}},
		{name: "populated_scalar_control", wantCount: 1, prepare: func(frame *flowFrame, _ *storage.FlowVariable) {
			frame.records["pending"] = storage.Record{Object: "Account", ID: "001000000000001", Fields: map[string]storage.Value{"Description": storage.StringValue("After")}}
		}},
		{name: "populated_collection_control", wantCount: 1, prepare: func(frame *flowFrame, variable *storage.FlowVariable) {
			variable.IsCollection = true
			frame.collections["pending"] = []storage.Record{{Object: "Account", ID: "001000000000001", Fields: map[string]storage.Value{"Description": storage.StringValue("After")}}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			object := org.Objects["Account"]
			object.Definition.Fields["Description"] = storage.Field{APIName: "Description", Type: storage.FieldString}
			object.Records["001000000000001"] = storage.Record{Object: "Account", ID: "001000000000001", Fields: map[string]storage.Value{"Name": storage.StringValue("Keep"), "Description": storage.StringValue("Before")}}
			object.Records["001000000000002"] = storage.Record{Object: "Account", ID: "001000000000002", Fields: map[string]storage.Value{"Name": storage.StringValue("Other"), "Description": storage.StringValue("Unchanged")}}
			org.Objects["Account"] = object
			before := org.Clone()
			frame := newFlowFrame()
			variable := storage.FlowVariable{Name: "pending", DataType: "SObject", ObjectType: "Account", IsInput: true, IsOutput: true}
			if tc.prepare != nil {
				tc.prepare(frame, &variable)
			}
			if !tc.undeclared {
				flowCoreSeedScalarDefaults([]storage.FlowVariable{variable}, frame)
			}
			engine := NewEngine(&org)
			count, err := engine.executeFlowRecordUpdate(storage.FlowRecordUpdate{
				Name: "PersistPending", ObjectName: "Account", InputReference: "pending",
			}, storage.Record{}, storage.ObjectDefinition{}, frame)
			if (err != nil) != tc.wantError || count != tc.wantCount {
				t.Fatalf("updated=%d, err=%v; want count=%d, error=%t", count, err, tc.wantCount, tc.wantError)
			}
			if tc.wantCount == 0 {
				if !reflect.DeepEqual(org.Objects["Account"].Records, before.Objects["Account"].Records) {
					t.Fatal("empty or rejected input changed stored records")
				}
				return
			}
			stored := org.Objects["Account"].Records["001000000000001"]
			if stored.Fields["Description"].String != "After" || stored.Fields["Name"].String != "Keep" || len(org.Objects["Account"].Records) != 2 {
				t.Fatalf("populated input changed unexpected fields or record count: %#v", org.Objects["Account"].Records)
			}
			if !reflect.DeepEqual(org.Objects["Account"].Records["001000000000002"], before.Objects["Account"].Records["001000000000002"]) {
				t.Fatal("populated input changed an unselected record")
			}
		})
	}
}

func TestFlowRecordTriggeredUpdateUntouchedScalar(t *testing.T) {
	org := testOrg()
	object := org.Objects["Account"]
	object.Definition.Fields["Description"] = storage.Field{APIName: "Description", Type: storage.FieldString}
	object.Definition.FlowRules = []storage.FlowRule{{
		Name: "LeavePendingUntouched", Active: true, TriggerType: "RecordAfterSave", RecordTriggerType: "Create",
		Variables: []storage.FlowVariable{{Name: "pending", DataType: "SObject", ObjectType: "Account", IsInput: true, IsOutput: true}},
		Steps: []storage.FlowStep{{Kind: "decision", Branches: []storage.FlowBranch{{Name: "Default", Default: true, Steps: []storage.FlowStep{{
			Kind: "recordUpdate", RecordUpdate: storage.FlowRecordUpdate{Name: "PersistPending", ObjectName: "Account", InputReference: "pending"},
		}}}}}},
	}}
	org.Objects["Account"] = object
	engine := NewEngine(&org)
	results := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Sprout"), "Description": storage.StringValue("Before")}}})
	if len(results) != 1 || !results[0].Success {
		t.Fatalf("insert with untouched record variable failed: %+v", results)
	}
	stored := org.Objects["Account"].Records[results[0].ID]
	if len(org.Objects["Account"].Records) != 1 || stored.Fields["Name"].String != "Sprout" || stored.Fields["Description"].String != "Before" {
		t.Fatalf("untouched variable changed inserted record: %#v", org.Objects["Account"].Records)
	}
}

func TestRunAutolaunchedFlowUntouchedRecordDefault(t *testing.T) {
	for _, tc := range []struct {
		name      string
		input     FlowInterviewInput
		wantError bool
		populated bool
	}{
		{name: "omitted_record_default"},
		{name: "explicit_empty_record_control", wantError: true, input: FlowInterviewInput{Records: map[string]storage.Record{
			"pending": {Object: "Account", Fields: map[string]storage.Value{}},
		}}},
		{name: "explicit_null_scalar_control", wantError: true, input: FlowInterviewInput{Scalars: map[string]storage.Value{
			"pending": storage.NullValue(),
		}}},
		{name: "partial_missing_id_control", wantError: true, input: FlowInterviewInput{Records: map[string]storage.Record{
			"pending": {Object: "Account", Fields: map[string]storage.Value{"Description": storage.StringValue("After")}},
		}}},
		{name: "populated_record_control", populated: true, input: FlowInterviewInput{Records: map[string]storage.Record{
			"pending": {Object: "Account", ID: "001000000000001", Fields: map[string]storage.Value{"Description": storage.StringValue("After")}},
		}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			object := org.Objects["Account"]
			object.Definition.Fields["Description"] = storage.Field{APIName: "Description", Type: storage.FieldString}
			object.Records["001000000000001"] = storage.Record{Object: "Account", ID: "001000000000001", Fields: map[string]storage.Value{"Name": storage.StringValue("Keep"), "Description": storage.StringValue("Before")}}
			org.Objects["Account"] = object
			before := org.Clone()
			rule := storage.FlowRule{
				Name: "PersistOptionalRecord", Active: true, ProcessType: "AutoLaunchedFlow",
				Variables: []storage.FlowVariable{{Name: "pending", DataType: "SObject", ObjectType: "Account", IsInput: true, IsOutput: true}},
				Steps: []storage.FlowStep{{Kind: "recordUpdate", RecordUpdate: storage.FlowRecordUpdate{
					Name: "PersistPending", ObjectName: "Account", InputReference: "pending",
				}}},
			}
			engine := NewEngine(&org)
			output, err := engine.RunAutolaunchedFlowWithOutput(rule, tc.input)
			if (err != nil) != tc.wantError {
				t.Fatalf("flow error=%v; want error=%t", err, tc.wantError)
			}
			if tc.name == "omitted_record_default" {
				// Preserve the existing local output representation. The native
				// cells establish no-op execution, not getVariableValue's shape.
				blank, exists := output.Records["pending"]
				if !exists || blank.Object != "Account" || blank.ID != "" || len(blank.Fields) != 0 {
					t.Fatalf("omitted record's existing output shape changed: %#v", output.Records)
				}
			}
			if !tc.populated {
				if !reflect.DeepEqual(org.Objects["Account"].Records, before.Objects["Account"].Records) {
					t.Fatal("omitted or rejected record changed stored records")
				}
				return
			}
			stored := org.Objects["Account"].Records["001000000000001"]
			if len(org.Objects["Account"].Records) != 1 || stored.Fields["Name"].String != "Keep" || stored.Fields["Description"].String != "After" {
				t.Fatalf("populated input changed unexpected fields or record count: %#v", org.Objects["Account"].Records)
			}
		})
	}
}

func TestRunAutolaunchedFlowLoopReplacementKeepsMissingIdentity(t *testing.T) {
	org := testOrg()
	before := org.Clone()
	engine := NewEngine(&org)
	_, err := engine.RunAutolaunchedFlowWithOutput(storage.FlowRule{
		Name: "PersistLoopRecord", Active: true, ProcessType: "AutoLaunchedFlow",
		Variables: []storage.FlowVariable{
			{Name: "pending", DataType: "SObject", ObjectType: "Account", IsInput: true, IsOutput: true},
			{Name: "rows", DataType: "SObject", ObjectType: "Account", IsCollection: true, IsInput: true},
		},
		Steps: []storage.FlowStep{{Kind: "loop", Loop: storage.FlowLoop{
			Name: "EachPending", CollectionReference: "rows", CurrentItemReference: "pending",
			Steps: []storage.FlowStep{{Kind: "recordUpdate", RecordUpdate: storage.FlowRecordUpdate{
				Name: "PersistPending", ObjectName: "Account", InputReference: "pending",
			}}},
		}}},
	}, FlowInterviewInput{Collections: map[string][]storage.Record{
		"rows": {{Object: "Account", Fields: map[string]storage.Value{}}},
	}})
	// A real empty loop item replaces the optional default. Preserve its
	// existing missing-ID rejection without claiming native diagnostic text.
	if err == nil {
		t.Fatal("explicit empty loop record was treated as an omitted default")
	}
	if !reflect.DeepEqual(org.Objects["Account"].Records, before.Objects["Account"].Records) {
		t.Fatal("rejected loop record changed stored records")
	}
}

func TestFlowRecordUpdateMissingIdentityMarkerKeepsEarlierFailures(t *testing.T) {
	// These are local error-classification controls. The native observations
	// establish catchability of a partial scalar record's missing-ID failure,
	// not new behavior for malformed or read-only record inputs.
	for _, tc := range []struct {
		name       string
		objectName string
		collection bool
		wantMarker bool
		wantCause  string
	}{
		{name: "earlier_read_only_failure", objectName: "MeadowConfig__mdt", wantCause: "dml: custom metadata object MeadowConfig__mdt is read-only"},
		{name: "actual_scalar_missing_id", objectName: "Account", wantMarker: true, wantCause: "dml: update requires id"},
		{name: "collection_missing_id_control", objectName: "Account", collection: true, wantCause: "dml: update requires id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			object := org.Objects["Account"]
			object.Definition.Fields["Description"] = storage.Field{APIName: "Description", Type: storage.FieldString}
			org.Objects["Account"] = object
			org.Objects["MeadowConfig__mdt"] = storage.ObjectState{
				Definition: storage.ObjectDefinition{
					APIName: "MeadowConfig__mdt", Metadata: map[string]string{"kind": "customMetadata"},
					Fields: map[string]storage.Field{"Description": {APIName: "Description", Type: storage.FieldString}},
				},
				Records: make(map[storage.ID]storage.Record),
			}
			frame := newFlowFrame()
			input := storage.Record{Object: tc.objectName, Fields: map[string]storage.Value{"Description": storage.StringValue("After")}}
			if tc.collection {
				frame.collections["pending"] = []storage.Record{input}
			} else {
				frame.records["pending"] = input
			}
			engine := NewEngine(&org)
			count, err := engine.executeFlowRecordUpdate(storage.FlowRecordUpdate{
				Name: "PersistPending", ObjectName: tc.objectName, InputReference: "pending",
			}, storage.Record{}, storage.ObjectDefinition{}, frame)
			if err == nil || count != 0 {
				t.Fatalf("updated=%d, error=%v; want an update error", count, err)
			}
			if got := IsFlowRecordUpdateMissingID(err); got != tc.wantMarker {
				t.Fatalf("missing-ID marker=%t, want %t: %v", got, tc.wantMarker, err)
			}
			if want := "dml: flow record update PersistPending failed: " + tc.wantCause; err.Error() != want {
				t.Fatalf("existing error text changed: %q, want %q", err.Error(), want)
			}
			var cause dmlError
			if !errors.As(err, &cause) || cause.code != "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY" {
				t.Fatalf("existing engine error cause changed: %v", err)
			}
			if len(org.Objects[tc.objectName].Records) != 0 {
				t.Fatal("failed update inserted a record")
			}
		})
	}
}
