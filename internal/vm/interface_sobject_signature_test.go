package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestInterfaceSObjectSignatureUsesDeclaringNamespace(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "birch"
	for _, namespace := range []string{"cedar", "birch"} {
		for _, name := range []string{"Envelope__c", "Sliver__c"} {
			qualified := namespace + "__" + name
			org.Objects[qualified] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: qualified}}
		}
	}
	machine := New(nil)
	machine.SetOrg(&org)
	for _, class := range []Class{
		{Name: "ToneChannel", Namespace: "cedar", IsInterface: true},
		{Name: "ToneRelay", Namespace: "birch"},
		{Name: "ToneChannelProof.QuietTone", Namespace: "birch"},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	machine.currentClass = "birch.ToneRelay"
	for _, name := range []string{"Envelope__c", "Sliver__c"} {
		t.Run(name, func(t *testing.T) {
			contract := Method{ClassName: "cedar.ToneChannel", Params: []Param{{Type: name}}}
			implementation := Method{ClassName: "birch.ToneChannelProof.QuietTone", Params: []Param{{Type: "cedar__" + name}}}
			if !machine.sameResolvedMethodParams(contract, implementation) || !machine.sameResolvedMethodParams(implementation, contract) {
				t.Fatal("provider-local and subscriber-qualified names of the same schema object must match")
			}
			other := Method{ClassName: implementation.ClassName, Params: []Param{{Type: "birch__" + name}}}
			if machine.sameResolvedMethodParams(contract, other) || machine.sameResolvedMethodParams(other, contract) {
				t.Fatal("objects with the same local name in different namespaces must remain distinct")
			}
		})
	}
	if got := machine.resolveTypeNameInClass("cedar.ToneChannel", "Absent__c"); got != "Absent__c" {
		t.Fatalf("unknown schema type was qualified without a declaration: %q", got)
	}
}
