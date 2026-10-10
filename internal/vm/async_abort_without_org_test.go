package vm

import "testing"

// Local host guard: this does not assert real scheduler behavior outside tests.
func TestAbortQueuedScheduleWithoutOrg(t *testing.T) {
	for _, test := range []struct{ name, typeName, iface, enqueue string }{
		{"scheduled", "GuardSchedule", "Schedulable", "System.schedule('guard', '0 0 0 * * ?', new GuardSchedule())"},
		{"batch", "GuardBatch", "Database.Batchable<SObject>", "System.scheduleBatch(new GuardBatch(), 'guard', 1)"},
	} {
		t.Run(test.name, func(t *testing.T) {
			machine := New(nil)
			machine.EnableTestContext()
			if machine.Org != nil {
				t.Fatal("guard requires no org")
			}
			if err := machine.RegisterClass(Class{Name: test.typeName, Interfaces: []string{test.iface}}); err != nil {
				t.Fatal(err)
			}
			enqueue, err := CompileAnonymousWithOptions("String scheduledId = "+test.enqueue+";", CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(enqueue); err != nil {
				t.Fatal(err)
			}
			if len(machine.testContext.AsyncJobs) != 1 {
				t.Fatalf("queued jobs: %d", len(machine.testContext.AsyncJobs))
			}
			jobID := cronTriggerID(machine.testContext.AsyncJobs[0].ID)
			abort, err := CompileAnonymousWithOptions("System.abortJob('"+jobID+"');", CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(abort); err != nil {
				t.Fatal(err)
			}
			// These workers intentionally have no callbacks. Executing cancelled work
			// would fail method resolution; retained queue bookkeeping is not observable.
			stop, err := CompileAnonymousWithOptions("Test.startTest(); Test.stopTest();", CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(stop); err != nil {
				t.Fatal(err)
			}
			if machine.Org != nil {
				t.Fatal("abort must not invent an org")
			}
		})
	}
}
