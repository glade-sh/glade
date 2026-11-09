package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func analyzeLocalWithOptionalDependency(t *testing.T, withDependency bool, consumer string) Result {
	t.Helper()
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	for _, dir := range []string{depRoot, consumerRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeSemaFile(t, filepath.Join(depRoot, "Dependency.cls"), `
global class Dependency {
  global static String hello() { return 'x'; }
}
`)
	writeSemaFile(t, filepath.Join(consumerRoot, "Helper.cls"), `
public class Helper {
  public static Integer count() { return 1; }
}
`)
	writeSemaFile(t, filepath.Join(consumerRoot, "Consumer.cls"), consumer)
	p := project.Project{
		Root:      consumerRoot,
		ApexFiles: []string{filepath.Join(consumerRoot, "Consumer.cls"), filepath.Join(consumerRoot, "Helper.cls")},
	}
	if withDependency {
		depProject := project.Project{
			Root:      depRoot,
			Namespace: "pkgx",
			ApexFiles: []string{filepath.Join(depRoot, "Dependency.cls")},
		}
		p.ManagedPackageDependencies = []project.ManagedPackageDependency{{
			Namespace: "pkgx", SourceRoot: depRoot, Project: &depProject, Status: "loaded",
		}}
	}
	return Analyze(typesys.Build(p, schema.Schema{}))
}

// A project's own broken class fails the same way with or without an
// unrelated source-backed dependency: the dependency's source says nothing
// about a local type or a local method.
func TestSourceDependencyKeepsUnrelatedLocalSemanticErrors(t *testing.T) {
	t.Parallel()
	const consumer = `
public class Consumer {
  public void run() {
    MissingLocal localRecord;
    Integer n = Helper.nonexistent();
    Integer m = Helper.count(1, 2);
  }
}
`
	without := analyzeLocalWithOptionalDependency(t, false, consumer)
	with := analyzeLocalWithOptionalDependency(t, true, consumer)
	count := func(r Result) (errs, warns int) {
		for _, d := range r.Diagnostics {
			switch d.Severity {
			case diagnostic.Error:
				errs++
			case diagnostic.Warning:
				warns++
			}
		}
		return
	}
	e0, w0 := count(without)
	e1, w1 := count(with)
	t.Logf("without dep: %d errors %d warnings; with dep: %d errors %d warnings", e0, w0, e1, w1)
	for _, d := range with.Diagnostics {
		t.Logf("with: %s %s %s", d.Severity, d.Code, d.Message)
	}
	if e0 == 0 {
		t.Fatalf("baseline should report local errors: %#v", without.Diagnostics)
	}
	if e1 != e0 {
		t.Fatalf("an unrelated dependency changed local error count %d -> %d", e0, e1)
	}
}
