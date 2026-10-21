package vm

import (
	"testing"
	"time"
)

// The field domains come from the Apex Developer Guide, "Using the
// System.Schedule Method":
// https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/apex_scheduler.htm
// These exercise the cron calculation, not native test-job lifecycle or DST.
func TestCronNextFireFieldDomains(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 17, 0, 500000000, time.UTC)
	for _, test := range []struct {
		name, expression, want string
	}{
		{"synthetic recurring hour", "0 17 * ? * * *", "2026-10-14T13:17:00Z"},
		{"fixed hour control", "0 17 3 ? * * *", "2026-10-15T03:17:00Z"},
		{"optional year", "0 17 * ? * *", "2026-10-14T13:17:00Z"},
		{"unordered hour list", "0 17 18,13,3 ? * * *", "2026-10-14T13:17:00Z"},
		{"hour increment", "0 17 0/5 ? * * *", "2026-10-14T15:17:00Z"},
		{"hour range", "0 17 13-15 ? * * *", "2026-10-14T13:17:00Z"},
		{"day list", "0 17 3 31,15 * ? *", "2026-10-15T03:17:00Z"},
		{"month names", "0 17 3 1 DEC,NOV ? *", "2026-11-01T03:17:00Z"},
		{"weekday range", "0 17 3 ? * MON-FRI *", "2026-10-15T03:17:00Z"},
		{"weekday list", "0 17 3 ? * SAT,SUN *", "2026-10-17T03:17:00Z"},
		{"lowercase weekday", "0 17 3 ? * mon *", "2026-10-19T03:17:00Z"},
		{"leap date", "0 17 * 29 FEB ? *", "2028-02-29T00:17:00Z"},
		{"far fixed year control", "0 17 3 1 1 ? 2099", "2099-01-01T03:17:00Z"},
		{"far year list", "0 17 * 1 1 ? 2099,2098", "2098-01-01T00:17:00Z"},
		{"year range", "0 17 * 1 JAN-MAR ? 2098-2099", "2098-01-01T00:17:00Z"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := cronNextFireTime(test.expression, now)
			if !ok || got != test.want {
				t.Fatalf("next fire for %q = %q (%v), want %q", test.expression, got, ok, test.want)
			}
		})
	}
}

func TestCronNextFireBoundaries(t *testing.T) {
	now := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	if got, ok := cronNextFireTime("0 0 * ? * *", now); !ok || got != "2027-01-01T00:00:00Z" {
		t.Fatalf("year rollover = %q (%v)", got, ok)
	}
	// Day-of-month L selects the last day of each month.
	if got, ok := cronNextFireTime("0 17 3 L * ?", now); !ok || got != "2027-01-31T03:17:00Z" {
		t.Fatalf("last day of month = %q (%v)", got, ok)
	}
	// Unsupported syntax and invalid domains must not normalize to other dates
	// or leave a partially parsed list in use. Registration diagnostics are separate.
	for _, expression := range []string{
		"0 17 * ? *", "0 17 * ? * * * extra",
		"0 60 * ? * *", "0 17 3,24 ? * *", "0 17 5-3 ? * *",
		"0 17 3, ? * *", "0 17 */0 ? * *", "0 17 3 ? BAD *",
		"0 17 3 ? * MON-BOGUS", "0 17 3 31 FEB ? 2099",
		"0 17 3 1 JAN ? 2000", "0 17 3 ? * MON#2",
	} {
		if got, ok := cronNextFireTime(expression, now); ok {
			t.Errorf("unexpected next fire for %q: %q", expression, got)
		}
	}
}
