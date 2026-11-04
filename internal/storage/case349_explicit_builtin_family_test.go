package storage

import "testing"

func TestCase349ExplicitBuiltinFamilyNotOverwritten(t *testing.T) {
	for _, person := range []bool{false, true} {
		org := NewOrgState()
		org.Objects["Account"] = ObjectState{Definition: ObjectDefinition{APIName: "Account", Fields: map[string]Field{}, RecordTypes: []RecordTypeInfo{{ID: "012000000000c03AAA", DeveloperName: "PersonAccount", PersonAccount: person}}}}
		ApplyOrgShape(&org, []string{"PersonAccounts"})
		for _, rt := range org.Objects["Account"].Definition.RecordTypes {
			if rt.ID == "012000000000c03AAA" && rt.PersonAccount != person {
				t.Errorf("explicit family changed from %v to %v", person, rt.PersonAccount)
			}
		}
	}
}
