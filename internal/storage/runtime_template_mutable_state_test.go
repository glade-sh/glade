package storage

import "testing"

func TestRuntimeTemplateCloneRuntimeOrgIsolatesNestedMutableState(t *testing.T) {
	accountID := ID("001000000000001")
	contactID := ID("003000000000001")
	parentID := ID("001000000000002")
	before := Record{
		ID:     accountID,
		Object: "Account",
		Fields: map[string]Value{
			"Tags__c": {Kind: ValueList, List: []Value{StringValue("before")}},
		},
	}
	after := before.Clone()
	after.Fields["Tags__c"] = Value{Kind: ValueList, List: []Value{StringValue("after")}}
	record := Record{
		ID:     accountID,
		Object: "Account",
		Fields: map[string]Value{
			"Tags__c": {Kind: ValueList, List: []Value{StringValue("base")}},
		},
		ExplicitNulls: map[string]bool{"Description": true},
		Children: map[string][]Record{
			"Contacts": {{
				ID:     contactID,
				Object: "Contact",
				Fields: map[string]Value{
					"Aliases__c": {Kind: ValueList, List: []Value{StringValue("child")}},
				},
			}},
		},
		ParentRelationships: map[string]Record{
			"Parent": {
				ID:     parentID,
				Object: "Account",
				Fields: map[string]Value{
					"Tags__c": {Kind: ValueList, List: []Value{StringValue("parent")}},
				},
			},
		},
		System: SystemFields{Locked: true},
	}
	org := NewOrgState()
	org.Objects["Account"] = ObjectState{
		Definition: ObjectDefinition{
			APIName: "Account",
			Fields:  map[string]Field{"Name": {APIName: "Name", Type: FieldString}},
		},
		Records: map[ID]Record{accountID: record},
		Indexes: map[string]IndexSet{
			"ByName": {
				Definition: IndexDefinition{Name: "ByName", Object: "Account", Fields: []string{"Name"}},
				Entries:    map[string][]ID{"base": {accountID}},
				Dirty:      true,
			},
		},
	}
	org.IDSequences["Account"] = 7
	org.Transactions = []TransactionFrame{{
		ID: "transaction-1",
		Mutations: []Mutation{{
			Op:     MutationUpdate,
			Object: "Account",
			ID:     accountID,
			Before: &before,
			After:  &after,
		}},
	}}

	template := NewRuntimeTemplate(org)
	first := template.CloneRuntimeOrg()
	second := template.CloneRuntimeOrg()

	firstObject := first.Objects["Account"]
	firstRecord := firstObject.Records[accountID]
	firstTags := firstRecord.Fields["Tags__c"]
	firstTags.List[0] = StringValue("changed")
	firstRecord.Fields["Tags__c"] = firstTags
	firstRecord.ExplicitNulls["Description"] = false
	firstChild := firstRecord.Children["Contacts"][0]
	firstChildAliases := firstChild.Fields["Aliases__c"]
	firstChildAliases.List[0] = StringValue("changed-child")
	firstChild.Fields["Aliases__c"] = firstChildAliases
	firstRecord.Children["Contacts"][0] = firstChild
	firstParent := firstRecord.ParentRelationships["Parent"]
	firstParentTags := firstParent.Fields["Tags__c"]
	firstParentTags.List[0] = StringValue("changed-parent")
	firstParent.Fields["Tags__c"] = firstParentTags
	firstRecord.ParentRelationships["Parent"] = firstParent
	firstRecord.System.Locked = false
	firstObject.Records[accountID] = firstRecord
	firstIndex := firstObject.Indexes["ByName"]
	firstIndex.Definition.Fields[0] = "Changed__c"
	firstIndex.Entries["base"][0] = parentID
	firstIndex.Dirty = false
	firstObject.Indexes["ByName"] = firstIndex
	first.Objects["Account"] = firstObject
	first.IDSequences["Account"] = 99
	firstBeforeTags := first.Transactions[0].Mutations[0].Before.Fields["Tags__c"]
	firstBeforeTags.List[0] = StringValue("changed-before")
	first.Transactions[0].Mutations[0].Before.Fields["Tags__c"] = firstBeforeTags
	firstAfterTags := first.Transactions[0].Mutations[0].After.Fields["Tags__c"]
	firstAfterTags.List[0] = StringValue("changed-after")
	first.Transactions[0].Mutations[0].After.Fields["Tags__c"] = firstAfterTags

	assertRuntimeTemplateMutableStateUnchanged(t, "template", template.Org, accountID)
	assertRuntimeTemplateMutableStateUnchanged(t, "sibling", second, accountID)
}

func assertRuntimeTemplateMutableStateUnchanged(t *testing.T, name string, org OrgState, accountID ID) {
	t.Helper()
	object := org.Objects["Account"]
	record := object.Records[accountID]
	if got := record.Fields["Tags__c"].List[0].String; got != "base" {
		t.Fatalf("%s record nested list = %q, want base", name, got)
	}
	if !record.ExplicitNulls["Description"] {
		t.Fatalf("%s explicit nulls changed: %#v", name, record.ExplicitNulls)
	}
	if got := record.Children["Contacts"][0].Fields["Aliases__c"].List[0].String; got != "child" {
		t.Fatalf("%s child nested list = %q, want child", name, got)
	}
	if got := record.ParentRelationships["Parent"].Fields["Tags__c"].List[0].String; got != "parent" {
		t.Fatalf("%s parent relationship nested list = %q, want parent", name, got)
	}
	if !record.System.Locked {
		t.Fatalf("%s record lock changed: %#v", name, record.System)
	}
	index := object.Indexes["ByName"]
	if got := index.Definition.Fields[0]; got != "Name" {
		t.Fatalf("%s index definition fields = %#v", name, index.Definition.Fields)
	}
	if got := index.Entries["base"][0]; got != accountID {
		t.Fatalf("%s index entries = %#v", name, index.Entries)
	}
	if !index.Dirty {
		t.Fatalf("%s index dirty flag changed", name)
	}
	if got := org.IDSequences["Account"]; got != 7 {
		t.Fatalf("%s Account sequence = %d, want 7", name, got)
	}
	mutation := org.Transactions[0].Mutations[0]
	if got := mutation.Before.Fields["Tags__c"].List[0].String; got != "before" {
		t.Fatalf("%s transaction before nested list = %q, want before", name, got)
	}
	if got := mutation.After.Fields["Tags__c"].List[0].String; got != "after" {
		t.Fatalf("%s transaction after nested list = %q, want after", name, got)
	}
}
