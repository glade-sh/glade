package compile

import (
	"path/filepath"
	"testing"
)

func TestProjectMarkersDoNotAuthorizeCompiler(t *testing.T) {
	managed := filepath.Join(t.TempDir(), "glade")
	project := t.TempDir()
	for _, root := range []string{managed, project} {
		for _, rel := range []string{"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/package.json", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs"} {
			writeCompileFixtureFile(t, filepath.Join(root, rel), "fixture")
		}
	}
	writeCompileFixtureFile(t, filepath.Join(project, "go.mod"), "module example.invalid/fixture")
	writeCompileFixtureFile(t, filepath.Join(project, "nested/.keep"), "")
	t.Setenv("XDG_DATA_HOME", filepath.Dir(managed))
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	t.Chdir(filepath.Join(project, "nested"))
	for _, value := range []string{"", "0", "true", " 1 "} {
		t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", value)
		roots, err := compileToolchainRoots()
		if err != nil {
			t.Fatal(err)
		}
		if roots.ScriptRoot != managed || roots.DependencyRoot != managed {
			t.Fatalf("opt-in %q: roots = %#v", value, roots)
		}
		if got := lwcCompilerCommand(roots).Args[1]; got != filepath.Join(managed, "third_party/lwc/compile.mjs") {
			t.Fatalf("command = %s", got)
		}
	}
	t.Setenv("GLADE_ROOT", project)
	roots, err := compileToolchainRoots()
	if err != nil || roots.ScriptRoot != project {
		t.Fatalf("explicit = %#v, %v", roots, err)
	}
	t.Setenv("GLADE_ROOT", "")
	t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "1")
	roots, err = compileToolchainRoots()
	if err != nil || roots.ScriptRoot != project {
		t.Fatalf("opt-in = %#v, %v", roots, err)
	}
}
