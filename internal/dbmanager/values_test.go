package dbmanager_test

import (
	"testing"

	"github.com/glade-sh/glade/internal/dbmanager"
	"github.com/glade-sh/glade/internal/storage"
)

func TestFieldInputToStorageValue(t *testing.T) {
	cases := []struct {
		name  string
		field storage.Field
		input dbmanager.FieldInput
		want  storage.Value
	}{
		{"picklist", storage.Field{APIName: "Industry", Type: storage.FieldPicklist}, dbmanager.FieldInput{State: "value", Value: "Technology"}, storage.StringValue("Technology")},
		{"multipicklist", storage.Field{APIName: "Services__c", Type: storage.FieldMultiPicklist}, dbmanager.FieldInput{State: "value", Values: []string{"Implementation", "Support"}}, storage.StringValue("Implementation;Support")},
		{"lookup", storage.Field{APIName: "OwnerId", Type: storage.FieldReference}, dbmanager.FieldInput{State: "value", ID: "005000000000001"}, storage.IDValue("005000000000001")},
		{"boolean", storage.Field{APIName: "Active__c", Type: storage.FieldBoolean}, dbmanager.FieldInput{State: "value", Value: true}, storage.BooleanValue(true)},
		{"integer", storage.Field{APIName: "Employees", Type: storage.FieldInteger}, dbmanager.FieldInput{State: "value", Value: "7"}, storage.IntegerValue(7)},
		{"decimal", storage.Field{APIName: "Amount", Type: storage.FieldDecimal}, dbmanager.FieldInput{State: "value", Value: "12.50"}, storage.DecimalValue("12.50")},
		{"date", storage.Field{APIName: "CloseDate", Type: storage.FieldDate}, dbmanager.FieldInput{State: "value", Value: "2026-07-01"}, storage.DateValue("2026-07-01")},
		{"datetime", storage.Field{APIName: "LastSeen", Type: storage.FieldDateTime}, dbmanager.FieldInput{State: "value", Value: "2026-07-01T12:30:00Z"}, storage.DateTimeValue("2026-07-01T12:30:00Z")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, explicitNull, err := dbmanager.FieldInputToStorageValue(tc.field, tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if explicitNull || !storage.ValuesEqual(got, tc.want) {
				t.Fatalf("got=%#v explicitNull=%t want=%#v", got, explicitNull, tc.want)
			}
		})
	}
}

func TestFieldInputToStorageValueExplicitNull(t *testing.T) {
	_, explicitNull, err := dbmanager.FieldInputToStorageValue(storage.Field{APIName: "Description", Type: storage.FieldString}, dbmanager.FieldInput{State: "null"})
	if err != nil {
		t.Fatal(err)
	}
	if !explicitNull {
		t.Fatal("explicitNull = false, want true")
	}
}

func TestFieldInputToStorageValueRejectsBadInteger(t *testing.T) {
	_, _, err := dbmanager.FieldInputToStorageValue(storage.Field{APIName: "Employees", Type: storage.FieldInteger}, dbmanager.FieldInput{State: "value", Value: "not-a-number"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFieldInputToStorageValueRejectsNonFiniteDecimal(t *testing.T) {
	_, _, err := dbmanager.FieldInputToStorageValue(storage.Field{APIName: "Amount", Type: storage.FieldDecimal}, dbmanager.FieldInput{State: "value", Value: "NaN"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestStorageValueJSONKeepsDecimalPrecision(t *testing.T) {
	got := dbmanager.StorageValueJSON(storage.DecimalValue("12345678901234567890.123456789"))
	if got != "12345678901234567890.123456789" {
		t.Fatalf("decimal JSON = %#v", got)
	}
}
