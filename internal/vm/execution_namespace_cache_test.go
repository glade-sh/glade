package vm

import (
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

func TestCurrentExecutionNamespaceAfterAnonymousLocalCleanup(t *testing.T) {
	machine := New(os.Stderr)
	if err := machine.RegisterTrigger(Trigger{Name: "anon", Namespace: "pkg", Object: "Account", Timing: "before", Operation: "insert"}); err != nil {
		t.Fatal(err)
	}
	machine.currentClass = "anon"
	for i := 0; i < 2; i++ {
		cleanup, err := machine.registerAnonymousLocalMethods(ir.Program{})
		if err != nil {
			t.Fatal(err)
		}
		if got := machine.currentExecutionNamespace(); got != "" {
			t.Fatalf("cycle %d during registration: namespace = %q, want empty", i, got)
		}
		cleanup()
		if got := machine.currentExecutionNamespace(); got != "pkg" {
			t.Fatalf("cycle %d after cleanup: namespace = %q, want pkg", i, got)
		}
	}
	if err := machine.RegisterClass(Class{Name: "anon", Namespace: "existing"}); err != nil {
		t.Fatal(err)
	}
	cleanup, err := machine.registerAnonymousLocalMethods(ir.Program{})
	if err != nil {
		t.Fatal(err)
	}
	cleanup()
	if got := machine.currentExecutionNamespace(); got != "existing" {
		t.Fatalf("after cleanup with existing class: namespace = %q, want existing", got)
	}
}

func TestCurrentExecutionNamespaceFollowsClassRegistration(t *testing.T) {
	machine := New(os.Stderr)
	if err := machine.RegisterClass(Class{Name: "Widget", Namespace: "alpha"}); err != nil {
		t.Fatal(err)
	}
	machine.currentClass = "Widget"
	for i := 0; i < 3; i++ {
		if got := machine.currentExecutionNamespace(); got != "alpha" {
			t.Fatalf("call %d: namespace = %q, want alpha", i, got)
		}
	}
	// A different current class must not reuse the memo.
	if err := machine.RegisterClass(Class{Name: "Gadget", Namespace: "beta"}); err != nil {
		t.Fatal(err)
	}
	machine.currentClass = "Gadget"
	if got := machine.currentExecutionNamespace(); got != "beta" {
		t.Fatalf("namespace = %q, want beta", got)
	}
	// An explicit current namespace still wins.
	machine.currentNamespace = " gamma "
	if got := machine.currentExecutionNamespace(); got != "gamma" {
		t.Fatalf("namespace = %q, want gamma", got)
	}
	machine.currentNamespace = ""
	// Replacing the class bumps the lookup generation and the memo drops.
	machine.Classes["Gadget"] = Class{Name: "Gadget", Namespace: "delta"}
	machine.rebuildClassLookup()
	if got := machine.currentExecutionNamespace(); got != "delta" {
		t.Fatalf("after rebuild namespace = %q, want delta", got)
	}
}

func BenchmarkCurrentExecutionNamespace(b *testing.B) {
	machine := New(os.Stderr)
	if err := machine.RegisterClass(Class{Name: "Widget", Namespace: "alpha"}); err != nil {
		b.Fatal(err)
	}
	machine.currentClass = "Widget"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if machine.currentExecutionNamespace() != "alpha" {
			b.Fatal("namespace")
		}
	}
}
