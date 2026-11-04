package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectMarkersDoNotAuthorizeRuntimeAssets(t *testing.T) {
	project := t.TempDir()
	for _, rel := range []string{"go.mod", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs"} {
		path := filepath.Join(project, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("GLADE_HOME", "")
	t.Setenv("GLADE_ROOT", "")
	t.Setenv("GLADE_ALLOW_PROJECT_TOOLCHAIN", "")
	t.Chdir(project)
	for _, dirs := range [][]string{lightningStaticShimDirs(), lightningRuntimeAssetDirs("shell")} {
		for _, dir := range dirs {
			if dir == filepath.Join(project, "lwcruntime/src/shims") || dir == filepath.Join(project, "lwcruntime/src/shell") {
				t.Fatalf("project runtime authorized: %s", dir)
			}
		}
	}
	t.Setenv("GLADE_ROOT", project)
	if dirs := lightningRuntimeAssetDirs("shell"); len(dirs) == 0 || dirs[0] != filepath.Join(project, "lwcruntime/src/shell") {
		t.Fatalf("explicit runtime = %v", dirs)
	}
}
