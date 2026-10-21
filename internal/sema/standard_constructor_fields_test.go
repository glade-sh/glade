package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// Standard objects come from a describe export of one clean org. Standard
// fields that export omits (for example Account.Tradestyle) must still be
// accepted as constructor initializers, in any letter case.
func TestStandardObjectConstructorAcceptsStandardFieldMissingFromCatalog(t *testing.T) {
	index := typesys.Index{Project: typesys.ProjectInfo{SourceAPIVersion: "67.0"}}
	for _, source := range []string{
		"Account a = new Account(Name = 'x', Tradestyle = 'y');",
		"Account a = (Account) new Account(Name = 'x', TradeStyle = 'y');",
		"Account a = new Account(Name = 'x', TRADESTYLE = 'y');",
	} {
		if result := AnalyzeAnonymous(index, source, "67.0"); result.HasErrors() {
			t.Errorf("%q: %v", source, result.Diagnostics)
		}
	}
}
