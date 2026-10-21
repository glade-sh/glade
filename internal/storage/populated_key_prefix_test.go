package storage

import (
	"reflect"
	"strings"
	"testing"
)

// Prefix normalization repairs schema allocation. Already imported record IDs
// and their typed references remain durable across normalization and export.
func TestPopulatedKeyPrefixNormalizationPreservesRecordsAndReferences(t *testing.T) {
	for _, standard := range []string{"CaseComment", "MessagingSession"} {
		t.Run(standard, func(t *testing.T) {
			prefix := StandardKeyPrefix(standard)
			if len(prefix) != 3 {
				t.Fatal("missing standard prefix")
			}
			parentID := ID(prefix + "000000000123")
			childID := ID("a42000000000124")
			parent := Record{ID: parentID, Object: "IdentityParent__c", Fields: map[string]Value{"Name": StringValue("parent")}}
			child := Record{ID: childID, Object: "IdentityChild__c", Fields: map[string]Value{"Name": StringValue("child"), "Parent__c": IDValue(parentID)}}
			org := NewOrgState()
			org.Objects["IdentityParent__c"] = ObjectState{Definition: ObjectDefinition{APIName: "IdentityParent__c", KeyPrefix: prefix, Fields: map[string]Field{"Name": {APIName: "Name", Type: FieldString}}}, Records: map[ID]Record{parentID: parent}}
			org.Objects["IdentityChild__c"] = ObjectState{Definition: ObjectDefinition{APIName: "IdentityChild__c", KeyPrefix: "a42", Fields: map[string]Field{"Name": {APIName: "Name", Type: FieldString}, "Parent__c": {APIName: "Parent__c", Type: FieldReference, ReferenceTo: []string{"IdentityParent__c"}}}}, Records: map[ID]Record{childID: child}}
			org.IDSequences["IdentityParent__c"] = 123
			org.Objects[standard] = ObjectState{Definition: ObjectDefinition{APIName: standard, KeyPrefix: "a43"}}
			before := FixtureFromOrg(org)
			EnsureUniqueKeyPrefixes(&org)
			if org.Objects[standard].Definition.KeyPrefix != prefix || org.Objects["IdentityParent__c"].Definition.KeyPrefix == prefix {
				t.Fatal("standard identity was not reserved")
			}
			if got := FixtureFromOrg(org); !reflect.DeepEqual(got, before) {
				t.Fatal("normalization changed stored records, references or sequence state")
			}
			after := org.Objects["IdentityParent__c"].Definition.KeyPrefix
			EnsureUniqueKeyPrefixes(&org)
			if org.Objects["IdentityParent__c"].Definition.KeyPrefix != after {
				t.Fatal("normalization is not idempotent")
			}
			restored := NewOrgState()
			for name, object := range org.Objects {
				restored.Objects[name] = ObjectState{Definition: object.Definition}
			}
			if err := ApplyFixture(&restored, FixtureFromOrg(org)); err != nil {
				t.Fatal(err)
			}
			for name, record := range map[string]Record{"IdentityParent__c": parent, "IdentityChild__c": child} {
				_, got, ok := LookupRecordByID(restored.Objects[name].Records, record.ID)
				if !ok || !reflect.DeepEqual(got.Fields, record.Fields) {
					t.Fatalf("fixture roundtrip changed %s: %+v", name, got)
				}
			}
			generator := NewRuntimeIDGeneratorForOrg(&org)
			next, err := generator.Next("IdentityParent__c")
			if err != nil || !strings.HasPrefix(string(next), after) || next == parentID {
				t.Fatalf("new allocation=%s: %v", next, err)
			}
		})
	}
}
