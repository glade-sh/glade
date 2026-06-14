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

func newVisualforceFixtureServer(t *testing.T, pageName, pageMarkup, controllerSource string) *Server {
	t.Helper()
	root := t.TempDir()
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
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
