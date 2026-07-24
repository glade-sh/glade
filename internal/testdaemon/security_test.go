package testdaemon

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestServePathsPutLongProjectFallbackInPrivateDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), strings.Repeat("long-project-segment-", 8))

	socket, pid := servePaths(root)
	if filepath.Dir(socket) != filepath.Dir(pid) {
		t.Fatalf("fallback parents differ: socket=%q pid=%q", socket, pid)
	}
	if filepath.Dir(socket) == os.TempDir() {
		t.Fatalf("fallback files are directly in shared temp directory: socket=%q pid=%q", socket, pid)
	}
	if got, want := filepath.Base(socket), "serve.sock"; got != want {
		t.Fatalf("socket basename = %q, want %q", got, want)
	}
	if got, want := filepath.Base(pid), "serve.pid"; got != want {
		t.Fatalf("PID basename = %q, want %q", got, want)
	}
}

func TestNewServerRestrictsExistingDefaultDaemonDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix daemon permissions")
	}
	root := t.TempDir()
	writeTestProject(t, root)
	daemonDir := filepath.Dir(ServePIDPath(root))
	if err := os.MkdirAll(daemonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(daemonDir, 0o755); err != nil {
		t.Fatal(err)
	}

	server, err := NewServer(ServerConfig{Root: root, Warm: false, Watch: false})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	info, err := os.Stat(daemonDir)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o700); got != want {
		t.Fatalf("daemon directory mode = %04o, want %04o", got, want)
	}
}

func TestEnsurePrivateDaemonDirRejectsSymlinkedParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix daemon permissions and symlinks")
	}
	root := t.TempDir()
	target := t.TempDir()
	targetTestDir := filepath.Join(target, "test")
	if err := os.MkdirAll(targetTestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(targetTestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, ".glade")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	opened, err := openPrivateDaemonDir(root, filepath.FromSlash(serveDir))
	if err == nil {
		opened.Close()
		t.Fatal("openPrivateDaemonDir accepted a symlinked parent")
	}
	info, err := os.Stat(targetTestDir)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o755); got != want {
		t.Fatalf("symlink target mode = %04o, want unchanged %04o", got, want)
	}
}

func TestWritePIDPublishesPrivateFileAndRefusesSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix daemon permissions and symlinks")
	}
	t.Run("private regular file", func(t *testing.T) {
		base := t.TempDir()
		root, err := openPrivateDaemonDir(base, "daemon")
		if err != nil {
			t.Fatal(err)
		}
		defer root.Close()
		path := filepath.Join(base, "daemon", "serve.pid")
		if err := writePID(root, "serve.pid", path); err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !info.Mode().IsRegular() {
			t.Fatalf("PID mode = %v, want regular file", info.Mode())
		}
		if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
			t.Fatalf("PID mode = %04o, want %04o", got, want)
		}
	})

	t.Run("symlink", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		const sentinel = "do not replace\n"
		if err := os.WriteFile(target, []byte(sentinel), 0o600); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "serve.pid")
		if err := os.Symlink(target, path); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		root, err := os.OpenRoot(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer root.Close()
		if err := writePID(root, "serve.pid", path); err == nil {
			t.Fatal("writePID accepted a symlink")
		}
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(data); got != sentinel {
			t.Fatalf("symlink target = %q, want %q", got, sentinel)
		}
	})
}

func TestListenAndServeRestrictsSocketBeforeServingAndPublishesPrivatePID(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix daemon permissions")
	}
	root := t.TempDir()
	writeTestProject(t, root)
	socket := ServeSocketPath(root)
	server, err := NewServer(ServerConfig{
		Root:  root,
		Warm:  false,
		Watch: false,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe(ctx, io.Discard)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-errCh:
			if err != nil {
				t.Errorf("ListenAndServe exit: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("ListenAndServe did not stop")
		}
	})

	pidPath := ServePIDPath(root)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Lstat(pidPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("PID file was not published at %q", pidPath)
		}
		time.Sleep(10 * time.Millisecond)
	}
	for path, wantType := range map[string]os.FileMode{
		socket:  os.ModeSocket,
		pidPath: 0,
	} {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if wantType != 0 && info.Mode()&wantType == 0 {
			t.Fatalf("%s mode = %v, want type %v", path, info.Mode(), wantType)
		}
		if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
			t.Fatalf("%s permissions = %04o, want %04o", path, got, want)
		}
	}
}
