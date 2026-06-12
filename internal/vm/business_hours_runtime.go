package vm

import (
	"fmt"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

type businessHoursWindow struct {
	start time.Duration
	end   time.Duration
}

type businessHoursCalendar struct {
	id       string
	location *time.Location
	windows  map[time.Weekday]businessHoursWindow
}

var businessHoursDayFields = []struct {
	weekday time.Weekday
	name    string
}{
	{time.Sunday, "Sunday"},
	{time.Monday, "Monday"},
	{time.Tuesday, "Tuesday"},
	{time.Wednesday, "Wednesday"},
	{time.Thursday, "Thursday"},
	{time.Friday, "Friday"},
	{time.Saturday, "Saturday"},
}

func (vm *VM) businessHoursAdd(callee string, args []Value) (Value, error) {
	if len(args) != 3 || args[0].Kind != ValueString || args[1].Kind != ValueObject || args[1].Type != "Datetime" || args[2].Kind != ValueInt {
		return Null, fmt.Errorf("%s expects String, Datetime, Long", callee)
	}
	calendar, err := vm.businessHoursCalendar(args[0].Text)
	if err != nil {
		return Null, err
	}
	start, err := parsePlatformDatetime(args[1])
	if err != nil {
		return Null, err
	}
	return platformScalar("Datetime", formatPlatformDatetime(calendar.add(start, time.Duration(args[2].Int)*time.Millisecond))), nil
}

func (vm *VM) businessHoursDiff(args []Value) (Value, error) {
	if len(args) != 3 || args[0].Kind != ValueString || args[1].Kind != ValueObject || args[1].Type != "Datetime" || args[2].Kind != ValueObject || args[2].Type != "Datetime" {
		return Null, fmt.Errorf("BusinessHours.diff expects String, Datetime, Datetime")
	}
	calendar, err := vm.businessHoursCalendar(args[0].Text)
	if err != nil {
		return Null, err
	}
	start, err := parsePlatformDatetime(args[1])
	if err != nil {
		return Null, err
	}
	end, err := parsePlatformDatetime(args[2])
	if err != nil {
		return Null, err
	}
	return Int(int64(calendar.diff(start, end) / time.Millisecond)), nil
}

func (vm *VM) businessHoursIsWithin(args []Value) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueObject || args[1].Type != "Datetime" {
		return Null, fmt.Errorf("BusinessHours.isWithin expects String, Datetime")
	}
	calendar, err := vm.businessHoursCalendar(args[0].Text)
	if err != nil {
		return Null, err
	}
	instant, err := parsePlatformDatetime(args[1])
	if err != nil {
		return Null, err
	}
	return Bool(calendar.isWithin(instant)), nil
}

func (vm *VM) businessHoursNextStartDate(args []Value) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueObject || args[1].Type != "Datetime" {
		return Null, fmt.Errorf("BusinessHours.nextStartDate expects String, Datetime")
	}
	calendar, err := vm.businessHoursCalendar(args[0].Text)
	if err != nil {
		return Null, err
	}
	instant, err := parsePlatformDatetime(args[1])
	if err != nil {
		return Null, err
	}
	next, ok := calendar.nextStart(instant)
	if !ok {
		return Null, unsupportedCallError("BusinessHours.nextStartDate no weekly business-hours window")
	}
	return platformScalar("Datetime", formatPlatformDatetime(next)), nil
}

func (vm *VM) businessHoursCalendar(id string) (businessHoursCalendar, error) {
	record, ok := vm.businessHoursRecord(id)
	if !ok {
		if strings.TrimSpace(id) == "" {
			return businessHoursCalendar{}, unsupportedCallError("BusinessHours default record missing")
		}
		return businessHoursCalendar{}, unsupportedCallError("BusinessHours record missing " + id)
	}
	zoneID := strings.TrimSpace(storageStringField(record, "TimeZoneSidKey"))
	if zoneID == "" {
		zoneID = "UTC"
	}
	location, err := time.LoadLocation(zoneID)
	if err != nil {
		return businessHoursCalendar{}, unsupportedCallError("BusinessHours timezone " + zoneID)
	}
	calendar := businessHoursCalendar{
		id:       string(record.ID),
		location: location,
		windows:  make(map[time.Weekday]businessHoursWindow),
	}
	for _, day := range businessHoursDayFields {
		startText := strings.TrimSpace(storageStringField(record, day.name+"StartTime"))
		endText := strings.TrimSpace(storageStringField(record, day.name+"EndTime"))
		if startText == "" || endText == "" {
			continue
		}
		start, err := businessHoursTimeOfDay(startText)
		if err != nil {
			return businessHoursCalendar{}, err
		}
		end, err := businessHoursTimeOfDay(endText)
		if err != nil {
			return businessHoursCalendar{}, err
		}
		if end > start {
			calendar.windows[day.weekday] = businessHoursWindow{start: start, end: end}
		}
	}
	return calendar, nil
}

func (vm *VM) businessHoursRecord(id string) (storage.Record, bool) {
	if vm == nil || vm.Org == nil {
		return storage.Record{}, false
	}
	object, ok := vm.Org.Objects["BusinessHours"]
	if !ok {
		return storage.Record{}, false
	}
	id = strings.TrimSpace(id)
	if id != "" {
		if record, ok := object.Records[storage.ID(id)]; ok {
			return record, true
		}
		for _, record := range object.Records {
			if string(record.ID) == id || storageStringField(record, "Id") == id {
				return record, true
			}
		}
		return storage.Record{}, false
	}
	for _, record := range object.Records {
		if strings.EqualFold(storageStringField(record, "IsDefault"), "true") && strings.EqualFold(storageStringField(record, "IsActive"), "true") {
			return record, true
		}
	}
	return storage.Record{}, false
}

func businessHoursTimeOfDay(text string) (time.Duration, error) {
	parsed, err := parseTimeText(text)
	if err != nil {
		return 0, err
	}
	t, err := time.Parse("15:04:05.000", ensureTimeMillis(parsed))
	if err != nil {
		return 0, err
	}
	return time.Duration(t.Hour())*time.Hour +
		time.Duration(t.Minute())*time.Minute +
		time.Duration(t.Second())*time.Second +
		time.Duration(t.Nanosecond()), nil
}

func (calendar businessHoursCalendar) isWithin(instant time.Time) bool {
	local := instant.In(calendar.location)
	window, ok := calendar.windows[local.Weekday()]
	if !ok {
		return false
	}
	offset := businessHoursLocalOffset(local)
	return offset >= window.start && offset < window.end
}

func (calendar businessHoursCalendar) add(start time.Time, amount time.Duration) time.Time {
	if amount == 0 {
		return start.UTC()
	}
	if amount < 0 {
		return calendar.addBackward(start, -amount)
	}
	remaining := amount
	cursor := start.In(calendar.location)
	for remaining > 0 {
		window, ok := calendar.windows[cursor.Weekday()]
		offset := businessHoursLocalOffset(cursor)
		if !ok || offset >= window.end {
			next, nextOK := calendar.nextStart(cursor)
			if !nextOK {
				return cursor.UTC()
			}
			cursor = next.In(calendar.location)
			continue
		}
		if offset < window.start {
			cursor = businessHoursLocalAt(cursor, window.start, calendar.location)
			offset = window.start
		}
		available := window.end - offset
		if remaining <= available {
			return cursor.Add(remaining).UTC()
		}
		cursor = businessHoursLocalAt(cursor, window.end, calendar.location)
		remaining -= available
	}
	return cursor.UTC()
}

func (calendar businessHoursCalendar) addBackward(start time.Time, amount time.Duration) time.Time {
	remaining := amount
	cursor := start.In(calendar.location)
	for remaining > 0 {
		window, ok := calendar.windows[cursor.Weekday()]
		offset := businessHoursLocalOffset(cursor)
		if !ok || offset <= window.start {
			previous, previousOK := calendar.previousEnd(cursor)
			if !previousOK {
				return cursor.UTC()
			}
			cursor = previous.In(calendar.location)
			continue
		}
		if offset > window.end {
			cursor = businessHoursLocalAt(cursor, window.end, calendar.location)
			offset = window.end
		}
		available := offset - window.start
		if remaining <= available {
			return cursor.Add(-remaining).UTC()
		}
		cursor = businessHoursLocalAt(cursor, window.start, calendar.location)
		remaining -= available
	}
	return cursor.UTC()
}

func (calendar businessHoursCalendar) diff(start, end time.Time) time.Duration {
	if end.Before(start) {
		return -calendar.diff(end, start)
	}
	var total time.Duration
	cursor := start.In(calendar.location)
	limit := end.In(calendar.location)
	for cursor.Before(limit) {
		window, ok := calendar.windows[cursor.Weekday()]
		offset := businessHoursLocalOffset(cursor)
		if !ok || offset >= window.end {
			next, nextOK := calendar.nextStart(cursor)
			if !nextOK || !next.Before(limit) {
				break
			}
			cursor = next.In(calendar.location)
			continue
		}
		if offset < window.start {
			cursor = businessHoursLocalAt(cursor, window.start, calendar.location)
			if !cursor.Before(limit) {
				break
			}
			offset = window.start
		}
		windowEnd := businessHoursLocalAt(cursor, window.end, calendar.location)
		if windowEnd.After(limit) {
			windowEnd = limit
		}
		total += windowEnd.Sub(cursor)
		cursor = windowEnd
		if !cursor.Before(limit) {
			break
		}
	}
	return total
}

func (calendar businessHoursCalendar) nextStart(instant time.Time) (time.Time, bool) {
	if calendar.isWithin(instant) {
		return instant.UTC(), true
	}
	local := instant.In(calendar.location)
	for i := 0; i < 8; i++ {
		day := local.AddDate(0, 0, i)
		window, ok := calendar.windows[day.Weekday()]
		if !ok {
			continue
		}
		start := businessHoursLocalAt(day, window.start, calendar.location)
		if !start.Before(local) {
			return start.UTC(), true
		}
	}
	return time.Time{}, false
}

func (calendar businessHoursCalendar) previousEnd(instant time.Time) (time.Time, bool) {
	local := instant.In(calendar.location)
	for i := 0; i < 8; i++ {
		day := local.AddDate(0, 0, -i)
		window, ok := calendar.windows[day.Weekday()]
		if !ok {
			continue
		}
		end := businessHoursLocalAt(day, window.end, calendar.location)
		if !end.After(local) {
			return end.UTC(), true
		}
	}
	return time.Time{}, false
}

func businessHoursLocalOffset(value time.Time) time.Duration {
	return time.Duration(value.Hour())*time.Hour +
		time.Duration(value.Minute())*time.Minute +
		time.Duration(value.Second())*time.Second +
		time.Duration(value.Nanosecond())
}

func businessHoursLocalAt(day time.Time, offset time.Duration, location *time.Location) time.Time {
	base := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, location)
	return base.Add(offset)
}
