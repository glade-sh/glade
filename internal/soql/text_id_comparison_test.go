package soql

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestTextComparisonPreservesTypedIDChecksum(t *testing.T) {
	for _, tt := range []struct {
		name string
		text string
		id   storage.ID
		want bool
	}{
		{"same ID", "001xx000003DGbYAAW", "001xx000003DGbYAAW", true},
		{"case-distinct ID", "001xx000003DGBYAA4", "001xx000003DGbYAAW", false},
		{"case-insensitive Text", "001xx000003dgbyaaw", "001xx000003DGbYAAW", true},
		{"Text has no checksum", "001xx000003DGbY", "001xx000003DGbYAAW", false},
		{"numeric Text has no checksum", "001000000000001", "001000000000001AAA", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := equalValues(storage.StringValue(tt.text), storage.IDValue(tt.id)); got != tt.want {
				t.Fatalf("Text %q versus Id %q = %t, want %t", tt.text, tt.id, got, tt.want)
			}
		})
	}

	// An Id field retains the existing case-insensitive quoted-string path.
	if !equalValues(storage.IDValue("001xx000003DGbY"), storage.StringValue("001xx000003dgby")) {
		t.Fatal("Id field no longer matches a case-insensitive quoted string")
	}
	// The field and bind remain ordinary Text even when both look like IDs.
	if equalValues(storage.StringValue("001xx000003DGbYAAW"), storage.StringValue("001xx000003DGbY")) {
		t.Fatal("Text comparison discarded the checksum suffix")
	}
}

func TestTextLikeUsesCompleteTypedID(t *testing.T) {
	id := storage.IDValue("001000000000001AAA")
	if !likeMatch(storage.StringValue("001000000000001AAA"), id) {
		t.Fatal("LIKE did not match the complete typed Id text")
	}
	if likeMatch(storage.StringValue("001000000000001"), id) {
		t.Fatal("LIKE matched only the 15-character prefix")
	}
}

func TestReferenceFieldsWithStringStorageRetainIDComparison(t *testing.T) {
	org := storage.NewOrgState()
	const objectName = "ReferenceFixture__c"
	org.Objects[objectName] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: objectName,
			Fields: map[string]storage.Field{
				"ReferenceId__c": {APIName: "ReferenceId__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
				"RecordId__c":    {APIName: "RecordId__c", Type: storage.FieldID},
				"TextId__c":      {APIName: "TextId__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {
				ID: "a00000000000001", Object: objectName,
				Fields: map[string]storage.Value{
					"ReferenceId__c": storage.StringValue("001000000000001"),
					"RecordId__c":    storage.StringValue("001000000000001"),
					"TextId__c":      storage.StringValue("001000000000001"),
				},
			},
		},
	}
	for _, tt := range []struct {
		field string
		id    storage.ID
		want  int
	}{
		{"ReferenceId__c", "001000000000001AAA", 1},
		{"RecordId__c", "001000000000001AAA", 1},
		{"TextId__c", "001000000000001AAA", 0},
	} {
		t.Run(tt.field, func(t *testing.T) {
			result, err := Execute(org, Query{
				Object: objectName,
				Fields: []string{"Id"},
				Where:  &Condition{Field: tt.field, Op: "=", Value: storage.IDValue(tt.id)},
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Rows != tt.want {
				t.Fatalf("string-backed %s rows = %d, want %d", tt.field, result.Rows, tt.want)
			}
		})
	}
}
