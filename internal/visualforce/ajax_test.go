package visualforce

import (
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestParseAjaxPayloadIdentifiesActionTargetsAndSubmittedFields(t *testing.T) {
	payload := ParseAjaxPayload(map[string]string{
		"__vf_ajax":                  "1",
		"__vf_rerender":              "count, f:details, count",
		"__vf_csrf":                  "token",
		ViewStateActionFieldName():   "{!save}",
		ViewStateFormFieldName():     "encoded-view-state",
		"f:name":                     "Ada",
		"mode":                       "quick",
		"com.salesforce.extra.field": "kept",
	})
	if !payload.IsAjax {
		t.Fatalf("IsAjax = false, want true")
	}
	if payload.Action != "{!save}" {
		t.Fatalf("Action = %q", payload.Action)
	}
	if want := []string{"count", "f:details"}; !reflect.DeepEqual(payload.RerenderTargets, want) {
		t.Fatalf("RerenderTargets = %#v, want %#v", payload.RerenderTargets, want)
	}
	wantFields := map[string]string{
		"f:name":                     "Ada",
		"mode":                       "quick",
		"com.salesforce.extra.field": "kept",
	}
	if !reflect.DeepEqual(payload.SubmittedFields, wantFields) {
		t.Fatalf("SubmittedFields = %#v, want %#v", payload.SubmittedFields, wantFields)
	}
}

func TestVisualforceAjaxScriptPostsMarkersAndRefreshesViewState(t *testing.T) {
	script := VisualforceAjaxScript()
	for _, want := range []string{
		"window.GLADEVF.submit",
		"options=options||{}",
		"appendControls(data,options.region)",
		"setStatus(options.status,true)",
		"setStatus(options.status,false)",
		"appendParams(data,options.params)",
		"window.location.assign(p.redirect)",
		`data.set("__vf_ajax","1")`,
		`data.set("__vf_rerender",targets||"")`,
		"p.messages",
		"console.warn",
		ViewStateActionFieldName(),
		ViewStateFormFieldName(),
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q: %s", want, script)
		}
	}
}

func TestVisualforceAjaxScriptClearsStatusBeforeRedirect(t *testing.T) {
	script := VisualforceAjaxScript()
	redirectIndex := strings.Index(script, "window.location.assign(p.redirect)")
	clearIndex := strings.Index(script, "setStatus(options.status,false);window.location.assign(p.redirect)")
	if redirectIndex == -1 {
		t.Fatalf("script missing redirect assignment: %s", script)
	}
	if clearIndex == -1 {
		t.Fatalf("script should clear actionStatus before redirect: %s", script)
	}
}

func TestVisualforceAjaxFunctionCallIncludesAssignToParam(t *testing.T) {
	call := VisualforceAjaxFunctionCall("{!save}", "out", "", []VisualforceAjaxParam{{Name: "delta", AssignTo: "deltaValue"}})
	for _, want := range []string{"name:'delta'", "assignTo:'deltaValue'", "value:delta"} {
		if !strings.Contains(call, want) {
			t.Fatalf("call missing %q: %s", want, call)
		}
	}
}

func TestRenderPartialTargetsFindsScopedRerenderID(t *testing.T) {
	targets := RenderPartialTargets(`<html><body><form id="f"><div id="count" data-rerender="count"><span>1</span></div></form></body></html>`, []string{"f:count"})
	if got := targets["f:count"]; !strings.Contains(got, `id="count"`) || !strings.Contains(got, ">1<") {
		t.Fatalf("target = %q", got)
	}
}

func TestRenderPartialTargetsPrefersElementIDOverRerenderMetadata(t *testing.T) {
	html := `<html><body><script data-rerender="count"></script><div id="count" data-rerender="count"><span>5</span></div></body></html>`
	targets := RenderPartialTargets(html, []string{"count"})
	if got := targets["count"]; !strings.Contains(got, `<div id="count"`) || !strings.Contains(got, ">5<") {
		t.Fatalf("target = %q", got)
	}
}

func TestNewPartialResponseReportsMissingTargetDiagnostic(t *testing.T) {
	response := NewPartialResponse(`<html><body><div id="count"><span>1</span></div></body></html>`, "next-view-state", []string{"count", "missing"})
	if response.ViewState != "next-view-state" {
		t.Fatalf("ViewState = %q", response.ViewState)
	}
	if got := response.Targets["count"]; !strings.Contains(got, `id="count"`) || !strings.Contains(got, ">1<") {
		t.Fatalf("count target = %q", got)
	}
	if got := response.Targets["missing"]; !strings.Contains(got, `unsupported Visualforce partial refresh target`) {
		t.Fatalf("missing target = %q", got)
	}
	if len(response.Messages) != 1 || !strings.Contains(response.Messages[0], `missing`) || !strings.Contains(response.Messages[0], "element id not found") {
		t.Fatalf("messages = %#v", response.Messages)
	}
}

func TestRenderAjaxCommandButtonEmitsSubmitHook(t *testing.T) {
	rendered := renderAjaxMarkupForTest(t, `<apex:page><apex:form><apex:commandButton value="Inc" action="{!increment}" reRender="count"/></apex:form></apex:page>`)
	for _, want := range []string{"window.GLADEVF.submit", "__vf_action", "{!increment}", "count"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered missing %q: %s", want, rendered)
		}
	}
}

func TestRenderAjaxCommandButtonKeepsLabelOutOfTextContent(t *testing.T) {
	rendered := renderAjaxMarkupForTest(t, `<apex:page><apex:form><apex:commandButton value="Step" action="{!step}" reRender="out"/></apex:form></apex:page>`)
	for _, want := range []string{`<input type="submit"`, `value="Step"`, "window.GLADEVF.submit"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered missing %q: %s", want, rendered)
		}
	}
	if strings.Contains(rendered, `>Step</button>`) {
		t.Fatalf("commandButton label leaked into text content: %s", rendered)
	}
}

func TestRenderAjaxCommandLinkEmitsSubmitHook(t *testing.T) {
	rendered := renderAjaxMarkupForTest(t, `<apex:page><apex:form><apex:commandLink value="Inc" action="{!increment}" reRender="count"/></apex:form></apex:page>`)
	for _, want := range []string{"this.closest", "window.GLADEVF.submit", "{!increment}", "count"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered missing %q: %s", want, rendered)
		}
	}
}

func TestRenderAjaxActionSupportEmitsSubmitHook(t *testing.T) {
	rendered := renderAjaxMarkupForTest(t, `<apex:page><apex:form><apex:actionSupport event="keyup" action="{!increment}" reRender="count"/></apex:form></apex:page>`)
	for _, want := range []string{`class="actionSupport"`, `data-event="keyup"`, "window.GLADEVF.submit", "{!increment}", "count"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered missing %q: %s", want, rendered)
		}
	}
}

func TestRenderAjaxActionFunctionFindsPageForm(t *testing.T) {
	rendered := renderAjaxMarkupForTest(t, `<apex:page><apex:form><apex:actionFunction name="refreshCount" action="{!increment}" reRender="count"/></apex:form></apex:page>`)
	for _, want := range []string{"function refreshCount()", "document.forms[0]", "window.GLADEVF.submit", "{!increment}", "count"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered missing %q: %s", want, rendered)
		}
	}
	if strings.Contains(rendered, "this.form") {
		t.Fatalf("actionFunction should not depend on this.form: %s", rendered)
	}
}

func TestRenderAjaxActionFunctionSubmitsNamedParamsAndStatus(t *testing.T) {
	rendered := renderAjaxMarkupForTest(t, `<apex:page><apex:form>
		<apex:actionFunction name="refreshCount" action="{!increment}" reRender="count" status="saveStatus">
			<apex:param name="delta"/>
			<apex:param name="mode" value="fast"/>
		</apex:actionFunction>
	</apex:form></apex:page>`)
	for _, want := range []string{
		"function refreshCount(delta,mode)",
		"window.GLADEVF.submit",
		"{!increment}",
		"count",
		"status:'saveStatus'",
		"{name:'delta',value:delta}",
		"{name:'mode',value:(mode!==undefined?mode:'fast')}",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered missing %q: %s", want, rendered)
		}
	}
}

func TestRenderAjaxActionFunctionEvaluatesParamDefaultValue(t *testing.T) {
	tree, err := ParseMarkupTree(`<apex:page><apex:form>
		<apex:actionFunction name="refreshCount" action="{!increment}" reRender="count">
			<apex:param name="delta" value="{!count}"/>
		</apex:actionFunction>
	</apex:form></apex:page>`)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := RenderMarkupTree(tree, &RenderContext{
		PageName: "Ajax",
		Expression: &ExpressionContext{Variables: map[string]vm.Value{
			"count": vm.Int(7),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"function refreshCount(delta)", "{name:'delta',value:(delta!==undefined?delta:'7')}"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered missing %q: %s", want, rendered)
		}
	}
	if strings.Contains(rendered, "{!count}") {
		t.Fatalf("rendered leaked raw expression: %s", rendered)
	}
}

func TestRenderAjaxCommandButtonPassesActionStatusAndNearestRegion(t *testing.T) {
	rendered := renderAjaxMarkupForTest(t, `<apex:page><apex:form>
		<apex:actionRegion id="editor">
			<apex:inputText id="inside" value="{!inside}"/>
			<apex:commandButton value="Save" action="{!save}" reRender="count" status="saveStatus"/>
		</apex:actionRegion>
		<apex:actionStatus id="saveStatus" startText="Saving" stopText="Saved"/>
	</apex:form></apex:page>`)
	for _, want := range []string{
		`data-vf-region="editor"`,
		"closest(&#39;[data-vf-region]&#39;)",
		"status:&#39;saveStatus&#39;",
		"window.GLADEVF.submit",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered missing %q: %s", want, rendered)
		}
	}
}

func renderAjaxMarkupForTest(t *testing.T, markup string) string {
	t.Helper()
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := RenderMarkupTree(tree, &RenderContext{PageName: "Ajax", Expression: &ExpressionContext{}})
	if err != nil {
		t.Fatal(err)
	}
	return rendered
}
