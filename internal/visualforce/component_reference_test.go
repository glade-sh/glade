package visualforce

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestRenderVisualforceComponentReferencesUseHierarchyClientIDs(t *testing.T) {
	markup := `<apex:page>
  <script>var nestedTextareaId = "{!$Component.namedForm.NestedTextInput}"; var pageScopeShortPath = "{!$Component.NestedTextInput}";</script>
  <apex:form>
    <script>var textareaId = "{!$Component.TemplateTextInput}"; var buttonId = "{!$Component.saveTemplate}";</script>
    <apex:outputPanel layout="none"><div class="hide"><apex:inputTextarea id="TemplateTextInput" value="{!draft}"/></div></apex:outputPanel>
    <apex:commandButton id="saveTemplate" value="Save" action="{!saveTemplate}"/>
  </apex:form>
  <apex:form id="namedForm">
    <apex:inputTextarea id="NestedTextInput" value="{!draft}"/>
  </apex:form>
</apex:page>`

	rendered := renderSupportMarkup(t, markup)
	for _, want := range []string{
		`<form id="j_id0:j_id2" name="j_id0:j_id2"`,
		`var textareaId = "j_id0:j_id2:TemplateTextInput";`,
		`var buttonId = "j_id0:j_id2:saveTemplate";`,
		`<textarea name="TemplateTextInput" id="j_id0:j_id2:TemplateTextInput">`,
		`<input type="submit" id="j_id0:j_id2:saveTemplate" name="j_id0:j_id2:saveTemplate"`,
		`<form id="j_id0:namedForm" name="j_id0:namedForm"`,
		`var nestedTextareaId = "j_id0:namedForm:NestedTextInput";`,
		`<textarea name="NestedTextInput" id="j_id0:namedForm:NestedTextInput">`,
		`var pageScopeShortPath = "";`,
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered missing %q: %s", want, rendered)
		}
	}
}

func TestRenderVisualforceOutputPanelHonorsRenderedAndLayoutNone(t *testing.T) {
	markup := `<apex:page><apex:form>
  <apex:outputPanel layout="none" rendered="{!showPrimary}"><span id="primaryBranch">primary</span></apex:outputPanel>
  <apex:outputPanel layout="none" rendered="{!NOT(showPrimary)}"><span id="secondaryBranch">secondary</span></apex:outputPanel>
</apex:form></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name          string
		showPrimary   bool
		visibleBranch string
		hiddenBranch  string
	}{
		{name: "primary", showPrimary: true, visibleBranch: "primaryBranch", hiddenBranch: "secondaryBranch"},
		{name: "secondary", showPrimary: false, visibleBranch: "secondaryBranch", hiddenBranch: "primaryBranch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			controller := vm.Object("VisualforceComponentContractController")
			controller.Fields["showPrimary"] = vm.Bool(tc.showPrimary)
			rendered, err := RenderMarkupTree(tree, &RenderContext{Expression: &ExpressionContext{Controller: controller}})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(rendered, tc.visibleBranch) {
				t.Errorf("rendered lacks visible branch %q: %s", tc.visibleBranch, rendered)
			}
			if strings.Contains(rendered, tc.hiddenBranch) {
				t.Errorf("rendered contains hidden branch %q: %s", tc.hiddenBranch, rendered)
			}
			if strings.Contains(rendered, `class="outputPanel"`) {
				t.Errorf("layout=none emitted an outputPanel wrapper: %s", rendered)
			}
		})
	}
}
