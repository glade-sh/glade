package vm

import "testing"

func TestExceptionRHSAnonymousUncapturedAPIBoundary(t *testing.T) {
	stack := []callFrame{
		{Symbol: "Example.fail", APIVersion: "61.0", Line: 1},
		{Symbol: "Example.fail", Line: 2, statement: true},
	}
	fault := runtimeError(Object("DmlException"), stack).(*RuntimeError)
	if len(fault.Stack) != 1 || fault.Stack[0].Line != 2 {
		t.Fatalf("uncaptured API changed: %#v", fault.Stack)
	}
}

func TestExceptionRHSAnonymousTriggerBoundary(t *testing.T) {
	stack := []callFrame{
		{Symbol: "Example.fail", APIVersion: "67.0", Line: 1, anonymousCallerLine: 3},
		{Symbol: "ExampleTrigger", File: "Example.trigger", Line: 2, statement: true},
	}
	value := Object("DmlException")
	value.Fields["__stackTrace"] = String("previous trace")
	value.exceptionAnonymousCallerLine = 3
	fault := runtimeError(value, stack).(*RuntimeError)
	if len(fault.Stack) != 2 {
		t.Fatalf("trigger boundary changed: %#v", fault.Stack)
	}
}
