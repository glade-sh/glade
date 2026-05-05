package storage

import "testing"

func TestResolveFieldNameResolvesIdCaseInsensitive(t *testing.T) {
	definition := ObjectDefinition{APIName: "Account", Fields: map[string]Field{"Name": {APIName: "Name", Type: FieldString}}}

	resolved, ok := ResolveFieldName(definition, "", "id")
	if !ok || resolved != "Id" {
		t.Fatalf("ResolveFieldName(id) = %q, %v", resolved, ok)
	}
}

func TestEnsureStandardObjectFieldsAddsAccountWebsiteWithoutClobber(t *testing.T) {
	definition := ObjectDefinition{
		APIName: "Account",
		Fields: map[string]Field{
			"Website": {APIName: "Website", Label: "Custom label", Type: FieldAny},
		},
	}

	EnsureStandardObjectFields(&definition)

	if definition.Fields["Website"].Type != FieldAny || definition.Fields["Website"].Label != "Custom label" {
		t.Fatalf("Website field was clobbered: %#v", definition.Fields["Website"])
	}
	if field, ok := definition.Fields["Phone"]; !ok || field.Type != FieldString {
		t.Fatalf("Phone field = %#v, %v", field, ok)
	}
	if field, ok := definition.Fields["PersonMailingStreet"]; !ok || field.Type != FieldString {
		t.Fatalf("PersonMailingStreet field = %#v, %v", field, ok)
	}
	if field, ok := definition.Fields["PersonOtherCountry"]; !ok || field.Type != FieldString {
		t.Fatalf("PersonOtherCountry field = %#v, %v", field, ok)
	}
}

func TestCloneRecordDoesNotShareMutableFieldState(t *testing.T) {
	original := Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]Value{
			"Name":    StringValue("Acme"),
			"Tags__c": ListValue(StringValue("a"), StringValue("b")),
		},
		ExplicitNulls: map[string]bool{"Description": true},
	}

	clone := original.Clone()
	clone.Fields["Name"] = StringValue("Changed")
	clone.Fields["Tags__c"].List[0] = StringValue("changed")
	clone.ExplicitNulls["Description"] = false

	if original.Fields["Name"].String != "Acme" {
		t.Fatalf("original name changed: %#v", original.Fields["Name"])
	}
	if original.Fields["Tags__c"].List[0].String != "a" {
		t.Fatalf("original list changed: %#v", original.Fields["Tags__c"])
	}
	if !original.ExplicitNulls["Description"] {
		t.Fatalf("original explicit null changed: %#v", original.ExplicitNulls)
	}
}

func TestCloneOrgStateDoesNotShareRecordsOrIndexes(t *testing.T) {
	org := NewOrgState()
	org.IDSequences["Account"] = 1
	org.Objects["Account"] = ObjectState{
		Definition: ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]Field{
				"Name": {APIName: "Name", Type: FieldString},
			},
			Indexes: []IndexDefinition{{Name: "Account.Name", Object: "Account", Fields: []string{"Name"}}},
		},
		Records: map[ID]Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]Value{"Name": StringValue("Acme")},
			},
		},
		Indexes: map[string]IndexSet{
			"Account.Name": {
				Definition: IndexDefinition{Name: "Account.Name", Object: "Account", Fields: []string{"Name"}},
				Entries:    map[string][]ID{"Acme": {"001000000000001"}},
			},
		},
	}

	clone := org.Clone()
	account := clone.Objects["Account"]
	account.Records["001000000000001"] = Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]Value{"Name": StringValue("Changed")},
	}
	account.Definition.Fields["Name"] = Field{APIName: "Name", Type: FieldBoolean}
	account.Definition.Indexes[0].Fields[0] = "OtherName"
	account.Indexes["Account.Name"].Entries["Acme"][0] = "001000000000002"
	clone.Objects["Account"] = account
	clone.IDSequences["Account"] = 2

	originalAccount := org.Objects["Account"]
	if org.IDSequences["Account"] != 1 {
		t.Fatalf("original sequence changed: %#v", org.IDSequences)
	}
	if originalAccount.Records["001000000000001"].Fields["Name"].String != "Acme" {
		t.Fatalf("original record changed: %#v", originalAccount.Records)
	}
	if originalAccount.Definition.Fields["Name"].Type != FieldString {
		t.Fatalf("original definition field changed: %#v", originalAccount.Definition.Fields)
	}
	if originalAccount.Definition.Indexes[0].Fields[0] != "Name" {
		t.Fatalf("original definition index changed: %#v", originalAccount.Definition.Indexes)
	}
	if originalAccount.Indexes["Account.Name"].Entries["Acme"][0] != "001000000000001" {
		t.Fatalf("original index changed: %#v", originalAccount.Indexes)
	}
}

func TestCloneTransactionFrameDoesNotShareMutationSnapshots(t *testing.T) {
	before := Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]Value{"Name": StringValue("Before")},
	}
	after := Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]Value{"Name": StringValue("After")},
	}
	frame := TransactionFrame{
		ID:    "tx-1",
		Depth: 1,
		Mutations: []Mutation{{
			Op:     MutationUpdate,
			Object: "Account",
			ID:     "001000000000001",
			Before: &before,
			After:  &after,
		}},
	}

	clone := frame.Clone()
	clone.Mutations[0].Before.Fields["Name"] = StringValue("Changed")
	clone.Mutations[0].After.Fields["Name"] = StringValue("Changed")

	if frame.Mutations[0].Before.Fields["Name"].String != "Before" {
		t.Fatalf("original before snapshot changed: %#v", frame.Mutations[0].Before)
	}
	if frame.Mutations[0].After.Fields["Name"].String != "After" {
		t.Fatalf("original after snapshot changed: %#v", frame.Mutations[0].After)
	}
}
