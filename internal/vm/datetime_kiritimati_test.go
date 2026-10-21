package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Datetime.date() extracts the context user's local calendar date; dateGMT()
// extracts its UTC date (Apex Reference Guide, Datetime class). Salesforce's
// Supported Time Zones lists Pacific/Kiritimati as GMT+14. IANA tzdb 2025b's
// australasia table also records the 1994 date-line change from -10 to +14:
// https://developer.salesforce.com/docs/atlas.en-us.apexref.meta/apexref/apex_methods_system_datetime.htm
// https://help.salesforce.com/s/articleView?id=sf.admin_supported_timezone.htm&language=en_US&type=5
// https://data.iana.org/time-zones/tzdb-2025b/australasia
// These are specification-derived tests, not new Salesforce capture rows.
func TestDatetimeDateKiritimatiBoundaries(t *testing.T) {
	cases := []struct{ name, instant, localDate, gmtDate string }{
		{"historicalOffset", "1970-01-01T10:30:00.000Z", "1969-12-31", "1970-01-01"},
		{"beforeNewYear", "2024-12-31T09:59:59.999Z", "2024-12-31", "2024-12-31"},
		{"newYear", "2024-12-31T10:00:00.000Z", "2025-01-01", "2024-12-31"},
		{"leapDay", "2024-02-28T10:00:00.001Z", "2024-02-29", "2024-02-28"},
		{"summer", "2024-07-01T10:00:00.000Z", "2024-07-02", "2024-07-01"},
		{"beforeDateLineChange", "1994-12-31T09:59:59.999Z", "1994-12-30", "1994-12-31"},
		{"afterDateLineChange", "1994-12-31T10:00:00.000Z", "1995-01-01", "1994-12-31"},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range cases {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				source := fmt.Sprintf(`Datetime stamp=(Datetime)JSON.deserialize('"%s"',Datetime.class);
Long millis=stamp.getTime();
System.assertEquals('%s',String.valueOf(stamp.date()));
System.assertEquals('%s',String.valueOf(stamp.dateGMT()));
System.assertEquals(millis,stamp.getTime());`, tc.instant, tc.localDate, tc.gmtDate)
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				machine.SetCurrentUser(storage.Record{ID: "005-owned-date-user", Object: "User", Fields: map[string]storage.Value{"TimeZoneSidKey": storage.StringValue("Pacific/Kiritimati")}})
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestDatetimeDateTimezoneAndPrecisionControls(t *testing.T) {
	for _, zone := range []string{"GMT", "America/Los_Angeles", "Asia/Tokyo"} {
		t.Run(zone, func(t *testing.T) {
			source := `Datetime stamp=Datetime.newInstance(1735689600001L);
System.assertEquals(Date.newInstance(2025,1,1),stamp.dateGMT());
System.assertEquals(1735689600001L,stamp.getTime());
Datetime other=Datetime.newInstance(1735689600002L);
System.assertEquals(false,stamp==other);System.assert(stamp<other);
System.assertEquals(true,stamp.equals(other));
System.assertEquals(true,stamp==Datetime.newInstance(stamp.getTime()));`
			if zone == "America/Los_Angeles" {
				source += `System.assertEquals(Date.newInstance(2024,12,31),stamp.date());`
			} else {
				source += `System.assertEquals(Date.newInstance(2025,1,1),stamp.date());`
			}
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetCurrentUser(storage.Record{ID: "005-owned-date-control", Object: "User", Fields: map[string]storage.Value{"TimeZoneSidKey": storage.StringValue(zone)}})
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDatetimeDateKiritimatiEpoch(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`Datetime epoch=Datetime.newInstance(0);
System.assertEquals(Date.newInstance(1969,12,31),epoch.date());
System.assertEquals(Date.newInstance(1970,1,1),epoch.dateGMT());
System.assertEquals(0,epoch.getTime());`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetCurrentUser(storage.Record{ID: "005-owned-epoch-user", Object: "User", Fields: map[string]storage.Value{"TimeZoneSidKey": storage.StringValue("Pacific/Kiritimati")}})
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDatetimeNewInstanceKiritimatiLocalConstructors(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`Datetime midnight=Datetime.newInstance(2025,1,1);
System.assertEquals('2024-12-31 10:00:00.000',midnight.formatGmt('yyyy-MM-dd HH:mm:ss.SSS'));
System.assertEquals(1735639200000L,midnight.getTime());
System.assertEquals(Date.newInstance(2025,1,1),midnight.date());
Datetime parts=Datetime.newInstance(2025,1,1,3,4,5);
System.assertEquals('2024-12-31 13:04:05.000',parts.formatGmt('yyyy-MM-dd HH:mm:ss.SSS'));
Datetime withClock=Datetime.newInstance(Date.newInstance(2025,1,1),Time.newInstance(0,0,0,123));
System.assertEquals('2024-12-31 10:00:00.123',withClock.formatGmt('yyyy-MM-dd HH:mm:ss.SSS'));
System.assertEquals(Date.newInstance(2025,1,1),withClock.date());
Datetime gmt=Datetime.newInstanceGmt(2025,1,1,0,0,0);
System.assertEquals('2025-01-01 00:00:00.000',gmt.formatGmt('yyyy-MM-dd HH:mm:ss.SSS'));
System.assertEquals(1735689600123L,Datetime.newInstance(1735689600123L).getTime());`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetCurrentUser(storage.Record{ID: "005-kiritimati-constructor", Object: "User", Fields: map[string]storage.Value{"TimeZoneSidKey": storage.StringValue("Pacific/Kiritimati")}})
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
