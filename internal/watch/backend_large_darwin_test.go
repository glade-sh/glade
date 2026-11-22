//go:build darwin

package watch

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func largeWatcherProject(tb testing.TB) (string, Snapshot) {
	tb.Helper()
	root := tb.TempDir()
	for i := 0; i < 1500; i++ {
		for _, ext := range []string{".cls", ".cls-meta.xml"} {
			path := filepath.Join(root, fmt.Sprintf("Class%04d%s", i, ext))
			if err := os.WriteFile(path, []byte("initial content\n"), 0600); err != nil {
				tb.Fatal(err)
			}
		}
	}
	initial, err := CaptureScope(NormalizeScope(Scope{Roots: []string{root}}))
	if err != nil {
		tb.Fatal(err)
	}
	return root, initial
}

func TestWatcherLargeProjectLifecycle(t *testing.T) {
	root, initial := largeWatcherProject(t)
	w, selected, err := NewBackendWatcher(context.Background(), Config{Root: root, Backend: BackendNative, Debounce: 25 * time.Millisecond}, initial)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if selected != BackendNative {
		t.Fatalf("backend = %s", selected)
	}
	path := filepath.Join(root, "Class0750.cls")
	replaceWatchFile(t, path, "edited content")
	awaitScopedChange(t, w, path, ChangeModified)
	renamed := filepath.Join(root, "Renamed.cls")
	if err := os.Rename(path, renamed); err != nil {
		t.Fatal(err)
	}
	awaitScopedChanges(t, w, map[string]ChangeOp{path: ChangeDeleted, renamed: ChangeAdded})
	if err := os.Remove(renamed); err != nil {
		t.Fatal(err)
	}
	awaitScopedChange(t, w, renamed, ChangeDeleted)
}

func BenchmarkWatcherLargeProjectIdle(b *testing.B) {
	root, initial := largeWatcherProject(b)
	w, _, err := NewBackendWatcher(context.Background(), Config{Root: root, Backend: BackendNative, Debounce: 200 * time.Millisecond}, initial)
	if err != nil {
		b.Fatal(err)
	}
	defer w.Close()
	output, err := exec.Command("lsof", "-Fn", "-p", fmt.Sprint(os.Getpid())).Output()
	if err != nil {
		b.Fatal(err)
	}
	fds := 0
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "f") {
			if _, err := strconv.Atoi(line[1:]); err == nil {
				fds++
			}
		}
	}
	var before, after syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &before); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		time.Sleep(12 * time.Second)
	}
	b.StopTimer()
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &after); err != nil {
		b.Fatal(err)
	}
	cpu := func(r syscall.Rusage) int64 {
		return r.Utime.Sec*1000000 + int64(r.Utime.Usec) + r.Stime.Sec*1000000 + int64(r.Stime.Usec)
	}
	b.ReportMetric(float64(cpu(after)-cpu(before))/float64(b.N), "cpu-us/op")
	b.ReportMetric(float64(fds), "open-fds")
}
