package pluginhost

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDoctorReportsOK(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	root := t.TempDir()
	exe := writeShellPlugin(t, root, "compat", `#!/bin/sh
if [ "$1" = "manifest" ] && [ "$2" = "--json" ]; then
  printf '{"apiVersion":"glade.plugin.v1","name":"compat","version":"0.1.0","commands":[{"path":["compat"]}]}'
  exit 0
fi
exit 0
`)
	store := NewStore(root)
	if err := store.WriteInstalled(InstalledState{Plugins: []InstalledPlugin{
		{Name: "compat", Version: "0.1.0", Executable: exe, Commands: []string{"compat"}},
	}}); err != nil {
		t.Fatal(err)
	}

	results, err := store.Doctor(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !results[0].OK || results[0].Message != "ok" {
		t.Fatalf("unexpected doctor results: %#v", results)
	}
}

func TestDoctorReportsMissingExecutable(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.WriteInstalled(InstalledState{Plugins: []InstalledPlugin{
		{Name: "compat", Version: "0.1.0", Executable: filepath.Join(t.TempDir(), "missing"), Commands: []string{"compat"}},
	}}); err != nil {
		t.Fatal(err)
	}

	results, err := store.Doctor(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].OK {
		t.Fatalf("expected missing executable result: %#v", results)
	}
	if !strings.Contains(results[0].Message, "missing executable") {
		t.Fatalf("unexpected result: %#v", results[0])
	}
}

func TestDoctorReportsMissingInstalledCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	root := t.TempDir()
	exe := writeShellPlugin(t, root, "compat", `#!/bin/sh
if [ "$1" = "manifest" ] && [ "$2" = "--json" ]; then
  printf '{"apiVersion":"glade.plugin.v1","name":"compat","version":"0.1.0","commands":[{"path":["surface"]}]}'
  exit 0
fi
exit 0
`)
	store := NewStore(root)
	if err := store.WriteInstalled(InstalledState{Plugins: []InstalledPlugin{
		{Name: "compat", Version: "0.1.0", Executable: exe, Commands: []string{"compat"}},
	}}); err != nil {
		t.Fatal(err)
	}

	results, err := store.Doctor(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].OK {
		t.Fatalf("expected command drift result: %#v", results)
	}
	if !strings.Contains(results[0].Message, "installed commands no longer appear") {
		t.Fatalf("unexpected result: %#v", results[0])
	}
}

func writeShellPlugin(t *testing.T, dir, name, script string) string {
	t.Helper()
	exe := filepath.Join(dir, "glade-plugin-"+name)
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe
}
