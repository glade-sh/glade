package soql

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCase332LikeScratchBounded(t *testing.T) {
	scratch := &predicateScratch{}
	for i := 0; i < 20; i++ {
		p := fmt.Sprintf("prefix%d%%", i)
		got := likeMatchWithScratch(storage.StringValue("prefix1-value"), storage.StringValue(p), scratch)
		want := likeMatch(storage.StringValue("prefix1-value"), storage.StringValue(p))
		if got != want {
			t.Fatalf("pattern %q: got %v want %v", p, got, want)
		}
		if len(scratch.patterns) > 1 {
			t.Fatalf("retained %d LIKE patterns, want at most one", len(scratch.patterns))
		}
	}
	long := strings.Repeat("a", 4097)
	likeMatchWithScratch(storage.StringValue("a"), storage.StringValue(long), scratch)
	if _, ok := scratch.patterns[long]; ok {
		t.Fatal("oversized pattern retained in scratch")
	}
}

func TestCase332DateScratchExcludesMutableZones(t *testing.T) {
	left := storage.DateTimeValue("2024-03-10T08:00:00Z")
	for _, zone := range []string{"America/Los_Angeles", "unknown/zone", "UTC+05:30"} {
		scratch := &predicateScratch{}
		condition := &Condition{Range: true, Op: "=", Value: storage.DateValue("2024-03-10"), Value2: storage.DateValue("2024-03-11"), DateLiteralTimeZoneID: zone}
		got, handled := matchesDateLiteralRangeWithScratch(left, condition, scratch)
		want, expectedHandled := matchesDateLiteralRange(left, condition)
		if got != want || handled != expectedHandled {
			t.Fatalf("%s: %v/%v versus %v/%v", zone, got, handled, want, expectedHandled)
		}
		if len(scratch.bounds) != 0 {
			t.Fatalf("zone %q cached despite mutable location data", zone)
		}
	}
}
