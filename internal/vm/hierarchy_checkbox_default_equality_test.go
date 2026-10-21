package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Hierarchy and ordinary equality are observed in hierarchy_checkbox_equality.
// Ordinary custom-object controls also retain sobject_projection_defaults B01/B02.
func TestHierarchyCheckboxDefaultEquality(t *testing.T) {
	for _, setting := range []bool{false, true} {
		name := "ordinary_custom_control"
		if setting {
			name = "hierarchy"
		}
		t.Run(name, func(t *testing.T) {
			org := testDataOrg()
			metadata := map[string]string{}
			if setting {
				metadata = map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}
			}
			org.Objects["QuietPreference__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
				APIName: "QuietPreference__c", Label: "Quiet Preference", KeyPrefix: "a02", Metadata: metadata,
				Fields: map[string]storage.Field{
					"Id":       {APIName: "Id", Type: storage.FieldID},
					"Muted__c": {APIName: "Muted__c", Type: storage.FieldBoolean, DefaultValue: "false"},
				},
			}}
			machine := New(nil)
			machine.SetOrg(&org)
			program, err := CompileAnonymous(`
QuietPreference__c left=(QuietPreference__c)QuietPreference__c.SObjectType.newSObject(null,true);
QuietPreference__c right=(QuietPreference__c)QuietPreference__c.SObjectType.newSObject(null,true);
System.assertEquals(left,right,'unmodified defaults control');
right.Id=null;
System.assertEquals(left,right,'cleared identity control');
left.Muted__c=false;
System.assertEquals(false,left.Muted__c);
System.assertEquals(false,right.Muted__c);
System.assertEquals(left,right,'false default assignment');
// Preserve existing local scope controls without claiming new native answers.
left.Muted__c=true;
System.assertNotEquals(left,right,'true assignment control');
QuietPreference__c unloadedLeft=(QuietPreference__c)QuietPreference__c.SObjectType.newSObject(null,false);
QuietPreference__c unloadedRight=(QuietPreference__c)QuietPreference__c.SObjectType.newSObject(null,false);
unloadedLeft.Muted__c=false;
System.assertNotEquals(unloadedLeft,unloadedRight,'unloaded default control');
`)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
