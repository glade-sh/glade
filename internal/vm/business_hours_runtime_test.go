package vm

import (
	"errors"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

const testBusinessHoursID = "01m000000000001AAA"

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

func TestExecBusinessHoursUnsupportedHolidayShapesAreFenced(t *testing.T) {
	program, err := CompileAnonymous(`
BusinessHours.isWithin('01m000000000001AAA', Datetime.newInstanceGmt(2026, 6, 15, 16, 0, 0));
`)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		fields map[string]storage.Value
		want   string
	}{
		{
			name: "partial-day holiday",
			fields: map[string]storage.Value{
				"IsAllDay":           storage.BooleanValue(false),
				"StartTimeInMinutes": storage.IntegerValue(720),
				"EndTimeInMinutes":   storage.IntegerValue(780),
			},
			want: "BusinessHours partial-day holidays",
		},
		{
			name: "recurring holiday",
			fields: map[string]storage.Value{
				"IsAllDay":              storage.BooleanValue(true),
				"RecurrenceType":        storage.StringValue("RecursYearly"),
				"RecurrenceStartDate":   storage.DateValue("2026-06-15"),
				"RecurrenceEndDateOnly": storage.DateValue("2028-06-15"),
			},
			want: "BusinessHours recurring holiday expansion",
		},
		{
			name: "calendar-scoped holiday",
			fields: map[string]storage.Value{
				"IsAllDay":        storage.BooleanValue(true),
				"BusinessHoursId": storage.IDValue(testBusinessHoursID),
			},
			want: "BusinessHours service-calendar associations",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := map[string]storage.Value{
				"Id":           storage.IDValue("0Ho000000000001AAA"),
				"Name":         storage.StringValue(tt.name),
				"ActivityDate": storage.DateValue("2026-06-15"),
			}
			for field, value := range tt.fields {
				fields[field] = value
			}
			org := testBusinessHoursOrg(t, storage.Record{
				ID:     "0Ho000000000001AAA",
				Object: "Holiday",
				Fields: fields,
			})
			machine := New(nil)
			machine.Org = &org
			_, err := machine.Execute(program)
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || !strings.Contains(runtimeErr.Message, tt.want) {
				t.Fatalf("error = %#v, want UnsupportedFeature containing %q", err, tt.want)
			}
		})
	}
}

func TestExecBusinessHoursOperatingHoursHolidayLinksAreFenced(t *testing.T) {
	program, err := CompileAnonymous(`
BusinessHours.isWithin('01m000000000001AAA', Datetime.newInstanceGmt(2026, 6, 15, 16, 0, 0));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testBusinessHoursOrg(t)
	storage.EnsureStandardObject(&org, "OperatingHoursHoliday")
	links := org.Objects["OperatingHoursHoliday"]
	links.Records["0OH000000000001AAA"] = storage.Record{
		ID:     "0OH000000000001AAA",
		Object: "OperatingHoursHoliday",
		Fields: map[string]storage.Value{
			"Id":               storage.IDValue("0OH000000000001AAA"),
			"HolidayId":        storage.IDValue("0Ho000000000001AAA"),
			"OperatingHoursId": storage.IDValue("0OHr00000000001AAA"),
		},
	}
	org.Objects["OperatingHoursHoliday"] = links

	machine := New(nil)
	machine.Org = &org
	_, err = machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || !strings.Contains(runtimeErr.Message, "BusinessHours service-calendar associations") {
		t.Fatalf("error = %#v, want unsupported service-calendar associations", err)
	}
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
