package server

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runtimeAssetFixture(t testing.TB, content []byte) string {
	t.Helper()
	root := t.TempDir()
	for name, data := range map[string][]byte{
		"go.mod":                                []byte("module fixture\n"),
		"lwcruntime/src/shell/app.mjs":          []byte("shell"),
		"lwcruntime/src/shims/wire-adapter.mjs": []byte("shim"),
		"lwcruntime/src/slds/slds-loader.mjs":   []byte("loader"),
		"lwcruntime/src/shell/asset.css":        content,
	} {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func runtimeAssetResponse(tag string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/lightning/runtime/shell/asset.css", nil)
	if tag != "" {
		req.Header.Set("If-None-Match", tag)
	}
	(&Server{}).serveLightningShellRuntime(rec, req, []string{"shell", "asset.css"})
	return rec
}

func TestLightningRuntimeAssetRevalidation(t *testing.T) {
	root := runtimeAssetFixture(t, []byte("first"))
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", root)
	t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "0")
	first := runtimeAssetResponse("")
	wantTag := fmt.Sprintf("\"%x\"", sha256.Sum256([]byte("first")))
	if first.Code != http.StatusOK || first.Body.String() != "first" || first.Header().Get("ETag") != wantTag || !strings.Contains(first.Header().Get("Cache-Control"), "no-cache") {
		t.Fatalf("first response: %d %q %v", first.Code, first.Body.String(), first.Header())
	}
	for _, tag := range []string{wantTag, "W/" + wantTag, "\"other\", " + wantTag, "*"} {
		got := runtimeAssetResponse(tag)
		if got.Code != http.StatusNotModified || got.Body.Len() != 0 || got.Header().Get("ETag") != wantTag {
			t.Fatalf("validator %q: %d %q %v", tag, got.Code, got.Body.String(), got.Header())
		}
	}
	if err := os.WriteFile(filepath.Join(root, "lwcruntime/src/shell/asset.css"), []byte("other"), 0644); err != nil {
		t.Fatal(err)
	}
	changed := runtimeAssetResponse(wantTag)
	if changed.Code != http.StatusOK || changed.Body.String() != "other" || changed.Header().Get("ETag") == wantTag {
		t.Fatalf("same-size edit: %d %q %v", changed.Code, changed.Body.String(), changed.Header())
	}
	t.Setenv("GLADE_ROOT", runtimeAssetFixture(t, []byte("third")))
	switched := runtimeAssetResponse(changed.Header().Get("ETag"))
	if switched.Code != http.StatusOK || switched.Body.String() != "third" {
		t.Fatalf("root changed: %d %q", switched.Code, switched.Body.String())
	}
	stale := runtimeAssetResponse("\"missing\"")
	if stale.Code != http.StatusOK || stale.Body.String() != "third" {
		t.Fatalf("stale validator: %d %q", stale.Code, stale.Body.String())
	}
}

func BenchmarkLightningRuntimeAssetTransfer(b *testing.B) {
	content, err := os.ReadFile("../../lwcruntime/src/slds/design-system-2/dist/css/bundled/slds2.cosmos.css")
	if err != nil {
		b.Fatal(err)
	}
	root := runtimeAssetFixture(b, content)
	b.Setenv("GLADE_HOME", "")
	b.Setenv("GLADE_ROOT", root)
	b.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "0")
	tag := fmt.Sprintf("\"%x\"", sha256.Sum256(content))
	for _, tc := range []struct{ name, tag string }{{"cold", ""}, {"conditional", tag}} {
		b.Run(tc.name, func(b *testing.B) {
			var bytes int
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				rec := runtimeAssetResponse(tc.tag)
				bytes += rec.Body.Len()
			}
			b.ReportMetric(float64(bytes)/float64(b.N), "transfer-B/op")
		})
	}
}
