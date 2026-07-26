package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func analyzeScopeProject(t *testing.T, source string) Result {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "Probe.cls")
	writeSemaFile(t, path, source)
	return Analyze(typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{}))
}

func hasScopeRedeclareDiagnostic(result Result, name string) bool {
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA014" && strings.Contains(diag.Message, `"`+name+`"`) {
			return true
		}
	}
	return false
}

func TestDuplicateParametersAreRejected(t *testing.T) {
	t.Parallel()
	result := analyzeScopeProject(t, `
public class Probe {
  void run(String value, Integer Value) {}
}
`)
	if !hasScopeRedeclareDiagnostic(result, "Value") && !hasScopeRedeclareDiagnostic(result, "value") {
		t.Fatalf("expected duplicate parameter diagnostic, got %#v", result.Diagnostics)
	}
}

func TestDuplicateLocalsAreRejected(t *testing.T) {
	t.Parallel()
	result := analyzeScopeProject(t, `
public class Probe {
  void run() {
    String value = 'a';
    String Value = 'b';
  }
}
`)
	if !hasScopeRedeclareDiagnostic(result, "Value") && !hasScopeRedeclareDiagnostic(result, "value") {
		t.Fatalf("expected duplicate local diagnostic, got %#v", result.Diagnostics)
	}
}

func TestParentBlockRedeclarationIsRejected(t *testing.T) {
	t.Parallel()
	result := analyzeScopeProject(t, `
public class Probe {
  void run() {
    String value = 'a';
    {
      String value = 'b';
    }
  }
}
`)
	if !hasScopeRedeclareDiagnostic(result, "value") {
		t.Fatalf("expected parent-block redeclaration diagnostic, got %#v", result.Diagnostics)
	}
}

func TestLoopVariableCollidesWithOuterLocal(t *testing.T) {
	t.Parallel()
	result := analyzeScopeProject(t, `
public class Probe {
  void run() {
    String value = 'a';
    for (String value : new List<String>()) {}
  }
}
`)
	if !hasScopeRedeclareDiagnostic(result, "value") {
		t.Fatalf("expected loop/local collision diagnostic, got %#v", result.Diagnostics)
	}
}

func TestCatchVariableCollidesWithOuterLocal(t *testing.T) {
	t.Parallel()
	result := analyzeScopeProject(t, `
public class Probe {
  void run() {
    String value = 'a';
    try {
    } catch (Exception value) {
    }
  }
}
`)
	if !hasScopeRedeclareDiagnostic(result, "value") {
		t.Fatalf("expected catch/local collision diagnostic, got %#v", result.Diagnostics)
	}
}

func TestParameterLocalCollisionIsRejected(t *testing.T) {
	t.Parallel()
	result := analyzeScopeProject(t, `
public class Probe {
  void run(String value) {
    String value = 'a';
  }
}
`)
	if !hasScopeRedeclareDiagnostic(result, "value") {
		t.Fatalf("expected parameter/local collision diagnostic, got %#v", result.Diagnostics)
	}
}

func TestSiblingScopesMayReuseNames(t *testing.T) {
	t.Parallel()
	result := analyzeScopeProject(t, `
public class Probe {
  void run() {
    {
      String value = 'a';
    }
    {
      String value = 'b';
    }
    for (String item : new List<String>()) {}
    for (String item : new List<String>()) {}
  }
}
`)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA014" {
			t.Fatalf("sibling scopes should allow reused names: %#v", result.Diagnostics)
		}
	}
}

func TestFieldMayBeShadowedByLocal(t *testing.T) {
	t.Parallel()
	result := analyzeScopeProject(t, `
public class Probe {
  String value;
  void run() {
    String value = 'local';
  }
}
`)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA014" {
			t.Fatalf("locals may shadow fields: %#v", result.Diagnostics)
		}
	}
}
