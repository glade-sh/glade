package vm

import (
	"strings"
	"testing"
)

func TestCompileBoundParentReturningRetainsAdjacentTokens(t *testing.T) {
	for _, scope := range []string{"ALL", "NAME"} {
		program, err := CompileAnonymousWithOptions("List<List<SObject>> rows=[FIND :term IN "+scope+" FIELDS RETURNING account(name),contact(lastname,account.Name)];", CompileOptions{APIVersion: "62.0"})
		if err != nil {
			t.Fatal(err)
		}
		if query := program.Instructions[0].Expr.Value; !strings.Contains(query, "account.Name") {
			t.Fatalf("adjacent source tokens separated in query: %s", query)
		}
	}
	program, err := CompileAnonymousWithOptions("List<List<SObject>> rows=[FIND :term IN ALL FIELDS RETURNING account(name),contact(lastname,account . Name)];", CompileOptions{APIVersion: "62.0"})
	if err != nil {
		t.Fatal(err)
	}
	if query := program.Instructions[0].Expr.Value; strings.Contains(query, "account.Name") {
		t.Fatalf("source-spaced path changed: %s", query)
	}
}
