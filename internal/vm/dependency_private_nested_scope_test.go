package vm

import (
	"strings"
	"testing"
)

// These controls retain q345's pre-existing rejection outside the legacy
// dependency-owned nested subclass class-literal path. They are not native rows.
func TestDependencyPrivateNestedReflectionScopeControls(t *testing.T) {
	for _, tc := range []struct {
		name, namespace, owner, super, caller, token string
		dependency                                   bool
	}{
		{"local class", "pkg", "Factory", "Factory", "pkg.Factory", "literal", false},
		{"unnamespaced dependency", "", "Factory", "Factory", "Factory", "literal", true},
		{"other caller", "pkg", "Factory", "Factory", "pkg.Other", "literal", true},
		{"non-subclass", "pkg", "Factory", "Other", "pkg.Factory", "literal", true},
		{"lookup token", "pkg", "Factory", "Factory", "pkg.Factory", "lookup", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			outer := Class{Name: tc.owner, Namespace: tc.namespace, Access: "global", Dependency: tc.dependency}
			inner := Class{Name: tc.owner + ".Inner", Namespace: tc.namespace, Access: "private", Nested: true, Dependency: tc.dependency, SuperClass: tc.super,
				Constructors: []Method{{Name: tc.owner + ".Inner.<init>", Access: "private", IsConstructor: true}}}
			for _, class := range []Class{outer, inner, {Name: "Other", Namespace: tc.namespace, Access: "public", Dependency: tc.dependency}} {
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			machine.currentClass = runtimeClassName(outer)
			machine.currentMethod = Method{Namespace: tc.namespace, SourceContextBound: true}
			var token Value
			if tc.token == "lookup" {
				token = machine.typeForName(tc.namespace, tc.owner+".Inner", true)
			} else {
				var err error
				token, err = machine.lookup(tc.owner + ".Inner.class")
				if err != nil {
					t.Fatal(err)
				}
			}
			if token.Kind == ValueNull {
				t.Fatal("control failed to obtain a Type token")
			}
			machine.currentClass = tc.caller
			_, handled, err := machine.callTypeObjectMember(token, "newInstance", nil, &Result{})
			if !handled || err == nil || !strings.Contains(err.Error(), "does not have a no-arg constructor") {
				t.Fatalf("outside legacy scope: handled=%t err=%v", handled, err)
			}
		})
	}
}
