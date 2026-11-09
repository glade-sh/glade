package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native expression rows: describe_text_default_expressions/org.tsv:10,20.
func TestDescribeDefaultMetadataLiteralScope(t *testing.T) {
	org := testDataOrg()
	account := org.Objects["Account"]
	for name, field := range map[string]storage.Field{
		"Literal__c":    {APIName: "Literal__c", Type: storage.FieldString, DisplayType: "STRING", DefaultValue: "'seed'", DefaultValueFromMetadata: true},
		"Expression__c": {APIName: "Expression__c", Type: storage.FieldString, DisplayType: "STRING", DefaultValue: "'a' + 'b'", DefaultValueFromMetadata: true},
		"Catalog__c":    {APIName: "Catalog__c", Type: storage.FieldString, DisplayType: "STRING", DefaultValue: "'seed'"},
	} {
		account.Definition.Fields[name] = field
	}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	for _, tc := range []struct {
		name   string
		isNull bool
	}{{"Literal__c", true}, {"Expression__c", true}, {"Catalog__c", false}} {
		description, err := machine.describeFieldValue("Account", tc.name)
		if err != nil {
			t.Fatal(err)
		}
		if got := description.Fields["defaultValue"].Kind == ValueNull; got != tc.isNull {
			t.Errorf("%s null=%v, want %v", tc.name, got, tc.isNull)
		}
	}
}
