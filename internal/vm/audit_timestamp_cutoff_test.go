package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestExecAuditTimestampCutoffAfterEarlierStatements(t *testing.T) {
	const source = `
insert new Account(Name='Meadow One');
insert new Account(Name='Meadow Two');
insert new Account(Name='Meadow Three');
List<Event> meetings=new List<Event>{
 new Event(Subject='Morning',ShowAs='Busy',StartDateTime=Datetime.now().addHours(1),EndDateTime=Datetime.now().addHours(2)),
 new Event(Subject='Free',ShowAs='Free',StartDateTime=Datetime.now().addHours(2),EndDateTime=Datetime.now().addHours(3)),
 new Event(Subject='Afternoon',ShowAs='Busy',StartDateTime=Datetime.now().addHours(3),EndDateTime=Datetime.now().addHours(4))};
insert meetings;
Test.startTest();
Datetime cutoff=Datetime.now();
Datetime earlier=cutoff.addHours(-1);
System.assertEquals(0,[SELECT COUNT() FROM Event WHERE LastModifiedDate > :cutoff AND ShowAs != 'Free']);
System.assertEquals(2,[SELECT COUNT() FROM Event WHERE LastModifiedDate > :earlier AND ShowAs != 'Free']);
Test.stopTest();
`
	program, err := CompileAnonymous(source)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Event")
	storage.EnsureDeterministicPlatformData(&org)
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	result, err := machine.Execute(program)
	if err != nil {
		event := org.Objects["Event"]
		for _, row := range event.Records {
			t.Logf("stored LastModifiedDate=%s", row.System.LastModifiedDate)
		}
		t.Logf("VM now=%s debug=%v", machine.fakeNow, result.Debug)
		t.Fatal(err)
	}
}
