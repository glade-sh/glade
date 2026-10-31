package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowFrameCaptureScope(t *testing.T) {
	variables := []storage.FlowVariable{
		{Name: "UnsetText", DataType: "String"},
		{Name: "UnsetNumber", DataType: "Number"},
		{Name: "UnsetFlag", DataType: "Boolean"},
	}
	t.Run("subflow_seed_keeps_unset_scalars_absent", func(t *testing.T) {
		frame := newFlowFrame()
		flowCoreSeedScalarDefaults(variables, frame)
		for _, v := range variables {
			if _, ok := frame.scalars[flowFrameKey(v.Name)]; ok {
				t.Errorf("%s unexpectedly seeded", v.Name)
			}
		}
	})
	for _, tc := range []struct {
		name, trigger, mode string
		invocable           bool
	}{
		{"update_route", "RecordBeforeSave", "Update", false},
		{"create_and_update_route", "RecordBeforeSave", "CreateAndUpdate", false},
		{"invocable_route", "RecordBeforeSave", "Create", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			engine := NewEngine(&org)
			engine.InvocableFlow = tc.invocable
			def := storage.ObjectDefinition{APIName: "Account", FlowRules: []storage.FlowRule{{Name: "Probe", TriggerType: tc.trigger, RecordTriggerType: tc.mode}}}
			rec := storage.Record{Object: "Account", Fields: map[string]storage.Value{}}
			frame := engine.newFlowStepFrame("Probe", &rec, def, nil, nil, variables)
			if _, ok := frame.scalars[flowFrameKey("UnsetText")]; ok {
				t.Error("uncaptured route unexpectedly seeded String")
			}
		})
	}
	for _, trigger := range []string{"RecordBeforeSave", "RecordAfterSave"} {
		t.Run(trigger, func(t *testing.T) {
			org := testOrg()
			engine := NewEngine(&org)
			def := storage.ObjectDefinition{APIName: "Account", FlowRules: []storage.FlowRule{{Name: "Probe", TriggerType: trigger, RecordTriggerType: "Create"}}}
			rec := storage.Record{Object: "Account", Fields: map[string]storage.Value{}}
			frame := engine.newFlowStepFrame("Probe", &rec, def, nil, nil, variables)
			if _, ok := frame.scalars[flowFrameKey("UnsetText")]; ok {
				t.Error("unset String unexpectedly seeded outside formula evaluation")
			}
			for _, name := range []string{"UnsetNumber", "UnsetFlag"} {
				if _, ok := frame.scalars[flowFrameKey(name)]; ok {
					t.Errorf("uncaptured %s unexpectedly seeded", name)
				}
			}
		})
	}
}

func TestFlowRelationshipIDPriorNonTextTargets(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	def := storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"AccountId": {APIName: "AccountId", Type: storage.FieldReference, RelationshipName: "Account", ReferenceTo: []string{"Account"}}}}
	for _, tc := range []struct {
		name   string
		target storage.FieldType
		parent storage.Value
		kind   storage.ValueKind
	}{
		{"lookup_populated", storage.FieldReference, storage.IDValue("001000000000001"), storage.ValueID},
		{"lookup_null", storage.FieldReference, storage.NullValue(), storage.ValueNull},
		{"boolean_populated", storage.FieldBoolean, storage.IDValue("001000000000001"), storage.ValueID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"AccountId": tc.parent}}
			value, ok := engine.flowRefsRelationshipIDValue("Account.Id", rec, def, storage.Field{Type: tc.target})
			if !ok || value.Kind != tc.kind {
				t.Fatalf("prior behavior: value=%#v supported=%v; want kind %s", value, ok, tc.kind)
			}
		})
	}
}

func TestFlowFormulaRecordPriorUncapturedBindings(t *testing.T) {
	for _, tc := range []struct {
		name, dataType string
		captured       bool
		want           string
	}{
		{"unframed_string", "String", false, "record"},
		{"framed_number", "Number", true, "record"},
		{"framed_string_default_resolver", "String", true, "record"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			engine := NewEngine(&org)
			engine.InvocableFlow = !tc.captured
			def := storage.ObjectDefinition{APIName: "Account", FlowRules: []storage.FlowRule{{Name: "Probe", TriggerType: "RecordBeforeSave", RecordTriggerType: "Create"}}}
			rec := storage.Record{Object: "Account", Fields: map[string]storage.Value{"item": storage.StringValue("record")}}
			frame := engine.newFlowStepFrame("Probe", &rec, def, nil, nil, []storage.FlowVariable{{Name: "Item", DataType: tc.dataType}})
			frame.scalars["item"] = storage.StringValue("variable")
			value := flowFrameFormulaRecord(frame).Fields["item"]
			if value.Kind != storage.ValueString || value.String != tc.want {
				t.Fatalf("got %#v; want %s", value, tc.want)
			}
		})
	}
}
