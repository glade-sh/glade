package typesys

import (
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
)

// The standard controller getRecord captures record the generic field read at
// API 62 and 67; it is a field, not a property or a concrete object schema.
func TestStandardSObjectIdentityField(t *testing.T) {
	symbol := requireStandardSymbol(t, StandardPlatformSymbols(), "SObject")
	var fields int
	for _, member := range symbol.Members {
		if member.Name != "Id" {
			continue
		}
		if member.Kind != apexast.DeclarationField || member.Type != "Id" {
			t.Fatalf("identity member = %#v", member)
		}
		fields++
	}
	if fields != 1 {
		t.Fatalf("identity field count = %d, want 1", fields)
	}
}
