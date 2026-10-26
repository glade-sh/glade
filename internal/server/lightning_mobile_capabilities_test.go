package server

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// Exercise the public HTTP routes using the same component and native rows as
// the module and project conformance tests.
func TestLWCMobileCapabilitiesHTTPRoutesNative(t *testing.T) {
	f, err := os.Open("../lwcbrowser/testdata/mobile_capabilities/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("want two native API rows, got %d", len(rows)-1)
	}
	org := storage.NewOrgState()
	handler := New(&org)
	for _, row := range rows[1:] {
		for _, suffix := range []string{"", ".js"} {
			t.Run(row[0]+"/mobileCapabilities"+suffix, func(t *testing.T) {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/shims/lightning/mobileCapabilities"+suffix, nil))
				if rec.Code != http.StatusOK {
					t.Fatalf("shim status=%d: %s", rec.Code, rec.Body.String())
				}
				if !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
					t.Fatal("shim must be JavaScript")
				}
				input, err := json.Marshal(map[string]string{"source": rec.Body.String(), "expected": row[2], "api": row[1][:2]})
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "node", "../lwcbrowser/testdata/mobile_capabilities/module.mjs")
				cmd.Stdin = bytes.NewReader(input)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("served module: %v\n%s", err, out)
				}
			})
		}
	}
}
