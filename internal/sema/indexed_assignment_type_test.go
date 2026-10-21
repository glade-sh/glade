package sema

import (
	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
	"testing"
)

func TestIndexedAssignmentExpressionHasSlotType(t *testing.T) {
	resolve := RuntimeExpressionTypeResolver(typesys.Index{Types: []typesys.TypeSymbol{{Name: "SlotFactory", Kind: apexast.DeclarationClass}}})
	for _, tc := range []struct{ expr, list, want string }{{"slots[0] = 7", "List<Integer>", "Integer"}, {"slots[0] = null", "List<Account>", "Account"}, {"slots.set(0, 7)", "List<Integer>", "void"}} {
		program, err := vm.CompileAnonymous("return " + tc.expr + ";")
		if err != nil {
			t.Fatal(err)
		}
		if got := resolve(program.Instructions[0].Expr, map[string]string{"slots": tc.list}, "SlotFactory"); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.expr, got, tc.want)
		}
	}
}
