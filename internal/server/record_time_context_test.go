package server

import (
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// A request keeps one snapshot of the user's settings for all records and fields.
func TestRecordTimeContextSnapshot(t *testing.T) {
	for _, tc := range []struct {
		zone, locale string
		listDisplay  any
		ldsOffset    int
	}{
		{"America/Los_Angeles", "en_US", "10/11/2026, 7:56 PM", -7 * 3600},
		{"Asia/Kolkata", "en_US", "10/12/2026, 8:26 AM", 19800},
		{"Invalid/OwnedZone", "en_US", nil, 0},
		{"", "en_US", nil, 0},
		{"America/Los_Angeles", "fr_FR", nil, -7 * 3600},
	} {
		t.Run(tc.zone+"/"+tc.locale, func(t *testing.T) {
			org := ldsTimezoneOrg(tc.zone, tc.locale)
			context := newRecordTimeContext(&org)
			// Mutating the org after construction must not change the request.
			users := org.Objects["User"]
			user := users.Records[defaultLocalUserID]
			user.Fields["TimeZoneSidKey"] = storage.StringValue("UTC")
			user.Fields["LocaleSidKey"] = storage.StringValue("en_US")
			users.Records[user.ID] = user
			org.Objects["User"] = users
			if got := context.listDisplay("2026-10-12T02:56:03.000Z"); got != tc.listDisplay {
				t.Fatalf("list display = %v; want %v", got, tc.listDisplay)
			}
			_, offset := time.Date(2026, 10, 12, 2, 56, 3, 0, time.UTC).In(context.ldsLocation()).Zone()
			if offset != tc.ldsOffset {
				t.Fatalf("LDS offset = %d; want %d", offset, tc.ldsOffset)
			}
			if got := context.listDisplay("invalid"); got != nil {
				t.Fatalf("invalid timestamp = %v", got)
			}
		})
	}
}
