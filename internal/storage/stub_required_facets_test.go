package storage

import (
	"reflect"
	"testing"
)

func TestStubRequiredFieldsRetainDescribeFacets(t *testing.T) {
	definition := ObjectDefinition{APIName: "AccountBrand"}
	EnsureStandardObjectFields(&definition)
	describe, ok, err := lookupStandardDescribeCatalogV2("AccountBrand")
	if err != nil || !ok {
		t.Fatalf("owned describe fixture: ok=%v err=%v", ok, err)
	}
	want := describeFieldMap(describe.Fields)
	for _, name := range []string{"Name", "AccountId"} {
		got, fixture := definition.Fields[name], want[name]
		if !reflect.DeepEqual(got.Nillable, fixture.Nillable) || !reflect.DeepEqual(got.Createable, fixture.Createable) || !reflect.DeepEqual(got.Updateable, fixture.Updateable) || got.Length != fixture.Length {
			t.Fatalf("%s required facets were lost: got=%+v fixture=%+v", name, got, fixture)
		}
		if got.Required {
			t.Fatalf("%s describe facet acquired an unobserved DML requirement", name)
		}
	}
	// The describe input, not the field name, supplies the required facets.
	explicit := true
	definition = ObjectDefinition{APIName: "AccountBrand", Fields: map[string]Field{
		"Name": {APIName: "Name", Type: FieldString, Nillable: &explicit},
	}}
	EnsureStandardObjectFields(&definition)
	if !*definition.Fields["Name"].Nillable || definition.Fields["Name"].Required {
		t.Fatal("explicit fixture facet was overwritten")
	}
	definition = ObjectDefinition{APIName: "AccountBrand", Fields: map[string]Field{
		"Name": {APIName: "Name", Type: FieldString, Required: true},
	}}
	EnsureStandardObjectFields(&definition)
	if !definition.Fields["Name"].Required {
		t.Fatal("explicit DML requirement was removed")
	}
}

func TestStubDescribeCandidatePreservesUnrelatedRequiredDefaults(t *testing.T) {
	for _, tc := range []struct {
		object string
		fields []string
	}{
		{"ContentDistribution", []string{"OwnerId", "PreferencesAllowPDFDownload", "PreferencesAllowOriginalDownload", "PreferencesPasswordRequired", "PreferencesNotifyOnVisit", "PreferencesLinkLatestVersion", "PreferencesAllowViewInBrowser", "PreferencesExpires", "PreferencesNotifyRndtnComplete"}},
		{"EmailMessage", []string{"Status"}},
	} {
		t.Run(tc.object, func(t *testing.T) {
			definition, found := StandardObjectDefinition(tc.object)
			if !found {
				t.Fatal("standard definition missing")
			}
			for _, name := range tc.fields {
				field, found := definition.Fields[name]
				if !found || field.Required || field.DefaultValue != "" {
					t.Fatalf("%s acquired a caller requirement or default: %+v, found=%v", name, field, found)
				}
			}
		})
	}
}
