package vm

import (
	"strings"
	"testing"
)

// Candidate public formatting guard; native constructed-stack rows are pending.
func TestConstructedExceptionPublicColumnBoundary(t *testing.T) {
	s := []callFrame{{Symbol: "TraceEmitter.constructed", Line: 3, Column: 9}}
	if got := stackTraceString(s); !strings.HasSuffix(got, "column 1") {
		t.Fatal(got)
	}
	if got := stackFrames(s); got[0].Column != 9 {
		t.Fatal("diagnostic position changed")
	}
}
