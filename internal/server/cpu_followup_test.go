package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// A short explicit cap exercises the same synchronous message as the native
// API 62/67 exhaustion rows in vm/testdata/conformance/cpu_time/native.
func TestCPUFollowupToolingMessage(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	handler.LimitMode = vm.LimitModeStrict
	caps, _ := vm.LimitCapsForProfile("strict-sync")
	caps.CPUTimeMS = 0
	handler.LimitCaps = caps
	for _, api := range []string{"62.0", "67.0"} {
		for run := 0; run < 2; run++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v"+api+"/tooling/executeAnonymous", strings.NewReader(`{"anonymousBody":"Integer i=1;"}`)))
			if api == "62.0" {
				if rec.Code != http.StatusNotFound {
					t.Fatalf("api %s: status=%d body=%s", api, rec.Code, rec.Body.String())
				}
				continue
			}
			if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"success":false`)) || !bytes.Contains(rec.Body.Bytes(), []byte("Apex CPU time limit exceeded")) {
				t.Fatalf("api %s run %d: status=%d body=%s", api, run, rec.Code, rec.Body.String())
			}
		}
	}
}
