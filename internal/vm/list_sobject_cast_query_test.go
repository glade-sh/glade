package vm

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

func TestCompileListSObjectCastQueryOperand(t *testing.T) {
	for _, tc := range []struct {
		name, expression string
		direct           bool
	}{
		{"dynamic", "(Account)Database.query(q)", true},
		{"dynamic-user-mode", "(Account)Database.query(q, AccessLevel.USER_MODE)", true},
		{"with-binds", "(Account)Database.queryWithBinds(q, binds, AccessLevel.SYSTEM_MODE)", true},
		{"locator", "(SObject)Database.getQueryLocator(q)", true},
		{"locator-with-binds", "(SObject)Database.getQueryLocatorWithBinds(q, binds, AccessLevel.SYSTEM_MODE)", true},
		{"inline", "(Account)[SELECT Id FROM Account]", true},
		{"parenthesized-query", "(Account)(Database.query(q))", true},
		{"parenthesized-receiver", "(Account)(Database).query(q)", true},
		{"stored", "(Account)rows", false},
		{"field", "(Account)holder.rows", false},
		{"object-bridge", "(Account)(Object)Database.query(q)", false},
		{"stored-object-bridge", "(Account)(Object)rows", false},
		{"constructor-bridge", "(Account)(Object)new List<Account>()", false},
		{"query-result-call", "(Account)Database.query(q).clone()", false},
		{"other-provider", "(Account)Provider.query(q)", false},
		{"sosl", "(Account)[FIND 'probe' RETURNING Account(Id)]", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous("Object result = " + tc.expression + ";")
			if err != nil {
				t.Fatal(err)
			}
			if len(program.Instructions) != 1 {
				t.Fatalf("instructions=%d, want one cast declaration", len(program.Instructions))
			}
			expr := program.Instructions[0].Expr
			if expr.Callee != "__cast:Account" && expr.Callee != "__cast:SObject" {
				t.Fatalf("not a cast: %#v", expr)
			}
			if expr.CastQueryOperand != tc.direct {
				t.Fatalf("direct query operand=%t, want=%t", expr.CastQueryOperand, tc.direct)
			}
			encoded, err := json.Marshal(program)
			if err != nil {
				t.Fatal(err)
			}
			var restored ir.Program
			if err := json.Unmarshal(encoded, &restored); err != nil {
				t.Fatal(err)
			}
			if restored.Instructions[0].Expr.CastQueryOperand != tc.direct {
				t.Fatal("serialized IR lost the cast operand form")
			}
		})
	}
}

// Compare cast outcomes with plain assignment rather than introduce a native
// answer for query-cast shapes whose capture is still pending.
func TestListSObjectDirectQueryCastMatchesAssignment(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, target := range []string{"Account", "SObject"} {
			for _, query := range []struct{ name, expression string }{
				{"dynamic", "Database.query(q)"},
				{"inline", "[SELECT Id FROM Account WHERE Id IN :selected ORDER BY Id]"},
				{"with-binds", "Database.queryWithBinds(q, binds, AccessLevel.SYSTEM_MODE)"},
				{"dynamic-user-mode", "Database.query(q, AccessLevel.USER_MODE)"},
			} {
				for _, rows := range []int{0, 1, 2} {
					for _, route := range []string{"anonymous", "isTest"} {
						t.Run(fmt.Sprintf("%s/%s/%s/rows%d/%s", api, target, query.name, rows, route), func(t *testing.T) {
							source := fmt.Sprintf(`
List<Account> made = new List<Account>{new Account(Name='Query cast first'), new Account(Name='Query cast second')};
insert made;
List<Id> selected = new List<Id>();
for (Integer i=0; i<%d; i++) { selected.add(made[i].Id); }
String q = 'SELECT Id FROM Account WHERE Id IN :selected ORDER BY Id';
Map<String,Object> binds = new Map<String,Object>{'selected'=>selected};
String assigned;
try {
    %s record = %s;
    assigned = 'ROW|' + String.valueOf(record.get('Id'));
} catch (Exception e) {
    assigned = e.getTypeName() + '|' + e.getMessage();
}
String casted;
try {
    %s record = (%s)%s;
    casted = 'ROW|' + String.valueOf(record.get('Id'));
} catch (Exception e) {
    casted = e.getTypeName() + '|' + e.getMessage();
}
System.assertEquals(assigned, casted);
`, rows, target, query.expression, target, target, query.expression)
							program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							machine := New(nil)
							org := testDataOrg()
							org.APIVersion = api
							machine.SetOrg(&org)
							if route == "isTest" {
								machine.EnableTestContext()
								_, err = machine.ExecuteInClass(program, "DirectQueryCastProbe")
							} else {
								_, err = machine.Execute(program)
							}
							if err != nil {
								t.Fatal(err)
							}
						})
					}
				}
			}
		}
	}
}
