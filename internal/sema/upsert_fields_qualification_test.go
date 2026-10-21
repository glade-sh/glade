package sema

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
)

// Salesforce documents Object.Fields.Field as an upsert field token:
// https://trailhead.salesforce.com/content/learn/modules/apex_database/apex_database_dml
func TestDMLUpsertFieldsQualification(t *testing.T) {
	for _, tc := range []struct {
		name, object, selector string
		wantDiagnostic         bool
	}{
		{"standard-direct-control", "Lead", "Lead.Email", false},
		{"standard-fields", "Lead", "Lead.Fields.Email", false},
		{"custom-direct-control", "Gift__c", "Gift__c.Link_Key__c", false},
		{"custom-fields", "Gift__c", "Gift__c.Fields.Link_Key__c", false},
		{"other-object-control", "Lead", "Contact.Fields.Email", true},
		{"nonexternal-direct-control", "Gift__c", "Gift__c.Other_Key__c", true},
		{"nonexternal-fields-control", "Gift__c", "Gift__c.Fields.Other_Key__c", true},
	} {
		for _, operation := range []string{"statement", "database"} {
			t.Run(tc.name+"/"+operation, func(t *testing.T) {
				body := fmt.Sprintf("upsert row %s;", tc.selector)
				if operation == "database" {
					body = fmt.Sprintf("Database.upsert(row, %s, false);", tc.selector)
				}
				result := analyzeQueryProbe(t, fmt.Sprintf(`
public class MeadowUpsert {
    public void run(%s row) { %s }
}
`, tc.object, body), schema.Schema{Objects: []schema.Object{{
					Name: "Gift__c",
					Fields: []schema.Field{
						{Name: "Link_Key__c", Type: "Text", ExternalID: true},
						{Name: "Other_Key__c", Type: "Text"},
					},
				}}})
				if got := hasDiagnosticCode(result.Diagnostics, "GLADESEMA034"); got != tc.wantDiagnostic {
					t.Fatalf("upsert contract diagnostic = %v, want %v: %#v", got, tc.wantDiagnostic, result.Diagnostics)
				}
				if !tc.wantDiagnostic && hasErrorOtherThan(result.Diagnostics, "") {
					t.Fatalf("documented upsert token rejected: %#v", result.Diagnostics)
				}
			})
		}
	}
}

func TestDMLUpsertSelectorRejectsUnrecognizedNestedQualification(t *testing.T) {
	// Preserve the selector validator's existing rejection of deeper paths;
	// the documented Fields token is a single qualification level.
	analyzer := &Analyzer{queryDeclaredObjects: []schema.Object{{
		Name:   "Gift__c",
		Fields: []schema.Field{{Name: "Link_Key__c", Type: "Text", ExternalID: true}},
	}}}
	for _, selector := range []string{
		"Gift__c.Fields.Fields.Link_Key__c",
		"Gift__c.Other.Fields.Link_Key__c",
		"Gift__c.Fields.Link_Key__c.Extra",
	} {
		t.Run(selector, func(t *testing.T) {
			if analyzer.semaUpsertSelectorAllowed("Gift__c", selector) {
				t.Fatalf("unexpected selector accepted: %s", selector)
			}
		})
	}
}
