package vm

import "testing"

// The Apex Developer Guide's cache-performance example measures elapsed time
// with successive currentTimeMillis calls. PENDING-ROWS: this checks progress
// of the local deterministic clock, not native per-call timing granularity.
func TestMillisecondClockProgressesInBoundedLoop(t *testing.T) {
	program, err := CompileAnonymous(`Long started=System.currentTimeMillis(); Long elapsed=0; Integer reads=0; while(elapsed<20 && reads<1000){elapsed=System.currentTimeMillis()-started;reads++;} System.assert(elapsed>=20);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestMillisecondClockProgressesAfterNowRead(t *testing.T) {
	program, err := CompileAnonymous(`Datetime sample=System.now(); Long started=System.currentTimeMillis(); System.assertEquals(sample.getTime(),started); Long elapsed=0;Integer reads=0;while(elapsed<20 && reads<1000){elapsed=System.currentTimeMillis()-started;reads++;}System.assert(elapsed>=20);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
