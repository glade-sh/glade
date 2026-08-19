package vm

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

const testBusinessHoursID = "01m000000000001AAA"

func TestBusinessHoursAcceptsSalesforceStringIDCoercion(t *testing.T) {
	vm := New(nil)
	vm.Org = &storage.OrgState{}
	now := platformScalar("Datetime", "2026-06-15T16:00:00.000Z")

	_, err := vm.businessHoursIsWithin([]Value{String("not-an-id"), now})
	var thrown *apexThrowError
	if !errors.As(err, &thrown) {
		t.Fatalf("String argument error = %v, want Salesforce MathException", err)
	}
	message, ok := thrown.value.Fields["message"]
	if !ok || thrown.value.Type != "System.MathException" || message.Kind != ValueString || message.Text != businessHoursRecordNotFoundMessage {
		t.Fatalf("String argument error = %#v, want MathException %q", thrown.value, businessHoursRecordNotFoundMessage)
	}
}

func TestExecBusinessHoursMissingIDExceptionsMatchSalesforce(t *testing.T) {
	const missingID = "01m000000000002AAA"
	const missingMessage = "BusinessHours record not found.This may indicate: 1) Invalid BusinessHours ID, 2) Data corruption, or 3) Missing setup."
	tests := []struct {
		name        string
		call        string
		nullType    string
		nullMessage string
	}{
		{name: "add", call: "BusinessHours.add(%s, nowValue, 1)", nullType: "System.NullPointerException", nullMessage: "Business Hours Id cannot be null"},
		{name: "addGmt", call: "BusinessHours.addGmt(%s, nowValue, 1)", nullType: "System.MathException", nullMessage: missingMessage},
		{name: "diff", call: "BusinessHours.diff(%s, nowValue, nowValue)", nullType: "System.NullPointerException", nullMessage: "Business Hours Id cannot be null"},
		{name: "isWithin", call: "BusinessHours.isWithin(%s, nowValue)", nullType: "System.NullPointerException", nullMessage: "Business Hours Id cannot be null"},
		{name: "nextStartDate", call: "BusinessHours.nextStartDate(%s, nowValue)", nullType: "System.NullPointerException", nullMessage: "Business Hours Id cannot be null"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := CompileAnonymous(fmt.Sprintf(`
Datetime nowValue = Datetime.now();
Id nullId = null;
Id missingId = '%s';
String caught = '';
try {
		%s;
		System.assert(false, 'expected null Id exception');
} catch (Exception e) {
		caught = e.getTypeName() + ':' + e.getMessage();
}
System.assertEquals('%s:%s', caught);
caught = '';
try {
		%s;
		System.assert(false, 'expected missing record exception');
} catch (Exception e) {
		caught = e.getTypeName() + ':' + e.getMessage();
}
System.assertEquals('System.MathException:%s', caught);
`, missingID, fmt.Sprintf(tt.call, "nullId"), tt.nullType, tt.nullMessage, fmt.Sprintf(tt.call, "missingId"), missingMessage))
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.Org = &storage.OrgState{}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}

	t.Run("String coercion", func(t *testing.T) {
		program, err := CompileAnonymous(fmt.Sprintf(`
Datetime nowValue = Datetime.now();
String businessHoursId = '%s';
String caught = '';
try {
	BusinessHours.isWithin(businessHoursId, nowValue);
	System.assert(false, 'expected String variable missing record exception');
} catch (Exception e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assertEquals('System.MathException:%s', caught);
caught = '';
try {
	BusinessHours.isWithin('not-an-id', nowValue);
	System.assert(false, 'expected String literal missing record exception');
} catch (Exception e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assertEquals('System.MathException:%s', caught);
`, missingID, missingMessage, missingMessage))
		if err != nil {
			t.Fatal(err)
		}
		machine := New(nil)
		machine.Org = &storage.OrgState{}
		if _, err := machine.Execute(program); err != nil {
			t.Fatal(err)
		}
	})
}

func TestExecBusinessHoursAllDayHolidaySkipsBusinessWindow(t *testing.T) {
	program, err := CompileAnonymous(`
Datetime mondayNine = Datetime.newInstanceGmt(2026, 6, 15, 16, 0, 0);
System.assertEquals(false, BusinessHours.isWithin('01m000000000001AAA', mondayNine));
System.assertEquals(Datetime.newInstanceGmt(2026, 6, 16, 16, 0, 0), BusinessHours.nextStartDate('01m000000000001AAA', mondayNine));

Datetime fridayFour = Datetime.newInstanceGmt(2026, 6, 12, 23, 0, 0);
Datetime tuesdayTen = BusinessHours.add('01m000000000001AAA', fridayFour, 2 * 60 * 60 * 1000);
System.assertEquals(Datetime.newInstanceGmt(2026, 6, 16, 17, 0, 0), tuesdayTen);
System.assertEquals(tuesdayTen, BusinessHours.addGmt('01m000000000001AAA', fridayFour, 2 * 60 * 60 * 1000));
System.assertEquals(0, BusinessHours.diff('01m000000000001AAA', mondayNine, Datetime.newInstanceGmt(2026, 6, 15, 18, 0, 0)));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testBusinessHoursOrg(t, storage.Record{
		ID:     "0Ho000000000001AAA",
		Object: "Holiday",
		Fields: map[string]storage.Value{
			"Id":                 storage.IDValue("0Ho000000000001AAA"),
			"Name":               storage.StringValue("Founders Day"),
			"ActivityDate":       storage.DateValue("2026-06-15"),
			"IsAllDay":           storage.BooleanValue(true),
			"IsRecurrence":       storage.StringValue("false"),
			"RecurrenceInterval": storage.IntegerValue(0),
			"StartTimeInMinutes": storage.IntegerValue(0),
			"EndTimeInMinutes":   storage.IntegerValue(0),
		},
	})

	machine.Org = &org
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecBusinessHoursFullLocalHolidayCalendar(t *testing.T) {
	tests := []struct {
		name        string
		holidays    []storage.Record
		links       []storage.Record
		expr        string
		wantLiteral string
	}{
		{
			name: "partial day closes only the time window",
			holidays: []storage.Record{testHoliday("0HoPartial000001AAA", map[string]storage.Value{
				"ActivityDate":       storage.DateValue("2026-06-15"),
				"IsAllDay":           storage.BooleanValue(false),
				"StartTimeInMinutes": storage.IntegerValue(12 * 60),
				"EndTimeInMinutes":   storage.IntegerValue(13 * 60),
			})},
			expr:        "BusinessHours.diff('01m000000000001AAA', Datetime.newInstanceGmt(2026, 6, 15, 18, 30, 0), Datetime.newInstanceGmt(2026, 6, 15, 20, 30, 0))",
			wantLiteral: "3600000",
		},
		{
			name: "daily recurrence honors interval and end date",
			holidays: []storage.Record{testHoliday("0HoDaily0000001AAA", map[string]storage.Value{
				"ActivityDate":            storage.DateValue("2026-06-15"),
				"IsAllDay":                storage.BooleanValue(true),
				"IsRecurrence":            storage.BooleanValue(true),
				"RecurrenceType":          storage.StringValue("RecursDaily"),
				"RecurrenceInterval":      storage.IntegerValue(2),
				"RecurrenceStartDate":     storage.DateValue("2026-06-15"),
				"RecurrenceEndDateOnly":   storage.DateValue("2026-06-19"),
				"RecurrenceDayOfWeekMask": storage.IntegerValue(0),
			})},
			expr:        "BusinessHours.isWithin('01m000000000001AAA', Datetime.newInstanceGmt(2026, 6, 17, 16, 0, 0))",
			wantLiteral: "false",
		},
		{
			name: "weekly recurrence uses day masks",
			holidays: []storage.Record{testHoliday("0HoWeekly000001AAA", map[string]storage.Value{
				"ActivityDate":            storage.DateValue("2026-06-15"),
				"IsAllDay":                storage.BooleanValue(true),
				"IsRecurrence":            storage.BooleanValue(true),
				"RecurrenceType":          storage.StringValue("RecursWeekly"),
				"RecurrenceInterval":      storage.IntegerValue(1),
				"RecurrenceDayOfWeekMask": storage.IntegerValue(2),
				"RecurrenceStartDate":     storage.DateValue("2026-06-15"),
				"RecurrenceEndDateOnly":   storage.DateValue("2026-06-29"),
			})},
			expr:        "BusinessHours.isWithin('01m000000000001AAA', Datetime.newInstanceGmt(2026, 6, 22, 16, 0, 0))",
			wantLiteral: "false",
		},
		{
			name: "monthly recurrence uses day of month",
			holidays: []storage.Record{testHoliday("0HoMonthlyDOM01AAA", map[string]storage.Value{
				"ActivityDate":          storage.DateValue("2026-06-15"),
				"IsAllDay":              storage.BooleanValue(true),
				"IsRecurrence":          storage.BooleanValue(true),
				"RecurrenceType":        storage.StringValue("RecursMonthly"),
				"RecurrenceInterval":    storage.IntegerValue(1),
				"RecurrenceDayOfMonth":  storage.IntegerValue(15),
				"RecurrenceStartDate":   storage.DateValue("2026-06-15"),
				"RecurrenceEndDateOnly": storage.DateValue("2026-08-15"),
			})},
			expr:        "BusinessHours.isWithin('01m000000000001AAA', Datetime.newInstanceGmt(2026, 7, 15, 16, 0, 0))",
			wantLiteral: "false",
		},
		{
			name: "monthly recurrence uses instance and weekday mask",
			holidays: []storage.Record{testHoliday("0HoMonthlyInst1AAA", map[string]storage.Value{
				"ActivityDate":            storage.DateValue("2026-06-15"),
				"IsAllDay":                storage.BooleanValue(true),
				"IsRecurrence":            storage.BooleanValue(true),
				"RecurrenceType":          storage.StringValue("RecursMonthly"),
				"RecurrenceInterval":      storage.IntegerValue(1),
				"RecurrenceDayOfWeekMask": storage.IntegerValue(2),
				"RecurrenceInstance":      storage.StringValue("Second"),
				"RecurrenceStartDate":     storage.DateValue("2026-06-01"),
				"RecurrenceEndDateOnly":   storage.DateValue("2026-08-31"),
			})},
			expr:        "BusinessHours.isWithin('01m000000000001AAA', Datetime.newInstanceGmt(2026, 7, 13, 16, 0, 0))",
			wantLiteral: "false",
		},
		{
			name: "yearly recurrence closes matching date",
			holidays: []storage.Record{testHoliday("0HoYearly000001AAA", map[string]storage.Value{
				"ActivityDate":          storage.DateValue("2026-06-15"),
				"IsAllDay":              storage.BooleanValue(true),
				"IsRecurrence":          storage.BooleanValue(true),
				"RecurrenceType":        storage.StringValue("RecursYearly"),
				"RecurrenceStartDate":   storage.DateValue("2026-06-15"),
				"RecurrenceEndDateOnly": storage.DateValue("2028-06-15"),
			})},
			expr:        "BusinessHours.isWithin('01m000000000001AAA', Datetime.newInstanceGmt(2027, 6, 15, 16, 0, 0))",
			wantLiteral: "false",
		},
		{
			name: "yearly recurrence uses month instance and weekday mask",
			holidays: []storage.Record{testHoliday("0HoYearlyInst01AAA", map[string]storage.Value{
				"ActivityDate":            storage.DateValue("2026-06-15"),
				"IsAllDay":                storage.BooleanValue(true),
				"IsRecurrence":            storage.BooleanValue(true),
				"RecurrenceType":          storage.StringValue("RecursYearly"),
				"RecurrenceDayOfWeekMask": storage.IntegerValue(2),
				"RecurrenceInstance":      storage.StringValue("Third"),
				"RecurrenceMonthOfYear":   storage.StringValue("June"),
				"RecurrenceStartDate":     storage.DateValue("2026-01-01"),
				"RecurrenceEndDateOnly":   storage.DateValue("2028-12-31"),
			})},
			expr:        "BusinessHours.isWithin('01m000000000001AAA', Datetime.newInstanceGmt(2027, 6, 21, 16, 0, 0))",
			wantLiteral: "false",
		},
		{
			name: "business hours id scopes holiday to one calendar",
			holidays: []storage.Record{testHoliday("0HoScoped0000001AAA", map[string]storage.Value{
				"ActivityDate":    storage.DateValue("2026-06-15"),
				"IsAllDay":        storage.BooleanValue(true),
				"BusinessHoursId": storage.IDValue(storage.ID(testBusinessHoursID)),
			})},
			expr:        "BusinessHours.isWithin('01m000000000001AAA', Datetime.newInstanceGmt(2026, 6, 15, 16, 0, 0))",
			wantLiteral: "false",
		},
		{
			name: "operating hours holiday link affects one calendar only",
			holidays: []storage.Record{testHoliday("0HoLinked000001AAA", map[string]storage.Value{
				"ActivityDate": storage.DateValue("2026-06-15"),
				"IsAllDay":     storage.BooleanValue(true),
			})},
			links:       []storage.Record{testOperatingHoursHoliday("0OHLinked000001AAA", "0HoLinked000001AAA", testBusinessHoursID)},
			expr:        "BusinessHours.isWithin('01m000000000001AAA', Datetime.newInstanceGmt(2026, 6, 15, 16, 0, 0))",
			wantLiteral: "false",
		},
		{
			name: "unlinked holiday remains global",
			holidays: []storage.Record{testHoliday("0HoGlobal0000001AAA", map[string]storage.Value{
				"ActivityDate": storage.DateValue("2026-06-15"),
				"IsAllDay":     storage.BooleanValue(true),
			})},
			expr:        "BusinessHours.isWithin('01m000000000001AAA', Datetime.newInstanceGmt(2026, 6, 15, 16, 0, 0))",
			wantLiteral: "false",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := CompileAnonymous("System.assertEquals(" + tt.wantLiteral + ", " + tt.expr + ");")
			if err != nil {
				t.Fatal(err)
			}
			org := testBusinessHoursOrg(t, tt.holidays...)
			testSeedOperatingHoursHolidayLinks(t, &org, tt.links...)
			machine := New(nil)
			machine.Org = &org
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExecBusinessHoursMalformedHolidayMetadataIsFenced(t *testing.T) {
	program, err := CompileAnonymous(`
BusinessHours.isWithin('01m000000000001AAA', Datetime.newInstanceGmt(2026, 6, 15, 16, 0, 0));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testBusinessHoursOrg(t, testHoliday("0HoBad000000001AAA", map[string]storage.Value{
		"ActivityDate":          storage.DateValue("2026-06-15"),
		"IsAllDay":              storage.BooleanValue(true),
		"IsRecurrence":          storage.BooleanValue(true),
		"RecurrenceType":        storage.StringValue("RecursSometimes"),
		"RecurrenceStartDate":   storage.DateValue("2026-06-15"),
		"RecurrenceEndDateOnly": storage.DateValue("2026-06-15"),
	}))

	machine := New(nil)
	machine.Org = &org
	_, err = machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || !strings.Contains(runtimeErr.Message, "BusinessHours malformed local holiday metadata RecurrenceType") {
		t.Fatalf("error = %#v, want malformed RecurrenceType UnsupportedFeature", err)
	}
}

func testHoliday(id string, fields map[string]storage.Value) storage.Record {
	full := map[string]storage.Value{
		"Id":   storage.IDValue(storage.ID(id)),
		"Name": storage.StringValue(id),
	}
	for field, value := range fields {
		full[field] = value
	}
	return storage.Record{ID: storage.ID(id), Object: "Holiday", Fields: full}
}

func testOperatingHoursHoliday(id, holidayID, operatingHoursID string) storage.Record {
	return storage.Record{
		ID:     storage.ID(id),
		Object: "OperatingHoursHoliday",
		Fields: map[string]storage.Value{
			"Id":               storage.IDValue(storage.ID(id)),
			"HolidayId":        storage.IDValue(storage.ID(holidayID)),
			"OperatingHoursId": storage.IDValue(storage.ID(operatingHoursID)),
		},
	}
}

func testSeedOperatingHoursHolidayLinks(t *testing.T, org *storage.OrgState, links ...storage.Record) {
	t.Helper()
	if len(links) == 0 {
		return
	}
	storage.EnsureStandardObject(org, "OperatingHoursHoliday")
	object := org.Objects["OperatingHoursHoliday"]
	for _, link := range links {
		object.Records[link.ID] = link
	}
	org.Objects["OperatingHoursHoliday"] = object
}

func testBusinessHoursOrg(t *testing.T, holidays ...storage.Record) storage.OrgState {
	t.Helper()
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "BusinessHours")
	businessHours := org.Objects["BusinessHours"]
	businessHours.Records[testBusinessHoursID] = storage.Record{
		ID:     testBusinessHoursID,
		Object: "BusinessHours",
		Fields: map[string]storage.Value{
			"Id":                 storage.IDValue(testBusinessHoursID),
			"Name":               storage.StringValue("Default"),
			"IsActive":           storage.BooleanValue(true),
			"IsDefault":          storage.BooleanValue(true),
			"TimeZoneSidKey":     storage.StringValue("America/Los_Angeles"),
			"MondayStartTime":    storage.StringValue("09:00:00.000Z"),
			"MondayEndTime":      storage.StringValue("17:00:00.000Z"),
			"TuesdayStartTime":   storage.StringValue("09:00:00.000Z"),
			"TuesdayEndTime":     storage.StringValue("17:00:00.000Z"),
			"WednesdayStartTime": storage.StringValue("09:00:00.000Z"),
			"WednesdayEndTime":   storage.StringValue("17:00:00.000Z"),
			"ThursdayStartTime":  storage.StringValue("09:00:00.000Z"),
			"ThursdayEndTime":    storage.StringValue("17:00:00.000Z"),
			"FridayStartTime":    storage.StringValue("09:00:00.000Z"),
			"FridayEndTime":      storage.StringValue("17:00:00.000Z"),
		},
	}
	org.Objects["BusinessHours"] = businessHours

	if len(holidays) == 0 {
		return org
	}
	storage.EnsureStandardObject(&org, "Holiday")
	holiday := org.Objects["Holiday"]
	for _, record := range holidays {
		holiday.Records[record.ID] = record
	}
	org.Objects["Holiday"] = holiday
	return org
}
