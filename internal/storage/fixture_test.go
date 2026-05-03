package storage

import (
	"strings"
	"testing"
)

func TestApplyFixtureResolvesAliasesAndRelationshipRefs(t *testing.T) {
	org := fixtureRelationshipOrg()

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

func TestApplyFixtureResolvesQualifiedAndPolymorphicAliases(t *testing.T) {
	org := fixtureRelationshipOrg()
	err := ApplyFixture(&org, Fixture{
		Version: FixtureVersion,
		Objects: []FixtureObject{
			{Name: "Account", Records: []FixtureRecord{{Alias: "shared", Fields: map[string]Value{"Name": StringValue("Acme")}}}},
			{Name: "Contact", Records: []FixtureRecord{{Alias: "shared", Fields: map[string]Value{"LastName": StringValue("Smith")}, FieldRefs: map[string]string{"AccountId": "Account.shared"}}}},
			{Name: "Task", Records: []FixtureRecord{{Alias: "task", FieldRefs: map[string]string{"WhatId": "Account.shared", "WhoId": "Contact.shared"}}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var accountID, contactID ID
	for id := range org.Objects["Account"].Records {
		accountID = id
	}
	for id := range org.Objects["Contact"].Records {
		contactID = id
	}
	for _, task := range org.Objects["Task"].Records {
		if got := task.Fields["WhatId"].ID; got != accountID {
			t.Fatalf("WhatId = %q, want %q", got, accountID)
		}
		if got := task.Fields["WhoId"].ID; got != contactID {
			t.Fatalf("WhoId = %q, want %q", got, contactID)
		}
	}
}

func TestApplyFixtureRejectsAmbiguousAndInvalidRelationshipAliases(t *testing.T) {
	ambiguous := fixtureRelationshipOrg()
	err := ApplyFixture(&ambiguous, Fixture{
		Version: FixtureVersion,
		Objects: []FixtureObject{
			{Name: "Account", Records: []FixtureRecord{{Alias: "shared", Fields: map[string]Value{"Name": StringValue("Acme")}}}},
			{Name: "Contact", Records: []FixtureRecord{{Alias: "shared", Fields: map[string]Value{"LastName": StringValue("Smith")}}}},
			{Name: "Task", Records: []FixtureRecord{{FieldRefs: map[string]string{"WhatId": "shared"}}}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "ambiguous fixture alias") {
		t.Fatalf("ambiguous error = %v", err)
	}

	duplicateSameObject := fixtureRelationshipOrg()
	err = ApplyFixture(&duplicateSameObject, Fixture{
		Version: FixtureVersion,
		Objects: []FixtureObject{
			{Name: "Account", Records: []FixtureRecord{
				{Alias: "shared", Fields: map[string]Value{"Name": StringValue("Acme")}},
				{Alias: "shared", Fields: map[string]Value{"Name": StringValue("GloboCorp")}},
			}},
			{Name: "Contact", Records: []FixtureRecord{{FieldRefs: map[string]string{"AccountId": "shared"}}}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "ambiguous fixture alias") {
		t.Fatalf("same-object duplicate error = %v", err)
	}

	wrongTarget := fixtureRelationshipOrg()
	err = ApplyFixture(&wrongTarget, Fixture{
		Version: FixtureVersion,
		Objects: []FixtureObject{
			{Name: "Contact", Records: []FixtureRecord{{Alias: "smith", Fields: map[string]Value{"LastName": StringValue("Smith")}}}},
			{Name: "Contact", Records: []FixtureRecord{{Alias: "jones", Fields: map[string]Value{"LastName": StringValue("Jones")}, FieldRefs: map[string]string{"AccountId": "Contact.smith"}}}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "cannot populate Contact.AccountId") {
		t.Fatalf("wrong target error = %v", err)
	}
}

func TestApplyFixtureInitializesNilExistingRecordMaps(t *testing.T) {
	org := NewOrgState()
	org.Objects["Account"] = ObjectState{
		Definition: ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]Field{"Name": {APIName: "Name", Type: FieldString}}},
	}
	err := ApplyFixture(&org, Fixture{
		Version: FixtureVersion,
		Objects: []FixtureObject{
			{Name: "Account", Records: []FixtureRecord{{Alias: "acme", Fields: map[string]Value{"Name": StringValue("Acme")}}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(org.Objects["Account"].Records) != 1 {
		t.Fatalf("records = %#v", org.Objects["Account"].Records)
	}
}

func TestApplyFixtureKeepsIDsForDuplicateObjectBlocks(t *testing.T) {
	org := fixtureRelationshipOrg()
	err := ApplyFixture(&org, Fixture{
		Version: FixtureVersion,
		Objects: []FixtureObject{
			{Name: "Account", Records: []FixtureRecord{
				{Alias: "acme", Fields: map[string]Value{"Name": StringValue("Acme")}},
				{Alias: "global", Fields: map[string]Value{"Name": StringValue("Global Media")}},
			}},
			{Name: "Account", Records: []FixtureRecord{
				{Alias: "edge", Fields: map[string]Value{"Name": StringValue("Edge")}},
			}},
			{Name: "Contact", Records: []FixtureRecord{
				{Fields: map[string]Value{"LastName": StringValue("Smith")}, FieldRefs: map[string]string{"AccountId": "Account.global"}},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(org.Objects["Account"].Records) != 3 {
		t.Fatalf("Account records = %d", len(org.Objects["Account"].Records))
	}
	var globalID ID
	for id, account := range org.Objects["Account"].Records {
		if account.Fields["Name"].String == "Global Media" {
			globalID = id
		}
	}
	if globalID == "" {
		t.Fatalf("missing Global Media record: %#v", org.Objects["Account"].Records)
	}
	for _, contact := range org.Objects["Contact"].Records {
		if got := contact.Fields["AccountId"].ID; got != globalID {
			t.Fatalf("AccountId = %q, want %q", got, globalID)
		}
	}
}

func fixtureRelationshipOrg() OrgState {
	org := NewOrgState()
	org.Objects["Account"] = ObjectState{
		Definition: ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]Field{"Name": {APIName: "Name", Type: FieldString}}},
		Records:    make(map[ID]Record),
	}
	org.Objects["Contact"] = ObjectState{
		Definition: ObjectDefinition{APIName: "Contact", KeyPrefix: "003", Fields: map[string]Field{
			"LastName":  {APIName: "LastName", Type: FieldString},
			"AccountId": {APIName: "AccountId", Type: FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
		}},
		Records: make(map[ID]Record),
	}
	org.Objects["Task"] = ObjectState{
		Definition: ObjectDefinition{APIName: "Task", KeyPrefix: "00T", Fields: map[string]Field{
			"Subject": {APIName: "Subject", Type: FieldString},
			"WhoId":   {APIName: "WhoId", Type: FieldReference, ReferenceTo: []string{"Contact"}},
			"WhatId":  {APIName: "WhatId", Type: FieldReference, ReferenceTo: []string{"Account"}},
		}},
		Records: make(map[ID]Record),
	}
	return org
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
