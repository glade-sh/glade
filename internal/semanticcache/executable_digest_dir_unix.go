//go:build unix

package semanticcache

import (
	"os"
	"syscall"
)

// privateDigestDir reports whether dir is a real directory owned by this user
// and not writable by group or others, so no other account can plant an entry.
func privateDigestDir(dir string) bool {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0o022 != 0 {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Getuid() // #nosec G115 -- uid fits int.
}
