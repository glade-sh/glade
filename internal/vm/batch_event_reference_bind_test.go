package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecEventOwnerReferenceBindAndBatchFieldControl(t *testing.T) {
	const seed = `
ServiceResource resource = new ServiceResource(Name = 'Meadow Resource', RelatedRecordId = UserInfo.getUserId(), IsActive = true, ResourceType = 'T');
insert resource;
Id meetingType = [SELECT Id FROM RecordType WHERE SObjectType = 'Event' AND DeveloperName = 'MeadowMeeting'].Id;
List<Event> meetings = new List<Event>{
new Event(Subject='Morning', RecordTypeId=meetingType, ShowAs='Busy', StartDateTime=Datetime.now().addHours(1), EndDateTime=Datetime.now().addHours(2)),
new Event(Subject='Free', RecordTypeId=meetingType, ShowAs='Free', StartDateTime=Datetime.now().addHours(2), EndDateTime=Datetime.now().addHours(3)),
new Event(Subject='Afternoon', RecordTypeId=meetingType, ShowAs='Busy', StartDateTime=Datetime.now().addHours(3), EndDateTime=Datetime.now().addHours(4))
};
insert meetings;
Datetime windowStart = Datetime.now().addHours(-1);
`
	for _, tc := range []struct {
		name   string
		source string
	}{
		{"owner_mapping", `
Set<Id> owners = new Set<Id>();
for (Event meeting : [SELECT OwnerId FROM Event]) owners.add(meeting.OwnerId);
System.assertEquals(1, owners.size());
System.assertEquals(1, [SELECT COUNT() FROM ServiceResource WHERE RelatedRecordId IN :owners AND IsActive = true]);
`},
		{"batch_field_bind", `Test.startTest(); Database.executeBatch(new MeadowBatch(), 100); Test.stopTest(); System.assertEquals(2, [SELECT COUNT() FROM Account WHERE Name LIKE 'Meadow %']);`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testDataOrg()
			configureStandardField(&org, "Event", "RecordTypeId")
			storage.EnsureStandardObject(&org, "Event")
			storage.EnsureStandardObject(&org, "ServiceResource")
			event := org.Objects["Event"]
			event.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000009", DeveloperName: "MeadowMeeting", Name: "Meadow Meeting", Active: true, Available: true}}
			org.Objects["Event"] = event
			storage.EnsureDeterministicPlatformData(&org)
			machine := New(nil)
			machine.SetOrg(&org)
			machine.EnableTestContext()
			if tc.name == "batch_field_bind" {
				constructor, err := CompileAnonymous(`this.windowStart = Datetime.now().addHours(-1); this.query = 'SELECT Id,OwnerId,StartDateTime,EndDateTime,ShowAs,Subject FROM Event WHERE RecordType.DeveloperName = \'MeadowMeeting\' AND LastModifiedDate > :windowStart AND ShowAs != \'Free\'';`)
				if err != nil {
					t.Fatal(err)
				}
				start, err := CompileAnonymous(`return Database.getQueryLocator(query);`)
				if err != nil {
					t.Fatal(err)
				}
				execute, err := CompileAnonymous(`List<Account> copies = new List<Account>(); for (Event meeting : scope) copies.add(new Account(Name = 'Meadow ' + meeting.Subject)); insert copies;`)
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterClass(Class{Name: "MeadowBatch", Interfaces: []string{"Database.Batchable<SObject>"},
					Fields:       map[string]Field{"windowStart": {Name: "windowStart", Type: "Datetime"}, "query": {Name: "query", Type: "String"}},
					Constructors: []Method{{Name: "MeadowBatch.<init>", ClassName: "MeadowBatch", IsConstructor: true, Program: constructor}},
					Methods: map[string]Method{
						"start":   {Name: "MeadowBatch.start", ClassName: "MeadowBatch", ReturnType: "Database.QueryLocator", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: start},
						"execute": {Name: "MeadowBatch.execute", ClassName: "MeadowBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<Event>"}}, Program: execute},
						"finish":  {Name: "MeadowBatch.finish", ClassName: "MeadowBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}},
					},
				}); err != nil {
					t.Fatal(err)
				}
			}
			program, err := CompileAnonymous(seed + tc.source)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
