package vm

import (
	"errors"
	"testing"
)

func TestSOQLBindReservedStringLocals(t *testing.T) {
	for _, word := range []string{"select", "class"} {
		_, err := CompileAnonymous("String " + word + "='Acme';")
		var syntax *ApexSyntaxError
		if !errors.As(err, &syntax) || syntax.NativeMessage != "Unexpected token '"+word+"'." {
			t.Fatalf("%s: %v", word, err)
		}
	}
	if _, err := CompileAnonymous("String token='Acme';"); err != nil {
		t.Fatal(err)
	}
}
