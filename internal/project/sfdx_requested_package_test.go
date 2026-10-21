package project

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestLoadRequestedSiblingPackageDirectories(t *testing.T) {
	for _, tc := range []struct {
		name         string
		dependencies string
		wantFiles    []string
	}{
		{
			name:         "nondefault_package",
			dependencies: `[{"package":"GroveSupport"}]`,
			wantFiles:    []string{"GroveSupport.cls"},
		},
		{
			name:         "transitive_local_packages",
			dependencies: `[{"package":"GroveLibrary"}]`,
			wantFiles:    []string{"GroveLibrary.cls", "GroveSupport.cls"},
		},
		{
			name:         "union_requests_from_one_supplier",
			dependencies: `[{"package":"GroveLibrary"},{"package":"GroveExtension"}]`,
			wantFiles:    []string{"GroveExtension.cls", "GroveLeaf.cls", "GroveLibrary.cls", "GroveSupport.cls"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workspace := t.TempDir()
			supplier := filepath.Join(workspace, "supplier")
			consumer := filepath.Join(workspace, "consumer")
			writeFile(t, filepath.Join(supplier, "sfdx-project.json"), `{
  "packageDirectories": [
    {"path":"scratch","default":true},
    {"path":"library","package":"GroveLibrary","dependencies":[{"package":"GroveSupport"}]},
    {"path":"support","package":"GroveSupport"},
    {"path":"extension","package":"GroveExtension","dependencies":[{"package":"GroveLeaf"}]},
    {"path":"leaf","package":"GroveLeaf"},
    {"path":"unrelated","package":"GroveUnrelated","dependencies":[{"package":"GroveUnavailable"}]}
  ]
}`)
			writeFile(t, filepath.Join(supplier, "scratch/.gitkeep"), "")
			for directory, class := range map[string]string{
				"library":   "GroveLibrary",
				"support":   "GroveSupport",
				"extension": "GroveExtension",
				"leaf":      "GroveLeaf",
				"unrelated": "GroveUnrelated",
			} {
				writeFile(t, filepath.Join(supplier, directory, "main/default/classes", class+".cls"), "public class "+class+" {}")
			}
			writeFile(t, filepath.Join(consumer, "sfdx-project.json"), `{
  "packageDirectories": [{"path":"force-app","default":true,"package":"GroveConsumer","dependencies":`+tc.dependencies+`}]
}`)
			writeFile(t, filepath.Join(consumer, "force-app/main/default/classes/GroveConsumer.cls"), "public class GroveConsumer {}")

			loaded, err := Load(consumer)
			if err != nil {
				t.Fatal(err)
			}
			if len(loaded.DependencyDiagnostics) != 0 {
				t.Fatalf("dependency diagnostics = %#v", loaded.DependencyDiagnostics)
			}
			if len(loaded.ManagedPackageDependencies) != 1 {
				t.Fatalf("dependencies = %#v, want one supplier", loaded.ManagedPackageDependencies)
			}
			dependency := loaded.ManagedPackageDependencies[0]
			if dependency.Status != "loaded" || dependency.SourceRoot != supplier || dependency.Project == nil {
				t.Fatalf("dependency = %#v", dependency)
			}
			var gotFiles []string
			for _, file := range dependency.Project.ApexFiles {
				gotFiles = append(gotFiles, filepath.Base(file))
			}
			sort.Strings(gotFiles)
			if !reflect.DeepEqual(gotFiles, tc.wantFiles) {
				t.Fatalf("loaded dependency Apex files = %v, want requested packages and their local dependencies %v", gotFiles, tc.wantFiles)
			}
		})
	}
}

func TestLoadRequestedSiblingPackagesRevisitSupplierThroughIntermediate(t *testing.T) {
	workspace := t.TempDir()
	supplier := filepath.Join(workspace, "supplier")
	intermediate := filepath.Join(workspace, "intermediate")
	consumer := filepath.Join(workspace, "consumer")
	writeFile(t, filepath.Join(supplier, "sfdx-project.json"), `{
  "packageDirectories": [
    {"path":"scratch","default":true},
    {"path":"first","package":"GroveFirst","dependencies":[{"package":"GroveMiddle"}]},
    {"path":"last","package":"GroveLast"}
  ]
}`)
	writeFile(t, filepath.Join(supplier, "scratch/.gitkeep"), "")
	writeFile(t, filepath.Join(supplier, "first/main/default/classes/GroveFirst.cls"), "public class GroveFirst {}")
	writeFile(t, filepath.Join(supplier, "last/main/default/classes/GroveLast.cls"), "public class GroveLast {}")
	writeFile(t, filepath.Join(intermediate, "sfdx-project.json"), `{
  "packageDirectories": [{"path":"middle","default":true,"package":"GroveMiddle","dependencies":[{"package":"GroveLast"}]}]
}`)
	writeFile(t, filepath.Join(intermediate, "middle/main/default/classes/GroveMiddle.cls"), "public class GroveMiddle {}")
	writeFile(t, filepath.Join(consumer, "sfdx-project.json"), `{
  "packageDirectories": [{"path":"force-app","default":true,"package":"GroveConsumer","dependencies":[{"package":"GroveFirst"}]}]
}`)
	writeFile(t, filepath.Join(consumer, "force-app/main/default/classes/GroveConsumer.cls"), "public class GroveConsumer {}")

	loaded, err := Load(consumer)
	if err != nil {
		t.Fatal(err)
	}
	var gotFiles []string
	var collectDependencies func(Project)
	collectDependencies = func(p Project) {
		t.Helper()
		if len(p.DependencyDiagnostics) != 0 {
			t.Fatalf("dependency diagnostics for %s = %#v", p.Root, p.DependencyDiagnostics)
		}
		for _, dependency := range p.ManagedPackageDependencies {
			if dependency.Status != "loaded" || dependency.Project == nil {
				t.Fatalf("dependency = %#v", dependency)
			}
			for _, file := range dependency.Project.ApexFiles {
				gotFiles = append(gotFiles, filepath.Base(file))
			}
			collectDependencies(*dependency.Project)
		}
	}
	collectDependencies(loaded)
	sort.Strings(gotFiles)
	wantFiles := []string{"GroveFirst.cls", "GroveLast.cls", "GroveMiddle.cls"}
	if !reflect.DeepEqual(gotFiles, wantFiles) {
		t.Fatalf("loaded dependency Apex files = %v, want acyclic package chain %v", gotFiles, wantFiles)
	}
}
