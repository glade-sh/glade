package playground

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"golang.org/x/sys/unix"
)

// Observe actual filesystem opens during a warm run, rather than an internal
// helper call: each runtime source should be read once for both hashes.
func TestWarmRunnerReadsRuntimeSourcesOnce(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, 2)
	for _, name := range []string{"FirstProbe", "SecondProbe"} {
		path := filepath.Join(ws.Root, "force-app", "main", "default", "classes", name+".cls")
		if err := os.WriteFile(path, []byte("public class "+name+" {}"), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	runner := NewRunner(ws, RunnerOptions{Version: "test"})
	req := RunRequest{AnonymousBody: "System.debug('ok');", UseCache: true}
	if result, err := runner.Run(t.Context(), req); err != nil || result.Status != RunStatusPass {
		t.Fatalf("warmup: status=%s err=%v", result.Status, err)
	}
	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unix.Close(fd) })
	watches := make(map[int32]string)
	for _, path := range paths {
		wd, err := unix.InotifyAddWatch(fd, path, unix.IN_OPEN|unix.IN_CLOSE_NOWRITE)
		if err != nil {
			t.Fatal(err)
		}
		watches[int32(wd)] = path
	}
	if result, err := runner.Run(t.Context(), req); err != nil || result.Status != RunStatusPass {
		t.Fatalf("warm run: status=%s err=%v", result.Status, err)
	}
	counts := make(map[string]int)
	buf := make([]byte, 4096)
	for {
		n, err := unix.Read(fd, buf)
		if err == unix.EAGAIN {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		for offset := 0; offset < n; {
			if n-offset < unix.SizeofInotifyEvent {
				t.Fatal("short inotify event")
			}
			wd := int32(binary.NativeEndian.Uint32(buf[offset:]))
			mask := binary.NativeEndian.Uint32(buf[offset+4:])
			length := int(binary.NativeEndian.Uint32(buf[offset+12:]))
			if mask&unix.IN_Q_OVERFLOW != 0 {
				t.Fatal("inotify event queue overflow")
			}
			if mask&unix.IN_OPEN != 0 {
				counts[watches[wd]]++
			}
			offset += unix.SizeofInotifyEvent + length
		}
	}
	for _, path := range paths {
		if counts[path] != 1 {
			t.Errorf("opens for %s = %d, want 1", filepath.Base(path), counts[path])
		}
	}
}
