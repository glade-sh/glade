package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecHierarchyCustomSettingSetupOwnerGetSObject(t *testing.T) {
	program, err := CompileAnonymous(`
List<SObject> rows = [SELECT SetupOwnerId, SetupOwner.Name, SetupOwner.Type FROM Hierarchy_Setting__c];
SObject owner = rows[0].getSObject('SetupOwner');
System.assertNotEquals(null, owner);
System.assertEquals('Alice', owner.get('Name'));
System.assertEquals('User', owner.get('Type'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := customDataOrg()
	org.Objects["User"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "User",
			KeyPrefix: "005",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
				"Type": {APIName: "Type", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"005000000000001": {
				ID:     "005000000000001",
				Object: "User",
				Fields: map[string]storage.Value{
					"Name": storage.StringValue("Alice"),
					"Type": storage.StringValue("User"),
				},
			},
		},
	}
	for _, objectName := range []string{"Organization", "Profile"} {
		org.Objects[objectName] = storage.ObjectState{Definition: storage.ObjectDefinition{
			APIName: objectName,
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
				"Type": {APIName: "Type", Type: storage.FieldString},
			},
		}}
	}
	hierarchy := org.Objects["Hierarchy_Setting__c"]
	hierarchy.Records["a02000000000001"].Fields["SetupOwnerId"] = storage.StringValue("005000000000001")
	org.Objects["Hierarchy_Setting__c"] = hierarchy
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
