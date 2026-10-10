package sema

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestSOQLSelectionCompileSyntaxUsesAssignmentTarget(t *testing.T) {
	// C017 and native control E002 report the authored target, not a fixed name.
	checker := newQuerySemanticsChecker(typesys.Index{})
	for _, target := range []string{"r", "result"} {
		source := strings.Repeat("\n", 5) + "Object " + target + "=[SELECT Id FROM Account WHERE Id IN (SELECT AccountId FROM Contact ORDER BY LastName)].size();"
		diagnostics := checker.checkFile("SOQLSelectionProbe.cls", source)
		if len(diagnostics) != 1 || diagnostics[0].Message != "Unexpected token '"+target+"'." {
			t.Fatalf("target %s diagnostics = %#v", target, diagnostics)
		}
		if diagnostics[0].Range == nil || diagnostics[0].Range.Start.Line != 6 {
			t.Fatalf("target %s range = %#v, want native line 6", target, diagnostics[0].Range)
		}
	}
}
