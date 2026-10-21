package vm

import "testing"

func TestNestedInterfaceSignatureUsesCanonicalNamespace(t *testing.T) {
	machine := New(nil)
	for _, namespace := range []string{"sprig", "grove"} {
		for _, name := range []string{"DispatchCradle", "DispatchCradle.Token", "DispatchCradle.Receiver", "DispatchCradleTest.Basket"} {
			if err := machine.RegisterClass(Class{Name: name, Namespace: namespace}); err != nil {
				t.Fatal(err)
			}
		}
	}
	contract := Method{ClassName: "sprig.DispatchCradle.Receiver", Params: []Param{{Type: "Token"}}}
	for _, parameter := range []string{"DispatchCradle.Token", "sprig.DispatchCradle.Token"} {
		t.Run(parameter, func(t *testing.T) {
			implementation := Method{ClassName: "sprig.DispatchCradleTest.Basket", Params: []Param{{Type: parameter}}}
			if !machine.sameResolvedMethodParams(implementation, contract) || !machine.sameResolvedMethodParams(contract, implementation) {
				t.Fatal("qualified and lexical spellings of the same nested parameter must match in both directions")
			}
		})
	}
	// Equal nested names in different namespaces remain different identities.
	other := Method{ClassName: "grove.DispatchCradleTest.Basket", Params: []Param{{Type: "DispatchCradle.Token"}}}
	if machine.sameResolvedMethodParams(other, contract) || machine.sameResolvedMethodParams(contract, other) {
		t.Fatal("nested parameters in distinct namespaces must not match")
	}
	foreign := Method{ClassName: "sprig.DispatchCradleTest.Basket", Params: []Param{{Type: "grove.DispatchCradle.Token"}}}
	if machine.sameResolvedMethodParams(foreign, contract) || machine.sameResolvedMethodParams(contract, foreign) {
		t.Fatal("an explicit foreign namespace must not be replaced by the declaring namespace")
	}
}
