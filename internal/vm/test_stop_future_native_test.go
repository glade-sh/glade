package vm

import (
	"errors"
	"fmt"
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
	got, ok := machine.limitValue("getLimitQueueableJobs")
	if !ok || got.Int != 7 {
		t.Fatalf("explicit getter = %v, %t", got, ok)
	}
}
