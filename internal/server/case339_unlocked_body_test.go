package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

type case339ObservedBody struct {
	s *Server
	io.Reader
	available bool
}

func (body *case339ObservedBody) Read(p []byte) (int, error) {
	if body.s.mu.TryLock() {
		body.available = true
		body.s.mu.Unlock()
	}
	return body.Reader.Read(p)
}

func TestCase339FixtureAndResetReadOutsideOrgLock(t *testing.T) {
	for _, tc := range []struct{ path, body string }{
		{"/glade/fixture", `{"objects":[]}`},
		{"/glade/reset", `{"scope":"data"}`},
		{"/sobjects/Account", `{"Name":"Synthetic"}`},
		{"/tooling/executeAnonymous", `{"anonymousBody":"System.debug('ok');"}`},
	} {
		org := testOrg()
		s := New(&org)
		reader := &case339ObservedBody{s: s, Reader: strings.NewReader(tc.body)}
		req := httptest.NewRequest(http.MethodPost, serverTestDataPath+tc.path, reader)
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		if rec.Code >= 400 {
			t.Errorf("%s valid request rejected: %d %s", tc.path, rec.Code, rec.Body.String())
		}
		if !reader.available {
			t.Errorf("%s read body while org lock held (status %d)", tc.path, rec.Code)
		}
	}
}

func TestCase339CompositeRejectsGLADEBodyRoutes(t *testing.T) {
	control := httptest.NewRequest(http.MethodGet, serverTestDataPath+"/sobjects/Account", nil)
	if !compositeSubrequestRouteSupported(control.URL, "v"+storage.DefaultRESTAPIVersion) {
		t.Fatal("composite route control is not supported")
	}
	for _, path := range []string{serverTestDataPath + "/glade/fixture", serverTestDataPath + "/glade/reset"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		if compositeSubrequestRouteSupported(req.URL, "v"+storage.DefaultRESTAPIVersion) {
			t.Fatalf("composite unexpectedly dispatches %s", path)
		}
	}
}

type case339ReadDeadlineWriter struct {
	*httptest.ResponseRecorder
	sawDeadline bool
}

func (w *case339ReadDeadlineWriter) SetReadDeadline(deadline time.Time) error {
	if !deadline.IsZero() {
		w.sawDeadline = true
	}
	return nil
}

func TestCase339FixtureAndResetUseReadDeadline(t *testing.T) {
	for _, path := range []string{"/glade/fixture", "/glade/reset"} {
		org := testOrg()
		s := New(&org)
		w := &case339ReadDeadlineWriter{ResponseRecorder: httptest.NewRecorder()}
		req := httptest.NewRequest(http.MethodPost, serverTestDataPath+path, strings.NewReader(`{}`))
		s.ServeHTTP(w, req)
		if !w.sawDeadline {
			t.Errorf("%s did not set a body read deadline", path)
		}
	}
}
