package project

import (
	"path/filepath"
	"sort"
	"testing"
)

// Two project.dependencies entries may name different packages of the same
// source project. Each entry selects its own package directories, so both
// packages' classes must load.
func TestLoadConfiguredDependenciesSameSourceDifferentPackages(t *testing.T) {
	for _, tc := range []struct {
		name string
		yml  string
	}{
		{"explicit package", `project:
  dependencies:
    - name: BaseLibrary
      path: ../vendor/supplier
      package: BaseLibrary
    - name: ToolLibrary
      path: ../vendor/supplier
      package: ToolLibrary
`},
		{"package from name", `project:
  dependencies:
    - name: BaseLibrary
      path: ../vendor/supplier
    - name: ToolLibrary
      path: ../vendor/supplier
`},
	} {
		t.Run(tc.name, func(t *testing.T) { checkSameSourceDependencies(t, tc.yml) })
	}
}

func checkSameSourceDependencies(t *testing.T, yml string) {
	workspace := t.TempDir()
	consumerRoot := filepath.Join(workspace, "consumer")
	supplierRoot := filepath.Join(workspace, "vendor", "supplier")
	writeFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{
  "packageDirectories": [{"path":"force-app","default":true,"dependencies":[{"package":"BaseLibrary"},{"package":"ToolLibrary"}]}]
}`)
	writeFile(t, filepath.Join(consumerRoot, "glade.yml"), yml)
	writeFile(t, filepath.Join(consumerRoot, "force-app/main/default/classes/Consumer.cls"), "public class Consumer {}")
	writeFile(t, filepath.Join(supplierRoot, "sfdx-project.json"), `{
  "packageDirectories": [
    {"path":"base","package":"BaseLibrary","default":true},
    {"path":"tools","package":"ToolLibrary"}
  ]
}`)
	writeFile(t, filepath.Join(supplierRoot, "base/main/default/classes/BaseService.cls"), "global class BaseService {}")
	writeFile(t, filepath.Join(supplierRoot, "tools/main/default/classes/ToolService.cls"), "global class ToolService {}")

	loaded, err := Load(consumerRoot)
	if err != nil {
		t.Fatalf("load consumer: %v", err)
	}
	if len(loaded.DependencyDiagnostics) != 0 {
		t.Fatalf("diagnostics=%#v", loaded.DependencyDiagnostics)
	}
	var classes []string
	for _, dep := range loaded.ManagedPackageDependencies {
		if dep.Project == nil {
			continue
		}
		for _, file := range dep.Project.ApexFiles {
			classes = append(classes, filepath.Base(file))
		}
	}
	sort.Strings(classes)
	if len(classes) != 2 || classes[0] != "BaseService.cls" || classes[1] != "ToolService.cls" {
		t.Fatalf("dependency classes = %v, want BaseService.cls and ToolService.cls (deps=%d)", classes, len(loaded.ManagedPackageDependencies))
	}
}
