package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// No native row captures a standard compound read after a component edit
// (DML_CONTROL_STANDARD never reads it), so BillingAddress keeps following
// BillingStreet. Only the custom DML_COMPONENT_AFTER_COMPOUND row backs a
// queried-compound snapshot.
func TestStandardAddressFollowsComponentEdit(t *testing.T) {
	bodies := map[string]string{
		"chained": `System.assertEquals('Three', r.BillingAddress.getStreet());`,
		"direct":  `Address a = r.BillingAddress; System.assertEquals('Three', a.getStreet());`,
	}
	for _, api := range []string{"62.0", "67.0"} {
		for name, read := range bodies {
			t.Run(api+"/"+name, func(t *testing.T) {
				org := testDataOrg()
				storage.EnsureStandardObject(&org, "Account")
				org.APIVersion = api
				machine := New(nil)
				machine.EnableTestContext()
				machine.SetOrg(&org)
				program, err := CompileAnonymousWithOptions(`
Account r = new Account(Name='Address Review', BillingStreet='One', BillingLatitude=12.5, BillingLongitude=25.0);
insert r;
r = [SELECT BillingAddress, BillingStreet FROM Account WHERE Id=:r.Id];
r.BillingStreet = 'Three';
`+read, CompileOptions{APIVersion: api})
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
