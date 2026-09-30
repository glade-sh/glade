package vm

import (
	"errors"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// This local capture contract is separate from Salesforce notification delivery.
// Required Subject is a deliberate local validation control, not a Task schema claim.
func taskNotificationTestOrg() storage.OrgState {
	org := testDataOrg()
	org.Objects["User"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "User", KeyPrefix: "005", Fields: map[string]storage.Field{}},
		Records:    map[storage.ID]storage.Record{"005000000000001": {ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{}}},
	}
	org.Objects["Task"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Task", KeyPrefix: "00T", Fields: map[string]storage.Field{
			"Subject": {APIName: "Subject", Type: storage.FieldString, Required: true},
			"OwnerId": {APIName: "OwnerId", Type: storage.FieldReference, ReferenceTo: []string{"User"}},
		}},
		Records: map[storage.ID]storage.Record{},
	}
	return org
}

func runTaskNotificationProgram(t *testing.T, machine *VM, source string) (Result, error) {
	t.Helper()
	program, err := CompileAnonymous(source)
	if err != nil {
		t.Fatal(err)
	}
	return machine.Execute(program)
}

func TestExecTaskAssignmentNotificationRequests(t *testing.T) {
	org := taskNotificationTestOrg()
	machine := New(nil)
	machine.SetOrg(&org)
	result, err := runTaskNotificationProgram(t, machine, `
Database.DMLOptions options = new Database.DMLOptions();
options.OptAllOrNone = true;
options.EmailHeader.triggerUserEmail = true;
Task notified = new Task(Subject='first', OwnerId='005000000000001');
System.assert(Database.insert(notified, options).isSuccess());
notified.Subject = 'updated';
System.assert(Database.update(notified, options).isSuccess());
System.assertEquals(true, options.EmailHeader.triggerUserEmail);
options.EmailHeader.triggerUserEmail = false;
Task quiet = new Task(Subject='quiet', OwnerId='005000000000001');
System.assert(Database.insert(quiet, options).isSuccess());
quiet.Subject = 'quiet updated';
System.assert(Database.update(quiet, options).isSuccess());
System.assertEquals(0, Limits.getEmailInvocations());
System.assertEquals(2, [SELECT count() FROM Task]);
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.CapturedEmails) != 2 {
		t.Fatalf("requests = %#v", result.CapturedEmails)
	}
	for _, request := range result.CapturedEmails {
		row, ok := machine.findOrgRecord("Task", storage.ID(request.WhatID))
		if !ok || row.Fields["Subject"].String != "updated" || request.WhatID != string(row.ID) || request.TargetObjectID != string(row.System.OwnerID) || request.TargetObjectID == "" {
			t.Fatalf("request must bind the successfully stored Task and owner: %#v / %#v", request, row)
		}
		if request.Kind != "TaskAssignmentNotificationRequest" || !request.TriggerUserEmail || request.Subject != "" || request.PlainTextBody != "" || request.HTMLBody != "" || request.TemplateID != "" || len(request.ToAddresses) != 0 {
			t.Fatalf("request fabricated delivery or content: %#v", request)
		}
	}
}

func TestExecTaskAssignmentNotificationFailuresAndSavepoint(t *testing.T) {
	org := taskNotificationTestOrg()
	machine := New(nil)
	machine.SetOrg(&org)
	result, err := runTaskNotificationProgram(t, machine, `
Database.DMLOptions options = new Database.DMLOptions();
options.EmailHeader.triggerUserEmail = true;
options.OptAllOrNone = false;
List<Database.SaveResult> partial = Database.insert(new List<Task>{
 new Task(OwnerId='005000000000001'), new Task(Subject='kept', OwnerId='005000000000001')
}, options);
System.assertEquals(false, partial[0].isSuccess());
System.assertEquals(true, partial[1].isSuccess());
System.Savepoint sp = Database.setSavepoint();
System.assert(Database.insert(new Task(Subject='rolled back', OwnerId='005000000000001'), options).isSuccess());
Database.rollback(sp);
options.OptAllOrNone = true;
Boolean caught = false;
try {
 Database.insert(new List<Task>{new Task(Subject='all or none', OwnerId='005000000000001'), new Task(OwnerId='005000000000001')}, options);
} catch (DmlException ex) { caught = true; }
System.assert(caught);
System.assertEquals(1, [SELECT count() FROM Task]);
System.assertEquals(0, Limits.getEmailInvocations());
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.CapturedEmails) != 1 {
		t.Fatalf("failed/rolled back rows left requests: %#v", result.CapturedEmails)
	}
	row, ok := machine.findOrgRecord("Task", storage.ID(result.CapturedEmails[0].WhatID))
	if !ok || row.Fields["Subject"].String != "kept" {
		t.Fatalf("wrong surviving request: %#v", result.CapturedEmails)
	}
}

func TestExecTaskAssignmentNotificationNestedDMLRollback(t *testing.T) {
	org := taskNotificationTestOrg()
	machine := New(nil)
	machine.SetOrg(&org)
	trigger, err := CompileAnonymous(`
Database.DMLOptions options = new Database.DMLOptions();
options.EmailHeader.triggerUserEmail = true;
System.assert(Database.insert(new Task(Subject='nested', OwnerId='005000000000001'), options).isSuccess());
for (Account row : Trigger.new) { row.addError('reject outer row'); }
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterTrigger(Trigger{Name: "RejectAccountAfterTask", Object: "Account", Timing: triggerTimingAfter, Operation: "insert", Program: trigger}); err != nil {
		t.Fatal(err)
	}
	result, err := runTaskNotificationProgram(t, machine, `
Boolean caught = false;
try { insert new Account(Name='outer'); } catch (DmlException ex) { caught = true; }
System.assert(caught);
System.assertEquals(0, [SELECT count() FROM Account]);
System.assertEquals(0, [SELECT count() FROM Task]);
System.assertEquals(0, Limits.getEmailInvocations());
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.CapturedEmails) != 0 {
		t.Fatalf("nested rollback left requests: %#v", result.CapturedEmails)
	}
}

func TestExecTaskAssignmentNotificationUnsupportedBoundaries(t *testing.T) {
	for name, source := range map[string]string{
		"non-task":   `Database.DMLOptions o=new Database.DMLOptions(); o.EmailHeader.triggerUserEmail=true; Database.insert(new Account(Name='not Task'),o);`,
		"other-flag": `Database.DMLOptions o=new Database.DMLOptions(); o.EmailHeader.triggerOtherEmail=false; Database.insert(new Task(Subject='not proved'),o);`,
		"per-record": `Database.DMLOptions o=new Database.DMLOptions(); o.EmailHeader.triggerUserEmail=true; Task row=new Task(Subject='not proved'); row.setOptions(o); insert row;`,
	} {
		t.Run(name, func(t *testing.T) {
			org := taskNotificationTestOrg()
			machine := New(nil)
			machine.SetOrg(&org)
			result, err := runTaskNotificationProgram(t, machine, source)
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" {
				t.Fatalf("expected unsupported boundary, got %v", err)
			}
			if len(result.CapturedEmails) != 0 || len(org.Objects["Task"].Records) != 0 || len(org.Objects["Account"].Records) != 0 {
				t.Fatalf("unsupported operation changed state: %#v", result)
			}
		})
	}
}
