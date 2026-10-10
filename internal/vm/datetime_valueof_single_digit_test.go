package vm

import "testing"

// The shared ISO parser stays separate from the String-overload clock parser.
func TestDatetimeValueOfSingleDigitClockParsingBoundary(t *testing.T) {
	if _, err := parseDatetimeText("2010-01-01 1:1:1"); err == nil {
		t.Fatal("shared parser accepted the valueOf-only clock form")
	}
	value, err := parseDatetimeValueOfText("2010-01-01 1:1:1")
	if err != nil || value.Format("2006-01-02 15:04:05") != "2010-01-01 01:01:01" {
		t.Fatalf("valueOf parser = %v, %v", value, err)
	}
	for _, input := range []string{
		"2010-5-4 1:1:1",
		"2010-05-4 01:01:01",
		"2010-5-04 01:01:01",
	} {
		if value, err := parseDatetimeValueOfText(input); err != nil || value.Format("2006-01-02 15:04:05") != "2010-05-04 01:01:01" {
			t.Fatalf("valueOf parser did not normalize %q: %v, %v", input, value, err)
		}
	}
	// Native K010/K049 (API62/67): suffixes are ignored and months carry.
	// This helper returns wall-clock fields; valueOf applies the user's zone.
	for _, tc := range []struct{ input, want string }{
		{"2010-01-01 1:1:1-7", "2010-01-01 01:01:01"},
		{"2010-13-01 1:1:1", "2011-01-01 01:01:01"},
	} {
		if value, err := parseDatetimeValueOfText(tc.input); err != nil || value.Format("2006-01-02 15:04:05") != tc.want {
			t.Fatalf("valueOf parser did not normalize %q: %v, %v", tc.input, value, err)
		}
	}
	for _, input := range []string{"2010-01-01 1:1"} {
		if _, err := parseDatetimeValueOfText(input); err == nil {
			t.Fatalf("valueOf parser accepted unproven format %q", input)
		}
	}
}
