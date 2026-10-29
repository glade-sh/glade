package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The ticket's native rows cover default/Boolean Database.update. Keep the existing
// local alias envelopes stable until those separate surfaces have native rows.
// This is a preservation check, not a Salesforce parity claim for the aliases.
func TestUpdateErrorMessagePreservesUncapturedAliases(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, method := range []string{"updateAsync", "updateImmediate"} {
			t.Run(api+"/"+method, func(t *testing.T) {
				org := testDataOrg()
				org.APIVersion = api
				account := org.Objects["Account"]
				account.Definition.Fields["Name"] = storage.Field{APIName: "Name", Type: storage.FieldString, Required: true}
				org.Objects["Account"] = account
				program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
Account row = new Account(Name = 'Original');
insert row;
row.Name = null;
Boolean failed = false;
try { Database.%s(row); }
catch (DmlException e) {
    failed = true;
    System.assert(e.getMessage().startsWith('Database.update failed: REQUIRED_FIELD_MISSING,'), e.getMessage());
}

System.assert(failed);
failed = false;
try { Database.%s(new Account(Name = 'Missing id')); }
catch (DmlException e) {
    failed = true;
    System.assertEquals('Update failed. First exception on row 0; first error: MISSING_ARGUMENT, Id not specified in an update call: []', e.getMessage());
}
System.assert(failed);
`, method, method), CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				machine.SetOrg(&org)
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// Options and explicit access-level overloads are outside the captured matrix.
// Preserve their existing local envelopes without claiming native parity.
func TestUpdateErrorMessagePreservesUncapturedOverloads(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []struct {
			name string
			args string
		}{
			{"options", "options"},
			{"access", "AccessLevel.SYSTEM_MODE"},
			{"boolean_access", "true, AccessLevel.SYSTEM_MODE"},
			{"options_access", "options, AccessLevel.SYSTEM_MODE"},
		} {
			for _, records := range []string{"row", "new List<Account>{row}"} {
				t.Run(api+"/"+route.name+"/"+records, func(t *testing.T) {
					org := testDataOrg()
					org.APIVersion = api
					account := org.Objects["Account"]
					account.Definition.Fields["Name"] = storage.Field{APIName: "Name", Type: storage.FieldString, Required: true}
					org.Objects["Account"] = account
					program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
Account row = new Account(Name = 'Original');
insert row;
row.Name = null;
Database.DMLOptions options = new Database.DMLOptions();
options.optAllOrNone = true;
Boolean failed = false;
try { Database.update(%s, %s); }
catch (DmlException e) {
    failed = true;
    System.assert(e.getMessage().startsWith('Database.update failed: REQUIRED_FIELD_MISSING,'), e.getMessage());
}
System.assert(failed);
`, records, route.args), CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					machine := New(nil)
					machine.SetOrg(&org)
					// Explicit SYSTEM_MODE requires a named/test execution context.
					machine.EnableTestContext()
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
