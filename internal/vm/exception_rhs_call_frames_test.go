package vm

import "testing"

func TestExceptionRHSAnonymousUncapturedAPIBoundary(t *testing.T) {
	stack := []callFrame{
		{Symbol: "Example.fail", APIVersion: "68.0", Line: 1, anonymousCallerLine: 3},
		{Symbol: "Example.fail", Line: 2, statement: true},
	}
	fault := runtimeError(Object("DmlException"), stack).(*RuntimeError)
	if len(fault.Stack) != 1 || fault.Stack[0].Line != 2 {
		t.Fatalf("uncaptured API changed: %#v", fault.Stack)
	}
}
