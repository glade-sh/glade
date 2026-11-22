package vm

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

// Neither a platform exception nor a custom exception with constructor
// arguments was captured on the uncaught anonymous reporting route.
func TestExceptionRHSConstructorNeighbors(t *testing.T) {
	for _, tc := range []struct{ name, source, typ, message string }{
		{"standard_no_arguments", "throw new IllegalArgumentException();", "IllegalArgumentException", "Script-thrown exception"},
		{"custom_with_argument", "throw new NeighborException('neighbor');", "NeighborException", "neighbor"},
		{"top_level_custom", "throw new NeighborException();", "NeighborException", "NeighborException:[__lineNumber=1, __stackTrace=Class.Caller.fail: line 1, column 1, __thrown=true]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New(nil)
			opts := CompileOptions{APIVersion: "67.0"}
			body, err := CompileAnonymousWithOptions(tc.source, opts)
			if err != nil {
				t.Fatal(err)
			}
			if err = m.RegisterClass(Class{Name: "NeighborException", SuperClass: "Exception"}); err != nil {
				t.Fatal(err)
			}
			if err = m.RegisterClass(Class{Name: "Caller", Methods: map[string]Method{"fail": {Name: "Caller.fail", ClassName: "Caller", APIVersion: "67.0", IsStatic: true, Program: body}}}); err != nil {
				t.Fatal(err)
			}
			p, err := CompileAnonymousWithOptions("Caller.fail();", opts)
			if err != nil {
				t.Fatal(err)
			}
			_, err = m.Execute(p)
			var fault *RuntimeError
			if !errors.As(err, &fault) {
				t.Fatalf("expected fault: %v", err)
			}
			want := []StackFrame{{Symbol: "Caller.fail", Line: 1, Column: 1}}
			if fault.Type != tc.typ || fault.Message != tc.message || !reflect.DeepEqual(fault.Stack, want) {
				t.Fatalf("fault: %#v stack=%#v", fault, fault.Stack)
			}
		})
	}
}

// The VM also accepts internally constructed rethrow instructions. Their
// diagnostic remains the base diagnostic even though Apex rejects bare throw.
func TestExceptionRHSInternalRethrowNeighbor(t *testing.T) {
	m := New(nil)
	opts := CompileOptions{APIVersion: "67.0"}
	body, err := CompileAnonymousWithOptions("throw new NeighborException();", opts)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.RegisterClass(Class{Name: "NeighborException", SuperClass: "Exception"}); err != nil {
		t.Fatal(err)
	}
	if err = m.RegisterClass(Class{Name: "Caller", Methods: map[string]Method{"fail": {Name: "Caller.fail", ClassName: "Caller", APIVersion: "67.0", IsStatic: true, Program: body}}}); err != nil {
		t.Fatal(err)
	}
	p, err := CompileAnonymousWithOptions("try { Caller.fail(); } catch (Exception e) { throw e; }", opts)
	if err != nil {
		t.Fatal(err)
	}
	p.Instructions[0].Catches[0].Body[0].Expr = ir.Expr{}
	_, err = m.Execute(p)
	var fault *RuntimeError
	if !errors.As(err, &fault) {
		t.Fatalf("expected fault: %v", err)
	}
	want := []StackFrame{{Symbol: "Caller.fail", Line: 1, Column: 1}}
	if fault.Type != "NeighborException" || fault.Message != "NeighborException:[__lineNumber=1, __stackTrace=Class.Caller.fail: line 1, column 1, __thrown=true]" || !reflect.DeepEqual(fault.Stack, want) {
		t.Fatalf("fault: %#v stack=%#v", fault, fault.Stack)
	}
}

func TestExceptionRHSNestedShapeNeighbors(t *testing.T) {
	for _, tc := range []struct {
		name, exception string
		extraMethod     bool
	}{
		{"different_owner", "Other.NeighborException", false},
		{"declared_method", "Caller.NeighborException", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New(nil)
			opts := CompileOptions{APIVersion: "67.0"}
			class := Class{Name: tc.exception, Nested: true, SuperClass: "Exception", APIVersion: "67.0"}
			if tc.extraMethod {
				class.Methods = map[string]Method{"extra": {Name: tc.exception + ".extra", ClassName: tc.exception, ReturnType: "void"}}
			}
			if err := m.RegisterClass(class); err != nil {
				t.Fatal(err)
			}
			body, err := CompileAnonymousWithOptions("throw new "+tc.exception+"();", opts)
			if err != nil {
				t.Fatal(err)
			}
			if err = m.RegisterClass(Class{Name: "Caller", Methods: map[string]Method{"fail": {Name: "Caller.fail", ClassName: "Caller", APIVersion: "67.0", IsStatic: true, Program: body}}}); err != nil {
				t.Fatal(err)
			}
			p, err := CompileAnonymousWithOptions("Caller.fail();", opts)
			if err != nil {
				t.Fatal(err)
			}
			_, err = m.Execute(p)
			var fault *RuntimeError
			if !errors.As(err, &fault) {
				t.Fatalf("expected fault: %v", err)
			}
			want := []StackFrame{{Symbol: "Caller.fail", Line: 1, Column: 1}}
			message := tc.exception + ":[__lineNumber=1, __stackTrace=Class.Caller.fail: line 1, column 1, __thrown=true]"
			if fault.Type != tc.exception || fault.Message != message || !reflect.DeepEqual(fault.Stack, want) {
				t.Fatalf("fault: %#v stack=%#v", fault, fault.Stack)
			}
		})
	}
}
