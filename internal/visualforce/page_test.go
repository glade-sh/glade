package visualforce

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestPageReferenceRenderErrorPreservesControlErrors(t *testing.T) {
	unsupported := vm.NewUnsupportedFeatureError("flow:interview local surface")
	limit := &vm.RuntimeError{Type: "System.LimitException", Message: "Too many SOQL queries: 101"}
	ordinary := errors.New("controller getter failed")
	cases := []struct {
		name string
		err  error
	}{
		{name: "unsupported feature", err: unsupported},
		{name: "governor limit", err: limit},
		{name: "canceled", err: context.Canceled},
		{name: "deadline exceeded", err: context.DeadlineExceeded},
		{name: "ordinary render failure", err: ordinary},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pageReferenceRenderError(tc.err)
			if got == nil {
				t.Fatal("pageReferenceRenderError returned nil")
			}
			if tc.name != "ordinary render failure" {
				if !errors.Is(got, tc.err) {
					t.Fatalf("err = %v, want identity of %v", got, tc.err)
				}
				return
			}
			if got == tc.err || got.Error() != tc.err.Error() {
				t.Fatalf("err = %v, want a distinct ExecutionException wrapper", got)
			}
		})
	}
	if got := pageReferenceRenderError(fmt.Errorf("wrapped: %w", context.Canceled)); !errors.Is(got, context.Canceled) {
		t.Fatalf("wrapped cancellation err = %v, want cancellation identity", got)
	}
}

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
