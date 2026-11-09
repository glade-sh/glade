package compile_test

import (
	"testing"
)

// Exercise the captured API 68 compiler rows without requiring a browser.
// The full L23 conformance test also covers exact diagnostics and DOM routes.
func TestL23API68CompileOutcomes(t *testing.T) {
	var data struct {
		Cases    []l23Case `json:"cases"`
		Controls []l23Case `json:"controls"`
	}
	l23Read(t, "testdata/l23_salesforce.json", &data)
	var cases []l23Case
	for _, c := range append(data.Cases, data.Controls...) {
		if c.Kind == "compile" {
			cases = append(cases, c)
		}
	}
	if len(cases) != 101 {
		t.Fatalf("expected 101 native compiler rows, got %d", len(cases))
	}
	results := l23CompileCases(t, "68.0", cases)
	for i, c := range cases {
		got := "COMPILE_OK"
		if results[i].Err != nil {
			got = "COMPILE_ERROR"
		}
		if want := c.Expected["68.0"]; got != want {
			t.Errorf("%s: got %s, want %s: %v", c.ID, got, want, results[i].Err)
		}
	}
}
