package compile

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
)

func TestCompilerCallsOverlap(t *testing.T) {
	base := t.TempDir()
	share := filepath.Join(base, "xdg", "glade")
	for _, rel := range []string{"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs"} {
		p := filepath.Join(share, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	bin, started, release := filepath.Join(base, "bin"), filepath.Join(base, "started"), filepath.Join(base, "release")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(started, 0755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncat >/dev/null\nif [ \"$2\" = \"--salesforce-import-references\" ]; then printf '[]'; exit 0; fi\ntouch \"$GLADE_TEST_STARTED/$$\"\nwhile [ ! -e \"$GLADE_TEST_RELEASE\" ]; do sleep 0.1; done\nprintf '{\"modules\":{}}'\n"
	if err := os.WriteFile(filepath.Join(bin, "node"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(base, "xdg"))
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	t.Setenv("GLADE_TEST_STARTED", started)
	t.Setenv("GLADE_TEST_RELEASE", release)
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		out := filepath.Join(base, fmt.Sprintf("out%d", i))
		go func() { _, err := Compile(project.Project{Root: base}, Options{OutDir: out}); done <- err }()
	}
	defer func() {
		_ = os.WriteFile(release, nil, 0644)
		for i := 0; i < 2; i++ {
			if err := <-done; err != nil {
				t.Errorf("compile: %v", err)
			}
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		entries, err := os.ReadDir(started)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("compiler processes did not overlap")
		}
		time.Sleep(25 * time.Millisecond)
	}
}
