package typesys

import (
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
)

// TestStandardPlatformSymbolOverlaysSystemShape closes the CB-08 system-shape
// rows by asserting the narrowly scoped standardPlatformSymbolOverlays entries
// merge into the generated System.StatusCode and System.Quiddity enums and the
// hand-written String class without disturbing existing members.
func TestStandardPlatformSymbolOverlaysSystemShape(t *testing.T) {
	symbols := StandardPlatformSymbols()

	statusCode := requireStandardSymbol(t, symbols, "StatusCode")
	if statusCode.Kind != apexast.DeclarationEnum {
		t.Fatalf("StatusCode kind = %q, want enum", statusCode.Kind)
	}
	for _, name := range []string{
		"PRINCIPAL_NOT_ASSIGNED",
		"PRINCIPAL_NOT_CONFIGURED",
		"PRINCIPAL_UNAUTHENTICATED",
		"COMMERCE_SEARCH_RULES_SYNC_FAILED",
	} {
		requireStandardPropertyStatic(t, statusCode, name, "StatusCode", true)
	}
	// Existing generated members must remain after the overlay merge.
	requireStandardPropertyStatic(t, statusCode, "APEX_FAILED", "StatusCode", true)

	quiddity := requireStandardSymbol(t, symbols, "Quiddity")
	if quiddity.Kind != apexast.DeclarationEnum {
		t.Fatalf("Quiddity kind = %q, want enum", quiddity.Kind)
	}
	requireStandardPropertyStatic(t, quiddity, "RUN_INTEGRATION_TESTS", "Quiddity", true)
	// Existing generated members must remain after the overlay merge.
	requireStandardPropertyStatic(t, quiddity, "QUEUEABLE", "Quiddity", true)

	stringClass := requireStandardSymbol(t, symbols, "String")
	// Existing hand-written overload must remain after the overlay merge.
	requireStandardMethod(t, stringClass, "template", []string{"Map<String,Object>"}, false)
	// Overlay adds the no-arg System.String.template() overload.
	requireStandardMethod(t, stringClass, "template", nil, false)
}