package vm

import (
	"strings"
	"testing"
)

func TestSOSLUnimplementedMatcherStillRejectsWithoutTestResults(t *testing.T) {
	machine := New(nil)
	_, _, err := machine.parseSOSLQuery("FIND 'Alpha?' RETURNING Contact(Id)")
	if err == nil || !strings.Contains(err.Error(), "SOSL fuzzy search operator ?") {
		t.Fatalf("unimplemented matcher accepted before record lookup: %v", err)
	}
}
