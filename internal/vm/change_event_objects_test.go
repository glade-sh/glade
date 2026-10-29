package vm

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestChangeEventTriggerCompileRegisterAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				org := testDataOrg()
				org.APIVersion = api
				org.Objects["Parcel__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
					APIName: "Parcel__c", Fields: map[string]storage.Field{
						"Name":      {APIName: "Name", Type: storage.FieldString},
						"Marker__c": {APIName: "Marker__c", Type: storage.FieldString},
					},
				}}
				machine := New(nil)
				machine.SetOrg(&org)
				if route == "istest" {
					machine.EnableTestContext()
				}
				for _, object := range []string{"AccountChangeEvent", "Parcel__ChangeEvent"} {
					body, err := CompileAnonymousWithOptions("System.assertEquals('"+object+"',Trigger.new[0].getSObjectType().getDescribe().getName());", CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterTrigger(Trigger{Name: object + "Observer", Object: object, Timing: "after", Operation: "insert", APIVersion: api, Program: body}); err != nil {
						t.Fatal(err)
					}
					if len(machine.Triggers[object]) != 1 {
						t.Fatalf("%s trigger was not registered", object)
					}
				}
			})
		}
	}
}

func TestChangeEventTriggerRuntimeObjectsAnonymousAndIsTestAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				org := testDataOrg()
				org.APIVersion = api
				storage.EnsureStandardObject(&org, "Account")
				custom := storage.ObjectDefinition{APIName: "Parcel__c", Fields: map[string]storage.Field{
					"Name":      {APIName: "Name", Type: storage.FieldString},
					"Marker__c": {APIName: "Marker__c", Type: storage.FieldString},
				}}
				storage.EnsureStandardObjectFields(&custom)
				org.Objects["Parcel__c"] = storage.ObjectState{Definition: custom}
				machine := New(nil)
				machine.SetOrg(&org)
				if route == "istest" {
					machine.EnableTestContext()
				}
				for _, object := range []string{"AccountChangeEvent", "Parcel__ChangeEvent"} {
					t.Run(object, func(t *testing.T) {
						if err := machine.RegisterTrigger(Trigger{Name: object + "Observer", Object: object, Timing: "after", Operation: "insert", APIVersion: api}); err != nil {
							t.Fatal(err)
						}
						state, ok := machine.Org.Objects[object]
						if !ok {
							t.Fatalf("registered %s trigger has no runtime storage object; native exposes base fields, ChangeEventHeader and ReplayId", object)
						}
						fields := []string{"Name", "Id", "CreatedDate", "OwnerId", "ChangeEventHeader", "ReplayId"}
						if object == "Parcel__ChangeEvent" {
							fields = append(fields, "Marker__c")
						} else {
							fields = append(fields, "Phone")
						}
						for _, field := range fields {
							if _, ok := state.Definition.Fields[field]; !ok {
								t.Errorf("native field %s.%s missing", object, field)
							}
						}
						if _, ok := state.Definition.Fields["Missing__c"]; ok {
							t.Error("unknown field materialized")
						}
					})
				}
			})
		}
	}
}

func TestChangeEventTriggerOrgInstallationRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			for _, install := range []string{"register_after_org", "set_org_after_register", "template_org_after_register"} {
				t.Run(api+"/"+route+"/"+install, func(t *testing.T) {
					org := testDataOrg()
					org.APIVersion = api
					org.Objects["Parcel__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
						APIName: "Parcel__c", Fields: map[string]storage.Field{"Marker__c": {APIName: "Marker__c", Type: storage.FieldString}},
					}}
					machine := New(nil)
					if install == "register_after_org" {
						machine.SetOrg(&org)
					}
					for _, name := range []string{"AccountChangeEvent", "Parcel__ChangeEvent"} {
						if err := machine.RegisterTrigger(Trigger{Name: name + "Observer", Object: name, Timing: "after", Operation: "insert", APIVersion: api}); err != nil {
							t.Fatal(err)
						}
					}
					if install == "set_org_after_register" {
						machine.SetOrg(&org)
					} else if install == "template_org_after_register" {
						template := storage.NewRuntimeTemplate(org)
						PrimeRuntimeTemplateSchema(&template)
						machine.PrimeMetadataSchema(&template.Org)
						org = template.CloneRuntimeOrg()
						machine.SetRuntimeTemplateOrg(&org)
					}
					if route == "istest" {
						machine.EnableTestContext()
					}
					for _, name := range []string{"AccountChangeEvent", "Parcel__ChangeEvent"} {
						state, ok := machine.Org.Objects[name]
						if !ok || len(state.Definition.Fields) == 0 {
							t.Fatalf("%s has no runtime fields", name)
						}
					}
				})
			}
		}
	}
}

func changeEventObservation(t *testing.T, machine *VM, api, source string) string {
	t.Helper()
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	var fault *RuntimeError
	if !errors.As(err, &fault) {
		t.Fatalf("missing observation: %v", err)
	}
	_, payload, ok := strings.Cut(fault.ExceptionMessage(), "OBSERVED|")
	if !ok {
		t.Fatalf("missing observation: %s", fault.ExceptionMessage())
	}
	return payload
}

// Native describe rows (change_event_describe, APIs 62.0 and 67.0): Account
// lists AccountChangeEvent.ParentId with a null relationship name and no
// custom-event lookup, and event objects have no child relationships.
func TestChangeEventDescribeChildRelationshipsAnonymousAndIsTestAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				org := testDataOrg()
				org.APIVersion = api
				storage.EnsureStandardObject(&org, "Account")
				storage.EnsureStandardObject(&org, "User")
				custom := storage.ObjectDefinition{APIName: "Parcel__c", Fields: map[string]storage.Field{
					"Name":    {APIName: "Name", Type: storage.FieldString},
					"Note__c": {APIName: "Note__c", Type: storage.FieldString, Length: 50},
					"Acct__c": {APIName: "Acct__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Acct__r", ChildRelationshipName: "Parcels__r"},
				}, Relations: []storage.Relationship{{Field: "Acct__c", ParentObjects: []string{"Account"}, ParentRelationship: "Acct__r", ChildRelationship: "Parcels__r"}}}
				storage.EnsureStandardObjectFields(&custom)
				org.Objects["Parcel__c"] = storage.ObjectState{Definition: custom}
				machine := New(nil)
				machine.SetOrg(&org)
				if route == "istest" {
					machine.EnableTestContext()
				}
				events := []string{"AccountChangeEvent", "Parcel__ChangeEvent"}
				for _, object := range events {
					if err := machine.RegisterTrigger(Trigger{Name: object + "Observer", Object: object, Timing: "after", Operation: "insert", APIVersion: api}); err != nil {
						t.Fatal(err)
					}
				}
				children := func(object string) []string {
					payload := changeEventObservation(t, machine, api, "List<String> children = new List<String>(); for (Schema.ChildRelationship child : Schema.getGlobalDescribe().get('"+object+"').getDescribe().getChildRelationships()) { children.add(String.valueOf(child.getChildSObject()) + '.' + child.getField() + ':' + child.getRelationshipName()); } System.assert(false, 'OBSERVED|' + JSON.serialize(children));")
					var out []string
					if err := json.Unmarshal([]byte(payload), &out); err != nil {
						t.Fatal(err)
					}
					return out
				}
				for _, event := range events {
					if got := children(event); len(got) != 0 {
						t.Errorf("%s child relationships = %v; native has none", event, got)
					}
				}
				var accountEvents []string
				for _, child := range children("Account") {
					object, _, _ := strings.Cut(child, ".")
					if object == "AccountChangeEvent" || object == "Parcel__ChangeEvent" || object == "Parcel__c" {
						accountEvents = append(accountEvents, child)
					}
				}
				sort.Strings(accountEvents)
				want := []string{"AccountChangeEvent.ParentId:null", "Parcel__c.Acct__c:Parcels__r"}
				if strings.Join(accountEvents, ",") != strings.Join(want, ",") {
					t.Errorf("Account children = %v; native %v", accountEvents, want)
				}
			})
		}
	}
}

// A template built after ChangeEvent triggers register already holds the
// event objects, so a template clone keeps the template schema stamp.
func TestChangeEventRestoredTemplateCloneKeepsSchemaStampAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				org := testDataOrg()
				org.APIVersion = api
				org.Objects["Parcel__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
					APIName: "Parcel__c", Fields: map[string]storage.Field{"Marker__c": {APIName: "Marker__c", Type: storage.FieldString}},
				}}
				if route == "istest" {
					// As in the project runner, finish test schema before freezing.
					setup := New(nil)
					setup.SetOrg(&org)
					setup.EnableTestContext()
				}
				base := New(nil)
				for _, name := range []string{"AccountChangeEvent", "Parcel__ChangeEvent"} {
					if err := base.RegisterTrigger(Trigger{Name: name + "Observer", Object: name, Timing: "after", Operation: "insert", APIVersion: api}); err != nil {
						t.Fatal(err)
					}
				}
				template := NewRestoredRuntimeTemplate(org, base)
				for i := 0; i < 2; i++ {
					machine := template.CloneMachine(nil)
					clone := template.CloneOrg()
					stamp := clone.RuntimeSchemaStamp
					machine.SetRuntimeTemplateOrg(&clone)
					if route == "istest" {
						machine.EnableTestContext()
					}
					if stamp == "" || machine.Org.RuntimeSchemaStamp != stamp {
						t.Fatalf("clone %d schema stamp %q became %q", i, stamp, machine.Org.RuntimeSchemaStamp)
					}
					for _, name := range []string{"AccountChangeEvent", "Parcel__ChangeEvent"} {
						if _, ok := machine.Org.Objects[name]; !ok {
							t.Fatalf("clone %d lacks %s", i, name)
						}
					}
				}
			})
		}
	}
}
