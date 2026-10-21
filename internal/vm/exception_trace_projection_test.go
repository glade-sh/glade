package vm

import (
	"reflect"
	"testing"
)

func TestExceptionTraceProjectionPreservesDiagnostics(t *testing.T) {
	stack := []callFrame{
		{Symbol: "OriginProbe.columnBoundary", File: "OriginProbe.cls", Line: 4, Column: 9},
		{Symbol: "TraceEmitter.constructed", File: "TraceEmitter.cls", Line: 2, Column: 5},
		{Symbol: "TraceEmitter.constructed", File: "TraceEmitter.cls", Line: 3, Column: 9, statement: true},
	}
	before := stackFrames(stack)
	value := annotateConstructedException(Object("DmlException"), stack)
	trace := value.Fields["__stackTrace"].Text
	// C03-01 named captures retain one helper and one caller at column 1.
	if want := "Class.TraceEmitter.constructed: line 3, column 1\nClass.OriginProbe.columnBoundary: line 4, column 1"; trace != want {
		t.Fatalf("trace = %q, want %q", trace, want)
	}
	if !reflect.DeepEqual(stackFrames(stack), before) {
		t.Fatal("Apex text projection changed structured diagnostic positions")
	}
	if got := value.Fields["__lineNumber"].Int; got != 3 {
		t.Fatalf("line number = %d, want 3", got)
	}
}

func TestAnonymousCaughtFaultTracePreservesDiagnostics(t *testing.T) {
	for _, symbol := range []string{"expr", "declare"} {
		stack := []callFrame{{Symbol: symbol, Line: 1, Column: 57, statement: true}}
		before := stackFrames(stack)
		value := annotateException(Object("NullPointerException"), stack)
		if got := value.Fields["__stackTrace"].Text; got != "AnonymousBlock: line 1, column 1" {
			t.Fatalf("trace = %q", got)
		}
		if !reflect.DeepEqual(stackFrames(stack), before) {
			t.Fatal("Apex trace projection changed diagnostic frame")
		}
	}
}
