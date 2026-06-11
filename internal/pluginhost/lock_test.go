package pluginhost

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteLockFileSkipsLinkedPluginsByDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), LockFileName)
	state := InstalledState{Plugins: []InstalledPlugin{
		{Name: "compat", Version: "0.1.0", Source: "registry:compat", Commands: []string{"compat"}, Linked: false},
		{Name: "local", Version: "0.1.0", Source: "link:/tmp/local", Commands: []string{"local"}, Linked: true},
	}}

	if err := WriteLockFile(path, state, false); err != nil {
		t.Fatal(err)
	}
	lock, err := ReadLockFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Plugins) != 1 || lock.Plugins[0].Name != "compat" {
		t.Fatalf("unexpected lock file: %#v", lock)
	}
}

func TestWriteLockFileCanIncludeLinkedPlugins(t *testing.T) {
	path := filepath.Join(t.TempDir(), LockFileName)
	state := InstalledState{Plugins: []InstalledPlugin{
		{Name: "compat", Version: "0.1.0", Source: "registry:compat", Commands: []string{"compat"}, Linked: false},
		{Name: "local", Version: "0.1.0", Source: "link:/tmp/local", Commands: []string{"local"}, Linked: true},
	}}

	if err := WriteLockFile(path, state, true); err != nil {
		t.Fatal(err)
	}
	lock, err := ReadLockFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Plugins) != 2 {
		t.Fatalf("unexpected lock file: %#v", lock)
	}
}

func TestRestoreLockFileInstallsExactVersions(t *testing.T) {
	store := NewStore(t.TempDir())
	lock := PluginLock{Version: 1, Plugins: []LockedPlugin{
		{Name: "compat", Version: "1.0.0", Source: "registry:compat", Commands: []string{"compat"}},
		{Name: "performance", Version: "2.0.0", Source: "registry:performance", Commands: []string{"performance"}},
	}}
	var calls []string

	err := store.RestoreLock(context.Background(), lock, func(ctx context.Context, name, version string) (InstalledPlugin, error) {
		calls = append(calls, name+"@"+version)
		return InstalledPlugin{Name: name, Version: version}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0] != "compat@1.0.0" || calls[1] != "performance@2.0.0" {
		t.Fatalf("unexpected restore calls: %#v", calls)
	}
}

func TestWriteLockFileUsesCanonicalRegistryIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "glade.plugins.lock.json")
	state := InstalledState{Plugins: []InstalledPlugin{{
		Name:          "compat",
		CanonicalName: "@glade/compat",
		Version:       "0.1.0",
		Registry:      "https://plugins.glade.sh/index.json",
		Trust:         "first-party",
		Publisher:     "glade",
		AssetOS:       "darwin",
		AssetArch:     "arm64",
		AssetSHA256:   strings.Repeat("a", 64),
		Source:        "registry:@glade/compat",
		Commands:      []string{"compat"},
	}}}
	if err := WriteLockFile(path, state, false); err != nil {
		t.Fatal(err)
	}
	lock, err := ReadLockFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := lock.Plugins[0]
	if got.Name != "@glade/compat" || got.Registry == "" || got.SHA256 == "" || got.OS != "darwin" || got.Arch != "arm64" {
		t.Fatalf("unexpected lock plugin: %#v", got)
	}
}

func TestRestoreLockInstallsExactCanonicalVersions(t *testing.T) {
	lock := PluginLock{Version: 1, Plugins: []LockedPlugin{{
		Name: "@acme/quality", Version: "1.2.0", Registry: "https://plugins.example.test/index.json", SHA256: strings.Repeat("b", 64),
	}}}
	var gotName, gotVersion string
	err := NewStore(t.TempDir()).RestoreLock(context.Background(), lock, func(ctx context.Context, name, version string) (InstalledPlugin, error) {
		gotName, gotVersion = name, version
		return InstalledPlugin{Name: "quality", CanonicalName: name, Version: version, AssetSHA256: strings.Repeat("b", 64)}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotName != "@acme/quality" || gotVersion != "1.2.0" {
		t.Fatalf("restore installed %q %q", gotName, gotVersion)
	}
}

func TestRestoreLockVerifiesLockedSHA256(t *testing.T) {
	lock := PluginLock{Version: 1, Plugins: []LockedPlugin{{
		Name: "@acme/quality", Version: "1.2.0", SHA256: strings.Repeat("b", 64),
	}}}
	err := NewStore(t.TempDir()).RestoreLock(context.Background(), lock, func(ctx context.Context, name, version string) (InstalledPlugin, error) {
		return InstalledPlugin{Name: "quality", CanonicalName: name, Version: version, AssetSHA256: strings.Repeat("a", 64)}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRestoreLockRestoresURLSource(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("archive executable mode is unix-specific")
	}
	root := t.TempDir()
	body := makePluginArchive(t, root, "quality", "1.2.0")
	sum := sha256.Sum256(body)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	defer server.Close()
	lock := PluginLock{Version: 1, Plugins: []LockedPlugin{{
		Name: "quality", Version: "1.2.0", Source: "url:" + server.URL + "/quality.tar.gz", SHA256: fmt.Sprintf("%x", sum), Trust: "unlisted",
	}}}
	store := NewStore(filepath.Join(root, "home"))
	if err := store.RestoreLock(context.Background(), lock, nil); err != nil {
		t.Fatal(err)
	}
	state, err := store.ReadInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Plugins) != 1 || state.Plugins[0].Source != "url:"+server.URL+"/quality.tar.gz" {
		t.Fatalf("unexpected restored state: %#v", state)
	}
}
