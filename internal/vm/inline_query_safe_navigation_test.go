package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

// Apex Developer Guide, Safe Navigation Operator, printed page 46:
// https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/salesforce_apex_developer_guide.pdf
// An immediate inline SOQL query followed by ?.Field evaluates to null when
// no objects are returned. Ordinary singleton assignment still throws:
// https://help.salesforce.com/s/articleView?id=000328824&language=en_US&type=1
// Owned inline_query_safe_empty observations confirm the read at API 62/67/68.
func TestExecInlineQuerySafeNavigation(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, testContext := range []bool{false, true} {
			context := "anonymous"
			if testContext {
				context = "test"
			}
			t.Run(api+"/"+context, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(`
String label = [SELECT Name FROM Account WHERE Id = null]?.Name;
System.assertEquals(null, label);
List<Account> rows = [SELECT Name FROM Account WHERE Id = null];
System.assertEquals(0, rows.size());
Boolean caught = false;
try {
    Account missing = [SELECT Name FROM Account WHERE Id = null];
} catch (System.QueryException failure) {
    caught = true;
    System.assertEquals('List has no rows for assignment to SObject', failure.getMessage());
}
System.assert(caught);
caught = false;
try {
    String missing = [SELECT Name FROM Account WHERE Id = null].Name;
} catch (System.QueryException failure) {
    caught = true;
    System.assertEquals('List has no rows for assignment to SObject', failure.getMessage());
}
System.assert(caught);
Account seed = new Account(Name = 'synthetic-one');
insert seed;
System.assertEquals('synthetic-one', [SELECT Name FROM Account WHERE Id = :seed.Id]?.Name);
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				if testContext {
					machine.EnableTestContext()
				}
				org := testDataOrg()
				machine.SetOrg(&org)
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestInlineQuerySafeNavigationNullFieldType(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	query := ir.Expr{Kind: ir.ExprSOQL, Value: "SELECT Name FROM Account WHERE Id = null"}
	value, err := machine.eval(ir.Expr{Kind: ir.ExprCall, Callee: "__safe_field:Name", Left: &query}, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueNull || value.Type != "String" || !isSafeNavigationNull(value) {
		t.Fatalf("safe query field = %#v; want String null with safe-navigation provenance", value)
	}
}
