package soql

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestPredicateScratchMatchesUncached(t *testing.T) {
	scratch := &predicateScratch{}
	for _, p := range []string{"a%", `a\_b`, "%", "a%", `a\_b`} {
		for _, value := range []string{"", "a", "a_b", "abc", "Mixed", "a"} {
			left, right := storage.StringValue(value), storage.StringValue(p)
			if got, want := likeMatchWithScratch(left, right, scratch), likeMatch(left, right); got != want {
				t.Fatalf("LIKE %q %q: cached %v, uncached %v", value, p, got, want)
			}
		}
	}
	for _, zone := range []string{"America/Los_Angeles", "UTC", "unknown/zone"} {
		for _, op := range []string{"=", "!=", ">", ">=", "<", "<="} {
			condition := &Condition{Range: true, Op: op, Value: storage.DateValue("2024-03-10"), Value2: storage.DateValue("2024-03-11"), DateLiteralTimeZoneID: zone}
			for _, value := range []storage.Value{storage.DateTimeValue("2024-03-10T07:59:59Z"), storage.DateTimeValue("2024-03-10T08:00:00Z"), storage.DateTimeValue("2024-03-11T07:00:00Z"), storage.DateValue("2024-03-10"), storage.DateTimeValue("invalid")} {
				got, handled := matchesDateLiteralRangeWithScratch(value, condition, scratch)
				want, wantHandled := matchesDateLiteralRange(value, condition)
				if got != want || handled != wantHandled {
					t.Fatalf("%s %s %#v: cached %v/%v uncached %v/%v", zone, op, value, got, handled, want, wantHandled)
				}
			}
		}
	}
}
