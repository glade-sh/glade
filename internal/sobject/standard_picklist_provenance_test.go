package sobject

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
)

func TestStandardPicklistProvenanceDescribeRoundTrip(t *testing.T) {
	definition := storage.ObjectDefinition{APIName: "User", Fields: map[string]storage.Field{
		"LocaleSidKey":   {APIName: "LocaleSidKey", Type: storage.FieldPicklist, RestrictedPicklist: true, RestrictedPicklistFromCatalog: true, StandardValueSetApplied: true},
		"TimeZoneSidKey": {APIName: "TimeZoneSidKey", Type: storage.FieldPicklist, RestrictedPicklist: true},
	}}
	for pass := 0; pass < 2; pass++ {
		described := FromObjectDefinition(definition)
		for fieldName, wantCatalog := range map[string]bool{"LocaleSidKey": true, "TimeZoneSidKey": false} {
			field := described.Fields[fieldName]
			if !field.RestrictedPicklist || field.RestrictedPicklistFromCatalog != wantCatalog {
				t.Fatalf("pass %d describe %s restriction=%v catalog=%v; want catalog=%v", pass, fieldName, field.RestrictedPicklist, field.RestrictedPicklistFromCatalog, wantCatalog)
			}
			if field.StandardValueSetApplied != wantCatalog {
				t.Fatalf("pass %d describe %s lost value set provenance", pass, fieldName)
			}
		}
		data, err := json.Marshal(described)
		if err != nil {
			t.Fatal(err)
		}
		var restored DescribeSObjectResult
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatal(err)
		}
		definition = ToObjectDefinition(restored)
		for fieldName, wantCatalog := range map[string]bool{"LocaleSidKey": true, "TimeZoneSidKey": false} {
			field := definition.Fields[fieldName]
			if !field.RestrictedPicklist || field.RestrictedPicklistFromCatalog != wantCatalog {
				t.Fatalf("pass %d stored %s restriction=%v catalog=%v; want catalog=%v", pass, fieldName, field.RestrictedPicklist, field.RestrictedPicklistFromCatalog, wantCatalog)
			}
			if field.StandardValueSetApplied != wantCatalog {
				t.Fatalf("pass %d stored %s lost value set provenance", pass, fieldName)
			}
		}
	}
}

func TestStandardPicklistProvenanceExplicitSchemaOverride(t *testing.T) {
	registry := BuildDescribeRegistry(schema.Schema{Objects: []schema.Object{{
		Name: "User",
		Fields: []schema.Field{{
			Name:               "LocaleSidKey",
			Label:              "Owned Locale",
			Type:               "Picklist",
			RestrictedPicklist: true,
			PicklistValues:     []schema.PicklistValue{{FullName: "Owned", Active: true}},
		}},
	}}})
	described, err := registry.Describe("User")
	if err != nil {
		t.Fatal(err)
	}
	definition := ToObjectDefinition(described)
	storage.EnsureStandardObjectFields(&definition)
	for fieldName, wantCatalog := range map[string]bool{"LocaleSidKey": false, "LanguageLocaleKey": true, "TimeZoneSidKey": false} {
		field := definition.Fields[fieldName]
		if !field.RestrictedPicklist || field.RestrictedPicklistFromCatalog != wantCatalog {
			t.Errorf("%s restriction=%v catalog=%v; want catalog=%v", fieldName, field.RestrictedPicklist, field.RestrictedPicklistFromCatalog, wantCatalog)
		}
	}
	field := definition.Fields["LocaleSidKey"]
	if field.Label != "Owned Locale" || len(field.PicklistValues) != 1 || field.PicklistValues[0].Value != "Owned" {
		t.Fatalf("explicit metadata was overwritten: %#v", field)
	}
}
