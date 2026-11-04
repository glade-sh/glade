package apextest

import (
	"encoding/json"
	"os"
	"testing"
)

func case345GapAllowed(allowed map[string]bool, name string) bool { return allowed[name] }

// Case345Skip exposes the test-only guard to external-package conformance tests.
var Case345Skip = case345Skip

// Use the same exact-leaf inventory as the CI result validator. Unexpected
// parent, nested, or changed data-driven routes fail instead of skipping.
func case345Skip(t *testing.T) {
	t.Helper()
	body, err := os.ReadFile("../../scripts/ci-apex-native-gaps.json")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	if err := json.Unmarshal(body, &names); err != nil {
		t.Fatal(err)
	}
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	if !case345GapAllowed(allowed, t.Name()) {
		t.Fatalf("unregistered native gap route: %s", t.Name())
	}
	t.Skip("captured native observation has a known local gap")
}
