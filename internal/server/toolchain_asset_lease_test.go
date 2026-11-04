//go:build unix

package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestManagedShimReadHoldsLease(t *testing.T)    { managedAssetLease(t, "shim") }
func TestManagedRuntimeReadHoldsLease(t *testing.T) { managedAssetLease(t, "runtime") }
func TestManagedVendorReadHoldsLease(t *testing.T)  { managedAssetLease(t, "vendor") }
func TestExternalShimReadHoldsLease(t *testing.T)   { managedAssetLease(t, "external-shim") }

func managedAssetLease(t *testing.T, route string) {
	t.Helper()
	if os.Getenv("GLADE_TEST_SHIM_CHILD") == "1" {
		var content []byte
		var err error
		switch route {
		case "shim", "external-shim":
			content, err = readLightningStaticShim("case342-lease-test.mjs")
		case "runtime":
			_, content, _ = lightningRuntimeAsset("shell", "case342-lease-test.mjs")
		case "vendor":
			response := httptest.NewRecorder()
			(&Server{}).handleLightningVendor(response, httptest.NewRequest(http.MethodGet, "/", nil), []string{"engine-dom.js"})
			content = response.Body.Bytes()
		}
		if err != nil || string(content) != "fixture" {
			t.Fatalf("asset content %q: %v", content, err)
		}
		return
	}
	root := t.TempDir()
	lockDir := filepath.Join(root, "glade")
	share := lockDir
	if route == "external-shim" {
		share = filepath.Join(root, "external")
		lockDir = share
	}
	for _, rel := range []string{"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs", ".install.lock"} {
		name := filepath.Join(share, rel)
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lockDir, ".install.lock"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	relative := "lwcruntime/src/shims/case342-lease-test.mjs"
	if route == "runtime" {
		relative = "lwcruntime/src/shell/case342-lease-test.mjs"
	}
	if route == "vendor" {
		relative = "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js"
	}
	fifo := filepath.Join(share, relative)
	if err := os.MkdirAll(filepath.Dir(fifo), 0755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", root)
	t.Setenv("GLADE_HOME", "")
	if route == "external-shim" {
		t.Setenv("GLADE_HOME", share)
	}
	t.Setenv("GLADE_ROOT", "")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	reader := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+t.Name()+"$")
	reader.Env = append(os.Environ(), "GLADE_TEST_SHIM_CHILD=1")
	// Run outside the source checkout so asset lookup uses the private share root, not a
	// checkout with linked node_modules (as remote test hosts have).
	reader.Dir = root
	if err := reader.Start(); err != nil {
		t.Fatal(err)
	}
	ready, release := filepath.Join(root, "ready"), filepath.Join(root, "release")
	program := `import errno,os,sys,time
while True:
 try:
  fd=os.open(sys.argv[1],os.O_WRONLY|os.O_NONBLOCK); break
 except OSError as e:
  if e.errno != errno.ENXIO: raise
  time.sleep(.05)
open(sys.argv[2],'w').close()
while not os.path.exists(sys.argv[3]): time.sleep(.05)
os.write(fd,b'fixture')
os.close(fd)
`
	writer := exec.CommandContext(ctx, "python3", "-c", program, fifo, ready, release)
	if err := writer.Start(); err != nil {
		_ = reader.Process.Kill()
		_ = reader.Wait()
		t.Fatal(err)
	}
	readerDone := make(chan error, 1)
	go func() { readerDone <- reader.Wait() }()
	defer func() {
		_ = os.WriteFile(release, nil, 0600)
		if err := writer.Wait(); err != nil {
			t.Errorf("FIFO writer: %v", err)
		}
		if err := <-readerDone; err != nil {
			t.Errorf("asset reader: %v", err)
		}
	}()
	deadline := time.Now().Add(20 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("shim reader did not open FIFO")
		}
		time.Sleep(50 * time.Millisecond)
	}
	probe := `import fcntl,sys
f=open(sys.argv[1], 'r+')
try:
 fcntl.lockf(f, fcntl.LOCK_EX | fcntl.LOCK_NB)
 print('free')
except BlockingIOError:
 print('held')
`
	out, err := exec.Command("python3", "-c", probe, filepath.Join(lockDir, ".install.lock")).CombinedOutput()
	if err != nil {
		t.Fatalf("lock probe: %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "held" {
		t.Fatalf("toolchain writer lock is %q while shim is being read", out)
	}
}
