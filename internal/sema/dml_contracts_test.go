package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/schema"
)

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

func TestDMLContractsRequireUpsertExternalIDOrIDLookupField(t *testing.T) {
	result := analyzeQueryProbe(t, `
public class Probe {
  public void run() {
    Account account = new Account();
    upsert account Account.Not_External__c;
  }
}
`, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Not_External__c", Type: "Text"}}}}})
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA034") {
		t.Fatalf("expected upsert field contract diagnostic: %#v", result.Diagnostics)
	}
}

func TestDMLContractsAllowUpsertExternalIDAndIDLookupFields(t *testing.T) {
	for name, field := range map[string]schema.Field{
		"external ID": {Name: "External_Key__c", Type: "Text", ExternalID: true},
		"id lookup":   {Name: "Lookup_Key__c", Type: "Text", IDLookup: true},
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeQueryProbe(t, `
public class Probe {
  public void run() {
    Account account = new Account();
    upsert account Account.`+field.Name+`;
  }
}
`, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{field}}}})
			if hasDiagnosticCode(result.Diagnostics, "GLADESEMA034") {
				t.Fatalf("valid upsert selector rejected: %#v", result.Diagnostics)
			}
		})
	}
}

func TestDMLContractsAllowUpsertExternalIDFromMatchingDuplicateSchemaObject(t *testing.T) {
	result := analyzeQueryProbe(t, `
public class Probe {
  public void run() {
    Account account = new Account();
    upsert account Account.External_Key__c;
  }
}
`, schema.Schema{Objects: []schema.Object{
		{Name: "Account", Fields: []schema.Field{{Name: "External_Key__c", Type: "Text"}}},
		{Name: "Account", Fields: []schema.Field{{Name: "External_Key__c", Type: "Text", ExternalID: true}}},
	}})
	if hasDiagnosticCode(result.Diagnostics, "GLADESEMA034") {
		t.Fatalf("external ID from a matching schema object was rejected: %#v", result.Diagnostics)
	}
}
