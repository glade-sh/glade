package gladehome

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRootFindsRepoCheckout(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var repo string
	for dir := wd; ; dir = filepath.Dir(dir) {
		if _, ok := validateRoot(dir); ok {
			repo = dir
			break
		}
		if dir == filepath.Dir(dir) {
			t.Skip("not inside glade repo with installed LWC toolchain")
		}
	}
	root, ok, err := findToolchainRoot(false)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || root != repo {
		t.Fatalf("findToolchainRoot = %q ok=%v want %q", root, ok, repo)
	}
	if _, err := RepoRoot(); err != nil {
		t.Fatal(err)
	}
}

func TestInstallFromRejectsSameSourceAndDestination(t *testing.T) {
	share := t.TempDir()
	t.Setenv("XDG_DATA_HOME", share)
	if err := InstallFrom(UserShareDir()); err == nil {
		t.Fatal("expected error when source equals destination")
	}
}

func TestInstallFromCWDSkipsGlobalShareAsSource(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var repo string
	for dir := wd; ; dir = filepath.Dir(dir) {
		if root, ok := validateRoot(dir); ok && hasGoMod(root) {
			repo = root
			break
		}
		if dir == filepath.Dir(dir) {
			t.Skip("not inside glade repo with installed LWC toolchain")
		}
	}
	dst := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dst)
	t.Setenv("GLADE_HOME", UserShareDir())
	if err := InstallFrom(repo); err != nil {
		t.Fatal(err)
	}
	if _, ok := validateRoot(UserShareDir()); !ok {
		t.Fatalf("toolchain not installed at %s", UserShareDir())
	}
}

func TestInstallFromCopiesToolchain(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var repo string
	for dir := wd; ; dir = filepath.Dir(dir) {
		if _, ok := validateRoot(dir); ok {
			repo = dir
			break
		}
		if dir == filepath.Dir(dir) {
			t.Skip("not inside glade repo with installed LWC toolchain")
		}
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if err := InstallFrom(repo); err != nil {
		t.Fatal(err)
	}
	if _, ok := validateRoot(UserShareDir()); !ok {
		t.Fatalf("installed toolchain missing at %s", UserShareDir())
	}
	for _, rel := range []string{
		"lwcruntime/src/experience/cmsDeliveryApi.mjs",
		"lwcruntime/src/lightning/button.mjs",
		"lwcruntime/src/shell/app.mjs",
		"lwcruntime/src/shims/lds-cache.mjs",
		"lwcruntime/src/shims/wire-adapter.mjs",
		"lwcruntime/src/slds/slds-loader.mjs",
	} {
		if _, err := os.Stat(filepath.Join(UserShareDir(), filepath.FromSlash(rel))); err != nil {
			t.Fatalf("installed toolchain missing %s: %v", rel, err)
		}
	}
}

func TestInstallFromCopiesEditorVSIX(t *testing.T) {
	repo := t.TempDir()
	writeMinimalToolchainFile(t, repo, "third_party/lwc/compile.mjs")
	writeMinimalToolchainFile(t, repo, "third_party/lwc/node_modules/@lwc/compiler/package.json")
	writeMinimalToolchainFile(t, repo, "lwcruntime/src/experience/cmsDeliveryApi.mjs")
	writeMinimalToolchainFile(t, repo, "lwcruntime/src/shims/wire-adapter.mjs")
	writeMinimalToolchainFile(t, repo, "lwcruntime/src/shims/lds-cache.mjs")
	writeMinimalToolchainFile(t, repo, "lwcruntime/src/shell/app.mjs")
	writeMinimalToolchainFile(t, repo, "lwcruntime/src/slds/slds-loader.mjs")
	writeMinimalToolchainFile(t, repo, "lwcruntime/src/lightning/button.mjs")
	sourceVSIX := filepath.Join(repo, "contrib", "vscode-glade", "dist", "vscode-glade-0.0.1.vsix")
	if err := os.MkdirAll(filepath.Dir(sourceVSIX), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourceVSIX, []byte("fresh-vsix"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	if err := InstallFrom(repo); err != nil {
		t.Fatal(err)
	}

	installedVSIX := filepath.Join(UserShareDir(), "editor", "vscode-glade.vsix")
	got, err := os.ReadFile(installedVSIX)
	if err != nil {
		t.Fatalf("installed editor vsix missing: %v", err)
	}
	if string(got) != "fresh-vsix" {
		t.Fatalf("installed editor vsix = %q, want fresh-vsix", string(got))
	}
}

func writeMinimalToolchainFile(t *testing.T, root, rel string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(rel), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureRootHonorsExplicitGladeHomeBeforeUserShare(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var repo string
	for dir := wd; ; dir = filepath.Dir(dir) {
		if root, ok := validateRoot(dir); ok && hasGoMod(root) {
			repo = root
			break
		}
		if dir == filepath.Dir(dir) {
			t.Skip("not inside glade repo with installed LWC toolchain")
		}
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if err := InstallFrom(repo); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GLADE_HOME", repo)

	root, err := EnsureRoot()
	if err != nil {
		t.Fatal(err)
	}
	if root != filepath.Clean(repo) {
		t.Fatalf("root = %s, want %s", root, repo)
	}
}

func TestToolchainStatusHonorsExplicitGladeHomeBeforeUserShare(t *testing.T) {
	explicit := t.TempDir()
	for _, rel := range []string{
		"third_party/lwc/compile.mjs",
		"third_party/lwc/node_modules/@lwc/compiler/package.json",
		"lwcruntime/src/shims/wire-adapter.mjs",
		"lwcruntime/src/shims/lds-cache.mjs",
		"lwcruntime/src/shell/app.mjs",
		"lwcruntime/src/slds/slds-loader.mjs",
		"lwcruntime/src/lightning/button.mjs",
	} {
		writeMinimalToolchainFile(t, explicit, rel)
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	global := UserShareDir()
	for _, rel := range []string{
		"third_party/lwc/compile.mjs",
		"third_party/lwc/node_modules/@lwc/compiler/package.json",
		"lwcruntime/src/shims/wire-adapter.mjs",
		"lwcruntime/src/shims/lds-cache.mjs",
		"lwcruntime/src/shell/app.mjs",
		"lwcruntime/src/slds/slds-loader.mjs",
		"lwcruntime/src/lightning/button.mjs",
	} {
		writeMinimalToolchainFile(t, global, rel)
	}
	t.Setenv("GLADE_HOME", explicit)

	path, ok, detail := ToolchainStatus()
	if !ok || path != filepath.Clean(explicit) || detail != "ok (explicit)" {
		t.Fatalf("ToolchainStatus = (%q, %v, %q), want (%q, true, %q)", path, ok, detail, filepath.Clean(explicit), "ok (explicit)")
	}
}
