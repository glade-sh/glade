package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These seven boundaries were observed in owned named source-62 tests over
// transport 67. Source-67 access failures establish no timing expectation.
func TestMultiSelectDMLTimingSource62(t *testing.T) {
	for _, tc := range []struct {
		name, supplied, before, mutation, after, caller, text string
		update                                                bool
	}{
		{name: "suppliedInsert", supplied: "';'", before: "null", after: "null", caller: "';'", text: "null"},
		{name: "suppliedUpdate", supplied: "';;'", before: "null", after: "null", caller: "';;'", text: "null", update: true},
		{name: "producedInsert", supplied: "'Alpha'", before: "'Alpha'", mutation: "';'", after: "null", caller: "'Alpha'", text: "null"},
		{name: "producedUpdate", supplied: "'Alpha'", before: "'Alpha'", mutation: "';;'", after: "null", caller: "'Alpha'", text: "null", update: true},
		{name: "producedValidInsert", supplied: "'Alpha'", before: "'Alpha'", mutation: "'Beta'", after: "'Beta'", caller: "'Alpha'", text: "null"},
		{name: "textFenceInsert", supplied: "'Alpha'", before: "'Alpha'", after: "'Alpha'", caller: "'Alpha'", text: "';'"},
		{name: "unknownInsert", supplied: "'NotDefined'", before: "'NotDefined'", after: "'NotDefined'", caller: "'NotDefined'", text: "null"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testDataOrg()
			org.APIVersion = "67.0"
			account := org.Objects["Account"]
			account.Definition.Fields["OwnedMultiTimingSelection__c"] = storage.Field{
				APIName: "OwnedMultiTimingSelection__c", Type: storage.FieldMultiPicklist,
				PicklistValues: []storage.PicklistValue{{Value: "Alpha"}, {Value: "Beta"}, {Value: "Gamma"}},
			}
			org.Objects["Account"] = account
			machine := New(nil)
			machine.SetOrg(&org)

			beforeReturn := tc.before
			mutation := ""
			if tc.mutation != "" {
				beforeReturn = tc.mutation
				mutation = "item.OwnedMultiTimingSelection__c = " + tc.mutation + ";"
			}
			beforeSource := fmt.Sprintf(`for (Account item : Trigger.new) {
 if (item.Name == 'observed') {
  System.assertEquals(%s, item.OwnedMultiTimingSelection__c, 'before input');
  System.assertEquals(%s, item.Description, 'before text');
  %s
  System.assertEquals(%s, item.OwnedMultiTimingSelection__c, 'before return');
 }
}`, tc.before, tc.text, mutation, beforeReturn)
			afterSource := fmt.Sprintf(`for (Account item : Trigger.new) {
 if (item.Name == 'observed') {
  System.assertEquals(%s, item.OwnedMultiTimingSelection__c, 'after trigger');
  System.assertEquals(%s, item.Description, 'after text');
 }
}`, tc.after, tc.text)
			operation := "insert"
			seed := ""
			if tc.update {
				operation = "update"
				seed = "item.Name = 'seed'; insert item; item.Name = 'observed';"
			}
			for _, trigger := range []struct{ timing, source string }{{triggerTimingBefore, beforeSource}, {triggerTimingAfter, afterSource}} {
				program, err := CompileAnonymous(trigger.source)
				if err != nil {
					t.Fatal(err)
				}
				program.APIVersion = "62.0"
				if err := machine.RegisterTrigger(Trigger{Name: "MultiTiming" + trigger.timing, Object: "Account", Timing: trigger.timing, Operation: operation, APIVersion: "62.0", Program: program}); err != nil {
					t.Fatal(err)
				}
			}
			program, err := CompileAnonymous(fmt.Sprintf(`
Account item = new Account(Name='observed', OwnedMultiTimingSelection__c='Alpha');
%s
item.OwnedMultiTimingSelection__c = %s;
item.Description = %s;
%s item;
System.assertEquals(%s, item.OwnedMultiTimingSelection__c, 'caller unchanged');
System.assertEquals(%s, item.Description, 'caller text');
Account saved = [SELECT OwnedMultiTimingSelection__c, Description FROM Account WHERE Id=:item.Id];
System.assertEquals(%s, saved.OwnedMultiTimingSelection__c, 'stored');
System.assertEquals(%s, saved.Description, 'stored text');
`, seed, tc.supplied, tc.text, operation, tc.caller, tc.text, tc.after, tc.text))
			if err != nil {
				t.Fatal(err)
			}
			program.APIVersion = "62.0"
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
