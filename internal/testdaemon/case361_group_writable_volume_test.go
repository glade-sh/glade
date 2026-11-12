//go:build darwin || linux

package testdaemon

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// A project on a volume whose root is group-writable must not use a
// project-local discovery directory. The daemon falls back to the per-user
// cache directory, created 0700, with one warning.
func TestCase361GroupWritableVolumeRootFallsBackToUserCache(t *testing.T) {
	volume, err := os.MkdirTemp("/tmp", "r361")
	if err != nil {
		t.Skip("no short temp directory")
	}
	defer os.RemoveAll(volume)
	if err := os.Chmod(volume, 0o775); err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("HOME", cache)
	var warn bytes.Buffer
	oldWriter := discoveryWarnWriter
	discoveryWarnWriter = &warn
	discoveryWarnOnce = sync.Once{}
	defer func() { discoveryWarnWriter = oldWriter; discoveryWarnOnce = sync.Once{} }()

	project := filepath.Join(volume, "proj")
	if err := os.Mkdir(project, 0o700); err != nil {
		t.Fatal(err)
	}
	socket := ServeSocketPath(project)
	if strings.HasPrefix(socket, volume) {
		t.Fatalf("socket %s stayed on the group-writable volume", socket)
	}
	userCache, err := os.UserCacheDir()
	if err != nil || !strings.HasPrefix(socket, filepath.Join(userCache, "glade")+string(filepath.Separator)) {
		t.Fatalf("socket %s is not under the user cache %s (%v)", socket, userCache, err)
	}
	if strings.Count(warn.String(), "\n") != 1 {
		t.Fatalf("want exactly one warning line, got %q", warn.String())
	}
	root, err := openPrivateDaemonDir(privateServeDirBase(t, project))
	if err != nil {
		t.Fatal(err)
	}
	_ = root.Close()
	info, err := os.Stat(filepath.Dir(socket))
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("daemon directory mode: %v %v", info, err)
	}
	if !secureDiscoveryPath(filepath.Dir(socket)) {
		t.Fatal("cache daemon directory failed owner and mode check")
	}
	// A directory loosened after creation is refused on the next use.
	if err := os.Chmod(filepath.Dir(socket), 0o770); err != nil {
		t.Fatal(err)
	}
	if secureDiscoveryPath(filepath.Dir(socket)) {
		t.Fatal("group-writable daemon directory accepted")
	}
}

func privateServeDirBase(t *testing.T, project string) (string, string) {
	t.Helper()
	return privateServeDir(project)
}

// A private volume root keeps the project-local directory.
func TestCase361PrivateRootKeepsLocalDiscovery(t *testing.T) {
	volume, err := os.MkdirTemp("/tmp", "r361")
	if err != nil {
		t.Skip("no short temp directory")
	}
	defer os.RemoveAll(volume)
	if err := os.Chmod(volume, 0o700); err != nil {
		t.Fatal(err)
	}
	socket := ServeSocketPath(filepath.Join(volume, "p"))
	if !strings.HasPrefix(socket, volume) {
		t.Fatalf("socket %s left a private root", socket)
	}
}

// When the project directory, the user cache directory and the temp root all
// fail the owner and mode check, discovery is refused: nothing is created,
// ValidateDiscoverySocket and NewServer return ErrUntrustedDiscovery, and the
// warning is printed once.
func TestCase361AllLocationsUntrustedRefuses(t *testing.T) {
	volume, err := os.MkdirTemp("/tmp", "r361")
	if err != nil {
		t.Skip("no short temp directory")
	}
	defer os.RemoveAll(volume)
	mk := func(name string) string {
		dir := filepath.Join(volume, name)
		if err := os.Mkdir(dir, 0o775); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, 0o775); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	home, tmp, project := mk("h"), mk("t"), mk("p")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", home)
	t.Setenv("TMPDIR", tmp)
	var warn bytes.Buffer
	oldWriter := discoveryWarnWriter
	discoveryWarnWriter = &warn
	discoveryWarnOnce = sync.Once{}
	defer func() { discoveryWarnWriter = oldWriter; discoveryWarnOnce = sync.Once{} }()

	socket := ServeSocketPath(project)
	if err := ValidateDiscoverySocket(socket); !errors.Is(err, ErrUntrustedDiscovery) {
		t.Fatalf("ValidateDiscoverySocket: %v", err)
	}
	srv, err := NewServer(ServerConfig{Root: project})
	if srv != nil {
		_ = srv.Close()
	}
	if !errors.Is(err, ErrUntrustedDiscovery) {
		t.Fatalf("NewServer: %v", err)
	}
	if strings.Count(warn.String(), "\n") != 1 {
		t.Fatalf("want one warning line, got %q", warn.String())
	}
	for _, dir := range []string{filepath.Dir(socket), filepath.Join(project, ".glade")} {
		if _, err := os.Lstat(dir); err == nil {
			t.Fatalf("created %s in an untrusted location", dir)
		}
	}
}

func untrustedFixture(t *testing.T) (home, tmp, project string) {
	t.Helper()
	volume, err := os.MkdirTemp("/tmp", "r361")
	if err != nil {
		t.Skip("no short temp directory")
	}
	t.Cleanup(func() { os.RemoveAll(volume) })
	mk := func(name string) string {
		dir := filepath.Join(volume, name)
		if err := os.Mkdir(dir, 0o775); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, 0o775); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	return mk("h"), mk("t"), mk("p")
}

// A custom socket must not make NewServer create the PID directory in an
// untrusted temp root, and its own directory is checked.
func TestCase361CustomSocketAllUntrustedCreatesNothing(t *testing.T) {
	home, tmp, project := untrustedFixture(t)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", home)
	t.Setenv("TMPDIR", tmp)
	custom := filepath.Join(project, "custom", "d.sock")
	srv, err := NewServer(ServerConfig{Root: project, Socket: custom})
	if srv != nil {
		_ = srv.Close()
	}
	if !errors.Is(err, ErrUntrustedDiscovery) {
		t.Fatalf("NewServer: %v", err)
	}
	for _, dir := range []string{filepath.Join(project, "custom"), filepath.Join(project, ".glade")} {
		if _, err := os.Lstat(dir); err == nil {
			t.Fatalf("created %s", dir)
		}
	}
	entries, _ := os.ReadDir(tmp)
	if len(entries) != 0 {
		t.Fatalf("created %d entries in the untrusted temp root", len(entries))
	}
}

// A group-writable parent of the user cache directory gets no new directory.
func TestCase361CacheBaseNotCreatedUnderWritableParent(t *testing.T) {
	home, _, _ := untrustedFixture(t)
	target := filepath.Join(home, "cache", "deeper")
	if ensureTrustedCacheBase(target) {
		t.Fatal("cache base accepted under a group-writable parent")
	}
	if _, err := os.Lstat(filepath.Join(home, "cache")); err == nil {
		t.Fatal("created a directory under a group-writable parent")
	}
}
