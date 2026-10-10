package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecScheduledBatchCronTriggerQueryAcceptsTypedIDAfterStopTest(t *testing.T) {
	scheduledProgram, err := CompileAnonymous("Database.executeBatch(new TypedScheduleBatchWorker(), 1);")
	if err != nil {
		t.Fatal(err)
	}
	batchStartProgram, err := CompileAnonymous("return new List<SObject>();")
	if err != nil {
		t.Fatal(err)
	}
	voidProgram, err := CompileAnonymous("")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
Id scheduleId = System.schedule('typed-id-schedule', '0 0 3 * * ?', new TypedScheduleBatchWorker());
Test.stopTest();
CronTrigger cron = [SELECT Id, CronExpression FROM CronTrigger WHERE Id = :scheduleId];
System.assertNotEquals(null, cron);
System.assertEquals('0 0 3 * * ?', cron.CronExpression);
`)
	if err != nil {
		t.Fatal(err)
	}

	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "TypedScheduleBatchWorker",
		Interfaces: []string{"Schedulable", "Database.Batchable<SObject>"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name:      "TypedScheduleBatchWorker.execute",
		ClassName: "TypedScheduleBatchWorker",
		Params:    []Param{{Name: "context", Type: "SchedulableContext"}},
		Program:   scheduledProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name:       "TypedScheduleBatchWorker.start",
		ClassName:  "TypedScheduleBatchWorker",
		ReturnType: "Iterable<SObject>",
		Params:     []Param{{Name: "context", Type: "Database.BatchableContext"}},
		Program:    batchStartProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name:      "TypedScheduleBatchWorker.execute",
		ClassName: "TypedScheduleBatchWorker",
		Params: []Param{
			{Name: "context", Type: "Database.BatchableContext"},
			{Name: "scope", Type: "List<SObject>"},
		},
		Program: voidProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name:      "TypedScheduleBatchWorker.finish",
		ClassName: "TypedScheduleBatchWorker",
		Params:    []Param{{Name: "context", Type: "Database.BatchableContext"}},
		Program:   voidProgram,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
