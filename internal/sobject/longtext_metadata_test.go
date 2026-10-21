package sobject

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
)

func TestDeclaredLongTextAreaSurvivesMetadataRoundTrips(t *testing.T) {
	for _, objectName := range []string{"BoundaryNote__c", "Account"} {
		t.Run(objectName, func(t *testing.T) {
			registry := BuildDescribeRegistry(schema.Schema{Objects: []schema.Object{{Name: objectName, Fields: []schema.Field{
				{Name: "Transcript__c", Type: "LongTextArea", Length: 32768},
				{Name: "Area__c", Type: "TextArea", Length: 32768},
				{Name: "Text__c", Type: "Text", Length: 32768},
			}}}})
			describe, err := registry.Describe(objectName)
			if err != nil {
				t.Fatal(err)
			}
			definition := ToObjectDefinition(describe)
			// The marker follows the declared metadata type, never a name or length.
			encoded, err := json.Marshal(definition)
			if err != nil {
				t.Fatal(err)
			}
			var decoded storage.ObjectDefinition
			if err = json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			roundTrip := FromObjectDefinition(decoded)
			for _, value := range []DescribeSObjectResult{describe, roundTrip} {
				for name, field := range value.Fields {
					want := name == "Transcript__c"
					if field.LongTextArea != want {
						t.Fatalf("%s marker=%t want=%t", name, field.LongTextArea, want)
					}
				}
				for _, name := range []string{"Transcript__c", "Area__c"} {
					if value.Fields[name].DisplayType != "TEXTAREA" {
						t.Fatalf("describe type changed for %s", name)
					}
				}
			}
		})
	}
}
