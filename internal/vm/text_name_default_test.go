package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Check local delegation to the same engine default with and without test
// context. This does not establish native default-mode or @IsTest behavior;
// security_access N001-N003/N009-N011 cover anonymous USER_MODE insert only.
func TestExecDeclaredTextNameDefersToInsertDefault(t *testing.T) {
	for _, operation := range []string{"insert", "upsert"} {
		for _, name := range []string{"", "Name=null", "Name=''"} {
			t.Run(fmt.Sprintf("%s/%s", operation, name), func(t *testing.T) {
				program, err := CompileAnonymous(fmt.Sprintf("NameLedger__c row=new NameLedger__c(%s); %s row;", name, operation))
				if err != nil {
					t.Fatal(err)
				}
				var reference storage.Record
				for _, testMode := range []bool{false, true} {
					machine := New(nil)
					org := textNameDefaultOrg()
					machine.SetOrg(&org)
					if testMode {
						machine.EnableTestContext()
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
					records := machine.Org.Objects["NameLedger__c"].Records
					if len(records) != 1 {
						t.Fatalf("test=%t: stored records=%d, want 1", testMode, len(records))
					}
					for _, record := range records {
						if !testMode {
							reference = record
							continue
						}
						if record.ID != reference.ID {
							t.Fatalf("test context changed generated Id: got %q, reference %q", record.ID, reference.ID)
						}
						got, gotPresent := record.GetField("Name")
						want, wantPresent := reference.GetField("Name")
						if gotPresent != wantPresent || got.Kind != want.Kind || got.String != want.String {
							t.Fatalf("test context replaced engine Name default: got %#v (present=%t), reference %#v (present=%t)", got, gotPresent, want, wantPresent)
						}
					}
				}
			})
		}
	}
}

func TestExecDeclaredTextNameDefaultControls(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(*storage.ObjectDefinition)
		code  string
	}{
		{"explicit", nil, `NameLedger__c row=new NameLedger__c(Name='Supplied'); insert row; System.assertEquals('Supplied',[SELECT Name FROM NameLedger__c WHERE Id=:row.Id].Name);`},
		{"default", func(d *storage.ObjectDefinition) {
			f := d.Fields["Name"]
			f.DefaultValue = "'Configured'"
			d.Fields["Name"] = f
		}, `NameLedger__c row=new NameLedger__c(); insert row; System.assertEquals('Configured',[SELECT Name FROM NameLedger__c WHERE Id=:row.Id].Name);`},
		{"autonumber", func(d *storage.ObjectDefinition) {
			f := d.Fields["Name"]
			f.AutoNumber = true
			f.DisplayFormat = "LED-{0000}"
			d.Fields["Name"] = f
			d.Metadata["nameFieldType"] = "AutoNumber"
		}, `NameLedger__c row=new NameLedger__c(); insert row; System.assert([SELECT Name FROM NameLedger__c WHERE Id=:row.Id].Name.startsWith('LED-'));`},
		{"incompleteMetadata", func(d *storage.ObjectDefinition) { delete(d.Metadata, "nameFieldType") }, `NameLedger__c row=new NameLedger__c(); insert row; System.assertEquals('Ledger Entry',[SELECT Name FROM NameLedger__c WHERE Id=:row.Id].Name);`},
		{"requiredField", func(d *storage.ObjectDefinition) {
			d.Fields["Required__c"] = storage.Field{APIName: "Required__c", Type: storage.FieldString, Required: true}
		}, `Database.SaveResult saved=Database.insert(new NameLedger__c(),false); System.assertEquals(false,saved.isSuccess()); System.assertEquals('REQUIRED_FIELD_MISSING',String.valueOf(saved.getErrors()[0].getStatusCode()));`},
	} {
		t.Run(test.name, func(t *testing.T) {
			org := textNameDefaultOrg()
			object := org.Objects["NameLedger__c"]
			if test.setup != nil {
				test.setup(&object.Definition)
			}
			org.Objects["NameLedger__c"] = object
			program, err := CompileAnonymous(test.code)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			machine.EnableTestContext()
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func textNameDefaultOrg() storage.OrgState {
	org := testDataOrg()
	org.Objects["NameLedger__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "NameLedger__c", KeyPrefix: "a0n", Label: "Ledger Entry",
		Metadata: map[string]string{"nameFieldType": "Text"},
		Fields: map[string]storage.Field{
			"Id":   {APIName: "Id", Type: storage.FieldID},
			"Name": {APIName: "Name", Type: storage.FieldString, Required: true},
		},
	}}
	return org
}
