//go:build darwin

package filestamp

import "syscall"

func mtimeNanos(st *syscall.Stat_t) int64 { return st.Mtimespec.Nano() }
func ctimeNanos(st *syscall.Stat_t) int64 { return st.Ctimespec.Nano() }
