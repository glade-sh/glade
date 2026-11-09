package vm

import "testing"

// Native: internal/apextest/testdata/conformance/csnpe_capture/org.tsv,
// CS_LIST_EXCEPTION and CS_INSTANCE_EXCEPTION at both APIs and routes.
func TestCustomSettingNullFieldAccessorVariants(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, accessor := range []string{"getValues", "getInstance"} {
			t.Run(api+"/"+accessor, func(t *testing.T) {
				source := `Local_Setting__c s = Local_Setting__c.` + accessor + `('Missing');
Boolean caught = false;
try { String v = s.Optional__c; } catch (NullPointerException e) {
 caught = true; System.assertEquals('Attempt to de-reference a null object', e.getMessage());
}
System.assert(caught);`
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				org := customDataOrg()
				machine.SetOrg(&org)
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
