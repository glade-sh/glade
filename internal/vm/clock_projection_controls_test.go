package vm

import (
	"testing"
	"time"
)

func TestClockReadPreservesExplicitEpochAndRawTime(t *testing.T) {
	program, err := CompileAnonymous(`
Datetime first=System.now();
Datetime explicit=Datetime.newInstance(1767225600001L);
Datetime second=Datetime.now();
Datetime third=DateTime.Now();
Long raw=System.currentTimeMillis();
System.assertEquals(0L,second.getTime()-first.getTime());
System.assertEquals(0L,third.getTime()-second.getTime());
System.assertEquals(1767225600001L,explicit.getTime());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	initial := machine.fakeNow
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if got := machine.Globals["raw"]; got.Int != machine.lastNow.UnixMilli() {
		t.Fatalf("raw clock changed by public projection: %v", got)
	}
	if machine.fakeNow.Sub(initial).Milliseconds() != 0 {
		t.Fatal("back-to-back clock reads must share a millisecond")
	}
}

func TestDatetimeNowPreservesMillisecondsAcrossReads(t *testing.T) {
	for _, source := range []string{"Datetime.now()", "System.now()", "DateTime.Now()"} {
		for _, millis := range []int{10, 999} {
			t.Run(source+time.Duration(millis).String(), func(t *testing.T) {
				machine := New(nil)
				machine.fakeNow = time.Date(2026, 10, 14, 3, 53, 57, millis*int(time.Millisecond), time.UTC)
				initial := machine.fakeNow
				program, err := CompileAnonymous("Datetime first=" + source + ";Datetime second=" + source + ";System.assertEquals(0L,second.getTime()-first.getTime());")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{"first", "second"} {
					instant, err := parsePlatformDatetime(machine.Globals[name])
					if err != nil {
						t.Fatal(err)
					}
					if instant.UnixMilli() != initial.UnixMilli() {
						t.Fatalf("%s lost milliseconds: %s", name, instant)
					}
				}
				if machine.fakeNow.Sub(initial) != 0 {
					t.Fatal("back-to-back clock reads must not advance the clock")
				}
			})
		}
	}
}

func TestClockAdvancesAfterWorkAndLongReadRuns(t *testing.T) {
	machine := New(nil)
	first := machine.readClock()
	if again := machine.readClock(); !again.Equal(first) {
		t.Fatalf("back-to-back reads differ: %s %s", first, again)
	}
	machine.limits.DMLStatements++
	afterWork := machine.readClock()
	if got := afterWork.Sub(first); got != time.Millisecond {
		t.Fatalf("a read after a DML statement moved the clock by %s, want 1ms", got)
	}
	if again := machine.readClock(); !again.Equal(afterWork) {
		t.Fatal("a second read after the DML statement must share its millisecond")
	}
	reads := 0
	for machine.readClock().Equal(afterWork) {
		reads++
		if reads > 10*clockReadsPerMillisecond {
			t.Fatal("a polling loop never saw the clock move")
		}
	}
	if reads < 100 {
		t.Fatalf("clock moved after %d reads, want a native-like run", reads)
	}
}
