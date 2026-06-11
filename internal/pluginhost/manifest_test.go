package pluginhost

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadManifestFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugin.json")
	data := []byte(`{"apiVersion":"glade.plugin.v1","name":"compat","version":"0.1.0","commands":[{"path":["compat"],"summary":"Compat commands."}]}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	manifest, err := LoadManifestFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Name != "compat" || manifest.CommandRoots()[0] != "compat" {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
}

func TestLoadManifestFromExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	exe := writeShellPlugin(t, t.TempDir(), "compat", `#!/bin/sh
if [ "$1" = "manifest" ] && [ "$2" = "--json" ]; then
  printf '{"apiVersion":"glade.plugin.v1","name":"compat","version":"0.1.0","commands":[{"path":["compat"],"summary":"Compat commands."}]}'
  exit 0
fi
exit 7
`)

	manifest, err := LoadManifestFromExecutable(context.Background(), exe)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Name != "compat" {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
}

func TestLinkExecutableStoresManifest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	root := t.TempDir()
	exe := writeShellPlugin(t, root, "compat", `#!/bin/sh
if [ "$1" = "manifest" ] && [ "$2" = "--json" ]; then
  printf '{"apiVersion":"glade.plugin.v1","name":"compat","version":"0.1.0","commands":[{"path":["compat"],"summary":"Compat commands."},{"path":["surface"],"summary":"Surface commands."}]}'
  exit 0
fi
echo "compat plugin"
`)

	store := NewStore(root)
	plugin, err := store.LinkExecutable(context.Background(), exe, "link:test")
	if err != nil {
		t.Fatal(err)
	}
	if plugin.Name != "compat" || !plugin.Linked || len(plugin.Commands) != 2 {
		t.Fatalf("unexpected linked plugin: %#v", plugin)
	}
	state, err := store.ReadInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Plugins) != 1 || state.Plugins[0].Commands[1] != "surface" {
		t.Fatalf("unexpected installed state: %#v", state)
	}
}
