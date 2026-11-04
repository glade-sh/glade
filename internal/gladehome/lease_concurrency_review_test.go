//go:build unix

package gladehome

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadLeasesOverlapAndFenceWriter(t *testing.T) {
	for _, mode := range []string{"active", "ensured", "managed"} {
		t.Run(mode, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "glade")
			t.Setenv("XDG_DATA_HOME", base)
			t.Setenv("GLADE_HOME", "")
			t.Setenv("GLADE_ROOT", "")
			for _, rel := range []string{"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs"} {
				writeMinimalToolchainFile(t, root, rel)
			}
			acquire := func() (func(), error) {
				switch mode {
				case "active":
					_, r, e := LeaseRoot()
					return r, e
				case "ensured":
					_, r, e := LeaseEnsuredRoot()
					return r, e
				default:
					return LeaseManagedRoot(root)
				}
			}
			first, err := acquire()
			if err != nil {
				t.Fatal(err)
			}
			type result struct {
				release func()
				err     error
			}
			done := make(chan result, 1)
			go func() { r, e := acquire(); done <- result{r, e} }()
			var second result
			select {
			case second = <-done:
			case <-time.After(time.Second):
				first()
				second = <-done
				if second.release != nil {
					second.release()
				}
				t.Fatal("second reader waited for an active read lease")
			}
			if second.err != nil {
				first()
				t.Fatal(second.err)
			}
			probe := func() string {
				t.Helper()
				out, e := exec.Command("python3", "-c", `import fcntl,sys
f=open(sys.argv[1], 'r+')
try: fcntl.lockf(f, fcntl.LOCK_EX | fcntl.LOCK_NB); print('free')
except BlockingIOError: print('held')`, filepath.Join(root, ".install.lock")).CombinedOutput()
				if e != nil {
					t.Fatalf("probe: %v %s", e, out)
				}
				return strings.TrimSpace(string(out))
			}
			if got := probe(); got != "held" {
				first()
				second.release()
				t.Fatalf("writer lock = %s with two readers", got)
			}
			writer := make(chan struct{})
			go func() { installMu.Lock(); close(writer); installMu.Unlock() }()
			select {
			case <-writer:
				first()
				second.release()
				t.Fatal("in-process writer entered during read leases")
			case <-time.After(50 * time.Millisecond):
			}
			first()
			if got := probe(); got != "held" {
				second.release()
				t.Fatalf("writer lock = %s after first reader exits", got)
			}
			second.release()
			select {
			case <-writer:
			case <-time.After(time.Second):
				t.Fatal("in-process writer did not enter after final reader")
			}
			if got := probe(); got != "free" {
				t.Fatalf("writer lock = %s after final reader exits", got)
			}
		})
	}
}

func TestReadLeaseAliasRetainsWriterFence(t *testing.T) {
	base := t.TempDir()
	physical := filepath.Join(base, "physical")
	alias := filepath.Join(base, "alias")
	if err := os.MkdirAll(physical, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(physical, alias); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(physical, "glade")
	t.Setenv("XDG_DATA_HOME", alias)
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	for _, rel := range []string{"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs"} {
		writeMinimalToolchainFile(t, root, rel)
	}
	_, first, err := LeaseRoot()
	if err != nil {
		t.Fatal(err)
	}
	second, err := LeaseManagedRoot(root)
	if err != nil {
		first()
		t.Fatal(err)
	}
	first()
	defer second()
	out, err := exec.Command("python3", "-c", `import fcntl,sys
f=open(sys.argv[1], 'r+')
try: fcntl.lockf(f, fcntl.LOCK_EX | fcntl.LOCK_NB); print('free')
except BlockingIOError: print('held')`, filepath.Join(root, ".install.lock")).CombinedOutput()
	if err != nil {
		t.Fatalf("probe: %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "held" {
		t.Fatalf("writer fence lost with alias lease still active: %s", out)
	}
}
