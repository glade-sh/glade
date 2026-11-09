package vm

import "testing"

// A wait loop that spins on the clock for five seconds (what remains once its
// own aggregate queries hit the cap) must end inside the loop-iteration guard.
func TestClockBusyWaitFiveSecondsEndsInsideLoopGuard(t *testing.T) {
	program, err := CompileAnonymous(`
Long started=System.currentTimeMillis();
Long endTime=started+5000;
for(;System.currentTimeMillis()<endTime;){ }
System.assert(System.currentTimeMillis()-started>=5000);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
