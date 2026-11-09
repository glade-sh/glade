package vm

import (
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/config"
)

// Without test.clock.start a run starts on a Wednesday in mid-month. The
// previous default, 2026-05-02, was a Saturday, so weekday-sensitive tests
// failed on every local run.
func TestDefaultTestClockIsMidweekWeekday(t *testing.T) {
	machine := New(nil)
	if want := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC); !machine.fakeNow.Equal(want) {
		t.Fatalf("default clock = %s, want %s", machine.fakeNow, want)
	}
	if got := (config.TestConfig{}).EffectiveEnvironment().Clock.Start; got != config.DefaultTestClockStart {
		t.Fatalf("effective clock start = %q, want %q", got, config.DefaultTestClockStart)
	}
	for offset := -12; offset <= 14; offset++ {
		local := machine.fakeNow.In(time.FixedZone("", offset*3600))
		for days := -1; days <= 1; days++ {
			day := local.AddDate(0, 0, days)
			if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
				t.Fatalf("UTC%+d: today%+d is %s", offset, days, day.Weekday())
			}
		}
		if week := local.AddDate(0, 0, -7); week.Month() != local.Month() {
			t.Fatalf("UTC%+d: a week earlier leaves the month", offset)
		}
		if week := local.AddDate(0, 0, 7); week.Month() != local.Month() {
			t.Fatalf("UTC%+d: a week later leaves the month", offset)
		}
	}
	program, err := CompileAnonymous(`
Date today = Date.today();
System.assertEquals(Date.newInstance(2026, 5, 13), today);
System.assertEquals('Wednesday', Datetime.now().format('EEEE'));
System.assertEquals(3, today.toStartOfWeek().daysBetween(today));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

// pinSaturdayTestClock starts machine at 2026-05-02T12:00:00Z, the default
// before v0.3.5, for tests whose expected dates, weekday codes and epoch
// values were written against that instant.
func pinSaturdayTestClock(machine *VM) {
	machine.fakeNow = time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
}
