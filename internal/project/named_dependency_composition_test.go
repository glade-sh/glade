package project

import (
	"path/filepath"
	"testing"
)

func TestNamedDependencyAndDiscoveryCompositionRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"explicitMapping", "missingExplicitMapping", "discoveryOnly"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				workspace := t.TempDir()
				consumer := filepath.Join(workspace, "consumer")
				library := filepath.Join(workspace, "external", "library")
				explicit := filepath.Join(workspace, "vendor", "library")
				writeFile(t, filepath.Join(consumer, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","package":"Consumer","dependencies":[{"package":"SharedLibrary"},{"package":"OtherLibrary"}]}]}`)
				cfg := "project:\n  packageDirs: [force-app]\n  dependencyPaths: [../external]\n"
				if route != "discoveryOnly" {
					cfg += "  dependencies:\n    - name: SharedLibrary\n      path: ../vendor/library\n"
				}
				writeFile(t, filepath.Join(consumer, "glade.yml"), cfg)
				writeFile(t, filepath.Join(library, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","package":"SharedLibrary"}]}`)
				writeFile(t, filepath.Join(library, "src/Discovered.cls"), "public class Discovered {}")
				other := filepath.Join(workspace, "external", "other")
				writeFile(t, filepath.Join(other, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","package":"OtherLibrary"}]}`)
				writeFile(t, filepath.Join(other, "src/Other.cls"), "public class Other {}")
				if route == "explicitMapping" {
					writeFile(t, filepath.Join(explicit, "Mapped.cls"), "public class Mapped {}")
				}
				p, err := Load(consumer)
				if err != nil {
					t.Fatal(err)
				}
				if len(p.PackageDirectories) != 1 || len(p.PackageDirectories[0].Dependencies) != 2 {
					t.Fatalf("manifest dependency declarations lost: %+v", p.PackageDirectories)
				}
				loaded := map[string]bool{}
				for _, dep := range p.ManagedPackageDependencies {
					loaded[dep.SourceRoot] = true
				}
				if !loaded[other] {
					t.Fatalf("explicit mappings suppressed other discovery: %+v", loaded)
				}
				if route == "missingExplicitMapping" {
					if len(loaded) != 1 || len(p.DependencyDiagnostics) != 1 || p.DependencyDiagnostics[0].Code != "dependency_missing" {
						t.Fatalf("missing mapping fell back to discovery: deps=%+v diagnostics=%+v", loaded, p.DependencyDiagnostics)
					}
					return
				}
				want := library
				if route == "explicitMapping" {
					want = explicit
				}
				if len(loaded) != 2 || !loaded[want] || len(p.DependencyDiagnostics) != 0 {
					t.Fatalf("composition: deps=%+v diagnostics=%+v, want %s", loaded, p.DependencyDiagnostics, want)
				}
			})
		}
	}
}
