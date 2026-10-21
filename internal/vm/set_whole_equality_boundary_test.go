package vm

import "testing"

// These controls preserve the pre-existing comparison path outside the stable
// custom-key shape captured by the whole-Set conformance fixture. Changed-hash
// and unsupported-declaration results are not additional native assertions.
func TestWholeSetEqualityPreservesBaselineBoundaries(t *testing.T) {
	machine := New(nil)
	for _, name := range []string{"StableKey", "EqualsOnlyKey", "NonBooleanKey"} {
		equals := Method{ReturnType: "Boolean", Params: []Param{{Name: "other", Type: "Object"}},
			Program: collisionProgram(t, "return true;")}
		if name == "NonBooleanKey" {
			equals.ReturnType = "String"
			equals.Program = collisionProgram(t, "return 'unused';")
		}
		class := Class{Name: name, Fields: map[string]Field{"code": {Type: "Integer", InitialValue: Int(17)}, "failHash": {Type: "Boolean", InitialValue: Bool(false)}},
			Methods: map[string]Method{"equals": equals}}
		if name != "EqualsOnlyKey" {
			class.Methods["hashCode"] = Method{ReturnType: "Integer", Program: collisionProgram(t, "if(failHash) throw new IllegalArgumentException('hash must not be reached'); return code;")}
		}
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct{ name, source string }{
		{"primitive", "Set<String> left=new Set<String>{'a'}; Set<String> right=new Set<String>{'A'};"},
		{"equalsOnly", "Set<EqualsOnlyKey> left=new Set<EqualsOnlyKey>{new EqualsOnlyKey()}; Set<EqualsOnlyKey> right=new Set<EqualsOnlyKey>{new EqualsOnlyKey()};"},
		{"nonBooleanSame", "NonBooleanKey key=new NonBooleanKey(); Set<NonBooleanKey> left=new Set<NonBooleanKey>{key}; Set<NonBooleanKey> right=new Set<NonBooleanKey>{key};"},
		{"nonBooleanDistinct", "Set<NonBooleanKey> left=new Set<NonBooleanKey>{new NonBooleanKey()}; Set<NonBooleanKey> right=new Set<NonBooleanKey>{new NonBooleanKey()};"},
		{"mixed", "Set<Object> left=new Set<Object>{new StableKey(),'a'}; Set<Object> right=new Set<Object>{new StableKey(),'a'};"},
		{"mixedThrowingHash", "StableKey key=new StableKey(); Set<Object> left=new Set<Object>{key,'a'}; Set<Object> right=new Set<Object>{new StableKey(),'a'}; key.failHash=true;"},
		{"changedLeftHash", "StableKey key=new StableKey(); Set<StableKey> left=new Set<StableKey>{key}; key.code=29; StableKey fresh=new StableKey(); fresh.code=29; Set<StableKey> right=new Set<StableKey>{fresh};"},
		{"changedRightHash", "StableKey fresh=new StableKey(); fresh.code=29; Set<StableKey> left=new Set<StableKey>{fresh}; StableKey key=new StableKey(); Set<StableKey> right=new Set<StableKey>{key}; key.code=29;"},
		{"unequalSize", "Set<StableKey> left=new Set<StableKey>{new StableKey()}; Set<StableKey> right=new Set<StableKey>();"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := machine.Execute(collisionProgram(t, tc.source)); err != nil {
				t.Fatal(err)
			}
			left, right := machine.Globals["left"], machine.Globals["right"]
			want := left.Equal(right)
			got, err := machine.apexEquals(left, right, &Result{})
			if err != nil || got != want {
				t.Fatalf("baseline=%v, comparison=%v err=%v", want, got, err)
			}
			if _, err := machine.Execute(collisionProgram(t, "System.assertEquals(left==right,left.equals(right));")); err != nil {
				t.Fatal(err)
			}
		})
	}
}
