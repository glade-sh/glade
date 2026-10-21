package dml

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Owned scale-zero Number observations reject values that round past the
// configured magnitude while preserving accepted fractional values unchanged.
func TestNumericMagnitudeRejectsOversizedNumberFields(t *testing.T) {
	for _, test := range []struct {
		name, field string
		value       storage.Value
	}{
		{"small fractional overflow", "Small__c", storage.DecimalValue("99.9999999")},
		{"small rounding overflow", "Small__c", storage.DecimalValue("99.5")},
		{"small negative rounding overflow", "Small__c", storage.DecimalValue("-99.5")},
		{"small positive boundary overflow", "Small__c", storage.IntegerValue(100)},
		{"small positive", "Small__c", storage.IntegerValue(1000)},
		{"small negative", "Small__c", storage.DecimalValue("-1000")},
		{"medium positive", "Medium__c", storage.DecimalValue("1000000")},
		{"medium negative", "Medium__c", storage.IntegerValue(-1000000)},
	} {
		t.Run(test.name, func(t *testing.T) {
			org := numericMagnitudeOrg()
			engine := NewEngine(&org)
			results := engine.Insert([]storage.Record{numericMagnitudeRecord(test.field, test.value)})
			if len(results) != 1 || results[0].Success || results[0].StatusCode != "NUMBER_OUTSIDE_VALID_RANGE" {
				t.Fatalf("oversized number result = %#v", results)
			}
			if !reflect.DeepEqual(results[0].Fields, []string{test.field}) {
				t.Fatalf("affected fields = %v, want %s", results[0].Fields, test.field)
			}
			if len(org.Objects["MagnitudeEntry__c"].Records) != 0 {
				t.Fatal("oversized number was persisted")
			}
		})
	}
}

func TestNumericMagnitudePreservesValidNumberValues(t *testing.T) {
	for _, test := range []struct {
		name, field string
		value       storage.Value
	}{
		{"small positive boundary", "Small__c", storage.IntegerValue(99)},
		{"small negative boundary", "Small__c", storage.DecimalValue("-99")},
		{"small positive fractional boundary", "Small__c", storage.DecimalValue("99.49")},
		{"small negative fractional boundary", "Small__c", storage.DecimalValue("-99.49")},
		{"small fractional control", "Small__c", storage.DecimalValue("99.1")},
		{"small negative fractional control", "Small__c", storage.DecimalValue("-99.1")},
		{"medium positive boundary", "Medium__c", storage.DecimalValue("999999")},
		{"medium negative boundary", "Medium__c", storage.IntegerValue(-999999)},
		{"zero", "Small__c", storage.IntegerValue(0)},
		{"null", "Small__c", storage.NullValue()},
		{"fraction exceeds declared scale", "Small__c", storage.DecimalValue("12.3456789")},
		{"negative fraction exceeds declared scale", "Small__c", storage.DecimalValue("-12.3456789")},
		{"medium fraction exceeds declared scale", "Medium__c", storage.DecimalValue("12.3456789")},
	} {
		t.Run(test.name, func(t *testing.T) {
			org := numericMagnitudeOrg()
			engine := NewEngine(&org)
			results := engine.Insert([]storage.Record{numericMagnitudeRecord(test.field, test.value)})
			if len(results) != 1 || !results[0].Success {
				t.Fatalf("valid number result = %#v", results)
			}
			stored := org.Objects["MagnitudeEntry__c"].Records[results[0].ID]
			if test.value.Kind == storage.ValueNull {
				if !stored.HasExplicitNull(test.field) {
					t.Fatalf("explicit null lost: %#v", stored)
				}
				if value, exists := stored.Fields[test.field]; exists && value.Kind != storage.ValueNull {
					t.Fatalf("explicit null acquired a numeric value: %#v", value)
				}
			} else if !reflect.DeepEqual(stored.Fields[test.field], test.value) {
				t.Fatalf("stored value = %#v, want unchanged %#v", stored.Fields[test.field], test.value)
			}
		})
	}
}

func TestNumericMagnitudePartialInsertKeepsValidRow(t *testing.T) {
	org := numericMagnitudeOrg()
	engine := NewEngine(&org)
	results := engine.Insert([]storage.Record{
		numericMagnitudeRecord("Small__c", storage.DecimalValue("12.3456789")),
		numericMagnitudeRecord("Small__c", storage.IntegerValue(1000)),
	})
	if len(results) != 2 || !results[0].Success || results[1].Success || results[1].StatusCode != "NUMBER_OUTSIDE_VALID_RANGE" {
		t.Fatalf("partial number results = %#v", results)
	}
	rows := org.Objects["MagnitudeEntry__c"].Records
	if len(rows) != 1 || rows[results[0].ID].Fields["Small__c"].Decimal != "12.3456789" {
		t.Fatalf("partial insert changed its valid row: %#v", rows)
	}
}

func TestNumericMagnitudeFailedUpdateRetainsStoredValue(t *testing.T) {
	org := numericMagnitudeOrg()
	engine := NewEngine(&org)
	inserted := engine.Insert([]storage.Record{numericMagnitudeRecord("Medium__c", storage.DecimalValue("12.3456789"))})
	if len(inserted) != 1 || !inserted[0].Success {
		t.Fatalf("seed insert = %#v", inserted)
	}
	update := numericMagnitudeRecord("Medium__c", storage.IntegerValue(1000000))
	update.ID = inserted[0].ID
	results := engine.Update([]storage.Record{update})
	if len(results) != 1 || results[0].Success || results[0].StatusCode != "NUMBER_OUTSIDE_VALID_RANGE" {
		t.Fatalf("oversized update result = %#v", results)
	}
	stored := org.Objects["MagnitudeEntry__c"].Records[inserted[0].ID]
	if stored.Fields["Medium__c"].Decimal != "12.3456789" {
		t.Fatalf("failed update changed stored value: %#v", stored)
	}
}

func numericMagnitudeOrg() storage.OrgState {
	org := storage.NewOrgState()
	org.Objects["MagnitudeEntry__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "MagnitudeEntry__c", KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
				"Small__c": {
					APIName: "Small__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE",
					Precision: 2, Scale: 0, ScaleSpecified: true,
				},
				"Medium__c": {
					APIName: "Medium__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE",
					Precision: 6, Scale: 0, ScaleSpecified: true,
				},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	return org
}

func numericMagnitudeRecord(field string, value storage.Value) storage.Record {
	return storage.Record{
		Object: "MagnitudeEntry__c",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Owned magnitude"), field: value},
	}
}
