package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Source-62 named observations establish these optional, unrestricted values.
func TestMultiSelectEmptySelectionStoredSource62(t *testing.T) {
	field := storage.Field{Type: storage.FieldMultiPicklist}
	for _, source := range []string{";", ";;", "Beta", "NotDefined"} {
		t.Run(source, func(t *testing.T) {
			caller := storage.StringValue(source)
			got := normalizeStoredFieldValue(field, caller)
			if source == ";" || source == ";;" {
				if got.Kind != storage.ValueNull {
					t.Fatalf("stored=%#v, want null", got)
				}
			} else if got.Kind != caller.Kind || got.String != caller.String {
				t.Fatalf("selection changed: got=%#v, want=%#v", got, caller)
			}
			if caller.String != source {
				t.Fatal("caller changed")
			}
		})
	}
}

// These preserve the existing behavior of unmeasured metadata/input shapes.
// They are local scope fences, not new native observations.
func TestMultiSelectEmptySelectionScopeFences(t *testing.T) {
	for _, field := range []storage.Field{
		{Type: storage.FieldString},
		{Type: storage.FieldPicklist},
		{Type: storage.FieldMultiPicklist, Required: true},
		{Type: storage.FieldMultiPicklist, RestrictedPicklist: true},
	} {
		input := storage.StringValue(";")
		if got := normalizeStoredFieldValue(field, input); got.Kind != input.Kind || got.String != input.String {
			t.Fatalf("unobserved field shape changed: field=%#v, value=%#v", field, got)
		}
	}
	for _, source := range []string{"; ;", " Alpha;", "Alpha;;Beta"} {
		input := storage.StringValue(source)
		if got := normalizeStoredFieldValue(storage.Field{Type: storage.FieldMultiPicklist}, input); got.Kind != input.Kind || got.String != input.String {
			t.Fatalf("unobserved selection changed: source=%q, value=%#v", source, got)
		}
	}
}

// The engine path also clears an existing selection without a VM caller.
func TestMultiSelectEmptySelectionEngineUpdate(t *testing.T) {
	for _, source := range []string{";", ";;"} {
		t.Run(source, func(t *testing.T) {
			org := testOrg()
			account := org.Objects["Account"]
			account.Definition.Fields["OwnedMultiTimingSelection__c"] = storage.Field{
				APIName: "OwnedMultiTimingSelection__c", Type: storage.FieldMultiPicklist,
				PicklistValues: []storage.PicklistValue{{Value: "Alpha"}, {Value: "Beta"}, {Value: "Gamma"}},
			}
			org.Objects["Account"] = account
			engine := NewEngine(&org)
			inserted := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{
				"Name": storage.StringValue("Owned selection"), "OwnedMultiTimingSelection__c": storage.StringValue("Alpha"),
			}}})
			if len(inserted) != 1 || !inserted[0].Success {
				t.Fatalf("seed insert: %#v", inserted)
			}
			input := storage.Record{Object: "Account", ID: inserted[0].ID, Fields: map[string]storage.Value{
				"OwnedMultiTimingSelection__c": storage.StringValue(source),
			}}
			updated := engine.Update([]storage.Record{input})
			if len(updated) != 1 || !updated[0].Success {
				t.Fatalf("selection update: %#v", updated)
			}
			stored := org.Objects["Account"].Records[inserted[0].ID]
			if value, present := stored.Fields["OwnedMultiTimingSelection__c"]; present && value.Kind != storage.ValueNull {
				t.Fatalf("seeded selection was not cleared: %#v", value)
			}
			if input.Fields["OwnedMultiTimingSelection__c"].String != source {
				t.Fatal("engine changed the caller input")
			}
		})
	}
}
