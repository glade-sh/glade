package storage

import "testing"

// Ruling 38b: definition family remains explicit; an existing row is not overwritten.
func TestCase349ExplicitFamilyDoesNotUseSeedRow38b(t *testing.T) {
	for _, family := range []bool{false, true} {
		t.Run(map[bool]string{false: "business", true: "person"}[family], func(t *testing.T) {
			org := NewOrgState()
			id := ID("012000000000101")
			org.Objects["Account"] = ObjectState{Definition: ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]Field{"Name": {APIName: "Name", Type: FieldString}}, RecordTypes: []RecordTypeInfo{{ID: id, DeveloperName: "Synthetic", Active: true, Available: true, PersonAccount: family}}}, Records: map[ID]Record{}}
			org.Objects["RecordType"] = ObjectState{Definition: ObjectDefinition{APIName: "RecordType", KeyPrefix: "012", Fields: map[string]Field{}}, Records: map[ID]Record{id: {ID: id, Object: "RecordType", Fields: map[string]Value{"SobjectType": StringValue("Account"), "DeveloperName": StringValue("Synthetic"), "IsPersonType": BooleanValue(!family)}}}}
			EnsureDeterministicPlatformData(&org)
			if got := org.Objects["Account"].Definition.RecordTypes[0].PersonAccount; got != family {
				t.Errorf("explicit family overwritten: got %v want %v", got, family)
			}
			if got := org.Objects["RecordType"].Records[id].Fields["IsPersonType"].Boolean; got == family {
				t.Errorf("existing explicit row overwritten: got %v", got)
			}
		})
	}
}
