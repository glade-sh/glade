package vm

import (
	"fmt"
	"testing"
)

// Fixed modern instants keep factory identity and offset checks independent of
// the current clock. Historical offsets and display labels are separate checks.
func TestTimeZoneSouthernFixedOffsets(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"johannesburg-id", `System.assertEquals('Africa/Johannesburg', TimeZone.getTimeZone('Africa/Johannesburg').getID());`},
		{"johannesburg-january", `System.assertEquals(7200000, TimeZone.getTimeZone('Africa/Johannesburg').getOffset(january));`},
		{"johannesburg-july", `System.assertEquals(7200000, TimeZone.getTimeZone('Africa/Johannesburg').getOffset(july));`},
		{"brisbane-id", `System.assertEquals('Australia/Brisbane', TimeZone.getTimeZone('Australia/Brisbane').getID());`},
		{"brisbane-january", `System.assertEquals(36000000, TimeZone.getTimeZone('Australia/Brisbane').getOffset(january));`},
		{"brisbane-july", `System.assertEquals(36000000, TimeZone.getTimeZone('Australia/Brisbane').getOffset(july));`},
		{"gmt-control", `TimeZone zone = TimeZone.getTimeZone('GMT');
System.assertEquals('GMT', zone.getID());
System.assertEquals(0, zone.getOffset(january));
System.assertEquals(0, zone.getOffset(july));`},
		{"kolkata-control", `TimeZone zone = TimeZone.getTimeZone('Asia/Kolkata');
System.assertEquals('Asia/Kolkata', zone.getID());
System.assertEquals(19800000, zone.getOffset(january));
System.assertEquals(19800000, zone.getOffset(july));`},
		{"new-york-control", `TimeZone zone = TimeZone.getTimeZone('America/New_York');
System.assertEquals('America/New_York', zone.getID());
System.assertEquals(-18000000, zone.getOffset(january));
System.assertEquals(-14400000, zone.getOffset(july));`},
		{"lowercase-johannesburg-control", `TimeZone zone = TimeZone.getTimeZone('africa/johannesburg');
System.assertEquals('GMT', zone.getID());
System.assertEquals(0, zone.getOffset(january));
System.assertEquals(0, zone.getOffset(july));`},
		{"lowercase-brisbane-control", `TimeZone zone = TimeZone.getTimeZone('australia/brisbane');
System.assertEquals('GMT', zone.getID());
System.assertEquals(0, zone.getOffset(january));
System.assertEquals(0, zone.getOffset(july));`},
		{"unknown-zone-control", `TimeZone zone = TimeZone.getTimeZone('Missing/OwnedZone');
System.assertEquals('GMT', zone.getID());
System.assertEquals(0, zone.getOffset(january));
System.assertEquals(0, zone.getOffset(july));`},
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/%s", api, tc.name), func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(`
Datetime january = Datetime.newInstanceGmt(2026, 1, 15, 12, 0, 0);
Datetime july = Datetime.newInstanceGmt(2026, 7, 15, 12, 0, 0);
`+tc.body, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := New(nil).Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
