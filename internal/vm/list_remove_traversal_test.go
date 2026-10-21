package vm

import (
	"errors"
	"testing"
)

// Breaks and uncaught continuation failures are qualified by the owned
// collection_traversal_native.json rows. Return exits and finer iterator phase
// remain local PENDING-ROWS checks, logged separately in that native fixture.
func TestExecListRemoveTraversalCandidateExit(t *testing.T) {
	for _, test := range []struct{ name, body string }{
		{"direct break", "values.remove(0); break;"},
		{"helper break", "TraversalFixture.drop(values); break;"},
		{"direct return", "values.remove(0); return;"},
		{"helper return", "TraversalFixture.drop(values); return;"},
	} {
		t.Run(test.name, func(t *testing.T) {
			machine := New(nil)
			helper, err := CompileAnonymous("values.remove(0);")
			if err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterMethod(Method{Name: "TraversalFixture.drop", ClassName: "TraversalFixture", IsStatic: true, Params: []Param{{Name: "values", Type: "List<Integer>"}}, Program: helper}); err != nil {
				t.Fatal(err)
			}
			program, err := CompileAnonymous("List<Integer> values=new List<Integer>{1,2,3}; for(Integer item:values){" + test.body + "}")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			values := machine.Globals["values"].List
			if len(values) != 2 || values[0].Int != 2 || values[1].Int != 3 {
				t.Fatalf("removal did not reach caller: %v", values)
			}
			if len(machine.activeCollectionTraversals) != 0 || len(machine.collectionTraversalMutationEpochs) != 0 {
				t.Fatal("traversal survived exit")
			}
		})
	}
}

func TestExecListRemoveTraversalCandidateNextStep(t *testing.T) {
	program, err := CompileAnonymous(`
List<Integer> values=new List<Integer>{1,2,3};
Integer visited=0;
try {
 for(Integer item:values) {
  visited++;
  values.remove(0);
 }
} catch (Exception failure) { System.assert(false,'Traversal failure was caught'); }
System.assert(false,'Traversal completed');
`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = New(nil).Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.FinalException" || runtimeErr.Message != "Cannot modify a collection while it is being iterated." {
		t.Fatalf("uncaught traversal outcome: %v", err)
	}
}

func TestExecListRemoveTraversalCandidateControls(t *testing.T) {
	program, err := CompileAnonymous(`
List<Integer> values=new List<Integer>{1,2,3};
Integer visited=0;
for(Integer item:values) {
 visited++;
 values.set(0,9);
 try { values.remove(99); } catch (System.ListException expected) {}
}
System.assertEquals(3,visited);
System.assertEquals(new List<Integer>{9,2,3},values);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
	// The native iterator row establishes this sequence's terminal failure,
	// not a readback of hasNext or the exact failing operation.
	program, err = CompileAnonymous(`
List<Integer> values=new List<Integer>{9,2};
Iterator<Integer> iterator=values.iterator();
iterator.next();
values.remove(0);
try {
 iterator.hasNext();
 iterator.next();
} catch(Exception failure) { System.assert(false,'Traversal failure was caught'); }
System.assert(false,'Iterator completed');
`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = New(nil).Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.FinalException" || runtimeErr.Message != "Cannot modify a collection while it is being iterated." {
		t.Fatalf("uncaught iterator outcome: %v", err)
	}
}
