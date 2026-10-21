package vm

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var dateValueOfExtendedYearPattern = regexp.MustCompile(`^(\d{5,})-(\d{1,2})-(\d{1,2})$`)

// Only the String overload with a local clock normalizes calendar overflow.
// The clock does not affect the resulting date, even if its seconds overflow.
func parseDateValueOfText(text string) (time.Time, error) {
	trimmed := strings.TrimSpace(text)
	if parts := datetimeValueOfLocalPartsPattern.FindStringSubmatch(trimmed); parts != nil {
		year, _ := strconv.Atoi(parts[1])
		month, _ := strconv.Atoi(parts[2])
		day, _ := strconv.Atoi(parts[3])
		value := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		if year < 1 || value.Year() < 1 {
			return time.Time{}, fmt.Errorf("unsupported Date value %q", text)
		}
		return value, nil
	}
	if value, ok := parseExtendedDateYear(trimmed); ok {
		return value, nil
	}
	return parseDateText(text)
}

func parseExtendedDateYear(text string) (time.Time, bool) {
	parts := dateValueOfExtendedYearPattern.FindStringSubmatch(text)
	if parts == nil {
		return time.Time{}, false
	}
	year, err := strconv.Atoi(parts[1])
	month, _ := strconv.Atoi(parts[2])
	day, _ := strconv.Atoi(parts[3])
	if err != nil || year < 1 || month < 1 || month > 12 || day < 1 || day > daysInMonth(year, time.Month(month)) {
		return time.Time{}, false
	}
	value := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return value, value.Year() == year && int(value.Month()) == month && value.Day() == day
}
