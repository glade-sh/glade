package vm

import (
	"fmt"
	"testing"

	"github.com/open-aer/oaer/internal/storage"
)

func TestStressDescribeHeavyExecution(t *testing.T) {
	var source string
	source += "Map<String,Object> describes = Schema.getGlobalDescribe();\n"
	source += "Object accountType = describes.get('Account');\n"
	source += "Object accountDescribe = accountType.getDescribe();\n"
	for i := 0; i < 80; i++ {
		source += fmt.Sprintf("Map<String,Object> fields%d = accountDescribe.fields.getMap();\n", i)
		source += fmt.Sprintf("System.assert(fields%d.containsKey('Name'));\n", i)
		source += fmt.Sprintf("List<Object> recordTypes%d = accountDescribe.getRecordTypeInfos();\n", i)
		source += fmt.Sprintf("System.assertEquals(3, recordTypes%d.size());\n", i)
		source += fmt.Sprintf("List<Object> children%d = accountDescribe.getChildRelationships();\n", i)
		source += fmt.Sprintf("System.assertEquals(1, children%d.size());\n", i)
	}
	program, err := CompileAnonymous(source)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.RecordTypes = []storage.RecordTypeInfo{
		{ID: "012000000000001", DeveloperName: "Business", Name: "Business Account", Active: true, Available: true, Default: true},
		{ID: "012000000000002", DeveloperName: "Consumer", Name: "Consumer Account", Active: true, Available: true},
		{ID: "012000000000003", DeveloperName: "Partner", Name: "Partner Account", Active: true, Available: true},
	}
	org.Objects["Account"] = account
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
			},
			Relations: []storage.Relationship{{
				Field:              "AccountId",
				ParentObjects:      []string{"Account"},
				ParentRelationship: "Account",
				ChildRelationship:  "Contacts",
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
