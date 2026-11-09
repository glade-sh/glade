package dml

import (
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// RegulatoryCode.IsActive is a system-computed checkbox: describe reports it
// calculated, not createable and not updateable, and a saved row holds true
// exactly while EffectiveFrom <= now and EffectiveTo is empty or later than now.
// industries_standard_objects/org.tsv, API 62.0 and 67.0.
func (e *Engine) applyRegulatoryCodeActive(record *storage.Record) {
	if e == nil || record == nil || !strings.EqualFold(record.Object, "RegulatoryCode") {
		return
	}
	from, ok := regulatoryCodeInstant(*record, "EffectiveFrom")
	if !ok {
		return
	}
	now := time.Now()
	if e.Now != nil {
		now = e.Now()
	}
	active := !from.After(now)
	if to, ok := regulatoryCodeInstant(*record, "EffectiveTo"); ok && !to.After(now) {
		active = false
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	record.Fields["IsActive"] = storage.BooleanValue(active)
	delete(record.ExplicitNulls, "IsActive")
}

func regulatoryCodeInstant(record storage.Record, field string) (time.Time, bool) {
	value, found := record.GetField(field)
	if !found || value.Kind != storage.ValueDateTime {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	return parsed, err == nil
}
