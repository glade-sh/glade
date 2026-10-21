package vm

import (
	"fmt"
	"testing"
)

// Salesforce lists Europe/Oslo with standard UTC+1 and summer UTC+2 offsets:
// https://help.salesforce.com/s/articleView?id=sf.admin_supported_timezone.htm&language=en_US&type=5
// Datetime.format(pattern, timezone) renders the instant in the requested zone:
// https://developer.salesforce.com/docs/atlas.en-us.apexref.meta/apexref/apex_methods_system_datetime.htm
func TestDatetimeFormatExplicitNamedZone(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct {
			name, instant, pattern, zone, want string
		}{
			{"winter", "2025, 2, 8, 9, 17, 0", "dd.MM.yyyy HH:mm", "Europe/Oslo", "08.02.2025 10:17"},
			{"summer", "2025, 8, 8, 9, 17, 0", "dd.MM.yyyy HH:mm", "Europe/Oslo", "08.08.2025 11:17"},
			{"custom-pattern", "2025, 2, 8, 9, 17, 0", "yyyy/MM/dd HH:mm", "Europe/Oslo", "2025/02/08 10:17"},
			{"gmt-control", "2025, 8, 8, 9, 17, 0", "yyyy/MM/dd HH:mm", "GMT", "2025/08/08 09:17"},
			{"existing-zone-control", "2025, 8, 8, 9, 17, 0", "yyyy/MM/dd HH:mm", "Europe/Berlin", "2025/08/08 11:17"},
			{"unknown-zone-control", "2025, 8, 8, 9, 17, 0", "yyyy/MM/dd HH:mm", "Unknown/Zone", "2025/08/08 09:17"},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
Datetime stamp = Datetime.newInstanceGmt(%s);
System.assertEquals('%s', stamp.format('%s', '%s'));
`, tc.instant, tc.want, tc.pattern, tc.zone), CompileOptions{APIVersion: api})
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
