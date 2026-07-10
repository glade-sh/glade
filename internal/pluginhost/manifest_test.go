package pluginhost

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
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

func TestLoadManifestFromExecutableTimeoutCleansUpDescendant(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper and process probe use POSIX commands")
	}
	pidPath := filepath.Join(t.TempDir(), "descendant.pid")
	cleanupNeeded := true
	t.Cleanup(func() {
		if cleanupNeeded {
			bestEffortKillRecordedPID(pidPath)
		}
	})
	t.Setenv("GLADE_PLUGIN_TEST_PID_FILE", pidPath)
	exe := writeShellPlugin(t, t.TempDir(), "compat", `#!/bin/sh
if [ "$1" = "prewarm" ]; then
	exit 0
fi
sleep 30 &
child=$!
printf '%s\n' "$child" > "$GLADE_PLUGIN_TEST_PID_FILE"
wait "$child"
`)
	if err := exec.Command(exe, "prewarm").Run(); err != nil {
		t.Fatalf("prewarm helper: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := LoadManifestFromExecutable(ctx, exe)
	elapsed := time.Since(started)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context deadline exceeded", err)
	}
	if elapsed >= time.Second {
		t.Fatalf("timeout cleanup took %s, want <1s", elapsed)
	}

	pid := readRecordedPID(t, pidPath)
	waitForProcessAbsent(t, pid, time.Second)
	cleanupNeeded = false
}

func TestLoadManifestFromExecutablePreservesOrdinaryFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	exe := writeShellPlugin(t, t.TempDir(), "compat", `#!/bin/sh
printf 'manifest failure sentinel\n' >&2
exit 7
`)

	_, err := LoadManifestFromExecutable(context.Background(), exe)
	if err == nil {
		t.Fatal("LoadManifestFromExecutable unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "exit status 7") || !strings.Contains(err.Error(), "manifest failure sentinel") {
		t.Fatalf("error did not preserve exit status and stderr: %v", err)
	}
}

func TestConfigureManifestCommandWithoutStartedProcessReportsProcessDone(t *testing.T) {
	cmd := exec.CommandContext(context.Background(), "unused")
	configureManifestCommand(cmd)

	err := cmd.Cancel()
	if !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("Cancel error = %v, want os.ErrProcessDone", err)
	}
}

func TestConfigureManifestCommandAfterProcessExitReportsProcessDone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	cmd := exec.CommandContext(context.Background(), "sh", "-c", "exit 0")
	configureManifestCommand(cmd)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	err := cmd.Cancel()
	if !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("Cancel error = %v, want os.ErrProcessDone", err)
	}
}

func readRecordedPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			pid, convErr := strconv.Atoi(strings.TrimSpace(string(data)))
			if convErr != nil || pid <= 0 {
				t.Fatalf("invalid recorded PID %q: %v", data, convErr)
			}
			return pid
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("read recorded PID: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for PID file %s", path)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func waitForProcessAbsent(t *testing.T, pid int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		err := exec.Command("kill", "-0", strconv.Itoa(pid)).Run()
		if err != nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("descendant PID %d remained after %s", pid, timeout)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func bestEffortKillRecordedPID(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return
	}
	_ = exec.Command("kill", "-KILL", strconv.Itoa(pid)).Run()
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
