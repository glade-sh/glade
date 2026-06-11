package pluginhost

import (
	"context"
	"path/filepath"
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
