//go:build linux

package filestamp

import "syscall"

func mtimeNanos(st *syscall.Stat_t) int64 { return st.Mtim.Nano() }
func ctimeNanos(st *syscall.Stat_t) int64 { return st.Ctim.Nano() }

func trustedFilesystem(fs *syscall.Statfs_t) bool {
	switch int64(fs.Type) { // #nosec G115 -- magic numbers from statfs(2).
	case 0xEF53, // ext2/3/4
		0x58465342, // xfs
		0x9123683E, // btrfs
		0x01021994, // tmpfs
		0x794c7630, // overlayfs: only the upper layer is checked, see package doc
		0x2fc12fc1, // zfs
		0xF2F52010: // f2fs
		return true
	}
	return false
}
