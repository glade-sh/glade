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

func TestCollectSOSLUsesSkipsBracketQueriesInStringsAndComments(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "single quoted string",
			source: "public class Searcher {\n" +
				"  String q = '[FIND {Acme} IN ALL FIELDS RETURNING Account(Id)]';\n" +
				"}\n",
		},
		{
			name: "double quoted string",
			source: "public class Searcher {\n" +
				"  String q = \"[FIND {Acme} IN ALL FIELDS RETURNING Account(Id)]\";\n" +
				"}\n",
		},
		{
			name: "line comment",
			source: "public class Searcher {\n" +
				"  // [FIND {Acme} IN ALL FIELDS RETURNING Account(Id)]\n" +
				"}\n",
		},
		{
			name: "block comment",
			source: "public class Searcher {\n" +
				"  /* [FIND {Acme} IN ALL FIELDS RETURNING Account(Id)] */\n" +
				"}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uses := codeintel.CollectSOSLUses("classes/Searcher.cls", tt.source)
			if len(uses) != 0 {
				t.Fatalf("CollectSOSLUses() = %#v, want no uses", uses)
			}
		})
	}
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
