package visualforce

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestRenderSetupLikePagePreservesHTMLAndEvaluatesNamespace(t *testing.T) {
	markup := `<apex:page controller="Demo" title="Setup"><apex:includeLightning /><apex:slds /><style type="text/css">#lightning { color: red; }</style><div id="lightning"><span>Loading</span></div><script type="text/javascript">$Lightning.use('{!JSENCODE(namespace)}:lightningOut');</script></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	runner := testRunner(t)
	runner.RegisterClass(vm.Class{
		Name: "Demo",
		Fields: map[string]vm.Field{
			"namespace": {
				Name:  "namespace",
				Type:  "String",
				Value: vm.String("verifiable"),
			},
		},
	})
	controller := vm.Object("Demo")
	controller.Type = "Demo"
	controller.Fields["namespace"] = vm.String("verifiable")
	ctx := RenderContext{
		VM:         runner,
		PageName:   "setup",
		Expression: &ExpressionContext{VM: runner, Controller: controller, ProjectNamespace: "verifiable"},
	}
	rendered, err := RenderMarkupTree(tree, &ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "<style") || !strings.Contains(rendered, "#lightning") {
		t.Fatalf("expected style tag, got %q", rendered)
	}
	if !strings.Contains(rendered, `<div id="lightning">`) {
		t.Fatalf("expected div wrapper, got %q", rendered)
	}
	if !strings.Contains(rendered, "verifiable:lightningOut") {
		t.Fatalf("expected namespace in script, got %q", rendered)
	}
	if !strings.Contains(rendered, "glade-vf-lightning-notice") {
		t.Fatalf("expected lightning notice, got %q", rendered)
	}
	if !strings.Contains(rendered, "salesforce-lightning-design-system") {
		t.Fatalf("expected SLDS stylesheet, got %q", rendered)
	}
}
