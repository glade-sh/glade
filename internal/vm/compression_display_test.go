package vm

import "testing"

// C025's native formatting correction preserves the existing user-class path.
func TestCompressionDisplayPreservesQualifiedUserObjects(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Outer.Inner"}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ value, want string }{
		{"x", "Innerx"},
		{"Outer.Inner:x", "Inner:x"},
	} {
		t.Run(tc.value, func(t *testing.T) {
			object := Object("Outer.Inner")
			object.Fields["value"] = String(tc.value)
			got, err := machine.displayString(object, &Result{})
			if err != nil || got != tc.want {
				t.Fatalf("display=%q want %q: %v", got, tc.want, err)
			}
		})
	}
}
