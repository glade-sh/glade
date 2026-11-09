package sema

import "testing"

// Generic local declarations written without whitespace between the closing
// angle bracket and the variable name are valid Apex and must bind the local
// to the generic type, exactly like the spaced form.
func TestNoSpaceGenericLocalDeclarationsBindWholeType(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"list":            "List<Account>recs = new List<Account>();\n Database.SaveResult[] r = Database.insert(recs,false);\n Integer n = r.size();",
		"list spaced":     "List<Account> recs = new List<Account>();\n Database.SaveResult[] r = Database.insert(recs,false);\n Integer n = r.size();",
		"map":             "Map<Id,Account>m = new Map<Id,Account>();\n Database.SaveResult[] r = Database.insert(m.values(),false);",
		"set":             "Set<String>s = new Set<String>();\n Boolean b = s.contains('a');",
		"nested map":      "Map<Id,List<Account>>nn = new Map<Id,List<Account>>();\n List<Account> x = nn.get(null);",
		"nested list":     "List<List<Account>>deep = new List<List<Account>>();\n List<Account> y = deep[0];",
		"array":           "Account[]a = new Account[]{};\n Database.SaveResult[] r = Database.insert(a,false);",
		"after statement": "Integer i = 0; List<Account>recs = new List<Account>();\n Database.SaveResult[] r = Database.insert(recs,false);",
	} {
		result := analyzeDeclarationProject(t, map[string]string{
			"NoSpace.cls": "public class NoSpace {\n public static void m() {\n " + body + "\n }\n}\n",
		})
		for _, diag := range result.Diagnostics {
			t.Errorf("%s: unexpected diagnostic %s %s", name, diag.Code, diag.Message)
		}
	}
}
