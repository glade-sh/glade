package vm

import (
	"errors"
	"fmt"
	"github.com/glade-sh/glade/internal/ir"
	"os"
	"strings"
	"testing"
)

// NATIVE testdata/test_stop_future_caps/org.tsv rows 2–7 capture the anonymous
// uncatchable boundary and the future callback's default limits at API 62/67.
func TestStopAndFutureLimitsNative(t *testing.T) {
	data, err := os.ReadFile("testdata/test_stop_future_caps/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 7 || lines[0] != "api\troute\tcase\tobservation" {
		t.Fatal("invalid native table")
	}
	for _, line := range lines[1:] {
		row := strings.Split(line, "\t")
		if len(row) != 4 {
			t.Fatalf("invalid row %q", line)
		}
		t.Run(row[0]+"/"+row[2], func(t *testing.T) {
			machine := New(nil)
			if row[2] == "start" || row[2] == "stop" {
				if row[1] != "anonymous" || row[3] != "UNCAUGHT|System.FinalException|Method only allowed during testing" {
					t.Fatalf("unexpected row %v", row)
				}
				source := fmt.Sprintf(`try { Test.%sTest(); } catch (Exception e) { System.debug('caught'); } System.debug('continued');`, row[2])
				p, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[0]})
				if err != nil {
					t.Fatal(err)
				}
				result, err := machine.Execute(p)
				var failure *RuntimeError
				if !errors.As(err, &failure) || failure.Type != "System.FinalException" || failure.Message != "Method only allowed during testing" || len(result.Debug) != 0 {
					t.Fatalf("result=%+v error=%v", result, err)
				}
				return
			}
			if row[1] != "istest" || row[2] != "future" || row[3] != "queue=0/1,future=0/50,dml=0/150" {
				t.Fatalf("unexpected row %v", row)
			}
			callback, err := CompileAnonymousWithOptions(`System.assertEquals(0,Limits.getQueueableJobs()); System.assertEquals(1,Limits.getLimitQueueableJobs()); System.assertEquals(0,Limits.getFutureCalls()); System.assertEquals(50,Limits.getLimitFutureCalls()); System.assertEquals(0,Limits.getDmlStatements()); System.assertEquals(150,Limits.getLimitDmlStatements()); System.debug('observed');`, CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			class := Class{Name: "FutureCapProbe", Methods: map[string]Method{"run": {Name: "FutureCapProbe.run", ClassName: "FutureCapProbe", ReturnType: "void", IsStatic: true, Modifiers: []string{"future"}, Program: callback}}}
			org := testDataOrg()
			machine.SetOrg(&org)
			machine.EnableTestContext()
			if err := machine.RegisterClass(class); err != nil {
				t.Fatal(err)
			}
			p, err := CompileAnonymousWithOptions(`Test.startTest(); FutureCapProbe.run(); Test.stopTest();`, CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(p)
			if err != nil || len(result.Debug) != 1 {
				t.Fatalf("result=%+v error=%v", result, err)
			}
		})
	}
}

func TestFutureQueueableGetterPreservesExplicitCap(t *testing.T) {
	machine := New(nil)
	caps := defaultLimitCaps()
	caps.QueueableJobs = 7
	machine.SetLimitCaps(caps)
	machine.currentAsyncKind = "Future"
	machine.EnableTestContext()
	machine.testContext.StopDraining = true
	got, ok := machine.limitValue("getLimitQueueableJobs")
	if !ok || got.Int != 7 {
		t.Fatalf("explicit getter = %v, %t", got, ok)
	}
}

// NATIVE testdata/test_stop_future_caps/queueable.tsv rows 2–7 capture
// post-stop Queueable status for a successful callback and two failure causes.
func TestQueueableStopStatusNative(t *testing.T) {
	data, err := os.ReadFile("testdata/test_stop_future_caps/queueable.tsv")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 7 || lines[0] != "api\troute\tcase\tstop\tafter\tbody" {
		t.Fatal("invalid native table")
	}
	for _, line := range lines[1:] {
		row := strings.Split(line, "\t")
		if len(row) != 6 || row[1] != "istest" {
			t.Fatalf("invalid row %q", line)
		}
		t.Run(row[0]+"/"+row[2], func(t *testing.T) {
			suffix := ""
			switch row[2] {
			case "leaf":
			case "failure":
				suffix = "throw new IllegalArgumentException('failure');"
			case "child":
				suffix = "System.enqueueJob(new StopQueueProbe());"
			default:
				t.Fatalf("invalid case %q", row[2])
			}
			body, err := CompileAnonymousWithOptions(`insert new Account(Name='owned-marker');`+suffix, CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			class := Class{Name: "StopQueueProbe", Interfaces: []string{"Queueable"}, Methods: map[string]Method{"execute": {Name: "StopQueueProbe.execute", ClassName: "StopQueueProbe", ReturnType: "void", Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: body}}}
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			machine.EnableTestContext()
			if err := machine.RegisterClass(class); err != nil {
				t.Fatal(err)
			}
			expectedStop := row[3] == "caught"
			fields := strings.Split(row[4], ":")
			if len(fields) != 5 {
				t.Fatalf("invalid status %q", row[4])
			}
			source := fmt.Sprintf(`Test.startTest(); Id id=System.enqueueJob(new StopQueueProbe()); Boolean caught=false; try { Test.stopTest(); } catch(Exception e) { caught=true; } System.assertEquals(%t,caught); AsyncApexJob j=[SELECT Status,JobType,NumberOfErrors,JobItemsProcessed,TotalJobItems FROM AsyncApexJob WHERE Id=:id]; System.assertEquals('%s',j.Status); System.assertEquals('%s',j.JobType); System.assertEquals(%s,j.NumberOfErrors); System.assertEquals(%s,j.JobItemsProcessed); System.assertEquals(%s,j.TotalJobItems); System.assertEquals(%s,[SELECT COUNT() FROM Account WHERE Name='owned-marker']);`, expectedStop, fields[0], fields[1], fields[2], fields[3], fields[4], row[5])
			p, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(p); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// This local contract keeps non-test and isolated-drain projections unchanged.
func TestQueueableFailureNonTestDrainRemainsFailed(t *testing.T) {
	for _, isolated := range []bool{false, true} {
		t.Run(fmt.Sprintf("isolated=%t", isolated), func(t *testing.T) {
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			machine.isolateAsyncJobFailures = isolated
			body, err := CompileAnonymous(`throw new IllegalArgumentException('failure');`)
			if err != nil {
				t.Fatal(err)
			}
			class := Class{Name: "LiveQueueProbe", Interfaces: []string{"Queueable"}, Methods: map[string]Method{"execute": {Name: "LiveQueueProbe.execute", ClassName: "LiveQueueProbe", ReturnType: "void", Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: body}}}
			if err := machine.RegisterClass(class); err != nil {
				t.Fatal(err)
			}
			p, err := CompileAnonymous(`System.enqueueJob(new LiveQueueProbe());`)
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(p)
			if err != nil {
				t.Fatal(err)
			}
			err = machine.DrainAsync(&result)
			if (!isolated && err == nil) || (isolated && err != nil) {
				t.Fatalf("drain error=%v", err)
			}
			state, ok := machine.Org.Objects["AsyncApexJob"]
			if !ok || len(state.Records) != 1 {
				t.Fatalf("job state=%+v", state)
			}
			for _, job := range state.Records {
				status, _ := job.GetField("Status")
				count, _ := job.GetField("NumberOfErrors")
				if status.String != "Failed" || count.Integer != 1 {
					t.Fatalf("status=%v errors=%v", status, count)
				}
			}
		})
	}
}

// Outside the captured no-finalizer body failure, post-stop status keeps the
// Failed projection. Neither route has a native post-stop row.
func TestQueueableStopStatusOutsideCapturedScopeRemainsFailed(t *testing.T) {
	for _, route := range []string{"finalizer", "nobody"} {
		t.Run(route, func(t *testing.T) {
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			machine.EnableTestContext()
			qParams := []Param{{Name: "context", Type: "QueueableContext"}}
			fParams := []Param{{Name: "context", Type: "FinalizerContext"}}
			mk := func(src string) ir.Program {
				p, err := CompileAnonymous(src)
				if err != nil {
					t.Fatal(err)
				}
				return p
			}
			classes := []Class{{Name: "StopFinProbe", Interfaces: []string{"Finalizer"}, Methods: map[string]Method{"execute": {Name: "StopFinProbe.execute", ClassName: "StopFinProbe", ReturnType: "void", Params: fParams, Program: mk(`Integer x = 1;`)}}}}
			switch route {
			case "finalizer":
				classes = append(classes, Class{Name: "StopQProbe", Interfaces: []string{"Queueable"}, Methods: map[string]Method{"execute": {Name: "StopQProbe.execute", ClassName: "StopQProbe", ReturnType: "void", Params: qParams, Program: mk(`System.attachFinalizer(new StopFinProbe()); throw new IllegalArgumentException('failure');`)}}})
			case "nobody":
				// Queueable without an execute method fails before any body runs.
				classes = append(classes, Class{Name: "StopQProbe", Interfaces: []string{"Queueable"}, Methods: map[string]Method{}})
			}
			for _, c := range classes {
				if err := machine.RegisterClass(c); err != nil {
					t.Fatal(err)
				}
			}
			check := "AsyncApexJob j=[SELECT Status,NumberOfErrors FROM AsyncApexJob LIMIT 1]; System.assertEquals('Failed',j.Status); System.assertEquals(1,j.NumberOfErrors);"
			if route == "finalizer" {
				p := mk(`Test.startTest(); System.enqueueJob(new StopQProbe()); Boolean caught=false; try { Test.stopTest(); } catch(Exception e) { caught=true; } System.assertEquals(true,caught); ` + check)
				if _, err := machine.Execute(p); err != nil {
					t.Fatal(err)
				}
				return
			}
			// The missing-method error is not catchable Apex; it escapes stopTest.
			if _, err := machine.Execute(mk(`Test.startTest(); System.enqueueJob(new StopQProbe()); Test.stopTest();`)); err == nil {
				t.Fatal("expected stopTest error")
			}
			if _, err := machine.Execute(mk(check)); err != nil {
				t.Fatal(err)
			}
		})
	}
}
