package storage

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
)

// Native queried-mdt rows: ../vm/testdata/constructed_metadata_defaults/org.tsv:
// 26-37 and 90-101 (API 62/67). Omitted stored scalar fields receive defaults.
func TestMetadataScalarDefaultsLoaderRoutes(t *testing.T) {
	definition := ObjectDefinition{APIName: "DefaultProbe__mdt", Fields: map[string]Field{
		"Priority__c": {APIName: "Priority__c", Type: FieldDecimal, DefaultValue: "0"},
		"Title__c":    {APIName: "Title__c", Type: FieldString, DefaultValue: "'seed'"},
		"Choice__c":   {APIName: "Choice__c", Type: FieldPicklist, DefaultValue: "'First'"},
	}}
	for _, route := range []string{"omitted", "explicit", "explicit-null"} {
		t.Run(route, func(t *testing.T) {
			source := schema.CustomMetadataRecord{DeveloperName: "Stored"}
			if route == "explicit" {
				source.Values = []schema.CustomMetadataValue{{Field: "Title__c", Value: "set"}}
			} else if route == "explicit-null" {
				source.Values = []schema.CustomMetadataValue{{Field: "Title__c", Nil: true}}
			}
			fields, _, err := customMetadataFields(NewOrgState(), definition, source)
			if err != nil {
				t.Fatal(err)
			}
			if value, ok := fields["Priority__c"]; !ok || !reflect.DeepEqual(value, DecimalValue("0")) {
				t.Errorf("stored priority=%+v present=%t; native 0", value, ok)
			}
			if value := fields["Choice__c"]; !reflect.DeepEqual(value, StringValue("First")) {
				t.Errorf("stored choice=%+v", value)
			}
			want := StringValue("seed")
			if route == "explicit" {
				want = StringValue("set")
			}
			if route == "explicit-null" {
				want = NullValue()
			}
			if value, ok := fields["Title__c"]; !ok || !reflect.DeepEqual(value, want) {
				t.Errorf("stored title=%+v present=%t; want %+v", value, ok, want)
			}
		})
	}
}
