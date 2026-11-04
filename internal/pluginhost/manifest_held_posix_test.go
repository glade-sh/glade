//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package pluginhost

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLoadManifestFromExecutableCancellationHeldDescendant(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), "child.pid")
	holdPath := filepath.Join(t.TempDir(), "hold")
	t.Setenv("GLADE_PLUGIN_TEST_PID_FILE", pidPath)
	t.Setenv("GLADE_PLUGIN_TEST_HOLD", holdPath)
	exe := writeShellPlugin(t, t.TempDir(), "compat", `#!/bin/sh
if [ "$1" = "prewarm" ]; then exit 0; fi
mkfifo "$GLADE_PLUGIN_TEST_HOLD" || exit 1
cat "$GLADE_PLUGIN_TEST_HOLD" &
child=$!
printf '%s\n' "$child" > "$GLADE_PLUGIN_TEST_PID_FILE"
wait "$child"
`)
	if err := exec.Command(exe, "prewarm").Run(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	exited := make(chan struct{})
	cleanupNeeded := true
	t.Cleanup(func() {
		cancel()
		if cleanupNeeded {
			data, err := os.ReadFile(pidPath)
			if err == nil {
				pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
				if parseErr != nil || pid <= 0 {
					t.Errorf("invalid child PID: %q", data)
				} else if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
					t.Errorf("kill child: %v", err)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Errorf("read child PID: %v", err)
			}
		}
		select {
		case <-exited:
		case <-time.After(30 * time.Second):
			t.Error("manifest loader did not exit")
		}
	})
	go func() { defer close(exited); _, err := LoadManifestFromExecutable(ctx, exe); finished <- err }()
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	var pid int
	for pid == 0 {
		data, err := os.ReadFile(pidPath)
		if err == nil && len(strings.TrimSpace(string(data))) > 0 {
			pid, err = strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil || pid <= 0 {
				t.Fatalf("invalid child PID: %q", data)
			}
			break
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		select {
		case err := <-finished:
			t.Fatalf("loader returned before readiness: %v", err)
		case <-deadline.C:
			t.Fatal("child PID not recorded")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want canceled", err)
		}
	case <-deadline.C:
		t.Fatal("cancellation did not return while descendant was held")
	}
	for {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			cleanupNeeded = false
			return
		}
		if err != nil && !errors.Is(err, syscall.EPERM) {
			t.Fatalf("probe child: %v", err)
		}
		select {
		case <-deadline.C:
			t.Fatalf("child %d still exists: %v", pid, err)
		case <-time.After(10 * time.Millisecond):
		}
	}
}
