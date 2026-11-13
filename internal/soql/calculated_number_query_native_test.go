package soql

import (
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

type calculatedNumberObservation struct{ api, route, input, blank, zero, rounded, value string }

func calculatedNumberObservations(t *testing.T) []calculatedNumberObservation {
	t.Helper()
	data, err := os.ReadFile("testdata/calculated_number_query/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 13 || lines[0] != "api\troute\tinput\tblank\tzero\trounded\tvalue" {
		t.Fatal("invalid native table")
	}
	var out []calculatedNumberObservation
	for _, line := range lines[1:] {
		r := strings.Split(line, "\t")
		if len(r) != 7 {
			t.Fatalf("invalid row %q", line)
		}
		out = append(out, calculatedNumberObservation{r[0], r[1], r[2], r[3], r[4], r[5], r[6]})
	}
	return out
}
func queryCalculatedNumberObservation(t *testing.T, input string) storage.Record {
	t.Helper()
	fields := map[string]storage.Field{
		"Name":    {APIName: "Name", Type: storage.FieldString},
		"Num__c":  {APIName: "Num__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE", Scale: 2, ScaleSpecified: true},
		"Text__c": {APIName: "Text__c", Type: storage.FieldString},
	}
	for _, f := range []struct{ name, formula, policy string }{
		{"Blank__c", "Num__c + 1", "BlankAsBlank"}, {"Zero__c", "Num__c + 1", "BlankAsZero"}, {"Rounded__c", "ROUND(Num__c / 3, 2)", "BlankAsBlank"}, {"Value__c", `IF(ISBLANK(Text__c), 0, VALUE(Text__c))`, "BlankAsBlank"},
	} {
		fields[f.name] = storage.Field{APIName: f.name, Type: storage.FieldCalculated, DisplayType: "DOUBLE", Formula: f.formula, FormulaTreatBlanksAs: f.policy, Scale: 2, ScaleSpecified: true}
	}
	def := storage.ObjectDefinition{APIName: "Probe__c", Fields: fields}
	row := storage.Record{ID: "a01000000000001", Object: "Probe__c", Fields: map[string]storage.Value{"Name": storage.StringValue(input)}}
	switch input {
	case "empty":
		row.Fields["Num__c"] = storage.DecimalValue("0.00")
	case "null":
	case "numeric":
		row.Fields["Num__c"] = storage.DecimalValue("1.25")
		row.Fields["Text__c"] = storage.StringValue("12.5")
	default:
		t.Fatalf("invalid input %q", input)
	}
	org := storage.NewOrgState()
	org.Objects["Probe__c"] = storage.ObjectState{Definition: def, Records: map[storage.ID]storage.Record{row.ID: row}}
	result, err := ParseAndExecute(org, "SELECT Blank__c, Zero__c, Rounded__c, Value__c FROM Probe__c")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("rows=%d", len(result.Records))
	}
	return result.Records[0]
}
func observedNumberText(v storage.Value) string {
	if v.Kind == storage.ValueNull {
		return "null"
	}
	return v.Decimal
}

// NATIVE testdata/calculated_number_query/org.tsv rows 2–13.
func TestCalculatedNumberQueryBlankPolicyNative(t *testing.T) {
	for _, o := range calculatedNumberObservations(t) {
		t.Run(o.api+"/"+o.route+"/"+o.input, func(t *testing.T) {
			row := queryCalculatedNumberObservation(t, o.input)
			for _, tc := range []struct{ field, want string }{{"Blank__c", o.blank}, {"Zero__c", o.zero}, {"Rounded__c", o.rounded}} {
				got := observedNumberText(row.Fields[tc.field])
				if (got == "null") != (tc.want == "null") {
					t.Errorf("%s=%s, want null=%t", tc.field, got, tc.want == "null")
				}
			}
		})
	}
}

// NATIVE testdata/calculated_number_query/org.tsv rows 2–13.
func TestCalculatedNumberQueryDeclaredScaleNative(t *testing.T) {
	for _, o := range calculatedNumberObservations(t) {
		t.Run(o.api+"/"+o.route+"/"+o.input, func(t *testing.T) {
			row := queryCalculatedNumberObservation(t, o.input)
			for _, tc := range []struct{ field, want string }{{"Blank__c", o.blank}, {"Zero__c", o.zero}, {"Rounded__c", o.rounded}, {"Value__c", o.value}} {
				got := observedNumberText(row.Fields[tc.field])
				if got != tc.want {
					t.Errorf("%s=%q, want %q", tc.field, got, tc.want)
				}
			}
		})
	}
}
