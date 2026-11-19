package vm

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The native Date and Datetime follow java.util.GregorianCalendar's hybrid
// calendar: a civil date before 1582-10-15 is Julian (a leap year every fourth
// year, so 1500-02-29 exists), and a date from the cutover on is Gregorian.
// Native CalShapes, CalArith and CalDate rows (surf_collections_strings
// dth_r3_*, API 62.0 and 67.0). A Datetime keeps its real instant, a Date keeps
// its civil label, and the helpers below convert between a label and the days
// since 1970-01-01. Years before 1 stay proleptic Gregorian (uncaptured).

const gregorianCutoverDays int64 = -141427 // 1582-10-15

func hybridJulian(year, month, day int) bool {
	if year < 1 {
		return false
	}
	return year < 1582 || year == 1582 && (month < 10 || month == 10 && day < 15)
}

// hybridNormalizeMonth folds a month outside 1..12 into the year.
func hybridNormalizeMonth(year, month int) (int, int) {
	month--
	year += floorDiv(month, 12)
	month = month - floorDiv(month, 12)*12
	return year, month + 1
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// hybridDays returns the days since 1970-01-01 of a civil date, leniently: a
// day past the end of the month rolls into the next, as GregorianCalendar does.
func hybridDays(year, month, day int) int64 {
	year, month = hybridNormalizeMonth(year, month)
	if !hybridJulian(year, month, day) {
		return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Unix() / 86400
	}
	return int64(julianDayNumber(year, month, 1)) - 2440588 + int64(day-1)
}

// hybridCivil returns the civil date of a day count.
func hybridCivil(days int64) (int, int, int) {
	if days >= gregorianCutoverDays {
		year, month, day := time.Unix(days*86400, 0).UTC().Date()
		return year, int(month), day
	}
	c := int(days) + 2440588 + 32082
	d := (4*c + 3) / 1461
	e := c - 1461*d/4
	m := (5*e + 2) / 153
	day := e - (153*m+2)/5 + 1
	month := m + 3 - 12*(m/10)
	year := d - 4800 + m/10
	if year < 1 {
		y, mo, dd := time.Unix(days*86400, 0).UTC().Date()
		return y, int(mo), dd
	}
	return year, month, day
}

func hybridIsLeap(year int) bool {
	if year < 1582 && year >= 1 {
		return year%4 == 0
	}
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

// hybridMonthLength is the number of days of a month in its own calendar.
func hybridMonthLength(year, month int) int {
	year, month = hybridNormalizeMonth(year, month)
	switch month {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	}
	if hybridIsLeap(year) {
		return 29
	}
	return 28
}

// hybridLabelTime is the label of a day count as a UTC time whose Gregorian
// fields are the label (the form the Date helpers format).
func hybridLabelTime(days int64) time.Time {
	year, month, day := hybridCivil(days)
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

// addMonthsHybrid adds months to a Datetime in the civil calendar of its date: a
// date before the cutover (or a result before it) is read and rebuilt in the
// hybrid calendar, with the day clamped to the target month's own length (native
// CalArith: Julian 1500-01-01 plus 100 years is 1600-01-01, Julian 1400-02-29
// plus one year is 1401-02-28). Other dates keep the Gregorian arithmetic.
func addMonthsHybrid(t time.Time, months int) time.Time {
	utc := t.UTC()
	seconds := utc.Unix()
	days := seconds / 86400
	if seconds%86400 < 0 {
		days--
	}
	year, month, day := hybridCivil(days)
	targetYear, targetMonth := hybridNormalizeMonth(year, month+months)
	if !utc.Before(gregorianCutover) && targetYear > 1582 {
		return addMonthsClamped(t, months)
	}
	if limit := hybridMonthLength(targetYear, targetMonth); day > limit {
		day = limit
	}
	clock := utc.Sub(time.Unix(days*86400, 0).UTC())
	return julianWallClock(targetYear, targetMonth, day, 0, 0, 0, int(clock/time.Millisecond)).In(t.Location())
}

// dateLabelParts reads a Date label "YYYY-MM-DD" without validating the day
// against the Gregorian calendar.
func dateLabelParts(text string) (int, int, int, bool) {
	text = strings.TrimSpace(text)
	if len(text) > 10 && (text[10] == ' ' || text[10] == 'T') {
		text = text[:10]
	}
	parts := strings.Split(text, "-")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	var out [3]int
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || part == "" {
			return 0, 0, 0, false
		}
		out[i] = n
	}
	return out[0], out[1], out[2], true
}

// datePreGregorian reports a Date label in years 1 through 1582, where the
// hybrid calendar differs from the proleptic Gregorian one.
func datePreGregorian(value Value) (int64, bool) {
	text, err := platformScalarText(value, "Date")
	if err != nil {
		return 0, false
	}
	y, m, d, ok := dateLabelParts(text)
	if !ok || y < 1 || y > 1582 || m < 1 || m > 12 || d < 1 || d > hybridMonthLength(y, m) {
		return 0, false
	}
	return hybridDays(y, m, d), true
}

func hybridLabelText(days int64) string {
	year, month, day := hybridCivil(days)
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}

func hybridDateValue(days int64) Value {
	return platformScalar("Date", hybridLabelText(days))
}

// dateHybridMember answers the Date members for a pre-cutover receiver (native
// CalDate rows: dayOfYear, addDays, addMonths, addYears, daysBetween,
// toStartOfWeek and the parts all follow the Julian civil date). It reports
// handled false for every other receiver.
func (vm *VM) dateHybridMember(receiver Value, method string, args []Value) (Value, bool) {
	days, ok := datePreGregorian(receiver)
	if !ok {
		return Null, false
	}
	year, month, day := hybridCivil(days)
	switch method {
	case "year":
		return Int(int64(year)), true
	case "month":
		return Int(int64(month)), true
	case "day":
		return Int(int64(day)), true
	case "dayOfYear":
		return Int(days - hybridDays(year, 1, 1) + 1), true
	case "addDays":
		if len(args) == 1 && args[0].Kind == ValueInt {
			return hybridDateValue(days + args[0].Int), true
		}
	case "addMonths", "addYears":
		if len(args) == 1 && args[0].Kind == ValueInt {
			months := int(args[0].Int)
			if method == "addYears" {
				months *= 12
			}
			targetYear, targetMonth := hybridNormalizeMonth(year, month+months)
			if limit := hybridMonthLength(targetYear, targetMonth); day > limit {
				day = limit
			}
			return hybridDateValue(hybridDays(targetYear, targetMonth, day)), true
		}
	case "daysBetween":
		if len(args) == 1 && args[0].Kind == ValueObject && args[0].Type == "Date" {
			if other, ok := datePreGregorian(args[0]); ok {
				return Int(other - days), true
			}
			if text, err := platformScalarText(args[0], "Date"); err == nil {
				if y, m, d, ok := dateLabelParts(text); ok {
					return Int(hybridDays(y, m, d) - days), true
				}
			}
		}
	case "toStartOfWeek":
		first := int64(salesforceLocaleFirstWeekday(vm.currentUserInfoField("LocaleSidKey", "en_US")))
		weekday := ((days+4)%7 + 7) % 7 // 1970-01-01 was a Thursday
		return hybridDateValue(days - ((weekday-first)%7+7)%7), true
	case "toStartOfMonth":
		return platformScalar("Date", hybridLabelText(hybridDays(year, month, 1))), true
	}
	return Null, false
}

// hybridYearEnd is 1583-01-01T00:00:00Z. Dates before it can differ from the
// Gregorian reading of their instant (year 1582 holds the cutover).
var hybridYearEnd = time.Date(1583, time.January, 1, 0, 0, 0, 0, time.UTC)

// julianOnlyLeapDay reports a Julian February 29 that the Gregorian calendar
// does not have (1500, 1400, 1300 ...) and its year, for an instant read at the
// given zone offset.
func julianOnlyLeapDay(instant time.Time, offset time.Duration) (int, bool) {
	seconds := instant.UTC().Unix() + int64(offset/time.Second)
	days := seconds / 86400
	if seconds%86400 < 0 {
		days--
	}
	year, month, day := hybridCivil(days)
	if year >= 1 && year < 1582 && month == 2 && day == 29 && !(year%4 == 0 && (year%100 != 0 || year%400 == 0)) {
		return year, true
	}
	return 0, false
}

var julianDatetimeTextPattern = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2}) (\d{2}):(\d{2}):(\d{2})$`)

// julianDatetimeText reads "yyyy-MM-dd HH:mm:ss" for a date in the Julian years
// whose day is valid in its own calendar but may not be in the Gregorian one
// (1500-02-29). Other texts return ok false and take the ordinary parse.
func julianDatetimeText(text string) (year, month, day, hour, minute, second int, ok bool) {
	m := julianDatetimeTextPattern.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return
	}
	var n [6]int
	for i := range n {
		n[i], _ = strconv.Atoi(m[i+1])
	}
	if n[0] < 1 || n[0] > 1582 || n[1] < 1 || n[1] > 12 || n[2] < 1 || n[2] > hybridMonthLength(n[0], n[1]) || n[3] > 23 || n[4] > 59 || n[5] > 59 {
		return
	}
	if julianLeapOnly := n[1] == 2 && n[2] == 29; !julianLeapOnly {
		return
	}
	return n[0], n[1], n[2], n[3], n[4], n[5], true
}

// dateLabelFromGregorian is the Date label of a proleptic Gregorian civil date,
// as JSON.deserialize reads it: the instant is the Gregorian date, and a Date
// before the cutover is labeled with that instant's Julian civil date, so the
// JSON text round-trips (json_short_year_date_test, native API 63 rows).
func dateLabelFromGregorian(value time.Time) string {
	if value.Year() >= 1 && value.Year() <= 1582 {
		return hybridLabelText(time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC).Unix() / 86400)
	}
	return value.Format("2006-01-02")
}

var englishWeekdays = [...]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

// hybridPatternLiterals rewrites the calendar tokens of a format pattern (year,
// day of year, week of year and of month, day of week in month, weekday name and
// number) into quoted literals computed from the hybrid calendar for a date
// before 1583, because the Gregorian time the formatter reads is a label shifted
// from the real day (native FmtJulian rows: 1500-02-29 is a Saturday, day 60,
// week 9). Sunday-first weeks with one minimal day (en_US). The other tokens
// keep reading the label.
func hybridPatternLiterals(pattern string, localDays int64, locale string) string {
	year, month, _ := hybridCivil(localDays)
	if year < 1 {
		return pattern
	}
	english := strings.HasPrefix(strings.ToLower(locale), "en")
	weekday := int(((localDays+4)%7 + 7) % 7) // Sunday = 0
	firstOfMonth := hybridDays(year, month, 1)
	monthDow := int(((firstOfMonth+4)%7 + 7) % 7)
	index := int(localDays-firstOfMonth) + 1 // the day's place in its month, counting the days the cutover skipped as absent
	jan1 := hybridDays(year, 1, 1)
	jan1Dow := int(((jan1+4)%7 + 7) % 7)
	doy := int(localDays-jan1) + 1
	week := (doy-1+jan1Dow)/7 + 1
	if nextJan1 := hybridDays(year+1, 1, 1); nextJan1-localDays <= int64(6-weekday) {
		week = 1
	}
	pad := func(n, width int) string { return fmt.Sprintf("%0*d", width, n) }
	var out strings.Builder
	for i := 0; i < len(pattern); {
		ch := pattern[i]
		if ch == '\'' {
			j := i + 1
			for j < len(pattern) {
				if pattern[j] == '\'' {
					if j+1 < len(pattern) && pattern[j+1] == '\'' {
						j += 2
						continue
					}
					break
				}
				j++
			}
			if j >= len(pattern) {
				j = len(pattern) - 1
			}
			out.WriteString(pattern[i : j+1])
			i = j + 1
			continue
		}
		j := i + 1
		for j < len(pattern) && pattern[j] == ch {
			j++
		}
		count := j - i
		literal := ""
		switch ch {
		case 'y':
			if count == 2 {
				literal = pad(year%100, 2)
			} else {
				literal = pad(year, count)
			}
		case 'D':
			literal = pad(doy, count)
		case 'w':
			literal = pad(week, count)
		case 'W':
			literal = pad((index-1+monthDow)/7+1, count)
		case 'F':
			literal = pad((index-1)/7+1, count)
		case 'E':
			if english {
				literal = englishWeekdays[weekday]
				if count < 4 {
					literal = literal[:3]
				}
			}
		case 'u':
			if english {
				n := weekday
				if n == 0 {
					n = 7
				}
				literal = pad(n, count)
			}
		}
		if literal == "" {
			out.WriteString(pattern[i:j])
		} else {
			out.WriteString("'" + literal + "'")
		}
		i = j
	}
	return out.String()
}

// hybridLocal returns the civil wall clock of a zone-local time in the hybrid
// calendar: the shift is decided by the local day, not the UTC day, so an
// instant just after the cutover that is still before it in a negative-offset
// zone reads the Julian date (1582-10-15 04:00Z in Los Angeles is 1582-10-04).
func hybridLocal(raw, local time.Time) time.Time {
	if !raw.Before(hybridYearEnd.Add(48 * time.Hour)) {
		return local
	}
	_, offset := local.Zone()
	wall := julianReadInstant(raw.UTC().Add(time.Duration(offset) * time.Second))
	return time.Date(wall.Year(), wall.Month(), wall.Day(), wall.Hour(), wall.Minute(), wall.Second(), wall.Nanosecond(), local.Location())
}
