package vm

import (
	"errors"
	"reflect"
	"testing"
)

// These non-anonymous entry routes have no matching uncaught native rows.
func TestExceptionRHSHostedRouteOutputs(t *testing.T) {
	const wantMessage = "NeighborException:[__lineNumber=1, __stackTrace=Class.Caller.fail: line 1, column 1, __thrown=true]"
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, route := range []string{"rest", "lwc", "visualforce"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				m := New(nil)
				p, err := CompileAnonymousWithOptions("throw new NeighborException();", CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if err := m.RegisterClass(Class{Name: "NeighborException", SuperClass: "Exception"}); err != nil {
					t.Fatal(err)
				}
				method := Method{Name: "Caller.fail", ClassName: "Caller", APIVersion: api, ReturnType: "void", IsStatic: route != "visualforce", Access: "public", Modifiers: []string{"AuraEnabled"}, Program: p}
				if err := m.RegisterClass(Class{Name: "Caller", Methods: map[string]Method{"fail": method}}); err != nil {
					t.Fatal(err)
				}
				switch route {
				case "rest":
					_, err = m.CallStatic("Caller.fail", nil)
				default:
					var out UIInvocationResult
					if route == "lwc" {
						out, err = m.InvokeLWCMethod("Caller", "fail", nil)
					} else {
						out, err = m.InvokeVisualforceAction("Caller", "fail", "/apex/Control", nil)
					}
					if err != nil || out.Success || out.Error == nil {
						t.Fatalf("result=%#v error=%v", out, err)
					}
					if out.Error.Type != "NeighborException" || out.Error.Code != "" || out.Error.Message != wantMessage || out.Error.StackTraceString() != "Class.Caller.fail: line 1, column 1" {
						t.Fatalf("UI error: %#v trace=%q", out.Error, out.Error.StackTraceString())
					}
					err = out.Error.cause
				}
				var fault *RuntimeError
				if !errors.As(err, &fault) {
					t.Fatalf("expected fault, got %v", err)
				}
				wantStack := []StackFrame{{Symbol: "Caller.fail", Line: 1, Column: 1}}
				if fault.Type != "NeighborException" || fault.Message != wantMessage || !reflect.DeepEqual(fault.Stack, wantStack) {
					t.Fatalf("fault: %#v stack=%#v", fault, fault.Stack)
				}
			})
		}
	}
}
