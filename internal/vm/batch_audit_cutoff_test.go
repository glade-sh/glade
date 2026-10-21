package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecBatchAuditCutoffWithQueriedOwnerBind(t *testing.T) {
	for _, tc := range []struct {
		name, cutoff string
		want         int
	}{
		{"current_cutoff", "Datetime.now()", 0},
		{"earlier_cutoff_control", "Datetime.now().addHours(-1)", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			constructor, err := CompileAnonymous(`this.cutoff = requestedCutoff;`)
			if err != nil {
				t.Fatal(err)
			}
			start, err := CompileAnonymous(`return Database.getQueryLocator('SELECT Id,OwnerId,Subject FROM Event WHERE LastModifiedDate > :cutoff AND ShowAs != \'Free\'');`)
			if err != nil {
				t.Fatal(err)
			}
			execute, err := CompileAnonymous(`
Set<Id> owners = new Set<Id>();
for (Event meeting : scope) owners.add(meeting.OwnerId);
Set<String> mappedOwners = new Set<String>();
for (ServiceResource resource : [SELECT RelatedRecordId FROM ServiceResource WHERE RelatedRecordId IN :owners AND IsActive = true]) {
    mappedOwners.add(String.valueOf(resource.RelatedRecordId));
}
List<Account> copies = new List<Account>();
for (Event meeting : scope) {
    if (mappedOwners.contains(String.valueOf(meeting.OwnerId))) {
        copies.add(new Account(Name = 'Meadow Copy ' + meeting.Subject));
    }
}
insert copies;
`)
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			storage.EnsureStandardObject(&org, "Event")
			storage.EnsureStandardObject(&org, "ServiceResource")
			storage.EnsureDeterministicPlatformData(&org)
			machine := New(nil)
			machine.SetOrg(&org)
			machine.EnableTestContext()
			if err := machine.RegisterClass(Class{
				Name:       "MeadowAuditBatch",
				Interfaces: []string{"Database.Batchable<SObject>"},
				Fields:     map[string]Field{"cutoff": {Name: "cutoff", Type: "Datetime"}},
				Constructors: []Method{{
					Name: "MeadowAuditBatch.<init>", ClassName: "MeadowAuditBatch", IsConstructor: true,
					Params: []Param{{Name: "requestedCutoff", Type: "Datetime"}}, Program: constructor,
				}},
				Methods: map[string]Method{
					"start":   {Name: "MeadowAuditBatch.start", ClassName: "MeadowAuditBatch", ReturnType: "Database.QueryLocator", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: start},
					"execute": {Name: "MeadowAuditBatch.execute", ClassName: "MeadowAuditBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<Event>"}}, Program: execute},
					"finish":  {Name: "MeadowAuditBatch.finish", ClassName: "MeadowAuditBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}},
				},
			}); err != nil {
				t.Fatal(err)
			}
			program, err := CompileAnonymous(fmt.Sprintf(`
insert new ServiceResource(Name = 'Meadow Resource', RelatedRecordId = UserInfo.getUserId(), IsActive = true, ResourceType = 'T');
insert new Account(Name = 'Earlier One');
insert new Account(Name = 'Earlier Two');
insert new Account(Name = 'Earlier Three');
List<Event> meetings = new List<Event>{
    new Event(Subject = 'Morning', ShowAs = 'Busy', StartDateTime = Datetime.now().addHours(1), EndDateTime = Datetime.now().addHours(2)),
    new Event(Subject = 'Free', ShowAs = 'Free', StartDateTime = Datetime.now().addHours(2), EndDateTime = Datetime.now().addHours(3)),
    new Event(Subject = 'Afternoon', ShowAs = 'Busy', StartDateTime = Datetime.now().addHours(3), EndDateTime = Datetime.now().addHours(4))
};
insert meetings;
Test.startTest();
Database.executeBatch(new MeadowAuditBatch(%s), 100);
Test.stopTest();
System.assertEquals(%d, [SELECT COUNT() FROM Account WHERE Name LIKE 'Meadow Copy %%']);
`, tc.cutoff, tc.want))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				for _, meeting := range machine.Org.Objects["Event"].Records {
					t.Logf("stored LastModifiedDate=%s", meeting.System.LastModifiedDate)
				}
				t.Fatalf("VM now=%s: %v", machine.fakeNow, err)
			}
		})
	}
}
