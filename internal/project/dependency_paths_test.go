package project

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadResolvesSFDXPackageDependencyFromConfiguredDependencyPaths(t *testing.T) {
	root := t.TempDir()
	consumer := filepath.Join(root, "work", "consumer")
	libs := filepath.Join(root, "elsewhere", "libs")
	writeFile(t, filepath.Join(consumer, "sfdx-project.json"), `{
  "packageDirectories": [{"path":"force-app","default":true,"package":"consumer","dependencies":[{"package":"shared-base"},{"package":"absent-pkg"}]}]
}`)
	writeFile(t, filepath.Join(consumer, "glade.yml"), "project:\n  dependencyPaths: [\"../../elsewhere/libs\"]\n")
	writeFile(t, filepath.Join(consumer, "force-app/main/default/classes/Consumer.cls"), "public class Consumer {}")
	writeFile(t, filepath.Join(libs, "base-repo/sfdx-project.json"), `{
  "packageDirectories": [{"path":"src","default":true,"package":"shared-base"}]
}`)
	writeFile(t, filepath.Join(libs, "base-repo/src/main/default/classes/BaseHelper.cls"), "public class BaseHelper {}")

	p, err := Load(consumer)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ManagedPackageDependencies) != 1 || p.ManagedPackageDependencies[0].Status != "loaded" {
		t.Fatalf("dependencies = %#v, want shared-base loaded", p.ManagedPackageDependencies)
	}
	if len(p.DependencyDiagnostics) != 1 || p.DependencyDiagnostics[0].Namespace != "absent-pkg" {
		t.Fatalf("diagnostics = %#v, want only absent-pkg missing", p.DependencyDiagnostics)
	}
	msg := p.DependencyDiagnostics[0].Message
	if !strings.Contains(msg, `"absent-pkg"`) || !strings.Contains(msg, "dependencyPaths") {
		t.Fatalf("message %q should name the package and the glade.yml key to add", msg)
	}
}

func TestLoadDependencyPathsBoundedDeclaredLookup(t *testing.T) {
	for _, route := range []string{"direct", "immediate_child", "ancestor_config", "own_config_precedence", "ambiguous", "nested_boundary", "undeclared_boundary"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			consumer := filepath.Join(root, "workspace", "consumer")
			library := filepath.Join(root, "external", "library")
			path := "../../external/library"
			if route == "immediate_child" || route == "ambiguous" || route == "nested_boundary" {
				path = "../../external"
			}
			if route == "nested_boundary" {
				library = filepath.Join(library, "nested")
			}
			dependencies := `[{"package":"SharedLibrary"}]`
			if route == "undeclared_boundary" {
				dependencies = "[]"
			}
			writeFile(t, filepath.Join(consumer, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true,"dependencies":`+dependencies+`}]}`)
			configRoot := consumer
			if route == "ancestor_config" {
				configRoot = filepath.Dir(consumer)
				path = "../external/library"
			}
			writeFile(t, filepath.Join(configRoot, "glade.yml"), "project:\n  dependencyPaths: [\""+path+"\"]\n")
			if route == "own_config_precedence" {
				writeFile(t, filepath.Join(filepath.Dir(consumer), "glade.yml"), "project:\n  dependencyPaths: [\"../missing\"]\n")
			}
			manifest := `{"packageDirectories":[{"path":"force-app","default":true,"package":"SharedLibrary"}]}`
			writeFile(t, filepath.Join(library, "sfdx-project.json"), manifest)
			writeFile(t, filepath.Join(library, "force-app/Shared.cls"), "public class Shared {}")
			if route == "ambiguous" {
				writeFile(t, filepath.Join(root, "external", "duplicate", "sfdx-project.json"), manifest)
			}
			p, err := Load(consumer)
			if err != nil {
				t.Fatal(err)
			}
			switch route {
			case "ambiguous", "nested_boundary":
				wantCode := "dependency_ambiguous"
				if route == "nested_boundary" {
					wantCode = "dependency_missing"
				}
				if len(p.ManagedPackageDependencies) != 0 || len(p.DependencyDiagnostics) != 1 || p.DependencyDiagnostics[0].Code != wantCode {
					t.Fatalf("dependencies=%#v diagnostics=%#v, want %s", p.ManagedPackageDependencies, p.DependencyDiagnostics, wantCode)
				}
			case "undeclared_boundary":
				if len(p.ManagedPackageDependencies) != 0 || len(p.DependencyDiagnostics) != 0 {
					t.Fatalf("undeclared package loaded: %#v", p.ManagedPackageDependencies)
				}
			default:
				if len(p.DependencyDiagnostics) != 0 || len(p.ManagedPackageDependencies) != 1 || !sameFilePath(p.ManagedPackageDependencies[0].SourceRoot, library) {
					t.Fatalf("dependencies=%#v diagnostics=%#v", p.ManagedPackageDependencies, p.DependencyDiagnostics)
				}
			}
		})
	}
}

func TestLoadConfiguredPackageDirsPreservesManifestDeclarationsRoutes(t *testing.T) {
	for _, route := range []string{"consumer_config", "provider_config", "consumer_and_provider_config"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			consumer := filepath.Join(root, "consumer")
			provider := filepath.Join(root, "external", "provider")
			writeFile(t, filepath.Join(consumer, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true,"package":"Consumer","dependencies":[{"package":"SharedLibrary","versionNumber":"1.0.0.LATEST"}]}]}`)
			writeFile(t, filepath.Join(provider, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","package":"SharedLibrary"},{"path":"other","default":true,"package":"UnrelatedLibrary"}]}`)
			writeFile(t, filepath.Join(provider, "src/Shared.cls"), "public class Shared {}")
			writeFile(t, filepath.Join(provider, "other/Unrelated.cls"), "public class Unrelated {}")
			consumerConfig := "project:\n  dependencyPaths: [\"../external\"]\n"
			if route != "provider_config" {
				consumerConfig += "  packageDirs: [\"./force-app/\"]\n"
			}
			writeFile(t, filepath.Join(consumer, "glade.yml"), consumerConfig)
			if route != "consumer_config" {
				writeFile(t, filepath.Join(provider, "glade.yml"), "project:\n  packageDirs: [\"./src/\", \"other\"]\n")
			}
			p, err := Load(consumer)
			if err != nil {
				t.Fatal(err)
			}
			wantDependencies := []PackageDependency{{Package: "SharedLibrary", VersionNumber: "1.0.0.LATEST"}}
			if len(p.PackageDirectories) != 1 || p.PackageDirectories[0].Package != "Consumer" || !p.PackageDirectories[0].Default || !reflect.DeepEqual(p.PackageDirectories[0].Dependencies, wantDependencies) {
				t.Fatalf("consumer manifest declarations lost: %#v", p.PackageDirectories)
			}
			if len(p.DependencyDiagnostics) != 0 || len(p.ManagedPackageDependencies) != 1 || p.ManagedPackageDependencies[0].Project == nil {
				t.Fatalf("dependencies=%#v diagnostics=%#v", p.ManagedPackageDependencies, p.DependencyDiagnostics)
			}
			dep := p.ManagedPackageDependencies[0].Project
			if len(dep.PackageDirectories) != 1 || dep.PackageDirectories[0].Package != "SharedLibrary" || len(dep.ApexFiles) != 1 || filepath.Base(dep.ApexFiles[0]) != "Shared.cls" {
				t.Fatalf("provider package selection lost: dirs=%#v files=%v", dep.PackageDirectories, dep.ApexFiles)
			}
		})
	}
}

func TestLoadConfiguredPackageDirsSelectionBoundariesRoutes(t *testing.T) {
	for _, route := range []string{"config_only", "config_directory_outside_manifest", "selected_manifest_subset"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			selected := "extra"
			if route == "selected_manifest_subset" {
				selected = "force-app"
			}
			if route != "config_only" {
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app"},{"path":"excluded","default":true,"package":"Excluded","dependencies":[{"package":"MissingLibrary"}]}]}`)
			}
			writeFile(t, filepath.Join(root, "glade.yml"), "project:\n  packageDirs: [\""+selected+"\"]\n")
			writeFile(t, filepath.Join(root, selected, "Selected.cls"), "public class Selected {}")
			writeFile(t, filepath.Join(root, "excluded/Excluded.cls"), "public class Excluded {}")
			p, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			want := []PackageDirectory{{Path: selected}}
			if !reflect.DeepEqual(p.PackageDirectories, want) || len(p.ManagedPackageDependencies) != 0 || len(p.DependencyDiagnostics) != 0 || len(p.ApexFiles) != 1 || filepath.Base(p.ApexFiles[0]) != "Selected.cls" {
				t.Fatalf("configured selection broadened: dirs=%#v dependencies=%#v diagnostics=%#v files=%v", p.PackageDirectories, p.ManagedPackageDependencies, p.DependencyDiagnostics, p.ApexFiles)
			}
		})
	}
}
