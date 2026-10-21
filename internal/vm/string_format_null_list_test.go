package vm

import "testing"

func TestStringStaticFormatObservedNullObjectList(t *testing.T) {
	value, err := stringStatic("String.format", []Value{String("{0}"), {Kind: ValueNull, Type: "List<Object>"}})
	if err != nil || value.Kind != ValueString || value.Text != "{0}" {
		t.Fatalf("C16 native literal placeholder; local=%#v error=%v", value, err)
	}
}
