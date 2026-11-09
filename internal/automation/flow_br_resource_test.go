package automation

import (
	"testing"

	"github.com/glade-sh/glade/internal/dml"
)

func TestFlowResourceFormulaFunctionName(t *testing.T) {
	if flowResourceFormulaFunction != dml.FlowResourceFormulaCall {
		t.Fatalf("%q != %q", flowResourceFormulaFunction, dml.FlowResourceFormulaCall)
	}
}

// A flow formula resource that references another resource containing BR()
// reads the finished value (org capture flow_br_surfaces).
func TestExpandFlowFormulaReferencesWrapsBRResources(t *testing.T) {
	declared := map[string]string{
		"fbr":        "'a' & BR() & 'b'",
		"fplain":     "'a' & 'b'",
		"fnest":      "TEXT(LEN({!FBr}))",
		"fnestplain": "TEXT(LEN({!FPlain}))",
	}
	got := expandFlowFormulaReferences(declared["fnest"], declared, map[string]bool{"fnest": true})
	if want := "TEXT(LEN(\x1fGLADEFLOWRESOURCE('a' & BR() & 'b')))"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got = expandFlowFormulaReferences(declared["fnestplain"], declared, map[string]bool{"fnestplain": true})
	if want := "TEXT(LEN(('a' & 'b')))"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
