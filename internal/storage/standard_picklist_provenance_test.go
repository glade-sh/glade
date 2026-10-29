package storage

import (
	"encoding/json"
	"testing"
)

func TestStandardPicklistProvenanceCatalogRoutes(t *testing.T) {
	for _, row := range []struct {
		object, field string
		restricted    bool
	}{
		{"Account", "BillingGeocodeAccuracy", true},
		{"User", "LocaleSidKey", true},
		{"Account", "Industry", false},
	} {
		t.Run(row.object+"."+row.field, func(t *testing.T) {
			definition, ok := StandardObjectDefinition(row.object)
			if !ok {
				t.Fatal("missing standard object")
			}
			packed, ok, err := lookupStandardDescribeCatalogV2(row.object)
			if err != nil || !ok {
				t.Fatalf("packed describe: present=%v err=%v", ok, err)
			}
			for route, field := range map[string]Field{
				"catalog": definition.Fields[row.field],
				"packed":  describeFieldMap(packed.Fields)[row.field],
			} {
				if field.RestrictedPicklist != row.restricted || field.RestrictedPicklistFromCatalog != row.restricted {
					t.Errorf("%s restriction=%v catalog=%v; want both %v", route, field.RestrictedPicklist, field.RestrictedPicklistFromCatalog, row.restricted)
				}
			}
		})
	}
	var typed standardDescribeField
	if err := json.Unmarshal([]byte(`{"name":"LocaleSidKey","type":"picklist","restrictedPicklist":true}`), &typed); err != nil {
		t.Fatal(err)
	}
	if field := describeField(typed); !field.RestrictedPicklist || !field.RestrictedPicklistFromCatalog {
		t.Fatalf("typed describe projection lost catalog provenance: %#v", field)
	}
}

func TestStandardPicklistProvenanceEnrichmentAndPersistence(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		name := "catalog"
		if explicit {
			name = "explicit"
		}
		t.Run(name, func(t *testing.T) {
			definition := ObjectDefinition{APIName: "User", Fields: map[string]Field{
				"LocaleSidKey": {APIName: "LocaleSidKey", Type: FieldPicklist, RestrictedPicklist: explicit},
			}}
			catalog := describeField(standardDescribeField{Name: "LocaleSidKey", Type: "picklist", RestrictedPicklist: true})
			for pass := 0; pass < 2; pass++ {
				EnsureStandardObjectFields(&definition)
				field := definition.Fields["LocaleSidKey"]
				enrichStandardField(&field, catalog)
				definition.Fields["LocaleSidKey"] = field
				if !field.RestrictedPicklist || field.RestrictedPicklistFromCatalog == explicit {
					t.Fatalf("pass %d restriction=%v catalog=%v; explicit=%v", pass, field.RestrictedPicklist, field.RestrictedPicklistFromCatalog, explicit)
				}
				data, err := json.Marshal(definition)
				if err != nil {
					t.Fatal(err)
				}
				var restored ObjectDefinition
				if err := json.Unmarshal(data, &restored); err != nil {
					t.Fatal(err)
				}
				if restored.Fields["LocaleSidKey"].RestrictedPicklistFromCatalog != field.RestrictedPicklistFromCatalog {
					t.Fatal("storage JSON round trip lost restriction provenance")
				}
				definition = restored
			}
			if field := definition.Fields["TimeZoneSidKey"]; !field.RestrictedPicklist || field.RestrictedPicklistFromCatalog {
				t.Fatalf("native User.TimeZoneSidKey restriction became catalog-only: %#v", field)
			}
		})
	}
}
