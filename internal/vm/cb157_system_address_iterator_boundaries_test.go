package vm

import (
	"errors"
	"strings"
	"testing"
)

func TestExecCB157AddressNewInstanceIsUnsupportedStaticCall(t *testing.T) {
	if canonical, ok := canonicalBuiltinStaticCall("Address.newInstance"); ok {
		t.Fatalf("canonical built-in static call = %q, want no admission", canonical)
	}

	program, err := CompileAnonymous("Address address = Address.newInstance();")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	_, err = machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) ||
		runtimeErr.Type != "UnsupportedFeature" ||
		runtimeErr.Message != "unsupported call \"Address.newInstance\"" {
		t.Fatalf("error = %#v, want unsupported Address.newInstance call", err)
	}
}

func TestExecCB157JSONSerializeRejectsListIterator(t *testing.T) {
	const want = "System.JSONException: Apex Type unsupported in JSON: system.ListIterator"
	source := strings.Join([]string{
		"List<Integer> values = new List<Integer>{1};",
		"Iterator<Integer> iterator = values.iterator();",
		"String caught = '';",
		"try {",
		"    JSON.serialize(iterator);",
		"} catch (JSONException e) {",
		"    caught = e.getTypeName() + ': ' + e.getMessage();",
		"}",
		"System.assertEquals('" + want + "', caught);",
		"caught = '';",
		"try {",
		"    JSON.serializePretty(iterator);",
		"} catch (JSONException e) {",
		"    caught = e.getTypeName() + ': ' + e.getMessage();",
		"}",
		"System.assertEquals('" + want + "', caught);",
	}, "\n")
	program, err := CompileAnonymous(source)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
