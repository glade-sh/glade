package visualforce

import (
	"strings"
	"testing"
)

func TestFlowInterviewDiagnosticIsExplicitBlocker(t *testing.T) {
	if got := FlowInterviewUnsupportedDiagnostic(); got != "flow:interview requires local Flow runtime support and blocks full Visualforce component support" {
		t.Fatalf("diagnostic = %q", got)
	}
	spec, ok := StandardComponentSpec("flow", "interview")
	if !ok {
		t.Fatal("flow:interview missing from component registry")
	}
	if spec.Status != ComponentUnsupported || spec.Reason != FlowInterviewUnsupportedDiagnostic() {
		t.Fatalf("spec = %#v", spec)
	}

	tree, err := ParseMarkupTree(`<flow:interview name="Signup"/>`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = RenderMarkupTree(tree, &RenderContext{Expression: &ExpressionContext{}})
	if err == nil || !strings.Contains(err.Error(), FlowInterviewUnsupportedDiagnostic()) {
		t.Fatalf("err = %v, want flow runtime blocker diagnostic", err)
	}
}
