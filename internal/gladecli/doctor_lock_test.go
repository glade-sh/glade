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
 lockPath := filepath.Join(os.Getenv("GLADE_HOME"), ".install.lock")
 ready := filepath.Join(t.TempDir(), "ready")
 holder := exec.Command("python3", "-c", `import fcntl,sys,time
f=open(sys.argv[1], 'w')
fcntl.lockf(f, fcntl.LOCK_EX)
open(sys.argv[2], 'w').close()
time.sleep(30)`, lockPath, ready)
 if err := holder.Start(); err != nil { t.Fatal(err) }
 defer func(){ _ = holder.Process.Kill(); _ = holder.Wait() }()
 deadline := time.Now().Add(5*time.Second)
 for { if _, err := os.Stat(ready); err == nil { break }; if time.Now().After(deadline) { t.Fatal("lock holder did not start") }; time.Sleep(10*time.Millisecond) }
 done := make(chan string, 1)
 go func(){ var out bytes.Buffer; _, err := runDoctor(context.Background(), []string{"--json"}, &out); if err != nil { done <- err.Error(); return }; done <- out.String() }()
 select {
 case out := <-done:
  if !strings.Contains(strings.ToLower(out), "busy") { t.Fatalf("doctor did not report busy toolchain: %s", out) }
 case <-time.After(3*time.Second):
  t.Fatal("doctor blocked on the toolchain install lock")
 }
}
