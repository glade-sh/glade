package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These are release-behavior controls, not new Salesforce observations. The
// captured division rows do not authorize changing blank-number +, - or *.
func TestCustomValidationBlankArithmeticRetainsReleaseBehavior(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, formula := range []string{
			"NumberValue__c + 1 = 1", "1 + NumberValue__c = 1",
			"NumberValue__c - 1 = -1", "1 - NumberValue__c = 1",
			"NumberValue__c * 2 = 0", "2 * NumberValue__c = 0",
		} {
			for _, route := range []string{"insert", "update"} {
				t.Run(api+"/"+formula+"/"+route, func(t *testing.T) {
					org := testOrg()
					org.APIVersion = api
					account := org.Objects["Account"]
					account.Definition.Fields["NumberValue__c"] = storage.Field{APIName: "NumberValue__c", Type: storage.FieldDecimal}
					org.Objects["Account"] = account
					engine := NewEngine(&org)
					record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Blank arithmetic control")}}
					if route == "update" {
						seed := engine.Insert([]storage.Record{record})
						if len(seed) != 1 || !seed[0].Success {
							t.Fatalf("seed: %+v", seed)
						}
						record.ID = seed[0].ID
					}
					account = org.Objects["Account"]
					account.Definition.ValidationRules = []storage.ValidationRule{{Name: "BlankArithmetic", Active: true, ErrorConditionFormula: formula, ErrorMessage: "Blank arithmetic rejected"}}
					org.Objects["Account"] = account
					var results []Result
					if route == "insert" {
						results = engine.Insert([]storage.Record{record})
					} else {
						results = engine.Update([]storage.Record{record})
					}
					if len(results) != 1 || results[0].Success || results[0].StatusCode != "FIELD_CUSTOM_VALIDATION_EXCEPTION" {
						t.Fatalf("release behavior rejects blank arithmetic: %+v", results)
					}
				})
			}
		}
	}
}
