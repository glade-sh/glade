package vm

import (
	"errors"
	"reflect"
	"testing"
)

// These uncaptured neighbors retain their origin/main diagnostics.
func TestExceptionRHSUncapturedNeighborOutputs(t *testing.T) {
	const dmlPrefix = "Insert failed. First exception on row 0; first error: CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY, Neighbor: execution of BeforeInsert\n\ncaused by: System.IllegalArgumentException: trigger\n\n"
	for _, tc := range []struct {
		name, body, triggerFile, typ, message string
		column                                int
		trigger                               bool
	}{
		{"standard", "throw new IllegalArgumentException('neighbor');", "", "IllegalArgumentException", "neighbor", 1, false},
		{"trigger_blank", "insert new Account(Name='neighbor');", "", "DmlException", dmlPrefix + "Neighbor: line 1, column 1\nClass.Caller.fail: []", 1, true},
		{"trigger_file", "insert new Account(Name='neighbor');", "Neighbor.trigger", "DmlException", dmlPrefix + "Trigger.Neighbor: line 1, column 1: []", 1, true},
		{"throw_variable", "NeighborException e = new NeighborException('neighbor'); throw e;", "", "NeighborException", "neighbor", 58, false},
		{"rethrow", "try { throw new NeighborException('neighbor'); } catch (NeighborException e) { throw e; }", "", "NeighborException", "neighbor", 80, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New(nil)
			org := testDataOrg()
			m.SetOrg(&org)
			opts := CompileOptions{APIVersion: "67.0"}
			body, err := CompileAnonymousWithOptions(tc.body, opts)
			if err != nil {
				t.Fatal(err)
			}
			if err := m.RegisterClass(Class{Name: "NeighborException", SuperClass: "Exception"}); err != nil {
				t.Fatal(err)
			}
			if err := m.RegisterClass(Class{Name: "Caller", Methods: map[string]Method{"fail": {Name: "Caller.fail", ClassName: "Caller", IsStatic: true, APIVersion: "67.0", Program: body}}}); err != nil {
				t.Fatal(err)
			}
			if tc.trigger {
				trigger, err := CompileAnonymousWithOptions("throw new IllegalArgumentException('trigger');", opts)
				if err != nil {
					t.Fatal(err)
				}
				if err := m.RegisterTrigger(Trigger{Name: "Neighbor", File: tc.triggerFile, Object: "Account", Timing: "before", Operation: "insert", APIVersion: "67.0", Program: trigger}); err != nil {
					t.Fatal(err)
				}
			}
			p, err := CompileAnonymousWithOptions("Caller.fail();", opts)
			if err != nil {
				t.Fatal(err)
			}
			_, err = m.Execute(p)
			var fault *RuntimeError
			if !errors.As(err, &fault) {
				t.Fatalf("expected runtime error, got %v", err)
			}
			want := []StackFrame{{Symbol: "Caller.fail", Line: 1, Column: tc.column}}
			if fault.Type != tc.typ || fault.Message != tc.message || !reflect.DeepEqual(fault.Stack, want) {
				t.Errorf("got type=%q message=%q stack=%#v; want type=%q message=%q stack=%#v", fault.Type, fault.Message, fault.Stack, tc.typ, tc.message, want)
			}
		})
	}
}
