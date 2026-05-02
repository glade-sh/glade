package storage

import "testing"

func TestApplyFixtureResolvesAliasesAndRelationshipRefs(t *testing.T) {
	org := NewOrgState()
	org.Objects["Account"] = ObjectState{
		Definition: ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]Field{"Name": {APIName: "Name", Type: FieldString}}},
		Records:    make(map[ID]Record),
	}
	org.Objects["Contact"] = ObjectState{
		Definition: ObjectDefinition{APIName: "Contact", KeyPrefix: "003", Fields: map[string]Field{"AccountId": {APIName: "AccountId", Type: FieldReference}}},
		Records:    make(map[ID]Record),
	}

	err := ApplyFixture(&org, Fixture{
		Version: FixtureVersion,
		Objects: []FixtureObject{
			{Name: "Account", Records: []FixtureRecord{{Alias: "acme", Fields: map[string]Value{"Name": StringValue("Acme")}}}},
			{Name: "Contact", Records: []FixtureRecord{{Alias: "smith", FieldRefs: map[string]string{"AccountId": "acme"}}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(org.Objects["Account"].Records) != 1 || len(org.Objects["Contact"].Records) != 1 {
		t.Fatalf("records = %#v", org.Objects)
	}
	var accountID ID
	for id := range org.Objects["Account"].Records {
		accountID = id
	}
	for _, contact := range org.Objects["Contact"].Records {
		if got := contact.Fields["AccountId"].ID; got != accountID {
			t.Fatalf("AccountId = %q, want %q", got, accountID)
		}
	}
}

func TestEnsureDeterministicPlatformData(t *testing.T) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	if len(org.Objects["User"].Records) != 1 || len(org.Objects["Profile"].Records) != 1 {
		t.Fatalf("platform records = %#v", InspectOrg("", org))
	}
	if _, ok := org.Objects["User"].Records["005000000000001"]; !ok {
		t.Fatalf("missing deterministic user: %#v", org.Objects["User"].Records)
	}
}
