package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestSubscriberStaticSessionRoutes(t *testing.T) {
	const receive = `
insert new Account(Name = StaticSessionBox.Value);
StaticSessionBox.Value = 'subscriber value';
StaticSessionBox.Hits++;
`
	for _, route := range []string{"ordinary", "explicit", "stop"} {
		t.Run(route, func(t *testing.T) {
			body := receive
			if route != "ordinary" {
				body = "EventBus.publish(new SessionSignal__e());"
				if route == "explicit" {
					body += "Test.getEventBus().deliver();"
				}
			}
			program, err := CompileAnonymous(`
StaticSessionBox.Value = 'caller value';
StaticSessionBox.Hits = 0;
Test.startTest();
` + body + `
Test.stopTest();
System.assertEquals('subscriber value', StaticSessionBox.Value);
System.assertEquals(1, StaticSessionBox.Hits);
System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'caller value']);
`)
			if err != nil {
				t.Fatal(err)
			}
			subscriber, err := CompileAnonymous(receive)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			org.Objects["SessionSignal__e"] = storage.ObjectState{
				Definition: storage.ObjectDefinition{
					APIName:  "SessionSignal__e",
					Metadata: map[string]string{"publishBehavior": "PublishImmediately"},
					Fields:   map[string]storage.Field{},
				},
				Records: map[storage.ID]storage.Record{},
			}
			machine.SetOrg(&org)
			machine.EnableTestContext()
			if err := machine.RegisterClass(Class{
				Name: "StaticSessionBox",
				StaticFields: map[string]Field{
					"Value": {Name: "Value", Type: "String", Static: true},
					"Hits":  {Name: "Hits", Type: "Integer", Static: true},
				},
			}); err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterTrigger(Trigger{
				Name: "SessionSignalSubscriber", Object: "SessionSignal__e",
				Timing: triggerTimingAfter, Operation: "insert", Program: subscriber,
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
