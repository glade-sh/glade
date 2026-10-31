package vm

import (
	"fmt"
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Unobserved schema shapes must retain the previous behavior. The native
// custom checkbox metadata is a non-nillable custom field; see
// testdata/linked_checkbox_review/org.tsv:2-3,13-14 and objects metadata.
func TestLinkedCheckboxReviewScopeControls(t *testing.T) {
	for _, tc := range []struct {
		name  string
		field storage.Field
	}{
		{"standard-field", storage.Field{APIName: "Flag", Type: storage.FieldBoolean, DefaultValue: "true", Nillable: storage.BoolFlag(false)}},
		{"nullable-custom", storage.Field{APIName: "Flag__c", Type: storage.FieldBoolean, DefaultValue: "true", Nillable: storage.BoolFlag(true)}},
		{"unknown-nillability", storage.Field{APIName: "Flag__c", Type: storage.FieldBoolean, DefaultValue: "true"}},
	} {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(fmt.Sprintf("%s/%s", api, tc.name), func(t *testing.T) {
				org := storage.NewOrgState()
				org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Parent__c", Fields: map[string]storage.Field{tc.field.APIName: tc.field}}}
				m := New(nil)
				m.SetOrg(&org)
				m.currentMethod.APIVersion = api
				row := Object("Parent__c")
				setExplicitSObjectField(&row, tc.field.APIName, Null)
				got, err := m.lookupPath(row, []string{tc.field.APIName})
				if err != nil || got.Kind != ValueBool || !got.Bool {
					t.Fatalf("unobserved schema scope: %#v err=%v; want preserved true", got, err)
				}
				idOnly := Object("Parent__c")
				idOnly.Fields["Id"] = String("a01000000000001AAA")
				got, handled, err := m.callSObjectMember(idOnly, "get", []Value{String(tc.field.APIName)})
				if err != nil || !handled || got.Kind != ValueBool || got.Bool {
					t.Fatalf("unobserved ID-only get: %#v handled=%t err=%v; want preserved false", got, handled, err)
				}
			})
		}
	}
}

// The capture covers caller, ID-only, clone and queried views, not trigger
// snapshots; keep the old missing-Boolean behavior for trigger receivers.
func TestLinkedCheckboxReviewTriggerScopeControl(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Parent__c", Fields: map[string]storage.Field{
		"Flag__c": {APIName: "Flag__c", Type: storage.FieldBoolean, DefaultValue: "true", Nillable: storage.BoolFlag(false)},
	}}}
	m := New(nil)
	m.SetOrg(&org)
	row := Object("Parent__c")
	row.Fields["Id"] = String("a01000000000001AAA")
	row.Fields[sobjectTriggerField] = Bool(true)
	got, handled, err := m.callSObjectMember(row, "get", []Value{String("Flag__c")})
	if err != nil || !handled || got.Kind != ValueBool || got.Bool {
		t.Fatalf("trigger control=%#v handled=%t err=%v; want preserved false", got, handled, err)
	}
}

// The native custom-checkbox null capture is on fresh and linked caller
// records, not trigger, parent-projection, or queried receiver snapshots.
func TestLinkedCheckboxReviewMaterializedScopeControls(t *testing.T) {
	field := storage.Field{APIName: "Flag__c", Type: storage.FieldBoolean, DefaultValue: "true", Nillable: storage.BoolFlag(false)}
	def := storage.ObjectDefinition{APIName: "Parent__c", Fields: map[string]storage.Field{"Flag__c": field}}
	for _, marker := range []string{sobjectTriggerField, sobjectParentProjectionField, sobjectQueriedFieldsField, sobjectCloneMarkerField} {
		t.Run(marker, func(t *testing.T) {
			row := Object("Parent__c")
			setExplicitSObjectField(&row, "Flag__c", Null)
			if marker == sobjectQueriedFieldsField {
				row.Fields[marker] = Map()
			} else {
				row.Fields[marker] = Bool(true)
			}
			got := coerceDottedSObjectBooleanNull(row, Null, "Flag__c", def, field)
			if got.Kind != ValueBool || !got.Bool {
				t.Fatalf("materialized scope=%#v, want prior true", got)
			}
		})
	}
}

// Contact is the captured standard object; other standard objects retain their
// previous explicit-null behavior until native rows establish their behavior.
func TestLinkedCheckboxReviewUnobservedStandardObjects(t *testing.T) {
	field := storage.Field{APIName: "Flag__c", Type: storage.FieldBoolean, DefaultValue: "true", Nillable: storage.BoolFlag(false)}
	for _, name := range []string{"Account", "Product2"} {
		t.Run(name, func(t *testing.T) {
			row := Object(name)
			setExplicitSObjectField(&row, "Flag__c", Null)
			got := coerceDottedSObjectBooleanNull(row, Null, "Flag__c", storage.ObjectDefinition{APIName: name}, field)
			if got.Kind != ValueBool || !got.Bool {
				t.Fatalf("unobserved object read=%#v; want prior true", got)
			}
		})
	}
}

// API 62/67 ID-only callers retain the omitted in-memory checkbox view even
// when a stored record has the default true: org.tsv:8-9,19-20.
func TestLinkedCheckboxReviewIDOnlyStoredDefault(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			const id = storage.ID("a01000000000001AAA")
			org := storage.NewOrgState()
			org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Parent__c", Fields: map[string]storage.Field{
				"Flag__c": {APIName: "Flag__c", Type: storage.FieldBoolean, DefaultValue: "true", Nillable: storage.BoolFlag(false)},
			}}, Records: map[storage.ID]storage.Record{id: {ID: id, Object: "Parent__c", Fields: map[string]storage.Value{"Flag__c": storage.BooleanValue(true)}}}}
			m := New(nil)
			m.SetOrg(&org)
			m.currentMethod.APIVersion = api
			row := Object("Parent__c")
			row.Fields["Id"] = String(string(id))
			got, err := m.lookupPath(row, []string{"Flag__c"})
			if err != nil || got.Kind != ValueBool || got.Bool {
				t.Fatalf("ID-only dotted=%#v err=%v; native false", got, err)
			}
			got, handled, err := m.callSObjectMember(row, "get", []Value{String("Flag__c")})
			if err != nil || !handled || got.Kind != ValueNull {
				t.Fatalf("ID-only dynamic=%#v handled=%t err=%v; native null", got, handled, err)
			}
		})
	}
}
