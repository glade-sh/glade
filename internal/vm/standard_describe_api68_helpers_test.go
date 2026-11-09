package vm

import "testing"

// This wrapper models the describe entry point rather than generic object lookup.
func fieldMapAPI68Wrapper(t *testing.T, machine *VM, object string) Value {
	t.Helper()
	name, definition, ok := machine.describeAPI68ObjectDefinition(object)
	if !ok {
		t.Fatalf("missing describe object %s", object)
	}
	return machine.describeSObjectValue(name, definition).Fields["fields"]
}
