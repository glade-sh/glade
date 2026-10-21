package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Apex method names are case-insensitive (Apex Developer Guide, Case
// Sensitivity). All spellings must reach the existing EventBus.publish path.
// https://trailhead.salesforce.com/content/learn/modules/apex_database/apex_database_intro
// https://developer.salesforce.com/docs/platform/platform-events/guide/platform-events-publish-apex.html
func TestExecEventPublishCasing(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, callee := range []string{"EventBus.publish", "Eventbus.publish", "EVENTBUS.PUBLISH", "System.Eventbus.PuBlIsH"} {
			for _, list := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/list=%t", api, callee, list), func(t *testing.T) {
					argument := "new DispatchNotice__e(Message__c='Owned notice')"
					body := fmt.Sprintf("Database.SaveResult answer = %s(%s); System.assert(answer.isSuccess());", callee, argument)
					if list {
						body = fmt.Sprintf("List<Database.SaveResult> answers = %s(new List<DispatchNotice__e>{%s}); System.assertEquals(1, answers.size()); System.assert(answers[0].isSuccess());", callee, argument)
					}
					program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := testDataOrg()
					org.Objects["DispatchNotice__e"] = storage.ObjectState{
						Definition: storage.ObjectDefinition{
							APIName:   "DispatchNotice__e",
							KeyPrefix: "e00",
							Metadata:  map[string]string{"publishBehavior": "PublishImmediately"},
							Fields: map[string]storage.Field{
								"Message__c": {APIName: "Message__c", Type: storage.FieldString},
							},
						},
						Records: map[storage.ID]storage.Record{},
					}
					machine := New(nil)
					machine.SetOrg(&org)
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestExecEventPublishCasingKeepsProjectShadow(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "EventBus"}); err != nil {
		t.Fatal(err)
	}
	body, err := CompileAnonymous(`return 'owned:' + value;`)
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name: "EventBus.publish", ClassName: "EventBus", IsStatic: true,
		ReturnType: "String", Params: []Param{{Name: "value", Type: "String"}}, Program: body,
	}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals('owned:first', EventBus.publish('first'));
System.assertEquals('owned:second', Eventbus.PUBLISH('second'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecEventPublishCasingKeepsInstanceFieldReceiver(t *testing.T) {
	machine := New(nil)
	for _, class := range []Class{
		{Name: "OwnedPublisher"},
		{Name: "NoticeOwner", Fields: map[string]Field{
			"EventBus": {Name: "EventBus", Type: "OwnedPublisher", InitialValue: Object("OwnedPublisher")},
		}},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	for _, method := range []struct {
		owner, name, returns, body string
		params                     []Param
	}{
		{"OwnedPublisher", "publish", "String", `return 'field:' + value;`, []Param{{Name: "value", Type: "String"}}},
		{"NoticeOwner", "run", "void", `System.assertEquals('field:first', EventBus.publish('first')); System.assertEquals('field:second', Eventbus.PUBLISH('second'));`, nil},
	} {
		body, err := CompileAnonymous(method.body)
		if err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterMethod(Method{
			Name: method.owner + "." + method.name, ClassName: method.owner,
			ReturnType: method.returns, Params: method.params, Program: body,
		}); err != nil {
			t.Fatal(err)
		}
	}
	program, err := CompileAnonymous(`new NoticeOwner().run();`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

// Apex identifiers are case-insensitive, including platform class and method
// names: https://developer.salesforce.com/blogs/2022/09/an-introduction-to-apex-for-java-developers
// Publication/validation outcomes reuse the owned platform_events observations.
// This test never delivers events or asserts subscriber ordering.
func TestEventBusPublishCasingPreservesSaveResults(t *testing.T) {
	for _, callee := range []string{
		"EventBus.publish", "System.EventBus.publish", "Eventbus.publish",
		"eventbus.PUBLISH", "EVENTBUS.Publish", "SYSTEM.EVENTBUS.PUBLISH",
	} {
		t.Run(callee, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
Database.SaveResult accepted = %[1]s(new Notice__e(Payload__c = 'message'));
System.assert(accepted.isSuccess());
Database.SaveResult rejected = %[1]s(new Notice__e());
System.assert(!rejected.isSuccess());
System.assertEquals('REQUIRED_FIELD_MISSING', String.valueOf(rejected.getErrors()[0].getStatusCode()));
List<Database.SaveResult> rows = %[1]s(new List<Notice__e>{new Notice__e(Payload__c = 'first'), new Notice__e()});
System.assertEquals(2, rows.size());
System.assert(rows[0].isSuccess());
System.assert(!rows[1].isSuccess());
List<Database.SaveResult> empty = %[1]s(new List<Notice__e>());
System.assertEquals(0, empty.size());
`, callee), CompileOptions{APIVersion: "65.0"})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.Objects["Notice__e"] = storage.ObjectState{
				Definition: storage.ObjectDefinition{
					APIName: "Notice__e", KeyPrefix: "e00",
					Metadata: map[string]string{"publishBehavior": "PublishImmediately"},
					Fields: map[string]storage.Field{
						"Payload__c": {APIName: "Payload__c", Type: storage.FieldString, Required: true},
					},
				},
				Records: map[storage.ID]storage.Record{},
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

func TestEventBusPublishCasingPreservesReceiverBindings(t *testing.T) {
	for _, binding := range []string{"local", "field", "class"} {
		t.Run(binding, func(t *testing.T) {
			machine := New(nil)
			body, err := CompileAnonymous(`return message + '-receiver';`)
			if err != nil {
				t.Fatal(err)
			}
			name := "PublicationReceiver"
			if binding == "class" {
				name = "Eventbus"
			}
			if err := machine.RegisterClass(Class{
				Name: name,
				Methods: map[string]Method{
					"publish": {Name: name + ".publish", ClassName: name, IsStatic: binding == "class", ReturnType: "String", Params: []Param{{Name: "message", Type: "String"}}, Program: body},
				},
			}); err != nil {
				t.Fatal(err)
			}
			source := `System.assertEquals('message-receiver', Eventbus.publish('message'));`
			if binding == "local" {
				source = `PublicationReceiver Eventbus = new PublicationReceiver();` + source
			}
			if binding == "field" {
				check, err := CompileAnonymous(source)
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterClass(Class{
					Name: "PublicationOwner",
					Fields: map[string]Field{
						"Eventbus": {Name: "Eventbus", Type: "PublicationReceiver", InitialValue: Object("PublicationReceiver")},
					},
					Methods: map[string]Method{
						"check": {Name: "PublicationOwner.check", ClassName: "PublicationOwner", ReturnType: "void", Program: check},
					},
				}); err != nil {
					t.Fatal(err)
				}
				source = `new PublicationOwner().check();`
			}
			program, err := CompileAnonymous(source)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
