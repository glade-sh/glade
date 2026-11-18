//go:build cgo

package apexast

import (
	"strings"
	"testing"
)

func TestNormalizeInlineSOQLSetOptionsKeepsOffsets(t *testing.T) {
	source := "List<Account> rows = [SELECT Id FROM Account LIMIT 0 SET OPTIONS :options];"
	got := normalizeInlineSOQLFormula(source)
	want := "List<Account> rows = [SELECT Id FROM Account LIMIT 0 " + strings.Repeat(" ", len("SET OPTIONS :options")) + "];"
	if got != want {
		t.Fatalf("normalized = %q", got)
	}
	for _, unchanged := range []string{
		"x = [SELECT Id FROM Account WHERE Name = 'SET OPTIONS :o'];",
		"x = [SELECT Id FROM Account WHERE Id IN (SELECT AccountId FROM Contact) LIMIT 1];",
	} {
		if got := normalizeInlineSOQLFormula(unchanged); got != unchanged {
			t.Fatalf("%q normalized to %q", unchanged, got)
		}
	}
}
