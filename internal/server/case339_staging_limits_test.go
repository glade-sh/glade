package server

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/visualforce"
)

type case339CountBody struct{ remaining, read int64 }

func (b *case339CountBody) Read(p []byte) (int, error) {
	if b.remaining == 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	for i := range p {
		p[i] = ' '
	}
	b.remaining -= int64(len(p))
	b.read += int64(len(p))
	return len(p), nil
}
func (b *case339CountBody) Close() error { return nil }

func TestCase339ReviewRouteStagingLimits(t *testing.T) {
	for _, version := range []string{"62.0", "67.0"} {
		prefix := "/services/data/v" + version
		for _, tc := range []struct {
			method, path, contentType string
			limit                     int64
		}{
			{"POST", prefix + "/composite/batch", "", maxLocalRequestBodyBytes},
			{"POST", prefix + "/glade/db-manager/objects/Account/records", "", maxLocalRequestBodyBytes},
			{"PATCH", prefix + "/glade/db-manager/objects/Account/records/123", "", maxLocalRequestBodyBytes},
			{"POST", prefix + "/glade/fixture", "", maxLocalRequestBodyBytes},
			{"POST", prefix + "/glade/reset", "", maxLocalRequestBodyBytes},
			{"POST", prefix + "/tooling/executeAnonymous", "", maxLocalRequestBodyBytes},
			{"POST", "/services/apexrest/Synthetic", "", maxLocalRequestBodyBytes},
			{"POST", "/apex/Synthetic/remoting", "", visualforce.MaxVisualforceRemotingRequestBytes},
			{"POST", "/apex/Synthetic/remoteObjects", "", visualforce.MaxVisualforceRemotingRequestBytes},
			{"POST", "/apex/Synthetic", "multipart/form-data; boundary=x", visualforce.MaxVisualforceUploadBytes + visualforce.MaxVisualforceHeaderBytes},
			{"POST", "/lightning/wire/getRecord", "", 1 << 20},
			{"POST", "/lightning/apex/Synthetic/method", "", 1 << 20},
		} {
			t.Run(version+tc.path, func(t *testing.T) {
				b := &case339CountBody{remaining: tc.limit + 65536}
				req := httptest.NewRequest(tc.method, tc.path, nil)
				req.Body = b
				req.ContentLength = -1
				req.Header.Set("Content-Type", tc.contentType)
				s := &Server{}
				cleanup, ok := s.stageRequestBody(httptest.NewRecorder(), req)
				if !ok {
					t.Fatal("staging failed")
				}
				defer cleanup()
				if b.read > tc.limit+1 {
					t.Fatalf("staged %d bytes; route limit is %d", b.read, tc.limit)
				}
			})
		}
	}
}

func TestCase339ReviewRouteDeclaredOversize(t *testing.T) {
	for _, path := range []string{"/composite", "/composite/batch", "/glade/db-manager/objects/Account/records", "/tooling/executeAnonymous"} {
		t.Run(path, func(t *testing.T) {
			org := testOrg()
			s := New(&org)
			b := &case339CountBody{remaining: maxLocalRequestBodyBytes + 100}
			req := httptest.NewRequest(http.MethodPost, serverTestDataPath+path, nil)
			req.Body = b
			req.ContentLength = maxLocalRequestBodyBytes + 100
			rec := httptest.NewRecorder()
			s.ServeHTTP(rec, req)
			if rec.Code != http.StatusRequestEntityTooLarge || b.read != 0 {
				t.Fatalf("status %d; read %d bytes, want 413 without reading: %s", rec.Code, b.read, rec.Body.String())
			}
		})
	}
}

func TestCase339ReviewRouteBulkRemainsUnbounded(t *testing.T) {
	b := &case339CountBody{remaining: maxLocalRequestBodyBytes + 100}
	req := httptest.NewRequest(http.MethodPut, "/services/data/v67.0/jobs/ingest/synthetic/batches", nil)
	req.Body = b
	req.ContentLength = -1
	s := &Server{}
	cleanup, ok := s.stageRequestBody(httptest.NewRecorder(), req)
	if !ok {
		t.Fatal("staging failed")
	}
	defer cleanup()
	if b.read != maxLocalRequestBodyBytes+100 {
		t.Fatalf("bulk read %d bytes", b.read)
	}
	contents, err := io.ReadAll(req.Body)
	if err != nil || len(contents) != int(b.read) || strings.TrimSpace(string(contents)) != "" {
		t.Fatal("bulk contents changed")
	}
}

func TestCase339ReviewGenericCompositePreservesLargeBody(t *testing.T) {
	for _, version := range []string{"60.0", "62.0", "65.0", "67.0"} {
		for _, known := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/known=%t", version, known), func(t *testing.T) {
				org := testOrg()
				server := New(&org)
				body := `{"ignored":"` + strings.Repeat("x", maxLocalRequestBodyBytes) + `","compositeRequest":[]}`
				req := httptest.NewRequest(http.MethodPost, "/services/data/v"+version+"/composite", strings.NewReader(body))
				if !known {
					req.ContentLength = -1
				}
				rec := httptest.NewRecorder()
				server.ServeHTTP(rec, req)
				// REST API 62 is outside the locally supported version window.
				if version == "62.0" {
					if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "unsupported API version v62.0") {
						t.Fatalf("API 62 routing: got %d: %s", rec.Code, rec.Body.String())
					}
					return
				}
				if known {
					if rec.Code != http.StatusRequestEntityTooLarge {
						t.Fatalf("declared length: got %d: %s", rec.Code, rec.Body.String())
					}
				} else if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "compositeRequest is required") {
					t.Fatalf("unknown length: got %d: %s", rec.Code, rec.Body.String())
				}
			})
		}
	}
}

func TestCase339ReviewRouteVisualforceURLEncodedLimit(t *testing.T) {
	const limit = 10 << 20 // net/http.Request.ParseForm's default form limit.
	body := &case339CountBody{remaining: limit + 65536}
	req := httptest.NewRequest(http.MethodPost, "/apex/Synthetic", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Body = body
	req.ContentLength = -1
	s := &Server{}
	cleanup, ok := s.stageRequestBody(httptest.NewRecorder(), req)
	if !ok {
		t.Fatal("staging failed")
	}
	defer cleanup()
	if body.read > limit+1 {
		t.Fatalf("staged %d bytes; form limit is %d", body.read, limit)
	}
}

func TestCase339ReviewGlobalStagingPreservesLargeBodies(t *testing.T) {
	const limit = int64(32 << 20)
	for _, path := range []string{"/unknown", "/services/data/v62.0/composite/tree/Account", "/services/data/v67.0/composite/graph", "/services/data/v67.0/jobs/ingest/synthetic/batches", "/services/async/67.0/job/synthetic/batch", "/services/data/v67.0/sobjects/Account"} {
		for _, known := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/known=%t", path, known), func(t *testing.T) {
				body := &case339CountBody{remaining: limit + 100}
				req := httptest.NewRequest(http.MethodPost, path, nil)
				req.Body = body
				if known {
					req.ContentLength = limit + 100
				} else {
					req.ContentLength = -1
				}
				rec := httptest.NewRecorder()
				cleanup, ok := (&Server{}).stageRequestBody(rec, req)
				if !ok {
					t.Fatalf("staging failed: %d %s", rec.Code, rec.Body.String())
				}
				defer cleanup()
				if body.read > limit+1 {
					t.Fatalf("staged %d, want bounded prefix", body.read)
				}
				n, err := io.Copy(io.Discard, req.Body)
				if err != nil || n != limit+100 {
					t.Fatalf("read %d: %v", n, err)
				}
			})
		}
	}
}
