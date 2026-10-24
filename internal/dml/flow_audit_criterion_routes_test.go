package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The native parent-update rows (internal/apextest/testdata/conformance/
// flow_decision_shapes/org.tsv, API 62.0 and 67.0, anonymous and istest,
// PARENT rows) observe a system-only $Record.LastModifiedDate only as an
// assignment value; that capture's only Update Records filter is a literal
// (Name NotEqualTo 'Closed'). An update criterion that reads the timestamp
// has no native row, so it keeps the base (a recorded revision) null resolution: the
// same result as an explicitly null source field. An explicit field value
// still compares.
func TestFlowAuditQualifiedAndLoweredRecordUpdateCriterionRoutes(t *testing.T) {
	const stamp = "2026-10-19T01:02:03.000Z"
	for _, api := range []string{"62.0", "67.0"} {
		for _, reference := range []string{"$Record.LastModifiedDate", "LastModifiedDate"} {
			for _, control := range []struct {
				name    string
				fields  map[string]storage.Value
				nulls   map[string]bool
				matches bool
			}{
				{name: "system_only", matches: false},
				{name: "explicit_null", nulls: map[string]bool{"LastModifiedDate": true}, matches: false},
				{name: "explicit_value", fields: map[string]storage.Value{"LastModifiedDate": storage.DateTimeValue(stamp)}, matches: true},
			} {
				t.Run(api+"/"+reference+"/"+control.name, func(t *testing.T) {
					org := testOrg()
					org.APIVersion = api
					engine := NewEngine(&org)
					frame := newFlowFrame()
					frame.saveDecisions = true
					source := storage.Record{Object: "Contact", Fields: control.fields, ExplicitNulls: control.nulls, System: storage.SystemFields{LastModifiedDate: stamp}}
					sourceDefinition := storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"LastModifiedDate": {APIName: "LastModifiedDate", Type: storage.FieldDateTime}}}
					targetDefinition := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Observed__c": {APIName: "Observed__c", Type: storage.FieldDateTime}}}
					target := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Observed__c": storage.DateTimeValue(stamp)}}
					seedFlowFrame(&engine, &source, frame)
					matches, ok := engine.evaluateFlowRecordUpdateCriterion(storage.WorkflowCriteriaItem{Field: "Observed__c", Operation: "equals", SourceField: reference}, target, targetDefinition, source, sourceDefinition, frame)
					if !ok || matches != control.matches {
						t.Fatalf("matches=%t supported=%t; want matches=%t", matches, ok, control.matches)
					}
				})
			}
		}
	}
}
