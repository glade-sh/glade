package vm

import (
	"errors"
	"reflect"
	"testing"
)

// Benchmark the uncaptured standard-exception neighbor, with the same outcome.
func BenchmarkExceptionRHSStandardNeighbor(b *testing.B) {
	m := New(nil)
	opts := CompileOptions{APIVersion: "67.0"}
	body, err := CompileAnonymousWithOptions("throw new IllegalArgumentException('neighbor');", opts)
	if err != nil {
		b.Fatal(err)
	}
	if err := m.RegisterClass(Class{Name: "Caller", Methods: map[string]Method{"fail": {Name: "Caller.fail", ClassName: "Caller", APIVersion: "67.0", IsStatic: true, Program: body}}}); err != nil {
		b.Fatal(err)
	}
	p, err := CompileAnonymousWithOptions("Caller.fail();", opts)
	if err != nil {
		b.Fatal(err)
	}
	check := func(err error) {
		var fault *RuntimeError
		if !errors.As(err, &fault) || fault.Type != "IllegalArgumentException" || fault.Message != "neighbor" || !reflect.DeepEqual(fault.Stack, []StackFrame{{Symbol: "Caller.fail", Line: 1, Column: 1}}) {
			b.Fatalf("unexpected fault: %#v", err)
		}
	}
	_, err = m.Execute(p)
	check(err)
	b.ResetTimer()
	for b.Loop() {
		_, err = m.Execute(p)
		if err == nil {
			b.Fatal("missing exception")
		}
	}
	b.StopTimer()
	check(err)
}

func BenchmarkExceptionRHSTopLevelCustomNeighbor(b *testing.B) {
	m := New(nil)
	opts := CompileOptions{APIVersion: "67.0"}
	body, err := CompileAnonymousWithOptions("throw new NeighborException();", opts)
	if err != nil {
		b.Fatal(err)
	}
	if err = m.RegisterClass(Class{Name: "NeighborException", SuperClass: "Exception", APIVersion: "67.0"}); err != nil {
		b.Fatal(err)
	}
	if err = m.RegisterClass(Class{Name: "Caller", Methods: map[string]Method{"fail": {Name: "Caller.fail", ClassName: "Caller", APIVersion: "67.0", IsStatic: true, Program: body}}}); err != nil {
		b.Fatal(err)
	}
	p, err := CompileAnonymousWithOptions("Caller.fail();", opts)
	if err != nil {
		b.Fatal(err)
	}
	check := func(err error) {
		var fault *RuntimeError
		if !errors.As(err, &fault) || fault.Type != "NeighborException" || fault.Message != "NeighborException:[__lineNumber=1, __stackTrace=Class.Caller.fail: line 1, column 1, __thrown=true]" || !reflect.DeepEqual(fault.Stack, []StackFrame{{Symbol: "Caller.fail", Line: 1, Column: 1}}) {
			b.Fatalf("unexpected fault: %#v", err)
		}
	}
	_, err = m.Execute(p)
	check(err)
	b.ResetTimer()
	for b.Loop() {
		_, err = m.Execute(p)
		if err == nil {
			b.Fatal("missing exception")
		}
	}
	b.StopTimer()
	check(err)
}
