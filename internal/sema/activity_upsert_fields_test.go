package sema

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Activity custom fields are shared by Task and Event. An explicitly declared
// external ID keeps its upsert capability on both record types.
// https://help.salesforce.com/s/articleView?id=How-do-I-create-custom-fields-for-tasks-or-events&language=en_US&type=1
func TestActivityExternalIDUpsertQualification(t *testing.T) {
	for _, api := range []string{"61.0", "62.0", "67.0", "68.0"} {
		for _, object := range []string{"Task", "Event"} {
			for _, operation := range []string{"statement", "database"} {
				for _, qualifier := range []string{"", "Fields."} {
					for _, external := range []bool{false, true} {
						name := fmt.Sprintf("%s/%s/%s/%s/external=%t", api, object, operation, qualifier, external)
						t.Run(name, func(t *testing.T) {
							selector := object + "." + qualifier + "Schedule_Key__c"
							body := "upsert row " + selector + ";"
							if operation == "database" {
								body = "Database.upsert(row, " + selector + ", false);"
							}
							source := fmt.Sprintf("public class ActivityKeyProbe { public void run(%s row) { %s } }", object, body)
							result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{SourceAPIVersion: api}, schema.Schema{Objects: []schema.Object{{
								Name:   "Activity",
								Fields: []schema.Field{{Name: "Schedule_Key__c", Type: "Text", ExternalID: external, Unique: true}},
							}}})
							if got := hasDiagnosticCode(result.Diagnostics, "GLADESEMA034"); got == external {
								t.Fatalf("upsert rejection = %t, external ID = %t: %#v", got, external, result.Diagnostics)
							}
							if external && result.HasErrors() {
								t.Fatalf("shared external ID token rejected: %#v", result.Diagnostics)
							}
						})
					}
				}
			}
		}
	}
}

func TestActivityUpsertSelectorKeepsOwnershipAndCapabilityBoundaries(t *testing.T) {
	analyzer := &Analyzer{queryDeclaredObjects: []schema.Object{{
		Name: "Activity",
		Fields: []schema.Field{
			{Name: "Schedule_Key__c", Type: "Text", ExternalID: true},
			{Name: "Ordinary_Key__c", Type: "Text"},
			{Name: "Unique_Key__c", Type: "Text", Unique: true},
			{Name: "Subject", Type: "Text", ExternalID: true},
		},
	}}}
	for _, test := range []struct{ object, selector string }{
		{"Task", "Event.Fields.Schedule_Key__c"},
		{"Event", "Task.Fields.Schedule_Key__c"},
		{"Account", "Account.Fields.Schedule_Key__c"},
		{"Task", "Task.Fields.Ordinary_Key__c"},
		{"Event", "Event.Fields.Unique_Key__c"},
		{"Task", "Task.Fields.Fields.Schedule_Key__c"},
		{"Event", "Event.Fields.Schedule_Key__c.Extra"},
		{"Task", "Task.Fields.Subject"},
	} {
		t.Run(test.object+"/"+test.selector, func(t *testing.T) {
			if analyzer.semaUpsertSelectorAllowed(test.object, test.selector) {
				t.Fatalf("invalid selector accepted: %s for %s", test.selector, test.object)
			}
		})
	}
}
