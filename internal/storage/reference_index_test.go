package storage

import "testing"

func TestReferenceIndexUsesSchemaIDIdentity(t *testing.T) {
	const short = "001000000000001"
	const long = short + "AAA"
	org := NewOrgState()
	org.Objects["Child__c"] = ObjectState{
		Definition: ObjectDefinition{APIName: "Child__c",
			Fields: map[string]Field{
				"Owner__c":   {APIName: "Owner__c", Type: FieldReference, ReferenceTo: []string{"Account"}},
				"Literal__c": {APIName: "Literal__c", Type: FieldString},
			},
			Indexes: []IndexDefinition{
				{Name: "Child.Owner", Object: "Child__c", Fields: []string{"Owner__c"}},
				{Name: "Child.Literal", Object: "Child__c", Fields: []string{"Literal__c"}},
			},
		},
		Records: map[ID]Record{
			"a00000000000001": {ID: "a00000000000001", Object: "Child__c", Fields: map[string]Value{"Owner__c": IDValue(long), "Literal__c": StringValue(long)}},
		},
	}
	RebuildIndexes(&org)
	object := org.Objects["Child__c"]
	for _, value := range []Value{IDValue(short), IDValue(long), StringValue(short), StringValue(long)} {
		ids, ok := LookupIndex(object, "owner__c", value)
		if !ok || len(ids) != 1 || ids[0] != "a00000000000001" {
			t.Fatalf("reference lookup %v: %v %v", value, ids, ok)
		}
	}
	ids, ok := LookupIndex(object, "Literal__c", StringValue(short))
	if !ok || len(ids) != 0 {
		t.Fatalf("text was treated as ID: %v %v", ids, ok)
	}
	ids, ok = LookupIndex(object, "Literal__c", StringValue(long))
	if !ok || len(ids) != 1 {
		t.Fatalf("exact text lookup: %v %v", ids, ok)
	}
}

func TestReferenceIndexKeepsMetadataNamesLiteral(t *testing.T) {
	for _, target := range []string{"EntityDefinition", "FieldDefinition"} {
		t.Run(target, func(t *testing.T) {
			const short = "Abcdefghijklmno"
			const long = short + "Pqr"
			org := NewOrgState()
			org.Objects["Link__mdt"] = ObjectState{
				Definition: ObjectDefinition{APIName: "Link__mdt", Fields: map[string]Field{"Target__c": {APIName: "Target__c", Type: FieldReference, ReferenceTo: []string{target}}}, Indexes: []IndexDefinition{{Name: "target", Fields: []string{"Target__c"}}}},
				Records:    map[ID]Record{"m00000000000001": {ID: "m00000000000001", Object: "Link__mdt", Fields: map[string]Value{"Target__c": StringValue(long)}}},
			}
			RebuildIndexes(&org)
			rows, ok := LookupIndex(org.Objects["Link__mdt"], "Target__c", StringValue(short))
			if !ok || len(rows) != 0 {
				t.Fatalf("metadata name shortened: %v indexed=%t", rows, ok)
			}
			rows, ok = LookupIndex(org.Objects["Link__mdt"], "Target__c", StringValue(long))
			if !ok || len(rows) != 1 {
				t.Fatalf("metadata name lost: %v indexed=%t", rows, ok)
			}
		})
	}
}
