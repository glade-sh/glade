package vm

import "testing"

// LH01 captures fixed source52/61 helpers below modern62/67/68 callers.
func TestSystemHashLegacySourceCaller(t *testing.T) {
	hashProgram, err := CompileAnonymous("return token;")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"52.0", "61.0", "62.0", "67.0", "68.0"} {
		for _, list := range []bool{false, true} {
			name := api + "/object"
			if list {
				name = api + "/concrete singleton"
			}
			t.Run(name, func(t *testing.T) {
				machine := New(nil)
				if err := machine.RegisterClass(Class{Name: "LegacyHashKey", Fields: map[string]Field{"token": {Name: "token", Type: "Integer"}}, Methods: map[string]Method{
					"hashCode": {Name: "LegacyHashKey.hashCode", ClassName: "LegacyHashKey", ReturnType: "Integer", Access: "public", APIVersion: "52.0", Program: hashProgram},
				}}); err != nil {
					t.Fatal(err)
				}
				machine.currentMethod = Method{APIVersion: api}
				key := Object("LegacyHashKey")
				key.Fields["token"] = Int(7)
				input := key
				want := int64(7)
				if list {
					input = typedList("List<LegacyHashKey>")
					input.List = append(input.List, key)
					want = 38
				}
				got, err := machine.systemCustomHashCode(input, &Result{})
				if err != nil {
					t.Fatal(err)
				}
				if got.Kind != ValueInt || got.Int != want {
					t.Fatalf("System.hashCode=%v, want %d", got, want)
				}
			})
		}
	}
}
