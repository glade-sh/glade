package storage

import (
	"encoding/csv"
	"os"
	"reflect"
	"sort"
	"testing"
)

// Direct missing-field and boundary rows: standard_constructor_complete/org.tsv.
// Configured fields and unqualified objects are local preservation controls.
func TestStandardConstructorCompleteNativePolicy(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/standard_constructor_complete/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		if row[3] != "anonymous" || row[2] == "missing-put" {
			continue
		}
		t.Run(row[0]+"/"+row[1]+"/"+row[2], func(t *testing.T) {
			field := "SyntheticAbsentField"
			if row[2] == "boundary-constructor" {
				field = map[string]string{"Contact": "LastName", "Lead": "Company", "PricebookEntry": "UnitPrice"}[row[1]]
			}
			want := row[4] == "true"
			for _, route := range []string{"cold", "warm"} {
				def := ObjectDefinition{APIName: row[1], Fields: map[string]Field{}}
				EnsureStandardObjectFieldsForFeatures(&def, nil)
				if route == "warm" {
					RefreshStandardObjectFieldOverlay(&def)
				}
				if got := StandardObjectConstructorFieldAllowedForFeatures(row[1], field, nil, def); got != want {
					t.Errorf("%s: native allowed=%t; policy=%t", route, want, got)
				}
			}
			def := ObjectDefinition{APIName: row[1], Fields: map[string]Field{field: {APIName: field, Type: FieldString}}}
			if !StandardObjectConstructorFieldAllowedForFeatures(row[1], field, nil, def) {
				t.Error("configured field lost")
			}
		})
	}
	var closed []string
	for object, policy := range standardFieldPolicies {
		if len(policy.ConstructorFields) > 0 {
			closed = append(closed, object)
		}
	}
	sort.Strings(closed)
	want := []string{"account", "contact", "emailmessage", "lead", "pricebookentry", "webstore"}
	if !reflect.DeepEqual(closed, want) {
		t.Fatalf("qualified constructor objects=%v; want %v", closed, want)
	}
	if !StandardObjectConstructorFieldAllowedForFeatures("Campaign", "SyntheticAbsentField", nil, ObjectDefinition{}) {
		t.Fatal("unqualified object fallback closed")
	}
}

// Preserve the parent policy's open admission for fields known only in retained
// profiles whose runtime expansion is deferred. This is a local compatibility
// control, not a new native constructor-acceptance claim. Native field presence:
// standard_describe_profiles P4/P10/PU granted describes and membership deltas.
func TestStandardConstructorCompleteDeferredProfileFallback(t *testing.T) {
	for object, fields := range map[string][]string{
		"Contact":        {"IsPriorityRecord"},
		"Lead":           {"IsPriorityRecord", "PartnerAccountId", "PartnerAccount"},
		"PricebookEntry": {"ActivePriceAdjustmentQuantity", "ProductSellingModelId", "ProductSellingModel"},
	} {
		for _, field := range fields {
			if !StandardObjectConstructorFieldAllowedForFeatures(object, field, nil, ObjectDefinition{APIName: object}) {
				t.Errorf("%s.%s lost the unqualified profile fallback", object, field)
			}
		}
	}
}
