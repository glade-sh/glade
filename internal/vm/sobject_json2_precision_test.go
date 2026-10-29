package vm

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native sobject_json2/org.tsv:41,86,106 captures padding to field scale. It
// does not authorize discarding a queried value's existing decimal precision.
func TestSObjectJSON2QueryNumericPrecision(t *testing.T) {
	for _, tc := range []struct {
		name, display, input, want string
		scale                      int
		compound, explicit, fresh  bool
	}{
		{name: "currencyOverprecision", display: "CURRENCY", scale: 2, input: "33.335", want: "33.335"},
		{name: "negativeCurrencyOverprecision", display: "CURRENCY", scale: 2, input: "-33.335", want: "-33.335"},
		{name: "numberOverprecision", display: "DOUBLE", scale: 4, input: "1.234567890123456789", want: "1.234567890123456789"},
		{name: "negativeNumberScaleZero", display: "DOUBLE", scale: 0, input: "-0.125", want: "-0.125"},
		{name: "existingTrailingPrecision", display: "CURRENCY", scale: 2, input: "33.3300", want: "33.3300"},
		{name: "nativeCurrencyPadding", display: "CURRENCY", scale: 2, input: "12", want: "12.00"},
		{name: "nativeCoordinatePadding", display: "DOUBLE", scale: 15, input: "12.5", want: "12.500000000000000"},
		{name: "nativeCompoundCoordinatePadding", display: "DOUBLE", scale: 15, compound: true, input: "12.5", want: "12.500000"},
		{name: "explicitQueryWrite", display: "CURRENCY", scale: 2, explicit: true, input: "12.5", want: "12.5"},
		{name: "explicitOverprecisionWrite", display: "CURRENCY", scale: 2, explicit: true, input: "33.335", want: "33.335"},
		{name: "unqueriedValue", display: "CURRENCY", scale: 2, fresh: true, input: "12.5", want: "12.5"},
		{name: "bareDecimalDefinition", scale: 2, input: "12.5", want: "12.5"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			field := storage.Field{APIName: "Amount__c", Type: storage.FieldDecimal, DisplayType: tc.display, Scale: tc.scale, ScaleSpecified: true}
			fields := map[string]storage.Field{}
			if tc.compound {
				field.APIName = "Point__Latitude__s"
				field.CompoundFieldName = "Point__c"
				fields["Point__c"] = storage.Field{APIName: "Point__c", Type: storage.FieldLocation, Scale: 6, ScaleSpecified: true}
			}
			fields[field.APIName] = field
			org := storage.OrgState{Objects: map[string]storage.ObjectState{
				"JSONPrecision__c": {Definition: storage.ObjectDefinition{APIName: "JSONPrecision__c", Fields: fields}},
			}}
			machine := New(nil)
			machine.SetOrg(&org)
			item, err := decimalFromText(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			value := Object("JSONPrecision__c")
			value.Fields[field.APIName] = item
			if !tc.fresh {
				value.Fields[jsonSObjectQueryProjectionField] = String(field.APIName)
			}
			if tc.explicit {
				markExplicitSObjectField(&value, field.APIName)
			}
			for _, suppress := range []bool{false, true} {
				data, err := jsonMarshalNoEscape(machine.jsonFromValueForSerialize(value, suppress))
				if err != nil {
					t.Fatal(err)
				}
				var decoded map[string]json.RawMessage
				if err := json.Unmarshal(data, &decoded); err != nil {
					t.Fatal(err)
				}
				if got := string(decoded[field.APIName]); got != tc.want {
					t.Errorf("suppress=%t serialized numeric field = %s, want %s", suppress, got, tc.want)
				}
				if got := decimalPlainText(value.Fields[field.APIName]); got != tc.input {
					t.Errorf("serialization mutated stored value: got %s, want %s", got, tc.input)
				}
			}
		})
	}
}
