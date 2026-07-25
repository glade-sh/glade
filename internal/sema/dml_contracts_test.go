package sema

import "testing"

func TestDMLContractsRejectNonSObjectOperands(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"insert string":            `public class Probe { public void run() { String value = 'x'; insert value; } }`,
		"update string list":       `public class Probe { public void run() { List<String> values = new List<String>(); update values; } }`,
		"merge different sobjects": `public class Probe { public void run() { Account account = new Account(); Contact contact = new Contact(); merge account contact; } }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA034") {
				t.Fatalf("expected DML contract diagnostic: %#v", result.Diagnostics)
			}
		})
	}
}

func TestDMLContractsAllowSObjectAndSObjectListOperands(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `public class Probe { public void run() { Account account = new Account(); List<Account> accounts = new List<Account>{account}; insert account; update accounts; } }`,
	})
	if hasDiagnosticCode(result.Diagnostics, "GLADESEMA034") {
		t.Fatalf("unexpected DML contract diagnostic: %#v", result.Diagnostics)
	}
}
