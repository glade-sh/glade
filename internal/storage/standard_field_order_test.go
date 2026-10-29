package storage

import (
	"reflect"
	"testing"
)

func TestVisitStandardFieldsPreservesDescribeSequence(t *testing.T) {
	// Whole-SObject JSON observes this sequence after filtering unavailable or
	// null fields: ../apextest/testdata/conformance/sobject_json2/org.tsv:51.
	want := []string{
		"Id", "IsDeleted", "Name", "CurrencyIsoCode", "CreatedDate", "CreatedById",
		"LastModifiedDate", "LastModifiedById", "SystemModstamp", "LastViewedDate",
		"LastReferencedDate", "IsActive", "IsArchived", "Description", "IsStandard",
	}
	for _, name := range []string{"Pricebook2", "pricebook2", " Pricebook2 "} {
		var got []string
		if !VisitStandardFields(name, func(field string) { got = append(got, field) }) {
			t.Fatalf("VisitStandardFields(%q) did not find the catalog object", name)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("VisitStandardFields(%q) = %v, want %v", name, got, want)
		}
	}
	for _, name := range []string{"", "MissingJSONOrder__c"} {
		if VisitStandardFields(name, func(field string) { t.Errorf("unknown object %q visited %q", name, field) }) {
			t.Errorf("VisitStandardFields(%q) unexpectedly found an object", name)
		}
	}
}

func TestStandardFieldOrderCoversCatalog(t *testing.T) {
	for _, name := range standardDescribeCatalogObjectNames {
		if !VisitStandardFields(name, func(string) {}) {
			t.Errorf("field sequence missing for catalog object %q", name)
		}
	}
}
