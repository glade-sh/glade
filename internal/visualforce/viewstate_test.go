package visualforce

import (
	"strings"
	"testing"
)

func TestEncodeDecodeViewStateRoundTrip(t *testing.T) {
	payload := ViewStatePayload{
		PageName:         "Edit",
		ControllerType:   "AccountController",
		ControllerFields: map[string]string{"name": "Acme"},
		CSRF:             "token",
	}
	encoded, err := EncodeViewState(payload, nil)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeViewState(encoded, nil)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.PageName != "Edit" || decoded.ControllerFields["name"] != "Acme" {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestDecodeViewStateRejectsTamperedPayload(t *testing.T) {
	payload := ViewStatePayload{PageName: "Edit", CSRF: "token"}
	encoded, err := EncodeViewState(payload, nil)
	if err != nil {
		t.Fatal(err)
	}
	tampered := encoded[:len(encoded)-4] + "XXXX"
	if _, err := DecodeViewState(tampered, nil); err == nil {
		t.Fatal("expected tampered view state error")
	}
}

func TestRenderRepeatAndDataTable(t *testing.T) {
	markup := `<apex:page><apex:repeat value="{!items}" var="item"><apex:outputText value="{!item}" /></apex:repeat><apex:dataTable value="{!items}" var="row"><apex:column value="{!row}" header="Name" /></apex:dataTable></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	runner := testRunner(t)
	items := vmList("Alpha", "Beta")
	controller := vmObject("Controller")
	controller.Fields["items"] = items
	ctx := RenderContext{
		VM:         runner,
		PageName:   "List",
		Expression: &ExpressionContext{VM: runner, Controller: controller},
		Scope:      NewScopeStack(),
		Metrics:    &RenderMetrics{ComponentCounts: map[string]int{}},
	}
	rendered, err := RenderMarkupTree(tree, &ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "Alpha") || !strings.Contains(rendered, "Beta") {
		t.Fatalf("rendered = %q", rendered)
	}
	if !strings.Contains(rendered, "<table") {
		t.Fatalf("rendered = %q", rendered)
	}
}

func TestRenderCustomComponentFallback(t *testing.T) {
	markup := `<apex:page><c:MissingBadge value="x" /></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	runner := testRunner(t)
	idx := Index{}
	ctx := RenderContext{
		VM:         runner,
		PageName:   "Demo",
		VFIndex:    &idx,
		Expression: &ExpressionContext{VM: runner},
	}
	rendered, err := RenderMarkupTree(tree, &ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "customComponentMissing") {
		t.Fatalf("rendered = %q", rendered)
	}
}

func TestRenderDynamicComponentFallback(t *testing.T) {
	markup := `<apex:page><apex:dynamicComponent componentValue="c:Missing" /></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	runner := testRunner(t)
	ctx := RenderContext{
		VM:         runner,
		Expression: &ExpressionContext{VM: runner},
	}
	rendered, err := RenderMarkupTree(tree, &ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "dynamicComponentFallback") {
		t.Fatalf("rendered = %q", rendered)
	}
}
