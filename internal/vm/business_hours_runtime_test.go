package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

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
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "BusinessHours")
	businessHours := org.Objects["BusinessHours"]
	businessHours.Records["01m000000000001AAA"] = storage.Record{
		ID:     "01m000000000001AAA",
		Object: "BusinessHours",
		Fields: map[string]storage.Value{
			"Id":                 storage.IDValue("01m000000000001AAA"),
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

	storage.EnsureStandardObject(&org, "Holiday")
	holiday := org.Objects["Holiday"]
	holiday.Records["0Ho000000000001AAA"] = storage.Record{
		ID:     "0Ho000000000001AAA",
		Object: "Holiday",
		Fields: map[string]storage.Value{
			"Id":           storage.IDValue("0Ho000000000001AAA"),
			"Name":         storage.StringValue("Founders Day"),
			"ActivityDate": storage.DateValue("2026-06-15"),
			"IsAllDay":     storage.BooleanValue(true),
		},
	}
	org.Objects["Holiday"] = holiday

	machine.Org = &org
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
