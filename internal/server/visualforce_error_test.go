package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVisualforceRenderErrorOverlayIncludesFileAndExpression(t *testing.T) {
	srv := newVisualforceFixtureServer(t, "Broken.page", `<apex:page><apex:outputText value="{!missing + }"/></apex:page>`, "")

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/Broken", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	assertContains(t, rec.Body.String(), "Visualforce render error")
	assertContains(t, rec.Body.String(), "Broken.page")
	assertContains(t, rec.Body.String(), "missing +")
}

func TestGLADEVisualforceSupportReturnsComponentStatuses(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, serverTestDataPath+"/glade/visualforce/support", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Components []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Reason string `json:"reason,omitempty"`
		} `json:"components"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("support json: %v body=%s", err, rec.Body.String())
	}
	foundPage := false
	foundFlow := false
	for _, component := range payload.Components {
		switch component.Name {
		case "page":
			foundPage = component.Status == "partial"
		case "flow:interview":
			foundFlow = component.Status == "unsupported" && strings.TrimSpace(component.Reason) != ""
		}
	}
	if !foundPage || !foundFlow {
		t.Fatalf("components missing expected support rows: %#v", payload.Components)
	}
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("missing %q in %s", needle, haystack)
	}
}
