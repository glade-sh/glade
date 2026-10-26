package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// API62/67 native org.tsv:53-55,117-119 retains explicit raw null but
// exposes false through dot access, even when the schema default is true.
func TestMetadataDefaultsExplicitNullDotAndGetRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Objects["DefaultProbe__mdt"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "DefaultProbe__mdt", Fields: map[string]storage.Field{
				"Enabled__c": {APIName: "Enabled__c", Type: storage.FieldBoolean, DefaultValue: "true"},
			}}}
			machine := New(nil)
			machine.SetOrg(&org)
			program, err := CompileAnonymousWithOptions(`DefaultProbe__mdt row=new DefaultProbe__mdt(Enabled__c=null);
System.assertEquals(false,row.Enabled__c);
System.assertEquals(null,row.get('Enabled__c'));
System.assertEquals(null,row.get(DefaultProbe__mdt.Enabled__c));
System.assertEquals(true,row.getPopulatedFieldsAsMap().containsKey('Enabled__c'));`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
