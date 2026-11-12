//go:build linux

package filestamp

import "syscall"

func mtimeNanos(st *syscall.Stat_t) int64 { return st.Mtim.Nano() }
func ctimeNanos(st *syscall.Stat_t) int64 { return st.Ctim.Nano() }
