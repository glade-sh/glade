package storage

import "testing"

func TestCase349PersistedFamilyIgnoresReusedForeignTypeID38a(t *testing.T) {
	org := NewOrgState()
	id := ID("012000000000111")
	org.Objects["Account"] = ObjectState{Definition: ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]Field{"Name": {APIName: "Name", Type: FieldString}}, RecordTypes: []RecordTypeInfo{{ID: id, DeveloperName: "Business", Active: true, Available: true}}}, Records: map[ID]Record{}}
	org.Objects["RecordType"] = ObjectState{Definition: ObjectDefinition{APIName: "RecordType", KeyPrefix: "012", Fields: map[string]Field{}}, Records: map[ID]Record{id: {ID: id, Object: "RecordType", Fields: map[string]Value{"SobjectType": StringValue("Contact"), "DeveloperName": StringValue("Foreign"), "IsPersonType": BooleanValue(true)}}}}
	EnsureDeterministicPlatformData(&org)
	for _, rt := range org.Objects["Account"].Definition.RecordTypes {
		if rt.DeveloperName == "Business" && (rt.PersonAccount || IDsEqual(rt.ID, id)) {
			t.Errorf("foreign family or ID reused: %#v", rt)
		}
	}
}
