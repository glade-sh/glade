//go:build darwin || linux

package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCase361VendorReportsMissingToolchainHome(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "")
	t.Setenv("TMPDIR", t.TempDir())
	recorder := httptest.NewRecorder()
	(&Server{}).handleLightningVendor(recorder, httptest.NewRequest(http.MethodGet, "/lightning/vendor/engine-dom.js", nil), []string{"engine-dom.js"})
	if recorder.Code < 400 || !strings.Contains(recorder.Body.String(), "HOME") {
		t.Fatalf("status %d, body %q", recorder.Code, recorder.Body.String())
	}
}
