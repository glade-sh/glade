package sema

import (
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
	"strings"
	"testing"
)

func TestSOQLBindExpressionVariablesAndIndexTypes(t *testing.T) {
	checker := newQuerySemanticsChecker(typesys.Index{})
	bindings := map[string]string{"a": "List<Account>", "ids": "List<Id>", "i": "Integer", "m": "Map<String,Id>"}
	for _, test := range []struct{ operand, message string }{
		{"(undefinedVar)", "Variable does not exist: undefinedVar"},
		{"a[undefinedVar].Id", "Variable does not exist: undefinedVar"},
		{"a['x'].Id", "List index must be of type Integer: String"},
		{"m['first']", "Expression must be a list type: Map<String,Id>"},
		{"a[i+1].Id", ""}, {"ids[0]", ""}, {"a[-1].Id", ""}, {"m.get('x')", ""},
	} {
		queryText := "SELECT Id FROM Account WHERE Id=:" + test.operand
		query, err := soql.Parse(queryText)
		if err != nil {
			t.Fatal(err)
		}
		ctx := queryTextContext{queryText: queryText, locator: newSemaSourceLocator(queryText)}
		diagnostics := checker.soqlBindExpressionDiagnostics(query, ctx, bindings)
		if test.message == "" {
			if len(diagnostics) != 0 {
				t.Fatalf("%s: %v", test.operand, diagnostics)
			}
			continue
		}
		if len(diagnostics) != 1 || diagnostics[0].Message != test.message {
			t.Fatalf("%s: %v", test.operand, diagnostics)
		}
	}
}

func TestSOQLSafeNavigationBindMemberType(t *testing.T) {
	expression, err := vm.CompileSOQLBindExpression("holder?.NumberOfEmployees")
	if err != nil {
		t.Fatal(err)
	}
	checker := newQuerySemanticsChecker(typesys.Index{})
	got := checker.soqlBindExpressionType(expression, map[string]string{"holder": "Account"})
	if got != "Integer" {
		t.Fatalf("safe-navigation bind member type = %q, want Integer", got)
	}
	expression, err = vm.CompileSOQLBindExpression("names?.get(0)")
	if err != nil {
		t.Fatal(err)
	}
	got = checker.soqlBindExpressionType(expression, map[string]string{"names": "List<String>"})
	if got != "String" {
		t.Fatalf("safe-navigation bind method type = %q, want String", got)
	}
}

func TestSOQLBindExpressionIntegerStringRejection(t *testing.T) {
	checker := newQuerySemanticsChecker(typesys.Index{})
	text := "SELECT Id FROM Account WHERE Name=:(1+1)"
	query, err := soql.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics := checker.soqlBindExpressionDiagnostics(query, queryTextContext{queryText: text, locator: newSemaSourceLocator(text)}, nil)
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, "Invalid bind expression type of Integer for column of type String") {
		t.Fatalf("%v", diagnostics)
	}
}
