package vm

import "testing"

func TestIllegalArgumentMessageDisplayPreservesOtherShapes(t *testing.T) {
	for _, tc := range []struct {
		name, typeName, want string
		args                 []Value
		thrown, messageSet   bool
	}{
		{name: "message", typeName: "IllegalArgumentException", args: []Value{String("detail")}, want: "IllegalArgumentException:[]: detail"},
		{name: "default", typeName: "IllegalArgumentException", want: "System.IllegalArgumentException: Script-thrown exception"},
		{name: "null", typeName: "IllegalArgumentException", args: []Value{Null}, want: "System.IllegalArgumentException: Script-thrown exception"},
		{name: "empty", typeName: "IllegalArgumentException", args: []Value{String("")}, want: "System.IllegalArgumentException: Script-thrown exception"},
		{name: "idArgument", typeName: "IllegalArgumentException", args: []Value{{Kind: ValueString, Type: "Id", Text: "detail"}}, want: "System.IllegalArgumentException: detail"},
		{name: "boxedStringArgument", typeName: "IllegalArgumentException", args: []Value{{Kind: ValueString, Static: "Object", Text: "detail"}}, want: "System.IllegalArgumentException: detail"},
		{name: "cause", typeName: "IllegalArgumentException", args: []Value{String("detail"), Object("Exception")}, want: "System.IllegalArgumentException: detail"},
		{name: "thrown", typeName: "IllegalArgumentException", args: []Value{String("detail")}, thrown: true, want: "System.IllegalArgumentException: detail"},
		{name: "changedMessage", typeName: "IllegalArgumentException", args: []Value{String("detail")}, messageSet: true, want: "System.IllegalArgumentException: detail"},
		{name: "otherBuiltin", typeName: "TypeException", args: []Value{String("detail")}, want: "System.TypeException: detail"},
		{name: "custom", typeName: "Parcel.DisplayException", args: []Value{String("detail")}, want: "Parcel.DisplayException:[]: detail"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := Object(tc.typeName)
			ok, err := applyExceptionConstructorArgs(&value, tc.args)
			if err != nil || !ok {
				t.Fatalf("constructor: accepted=%t err=%v", ok, err)
			}
			value.Fields["__thrown"] = Bool(tc.thrown)
			value.Fields["__messageSet"] = Bool(tc.messageSet)
			if got := exceptionToString(value); got != tc.want {
				t.Fatalf("display=%q; want %q", got, tc.want)
			}
		})
	}
	// Runtime-created errors do not acquire constructor provenance from their
	// public message/type fields. This is a local preservation control.
	fault := Object("IllegalArgumentException")
	fault.Fields["message"] = String("detail")
	if got := exceptionToString(fault); got != "System.IllegalArgumentException: detail" {
		t.Fatalf("runtime fault changed: %q", got)
	}
}
