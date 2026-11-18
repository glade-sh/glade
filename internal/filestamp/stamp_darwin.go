//go:build darwin

package filestamp

import "syscall"

func mtimeNanos(st *syscall.Stat_t) int64 { return st.Mtimespec.Nano() }
func ctimeNanos(st *syscall.Stat_t) int64 { return st.Ctimespec.Nano() }

func trustedFilesystem(fs *syscall.Statfs_t) bool {
	var name []byte
	for _, c := range fs.Fstypename {
		if c == 0 {
			break
		}
		name = append(name, byte(c)) // #nosec G115 -- ASCII file system name.
	}
	switch string(name) {
	case "apfs", "hfs":
		return true
	}
	return false
}
