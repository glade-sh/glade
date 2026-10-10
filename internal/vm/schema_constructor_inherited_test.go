package vm

import (
	"fmt"
	"testing"
)

// the native compatibility contract binds these exact assertions to retained Salesforce anonymous
// invocations with project/request API 67. The other versions below are local
// regressions, not Salesforce interval or complete constructor proof.
func TestInheritedSchemaConstructorFacetsVersioned(t *testing.T) {
	cases := []struct {
		row      int
		contract string
		source   string
	}{
		{393, "apex.behavior.schema.sobjecttype-newsobject", `
SObject value = Account.SObjectType.newSObject();
System.assertNotEquals(null, value);
`},
		{394, "apex.behavior.schema.sobjecttype-newsobject-id", `
Id recordId = '001000000000001AAA';
SObject value = Account.SObjectType.newSObject(recordId);
System.assertEquals(Account.SObjectType, value.getSObjectType());
System.assertEquals(recordId, value.get('Id'));
`},
		{395, "apex.behavior.schema.sobjecttype-newsobject-recordtype-defaults", `
SObject value = Account.SObjectType.newSObject(null, true);
System.assertNotEquals(null, value);
`},
	}
	for api := 62; api <= 67; api++ {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("API%d/row%d", api, tc.row), func(t *testing.T) {
				t.Log(tc.contract)
				program, err := CompileAnonymousWithOptions(tc.source, CompileOptions{
					APIVersion: fmt.Sprintf("%d.0", api),
				})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				org := testDataOrg()
				machine.SetOrg(&org)
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
