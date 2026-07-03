package visualforce

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/vm"
)

func TestEncodeDecodeViewStateRoundTrip(t *testing.T) {
	payload := ViewStatePayload{
		PageName:         "Edit",
		ControllerType:   "AccountController",
		ControllerValues: map[string]vm.Value{"count": vm.Int(7), "active": vm.Bool(true)},
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
	if decoded.ControllerValues["count"].Kind != vm.ValueInt || decoded.ControllerValues["count"].Int != 7 {
		t.Fatalf("typed count = %#v", decoded.ControllerValues["count"])
	}
	if decoded.ControllerValues["active"].Kind != vm.ValueBool || !decoded.ControllerValues["active"].Bool {
		t.Fatalf("typed active = %#v", decoded.ControllerValues["active"])
	}
	if decoded.Version != CurrentViewStateVersion {
		t.Fatalf("decoded version = %d, want %d", decoded.Version, CurrentViewStateVersion)
	}
}

func TestApplyViewStateValuesRestoresTypedControllerFields(t *testing.T) {
	controller := vm.Object("CounterController")
	applyValueFields(&controller, map[string]vm.Value{
		"count":  vm.Int(9),
		"active": vm.Bool(true),
		"label":  vm.String("ready"),
	})
	if controller.Fields["count"].Kind != vm.ValueInt || controller.Fields["count"].Int != 9 {
		t.Fatalf("count = %#v", controller.Fields["count"])
	}
	if controller.Fields["active"].Kind != vm.ValueBool || !controller.Fields["active"].Bool {
		t.Fatalf("active = %#v", controller.Fields["active"])
	}
	if controller.Fields["label"].Kind != vm.ValueString || controller.Fields["label"].Text != "ready" {
		t.Fatalf("label = %#v", controller.Fields["label"])
	}
}

func TestRenderPageRejectsViewStateForDifferentPageOrController(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root+"/sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, root+"/force-app/main/default/pages/Edit.page", `<apex:page controller="EditController"><apex:outputText value="{!name}"/></apex:page>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}

	_, err = RenderPage(PageRenderRequest{
		Project:   p,
		VFIndex:   idx,
		Machine:   vm.New(nil),
		PageName:  "Edit",
		ViewState: &ViewStatePayload{PageName: "Other", ControllerType: "EditController", ControllerValues: map[string]vm.Value{"name": vm.String("leaked")}},
	})
	if err == nil || !strings.Contains(err.Error(), "view state page mismatch") {
		t.Fatalf("err = %v, want page mismatch", err)
	}

	_, err = RenderPage(PageRenderRequest{
		Project:   p,
		VFIndex:   idx,
		Machine:   vm.New(nil),
		PageName:  "Edit",
		ViewState: &ViewStatePayload{PageName: "Edit", ControllerType: "OtherController", ControllerValues: map[string]vm.Value{"name": vm.String("leaked")}},
	})
	if err == nil || !strings.Contains(err.Error(), "view state controller mismatch") {
		t.Fatalf("err = %v, want controller mismatch", err)
	}
}

func TestDecodeViewStateRejectsTamperedPayload(t *testing.T) {
	payload := ViewStatePayload{PageName: "Edit", CSRF: "token"}
	encoded, err := EncodeViewState(payload, nil)
	if err != nil {
		t.Fatal(err)
	}
	tampered := encoded[:len(encoded)-4] + "ZZZZ"
	if _, err := DecodeViewState(tampered, nil); err == nil {
		t.Fatal("expected tampered view state error")
	}
}

func TestVerifyViewStateCSRFRejectsEmptyPayloadToken(t *testing.T) {
	if err := VerifyViewStateCSRF(ViewStatePayload{}, "token"); err == nil {
		t.Fatal("expected missing payload CSRF to be rejected")
	}
}

func TestRenderPageInjectsCSRFFieldForLocalClients(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root+"/sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, root+"/force-app/main/default/pages/Edit.page", `<apex:page><apex:form><apex:outputText value="ready"/></apex:form></apex:page>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Machine: vm.New(nil), PageName: "Edit"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, `name="__vf_csrf"`) {
		t.Fatalf("html missing csrf field: %s", result.HTML)
	}
}

func TestInjectViewStateAddsFieldToEveryForm(t *testing.T) {
	rendered := InjectViewState(`<form id="a"></form><form id="b"></form>`, "state")
	if got := strings.Count(rendered, ViewStateFormFieldName()); got != 2 {
		t.Fatalf("view state fields = %d html=%s", got, rendered)
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
