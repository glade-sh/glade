//go:build darwin || linux

package testdaemon

import (
	"bytes"
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
