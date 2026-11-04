package server

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glade-sh/glade/internal/visualforce"
)

func TestCase339ServerBodyCapBoundary(t *testing.T) {
	limit := visualforce.MaxVisualforceUploadBytes + visualforce.MaxVisualforceHeaderBytes
	for _, tc := range []struct{ size, want int }{{limit, http.StatusNotFound}, {limit + 1, http.StatusNotFound}} {
		org := testOrg()
		s := New(&org)
		req := httptest.NewRequest(http.MethodPost, "/not-salesforce", bytes.NewReader(make([]byte, tc.size)))
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("body length %d: status %d, want %d", tc.size, rec.Code, tc.want)
		}
	}
}

type case339ReadProbe struct{ read bool }

func (body *case339ReadProbe) Read([]byte) (int, error) {
	body.read = true
	return 0, io.EOF
}
func (body *case339ReadProbe) Close() error { return nil }

func TestCase339UnboundedDeclaredBodyStillReachesHandler(t *testing.T) {
	org := testOrg()
	s := New(&org)
	body := &case339ReadProbe{}
	req := httptest.NewRequest(http.MethodPost, "/not-salesforce", nil)
	req.Body = body
	req.ContentLength = int64(visualforce.MaxVisualforceUploadBytes + visualforce.MaxVisualforceHeaderBytes + 1)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || !body.read {
		t.Fatalf("status=%d bodyRead=%t, want 404 after reading", rec.Code, body.read)
	}
}
