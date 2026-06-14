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
