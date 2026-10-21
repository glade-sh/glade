package vm

import (
	"strconv"
	"testing"
	"time"
)

// Apex Developer Guide, "Using the System.schedule Method", documents wildcard
// hours, Sunday-first weekday names, and evaluation in the scheduling user's zone.
// https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/apex_scheduler.htm
// These tests cover calendar calculations, not native post-stopTest visibility.
func TestCronNextFireDateHourlyDomain(t *testing.T) {
	zone := time.FixedZone("schedule-user", -7*60*60)
	cases := []struct {
		name, expression string
		now, want        time.Time
	}{
		{"same hour", "0 30 * ? * * *", time.Date(2026, 5, 2, 12, 10, 0, 0, zone), time.Date(2026, 5, 2, 12, 30, 0, 0, zone)},
		{"strictly after", "0 30 * ? * *", time.Date(2026, 5, 2, 12, 30, 0, 0, zone), time.Date(2026, 5, 2, 13, 30, 0, 0, zone)},
		{"next day", "0 5 * ? * * *", time.Date(2026, 5, 2, 23, 10, 0, 0, zone), time.Date(2026, 5, 3, 0, 5, 0, 0, zone)},
		{"explicit date", "0 15 * 15 6 ? 2040", time.Date(2026, 5, 2, 12, 10, 0, 0, zone), time.Date(2040, 6, 15, 0, 15, 0, 0, zone)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := cronNextFireDate(tc.expression, tc.now)
			if !ok || !got.Equal(tc.want) || got.Location() != zone {
				t.Fatalf("next date = %v, %t; want %v in scheduling zone", got, ok, tc.want)
			}
		})
	}
	for minute := 0; minute < 60; minute += 5 {
		now := time.Date(2026, 5, 2, 12, 33, 0, 0, zone)
		expression := "0 " + strconv.Itoa(minute) + " * ? * * *"
		got, ok := cronNextFireDate(expression, now)
		wantHour := 12
		if minute <= now.Minute() {
			wantHour++
		}
		want := time.Date(2026, 5, 2, wantHour, minute, 0, 0, zone)
		if !ok || !got.Equal(want) {
			t.Fatalf("minute %d: next date = %v, %t; want %v", minute, got, ok, want)
		}
	}
}

func TestCronNextFireDateNamedWeekday(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 33, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		day  int
	}{{"SUN", 3}, {"MON", 4}, {"TUE", 5}, {"WED", 6}, {"THU", 7}, {"FRI", 8}, {"SAT", 9}, {"mon", 4}, {"MON-FRI", 4}, {"MON,WED", 4}} {
		expression := "0 0 12 ? * " + tc.name + " *"
		got, ok := cronNextFireDate(expression, now)
		want := time.Date(2026, 5, tc.day, 12, 0, 0, 0, time.UTC)
		if !ok || !got.Equal(want) {
			t.Fatalf("%s: next date = %v, %t; want %v", tc.name, got, ok, want)
		}
	}
}

func TestCronNextFireDateUnsupportedDomainControls(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 33, 0, 0, time.UTC)
	for _, expression := range []string{
		"0 0,15 * ? * * *", "* 0 12 ? * * *", "0 0 ? ? * * *",
		"0 0 24 ? * MON *", "0 60 * ? * * *", "0 0 12 ? * MON-BOGUS *",
		"0 0 12 ? * MON,BOGUS *", "0 0 12 ? * UNKNOWN *", "0 0 * ? * * * EXTRA",
		"0 0 0 1 1 ? 2000",
	} {
		if got, ok := cronNextFireDate(expression, now); ok {
			t.Fatalf("unsupported/past expression %q produced %v", expression, got)
		}
	}
}
