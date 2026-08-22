package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAnalyzeSystemLocationNewInstance(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	file := filepath.Join(root, "UsesSystemLocation.cls")
	writeSemaFile(t, file, `
public class UsesSystemLocation {
  public Location make() {
    return System.Location.newInstance(37.775, -122.418);
  }
}
`)

	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{file}}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("System.Location.newInstance should resolve: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSchemaLocationRemainsSchemaObject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	file := filepath.Join(root, "UsesSchemaLocation.cls")
	writeSemaFile(t, file, `
public class UsesSchemaLocation {
  public String name(Schema.Location location) {
    return location.Name;
  }
}
`)

	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{file}}, schema.Schema{
		Objects: []schema.Object{{Name: "Location"}},
	})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("Schema.Location should remain an SObject type: %#v", result.Diagnostics)
	}
}
