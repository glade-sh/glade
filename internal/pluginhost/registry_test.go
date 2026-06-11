package pluginhost

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRegistryURLDefaultsAndEnvOverride(t *testing.T) {
	t.Setenv("GLADE_PLUGIN_REGISTRY_URL", "")
	if got := RegistryURL(); got != DefaultRegistryURL {
		t.Fatalf("RegistryURL()=%q, want %q", got, DefaultRegistryURL)
	}
	t.Setenv("GLADE_PLUGIN_REGISTRY_URL", "http://example.test/index.json")
	if got := RegistryURL(); got != "http://example.test/index.json" {
		t.Fatalf("RegistryURL()=%q", got)
	}
}

func TestFetchRegistryAndAssetFor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"version":1,"plugins":[{"name":"compat","version":"0.1.0","assets":[{"os":%q,"arch":%q,"url":"http://asset","sha256":"%s"}]}]}`,
			runtime.GOOS, runtime.GOARCH, strings.Repeat("a", 64))
	}))
	defer server.Close()

	index, err := FetchRegistry(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	plugin, asset, ok := index.AssetFor("compat", runtime.GOOS, runtime.GOARCH)
	if !ok {
		t.Fatal("expected asset")
	}
	if plugin.Name != "compat" || asset.URL != "http://asset" {
		t.Fatalf("unexpected asset: %#v %#v", plugin, asset)
	}
}

func TestRegistryMissingPlugin(t *testing.T) {
	index := RegistryIndex{Version: 1}

	_, _, ok := index.AssetFor("missing", runtime.GOOS, runtime.GOARCH)
	if ok {
		t.Fatal("expected no asset")
	}
	err := index.NotFoundError("missing")
	if !strings.Contains(err.Error(), `plugin "missing" was not found in registry`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistryFindsScopedNameAliasAndSearchResults(t *testing.T) {
	index := RegistryIndex{Version: 1, Plugins: []RegistryPlugin{{
		Name:      "@glade/compat",
		Aliases:   []string{"compat"},
		Version:   "0.1.0",
		Publisher: "glade",
		Trust:     "first-party",
		Summary:   "Compatibility fixtures.",
		Commands:  []string{"compat", "surface"},
		DocsURL:   "https://glade.sh/guide/plugins/compat",
		Assets: []RegistryAsset{{
			OS: runtime.GOOS, Arch: runtime.GOARCH, URL: "https://example.test/compat.tar.gz", SHA256: strings.Repeat("a", 64),
		}},
	}}}
	ref, err := ParsePluginRef("compat")
	if err != nil {
		t.Fatal(err)
	}
	plugin, asset, ok := index.AssetForRef(ref, runtime.GOOS, runtime.GOARCH)
	if !ok {
		t.Fatal("expected compat alias to resolve")
	}
	if plugin.Name != "@glade/compat" || asset.URL == "" {
		t.Fatalf("unexpected plugin/asset: %#v %#v", plugin, asset)
	}
	results := index.Search("surface")
	if len(results) != 1 || results[0].Name != "@glade/compat" {
		t.Fatalf("unexpected search results: %#v", results)
	}
}

func TestRegistryUnsupportedPlatformErrorNamesAvailablePlatforms(t *testing.T) {
	index := RegistryIndex{Version: 1, Plugins: []RegistryPlugin{{
		Name: "@acme/quality", Version: "1.2.0",
		Assets: []RegistryAsset{{OS: "linux", Arch: "amd64", URL: "https://example.test/q.tar.gz", SHA256: strings.Repeat("b", 64)}},
	}}}
	ref, err := ParsePluginRef("@acme/quality")
	if err != nil {
		t.Fatal(err)
	}
	err = index.NotFoundErrorForRef(ref, runtime.GOOS, runtime.GOARCH)
	if err == nil || !strings.Contains(err.Error(), "available platforms: linux/amd64") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstallFromRegistryWritesInstalledJSON(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("archive executable mode is unix-specific")
	}
	root := t.TempDir()
	body := makePluginArchive(t, root, "compat", "0.1.0")
	sum := sha256.Sum256(body)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			fmt.Fprintf(w, `{"version":1,"plugins":[{"name":"compat","version":"0.1.0","assets":[{"os":%q,"arch":%q,"url":%q,"sha256":"%x"}]}]}`,
				runtime.GOOS, runtime.GOARCH, server.URL+"/compat.tar.gz", sum)
		case "/compat.tar.gz":
			w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("GLADE_PLUGIN_REGISTRY_URL", server.URL+"/index.json")
	store := NewStore(filepath.Join(root, "home"))

	plugin, err := store.InstallFromRegistry(context.Background(), "compat")
	if err != nil {
		t.Fatal(err)
	}
	if plugin.Name != "compat" || plugin.Source != "registry:compat" {
		t.Fatalf("unexpected installed plugin: %#v", plugin)
	}
	state, err := store.ReadInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Plugins) != 1 || state.Plugins[0].Version != "0.1.0" {
		t.Fatalf("unexpected installed state: %#v", state)
	}
}

func TestInstallFromRegistryScopedNameStoresCanonicalMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("archive executable mode is unix-specific")
	}
	root := t.TempDir()
	body := makePluginArchive(t, root, "compat", "0.1.0")
	sum := sha256.Sum256(body)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			fmt.Fprintf(w, `{"version":1,"plugins":[{"name":"@glade/compat","aliases":["compat"],"version":"0.1.0","publisher":"glade","trust":"first-party","docsURL":"https://glade.sh/guide/plugins/compat","assets":[{"os":%q,"arch":%q,"url":%q,"sha256":"%x"}]}]}`,
				runtime.GOOS, runtime.GOARCH, server.URL+"/compat.tar.gz", sum)
		case "/compat.tar.gz":
			w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("GLADE_PLUGIN_REGISTRY_URL", server.URL+"/index.json")
	store := NewStore(filepath.Join(root, "home"))

	plugin, err := store.InstallFromRegistry(context.Background(), "@glade/compat")
	if err != nil {
		t.Fatal(err)
	}
	if plugin.Name != "compat" || plugin.CanonicalName != "@glade/compat" || plugin.StorageName != "glade__compat" {
		t.Fatalf("unexpected plugin metadata: %#v", plugin)
	}
	if plugin.Registry != server.URL+"/index.json" || plugin.Trust != "first-party" || plugin.AssetSHA256 == "" {
		t.Fatalf("missing registry metadata: %#v", plugin)
	}
	if _, err := os.Stat(filepath.Join(root, "home", "plugins", "glade__compat", "0.1.0", "plugin.json")); err != nil {
		t.Fatalf("scoped storage missing: %v", err)
	}
}

func TestInstallFromRegistryRejectsCatalogManifestMismatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("archive executable mode is unix-specific")
	}
	root := t.TempDir()
	body := makePluginArchive(t, root, "wrong-name", "0.1.0")
	sum := sha256.Sum256(body)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			fmt.Fprintf(w, `{"version":1,"plugins":[{"name":"@acme/quality","version":"0.1.0","assets":[{"os":%q,"arch":%q,"url":%q,"sha256":"%x"}]}]}`,
				runtime.GOOS, runtime.GOARCH, server.URL+"/quality.tar.gz", sum)
		case "/quality.tar.gz":
			w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("GLADE_PLUGIN_REGISTRY_URL", server.URL+"/index.json")

	_, err := NewStore(filepath.Join(root, "home")).InstallFromRegistry(context.Background(), "@acme/quality")
	if err == nil || !strings.Contains(err.Error(), `manifest name "wrong-name" does not match catalog package "quality"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstallFromRegistryVersionUsesExactVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("archive executable mode is unix-specific")
	}
	root := t.TempDir()
	body100 := makePluginArchive(t, root, "compat", "1.0.0")
	body200 := makePluginArchive(t, root, "compat", "2.0.0")
	sum100 := sha256.Sum256(body100)
	sum200 := sha256.Sum256(body200)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			fmt.Fprintf(w, `{"version":1,"plugins":[`+
				`{"name":"compat","version":"2.0.0","assets":[{"os":%q,"arch":%q,"url":%q,"sha256":"%x"}]},`+
				`{"name":"compat","version":"1.0.0","assets":[{"os":%q,"arch":%q,"url":%q,"sha256":"%x"}]}`+
				`]}`,
				runtime.GOOS, runtime.GOARCH, server.URL+"/compat-2.tar.gz", sum200,
				runtime.GOOS, runtime.GOARCH, server.URL+"/compat-1.tar.gz", sum100)
		case "/compat-1.tar.gz":
			w.Write(body100)
		case "/compat-2.tar.gz":
			w.Write(body200)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("GLADE_PLUGIN_REGISTRY_URL", server.URL+"/index.json")

	plugin, err := NewStore(filepath.Join(root, "home")).InstallFromRegistryVersion(context.Background(), "compat", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if plugin.Version != "1.0.0" {
		t.Fatalf("installed version %q, want 1.0.0", plugin.Version)
	}
}

func TestInstallFromRegistryRejectsUnsafeRequestedNameBeforeFetch(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		http.Error(w, "should not fetch", http.StatusInternalServerError)
	}))
	defer server.Close()
	t.Setenv("GLADE_PLUGIN_REGISTRY_URL", server.URL)

	_, err := NewStore(t.TempDir()).InstallFromRegistry(context.Background(), "../compat")
	if err == nil {
		t.Fatal("expected unsafe requested plugin name to fail")
	}
	if called {
		t.Fatal("registry was fetched before requested name was validated")
	}
}

func TestInstallFromRegistryRejectsUnsafeRegistryNameBeforeDownload(t *testing.T) {
	root := t.TempDir()
	downloaded := false
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			fmt.Fprintf(w, `{"version":1,"plugins":[{"name":"compat","version":"../1.0.0","assets":[{"os":%q,"arch":%q,"url":%q,"sha256":"%s"}]}]}`,
				runtime.GOOS, runtime.GOARCH, server.URL+"/compat.tar.gz", strings.Repeat("0", 64))
		case "/compat.tar.gz":
			downloaded = true
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("GLADE_PLUGIN_REGISTRY_URL", server.URL+"/index.json")

	_, err := NewStore(filepath.Join(root, "home")).InstallFromRegistry(context.Background(), "compat")
	if err == nil {
		t.Fatal("expected unsafe registry plugin version to fail")
	}
	if downloaded {
		t.Fatal("registry asset was downloaded before registry metadata was validated")
	}
	if _, statErr := os.Stat(filepath.Join(root, "home", "plugins", "compat.tar.gz")); !os.IsNotExist(statErr) {
		t.Fatalf("unexpected file write from unsafe registry metadata: %v", statErr)
	}
}

func TestInstallFromRegistryRejectsChecksumMismatch(t *testing.T) {
	root := t.TempDir()
	body := makePluginArchive(t, root, "compat", "0.1.0")
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			fmt.Fprintf(w, `{"version":1,"plugins":[{"name":"compat","version":"0.1.0","assets":[{"os":%q,"arch":%q,"url":%q,"sha256":"%s"}]}]}`,
				runtime.GOOS, runtime.GOARCH, server.URL+"/compat.tar.gz", strings.Repeat("0", 64))
		case "/compat.tar.gz":
			w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("GLADE_PLUGIN_REGISTRY_URL", server.URL+"/index.json")

	_, err := NewStore(filepath.Join(root, "home")).InstallFromRegistry(context.Background(), "compat")
	if err == nil {
		t.Fatal("expected checksum failure")
	}
	if !strings.Contains(err.Error(), "registry asset checksum mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func makePluginArchive(t *testing.T, root, name, version string) []byte {
	t.Helper()
	path := filepath.Join(root, name+"-"+version+".tar.gz")
	manifest := testManifestJSON(name, version)
	exe := []byte("#!/bin/sh\nexit 0\n")
	writeTestArchive(t, path, map[string]archiveFile{
		"plugin.json":              {Data: manifest, Mode: 0o644},
		"checksums.txt":            {Data: testChecksums(map[string][]byte{"plugin.json": manifest, "bin/glade-plugin-" + name: exe}), Mode: 0o644},
		"bin/glade-plugin-" + name: {Data: exe, Mode: 0o755},
	})
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
