package vm

import "testing"

// Source hash dispatch is an existing contract; this test preserves it when a
// source class shadows a standard record name. It does not assert a new native rule.
func TestSystemHashSourceClassShadowsRecordName(t *testing.T) {
	program, err := CompileAnonymous("return 71;")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, name := range []string{"Account", "Contact", "Opportunity"} {
			t.Run(api+"/"+name, func(t *testing.T) {
				machine := New(nil)
				if err := machine.RegisterClass(Class{Name: "HashParent"}); err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterClass(Class{Name: name, SuperClass: "HashParent", Methods: map[string]Method{
					"hashCode": {Name: name + ".hashCode", ClassName: name, ReturnType: "Integer", Access: "public", APIVersion: api, Program: program},
				}}); err != nil {
					t.Fatal(err)
				}
				machine.currentMethod = Method{APIVersion: api}
				value := Object(name)
				value.classInstance = true
				got, err := machine.systemCustomHashCode(value, &Result{})
				if err != nil || got.Kind != ValueInt || got.Int != 71 {
					t.Fatalf("System.hashCode(%s) = %v, err %v; want 71", name, got, err)
				}
			})
		}
	}
}
