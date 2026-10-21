package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// StandardController.getId returns String. The owned V04 e_getId_cast native
// diagnostic identifies String as the return type, independently of the public
// Apex Reference Guide signature. These assertions exercise String members;
// the owned controller identity conformance rows separately assert text width.
func TestStandardControllerIDSupportsStringMembers(t *testing.T) {
	for _, api := range []string{"62.0", "66.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
DispatchEntry__c inserted = new DispatchEntry__c(Name='Owned entry');
insert inserted;
for (DispatchEntry__c row : new List<DispatchEntry__c>{
    inserted,
    new DispatchEntry__c(Id=String.valueOf(inserted.Id).substring(0,15)),
    new DispatchEntry__c(Id=String.valueOf(inserted.Id))
}) {
    ApexPages.StandardController controller = new ApexPages.StandardController(row);
    String text = String.valueOf(controller.getId());
    System.assertEquals(text.length(), controller.getId().length());
    System.assertEquals(text.substring(0,3), controller.getId().substring(0,3));
    System.assertEquals('task_' + text, 'task_' + controller.getId());
    System.assertEquals(text + '_task', controller.getId() + '_task');
    System.assertEquals(row.Id, (Id)controller.getId());
}
ApexPages.StandardController empty = new ApexPages.StandardController(new DispatchEntry__c());
System.assertEquals(null, empty.getId());
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			org.Objects["DispatchEntry__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
				APIName: "DispatchEntry__c", KeyPrefix: "a14",
				Fields: map[string]storage.Field{
					"Id":   {APIName: "Id", Type: storage.FieldID},
					"Name": {APIName: "Name", Type: storage.FieldString},
				},
			}}
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
