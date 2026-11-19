package compile

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Enabling the captured currency feature must not change the remaining
// custom-object import schema for the same project.
func TestCustomCurrencyOverlayPreservesOtherImports(t *testing.T) {
	load := func(features string) salesforceSchemaObject {
		t.Helper()
		root := t.TempDir()
		writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
		writeCompileFixtureFile(t, filepath.Join(root, "config", "project-scratch-def.json"), `{"edition":"Developer","features":`+features+`}`)
		body, err := os.ReadFile(filepath.Join("testdata", "pr163_custom_currency", "Shape__c.object-meta.xml"))
		if err != nil {
			t.Fatal(err)
		}
		writeCompileFixtureFile(t, filepath.Join(root, "force-app", "main", "default", "objects", "Shape__c", "Shape__c.object-meta.xml"), string(body))
		writeCompileFixtureFile(t, filepath.Join(root, "force-app", "main", "default", "objects", "Shape__c", "fields", "Known__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Known__c</fullName><label>Known</label><length>30</length><type>Text</type></CustomField>`)
		writeCompileFixtureFile(t, filepath.Join(root, "force-app", "main", "default", "objects", "Shape__c", "fields", "Parent__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Parent__c</fullName><label>Parent</label><referenceTo>Account</referenceTo><relationshipLabel>Shapes</relationshipLabel><relationshipName>Shapes</relationshipName><type>Lookup</type></CustomField>`)
		p, err := project.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := loadSalesforceImports(p, []string{"@salesforce/schema/Shape__c.CurrencyIsoCode"})
		if err != nil {
			t.Fatal(err)
		}
		object, ok := cfg.Schema["Shape__c"]
		if !ok {
			t.Fatal("custom object missing")
		}
		return object
	}
	plain := load(`[]`)
	multi := load(`["MultiCurrency"]`)
	if plain.Fields["CurrencyIsoCode"] || !multi.Fields["CurrencyIsoCode"] {
		t.Fatalf("currency feature admission: plain %v multi %v", plain.Fields["CurrencyIsoCode"], multi.Fields["CurrencyIsoCode"])
	}
	delete(multi.Fields, "CurrencyIsoCode")
	if !reflect.DeepEqual(plain, multi) {
		t.Fatalf("unrelated imports changed: plain %#v multi %#v", plain, multi)
	}
}
