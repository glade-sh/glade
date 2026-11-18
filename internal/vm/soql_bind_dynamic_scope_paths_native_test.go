package vm

import (
	"strings"
	"testing"
)

// NATIVE a dotted bind in query, countQuery and getQueryLocator never names a variable
// internal/apextest/testdata/conformance/dynamic_bind_scope_paths/org.tsv anonymous 62.0 67.0
// NATIVE a call with an argument in a bind is an unexpected '(' token
// internal/apextest/testdata/conformance/dynamic_bind_scope_paths/org.tsv anonymous 62.0 67.0
func TestDynamicSOQLScopeBindPathsNative(t *testing.T) {
	const q = "SELECT Id FROM Account WHERE Name = "
	const cq = "SELECT COUNT() FROM Account WHERE Name = "
	for _, test := range []struct {
		name, decl, call, want string
	}{
		{"declared record", "Account acc = new Account(Name='x');", "Database.query('" + q + ":acc.Name')", "Variable does not exist: acc.Name"},
		{"declared record count", "Account acc = new Account(Name='x');", "Database.countQuery('" + cq + ":acc.Name')", "Variable does not exist: acc.Name"},
		{"declared record locator", "Account acc = new Account(Name='x');", "Database.getQueryLocator('" + q + ":acc.Name')", "Variable does not exist: acc.Name"},
		{"null root", "Account acc;", "Database.query('" + q + ":acc.Name')", "Variable does not exist: acc.Name"},
		{"unknown member", "Account acc = new Account(Name='x');", "Database.query('" + q + ":acc.Bogus__c')", "Variable does not exist: acc.Bogus__c"},
		{"nested path", "Contact c = new Contact(LastName='x');", "Database.query('" + q + ":c.Account.Name')", "Variable does not exist: c.Account.Name"},
		{"list member", "List<String> xs = new List<String>{'x'};", "Database.query('" + q + ":xs.size')", "Variable does not exist: xs.size"},
		{"map get", "Map<String,String> m = new Map<String,String>{'k'=>'x'};", "Database.query('" + q + ":m.get(\\'k\\')')", "unexpected token: '('"},
		{"map get count", "Map<String,String> m = new Map<String,String>{'k'=>'x'};", "Database.countQuery('" + cq + ":m.get(\\'k\\')')", "unexpected token: '('"},
		{"map get locator", "Map<String,String> m = new Map<String,String>{'k'=>'x'};", "Database.getQueryLocator('" + q + ":m.get(\\'k\\')')", "unexpected token: '('"},
	} {
		t.Run(test.name, func(t *testing.T) {
			program, err := CompileAnonymous("String got = 'none'; try { " + test.decl + " Object o = " + test.call + "; } catch (QueryException e) { got = e.getMessage(); } System.assertEquals('" + strings.ReplaceAll(test.want, "'", "\\'") + "', got);")
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

func TestDynamicSOQLScopeBindPathsKeepAcceptedForms(t *testing.T) {
	machine := New(nil)
	for _, query := range []string{
		"SELECT Id FROM Account WHERE Id IN :accountsById.values()",
		"SELECT Id FROM Account WHERE Name = :plain",
	} {
		if err := machine.validateDynamicSOQLBindSyntaxMode(query, false, true); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
}
