package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

const commerceWebStoreAutoFacetingField = "OptionsAutoFacetingEnabled"

func commerceWebStoreNativeField(t *testing.T, api string) standardDescribeField {
	t.Helper()
	path := filepath.Join("../../testdata/conformance/commerce_enabled_surface", "webstore-fields-"+api+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var describe standardDescribeObject
	if err := json.Unmarshal(raw, &describe); err != nil {
		t.Fatal(err)
	}
	for _, field := range describe.Fields {
		if field.Name == commerceWebStoreAutoFacetingField {
			if field.Type != "boolean" || field.Createable == nil || !*field.Createable || field.Updateable == nil || !*field.Updateable || field.Nillable == nil || *field.Nillable {
				t.Fatalf("unexpected native field facets at API %s: %+v", api, field)
			}
			return field
		}
	}
	t.Fatalf("native API %s describe omits %s", api, commerceWebStoreAutoFacetingField)
	return standardDescribeField{}
}

// commerce_enabled_surface/org.tsv:9,12,20,23,30-31 captures constructor
// acceptance, membership and rich Boolean facets at APIs 62/67. Source-only
// shape retains the supported catalog; it is a compatibility fallback rather
// than a claim that an unknown org has Commerce provisioned.
func TestCommerceWebStoreNativeFieldCatalogAndPersistedRefresh(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		native := commerceWebStoreNativeField(t, api)
		for _, shape := range []struct {
			name     string
			features []string
		}{
			{"source-only", []string{project.UnspecifiedStandardFieldShape}},
			{"commerce", []string{"B2BCommerce", "Communities", "CommerceEnabled"}},
		} {
			t.Run(api+"/"+shape.name, func(t *testing.T) {
				definition := ObjectDefinition{APIName: "WebStore"}
				EnsureStandardObjectFieldsForFeatures(&definition, shape.features)
				check := func() {
					t.Helper()
					name, exists := ResolveFieldName(definition, "", commerceWebStoreAutoFacetingField)
					if !exists {
						t.Fatalf("native field %s is missing", commerceWebStoreAutoFacetingField)
					}
					field := definition.Fields[name]
					if field.Type != FieldBoolean || field.DisplayType != "BOOLEAN" || field.Label != native.Label {
						t.Fatalf("native Boolean field shape differs: %+v", field)
					}
					for _, facet := range []struct {
						name string
						got  *bool
						want *bool
					}{
						{"createable", field.Createable, native.Createable},
						{"updateable", field.Updateable, native.Updateable},
						{"nillable", field.Nillable, native.Nillable},
						{"defaultedOnCreate", field.DefaultedOnCreate, native.DefaultedOnCreate},
						{"filterable", field.Filterable, native.Filterable},
						{"groupable", field.Groupable, native.Groupable},
						{"sortable", field.Sortable, native.Sortable},
						{"aggregatable", field.Aggregatable, native.Aggregatable},
					} {
						if facet.got == nil || facet.want == nil || *facet.got != *facet.want {
							t.Errorf("%s differs from the API %s native describe", facet.name, api)
						}
					}
					if StandardObjectFieldsNeedWrite(definition) {
						t.Fatal("completed overlay still requests repair")
					}
					if RefreshStandardObjectFieldOverlay(&definition) {
						t.Fatal("warm refresh changed a complete overlay")
					}
				}
				check()
				delete(definition.Fields, commerceWebStoreAutoFacetingField)
				if !StandardObjectFieldsNeedWrite(definition) {
					t.Fatal("current-marker overlay did not detect the missing native field")
				}
				if !RefreshStandardObjectFieldOverlay(&definition) {
					t.Fatal("current-marker overlay did not repair the missing native field")
				}
				check()
			})
		}
	}
}

// P10/license-only/org.tsv:2455,5059,12960,15736 proves object absence;
// P10/org.tsv:2502,5181,13305,16157 proves the enabled field. P8 named rows
// 4942,10050,15440,20902 retain positive membership despite REST omission.
// Provisioning is checked separately from the reusable object schema template.
func TestCommerceWebStoreNativeProvisioningAndConfiguredFieldTransitions(t *testing.T) {
	for _, configured := range []bool{false, true} {
		name := "catalog"
		if configured {
			name = "configured"
		}
		t.Run(name, func(t *testing.T) {
			definition := ObjectDefinition{APIName: "WebStore"}
			if configured {
				definition.Fields = map[string]Field{
					commerceWebStoreAutoFacetingField: {APIName: commerceWebStoreAutoFacetingField, Label: "Owned facet label", Type: FieldBoolean, Nillable: BoolFlag(true)},
					"OwnedBoundary__c":                {APIName: "OwnedBoundary__c", Label: "Owned boundary", Type: FieldString},
				}
			}
			for _, shape := range []struct {
				name      string
				features  []string
				available bool
			}{
				{"commerce", []string{"B2BCommerce", "Communities", "CommerceEnabled"}, true},
				{"license-only", []string{"B2BCommerce", "Communities"}, false},
				{"bare", nil, false},
				{"order-management", []string{"OrderManagement"}, true},
				{"source-only", []string{project.UnspecifiedStandardFieldShape}, true},
				{"commerce-again", []string{"B2BCommerce", "Communities", "CommerceEnabled"}, true},
			} {
				EnsureStandardObjectFieldsForFeatures(&definition, shape.features)
				if got := StandardObjectConstructorAvailableForFeatures("WebStore", shape.features); got != shape.available {
					t.Fatalf("%s object provisioning=%t, want %t", shape.name, got, shape.available)
				}
				if StandardObjectFieldsNeedWrite(definition) || RefreshStandardObjectFieldOverlay(&definition) {
					t.Fatalf("%s completed overlay is unstable", shape.name)
				}
				if configured {
					field, exists := definition.Fields[commerceWebStoreAutoFacetingField]
					if !exists || field.Label != "Owned facet label" || field.Type != FieldBoolean || field.Nillable == nil || !*field.Nillable {
						t.Fatalf("%s changed explicit field metadata: %+v", shape.name, field)
					}
					if field, exists := definition.Fields["OwnedBoundary__c"]; !exists || field.Label != "Owned boundary" || field.Type != FieldString {
						t.Fatalf("%s changed unrelated project field: %+v", shape.name, field)
					}
				}
			}
		})
	}
}
