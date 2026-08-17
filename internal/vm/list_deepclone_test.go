package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestListDeepClonePreserveOptions(t *testing.T) {
	for _, test := range []struct {
		name       string
		arguments  string
		assertions string
	}{
		{
			name:      "defaults clear system and autonumber fields",
			arguments: "",
			assertions: `
System.assertEquals(null, clone.Id);
System.assertEquals(null, clone.CreatedById);
System.assertEquals(null, clone.CreatedDate);
System.assertEquals(null, clone.LastModifiedById);
System.assertEquals(null, clone.LastModifiedDate);
System.assertEquals(null, clone.Auto__c);`,
		},
		{
			name:      "all false clears system and autonumber fields",
			arguments: "false, false, false",
			assertions: `
System.assertEquals(null, clone.Id);
System.assertEquals(null, clone.CreatedById);
System.assertEquals(null, clone.CreatedDate);
System.assertEquals(null, clone.LastModifiedById);
System.assertEquals(null, clone.LastModifiedDate);
System.assertEquals(null, clone.Auto__c);`,
		},
		{
			name:      "all true retains system and autonumber fields",
			arguments: "true, true, true",
			assertions: `
System.assertEquals(source.Id, clone.Id);
System.assertEquals(source.CreatedById, clone.CreatedById);
System.assertEquals(source.CreatedDate, clone.CreatedDate);
System.assertEquals(source.LastModifiedById, clone.LastModifiedById);
System.assertEquals(source.LastModifiedDate, clone.LastModifiedDate);
System.assertEquals(source.Auto__c, clone.Auto__c);`,
		},
		{
			name:      "mixed flags apply independently",
			arguments: "true, false, true",
			assertions: `
System.assertEquals(source.Id, clone.Id);
System.assertEquals(null, clone.CreatedById);
System.assertEquals(null, clone.CreatedDate);
System.assertEquals(null, clone.LastModifiedById);
System.assertEquals(null, clone.LastModifiedDate);
System.assertEquals(source.Auto__c, clone.Auto__c);`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			program, err := CompileAnonymous(`
DeepCloneProbe__c source = new DeepCloneProbe__c(
    Id = 'a00000000000001AAA',
    CreatedById = '005000000000001AAA',
    CreatedDate = DateTime.newInstance(2026, 1, 2, 3, 4, 5),
    LastModifiedById = '005000000000002AAA',
    LastModifiedDate = DateTime.newInstance(2026, 2, 3, 4, 5, 6),
    Auto__c = 'AUTO-1',
    Name = 'source'
);
DeepCloneProbe__c clone = new List<DeepCloneProbe__c>{source}.deepClone(` + test.arguments + `)[0];
` + test.assertions + `
clone.Name = 'clone';
System.assertEquals('source', source.Name);
`)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := storage.NewOrgState()
			org.Objects["DeepCloneProbe__c"] = storage.ObjectState{
				Definition: storage.ObjectDefinition{
					APIName: "DeepCloneProbe__c",
					Fields: map[string]storage.Field{
						"Name":    {APIName: "Name", Type: storage.FieldString},
						"Auto__c": {APIName: "Auto__c", Type: storage.FieldString, AutoNumber: true},
					},
				},
			}
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
