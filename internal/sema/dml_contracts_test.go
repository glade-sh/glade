package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestDMLContractsRejectNonSObjectOperands(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"insert string":            `public class Probe { public void run() { String value = 'x'; insert value; } }`,
		"update string list":       `public class Probe { public void run() { List<String> values = new List<String>(); update values; } }`,
		"merge different sobjects": `public class Probe { public void run() { Account account = new Account(); Contact contact = new Contact(); merge account contact; } }`,
		"merge string list":        `public class Probe { public void run() { Account account = new Account(); List<String> values = new List<String>(); merge account values; } }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA034") {
				t.Fatalf("expected DML contract diagnostic: %#v", result.Diagnostics)
			}
		})
	}
}

func TestDMLContractsAllowMergeOfSObjectAndSameSObjectCollection(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `public class Probe { public void run() { Account master = new Account(); List<Account> duplicates = new List<Account>{new Account()}; merge master duplicates; } }`,
	})
	if hasDiagnosticCode(result.Diagnostics, "GLADESEMA034") {
		t.Fatalf("merge of an SObject master and same-SObject collection was rejected: %#v", result.Diagnostics)
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

func TestDMLContractsResolveUnqualifiedUpsertSelectorsAgainstNamespacedSchema(t *testing.T) {
	for _, test := range []struct {
		name           string
		localField     string
		field          schema.Field
		wantDiagnostic bool
	}{
		{
			name:       "external ID",
			localField: "External_Key__c",
			field: schema.Field{
				Name:       "pkg__External_Key__c",
				Type:       "Text",
				ExternalID: true,
			},
		},
		{
			name:           "non external field",
			localField:     "Not_External__c",
			field:          schema.Field{Name: "pkg__Not_External__c", Type: "Text"},
			wantDiagnostic: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := analyzeQueryProbeWithProject(t, `
public class Probe {
  public void run() {
    Thing__c row = new Thing__c();
    upsert row `+test.localField+`;
  }
}
`, typesys.ProjectInfo{Namespace: "pkg"}, schema.Schema{
				Objects: []schema.Object{{
					Name:   "pkg__Thing__c",
					Fields: []schema.Field{test.field},
				}},
			})
			got := hasDiagnosticCode(result.Diagnostics, "GLADESEMA034")
			if got != test.wantDiagnostic {
				t.Fatalf("GLADESEMA034 = %v, want %v: %#v", got, test.wantDiagnostic, result.Diagnostics)
			}
		})
	}
}
