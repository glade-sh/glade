package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// The Apex Developer Guide's "Using the super Keyword" permits an override
// to call its virtual superclass implementation. Sibling inner superclass
// names must resolve in their enclosing class, as in ordinary inheritance.
func TestAnalyzeNestedSuperMethodCalls(t *testing.T) {
	for _, namespace := range []string{"", "samplepkg"} {
		t.Run("namespace="+namespace, func(t *testing.T) {
			root := t.TempDir()
			writeSemaFile(t, filepath.Join(root, "LabelFactory.cls"), `
public class LabelFactory {
  public abstract class Base {
    protected virtual String label() { return 'base'; }
  }
  public class Child extends Base {
    protected override String label() { return super.label() + '-child'; }
  }
}
`)
			writeSemaFile(t, filepath.Join(root, "NumberFactory.cls"), `
public class NumberFactory {
  public abstract class Base {
    protected virtual Integer label() { return 7; }
  }
  public class Child extends Base {
    protected override Integer label() { return super.label() + 1; }
  }
}
`)
			index := typesys.Build(project.Project{Root: root, Namespace: namespace, ApexFiles: []string{
				filepath.Join(root, "LabelFactory.cls"),
				filepath.Join(root, "NumberFactory.cls"),
			}}, schema.Schema{})
			if result := Analyze(index); result.HasErrors() {
				t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
			}
		})
	}
}

func TestAnalyzeNestedSuperMissingMethod(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "LabelFactory.cls"), `
public class LabelFactory {
  public abstract class Base {
    protected virtual String label() { return 'base'; }
  }
  public class Child extends Base {
    protected override String label() { return super.absent(); }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "LabelFactory.cls"),
	}}, schema.Schema{})
	result := Analyze(index)
	found := false
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "super.absent") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unknown superclass method diagnostic: %#v", result.Diagnostics)
	}
}
