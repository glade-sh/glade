package sobject

import (
	"github.com/glade-sh/glade/internal/schema"
	"testing"
)

func TestMasterDetailRequirednessPreservesLookupMetadata(t *testing.T) {
	input := schema.Schema{Objects: []schema.Object{{Name: "OwnedDetail__c", Fields: []schema.Field{
		{Name: "Master__c", Type: "MasterDetail", ReferenceTo: []string{"Account"}},
		{Name: "Optional__c", Type: "Lookup", ReferenceTo: []string{"Account"}},
		{Name: "Required__c", Type: "Lookup", ReferenceTo: []string{"Account"}, Required: true},
	}}}}
	registry := BuildDescribeRegistry(input)
	definition := ToObjectDefinition(registry.Objects["OwnedDetail__c"])
	for name, want := range map[string]bool{"Master__c": true, "Optional__c": false, "Required__c": true} {
		if got := definition.Fields[name].Required; got != want {
			t.Errorf("%s required=%v want=%v", name, got, want)
		}
	}
	if input.Objects[0].Fields[0].Required {
		t.Fatal("raw metadata changed")
	}
}
