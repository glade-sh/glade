package compile

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
)

func TestCompilerHoldsToolchainLeaseUntilChildExits(t *testing.T)      { compilerLease(t, false) }
func TestCompilerBatchHoldsToolchainLeaseUntilChildExits(t *testing.T) { compilerLease(t, true) }

func compilerLease(t *testing.T, batch bool) {
	t.Helper()
	if os.Getenv("GLADE_TEST_COMPILER_CHILD") == "1" {
		root := os.Getenv("GLADE_TEST_ROOT")
		p := project.Project{Root: root}
		var err error
		if batch {
			p.LWCMetaFiles = []string{filepath.Join(root, "probe", "probe.js-meta.xml")}
			_, err = CompileBatch(p, Options{OutDir: filepath.Join(root, "out")})
		} else {
			_, err = Compile(p, Options{OutDir: filepath.Join(root, "out")})
		}
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	root := t.TempDir()
	share := filepath.Join(root, "xdg", "glade")
	for _, rel := range []string{
		"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js",
		"lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs",
		"lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs",
		"lwcruntime/src/lightning/button.mjs", ".install.lock",
	} {
		path := filepath.Join(share, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("// glade-compiler-protocol: two-phase-v1"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if batch {
		meta := filepath.Join(root, "probe", "probe.js-meta.xml")
		if err := os.MkdirAll(filepath.Dir(meta), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(meta, []byte(`<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion></LightningComponentBundle>`), 0644); err != nil {
			t.Fatal(err)
		}
	}
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	node := "#!/bin/sh\nif [ \"$2\" = \"--two-phase\" ]; then IFS= read -r cfg; printf '[]\\n'; IFS= read -r metadata; else cat >/dev/null; fi\nif [ \"$2\" = \"--salesforce-import-references\" ]; then printf '[]'; exit 0; fi\ntouch \"$GLADE_TEST_STARTED\"\nwhile [ ! -e \"$GLADE_TEST_RELEASE\" ]; do sleep 0.1; done\nprintf '{\"modules\":{},\"bundles\":{\"probe\":{\"modules\":{}}}}'\n"
	if err := os.WriteFile(filepath.Join(bin, "node"), []byte(node), 0755); err != nil {
		t.Fatal(err)
	}
	started, release := filepath.Join(root, "started"), filepath.Join(root, "release")
	child := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$")
	var childOutput bytes.Buffer
	child.Stdout = &childOutput
	child.Stderr = &childOutput
	child.Env = append(os.Environ(), "GLADE_TEST_COMPILER_CHILD=1", "GLADE_TEST_ROOT="+root, "GLADE_TEST_STARTED="+started, "GLADE_TEST_RELEASE="+release, "XDG_DATA_HOME="+filepath.Join(root, "xdg"), "GLADE_HOME=", "GLADE_ROOT=", "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- child.Wait()
	}()
	defer func() {
		_ = os.WriteFile(release, nil, 0644)
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("compiler child: %v: %s", err, childOutput.String())
			}
		case <-time.After(5 * time.Second):
			_ = child.Process.Kill()
			<-done
		}
	}()
	deadline := time.Now().Add(20 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("compiler child did not start")
		}
		select {
		case err := <-done:
			done <- err
			t.Fatalf("compiler child exited before starting: %v: %s", err, childOutput.String())
		default:
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
	out, err := exec.Command("python3", "-c", probe, filepath.Join(share, ".install.lock")).CombinedOutput()
	if err != nil {
		t.Fatalf("lock probe: %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "held" {
		t.Fatalf("toolchain writer lock is %q while compiler is running", out)
	}
}
