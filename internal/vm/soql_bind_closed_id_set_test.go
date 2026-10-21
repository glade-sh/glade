package vm

import (
	"fmt"
	"testing"
)

// These controls preserve local behavior outside the captured countQuery
// boundary. They do not claim native behavior for the unobserved shapes.
func TestClosedIDSetBindPreservesUncapturedShapes(t *testing.T) {
	for _, tc := range []struct {
		name, api, route, declaration, block string
	}{
		{"query", "67.0", "query", "Set<Id> selected = new Set<Id>{'001000000000001'};", "if"},
		{"locator", "67.0", "getQueryLocator", "Set<Id> selected = new Set<Id>{'001000000000001'};", "if"},
		{"earlier API", "59.0", "countQuery", "Set<Id> selected = new Set<Id>{'001000000000001'};", "if"},
		{"intermediate API", "65.0", "countQuery", "Set<Id> selected = new Set<Id>{'001000000000001'};", "if"},
		{"later API", "69.0", "countQuery", "Set<Id> selected = new Set<Id>{'001000000000001'};", "if"},
		{"String set", "67.0", "countQuery", "Set<String> selected = new Set<String>{'001000000000001'};", "if"},
		{"null set", "67.0", "countQuery", "Set<Id> selected = null;", "if"},
		{"loop", "67.0", "countQuery", "Set<Id> selected = new Set<Id>{'001000000000001'};", "loop"},
		{"try", "67.0", "countQuery", "Set<Id> selected = new Set<Id>{'001000000000001'};", "try"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query := "SELECT Id FROM Account WHERE Id IN :selected"
			if tc.route == "countQuery" {
				query = "SELECT COUNT() FROM Account WHERE Id IN :selected"
			}
			body := tc.declaration + " query = '" + query + "';"
			switch tc.block {
			case "loop":
				// Resolve the bind while its declaration is in scope; do not rely on
				// a loop-local value leaking into the enclosing block.
				body = fmt.Sprintf("for (Integer i = 0; i < 1; i++) {%s query = '%s'; try { Database.%s(query); } catch (QueryException issue) { rejected = true; }}", tc.declaration, query, tc.route)
			case "try":
				body = "try {" + body + "} catch (Exception ignored) {}"
			default:
				body = "if (query == null) {" + body + "}"
			}
			var source string
			if tc.block == "loop" {
				source = fmt.Sprintf(`String query;
Boolean rejected = false;
%s
System.assertEquals(false, rejected, 'Uncaptured lookup boundary changed');`, body)
			} else {
				source = fmt.Sprintf(`String query;
%s
Boolean rejected = false;
try { Database.%s(query); } catch (QueryException issue) { rejected = true; }
				System.assertEquals(true, rejected, 'Uncaptured lookup boundary changed');`, body, tc.route)
			}
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: tc.api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestClosedIDSetBindPreservesEarlierString(t *testing.T) {
	source := `insert new Account(Name='Prior binding');
if (true) { String selected = 'Prior binding'; }
if (true) { Set<Id> selected = new Set<Id>{'001000000000001'}; }
List<Account> found = Database.query('SELECT Id FROM Account WHERE Name = :selected');
System.assertEquals(1, found.size());`
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "62.0"})
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
