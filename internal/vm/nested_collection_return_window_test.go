package vm

import (
	"fmt"
	"testing"
)

func TestNestedCollectionReturnSkipsUnchangedReceiver(t *testing.T) {
	program, err := CompileAnonymous("other.add('changed');")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{20, 200} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			machine := New(nil)
			if err := machine.RegisterClass(Class{Name: "ReturnReader", Fields: map[string]Field{
				"lines": {Name: "lines", Type: "List<Object>"},
			}}); err != nil {
				t.Fatal(err)
			}
			rows := make([]Value, width)
			for i := range rows {
				rows[i] = Object("CallerEntry")
				rows[i].Fields["tags"] = List(String("synthetic"))
			}
			caller := Object("CallerEnvelope")
			caller.Fields["rows"] = List(rows...)
			receiver := Object("ReturnReader")
			receiver.Fields["lines"] = List(Object("ReaderLine"))
			other := testTypedList("List<String>", String("before"))
			machine.Globals["caller"], machine.Globals["reader"], machine.Globals["other"] = caller, receiver, other
			observer := &nestedCollectionRefreshWorkObserver{}
			machine.scopeAliasTraversalObserver = observer
			method := Method{Name: "ReturnReader.read", ClassName: "ReturnReader", ReturnType: "void",
				Params: []Param{{Name: "other", Type: "List<String>"}}, Program: program}
			if _, err := machine.callMethodWithReceiver(method, receiver, []Value{other}, &Result{}); err != nil {
				t.Fatal(err)
			}
			if got := len(machine.Globals["other"].List); got != 2 {
				t.Fatalf("unrelated mutation lost: size=%d", got)
			}
			t.Logf("width=%d return precheck visits=%d", width, observer.visits)
			if observer.visits > 16 {
				t.Fatalf("unchanged receiver scanned caller graph: visits=%d; want <=16", observer.visits)
			}
		})
	}
}
