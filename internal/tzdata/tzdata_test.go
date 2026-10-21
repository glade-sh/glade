package tzdata

import (
	"testing"
	"time"
)

// This also runs in a binary with no host zoneinfo or GOROOT database. Merely
// accepting a UTC fallback would conceal a missing embedded database.
func TestNamedLocationsApplyOffsets(t *testing.T) {
	for _, tc := range []struct {
		name   string
		month  time.Month
		offset int
	}{
		{"America/Los_Angeles", time.January, -8 * 60 * 60},
		{"America/Los_Angeles", time.July, -7 * 60 * 60},
		{"Asia/Kolkata", time.January, 5*60*60 + 30*60},
	} {
		t.Run(tc.name+"/"+tc.month.String(), func(t *testing.T) {
			zone, err := time.LoadLocation(tc.name)
			if err != nil {
				t.Fatalf("load named timezone: %v", err)
			}
			if zone == time.UTC || zone.String() != tc.name {
				t.Fatalf("loaded %v instead of %s", zone, tc.name)
			}
			_, offset := time.Date(2026, tc.month, 15, 12, 0, 0, 0, time.UTC).In(zone).Zone()
			if offset != tc.offset {
				t.Fatalf("offset = %d; want %d", offset, tc.offset)
			}
		})
	}
	if _, err := time.LoadLocation("Invalid/OwnedZone"); err == nil {
		t.Fatal("unknown timezone unexpectedly loaded")
	}
}
