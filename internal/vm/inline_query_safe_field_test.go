package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Apex Developer Guide, Safe Navigation Operator (p46), documents a null
// result for an empty immediate SOQL query followed by ?.field. This does not
// turn an ordinary query's empty List into a null singleton record.
func TestExecInlineQuerySafeFieldHandlesEmptyResults(t *testing.T) {
	cases := []struct {
		name, source, failure string
	}{
		{"emptyField", "System.assertEquals(null, [SELECT Name FROM Account WHERE Id = null]?.Name);", ""},
		{"emptyFieldChain", "System.assertEquals(null, [SELECT Name FROM Account WHERE Id = null]?.Name?.toUpperCase());", ""},
		{"emptyNumber", "Integer employees = [SELECT NumberOfEmployees FROM Account WHERE Id = null]?.NumberOfEmployees; System.assertEquals(null, employees);", ""},
		{"emptyBlobChain", "System.assertEquals(null, [SELECT VersionData FROM ContentVersion WHERE Id = null]?.VersionData?.toString());", ""},
		{"storedEmptyList", "List<Account> records = [SELECT Name FROM Account WHERE Id = null]; System.assertEquals(0, records.size()); System.assertEquals(0, records?.size());", ""},
		{"singleSafeField", "System.assertEquals('Acme', [SELECT Name FROM Account WHERE Name = 'Acme']?.Name);", ""},
		{"singleAssignment", "Account record = [SELECT Name FROM Account WHERE Name = 'Acme']; System.assertEquals('Acme', record.Name);", ""},
		{"emptyAssignment", "Account record = [SELECT Name FROM Account WHERE Id = null];", "List has no rows"},
		{"multipleAssignment", "Account record = [SELECT Name FROM Account];", "List has more than 1 row"},
		// Preserve rejection on the existing multirow field path without
		// claiming native evidence for its exact diagnostic.
		{"multipleSafeField", "String name = [SELECT Name FROM Account]?.Name;", "cannot access"},
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			for _, item := range cases {
				t.Run(api+"/"+route+"/"+item.name, func(t *testing.T) {
					program, err := CompileAnonymousWithOptions(item.source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := singleSOQLAssignmentOrg()
					account := org.Objects["Account"]
					account.Definition.Fields["NumberOfEmployees"] = storage.Field{APIName: "NumberOfEmployees", Type: storage.FieldInteger}
					org.Objects["Account"] = account
					org.Objects["ContentVersion"] = storage.ObjectState{
						Definition: storage.ObjectDefinition{APIName: "ContentVersion", Fields: map[string]storage.Field{
							"VersionData": {APIName: "VersionData", Type: storage.FieldBlob},
						}},
						Records: make(map[storage.ID]storage.Record),
					}
					machine := New(nil)
					machine.SetOrg(&org)
					if route == "isTest" {
						machine.EnableTestContext()
					}
					_, err = machine.Execute(program)
					if item.failure == "" {
						if err != nil {
							t.Fatal(err)
						}
					} else if err == nil || !strings.Contains(err.Error(), item.failure) {
						t.Fatalf("error = %v; expected existing %q rejection", err, item.failure)
					}
				})
			}
		}
	}
}
