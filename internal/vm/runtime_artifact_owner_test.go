package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Both sides of a clone are owners. Registration may change either side after
// publication; cached matches must never publish that private change to others.
func TestRuntimeArtifactOwnersIsolateRegistration(t *testing.T) {
	for _, sourceWrites := range []bool{false, true} {
		for _, change := range []string{"method", "remove-method", "trigger"} {
			t.Run(fmt.Sprintf("source=%v/%s", sourceWrites, change), func(t *testing.T) {
				source := New(nil)
				original := Method{Name: "Owned.original", ClassName: "Owned", ReturnType: "void", IsStatic: true}
				if err := source.RegisterMethod(original); err != nil {
					t.Fatal(err)
				}
				trigger := Trigger{Name: "Original", Object: "Account", Timing: "before", Operation: "insert"}
				if err := source.RegisterTrigger(trigger); err != nil {
					t.Fatal(err)
				}
				source.triggersForOperation("Account", "before", "insert")
				clone := source.CloneRuntime(nil)
				writer, observer := clone, source
				if sourceWrites {
					writer, observer = source, clone
				}
				switch change {
				case "method":
					if err := writer.RegisterMethod(Method{Name: "Owned.late", ClassName: "Owned", ReturnType: "void", IsStatic: true}); err != nil {
						t.Fatal(err)
					}
					if _, ok := observer.registeredMethod("Owned.late"); ok {
						t.Fatal("registration reached the other owner")
					}
				case "remove-method":
					writer.unregisterMethod(original)
					if _, ok := observer.registeredMethod(original.Name); !ok {
						t.Fatal("removal reached the other owner")
					}
				case "trigger":
					trigger.Name = "Late"
					if err := writer.RegisterTrigger(trigger); err != nil {
						t.Fatal(err)
					}
					if got := len(writer.triggersForOperation("Account", "before", "insert")); got != 2 {
						t.Fatalf("writer matches=%d", got)
					}
					if got := len(observer.triggersForOperation("Account", "before", "insert")); got != 1 {
						t.Fatalf("observer matches=%d", got)
					}
				}
			})
		}
	}
}

func TestTriggerMatchesObserveSchemaGeneration(t *testing.T) {
	for _, initiallyPresent := range []bool{false, true} {
		for _, boundary := range []string{"clear", "runtime", "mutable"} {
			t.Run(fmt.Sprintf("present=%v/%s", initiallyPresent, boundary), func(t *testing.T) {
				org := storage.NewOrgState()
				storage.EnsureStandardObject(&org, "User")
				if initiallyPresent {
					storage.EnsureStandardObject(&org, "Account")
				}
				machine := New(nil)
				machine.SetOrg(&org)
				trigger := Trigger{Name: "Qualified", Object: "Schema.Account", Timing: "before", Operation: "insert"}
				if err := machine.RegisterTrigger(trigger); err != nil {
					t.Fatal(err)
				}
				before := len(machine.triggersForOperation("Account", "before", "insert"))
				if initiallyPresent {
					delete(org.Objects, "Account")
				} else {
					storage.EnsureStandardObject(&org, "Account")
				}
				switch boundary {
				case "clear":
					org.ClearRuntimeSchemaStamp()
				case "runtime":
					org.InvalidateRuntimeGeneration()
				case "mutable":
					storage.EnsureMutableObjectDefinition(&org, "User")
				}
				fresh := New(nil)
				freshOrg := org.Clone()
				fresh.SetOrg(&freshOrg)
				if err := fresh.RegisterTrigger(trigger); err != nil {
					t.Fatal(err)
				}
				want := len(fresh.triggersForOperation("Account", "before", "insert"))
				if want == before {
					t.Fatalf("control did not change: %d", before)
				}
				if got := len(machine.triggersForOperation("Account", "before", "insert")); got != want {
					t.Fatalf("warm matches=%d fresh=%d", got, want)
				}
			})
		}
	}
}
