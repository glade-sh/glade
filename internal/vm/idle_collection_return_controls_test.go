package vm

import "testing"

func TestIdleCollectionReturnStaticArgumentPreservesInCallMutation(t *testing.T) {
	machine, receiver, _, forward, _, _ := idleCollectionReturnMachine(t, 1)
	if err := machine.RegisterClass(Class{Name: "IdleEntry", Fields: map[string]Field{
		"values": {Name: "values", Type: "List<Integer>", Access: "public"},
	}}); err != nil {
		t.Fatal(err)
	}
	machine.Globals["leaf"] = receiver.Fields["lines"].List[0].Fields["values"]
	program, err := CompileAnonymous("IdleReader typed = (IdleReader)input; typed.lines[0].values.add(42); return typed;")
	if err != nil {
		t.Fatal(err)
	}
	forward.Program = program
	before := machine.collectionMutationSeq
	got, err := machine.callMethodWithReceiver(forward, Null, []Value{receiver}, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if machine.collectionMutationSeq <= before {
		t.Fatal("in-call mutation did not advance the return window")
	}
	for route, value := range map[string]Value{
		"returned_argument": got.Fields["lines"].List[0].Fields["values"],
		"caller_holder":     machine.Globals["reader"].Fields["lines"].List[0].Fields["values"],
		"caller_leaf_alias": machine.Globals["leaf"],
	} {
		if len(value.List) != 2 || value.List[0].Int != 0 || value.List[1].Int != 42 {
			t.Fatalf("%s lost the nested mutation: %v", route, value)
		}
	}
}
