package vm

import "testing"

func typeResolutionOwnerFixture(tb testing.TB) *VM {
	tb.Helper()
	machine := New(nil)
	for _, name := range []string{"First", "First.Item", "Second", "Second.Item", "Third", "Third.Item"} {
		if err := machine.RegisterClass(Class{Name: name}); err != nil {
			tb.Fatal(err)
		}
	}
	return machine
}

func TestTypeResolutionOwnerPrecedence(t *testing.T) {
	machine := typeResolutionOwnerFixture(t)
	for _, tc := range []struct {
		current, methodClass, methodName, want string
	}{
		{"First", "Second", "Third.run", "First.Item"},
		{"Missing", "Second", "Third.run", "Second.Item"},
		{"Missing", "Other", "Third.run", "Third.Item"},
		{"", "", "Missing.run", ""},
		{" first ", "FIRST", "Second.run", "First.Item"},
	} {
		machine.currentClass = tc.current
		machine.currentMethod = Method{ClassName: tc.methodClass, Name: tc.methodName}
		if got := machine.resolveNestedTypeNameInCurrentExecutionContext("Item"); got != tc.want {
			t.Errorf("owners %q %q %q: got %q, want %q", tc.current, tc.methodClass, tc.methodName, got, tc.want)
		}
	}
	machine.currentClass = "First"
	if got := machine.resolveNestedTypeNameInCurrentExecutionContext("Other.Item"); got != "" {
		t.Fatalf("qualified type: got %q, want no nested lookup", got)
	}
}

func BenchmarkTypeResolutionOwners(b *testing.B) {
	machine := typeResolutionOwnerFixture(b)
	machine.currentClass = "First"
	machine.currentMethod = Method{ClassName: "Second", Name: "Third.run"}
	b.ReportAllocs()
	for b.Loop() {
		_ = machine.resolveNestedTypeNameInCurrentExecutionContext("Missing")
	}
}
