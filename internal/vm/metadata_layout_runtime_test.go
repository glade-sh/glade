package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestMetadataLayoutRetrieveBuildsLocalLayoutShape(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Case"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Case",
		Fields: map[string]storage.Field{
			"Subject": {APIName: "Subject", Type: storage.FieldString},
			"Status":  {APIName: "Status", Type: storage.FieldPicklist},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)

	got, err := machine.metadataRetrieve([]Value{
		{Kind: ValueObject, Type: "Metadata.MetadataType", Text: "Layout"},
		List(String("Case-Case Layout")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != ValueList || len(got.List) != 1 || got.List[0].Type != "Metadata.Layout" {
		t.Fatalf("layout result = %#v", got)
	}
	sections := got.List[0].Fields["layoutSections"]
	if sections.Kind != ValueList || len(sections.List) != 1 {
		t.Fatalf("layout sections = %#v", sections)
	}
	columns := sections.List[0].Fields["layoutColumns"]
	items := columns.List[0].Fields["layoutItems"]
	if columns.Kind != ValueList || len(columns.List) != 1 || items.Kind != ValueList || len(items.List) != 2 {
		t.Fatalf("layout columns/items = %#v", sections)
	}
}
