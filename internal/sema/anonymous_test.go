package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestAnalyzeAnonymousUsesBodyContracts(t *testing.T) {
	result := AnalyzeAnonymous(typesys.Index{}, `String value = 'x'; insert value;`)
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA034") {
		t.Fatalf("expected anonymous DML diagnostic: %#v", result.Diagnostics)
	}
}

func TestAnalyzeAnonymousRejectsNonSalesforcePatternSurface(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "compile overload", source: `Pattern.compile('a', 2);`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := AnalyzeAnonymous(typesys.Index{}, tc.source)
			if len(result.Diagnostics) == 0 {
				t.Fatalf("expected non-Salesforce Pattern API to be rejected, got no diagnostics")
			}
		})
	}
}
