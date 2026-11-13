package vm

import (
	"github.com/glade-sh/glade/internal/ir"
	"strings"
	"testing"
)

// Queues that no async_stop_order, async_stop_mix, async_stop_boundary or
// async_stop_shape row observes keep the origin/main order: submission order
// and single-pass batches. Every expected string except the two captured
// controls was produced by origin/main.
func TestStopOrderBarriersPreserveSubmissionOrder(t *testing.T) {
	const s, e = "Test.startTest(); ", " Test.stopTest();"
	const q, f, b, fault = "System.enqueueJob(new BarrierQ());", "BarrierFuture.go();", "Database.executeBatch(new BarrierBatch());", "Database.executeBatch(new BarrierBatch(), 1);"
	const sched = "System.schedule('job', '0 0 0 1 1 ? 2099', new BarrierSched());"
	for _, tc := range []struct{ name, source, want string }{
		// Barriers: work no row covers.
		{"fault batch", s + q + fault + f + e, "q,bs,bx,bf,f"},
		{"delayed queueable", s + "System.enqueueJob(new BarrierQ(), 1);" + f + e, "q,f"},
		{"scheduled batch between", s + q + "System.scheduleBatch(new BarrierBatch(), 'batch', 1);" + f + e, "q,f"},
		{"pre schedulable between", q + sched + s + f + e, "q,s,f"},
		{"pre schedulable same side", sched + s + q + f + e, "s,q,f"},
		{"scheduled batch same side", s + "System.scheduleBatch(new BarrierBatch(), 'batch', 1);" + q + f + e, "q,f"},
		// Uncaptured neighbors of captured queues (qqf, qfqf, bqf, bboom, qfs).
		{"three queueables then future", s + q + q + q + f + e, "q,q,q,f"},
		{"queueable future queueable queueable", s + q + f + q + q + e, "q,f,q,q"},
		{"two queueables two futures", s + q + q + f + f + e, "q,q,f,f"},
		{"five jobs", s + q + f + q + f + q + e, "q,f,q,f,q"},
		{"batch then three queueables", s + b + q + q + q + e, "bs,bx,bf,q,q,q"},
		{"two queueables batch future", s + q + q + b + f + e, "q,q,bs,bx,bf,f"},
		{"schedulable then queueable", s + sched + q + e, "s,q"},
		{"schedulable then future", s + sched + f + e, "s,f"},
		{"schedulable batch queueable", s + sched + b + q + e, "s,bs,bx,bf,q"},
		{"pre batch pre queueable", b + q + s + e, "bs,bx,bf,q"},
		{"pre future in-test queueable", f + s + q + e, "f,q"},
		// Captured queues sort and split (async_stop_order qqf, async_stop_mix
		// bxq); origin/main returns q,q,f and bs,bx,bf,q for these two.
		{"captured qqf", s + q + q + f + e, "f,q,q"},
		{"captured bq", s + b + q + e, "bs,q,bx,bf"},
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
