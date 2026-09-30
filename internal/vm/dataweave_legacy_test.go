package vm

import (
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/storage"
)

// Real DWL bytes and declared resource APIs replace the historical name-only fixtures.
func legacyDataWeaveMachine(t *testing.T, api string, names ...string) *VM {
	t.Helper()
	if home := os.Getenv("GLADE_DATAWEAVE_APEX_TEST_HOME"); home != "" {
		t.Setenv("GLADE_HOME", home)
	}
	if _, err := gladehome.DataWeaveRuntime(); err != nil {
		t.Fatalf("legacy DataWeave tests require an installed real toolchain: %v", err)
	}
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "Contact")
	for _, name := range names {
		source, ok := legacyDataWeaveSources[name]
		if !ok {
			t.Fatalf("missing real legacy DWL %s", name)
		}
		version := "62.0"
		if name == "records" {
			version = api
		}
		org.Metadata.DataWeaveResources = append(org.Metadata.DataWeaveResources, storage.DataWeaveResourceMetadata{Name: name, Content: source, APIVersion: version, ContentPath: "force-app/main/default/dw/" + name + ".dwl", MetadataPath: "force-app/main/default/dw/" + name + ".dwl-meta.xml"})
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	return machine
}

var legacyDataWeaveSources = map[string]string{
	"csvToContacts":              "%dw 2.0\ninput records application/csv\noutput application/apex\n---\nrecords map(record) -> {\n    FirstName: record.first_name,\n    LastName: record.last_name,\n    Email: record.email\n} as Object {class: \"Contact\"}",
	"csvToJsonBasic":             "%dw 2.0 \ninput payload application/csv\noutput application/json\n--- \npayload",
	"csvToJsonWithFieldRenaming": "%dw 2.0 \ninput payload application/csv\noutput application/json\nfun renameKey(key: Key) = key match {\n    case \"first_name\" -> \"FirstName\"\n    case \"last_name\" -> \"LastName\"\n    case \"company\" -> \"Company\"\n    case \"phone1\" -> \"HomePhone\"\n    case \"phone\" -> \"Phone\"\n    case \"email\" -> \"Email\"\n    case \"date\" -> \"DateofBirth\"\n    case \"address\" -> \"MailingStreet\"\n    case \"city\" -> \"MailingCity\"\n    case \"county\" -> \"MailingCountry\"\n    case \"state\" -> \"MailingState\"\n    case \"zip\" -> \"MailingPostalCode\"\n    else -> (key)\n}\n--- \npayload map (contact) ->\ncontact mapObject (value, key) -> {\n    (renameKey(key)) : value\n}",
	"error":                      "%dw 2.0 \noutput application/json\n--- \n1/0",
	"excelOutputError":           "%dw 2.0 \ninput records application/java\n// Excel Format support https://docs.mulesoft.com/dataweave/2.4/dataweave-formats-excel\n// Note that Excel is not currently supported for DW in Apex\noutput application/xlsx\n--- \nrecords",
	"helloWorld":                 "%dw 2.0\n--- \nlog(\"Hello World\")",
	"jsonDateFormat":             "%dw 2.0\ninput records application/java\noutput application/json\n--- \n{\n    users: records map(record) -> {\n        firstName: record.FirstName,\n        lastName: record.LastName,\n        // https://docs.mulesoft.com/dataweave/2.4/dataweave-cookbook-format-dates\n        createdDate: (record.CreatedDate >> \"UTC\") as String {format: \"hh:mm:ss a, MMMM dd, uuuu\"} \n    }\n}",
	"jsonToContacts":             "%dw 2.0\ninput records application/json\noutput application/apex\n---\nrecords map(record) -> {\n    FirstName: record.first_name,\n    LastName: record.last_name,\n    Email: record.email\n} as Object {class: \"Contact\"}",
	"multipleInputs":             "%dw 2.0 \n// This example is taken from:\n// https://docs.mulesoft.com/dataweave/2.5/dataweave-cookbook-reference-multiple-inputs\n// More on DataWeave formats:\n// https://docs.mulesoft.com/dataweave/2.5/dataweave-formats\ninput products application/json\ninput attributes application/json\ninput exchangeRates application/json\noutput application/xml\n--- \nbooks: {\n    (products filter ($.properties.year > attributes.publishedAfter) map  (item)   ->  {\n        book @(year: item.properties.year): {\n            (exchangeRates.USD map {\n                price @(currency: $.currency): $.ratio * item.price\n            }),\n            title: item.properties.title,\n            authors: { (item.properties.author map {\n                author: $\n            }) }\n        }\n    } )\n}",
	"records":                    "%dw 2.0\ninput records application/java\noutput application/apex\n---\nrecords\n",
}
