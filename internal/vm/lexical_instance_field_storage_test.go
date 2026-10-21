package vm

import (
	"strings"
	"testing"
)

func TestLexicalInstanceStorageDoesNotLeak(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "AtlasState", Fields: map[string]Field{
		"visible": {Name: "visible", Type: "String", Access: "public"},
	}}); err != nil {
		t.Fatal(err)
	}
	value := Object("AtlasState")
	value.Fields["visible"] = String("ordinary")
	value.Fields["__gladeLexicalInstance$base$visible"] = String("base storage")
	if display := value.String(); strings.Contains(display, "__glade") || strings.Contains(display, "base storage") {
		t.Fatalf("internal field storage escaped into display: %s", display)
	}
	machine.Globals["value"] = value
	program, err := CompileAnonymous(`String encoded = JSON.serialize(value); System.assert(encoded.contains('ordinary')); System.assert(!encoded.contains('__glade')); System.assert(!encoded.contains('base storage'));`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
