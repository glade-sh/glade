package dml

import (
	"fmt"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// A DateTime field holds instants from 1700-01-01T00:00:00Z through
// 4000-12-31T00:00:00Z, and a Date field 1700-01-01 through 4000-12-31. Native
// dth_r2_rngins, rngdb and rngupd (surf_collections_strings, API 62.0 and
// 67.0): an insert, a Database.insert or an update past either end fails with
// FIELD_INTEGRITY_EXCEPTION naming the value in java.util.Date text. The rows
// bracket both bounds: 1699-12-31 23:59:59 and 1700-01-01 00:00:00, and
// 4000-12-31 00:00:00 and 00:00:01.
var (
	dateTimeFieldMin = time.Date(1700, time.January, 1, 0, 0, 0, 0, time.UTC)
	dateTimeFieldMax = time.Date(4000, time.December, 31, 0, 0, 0, 0, time.UTC)
)

func validateDateTimeFieldRange(fieldName string, field storage.Field, value storage.Value) error {
	var instant time.Time
	switch {
	case field.Type == storage.FieldDateTime && value.Kind == storage.ValueDateTime:
		parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value.String))
		if err != nil {
			return nil
		}
		instant = parsed.UTC()
	case field.Type == storage.FieldDate && value.Kind == storage.ValueDate:
		// A Date field holds 1700-01-01 through 4000-12-31 (the whole last day).
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value.String))
		if err != nil {
			return nil
		}
		instant = parsed.UTC()
		if !instant.Before(dateTimeFieldMin) && !instant.After(dateTimeFieldMax) {
			return nil
		}
	default:
		return nil
	}
	if !instant.Before(dateTimeFieldMin) && !instant.After(dateTimeFieldMax) {
		return nil
	}
	label := strings.TrimSpace(field.Label)
	if label == "" {
		label = fieldName
	}
	return dmlErrorf("FIELD_INTEGRITY_EXCEPTION", []string{fieldName}, "%s: invalid date: %s", label, javaDateText(instant))
}

// javaDateText renders an instant as java.util.Date.toString does in GMT:
// "Thu Oct 04 23:59:59 GMT 1582". Instants before 1582-10-15 use the Julian
// calendar, as the hybrid calendar behind java.util.Date does.
func javaDateText(instant time.Time) string {
	year, month, day := instant.Date()
	if instant.Before(time.Date(1582, time.October, 15, 0, 0, 0, 0, time.UTC)) {
		days := instant.Unix() / 86400
		if instant.Unix()%86400 < 0 {
			days--
		}
		c := int(days) + 2440588 + 32082
		d := (4*c + 3) / 1461
		e := c - 1461*d/4
		m := (5*e + 2) / 153
		day = e - (153*m+2)/5 + 1
		month = time.Month(m + 3 - 12*(m/10))
		year = d - 4800 + m/10
	}
	return fmt.Sprintf("%s %s %02d %02d:%02d:%02d GMT %d", instant.Weekday().String()[:3], month.String()[:3], day, instant.Hour(), instant.Minute(), instant.Second(), year)
}
