package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Salesforce Formula Operators and Functions: FIND is case-sensitive, uses
// one-based positions, and returns zero for blank text or an out-of-range start.
// Native API 62/67/68 observations additionally show that start zero means one.
// https://help.salesforce.com/s/articleView?id=sf.customize_functions_find.htm&language=en_US&type=5
func TestFormulaFindDocumentedSemantics(t *testing.T) {
	definition := storage.ObjectDefinition{APIName: "LyricTile__c", Fields: map[string]storage.Field{
		"Name":             {APIName: "Name", Type: storage.FieldString},
		"OptionalGlyph__c": {APIName: "OptionalGlyph__c", Type: storage.FieldString},
	}}
	for _, tc := range []struct {
		name, formula, input, want string
	}{
		{"first-space", `FIND(" ",Name)`, "alpha beta", "6"},
		{"case-sensitive", `FIND("B",Name)`, "alpha beta", "0"},
		{"no-match", `FIND("z",Name)`, "alpha beta", "0"},
		{"optional-start", `FIND("a",Name,3)`, "alpha beta", "5"},
		{"later-match", `FIND("a",Name,6)`, "alpha beta", "10"},
		{"last-character", `FIND("a",Name,10)`, "alpha beta", "10"},
		{"zero-start", `FIND("a",Name,0)`, "alpha beta", "1"},
		{"negative-start", `FIND("a",Name,-1)`, "alpha beta", "0"},
		{"past-end", `FIND("a",Name,11)`, "alpha beta", "0"},
		{"blank-text", `FIND("a",Name)`, "", "0"},
		{"unset-text", `FIND("x",OptionalGlyph__c)`, "alpha beta", "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			field := storage.Field{APIName: "SpaceOrdinal__c", Type: storage.FieldCalculated, DisplayType: "DECIMAL", Formula: tc.formula}
			record := storage.Record{Object: definition.APIName, Fields: map[string]storage.Value{"Name": storage.StringValue(tc.input)}}
			value, isNull, ok := EvaluateRecordFormulaValueInOrg(tc.formula, field, nil, definition, record)
			if !ok || isNull || value.Kind != storage.ValueDecimal || value.Decimal != tc.want {
				t.Fatalf("FIND = %#v null=%v ok=%v; want %s", value, isNull, ok, tc.want)
			}
		})
	}
	field := storage.Field{APIName: "CaptionGlyph__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: `TEXT(FIND(" ",Name))`}
	record := storage.Record{Object: definition.APIName, Fields: map[string]storage.Value{"Name": storage.StringValue("alpha beta")}}
	value, isNull, ok := EvaluateRecordFormulaValueInOrg(field.Formula, field, nil, definition, record)
	if !ok || isNull || value.Kind != storage.ValueString || value.String != "6" {
		t.Fatalf("TEXT(FIND) = %#v null=%v ok=%v; want text 6", value, isNull, ok)
	}
}

func TestFormulaFindBuilderRecognizesNumericResultAndOptionalStart(t *testing.T) {
	for _, formula := range []string{`FIND(" ","alpha beta")`, `FIND("a","alpha beta",3)`} {
		if message := ValidateFormulaInstance(formula, nil, storage.ObjectDefinition{}, "DECIMAL", false); message != "" {
			t.Fatalf("valid FIND rejected: %s", message)
		}
		if message := ValidateFormulaInstance(formula, nil, storage.ObjectDefinition{}, "STRING", false); message == "" {
			t.Fatal("numeric FIND accepted as a String result")
		}
	}
	for _, formula := range []string{`FIND("a")`, `FIND("a","alpha beta",1,2)`} {
		if message := ValidateFormulaInstance(formula, nil, storage.ObjectDefinition{}, "DECIMAL", false); message == "" {
			t.Fatal("FIND accepted an undocumented argument count")
		}
	}
	if message := ValidateFormulaInstance(`TEXT(FIND(" ","alpha beta"))`, nil, storage.ObjectDefinition{}, "STRING", false); message != "" {
		t.Fatalf("TEXT(FIND) rejected: %s", message)
	}
}
