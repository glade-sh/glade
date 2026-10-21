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
System.assertEquals(1L,second.getTime()-first.getTime());
System.assertEquals(1L,third.getTime()-second.getTime());
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
	if machine.fakeNow.Sub(initial).Milliseconds() != 3 {
		t.Fatal("public clock reads changed elapsed-time simulation")
	}
}

func TestDatetimeNowPreservesMillisecondsAcrossReads(t *testing.T) {
	for _, source := range []string{"Datetime.now()", "System.now()", "DateTime.Now()"} {
		for _, millis := range []int{10, 999} {
			t.Run(source+time.Duration(millis).String(), func(t *testing.T) {
				machine := New(nil)
				machine.fakeNow = time.Date(2026, 10, 14, 3, 53, 57, millis*int(time.Millisecond), time.UTC)
				initial := machine.fakeNow
				program, err := CompileAnonymous("Datetime first=" + source + ";Datetime second=" + source + ";System.assertEquals(1L,second.getTime()-first.getTime());")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
				for i, name := range []string{"first", "second"} {
					instant, err := parsePlatformDatetime(machine.Globals[name])
					if err != nil {
						t.Fatal(err)
					}
					if instant.UnixMilli() != initial.UnixMilli()+int64(i) {
						t.Fatalf("%s lost milliseconds: %s", name, instant)
					}
				}
				if machine.fakeNow.Sub(initial) != 2*time.Millisecond {
					t.Fatal("each clock read must advance exactly one millisecond")
				}
			})
		}
	}
}
