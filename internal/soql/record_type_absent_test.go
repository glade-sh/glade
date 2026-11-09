package soql

import (
	"errors"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// record_type_visibility rows: RecordTypeId and the RecordType relationship
// exist only on an object with a record type besides Master.
func TestAbsentRecordTypeReferenceMessages(t *testing.T) {
	org := storage.NewOrgState()
	fields := map[string]storage.Field{
		"Id":           {APIName: "Id", Type: storage.FieldID},
		"Name":         {APIName: "Name", Type: storage.FieldString},
		"RecordTypeId": {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"},
	}
	plain := map[string]storage.Field{"Id": fields["Id"], "Name": fields["Name"]}
	org.Objects["Plain__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Plain__c", KeyPrefix: "a01", Fields: plain}, Records: map[storage.ID]storage.Record{}}
	// A definition that declares RecordTypeId keeps it.
	org.Objects["Declared__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Declared__c", KeyPrefix: "a03", Fields: fields}, Records: map[storage.ID]storage.Record{}}
	org.Objects["Typed__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Typed__c", KeyPrefix: "a02", Fields: fields,
		RecordTypes: []storage.RecordTypeInfo{{ID: "012000000000101", DeveloperName: "Alpha", Active: true}}}, Records: map[storage.ID]storage.Record{}}
	org.Objects["RecordType"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "RecordType", KeyPrefix: "012", Fields: map[string]storage.Field{
		"Id": {APIName: "Id", Type: storage.FieldID}, "Name": {APIName: "Name", Type: storage.FieldString},
	}}, Records: map[storage.ID]storage.Record{}}
	column := "No such column 'RecordTypeId' on entity 'Plain__c'. If you are attempting to use a custom field"
	relationship := "Didn't understand relationship 'RecordType' in field path. If you are attempting to use a custom relationship"
	for query, want := range map[string]string{
		"SELECT RecordTypeId FROM Plain__c LIMIT 1":                            column,
		"SELECT Id FROM Plain__c WHERE RecordTypeId = null LIMIT 1":            column,
		"SELECT Id FROM Plain__c ORDER BY RecordTypeId LIMIT 1":                column,
		"SELECT RecordType.Name FROM Plain__c LIMIT 1":                         relationship,
		"SELECT Id FROM Plain__c WHERE RecordType.Name = 'x' LIMIT 1":          relationship,
		"SELECT Id FROM Plain__c ORDER BY RecordType.Name LIMIT 1":             relationship,
		"SELECT RecordTypeId, COUNT(Id) n FROM Plain__c GROUP BY RecordTypeId": "Invalid field: 'RecordTypeId'",
		"SELECT RecordTypeId, RecordType.Name FROM Typed__c LIMIT 1":           "",
	} {
		_, err := ParseAndExecute(org, query)
		var queryErr *QueryError
		switch {
		case want == "" && err != nil:
			t.Errorf("%s: %v", query, err)
		case want != "" && (!errors.As(err, &queryErr) || !strings.HasPrefix(queryErr.Message, want)):
			t.Errorf("%s: got %v; want %q", query, err, want)
		}
	}
}
