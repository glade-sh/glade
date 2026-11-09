package soql

import (
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

func TestLikeMatchesUnicodeCodePoints(t *testing.T) {
	for _, tc := range []struct {
		text, pattern string
		want          bool
	}{
		{"É", "_", true},
		{"É", "__", false},
		{"É", "é", true},
		{"é", "É", true},
		{"Éa", "_a", true},
		{"Éa", "%a", true},
		{"naïve café", "NA_VE%É", true},
		{"naive", "na_ve", true},
		{"Service", "service", true},
		{"É", "e", false},
	} {
		if got := matchLikePattern(tc.text, tc.pattern); got != tc.want {
			t.Errorf("LIKE %q %q = %t, want %t", tc.text, tc.pattern, got, tc.want)
		}
	}
}

func TestWeekLiteralsFollowFirstWeekday(t *testing.T) {
	// Friday 2026-11-06. Captured in a scratch org: a Monday-start locale puts
	// THIS_WEEK at Mon 11-02..Sun 11-08; a Sunday-start locale at Sun 11-01..Sat 11-07.
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		literal, start, end string
		first               time.Weekday
	}{
		{"THIS_WEEK", "2026-11-02", "2026-11-09", time.Monday},
		{"THIS_WEEK", "2026-11-01", "2026-11-08", time.Sunday},
		{"LAST_WEEK", "2026-10-26", "2026-11-02", time.Monday},
		{"NEXT_WEEK", "2026-11-09", "2026-11-16", time.Monday},
		{"LAST_N_WEEKS:2", "2026-10-19", "2026-11-02", time.Monday},
		{"NEXT_N_WEEKS:1", "2026-11-09", "2026-11-16", time.Monday},
		{"N_WEEKS_AGO:1", "2026-10-26", "2026-11-02", time.Monday},
		{"LAST_WEEK", "2026-10-25", "2026-11-01", time.Sunday},
	} {
		start, end, ok := dateLiteralWithWeekStart(tc.literal, now, 1, tc.first)
		if !ok || start.String != tc.start || end.String != tc.end {
			t.Errorf("%s first=%s: got %q..%q ok=%t, want %s..%s", tc.literal, tc.first, start.String, end.String, ok, tc.start, tc.end)
		}
	}
}

func TestParseWithWeekStartSetsTimeZoneAndCachesPerWeekStart(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	monday, err := ParseAtWithFiscalYearStartMonthTimeZoneAndWeekStart("SELECT Id FROM Contact WHERE Birthdate = THIS_WEEK", now, 1, "GMT", time.Monday)
	if err != nil {
		t.Fatal(err)
	}
	sunday, err := ParseAtWithFiscalYearStartMonthAndTimeZone("SELECT Id FROM Contact WHERE Birthdate = THIS_WEEK", now, 1, "GMT")
	if err != nil {
		t.Fatal(err)
	}
	if monday.Where.Value.String != "2026-11-02" || sunday.Where.Value.String != "2026-11-01" {
		t.Fatalf("week bounds: monday=%q sunday=%q", monday.Where.Value.String, sunday.Where.Value.String)
	}
	if monday.TimeZoneID != "GMT" {
		t.Fatalf("TimeZoneID = %q", monday.TimeZoneID)
	}
}

func TestTextOrderTiesOnCaseDifferences(t *testing.T) {
	// Native ORDER BY Name,Id: names equal ignoring case tie so Id decides.
	if got := compareSOQLText("SERVICE", "Service"); got != 0 {
		t.Fatalf("compareSOQLText case tie = %d", got)
	}
	if got := compareSOQLText("a", "B"); got >= 0 {
		t.Fatalf("compareSOQLText(a,B) = %d", got)
	}
}

func TestTurkishTextEqualityFold(t *testing.T) {
	org := storage.OrgState{}
	def := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}
	eq := func(locale, left, right string) bool {
		return equalFieldValuesInOrg(org, def, "Name", storage.StringValue(left), storage.StringValue(right), locale)
	}
	if !eq("en_US", "Service", "SERVICE") || eq("en_US", "Service", "SERVİCE") {
		t.Fatal("en_US folding changed")
	}
	if !eq("tr_TR", "Service", "SERVİCE") || eq("tr_TR", "Service", "SERVICE") {
		t.Fatal("tr_TR folding must pair i with the dotted capital and I with the dotless i")
	}
}
