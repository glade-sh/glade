package storage

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"
)

// Membership only: native positive and negative rows from two fresh profiles.
func TestB5B9FeatureFieldProbe(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/standard_describe_profiles/b5b9_probe/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		if row[2] != "anonymous" || (row[4] != "true" && row[4] != "false") {
			continue
		}
		parts := strings.Split(row[3], ".")
		if len(parts) != 2 {
			t.Fatalf("invalid case %q", row[3])
		}
		t.Run(row[0]+"/"+row[1]+"/"+row[3], func(t *testing.T) {
			var features []string
			if row[1] == "commerce" {
				features = []string{"B2BCommerce", "Communities", "CommerceEnabled"}
			}
			definition := ObjectDefinition{APIName: parts[0]}
			EnsureStandardObjectFieldsForFeatures(&definition, features)
			_, got := ResolveFieldName(definition, "", parts[1])
			if want := row[4] == "true"; got != want {
				t.Errorf("field present=%t; native=%t", got, want)
			}
		})
	}
}

// P10/enablement-differences.tsv lines 14-15, 239-240, 316-317 and
// 544-545: the Commerce setting adds both fields over the license-only shape.
func TestB5B9CommercePricebookAdjacentFields(t *testing.T) {
	for _, tc := range []struct {
		name     string
		features []string
		want     bool
	}{
		{"license-only", []string{"B2BCommerce", "Communities"}, false},
		{"enabled", []string{"B2BCommerce", "Communities", "CommerceEnabled"}, true},
	} {
		for _, field := range []string{"ActivePriceAdjustmentQuantity", "ProductSellingModelId"} {
			t.Run(tc.name+"/"+field, func(t *testing.T) {
				def := ObjectDefinition{APIName: "PricebookEntry"}
				EnsureStandardObjectFieldsForFeatures(&def, tc.features)
				_, got := ResolveFieldName(def, "", field)
				if got != tc.want {
					t.Errorf("field present=%t; native=%t", got, tc.want)
				}
			})
		}
	}
}
