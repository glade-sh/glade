package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The owned Flow lookup packet passes an 18-character caller String to an Id
// lookup over the same persisted record. Flow criteria compare record identity
// for a schema-declared Id; an ordinary text field retains text comparison.
func TestFlowLookupInputIDUsesRecordIdentity(t *testing.T) {
	const id15 = "500000000000071"
	const id18 = id15 + "AAA"
	const other15 = "500000000000072"
	const other18 = other15 + "AAA"
	for _, selected := range []struct {
		name, fieldName string
		field           storage.Field
		target, input   storage.Value
		want            bool
	}{
		{"id15String18", "Id", storage.Field{APIName: "Id", Type: storage.FieldID}, storage.IDValue(id15), storage.StringValue(id18), true},
		{"id18String15", "Id", storage.Field{APIName: "Id", Type: storage.FieldID}, storage.IDValue(id18), storage.StringValue(id15), true},
		{"differentId15String18", "Id", storage.Field{APIName: "Id", Type: storage.FieldID}, storage.IDValue(id15), storage.StringValue(other18), false},
		{"differentId18String15", "Id", storage.Field{APIName: "Id", Type: storage.FieldID}, storage.IDValue(id18), storage.StringValue(other15), false},
		{"sameId15", "Id", storage.Field{APIName: "Id", Type: storage.FieldID}, storage.IDValue(id15), storage.StringValue(id15), true},
		{"sameId18", "Id", storage.Field{APIName: "Id", Type: storage.FieldID}, storage.IDValue(id18), storage.StringValue(id18), true},
		{"textKeepsDifferentLengths", "Text__c", storage.Field{APIName: "Text__c", Type: storage.FieldString}, storage.StringValue(id15), storage.StringValue(id18), false},
		{"textKeepsExactMatch", "Text__c", storage.Field{APIName: "Text__c", Type: storage.FieldString}, storage.StringValue(id15), storage.StringValue(id15), true},
		{"textKeepsCaseRule", "Text__c", storage.Field{APIName: "Text__c", Type: storage.FieldString}, storage.StringValue("owned"), storage.StringValue("OWNED"), true},
		// Reference controls retain already-working same-representation cases.
		// No new Flow Reference-field 15/18 observation is claimed here.
		{"referenceSame15", "Link__c", storage.Field{APIName: "Link__c", Type: storage.FieldReference, ReferenceTo: []string{"Case"}}, storage.IDValue(id15), storage.StringValue(id15), true},
		{"referenceDifferent15", "Link__c", storage.Field{APIName: "Link__c", Type: storage.FieldReference, ReferenceTo: []string{"Case"}}, storage.IDValue(id15), storage.StringValue(other15), false},
		// Local preservation controls: invalid IDs must not acquire a new
		// prefix-only identity rule merely from the field's declared type.
		{"invalidDifferentLength", "Id", storage.Field{APIName: "Id", Type: storage.FieldID}, storage.IDValue(id15), storage.StringValue(id15 + "invalid"), false},
		{"invalidSameText", "Id", storage.Field{APIName: "Id", Type: storage.FieldID}, storage.IDValue("invalid"), storage.StringValue("invalid"), true},
	} {
		t.Run(selected.name, func(t *testing.T) {
			org := storage.NewOrgState()
			engine := NewEngine(&org)
			definition := storage.ObjectDefinition{APIName: "OwnedRecord__c", Fields: map[string]storage.Field{selected.fieldName: selected.field}}
			target := storage.Record{Object: definition.APIName, Fields: map[string]storage.Value{}}
			if selected.fieldName == "Id" {
				target.ID = selected.target.ID
			} else {
				target.Fields[selected.fieldName] = selected.target
			}
			frame := newFlowFrame()
			frame.scalars[flowFrameKey("InputID")] = selected.input
			for _, operation := range []string{"equals", "notequal"} {
				want := selected.want
				if operation == "notequal" {
					want = !want
				}
				matches, ok := engine.evaluateFlowLookupCriteria(storage.WorkflowCriteriaItem{
					Field: selected.fieldName, Operation: operation, SourceField: "InputID",
				}, target, definition, storage.Record{}, storage.ObjectDefinition{}, frame)
				if !ok || matches != want {
					t.Errorf("%s: matches=%v, resolved=%v; want %v", operation, matches, ok, want)
				}
			}
		})
	}
}
