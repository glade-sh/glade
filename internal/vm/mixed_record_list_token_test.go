package vm

import "testing"

// Preserve the existing homogeneous and concrete-runtime controls while the
// exact heterogeneous generic-list token result awaits native capture.
func TestExecMixedRecordListTokenPreservesConcreteControls(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, withOrg := range []bool{false, true} {
			name := api + "/withoutOrg"
			if withOrg {
				name = api + "/withOrg"
			}
			t.Run(name, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(`
List<SObject> mixed=new List<SObject>{new Account(),new Contact()};
Object observed=mixed.getSObjectType();
List<SObject> same=new List<SObject>{new Account(),new Account()};
System.assertEquals(Account.SObjectType,same.getSObjectType());
List<SObject> qualified=new List<SObject>{new Account(),new Schema.Account()};
System.assertEquals(Account.SObjectType,qualified.getSObjectType());
List<SObject> view=new List<Contact>{new Contact()};
System.assertEquals(Contact.SObjectType,view.getSObjectType());
List<SObject> empty=new List<SObject>();
System.assertEquals(null,empty.getSObjectType());
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				if withOrg {
					org := testDataOrg()
					machine.SetOrg(&org)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
				// Compare actual dispatch with the metadata-aware handler on the
				// same list. This asserts local dispatch consistency, without
				// treating the provisional mixed-list result as a native oracle.
				receiver, err := machine.lookup("mixed")
				if err != nil {
					t.Fatal(err)
				}
				want, handled, err := machine.callListValueMember("mixed", receiver, "getSObjectType", nil, &Result{})
				if err != nil || !handled {
					t.Fatalf("metadata-aware call: handled=%t, error=%v", handled, err)
				}
				if got := machine.Globals["observed"]; !got.Equal(want) {
					t.Fatalf("dispatch returned %s; metadata-aware handler returned %s", got.String(), want.String())
				}
			})
		}
	}
}
