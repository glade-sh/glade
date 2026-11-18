package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/visualforce"
)

// PstTextId in vf_render_markup/postback.tsv records the value and action
// result; its native command button posts by client ID.
func TestHandleVisualforceNativeCommandButtonPostback(t *testing.T) {
	const controller = `public class AjaxController {
  public String s { get; set; }
  public String msg { get; set; }
  public PageReference save() { msg = 's=' + s; return null; }
}`
	srv := newVisualforceFixtureServer(t, "Plain.page", `<apex:page controller="AjaxController"><apex:form id="f1"><apex:inputText value="{!s}" id="i1"/><apex:commandButton value="Save" action="{!save}" id="btn"/><apex:outputText value="{!msg}" id="msg"/></apex:form></apex:page>`, controller)
	first := httptest.NewRecorder()
	srv.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/apex/Plain", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("GET status=%d", first.Code)
	}
	page := first.Body.String()
	form := url.Values{}
	form.Set(visualforce.ViewStateFormFieldName(), extractHTMLInput(page, visualforce.ViewStateFormFieldName()))
	form.Set("__vf_csrf", extractHTMLInput(page, "__vf_csrf"))
	form.Set("__vf_form", "j_id0:f1")
	form.Set("j_id0:f1:i1", "v1")
	form.Set("j_id0:f1:btn", "Save")
	req := httptest.NewRequest(http.MethodPost, "/apex/Plain", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post := httptest.NewRecorder()
	srv.ServeHTTP(post, req)
	if post.Code != http.StatusOK || !strings.Contains(post.Body.String(), "s=v1") {
		t.Fatalf("POST status=%d body=%s", post.Code, post.Body.String())
	}
}

// The captured plain actionFunction calls the JSF submit function. This
// exercises the HTTP payload that the local bridge sends after that call.
func TestHandleVisualforceNativeActionFunctionPostback(t *testing.T) {
	const controller = `public class AjaxController {
  public String s { get; set; }
  public String msg { get; set; }
  public PageReference save() { msg = 's=' + s; return null; }
}`
	srv := newVisualforceFixtureServer(t, "Plain.page", `<apex:page controller="AjaxController"><apex:form id="f1"><apex:inputText value="{!s}" id="i1"/><apex:actionFunction name="af" action="{!save}" id="invoke"/><apex:outputText value="{!msg}" id="msg"/></apex:form></apex:page>`, controller)
	first := httptest.NewRecorder()
	srv.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/apex/Plain", nil))
	page := first.Body.String()
	if first.Code != http.StatusOK || !strings.Contains(page, `function af() { SfdcApp.Visualforce.VSManager.vfPrepareForms`) {
		t.Fatalf("GET status=%d native action function missing", first.Code)
	}
	form := url.Values{}
	form.Set(visualforce.ViewStateFormFieldName(), extractHTMLInput(page, visualforce.ViewStateFormFieldName()))
	form.Set("__vf_csrf", extractHTMLInput(page, "__vf_csrf"))
	form.Set("__vf_form", "j_id0:f1")
	form.Set(visualforce.ViewStateActionFieldName(), "{!save}")
	form.Set("j_id0:f1:i1", "v1")
	req := httptest.NewRequest(http.MethodPost, "/apex/Plain", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post := httptest.NewRecorder()
	srv.ServeHTTP(post, req)
	if post.Code != http.StatusOK || !strings.Contains(post.Body.String(), "s=v1") {
		t.Fatalf("POST status=%d body=%s", post.Code, post.Body.String())
	}
}
