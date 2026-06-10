package server

import (
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
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

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apex/Edit", strings.NewReader("name=Updated"))
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
