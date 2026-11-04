package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCase339FixtureRequestSizeBound(t *testing.T) {
	org := testOrg()
	s := New(&org)
	body := `{"padding":"` + strings.Repeat("a", maxLocalRequestBodyBytes+1) + `"}`
	for _, path := range []string{"/glade/fixture", "/glade/reset"} {
		for _, unknownLength := range []bool{false, true} {
			req := httptest.NewRequest(http.MethodPost, serverTestDataPath+path, strings.NewReader(body))
			if unknownLength {
				req.ContentLength = -1
			}
			rec := httptest.NewRecorder()
			s.ServeHTTP(rec, req)
			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Errorf("%s unknown length %t: status %d, want 413", path, unknownLength, rec.Code)
			}
		}
	}
}
