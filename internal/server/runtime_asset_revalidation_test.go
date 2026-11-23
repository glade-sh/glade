package server

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

func runtimeAssetResponse(s *Server, tag string) *httptest.ResponseRecorder {
	return runtimeAssetResponseAt(s, "/lightning/runtime/shell/asset.css", tag)
}

func runtimeAssetResponseAt(s *Server, target, tag string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if tag != "" {
		req.Header.Set("If-None-Match", tag)
	}
	s.serveLightningShellRuntime(rec, req, []string{"shell", "asset.css"})
	return rec
}

func useRuntimeAssetRoot(t testing.TB, root string) {
	t.Helper()
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", root)
	t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "0")
	resetRuntimeAssetMemo()
	previous := runtimeAssetTrustAge
	runtimeAssetTrustAge = 0
	t.Cleanup(func() { runtimeAssetTrustAge = previous; runtimeAssetHashHook = nil; resetRuntimeAssetMemo() })
}

func TestLightningRuntimeAssetRevalidation(t *testing.T) {
	root := runtimeAssetFixture(t, []byte("first"))
	useRuntimeAssetRoot(t, root)
	var hashes atomic.Int32
	runtimeAssetHashHook = func(string) { hashes.Add(1) }
	s := &Server{}
	first := runtimeAssetResponse(s, "")
	wantTag := first.Header().Get("ETag")
	if first.Code != http.StatusOK || first.Body.String() != "first" || wantTag == "" || !strings.Contains(first.Header().Get("Cache-Control"), "no-cache") {
		t.Fatalf("first response: %d %q %v", first.Code, first.Body.String(), first.Header())
	}
	for _, tag := range []string{wantTag, strings.TrimPrefix(wantTag, "W/"), "\"other\", " + wantTag, "*"} {
		got := runtimeAssetResponse(s, tag)
		if got.Code != http.StatusNotModified || got.Body.Len() != 0 || got.Header().Get("ETag") != wantTag {
			t.Fatalf("validator %q: %d %q %v", tag, got.Code, got.Body.String(), got.Header())
		}
	}
	if n := hashes.Load(); n != 0 {
		t.Fatalf("unversioned requests hashed a body %d times", n)
	}
	asset := filepath.Join(root, "lwcruntime/src/shell/asset.css")
	info, err := os.Stat(asset)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(asset, []byte("other"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(asset, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	changed := runtimeAssetResponse(s, wantTag)
	if changed.Code != http.StatusOK || changed.Body.String() != "other" || changed.Header().Get("ETag") == wantTag {
		t.Fatalf("same-size edit with restored mtime: %d %q %v", changed.Code, changed.Body.String(), changed.Header())
	}
	secondRoot := runtimeAssetFixture(t, []byte("third"))
	t.Setenv("GLADE_ROOT", secondRoot)
	switched := runtimeAssetResponse(s, changed.Header().Get("ETag"))
	if switched.Code != http.StatusOK || switched.Body.String() != "third" {
		t.Fatalf("root changed: %d %q", switched.Code, switched.Body.String())
	}
	stale := runtimeAssetResponse(s, "\"missing\"")
	if stale.Code != http.StatusOK || stale.Body.String() != "third" {
		t.Fatalf("stale validator: %d %q", stale.Code, stale.Body.String())
	}
	if err := os.Remove(filepath.Join(secondRoot, "lwcruntime/src/shell/asset.css")); err != nil {
		t.Fatal(err)
	}
	missing := runtimeAssetResponse(s, switched.Header().Get("ETag"))
	if missing.Code == http.StatusNotModified || missing.Code == http.StatusOK || missing.Header().Get("ETag") != "" || strings.Contains(missing.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("removed asset: %d %q %v", missing.Code, missing.Body.String(), missing.Header())
	}
}

// A file written inside the trust window has no stamp validator and is never
// remembered, so a coarse clock cannot serve stale bytes.
func TestLightningRuntimeAssetFreshFileHasNoValidator(t *testing.T) {
	root := runtimeAssetFixture(t, []byte("first"))
	useRuntimeAssetRoot(t, root)
	runtimeAssetTrustAge = time.Hour
	var hashes atomic.Int32
	runtimeAssetHashHook = func(string) { hashes.Add(1) }
	rec := runtimeAssetResponse(&Server{}, "*")
	if rec.Code != http.StatusOK || rec.Header().Get("ETag") != "" || rec.Body.String() != "first" {
		t.Fatalf("fresh file: %d %q %v", rec.Code, rec.Body.String(), rec.Header())
	}
	a := lightningRuntimeAssetURL("shell", "asset.css")
	b := lightningRuntimeAssetURL("shell", "asset.css")
	if a != b || hashes.Load() != 2 {
		t.Fatalf("fresh file was remembered: %s %s hashes=%d", a, b, hashes.Load())
	}
}

// The URL carries the content hash, computed once per content version, and the
// memo follows content changes under concurrent use.
func TestRuntimeAssetContentVersionMemo(t *testing.T) {
	root := runtimeAssetFixture(t, []byte("first"))
	useRuntimeAssetRoot(t, root)
	var hashes atomic.Int32
	runtimeAssetHashHook = func(string) { hashes.Add(1) }
	asset := filepath.Join(root, "lwcruntime/src/shell/asset.css")
	want := fmt.Sprintf("/lightning/runtime/shell/asset.css?v=%x", sha256.Sum256([]byte("first")))[:len("/lightning/runtime/shell/asset.css?v=")+16]
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := lightningRuntimeAssetURL("shell", "asset.css"); got != want {
				t.Error(got)
			}
		}()
	}
	wg.Wait()
	if n := hashes.Load(); n < 1 || n > 32 {
		t.Fatalf("hashes = %d", n)
	}
	before := hashes.Load()
	for i := 0; i < 20; i++ {
		lightningRuntimeAssetURL("shell", "asset.css")
	}
	if hashes.Load() != before {
		t.Fatalf("unchanged content hashed again: %d -> %d", before, hashes.Load())
	}
	// Same size, restored mtime: only ctime differs.
	info, _ := os.Stat(asset)
	if err := os.WriteFile(asset, []byte("FIRST"), 0644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(asset, info.ModTime(), info.ModTime())
	changed := lightningRuntimeAssetURL("shell", "asset.css")
	if changed == want || hashes.Load() != before+1 {
		t.Fatalf("memo kept stale version: %s hashes=%d", changed, hashes.Load())
	}
	// Versioned requests: a matching v is immutable, a stale v is not.
	version := changed[strings.Index(changed, "?v=")+3:]
	ok := runtimeAssetResponseAt(&Server{}, "/lightning/runtime/shell/asset.css?v="+version, "")
	if ok.Code != http.StatusOK || !strings.Contains(ok.Header().Get("Cache-Control"), "immutable") || ok.Body.String() != "FIRST" {
		t.Fatalf("versioned: %d %v", ok.Code, ok.Header())
	}
	old := runtimeAssetResponseAt(&Server{}, "/lightning/runtime/shell/asset.css?v="+strings.TrimPrefix(want, "/lightning/runtime/shell/asset.css?v="), "")
	if old.Code != http.StatusOK || strings.Contains(old.Header().Get("Cache-Control"), "immutable") || old.Body.String() != "FIRST" {
		t.Fatalf("old URL: %d %v", old.Code, old.Header())
	}
	if missing := lightningRuntimeAssetURL("shell", "absent.css"); missing != "/lightning/runtime/shell/absent.css" {
		t.Fatal(missing)
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
	// The stamp is trusted two seconds after the last write.
	time.Sleep(2100 * time.Millisecond)
	_, probe := runtimeAssetFixtureTag(b, root)
	s := &Server{}
	for _, tc := range []struct {
		name, tag string
		first     bool
	}{{"first", "", true}, {"cold", "", false}, {"conditional", probe, false}} {
		b.Run(tc.name, func(b *testing.B) {
			var bytes int
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				server := s
				if tc.first {
					server = &Server{}
				}
				rec := runtimeAssetResponse(server, tc.tag)
				bytes += rec.Body.Len()
			}
			b.ReportMetric(float64(bytes)/float64(b.N), "transfer-B/op")
		})
	}
}

func runtimeAssetFixtureTag(b testing.TB, _ string) (*httptest.ResponseRecorder, string) {
	rec := runtimeAssetResponse(&Server{}, "")
	return rec, rec.Header().Get("ETag")
}
