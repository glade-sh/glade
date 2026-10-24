package storage

import (
	"reflect"
	"testing"
)

func TestStandardDefinitionCachePreparedRelationshipMutationRoutes(t *testing.T) {
	for _, route := range []struct {
		name     string
		catalog  bool
		features []string
	}{
		{name: "catalog", catalog: true},
		{name: "default_features"},
		{name: "person_accounts", features: []string{"PersonAccounts"}},
		{name: "currency_and_picklists", features: []string{"MultiCurrency", "StateAndCountryPicklist"}},
	} {
		t.Run(route.name, func(t *testing.T) {
			read := func() ObjectDefinition {
				if route.catalog {
					definition, ok := StandardObjectDefinition("contact")
					if !ok {
						t.Fatal("missing Contact definition")
					}
					return definition
				}
				definition := ObjectDefinition{APIName: "Contact"}
				EnsureStandardObjectFieldsForFeatures(&definition, route.features)
				return definition
			}
			want, got := read(), read()
			relationshipIndex := -1
			for i, relationship := range got.Relations {
				if relationship.Field == "AccountId" {
					relationshipIndex = i
					if match, prepared := relationship.PreparedParentObjectMatch("account"); !match || !prepared {
						t.Fatalf("Contact.AccountId before mutation: match=%v prepared=%v", match, prepared)
					}
					break
				}
			}
			if relationshipIndex < 0 {
				t.Fatal("missing Contact.AccountId relationship")
			}
			defer func() {
				if failure := recover(); failure != nil {
					t.Errorf("public catalog mutation panicked: %v", failure)
				}
			}()
			mutateCatalogValue(reflect.ValueOf(&got).Elem())
			if got.APIName != "mutated" || got.Fields["AccountId"].APIName != "mutated" || got.Relations[relationshipIndex].ParentObjects[0] != "mutated" {
				t.Fatal("mutation did not reach public catalog metadata")
			}
			next := read()
			if !reflect.DeepEqual(next, want) {
				t.Fatal("public mutation corrupted the cached definition or prepared metadata")
			}
			if match, prepared := next.Relations[relationshipIndex].PreparedParentObjectMatch("account"); !match || !prepared {
				t.Fatalf("Contact.AccountId after mutation: match=%v prepared=%v", match, prepared)
			}
		})
	}
}
