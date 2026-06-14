package visualforce

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestRenderCoreVisualforceComponents(t *testing.T) {
	rendered := renderCoreComponentMarkup(t, `<apex:page title="Core">
		<apex:outputFormat value="Hello {0}, balance {1}">
			<apex:param value="{!name}"/>
			<apex:param value="42"/>
		</apex:outputFormat>
		<apex:message for="name"/>
		<apex:inputTextarea id="notes" value="{!notes}" rows="4" cols="30"/>
		<apex:inputSecret id="secret" value="{!secret}"/>
		<apex:inputCheckbox id="active" selected="{!active}"/>
		<apex:iframe src="/apex/Nested" width="320" height="200"/>
	</apex:page>`)

	for _, want := range []string{
		`Hello Ada, balance 42`,
		`class="message"`,
		`data-for="name"`,
		`<textarea name="notes" rows="4" cols="30">Line 1`,
		`&lt;Line 2&gt;</textarea>`,
		`<input type="password" name="secret" value="sauce" />`,
		`<input type="hidden" name="active" value="false" /><input type="checkbox" name="active" value="true" checked="checked" />`,
		`<iframe src="/apex/Nested" width="320" height="200"></iframe>`,
	} {
		assertContains(t, rendered, want)
	}
}

func TestRenderCoreSelectComponents(t *testing.T) {
	rendered := renderCoreComponentMarkup(t, `<apex:page>
		<apex:selectCheckboxes id="colors" value="{!colors}">
			<apex:selectOption itemValue="red" itemLabel="Red"/>
			<apex:selectOption itemValue="blue" itemLabel="Blue"/>
		</apex:selectCheckboxes>
		<apex:selectRadio id="size" value="{!size}">
			<apex:selectOption itemValue="small" itemLabel="Small"/>
			<apex:selectOption itemValue="large" itemLabel="Large"/>
		</apex:selectRadio>
	</apex:page>`)

	for _, want := range []string{
		`<span class="selectCheckboxes"`,
		`<input type="checkbox" name="colors" value="red" checked="checked" />`,
		`<label>Red</label>`,
		`<input type="checkbox" name="colors" value="blue" />`,
		`<span class="selectRadio"`,
		`<input type="radio" name="size" value="large" checked="checked" />`,
		`<label>Large</label>`,
	} {
		assertContains(t, rendered, want)
	}
}

func TestRenderSelectListExpandsSelectOptionsValue(t *testing.T) {
	rendered := renderCoreComponentMarkup(t, `<apex:page>
		<apex:selectList id="country" value="{!country}" size="1">
			<apex:selectOptions value="{!countryOptions}"/>
		</apex:selectList>
	</apex:page>`)

	for _, want := range []string{
		`<option value="US" selected="selected">United States</option>`,
		`<option value="CA">Canada</option>`,
		`<option value="MX">Mexico</option>`,
	} {
		assertContains(t, rendered, want)
	}
	if strings.Contains(rendered, "List[US, CA, MX]") {
		t.Fatalf("selectOptions rendered list text instead of option labels: %s", rendered)
	}
}

func TestRenderPageBlockTableDerivesHeaderFromFieldExpression(t *testing.T) {
	rendered := renderCoreComponentMarkup(t, `<apex:page>
		<apex:pageBlockTable value="{!accounts}" var="a">
			<apex:column value="{!a.Name}"/>
		</apex:pageBlockTable>
	</apex:page>`)

	for _, want := range []string{
		`<th>Account Name</th>`,
		`<td>Acme Probe</td>`,
	} {
		assertContains(t, rendered, want)
	}
}

func TestRenderDataTableDoesNotDeriveHeaderFromFieldExpression(t *testing.T) {
	rendered := renderCoreComponentMarkup(t, `<apex:page>
		<apex:dataTable value="{!accounts}" var="a">
			<apex:column value="{!a.Name}"/>
		</apex:dataTable>
	</apex:page>`)

	assertContains(t, rendered, `<table class="dataTable"><thead><tr><th></th></tr></thead>`)
	if strings.Contains(rendered, "Account Name") {
		t.Fatalf("dataTable derived default field header: %s", rendered)
	}
}

func TestRenderPanelGridUsesTableRowsAndFacets(t *testing.T) {
	rendered := renderCoreComponentMarkup(t, `<apex:page>
		<apex:panelGrid columns="2" id="probeGrid" captionClass="gridCaption" headerClass="gridHeader" footerClass="gridFooter">
			<apex:facet name="caption">Probe grid</apex:facet>
			<apex:facet name="header"><span>Left</span><span>Right</span></apex:facet>
			<apex:outputText value="A"/>
			<apex:outputText value="B"/>
			<apex:outputText value="C"/>
			<apex:facet name="footer">Done</apex:facet>
		</apex:panelGrid>
	</apex:page>`)

	for _, want := range []string{
		`<table id="probeGrid">`,
		`<caption class="gridCaption">Probe grid</caption>`,
		`<thead><tr><th class="gridHeader" colspan="2"><span>Left</span><span>Right</span></th></tr></thead>`,
		`<tbody><tr><td>A</td><td>B</td></tr><tr><td>C</td></tr></tbody>`,
		`<tfoot><tr><td class="gridFooter" colspan="2">Done</td></tr></tfoot>`,
	} {
		assertContains(t, rendered, want)
	}
	if strings.Contains(strings.ToLower(rendered), "<facet") {
		t.Fatalf("panelGrid leaked facet element: %s", rendered)
	}
}

func TestRenderCoreActionFunctionRegionStatusPoller(t *testing.T) {
	rendered := renderCoreComponentMarkup(t, `<apex:page>
		<apex:actionFunction name="refreshSummary" action="{!refresh}" reRender="summary"/>
		<apex:actionRegion id="editor"><span>Edit</span></apex:actionRegion>
		<apex:actionStatus id="saveStatus">
			<apex:facet name="start">Saving</apex:facet>
			<apex:facet name="stop">Saved</apex:facet>
		</apex:actionStatus>
		<apex:actionPoller action="{!tick}" reRender="summary" interval="3" enabled="true"/>
	</apex:page>`)

	for _, want := range []string{
		`function refreshSummary()`,
		`data-action="{!refresh}"`,
		`data-rerender="summary"`,
		`class="actionRegion"`,
		`data-region="editor"`,
		`class="actionStatus"`,
		`data-status="saveStatus"`,
		`<span class="actionStatusStart">Saving</span>`,
		`<span class="actionStatusStop">Saved</span>`,
		`class="actionPoller"`,
		`data-interval="5"`,
		`data-enabled="true"`,
	} {
		assertContains(t, rendered, want)
	}
}

func TestRenderActionStatusUsesStartAndStopTextAttributes(t *testing.T) {
	rendered := renderCoreComponentMarkup(t, `<apex:page>
		<apex:actionStatus id="status" startText="Working" stopText="Done"/>
	</apex:page>`)

	for _, want := range []string{
		`<span class="actionStatusStart">Working</span>`,
		`<span class="actionStatusStop">Done</span>`,
	} {
		assertContains(t, rendered, want)
	}
}

func TestRenderPageMessageIncludesDetailText(t *testing.T) {
	rendered := renderCoreComponentMarkup(t, `<apex:page>
		<apex:pageMessage severity="warning" summary="Security probe" detail="Rendered with safe text."/>
	</apex:page>`)

	for _, want := range []string{"Security probe", "Rendered with safe text."} {
		assertContains(t, rendered, want)
	}
}

func TestCoreComponentRegistryStatusesRemainPartial(t *testing.T) {
	for _, name := range []string{
		"outputFormat",
		"message",
		"inputTextarea",
		"inputSecret",
		"selectCheckboxes",
		"selectRadio",
		"actionFunction",
		"actionRegion",
		"actionStatus",
		"actionPoller",
		"iframe",
	} {
		spec, ok := StandardComponentSpec("apex", name)
		if !ok {
			t.Fatalf("missing registry spec for apex:%s", name)
		}
		if spec.Status != ComponentPartial {
			t.Fatalf("apex:%s status = %s, want %s", name, spec.Status, ComponentPartial)
		}
		if spec.Render == nil {
			t.Fatalf("apex:%s has nil renderer", name)
		}
	}
}

func renderCoreComponentMarkup(t *testing.T, markup string) string {
	t.Helper()
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	controller := vm.Object("CoreController")
	controller.Fields["name"] = vm.String("Ada")
	controller.Fields["notes"] = vm.String("Line 1\n<Line 2>")
	controller.Fields["secret"] = vm.String("sauce")
	controller.Fields["active"] = vm.Bool(true)
	controller.Fields["colors"] = vm.List(vm.String("red"))
	controller.Fields["size"] = vm.String("large")
	controller.Fields["country"] = vm.String("US")
	controller.Fields["countryOptions"] = vm.List(
		selectOptionValue("US", "United States"),
		selectOptionValue("CA", "Canada"),
		selectOptionValue("MX", "Mexico"),
	)
	account := vm.Object("Account")
	account.Fields["Name"] = vm.String("Acme Probe")
	controller.Fields["accounts"] = vm.List(account)
	scope := NewScopeStack()
	rendered, err := RenderMarkupTree(tree, &RenderContext{
		PageName: "Core",
		Expression: &ExpressionContext{
			Controller: controller,
			Scope:      scope,
		},
		Scope: scope,
	})
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(rendered, "\n", "")
}

func selectOptionValue(value, label string) vm.Value {
	option := vm.Object("SelectOption")
	option.Fields["value"] = vm.String(value)
	option.Fields["label"] = vm.String(label)
	option.Fields["disabled"] = vm.Bool(false)
	option.Fields["escapeItem"] = vm.Bool(true)
	return option
}
