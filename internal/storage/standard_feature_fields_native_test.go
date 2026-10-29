package storage

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These profiles isolate one feature without changing edition or other settings.
// Their effective membership combines positive named Apex and granted REST
// observations; a field hidden only by FLS is not a feature gate.
func TestStandardFeatureGateNativeStorageRoutes(t *testing.T) {
	const dir = "../apextest/testdata/conformance/standard_describe_profiles"
	file, err := os.Open(filepath.Join(dir, "feature-set-differences.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	features := map[string]string{
		"P1": "MultiCurrency", "P2": "PersonAccounts", "P3": "StateAndCountryPicklist",
	}
	counts := map[string]int{}
	for _, row := range rows[1:] {
		feature, isolated := features[row[1]]
		if !isolated || row[3] != "effective-membership" || row[6] != "added" {
			continue
		}
		api, object, field := row[0], row[4], row[5]
		counts[api+"/"+feature]++
		t.Run(api+"/"+feature+"/"+object+"."+field, func(t *testing.T) {
			t.Run("constructor-gate-policy", func(t *testing.T) {
				if !StandardObjectFieldRequiresFeature(object, field) ||
					!StandardObjectFieldRequiresFeature(strings.ToLower(object), strings.ToLower(field)) {
					t.Fatal("native isolated feature adds this field; constructor policy must recognize its gate case-insensitively")
				}
			})
			for _, enabled := range []bool{false, true} {
				label := "disabled"
				var profile []string
				if enabled {
					label, profile = "enabled", []string{feature}
				}
				t.Run(label+"/catalog-describe", func(t *testing.T) {
					definition := ObjectDefinition{APIName: object}
					EnsureStandardObjectFieldsForFeatures(&definition, profile)
					if _, exists := ResolveFieldName(definition, "", field); exists != enabled {
						t.Fatalf("native membership=%t; catalog membership=%t", enabled, exists)
					}
				})
				t.Run(label+"/persisted-overlay", func(t *testing.T) {
					definition := ObjectDefinition{APIName: object}
					EnsureStandardObjectFieldsForFeatures(&definition, []string{feature})
					EnsureStandardObjectFieldsForFeatures(&definition, profile)
					EnsureStandardObjectFields(&definition)
					if _, exists := ResolveFieldName(definition, "", field); exists != enabled {
						t.Fatalf("native membership=%t; refreshed membership=%t", enabled, exists)
					}
					for _, relation := range definition.Relations {
						if !enabled && strings.EqualFold(relation.Field, field) {
							t.Fatalf("disabled field retains relationship: %+v", relation)
						}
					}
				})
				t.Run(label+"/relationship-visitor", func(t *testing.T) {
					VisitStandardObjectRelationships(object, profile, func(relation Relationship) {
						if !enabled && strings.EqualFold(relation.Field, field) {
							t.Fatalf("disabled field remains visible through relationship visitor: %+v", relation)
						}
					})
				})
			}
		})
	}
	for _, api := range []string{"62.0", "67.0"} {
		for profile, want := range map[string]int{"MultiCurrency": 25, "PersonAccounts": 46, "StateAndCountryPicklist": 30} {
			if got := counts[api+"/"+profile]; got != want {
				t.Errorf("%s/%s: read %d native rows, want %d", api, profile, got, want)
			}
		}
	}
}
