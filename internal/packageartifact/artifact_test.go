package packageartifact

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

func TestBuildAppliesInstalledNamespaceToCustomSchema(t *testing.T) {
	artifact, err := Build("pkg", "1.0", project.Project{Root: t.TempDir()}, schema.Schema{Objects: []schema.Object{
		{
			Name: "CartItemLine__c",
			Fields: []schema.Field{
				{Name: "Product__c", Type: "reference", ReferenceTo: []string{"Membership__c"}},
				{Name: "Name", Type: "string"},
			},
		},
		{
			Name:   "Account",
			Fields: []schema.Field{{Name: "Name", Type: "string"}},
		},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Objects[0].Name != "pkg__CartItemLine__c" {
		t.Fatalf("object name = %q", artifact.Objects[0].Name)
	}
	if artifact.Objects[0].Fields[0].Name != "pkg__Product__c" {
		t.Fatalf("field name = %q", artifact.Objects[0].Fields[0].Name)
	}
	if artifact.Objects[0].Fields[0].ReferenceTo[0] != "pkg__Membership__c" {
		t.Fatalf("referenceTo = %#v", artifact.Objects[0].Fields[0].ReferenceTo)
	}
	if artifact.Objects[0].Fields[1].Name != "Name" {
		t.Fatalf("standard field name = %q", artifact.Objects[0].Fields[1].Name)
	}
	if artifact.Objects[1].Name != "Account" {
		t.Fatalf("standard object name = %q", artifact.Objects[1].Name)
	}
}
