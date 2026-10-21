package sema

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
)

func upsertFieldTokenQualificationSchema() schema.Schema {
	return schema.Schema{Objects: []schema.Object{{
		Name: "MeadowEntry__c",
		Fields: []schema.Field{
			{Name: "Key__c", Type: "Text", ExternalID: true},
			{Name: "Other__c", Type: "Text"},
			{Name: "Unique_Key__c", Type: "Text", Unique: true},
		},
	}}}
}

func TestDatabaseUpsertFieldTokenQualification(t *testing.T) {
	// The owned field_token_alias rows establish singular Field token identity
	// and scalar Database.upsert through a local token. Inline tokens must keep
	// the same owner and selector capability checks as the direct spelling.
	for _, tc := range []struct {
		name, object, selector string
	}{
		{"standard_singular", "Contact", "Contact.Field.Email"},
		{"standard_direct_control", "Contact", "Contact.Email"},
		{"standard_plural_control", "Contact", "Contact.Fields.Email"},
		{"custom_singular", "MeadowEntry__c", "MeadowEntry__c.Field.Key__c"},
		{"custom_direct_control", "MeadowEntry__c", "MeadowEntry__c.Key__c"},
		{"custom_plural_control", "MeadowEntry__c", "MeadowEntry__c.Fields.Key__c"},
	} {
		for _, variable := range []bool{false, true} {
			mode := "inline"
			body := fmt.Sprintf("Database.upsert(row, %s, true);", tc.selector)
			if variable {
				mode = "variable_control"
				body = fmt.Sprintf("Schema.SObjectField key = %s; Database.upsert(row, key, true);", tc.selector)
			}
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				result := analyzeQueryProbe(t, fmt.Sprintf(`
public class MeadowTokenUpsert {
    public void run(%s row) { %s }
}
`, tc.object, body), upsertFieldTokenQualificationSchema())
				if result.HasErrors() {
					t.Fatalf("eligible scalar field token rejected: %#v", result.Diagnostics)
				}
			})
		}
	}
}

func TestDatabaseUpsertFieldTokenQualificationBoundaries(t *testing.T) {
	// These controls preserve existing eligibility, overload and malformed-path
	// boundaries. They do not assert new native diagnostic text for those paths.
	for _, tc := range []struct {
		name, object, selector string
		wantError              bool
		wantContract           bool
	}{
		{"wrong_owner", "Contact", "Lead.Field.Email", true, true},
		{"standard_nonidentity", "Contact", "Contact.Field.Description", true, true},
		{"standard_name_not_identity", "Account", "Account.Field.Name", true, true},
		{"custom_nonexternal", "MeadowEntry__c", "MeadowEntry__c.Field.Other__c", true, true},
		{"custom_unique_not_identity", "MeadowEntry__c", "MeadowEntry__c.Field.Unique_Key__c", true, true},
		{"direct_nonexternal_control", "MeadowEntry__c", "MeadowEntry__c.Other__c", true, true},
		{"plural_nonexternal_control", "MeadowEntry__c", "MeadowEntry__c.Fields.Other__c", true, true},
		{"unknown_leaf", "MeadowEntry__c", "MeadowEntry__c.Field.Absent__c", true, false},
		{"deeper_alias", "MeadowEntry__c", "MeadowEntry__c.Field.Fields.Key__c", true, false},
		{"misspelled_alias", "MeadowEntry__c", "MeadowEntry__c.Filed.Key__c", true, false},
		{"generic_object_overload", "Object", "Contact.Field.Email", true, true},
		{"generic_sobject_defers_eligibility", "SObject", "Contact.Field.Description", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzeQueryProbe(t, fmt.Sprintf(`
public class MeadowTokenBoundary {
    public void run(%s row) { Database.upsert(row, %s, true); }
}
`, tc.object, tc.selector), upsertFieldTokenQualificationSchema())
			if got := result.HasErrors(); got != tc.wantError {
				t.Fatalf("error = %v, want %v: %#v", got, tc.wantError, result.Diagnostics)
			}
			if tc.wantContract && !hasDiagnosticCode(result.Diagnostics, "GLADESEMA034") {
				t.Fatalf("missing existing DML contract rejection: %#v", result.Diagnostics)
			}
		})
	}
}

func TestUpsertStatementRetainsSingularFieldBoundary(t *testing.T) {
	// The observed token expressions do not establish statement-selector syntax.
	// Preserve that separate local boundary while repairing Database arguments.
	result := analyzeQueryProbe(t, `
public class MeadowStatementBoundary {
    public void run(MeadowEntry__c row) { upsert row MeadowEntry__c.Field.Key__c; }
}
`, upsertFieldTokenQualificationSchema())
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA034") {
		t.Fatalf("statement selector boundary changed: %#v", result.Diagnostics)
	}
}
