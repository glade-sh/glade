package vm

import "testing"

func TestListSObjectTypeUsesAllocation(t *testing.T) {
	program, err := CompileAnonymous(`
List<SObject> generic = new List<SObject>{null,new Contact()};
System.assertEquals(2,generic.deepClone().size());
List<Contact> concrete = new List<Contact>{new Contact()};
List<SObject> widened = concrete;
System.assertEquals(Contact.SObjectType,widened.getSObjectType());
System.assertEquals(Contact.SObjectType,(new List<Contact>()).getSObjectType());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
