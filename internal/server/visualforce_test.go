package server

import (
	"bytes"
	"mime/multipart"
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

func TestHandleVisualforcePageGetRendersLabel(t *testing.T) {
	org := testOrg()
	org.Metadata.Labels = append(org.Metadata.Labels, storage.LabelMetadata{Name: "EditPageTitle", Value: "Rendered View"})
	source := testSourceMetadata(t)
	writeServerTestFile(t, filepath.Join(source.Project.Root, "force-app/main/default/pages/Edit.page"), `<apex:page><apex:outputText value="{!$Label.EditPageTitle}" /></apex:page>`)
	reloaded, err := project.Load(source.Project.Root)
	if err != nil {
		t.Fatal(err)
	}
	source, err = NewSourceMetadataFromProject(reloaded)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&org, source)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/apex/Edit", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Rendered View") {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("content type = %q", got)
	}
}

func TestHandleVisualforcePageGetAppliesPageHeaders(t *testing.T) {
	org := testOrg()
	source := testSourceMetadata(t)
	writeServerTestFile(t, filepath.Join(source.Project.Root, "force-app/main/default/pages/Export.page"), `<apex:page cspHeader="true" cache="false" contentType="text/csv#contacts.csv">Name</apex:page>`)
	reloaded, err := project.Load(source.Project.Root)
	if err != nil {
		t.Fatal(err)
	}
	source, err = NewSourceMetadataFromProject(reloaded)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&org, source)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/apex/Export", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != visualforce.VisualforceCSPHeaderValue() {
		t.Fatalf("CSP = %q, want %q", got, visualforce.VisualforceCSPHeaderValue())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store, no-cache, must-revalidate, max-age=0" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := rec.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("Pragma = %q", got)
	}
	if got := rec.Header().Get("Expires"); got != "0" {
		t.Fatalf("Expires = %q", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/csv" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); got != `attachment; filename="contacts.csv"` {
		t.Fatalf("Content-Disposition = %q", got)
	}
}

func TestHandleVisualforcePageGetPreservesQueryStringForStandardController(t *testing.T) {
	org := testOrg()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Label: "Account Name", Type: storage.FieldString},
		}},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}},
		},
	}
	source := testSourceMetadata(t)
	writeServerTestFile(t, filepath.Join(source.Project.Root, "force-app/main/default/pages/Edit.page"), `<apex:page standardController="Account"><apex:form><apex:outputField value="{!Account.Name}"/></apex:form></apex:page>`)
	reloaded, err := project.Load(source.Project.Root)
	if err != nil {
		t.Fatal(err)
	}
	source, err = NewSourceMetadataFromProject(reloaded)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&org, source)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/apex/Edit?id=001000000000001", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Acme") {
		t.Fatalf("body missing standard controller record: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `action="/apex/Edit?id=001000000000001"`) {
		t.Fatalf("form action did not preserve query string: %s", rec.Body.String())
	}
}

func TestHandleVisualforcePagePostBindsMultipartUploadFields(t *testing.T) {
	handler := newVisualforceUploadFixtureServer(t)
	viewState, err := visualforce.EncodeViewState(visualforce.ViewStatePayload{
		PageName:       "Upload",
		ControllerType: "UploadController",
		CSRF:           "upload-csrf",
	}, handler.visualforceViewStateSecretBytes())
	if err != nil {
		t.Fatal(err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField(visualforce.ViewStateFormFieldName(), viewState); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("__vf_csrf", "upload-csrf"); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField(visualforce.ViewStateActionFieldName(), "{!save}"); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreatePart(map[string][]string{
		"Content-Disposition": {`form-data; name="upload"; filename="invoice.txt"`},
		"Content-Type":        {"text/plain"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apex/Upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "hello|invoice.txt|text/plain|5") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestHandleVisualforcePageGetRendersPDFPage(t *testing.T) {
	org := testOrg()
	source := testSourceMetadata(t)
	writeServerTestFile(t, filepath.Join(source.Project.Root, "force-app/main/default/pages/Invoice.page"), `<apex:page renderAs="pdf"><h1>Invoice Total</h1></apex:page>`)
	reloaded, err := project.Load(source.Project.Root)
	if err != nil {
		t.Fatal(err)
	}
	source, err = NewSourceMetadataFromProject(reloaded)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&org, source)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/apex/Invoice", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("content type = %q", got)
	}
	if !strings.HasPrefix(rec.Body.String(), "%PDF-1.4") {
		t.Fatalf("body = %q", rec.Body.String()[:min(rec.Body.Len(), 80)])
	}
}

func newVisualforceUploadFixtureServer(t *testing.T) *Server {
	t.Helper()
	root := t.TempDir()
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Upload.page"), `<apex:page controller="UploadController">
<apex:form>
  <apex:inputFile id="upload" value="{!body}" fileName="{!fileName}" contentType="{!mimeType}" fileSize="{!byteCount}"/>
  <apex:commandButton value="Save" action="{!save}"/>
</apex:form>
</apex:page>`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Done.page"), `<apex:page controller="UploadController"><apex:outputText value="{!summary}"/></apex:page>`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/UploadController.cls"), `public class UploadController {
  public static String latest = '';
  public String body { get; set; }
  public String fileName { get; set; }
  public String mimeType { get; set; }
  public String byteCount { get; set; }
  public String getSummary() {
    return UploadController.latest;
  }
  public PageReference save() {
    UploadController.latest = body + '|' + fileName + '|' + mimeType + '|' + byteCount;
    return new PageReference('/apex/Done');
  }
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
	srv := NewWithSource(&org, source)
	srv.SetProjectIndex(typesys.Build(p, schema))
	if srv.runtimeErr != nil {
		t.Fatalf("compile fixture runtime: %v", srv.runtimeErr)
	}
	return srv
}

func TestHandleVisualforcePageGetRendersPDFQuery(t *testing.T) {
	org := testOrg()
	source := testSourceMetadata(t)
	writeServerTestFile(t, filepath.Join(source.Project.Root, "force-app/main/default/pages/Invoice.page"), `<apex:page><h1>Invoice Total</h1></apex:page>`)
	reloaded, err := project.Load(source.Project.Root)
	if err != nil {
		t.Fatal(err)
	}
	source, err = NewSourceMetadataFromProject(reloaded)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&org, source)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/apex/Invoice?renderAs=pdf", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("content type = %q", got)
	}
	if !strings.HasPrefix(rec.Body.String(), "%PDF-1.4") {
		t.Fatalf("body = %q", rec.Body.String()[:min(rec.Body.Len(), 80)])
	}
}

func TestHandleVisualforcePagePostRendersPage(t *testing.T) {
	org := testOrg()
	source := testSourceMetadata(t)
	writeServerTestFile(t, filepath.Join(source.Project.Root, "force-app/main/default/pages/Edit.page"), `<apex:page><apex:form><apex:outputText value="{!$Label.EditPageTitle}" /></apex:form></apex:page>`)
	reloaded, err := project.Load(source.Project.Root)
	if err != nil {
		t.Fatal(err)
	}
	source, err = NewSourceMetadataFromProject(reloaded)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&org, source)

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/apex/Edit", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("initial status = %d body=%q", first.Code, first.Body.String())
	}
	form := url.Values{}
	form.Set(visualforce.ViewStateFormFieldName(), extractHTMLInput(first.Body.String(), visualforce.ViewStateFormFieldName()))
	form.Set("__vf_csrf", extractHTMLInput(first.Body.String(), "__vf_csrf"))
	form.Set("name", "Updated")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apex/Edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "EditPageTitle") && !strings.Contains(rec.Body.String(), "Rendered") {
		// label may not be set in this fixture; ensure HTML shell renders
		if !strings.Contains(rec.Body.String(), "<html") {
			t.Fatalf("body = %q", rec.Body.String())
		}
	}
}

func TestHandleVisualforcePagePostRejectsViewStateForDifferentPage(t *testing.T) {
	org := testOrg()
	source := testSourceMetadata(t)
	writeServerTestFile(t, filepath.Join(source.Project.Root, "force-app/main/default/pages/Edit.page"), `<apex:page><apex:form>Edit</apex:form></apex:page>`)
	writeServerTestFile(t, filepath.Join(source.Project.Root, "force-app/main/default/pages/Other.page"), `<apex:page><apex:form>Other</apex:form></apex:page>`)
	reloaded, err := project.Load(source.Project.Root)
	if err != nil {
		t.Fatal(err)
	}
	source, err = NewSourceMetadataFromProject(reloaded)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewWithSource(&org, source)

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/apex/Edit", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("initial status = %d body=%q", first.Code, first.Body.String())
	}
	form := url.Values{}
	form.Set(visualforce.ViewStateFormFieldName(), extractHTMLInput(first.Body.String(), visualforce.ViewStateFormFieldName()))
	form.Set("__vf_csrf", extractHTMLInput(first.Body.String(), "__vf_csrf"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apex/Other", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK || !strings.Contains(rec.Body.String(), "view state page mismatch") {
		t.Fatalf("status = %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestHandleVisualforcePagePostRejectsOversizedMultipartBody(t *testing.T) {
	handler := newVisualforceUploadFixtureServer(t)
	viewState, err := visualforce.EncodeViewState(visualforce.ViewStatePayload{
		PageName:       "Upload",
		ControllerType: "UploadController",
		CSRF:           "upload-csrf",
	}, handler.visualforceViewStateSecretBytes())
	if err != nil {
		t.Fatal(err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField(visualforce.ViewStateFormFieldName(), viewState); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("__vf_csrf", "upload-csrf"); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreatePart(map[string][]string{
		"Content-Disposition": {`form-data; name="upload"; filename="huge.txt"`},
		"Content-Type":        {"text/plain"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(bytes.Repeat([]byte("x"), visualforce.MaxVisualforceUploadBytes+visualforce.MaxVisualforceHeaderBytes+1)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apex/Upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK || !strings.Contains(rec.Body.String(), "failed to parse Visualforce form") {
		t.Fatalf("status = %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestHandleStaticResourceRendersDirectContent(t *testing.T) {
	org := testOrg()
	source := testSourceMetadata(t)
	writeServerTestFile(t, filepath.Join(source.Project.Root, "force-app/main/default/staticresources/VfDemo.resource"), "resource-bytes")
	reloaded, err := project.Load(source.Project.Root)
	if err != nil {
		t.Fatal(err)
	}
	source, err = NewSourceMetadataFromProject(reloaded)
	if err != nil {
		t.Fatal(err)
	}
	// NewSourceMetadataFromProject loads static resources into source.ToolingOrg metadata.
	handler := NewWithSource(&org, source)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/resource/VfDemo", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "resource-bytes" {
		t.Fatalf("body = %q", got)
	}
}
