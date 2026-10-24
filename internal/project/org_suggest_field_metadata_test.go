package project_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestSetupAdviceFieldMetadataNativeCatalogRoutes(t *testing.T) {
	data, err := os.ReadFile("org_setup_field_metadata.json")
	if err != nil {
		t.Fatal(err)
	}
	var features map[string]map[string][]string
	if err := json.Unmarshal(data, &features); err != nil {
		t.Fatal(err)
	}
	if len(features) != 2 {
		t.Fatalf("feature metadata count=%d", len(features))
	}
	for feature, objects := range features {
		for object, fields := range objects {
			t.Run(feature+"/"+object, func(t *testing.T) {
				base, enabled := storage.ObjectDefinition{APIName: object}, storage.ObjectDefinition{APIName: object}
				// Keep the independent address-picklist gate enabled on both sides.
				storage.EnsureStandardObjectFieldsForFeatures(&base, []string{"StateAndCountryPicklist"})
				storage.EnsureStandardObjectFieldsForFeatures(&enabled, []string{"StateAndCountryPicklist", feature})
				for _, field := range fields {
					_, without := storage.ResolveFieldName(base, "", field)
					_, with := storage.ResolveFieldName(enabled, "", field)
					if without || !with {
						t.Errorf("%s.%s metadata differs from the native-backed catalog: disabled=%v enabled=%v", object, field, without, with)
					}
				}
			})
		}
	}
}
