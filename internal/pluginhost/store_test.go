package pluginhost

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultRootUsesGladeHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GLADE_HOME", home)

	if got := DefaultRoot(); got != home {
		t.Fatalf("DefaultRoot()=%q, want %q", got, home)
	}
}

func TestStoreReadWriteInstalledState(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	state := InstalledState{
		Version: 1,
		Plugins: []InstalledPlugin{{
			Name:       "compat",
			Version:    "0.1.0",
			Executable: filepath.Join(root, "plugins", "compat", "0.1.0", "bin", "glade-plugin-compat"),
			Manifest:   filepath.Join(root, "plugins", "compat", "0.1.0", "plugin.json"),
			Source:     "link:/tmp/glade-tools",
			Linked:     true,
			Commands:   []string{"compat", "surface"},
		}},
	}

	if err := store.WriteInstalled(state); err != nil {
		t.Fatal(err)
	}
	got, err := store.ReadInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Plugins) != 1 || got.Plugins[0].Name != "compat" || !got.Plugins[0].Linked {
		t.Fatalf("unexpected installed state: %#v", got)
	}
}

func TestStoreMissingInstalledStateIsEmpty(t *testing.T) {
	store := NewStore(t.TempDir())

	got, err := store.ReadInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 || len(got.Plugins) != 0 {
		t.Fatalf("unexpected empty state: %#v", got)
	}
}

func TestStoreListInstalled(t *testing.T) {
	store := NewStore(t.TempDir())
	want := InstalledPlugin{Name: "compat", Version: "0.1.0", Commands: []string{"compat"}}
	if err := store.WriteInstalled(InstalledState{Plugins: []InstalledPlugin{want}}); err != nil {
		t.Fatal(err)
	}

	got, err := store.ListInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != want.Name {
		t.Fatalf("unexpected plugin list: %#v", got)
	}
}

func TestReplaceInstalledReplacesLegacyFirstPartyEntry(t *testing.T) {
	legacy := InstalledPlugin{Name: "compat", Version: "0.0.1", Commands: []string{"compat"}}
	next := InstalledPlugin{
		Name:          "compat",
		CanonicalName: "@glade/compat",
		StorageName:   "glade__compat",
		Version:       "0.1.0",
		Commands:      []string{"compat"},
	}

	got := replaceInstalled([]InstalledPlugin{legacy}, next)
	if len(got) != 1 {
		t.Fatalf("expected one installed plugin, got %#v", got)
	}
	if got[0].CanonicalName != "@glade/compat" || got[0].Version != "0.1.0" {
		t.Fatalf("legacy entry was not replaced: %#v", got)
	}
}

func TestStoreRemoveInstalledAndKeepsLinkedExecutable(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "linked-plugin")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	if err := store.WriteInstalled(InstalledState{Plugins: []InstalledPlugin{
		{Name: "compat", Version: "0.1.0", Executable: exe, Linked: true},
	}}); err != nil {
		t.Fatal(err)
	}

	if err := store.Remove("compat"); err != nil {
		t.Fatal(err)
	}
	state, err := store.ReadInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Plugins) != 0 {
		t.Fatalf("expected no installed plugins, got %#v", state.Plugins)
	}
	if _, err := os.Stat(exe); err != nil {
		t.Fatalf("linked executable was removed: %v", err)
	}
}

func TestStoreRemoveInstalledDeletesUnlinkedPluginDirectory(t *testing.T) {
	root := t.TempDir()
	pluginDir := filepath.Join(root, "plugins", "compat")
	exe := filepath.Join(pluginDir, "0.1.0", "bin", "glade-plugin-compat")
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	if err := store.WriteInstalled(InstalledState{Plugins: []InstalledPlugin{
		{Name: "compat", Version: "0.1.0", Executable: exe, Linked: false},
	}}); err != nil {
		t.Fatal(err)
	}

	if err := store.Remove("compat"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(pluginDir); !os.IsNotExist(err) {
		t.Fatalf("expected plugin directory removed, stat err=%v", err)
	}
}

func TestStoreRemoveRejectsUnsafeName(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if err := store.WriteInstalled(InstalledState{Plugins: []InstalledPlugin{
		{Name: "../../owned", Version: "0.1.0", Linked: false},
	}}); err != nil {
		t.Fatal(err)
	}

	err := store.Remove("../../owned")
	if err == nil {
		t.Fatal("expected unsafe remove name to fail")
	}
	if _, statErr := os.Stat(filepath.Join(root, "..", "owned")); !os.IsNotExist(statErr) {
		t.Fatalf("unsafe remove path was touched, stat err=%v", statErr)
	}
}
