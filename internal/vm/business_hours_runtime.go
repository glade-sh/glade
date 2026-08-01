package vm

import (
	"fmt"
	"strconv"
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
	holidays []businessHoursHolidayRule
}

type businessHoursDayClosure struct {
	allDay  bool
	windows []businessHoursWindow
}

type businessHoursHolidayRule struct {
	id          storage.ID
	activity    time.Time
	allDay      bool
	window      businessHoursWindow
	recurrence  businessHoursRecurrence
	calendarIDs map[string]struct{}
}

type businessHoursRecurrence struct {
	enabled    bool
	typ        string
	interval   int
	start      time.Time
	end        time.Time
	hasEnd     bool
	dayOfMonth int
	dayMask    int
	instance   int
	month      time.Month
}

const businessHoursRecordNotFoundMessage = "BusinessHours record not found.This may indicate: 1) Invalid BusinessHours ID, 2) Data corruption, or 3) Missing setup."

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
	if len(args) != 3 || args[1].Kind != ValueObject || args[1].Type != "Datetime" || args[2].Kind != ValueInt {
		return Null, fmt.Errorf("%s expects Id, Datetime, Long", callee)
	}
	id, err := businessHoursIDArgument(callee, args[0])
	if err != nil {
		return Null, err
	}
	calendar, err := vm.businessHoursCalendar(id)
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
	if len(args) != 3 || args[1].Kind != ValueObject || args[1].Type != "Datetime" || args[2].Kind != ValueObject || args[2].Type != "Datetime" {
		return Null, fmt.Errorf("BusinessHours.diff expects Id, Datetime, Datetime")
	}
	id, err := businessHoursIDArgument("BusinessHours.diff", args[0])
	if err != nil {
		return Null, err
	}
	calendar, err := vm.businessHoursCalendar(id)
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
	if len(args) != 2 || args[1].Kind != ValueObject || args[1].Type != "Datetime" {
		return Null, fmt.Errorf("BusinessHours.isWithin expects Id, Datetime")
	}
	id, err := businessHoursIDArgument("BusinessHours.isWithin", args[0])
	if err != nil {
		return Null, err
	}
	calendar, err := vm.businessHoursCalendar(id)
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
	if len(args) != 2 || args[1].Kind != ValueObject || args[1].Type != "Datetime" {
		return Null, fmt.Errorf("BusinessHours.nextStartDate expects Id, Datetime")
	}
	id, err := businessHoursIDArgument("BusinessHours.nextStartDate", args[0])
	if err != nil {
		return Null, err
	}
	calendar, err := vm.businessHoursCalendar(id)
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

func businessHoursIDArgument(callee string, value Value) (string, error) {
	if value.Kind == ValueNull {
		if callee == "BusinessHours.addGmt" {
			return "", newExceptionError("System.MathException", businessHoursRecordNotFoundMessage)
		}
		return "", newExceptionError("System.NullPointerException", "Business Hours Id cannot be null")
	}
	id, ok := idTextFromValue(value)
	if !ok {
		return "", fmt.Errorf("%s expects Id", callee)
	}
	return id, nil
}

func (vm *VM) businessHoursCalendar(id string) (businessHoursCalendar, error) {
	record, ok := vm.businessHoursRecord(id)
	if !ok {
		return businessHoursCalendar{}, newExceptionError("System.MathException", businessHoursRecordNotFoundMessage)
	}
	zoneID := strings.TrimSpace(storageStringField(record, "TimeZoneSidKey"))
	if zoneID == "" {
		zoneID = "UTC"
	}
	location, err := time.LoadLocation(zoneID)
	if err != nil {
		return businessHoursCalendar{}, unsupportedCallError("BusinessHours timezone " + zoneID)
	}
	holidays, err := vm.businessHoursHolidayRules(string(record.ID))
	if err != nil {
		return businessHoursCalendar{}, err
	}
	calendar := businessHoursCalendar{
		id:       string(record.ID),
		location: location,
		windows:  make(map[time.Weekday]businessHoursWindow),
		holidays: holidays,
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

func (vm *VM) businessHoursHolidayRules(calendarID string) ([]businessHoursHolidayRule, error) {
	var holidays []businessHoursHolidayRule
	if vm == nil || vm.Org == nil {
		return holidays, nil
	}
	object, ok := vm.Org.Objects["Holiday"]
	if !ok {
		return holidays, nil
	}
	linkedCalendars := vm.businessHoursHolidayCalendarLinks()
	for _, record := range object.Records {
		rule, err := businessHoursParseHolidayRule(record)
		if err != nil {
			return nil, err
		}
		for linked := range linkedCalendars[rule.id] {
			if rule.calendarIDs == nil {
				rule.calendarIDs = make(map[string]struct{})
			}
			rule.calendarIDs[linked] = struct{}{}
		}
		if len(rule.calendarIDs) != 0 {
			if _, ok := rule.calendarIDs[calendarID]; !ok {
				continue
			}
		}
		if rule.activity.IsZero() {
			continue
		}
		holidays = append(holidays, rule)
	}
	return holidays, nil
}

func (vm *VM) businessHoursHolidayCalendarLinks() map[storage.ID]map[string]struct{} {
	links := make(map[storage.ID]map[string]struct{})
	if vm == nil || vm.Org == nil {
		return links
	}
	for _, objectName := range []string{"OperatingHoursHoliday", "BusinessHoursHoliday"} {
		object, ok := vm.Org.Objects[objectName]
		if !ok {
			continue
		}
		for _, record := range object.Records {
			holidayID := storage.ID(strings.TrimSpace(storageStringField(record, "HolidayId")))
			calendarID := strings.TrimSpace(storageStringField(record, "OperatingHoursId"))
			if calendarID == "" {
				calendarID = strings.TrimSpace(storageStringField(record, "BusinessHoursId"))
			}
			if holidayID == "" || calendarID == "" {
				continue
			}
			if links[holidayID] == nil {
				links[holidayID] = make(map[string]struct{})
			}
			links[holidayID][calendarID] = struct{}{}
		}
	}
	return links
}

func businessHoursParseHolidayRule(record storage.Record) (businessHoursHolidayRule, error) {
	rule := businessHoursHolidayRule{id: record.ID}
	if rule.id == "" {
		rule.id = storage.ID(strings.TrimSpace(storageStringField(record, "Id")))
	}
	if directCalendarID := strings.TrimSpace(storageStringField(record, "BusinessHoursId")); directCalendarID != "" {
		rule.calendarIDs = map[string]struct{}{directCalendarID: {}}
	}
	if directCalendarID := strings.TrimSpace(storageStringField(record, "OperatingHoursId")); directCalendarID != "" {
		if rule.calendarIDs == nil {
			rule.calendarIDs = make(map[string]struct{})
		}
		rule.calendarIDs[directCalendarID] = struct{}{}
	}
	activity, ok, err := businessHoursHolidayDate(record, "ActivityDate")
	if err != nil {
		return rule, err
	}
	if ok {
		rule.activity = activity
	}
	allDay, ok, err := businessHoursHolidayBool(record, "IsAllDay")
	if err != nil {
		return rule, err
	}
	rule.allDay = !ok || allDay
	if !rule.allDay {
		start, ok, err := businessHoursHolidayInt(record, "StartTimeInMinutes")
		if err != nil || !ok {
			return rule, businessHoursMalformedHoliday("StartTimeInMinutes")
		}
		end, ok, err := businessHoursHolidayInt(record, "EndTimeInMinutes")
		if err != nil || !ok {
			return rule, businessHoursMalformedHoliday("EndTimeInMinutes")
		}
		if start < 0 || end < 0 || start >= end || end > 24*60 {
			return rule, businessHoursMalformedHoliday("StartTimeInMinutes")
		}
		rule.window = businessHoursWindow{start: time.Duration(start) * time.Minute, end: time.Duration(end) * time.Minute}
	}
	recurs, ok, err := businessHoursHolidayBool(record, "IsRecurrence")
	if err != nil {
		return rule, err
	}
	if (ok && recurs) || businessHoursHolidayRecurrenceFieldPresent(record) {
		recurrence, err := businessHoursParseRecurrence(record, rule.activity)
		if err != nil {
			return rule, err
		}
		rule.recurrence = recurrence
	}
	return rule, nil
}

func businessHoursHolidayFieldPresent(record storage.Record, field string) bool {
	value, ok := record.GetField(field)
	return ok && value.Kind != storage.ValueNull
}

func businessHoursHolidayMeaningfulField(record storage.Record, field string) bool {
	value, ok := record.GetField(field)
	if !ok || value.Kind == storage.ValueNull {
		return false
	}
	switch value.Kind {
	case storage.ValueBoolean:
		return value.Boolean
	case storage.ValueInteger:
		return value.Integer != 0
	case storage.ValueDecimal:
		text := strings.TrimSpace(value.Decimal)
		return text != "" && text != "0" && text != "0.0"
	default:
		text := strings.TrimSpace(storageStringField(record, field))
		return text != "" && text != "0" && text != "0.0" && !strings.EqualFold(text, "false")
	}
}

func businessHoursHolidayRecurrenceFieldPresent(record storage.Record) bool {
	for _, field := range []string{
		"RecurrenceDayOfMonth",
		"RecurrenceDayOfWeekMask",
		"RecurrenceEndDateOnly",
		"RecurrenceInstance",
		"RecurrenceInterval",
		"RecurrenceMonthOfYear",
		"RecurrenceStartDate",
		"RecurrenceType",
	} {
		if businessHoursHolidayMeaningfulField(record, field) {
			return true
		}
	}
	return false
}

func businessHoursParseRecurrence(record storage.Record, activity time.Time) (businessHoursRecurrence, error) {
	recurrence := businessHoursRecurrence{enabled: true, interval: 1}
	recurrence.typ = strings.TrimSpace(storageStringField(record, "RecurrenceType"))
	if recurrence.typ == "" {
		return recurrence, businessHoursMalformedHoliday("RecurrenceType")
	}
	switch recurrence.typ {
	case "RecursDaily", "RecursWeekly", "RecursMonthly", "RecursYearly":
	default:
		return recurrence, businessHoursMalformedHoliday("RecurrenceType")
	}
	if interval, ok, err := businessHoursHolidayInt(record, "RecurrenceInterval"); err != nil {
		return recurrence, err
	} else if ok {
		if interval <= 0 {
			return recurrence, businessHoursMalformedHoliday("RecurrenceInterval")
		}
		recurrence.interval = interval
	}
	start, ok, err := businessHoursHolidayDate(record, "RecurrenceStartDate")
	if err != nil {
		return recurrence, err
	}
	if !ok {
		start = activity
	}
	if start.IsZero() {
		return recurrence, businessHoursMalformedHoliday("RecurrenceStartDate")
	}
	recurrence.start = start
	end, ok, err := businessHoursHolidayDate(record, "RecurrenceEndDateOnly")
	if err != nil {
		return recurrence, err
	}
	if ok {
		recurrence.end = end
		recurrence.hasEnd = true
	}
	if dayOfMonth, ok, err := businessHoursHolidayInt(record, "RecurrenceDayOfMonth"); err != nil {
		return recurrence, err
	} else if ok {
		if dayOfMonth < 1 || dayOfMonth > 31 {
			return recurrence, businessHoursMalformedHoliday("RecurrenceDayOfMonth")
		}
		recurrence.dayOfMonth = dayOfMonth
	}
	if dayMask, ok, err := businessHoursHolidayInt(record, "RecurrenceDayOfWeekMask"); err != nil {
		return recurrence, err
	} else if ok {
		if dayMask < 0 || dayMask > 127 {
			return recurrence, businessHoursMalformedHoliday("RecurrenceDayOfWeekMask")
		}
		recurrence.dayMask = dayMask
	}
	instance := strings.TrimSpace(storageStringField(record, "RecurrenceInstance"))
	if instance != "" {
		parsed, ok := businessHoursRecurrenceInstance(instance)
		if !ok {
			return recurrence, businessHoursMalformedHoliday("RecurrenceInstance")
		}
		recurrence.instance = parsed
	}
	month := strings.TrimSpace(storageStringField(record, "RecurrenceMonthOfYear"))
	if month != "" {
		parsed, ok := businessHoursRecurrenceMonth(month)
		if !ok {
			return recurrence, businessHoursMalformedHoliday("RecurrenceMonthOfYear")
		}
		recurrence.month = parsed
	}
	return recurrence, nil
}

func businessHoursMalformedHoliday(field string) error {
	return unsupportedCallError("BusinessHours malformed local holiday metadata " + field)
}

func businessHoursHolidayDate(record storage.Record, field string) (time.Time, bool, error) {
	if !businessHoursHolidayFieldPresent(record, field) {
		return time.Time{}, false, nil
	}
	text := strings.TrimSpace(storageStringField(record, field))
	if text == "" {
		return time.Time{}, false, businessHoursMalformedHoliday(field)
	}
	parsed, err := time.Parse("2006-01-02", text)
	if err != nil {
		return time.Time{}, false, businessHoursMalformedHoliday(field)
	}
	return parsed, true, nil
}

func businessHoursHolidayBool(record storage.Record, field string) (bool, bool, error) {
	if !businessHoursHolidayFieldPresent(record, field) {
		return false, false, nil
	}
	text := strings.TrimSpace(storageStringField(record, field))
	if strings.EqualFold(text, "true") {
		return true, true, nil
	}
	if strings.EqualFold(text, "false") {
		return false, true, nil
	}
	return false, false, businessHoursMalformedHoliday(field)
}

func businessHoursHolidayInt(record storage.Record, field string) (int, bool, error) {
	if !businessHoursHolidayFieldPresent(record, field) {
		return 0, false, nil
	}
	text := strings.TrimSpace(storageStringField(record, field))
	if text == "" {
		return 0, false, businessHoursMalformedHoliday(field)
	}
	value, err := strconv.Atoi(text)
	if err != nil {
		return 0, false, businessHoursMalformedHoliday(field)
	}
	return value, true, nil
}

func businessHoursRecurrenceInstance(text string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "first", "1", "1st":
		return 1, true
	case "second", "2", "2nd":
		return 2, true
	case "third", "3", "3rd":
		return 3, true
	case "fourth", "4", "4th":
		return 4, true
	case "last":
		return -1, true
	default:
		return 0, false
	}
}

func businessHoursRecurrenceMonth(text string) (time.Month, bool) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "1", "january":
		return time.January, true
	case "2", "february":
		return time.February, true
	case "3", "march":
		return time.March, true
	case "4", "april":
		return time.April, true
	case "5", "may":
		return time.May, true
	case "6", "june":
		return time.June, true
	case "7", "july":
		return time.July, true
	case "8", "august":
		return time.August, true
	case "9", "september":
		return time.September, true
	case "10", "october":
		return time.October, true
	case "11", "november":
		return time.November, true
	case "12", "december":
		return time.December, true
	default:
		return 0, false
	}
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
	offset := businessHoursLocalOffset(local)
	for _, segment := range calendar.openSegments(local) {
		if offset >= segment.start && offset < segment.end {
			return true
		}
	}
	return false
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
		segments := calendar.openSegments(cursor)
		offset := businessHoursLocalOffset(cursor)
		segment, ok := businessHoursCurrentOrNextSegment(segments, offset)
		if !ok {
			next, nextOK := calendar.nextStart(cursor)
			if !nextOK {
				return cursor.UTC()
			}
			cursor = next.In(calendar.location)
			continue
		}
		if offset < segment.start {
			cursor = businessHoursLocalAt(cursor, segment.start, calendar.location)
			offset = segment.start
		}
		available := segment.end - offset
		if remaining <= available {
			return cursor.Add(remaining).UTC()
		}
		cursor = businessHoursLocalAt(cursor, segment.end, calendar.location)
		remaining -= available
	}
	return cursor.UTC()
}

func (calendar businessHoursCalendar) addBackward(start time.Time, amount time.Duration) time.Time {
	remaining := amount
	cursor := start.In(calendar.location)
	for remaining > 0 {
		segments := calendar.openSegments(cursor)
		offset := businessHoursLocalOffset(cursor)
		segment, ok := businessHoursCurrentOrPreviousSegment(segments, offset)
		if !ok {
			previous, previousOK := calendar.previousEnd(cursor)
			if !previousOK {
				return cursor.UTC()
			}
			cursor = previous.In(calendar.location)
			continue
		}
		if offset > segment.end {
			cursor = businessHoursLocalAt(cursor, segment.end, calendar.location)
			offset = segment.end
		}
		available := offset - segment.start
		if remaining <= available {
			return cursor.Add(-remaining).UTC()
		}
		cursor = businessHoursLocalAt(cursor, segment.start, calendar.location)
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
		segments := calendar.openSegments(cursor)
		offset := businessHoursLocalOffset(cursor)
		segment, ok := businessHoursCurrentOrNextSegment(segments, offset)
		if !ok {
			next, nextOK := calendar.nextStart(cursor)
			if !nextOK || !next.Before(limit) {
				break
			}
			cursor = next.In(calendar.location)
			continue
		}
		if offset < segment.start {
			cursor = businessHoursLocalAt(cursor, segment.start, calendar.location)
			if !cursor.Before(limit) {
				break
			}
			offset = segment.start
		}
		windowEnd := businessHoursLocalAt(cursor, segment.end, calendar.location)
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
		for _, segment := range calendar.openSegments(day) {
			start := businessHoursLocalAt(day, segment.start, calendar.location)
			if !start.Before(local) {
				return start.UTC(), true
			}
		}
	}
	return time.Time{}, false
}

func (calendar businessHoursCalendar) previousEnd(instant time.Time) (time.Time, bool) {
	local := instant.In(calendar.location)
	for i := 0; i < 8; i++ {
		day := local.AddDate(0, 0, -i)
		segments := calendar.openSegments(day)
		for index := len(segments) - 1; index >= 0; index-- {
			end := businessHoursLocalAt(day, segments[index].end, calendar.location)
			if !end.After(local) {
				return end.UTC(), true
			}
		}
	}
	return time.Time{}, false
}

func (calendar businessHoursCalendar) openSegments(local time.Time) []businessHoursWindow {
	window, ok := calendar.windows[local.Weekday()]
	if !ok {
		return nil
	}
	closure := calendar.holidayClosure(local)
	if closure.allDay {
		return nil
	}
	segments := []businessHoursWindow{window}
	for _, closed := range closure.windows {
		segments = businessHoursSubtractWindow(segments, closed)
	}
	return segments
}

func (calendar businessHoursCalendar) holidayClosure(local time.Time) businessHoursDayClosure {
	var closure businessHoursDayClosure
	for _, holiday := range calendar.holidays {
		if !holiday.matches(local) {
			continue
		}
		if holiday.allDay {
			closure.allDay = true
			closure.windows = nil
			return closure
		}
		closure.windows = append(closure.windows, holiday.window)
	}
	return closure
}

func businessHoursCurrentOrNextSegment(segments []businessHoursWindow, offset time.Duration) (businessHoursWindow, bool) {
	for _, segment := range segments {
		if offset < segment.end {
			return segment, true
		}
	}
	return businessHoursWindow{}, false
}

func businessHoursCurrentOrPreviousSegment(segments []businessHoursWindow, offset time.Duration) (businessHoursWindow, bool) {
	for index := len(segments) - 1; index >= 0; index-- {
		if offset > segments[index].start {
			return segments[index], true
		}
	}
	return businessHoursWindow{}, false
}

func businessHoursSubtractWindow(segments []businessHoursWindow, closed businessHoursWindow) []businessHoursWindow {
	var out []businessHoursWindow
	for _, segment := range segments {
		if closed.end <= segment.start || closed.start >= segment.end {
			out = append(out, segment)
			continue
		}
		if closed.start > segment.start {
			out = append(out, businessHoursWindow{start: segment.start, end: minDuration(closed.start, segment.end)})
		}
		if closed.end < segment.end {
			out = append(out, businessHoursWindow{start: maxDuration(closed.end, segment.start), end: segment.end})
		}
	}
	return out
}

func (holiday businessHoursHolidayRule) matches(local time.Time) bool {
	day := businessHoursDateOnly(local)
	if !holiday.recurrence.enabled {
		return businessHoursSameDate(day, holiday.activity)
	}
	recurrence := holiday.recurrence
	if day.Before(recurrence.start) {
		return false
	}
	if recurrence.hasEnd && day.After(recurrence.end) {
		return false
	}
	switch recurrence.typ {
	case "RecursDaily":
		return int(day.Sub(recurrence.start).Hours()/24)%recurrence.interval == 0
	case "RecursWeekly":
		weeks := int(businessHoursWeekStart(day).Sub(businessHoursWeekStart(recurrence.start)).Hours() / 24 / 7)
		return weeks >= 0 && weeks%recurrence.interval == 0 && businessHoursDayMaskMatches(recurrenceMaskOrWeekday(recurrence.dayMask, recurrence.start.Weekday()), day.Weekday())
	case "RecursMonthly":
		months := businessHoursMonthsBetween(recurrence.start, day)
		return months >= 0 && months%recurrence.interval == 0 && businessHoursMatchesMonthlyDay(recurrence, holiday.activity, day)
	case "RecursYearly":
		years := day.Year() - recurrence.start.Year()
		if years < 0 || years%recurrence.interval != 0 {
			return false
		}
		month := recurrence.month
		if month == 0 {
			month = holiday.activity.Month()
		}
		return day.Month() == month && businessHoursMatchesMonthlyDay(recurrence, holiday.activity, day)
	default:
		return false
	}
}

func businessHoursMatchesMonthlyDay(recurrence businessHoursRecurrence, activity time.Time, day time.Time) bool {
	if recurrence.dayOfMonth != 0 {
		return day.Day() == recurrence.dayOfMonth
	}
	if recurrence.instance != 0 {
		mask := recurrenceMaskOrWeekday(recurrence.dayMask, activity.Weekday())
		return businessHoursDayMaskMatches(mask, day.Weekday()) && businessHoursWeekdayInstance(day) == recurrence.instance
	}
	if !activity.IsZero() {
		return day.Day() == activity.Day()
	}
	return false
}

func businessHoursWeekdayInstance(day time.Time) int {
	nextWeek := day.AddDate(0, 0, 7)
	if nextWeek.Month() != day.Month() {
		return -1
	}
	return ((day.Day() - 1) / 7) + 1
}

func recurrenceMaskOrWeekday(mask int, weekday time.Weekday) int {
	if mask != 0 {
		return mask
	}
	return 1 << int(weekday)
}

func businessHoursDayMaskMatches(mask int, weekday time.Weekday) bool {
	return mask&(1<<int(weekday)) != 0
}

func businessHoursWeekStart(day time.Time) time.Time {
	date := businessHoursDateOnly(day)
	return date.AddDate(0, 0, -int(date.Weekday()))
}

func businessHoursMonthsBetween(start, day time.Time) int {
	return (day.Year()-start.Year())*12 + int(day.Month()-start.Month())
}

func businessHoursDateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func businessHoursSameDate(left, right time.Time) bool {
	return left.Year() == right.Year() && left.Month() == right.Month() && left.Day() == right.Day()
}

func minDuration(left, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}

func maxDuration(left, right time.Duration) time.Duration {
	if left > right {
		return left
	}
	return right
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
