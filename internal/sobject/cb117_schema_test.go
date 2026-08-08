package sobject

import (
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
)

func TestCB117BuildDescribeRegistryPreservesAPI67FieldTypes(t *testing.T) {
	registry := BuildDescribeRegistry(schema.Schema{Objects: []schema.Object{{
		Name: "Probe__c",
		Fields: []schema.Field{
			{Name: "Email__c", Type: "Email"},
			{Name: "Url__c", Type: "Url"},
			{Name: "Auto__c", Type: "AutoNumber"},
			{Name: "External__c", Type: "Text", ExternalID: true},
		},
	}}})

	describe, err := registry.Describe("Probe__c")
	if err != nil {
		t.Fatal(err)
	}
	if got := describe.Fields["Email__c"].DisplayType; got != "EMAIL" {
		t.Fatalf("Email display type = %q, want EMAIL", got)
	}
	if got := describe.Fields["Url__c"].DisplayType; got != "URL" {
		t.Fatalf("Url display type = %q, want URL", got)
	}
	auto := describe.Fields["Auto__c"]
	if auto.Type != storage.FieldString || auto.DisplayType != "STRING" || !auto.AutoNumber {
		t.Fatalf("AutoNumber describe = %#v", auto)
	}
	if !describe.Fields["External__c"].IDLookup {
		t.Fatalf("external id was not exposed as an id lookup: %#v", describe.Fields["External__c"])
	}
}

func TestCB117BuildDescribeRegistryCarriesObjectSearchability(t *testing.T) {
	registry := BuildDescribeRegistry(schema.Schema{Objects: []schema.Object{
		{Name: "Default__c"},
		{Name: "Enabled__c", EnableSearch: true},
	}})

	defaultDescribe, err := registry.Describe("Default__c")
	if err != nil {
		t.Fatal(err)
	}
	if defaultDescribe.EnableSearch {
		t.Fatal("custom object without enableSearch defaulted to searchable")
	}
	enabledDescribe, err := registry.Describe("Enabled__c")
	if err != nil {
		t.Fatal(err)
	}
	if !enabledDescribe.EnableSearch {
		t.Fatal("explicitly enabled custom object lost enableSearch")
	}
}
