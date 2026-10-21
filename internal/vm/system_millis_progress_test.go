package vm

import (
	"testing"
	"time"
)

// This checks the local deterministic clock policy. Native rows for the exact
// millis progression remain pending; no exact per-read delta is asserted.
func TestExecSystemMillisProgressCompletesBoundedWait(t *testing.T) {
	for _, api := range []string{"54.0", "62.0", "67.0", "68.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(`
Datetime cached = Datetime.now();
Long start = System.currentTimeMillis();
Long elapsed = 0;
Integer iterations = 0;
while (elapsed < 10 && iterations < 100000) {
    elapsed = System.currentTimeMillis() - start;
    iterations++;
}
System.assert(elapsed >= 10);
System.assert(iterations < 100000);
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				if route == "isTest" {
					machine.EnableTestContext()
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestSystemMillisProgressObservesClockAdvance(t *testing.T) {
	machine := New(nil)
	before, err := CompileAnonymous("Datetime observed = Datetime.now();")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(before); err != nil {
		t.Fatal(err)
	}
	machine.AdvanceDeterministicTime(time.Second)
	advanced := machine.fakeNow.UnixMilli()
	program, err := CompileAnonymous("Long observedMillis = System.currentTimeMillis();")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if value := machine.Globals["observedMillis"]; value.Int < advanced {
		t.Fatalf("millis read %d masked clock advance to %d", value.Int, advanced)
	}
}
