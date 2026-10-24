package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These storage-level controls retain rejection on each validator route.
// Native JSON formula rows succeed and do not establish a first-error order.
func TestFieldRejectionInsertUpdateAndNullRoutes(t *testing.T) {
	definition := storage.ObjectDefinition{APIName: "OrderSample__c", Fields: map[string]storage.Field{}}
	for _, name := range []string{"Alpha__c", "Zeta__c"} {
		definition.Fields[name] = storage.Field{APIName: name, Type: storage.FieldCalculated, Formula: "1", Createable: storage.BoolFlag(false), Updateable: storage.BoolFlag(false)}
	}
	for _, route := range []string{"insertWriteability", "updateWriteability", "fieldValidation"} {
		for _, shape := range []string{"values", "mixed_null", "explicit_nulls"} {
			t.Run(route+"/"+shape, func(t *testing.T) {
				for attempt := 0; attempt < 64; attempt++ {
					record := storage.Record{Object: definition.APIName, Fields: map[string]storage.Value{"Zeta__c": storage.DecimalValue("2")}}
					if shape != "values" {
						record.ExplicitNulls = map[string]bool{"Alpha__c": true}
					} else {
						record.Fields["Alpha__c"] = storage.DecimalValue("1")
					}
					if shape == "explicit_nulls" {
						delete(record.Fields, "Zeta__c")
						record.ExplicitNulls["Zeta__c"] = true
					}
					var err error
					if route == "fieldValidation" {
						err = validateFields(definition, "", record)
					} else {
						err = validateFieldWriteability(definition, "", record, route == "insertWriteability")
					}
					detail, ok := err.(dmlError)
					if !ok || detail.code != "INVALID_FIELD_FOR_INSERT_UPDATE" || len(detail.fields) != 1 ||
						(detail.fields[0] != "Alpha__c" && detail.fields[0] != "Zeta__c") {
						t.Fatalf("expected a rejected supplied field, got %v", err)
					}
				}
			})
		}
	}
}
