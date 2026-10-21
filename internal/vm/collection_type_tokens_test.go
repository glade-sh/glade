package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestListSObjectTypeNameCanonicalizesSchemaElements(t *testing.T) {
	for _, objectName := range []string{"Account", "Contact", "Widget__c", "sample__Widget__c"} {
		for _, prefix := range []string{"", "Schema.", "schema."} {
			for _, source := range []string{"static", "runtime", "type", "item"} {
				t.Run(objectName+"/"+prefix+"/"+source, func(t *testing.T) {
					records := List()
					records.Type = "List<SObject>"
					qualified := prefix + objectName
					switch source {
					case "static":
						records.Static = "List<" + qualified + ">"
					case "runtime":
						records.Runtime = "List<" + qualified + ">"
					case "type":
						records.Type = "List<" + qualified + ">"
					case "item":
						records.List = []Value{Object(qualified)}
					}
					if got := listSObjectTypeName(records); got != objectName {
						t.Fatalf("object type = %q, want %q", got, objectName)
					}
				})
			}
		}
	}
	for _, elementType := range []string{"SObject", "Schema.SObject", "String", "Schema.String", "ResidualLens"} {
		records := List()
		records.Type = "List<" + elementType + ">"
		if got := listSObjectTypeName(records); got != "" {
			t.Errorf("%s object type = %q, want no concrete token", records.Type, got)
		}
	}
}

// Standard-object reflection behavior is also covered by owned native cells.
// Custom-object cases below retain local coverage without a native parity claim.
func TestExecReflectedListSObjectTypeUsesSchemaElementIdentity(t *testing.T) {
	for _, objectName := range []string{"Account", "Widget__c"} {
		for _, testContext := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/test=%t", objectName, testContext), func(t *testing.T) {
				program, err := CompileAnonymous(fmt.Sprintf(`
List<SObject> literal = new List<%[1]s>();
Schema.SObjectType expected = literal.getSObjectType();
List<SObject> rows = (List<SObject>)Type.forName('List<%[1]s>').newInstance();
System.assertEquals(expected, rows.getSObjectType());
System.assertEquals('%[1]s', rows.getSObjectType().getDescribe().getName());
rows.add(new %[1]s());
System.assertEquals(expected, rows.getSObjectType());
rows.clear();
System.assertEquals(expected, rows.getSObjectType());
System.assertEquals(null, new List<SObject>().getSObjectType());
`, objectName))
				if err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				storage.EnsureStandardObject(&org, "Account")
				org.Objects["Widget__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Widget__c"}}
				machine := New(nil)
				machine.SetOrg(&org)
				if testContext {
					machine.EnableTestContext()
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
