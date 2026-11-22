package vm

import (
	"errors"
	"reflect"
	"testing"
)

func TestExceptionRHSAsyncOutputs(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			m := New(nil)
			org := testDataOrg()
			m.SetOrg(&org)
			m.EnableTestContext()
			opts := CompileOptions{APIVersion: api}
			body, err := CompileAnonymousWithOptions("throw new NeighborException();", opts)
			if err != nil {
				t.Fatal(err)
			}
			if err := m.RegisterClass(Class{Name: "NeighborException", SuperClass: "Exception"}); err != nil {
				t.Fatal(err)
			}
			method := Method{Name: "Worker.execute", ClassName: "Worker", APIVersion: api, ReturnType: "void", Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: body}
			if err := m.RegisterClass(Class{Name: "Worker", Interfaces: []string{"Queueable"}, Methods: map[string]Method{"execute": method}}); err != nil {
				t.Fatal(err)
			}
			p, err := CompileAnonymousWithOptions("System.enqueueJob(new Worker());", opts)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = m.Execute(p); err != nil {
				t.Fatal(err)
			}
			err = m.DrainTestSetupAsync()
			var fault *RuntimeError
			if !errors.As(err, &fault) {
				t.Fatalf("expected fault: %v", err)
			}
			wantMessage := "NeighborException:[__lineNumber=1, __stackTrace=Class.Worker.execute: line 1, column 1, __thrown=true]"
			wantStack := []StackFrame{{Symbol: "Worker.execute", Line: 1, Column: 1}}
			if fault.Type != "NeighborException" || fault.Message != wantMessage || !reflect.DeepEqual(fault.Stack, wantStack) {
				t.Fatalf("fault: %#v stack=%#v", fault, fault.Stack)
			}
		})
	}
}

func TestExceptionRHSAnonymousRethrowOutputs(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			m := New(nil)
			opts := CompileOptions{APIVersion: api}
			body, err := CompileAnonymousWithOptions("throw new NeighborException();", opts)
			if err != nil {
				t.Fatal(err)
			}
			if err := m.RegisterClass(Class{Name: "NeighborException", SuperClass: "Exception"}); err != nil {
				t.Fatal(err)
			}
			if err := m.RegisterClass(Class{Name: "Caller", Methods: map[string]Method{"fail": {Name: "Caller.fail", ClassName: "Caller", APIVersion: api, IsStatic: true, Program: body}}}); err != nil {
				t.Fatal(err)
			}
			p, err := CompileAnonymousWithOptions("try { Caller.fail(); } catch (Exception e) { throw e; }", opts)
			if err != nil {
				t.Fatal(err)
			}
			_, err = m.Execute(p)
			var fault *RuntimeError
			if !errors.As(err, &fault) {
				t.Fatalf("expected fault: %v", err)
			}
			wantMessage := "NeighborException:[__lineNumber=1, __stackTrace=Class.Caller.fail: line 1, column 1, __thrown=true]"
			wantStack := []StackFrame{{Symbol: "throw", Line: 1, Column: 46}}
			if fault.Type != "NeighborException" || fault.Message != wantMessage || !reflect.DeepEqual(fault.Stack, wantStack) {
				t.Fatalf("fault: %#v stack=%#v", fault, fault.Stack)
			}
		})
	}
}
