package vm

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Describe-only rich-text facets must not rewrite stored definitions or replace
// an explicit schema length/permission flag with an unrelated catalog default.
func TestRichTextCatalogDescribePreservesExplicitDefinition(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "LiveChatTranscript")
	object := org.Objects["LiveChatTranscript"]
	field := object.Definition.Fields["Body"]
	field.Length = 512
	field.Accessible = storage.BoolFlag(false)
	field.Createable = storage.BoolFlag(false)
	field.Updateable = storage.BoolFlag(false)
	object.Definition.Fields["Body"] = field
	org.Objects["LiveChatTranscript"] = object
	machine := New(nil)
	machine.SetOrg(&org)
	program, err := CompileAnonymous(`
Schema.DescribeFieldResult detail=Schema.LiveChatTranscript.Body.getDescribe();
System.assertEquals(Schema.DisplayType.TEXTAREA,detail.getType());
System.assertEquals(true,detail.isHtmlFormatted());
System.assertEquals(512,detail.getLength());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if got := machine.Org.Objects["LiveChatTranscript"].Definition.Fields["Body"]; !reflect.DeepEqual(got, field) {
		t.Fatalf("describe mutated stored definition: %#v", got)
	}
}
