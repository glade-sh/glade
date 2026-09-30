package soql

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestRelationshipContractFamilyParsesAndExecutesCustomShapes(t *testing.T) {
	queryText := "SELECT Id, Name__c, (SELECT Id, Label__c FROM Items__r) FROM Contract__c ORDER BY Name__c"
	query, err := Parse(queryText)
	if err != nil {
		t.Fatalf("Parse relationship projection: %v", err)
	}
	if !reflect.DeepEqual(query.Fields, []string{"Id", "Name__c"}) {
		t.Fatalf("selected fields = %#v", query.Fields)
	}
	if len(query.ChildQueries) != 1 || query.ChildQueries[0].Relationship != "Items__r" || !reflect.DeepEqual(query.ChildQueries[0].Query.Fields, []string{"Id", "Label__c"}) {
		t.Fatalf("child relationship query = %#v", query.ChildQueries)
	}
	childPath, err := Parse("SELECT Contract__r.Name__c FROM Item__c")
	if err != nil || !reflect.DeepEqual(childPath.Fields, []string{"Contract__r.Name__c"}) {
		t.Fatalf("child-to-parent path = %#v, err=%v", childPath, err)
	}
	for _, malformed := range []string{
		"SELECT Id, (SELECT Id, Label__c FROM ) FROM Contract__c",
		"SELECT Id, (SELECT Id, Label__c FROM Items__r FROM Contract__c",
	} {
		if _, err := Parse(malformed); err == nil {
			t.Errorf("Parse accepted malformed relationship query %q", malformed)
		}
	}

	org := relationshipContractOrg()
	result, err := ParseAndExecute(org, "SELECT Id, Name__c, (SELECT Id, Label__c FROM Items__r) FROM Contract__c ORDER BY Name__c")
	if err != nil {
		t.Fatalf("Execute parent-to-child query: %v", err)
	}
	if result.Rows != 2 || len(result.Records) != 2 {
		t.Fatalf("parent rows = %d, records = %#v", result.Rows, result.Records)
	}
	if children := result.Records[0].Children["Items__r"]; len(children) != 1 || children[0].Fields["Label__c"].String != "Linked item" {
		t.Fatalf("linked children = %#v", result.Records[0].Children)
	}
	if children := result.Records[1].Children["Items__r"]; len(children) != 0 {
		t.Fatalf("empty child result = %#v, want zero rows", children)
	}

	children, err := ParseAndExecute(org, "SELECT Id, Label__c, Contract__r.Name__c FROM Item__c ORDER BY Label__c")
	if err != nil {
		t.Fatalf("Execute child-to-parent query: %v", err)
	}
	if got := children.Records[0].Fields["Contract__r.Name__c"]; got.Kind != storage.ValueString || got.String != "Alpha contract" {
		t.Fatalf("linked parent projection = %#v", got)
	}
	if len(children.Records) != 2 || children.Records[1].ID != "a01000000000002" {
		t.Fatalf("orphan child row was not preserved: %#v", children.Records)
	}
	nullParents, err := ParseAndExecute(org, "SELECT Id FROM Item__c WHERE Contract__r.Name__c = NULL")
	if err != nil || nullParents.Rows != 1 {
		t.Fatalf("parent-null filter rows=%d err=%v", nullParents.Rows, err)
	}
	if _, err := ParseAndExecute(org, "SELECT Missing__r.Name__c FROM Item__c"); err == nil {
		t.Fatal("Execute accepted an undeclared parent relationship")
	}
}

func relationshipContractOrg() storage.OrgState {
	org := storage.NewOrgState()
	contractDefinition := storage.ObjectDefinition{
		APIName: "Contract__c",
		Fields: map[string]storage.Field{
			"Name__c": {APIName: "Name__c", Type: storage.FieldString},
		},
		Relations: []storage.Relationship{{
			Field:              "Contract__c",
			ParentObjects:      []string{"Contract__c"},
			ParentRelationship: "Contract__r",
			ChildRelationship:  "Items__r",
		}},
	}
	itemDefinition := storage.ObjectDefinition{
		APIName: "Item__c",
		Fields: map[string]storage.Field{
			"Label__c":    {APIName: "Label__c", Type: storage.FieldString},
			"Contract__c": {APIName: "Contract__c", Type: storage.FieldReference, ReferenceTo: []string{"Contract__c"}, RelationshipName: "Contract__r", ChildRelationshipName: "Items__r"},
		},
		Relations: []storage.Relationship{{
			Field:              "Contract__c",
			ParentObjects:      []string{"Contract__c"},
			ParentRelationship: "Contract__r",
			ChildRelationship:  "Items__r",
		}},
	}
	storage.EnsureStandardObjectFields(&contractDefinition)
	storage.EnsureStandardObjectFields(&itemDefinition)
	org.Objects["Contract__c"] = storage.ObjectState{
		Definition: contractDefinition,
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {ID: "a00000000000001", Object: "Contract__c", Fields: map[string]storage.Value{"Name__c": storage.StringValue("Alpha contract")}},
			"a00000000000002": {ID: "a00000000000002", Object: "Contract__c", Fields: map[string]storage.Value{"Name__c": storage.StringValue("Empty contract")}},
		},
	}
	org.Objects["Item__c"] = storage.ObjectState{
		Definition: itemDefinition,
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {ID: "a01000000000001", Object: "Item__c", Fields: map[string]storage.Value{"Label__c": storage.StringValue("Linked item"), "Contract__c": storage.IDValue("a00000000000001")}},
			"a01000000000002": {ID: "a01000000000002", Object: "Item__c", Fields: map[string]storage.Value{"Label__c": storage.StringValue("Orphan item")}},
		},
	}
	return org
}
