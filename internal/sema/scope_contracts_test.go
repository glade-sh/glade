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

func TestDuplicateLocalCanonicalKeyPreservesDiagnosticSpellingAndOffset(t *testing.T) {
	t.Parallel()
	source := `
public class Probe {
  void run() {
    String value = 'a';
    String Value = 'b';
  }
}
`
	result := analyzeScopeProject(t, source)
	for _, diag := range result.Diagnostics {
		if diag.Code != "GLADESEMA014" || !strings.Contains(diag.Message, `"Value"`) {
			continue
		}
		if diag.Range == nil {
			t.Fatal("duplicate local diagnostic has no range")
		}
		start := diag.Range.Start.Offset
		end := diag.Range.End.Offset
		if start < 0 || end > len(source) || start >= end {
			t.Fatalf("duplicate local diagnostic range = %d:%d for source length %d", start, end, len(source))
		}
		if got := source[start:end]; got != "Value" {
			t.Fatalf("duplicate local diagnostic source = %q, want Value", got)
		}
		return
	}
	t.Fatalf("missing duplicate local diagnostic with original spelling: %#v", result.Diagnostics)
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
    for (Id cartId : new List<Id>()) {
      System.debug(cartId);
    }
    Integer completed = 1;
    for (Id cartId : new List<Id>()) {
      System.debug(cartId);
    }
  }
}
`)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA014" {
			t.Fatalf("sibling scopes should allow reused names: %#v", result.Diagnostics)
		}
	}
}

func TestNestedEnhancedForLocalCannotRedeclareOuterLoopLocal(t *testing.T) {
	t.Parallel()
	result := analyzeScopeProject(t, `
public class Probe {
  void run() {
    for (Id cartId : new List<Id>()) {
      for (Id cartId : new List<Id>()) {
        System.debug(cartId);
      }
    }
  }
}
`)
	if !hasScopeRedeclareDiagnostic(result, "cartId") {
		t.Fatalf("expected nested enhanced-for redeclaration diagnostic, got %#v", result.Diagnostics)
	}
}

func TestCommentedSiblingEnhancedForLocalsDoNotRedeclare(t *testing.T) {
	t.Parallel()
	result := analyzeScopeProject(t, `
public class Probe {
  void run() {
    /*
    for (Id cartId : new List<Id>()) {
      System.debug(cartId);
    }
    Integer completed = 1;
    for (Id cartId : new List<Id>()) {
      System.debug(cartId);
    }
    */
  }
}
`)
	if hasScopeRedeclareDiagnostic(result, "cartId") {
		t.Fatalf("commented enhanced-for locals must not redeclare: %#v", result.Diagnostics)
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
