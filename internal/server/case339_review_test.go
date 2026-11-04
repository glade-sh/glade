package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/visualforce"
)

func TestCase339ReviewUnboundedRoutesRetainExistingBodyBehavior(t *testing.T) {
	limit := visualforce.MaxVisualforceUploadBytes + visualforce.MaxVisualforceHeaderBytes
	for _, path := range []string{"/not-salesforce", "/services/async/62.0/job/nonexistent/batch"} {
		org := testOrg()
		s := New(&org)
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(make([]byte, limit+1)))
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		if rec.Code == http.StatusRequestEntityTooLarge {
			t.Errorf("%s incorrectly rejected by generic transport limit", path)
		}
	}
}

func TestCase339ReviewBulkCSVExceedsFormerTransportCap(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	job := createBulkV1TestJob(t, handler, "insert", "Account", "")
	limit := visualforce.MaxVisualforceUploadBytes + visualforce.MaxVisualforceHeaderBytes
	body := "Name\n" + strings.Repeat("x", limit) + "\n"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, bulkV1TestPath+"/job/"+job+"/batch", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("bulk upload status=%d body=%s", rec.Code, rec.Body.String())
	}
}
