package vm

import (
	"fmt"
	"testing"
)

func TestRegisterClassKeepsStaticInitializationStateSparse(t *testing.T) {
	machine := New(nil)
	for i := 0; i < 5000; i++ {
		if err := machine.RegisterClass(Class{Name: fmt.Sprintf("Unused%04d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	if got := len(machine.staticInitState); got != 0 {
		t.Fatalf("registered unused classes created %d initialization-state entries, want 0", got)
	}
}

func TestCloneRuntimeStartsWithSparseStaticInitializationState(t *testing.T) {
	template := New(nil)
	if err := template.RegisterClass(Class{Name: "Done"}); err != nil {
		t.Fatal(err)
	}
	if err := template.RegisterClass(Class{Name: "Running"}); err != nil {
		t.Fatal(err)
	}
	template.staticInitState = map[string]staticInitState{
		"Done":    staticInitDone,
		"Running": staticInitRunning,
	}

	clone := template.CloneRuntime(nil)
	if got := len(clone.staticInitState); got != 0 {
		t.Fatalf("clone copied %d template initialization-state entries, want 0", got)
	}
}

func TestStaticInitializationStateUsesOneCanonicalAliasKey(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Worker", Namespace: "pkg"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.ensureClassInitialized("Worker"); err != nil {
		t.Fatal(err)
	}
	if err := machine.ensureClassInitialized("pkg.Worker"); err != nil {
		t.Fatal(err)
	}
	if got := len(machine.staticInitState); got != 1 {
		t.Fatalf("alias initialization created %d state entries, want 1", got)
	}
	if got := machine.staticInitState["pkg.Worker"]; got != staticInitDone {
		t.Fatalf("canonical state = %v, want done", got)
	}
	if _, ok := machine.staticInitState["Worker"]; ok {
		t.Fatal("short alias has a duplicate initialization-state entry")
	}
}

func TestStaticInitializerFailureRemovesSparseState(t *testing.T) {
	fail, err := CompileAnonymous("throw new Exception('broken');")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Broken",
		StaticInitializers: []Method{{
			Name:      "Broken.<static_init>",
			ClassName: "Broken",
			IsStatic:  true,
			Program:   fail,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.ensureClassInitialized("Broken"); err == nil {
		t.Fatal("initializer succeeded, want failure")
	}
	if got := len(machine.staticInitState); got != 0 {
		t.Fatalf("failed initializer retained %d state entries, want 0", got)
	}
}

func TestNamespacedRegistrationDoesNotResetLocalCanonicalStaticState(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Shared"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.ensureClassInitialized("Shared"); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Shared", Namespace: "pkg", Dependency: true}); err != nil {
		t.Fatal(err)
	}
	if got := machine.staticInitState["Shared"]; got != staticInitDone {
		t.Fatalf("local canonical state = %v, want done", got)
	}
	if _, ok := machine.staticInitState["pkg.Shared"]; ok {
		t.Fatal("new dependency class allocated an initialization-state entry")
	}
}

func TestAsyncStaticResetOnlyRemovesAffectedSparseStates(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "CollectionState",
		StaticFields: map[string]Field{
			"Values": {Name: "Values", Type: "Set<String>", Static: true},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "ScalarState",
		StaticFields: map[string]Field{
			"Value": {Name: "Value", Type: "Integer", Static: true},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.ensureClassInitialized("CollectionState"); err != nil {
		t.Fatal(err)
	}
	if err := machine.ensureClassInitialized("ScalarState"); err != nil {
		t.Fatal(err)
	}
	if err := machine.ResetTestAsyncStaticCollections(); err != nil {
		t.Fatal(err)
	}
	if _, ok := machine.staticInitState["CollectionState"]; ok {
		t.Fatal("collection-backed class remained initialized after async reset")
	}
	if got := machine.staticInitState["ScalarState"]; got != staticInitDone {
		t.Fatalf("unaffected scalar class state = %v, want done", got)
	}
}

func BenchmarkSparseStaticInitClone(b *testing.B) {
	for _, classCount := range []int{1, 100, 5000} {
		b.Run(fmt.Sprintf("classes_%d", classCount), func(b *testing.B) {
			template := New(nil)
			for i := 0; i < classCount; i++ {
				if err := template.RegisterClass(Class{Name: fmt.Sprintf("Class%04d", i)}); err != nil {
					b.Fatal(err)
				}
			}
			template.FreezeClassLookup()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				clone := template.CloneRuntime(nil)
				if len(clone.staticInitState) != 0 {
					b.Fatalf("clone state entries = %d, want 0", len(clone.staticInitState))
				}
			}
		})
	}
}
