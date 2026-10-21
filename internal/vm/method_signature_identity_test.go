package vm

import "testing"

func TestMethodSignatureMatchesRegisteredNamespaceAlias(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Envelope.Payload", Namespace: "sample", IsInterface: true}); err != nil {
		t.Fatal(err)
	}
	left := Method{ClassName: "sample.Receiver", Params: []Param{{Type: "Envelope.Payload"}}}
	right := Method{ClassName: "sample.Envelope.Handler", Params: []Param{{Type: "sample.Envelope.Payload"}}}
	if !machine.sameResolvedMethodParams(left, right) {
		t.Fatal("registered aliases of one parameter type failed to match")
	}
	if err := machine.RegisterClass(Class{Name: "Envelope.Payload", Namespace: "other", IsInterface: true}); err != nil {
		t.Fatal(err)
	}
	left.Params[0].Type = "other.Envelope.Payload"
	if machine.sameResolvedMethodParams(left, right) {
		t.Fatal("distinct namespaces matched the same method signature")
	}
}
