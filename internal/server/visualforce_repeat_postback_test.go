package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
)

var repeatPostbackTextInputs = regexp.MustCompile(`<input type="text" name="([^"]+)"`)

// Inputs bound to fields of the apex:repeat variable bind each iteration to
// its own row through the HTTP postback route, for an Apex inner-class list
// and a Contact list, nested (native rows RepConText and RepNestedRows of
// internal/visualforce/testdata/vf_render_markup/postback.tsv). A plain HTML
// form in the page takes no view state (row IdEmptyFormHtml).
func TestHandleVisualforcePostBindsRepeatRowInputs(t *testing.T) {
	const controller = `public class AjaxController {
  public class Row { public String v { get; set; } }
  public String v { get; set; }
  public List<Row> rows { get; set; }
  public List<Contact> cons { get; set; }
  public String msg { get; set; }
  public AjaxController() { rows = new List<Row>{ new Row(), new Row() }; cons = new List<Contact>{ new Contact(), new Contact() }; }
  public PageReference save2() { String r = ''; for (Contact x : cons) { r += x.LastName + ',' + x.DoNotCall + ';'; } msg = 'v=' + v + '|cons=' + r + '|rows=' + rows[0].v + ',' + rows[1].v; return null; }
}`
	for _, tc := range []struct {
		name, body, posted, want string
	}{
		{"contacts", `<apex:repeat value="{!cons}" var="c"><apex:inputText value="{!c.LastName}"/></apex:repeat>`,
			"j_id0:f1:j_id2:0:j_id3=v1;j_id0:f1:j_id2:1:j_id3=v2", "v=null|cons=v1,false;v2,false;|rows=null,null"},
		{"nested", `<apex:repeat value="{!rows}" var="r"><apex:repeat value="{!cons}" var="c"><apex:inputText value="{!c.LastName}"/></apex:repeat><apex:inputText value="{!r.v}"/></apex:repeat>`,
			"j_id0:f1:j_id2:0:j_id3:0:j_id4=v1;j_id0:f1:j_id2:0:j_id3:1:j_id4=v2;j_id0:f1:j_id2:0:j_id5=v3;j_id0:f1:j_id2:1:j_id3:0:j_id4=v4;j_id0:f1:j_id2:1:j_id3:1:j_id4=v5;j_id0:f1:j_id2:1:j_id5=v6",
			"v=null|cons=v4,false;v5,false;|rows=v3,v6"},
		// Not a captured row: a controller property of the row field's name,
		// bound outside the repeat, keeps its own value.
		{"outside property", `<apex:repeat value="{!rows}" var="r"><apex:inputText value="{!r.v}"/></apex:repeat><apex:inputText value="{!v}" id="vo"/>`,
			"r0=a;r1=b;vo=c", "v=c|cons=null,false;null,false;|rows=a,b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := newVisualforceFixtureServer(t, "Rep.page", `<apex:page controller="AjaxController"><apex:form id="f1">`+tc.body+
				`<apex:commandButton value="Save" action="{!save2}" id="btn"/><apex:outputText value="{!msg}" id="msg"/></apex:form><form>plain</form></apex:page>`, controller)
			first := httptest.NewRecorder()
			srv.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/apex/Rep", nil))
			if first.Code != http.StatusOK {
				t.Fatalf("initial status = %d body=%s", first.Code, first.Body.String())
			}
			page := first.Body.String()
			if !strings.Contains(page, "<form>plain</form>") {
				t.Fatalf("plain form took Glade's fields: %s", page)
			}
			form := url.Values{}
			form.Set(visualforce.ViewStateFormFieldName(), extractHTMLInput(page, visualforce.ViewStateFormFieldName()))
			form.Set("__vf_csrf", extractHTMLInput(page, "__vf_csrf"))
			form.Set(visualforce.ViewStateActionFieldName(), "{!save2}")
			form.Set("__vf_form", "j_id0:f1")
			// The org's page wraps the form in a div, so its generated ids
			// differ; post the native values to Glade's text inputs in order.
			names := repeatPostbackTextInputs.FindAllStringSubmatch(page, -1)
			pairs := strings.Split(tc.posted, ";")
			if len(names) != len(pairs) {
				t.Fatalf("text inputs %q, native %q", names, pairs)
			}
			for i, pair := range pairs {
				_, v, _ := strings.Cut(pair, "=")
				form.Set(names[i][1], v)
			}
			req := httptest.NewRequest(http.MethodPost, "/apex/Rep", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("want %q in %s", tc.want, rec.Body.String())
			}
		})
	}
}

// An apex:include of a page with its own controller (not a captured shape):
// the postback of the including page must not evaluate the included page's
// expressions against the including page's controller.
func TestHandleVisualforcePostIncludeOfPageWithOwnController(t *testing.T) {
	root := t.TempDir()
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Outer.page"), `<apex:page controller="AjaxController"><apex:form id="f1"><apex:inputText value="{!name}" id="n"/><apex:commandButton value="Save" action="{!save}" id="btn"/><apex:outputText value="{!msg}" id="msg"/></apex:form><apex:include pageName="Inner"/></apex:page>`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Inner.page"), `<apex:page controller="InnerController"><apex:repeat value="{!items}" var="i"><apex:outputText value="{!i}"/></apex:repeat><apex:outputText value="shown" rendered="{!flag}"/></apex:page>`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/AjaxController.cls"), `public class AjaxController {
  public String name { get; set; }
  public String msg { get; set; }
  public PageReference save() { msg = 'saved ' + name; return null; }
}`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/InnerController.cls"), `public class InnerController {
  public List<String> getItems() { return new List<String>{ 'i1', 'i2' }; }
  public Boolean getFlag() { return true; }
}`)
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
	srv := newLightningTestServer(t, &org, source)
	if srv.VisualforceHTMLUserID == "" {
		configureVisualforceTestPrincipal(t, srv)
	}
	srv.SetProjectIndex(typesys.Build(p, schema))
	if srv.runtimeErr != nil {
		t.Fatalf("compile fixture runtime: %v", srv.runtimeErr)
	}
	first := httptest.NewRecorder()
	srv.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/apex/Outer", nil))
	if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), "i1") || !strings.Contains(first.Body.String(), "shown") {
		t.Fatalf("initial status = %d body=%s", first.Code, first.Body.String())
	}
	page := first.Body.String()
	form := url.Values{}
	form.Set(visualforce.ViewStateFormFieldName(), extractHTMLInput(page, visualforce.ViewStateFormFieldName()))
	form.Set("__vf_csrf", extractHTMLInput(page, "__vf_csrf"))
	form.Set(visualforce.ViewStateActionFieldName(), "{!save}")
	form.Set("__vf_form", "j_id0:f1")
	form.Set("j_id0:f1:n", "ann")
	req := httptest.NewRequest(http.MethodPost, "/apex/Outer", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "saved ann") || !strings.Contains(rec.Body.String(), "i2") {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}
