package dml

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// A required Contact custom field is mirrored onto Account as a __pc field.
// Native rows: apextest/testdata/conformance/person_account_pc_required_formula
// (biz_insert, pa_no_pc_insert, pa_pc_insert). The mirror is required for
// Person Account inserts only; Business Account inserts are unaffected.
func TestPersonAccountMirroredRequiredFieldDoesNotBlockBusinessAccountInsert(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := storage.NewOrgState()
			org.APIVersion = api
			storage.EnsureDeterministicPlatformData(&org)
			contact := org.Objects["Contact"]
			contact.Definition = contact.Definition.Clone()
			contact.Definition.Fields["Must__c"] = storage.Field{APIName: "Must__c", Type: storage.FieldString, Length: 40, Required: true}
			org.Objects["Contact"] = contact
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			engine := NewEngine(&org)
			for _, tc := range []struct {
				name    string
				fields  map[string]storage.Value
				success bool
				code    string
				message string
				missing []string
			}{
				{name: "biz_insert", fields: map[string]storage.Value{"Name": storage.StringValue("Biz")}, success: true},
				{name: "pa_no_pc_insert", fields: map[string]storage.Value{"LastName": storage.StringValue("P1")},
					code: "REQUIRED_FIELD_MISSING", message: "Required fields are missing: [Must__pc]", missing: []string{"Must__pc"}},
				{name: "pa_pc_insert", fields: map[string]storage.Value{"LastName": storage.StringValue("P2"), "Must__pc": storage.StringValue("x")}, success: true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					results := engine.Insert([]storage.Record{{Object: "Account", Fields: tc.fields}})
					if len(results) != 1 || results[0].Success != tc.success {
						t.Fatalf("insert: %+v", results)
					}
					if tc.success {
						return
					}
					if results[0].StatusCode != tc.code || results[0].Error != tc.message || !reflect.DeepEqual(results[0].Fields, tc.missing) {
						t.Fatalf("native %s %q %v; Glade %+v", tc.code, tc.message, tc.missing, results[0])
					}
				})
			}
		})
	}
}
