package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestCase340LightningCachePrivatePerUse(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	shared := filepath.Join(tmp, "glade-lwc-cache")
	if err := os.Mkdir(shared, 0777); err != nil {
		t.Fatal(err)
	}
	p := project.Project{Root: filepath.Join(tmp, "project")}
	first := lightningCacheRoot(p)
	second := lightningCacheRoot(p)
	if first == second {
		t.Fatalf("cache roots are reusable: %q", first)
	}
	for _, path := range []string{first, second} {
		if filepath.Dir(path) != tmp {
			t.Fatalf("cache root %q is under shared parent", path)
		}
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() || info.Mode().Perm() != 0700 {
			t.Fatalf("cache root mode = %v", info.Mode())
		}
	}
}

func TestCase340LightningCacheRejectsWritableTempParent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	if err := os.Chmod(tmp, 0777); err != nil {
		t.Fatal(err)
	}
	if root := lightningCacheRoot(project.Project{}); root != "" {
		t.Fatalf("accepted unsafe temporary parent: %q", root)
	}
}

func TestCase340LightningCacheRejectsWritableTempAncestor(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "parent")
	child := filepath.Join(parent, "child")
	if err := os.MkdirAll(child, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0777); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", child)
	if root := lightningCacheRoot(project.Project{}); root != "" {
		t.Fatalf("accepted unsafe temporary ancestor: %q", root)
	}
}

func TestCase340LightningCacheRejectsUntrustedSymlinkPath(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	unsafeParent := filepath.Join(root, "unsafe")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(unsafeParent, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unsafeParent, 0777); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(unsafeParent, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", link)
	if cache := lightningCacheRoot(project.Project{}); cache != "" {
		t.Fatalf("accepted unsafe symlink path: %q", cache)
	}
}

func TestCase340ServerCloseRemovesLightningCache(t *testing.T) {
	root := filepath.Join(t.TempDir(), "cache")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	handler := New(&org)
	handler.lightning.cacheRoot = root
	if err := handler.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatalf("cache still exists or cannot be inspected: %v", err)
	}
	if err := handler.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCase340WarmLightningModuleUsesPrivateCache(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	cache := lightningCacheRoot(project.Project{Root: filepath.Join(tmp, "project")})
	if cache == "" {
		t.Fatal("private cache was not created")
	}
	module := filepath.Join(cache, "lwc", "c", "widget", "widget.js")
	if err := os.MkdirAll(filepath.Dir(module), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(module, []byte("export default 42;"), 0600); err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	handler := New(&org)
	handler.lightning = lightningState{cacheRoot: cache, cacheDir: filepath.Join(cache, "lwc"), manifest: lwcbrowser.Manifest{Modules: map[string]lwcbrowser.ModuleEntry{"c:widget": {URL: "/lightning/modules/c/widget/widget.js"}}}}
	defer handler.Close()
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/modules/c/widget/widget.js", nil))
		if rec.Code != http.StatusOK || rec.Body.String() != "export default 42;" {
			t.Fatalf("warm request %d: status=%d body=%q", i, rec.Code, rec.Body.String())
		}
	}
}

func TestCase340UnsafeTempHasActionableError(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Chmod(tmp, 0777); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tmp)
	previous := ensureLightningRoot
	ensureLightningRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { ensureLightningRoot = previous })
	org := storage.NewOrgState()
	handler := New(&org)
	err := handler.ensureLightningLocked()
	if err == nil || !strings.Contains(err.Error(), "TMPDIR") {
		t.Fatalf("missing temp-directory guidance: %v", err)
	}
}

func TestCase340VisualforceUsesWarmLightningCache(t *testing.T) {
	root, err := lightningTestRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(filepath.Join(root, "testdata", "local-tests", "lightning-out-vf"))
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	handler := newVisualforceHTMLTestServer(t, &org, source)
	manifest := lwcbrowser.Manifest{Modules: map[string]lwcbrowser.ModuleEntry{"c:counter": {URL: "/lightning/modules/c/counter/counter.js"}}}
	handler.lightning = lightningState{cacheDir: t.TempDir(), manifest: manifest, pageCfg: lwcbrowser.PageConfig{Manifest: manifest}}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/WidgetHost", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "glade-lightning-config") {
		t.Fatalf("status = %d body=%q", rec.Code, rec.Body.String())
	}
}
