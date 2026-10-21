package vm

import "testing"

func TestIDTypeLookupKeepsSchemaTokenBoundary(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(Contact.SObjectType,Id.valueOf('003000000000001').getSObjectType());
System.assertEquals(CaseComment.SObjectType,new CaseComment().getSObjectType());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
