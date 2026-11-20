package gladecli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// A FIFO makes an otherwise invisible status read observable without relying
// on timing or allocation budgets. --no-cache disables execution cache reads.
func TestRedirectedTestSkipsStartupCacheStatus(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/ExampleTest.cls"), `@IsTest private class ExampleTest { @IsTest static void passes() { System.assert(true); } }`)
	cacheDir := filepath.Join(root, ".glade", "test")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(cacheDir, "startup.meta.json")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan int, 1)
	var stdout, stderr bytes.Buffer
	go func() {
		done <- Run(context.Background(), []string{"test", "--project", root, "--filter", "ExampleTest", "--no-serve", "--no-cache"}, &stdout, &stderr)
	}()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case code := <-done:
			if code != 0 || stderr.Len() != 0 {
				t.Fatalf("exit=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
			}
			return
		case <-ticker.C:
			fd, err := syscall.Open(fifo, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
			if err == nil {
				syscall.Close(fd)
				<-done
				t.Fatal("redirected progress read the startup cache for a discarded warning")
			}
			if err != syscall.ENXIO {
				t.Fatalf("open FIFO: %v", err)
			}
		case <-deadline.C:
			t.Fatal("test command did not finish")
		}
	}
}
