//go:build linux || darwin

package testdaemon

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCase340RejectsDifferentUIDPeer(t *testing.T) {
	if err := exec.Command("sudo", "-n", "-u", "nobody", "true").Run(); err != nil {
		t.Skip("noninteractive different-UID helper unavailable")
	}
	base := os.TempDir()
	if exec.Command("sudo", "-n", "-u", "nobody", "test", "-x", base).Run() != nil {
		// Stay on the same filesystem as the runner when looking for a
		// directory accessible to the unprivileged helper.
		current, err1 := os.Stat(base)
		alternate, err2 := os.Stat("/var/tmp")
		if err1 != nil || err2 != nil {
			t.Skip("no accessible temporary directory")
		}
		a, ok1 := current.Sys().(*syscall.Stat_t)
		b, ok2 := alternate.Sys().(*syscall.Stat_t)
		if !ok1 || !ok2 || a.Dev != b.Dev {
			t.Skip("no accessible temporary directory on the runner filesystem")
		}
		base = "/var/tmp"
	}
	var space syscall.Statfs_t
	if err := syscall.Statfs(base, &space); err != nil || space.Bavail*uint64(space.Bsize) < 50*1024*1024*1024 {
		t.Skip("temporary filesystem has less than 50 GiB available")
	}
	parent, err := os.MkdirTemp(base, "case340-peer-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(parent)
	if err := os.Chmod(parent, 0777); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(parent, "daemon")
	socket := filepath.Join(dir, "serve.sock")
	const helper = `import os, socket, sys
os.umask(0)
os.mkdir(sys.argv[1], 0o775)
s = socket.socket(socket.AF_UNIX)
s.bind(sys.argv[2])
os.chmod(sys.argv[2], 0o777)
s.listen(1)
s.settimeout(10)
print('ready', flush=True)
c, _ = s.accept()
c.close()
s.close()`
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sudo", "-n", "-u", "nobody", "python3", "-c", helper, dir, socket)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := false
	defer func() {
		if !done {
			cancel()
			_ = cmd.Wait()
		}
	}()
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "ready" {
		t.Fatalf("helper did not start: %v", scanner.Err())
	}
	_, err = PingV1(ctx, socket)
	if err == nil || !strings.Contains(err.Error(), "unexpected peer owner") {
		t.Fatalf("different-UID peer error = %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("helper: %v", err)
	}
	done = true
}
