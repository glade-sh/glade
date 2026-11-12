package compile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// An explicitly selected shared dependency directory is independent of the
// trusted compiler script root. The default continues to use the managed root.
func TestLWCCompilerSharedDependencyOverride(t *testing.T) {
	base := resolvedTempDir(t)
	bin := filepath.Join(base, "bin")
	managed := filepath.Join(base, "managed", "third_party", "lwc")
	shared := filepath.Join(base, "shared")
	for _, dir := range []string{bin, managed, shared} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(bin, "node"), []byte("#!/bin/sh\nprintf '%s\\n%s\\n' \"$PWD\" \"$GLADE_LWC_TOOLCHAIN_DIR\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	roots := compileRoots{ScriptRoot: filepath.Join(base, "scripts"), DependencyRoot: filepath.Join(base, "managed")}
	scriptPath := filepath.Join(roots.ScriptRoot, "third_party", "lwc", "compile.mjs")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scriptPath, []byte("// glade-compiler-protocol: two-phase-v1"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, override, want string }{
		{"default", "", managed},
		{"shared", shared, shared},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GLADE_LWC_DEPS_DIR", tc.override)
			cmd := lwcCompilerCommand(roots)
			if got := cmd.Args[1]; got != filepath.Join(roots.ScriptRoot, "third_party", "lwc", "compile.mjs") {
				t.Fatalf("compiler script = %q", got)
			}
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("node launch: %v: %s", err, out)
			}
			if got, want := strings.TrimSpace(string(out)), tc.want+"\n"+tc.want; got != want {
				t.Fatalf("working directory and toolchain directory = %q, want %q", got, want)
			}
		})
	}
}

// Both compiler invocations, including the import-reference pass, must use
// the selected dependency directory; their script continues to come from roots.
func TestLWCCompilerSharedDependencyOverrideBothPasses(t *testing.T) {
	base := resolvedTempDir(t)
	bin, shared := filepath.Join(base, "bin"), filepath.Join(base, "shared")
	for _, dir := range []string{bin, shared} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	trace := filepath.Join(base, "trace")
	script := `#!/bin/sh
if [ "$PWD" != "$EXPECTED_DEPS" ] || [ "$GLADE_LWC_TOOLCHAIN_DIR" != "$EXPECTED_DEPS" ]; then exit 13; fi
if [ "$2" = "--two-phase" ]; then
  IFS= read -r config || exit 14
  printf 'references\n' >> "$TRACE_FILE"
  printf '[]\n'
  IFS= read -r metadata || exit 15
  printf 'compile\n' >> "$TRACE_FILE"
  printf '{"modules":{}}'
elif [ "$2" = "--salesforce-import-references" ]; then printf 'references\n' >> "$TRACE_FILE"; printf '[]'; else printf 'compile\n' >> "$TRACE_FILE"; printf '{"modules":{}}'; fi
`
	if err := os.WriteFile(filepath.Join(bin, "node"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GLADE_LWC_DEPS_DIR", shared)
	t.Setenv("EXPECTED_DEPS", shared)
	t.Setenv("TRACE_FILE", trace)
	roots := compileRoots{ScriptRoot: filepath.Join(base, "scripts"), DependencyRoot: filepath.Join(base, "managed")}
	if _, err := runCompiler(project.Project{Root: base}, compileConfig{}, roots); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "references\ncompile\n" {
		t.Fatalf("compiler passes = %q", got)
	}
}

func TestLWCSharedDependencyOverrideSourceRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "source")
	for _, rel := range []string{"go.mod", "third_party/lwc/compile.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/slds/slds-loader.mjs"} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("synthetic"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", root)
	t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "")
	t.Setenv("GLADE_LWC_DEPS_DIR", t.TempDir())
	got, err := FindRepoRoot()
	if err != nil || got != root {
		t.Fatalf("explicit source root = %q, %v; want %q", got, err, root)
	}
}

// Installed compiler scripts from earlier releases still use two invocations.
func TestLWCCompilerLegacyScriptProtocol(t *testing.T) {
	base := resolvedTempDir(t)
	bin := filepath.Join(base, "bin")
	scriptRoot := filepath.Join(base, "scripts")
	script := filepath.Join(scriptRoot, "third_party", "lwc", "compile.mjs")
	if err := os.MkdirAll(filepath.Dir(script), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("// legacy compiler\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	node := `#!/bin/sh
printf '%s\n' "$2" >> "$TRACE_FILE"
if [ "$2" = "--salesforce-import-references" ]; then printf '[]'; elif [ "$2" = "--two-phase" ]; then printf '[]'; else printf '{"modules":{}}'; fi
`
	if err := os.WriteFile(filepath.Join(bin, "node"), []byte(node), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GLADE_LWC_DEPS_DIR", base)
	trace := filepath.Join(base, "trace")
	t.Setenv("TRACE_FILE", trace)
	if _, err := runCompiler(project.Project{Root: base}, compileConfig{}, compileRoots{ScriptRoot: scriptRoot, DependencyRoot: base}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "--salesforce-import-references\n\n" {
		t.Fatalf("legacy phases = %q", got)
	}
}

func TestLWCCompilerProtocolFailures(t *testing.T) {
	for _, tc := range []struct{ name, script, want string }{
		{"reference failure", "printf 'reference failure\\n' >&2; exit 12", "reference failure"},
		{"invalid references", "printf 'not-json\\n'", "decode Salesforce import references"},
		{"compile failure", "IFS= read -r cfg; printf '[]\\n'; IFS= read -r metadata; printf 'compile failure\\n' >&2; exit 12", "compile failure"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := resolvedTempDir(t)
			bin, root := filepath.Join(base, "bin"), filepath.Join(base, "script")
			marker := filepath.Join(root, "third_party", "lwc", "compile.mjs")
			if err := os.MkdirAll(filepath.Dir(marker), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(marker, []byte("// glade-compiler-protocol: two-phase-v1"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(bin, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(bin, "node"), []byte("#!/bin/sh\n"+tc.script+"\n"), 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("GLADE_LWC_DEPS_DIR", base)
			_, err := runCompiler(project.Project{Root: base}, compileConfig{}, compileRoots{ScriptRoot: root, DependencyRoot: base})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}

// resolvedTempDir returns a temporary directory with symlinks resolved, so
// path comparisons hold where the temporary root is itself a symlink.
func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
