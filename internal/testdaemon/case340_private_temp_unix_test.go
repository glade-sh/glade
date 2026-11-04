//go:build darwin || linux

package testdaemon

import (
	"os"
	"path/filepath"
	"syscall"
)

// preparePrivateTestTemp gives tests using private daemon/cache paths a
// non-replaceable temporary ancestry, even when the runner uses a shared TMPDIR.
func preparePrivateTestTemp() func() error {
	oldMask := syscall.Umask(0027)
	restoreMask := func() error { syscall.Umask(oldMask); return nil }
	if testTempAncestryTrusted(os.TempDir()) {
		return restoreMask
	}
	home, err := os.UserHomeDir()
	if err != nil || !testTempAncestryTrusted(home) {
		return restoreMask
	}
	var fs syscall.Statfs_t
	if syscall.Statfs(home, &fs) != nil || fs.Bavail*uint64(fs.Bsize) < 50*1024*1024*1024 {
		return restoreMask
	}
	root, err := os.MkdirTemp(home, ".glade-test-")
	if err != nil {
		return restoreMask
	}
	oldTMP, hadTMP := os.LookupEnv("TMPDIR")
	oldGoTMP, hadGoTMP := os.LookupEnv("GOTMPDIR")
	if os.Setenv("TMPDIR", root) != nil || os.Setenv("GOTMPDIR", root) != nil {
		_ = os.RemoveAll(root)
		restoreTestTempEnv("TMPDIR", oldTMP, hadTMP)
		restoreTestTempEnv("GOTMPDIR", oldGoTMP, hadGoTMP)
		return restoreMask
	}
	return func() error {
		restoreTestTempEnv("TMPDIR", oldTMP, hadTMP)
		restoreTestTempEnv("GOTMPDIR", oldGoTMP, hadGoTMP)
		err := os.RemoveAll(root)
		syscall.Umask(oldMask)
		return err
	}
}

func restoreTestTempEnv(key, value string, existed bool) {
	if existed {
		_ = os.Setenv(key, value)
	} else {
		_ = os.Unsetenv(key)
	}
}

func testTempAncestryTrusted(path string) bool {
	absolute, err := filepath.Abs(path)
	if err != nil || !testTempPathTrusted(absolute) {
		return false
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	return err == nil && testTempPathTrusted(resolved)
}

func testTempPathTrusted(path string) bool {
	for {
		info, err := os.Lstat(path)
		if err != nil {
			return false
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || (stat.Uid != 0 && stat.Uid != uint32(os.Geteuid())) {
			return false
		}
		if info.Mode()&os.ModeSymlink == 0 && (!info.IsDir() || (info.Mode().Perm()&0022 != 0 && info.Mode()&os.ModeSticky == 0)) {
			return false
		}
		parent := filepath.Dir(path)
		if parent == path {
			return true
		}
		path = parent
	}
}
