//go:build darwin

package apextest

import "syscall"

func statMtimeNanos(st *syscall.Stat_t) int64 { return st.Mtimespec.Nano() }
func statCtimeNanos(st *syscall.Stat_t) int64 { return st.Ctimespec.Nano() }
