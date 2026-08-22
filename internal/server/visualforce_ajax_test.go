package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
)

func TestHandleVisualforceAjaxPartialRefresh(t *testing.T) {
	srv := newVisualforceFixtureServer(t, "Ajax.page", `<apex:page controller="AjaxController">
<apex:form id="f">
  <apex:outputPanel id="count"><apex:outputText value="{!count}"/></apex:outputPanel>
  <apex:commandButton id="inc" value="Inc" action="{!increment}" reRender="count"/>
</apex:form>
</apex:page>`, `public class AjaxController {
  public static String latest = '0';
  public String getCount() {
    return AjaxController.latest;
  }
  public PageReference increment() {
    AjaxController.latest = '1';
    return null;
  }
}`)

	first := httptest.NewRecorder()
	srv.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/apex/Ajax", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("initial status = %d body=%s", first.Code, first.Body.String())
	}
	viewState := extractHTMLInput(first.Body.String(), visualforce.ViewStateFormFieldName())
	if strings.TrimSpace(viewState) == "" {
		t.Fatalf("missing view state in initial html: %s", first.Body.String())
	}

	form := url.Values{}
	form.Set(visualforce.ViewStateFormFieldName(), viewState)
	form.Set("__vf_csrf", extractHTMLInput(first.Body.String(), "__vf_csrf"))
	form.Set(visualforce.ViewStateActionFieldName(), "{!increment}")
	form.Set("count", "1")
	form.Set("__vf_ajax", "1")
	form.Set("__vf_rerender", "f:count")
	req := httptest.NewRequest(http.MethodPost, "/apex/Ajax", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	second := httptest.NewRecorder()
	srv.ServeHTTP(second, req)
	if second.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", second.Code, second.Body.String())
	}
	if got := second.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("content type = %q body=%s", got, second.Body.String())
	}
	var payload visualforce.PartialResponse
	if err := json.Unmarshal(second.Body.Bytes(), &payload); err != nil {
		t.Fatalf("partial response json: %v body=%s", err, second.Body.String())
	}
	target, ok := payload.Targets["f:count"]
	if !ok {
		t.Fatalf("targets = %#v", payload.Targets)
	}
	if !strings.Contains(target, ">1<") {
		t.Fatalf("target html = %s", target)
	}
	if strings.TrimSpace(payload.ViewState) == "" {
		t.Fatalf("missing refreshed view state: %#v", payload)
	}
}

func TestHandleVisualforceGetRendersActionFunctionParamsStatusAndRegionHooks(t *testing.T) {
	srv := newVisualforceFixtureServer(t, "Ajax.page", `<apex:page controller="AjaxController">
<apex:form id="f">
  <apex:actionFunction name="refreshCount" action="{!increment}" reRender="count" status="saveStatus">
    <apex:param name="delta"/>
  </apex:actionFunction>
  <apex:actionRegion id="editor">
    <apex:inputText id="inside" value="{!inside}"/>
    <apex:commandButton value="Save" action="{!increment}" reRender="count" status="saveStatus"/>
  </apex:actionRegion>
  <apex:actionStatus id="saveStatus" startText="Saving" stopText="Saved"/>
  <apex:outputPanel id="count"><apex:outputText value="{!count}"/></apex:outputPanel>
</apex:form>
</apex:page>`, `public class AjaxController {
  public String inside { get; set; }
  public String getCount() { return '0'; }
  public PageReference increment() { return null; }
}`)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/Ajax", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"function refreshCount(delta)",
		"{name:'delta',value:delta}",
		"status:'saveStatus'",
		"closest(&#39;[data-vf-region]&#39;)",
		`data-vf-region="editor"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestHandleVisualforceAjaxActionFunctionParamsReachControllerAction(t *testing.T) {
	srv := newVisualforceFixtureServer(t, "Ajax.page", `<apex:page controller="AjaxController">
<apex:form id="f">
  <apex:actionFunction name="refreshCount" action="{!increment}" reRender="count">
    <apex:param name="delta" assignTo="{!deltaValue}"/>
  </apex:actionFunction>
  <apex:outputPanel id="count"><apex:outputText value="{!count}"/></apex:outputPanel>
</apex:form>
</apex:page>`, `public class AjaxController {
  public static String latest = '0';
  public String deltaValue { get; set; }
  public String getCount() { return AjaxController.latest; }
  public PageReference increment() {
    AjaxController.latest = deltaValue + ':' + ApexPages.currentPage().getParameters().get('delta');
    return null;
  }
}`)

	first := httptest.NewRecorder()
	srv.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/apex/Ajax", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("initial status = %d body=%s", first.Code, first.Body.String())
	}

	form := url.Values{}
	form.Set(visualforce.ViewStateFormFieldName(), extractHTMLInput(first.Body.String(), visualforce.ViewStateFormFieldName()))
	form.Set("__vf_csrf", extractHTMLInput(first.Body.String(), "__vf_csrf"))
	form.Set(visualforce.ViewStateActionFieldName(), "{!increment}")
	form.Set("__vf_ajax", "1")
	form.Set("__vf_rerender", "f:count")
	form.Set("delta", "5")
	form.Set("deltaValue", "5")
	req := httptest.NewRequest(http.MethodPost, "/apex/Ajax", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload visualforce.PartialResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("partial response json: %v body=%s", err, rec.Body.String())
	}
	if got := payload.Targets["f:count"]; !strings.Contains(got, ">5:5<") {
		t.Fatalf("target html = %s", got)
	}
}

func TestHandleVisualforceAjaxResponseCarriesRedirect(t *testing.T) {
	srv := newVisualforceFixtureServer(t, "Ajax.page", `<apex:page controller="AjaxController">
<apex:form id="f">
  <apex:commandButton value="Save" action="{!save}" reRender="f"/>
</apex:form>
</apex:page>`, `public class AjaxController {
  public PageReference save() {
    PageReference next = new PageReference('/apex/Done');
    next.setRedirect(true);
    return next;
  }
}`)

	first := httptest.NewRecorder()
	srv.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/apex/Ajax", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("initial status = %d body=%s", first.Code, first.Body.String())
	}

	form := url.Values{}
	form.Set(visualforce.ViewStateFormFieldName(), extractHTMLInput(first.Body.String(), visualforce.ViewStateFormFieldName()))
	form.Set("__vf_csrf", extractHTMLInput(first.Body.String(), "__vf_csrf"))
	form.Set(visualforce.ViewStateActionFieldName(), "{!save}")
	form.Set("__vf_ajax", "1")
	form.Set("__vf_rerender", "f")
	req := httptest.NewRequest(http.MethodPost, "/apex/Ajax", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload visualforce.PartialResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("partial response json: %v body=%s", err, rec.Body.String())
	}
	if payload.Redirect != "/apex/Done" {
		t.Fatalf("redirect = %q payload=%#v", payload.Redirect, payload)
	}
}

func TestHandleVisualforcePostRequiresViewState(t *testing.T) {
	srv := newVisualforceFixtureServer(t, "Ajax.page", `<apex:page controller="AjaxController">
<apex:form><apex:commandButton value="Inc" action="{!increment}"/></apex:form>
</apex:page>`, `public class AjaxController {
  public PageReference increment() { return null; }
}`)
	form := url.Values{}
	form.Set(visualforce.ViewStateActionFieldName(), "{!increment}")
	req := httptest.NewRequest(http.MethodPost, "/apex/Ajax", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "missing Visualforce view state") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestHandleVisualforcePostRejectsMissingCSRF(t *testing.T) {
	srv := newVisualforceFixtureServer(t, "Ajax.page", `<apex:page controller="AjaxController">
<apex:form><apex:commandButton value="Inc" action="{!increment}"/></apex:form>
</apex:page>`, `public class AjaxController {
  public PageReference increment() { return null; }
}`)
	first := httptest.NewRecorder()
	srv.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/apex/Ajax", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("initial status = %d body=%s", first.Code, first.Body.String())
	}
	form := url.Values{}
	form.Set(visualforce.ViewStateFormFieldName(), extractHTMLInput(first.Body.String(), visualforce.ViewStateFormFieldName()))
	form.Set(visualforce.ViewStateActionFieldName(), "{!increment}")
	req := httptest.NewRequest(http.MethodPost, "/apex/Ajax", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "view state csrf mismatch") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestHandleVisualforcePostRejectsWrongCSRF(t *testing.T) {
	srv := newVisualforceFixtureServer(t, "Ajax.page", `<apex:page controller="AjaxController">
<apex:form><apex:commandButton value="Inc" action="{!increment}"/></apex:form>
</apex:page>`, `public class AjaxController {
  public PageReference increment() { return null; }
}`)
	first := httptest.NewRecorder()
	srv.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/apex/Ajax", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("initial status = %d body=%s", first.Code, first.Body.String())
	}
	form := url.Values{}
	form.Set(visualforce.ViewStateFormFieldName(), extractHTMLInput(first.Body.String(), visualforce.ViewStateFormFieldName()))
	form.Set("__vf_csrf", "wrong-token")
	form.Set(visualforce.ViewStateActionFieldName(), "{!increment}")
	req := httptest.NewRequest(http.MethodPost, "/apex/Ajax", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "view state csrf mismatch") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestVisualforceGetInjectsCSRFIntoEveryForm(t *testing.T) {
	srv := newVisualforceFixtureServer(t, "Ajax.page", `<apex:page controller="AjaxController">
<apex:form id="a"></apex:form>
<apex:form id="b"></apex:form>
</apex:page>`, `public class AjaxController {}`)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/Ajax", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := strings.Count(rec.Body.String(), `name="__vf_csrf"`); got != 2 {
		t.Fatalf("csrf fields = %d body=%s", got, rec.Body.String())
	}
	if got := strings.Count(rec.Body.String(), `name="`+visualforce.ViewStateFormFieldName()+`"`); got != 2 {
		t.Fatalf("view state fields = %d body=%s", got, rec.Body.String())
	}
}

func TestLatestVisualforceFormValuesPreservesRepeatedFieldValues(t *testing.T) {
	values := latestFormValues(map[string][]string{
		"choices":                              {"A", "B"},
		"flag":                                 {"false", "true"},
		visualforce.ViewStateActionFieldName(): {"old", "{!save}"},
		visualforce.ViewStateFormFieldName():   {"old-state", "new-state"},
		"__vf_csrf":                            {"old-token", "new-token"},
	})
	if values["choices"] != "A;B" {
		t.Fatalf("choices = %q", values["choices"])
	}
	if values["flag"] != "true" {
		t.Fatalf("flag = %q", values["flag"])
	}
	if values[visualforce.ViewStateActionFieldName()] != "{!save}" || values[visualforce.ViewStateFormFieldName()] != "new-state" || values["__vf_csrf"] != "new-token" {
		t.Fatalf("control values = %#v", values)
	}
}

func newVisualforceFixtureServer(t *testing.T, pageName, pageMarkup, controllerSource string) *Server {
	t.Helper()
	root := t.TempDir()
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages", pageName), pageMarkup)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/AjaxController.cls"), controllerSource)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	srv := NewWithSource(&org, source)
	srv.SetProjectIndex(typesys.Build(p, schema))
	if srv.runtimeErr != nil {
		t.Fatalf("compile fixture runtime: %v", srv.runtimeErr)
	}
	return srv
}

func extractHTMLInput(htmlText, name string) string {
	marker := `name="` + name + `"`
	at := strings.Index(htmlText, marker)
	if at < 0 {
		marker = `name='` + name + `'`
		at = strings.Index(htmlText, marker)
	}
	if at < 0 {
		return ""
	}
	inputStart := strings.LastIndex(htmlText[:at], "<input")
	if inputStart < 0 {
		inputStart = at
	}
	inputEnd := strings.Index(htmlText[inputStart:], ">")
	if inputEnd < 0 {
		inputEnd = len(htmlText) - inputStart
	}
	input := htmlText[inputStart : inputStart+inputEnd]
	for _, prefix := range []string{`value="`, `value='`} {
		valueAt := strings.Index(input, prefix)
		if valueAt < 0 {
			continue
		}
		valueAt += len(prefix)
		quote := prefix[len(prefix)-1]
		valueEnd := strings.IndexByte(input[valueAt:], quote)
		if valueEnd < 0 {
			return input[valueAt:]
		}
		return input[valueAt : valueAt+valueEnd]
	}
	return ""
}
