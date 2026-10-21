package apextest

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSourceUnitGenerationClassAndCustomMetadataRoutesRequireIndexedRoot(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	file := filepath.Join(root, "Owned.cls")
	writeFile(t, file, "public class Owned {}")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{}`)
	for _, route := range []string{"class", "custom-metadata"} {
		t.Run(route, func(t *testing.T) {
			index := typesys.Build(project.Project{Root: root, ApexFiles: []string{file, file}}, schema.Schema{})
			index.Project.Root = ""
			for i := range index.Types {
				index.Types[i].SourceRoot = ""
			}
			if route == "custom-metadata" {
				index.Types = nil
				index.CustomMetadataRecords = []schema.CustomMetadataRecord{
					{ObjectName: "Owned__mdt", DeveloperName: "Seed", File: file},
					{ObjectName: "Owned__mdt", DeveloperName: "Seed", File: file},
				}
			}
			generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
			if _, err := captureRuntimeSourceUnits(index, &generation, map[string]string{}); err == nil {
				t.Fatal("source-unit discovery accepted a missing indexed root")
			}
		})
	}
}

func TestSourceUnitGenerationRelativeRootRouteStopsBeforeParentMarker(t *testing.T) {
	parent := t.TempDir()
	t.Chdir(parent)
	writeFile(t, filepath.Join(parent, "sfdx-project.json"), `{}`)
	file := filepath.Join(parent, "source", "Owned.cls")
	writeFile(t, file, "public class Owned {}")
	index := typesys.Build(project.Project{Root: "source", ApexFiles: []string{file, file}}, schema.Schema{})
	generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
	units, err := captureRuntimeSourceUnits(index, &generation, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if units[file] != "file:"+file || len(generation.inputs) != 1 {
		t.Fatalf("relative indexed root crossed into parent: units=%v inputs=%v", units, generation.inputs)
	}
}
