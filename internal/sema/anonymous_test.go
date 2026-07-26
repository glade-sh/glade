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
