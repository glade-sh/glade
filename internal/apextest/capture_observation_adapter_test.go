package apextest

import "testing"

func TestCaptureObservationAdapter(t *testing.T) {
	cases := []struct {
		name, message, marker string
		valid                 bool
	}{
		{"exact", `System.AssertException: CAPTURE|case.one|{"value":1}`, "case.one", true},
		{"wrong marker", `CAPTURE|case.two|{"value":1}`, "case.one", false},
		{"duplicate marker", `CAPTURE|case.one|{} CAPTURE|case.one|{}`, "case.one", false},
		{"trailing content", `CAPTURE|case.one|{} trailing`, "case.one", false},
		{"nonobject", `CAPTURE|case.one|[]`, "case.one", false},
		{"malformed", `CAPTURE|case.one|{`, "case.one", false},
		{"missing", `ordinary assertion`, "case.one", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseCaptureObservation(tc.message, tc.marker)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}
