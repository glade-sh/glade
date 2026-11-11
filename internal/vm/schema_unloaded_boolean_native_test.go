package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// load_defaults_boolean_false/org.tsv records presence before and after read,
// after an explicit write, and the loaded control for these standard fields.
func TestSchemaUnloadedBooleanPresenceNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ object, field string }{
			{"Product2", "IsActive"}, {"Opportunity", "IsPrivate"}, {"Task", "IsReminderSet"},
		} {
			t.Run(api+"/"+tc.object, func(t *testing.T) {
				source := fmt.Sprintf(`
SObject blank=%s.SObjectType.newSObject(null,false);
System.assertEquals(false,blank.getPopulatedFieldsAsMap().containsKey('%s'));
System.assertEquals(null,blank.get('%s'));
System.assertEquals(false,blank.getPopulatedFieldsAsMap().containsKey('%s'));
blank.put('%s',false);
System.assertEquals(true,blank.getPopulatedFieldsAsMap().containsKey('%s'));
SObject loaded=%s.SObjectType.newSObject(null,true);
System.assertEquals(false,loaded.get('%s'));
System.assertEquals(true,loaded.getPopulatedFieldsAsMap().containsKey('%s'));
`, tc.object, tc.field, tc.field, tc.field, tc.field, tc.field, tc.object, tc.field, tc.field)
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := testDataOrg()
				storage.EnsureStandardObject(&org, tc.object)
				machine := New(nil)
				machine.SetOrg(&org)
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
