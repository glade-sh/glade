package vm

import "testing"

func registerCustomException(t *testing.T, machine *VM, name string) {
	t.Helper()
	if err := machine.RegisterClass(Class{Name: name, SuperClass: "Exception"}); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeGeneratedStandardExceptionsPreserveCatchTypeAndMessage(t *testing.T) {
	machine := New(nil)
	for _, test := range []struct {
		name    string
		message string
	}{
		{name: "Exception", message: "base runtime"},
		{name: "InvalidParameterValueException", message: "invalid runtime"},
		{name: "NoAccessException", message: "access runtime"},
		{name: "NoDataFoundException", message: "data runtime"},
		{name: "NullPointerException", message: "null runtime"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := newExceptionError(test.name, test.message)
			thrown, ok := err.(*apexThrowError)
			if !ok {
				t.Fatalf("newExceptionError() = %#v, want apex throw", err)
			}
			if thrown.value.Type != test.name {
				t.Fatalf("runtime type = %q, want %q", thrown.value.Type, test.name)
			}
			message, ok := thrown.value.Fields["message"]
			if !ok || message.Kind != ValueString || message.Text != test.message {
				t.Fatalf("runtime message = %#v, want %q", message, test.message)
			}
			if !machine.exceptionMatches("Exception", thrown.value) || !machine.exceptionMatches(test.name, thrown.value) {
				t.Fatalf("runtime %s did not match its concrete or base catch type", test.name)
			}
		})
	}
}
