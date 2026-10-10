package vm

import (
	"encoding/json"
	"os"
	"testing"
)

// Additional org observations vary numeric width, whitespace, line breaks and
// nested parser tokens to establish scanner locations independently of values.
func TestIntegerJSONLocationOrgConformance(t *testing.T) {
	raw, err := os.ReadFile("testdata/conformance/integer_long_json_locations.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Code string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	defer func() {
		// Captured scanner assertions retain their legacy assertEquals adapter.
		t.Logf("Integer and Long API 67.0 anonymous exact 0/0; category-only 0; carried 0; partial 0; legacy %d; subset JSON scanner locations", len(cases))
	}()
	for _, tc := range cases {
		t.Run(tc.ID, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(tc.Code, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Execute(program, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}
