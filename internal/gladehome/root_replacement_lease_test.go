//go:build unix

package gladehome

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRootWaitsWhenManagedToolchainIsBeingReplaced(t *testing.T) {
	if os.Getenv("GLADE_TEST_ROOT_CHILD") == "1" {
		_ = os.WriteFile(os.Getenv("GLADE_TEST_ROOT_STARTED"), nil, 0600)
		root, err := Root()
		value := root
		if err != nil {
			value = "error"
		}
		_ = os.WriteFile(os.Getenv("GLADE_TEST_ROOT_RESULT"), []byte(value), 0600)
		return
	}
	base := t.TempDir()
	share := filepath.Join(base, "glade")
	for _, rel := range []string{"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs", ".install.lock"} {
		writeMinimalToolchainFile(t, share, rel)
	}
	t.Setenv("XDG_DATA_HOME", base)
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	ready, release := filepath.Join(base, "ready"), filepath.Join(base, "release")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	holder := exec.CommandContext(ctx, "python3", "-c", `import fcntl,os,sys,time
f=open(sys.argv[1],'r+'); fcntl.lockf(f,fcntl.LOCK_EX)
open(sys.argv[2],'w').close()
while not os.path.exists(sys.argv[3]): time.sleep(.05)
`, filepath.Join(share, ".install.lock"), ready, release)
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.WriteFile(release, nil, 0600); _ = holder.Wait() }()
	waitFile := func(name string) {
		deadline := time.Now().Add(15 * time.Second)
		for {
			if _, err := os.Stat(name); err == nil {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", name)
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
	waitFile(ready)
	compile := filepath.Join(share, "third_party/lwc/compile.mjs")
	if err := os.Rename(compile, compile+".old"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Rename(compile+".old", compile) }()
	started, result := filepath.Join(base, "started"), filepath.Join(base, "result")
	reader := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRootWaitsWhenManagedToolchainIsBeingReplaced$")
	reader.Dir = base
	reader.Env = append(os.Environ(), "GLADE_TEST_ROOT_CHILD=1", "GLADE_TEST_ROOT_STARTED="+started, "GLADE_TEST_ROOT_RESULT="+result)
	if err := reader.Start(); err != nil {
		t.Fatal(err)
	}
	readerDone := make(chan error, 1)
	go func() { readerDone <- reader.Wait() }()
	waited := false
	defer func() {
		_ = os.Rename(compile+".old", compile)
		_ = os.WriteFile(release, nil, 0600)
		if !waited {
			<-readerDone
		}
	}()
	waitFile(started)
	time.Sleep(300 * time.Millisecond)
	if data, err := os.ReadFile(result); err == nil {
		t.Fatalf("Root returned during replacement: %s", data)
	}
	if err := os.Rename(compile+".old", compile); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	err := <-readerDone
	waited = true
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(result)
	if err != nil || string(got) != share {
		t.Fatalf("Root after replacement = %q, %v", got, err)
	}
}
