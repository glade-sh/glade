package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
)

func TestRichTextMetadataUsesPublicTextAreaDisplayType(t *testing.T) {
	registry := sobject.BuildDescribeRegistry(schema.Schema{Objects: []schema.Object{{
		Name: "MessagePanel__c", Label: "Message Panel", NameField: schema.NameField{Type: "Text", Label: "Panel Name"},
		Fields: []schema.Field{
			{Name: "Markup__c", Type: "Html", Length: 32768},
			{Name: "Notes__c", Type: "LongTextArea", Length: 32768},
			{Name: "Caption__c", Type: "Text", Length: 40},
		},
	}}})
	describe, err := registry.Describe("MessagePanel__c")
	if err != nil {
		t.Fatal(err)
	}
	definition := sobject.ToObjectDefinition(describe)
	// Keep the internal HTML distinction: only the public getType projection
	// is shared with ordinary text areas.
	if definition.Fields["Markup__c"].DisplayType != "RICHTEXTAREA" {
		t.Fatal("rich-text import provenance was lost")
	}
	org := storage.NewOrgState()
	org.Objects["MessagePanel__c"] = storage.ObjectState{Definition: definition}
	machine := New(nil)
	machine.SetOrg(&org)
	for _, control := range []struct{ name, body string }{
		{"plain text type", `System.assertEquals(Schema.DisplayType.STRING,MessagePanel__c.Caption__c.getDescribe().getType());`},
		{"long text type", `System.assertEquals(Schema.DisplayType.TEXTAREA,MessagePanel__c.Notes__c.getDescribe().getType());`},
		{"HTML distinction retained", `System.assertEquals(true,MessagePanel__c.Markup__c.getDescribe().isHtmlFormatted()); System.assertEquals(false,MessagePanel__c.Notes__c.getDescribe().isHtmlFormatted());`},
		{"declared lengths retained", `System.assertEquals(32768,MessagePanel__c.Markup__c.getDescribe().getLength()); System.assertEquals(40,MessagePanel__c.Caption__c.getDescribe().getLength());`},
		{"long text filtering retained", `System.assertEquals(false,MessagePanel__c.Markup__c.getDescribe().isFilterable()); System.assertEquals(false,MessagePanel__c.Notes__c.getDescribe().isFilterable());`},
		{"rich text public type", `System.assertEquals(Schema.DisplayType.TEXTAREA,MessagePanel__c.Markup__c.getDescribe().getType());`},
	} {
		t.Run(control.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(control.body, CompileOptions{APIVersion: "61.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
