package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestSObjectInternalFieldsAreUnknown(t *testing.T) {
	for _, field := range []struct{ raw, resolved, namespace string }{
		{"__glade_explicit_fields", "__glade_explicit_fields", ""},
		{"Account.__glade_explicit_fields", "__glade_explicit_fields", ""},
		{"AcCoUnT.__GLADE_EXPLICIT_FIELDS", "__GLADE_EXPLICIT_FIELDS", ""},
		{"ns____glade_explicit_fields", "__glade_explicit_fields", "ns"},
		{"Account.ns____glade_explicit_fields", "__glade_explicit_fields", "ns"},
	} {
		for _, member := range []struct{ method, argument, message, missingMessage string }{
			{"get", "", "Invalid field " + field.resolved + " for Account", "Invalid field NoSuchField__c for Account"},
			// Internal names use the established invalid-field path.
			{"put", ", new Map<String,Object>()", "Invalid field " + field.resolved + " for Account", ""},
			{"isSet", "", "Invalid field: " + field.raw, "Invalid field: NoSuchField__c"},
			{"getSObject", "", "Invalid relationship " + field.raw + " for Account", "Invalid relationship NoSuchField__c for Account"},
			{"putSObject", ", new Account()", "Invalid relationship " + field.raw + " for Account", "Invalid relationship NoSuchField__c for Account"},
			{"getSObjects", "", "Invalid aggregate relationship " + field.raw + " for Account", "Invalid aggregate relationship NoSuchField__c for Account"},
		} {
			t.Run(field.raw+"/"+member.method, func(t *testing.T) {
				machine, _ := runDynamicObjectAliasProgram(t, "")
				if field.namespace != "" {
					machine.Org.Namespace = field.namespace
					machine.SetOrg(machine.Org)
				}
				source := fmt.Sprintf(`
Account record = new Account(Name = 'safe');
String rejection;
String exceptionType;
try {
    record.%s('%s'%s);
} catch (SObjectException e) {
    rejection = e.getMessage();
    exceptionType = e.getTypeName();
}
System.assertEquals('System.SObjectException', exceptionType);
System.assertEquals('%s', rejection, 'internal name must follow the unknown-field or relationship path');
System.assertEquals('safe', record.get('Name'));
System.assertEquals(true, record.isSet('Name'));
System.assertEquals('safe', record.put('Name', 'changed'));
System.assertEquals('changed', record.get('Name'));
`, member.method, field.raw, member.argument, member.message)
				if member.missingMessage != "" {
					source += fmt.Sprintf(`
String missingRejection;
String missingType;
try {
    record.%s('NoSuchField__c'%s);
} catch (SObjectException e) {
    missingRejection = e.getMessage();
    missingType = e.getTypeName();
}
System.assertEquals('System.SObjectException', missingType);
System.assertEquals('%s', missingRejection, 'existing unknown-field message must be preserved');
`, member.method, member.argument, member.missingMessage)
				}
				program, err := CompileAnonymous(source)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestSObjectInternalFieldsOrdinaryRelationships(t *testing.T) {
	machine, _ := runDynamicObjectAliasProgram(t, "")
	account := machine.Org.Objects["Account"]
	account.Definition.Relations = append(account.Definition.Relations, storage.Relationship{
		Field: "ParentId", ParentObjects: []string{"Account"}, ParentRelationship: "Parent", ChildRelationship: "Children",
	})
	machine.Org.Objects["Account"] = account
	machine.SetOrg(machine.Org)
	program, err := CompileAnonymous(`
Account record = new Account(Name = 'child');
Account parent = new Account(Name = 'parent');
System.assertEquals(null, record.putSObject('Parent', parent));
System.assertEquals('parent', record.getSObject('Parent').get('Name'));
String childPutType;
String childPutMessage;
try {
    parent.put('Children', new List<Account>{new Account(Name = 'child')});
} catch (SObjectException e) {
    childPutType = e.getTypeName();
    childPutMessage = e.getMessage();
}
System.assertEquals('System.SObjectException', childPutType);
System.assertEquals('Invalid field Children for Account', childPutMessage);
List<Account> children = new List<Account>{new Account(Name = 'child')};
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	// put rejects relationship names (S006/S007); model a query-result graph
	// internally so relationship reads remain covered alongside putSObject (S012).
	aliasSObjectPruneInjectField(machine, "parent", "Children", machine.Globals["children"])
	aliasSObjectPruneExecute(t, machine, `
System.assertEquals(1, parent.getSObjects('Children').size());
System.assertEquals('child', parent.getSObjects('Children')[0].get('Name'));
`)
}
