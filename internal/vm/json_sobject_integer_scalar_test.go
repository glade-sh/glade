package vm

import "testing"

// Native rows: apextest/testdata/conformance/json_sobject_number_* (62.0, 67.0).
func TestJSONSObjectIntegerScalar(t *testing.T) {
	cases := []struct {
		display, token, text string
		ok                   bool
		boolean              bool
	}{
		{"BOOLEAN", "1", "", true, true},
		{"BOOLEAN", "-1", "", true, true},
		{"BOOLEAN", "0", "", true, false},
		{"BOOLEAN", "1.5", "", false, false},
		{"BOOLEAN", "0.0", "", false, false},
		{"DATE", "1730764800000", "2024-11-05", true, false},
		{"DATE", "-1730764800000", "1915-02-27", true, false},
		{"DATE", "0", "0000-01-01", true, false},
		{"DATE", "-1", "-0001-01-01", true, false},
		{"DATE", "292278994", "292278994-01-01", true, false},
		{"DATE", "292278995", "1970-01-04", true, false},
		{"DATE", "1730764800.5", "", false, false},
		{"DATETIME", "1730764812345", "2024-11-05T00:00:12.345Z", true, false},
		{"DATETIME", "1", "0001-01-01T00:00:00.000Z", true, false},
		{"DATETIME", "-2", "-0002-01-01T00:00:00.000Z", true, false},
		{"DATETIME", "1.7307648E12", "", false, false},
		{"DATETIME", "99999999999999999999", "", false, false},
		{"INTEGER", "1", "", false, false},
	}
	for _, c := range cases {
		value, ok := jsonSObjectIntegerScalar(c.display, c.token)
		if ok != c.ok {
			t.Fatalf("%s %s: accepted=%v, want %v", c.display, c.token, ok, c.ok)
		}
		if !ok {
			continue
		}
		if c.display == "BOOLEAN" {
			if value.Kind != ValueBool || value.Bool != c.boolean {
				t.Fatalf("%s %s: got %+v", c.display, c.token, value)
			}
		} else if got := value.Fields["value"].Text; got != c.text {
			t.Fatalf("%s %s: got %q, want %q", c.display, c.token, got, c.text)
		}
	}
}
