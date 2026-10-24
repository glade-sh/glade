package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestLocalDependencyMappingNativeConformance(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			apiDir := "api" + strings.TrimSuffix(api, ".0")
			fixtureRoot := filepath.Join("testdata", "conformance", "local_dependency_mapping", "native", apiDir)
			rowPath := filepath.Join(fixtureRoot, "org.tsv")
			rowData, err := os.ReadFile(rowPath)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(rowData)), "\n")
			if len(lines) != 2 || lines[0] != "case_id\toutcome\tmessage" {
				t.Fatalf("native rows = %q, want one captured case", rowData)
			}
			fields := strings.SplitN(lines[1], "\t", 3)
			if len(fields) != 3 || fields[0] != "unnamespaced_library_call" || fields[1] != "accepted" {
				t.Fatalf("native row = %q, want accepted unnamespaced library call", lines[1])
			}

			workspace := t.TempDir()
			consumerRoot := filepath.Join(workspace, "consumer")
			dependencyRoot := filepath.Join(workspace, "vendor", "library")
			consumerClasses := filepath.Join(consumerRoot, "force-app", "main", "default", "classes")
			dependencyClasses := filepath.Join(dependencyRoot, "classes")
			for _, className := range []string{"MappedConsumer", "MappedConsumerTest"} {
				copyLocalDependencyFixtureFile(t, fixtureRoot, consumerClasses, className+".cls")
				copyLocalDependencyFixtureFile(t, fixtureRoot, consumerClasses, className+".cls-meta.xml")
			}
			copyLocalDependencyFixtureFile(t, fixtureRoot, dependencyClasses, "MappedLibrary.cls")
			copyLocalDependencyFixtureFile(t, fixtureRoot, dependencyClasses, "MappedLibrary.cls-meta.xml")
			writeLocalDependencyMappingFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), fmt.Sprintf(`{
  "sourceApiVersion":"%s",
  "packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"MappedAlias","versionNumber":"1.0.0.LATEST"}]}],
  "packageAliases":{"MappedAlias":"0Ho000000000000AAA"}
}`, api))
			writeLocalDependencyMappingFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  dependencies:
    - name: MappedAlias
      path: ../vendor/library
`)

			loaded, err := project.Load(consumerRoot)
			if err != nil {
				t.Fatalf("load local dependency project: %v", err)
			}
			if len(loaded.DependencyDiagnostics) != 0 || len(loaded.ManagedPackageDependencies) != 1 {
				t.Fatalf("dependencies=%#v diagnostics=%#v", loaded.ManagedPackageDependencies, loaded.DependencyDiagnostics)
			}
			dependency := loaded.ManagedPackageDependencies[0]
			if dependency.Status != "loaded" || dependency.Namespace != "" || dependency.Project == nil {
				t.Fatalf("dependency = %#v, want a loaded unnamespaced source dependency", dependency)
			}
			if len(dependency.Project.ApexFiles) != 1 || filepath.Base(dependency.Project.ApexFiles[0]) != "MappedLibrary.cls" {
				t.Fatalf("dependency Apex files = %#v", dependency.Project.ApexFiles)
			}

			index := typesys.Build(loaded, schema.Schema{})
			diagnostics := append([]diagnostic.Diagnostic(nil), index.Diagnostics...)
			diagnostics = append(diagnostics, sema.Analyze(index).Diagnostics...)
			for _, got := range diagnostics {
				if got.Severity == diagnostic.Error {
					t.Fatalf("native accepted %s; Glade rejected with %s: %s", fields[0], got.Code, got.Message)
				}
			}
			run := Run(index, Options{SelectedClasses: []string{"MappedConsumerTest"}, NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || !summary.OK() {
				t.Fatalf("native accepted %s; Glade test result = %#v", fields[0], summary)
			}
		})
	}
}

func copyLocalDependencyFixtureFile(t *testing.T, fixtureRoot, destinationRoot, name string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixtureRoot, "force-app", "main", "default", "classes", name))
	if err != nil {
		t.Fatal(err)
	}
	writeLocalDependencyMappingFile(t, filepath.Join(destinationRoot, name), string(raw))
}

func writeLocalDependencyMappingFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
