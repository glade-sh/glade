package project

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfiguredBareDependencyOverridesImplicitPackageScan(t *testing.T) {
	workspace := t.TempDir()
	consumerRoot := filepath.Join(workspace, "consumer")
	implicitRoot := filepath.Join(workspace, "implicit")
	explicitRoot := filepath.Join(workspace, "vendor", "library")
	managedRoot := filepath.Join(workspace, "managed", "library")
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{
  "namespace": "",
  "packageDirectories": [{"path":"force-app","default":true,"package":"Consumer","dependencies":[{"package":"LibraryAlias","versionNumber":"1.0.0.LATEST"}]}],
  "packageAliases": {"LibraryAlias":"0Ho000000000000AAA"}
}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  dependencies:
    - name: LibraryAlias
      path: ../vendor/library
  managedPackageDependencies:
    - namespace: LibraryAlias
      sourceRoot: ../managed/library
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/classes/Consumer.cls"), "public class Consumer {}")
	writeFile(t, filepath.Join(explicitRoot, "classes/MappedLibrary.cls"), "global class MappedLibrary {}")
	writeFile(t, filepath.Join(managedRoot, "sfdx-project.json"), `{
  "namespace": "LibraryAlias",
  "packageDirectories": [{"path":"force-app","default":true}]
}`)
	writeFile(t, filepath.Join(managedRoot, "force-app/main/default/classes/ManagedLibrary.cls"), "global class ManagedLibrary {}")
	writeFile(t, filepath.Join(implicitRoot, "sfdx-project.json"), `{
  "packageDirectories": [{"path":"force-app","default":true,"package":"LibraryAlias"}]
}`)
	writeFile(t, filepath.Join(implicitRoot, "force-app/main/default/classes/ImplicitLibrary.cls"), "global class ImplicitLibrary {}")

	loaded, err := Load(consumerRoot)
	if err != nil {
		t.Fatalf("load consumer: %v", err)
	}
	if len(loaded.DependencyDiagnostics) != 0 {
		t.Fatalf("dependency diagnostics = %#v", loaded.DependencyDiagnostics)
	}
	if len(loaded.ManagedPackageDependencies) != 1 {
		t.Fatalf("dependencies = %#v, want one configured library", loaded.ManagedPackageDependencies)
	}
	dep := loaded.ManagedPackageDependencies[0]
	if dep.Status != "loaded" || dep.SourceRoot != explicitRoot || dep.Namespace != "" || dep.Project == nil {
		t.Fatalf("configured dependency = %#v", dep)
	}
	if len(dep.Project.ApexFiles) != 1 || filepath.Base(dep.Project.ApexFiles[0]) != "MappedLibrary.cls" {
		t.Fatalf("configured dependency files = %#v, want only the explicit bare source", dep.Project.ApexFiles)
	}
}

func TestLoadConfiguredSFDXDependencySelectsRequestedPackage(t *testing.T) {
	workspace := t.TempDir()
	consumerRoot := filepath.Join(workspace, "consumer")
	supplierRoot := filepath.Join(workspace, "vendor", "supplier")
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{
  "packageDirectories": [{"path":"force-app","default":true,"dependencies":[{"package":"BaseLibrary"}]}]
}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  dependencies:
    - name: BaseLibrary
      path: ../vendor/supplier
      package: BaseLibrary
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/classes/Consumer.cls"), "public class Consumer {}")
	writeFile(t, filepath.Join(supplierRoot, "sfdx-project.json"), `{
  "packageDirectories": [
    {"path":"base","package":"BaseLibrary"},
    {"path":"tools","package":"ToolLibrary"}
  ]
}`)
	writeFile(t, filepath.Join(supplierRoot, "base/main/default/classes/BaseService.cls"), "global class BaseService {}")
	writeFile(t, filepath.Join(supplierRoot, "tools/main/default/classes/ToolService.cls"), "global class ToolService {}")

	loaded, err := Load(consumerRoot)
	if err != nil {
		t.Fatalf("load consumer: %v", err)
	}
	if len(loaded.DependencyDiagnostics) != 0 || len(loaded.ManagedPackageDependencies) != 1 {
		t.Fatalf("dependencies=%#v diagnostics=%#v", loaded.ManagedPackageDependencies, loaded.DependencyDiagnostics)
	}
	dep := loaded.ManagedPackageDependencies[0]
	if dep.Project == nil || dep.Project.Namespace != "" || len(dep.Project.ApexFiles) != 1 || filepath.Base(dep.Project.ApexFiles[0]) != "BaseService.cls" {
		t.Fatalf("selected dependency project = %#v, want only the unnamespaced BaseLibrary source", dep.Project)
	}
}

func TestLoadConfiguredMissingDependencyReportsInsteadOfUsingImplicitScan(t *testing.T) {
	workspace := t.TempDir()
	consumerRoot := filepath.Join(workspace, "consumer")
	implicitRoot := filepath.Join(workspace, "provider")
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{
  "packageDirectories": [{"path":"force-app","default":true,"dependencies":[{"package":"LibraryAlias"}]}],
  "packageAliases": {"LibraryAlias":"0Ho000000000000AAA"}
}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  dependencies:
    - name: LibraryAlias
      path: ../missing/library
`)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/classes/Consumer.cls"), "public class Consumer {}")
	writeFile(t, filepath.Join(implicitRoot, "sfdx-project.json"), `{
  "packageDirectories": [{"path":"force-app","default":true,"package":"LibraryAlias"}]
}`)
	writeFile(t, filepath.Join(implicitRoot, "force-app/main/default/classes/ImplicitLibrary.cls"), "global class ImplicitLibrary {}")

	loaded, err := Load(consumerRoot)
	if err != nil {
		t.Fatalf("load consumer: %v", err)
	}
	if len(loaded.ManagedPackageDependencies) != 0 {
		t.Fatalf("dependencies = %#v, want explicit missing path to block implicit fallback", loaded.ManagedPackageDependencies)
	}
	if len(loaded.DependencyDiagnostics) != 1 || loaded.DependencyDiagnostics[0].Code != "dependency_missing" || !strings.Contains(loaded.DependencyDiagnostics[0].Message, "LibraryAlias") {
		t.Fatalf("dependency diagnostics = %#v, want a named missing-source diagnostic", loaded.DependencyDiagnostics)
	}
}
