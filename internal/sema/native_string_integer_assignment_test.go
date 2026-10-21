package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// An assignment of a String to an Integer publishes the platform's
// diagnostic text as the message, as the editor path does for every other
// assignment mismatch with captured text.
func TestNativeLifecycleDiagnosticsPublishStringToIntegerAssignmentText(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ProbeStringToInteger.cls")
	writeSemaFile(t, path, `
public class ProbeStringToInteger {
  public Integer value;
  public void prepare() {
    value = 'not-an-integer';
  }
  public void viaLocal() {
    Integer n;
    n = 'also-not';
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{})
	diagnostics := NativeLifecycleDiagnostics(index, Analyze(index).Diagnostics)
	found := 0
	for _, diag := range diagnostics {
		if diag.Code != "GLADESEMA018" {
			continue
		}
		found++
		if diag.Message != "Illegal assignment from String to Integer" {
			t.Fatalf("message = %q, want the platform text", diag.Message)
		}
	}
	if found != 2 {
		t.Fatalf("GLADESEMA018 diagnostics = %d, want 2 (field and local); all = %#v", found, diagnostics)
	}
}
