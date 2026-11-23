package compile

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func emptyProjectToolchain(t *testing.T) string {
	t.Helper()
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "third_party", "lwc", "node_modules", "@lwc", "compiler", "package.json")); err != nil {
		t.Skip("run npm install in third_party/lwc")
	}
	return root
}

// A project without LWC sources must not start Node, and its manifest must
// match the file the compiler writes.
func TestCompileProjectWithoutBundlesSkipsNode(t *testing.T) {
	root := emptyProjectToolchain(t)
	projectRoot := t.TempDir()
	p := project.Project{Root: projectRoot}

	marker := filepath.Join(t.TempDir(), "node-ran")
	bin := t.TempDir()
	script := "#!/bin/sh\ntouch '" + marker + "'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "node"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	fast := filepath.Join(t.TempDir(), "out")
	manifest, err := Compile(p, Options{OutDir: fast, Namespace: "c", RepoRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("node started for a project with no LWC sources")
	}
	if manifest.Modules == nil || len(manifest.Modules) != 0 {
		t.Fatalf("modules = %#v", manifest.Modules)
	}

	// The compiler itself is the reference for the manifest bytes.
	t.Setenv("PATH", os.Getenv("PATH")[len(bin)+1:])
	reference := filepath.Join(t.TempDir(), "out")
	cfg, err := buildCompileConfig(p, projectRoot, reference, "c")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := json.Marshal(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := runCompiler(p, cfg, compileToolchainRootsFor(root)); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(reference, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(fast, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("manifest.json = %q, compiler wrote %q", got, want)
	}
}

// With the toolchain missing the compiler's own failure must still surface.
func TestCompileProjectWithoutBundlesKeepsToolchainFailure(t *testing.T) {
	root := emptyProjectToolchain(t)
	t.Setenv("GLADE_LWC_DEPS_DIR", t.TempDir())
	p := project.Project{Root: t.TempDir()}
	if _, err := Compile(p, Options{OutDir: filepath.Join(t.TempDir(), "out"), Namespace: "c", RepoRoot: root}); err == nil {
		t.Fatal("missing toolchain was accepted")
	}
}
