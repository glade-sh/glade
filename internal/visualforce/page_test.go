package visualforce

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestRenderPageSetsProjectNamespaceForMemberAccess(t *testing.T) {
	machine := vm.New(nil)
	machine.SetCurrentNamespace("samplepkg")
	if got := machine.CurrentPage(); got.Kind != vm.ValueNull {
		t.Fatalf("unexpected current page %#v", got)
	}
	// Namespaced access is allowed when caller namespace matches owner namespace.
	if err := machine.RegisterClass(vm.Class{
		Name:      "constants",
		Namespace: "samplepkg",
		Access:    "public",
	}); err != nil {
		t.Fatal(err)
	}
	// Without namespace context this would fail checkClassAccess.
	machine.SetCurrentNamespace("")
	_, err := machine.ConstructController("constants")
	if err == nil || !strings.Contains(err.Error(), "not global and not visible outside namespace samplepkg") {
		t.Fatalf("ConstructController err = %v, want namespace visibility error", err)
	}
	machine.SetCurrentNamespace("samplepkg")
	if _, err := machine.ConstructController("constants"); err != nil {
		t.Fatalf("ConstructController with namespace = %v", err)
	}
}

func TestExternalPageReferenceContentThrowsCatchableVisualforceException(t *testing.T) {
	program, err := vm.CompileAnonymous(`
Boolean caught = false;
try {
    new PageReference('http://www.google.com/').getContent();
} catch (VisualforceException e) {
    caught = true;
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vm.New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}
