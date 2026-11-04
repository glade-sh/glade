package soql

import (
	"github.com/glade-sh/glade/internal/storage"
	"strings"
	"testing"
)

// case332OriginalDateRange is the pre-optimization date-range matcher.
func case332OriginalDateRange(left storage.Value, condition *Condition) (bool, bool) {
	if left.Kind == storage.ValueNull || condition.Value.Kind != storage.ValueDate || condition.Value2.Kind != storage.ValueDate {
		return false, false
	}
	compareStart, compareEnd := 0, 0
	if left.Kind == storage.ValueDate {
		compareStart = strings.Compare(left.String, condition.Value.String)
		compareEnd = strings.Compare(left.String, condition.Value2.String)
	} else if left.Kind == storage.ValueDateTime {
		start, end, ok := dateLiteralUTCBounds(condition.Value.String, condition.Value2.String, condition.DateLiteralTimeZoneID)
		if !ok {
			return false, false
		}
		value, ok := parseISODateTime(left.String)
		if !ok {
			return false, false
		}
		compareStart = compareTime(value, start)
		compareEnd = compareTime(value, end)
	} else {
		return false, false
	}

	switch condition.Op {
	case "=":
		return compareStart >= 0 && compareEnd < 0, true
	case "!=":
		return compareStart < 0 || compareEnd >= 0, true
	case ">":
		return compareEnd > 0, true
	case ">=":
		return compareStart >= 0, true
	case "<":
		return compareStart < 0, true
	case "<=":
		return compareEnd < 0, true
	default:
		return false, false
	}
}

func TestCase332DateAgainstOriginal(t *testing.T) {
	scratch := &predicateScratch{}
	for _, zone := range []string{"UTC", " GMT ", "utc", "America/New_York", "UTC+05:30", "No/Such_Zone", ""} {
		for _, op := range []string{"=", "!=", ">", ">=", "<", "<=", "?"} {
			for _, value := range []storage.Value{storage.NullValue(), storage.StringValue("x"), storage.DateValue("2024-03-10"), storage.DateTimeValue("2024-03-10T00:00:00Z"), storage.DateTimeValue("2024-03-11T00:00:00Z"), storage.DateTimeValue("bad")} {
				condition := Condition{Op: op, Range: true, Value: storage.DateValue("2024-03-10"), Value2: storage.DateValue("2024-03-11"), DateLiteralTimeZoneID: zone}
				want, wh := case332OriginalDateRange(value, &condition)
				got, gh := matchesDateLiteralRangeWithScratch(value, &condition, scratch)
				if got != want || gh != wh {
					t.Fatalf("zone %q op %q value %#v: got %v/%v want %v/%v", zone, op, value, got, gh, want, wh)
				}
			}
		}
	}
	for _, end := range []string{"bad", strings.Repeat("x", 65)} {
		condition := Condition{Op: "=", Range: true, Value: storage.DateValue("2024-03-10"), Value2: storage.DateValue(end), DateLiteralTimeZoneID: "UTC"}
		value := storage.DateTimeValue("2024-03-10T00:00:00Z")
		for i := 0; i < 2; i++ {
			want, wh := case332OriginalDateRange(value, &condition)
			got, gh := matchesDateLiteralRangeWithScratch(value, &condition, scratch)
			if got != want || gh != wh {
				t.Fatalf("invalid end: got %v/%v want %v/%v", got, gh, want, wh)
			}
		}
	}
}
