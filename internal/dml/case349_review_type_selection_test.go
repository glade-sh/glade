package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestCase349RecordTypeFamilySelection(t *testing.T) {
	types := []storage.RecordTypeInfo{
		{ID: "012000000000001", DeveloperName: "Person_Commerce", Name: "Person Commerce", Active: true, Available: true, Default: true},
		{ID: "012000000000002", DeveloperName: "Patient", Name: "Patient", Active: true, Available: true, PersonAccount: true},
	}
	if recordTypeLooksPersonAccount(types[0]) {
		t.Error("business record type classified as person")
	}
	if !recordTypeLooksPersonAccount(types[1]) {
		t.Error("person record type not recognized")
	}
	business, ok := businessAccountRecordType(types)
	if !ok || business.ID != types[0].ID {
		t.Errorf("business selection: %#v, %v", business, ok)
	}
	person, ok := personAccountRecordType(types)
	if !ok || person.ID != types[1].ID {
		t.Errorf("person selection: %#v, %v", person, ok)
	}
}
