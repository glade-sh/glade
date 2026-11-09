package vm

import "testing"

// Native rows: apextest/testdata/conformance/namespace_subscriber_members
// S05-S08 and S19. A subscriber's Type.forName finds a package's global nested
// class and not its private or public nested classes, on both overloads; the
// package itself still finds all of them.
func TestTypeForNameDependencyNestedVisibility(t *testing.T) {
	for _, tc := range []struct {
		access, caller string
		visible        bool
	}{
		{"global", "", true},
		{"public", "", false},
		{"private", "", false},
		{"public", "pkg", true},
		{"private", "pkg", true},
		{"private", "other", false},
	} {
		for _, lookup := range []string{"single", "two"} {
			t.Run(tc.access+"/"+tc.caller+"/"+lookup, func(t *testing.T) {
				machine := New(nil)
				outer := Class{Name: "Factory", Namespace: "pkg", Access: "global", Dependency: true}
				inner := Class{Name: "Factory.Inner", Namespace: "pkg", Access: tc.access, Nested: true, Dependency: true}
				for _, class := range []Class{outer, inner} {
					if err := machine.RegisterClass(class); err != nil {
						t.Fatal(err)
					}
				}
				machine.currentClass = tc.caller
				machine.currentMethod = Method{Namespace: tc.caller, SourceContextBound: true}
				var token Value
				if lookup == "single" {
					token = machine.typeForName("", "pkg.Factory.Inner", false)
				} else {
					token = machine.typeForName("pkg", "Factory.Inner", true)
				}
				if got := token.Kind != ValueNull; got != tc.visible {
					t.Fatalf("visible=%t, want %t", got, tc.visible)
				}
			})
		}
	}
}
