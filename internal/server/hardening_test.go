package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNoPanicOnMalformedServerRequests(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	requests := []*http.Request{
		httptest.NewRequest(http.MethodGet, "/not-salesforce", nil),
		httptest.NewRequest(http.MethodPost, "/services/data/v65.0/sobjects/Account", strings.NewReader("{")),
		httptest.NewRequest(http.MethodPatch, "/services/data/v65.0/sobjects/Account/bad-id", strings.NewReader(`{"Name":"Acme"}`)),
		httptest.NewRequest(http.MethodGet, "/services/data/v65.0/query?q=SELECT", nil),
		httptest.NewRequest(http.MethodPost, "/services/data/v65.0/tooling/executeAnonymous", strings.NewReader(`{"anonymousBody":"System.debug("}`)),
		httptest.NewRequest(http.MethodPost, "/services/data/v65.0/glade/fixture", strings.NewReader(`{"version":"wrong"}`)),
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
