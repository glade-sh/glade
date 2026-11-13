package vm

import (
	"github.com/glade-sh/glade/internal/ir"
	"strings"
	"testing"
)

// Order relative to a barrier equals origin/main; jobs on one side follow the captured sort.
func TestStopOrderBarriersPreserveSubmissionOrder(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"fault batch", `Test.startTest(); System.enqueueJob(new BarrierQ()); Database.executeBatch(new BarrierBatch(), 1); BarrierFuture.go(); Test.stopTest();`, "q,bs,bx,bf,f"},
		{"delayed queueable", `Test.startTest(); System.enqueueJob(new BarrierQ(), 1); BarrierFuture.go(); Test.stopTest();`, "q,f"},
		{"scheduled batch between", `Test.startTest(); System.enqueueJob(new BarrierQ()); System.scheduleBatch(new BarrierBatch(), 'batch', 1); BarrierFuture.go(); Test.stopTest();`, "q,f"},
		{"pre schedulable between", `System.enqueueJob(new BarrierQ()); System.schedule('pre', '0 0 0 1 1 ? 2099', new BarrierSched()); Test.startTest(); BarrierFuture.go(); Test.stopTest();`, "q,s,f"},
		// The same-side cases follow async_stop_order qqf.
		{"pre schedulable same side", `System.schedule('pre', '0 0 0 1 1 ? 2099', new BarrierSched()); Test.startTest(); System.enqueueJob(new BarrierQ()); BarrierFuture.go(); Test.stopTest();`, "s,f,q"},
		{"scheduled batch same side", `Test.startTest(); System.scheduleBatch(new BarrierBatch(), 'batch', 1); System.enqueueJob(new BarrierQ()); BarrierFuture.go(); Test.stopTest();`, "f,q"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			org := testDataOrg()
			org.APIVersion = "62.0"
			machine.SetOrg(&org)
			machine.EnableTestContext()
			compile := func(s string) ir.Program {
				t.Helper()
				p, err := CompileAnonymousWithOptions(s, CompileOptions{APIVersion: "62.0"})
				if err != nil {
					t.Fatal(err)
				}
				return p
			}
			classes := []Class{
				{Name: "BarrierQ", Interfaces: []string{"Queueable"}, Methods: map[string]Method{"execute": {Name: "BarrierQ.execute", ClassName: "BarrierQ", ReturnType: "void", Params: []Param{{Name: "ctx", Type: "QueueableContext"}}, Program: compile(`System.debug('q');`)}}},
				{Name: "BarrierFuture", Methods: map[string]Method{"go": {Name: "BarrierFuture.go", ClassName: "BarrierFuture", ReturnType: "void", IsStatic: true, Modifiers: []string{"future"}, Program: compile(`System.debug('f');`)}}},
				{Name: "BarrierSched", Interfaces: []string{"Schedulable"}, Methods: map[string]Method{"execute": {Name: "BarrierSched.execute", ClassName: "BarrierSched", ReturnType: "void", Params: []Param{{Name: "ctx", Type: "SchedulableContext"}}, Program: compile(`System.debug('s');`)}}},
				{Name: "BarrierBatch", APIVersion: "62.0", Interfaces: []string{"Database.Batchable<SObject>"}, Methods: map[string]Method{
					"start":   {Name: "BarrierBatch.start", ClassName: "BarrierBatch", ReturnType: "Iterable<SObject>", Params: []Param{{Name: "ctx", Type: "Database.BatchableContext"}}, Program: compile(`System.debug('bs'); return new List<SObject>{new Account(Name='scope')};`)},
					"execute": {Name: "BarrierBatch.execute", ClassName: "BarrierBatch", ReturnType: "void", Params: []Param{{Name: "ctx", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}, Program: compile(`System.debug('bx');`)},
					"finish":  {Name: "BarrierBatch.finish", ClassName: "BarrierBatch", ReturnType: "void", Params: []Param{{Name: "ctx", Type: "Database.BatchableContext"}}, Program: compile(`System.debug('bf');`)},
				}},
			}
			for _, c := range classes {
				if err := machine.RegisterClass(c); err != nil {
					t.Fatal(err)
				}
			}
			result, err := machine.Execute(compile(tc.source))
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Join(result.Debug, ",")
			if got != tc.want {
				t.Fatalf("order=%s want=%s", got, tc.want)
			}
		})
	}
}
