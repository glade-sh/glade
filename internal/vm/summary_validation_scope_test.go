package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// summaryValidationScopeOrg has an unfiltered COUNT roll-up from Line__c to
// Account. The Account rule rejects any positive count.
func summaryValidationScopeOrg(grandparent bool) storage.OrgState {
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["LineCount__c"] = storage.Field{
		APIName: "LineCount__c", Type: storage.FieldSummary, DisplayType: "INTEGER",
		SummaryForeignKey: "Line__c.Account__c", SummaryOperation: "count",
	}
	account.Definition.ValidationRules = []storage.ValidationRule{{
		Name: "RejectLines", Active: true, ErrorConditionFormula: "LineCount__c > 0", ErrorMessage: "no lines",
	}}
	org.Objects["Account"] = account
	org.Objects["Line__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Line__c", KeyPrefix: "a05",
			Fields: map[string]storage.Field{
				"Account__c": {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account__r"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	return org
}

func runSummaryValidationScope(t *testing.T, org storage.OrgState, afterTrigger, script string) {
	t.Helper()
	machine := New(nil)
	machine.SetOrg(&org)
	if afterTrigger != "" {
		program, err := CompileAnonymous(afterTrigger)
		if err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterTrigger(Trigger{Name: "AccountAfterUpdate", Object: "Account", Timing: triggerTimingAfter, Operation: "update", Program: program}); err != nil {
			t.Fatal(err)
		}
	}
	program, err := CompileAnonymous(script)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

// The captured shape rejects a child whose parent validation fails.
func TestSummaryValidationRejectsDirectChildInserts(t *testing.T) {
	runSummaryValidationScope(t, summaryValidationScopeOrg(false), "", `
Account parent = new Account(Name = 'A');
insert parent;
Database.SaveResult r = Database.insert(new Line__c(Account__c = parent.Id), false);
System.assertEquals(false, r.isSuccess());
System.assertEquals(0, [SELECT COUNT() FROM Line__c]);
`)
}

// Inputs beyond the 200-row boundary were not captured and keep the earlier
// behavior, which accepts them.
func TestSummaryValidationLeavesOver200InputsOnEarlierPath(t *testing.T) {
	runSummaryValidationScope(t, summaryValidationScopeOrg(false), "", `
Account parent = new Account(Name = 'A');
insert parent;
List<Line__c> lines = new List<Line__c>();
for (Integer i = 0; i < 201; i++) { lines.add(new Line__c(Account__c = parent.Id)); }
List<Database.SaveResult> rs = Database.insert(lines, false);
for (Database.SaveResult r : rs) { System.assertEquals(true, r.isSuccess()); }
System.assertEquals(201, [SELECT COUNT() FROM Line__c]);
`)
}

// A parent after-update trigger row error was not captured. The earlier path
// ignores it, so it must not reject an insert whose validation never fires.
func TestSummaryValidationIgnoresParentAfterTriggerRowErrors(t *testing.T) {
	org := summaryValidationScopeOrg(false)
	account := org.Objects["Account"]
	account.Definition.ValidationRules[0].ErrorConditionFormula = "Name = 'never'"
	org.Objects["Account"] = account
	runSummaryValidationScope(t, org, `
for (Account a : Trigger.new) { a.addError('after'); }
`, `
Account parent = new Account(Name = 'A');
insert parent;
Database.SaveResult r = Database.insert(new Line__c(Account__c = parent.Id), false);
System.assertEquals(true, r.isSuccess());
System.assertEquals(1, [SELECT COUNT() FROM Line__c]);
`)
}

// A parent that is itself rolled up is a multi-level chain with no native
// evidence, so it keeps the earlier path.
func TestSummaryValidationLeavesMultiLevelRollupsOnEarlierPath(t *testing.T) {
	org := summaryValidationScopeOrg(true)
	org.Objects["Region__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Region__c", KeyPrefix: "a06",
			Fields: map[string]storage.Field{
				"AccountCount__c": {APIName: "AccountCount__c", Type: storage.FieldSummary, DisplayType: "INTEGER", SummaryForeignKey: "Account.Region__c", SummaryOperation: "count"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	account := org.Objects["Account"]
	account.Definition.Fields["Region__c"] = storage.Field{APIName: "Region__c", Type: storage.FieldReference, ReferenceTo: []string{"Region__c"}, RelationshipName: "Region__r"}
	org.Objects["Account"] = account
	runSummaryValidationScope(t, org, "", `
Region__c region = new Region__c();
insert region;
Account parent = new Account(Name = 'A', Region__c = region.Id);
insert parent;
Database.SaveResult r = Database.insert(new Line__c(Account__c = parent.Id), false);
System.assertEquals(true, r.isSuccess());
`)
}

// Trigger chunking must not bring an unqualified bulk operation into the
// parent-validation path. The 200-row control remains within captured scope.
func TestSummaryValidationTriggeredBulkScope(t *testing.T) {
 for _, tc := range []struct{ size int; success bool }{{200, false}, {201, true}} {
  t.Run(fmt.Sprint(tc.size), func(t *testing.T) {
   org := summaryValidationScopeOrg(false)
   machine := New(nil)
   machine.SetOrg(&org)
   trigger, err := CompileAnonymous(`Integer n = Trigger.new.size();`)
   if err != nil { t.Fatal(err) }
   if err := machine.RegisterTrigger(Trigger{Name: "LineBeforeInsert", Object: "Line__c", Timing: triggerTimingBefore, Operation: "insert", Program: trigger}); err != nil { t.Fatal(err) }
   source := fmt.Sprintf(`
Account parent = new Account(Name = 'A'); insert parent;
List<Line__c> lines = new List<Line__c>();
for (Integer i = 0; i < %d; i++) { lines.add(new Line__c(Account__c = parent.Id)); }
List<Database.SaveResult> rs = Database.insert(lines, false);
for (Database.SaveResult r : rs) { System.assertEquals(%t, r.isSuccess()); }
`, tc.size, tc.success)
   program, err := CompileAnonymous(source)
   if err != nil { t.Fatal(err) }
   if _, err := machine.Execute(program); err != nil { t.Fatal(err) }
  })
 }
}
