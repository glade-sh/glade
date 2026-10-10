package vm

import "testing"

func TestResolveEnumClassEmptyRuntimeAvoidsLookupBookkeeping(t *testing.T) {
	machine := New(nil)
	machine.currentClass = "OneTarget"
	lookupWasNil := machine.enumLookup == nil
	suffixWasNil := machine.enumSuffixLookup == nil
	for _, typeName := range []string{"String", "State"} {
		if _, ok := machine.resolveEnumClass(typeName); ok {
			t.Fatalf("empty runtime resolved enum %q", typeName)
		}
	}
	if len(machine.enumLookup) != 0 || len(machine.enumSuffixLookup) != 0 ||
		(machine.enumLookup == nil) != lookupWasNil || (machine.enumSuffixLookup == nil) != suffixWasNil {
		t.Fatal("empty runtime retained enum lookup bookkeeping")
	}

	if err := machine.RegisterClass(Class{Name: "OneTarget.State", EnumValues: []string{"Ready"}}); err != nil {
		t.Fatal(err)
	}
	class, ok := machine.resolveEnumClass("State")
	if !ok || class.Name != "OneTarget.State" {
		t.Fatalf("registered enum resolution = (%q, %v), want OneTarget.State", class.Name, ok)
	}
	coerced, err := machine.coerceAssignable("State", String("Ready"))
	if err != nil {
		t.Fatal(err)
	}
	if coerced.Kind != ValueObject || coerced.Type != class.Name || coerced.Text != "Ready" {
		t.Fatalf("registered enum coercion = %#v", coerced)
	}
}
