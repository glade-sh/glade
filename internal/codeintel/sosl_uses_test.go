package codeintel_test

import (
	"testing"

	"github.com/glade-sh/glade/internal/codeintel"
)

func TestCollectSOSLUsesFromStaticBracketQuery(t *testing.T) {
	source := "public class Searcher {\n" +
		"  void run() {\n" +
		"    List<List<SObject>> rows = [FIND 'Acme' IN ALL FIELDS RETURNING Account(Id, Name), Contact(Id)];\n" +
		"  }\n" +
		"}\n"

	uses := codeintel.CollectSOSLUses("classes/Searcher.cls", source)
	assertUse(t, uses, codeintel.SObjectID("Account"), codeintel.UseQuery, "Account", 3, 69)
	assertUse(t, uses, codeintel.SObjectFieldID("Account", "Id"), codeintel.UseQuery, "Id", 3, 77)
	assertUse(t, uses, codeintel.SObjectFieldID("Account", "Name"), codeintel.UseQuery, "Name", 3, 81)
	assertUse(t, uses, codeintel.SObjectID("Contact"), codeintel.UseQuery, "Contact", 3, 88)
	assertUse(t, uses, codeintel.SObjectFieldID("Contact", "Id"), codeintel.UseQuery, "Id", 3, 96)
}

func assertUse(t *testing.T, uses []codeintel.Use, id codeintel.SymbolID, kind codeintel.UseKind, name string, line, column int) {
	t.Helper()
	for _, use := range uses {
		if use.SymbolID != id || use.Kind != kind || use.Name != name {
			continue
		}
		if !use.Resolved {
			t.Fatalf("use %s = %#v", id, use)
		}
		if use.Range.Start.Line != line || use.Range.Start.Column != column {
			t.Fatalf("use %s range = %#v, want %d:%d", id, use.Range, line, column)
		}
		return
	}
	t.Fatalf("missing use %s in %#v", id, uses)
}
