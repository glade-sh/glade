package server

import (
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// recordTimeContext captures the current user's display settings once per request.
// A nil location preserves list's unknown-zone behavior; LDS uses UTC instead.
type recordTimeContext struct {
	location *time.Location
	locale   string
}

func newRecordTimeContext(org *storage.OrgState) recordTimeContext {
	if org == nil {
		return recordTimeContext{}
	}
	user := (&Server{Org: org}).currentUser(nil, "")
	zone := userString(user, "TimeZoneSidKey", "")
	var location *time.Location
	if zone != "" {
		location, _ = time.LoadLocation(zone)
	}
	return recordTimeContext{location: location, locale: userString(user, "LocaleSidKey", "")}
}

func (context recordTimeContext) ldsLocation() *time.Location {
	if context.location == nil {
		return time.UTC
	}
	return context.location
}

func (context recordTimeContext) listDisplay(value string) any {
	if context.locale != "en_US" || context.location == nil {
		return nil
	}
	stamp, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil
	}
	return stamp.In(context.location).Format("1/2/2006, 3:04 PM")
}
