package visualforce

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRenderMarkupTreeRendersOutputTextAndOutputField(t *testing.T) {
	markup := `<apex:page><apex:outputText value="{!greeting}" /><apex:outputText value="{!$Label.EditTitle}" /><apex:outputField value="{!account.Name}" /></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Metadata.Labels = append(org.Metadata.Labels, storage.LabelMetadata{Name: "EditTitle", Value: "Hello"})
	runner := vm.New(nil)
	runner.Org = &org
	controller := vm.Object("Controller")
	controller.Fields["greeting"] = vm.String("Greetings")
	account := vm.Object("Account")
	account.Fields["Name"] = vm.String("Acme")
	controller.Fields["account"] = account
	ctx := RenderContext{
		VM:         runner,
		PageName:   "Test",
		Expression: &ExpressionContext{VM: runner, Controller: controller},
	}
	rendered, err := RenderMarkupTree(tree, &ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "Greetings") {
		t.Fatalf("rendered = %q", rendered)
	}
	if !strings.Contains(rendered, "Hello") {
		t.Fatalf("rendered = %q", rendered)
	}
	if !strings.Contains(rendered, "Acme") {
		t.Fatalf("rendered = %q", rendered)
	}
}

func TestRenderMarkupTreeRendersPagePrimitives(t *testing.T) {
	markup := `<apex:page title="Demo"><apex:form><apex:outputPanel><apex:outputLink value="/next">Next</apex:outputLink></apex:outputPanel></apex:form><apex:pageBlock><apex:pageBlockSection><apex:pageBlockSectionItem><apex:outputText value="{!status}" /></apex:pageBlockSectionItem></apex:pageBlockSection></apex:pageBlock></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	runner := vm.New(nil)
	runner.Org = &org
	controller := vm.Object("Controller")
	controller.Fields["status"] = vm.String("ok")
	ctx := RenderContext{
		VM:         runner,
		PageName:   "Demo",
		Expression: &ExpressionContext{VM: runner, Controller: controller},
	}
	rendered, err := RenderMarkupTree(tree, &ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "<form") {
		t.Fatalf("rendered = %q", rendered)
	}
	if !strings.Contains(rendered, "<a href=\"/next\">Next</a>") {
		t.Fatalf("rendered = %q", rendered)
	}
	if !strings.Contains(rendered, "bPageBlock") {
		t.Fatalf("rendered = %q", rendered)
	}
}
