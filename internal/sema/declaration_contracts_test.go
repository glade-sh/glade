package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func analyzeDeclarationProject(t *testing.T, files map[string]string) Result {
	t.Helper()
	root := t.TempDir()
	paths := make([]string, 0, len(files))
	for name, contents := range files {
		path := filepath.Join(root, name)
		writeSemaFile(t, path, contents)
		paths = append(paths, path)
	}
	return Analyze(typesys.Build(project.Project{Root: root, ApexFiles: paths}, schema.Schema{}))
}

func declarationDiagnosticMatching(result Result, substring string) bool {
	for _, diag := range result.Diagnostics {
		if strings.Contains(strings.ToLower(diag.Message), strings.ToLower(substring)) {
			return true
		}
	}
	return false
}

func TestDuplicateDeclarationSameOwnerClassAndInterface(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"ProbeDuplicateTypeName.cls": `
public class ProbeDuplicateTypeName {
  class Item {}
  interface Item {}
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected same-owner class/interface duplicate error, got %#v", result.Diagnostics)
	}
	found := false
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADETYPE001" && diag.Severity == diagnostic.Error {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected GLADETYPE001 error, got %#v", result.Diagnostics)
	}
}

func TestDuplicateDeclarationCrossFileWorkspaceAmbiguityRemainsWarning(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"One.cls":  "public class Hello {}",
		"Two.cls":  "public class hello {}",
	})
	if result.HasErrors() {
		t.Fatalf("cross-file workspace ambiguity should remain warning-only: %#v", result.Diagnostics)
	}
	found := false
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADETYPE001" && diag.Severity == diagnostic.Warning {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected GLADETYPE001 warning for cross-file ambiguity, got %#v", result.Diagnostics)
	}
}

func TestInnerTypeEqualToAncestor(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Holder.cls": `
public class Holder {
  class Mid {
    class Holder {}
  }
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected inner type equal to ancestor error, got %#v", result.Diagnostics)
	}
	if !declarationDiagnosticMatching(result, "ancestor") && !declarationDiagnosticMatching(result, "already in use") {
		t.Fatalf("expected ancestor/type-name diagnostic, got %#v", result.Diagnostics)
	}
}

func TestDuplicateMemberCaseInsensitiveFields(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"DupFields.cls": `
public class DupFields {
  public Integer value;
  public String Value;
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected duplicate field error, got %#v", result.Diagnostics)
	}
	if !declarationDiagnosticMatching(result, "value") {
		t.Fatalf("expected diagnostic naming the field, got %#v", result.Diagnostics)
	}
}

func TestDuplicateMemberRemainsErrorWithSourceBackedDependency(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	for _, dir := range []string{depRoot, consumerRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	depFile := filepath.Join(depRoot, "DepHelper.cls")
	consumerFile := filepath.Join(consumerRoot, "DupFields.cls")
	writeSemaFile(t, depFile, `
global class DepHelper {
  global static String ok() { return 'ok'; }
}
`)
	writeSemaFile(t, consumerFile, `
public class DupFields {
  public Integer value;
  public String Value;
}
`)
	depProject := project.Project{
		Root:      depRoot,
		Namespace: "deppkg",
		ApexFiles: []string{depFile},
	}
	result := Analyze(typesys.Build(project.Project{
		Root:      consumerRoot,
		ApexFiles: []string{consumerFile},
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "deppkg",
			SourceRoot: depRoot,
			Project:    &depProject,
			Status:     "loaded",
		}},
	}, schema.Schema{}))
	if !result.HasErrors() {
		t.Fatalf("same-owner duplicate member must remain an error with source-backed deps: %#v", result.Diagnostics)
	}
	found := false
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA031" && diag.Severity == diagnostic.Error {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected GLADESEMA031 error, got %#v", result.Diagnostics)
	}
}

func TestDuplicateMemberCaseInsensitiveProperties(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"DupProps.cls": `
public class DupProps {
  public Integer value { get; set; }
  public String Value { get; set; }
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected duplicate property error, got %#v", result.Diagnostics)
	}
	if !declarationDiagnosticMatching(result, "value") {
		t.Fatalf("expected diagnostic naming the property, got %#v", result.Diagnostics)
	}
}

func TestDuplicateMemberConstructors(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"DupCtor.cls": `
public class DupCtor {
  public DupCtor(Integer value) {}
  public DupCtor(Integer other) {}
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected duplicate constructor error, got %#v", result.Diagnostics)
	}
	if !declarationDiagnosticMatching(result, "constructor") {
		t.Fatalf("expected constructor diagnostic, got %#v", result.Diagnostics)
	}
}

func TestDuplicateMemberMethods(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"DupMethods.cls": `
public class DupMethods {
  public void run(Integer value) {}
  public void run(Integer other) {}
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected duplicate method error, got %#v", result.Diagnostics)
	}
	if !declarationDiagnosticMatching(result, "run") {
		t.Fatalf("expected diagnostic naming the method, got %#v", result.Diagnostics)
	}
}

func TestDuplicateMemberMethodsDifferOnlyByReturnType(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"DupReturn.cls": `
public class DupReturn {
  public Integer run(Integer value) { return value; }
  public String run(Integer value) { return String.valueOf(value); }
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected return-type-only overload to be an error, got %#v", result.Diagnostics)
	}
	if !declarationDiagnosticMatching(result, "run") {
		t.Fatalf("expected diagnostic naming the method, got %#v", result.Diagnostics)
	}
}

func TestDuplicateMemberCaseOnlyMethodNames(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"DupCase.cls": `
public class DupCase {
  public void run() {}
  public void Run() {}
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected case-only method name collision error, got %#v", result.Diagnostics)
	}
	if !declarationDiagnosticMatching(result, "run") {
		t.Fatalf("expected diagnostic naming the method, got %#v", result.Diagnostics)
	}
}
