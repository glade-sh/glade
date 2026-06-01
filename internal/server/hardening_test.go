package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestNoPanicOnMalformedServerRequests(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	dataPath := "/services/data/v" + storage.DefaultRESTAPIVersion
	requests := []*http.Request{
		httptest.NewRequest(http.MethodGet, "/not-salesforce", nil),
		httptest.NewRequest(http.MethodPost, dataPath+"/sobjects/Account", strings.NewReader("{")),
		httptest.NewRequest(http.MethodPatch, dataPath+"/sobjects/Account/bad-id", strings.NewReader(`{"Name":"Acme"}`)),
		httptest.NewRequest(http.MethodGet, dataPath+"/query?q=SELECT", nil),
		httptest.NewRequest(http.MethodPost, dataPath+"/tooling/executeAnonymous", strings.NewReader(`{"anonymousBody":"System.debug("}`)),
		httptest.NewRequest(http.MethodPost, dataPath+"/glade/fixture", strings.NewReader(`{"version":"wrong"}`)),
	}
	for _, request := range requests {
		assertNoPanic(t, func() {
			handler.ServeHTTP(httptest.NewRecorder(), request)
		})
	}
}

func assertNoPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()
	fn()
}
