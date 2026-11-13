//go:build unix

package gladecli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDoctorReportsBusyToolchain(t *testing.T) {
	useDoctorTestToolchain(t)
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeTestFile(t, filepath.Join(root, "glade.yml"), "project:\n  root: .\n")
	lockPath := filepath.Join(os.Getenv("GLADE_HOME"), ".install.lock")
	ready := filepath.Join(t.TempDir(), "ready")
	holder := exec.Command("python3", "-c", `import fcntl,sys,time
f=open(sys.argv[1], 'w')
fcntl.lockf(f, fcntl.LOCK_EX)
open(sys.argv[2], 'w').close()
time.sleep(120)`, lockPath, ready)
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Process.Kill(); _ = holder.Wait() }()
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("lock holder did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	done := make(chan string, 1)
	go func() {
        for _, format := range []string{"text", "json"} {
            var out bytes.Buffer
            args := []string{"--project", root}
            if format == "json" { args = append(args, "--json") }
            _, err := runDoctor(context.Background(), args, &out)
            if err != nil { done <- err.Error(); return }
            output := strings.ToLower(out.String())
            if !strings.Contains(output, "busy") || !strings.Contains(output, "retry") || strings.Contains(output, "reinstall") {
                done <- format + ": " + out.String(); return
            }
        }
        done <- "ok"
	}()
	select {
	case out := <-done:
        if out != "ok" { t.Fatalf("doctor busy guidance: %s", out) }
	case <-time.After(20 * time.Second):
		t.Fatal("doctor blocked on the toolchain install lock")
	}
}
