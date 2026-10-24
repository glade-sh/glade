package vm

import (
	"fmt"
	"testing"
)

func TestIdleCollectionReturnCallRoutesBoundReceiverWork(t *testing.T) {
	for _, width := range []int{20, 200} {
		for _, route := range []string{"instance_receiver", "static_object_argument", "static_nested_argument", "getter_receiver", "nested_receiver"} {
			t.Run(fmt.Sprintf("%s/%d", route, width), func(t *testing.T) {
				machine, receiver, read, forward, nested, getter := idleCollectionReturnMachine(t, width)
				observer := &nestedCollectionRefreshWorkObserver{}
				machine.scopeAliasTraversalObserver = observer
				var got Value
				var err error
				switch route {
				case "instance_receiver":
					got, err = machine.callMethodWithReceiver(read, receiver, nil, &Result{})
				case "static_object_argument":
					got, err = machine.callMethodWithReceiver(forward, Null, []Value{receiver}, &Result{})
				case "static_nested_argument":
					forward.Program, err = CompileAnonymous("IdleReader typed = (IdleReader)input; return typed.read();")
					if err == nil {
						got, err = machine.callMethodWithReceiver(forward, Null, []Value{receiver}, &Result{})
					}
				case "getter_receiver":
					got, err = machine.callGetter("IdleReader", getter, receiver)
				case "nested_receiver":
					got, err = machine.callMethodWithReceiver(nested, receiver, nil, &Result{})
				}
				if err != nil {
					t.Fatal(err)
				}
				if got.Ref != receiver.Ref {
					t.Fatal("read-only call changed the receiver identity")
				}
				t.Logf("width=%d idle return visits=%d", width, observer.visits)
				if observer.visits > 16 {
					t.Fatalf("idle call walked the receiver graph: visits=%d; want <=16", observer.visits)
				}
			})
		}
	}
}

func idleCollectionReturnMachine(t testing.TB, width int) (*VM, Value, Method, Method, Method, Field) {
	t.Helper()
	readProgram, err := CompileAnonymous("return this;")
	if err != nil {
		t.Fatal(err)
	}
	forwardProgram, err := CompileAnonymous("return (IdleReader)input;")
	if err != nil {
		t.Fatal(err)
	}
	nestedProgram, err := CompileAnonymous("return read();")
	if err != nil {
		t.Fatal(err)
	}
	read := Method{Name: "IdleReader.read", ClassName: "IdleReader", ReturnType: "IdleReader", Access: "public", Program: readProgram}
	forward := Method{Name: "IdleReader.forward", ClassName: "IdleReader", ReturnType: "IdleReader", Access: "public", IsStatic: true,
		Params: []Param{{Name: "input", Type: "Object"}}, Program: forwardProgram}
	nested := Method{Name: "IdleReader.nested", ClassName: "IdleReader", ReturnType: "IdleReader", Access: "public", Program: nestedProgram}
	getterMethod := read
	getterMethod.Name = "IdleReader.current.get"
	getter := Field{Name: "current", Type: "IdleReader", Access: "public", Property: true, HasGetter: true, Getter: &getterMethod}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "IdleReader", Fields: map[string]Field{
		"lines": {Name: "lines", Type: "List<Object>", Access: "public"}, "current": getter,
	}, Methods: map[string]Method{"read": read, "forward": forward, "nested": nested}}); err != nil {
		t.Fatal(err)
	}
	rows := make([]Value, width)
	for i := range rows {
		rows[i] = Object("IdleEntry")
		rows[i].Fields["values"] = List(Int(int64(i)))
	}
	receiver := Object("IdleReader")
	receiver.Static = "IdleReader"
	receiver.Fields["lines"] = List(rows...)
	caller := Object("IdleCaller")
	caller.Fields["reader"] = receiver
	machine.Globals["reader"], machine.Globals["caller"] = receiver, caller
	// Earlier writes leave nonempty history, while the measured calls only read.
	machine.collectionMutationSeq = 7
	machine.recordCollectionMutation(newValueRef())
	return machine, receiver, read, forward, nested, getter
}

func BenchmarkIdleCollectionReturnStaticArgument(b *testing.B) {
	for _, width := range []int{20, 2000} {
		b.Run(fmt.Sprint(width), func(b *testing.B) {
			machine, receiver, _, forward, _, _ := idleCollectionReturnMachine(b, width)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				got, err := machine.callMethodWithReceiver(forward, Null, []Value{receiver}, &Result{})
				if err != nil {
					b.Fatal(err)
				}
				if got.Ref != receiver.Ref {
					b.Fatal("read-only call changed the receiver identity")
				}
			}
		})
	}
}
