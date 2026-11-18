package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
)

// Ruling 49 closes static field reads through an undeclared namespace
// (namespace_external_positions A12). A member chain that starts at this or
// super names an inherited or own field, not a namespace, and keeps the
// pre-family answer (accepted at asm4 f38fdd23c).
func TestNamespaceReceiverClosureSkipsThisAndSuperChains(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Base.cls": `public virtual class Base {
  public Base.Data prot = new Base.Data();
  public class Data { public Integer n; public Base.Data next; }
}`,
		"Kid.cls": `public class Kid extends Base {
  public Integer go() {
    Integer a = super.prot.n;
    Integer b = this.prot.next.n;
    return a + b;
  }
}`,
	})
	for _, d := range result.Diagnostics {
		if d.Severity == diagnostic.Error {
			t.Errorf("unexpected error %s %q", d.Code, d.Message)
		}
	}
}
