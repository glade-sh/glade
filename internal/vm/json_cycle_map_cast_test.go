package vm

import (
	"errors"
	"testing"
)

// E21 A01-A06: a Map of class or SObject values held as Object fails the
// cast to that class, Account or SObject with one TypeException, cyclic or not.
func TestJSONCycleMapCastsRejectBeforeConversion(t *testing.T) {
	assertJSONCycleGuardBounded(t)
	for _, target := range []string{"CycleNode", "Account", "SObject"} {
		for _, cyclic := range []bool{false, true} {
			machine := jsonCycleTestMachine(t)
			value := Map()
			if target == "CycleNode" {
				var parent, child Value
				value, parent, child = syntheticJSONClassGraph()
				if cyclic {
					child.Fields["parent"] = parent
				}
			} else {
				value.Type = "Map<String,Account>"
				record := Object("Account")
				record.Fields["Name"] = String("plain")
				if cyclic {
					record.Fields["Parent"] = record
				}
				value.Map["root"] = record
			}
			value.Static = "Object"
			_, err := machine.coerceCast(target, value)
			var thrown *apexThrowError
			want := "Invalid conversion from runtime type " + value.Type + " to " + target
			if !errors.As(err, &thrown) || thrown.value.Type != "System.TypeException" || thrown.value.Fields["message"].Text != want {
				t.Fatalf("target=%s cyclic=%v: %v; want TypeException %q", target, cyclic, err, want)
			}
		}
	}
}
