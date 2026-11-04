package vm

import (
	"strings"
	"testing"
)

// Review repro: this is a synthetic local test; package access needs native evidence.
func TestDependencyReflectionPrivateConstructorReview(t *testing.T) {
	for _, ownerName := range []string{"Factory", "Builder", "Creator"} {
		t.Run(ownerName, func(t *testing.T) {
			machine := New(nil)
			outer := Class{Name: ownerName, Namespace: "pkg", Access: "global", Dependency: true}
			inner := Class{Name: ownerName + ".Inner", Namespace: "pkg", Access: "private", Nested: true, Dependency: true, SuperClass: ownerName, Constructors: []Method{{Name: ownerName + ".Inner.<init>", Access: "private", IsConstructor: true}}}
			for _, c := range []Class{outer, inner} {
				if err := machine.RegisterClass(c); err != nil {
					t.Fatal(err)
				}
			}
			machine.currentClass = runtimeClassName(outer)
			machine.currentMethod = Method{Namespace: "pkg", SourceContextBound: true}
			token, err := machine.lookup(ownerName + ".Inner.class")
			if err != nil || token.Kind == ValueNull {
				t.Fatalf("type token: %v", err)
			}
			_, handled, err := machine.callTypeObjectMember(token, "newInstance", nil, &Result{})
			if !handled || err == nil || !strings.Contains(err.Error(), "does not have a no-arg constructor") {
				t.Fatalf("handled=%v err=%v", handled, err)
			}
		})
	}
}
