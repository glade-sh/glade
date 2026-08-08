package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestDataRuntimeSObjectAddErrorAcceptsSObjectFieldTokenAlias(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)

	record := Object("Account")
	token := sObjectFieldToken("Account", "Name")
	token.Type = "SObjectField"
	if _, handled, err := machine.callSObjectMember(record, "addError", []Value{token, String("blocked")}); err != nil || !handled {
		t.Fatalf("callSObjectMember() = handled %v, err %v; want handled", handled, err)
	}

	errors := sobjectErrors(record)
	if len(errors) != 1 {
		t.Fatalf("error count = %d, want 1", len(errors))
	}
	fields := errors[0].Fields["fields"]
	if fields.Kind != ValueList || len(fields.List) != 1 || fields.List[0].Text != "Name" {
		t.Fatalf("error fields = %#v, want one Name token", fields)
	}
}

func TestDataRuntimeSObjectAddErrorTokenRetainsRepeatedFieldErrors(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)

	record := Object("Account")
	token := sObjectFieldToken("Account", "Name")
	if _, handled, err := machine.callSObjectMember(record, "addError", []Value{token, String("first")}); err != nil || !handled {
		t.Fatalf("first addError() = handled %v, err %v; want handled", handled, err)
	}
	if _, handled, err := machine.callSObjectMember(record, "addError", []Value{token, String("second"), Bool(false)}); err != nil || !handled {
		t.Fatalf("second addError() = handled %v, err %v; want handled", handled, err)
	}

	errors := sobjectErrors(record)
	if len(errors) != 2 {
		t.Fatalf("error count = %d, want 2", len(errors))
	}
	if got := errors[0].Fields["message"].Text; got != "first" {
		t.Fatalf("first error message = %q, want first", got)
	}
	if got := errors[1].Fields["message"].Text; got != "second" {
		t.Fatalf("second error message = %q, want second", got)
	}
}

func TestDataRuntimeSObjectAddErrorStringOverloadsReplaceSalesforceErrors(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)

	record := Object("Account")
	args := [][]Value{
		{String("row")},
		{String("row 2"), Bool(false)},
		{String("Name"), String("name")},
		{String("Name"), String("name 2"), Bool(false)},
	}
	for _, call := range args {
		if _, handled, err := machine.callSObjectMember(record, "addError", call); err != nil || !handled {
			t.Fatalf("addError(%#v) = handled %v, err %v; want handled", call, handled, err)
		}
	}
	if errors := sobjectErrors(record); len(errors) != 2 {
		t.Fatalf("error count = %d, want 2", len(errors))
	}
}

func TestDataRuntimeListCustomSettingShellAccessorsUseNamedRecord(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,Fixture_Setting__c> allSettings = Fixture_Setting__c.getAll();
System.assertEquals(1, allSettings.size());
Fixture_Setting__c setting = Fixture_Setting__c.getInstance('Default');
System.assertEquals(true, setting.Enabled__c);
`)
	if err != nil {
		t.Fatal(err)
	}

	machine := New(nil)
	org := testDataOrg()
	org.Objects["Fixture_Setting__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Fixture_Setting__c",
			Fields: map[string]storage.Field{
				"Name":       {APIName: "Name", Type: storage.FieldString},
				"Enabled__c": {APIName: "Enabled__c", Type: storage.FieldBoolean},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {
				ID:     "a01000000000001",
				Object: "Fixture_Setting__c",
				Fields: map[string]storage.Value{
					"Name":       storage.StringValue("Default"),
					"Enabled__c": storage.BooleanValue(true),
				},
			},
		},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestDataRuntimeCustomSettingAccessorsWithoutOrgKeepTypedShapes(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,Fixture_Setting__c> allSettings = Fixture_Setting__c.getAll();
System.assertEquals(0, allSettings.size());
Fixture_Setting__c setting = Fixture_Setting__c.getInstance('Missing');
System.assertEquals(null, setting);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}
